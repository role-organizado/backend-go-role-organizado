package in

import (
	"context"

	"github.com/role-organizado/backend-go-role-organizado/internal/domain/guest"
)

// ---- Guest use cases ----

// CreateOrFindGuestInput is the payload for create-or-find by phone/email.
type CreateOrFindGuestInput struct {
	Nome     string
	Telefone string
	Email    string
}

// CreateOrFindGuestUseCase deduplicates by phone-or-email then either updates the
// name (if changed) or creates a new Guest with a fresh UUID.
type CreateOrFindGuestUseCase interface {
	Execute(ctx context.Context, in CreateOrFindGuestInput) (*guest.Guest, error)
}

// GetGuestUseCase fetches a guest by ID (404 if missing).
type GetGuestUseCase interface {
	Execute(ctx context.Context, id string) (*guest.Guest, error)
}

// GetGuestByTelefoneUseCase fetches a guest by E.164 phone (404 if missing).
type GetGuestByTelefoneUseCase interface {
	Execute(ctx context.Context, telefone string) (*guest.Guest, error)
}

// GetGuestByEmailUseCase fetches a guest by email (404 if missing).
type GetGuestByEmailUseCase interface {
	Execute(ctx context.Context, email string) (*guest.Guest, error)
}

// ListGuestsUseCase returns up to the first 100 guests (Java parity hard cap).
type ListGuestsUseCase interface {
	Execute(ctx context.Context) ([]guest.Guest, error)
}

// BatchGetGuestsUseCase returns guests for a slice of IDs (empty/nil → empty slice).
type BatchGetGuestsUseCase interface {
	Execute(ctx context.Context, ids []string) ([]guest.Guest, error)
}

// ---- Biometric use cases ----

// GenerateChallengeInput is the payload for /biometric/challenge.
type GenerateChallengeInput struct {
	UsuarioID string
	DeviceID  string
}

// BiometricChallengeOutput is the response of /biometric/challenge.
type BiometricChallengeOutput struct {
	Challenge        string // Base64-encoded 32-byte nonce
	ExpiresInSeconds int
}

// GenerateBiometricChallengeUseCase issues a 32-byte challenge and stores it with a 5-minute TTL.
type GenerateBiometricChallengeUseCase interface {
	Execute(ctx context.Context, in GenerateChallengeInput) (*BiometricChallengeOutput, error)
}

// BiometricAuthenticateInput is the payload for /biometric/authenticate.
type BiometricAuthenticateInput struct {
	UsuarioID string
	DeviceID  string
	Signature string // Base64-encoded SHA256withECDSA signature over the challenge bytes
}

// BiometricAuthenticateUseCase verifies the signature against the active credential's
// public key and issues a fresh AuthOutput (access + refresh + usuario).
type BiometricAuthenticateUseCase interface {
	Execute(ctx context.Context, in BiometricAuthenticateInput, clientIP string) (*AuthOutput, error)
}

// RegisterCredentialInput is the payload for /biometric/register (JWT-authenticated).
type RegisterCredentialInput struct {
	DeviceID       string
	PublicKey      string // Base64-encoded X.509 SubjectPublicKeyInfo (EC P-256)
	DeviceName     string
	DeviceModel    string
	AndroidVersion string
}

// BiometricRegisterOutput is the response of /biometric/register.
type BiometricRegisterOutput struct {
	Success      bool
	Message      string
	DeviceID     string
	DeviceName   string
	RegisteredAt string // ISO-8601 timestamp
}

// RegisterBiometricCredentialUseCase saves a new credential for an already-logged-in user.
// If an active credential already exists for the (UsuarioID, DeviceID) pair it is
// soft-revoked first ("Substituída por novo registro").
type RegisterBiometricCredentialUseCase interface {
	Execute(ctx context.Context, usuarioID string, in RegisterCredentialInput) (*BiometricRegisterOutput, error)
}

// BiometricDeviceView is the per-device row returned by /biometric/devices.
type BiometricDeviceView struct {
	ID              string
	DeviceID        string
	DeviceName      string
	DeviceModel     string
	AndroidVersion  string
	RegisteredAt    string // ISO-8601 timestamp
	LastUsedAt      string // ISO-8601 timestamp (may be empty when never used)
	IsActive        bool
	IsCurrentDevice bool
}

// ListBiometricDevicesUseCase returns the active credentials for a user; if
// currentDeviceID is non-empty the matching device is flagged as current.
type ListBiometricDevicesUseCase interface {
	Execute(ctx context.Context, usuarioID, currentDeviceID string) ([]BiometricDeviceView, error)
}

// RevokeBiometricDeviceUseCase soft-revokes a credential and purges its pending challenges.
type RevokeBiometricDeviceUseCase interface {
	Execute(ctx context.Context, usuarioID, deviceID string) error
}

// CheckBiometricStatusUseCase returns whether a deviceID has any active credential.
// Public endpoint — no JWT required.
type CheckBiometricStatusUseCase interface {
	Execute(ctx context.Context, deviceID string) (bool, error)
}
