package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/activity"
)

// ─── Signals ──────────────────────────────────────────────────────────────────
// Signal names MUST match the Java ParticipantLifecycleWorkflowImpl so signals
// sent by the Java backend reach the Go workflow during the Strangler Fig migration.

const (
	// SignalProceedWithChange instructs the workflow to apply the recalculation.
	SignalProceedWithChange = "proceedWithChange"
	// SignalCancelOperation instructs the workflow to abort without applying.
	SignalCancelOperation = "cancelOperation"
)

// ParticipantRecalculation lifecycle statuses (mirror the Java workflow states).
const (
	ParticipantRecalcStatusCalculating          = "CALCULATING"
	ParticipantRecalcStatusAwaitingConfirmation = "AWAITING_CONFIRMATION"
	ParticipantRecalcStatusApplying             = "APPLYING"
	ParticipantRecalcStatusCompleted            = "COMPLETED"
	ParticipantRecalcStatusCancelled            = "CANCELLED"
	ParticipantRecalcStatusFailed               = "FAILED"
)

// participantRecalcConfirmationTimeout bounds how long the workflow waits for an
// operator decision. On timeout the operation is cancelled (fail-safe default).
const participantRecalcConfirmationTimeout = 24 * time.Hour

// ParticipantRecalculationInput is the input for ParticipantRecalculationWorkflow.
type ParticipantRecalculationInput struct {
	// EventID identifies the event whose finance summary is being recalculated.
	EventID string `json:"eventId"`
	// UserID is the operator that triggered the recalculation (optional).
	UserID string `json:"userId,omitempty"`
	// CorrelationID is an optional tracing identifier propagated from the caller.
	CorrelationID string `json:"correlationId,omitempty"`
}

// ParticipantRecalculationState holds the observable state of a recalculation run.
type ParticipantRecalculationState struct {
	// Status is one of the ParticipantRecalcStatus* constants.
	Status string `json:"status"`
	// Preview holds the projected finance summary once CALCULATING completes.
	Preview *activity.CalculationPreview `json:"preview,omitempty"`
	// Result is a human-readable summary after the workflow terminates.
	Result string `json:"result,omitempty"`
}

// ParticipantRecalculationWorkflow recalculates an event's finance summary with a
// human-in-the-loop confirmation step, mirroring the Java
// ParticipantLifecycleWorkflowImpl.
//
// Flow:
//  1. CALCULATING — run CalculatePreview activity to project the impact.
//  2. AWAITING_CONFIRMATION — wait for proceedWithChange or cancelOperation.
//  3. proceedWithChange → APPLYING → ApplyRecalculation → COMPLETED.
//     cancelOperation (or timeout) → CANCELLED.
//
// Signals: proceedWithChange, cancelOperation.
// Queries: getCurrentState, getWorkflowStatus, getCalculationPreview.
func ParticipantRecalculationWorkflow(ctx workflow.Context, input ParticipantRecalculationInput) error {
	state := ParticipantRecalculationState{Status: ParticipantRecalcStatusCalculating}

	// ── Query handlers ──────────────────────────────────────────────────────────
	if err := workflow.SetQueryHandler(ctx, "getWorkflowStatus", func() (string, error) {
		return state.Status, nil
	}); err != nil {
		return fmt.Errorf("register getWorkflowStatus query: %w", err)
	}
	if err := workflow.SetQueryHandler(ctx, "getCurrentState", func() (ParticipantRecalculationState, error) {
		return state, nil
	}); err != nil {
		return fmt.Errorf("register getCurrentState query: %w", err)
	}
	if err := workflow.SetQueryHandler(ctx, "getCalculationPreview", func() (*activity.CalculationPreview, error) {
		return state.Preview, nil
	}); err != nil {
		return fmt.Errorf("register getCalculationPreview query: %w", err)
	}

	retryPolicy := &temporal.RetryPolicy{MaximumAttempts: 3}
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy:         retryPolicy,
	}
	actCtx := workflow.WithActivityOptions(ctx, ao)

	// ── Step 1: compute preview ───────────────────────────────────────────────
	var preview activity.CalculationPreview
	if err := workflow.ExecuteActivity(actCtx, "CalculatePreview", input.EventID).Get(actCtx, &preview); err != nil {
		state.Status = ParticipantRecalcStatusFailed
		return fmt.Errorf("participant recalculation: calculate preview: %w", err)
	}
	state.Preview = &preview
	state.Status = ParticipantRecalcStatusAwaitingConfirmation

	// ── Step 2: await operator decision ───────────────────────────────────────
	proceedCh := workflow.GetSignalChannel(ctx, SignalProceedWithChange)
	cancelCh := workflow.GetSignalChannel(ctx, SignalCancelOperation)

	proceed := false
	cancelled := false

	selector := workflow.NewSelector(ctx)
	selector.AddReceive(proceedCh, func(ch workflow.ReceiveChannel, _ bool) {
		ch.Receive(ctx, nil)
		proceed = true
	})
	selector.AddReceive(cancelCh, func(ch workflow.ReceiveChannel, _ bool) {
		ch.Receive(ctx, nil)
		cancelled = true
	})

	// Bound the wait: on timeout, default to cancelling the operation.
	timerCtx, cancelTimer := workflow.WithCancel(ctx)
	defer cancelTimer()
	timedOut := false
	selector.AddFuture(workflow.NewTimer(timerCtx, participantRecalcConfirmationTimeout), func(f workflow.Future) {
		if err := f.Get(ctx, nil); err == nil {
			timedOut = true
		}
	})

	selector.Select(ctx)
	cancelTimer()

	if cancelled || timedOut || !proceed {
		state.Status = ParticipantRecalcStatusCancelled
		if timedOut {
			state.Result = "recalculation cancelled: confirmation timed out"
		} else {
			state.Result = "recalculation cancelled by operator"
		}
		return nil
	}

	// ── Step 3: apply the recalculation ───────────────────────────────────────
	state.Status = ParticipantRecalcStatusApplying
	if err := workflow.ExecuteActivity(actCtx, "ApplyRecalculation", input.EventID).Get(actCtx, nil); err != nil {
		state.Status = ParticipantRecalcStatusFailed
		return fmt.Errorf("participant recalculation: apply: %w", err)
	}

	state.Status = ParticipantRecalcStatusCompleted
	state.Result = fmt.Sprintf("recalculation applied for event %s", input.EventID)
	return nil
}
