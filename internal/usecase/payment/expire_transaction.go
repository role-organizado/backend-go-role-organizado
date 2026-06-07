package payment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// errTransactionAlreadyTerminal is returned when the transaction is already in a
// final state and cannot be expired. The Temporal activity wraps this as a
// non-retryable error (TransactionAlreadyTerminal type).
var errTransactionAlreadyTerminal = errors.New("transaction already terminal")

// expireTransactionUseCase marks a PENDING/PROCESSING transaction as CANCELLED
// with reason "TIMEOUT". Called exclusively by the PaymentExpirationWorkflow activity.
type expireTransactionUseCase struct {
	txRepo portout.PaymentTransactionRepository
}

// NewExpireTransaction creates a new ExpireTransactionUseCase.
func NewExpireTransaction(txRepo portout.PaymentTransactionRepository) portin.ExpireTransactionUseCase {
	return &expireTransactionUseCase{txRepo: txRepo}
}

// Execute marks the transaction identified by transactionID as CANCELLED (TIMEOUT).
// Returns errTransactionAlreadyTerminal if the transaction is already in a terminal
// state — the caller (Temporal activity) converts this to a non-retryable error.
func (uc *expireTransactionUseCase) Execute(ctx context.Context, transactionID string) error {
	tx, err := uc.txRepo.FindByID(ctx, transactionID)
	if err != nil {
		if apierr.IsNotFound(err) {
			// Transaction not found — treat as already resolved (idempotent).
			slog.WarnContext(ctx, "expire transaction: not found, treating as no-op",
				"transactionID", transactionID)
			return nil
		}
		return fmt.Errorf("expire transaction: find by id: %w", err)
	}

	// If already terminal, signal the Temporal activity to skip retry.
	if tx.IsTerminal() {
		slog.InfoContext(ctx, "expire transaction: already terminal, skipping",
			"transactionID", transactionID,
			"status", tx.Status,
		)
		return errTransactionAlreadyTerminal
	}

	// Expire: mark CANCELLED with reason TIMEOUT.
	tx.Status = domain.TransactionStatusCancelled
	tx.FailureReason = "TIMEOUT"
	tx.UpdatedAt = time.Now()

	if updateErr := uc.txRepo.Update(ctx, tx); updateErr != nil {
		return fmt.Errorf("expire transaction: update: %w", updateErr)
	}

	slog.InfoContext(ctx, "expire transaction: marked as CANCELLED/TIMEOUT",
		"transactionID", transactionID,
	)
	return nil
}

// compile-time assertion.
var _ portin.ExpireTransactionUseCase = (*expireTransactionUseCase)(nil)
