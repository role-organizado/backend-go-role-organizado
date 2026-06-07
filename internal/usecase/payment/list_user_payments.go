package payment

import (
	"context"
	"fmt"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ListUserPayments implements portin.ListUserPaymentsUseCase.
// When requesterID != targetUserID, a 403 is returned (Java-compat ownership rule).
type ListUserPayments struct {
	txRepo portout.PaymentTransactionRepository
}

// NewListUserPayments creates a new ListUserPayments use case.
func NewListUserPayments(txRepo portout.PaymentTransactionRepository) *ListUserPayments {
	return &ListUserPayments{txRepo: txRepo}
}

// Execute returns transactions for userID, with optional filters.
// requesterID is used for ownership validation:
//   - If requesterID == userID → owner listing, always allowed.
//   - If requesterID != userID → 403 Forbidden (Java-compat admin check not yet wired).
func (uc *ListUserPayments) Execute(
	ctx context.Context,
	userID string,
	filter portin.ListUserPaymentsFilter,
) ([]*domain.PaymentTransaction, error) {
	repoFilter := portout.TransactionFilter{
		EventoID: filter.EventoID,
		From:     filter.From,
		To:       filter.To,
		Status:   filter.Status,
	}

	txs, _, err := uc.txRepo.FindByUserID(ctx, userID, repoFilter)
	if err != nil {
		return nil, fmt.Errorf("list user payments: %w", err)
	}
	return txs, nil
}

// ListUserPaymentsForRequester is a convenience method that validates ownership
// before listing: if requesterID != targetUserID the call returns 403.
// This mirrors the Java endpoint GET /api/v1/payments/user/{userId}.
func (uc *ListUserPayments) ListForRequester(
	ctx context.Context,
	targetUserID, requesterID string,
	filter portin.ListUserPaymentsFilter,
) ([]*domain.PaymentTransaction, error) {
	if requesterID != targetUserID {
		return nil, apierr.Forbidden("acesso negado: você só pode visualizar seus próprios pagamentos")
	}
	return uc.Execute(ctx, targetUserID, filter)
}

// compile-time assertion: *ListUserPayments implements portin.ListUserPaymentsUseCase.
var _ portin.ListUserPaymentsUseCase = (*ListUserPayments)(nil)
