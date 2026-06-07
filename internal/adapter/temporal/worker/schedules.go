package worker

import (
	"context"
	"fmt"
	"log/slog"

	"go.temporal.io/sdk/client"

	temporalworkflow "github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/workflow"
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
//
// Uses upsert semantics: if the schedule already exists (e.g. created by the Java
// backend), it is updated with the Go workflow definition. If the Java schedule
// already covers the same workflow on the same task queue, this call is a no-op
// after initial setup.
//
// Java reference: app.temporal.schedules.psp-reconciliation (6 AM daily, UTC).
func (si *ScheduleInitializer) InitializeReconciliationSchedule(ctx context.Context) error {
	scheduleID := temporalworkflow.PspReconciliationScheduledID

	handle := si.client.ScheduleClient().GetHandle(ctx, scheduleID)
	_, descErr := handle.Describe(ctx)
	if descErr == nil {
		// Schedule already exists — skip to avoid conflicting with Java-managed schedule.
		slog.InfoContext(ctx, "temporal: reconciliation schedule already exists, skipping upsert",
			"scheduleID", scheduleID)
		return nil
	}

	_, err := si.client.ScheduleClient().Create(ctx, client.ScheduleOptions{
		ID: scheduleID,
		Spec: client.ScheduleSpec{
			// 06:00 daily UTC — matches Java app.temporal.schedules.psp-reconciliation.
			CronExpressions: []string{"0 6 * * *"},
		},
		Action: &client.ScheduleWorkflowAction{
			Workflow:  temporalworkflow.ReconciliationWorkflow,
			TaskQueue: temporalworkflow.ReconciliationTaskQueue,
		},
		// Default overlap policy (SKIP) — prevents concurrent runs if previous is still running.
	})
	if err != nil {
		return fmt.Errorf("temporal: create reconciliation schedule: %w", err)
	}

	slog.InfoContext(ctx, "temporal: reconciliation schedule created", "scheduleID", scheduleID)
	return nil
}
