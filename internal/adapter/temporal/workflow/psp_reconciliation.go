package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/activity"
)

// ─── Signals ──────────────────────────────────────────────────────────────────
// Signal names MUST match the Java PspReconciliationWorkflowImpl so signals sent
// by the Java backend reach the Go workflow during the Strangler Fig migration.

const (
	// SignalPauseReconciliation pauses the workflow before the reconciliation activity runs.
	SignalPauseReconciliation = "pauseReconciliation"
	// SignalResumeReconciliation resumes a paused workflow.
	SignalResumeReconciliation = "resumeReconciliation"
	// SignalCancelReconciliation cancels the workflow without running the activity.
	SignalCancelReconciliation = "cancelReconciliation"
)

// PspReconciliation lifecycle statuses.
const (
	PspReconStatusRunning   = "RUNNING"
	PspReconStatusPaused    = "PAUSED"
	PspReconStatusCompleted = "COMPLETED"
	PspReconStatusCancelled = "CANCELLED"
	PspReconStatusFailed    = "FAILED"
)

// PspReconciliationInput is the input for PspReconciliationWorkflow.
type PspReconciliationInput struct {
	// ScopeID identifies the reconciliation scope (used in the workflow ID).
	ScopeID string `json:"scopeId"`
	// ReferenceDate labels this run (e.g. "2026-06-07").
	ReferenceDate string `json:"referenceDate"`
	// From / To bound the reconciliation window. Empty From defaults to last 24h.
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

// PspReconciliationState holds the observable state of a reconciliation run.
type PspReconciliationState struct {
	Status string                            `json:"status"`
	Paused bool                              `json:"paused"`
	Result *activity.PspReconciliationResult `json:"result,omitempty"`
}

// PspReconciliationWorkflow reconciles local transaction states against the PSP,
// mirroring the Java PspReconciliationWorkflowImpl. It supports pause/resume/cancel
// signals so operators can interrupt long-running reconciliation windows.
//
// ActivityOptions: 10-minute StartToCloseTimeout, 30-minute ScheduleToClose, 3 retries.
//
// Signals: pauseReconciliation, resumeReconciliation, cancelReconciliation.
// Queries: getWorkflowStatus, getCurrentState.
func PspReconciliationWorkflow(ctx workflow.Context, input PspReconciliationInput) error {
	state := PspReconciliationState{Status: PspReconStatusRunning}

	if err := workflow.SetQueryHandler(ctx, "getWorkflowStatus", func() (string, error) {
		return state.Status, nil
	}); err != nil {
		return fmt.Errorf("register getWorkflowStatus query: %w", err)
	}
	if err := workflow.SetQueryHandler(ctx, "getCurrentState", func() (PspReconciliationState, error) {
		return state, nil
	}); err != nil {
		return fmt.Errorf("register getCurrentState query: %w", err)
	}

	// Default window: last 24 hours if not explicitly set.
	from := input.From
	to := input.To
	if from.IsZero() {
		to = workflow.Now(ctx)
		from = to.Add(-24 * time.Hour)
	}
	if to.IsZero() {
		to = workflow.Now(ctx)
	}

	// ── Activity context (cancellable via the cancelReconciliation signal) ─────
	actBaseCtx, cancelActivity := workflow.WithCancel(ctx)
	defer cancelActivity()

	// ── Signal dispatcher ─────────────────────────────────────────────────────
	cancelled := false
	pauseCh := workflow.GetSignalChannel(ctx, SignalPauseReconciliation)
	resumeCh := workflow.GetSignalChannel(ctx, SignalResumeReconciliation)
	cancelCh := workflow.GetSignalChannel(ctx, SignalCancelReconciliation)

	workflow.Go(ctx, func(gctx workflow.Context) {
		sel := workflow.NewSelector(gctx)
		sel.AddReceive(pauseCh, func(ch workflow.ReceiveChannel, _ bool) {
			ch.Receive(gctx, nil)
			state.Paused = true
			if state.Status == PspReconStatusRunning {
				state.Status = PspReconStatusPaused
			}
		})
		sel.AddReceive(resumeCh, func(ch workflow.ReceiveChannel, _ bool) {
			ch.Receive(gctx, nil)
			state.Paused = false
			if state.Status == PspReconStatusPaused {
				state.Status = PspReconStatusRunning
			}
		})
		sel.AddReceive(cancelCh, func(ch workflow.ReceiveChannel, _ bool) {
			ch.Receive(gctx, nil)
			cancelled = true
			state.Paused = false
			cancelActivity() // abort any in-flight reconciliation activity
		})
		for !cancelled {
			sel.Select(gctx)
		}
	})

	// ── Pause gate ────────────────────────────────────────────────────────────
	// Block while paused; proceed when resumed or cancelled.
	if err := workflow.Await(ctx, func() bool { return cancelled || !state.Paused }); err != nil {
		return fmt.Errorf("psp reconciliation: await resume: %w", err)
	}
	if cancelled {
		state.Status = PspReconStatusCancelled
		return nil
	}

	// ── Run reconciliation ────────────────────────────────────────────────────
	ao := workflow.ActivityOptions{
		StartToCloseTimeout:    10 * time.Minute,
		ScheduleToCloseTimeout: 30 * time.Minute,
		RetryPolicy:            &temporal.RetryPolicy{MaximumAttempts: 3},
	}
	actCtx := workflow.WithActivityOptions(actBaseCtx, ao)

	actInput := activity.PspReconciliationActivityInput{
		ReferenceDate: input.ReferenceDate,
		From:          from,
		To:            to,
	}

	var result activity.PspReconciliationResult
	if err := workflow.ExecuteActivity(actCtx, "RunPspReconciliation", actInput).Get(actCtx, &result); err != nil {
		// A cancellation signal aborts the activity — treat as a clean cancel.
		if cancelled || temporal.IsCanceledError(err) {
			state.Status = PspReconStatusCancelled
			return nil
		}
		state.Status = PspReconStatusFailed
		return fmt.Errorf("psp reconciliation workflow: %w", err)
	}

	state.Result = &result
	state.Status = PspReconStatusCompleted
	return nil
}
