package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/activity"
)

// ReconciliationWorkflowInput carries the parameters for a single reconciliation run.
type ReconciliationWorkflowInput struct {
	// ReferenceDate is the label for this run (e.g. "2025-06-07").
	ReferenceDate string `json:"referenceDate"`
	// From is the start of the reconciliation window.
	From time.Time `json:"from"`
	// To is the end of the reconciliation window.
	To time.Time `json:"to"`
}

// ReconciliationWorkflow reconciles local payment transaction states against the PSP
// for a configurable time window. Typically scheduled as a Temporal Schedule running
// at 06:00 daily (mirrors Java app.temporal.schedules.psp-reconciliation config).
//
// The workflow runs a single activity with retry policy, saving the result to the
// reconciliation_reports MongoDB collection for audit purposes.
func ReconciliationWorkflow(ctx workflow.Context, input ReconciliationWorkflowInput) error {
	// Default window: last 24 hours if not explicitly set.
	from := input.From
	to := input.To
	if from.IsZero() {
		to = workflow.Now(ctx)
		from = to.Add(-24 * time.Hour)
	}
	if to.IsZero() {
		to = workflow.Now(ctx)
	}

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		HeartbeatTimeout:    5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:    3,
			InitialInterval:    30 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    5 * time.Minute,
		},
	}
	actCtx := workflow.WithActivityOptions(ctx, ao)

	var a *activity.PaymentActivities
	actInput := activity.ReconciliationActivityInput{
		ReferenceDate: input.ReferenceDate,
		From:          from,
		To:            to,
	}

	if err := workflow.ExecuteActivity(actCtx, a.ReconcilePspTransactions, actInput).Get(ctx, nil); err != nil {
		return fmt.Errorf("reconciliation workflow: %w", err)
	}

	return nil
}
