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

// FinanceReconciliationResult is the result of a Finance Triple Reconciliation activity.
// Mirrors Java's FinanceReconciliationActivityResult record.
type FinanceReconciliationResult struct {
	EventsChecked int    `json:"eventsChecked"`
	Divergences   int    `json:"divergences"`
	Critical      int    `json:"critical"`
	DurationMs    int64  `json:"durationMs"`
	Completed     bool   `json:"completed"`
	Message       string `json:"message"`
}

// NewFinanceReconciliationResultSuccess creates a successful reconciliation result.
func NewFinanceReconciliationResultSuccess(eventsChecked, divergences, critical int, durationMs int64) FinanceReconciliationResult {
	return FinanceReconciliationResult{
		EventsChecked: eventsChecked,
		Divergences:   divergences,
		Critical:      critical,
		DurationMs:    durationMs,
		Completed:     true,
		Message: fmt.Sprintf("Triple reconciliation completed: checked=%d divergences=%d critical=%d",
			eventsChecked, divergences, critical),
	}
}

// NewFinanceReconciliationResultError creates a failed reconciliation result.
func NewFinanceReconciliationResultError(msg string) FinanceReconciliationResult {
	return FinanceReconciliationResult{
		Completed: false,
		Message:   msg,
	}
}

// ── Activity interface ────────────────────────────────────────────────────────

// FinanceReconciliationActivities holds the activity methods for the finance reconciliation workflow.
// Mirrors Java's FinanceReconciliationActivities interface.
type FinanceReconciliationActivities struct{}

// RunReconciliation runs the triple reconciliation check for the given reference date.
// Mirrors Java's FinanceReconciliationActivities.runReconciliation.
func (a *FinanceReconciliationActivities) RunReconciliation(ctx context.Context, referenceDate string) (FinanceReconciliationResult, error) {
	activity.GetLogger(ctx).Info("finance reconciliation activity", "referenceDate", referenceDate)
	return FinanceReconciliationResult{}, fmt.Errorf("not implemented")
}

// ── Workflow ──────────────────────────────────────────────────────────────────

// FinanceReconciliationWorkflow is the Temporal workflow for Finance Triple Reconciliation (E3-02).
// Runs daily at 02:00 BRT via Temporal Schedule, replacing the legacy @Scheduled job.
// Mirrors Java's FinanceReconciliationWorkflowImpl.
//
// States: PENDING → RUNNING → COMPLETED | FAILED
func FinanceReconciliationWorkflow(ctx workflow.Context, referenceDate string) error {
	logger := workflow.GetLogger(ctx)

	// ── Mutable state (query-accessible) ─────────────────────────────────────
	workflowStatus := "PENDING"
	currentState := "PENDING"
	var result *FinanceReconciliationResult

	// ── Query handlers (registered before first yield) ───────────────────────
	if err := workflow.SetQueryHandler(ctx, "getWorkflowStatus", func() (string, error) {
		return workflowStatus, nil
	}); err != nil {
		return err
	}
	if err := workflow.SetQueryHandler(ctx, "getCurrentState", func() (string, error) {
		return currentState, nil
	}); err != nil {
		return err
	}
	if err := workflow.SetQueryHandler(ctx, "getResult", func() (*FinanceReconciliationResult, error) {
		return result, nil
	}); err != nil {
		return err
	}

	// ── Activity options ──────────────────────────────────────────────────────
	ao := workflow.ActivityOptions{
		StartToCloseTimeout:    15 * time.Minute,
		ScheduleToCloseTimeout: 45 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    30 * time.Second,
			MaximumAttempts:    3,
			BackoffCoefficient: 2.0,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	workflowStatus = "RUNNING"
	currentState = "RUNNING"
	logger.Info("[FINANCE-RECONCILIATION] Starting", "referenceDate", referenceDate)

	// ── Execute activity ──────────────────────────────────────────────────────
	var a *FinanceReconciliationActivities
	var actResult FinanceReconciliationResult
	err := workflow.ExecuteActivity(ctx, a.RunReconciliation, referenceDate).Get(ctx, &actResult)
	if err != nil {
		workflowStatus = "FAILED"
		currentState = "FAILED"
		logger.Error("[FINANCE-RECONCILIATION] Activity failed", "error", err)
		return err
	}

	result = &actResult
	if actResult.Completed {
		workflowStatus = "COMPLETED"
		currentState = "COMPLETED"
		logger.Info("[FINANCE-RECONCILIATION] Completed",
			"referenceDate", referenceDate,
			"divergences", actResult.Divergences,
			"critical", actResult.Critical)
	} else {
		workflowStatus = "FAILED"
		currentState = "FAILED"
		logger.Error("[FINANCE-RECONCILIATION] Activity returned non-completed result", "message", actResult.Message)
	}

	return nil
}
