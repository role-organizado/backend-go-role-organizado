package admin

import (
	"context"

	"github.com/role-organizado/backend-go-role-organizado/internal/domain/admin"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
)

// ListReconciliationReports implements portin.ListReconciliationReportsUseCase.
type ListReconciliationReports struct {
	repo portout.ReconciliationReportReader
}

// NewListReconciliationReports creates a new ListReconciliationReports use case.
func NewListReconciliationReports(r portout.ReconciliationReportReader) *ListReconciliationReports {
	return &ListReconciliationReports{repo: r}
}

// Execute lists recent reconciliation reports (most recent first).
func (uc *ListReconciliationReports) Execute(ctx context.Context, limit int) ([]admin.ReconciliationReport, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return uc.repo.FindRecent(ctx, limit)
}

// GetLatestReconciliationReport implements portin.GetLatestReconciliationReportUseCase.
type GetLatestReconciliationReport struct {
	repo portout.ReconciliationReportReader
}

// NewGetLatestReconciliationReport creates a new GetLatestReconciliationReport use case.
func NewGetLatestReconciliationReport(r portout.ReconciliationReportReader) *GetLatestReconciliationReport {
	return &GetLatestReconciliationReport{repo: r}
}

// Execute returns the most recent reconciliation report, or a NotFound error.
func (uc *GetLatestReconciliationReport) Execute(ctx context.Context) (*admin.ReconciliationReport, error) {
	return uc.repo.FindLatest(ctx)
}
