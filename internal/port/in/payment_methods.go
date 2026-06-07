package in

import (
	"context"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
)

// ManageSavedCardsUseCase covers listing, setting default, and removing saved credit cards.
// Create/tokenisation is handled separately via the payment processing flow.
type ManageSavedCardsUseCase interface {
	// List returns all active saved cards for the user, default first.
	List(ctx context.Context, userID string) ([]*domain.SavedCreditCard, error)
	// SetDefault atomically marks the card as default and clears all others.
	SetDefault(ctx context.Context, cardID, userID string) error
	// Delete soft-deletes the card (sets is_active = false).
	Delete(ctx context.Context, cardID, userID string) error
}
