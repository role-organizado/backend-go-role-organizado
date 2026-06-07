package worker

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"go.temporal.io/sdk/client"
)

const (
	// PricingPspReviewScheduleID is the unique ID of the daily pricing PSP review schedule.
	PricingPspReviewScheduleID = "pricing-psp-review-daily-workflow"

	// pricingPspReviewTaskQueue is the task queue polled by the PricingPspReview worker.
	pricingPspReviewTaskQueue = "PRICING_PSP_REVIEW_QUEUE"

	// pricingPspReviewCron is the UTC cron expression for 02:30 BRT (= 05:30 UTC).
	pricingPspReviewCron = "30 5 * * *"
)

// InitPricingPspReviewSchedule creates the daily Temporal schedule for PricingPspReviewWorkflow.
//
// The schedule fires at 02:30 BRT (05:30 UTC) every day.
// If a schedule with the same ID already exists, the method logs a warning and returns nil.
func (r *Registry) InitPricingPspReviewSchedule(ctx context.Context) error {
	handle, err := r.client.ScheduleClient().Create(ctx, client.ScheduleOptions{
		ID: PricingPspReviewScheduleID,
		Spec: client.ScheduleSpec{
			CronExpressions: []string{pricingPspReviewCron},
		},
		Action: &client.ScheduleWorkflowAction{
			Workflow:  "PricingPspReviewWorkflow",
			TaskQueue: pricingPspReviewTaskQueue,
			// referenceDate is intentionally omitted; the workflow determines
			// the current date from workflow.Now() when needed.
		},
	})
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			slog.Warn("pricing psp review schedule already exists — skipping",
				"scheduleId", PricingPspReviewScheduleID)
			return nil
		}
		return fmt.Errorf("criar schedule pricing-psp-review: %w", err)
	}
	_ = handle // handle available for future describe/trigger/pause operations

	slog.Info("pricing psp review schedule created",
		"scheduleId", PricingPspReviewScheduleID,
		"cron", pricingPspReviewCron,
		"taskQueue", pricingPspReviewTaskQueue,
	)
	return nil
}
