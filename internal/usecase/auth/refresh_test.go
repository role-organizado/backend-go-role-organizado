package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/auth"
	ucauth "github.com/role-organizado/backend-go-role-organizado/internal/usecase/auth"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ---- Tests: RefreshToken ----

func TestRefreshToken_Success(t *testing.T) {
	uRepo := &mockUsuarioRepo{}
	rtRepo := &mockRefreshTokenRepo{}
	jwtSvc := newTestJWT(t)
	uc := ucauth.NewRefreshToken(rtRepo, uRepo, jwtSvc)
	ctx := context.Background()

	existingToken := "some-valid-refresh-token"
	userID := "507f1f77bcf86cd799439011" // ObjectID hex (Go-created user)

	rtRepo.On("FindByToken", mock.Anything, existingToken).Return(&domain.RefreshToken{
		ID:        "rt-id",
		UsuarioID: userID,
		Token:     existingToken,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		Used:      false,
	}, nil)
	rtRepo.On("Revoke", mock.Anything, existingToken).Return(nil)
	uRepo.On("FindByID", mock.Anything, userID).Return(&domain.Usuario{
		ID:    userID,
		Email: "user@example.com",
		Nome:  "Test",
		Ativo: true,
		Roles: []domain.Role{domain.RoleUser},
	}, nil)
	rtRepo.On("Save", mock.Anything, mock.Anything).Return(&domain.RefreshToken{
		Token:     "new-refresh-token",
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}, nil)

	out, err := uc.Execute(ctx, existingToken)

	require.NoError(t, err)
	assert.NotEmpty(t, out.AccessToken)
	assert.NotEmpty(t, out.RefreshToken)
	assert.Equal(t, userID, out.Usuario.ID)
	rtRepo.AssertExpectations(t)
	uRepo.AssertExpectations(t)
}

func TestRefreshToken_TokenNotFound_ReturnsUnauthorized(t *testing.T) {
	uRepo := &mockUsuarioRepo{}
	rtRepo := &mockRefreshTokenRepo{}
	jwtSvc := newTestJWT(t)
	uc := ucauth.NewRefreshToken(rtRepo, uRepo, jwtSvc)
	ctx := context.Background()

	rtRepo.On("FindByToken", mock.Anything, "nonexistent").Return(nil, apierr.NotFound("refresh_token", "nonexistent"))

	_, err := uc.Execute(ctx, "nonexistent")

	require.Error(t, err)
	assert.True(t, apierr.IsUnauthorized(err), "expected Unauthorized, got %v", err)
}

func TestRefreshToken_TokenExpired_ReturnsUnauthorized(t *testing.T) {
	uRepo := &mockUsuarioRepo{}
	rtRepo := &mockRefreshTokenRepo{}
	jwtSvc := newTestJWT(t)
	uc := ucauth.NewRefreshToken(rtRepo, uRepo, jwtSvc)
	ctx := context.Background()

	rtRepo.On("FindByToken", mock.Anything, "expired-token").Return(&domain.RefreshToken{
		Token:     "expired-token",
		UsuarioID: "uid",
		ExpiresAt: time.Now().Add(-1 * time.Hour), // already expired
		Used:      false,
	}, nil)

	_, err := uc.Execute(ctx, "expired-token")

	require.Error(t, err)
	assert.True(t, apierr.IsUnauthorized(err), "expected Unauthorized, got %v", err)
}

func TestRefreshToken_TokenAlreadyUsed_ReturnsUnauthorized(t *testing.T) {
	uRepo := &mockUsuarioRepo{}
	rtRepo := &mockRefreshTokenRepo{}
	jwtSvc := newTestJWT(t)
	uc := ucauth.NewRefreshToken(rtRepo, uRepo, jwtSvc)
	ctx := context.Background()

	rtRepo.On("FindByToken", mock.Anything, "used-token").Return(&domain.RefreshToken{
		Token:     "used-token",
		UsuarioID: "uid",
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		Used:      true, // already used
	}, nil)

	_, err := uc.Execute(ctx, "used-token")

	require.Error(t, err)
	assert.True(t, apierr.IsUnauthorized(err), "expected Unauthorized, got %v", err)
}

// ---- Tests: ValidateToken ----

func TestValidateToken_Success(t *testing.T) {
	uRepo := &mockUsuarioRepo{}
	jwtSvc := newTestJWT(t)
	uc := ucauth.NewValidateToken(uRepo, jwtSvc)
	ctx := context.Background()

	// Generate a real token to validate
	pair, err := jwtSvc.GenerateTokenPair("user-id-123", "u@example.com", "U", "", []string{"USER"})
	require.NoError(t, err)

	uRepo.On("FindByID", mock.Anything, "user-id-123").Return(&domain.Usuario{
		ID:    "user-id-123",
		Email: "u@example.com",
		Nome:  "U",
		Ativo: true,
	}, nil)

	out, err := uc.Execute(ctx, pair.AccessToken)

	require.NoError(t, err)
	assert.Equal(t, "user-id-123", out.Usuario.ID)
	assert.Equal(t, pair.AccessToken, out.AccessToken)
}

func TestValidateToken_InvalidToken_ReturnsUnauthorized(t *testing.T) {
	uRepo := &mockUsuarioRepo{}
	jwtSvc := newTestJWT(t)
	uc := ucauth.NewValidateToken(uRepo, jwtSvc)
	ctx := context.Background()

	_, err := uc.Execute(ctx, "this.is.not.a.valid.jwt")

	require.Error(t, err)
	assert.True(t, apierr.IsUnauthorized(err), "expected Unauthorized, got %v", err)
}
