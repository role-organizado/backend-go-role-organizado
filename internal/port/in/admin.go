package in

import (
	"context"

	"github.com/role-organizado/backend-go-role-organizado/internal/domain/admin"
	"github.com/role-organizado/backend-go-role-organizado/internal/domain/auth"
	"github.com/role-organizado/backend-go-role-organizado/internal/domain/config"
	"github.com/role-organizado/backend-go-role-organizado/internal/domain/event"
	"github.com/role-organizado/backend-go-role-organizado/internal/domain/finance"
	"github.com/role-organizado/backend-go-role-organizado/internal/domain/outbound"
)

// ---- Dashboard ----

// GetDashboardStatsUseCase returns the aggregate dashboard counts.
type GetDashboardStatsUseCase interface {
	Execute(ctx context.Context) (admin.DashboardCounts, error)
}

// GetDashboardHealthUseCase reports whether the database is reachable.
type GetDashboardHealthUseCase interface {
	Execute(ctx context.Context) (bool, error)
}

// GetDashboardFinanceUseCase aggregates the finance summaries for the dashboard.
type GetDashboardFinanceUseCase interface {
	Execute(ctx context.Context) (map[string]any, error)
}

// GetPendingOutboundUseCase aggregates pending outbound requests across events.
type GetPendingOutboundUseCase interface {
	Execute(ctx context.Context, limit int) (admin.PendingOutboundSummary, error)
}

// ---- Feature flags ----

// ListFeatureFlagsUseCase lists all feature flags.
type ListFeatureFlagsUseCase interface {
	Execute(ctx context.Context) ([]admin.FeatureFlag, error)
}

// UpdateFeatureFlagUseCase patches a feature flag by chave.
type UpdateFeatureFlagUseCase interface {
	Execute(ctx context.Context, chave string, upd admin.FeatureFlagUpdate) (*admin.FeatureFlag, error)
}

// ---- Approvals ----

// CountPendingApprovalsUseCase counts PENDING approval items for an approver.
type CountPendingApprovalsUseCase interface {
	Execute(ctx context.Context, approverID string) (int64, error)
}

// ListPendingApprovalsUseCase lists PENDING approval items for an approver.
type ListPendingApprovalsUseCase interface {
	Execute(ctx context.Context, approverID string) ([]admin.ApprovalItem, error)
}

// ListApprovalHistoryUseCase lists resolved approval items for an approver.
type ListApprovalHistoryUseCase interface {
	Execute(ctx context.Context, approverID string) ([]admin.ApprovalItem, error)
}

// ---- Dominios (admin extras) ----

// GetDominioByIDUseCase returns a single Dominio by its ID.
type GetDominioByIDUseCase interface {
	Execute(ctx context.Context, id string) (*config.Dominio, error)
}

// ToggleDominioUseCase flips the ativo flag of a Dominio by ID.
type ToggleDominioUseCase interface {
	Execute(ctx context.Context, id string) (*config.Dominio, error)
}

// ListDominioCategoriasUseCase returns the distinct, sorted categorias.
type ListDominioCategoriasUseCase interface {
	Execute(ctx context.Context) ([]string, error)
}

// ---- Cancellation policies ----

// ListCancelamentoPoliciesUseCase lists the politica_cancelamento dominios.
type ListCancelamentoPoliciesUseCase interface {
	Execute(ctx context.Context) ([]config.Dominio, error)
}

// UpdateCancelamentoTiersUseCase replaces metadata.tiers of a cancellation policy.
type UpdateCancelamentoTiersUseCase interface {
	Execute(ctx context.Context, id string, tiers []admin.CancellationTier) (*config.Dominio, error)
}

// ---- Eventos (admin) ----

// ListEventosAdminInput carries optional filters + pagination for the admin list.
type ListEventosAdminInput struct {
	Status   string
	Tipo     string
	Nome     string
	Page     int
	PageSize int
}

// EventosAdminPage is the paginated result of ListEventosAdmin.
type EventosAdminPage struct {
	Eventos    []event.Evento
	TotalCount int64
	HasMore    bool
	Page       int
	PageSize   int
}

// ListEventosAdminUseCase lists events for the admin surface.
type ListEventosAdminUseCase interface {
	Execute(ctx context.Context, in ListEventosAdminInput) (EventosAdminPage, error)
}

// EventoCompletoAdmin is the composed detail view of an event for admins.
type EventoCompletoAdmin struct {
	Evento          *event.Evento
	Organizador     *auth.Usuario
	Finance         *finance.FinanceSummary
	PendingOutbound []outbound.OutboundRequest
}

// GetEventoCompletoAdminUseCase composes the full admin view of an event.
type GetEventoCompletoAdminUseCase interface {
	Execute(ctx context.Context, eventoID string) (*EventoCompletoAdmin, error)
}

// CancelarEventoAdminInput carries the cancellation reason + audit context.
type CancelarEventoAdminInput struct {
	EventoID    string
	Motivo      string
	AdminUserID string
}

// CancelarEventoAdminUseCase cancels an event (admin override).
type CancelarEventoAdminUseCase interface {
	Execute(ctx context.Context, in CancelarEventoAdminInput) (*event.Evento, error)
}

// FecharFinanceiroAdminUseCase concludes an event's financial cycle.
type FecharFinanceiroAdminUseCase interface {
	Execute(ctx context.Context, eventoID string) (*event.Evento, error)
}

// ---- Reconciliation reports ----

// ListReconciliationReportsUseCase lists recent reconciliation reports.
type ListReconciliationReportsUseCase interface {
	Execute(ctx context.Context, limit int) ([]admin.ReconciliationReport, error)
}

// GetLatestReconciliationReportUseCase returns the most recent report.
type GetLatestReconciliationReportUseCase interface {
	Execute(ctx context.Context) (*admin.ReconciliationReport, error)
}
