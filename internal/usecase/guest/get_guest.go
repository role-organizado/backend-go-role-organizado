package guest

import (
	"context"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/guest"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
)

// GuestListHardCap matches the Java GuestController which truncates results at 100.
const GuestListHardCap = 100

// GetGuest implements portin.GetGuestUseCase.
type GetGuest struct {
	guests portout.GuestRepository
}

// NewGetGuest creates the use case.
func NewGetGuest(guests portout.GuestRepository) *GetGuest {
	return &GetGuest{guests: guests}
}

// Execute returns the guest (or NOT_FOUND via repository).
func (uc *GetGuest) Execute(ctx context.Context, id string) (*domain.Guest, error) {
	return uc.guests.FindByID(ctx, id)
}

// GetGuestByTelefone implements portin.GetGuestByTelefoneUseCase.
type GetGuestByTelefone struct {
	guests portout.GuestRepository
}

// NewGetGuestByTelefone creates the use case.
func NewGetGuestByTelefone(guests portout.GuestRepository) *GetGuestByTelefone {
	return &GetGuestByTelefone{guests: guests}
}

// Execute looks up by the input phone after E.164 normalization.
func (uc *GetGuestByTelefone) Execute(ctx context.Context, telefone string) (*domain.Guest, error) {
	return uc.guests.FindByTelefone(ctx, domain.NormalizeTelefone(telefone))
}

// GetGuestByEmail implements portin.GetGuestByEmailUseCase.
type GetGuestByEmail struct {
	guests portout.GuestRepository
}

// NewGetGuestByEmail creates the use case.
func NewGetGuestByEmail(guests portout.GuestRepository) *GetGuestByEmail {
	return &GetGuestByEmail{guests: guests}
}

// Execute looks up by the (trimmed) email — no further normalisation.
func (uc *GetGuestByEmail) Execute(ctx context.Context, email string) (*domain.Guest, error) {
	return uc.guests.FindByEmail(ctx, email)
}

// ListGuests implements portin.ListGuestsUseCase with the 100-row hard cap.
type ListGuests struct {
	guests portout.GuestRepository
}

// NewListGuests creates the use case.
func NewListGuests(guests portout.GuestRepository) *ListGuests {
	return &ListGuests{guests: guests}
}

// Execute lists up to 100 guests (Java parity).
func (uc *ListGuests) Execute(ctx context.Context) ([]domain.Guest, error) {
	return uc.guests.FindAll(ctx, GuestListHardCap)
}

// BatchGetGuests implements portin.BatchGetGuestsUseCase.
type BatchGetGuests struct {
	guests portout.GuestRepository
}

// NewBatchGetGuests creates the use case.
func NewBatchGetGuests(guests portout.GuestRepository) *BatchGetGuests {
	return &BatchGetGuests{guests: guests}
}

// Execute returns guests for the provided IDs; empty/nil input returns an empty slice.
func (uc *BatchGetGuests) Execute(ctx context.Context, ids []string) ([]domain.Guest, error) {
	if len(ids) == 0 {
		return []domain.Guest{}, nil
	}
	return uc.guests.FindAllByIDs(ctx, ids)
}
