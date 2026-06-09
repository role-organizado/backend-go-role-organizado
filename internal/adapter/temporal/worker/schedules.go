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

	OverdueInstallmentScheduleID = "overdue-installment-daily-workflow"
	overdueInstallmentCron       = "0 6 * * *"
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

// InitFinanceReconciliationSchedule creates the daily finance reconciliation schedule.
// Cron: "0 5 * * *" (02:00 BRT = 05:00 UTC). Idempotent — skips if already exists.
func InitFinanceReconciliationSchedule(ctx context.Context, c client.Client) error {
	const scheduleID = "finance-reconciliation-daily-workflow"

	_, err := c.ScheduleClient().Create(ctx, client.ScheduleOptions{
		ID: scheduleID,
		Spec: client.ScheduleSpec{
			CronExpressions: []string{"0 5 * * *"},
		},
		Action: &client.ScheduleWorkflowAction{
			ID:        "finance-reconciliation-daily",
			Workflow:  temporalworkflow.FinanceReconciliationWorkflow,
			TaskQueue: FinanceReconciliationQueue,
			Args:      []any{""},
		},
	})
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "already") {
			slog.Info("temporal schedule already exists, skipping creation",
				"scheduleID", scheduleID)
			return nil
		}
		return fmt.Errorf("create finance reconciliation schedule %q: %w", scheduleID, err)
	}

	slog.Info("temporal schedule created",
		"scheduleID", scheduleID,
		"cron", "0 5 * * *",
		"queue", FinanceReconciliationQueue,
	)
	return nil
}

// InitOverdueInstallmentSchedule creates the daily overdue installment schedule.
// Fires at 06:00 UTC (03:00 BRT). Idempotent — skips if already exists.
func InitOverdueInstallmentSchedule(ctx context.Context, c client.Client) error {
	scheduleClient := c.ScheduleClient()

	handle := scheduleClient.GetHandle(ctx, OverdueInstallmentScheduleID)
	if _, err := handle.Describe(ctx); err == nil {
		slog.InfoContext(ctx, "overdue installment schedule already exists",
			"scheduleID", OverdueInstallmentScheduleID)
		return nil
	}

	_, err := scheduleClient.Create(ctx, client.ScheduleOptions{
		ID: OverdueInstallmentScheduleID,
		Spec: client.ScheduleSpec{
			CronExpressions: []string{overdueInstallmentCron},
		},
		Action: &client.ScheduleWorkflowAction{
			Workflow:  temporalworkflow.OverdueInstallmentWorkflow,
			TaskQueue: OverdueInstallmentQueue,
		},
	})
	if err != nil {
		return fmt.Errorf("creating overdue installment schedule: %w", err)
	}

	slog.InfoContext(ctx, "overdue installment schedule created",
		"scheduleID", OverdueInstallmentScheduleID,
		"cron", overdueInstallmentCron,
	)
	return nil
}
