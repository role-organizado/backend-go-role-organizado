// Package workflow contains Temporal workflow implementations.
package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// FinanceReconciliationState holds the observable state of a finance reconciliation run.
type FinanceReconciliationState struct {
	// Status is one of "running", "completed", or "failed".
	Status string
	// Result contains a human-readable summary after the workflow completes.
	Result string
}

// FinanceReconciliationWorkflow runs a triple-check finance reconciliation:
//   - Pass 1 — 15-minute StartToCloseTimeout, up to 3 retries with 1-min/2× backoff.
//   - Passes 2 & 3 — 45-minute StartToCloseTimeout, same retry policy.
//
// If referenceDate is empty the workflow computes yesterday's date (UTC), which
// is the expected reference date for scheduled (02:00 BRT) runs.
//
// Queries exposed: GetWorkflowStatus, GetCurrentState, GetResult.
func FinanceReconciliationWorkflow(ctx workflow.Context, referenceDate string) error {
	if referenceDate == "" {
		// Use yesterday in UTC — standard reference date for nightly reconciliation.
		yesterday := workflow.Now(ctx).UTC().AddDate(0, 0, -1)
		referenceDate = yesterday.Format("2006-01-02")
	}

	state := FinanceReconciliationState{Status: "running"}

	_ = workflow.SetQueryHandler(ctx, "GetWorkflowStatus", func() (string, error) {
		return state.Status, nil
	})
	_ = workflow.SetQueryHandler(ctx, "GetCurrentState", func() (FinanceReconciliationState, error) {
		return state, nil
	})
	_ = workflow.SetQueryHandler(ctx, "GetResult", func() (string, error) {
		return state.Result, nil
	})

	retryPolicy := &temporal.RetryPolicy{
		MaximumAttempts:    3,
		InitialInterval:    time.Minute,
		BackoffCoefficient: 2.0,
	}

	// Pass 1: 15-minute timeout — initial reconciliation run.
	ao1 := workflow.ActivityOptions{
		StartToCloseTimeout: 15 * time.Minute,
		RetryPolicy:         retryPolicy,
	}
	// Passes 2 & 3: 45-minute timeout — verification passes.
	ao2 := workflow.ActivityOptions{
		StartToCloseTimeout: 45 * time.Minute,
		RetryPolicy:         retryPolicy,
	}

	ctx1 := workflow.WithActivityOptions(ctx, ao1)
	ctx2 := workflow.WithActivityOptions(ctx, ao2)

	// Pass 1
	if err := workflow.ExecuteActivity(ctx1, "RunReconciliation", referenceDate).Get(ctx1, nil); err != nil {
		state.Status = "failed"
		return fmt.Errorf("finance reconciliation pass 1: %w", err)
	}

	// Pass 2 — consistency verification
	if err := workflow.ExecuteActivity(ctx2, "RunReconciliation", referenceDate).Get(ctx2, nil); err != nil {
		state.Status = "failed"
		return fmt.Errorf("finance reconciliation pass 2: %w", err)
	}

	// Pass 3 — final triple-check confirmation
	if err := workflow.ExecuteActivity(ctx2, "RunReconciliation", referenceDate).Get(ctx2, nil); err != nil {
		state.Status = "failed"
		return fmt.Errorf("finance reconciliation pass 3: %w", err)
	}

	state.Status = "completed"
	state.Result = fmt.Sprintf("triple-check reconciliation completed for %s", referenceDate)
	return nil
}
