package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/activity"
)

// OutboundExecutionInput is the input for OutboundExecutionWorkflow.
type OutboundExecutionInput struct {
	RequestID      string `json:"requestId"`
	EventID        string `json:"eventId"`
	ApproverUserID string `json:"approverUserId"`
	ApprovalNotes  string `json:"approvalNotes"`
}

// OutboundProviderCallback is the payload of the onProviderCallback signal.
type OutboundProviderCallback struct {
	Provider        string `json:"provider"`
	ProviderStatus  string `json:"providerStatus"`
	Reason          string `json:"reason"`
	ProviderEventID string `json:"providerEventId"`
}

// OutboundExecutionState is the observable state of an outbound execution run.
type OutboundExecutionState struct {
	// Status is one of "PREPARING", "AWAITING_CALLBACK", "APPLYING_CALLBACK",
	// "TIMED_OUT", "FINALIZING", "COMPLETED", or "FAILED".
	Status           string `json:"status"`
	RequestID        string `json:"requestId"`
	Provider         string `json:"provider"`
	ProviderStatus   string `json:"providerStatus"`
	ProviderEventID  string `json:"providerEventId"`
	CallbackReceived bool   `json:"callbackReceived"`
	TimedOut         bool   `json:"timedOut"`
}

// outboundCallbackTimeout (CALLBACK_TIMEOUT) bounds how long the workflow waits
// for a provider callback before falling back to timeout handling.
const outboundCallbackTimeout = 15 * time.Minute

// OutboundExecutionWorkflow orchestrates an approved outbound transfer: it
// approves the request, awaits an asynchronous provider callback (up to
// CALLBACK_TIMEOUT), applies the callback (or handles the timeout), then
// finalizes.
//
// Versioning: gated on change ID "outbound-execution-v2".
// Signal:     onProviderCallback(provider, providerStatus, reason, providerEventId)
// Queries:    getCurrentState, getWorkflowStatus
func OutboundExecutionWorkflow(ctx workflow.Context, input OutboundExecutionInput) error {
	_ = workflow.GetVersion(ctx, "outbound-execution-v2", workflow.DefaultVersion, 2)

	state := OutboundExecutionState{
		Status:    "PREPARING",
		RequestID: input.RequestID,
	}

	_ = workflow.SetQueryHandler(ctx, "getCurrentState", func() (OutboundExecutionState, error) {
		return state, nil
	})
	_ = workflow.SetQueryHandler(ctx, "getWorkflowStatus", func() (string, error) {
		return state.Status, nil
	})

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:    3,
			InitialInterval:    5 * time.Second,
			BackoffCoefficient: 2.0,
		},
	}
	actCtx := workflow.WithActivityOptions(ctx, ao)

	baseInput := activity.OutboundExecutionActivityInput{
		RequestID:      input.RequestID,
		EventID:        input.EventID,
		ApproverUserID: input.ApproverUserID,
		ApprovalNotes:  input.ApprovalNotes,
	}

	if err := workflow.ExecuteActivity(actCtx, "PrepareExecution", baseInput).Get(ctx, nil); err != nil {
		state.Status = "FAILED"
		return fmt.Errorf("outbound prepare execution: %w", err)
	}

	state.Status = "AWAITING_CALLBACK"
	if err := workflow.ExecuteActivity(actCtx, "MarkAwaitingCallback", baseInput).Get(ctx, nil); err != nil {
		state.Status = "FAILED"
		return fmt.Errorf("outbound mark awaiting callback: %w", err)
	}

	callbackCh := workflow.GetSignalChannel(ctx, "onProviderCallback")
	var cb OutboundProviderCallback
	received := false

	timerCtx, cancelTimer := workflow.WithCancel(ctx)
	sel := workflow.NewSelector(ctx)
	sel.AddReceive(callbackCh, func(c workflow.ReceiveChannel, _ bool) {
		c.Receive(ctx, &cb)
		received = true
	})
	sel.AddFuture(workflow.NewTimer(timerCtx, outboundCallbackTimeout), func(workflow.Future) {
		state.TimedOut = true
	})
	sel.Select(ctx)
	cancelTimer()

	if received {
		state.Status = "APPLYING_CALLBACK"
		state.CallbackReceived = true
		state.Provider = cb.Provider
		state.ProviderStatus = cb.ProviderStatus
		state.ProviderEventID = cb.ProviderEventID

		cbInput := activity.OutboundCallbackActivityInput{
			RequestID:       input.RequestID,
			Provider:        cb.Provider,
			ProviderStatus:  cb.ProviderStatus,
			Reason:          cb.Reason,
			ProviderEventID: cb.ProviderEventID,
		}
		if err := workflow.ExecuteActivity(actCtx, "ApplyCallback", cbInput).Get(ctx, nil); err != nil {
			state.Status = "FAILED"
			return fmt.Errorf("outbound apply callback: %w", err)
		}
	} else {
		state.Status = "TIMED_OUT"
		if err := workflow.ExecuteActivity(actCtx, "HandleTimeout", baseInput).Get(ctx, nil); err != nil {
			state.Status = "FAILED"
			return fmt.Errorf("outbound handle timeout: %w", err)
		}
	}

	state.Status = "FINALIZING"
	if err := workflow.ExecuteActivity(actCtx, "FinalizeExecution", baseInput).Get(ctx, nil); err != nil {
		state.Status = "FAILED"
		return fmt.Errorf("outbound finalize execution: %w", err)
	}

	state.Status = "COMPLETED"
	return nil
}
