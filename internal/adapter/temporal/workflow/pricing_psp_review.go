package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// PricingPspReviewWorkflow is the Temporal workflow for the daily PSP cost review.
//
// It exposes two query handlers:
//   - GetWorkflowStatus: returns the current lifecycle status ("running", "completed", "failed").
//   - GetResult: reserved for a future result payload; currently returns an empty string.
//
// Task queue: PRICING_PSP_REVIEW_QUEUE
// Schedule:   02:30 BRT (05:30 UTC) daily
func PricingPspReviewWorkflow(ctx workflow.Context, referenceDate string) error {
	status := "running"
	result := ""

	// Query handler: lifecycle status
	_ = workflow.SetQueryHandler(ctx, "GetWorkflowStatus", func() (string, error) {
		return status, nil
	})
	// Query handler: result payload (updated via closure when available)
	_ = workflow.SetQueryHandler(ctx, "GetResult", func() (string, error) {
		return result, nil
	})

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	if err := workflow.ExecuteActivity(ctx, "RunPspCostReview", referenceDate).Get(ctx, nil); err != nil {
		status = "failed"
		return err
	}

	status = "completed"
	return nil
}
