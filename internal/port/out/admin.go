package out

import (
	"context"

	"github.com/role-organizado/backend-go-role-organizado/internal/domain/admin"
)

// AdminMetricsRepository is the output port for the admin dashboard metrics.
// It reads aggregate counts and the database health needed by the dashboard.
type AdminMetricsRepository interface {
	// DashboardCounts returns the aggregate "big number" totals.
	DashboardCounts(ctx context.Context) (admin.DashboardCounts, error)
	// Ping reports database reachability (used by the health endpoint).
	Ping(ctx context.Context) error
	// FinanceSummaryTotals aggregates the finance_summaries collection.
	// The shape is intentionally loose (map) to mirror Java's aggregation output.
	FinanceSummaryTotals(ctx context.Context) (map[string]any, error)
}

// FeatureFlagRepository is the output port for the feature_flags collection.
type FeatureFlagRepository interface {
	FindAll(ctx context.Context) ([]admin.FeatureFlag, error)
	Update(ctx context.Context, chave string, upd admin.FeatureFlagUpdate) (*admin.FeatureFlag, error)
}

// ApprovalRepository is the output port for the approval_items collection.
// approver_id / evento_id / solicitante_id are stored as UUID Binary subtype 4.
type ApprovalRepository interface {
	CountPending(ctx context.Context, approverID string) (int64, error)
	FindPending(ctx context.Context, approverID string) ([]admin.ApprovalItem, error)
	FindHistory(ctx context.Context, approverID string) ([]admin.ApprovalItem, error)
}

// ReconciliationReportReader is the output port for reading reconciliation reports.
type ReconciliationReportReader interface {
	FindRecent(ctx context.Context, limit int) ([]admin.ReconciliationReport, error)
	FindLatest(ctx context.Context) (*admin.ReconciliationReport, error)
}
