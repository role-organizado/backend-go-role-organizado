package admin_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainadmin "github.com/role-organizado/backend-go-role-organizado/internal/domain/admin"
	"github.com/role-organizado/backend-go-role-organizado/internal/domain/config"
	"github.com/role-organizado/backend-go-role-organizado/internal/domain/event"
	"github.com/role-organizado/backend-go-role-organizado/internal/domain/outbound"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	"github.com/role-organizado/backend-go-role-organizado/internal/usecase/admin"
)

var errNotFound = errors.New("not found")

func ctx() context.Context { return context.Background() }

// ---- Dashboard ----

func TestGetDashboardStats(t *testing.T) {
	m := &fakeMetricsRepo{counts: domainadmin.DashboardCounts{TotalUsuarios: 5, TotalEventos: 3}}
	got, err := admin.NewGetDashboardStats(m).Execute(ctx())
	require.NoError(t, err)
	assert.Equal(t, int64(5), got.TotalUsuarios)
	assert.Equal(t, int64(3), got.TotalEventos)
}

func TestGetDashboardHealth(t *testing.T) {
	up, err := admin.NewGetDashboardHealth(&fakeMetricsRepo{pingErr: nil}).Execute(ctx())
	require.NoError(t, err)
	assert.True(t, up)

	down, err := admin.NewGetDashboardHealth(&fakeMetricsRepo{pingErr: errors.New("db down")}).Execute(ctx())
	require.NoError(t, err)
	assert.False(t, down)
}

func TestGetDashboardFinance_ErrorFallsBackToZero(t *testing.T) {
	m := &fakeMetricsRepo{financeErr: errors.New("agg failed")}
	got, err := admin.NewGetDashboardFinance(m).Execute(ctx())
	require.NoError(t, err)
	assert.Equal(t, 0, got["totalEventos"])
}

func TestGetDashboardFinance_ReturnsTotals(t *testing.T) {
	m := &fakeMetricsRepo{finance: map[string]any{"totalEventos": 7}}
	got, err := admin.NewGetDashboardFinance(m).Execute(ctx())
	require.NoError(t, err)
	assert.Equal(t, 7, got["totalEventos"])
}

// ---- Pending outbound ----

func TestGetPendingOutbound_AggregatesAndSorts(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	t1 := time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)
	evRepo := &fakeEventoRepo{
		all: []event.Evento{
			{ID: "ev-small", Nome: "Small"},
			{ID: "ev-big", Nome: "Big"},
			{ID: "ev-none", Nome: "None"},
		},
		total: 3,
	}
	outRepo := &fakeOutboundRepo{pending: map[string][]outbound.OutboundRequest{
		"ev-small": {{ID: "a", AmountCents: 100, CreatedAt: t1}},
		"ev-big":   {{ID: "b", AmountCents: 500, CreatedAt: t0}, {ID: "c", AmountCents: 200, CreatedAt: t1}},
	}}

	got, err := admin.NewGetPendingOutbound(evRepo, outRepo).Execute(ctx(), 20)
	require.NoError(t, err)

	assert.Equal(t, 2, got.TotalEventsWithPending)
	assert.Equal(t, 3, got.TotalPendingRequests)
	assert.Equal(t, int64(800), got.TotalPendingAmountCents)
	// sorted by amount desc → big (700) first
	require.Len(t, got.Items, 2)
	assert.Equal(t, "ev-big", got.Items[0].EventID)
	assert.Equal(t, int64(700), got.Items[0].PendingAmountCents)
	assert.Equal(t, 2, got.Items[0].PendingCount)
	require.NotNil(t, got.Items[0].OldestPendingAt)
	assert.Equal(t, t0, *got.Items[0].OldestPendingAt)
}

func TestGetPendingOutbound_RespectsLimit(t *testing.T) {
	evRepo := &fakeEventoRepo{
		all: []event.Evento{{ID: "e1"}, {ID: "e2"}, {ID: "e3"}},
		total: 3,
	}
	outRepo := &fakeOutboundRepo{pending: map[string][]outbound.OutboundRequest{
		"e1": {{AmountCents: 300}},
		"e2": {{AmountCents: 200}},
		"e3": {{AmountCents: 100}},
	}}
	got, err := admin.NewGetPendingOutbound(evRepo, outRepo).Execute(ctx(), 2)
	require.NoError(t, err)
	assert.Equal(t, 3, got.TotalEventsWithPending)
	assert.Len(t, got.Items, 2) // truncated to limit
}

// ---- Feature flags ----

func TestListFeatureFlags(t *testing.T) {
	repo := &fakeFlagRepo{all: []domainadmin.FeatureFlag{{Chave: "x", Enabled: true}}}
	got, err := admin.NewListFeatureFlags(repo).Execute(ctx())
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "x", got[0].Chave)
}

func TestUpdateFeatureFlag(t *testing.T) {
	enabled := true
	repo := &fakeFlagRepo{updated: &domainadmin.FeatureFlag{Chave: "x", Enabled: true}}
	got, err := admin.NewUpdateFeatureFlag(repo).Execute(ctx(), "x", domainadmin.FeatureFlagUpdate{Enabled: &enabled})
	require.NoError(t, err)
	assert.Equal(t, "x", got.Chave)
	assert.Equal(t, "x", repo.lastChave)
	require.NotNil(t, repo.lastUpd.Enabled)
	assert.True(t, *repo.lastUpd.Enabled)
}

// ---- Approvals ----

func TestCountPendingApprovals_ErrorReturnsZero(t *testing.T) {
	repo := &fakeApprovalRepo{countErr: errors.New("boom")}
	got, err := admin.NewCountPendingApprovals(repo).Execute(ctx(), "u1")
	require.NoError(t, err)
	assert.Equal(t, int64(0), got)
}

func TestCountPendingApprovals(t *testing.T) {
	repo := &fakeApprovalRepo{count: 4}
	got, err := admin.NewCountPendingApprovals(repo).Execute(ctx(), "u1")
	require.NoError(t, err)
	assert.Equal(t, int64(4), got)
}

func TestListPendingApprovals_ErrorReturnsEmpty(t *testing.T) {
	repo := &fakeApprovalRepo{pendingErr: errors.New("boom")}
	got, err := admin.NewListPendingApprovals(repo).Execute(ctx(), "u1")
	require.NoError(t, err)
	assert.Empty(t, got)
	assert.NotNil(t, got)
}

func TestListApprovalHistory(t *testing.T) {
	repo := &fakeApprovalRepo{history: []domainadmin.ApprovalItem{{ID: "1", Status: "APROVADO"}}}
	got, err := admin.NewListApprovalHistory(repo).Execute(ctx(), "u1")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "APROVADO", got[0].Status)
}

// ---- Dominios extras ----

func TestToggleDominio(t *testing.T) {
	repo := &fakeDominioRepo{byID: map[string]*config.Dominio{
		"d1": {ID: "d1", Ativo: true},
	}}
	got, err := admin.NewToggleDominio(repo).Execute(ctx(), "d1")
	require.NoError(t, err)
	assert.False(t, got.Ativo)
	require.NotNil(t, repo.saved)
	assert.False(t, repo.saved.Ativo)
}

func TestToggleDominio_NotFound(t *testing.T) {
	repo := &fakeDominioRepo{byID: map[string]*config.Dominio{}}
	_, err := admin.NewToggleDominio(repo).Execute(ctx(), "missing")
	require.Error(t, err)
}

func TestListDominioCategorias_DistinctSorted(t *testing.T) {
	repo := &fakeDominioRepo{all: []config.Dominio{
		{Categoria: "metodo_pagamento"},
		{Categoria: "tipo_evento"},
		{Categoria: "metodo_pagamento"},
		{Categoria: ""},
	}}
	got, err := admin.NewListDominioCategorias(repo).Execute(ctx())
	require.NoError(t, err)
	assert.Equal(t, []string{"metodo_pagamento", "tipo_evento"}, got)
}

// ---- Cancellation policies ----

func TestListCancelamentoPolicies(t *testing.T) {
	repo := &fakeDominioRepo{byCat: map[string][]config.Dominio{
		"politica_cancelamento": {{ID: "p1", Categoria: "politica_cancelamento"}},
	}}
	got, err := admin.NewListCancelamentoPolicies(repo).Execute(ctx())
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "p1", got[0].ID)
}

func TestUpdateCancelamentoTiers_Valid(t *testing.T) {
	repo := &fakeDominioRepo{byID: map[string]*config.Dominio{
		"p1": {ID: "p1", Categoria: "politica_cancelamento"},
	}}
	tiers := []domainadmin.CancellationTier{
		{TriggerType: "DAYS_BEFORE_EVENT", Threshold: 7, RefundPercent: 100, Label: "1 semana"},
	}
	got, err := admin.NewUpdateCancelamentoTiers(repo).Execute(ctx(), "p1", tiers)
	require.NoError(t, err)
	require.NotNil(t, got.Metadata["tiers"])
	saved := got.Metadata["tiers"].([]map[string]any)
	require.Len(t, saved, 1)
	assert.Equal(t, "DAYS_BEFORE_EVENT", saved[0]["triggerType"])
	assert.Equal(t, 100, saved[0]["refundPercent"])
}

func TestUpdateCancelamentoTiers_Empty(t *testing.T) {
	repo := &fakeDominioRepo{}
	_, err := admin.NewUpdateCancelamentoTiers(repo).Execute(ctx(), "p1", nil)
	require.Error(t, err)
}

func TestUpdateCancelamentoTiers_InvalidTier(t *testing.T) {
	repo := &fakeDominioRepo{}
	tiers := []domainadmin.CancellationTier{
		{TriggerType: "BOGUS", Threshold: 1, RefundPercent: 50, Label: "x"},
	}
	_, err := admin.NewUpdateCancelamentoTiers(repo).Execute(ctx(), "p1", tiers)
	require.Error(t, err)
}

func TestUpdateCancelamentoTiers_WrongCategoria(t *testing.T) {
	repo := &fakeDominioRepo{byID: map[string]*config.Dominio{
		"d1": {ID: "d1", Categoria: "tipo_evento"},
	}}
	tiers := []domainadmin.CancellationTier{
		{TriggerType: "DAYS_BEFORE_EVENT", Threshold: 7, RefundPercent: 100, Label: "x"},
	}
	_, err := admin.NewUpdateCancelamentoTiers(repo).Execute(ctx(), "d1", tiers)
	require.Error(t, err)
}

// ---- Eventos admin ----

func TestListEventosAdmin_NoFilter(t *testing.T) {
	repo := &fakeEventoRepo{
		all:   []event.Evento{{ID: "e1"}, {ID: "e2"}},
		total: 10,
	}
	got, err := admin.NewListEventosAdmin(repo).Execute(ctx(), portin.ListEventosAdminInput{Page: 0, PageSize: 2})
	require.NoError(t, err)
	assert.Len(t, got.Eventos, 2)
	assert.Equal(t, int64(10), got.TotalCount)
	assert.True(t, got.HasMore)
}

func TestListEventosAdmin_StatusFilter(t *testing.T) {
	repo := &fakeEventoRepo{
		all: []event.Evento{
			{ID: "e1", Status: event.EventoStatusPublicado},
			{ID: "e2", Status: event.EventoStatusCancelado},
			{ID: "e3", Status: event.EventoStatusPublicado},
		},
		total: 3,
	}
	got, err := admin.NewListEventosAdmin(repo).Execute(ctx(), portin.ListEventosAdminInput{Status: "PUBLICADO"})
	require.NoError(t, err)
	assert.Len(t, got.Eventos, 2)
	assert.Equal(t, int64(2), got.TotalCount)
	assert.False(t, got.HasMore)
}

func TestListEventosAdmin_NomeFilterCaseInsensitive(t *testing.T) {
	repo := &fakeEventoRepo{
		all: []event.Evento{
			{ID: "e1", Nome: "Festa Junina"},
			{ID: "e2", Nome: "Casamento"},
		},
		total: 2,
	}
	got, err := admin.NewListEventosAdmin(repo).Execute(ctx(), portin.ListEventosAdminInput{Nome: "festa"})
	require.NoError(t, err)
	require.Len(t, got.Eventos, 1)
	assert.Equal(t, "e1", got.Eventos[0].ID)
}

func TestCancelarEventoAdmin(t *testing.T) {
	repo := &fakeEventoRepo{byID: map[string]*event.Evento{
		"e1": {ID: "e1", Status: event.EventoStatusPublicado},
	}}
	got, err := admin.NewCancelarEventoAdmin(repo).Execute(ctx(), portin.CancelarEventoAdminInput{
		EventoID: "e1", Motivo: "fraude", AdminUserID: "admin1",
	})
	require.NoError(t, err)
	assert.Equal(t, event.EventoStatusCancelado, got.Status)
}

func TestCancelarEventoAdmin_EmptyMotivo(t *testing.T) {
	repo := &fakeEventoRepo{}
	_, err := admin.NewCancelarEventoAdmin(repo).Execute(ctx(), portin.CancelarEventoAdminInput{EventoID: "e1"})
	require.Error(t, err)
}

func TestCancelarEventoAdmin_AlreadyCancelled(t *testing.T) {
	repo := &fakeEventoRepo{byID: map[string]*event.Evento{
		"e1": {ID: "e1", Status: event.EventoStatusCancelado},
	}}
	_, err := admin.NewCancelarEventoAdmin(repo).Execute(ctx(), portin.CancelarEventoAdminInput{
		EventoID: "e1", Motivo: "x",
	})
	require.Error(t, err)
}

func TestFecharFinanceiroAdmin(t *testing.T) {
	repo := &fakeEventoRepo{byID: map[string]*event.Evento{
		"e1": {ID: "e1", Fase: event.FaseExecucao},
	}}
	got, err := admin.NewFecharFinanceiroAdmin(repo).Execute(ctx(), "e1")
	require.NoError(t, err)
	assert.Equal(t, event.FaseFinalizado, got.Fase)
}

func TestGetEventoCompletoAdmin_DegradesGracefully(t *testing.T) {
	evRepo := &fakeEventoRepo{byID: map[string]*event.Evento{
		"e1": {ID: "e1", Nome: "Festa", UsuarioID: ""},
	}}
	got, err := admin.NewGetEventoCompletoAdmin(evRepo, &fakeUsuarioRepo{}, &fakeFinanceRepo{}, &fakeOutboundRepo{}).Execute(ctx(), "e1")
	require.NoError(t, err)
	require.NotNil(t, got.Evento)
	assert.Equal(t, "Festa", got.Evento.Nome)
	assert.Nil(t, got.Organizador)
	assert.Nil(t, got.Finance)
}

// ---- Reconciliation reports ----

func TestListReconciliationReports_ClampsLimit(t *testing.T) {
	repo := &fakeReconReader{recent: []domainadmin.ReconciliationReport{{ID: "r1"}}}
	got, err := admin.NewListReconciliationReports(repo).Execute(ctx(), 0)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, 20, repo.lastLimit) // 0 → default 20
}

func TestGetLatestReconciliationReport(t *testing.T) {
	repo := &fakeReconReader{latest: &domainadmin.ReconciliationReport{ID: "r9"}}
	got, err := admin.NewGetLatestReconciliationReport(repo).Execute(ctx())
	require.NoError(t, err)
	assert.Equal(t, "r9", got.ID)
}

func TestGetLatestReconciliationReport_NotFound(t *testing.T) {
	repo := &fakeReconReader{latestErr: errNotFound}
	_, err := admin.NewGetLatestReconciliationReport(repo).Execute(ctx())
	require.Error(t, err)
}
