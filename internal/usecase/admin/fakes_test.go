package admin_test

import (
	"context"

	domainadmin "github.com/role-organizado/backend-go-role-organizado/internal/domain/admin"
	"github.com/role-organizado/backend-go-role-organizado/internal/domain/config"
	"github.com/role-organizado/backend-go-role-organizado/internal/domain/event"
	"github.com/role-organizado/backend-go-role-organizado/internal/domain/finance"
	"github.com/role-organizado/backend-go-role-organizado/internal/domain/outbound"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
)

// The fakes embed the port interface so they satisfy it at compile time while
// only overriding the methods exercised by a given test. Unimplemented methods
// panic (nil interface) if accidentally called — which surfaces as a test failure.

// ---- DominioRepository ----

type fakeDominioRepo struct {
	portout.DominioRepository
	byID       map[string]*config.Dominio
	all        []config.Dominio
	byCat      map[string][]config.Dominio
	saved      *config.Dominio
	findByIDErr error
}

func (f *fakeDominioRepo) FindByID(_ context.Context, id string) (*config.Dominio, error) {
	if f.findByIDErr != nil {
		return nil, f.findByIDErr
	}
	d, ok := f.byID[id]
	if !ok {
		return nil, errNotFound
	}
	return d, nil
}
func (f *fakeDominioRepo) FindAll(_ context.Context) ([]config.Dominio, error) {
	return f.all, nil
}
func (f *fakeDominioRepo) FindByCategoria(_ context.Context, cat string) ([]config.Dominio, error) {
	return f.byCat[cat], nil
}
func (f *fakeDominioRepo) Save(_ context.Context, d *config.Dominio) (*config.Dominio, error) {
	f.saved = d
	return d, nil
}

// ---- EventoRepository ----

type fakeEventoRepo struct {
	portout.EventoRepository
	all     []event.Evento
	total   int64
	byID    map[string]*event.Evento
	updated *event.Evento
}

func (f *fakeEventoRepo) FindAll(_ context.Context, _, _ int) ([]event.Evento, int64, error) {
	return f.all, f.total, nil
}
func (f *fakeEventoRepo) FindByID(_ context.Context, id string) (*event.Evento, error) {
	d, ok := f.byID[id]
	if !ok {
		return nil, errNotFound
	}
	// return a copy so use cases mutate a fresh struct
	cp := *d
	return &cp, nil
}
func (f *fakeEventoRepo) Update(_ context.Context, e *event.Evento) (*event.Evento, error) {
	f.updated = e
	return e, nil
}

// ---- OutboundRequestRepository ----

type fakeOutboundRepo struct {
	portout.OutboundRequestRepository
	pending map[string][]outbound.OutboundRequest
}

func (f *fakeOutboundRepo) FindPendingByEventID(_ context.Context, eventID string) ([]outbound.OutboundRequest, error) {
	return f.pending[eventID], nil
}

// ---- UsuarioRepository ----

type fakeUsuarioRepo struct {
	portout.UsuarioRepository
}

// ---- FinanceSummaryRepository ----

type fakeFinanceRepo struct {
	portout.FinanceSummaryRepository
}

func (f *fakeFinanceRepo) FindByEventID(_ context.Context, _ string) (*finance.FinanceSummary, error) {
	return nil, errNotFound
}

// ---- AdminMetricsRepository ----

type fakeMetricsRepo struct {
	counts     domainadmin.DashboardCounts
	pingErr    error
	finance    map[string]any
	financeErr error
}

func (f *fakeMetricsRepo) DashboardCounts(_ context.Context) (domainadmin.DashboardCounts, error) {
	return f.counts, nil
}
func (f *fakeMetricsRepo) Ping(_ context.Context) error { return f.pingErr }
func (f *fakeMetricsRepo) FinanceSummaryTotals(_ context.Context) (map[string]any, error) {
	return f.finance, f.financeErr
}

// ---- FeatureFlagRepository ----

type fakeFlagRepo struct {
	all       []domainadmin.FeatureFlag
	updated   *domainadmin.FeatureFlag
	lastChave string
	lastUpd   domainadmin.FeatureFlagUpdate
	updErr    error
}

func (f *fakeFlagRepo) FindAll(_ context.Context) ([]domainadmin.FeatureFlag, error) {
	return f.all, nil
}
func (f *fakeFlagRepo) Update(_ context.Context, chave string, upd domainadmin.FeatureFlagUpdate) (*domainadmin.FeatureFlag, error) {
	f.lastChave = chave
	f.lastUpd = upd
	if f.updErr != nil {
		return nil, f.updErr
	}
	return f.updated, nil
}

// ---- ApprovalRepository ----

type fakeApprovalRepo struct {
	count      int64
	countErr   error
	pending    []domainadmin.ApprovalItem
	pendingErr error
	history    []domainadmin.ApprovalItem
}

func (f *fakeApprovalRepo) CountPending(_ context.Context, _ string) (int64, error) {
	return f.count, f.countErr
}
func (f *fakeApprovalRepo) FindPending(_ context.Context, _ string) ([]domainadmin.ApprovalItem, error) {
	return f.pending, f.pendingErr
}
func (f *fakeApprovalRepo) FindHistory(_ context.Context, _ string) ([]domainadmin.ApprovalItem, error) {
	return f.history, nil
}

// ---- ReconciliationReportReader ----

type fakeReconReader struct {
	recent    []domainadmin.ReconciliationReport
	latest    *domainadmin.ReconciliationReport
	latestErr error
	lastLimit int
}

func (f *fakeReconReader) FindRecent(_ context.Context, limit int) ([]domainadmin.ReconciliationReport, error) {
	f.lastLimit = limit
	return f.recent, nil
}
func (f *fakeReconReader) FindLatest(_ context.Context) (*domainadmin.ReconciliationReport, error) {
	return f.latest, f.latestErr
}
