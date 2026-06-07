package payment

import (
	"context"
	"fmt"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
)

// manageSavedCardsUseCase implements portin.ManageSavedCardsUseCase.
type manageSavedCardsUseCase struct {
	repo portout.SavedCreditCardRepository
}

// NewManageSavedCards creates a new ManageSavedCardsUseCase.
func NewManageSavedCards(repo portout.SavedCreditCardRepository) portin.ManageSavedCardsUseCase {
	return &manageSavedCardsUseCase{repo: repo}
}

// List returns all active saved credit cards for the user, default first.
func (uc *manageSavedCardsUseCase) List(ctx context.Context, userID string) ([]*domain.SavedCreditCard, error) {
	cards, err := uc.repo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("listar cartões salvos: %w", err)
	}
	return cards, nil
}

// SetDefault atomically marks the card as default and clears all others.
// The userID is used to scope the "clear all" step to the correct user.
func (uc *manageSavedCardsUseCase) SetDefault(ctx context.Context, cardID, userID string) error {
	if err := uc.repo.SetDefault(ctx, userID, cardID); err != nil {
		return fmt.Errorf("definir cartão padrão: %w", err)
	}
	return nil
}

// Delete soft-deletes a saved credit card (sets is_active = false).
func (uc *manageSavedCardsUseCase) Delete(ctx context.Context, cardID, userID string) error {
	if err := uc.repo.DeleteByID(ctx, cardID); err != nil {
		return fmt.Errorf("remover cartão salvo: %w", err)
	}
	return nil
}
