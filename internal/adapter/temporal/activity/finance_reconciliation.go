// Package activity contains Temporal activity implementations.
package activity

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/activity"
)

// financeReconciler is the minimal contract that the finance reconciliation
// activity needs. Satisfied by *finance.ReconciliationUseCase. Defined here as
// an interface so the activity can be unit-tested with a stub without depending
// on the concrete use case package.
type financeReconciler interface {
	Execute(ctx context.Context, referenceDate string) error
}

// FinanceReconciliationActivities holds dependencies for finance reconciliation activities.
type FinanceReconciliationActivities struct {
	reconciliationUC financeReconciler
}

// NewFinanceReconciliationActivities creates a new FinanceReconciliationActivities instance.
//
// The use case is the native (Go) reconciliation implementation backed by
// MongoDB repos — the previous Java HTTP bridge has been removed.
func NewFinanceReconciliationActivities(uc financeReconciler) *FinanceReconciliationActivities {
	return &FinanceReconciliationActivities{reconciliationUC: uc}
}

// RunReconciliation executes a single finance reconciliation pass for the given
// reference date using the native Go use case.
//
// Triple-check pattern: the workflow calls this activity 3 times — pass 1 with
// a 15-minute timeout (StartToCloseTimeout) and passes 2 & 3 with a 45-minute
// timeout. Each pass independently validates the reconciliation output.
// Consistency verification between passes is handled at the workflow level.
//
// The Go implementation is read-heavy and idempotent: each pass walks the same
// data and is safe to retry without side-effects (apart from appending the
// per-pass report row).
func (a *FinanceReconciliationActivities) RunReconciliation(ctx context.Context, referenceDate string) error {
	logger := activity.GetLogger(ctx)
	info := activity.GetInfo(ctx)

	logger.Info("running finance reconciliation pass (native)",
		"referenceDate", referenceDate,
		"attempt", info.Attempt,
	)

	if err := a.reconciliationUC.Execute(ctx, referenceDate); err != nil {
		return fmt.Errorf("finance reconciliation pass (attempt %d): %w", info.Attempt, err)
	}

	logger.Info("finance reconciliation pass completed (native)",
		"referenceDate", referenceDate,
		"attempt", info.Attempt,
	)
	return nil
}
