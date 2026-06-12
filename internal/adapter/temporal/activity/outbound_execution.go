package activity

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/activity"

	outbounddomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/outbound"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
)

// outboundApprover approves an outbound request, kicking off the transfer.
// Satisfied by outbound.ApproveOutboundRequest (ApproveOutboundRequestUseCase).
type outboundApprover interface {
	Execute(ctx context.Context, in portin.ApproveOutboundRequestInput) (*outbounddomain.OutboundRequest, error)
}

// outboundCallbackHandler applies a provider callback to an outbound request.
// Satisfied by outbound.HandleOutboundTransferCallback (HandleOutboundTransferCallbackUseCase).
type outboundCallbackHandler interface {
	Execute(ctx context.Context, in portin.OutboundCallbackInput) (*outbounddomain.OutboundRequest, error)
}

// OutboundExecutionActivityInput is the input for the non-callback activities.
type OutboundExecutionActivityInput struct {
	RequestID      string `json:"requestId"`
	EventID        string `json:"eventId"`
	ApproverUserID string `json:"approverUserId"`
	ApprovalNotes  string `json:"approvalNotes"`
}

// OutboundCallbackActivityInput is the input for ApplyCallback.
type OutboundCallbackActivityInput struct {
	RequestID       string `json:"requestId"`
	Provider        string `json:"provider"`
	ProviderStatus  string `json:"providerStatus"`
	Reason          string `json:"reason"`
	ProviderEventID string `json:"providerEventId"`
}

// OutboundExecutionActivities holds dependencies for outbound execution activities.
type OutboundExecutionActivities struct {
	approver        outboundApprover
	callbackHandler outboundCallbackHandler
}

// NewOutboundExecutionActivities creates a new OutboundExecutionActivities instance.
func NewOutboundExecutionActivities(approver outboundApprover, callbackHandler outboundCallbackHandler) *OutboundExecutionActivities {
	return &OutboundExecutionActivities{approver: approver, callbackHandler: callbackHandler}
}

// PrepareExecution approves the outbound request, initiating the provider transfer.
func (a *OutboundExecutionActivities) PrepareExecution(ctx context.Context, input OutboundExecutionActivityInput) error {
	logger := activity.GetLogger(ctx)
	logger.Info("preparing outbound execution (native)", "requestId", input.RequestID)

	if _, err := a.approver.Execute(ctx, portin.ApproveOutboundRequestInput{
		RequestID:      input.RequestID,
		EventID:        input.EventID,
		ApproverUserID: input.ApproverUserID,
		ApprovalNotes:  input.ApprovalNotes,
	}); err != nil {
		return fmt.Errorf("prepare outbound execution for %s: %w", input.RequestID, err)
	}
	return nil
}

// MarkAwaitingCallback records that the request is now awaiting a provider callback.
func (a *OutboundExecutionActivities) MarkAwaitingCallback(ctx context.Context, input OutboundExecutionActivityInput) error {
	activity.GetLogger(ctx).Info("outbound request awaiting provider callback (native)", "requestId", input.RequestID)
	return nil
}

// ApplyCallback applies a received provider callback to the outbound request.
func (a *OutboundExecutionActivities) ApplyCallback(ctx context.Context, input OutboundCallbackActivityInput) error {
	logger := activity.GetLogger(ctx)
	logger.Info("applying outbound provider callback (native)",
		"requestId", input.RequestID,
		"provider", input.Provider,
		"providerStatus", input.ProviderStatus,
	)

	if _, err := a.callbackHandler.Execute(ctx, portin.OutboundCallbackInput{
		RequestID:       input.RequestID,
		Provider:        input.Provider,
		ProviderStatus:  input.ProviderStatus,
		ProviderEventID: input.ProviderEventID,
		Reason:          input.Reason,
	}); err != nil {
		return fmt.Errorf("apply outbound callback for %s: %w", input.RequestID, err)
	}
	return nil
}

// HandleTimeout reacts to a callback timeout by recording the timeout against the
// request via a synthetic TIMEOUT callback, so the request does not hang forever.
func (a *OutboundExecutionActivities) HandleTimeout(ctx context.Context, input OutboundExecutionActivityInput) error {
	logger := activity.GetLogger(ctx)
	logger.Warn("outbound execution callback timed out (native)", "requestId", input.RequestID)

	if _, err := a.callbackHandler.Execute(ctx, portin.OutboundCallbackInput{
		RequestID:      input.RequestID,
		ProviderStatus: "TIMEOUT",
		Reason:         "callback not received within timeout window",
	}); err != nil {
		return fmt.Errorf("handle outbound timeout for %s: %w", input.RequestID, err)
	}
	return nil
}

// FinalizeExecution performs terminal bookkeeping after the request reaches a
// terminal state (callback applied or timed out).
func (a *OutboundExecutionActivities) FinalizeExecution(ctx context.Context, input OutboundExecutionActivityInput) error {
	activity.GetLogger(ctx).Info("finalizing outbound execution (native)", "requestId", input.RequestID)
	return nil
}
