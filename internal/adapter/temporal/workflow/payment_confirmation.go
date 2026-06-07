package workflow

import (
	"fmt"
	"strings"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/activity"
)

// PaymentConfirmationInput is the input for PaymentConfirmationWorkflow.
type PaymentConfirmationInput struct {
	// ProviderTransactionID is the PSP-side transaction identifier (e.g. Asaas payment ID).
	ProviderTransactionID string `json:"providerTransactionId"`
	// ProviderName identifies the PSP (e.g. "ASAAS", "MOCK").
	ProviderName string `json:"providerName"`
	// CallbackType classifies the confirmation: APPROVED, FAILED, or CANCELLED.
	CallbackType string `json:"callbackType"`
	// FailureReason is the provider-supplied failure message (for FAILED/CANCELLED).
	FailureReason string `json:"failureReason,omitempty"`
	// ProviderEventID is the unique webhook event ID for idempotency enforcement.
	ProviderEventID string `json:"providerEventId"`
	// EventType is the provider webhook event type string (e.g. "PAYMENT_RECEIVED").
	EventType string `json:"eventType"`
}

// defaultConfirmationRetryPolicy provides retry semantics for confirmation activities.
// Configured to retry up to 5 times with exponential backoff, max 2 minutes.
var defaultConfirmationRetryPolicy = &temporal.RetryPolicy{
	MaximumAttempts:    5,
	InitialInterval:    5 * time.Second,
	BackoffCoefficient: 2.0,
	MaximumInterval:    2 * time.Minute,
}

// PaymentConfirmationWorkflow processes a payment provider callback asynchronously,
// routing to the appropriate confirmation activity based on CallbackType.
//
// Activity idempotency: each confirmation activity delegates to HandlePaymentCallbackUseCase
// which enforces idempotency via (provider, eventID) unique index. Safe to retry.
//
// This workflow is started by the webhook handler (HandlePaymentCallback) and provides
// Temporal's guaranteed retry semantics for downstream effects (installment marking,
// subledger, notifications) that may fail transiently.
func PaymentConfirmationWorkflow(ctx workflow.Context, input PaymentConfirmationInput) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		RetryPolicy:         defaultConfirmationRetryPolicy,
	}
	actCtx := workflow.WithActivityOptions(ctx, ao)

	var a *activity.PaymentActivities
	actInput := activity.PaymentConfirmationActivityInput{
		ProviderTransactionID: input.ProviderTransactionID,
		ProviderEventID:       input.ProviderEventID,
		ProviderName:          input.ProviderName,
		CallbackType:          input.CallbackType,
		FailureReason:         input.FailureReason,
		EventType:             input.EventType,
	}

	switch strings.ToUpper(input.CallbackType) {
	case "APPROVED":
		if err := workflow.ExecuteActivity(actCtx, a.ConfirmPaymentApproved, actInput).Get(ctx, nil); err != nil {
			return fmt.Errorf("payment confirmation (APPROVED): %w", err)
		}
	case "FAILED":
		if err := workflow.ExecuteActivity(actCtx, a.ConfirmPaymentFailed, actInput).Get(ctx, nil); err != nil {
			return fmt.Errorf("payment confirmation (FAILED): %w", err)
		}
	case "CANCELLED":
		if err := workflow.ExecuteActivity(actCtx, a.ConfirmPaymentCancelled, actInput).Get(ctx, nil); err != nil {
			return fmt.Errorf("payment confirmation (CANCELLED): %w", err)
		}
	default:
		// Unknown callback type — non-retryable; log and exit cleanly.
		return temporal.NewNonRetryableApplicationError(
			fmt.Sprintf("unknown callback type: %s", input.CallbackType),
			"UnknownCallbackType",
			nil,
		)
	}

	return nil
}
