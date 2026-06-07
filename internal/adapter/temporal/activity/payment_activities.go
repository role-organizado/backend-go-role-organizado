// Package activity provides Temporal activity implementations for the payment domain.
// Activities are the I/O boundary of Temporal workflows: they wrap repository calls,
// HTTP client invocations, and use-case orchestration.
//
// Architecture note: activities MUST NOT contain business logic. They delegate to
// port/in use cases, keeping the domain layer independent of Temporal.
package activity

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
)

// PaymentActivities groups all payment-related Temporal activities.
// Register the struct with the worker; Temporal dispatches individual method calls.
type PaymentActivities struct {
	txRepo          portout.PaymentTransactionRepository
	expireUC        portin.ExpireTransactionUseCase
	handleCallback  portin.HandlePaymentCallbackUseCase
	reconcileUC     portin.ReconcilePspTransactionsUseCase
	reportRepo      ReconciliationReportRepository
}

// ReconciliationReportRepository persists reconciliation run results.
// Kept in this package to avoid circular imports; the MongoDB adapter implements it.
type ReconciliationReportRepository interface {
	Save(ctx context.Context, report *ReconciliationReport) error
}

// ReconciliationReport captures metrics from a single reconciliation run.
type ReconciliationReport struct {
	ID            string
	ReferenceDate string
	RunAt         time.Time
	CheckedCount  int64
	UpdatedCount  int64
	FailedCount   int64
	Updates       []ReconciliationUpdate
	Errors        []string
}

// ReconciliationUpdate describes a single transaction status correction.
type ReconciliationUpdate struct {
	TransactionID  string
	PreviousStatus string
	NewStatus      string
	ProviderStatus string
	UpdatedAt      time.Time
}

// NewPaymentActivities creates a PaymentActivities instance with all required dependencies.
func NewPaymentActivities(
	txRepo portout.PaymentTransactionRepository,
	expireUC portin.ExpireTransactionUseCase,
	handleCallback portin.HandlePaymentCallbackUseCase,
	reconcileUC portin.ReconcilePspTransactionsUseCase,
	reportRepo ReconciliationReportRepository,
) *PaymentActivities {
	return &PaymentActivities{
		txRepo:         txRepo,
		expireUC:       expireUC,
		handleCallback: handleCallback,
		reconcileUC:    reconcileUC,
		reportRepo:     reportRepo,
	}
}

// ─── Expiration Activity ───────────────────────────────────────────────────────

// ExpireTransaction marks a payment transaction as CANCELLED due to timeout.
// Called by PaymentExpirationWorkflow after the expiry timer fires.
//
// Non-retryable: if the transaction is already in a terminal state (COMPLETED,
// FAILED, CANCELLED) we return a non-retryable error so the workflow knows to
// transition to SKIPPED rather than retrying indefinitely.
func (a *PaymentActivities) ExpireTransaction(ctx context.Context, transactionID string) error {
	logger := activity.GetLogger(ctx)
	logger.Info("expiring transaction", "transactionID", transactionID)

	if err := a.expireUC.Execute(ctx, transactionID); err != nil {
		// Wrap as non-retryable if the transaction is already terminal.
		if isAlreadyTerminal(err) {
			return temporal.NewNonRetryableApplicationError(
				"transaction already terminal",
				"TransactionAlreadyTerminal",
				err,
			)
		}
		return fmt.Errorf("expire transaction %s: %w", transactionID, err)
	}

	slog.InfoContext(ctx, "temporal: expire transaction activity completed", "transactionID", transactionID)
	return nil
}

// ─── Confirmation Activities ──────────────────────────────────────────────────

// ConfirmPaymentApproved processes an APPROVED payment confirmation via the
// HandlePaymentCallback use case. Due to idempotency enforcement in the use case,
// calling this multiple times with the same ProviderEventID is safe.
func (a *PaymentActivities) ConfirmPaymentApproved(ctx context.Context, input PaymentConfirmationActivityInput) error {
	return a.runCallback(ctx, input, "RECEIVED")
}

// ConfirmPaymentFailed processes a FAILED/REJECTED payment confirmation.
func (a *PaymentActivities) ConfirmPaymentFailed(ctx context.Context, input PaymentConfirmationActivityInput) error {
	return a.runCallback(ctx, input, "FAILED")
}

// ConfirmPaymentCancelled processes a CANCELLED/REFUNDED payment confirmation.
func (a *PaymentActivities) ConfirmPaymentCancelled(ctx context.Context, input PaymentConfirmationActivityInput) error {
	return a.runCallback(ctx, input, "CANCELLED")
}

// PaymentConfirmationActivityInput carries data for confirmation activities.
type PaymentConfirmationActivityInput struct {
	ProviderTransactionID string
	ProviderEventID       string
	ProviderName          string
	CallbackType          string // APPROVED|FAILED|CANCELLED
	FailureReason         string
	EventType             string
}

// runCallback delegates to HandlePaymentCallbackUseCase with the normalised status.
func (a *PaymentActivities) runCallback(ctx context.Context, input PaymentConfirmationActivityInput, status string) error {
	if input.FailureReason != "" && status != "FAILED" {
		status = input.FailureReason
	}
	payload := portin.PaymentCallbackPayload{
		Provider:              domain.PaymentProvider(input.ProviderName),
		ProviderEventID:       input.ProviderEventID,
		ProviderTransactionID: input.ProviderTransactionID,
		EventType:             input.EventType,
		NewStatus:             status,
	}
	if err := a.handleCallback.Execute(ctx, payload); err != nil {
		return fmt.Errorf("payment confirmation activity (%s): %w", input.CallbackType, err)
	}
	return nil
}

// ─── Reconciliation Activity ───────────────────────────────────────────────────

// ReconcilePspTransactions reconciles local payment transactions against the PSP
// for a given time window, correcting any status divergences.
// Called by the daily ReconciliationWorkflow schedule.
func (a *PaymentActivities) ReconcilePspTransactions(ctx context.Context, input ReconciliationActivityInput) error {
	logger := activity.GetLogger(ctx)
	logger.Info("starting PSP reconciliation", "referenceDate", input.ReferenceDate, "from", input.From, "to", input.To)

	result, err := a.reconcileUC.Execute(ctx, portin.ReconcileFilter{
		From: input.From,
		To:   input.To,
	})
	if err != nil {
		return fmt.Errorf("reconcile PSP transactions: %w", err)
	}

	slog.InfoContext(ctx, "temporal: reconciliation activity completed",
		"referenceDate", input.ReferenceDate,
		"checked", result.Checked,
		"updated", result.Updated,
		"failed", result.Failed,
	)
	return nil
}

// ReconciliationActivityInput carries parameters for the reconciliation activity.
type ReconciliationActivityInput struct {
	ReferenceDate string
	From          time.Time
	To            time.Time
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// isAlreadyTerminal returns true if the error signals that the transaction
// is already in a terminal state and should not be retried.
func isAlreadyTerminal(err error) bool {
	if err == nil {
		return false
	}
	return err.Error() == "transaction already terminal"
}
