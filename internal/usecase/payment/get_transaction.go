package payment

import (
	"context"
	"fmt"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// GetPaymentTransaction implements portin.GetPaymentTransactionUseCase.
// Returns the transaction by ID, enforcing that the requester is the owner (403 otherwise).
type GetPaymentTransaction struct {
	txRepo portout.PaymentTransactionRepository
}

// NewGetPaymentTransaction creates a new GetPaymentTransaction use case.
func NewGetPaymentTransaction(txRepo portout.PaymentTransactionRepository) *GetPaymentTransaction {
	return &GetPaymentTransaction{txRepo: txRepo}
}

// Execute retrieves the transaction by ID and enforces ownership.
func (uc *GetPaymentTransaction) Execute(ctx context.Context, transactionID, requesterID string) (*domain.PaymentTransaction, error) {
	tx, err := uc.txRepo.FindByID(ctx, transactionID)
	if err != nil {
		return nil, fmt.Errorf("get payment transaction: %w", err)
	}
	if !tx.IsOwner(requesterID) {
		return nil, apierr.Forbidden("acesso negado: transação pertence a outro usuário")
	}
	return tx, nil
}

// compile-time assertion: *GetPaymentTransaction implements portin.GetPaymentTransactionUseCase.
var _ portin.GetPaymentTransactionUseCase = (*GetPaymentTransaction)(nil)
