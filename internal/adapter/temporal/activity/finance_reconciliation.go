// Package activity contains Temporal activity implementations.
package activity

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/activity"

	"github.com/role-organizado/backend-go-role-organizado/internal/usecase/finance"
)

// FinanceReconciliationActivities holds dependencies for finance reconciliation activities.
type FinanceReconciliationActivities struct {
	reconciliationUC *finance.ReconciliationUseCase
}

// NewFinanceReconciliationActivities creates a new FinanceReconciliationActivities instance.
func NewFinanceReconciliationActivities(javaBackendURL string) *FinanceReconciliationActivities {
	return &FinanceReconciliationActivities{
		reconciliationUC: finance.NewReconciliationUseCase(javaBackendURL),
	}
}

// RunReconciliation executes a single finance reconciliation pass for the given
// reference date, delegating to the Java backend.
//
// Triple-check pattern: the workflow calls this activity 3 times — pass 1 with
// a 15-minute timeout (StartToCloseTimeout) and passes 2 & 3 with a 45-minute
// timeout. Each pass independently validates the reconciliation output.
// Consistency verification between passes is handled at the workflow level.
//
// TODO(spec-168): implementar nativo Go — atualmente delega ao Java via HTTP POST.
func (a *FinanceReconciliationActivities) RunReconciliation(ctx context.Context, referenceDate string) error {
	logger := activity.GetLogger(ctx)
	info := activity.GetInfo(ctx)

	logger.Info("running finance reconciliation pass",
		"referenceDate", referenceDate,
		"attempt", info.Attempt,
	)

	if err := a.reconciliationUC.Execute(ctx, referenceDate); err != nil {
		return fmt.Errorf("finance reconciliation pass (attempt %d): %w", info.Attempt, err)
	}

	logger.Info("finance reconciliation pass completed",
		"referenceDate", referenceDate,
		"attempt", info.Attempt,
	)
	return nil
}
