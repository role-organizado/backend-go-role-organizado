package worker

import (
	"context"
	"fmt"
	"log/slog"

	"go.temporal.io/sdk/client"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/workflow"
)

// Schedule constants for the overdue-installment daily job.
const (
	// OverdueInstallmentScheduleID is the stable schedule identifier in Temporal.
	OverdueInstallmentScheduleID = "overdue-installment-daily-workflow"

	// overdueInstallmentCron fires at 06:00 UTC = 03:00 BRT (America/Sao_Paulo).
	overdueInstallmentCron = "0 6 * * *"
)

// InitOverdueInstallmentSchedule creates the daily schedule if it does not already exist.
// Idempotent: safely called on every startup.
func InitOverdueInstallmentSchedule(ctx context.Context, c client.Client) error {
	scheduleClient := c.ScheduleClient()

	// Check for idempotency: if the schedule already exists, Describe returns nil error.
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
			Workflow:  workflow.OverdueInstallmentWorkflow,
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
