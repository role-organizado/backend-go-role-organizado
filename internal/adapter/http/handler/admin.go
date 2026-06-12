package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/role-organizado/backend-go-role-organizado/internal/domain/admin"
	"github.com/role-organizado/backend-go-role-organizado/internal/domain/auth"
	"github.com/role-organizado/backend-go-role-organizado/internal/domain/event"
	"github.com/role-organizado/backend-go-role-organizado/internal/domain/finance"
	"github.com/role-organizado/backend-go-role-organizado/internal/domain/outbound"
	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/middleware"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// AdminHandler handles the admin surface: dashboard, feature flags, dominios
// extras, cancellation policies, events admin, user roles, and reconciliation
// reports. It is fully hexagonal — every operation goes through a use-case port.
type AdminHandler struct {
	// Dashboard
	statsUC          portin.GetDashboardStatsUseCase
	healthUC         portin.GetDashboardHealthUseCase
	financeUC        portin.GetDashboardFinanceUseCase
	pendingOutboundUC portin.GetPendingOutboundUseCase

	// Feature flags
	listFlagsUC  portin.ListFeatureFlagsUseCase
	updateFlagUC portin.UpdateFeatureFlagUseCase

	// Dominios extras
	getDominioUC     portin.GetDominioByIDUseCase
	toggleDominioUC  portin.ToggleDominioUseCase
	listCategoriasUC portin.ListDominioCategoriasUseCase

	// Cancellation policies
	listPoliciesUC portin.ListCancelamentoPoliciesUseCase
	updateTiersUC  portin.UpdateCancelamentoTiersUseCase

	// Eventos admin
	listEventosUC portin.ListEventosAdminUseCase
	completoUC    portin.GetEventoCompletoAdminUseCase
	cancelarUC    portin.CancelarEventoAdminUseCase
	fecharUC      portin.FecharFinanceiroAdminUseCase

	// User roles
	addRoleUC    portin.AddUserRoleUseCase
	removeRoleUC portin.RemoveUserRoleUseCase

	// Reconciliation reports
	listReportsUC  portin.ListReconciliationReportsUseCase
	latestReportUC portin.GetLatestReconciliationReportUseCase
}

// AdminHandlerDeps groups the use cases injected into the AdminHandler.
// A deps struct is used (instead of a long positional constructor) because the
// admin surface aggregates many independent use cases.
type AdminHandlerDeps struct {
	Stats           portin.GetDashboardStatsUseCase
	Health          portin.GetDashboardHealthUseCase
	Finance         portin.GetDashboardFinanceUseCase
	PendingOutbound portin.GetPendingOutboundUseCase

	ListFlags  portin.ListFeatureFlagsUseCase
	UpdateFlag portin.UpdateFeatureFlagUseCase

	GetDominio     portin.GetDominioByIDUseCase
	ToggleDominio  portin.ToggleDominioUseCase
	ListCategorias portin.ListDominioCategoriasUseCase

	ListPolicies portin.ListCancelamentoPoliciesUseCase
	UpdateTiers  portin.UpdateCancelamentoTiersUseCase

	ListEventos portin.ListEventosAdminUseCase
	Completo    portin.GetEventoCompletoAdminUseCase
	Cancelar    portin.CancelarEventoAdminUseCase
	Fechar      portin.FecharFinanceiroAdminUseCase

	AddRole    portin.AddUserRoleUseCase
	RemoveRole portin.RemoveUserRoleUseCase

	ListReports  portin.ListReconciliationReportsUseCase
	LatestReport portin.GetLatestReconciliationReportUseCase
}

// NewAdminHandler creates a new AdminHandler from its dependencies.
func NewAdminHandler(d AdminHandlerDeps) *AdminHandler {
	return &AdminHandler{
		statsUC:           d.Stats,
		healthUC:          d.Health,
		financeUC:         d.Finance,
		pendingOutboundUC: d.PendingOutbound,
		listFlagsUC:       d.ListFlags,
		updateFlagUC:      d.UpdateFlag,
		getDominioUC:      d.GetDominio,
		toggleDominioUC:   d.ToggleDominio,
		listCategoriasUC:  d.ListCategorias,
		listPoliciesUC:    d.ListPolicies,
		updateTiersUC:     d.UpdateTiers,
		listEventosUC:     d.ListEventos,
		completoUC:        d.Completo,
		cancelarUC:        d.Cancelar,
		fecharUC:          d.Fechar,
		addRoleUC:         d.AddRole,
		removeRoleUC:      d.RemoveRole,
		listReportsUC:     d.ListReports,
		latestReportUC:    d.LatestReport,
	}
}

// RegisterAdminRoutes registers admin routes. JWT/admin auth is enforced upstream.
func (h *AdminHandler) RegisterAdminRoutes(r chi.Router) {
	// Dashboard
	r.Get("/api/v1/admin/dashboard/stats", h.GetDashboardStats)
	r.Get("/api/v1/admin/dashboard/health", h.GetDashboardHealth)
	r.Get("/api/v1/admin/dashboard/finance", h.GetDashboardFinance)
	r.Get("/api/v1/admin/dashboard/accounting/pending-outbound", h.GetPendingOutbound)

	// Feature flags
	r.Get("/api/v1/admin/feature-flags", h.ListFeatureFlags)
	r.Put("/api/v1/admin/feature-flags/{chave}", h.UpdateFeatureFlag)
	r.Patch("/api/v1/admin/feature-flags/{chave}", h.UpdateFeatureFlag)

	// Dominios extras (CRUD lives in ConfigHandler; these complete the surface)
	r.Get("/api/v1/admin/dominios/categorias", h.ListDominioCategorias)
	r.Get("/api/v1/admin/dominios/{id}", h.GetDominio)
	r.Patch("/api/v1/admin/dominios/{id}/toggle", h.ToggleDominio)

	// Cancellation policies
	r.Get("/api/v1/admin/cancelamento-policies", h.ListCancelamentoPolicies)
	r.Put("/api/v1/admin/cancelamento-policies/{id}/tiers", h.UpdateCancelamentoTiers)

	// Eventos admin
	r.Get("/api/v1/admin/eventos", h.ListEventos)
	r.Get("/api/v1/admin/eventos/{eventoId}/completo", h.GetEventoCompleto)
	r.Post("/api/v1/admin/eventos/{eventoId}/cancelar", h.CancelarEvento)
	r.Post("/api/v1/admin/eventos/{eventoId}/fechar-financeiro", h.FecharFinanceiro)

	// User roles (PUT set-roles lives in UsuarioHandler; add/remove here)
	r.Post("/api/v1/admin/usuarios/{id}/roles", h.AddUsuarioRole)
	r.Delete("/api/v1/admin/usuarios/{id}/roles/{role}", h.RemoveUsuarioRole)

	// Reconciliation reports
	r.Get("/api/v1/admin/reconciliation/reports", h.ListReconciliationReports)
	r.Get("/api/v1/admin/reconciliation/reports/latest", h.GetLatestReconciliationReport)
}

// ---- Dashboard ----

func (h *AdminHandler) GetDashboardStats(w http.ResponseWriter, r *http.Request) {
	counts, err := h.statsUC.Execute(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"bigNumbers": map[string]any{
			"totalUsuarios":     counts.TotalUsuarios,
			"totalEventos":      counts.TotalEventos,
			"totalDrafts":       counts.TotalDrafts,
			"totalPagamentos":   counts.TotalPagamentos,
			"totalNotificacoes": counts.TotalNotificacoes,
		},
		"serviceHealth": map[string]any{
			"database": "UP",
			"backend":  "UP",
		},
		"recentActivity": []any{},
	})
}

func (h *AdminHandler) GetDashboardHealth(w http.ResponseWriter, r *http.Request) {
	up, _ := h.healthUC.Execute(r.Context())
	status := "DOWN"
	if up {
		status = "UP"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":    status,
		"database":  status,
		"backend":   "UP",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func (h *AdminHandler) GetDashboardFinance(w http.ResponseWriter, r *http.Request) {
	totals, err := h.financeUC.Execute(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, totals)
}

func (h *AdminHandler) GetPendingOutbound(w http.ResponseWriter, r *http.Request) {
	limit := pageParam(r, "limit", 20)
	summary, err := h.pendingOutboundUC.Execute(r.Context(), limit)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, pendingOutboundResponse(summary))
}

func pendingOutboundResponse(s admin.PendingOutboundSummary) map[string]any {
	items := make([]map[string]any, 0, len(s.Items))
	for _, it := range s.Items {
		var oldest any
		if it.OldestPendingAt != nil {
			oldest = it.OldestPendingAt.UTC().Format(time.RFC3339)
		}
		items = append(items, map[string]any{
			"eventId":            it.EventID,
			"eventName":          it.EventName,
			"pendingCount":       it.PendingCount,
			"pendingAmountCents": it.PendingAmountCents,
			"oldestPendingAt":    oldest,
		})
	}
	return map[string]any{
		"timestamp":               s.Timestamp.UTC().Format(time.RFC3339),
		"totalEventsWithPending":  s.TotalEventsWithPending,
		"totalPendingRequests":    s.TotalPendingRequests,
		"totalPendingAmountCents": s.TotalPendingAmountCents,
		"items":                   items,
	}
}

// ---- Feature flags ----

func (h *AdminHandler) ListFeatureFlags(w http.ResponseWriter, r *http.Request) {
	flags, err := h.listFlagsUC.Execute(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, featureFlagsResponse(flags))
}

func (h *AdminHandler) UpdateFeatureFlag(w http.ResponseWriter, r *http.Request) {
	chave := chi.URLParam(r, "chave")

	var req struct {
		Enabled   *bool   `json:"enabled"`
		Nome      *string `json:"nome"`
		Descricao *string `json:"descricao"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo inválido"))
		return
	}

	flag, err := h.updateFlagUC.Execute(r.Context(), chave, admin.FeatureFlagUpdate{
		Enabled:   req.Enabled,
		Nome:      req.Nome,
		Descricao: req.Descricao,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, featureFlagResponse(*flag))
}

func featureFlagResponse(f admin.FeatureFlag) map[string]any {
	return map[string]any{
		"id":           f.ID,
		"chave":        f.Chave,
		"nome":         f.Nome,
		"enabled":      f.Enabled,
		"descricao":    f.Descricao,
		"categoria":    f.Categoria,
		"metadata":     f.Metadata,
		"criadoEm":     f.CriadoEm,
		"atualizadoEm": f.AtualizadoEm,
	}
}

func featureFlagsResponse(flags []admin.FeatureFlag) []map[string]any {
	out := make([]map[string]any, 0, len(flags))
	for _, f := range flags {
		out = append(out, featureFlagResponse(f))
	}
	return out
}

// ---- Dominios extras ----

func (h *AdminHandler) GetDominio(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	d, err := h.getDominioUC.Execute(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toDominioResponse(*d))
}

func (h *AdminHandler) ToggleDominio(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	d, err := h.toggleDominioUC.Execute(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":      d.ID,
		"ativo":   d.Ativo,
		"message": "estado atualizado",
	})
}

func (h *AdminHandler) ListDominioCategorias(w http.ResponseWriter, r *http.Request) {
	cats, err := h.listCategoriasUC.Execute(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	if cats == nil {
		cats = []string{}
	}
	writeJSON(w, http.StatusOK, cats)
}

// ---- Cancellation policies ----

func (h *AdminHandler) ListCancelamentoPolicies(w http.ResponseWriter, r *http.Request) {
	policies, err := h.listPoliciesUC.Execute(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]dominioResponse, 0, len(policies))
	for _, p := range policies {
		out = append(out, toDominioResponse(p))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *AdminHandler) UpdateCancelamentoTiers(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req struct {
		Tiers []admin.CancellationTier `json:"tiers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo inválido"))
		return
	}

	d, err := h.updateTiersUC.Execute(r.Context(), id, req.Tiers)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toDominioResponse(*d))
}

// ---- Eventos admin ----

func (h *AdminHandler) ListEventos(w http.ResponseWriter, r *http.Request) {
	page, err := h.listEventosUC.Execute(r.Context(), portin.ListEventosAdminInput{
		Status:   r.URL.Query().Get("status"),
		Tipo:     r.URL.Query().Get("tipo"),
		Nome:     r.URL.Query().Get("nome"),
		Page:     pageParam(r, "page", 0),
		PageSize: pageParam(r, "pageSize", 20),
	})
	if err != nil {
		writeError(w, err)
		return
	}
	eventos := make([]map[string]any, 0, len(page.Eventos))
	for i := range page.Eventos {
		eventos = append(eventos, eventoAdminListItem(&page.Eventos[i]))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"eventos": eventos,
		"pagination": map[string]any{
			"hasMore":    page.HasMore,
			"totalCount": page.TotalCount,
			"page":       page.Page,
			"pageSize":   page.PageSize,
		},
	})
}

func eventoAdminListItem(e *event.Evento) map[string]any {
	return map[string]any{
		"id":        e.ID,
		"nome":      e.Nome,
		"tipo":      e.Tipo,
		"status":    string(e.Status),
		"fase":      string(e.Fase),
		"data":      e.Data,
		"local":     e.Local,
		"criadoEm":  e.CriadoEm,
		"organizadorId": e.UsuarioID,
	}
}

func (h *AdminHandler) GetEventoCompleto(w http.ResponseWriter, r *http.Request) {
	eventoID := chi.URLParam(r, "eventoId")
	completo, err := h.completoUC.Execute(r.Context(), eventoID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, eventoCompletoResponse(completo))
}

func eventoCompletoResponse(c *portin.EventoCompletoAdmin) map[string]any {
	resp := map[string]any{
		"evento": eventoAdminListItem(c.Evento),
	}
	if c.Organizador != nil {
		resp["organizador"] = organizadorResponse(c.Organizador)
	}
	if c.Finance != nil {
		resp["financeiro"] = financeResumoResponse(c.Finance)
	}
	pending := make([]map[string]any, 0, len(c.PendingOutbound))
	for i := range c.PendingOutbound {
		pending = append(pending, pendingOutboundItemResponse(&c.PendingOutbound[i]))
	}
	resp["pendingOutbound"] = pending
	return resp
}

func organizadorResponse(u *auth.Usuario) map[string]any {
	return map[string]any{
		"id":    u.ID,
		"nome":  u.Nome,
		"email": u.Email,
	}
}

func financeResumoResponse(s *finance.FinanceSummary) map[string]any {
	return map[string]any{
		"goal":                   s.Goal,
		"collected":              s.Collected,
		"progressPercentage":     s.ProgressPercentage,
		"availableForWithdrawal": s.AvailableForWithdrawal,
		"pendingWithdrawals":     s.PendingWithdrawals,
	}
}

func pendingOutboundItemResponse(o *outbound.OutboundRequest) map[string]any {
	return map[string]any{
		"id":          o.ID,
		"amountCents": o.AmountCents,
		"status":      string(o.Status),
		"createdAt":   o.CreatedAt,
	}
}

func (h *AdminHandler) CancelarEvento(w http.ResponseWriter, r *http.Request) {
	eventoID := chi.URLParam(r, "eventoId")

	var req struct {
		Motivo string `json:"motivo"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	ev, err := h.cancelarUC.Execute(r.Context(), portin.CancelarEventoAdminInput{
		EventoID:    eventoID,
		Motivo:      req.Motivo,
		AdminUserID: middleware.UserIDFromContext(r.Context()),
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"eventoId": ev.ID,
		"status":   string(ev.Status),
		"message":  "evento cancelado",
	})
}

func (h *AdminHandler) FecharFinanceiro(w http.ResponseWriter, r *http.Request) {
	eventoID := chi.URLParam(r, "eventoId")
	ev, err := h.fecharUC.Execute(r.Context(), eventoID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"eventoId": ev.ID,
		"fase":     string(ev.Fase),
		"message":  "financeiro fechado",
	})
}

// ---- User roles ----

func (h *AdminHandler) AddUsuarioRole(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo inválido"))
		return
	}

	u, err := h.addRoleUC.Execute(r.Context(), portin.ModifyUserRoleInput{
		UsuarioID: id,
		Role:      auth.Role(req.Role),
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, usuarioRolesResponse(u))
}

func (h *AdminHandler) RemoveUsuarioRole(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	role := chi.URLParam(r, "role")

	u, err := h.removeRoleUC.Execute(r.Context(), portin.ModifyUserRoleInput{
		UsuarioID: id,
		Role:      auth.Role(role),
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, usuarioRolesResponse(u))
}

func usuarioRolesResponse(u *auth.Usuario) map[string]any {
	return map[string]any{
		"id":    u.ID,
		"email": u.Email,
		"nome":  u.Nome,
		"roles": u.RoleStrings(),
	}
}

// ---- Reconciliation reports ----

func (h *AdminHandler) ListReconciliationReports(w http.ResponseWriter, r *http.Request) {
	limit := pageParam(r, "limit", 20)
	reports, err := h.listReportsUC.Execute(r.Context(), limit)
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(reports))
	for i := range reports {
		out = append(out, reconciliationReportResponse(&reports[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *AdminHandler) GetLatestReconciliationReport(w http.ResponseWriter, r *http.Request) {
	report, err := h.latestReportUC.Execute(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, reconciliationReportResponse(report))
}

func reconciliationReportResponse(rep *admin.ReconciliationReport) map[string]any {
	return map[string]any{
		"id":            rep.ID,
		"referenceDate": rep.ReferenceDate,
		"runAt":         rep.RunAt,
		"checkedCount":  rep.CheckedCount,
		"updatedCount":  rep.UpdatedCount,
		"failedCount":   rep.FailedCount,
		"errors":        rep.Errors,
	}
}
