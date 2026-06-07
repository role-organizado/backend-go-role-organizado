package workflow

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// ── Result type ───────────────────────────────────────────────────────────────

// PricingPspReviewResult is the result of a Pricing PSP Review workflow execution.
// Mirrors Java's PricingPspReviewResult record.
type PricingPspReviewResult struct {
	Completed     bool   `json:"completed"`
	ReviewedCount int    `json:"reviewedCount"`
	AppliedCount  int    `json:"appliedCount"`
	DurationMs    int64  `json:"durationMs"`
	Message       string `json:"message"`
}

// NewPricingPspReviewResultSuccess creates a successful result.
func NewPricingPspReviewResultSuccess(reviewed, applied int, durationMs int64) PricingPspReviewResult {
	return PricingPspReviewResult{
		Completed:     true,
		ReviewedCount: reviewed,
		AppliedCount:  applied,
		DurationMs:    durationMs,
		Message:       fmt.Sprintf("PSP review completed: reviewed=%d applied=%d", reviewed, applied),
	}
}

// NewPricingPspReviewResultError creates a failed result.
func NewPricingPspReviewResultError(msg string, durationMs int64) PricingPspReviewResult {
	return PricingPspReviewResult{
		Completed:  false,
		DurationMs: durationMs,
		Message:    msg,
	}
}

// ── Activity interface ────────────────────────────────────────────────────────

// PricingPspReviewActivities holds the activity methods for the pricing PSP review workflow.
// Mirrors Java's PricingPspReviewActivities interface.
type PricingPspReviewActivities struct{}

// RunReview runs the PSP cost review for the given reference date.
// Mirrors Java's PricingPspReviewActivities.runReview.
func (a *PricingPspReviewActivities) RunReview(ctx context.Context, referenceDate string) (PricingPspReviewResult, error) {
	activity.GetLogger(ctx).Info("pricing psp review activity", "referenceDate", referenceDate)
	return PricingPspReviewResult{}, fmt.Errorf("not implemented")
}

// ── Workflow ──────────────────────────────────────────────────────────────────

// PricingPspReviewWorkflow is the Temporal workflow for the daily pricing PSP review (E5-04).
// Replaces the legacy @Scheduled PricingPspReviewJob at 02:30 BRT.
// Mirrors Java's PricingPspReviewWorkflowImpl.
//
// States: PENDING → RUNNING → COMPLETED | FAILED
func PricingPspReviewWorkflow(ctx workflow.Context, referenceDate string) error {
	logger := workflow.GetLogger(ctx)

	// ── Mutable state (query-accessible) ─────────────────────────────────────
	workflowStatus := "PENDING"
	var result *PricingPspReviewResult

	// ── Query handlers (registered before first yield) ───────────────────────
	if err := workflow.SetQueryHandler(ctx, "getWorkflowStatus", func() (string, error) {
		return workflowStatus, nil
	}); err != nil {
		return err
	}
	if err := workflow.SetQueryHandler(ctx, "getResult", func() (*PricingPspReviewResult, error) {
		return result, nil
	}); err != nil {
		return err
	}

	// ── Activity options ──────────────────────────────────────────────────────
	ao := workflow.ActivityOptions{
		StartToCloseTimeout:    30 * time.Minute,
		ScheduleToCloseTimeout: 60 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    30 * time.Second,
			MaximumAttempts:    3,
			BackoffCoefficient: 2.0,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	workflowStatus = "RUNNING"
	logger.Info("[PRICING-PSP-REVIEW] Starting", "referenceDate", referenceDate)

	// ── Execute activity ──────────────────────────────────────────────────────
	var a *PricingPspReviewActivities
	var actResult PricingPspReviewResult
	err := workflow.ExecuteActivity(ctx, a.RunReview, referenceDate).Get(ctx, &actResult)
	if err != nil {
		workflowStatus = "FAILED"
		logger.Error("[PRICING-PSP-REVIEW] Activity failed", "error", err)
		return err
	}

	result = &actResult
	if actResult.Completed {
		workflowStatus = "COMPLETED"
		logger.Info("[PRICING-PSP-REVIEW] Completed",
			"referenceDate", referenceDate,
			"reviewed", actResult.ReviewedCount,
			"applied", actResult.AppliedCount)
	} else {
		workflowStatus = "FAILED"
		logger.Warn("[PRICING-PSP-REVIEW] Activity returned non-completed result", "message", actResult.Message)
	}

	return nil
}
