package guest_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	authdomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/auth"
	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/guest"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	ucguest "github.com/role-organizado/backend-go-role-organizado/internal/usecase/guest"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
	pkgjwt "github.com/role-organizado/backend-go-role-organizado/pkg/jwt"
)

// ============================================================
// Mocks
// ============================================================

type mockCredRepo struct{ mock.Mock }

func (m *mockCredRepo) Save(ctx context.Context, c *domain.BiometricCredential) (*domain.BiometricCredential, error) {
	args := m.Called(ctx, c)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.BiometricCredential), args.Error(1)
}
func (m *mockCredRepo) Update(ctx context.Context, c *domain.BiometricCredential) (*domain.BiometricCredential, error) {
	args := m.Called(ctx, c)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.BiometricCredential), args.Error(1)
}
func (m *mockCredRepo) FindByID(ctx context.Context, id string) (*domain.BiometricCredential, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.BiometricCredential), args.Error(1)
}
func (m *mockCredRepo) FindByDeviceID(ctx context.Context, deviceID string) (*domain.BiometricCredential, error) {
	args := m.Called(ctx, deviceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.BiometricCredential), args.Error(1)
}
func (m *mockCredRepo) FindByUsuarioIDAndDeviceID(ctx context.Context, usuarioID, deviceID string) (*domain.BiometricCredential, error) {
	args := m.Called(ctx, usuarioID, deviceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.BiometricCredential), args.Error(1)
}
func (m *mockCredRepo) FindByUsuarioIDAndDeviceIDActive(ctx context.Context, usuarioID, deviceID string) (*domain.BiometricCredential, error) {
	args := m.Called(ctx, usuarioID, deviceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.BiometricCredential), args.Error(1)
}
func (m *mockCredRepo) FindByUsuarioIDActive(ctx context.Context, usuarioID string) ([]domain.BiometricCredential, error) {
	args := m.Called(ctx, usuarioID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.BiometricCredential), args.Error(1)
}
func (m *mockCredRepo) FindByUsuarioID(ctx context.Context, usuarioID string) ([]domain.BiometricCredential, error) {
	args := m.Called(ctx, usuarioID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.BiometricCredential), args.Error(1)
}
func (m *mockCredRepo) ExistsByDeviceIDActive(ctx context.Context, deviceID string) (bool, error) {
	args := m.Called(ctx, deviceID)
	return args.Bool(0), args.Error(1)
}
func (m *mockCredRepo) DeleteByID(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockCredRepo) DeleteByUsuarioID(ctx context.Context, usuarioID string) error {
	return m.Called(ctx, usuarioID).Error(0)
}
func (m *mockCredRepo) CountByUsuarioIDActive(ctx context.Context, usuarioID string) (int64, error) {
	args := m.Called(ctx, usuarioID)
	return args.Get(0).(int64), args.Error(1)
}

type mockChallengeRepo struct{ mock.Mock }

func (m *mockChallengeRepo) Save(ctx context.Context, c *domain.BiometricChallenge) (*domain.BiometricChallenge, error) {
	args := m.Called(ctx, c)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.BiometricChallenge), args.Error(1)
}
func (m *mockChallengeRepo) Update(ctx context.Context, c *domain.BiometricChallenge) (*domain.BiometricChallenge, error) {
	args := m.Called(ctx, c)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.BiometricChallenge), args.Error(1)
}
func (m *mockChallengeRepo) FindByID(ctx context.Context, id string) (*domain.BiometricChallenge, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.BiometricChallenge), args.Error(1)
}
func (m *mockChallengeRepo) FindLatestByDeviceIDUnused(ctx context.Context, deviceID string) (*domain.BiometricChallenge, error) {
	args := m.Called(ctx, deviceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.BiometricChallenge), args.Error(1)
}
func (m *mockChallengeRepo) DeleteByDeviceID(ctx context.Context, deviceID string) error {
	return m.Called(ctx, deviceID).Error(0)
}
func (m *mockChallengeRepo) DeleteByID(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}

type mockUsuarioRepo struct{ mock.Mock }

func (m *mockUsuarioRepo) FindByID(ctx context.Context, id string) (*authdomain.Usuario, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authdomain.Usuario), args.Error(1)
}
func (m *mockUsuarioRepo) FindByEmail(ctx context.Context, email string) (*authdomain.Usuario, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authdomain.Usuario), args.Error(1)
}
func (m *mockUsuarioRepo) FindByProviderID(ctx context.Context, provider, providerUserID string) (*authdomain.Usuario, error) {
	args := m.Called(ctx, provider, providerUserID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authdomain.Usuario), args.Error(1)
}
func (m *mockUsuarioRepo) Save(ctx context.Context, u *authdomain.Usuario) (*authdomain.Usuario, error) {
	args := m.Called(ctx, u)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authdomain.Usuario), args.Error(1)
}
func (m *mockUsuarioRepo) Update(ctx context.Context, u *authdomain.Usuario) (*authdomain.Usuario, error) {
	args := m.Called(ctx, u)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authdomain.Usuario), args.Error(1)
}
func (m *mockUsuarioRepo) FindAll(ctx context.Context, page, pageSize int) ([]authdomain.Usuario, int64, error) {
	args := m.Called(ctx, page, pageSize)
	return nil, 0, args.Error(0)
}
func (m *mockUsuarioRepo) DeleteByID(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}

type mockRefreshRepo struct{ mock.Mock }

func (m *mockRefreshRepo) Save(ctx context.Context, rt *authdomain.RefreshToken) (*authdomain.RefreshToken, error) {
	args := m.Called(ctx, rt)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authdomain.RefreshToken), args.Error(1)
}
func (m *mockRefreshRepo) FindByToken(ctx context.Context, token string) (*authdomain.RefreshToken, error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authdomain.RefreshToken), args.Error(1)
}
func (m *mockRefreshRepo) Revoke(ctx context.Context, token string) error {
	return m.Called(ctx, token).Error(0)
}
func (m *mockRefreshRepo) RevokeAllForUser(ctx context.Context, usuarioID string) error {
	return m.Called(ctx, usuarioID).Error(0)
}

// ============================================================
// Helpers
// ============================================================

const (
	bUserID   = "user-1"
	bDeviceID = "device-abc"
	jwtSecret = "test-secret-with-at-least-32-bytes-1234"
)

// genECKey returns an ECDSA P-256 key pair plus the Base64 X.509 SPKI form of the public key.
func genECKey(t *testing.T) (*ecdsa.PrivateKey, string) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	pubBytes, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	require.NoError(t, err)
	return priv, base64.StdEncoding.EncodeToString(pubBytes)
}

// signChallenge signs SHA-256(challengeBytes) with ECDSA ASN.1, returning Base64.
func signChallenge(t *testing.T, priv *ecdsa.PrivateKey, challengeB64 string) string {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(challengeB64)
	require.NoError(t, err)
	hash := sha256.Sum256(raw)
	sig, err := ecdsa.SignASN1(rand.Reader, priv, hash[:])
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(sig)
}

func newJWTService(t *testing.T) *pkgjwt.Service {
	t.Helper()
	svc, err := pkgjwt.NewService(jwtSecret, 15*time.Minute, 24*time.Hour)
	require.NoError(t, err)
	return svc
}

// ============================================================
// GenerateBiometricChallenge
// ============================================================

func TestGenerateBiometricChallenge_Execute(t *testing.T) {
	ctx := context.Background()
	creds := &mockCredRepo{}
	challenges := &mockChallengeRepo{}

	cred := &domain.BiometricCredential{
		ID: "c1", UsuarioID: bUserID, DeviceID: bDeviceID, IsActive: true,
	}
	creds.On("FindByUsuarioIDAndDeviceIDActive", ctx, bUserID, bDeviceID).Return(cred, nil)
	challenges.On("DeleteByDeviceID", ctx, bDeviceID).Return(nil)
	challenges.On("Save", ctx, mock.MatchedBy(func(c *domain.BiometricChallenge) bool {
		return c.DeviceID == bDeviceID && len(c.Challenge) > 0
	})).Return(&domain.BiometricChallenge{}, nil)

	uc := ucguest.NewGenerateBiometricChallenge(creds, challenges)
	out, err := uc.Execute(ctx, portin.GenerateChallengeInput{UsuarioID: bUserID, DeviceID: bDeviceID})
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, 300, out.ExpiresInSeconds)
	raw, _ := base64.StdEncoding.DecodeString(out.Challenge)
	assert.Len(t, raw, 32)
	creds.AssertExpectations(t)
	challenges.AssertExpectations(t)
}

func TestGenerateBiometricChallenge_NoCredential(t *testing.T) {
	ctx := context.Background()
	creds := &mockCredRepo{}
	challenges := &mockChallengeRepo{}
	creds.On("FindByUsuarioIDAndDeviceIDActive", ctx, bUserID, bDeviceID).
		Return(nil, apierr.NotFound("biometric_credential", "x"))
	uc := ucguest.NewGenerateBiometricChallenge(creds, challenges)
	_, err := uc.Execute(ctx, portin.GenerateChallengeInput{UsuarioID: bUserID, DeviceID: bDeviceID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "biometria")
}

// ============================================================
// BiometricAuthenticate
// ============================================================

func TestBiometricAuthenticate_Execute_HappyPath(t *testing.T) {
	ctx := context.Background()
	priv, pubB64 := genECKey(t)

	creds := &mockCredRepo{}
	challenges := &mockChallengeRepo{}
	usuarios := &mockUsuarioRepo{}
	refreshes := &mockRefreshRepo{}
	jwtSvc := newJWTService(t)

	cred := &domain.BiometricCredential{
		ID: "c1", UsuarioID: bUserID, DeviceID: bDeviceID, PublicKey: pubB64, IsActive: true,
	}
	// Build a real challenge document.
	nonce := make([]byte, 32)
	_, _ = rand.Read(nonce)
	challengeB64 := base64.StdEncoding.EncodeToString(nonce)
	ch := &domain.BiometricChallenge{
		ID: "ch1", DeviceID: bDeviceID, Challenge: challengeB64,
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().Add(domain.ChallengeTTL).UTC(),
	}
	sig := signChallenge(t, priv, challengeB64)
	usuario := &authdomain.Usuario{ID: bUserID, Email: "a@b.com", Nome: "Alice", Roles: []authdomain.Role{authdomain.RoleUser}}

	creds.On("FindByUsuarioIDAndDeviceIDActive", ctx, bUserID, bDeviceID).Return(cred, nil)
	challenges.On("FindLatestByDeviceIDUnused", ctx, bDeviceID).Return(ch, nil)
	challenges.On("Update", ctx, mock.MatchedBy(func(c *domain.BiometricChallenge) bool {
		return c.Used && c.UsedFromIP == "1.2.3.4"
	})).Return(ch, nil)
	creds.On("Update", ctx, mock.MatchedBy(func(c *domain.BiometricCredential) bool {
		return !c.LastUsedAt.IsZero()
	})).Return(cred, nil)
	usuarios.On("FindByID", ctx, bUserID).Return(usuario, nil)
	refreshes.On("Save", ctx, mock.AnythingOfType("*auth.RefreshToken")).Return(&authdomain.RefreshToken{}, nil)

	uc := ucguest.NewBiometricAuthenticate(creds, challenges, usuarios, refreshes, jwtSvc)
	out, err := uc.Execute(ctx,
		portin.BiometricAuthenticateInput{UsuarioID: bUserID, DeviceID: bDeviceID, Signature: sig},
		"1.2.3.4")
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.NotEmpty(t, out.AccessToken)
	assert.NotEmpty(t, out.RefreshToken)
	assert.Equal(t, bUserID, out.Usuario.ID)
	creds.AssertExpectations(t)
	challenges.AssertExpectations(t)
	usuarios.AssertExpectations(t)
	refreshes.AssertExpectations(t)
}

func TestBiometricAuthenticate_Execute_BadSignatureReturns401(t *testing.T) {
	ctx := context.Background()
	_, pubB64 := genECKey(t)
	creds := &mockCredRepo{}
	challenges := &mockChallengeRepo{}
	usuarios := &mockUsuarioRepo{}
	refreshes := &mockRefreshRepo{}
	jwtSvc := newJWTService(t)

	cred := &domain.BiometricCredential{
		ID: "c1", UsuarioID: bUserID, DeviceID: bDeviceID, PublicKey: pubB64, IsActive: true,
	}
	nonce := make([]byte, 32)
	_, _ = rand.Read(nonce)
	challengeB64 := base64.StdEncoding.EncodeToString(nonce)
	ch := &domain.BiometricChallenge{
		ID: "ch1", DeviceID: bDeviceID, Challenge: challengeB64,
		ExpiresAt: time.Now().Add(domain.ChallengeTTL).UTC(),
	}

	creds.On("FindByUsuarioIDAndDeviceIDActive", ctx, bUserID, bDeviceID).Return(cred, nil)
	challenges.On("FindLatestByDeviceIDUnused", ctx, bDeviceID).Return(ch, nil)

	uc := ucguest.NewBiometricAuthenticate(creds, challenges, usuarios, refreshes, jwtSvc)
	bogusSig := base64.StdEncoding.EncodeToString([]byte("not-a-real-signature"))
	_, err := uc.Execute(ctx,
		portin.BiometricAuthenticateInput{UsuarioID: bUserID, DeviceID: bDeviceID, Signature: bogusSig},
		"1.2.3.4")
	require.Error(t, err)
	assert.True(t, apierr.IsUnauthorized(err))
}

func TestBiometricAuthenticate_Execute_ExpiredChallenge(t *testing.T) {
	ctx := context.Background()
	_, pubB64 := genECKey(t)
	creds := &mockCredRepo{}
	challenges := &mockChallengeRepo{}
	usuarios := &mockUsuarioRepo{}
	refreshes := &mockRefreshRepo{}
	jwtSvc := newJWTService(t)

	cred := &domain.BiometricCredential{
		ID: "c1", UsuarioID: bUserID, DeviceID: bDeviceID, PublicKey: pubB64, IsActive: true,
	}
	ch := &domain.BiometricChallenge{
		ID: "ch1", DeviceID: bDeviceID, Challenge: base64.StdEncoding.EncodeToString([]byte("x")),
		ExpiresAt: time.Now().Add(-1 * time.Second).UTC(),
	}
	creds.On("FindByUsuarioIDAndDeviceIDActive", ctx, bUserID, bDeviceID).Return(cred, nil)
	challenges.On("FindLatestByDeviceIDUnused", ctx, bDeviceID).Return(ch, nil)

	uc := ucguest.NewBiometricAuthenticate(creds, challenges, usuarios, refreshes, jwtSvc)
	_, err := uc.Execute(ctx,
		portin.BiometricAuthenticateInput{UsuarioID: bUserID, DeviceID: bDeviceID, Signature: "x"},
		"1.1.1.1")
	require.Error(t, err)
	assert.True(t, apierr.IsUnauthorized(err))
}

// ============================================================
// RegisterBiometricCredential
// ============================================================

func TestRegisterBiometricCredential_Execute_NewDevice(t *testing.T) {
	ctx := context.Background()
	_, pubB64 := genECKey(t)
	creds := &mockCredRepo{}
	usuarios := &mockUsuarioRepo{}
	usuarios.On("FindByID", ctx, bUserID).Return(&authdomain.Usuario{ID: bUserID, Nome: "U"}, nil)
	creds.On("FindByUsuarioIDAndDeviceIDActive", ctx, bUserID, bDeviceID).
		Return(nil, apierr.NotFound("biometric_credential", "x"))
	creds.On("Save", ctx, mock.MatchedBy(func(c *domain.BiometricCredential) bool {
		return c.UsuarioID == bUserID && c.DeviceID == bDeviceID && c.IsActive
	})).Return(&domain.BiometricCredential{ID: "new", DeviceID: bDeviceID, DeviceName: "Pixel", CreatedAt: time.Now().UTC()}, nil)

	uc := ucguest.NewRegisterBiometricCredential(creds, usuarios)
	out, err := uc.Execute(ctx, bUserID, portin.RegisterCredentialInput{
		DeviceID: bDeviceID, PublicKey: pubB64, DeviceName: "Pixel",
	})
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.True(t, out.Success)
	assert.Equal(t, bDeviceID, out.DeviceID)
	creds.AssertExpectations(t)
	usuarios.AssertExpectations(t)
}

func TestRegisterBiometricCredential_Execute_SoftRevokesExisting(t *testing.T) {
	ctx := context.Background()
	_, pubB64 := genECKey(t)
	creds := &mockCredRepo{}
	usuarios := &mockUsuarioRepo{}
	usuarios.On("FindByID", ctx, bUserID).Return(&authdomain.Usuario{ID: bUserID, Nome: "U"}, nil)
	existing := &domain.BiometricCredential{ID: "old", UsuarioID: bUserID, DeviceID: bDeviceID, IsActive: true}
	creds.On("FindByUsuarioIDAndDeviceIDActive", ctx, bUserID, bDeviceID).Return(existing, nil)
	creds.On("Update", ctx, mock.MatchedBy(func(c *domain.BiometricCredential) bool {
		return !c.IsActive && c.RevokedReason == "Substituída por novo registro"
	})).Return(existing, nil)
	creds.On("Save", ctx, mock.AnythingOfType("*guest.BiometricCredential")).
		Return(&domain.BiometricCredential{ID: "new", DeviceID: bDeviceID, DeviceName: "P", CreatedAt: time.Now().UTC()}, nil)

	uc := ucguest.NewRegisterBiometricCredential(creds, usuarios)
	out, err := uc.Execute(ctx, bUserID, portin.RegisterCredentialInput{
		DeviceID: bDeviceID, PublicKey: pubB64, DeviceName: "P",
	})
	require.NoError(t, err)
	assert.True(t, out.Success)
	creds.AssertExpectations(t)
}

func TestRegisterBiometricCredential_Execute_RejectsShortPublicKey(t *testing.T) {
	ctx := context.Background()
	creds := &mockCredRepo{}
	usuarios := &mockUsuarioRepo{}
	uc := ucguest.NewRegisterBiometricCredential(creds, usuarios)
	_, err := uc.Execute(ctx, bUserID, portin.RegisterCredentialInput{
		DeviceID: bDeviceID, PublicKey: "too-short", DeviceName: "P",
	})
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "publickey")
}

// ============================================================
// Devices: list / revoke / status
// ============================================================

func TestListBiometricDevices_FlagsCurrent(t *testing.T) {
	ctx := context.Background()
	creds := &mockCredRepo{}
	creds.On("FindByUsuarioIDActive", ctx, bUserID).Return([]domain.BiometricCredential{
		{ID: "a", DeviceID: bDeviceID, DeviceName: "Pixel", IsActive: true, CreatedAt: time.Now().UTC()},
		{ID: "b", DeviceID: "other", DeviceName: "Note", IsActive: true, CreatedAt: time.Now().UTC()},
	}, nil)
	uc := ucguest.NewListBiometricDevices(creds)
	out, err := uc.Execute(ctx, bUserID, bDeviceID)
	require.NoError(t, err)
	assert.Len(t, out, 2)
	assert.True(t, out[0].IsCurrentDevice)
	assert.False(t, out[1].IsCurrentDevice)
}

func TestRevokeBiometricDevice_PurgesChallenges(t *testing.T) {
	ctx := context.Background()
	creds := &mockCredRepo{}
	challenges := &mockChallengeRepo{}
	existing := &domain.BiometricCredential{ID: "c", UsuarioID: bUserID, DeviceID: bDeviceID, IsActive: true}
	creds.On("FindByUsuarioIDAndDeviceIDActive", ctx, bUserID, bDeviceID).Return(existing, nil)
	creds.On("Update", ctx, mock.MatchedBy(func(c *domain.BiometricCredential) bool {
		return !c.IsActive && c.RevokedReason == "Revogado pelo usuário"
	})).Return(existing, nil)
	challenges.On("DeleteByDeviceID", ctx, bDeviceID).Return(nil)
	uc := ucguest.NewRevokeBiometricDevice(creds, challenges)
	err := uc.Execute(ctx, bUserID, bDeviceID)
	require.NoError(t, err)
	creds.AssertExpectations(t)
	challenges.AssertExpectations(t)
}

func TestRevokeBiometricDevice_NotFoundReturns404(t *testing.T) {
	ctx := context.Background()
	creds := &mockCredRepo{}
	challenges := &mockChallengeRepo{}
	creds.On("FindByUsuarioIDAndDeviceIDActive", ctx, bUserID, bDeviceID).
		Return(nil, apierr.NotFound("biometric_credential", "x"))
	uc := ucguest.NewRevokeBiometricDevice(creds, challenges)
	err := uc.Execute(ctx, bUserID, bDeviceID)
	require.Error(t, err)
	assert.True(t, apierr.IsNotFound(err))
}

func TestCheckBiometricStatus_DelegatesToRepo(t *testing.T) {
	ctx := context.Background()
	creds := &mockCredRepo{}
	creds.On("ExistsByDeviceIDActive", ctx, bDeviceID).Return(true, nil)
	uc := ucguest.NewCheckBiometricStatus(creds)
	enabled, err := uc.Execute(ctx, bDeviceID)
	require.NoError(t, err)
	assert.True(t, enabled)
}
