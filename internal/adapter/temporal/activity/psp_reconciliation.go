// Package activity contains Temporal activity implementations.
package activity

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.temporal.io/sdk/activity"

	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
)

// PspReconciliationActivities groups the activities for the PSP reconciliation
// workflow. It reuses the ReconcilePspTransactions use case and persists a run
// report via the same ReconciliationReportRepository used by the daily
// ReconciliationWorkflow.
type PspReconciliationActivities struct {
	reconcileUC portin.ReconcilePspTransactionsUseCase
	reportRepo  ReconciliationReportRepository
}

// NewPspReconciliationActivities constructs PspReconciliationActivities.
func NewPspReconciliationActivities(
	reconcileUC portin.ReconcilePspTransactionsUseCase,
	reportRepo ReconciliationReportRepository,
) *PspReconciliationActivities {
	return &PspReconciliationActivities{reconcileUC: reconcileUC, reportRepo: reportRepo}
}

// PspReconciliationActivityInput carries parameters for the reconciliation activity.
type PspReconciliationActivityInput struct {
	ReferenceDate string    `json:"referenceDate"`
	From          time.Time `json:"from"`
	To            time.Time `json:"to"`
}

// PspReconciliationResult is the serialisable outcome of the reconciliation activity.
type PspReconciliationResult struct {
	Checked int64 `json:"checked"`
	Updated int64 `json:"updated"`
	Failed  int64 `json:"failed"`
}

// RunPspReconciliation reconciles local transactions against the PSP for the
// given window and persists a report. Registered as activity "RunPspReconciliation".
func (a *PspReconciliationActivities) RunPspReconciliation(ctx context.Context, input PspReconciliationActivityInput) (*PspReconciliationResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("starting PSP reconciliation run",
		"referenceDate", input.ReferenceDate,
		"from", input.From,
		"to", input.To,
	)

	result, err := a.reconcileUC.Execute(ctx, portin.ReconcileFilter{From: input.From, To: input.To})
	if err != nil {
		return nil, fmt.Errorf("psp reconciliation: %w", err)
	}

	// Persist an audit report (best-effort: a save failure must not lose the
	// reconciliation result already applied to the transactions).
	if a.reportRepo != nil {
		report := &ReconciliationReport{
			ID:            uuid.NewString(),
			ReferenceDate: input.ReferenceDate,
			RunAt:         time.Now().UTC(),
			CheckedCount:  result.Checked,
			UpdatedCount:  result.Updated,
			FailedCount:   result.Failed,
		}
		if saveErr := a.reportRepo.Save(ctx, report); saveErr != nil {
			logger.Warn("psp reconciliation: failed to persist report", "error", saveErr)
		}
	}

	logger.Info("PSP reconciliation run completed",
		"referenceDate", input.ReferenceDate,
		"checked", result.Checked,
		"updated", result.Updated,
		"failed", result.Failed,
	)
	return &PspReconciliationResult{
		Checked: result.Checked,
		Updated: result.Updated,
		Failed:  result.Failed,
	}, nil
}
