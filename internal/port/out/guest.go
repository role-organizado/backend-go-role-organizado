package out

import (
	"context"

	convitedomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/convite"
	"github.com/role-organizado/backend-go-role-organizado/internal/domain/guest"
)

// VinculacaoParticipantPort exposes the narrow participant operations needed to
// migrate participations from GUEST to USER when a guest evolves into a user.
// Optional — a nil implementation disables participant migration (counts stay 0).
type VinculacaoParticipantPort interface {
	// FindByID locates a participant by its UUID id (explicit invite-link mode).
	FindByID(ctx context.Context, id string) (*convitedomain.Participant, error)
	// FindByTipoParticipanteAndUsuarioID returns participants whose tipo + usuarioId
	// match — used to find GUEST participations carrying the guestId as usuarioId.
	FindByTipoParticipanteAndUsuarioID(ctx context.Context, tipo convitedomain.TipoParticipante, usuarioID string) ([]convitedomain.Participant, error)
	// Save persists participant mutations (tipo, usuarioId, timestamps).
	Save(ctx context.Context, p *convitedomain.Participant) (*convitedomain.Participant, error)
}

// VinculacaoDraftPort exposes the narrow draft operations needed to rewrite
// guestId references to the new userId across event drafts. Optional (nil-safe).
type VinculacaoDraftPort interface {
	// FindDraftIDsByConvidadosGuestID returns the ids of drafts referencing guestID.
	FindDraftIDsByConvidadosGuestID(ctx context.Context, guestID string) ([]string, error)
	// ConvertGuestToUserInConvidados rewrites guestID → userID in the draft's guest list.
	ConvertGuestToUserInConvidados(ctx context.Context, draftID, guestID, userID string) error
}

// GuestRepository is the output port for Guest persistence (collection: guests).
type GuestRepository interface {
	Save(ctx context.Context, g *guest.Guest) (*guest.Guest, error)
	Update(ctx context.Context, g *guest.Guest) (*guest.Guest, error)
	FindByID(ctx context.Context, id string) (*guest.Guest, error)
	FindByTelefone(ctx context.Context, telefone string) (*guest.Guest, error)
	FindByEmail(ctx context.Context, email string) (*guest.Guest, error)
	FindByTelefoneOrEmail(ctx context.Context, telefone, email string) (*guest.Guest, error)
	FindAll(ctx context.Context, limit int) ([]guest.Guest, error)
	FindAllByIDs(ctx context.Context, ids []string) ([]guest.Guest, error)
	ExistsByTelefone(ctx context.Context, telefone string) (bool, error)
	ExistsByEmail(ctx context.Context, email string) (bool, error)
}

// BiometricCredentialRepository persists FIDO2-like credentials.
// Collection: biometric_credentials.
type BiometricCredentialRepository interface {
	Save(ctx context.Context, c *guest.BiometricCredential) (*guest.BiometricCredential, error)
	Update(ctx context.Context, c *guest.BiometricCredential) (*guest.BiometricCredential, error)
	FindByID(ctx context.Context, id string) (*guest.BiometricCredential, error)
	FindByDeviceID(ctx context.Context, deviceID string) (*guest.BiometricCredential, error)
	FindByUsuarioIDAndDeviceID(ctx context.Context, usuarioID, deviceID string) (*guest.BiometricCredential, error)
	FindByUsuarioIDAndDeviceIDActive(ctx context.Context, usuarioID, deviceID string) (*guest.BiometricCredential, error)
	FindByUsuarioIDActive(ctx context.Context, usuarioID string) ([]guest.BiometricCredential, error)
	FindByUsuarioID(ctx context.Context, usuarioID string) ([]guest.BiometricCredential, error)
	ExistsByDeviceIDActive(ctx context.Context, deviceID string) (bool, error)
	DeleteByID(ctx context.Context, id string) error
	DeleteByUsuarioID(ctx context.Context, usuarioID string) error
	CountByUsuarioIDActive(ctx context.Context, usuarioID string) (int64, error)
}

// BiometricChallengeRepository persists single-use challenges with a TTL.
// Collection: biometric_challenges.
type BiometricChallengeRepository interface {
	Save(ctx context.Context, c *guest.BiometricChallenge) (*guest.BiometricChallenge, error)
	Update(ctx context.Context, c *guest.BiometricChallenge) (*guest.BiometricChallenge, error)
	FindByID(ctx context.Context, id string) (*guest.BiometricChallenge, error)
	FindLatestByDeviceIDUnused(ctx context.Context, deviceID string) (*guest.BiometricChallenge, error)
	DeleteByDeviceID(ctx context.Context, deviceID string) error
	DeleteByID(ctx context.Context, id string) error
}
