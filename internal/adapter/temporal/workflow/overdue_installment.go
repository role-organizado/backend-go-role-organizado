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

// OverdueInstallmentResult is the result of an Overdue Installment workflow execution.
// Mirrors Java's OverdueInstallmentActivityResult record but split across two Go activities.
type OverdueInstallmentResult struct {
	InstallmentsProcessed int    `json:"installmentsProcessed"`
	NotificationsSent     int    `json:"notificationsSent"`
	DurationMs            int64  `json:"durationMs"`
	Completed             bool   `json:"completed"`
	Message               string `json:"message"`
}

// NewOverdueInstallmentResultSuccess creates a successful result.
func NewOverdueInstallmentResultSuccess(installments, notifications int, durationMs int64) OverdueInstallmentResult {
	return OverdueInstallmentResult{
		InstallmentsProcessed: installments,
		NotificationsSent:     notifications,
		DurationMs:            durationMs,
		Completed:             true,
		Message: fmt.Sprintf("Processed %d installments, %d notifications sent in %d ms",
			installments, notifications, durationMs),
	}
}

// NewOverdueInstallmentResultError creates a failed result.
func NewOverdueInstallmentResultError(msg string) OverdueInstallmentResult {
	return OverdueInstallmentResult{Completed: false, Message: msg}
}

// ── Activity interface ────────────────────────────────────────────────────────

// OverdueInstallmentActivities holds the activity methods for the overdue installment workflow.
// Split from Java's single processOverdueInstallments into two discrete activities for
// better observability and retry granularity.
type OverdueInstallmentActivities struct{}

// FindAndMarkOverdueInstallments finds all PENDING installments past their due date,
// marks them as OVERDUE, and returns the count of installments processed.
func (a *OverdueInstallmentActivities) FindAndMarkOverdueInstallments(ctx context.Context, referenceDate string) (int, error) {
	activity.GetLogger(ctx).Info("find and mark overdue installments", "referenceDate", referenceDate)
	return 0, fmt.Errorf("not implemented")
}

// DispatchNotifications dispatches notification events for the given number of
// overdue installments. Only called when count > 0.
func (a *OverdueInstallmentActivities) DispatchNotifications(ctx context.Context, count int) error {
	activity.GetLogger(ctx).Info("dispatch overdue notifications", "count", count)
	return fmt.Errorf("not implemented")
}

// ── Workflow ──────────────────────────────────────────────────────────────────

// OverdueInstallmentWorkflow is the Temporal workflow for Overdue Installment detection (E3-06).
// Runs daily at 03:00 BRT via Temporal Schedule, replacing the legacy @Scheduled job.
//
// Logic:
//  1. FindAndMarkOverdueInstallments → count
//  2. If count > 0: DispatchNotifications(count)   [skipped when count == 0]
//
// States: PENDING → RUNNING → COMPLETED | FAILED
func OverdueInstallmentWorkflow(ctx workflow.Context, referenceDate string) (*OverdueInstallmentResult, error) {
	logger := workflow.GetLogger(ctx)

	// ── Mutable state (query-accessible) ─────────────────────────────────────
	workflowStatus := "PENDING"
	currentState := "PENDING"
	var result *OverdueInstallmentResult

	// ── Query handlers (registered before first yield) ───────────────────────
	if err := workflow.SetQueryHandler(ctx, "getWorkflowStatus", func() (string, error) {
		return workflowStatus, nil
	}); err != nil {
		return nil, err
	}
	if err := workflow.SetQueryHandler(ctx, "getCurrentState", func() (string, error) {
		return currentState, nil
	}); err != nil {
		return nil, err
	}
	if err := workflow.SetQueryHandler(ctx, "getResult", func() (*OverdueInstallmentResult, error) {
		return result, nil
	}); err != nil {
		return nil, err
	}

	// ── Activity options ──────────────────────────────────────────────────────
	ao := workflow.ActivityOptions{
		StartToCloseTimeout:    10 * time.Minute,
		ScheduleToCloseTimeout: 30 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    30 * time.Second,
			MaximumAttempts:    3,
			BackoffCoefficient: 2.0,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	workflowStatus = "RUNNING"
	currentState = "RUNNING"
	logger.Info("[OVERDUE-INSTALLMENT] Starting", "referenceDate", referenceDate)

	var a *OverdueInstallmentActivities

	// ── Activity 1: find and mark overdue installments ────────────────────────
	var count int
	if err := workflow.ExecuteActivity(ctx, a.FindAndMarkOverdueInstallments, referenceDate).Get(ctx, &count); err != nil {
		workflowStatus = "FAILED"
		currentState = "FAILED"
		logger.Error("[OVERDUE-INSTALLMENT] FindAndMarkOverdueInstallments failed", "error", err)
		return nil, err
	}

	logger.Info("[OVERDUE-INSTALLMENT] Installments found", "count", count)

	// ── Activity 2: dispatch notifications (only if there are overdue items) ──
	notificationsSent := 0
	if count > 0 {
		if err := workflow.ExecuteActivity(ctx, a.DispatchNotifications, count).Get(ctx, nil); err != nil {
			workflowStatus = "FAILED"
			currentState = "FAILED"
			logger.Error("[OVERDUE-INSTALLMENT] DispatchNotifications failed", "error", err)
			return nil, err
		}
		notificationsSent = count
	}

	workflowStatus = "COMPLETED"
	currentState = "COMPLETED"

	r := &OverdueInstallmentResult{
		InstallmentsProcessed: count,
		NotificationsSent:     notificationsSent,
		DurationMs:            0, // populated by caller / observability layer
		Completed:             true,
		Message:               fmt.Sprintf("Processed %d installments, %d notifications sent", count, notificationsSent),
	}
	result = r

	logger.Info("[OVERDUE-INSTALLMENT] Completed",
		"referenceDate", referenceDate,
		"count", count,
		"notificationsSent", notificationsSent)

	return r, nil
}
