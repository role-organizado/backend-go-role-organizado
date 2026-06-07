package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// OverdueInstallmentState holds mutable workflow state exposed via query handlers.
type OverdueInstallmentState struct {
	Status      string
	Result      string
	MarkedCount int
}

// OverdueInstallmentWorkflow marks past-due installments and dispatches notifications.
//
// Activities:
//  1. FindAndMarkOverdueInstallments(referenceDate) → markedCount
//  2. DispatchNotifications(referenceDate, markedCount)  — skipped when markedCount == 0
//
// Queries: GetWorkflowStatus, GetCurrentState, GetResult
// Schedule: daily 03:00 BRT (06:00 UTC), scheduleId: overdue-installment-daily-workflow
func OverdueInstallmentWorkflow(ctx workflow.Context, referenceDate string) error {
	state := OverdueInstallmentState{Status: "running"}

	_ = workflow.SetQueryHandler(ctx, "GetWorkflowStatus", func() (string, error) {
		return state.Status, nil
	})
	_ = workflow.SetQueryHandler(ctx, "GetCurrentState", func() (OverdueInstallmentState, error) {
		return state, nil
	})
	_ = workflow.SetQueryHandler(ctx, "GetResult", func() (string, error) {
		return state.Result, nil
	})

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 3},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var markedCount int
	if err := workflow.ExecuteActivity(ctx, "FindAndMarkOverdueInstallments", referenceDate).Get(ctx, &markedCount); err != nil {
		state.Status = "failed"
		return err
	}
	state.MarkedCount = markedCount

	if markedCount > 0 {
		if err := workflow.ExecuteActivity(ctx, "DispatchNotifications", referenceDate, markedCount).Get(ctx, nil); err != nil {
			state.Status = "failed"
			return err
		}
	}

	state.Status = "completed"
	return nil
}
