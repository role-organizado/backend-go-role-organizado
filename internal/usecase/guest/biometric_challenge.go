package guest

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"time"

	"github.com/google/uuid"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/guest"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ChallengeBytes is the size of the nonce — 32 bytes (256 bits), matching Java.
const ChallengeBytes = 32

// GenerateBiometricChallenge implements portin.GenerateBiometricChallengeUseCase.
type GenerateBiometricChallenge struct {
	creds      portout.BiometricCredentialRepository
	challenges portout.BiometricChallengeRepository
}

// NewGenerateBiometricChallenge wires the use case.
func NewGenerateBiometricChallenge(
	creds portout.BiometricCredentialRepository,
	challenges portout.BiometricChallengeRepository,
) *GenerateBiometricChallenge {
	return &GenerateBiometricChallenge{creds: creds, challenges: challenges}
}

// Execute generates and stores a Base64-encoded 32-byte nonce with a 5-minute TTL.
//
// Java parity:
//   - Requires an active credential for (UsuarioID, DeviceID); otherwise 404.
//   - Deletes all existing challenges for the deviceID BEFORE creating the new one
//     so there is only ever one pending challenge per device.
func (uc *GenerateBiometricChallenge) Execute(
	ctx context.Context,
	in portin.GenerateChallengeInput,
) (*portin.BiometricChallengeOutput, error) {
	if in.UsuarioID == "" || in.DeviceID == "" {
		return nil, apierr.BadRequest("usuarioId e deviceId são obrigatórios")
	}
	cred, err := uc.creds.FindByUsuarioIDAndDeviceIDActive(ctx, in.UsuarioID, in.DeviceID)
	if err != nil {
		if apierr.IsNotFound(err) {
			return nil, apierr.NotFoundMsg("biometria não registrada para este dispositivo")
		}
		return nil, err
	}
	_ = cred // confirms credential exists; not used further here

	if err := uc.challenges.DeleteByDeviceID(ctx, in.DeviceID); err != nil {
		return nil, err
	}

	nonce := make([]byte, ChallengeBytes)
	if _, err := rand.Read(nonce); err != nil {
		return nil, apierr.Internal("falha ao gerar nonce: " + err.Error())
	}
	encoded := base64.StdEncoding.EncodeToString(nonce)

	now := time.Now().UTC()
	ch := &domain.BiometricChallenge{
		ID:        uuid.New().String(),
		DeviceID:  in.DeviceID,
		Challenge: encoded,
		CreatedAt: now,
		ExpiresAt: now.Add(domain.ChallengeTTL),
	}
	if _, err := uc.challenges.Save(ctx, ch); err != nil {
		return nil, err
	}

	return &portin.BiometricChallengeOutput{
		Challenge:        encoded,
		ExpiresInSeconds: int(domain.ChallengeTTL / time.Second),
	}, nil
}
