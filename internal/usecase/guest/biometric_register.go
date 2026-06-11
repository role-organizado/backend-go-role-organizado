package guest

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/guest"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// MinPublicKeySize is the same @Size(min=50) constraint as the Java DTO.
const MinPublicKeySize = 50

// RegisterBiometricCredential implements portin.RegisterBiometricCredentialUseCase.
type RegisterBiometricCredential struct {
	creds    portout.BiometricCredentialRepository
	usuarios portout.UsuarioRepository
}

// NewRegisterBiometricCredential wires the use case.
func NewRegisterBiometricCredential(
	creds portout.BiometricCredentialRepository,
	usuarios portout.UsuarioRepository,
) *RegisterBiometricCredential {
	return &RegisterBiometricCredential{creds: creds, usuarios: usuarios}
}

// Execute creates a new BiometricCredential for the (UsuarioID, DeviceID) pair.
// If an active credential already exists for the same pair it is soft-revoked first
// with reason "Substituída por novo registro" (idempotent re-registration).
func (uc *RegisterBiometricCredential) Execute(
	ctx context.Context,
	usuarioID string,
	in portin.RegisterCredentialInput,
) (*portin.BiometricRegisterOutput, error) {
	if usuarioID == "" {
		return nil, apierr.Unauthorized("autenticação necessária")
	}
	deviceID := strings.TrimSpace(in.DeviceID)
	if deviceID == "" {
		return nil, apierr.BadRequest("deviceId é obrigatório")
	}
	pubKey := strings.TrimSpace(in.PublicKey)
	if len(pubKey) < MinPublicKeySize {
		return nil, apierr.BadRequest("publicKey deve ter no mínimo 50 caracteres")
	}
	if _, err := parsePublicKey(pubKey); err != nil {
		return nil, apierr.BadRequest("publicKey inválida: " + err.Error())
	}
	deviceName := strings.TrimSpace(in.DeviceName)
	if deviceName == "" {
		return nil, apierr.BadRequest("deviceName é obrigatório")
	}
	if len(deviceName) > 100 {
		return nil, apierr.BadRequest("deviceName excede 100 caracteres")
	}

	if _, err := uc.usuarios.FindByID(ctx, usuarioID); err != nil {
		if apierr.IsNotFound(err) {
			return nil, apierr.BadRequest("usuário não encontrado")
		}
		return nil, err
	}

	// Soft-revoke any existing active credential for the same (user, device).
	existing, err := uc.creds.FindByUsuarioIDAndDeviceIDActive(ctx, usuarioID, deviceID)
	if err != nil && !apierr.IsNotFound(err) {
		return nil, err
	}
	if existing != nil {
		now := time.Now().UTC()
		existing.IsActive = false
		existing.RevokedAt = &now
		existing.RevokedReason = "Substituída por novo registro"
		if _, err := uc.creds.Update(ctx, existing); err != nil {
			return nil, err
		}
	}

	now := time.Now().UTC()
	cred := &domain.BiometricCredential{
		ID:             uuid.New().String(),
		UsuarioID:      usuarioID,
		DeviceID:       deviceID,
		PublicKey:      pubKey,
		DeviceName:     deviceName,
		DeviceModel:    strings.TrimSpace(in.DeviceModel),
		AndroidVersion: strings.TrimSpace(in.AndroidVersion),
		CreatedAt:      now,
		LastUsedAt:     now,
		IsActive:       true,
	}
	saved, err := uc.creds.Save(ctx, cred)
	if err != nil {
		return nil, err
	}

	return &portin.BiometricRegisterOutput{
		Success:      true,
		Message:      "Credencial biométrica registrada com sucesso",
		DeviceID:     saved.DeviceID,
		DeviceName:   saved.DeviceName,
		RegisteredAt: saved.CreatedAt.UTC().Format(time.RFC3339Nano),
	}, nil
}
