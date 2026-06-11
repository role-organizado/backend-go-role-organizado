package guest

import (
	"context"
	"strings"
	"time"

	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ListBiometricDevices implements portin.ListBiometricDevicesUseCase.
type ListBiometricDevices struct {
	creds portout.BiometricCredentialRepository
}

// NewListBiometricDevices wires the use case.
func NewListBiometricDevices(creds portout.BiometricCredentialRepository) *ListBiometricDevices {
	return &ListBiometricDevices{creds: creds}
}

// Execute returns the active credentials for the authenticated user; if currentDeviceID
// is provided, the matching entry is flagged with IsCurrentDevice=true.
func (uc *ListBiometricDevices) Execute(
	ctx context.Context,
	usuarioID, currentDeviceID string,
) ([]portin.BiometricDeviceView, error) {
	if usuarioID == "" {
		return nil, apierr.Unauthorized("autenticação necessária")
	}
	credentials, err := uc.creds.FindByUsuarioIDActive(ctx, usuarioID)
	if err != nil {
		return nil, err
	}
	out := make([]portin.BiometricDeviceView, 0, len(credentials))
	cur := strings.TrimSpace(currentDeviceID)
	for _, c := range credentials {
		view := portin.BiometricDeviceView{
			ID:              c.ID,
			DeviceID:        c.DeviceID,
			DeviceName:      c.DeviceName,
			DeviceModel:     c.DeviceModel,
			AndroidVersion:  c.AndroidVersion,
			RegisteredAt:    c.CreatedAt.UTC().Format(time.RFC3339Nano),
			IsActive:        c.IsActive,
			IsCurrentDevice: cur != "" && cur == c.DeviceID,
		}
		if !c.LastUsedAt.IsZero() {
			view.LastUsedAt = c.LastUsedAt.UTC().Format(time.RFC3339Nano)
		}
		out = append(out, view)
	}
	return out, nil
}

// RevokeBiometricDevice implements portin.RevokeBiometricDeviceUseCase.
type RevokeBiometricDevice struct {
	creds      portout.BiometricCredentialRepository
	challenges portout.BiometricChallengeRepository
}

// NewRevokeBiometricDevice wires the use case.
func NewRevokeBiometricDevice(
	creds portout.BiometricCredentialRepository,
	challenges portout.BiometricChallengeRepository,
) *RevokeBiometricDevice {
	return &RevokeBiometricDevice{creds: creds, challenges: challenges}
}

// Execute soft-revokes the credential and purges all its pending challenges.
// 404 when the (UsuarioID, DeviceID) pair has no active credential — matches Java's
// ResourceNotFoundException.
func (uc *RevokeBiometricDevice) Execute(ctx context.Context, usuarioID, deviceID string) error {
	if usuarioID == "" {
		return apierr.Unauthorized("autenticação necessária")
	}
	if strings.TrimSpace(deviceID) == "" {
		return apierr.BadRequest("deviceId é obrigatório")
	}
	cred, err := uc.creds.FindByUsuarioIDAndDeviceIDActive(ctx, usuarioID, deviceID)
	if err != nil {
		if apierr.IsNotFound(err) {
			return apierr.NotFoundMsg("dispositivo biométrico não encontrado")
		}
		return err
	}
	now := time.Now().UTC()
	cred.IsActive = false
	cred.RevokedAt = &now
	cred.RevokedReason = "Revogado pelo usuário"
	if _, err := uc.creds.Update(ctx, cred); err != nil {
		return err
	}
	// Best-effort purge — challenges are TTL-driven anyway.
	_ = uc.challenges.DeleteByDeviceID(ctx, deviceID)
	return nil
}

// CheckBiometricStatus implements portin.CheckBiometricStatusUseCase.
type CheckBiometricStatus struct {
	creds portout.BiometricCredentialRepository
}

// NewCheckBiometricStatus wires the use case.
func NewCheckBiometricStatus(creds portout.BiometricCredentialRepository) *CheckBiometricStatus {
	return &CheckBiometricStatus{creds: creds}
}

// Execute reports whether the deviceID has any active credential.
// Public endpoint — must not leak which user owns the device.
func (uc *CheckBiometricStatus) Execute(ctx context.Context, deviceID string) (bool, error) {
	if strings.TrimSpace(deviceID) == "" {
		return false, nil
	}
	return uc.creds.ExistsByDeviceIDActive(ctx, deviceID)
}
