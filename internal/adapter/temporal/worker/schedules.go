package worker

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"go.temporal.io/sdk/client"

	temporalworkflow "github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/workflow"
)

const (
	PricingPspReviewScheduleID = "pricing-psp-review-daily-workflow"
	pricingPspReviewTaskQueue  = "PRICING_PSP_REVIEW_QUEUE"
	pricingPspReviewCron       = "30 5 * * *"
)

// ScheduleInitializer creates and upserts Temporal Schedules for periodic workflows.
// It mirrors the Java TemporalScheduleInitializer / app.temporal.schedules configuration.
type ScheduleInitializer struct {
	client client.Client
}

// NewScheduleInitializer creates a new ScheduleInitializer backed by a Temporal client.
func NewScheduleInitializer(c client.Client) *ScheduleInitializer {
	return &ScheduleInitializer{client: c}
}

// InitializeReconciliationSchedule creates or updates the daily PSP reconciliation
// Temporal Schedule. The schedule triggers ReconciliationWorkflow every day at 06:00.
func (si *ScheduleInitializer) InitializeReconciliationSchedule(ctx context.Context) error {
	scheduleID := temporalworkflow.PspReconciliationScheduledID

	handle := si.client.ScheduleClient().GetHandle(ctx, scheduleID)
	_, descErr := handle.Describe(ctx)
	if descErr == nil {
		slog.InfoContext(ctx, "temporal: reconciliation schedule already exists, skipping upsert",
			"scheduleID", scheduleID)
		return nil
	}

	_, err := si.client.ScheduleClient().Create(ctx, client.ScheduleOptions{
		ID: scheduleID,
		Spec: client.ScheduleSpec{
			CronExpressions: []string{"0 6 * * *"},
		},
		Action: &client.ScheduleWorkflowAction{
			Workflow:  temporalworkflow.ReconciliationWorkflow,
			TaskQueue: temporalworkflow.ReconciliationTaskQueue,
		},
	})
	if err != nil {
		return fmt.Errorf("temporal: create reconciliation schedule: %w", err)
	}

	slog.InfoContext(ctx, "temporal: reconciliation schedule created", "scheduleID", scheduleID)
	return nil
}

// InitPricingPspReviewSchedule creates the daily Temporal schedule for PricingPspReviewWorkflow.
// Fires at 02:30 BRT (05:30 UTC) every day. Idempotent — skips if already exists.
func (r *Registry) InitPricingPspReviewSchedule(ctx context.Context) error {
	handle, err := r.client.ScheduleClient().Create(ctx, client.ScheduleOptions{
		ID: PricingPspReviewScheduleID,
		Spec: client.ScheduleSpec{
			CronExpressions: []string{pricingPspReviewCron},
		},
		Action: &client.ScheduleWorkflowAction{
			Workflow:  "PricingPspReviewWorkflow",
			TaskQueue: pricingPspReviewTaskQueue,
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
	_ = handle

	slog.Info("pricing psp review schedule created",
		"scheduleId", PricingPspReviewScheduleID,
		"cron", pricingPspReviewCron,
		"taskQueue", pricingPspReviewTaskQueue,
	)
	return nil
}
