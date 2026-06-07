package in

import (
	"context"

	"github.com/role-organizado/backend-go-role-organizado/internal/domain/social"
)

// --- Get ---

// GetSocialFeaturesInput holds the parameters to retrieve social features for an event.
type GetSocialFeaturesInput struct {
	EventoID  string
	UsuarioID string
}

// GetSocialFeaturesUseCase retrieves the social features document for an event.
// Returns a zero-value structure (no lazy-create) when no document exists yet.
type GetSocialFeaturesUseCase interface {
	Execute(ctx context.Context, in GetSocialFeaturesInput) (*social.EventoSocialFeatures, error)
}

// --- DressCode ---

// SetDressCodeInput holds the parameters to set or update the dress code of an event.
type SetDressCodeInput struct {
	EventoID          string
	UsuarioID         string
	Tipo              string
	DescricaoTematico string
}

// SetDressCodeUseCase sets or updates the dress code for an event (organizador only).
// Lazy-creates the social features document if needed.
type SetDressCodeUseCase interface {
	Execute(ctx context.Context, in SetDressCodeInput) (*social.EventoSocialFeatures, error)
}

// RemoveDressCodeInput holds the parameters to remove the dress code from an event.
type RemoveDressCodeInput struct {
	EventoID  string
	UsuarioID string
}

// RemoveDressCodeUseCase removes the dress code from an event (organizador only).
// Idempotent: no error when dress code is already absent.
type RemoveDressCodeUseCase interface {
	Execute(ctx context.Context, in RemoveDressCodeInput) error
}

// --- Playlists ---

// AddPlaylistInput holds the parameters to add a playlist link to an event.
type AddPlaylistInput struct {
	EventoID  string
	UsuarioID string
	URL       string
	Nome      string
}

// AddPlaylistUseCase adds a playlist link to an event (organizador only, max 3).
// Provider and embedURL are auto-detected from the URL.
type AddPlaylistUseCase interface {
	Execute(ctx context.Context, in AddPlaylistInput) (*social.PlaylistLink, error)
}

// RemovePlaylistInput holds the parameters to remove a playlist link from an event.
type RemovePlaylistInput struct {
	EventoID   string
	PlaylistID string
	UsuarioID  string
}

// RemovePlaylistUseCase removes a playlist link from an event (organizador only).
// Idempotent: no error when the playlist does not exist.
type RemovePlaylistUseCase interface {
	Execute(ctx context.Context, in RemovePlaylistInput) error
}

// --- BringList ---

// AddBringListItemInput holds the parameters to add an item to the bring list.
type AddBringListItemInput struct {
	EventoID  string
	UsuarioID string
	Nome      string
	Quantidade string
}

// AddBringListItemUseCase adds an item to the event bring list (organizador only).
type AddBringListItemUseCase interface {
	Execute(ctx context.Context, in AddBringListItemInput) (*social.BringListItem, error)
}

// UpdateBringListItemInput holds the parameters to update a bring list item.
type UpdateBringListItemInput struct {
	EventoID  string
	ItemID    string
	UsuarioID string
	Nome      string
	Quantidade string
}

// UpdateBringListItemUseCase updates nome and/or quantidade of a bring list item (organizador only).
type UpdateBringListItemUseCase interface {
	Execute(ctx context.Context, in UpdateBringListItemInput) error
}

// RemoveBringListItemInput holds the parameters to remove an item from the bring list.
type RemoveBringListItemInput struct {
	EventoID  string
	ItemID    string
	UsuarioID string
}

// RemoveBringListItemUseCase removes an item from the event bring list (organizador only).
// Idempotent: no error when the item does not exist.
type RemoveBringListItemUseCase interface {
	Execute(ctx context.Context, in RemoveBringListItemInput) error
}

// ClaimBringListItemInput holds the parameters to claim a bring list item.
type ClaimBringListItemInput struct {
	EventoID    string
	ItemID      string
	UsuarioID   string
	UsuarioNome string
}

// ClaimBringListItemUseCase atomically claims a bring list item for a participant.
// Returns 409 BRING_LIST_ITEM_ALREADY_CLAIMED if the item is already claimed.
type ClaimBringListItemUseCase interface {
	Execute(ctx context.Context, in ClaimBringListItemInput) error
}

// UnclaimBringListItemInput holds the parameters to release a bring list item claim.
type UnclaimBringListItemInput struct {
	EventoID  string
	ItemID    string
	UsuarioID string
}

// UnclaimBringListItemUseCase releases the claim on a bring list item.
// Idempotent: clears claimedBy, claimedByNome, and claimedAt.
type UnclaimBringListItemUseCase interface {
	Execute(ctx context.Context, in UnclaimBringListItemInput) error
}

// --- Checkin ---

// SetCheckinHabilitadoInput holds the parameters to enable or disable check-in.
type SetCheckinHabilitadoInput struct {
	EventoID  string
	UsuarioID string
	Habilitado bool
}

// SetCheckinHabilitadoUseCase enables or disables check-in for an event (organizador only).
type SetCheckinHabilitadoUseCase interface {
	Execute(ctx context.Context, in SetCheckinHabilitadoInput) error
}

// DoCheckinInput holds the parameters for a participant to check in to an event.
type DoCheckinInput struct {
	EventoID    string
	UsuarioID   string
	UsuarioNome string
}

// DoCheckinUseCase registers a participant's check-in.
// Returns 409 CHECKIN_NOT_ENABLED if checkinHabilitado is false.
// Returns 409 CHECKIN_ALREADY_REGISTERED if the user already checked in.
type DoCheckinUseCase interface {
	Execute(ctx context.Context, in DoCheckinInput) (*social.Checkin, error)
}

// --- AlbumLinks ---

// AddAlbumLinkInput holds the parameters to add a photo album link to an event.
type AddAlbumLinkInput struct {
	EventoID  string
	UsuarioID string
	URL       string
	Nome      string
}

// AddAlbumLinkUseCase adds a photo album link to an event (participante confirmado ou organizador, max 5).
// Provider is auto-detected from the URL; adicionadoPor is set to UsuarioID.
type AddAlbumLinkUseCase interface {
	Execute(ctx context.Context, in AddAlbumLinkInput) (*social.AlbumLink, error)
}

// RemoveAlbumLinkInput holds the parameters to remove a photo album link from an event.
type RemoveAlbumLinkInput struct {
	EventoID  string
	LinkID    string
	UsuarioID string
}

// RemoveAlbumLinkUseCase removes a photo album link from an event (organizador only).
// Idempotent: no error when the link does not exist.
type RemoveAlbumLinkUseCase interface {
	Execute(ctx context.Context, in RemoveAlbumLinkInput) error
}
