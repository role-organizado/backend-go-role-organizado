package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/activity"
)

// WorkflowState represents the lifecycle state of a PaymentExpirationWorkflow.
// States mirror the Java PaymentExpirationWorkflow states for operational consistency.
type WorkflowState string

const (
	// WorkflowStateWaiting indicates the workflow is waiting for the expiry timer.
	WorkflowStateWaiting WorkflowState = "WAITING"
	// WorkflowStateExpiring indicates the timer fired and the expiry activity is running.
	WorkflowStateExpiring WorkflowState = "EXPIRING"
	// WorkflowStateCompleted indicates the expiry activity ran successfully.
	WorkflowStateCompleted WorkflowState = "COMPLETED"
	// WorkflowStateFailed indicates the expiry activity failed.
	WorkflowStateFailed WorkflowState = "FAILED"
	// WorkflowStateSkipped indicates the workflow exited without action (e.g. already expired).
	WorkflowStateSkipped WorkflowState = "SKIPPED"
	// WorkflowStatePaid indicates the paymentCompleted signal was received before expiry.
	WorkflowStatePaid WorkflowState = "PAID"
)

// SignalPaymentCompleted is the Temporal signal name used to interrupt expiration.
//
// IMPORTANT: This MUST match the signal name used by the Java backend webhook handler
// (Java: SignalNames.PAYMENT_COMPLETED or equivalent).
// If the names diverge, webhook signals from Java will not reach the Go workflow.
const SignalPaymentCompleted = "paymentCompleted"

// PaymentExpirationInput is the input for PaymentExpirationWorkflow.
type PaymentExpirationInput struct {
	// TransactionID is the platform-internal transaction ID (used for activity calls
	// and workflow queries).
	TransactionID string `json:"transactionId"`
	// ExpiresAtMillis is the Unix epoch timestamp (milliseconds) at which the
	// transaction should be considered expired if no payment has been confirmed.
	ExpiresAtMillis int64 `json:"expiresAtMillis"`
	// CorrelationID is an optional tracing identifier propagated from the caller.
	CorrelationID string `json:"correlationId,omitempty"`
}

// PaymentExpirationWorkflow waits until the expiry time, then marks the transaction
// as CANCELLED (TIMEOUT) unless a paymentCompleted signal arrives first.
//
// Behaviour:
//   - Registers "getWorkflowStatus" and "getCurrentState" query handlers (Java parity).
//   - Races workflow.NewTimer vs workflow.GetSignalChannel("paymentCompleted").
//   - Signal wins → state = PAID, workflow exits cleanly.
//   - Timer fires → state = EXPIRING, calls ExpireTransaction activity, state = COMPLETED.
//   - Already past expiry on start → state = SKIPPED (edge case protection).
func PaymentExpirationWorkflow(ctx workflow.Context, input PaymentExpirationInput) error {
	state := WorkflowStateWaiting

	// ── Query handlers ──────────────────────────────────────────────────────────
	if err := workflow.SetQueryHandler(ctx, "getWorkflowStatus", func() (string, error) {
		return string(state), nil
	}); err != nil {
		return fmt.Errorf("register getWorkflowStatus query: %w", err)
	}
	if err := workflow.SetQueryHandler(ctx, "getCurrentState", func() (string, error) {
		return string(state), nil
	}); err != nil {
		return fmt.Errorf("register getCurrentState query: %w", err)
	}

	// ── Calculate sleep duration ────────────────────────────────────────────────
	expiresAt := time.UnixMilli(input.ExpiresAtMillis)
	now := workflow.Now(ctx)
	sleepDuration := expiresAt.Sub(now)

	if sleepDuration <= 0 {
		// Transaction already past its expiry time — skip gracefully.
		state = WorkflowStateSkipped
		return nil
	}

	// ── Race: timer vs paymentCompleted signal ─────────────────────────────────
	timerCtx, cancelTimer := workflow.WithCancel(ctx)
	defer cancelTimer()

	timer := workflow.NewTimer(timerCtx, sleepDuration)
	signalCh := workflow.GetSignalChannel(ctx, SignalPaymentCompleted)

	timerFired := false
	selector := workflow.NewSelector(ctx)

	selector.AddFuture(timer, func(f workflow.Future) {
		// If the timer was cancelled (signal arrived first), f.Get returns an error.
		if err := f.Get(ctx, nil); err == nil {
			timerFired = true
		}
	})

	selector.AddReceive(signalCh, func(ch workflow.ReceiveChannel, _ bool) {
		var reason string
		ch.Receive(ctx, &reason)
		cancelTimer() // cancel the pending timer
	})

	// Block until either the timer fires or a signal is received.
	selector.Select(ctx)

	if !timerFired {
		// Signal arrived first — payment was completed.
		state = WorkflowStatePaid
		return nil
	}

	// ── Timer fired: expire the transaction ────────────────────────────────────
	state = WorkflowStateExpiring

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:    3,
			InitialInterval:    5 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    30 * time.Second,
			// If the transaction is already terminal, retrying is pointless.
			NonRetryableErrorTypes: []string{"TransactionAlreadyTerminal"},
		},
	}
	actCtx := workflow.WithActivityOptions(ctx, ao)

	// Use a typed nil pointer so Temporal resolves the activity by method name at
	// registration time — avoids string-based dispatch while keeping the activity
	// package as a clean dependency.
	var a *activity.PaymentActivities
	if err := workflow.ExecuteActivity(actCtx, a.ExpireTransaction, input.TransactionID).Get(ctx, nil); err != nil {
		state = WorkflowStateFailed
		return fmt.Errorf("payment expiration: expire transaction activity: %w", err)
	}

	state = WorkflowStateCompleted
	return nil
}
