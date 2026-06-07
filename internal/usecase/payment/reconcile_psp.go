package payment

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
)

// ReconcilePspTransactions implements portin.ReconcilePspTransactionsUseCase.
//
// It scans PENDING/PROCESSING transactions whose createdAt is older than
// filter.From (i.e. they were created before the threshold) and queries the PSP
// for their current status. If the PSP reports a terminal status that differs
// from the local record, the local transaction is updated accordingly.
//
// The reconcile loop is intentionally non-transactional: each transaction is
// updated individually. Failures are counted and logged but do not abort the
// remaining items, matching the Java ReconcilePspTransactionsUseCase behaviour.
type ReconcilePspTransactions struct {
	txRepo   portout.PaymentTransactionRepository
	provider portout.PaymentProvider
}

// NewReconcilePspTransactions creates a new ReconcilePspTransactions use case.
func NewReconcilePspTransactions(
	txRepo portout.PaymentTransactionRepository,
	provider portout.PaymentProvider,
) *ReconcilePspTransactions {
	return &ReconcilePspTransactions{txRepo: txRepo, provider: provider}
}

// Execute reconciles PENDING/PROCESSING transactions created before filter.From.
// A zero filter.From defaults to 24 hours ago.
func (uc *ReconcilePspTransactions) Execute(ctx context.Context, filter portin.ReconcileFilter) (*portin.ReconcileResult, error) {
	threshold := filter.From
	if threshold.IsZero() {
		threshold = time.Now().Add(-24 * time.Hour)
	}

	txs, err := uc.txRepo.FindPendingOlderThan(ctx, threshold)
	if err != nil {
		return nil, fmt.Errorf("reconcile psp: find pending older than %s: %w", threshold.Format(time.RFC3339), err)
	}

	result := &portin.ReconcileResult{Checked: int64(len(txs))}

	for _, tx := range txs {
		if tx.ProviderTransactionID == "" {
			// Transactions without a provider ID were never submitted — skip.
			slog.WarnContext(ctx, "reconcile psp: transaction has no providerTransactionID, skipping",
				"transactionID", tx.ID)
			continue
		}

		providerPayment, getErr := uc.provider.GetPayment(ctx, tx.ProviderTransactionID)
		if getErr != nil {
			slog.WarnContext(ctx, "reconcile psp: GetPayment failed",
				"transactionID", tx.ID,
				"providerTransactionID", tx.ProviderTransactionID,
				"error", getErr)
			result.Failed++
			continue
		}

		if !providerPayment.Status.IsTerminal() {
			// Not yet terminal on the provider side — nothing to correct.
			continue
		}

		now := time.Now()
		prevStatus := tx.Status

		switch providerPayment.Status {
		case portout.ProviderStatusReceived, portout.ProviderStatusConfirmed:
			tx.MarkCompleted(now)
		case portout.ProviderStatusRefunded, portout.ProviderStatusCanceled:
			tx.Status = domain.TransactionStatusCancelled
			tx.FailureReason = string(providerPayment.Status)
			tx.UpdatedAt = now
		case portout.ProviderStatusOverdue:
			tx.Status = domain.TransactionStatusFailed
			tx.FailureReason = "OVERDUE"
			tx.UpdatedAt = now
		default:
			// Other terminal statuses (chargebacks, dunning, etc.)
			tx.Status = domain.TransactionStatusFailed
			tx.FailureReason = string(providerPayment.Status)
			tx.UpdatedAt = now
		}

		if updateErr := uc.txRepo.Update(ctx, tx); updateErr != nil {
			slog.ErrorContext(ctx, "reconcile psp: update transaction failed",
				"transactionID", tx.ID, "error", updateErr)
			result.Failed++
			continue
		}

		result.Updated++
		slog.InfoContext(ctx, "reconcile psp: corrected divergent transaction",
			"transactionID", tx.ID,
			"prevStatus", prevStatus,
			"newStatus", tx.Status,
			"providerStatus", providerPayment.Status,
		)
	}

	return result, nil
}

// compile-time assertion: *ReconcilePspTransactions implements portin.ReconcilePspTransactionsUseCase.
var _ portin.ReconcilePspTransactionsUseCase = (*ReconcilePspTransactions)(nil)
