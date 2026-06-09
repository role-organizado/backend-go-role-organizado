package out

import (
	"context"
	"time"

	"github.com/role-organizado/backend-go-role-organizado/internal/domain/social"
)

// SocialFeaturesRepository defines persistence operations for EventoSocialFeatures.
// All mutating methods lazily create the document when it does not yet exist,
// except FindByEventoID which returns nil without error.
type SocialFeaturesRepository interface {
	// FindByEventoID returns the social features document for the given event,
	// or nil (no error) when the document does not exist.
	FindByEventoID(ctx context.Context, eventoID string) (*social.EventoSocialFeatures, error)

	// FindOrCreate returns the existing document or creates an empty one.
	FindOrCreate(ctx context.Context, eventoID string) (*social.EventoSocialFeatures, error)

	// SetDressCode sets or replaces the dress code (upsert).
	SetDressCode(ctx context.Context, eventoID string, dc *social.DressCode) error

	// AddPlaylist appends a playlist link to the event's playlist list.
	AddPlaylist(ctx context.Context, eventoID string, p social.PlaylistLink) error

	// RemovePlaylist removes a playlist link by ID ($pull, idempotent).
	RemovePlaylist(ctx context.Context, eventoID, playlistID string) error

	// AddBringListItem appends an item to the event's bring list.
	AddBringListItem(ctx context.Context, eventoID string, item social.BringListItem) error

	// UpdateBringListItem updates nome and quantidade of a bring list item via positional operator.
	UpdateBringListItem(ctx context.Context, eventoID, itemID, nome, quantidade string) error

	// RemoveBringListItem removes a bring list item by ID ($pull, idempotent).
	RemoveBringListItem(ctx context.Context, eventoID, itemID string) error

	// ClaimBringListItem atomically sets claimedBy/claimedByNome/claimedAt on an unclaimed item.
	// Returns an error (business conflict) when the item is already claimed.
	ClaimBringListItem(ctx context.Context, eventoID, itemID, usuarioID, usuarioNome string, claimedAt time.Time) error

	// UnclaimBringListItem clears the claim fields on a bring list item (idempotent).
	UnclaimBringListItem(ctx context.Context, eventoID, itemID string) error

	// SetCheckinHabilitado sets the checkinHabilitado flag (upsert).
	SetCheckinHabilitado(ctx context.Context, eventoID string, habilitado bool) error

	// AddCheckin appends a check-in record (atomic; caller must validate preconditions).
	AddCheckin(ctx context.Context, eventoID string, c social.Checkin) error

	// AddAlbumLink appends a photo album link to the event's album list.
	AddAlbumLink(ctx context.Context, eventoID string, link social.AlbumLink) error

	// RemoveAlbumLink removes a photo album link by ID ($pull, idempotent).
	RemoveAlbumLink(ctx context.Context, eventoID, linkID string) error
}

// EventoAuthPort provides authorization helpers required by the social use cases.
// Implementations query the evento and participante collections to enforce access rules.
type EventoAuthPort interface {
	// FaseAtLeast validates that the event's current phase is at least the given minimum.
	// Returns a domain/validation error when the phase requirement is not met.
	FaseAtLeast(ctx context.Context, eventoID string, fase social.EventoFase) error

	// IsOrganizador returns true when the given user is the organizer of the event.
	IsOrganizador(ctx context.Context, eventoID, usuarioID string) (bool, error)

	// IsParticipanteConfirmadoOuOrganizador returns true when the given user is either
	// a confirmed participant (ACEITO status) or the organizer of the event.
	IsParticipanteConfirmadoOuOrganizador(ctx context.Context, eventoID, usuarioID string) (bool, error)
}
