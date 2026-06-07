package worker

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"go.temporal.io/sdk/client"

	temporalworkflow "github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/workflow"
)

// InitFinanceReconciliationSchedule creates the daily finance reconciliation schedule.
//
// Schedule ID  : "finance-reconciliation-daily-workflow"
// Cron         : "0 5 * * *"  (02:00 BRT = 05:00 UTC)
// Task queue   : FINANCE_RECONCILIATION_QUEUE
//
// The workflow is started with an empty referenceDate; the workflow itself computes
// yesterday's date (UTC) at execution time — the expected reference for a nightly run.
//
// Idempotent: if the schedule already exists the function logs and returns nil.
func InitFinanceReconciliationSchedule(ctx context.Context, c client.Client) error {
	const scheduleID = "finance-reconciliation-daily-workflow"

	_, err := c.ScheduleClient().Create(ctx, client.ScheduleOptions{
		ID: scheduleID,
		Spec: client.ScheduleSpec{
			// 02:00 BRT = 05:00 UTC (BRT = UTC-3)
			CronExpressions: []string{"0 5 * * *"},
		},
		Action: &client.ScheduleWorkflowAction{
			// WorkflowID for scheduled runs — distinct from manual runs which use
			// ids.FinanceReconciliationPrimaryID(referenceDate).
			ID:        "finance-reconciliation-daily",
			Workflow:  temporalworkflow.FinanceReconciliationWorkflow,
			TaskQueue: FinanceReconciliationQueue,
			// Empty referenceDate → workflow computes yesterday's date at execution time.
			Args: []any{""},
		},
	})
	if err != nil {
		// Idempotent: skip if schedule already exists.
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
