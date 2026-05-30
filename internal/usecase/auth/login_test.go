package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/auth"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	ucauth "github.com/role-organizado/backend-go-role-organizado/internal/usecase/auth"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
	"github.com/role-organizado/backend-go-role-organizado/pkg/jwt"
)

// ---- Mocks ----

type mockUsuarioRepo struct{ mock.Mock }

func (m *mockUsuarioRepo) FindByID(ctx context.Context, id string) (*domain.Usuario, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Usuario), args.Error(1)
}
func (m *mockUsuarioRepo) FindByEmail(ctx context.Context, email string) (*domain.Usuario, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Usuario), args.Error(1)
}
func (m *mockUsuarioRepo) FindByProviderID(ctx context.Context, provider, puid string) (*domain.Usuario, error) {
	args := m.Called(ctx, provider, puid)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Usuario), args.Error(1)
}
func (m *mockUsuarioRepo) Save(ctx context.Context, u *domain.Usuario) (*domain.Usuario, error) {
	args := m.Called(ctx, u)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Usuario), args.Error(1)
}
func (m *mockUsuarioRepo) Update(ctx context.Context, u *domain.Usuario) (*domain.Usuario, error) {
	args := m.Called(ctx, u)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Usuario), args.Error(1)
}
func (m *mockUsuarioRepo) FindAll(ctx context.Context, page, size int) ([]domain.Usuario, int64, error) {
	args := m.Called(ctx, page, size)
	return args.Get(0).([]domain.Usuario), args.Get(1).(int64), args.Error(2)
}
func (m *mockUsuarioRepo) DeleteByID(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// ensure interface satisfaction at compile time
var _ portout.UsuarioRepository = (*mockUsuarioRepo)(nil)

type mockRefreshTokenRepo struct{ mock.Mock }

func (m *mockRefreshTokenRepo) Save(ctx context.Context, rt *domain.RefreshToken) (*domain.RefreshToken, error) {
	args := m.Called(ctx, rt)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.RefreshToken), args.Error(1)
}
func (m *mockRefreshTokenRepo) FindByToken(ctx context.Context, token string) (*domain.RefreshToken, error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.RefreshToken), args.Error(1)
}
func (m *mockRefreshTokenRepo) Revoke(ctx context.Context, token string) error {
	args := m.Called(ctx, token)
	return args.Error(0)
}
func (m *mockRefreshTokenRepo) RevokeAllForUser(ctx context.Context, uid string) error {
	args := m.Called(ctx, uid)
	return args.Error(0)
}

var _ portout.RefreshTokenRepository = (*mockRefreshTokenRepo)(nil)

// ---- helpers ----

func newTestJWT(t *testing.T) *jwt.Service {
	t.Helper()
	svc, err := jwt.NewService("test-secret-at-least-32bytes!!!x", time.Hour, 7*24*time.Hour)
	require.NoError(t, err)
	return svc
}

// ---- Tests: Login ----

func TestLogin_Success(t *testing.T) {
	uRepo := &mockUsuarioRepo{}
	rtRepo := &mockRefreshTokenRepo{}
	jwtSvc := newTestJWT(t)
	uc := ucauth.NewLogin(uRepo, rtRepo, jwtSvc)
	ctx := context.Background()

	hash, _ := bcrypt.GenerateFromPassword([]byte("senha123"), bcrypt.DefaultCost)
	usuario := &domain.Usuario{
		ID:        "abc123",
		Email:     "user@example.com",
		Nome:      "Test",
		SenhaHash: string(hash),
		Ativo:     true,
		Roles:     []domain.Role{domain.RoleUser},
	}

	uRepo.On("FindByEmail", ctx, "user@example.com").Return(usuario, nil)
	rtRepo.On("Save", ctx, mock.Anything).Return(&domain.RefreshToken{
		ID:        "rt1",
		UsuarioID: "abc123",
		Token:     "some-refresh-token",
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}, nil)

	out, err := uc.Execute(ctx, portin.LoginInput{Email: "user@example.com", Senha: "senha123"})
	require.NoError(t, err)
	assert.NotEmpty(t, out.AccessToken)
	assert.NotEmpty(t, out.RefreshToken)
	assert.Equal(t, "abc123", out.Usuario.ID)
	uRepo.AssertExpectations(t)
	rtRepo.AssertExpectations(t)
}

func TestLogin_WrongPassword_ReturnsUnauthorized(t *testing.T) {
	uRepo := &mockUsuarioRepo{}
	rtRepo := &mockRefreshTokenRepo{}
	jwtSvc := newTestJWT(t)
	uc := ucauth.NewLogin(uRepo, rtRepo, jwtSvc)
	ctx := context.Background()

	hash, _ := bcrypt.GenerateFromPassword([]byte("correct"), bcrypt.DefaultCost)
	usuario := &domain.Usuario{ID: "1", Email: "a@b.com", SenhaHash: string(hash)}
	uRepo.On("FindByEmail", ctx, "a@b.com").Return(usuario, nil)

	_, err := uc.Execute(ctx, portin.LoginInput{Email: "a@b.com", Senha: "wrong"})
	require.Error(t, err)
	assert.True(t, apierr.IsUnauthorized(err))
}

func TestLogin_UserNotFound_ReturnsUnauthorized(t *testing.T) {
	uRepo := &mockUsuarioRepo{}
	rtRepo := &mockRefreshTokenRepo{}
	jwtSvc := newTestJWT(t)
	uc := ucauth.NewLogin(uRepo, rtRepo, jwtSvc)
	ctx := context.Background()

	uRepo.On("FindByEmail", ctx, "noone@example.com").Return(nil, apierr.NotFound("usuario", "noone@example.com"))

	_, err := uc.Execute(ctx, portin.LoginInput{Email: "noone@example.com", Senha: "x"})
	require.Error(t, err)
	assert.True(t, apierr.IsUnauthorized(err))
}

// ---- Tests: Register ----

func TestRegister_Success_CreatesUserAndIssuesTokens(t *testing.T) {
	uRepo := &mockUsuarioRepo{}
	rtRepo := &mockRefreshTokenRepo{}
	jwtSvc := newTestJWT(t)
	uc := ucauth.NewRegister(uRepo, rtRepo, jwtSvc)
	ctx := context.Background()

	uRepo.On("FindByEmail", ctx, "new@example.com").Return(nil, apierr.NotFound("u", "x"))
	uRepo.On("Save", ctx, mock.Anything).Return(&domain.Usuario{
		ID:    "new_id",
		Email: "new@example.com",
		Nome:  "New User",
		Roles: []domain.Role{domain.RoleUser},
	}, nil)
	rtRepo.On("Save", ctx, mock.Anything).Return(&domain.RefreshToken{
		Token:     "token",
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}, nil)

	out, err := uc.Execute(ctx, portin.RegisterInput{Nome: "New User", Email: "new@example.com", Senha: "strongpass"})
	require.NoError(t, err)
	assert.Equal(t, "new_id", out.Usuario.ID)
	assert.NotEmpty(t, out.AccessToken)
}

func TestRegister_EmailAlreadyExists_ReturnsConflict(t *testing.T) {
	uRepo := &mockUsuarioRepo{}
	rtRepo := &mockRefreshTokenRepo{}
	jwtSvc := newTestJWT(t)
	uc := ucauth.NewRegister(uRepo, rtRepo, jwtSvc)
	ctx := context.Background()

	uRepo.On("FindByEmail", ctx, "exists@example.com").Return(&domain.Usuario{ID: "1"}, nil)

	_, err := uc.Execute(ctx, portin.RegisterInput{Email: "exists@example.com", Senha: "x", Nome: "n"})
	require.Error(t, err)
	var apiErr *apierr.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, apierr.CodeConflict, apiErr.Code)
}

// ---- Tests: Logout ----

func TestLogout_RevokesAllTokens(t *testing.T) {
	rtRepo := &mockRefreshTokenRepo{}
	uc := ucauth.NewLogout(rtRepo)
	ctx := context.Background()

	rtRepo.On("RevokeAllForUser", ctx, "uid1").Return(nil)

	err := uc.Execute(ctx, "uid1")
	require.NoError(t, err)
	rtRepo.AssertExpectations(t)
}
