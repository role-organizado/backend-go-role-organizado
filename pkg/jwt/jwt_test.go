package jwt_test

import (
	"testing"
	"time"

	"github.com/role-organizado/backend-go-role-organizado/pkg/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecret = "test-secret-key-that-is-at-least-32-bytes-long-ok"

func newTestService(t *testing.T) *jwt.Service {
	t.Helper()
	svc, err := jwt.NewService(testSecret, 1*time.Hour, 7*24*time.Hour)
	require.NoError(t, err)
	return svc
}

func TestNewService_ShortSecret(t *testing.T) {
	_, err := jwt.NewService("short", time.Hour, time.Hour)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "32 bytes")
}

func TestGenerateAndValidateAccessToken(t *testing.T) {
	svc := newTestService(t)

	token, err := svc.GenerateAccessToken("user123", "joao@example.com", "João Silva", "+5511999999999", []string{"USER"})
	require.NoError(t, err)
	require.NotEmpty(t, token)

	claims, err := svc.ValidateToken(token)
	require.NoError(t, err)

	assert.Equal(t, "user123", claims.Sub)
	assert.Equal(t, "joao@example.com", claims.Email)
	assert.Equal(t, "João Silva", claims.Nome)
	assert.Equal(t, "+5511999999999", claims.Telefone)
	assert.Equal(t, []string{"USER"}, claims.Roles)
}

func TestGenerateTokenPair(t *testing.T) {
	svc := newTestService(t)

	pair, err := svc.GenerateTokenPair("user456", "maria@example.com", "Maria", "", []string{"USER", "ADMIN"})
	require.NoError(t, err)
	require.NotEmpty(t, pair.AccessToken)
	require.NotEmpty(t, pair.RefreshToken)
	assert.NotEqual(t, pair.AccessToken, pair.RefreshToken)

	claims, err := svc.ValidateToken(pair.AccessToken)
	require.NoError(t, err)
	assert.True(t, claims.HasRole("ADMIN"))
	assert.True(t, claims.HasRole("USER"))
	assert.False(t, claims.HasRole("MODERATOR"))
}

func TestValidateToken_Expired(t *testing.T) {
	svc, err := jwt.NewService(testSecret, -1*time.Minute, time.Hour)
	require.NoError(t, err)

	token, err := svc.GenerateAccessToken("userX", "x@example.com", "X", "", []string{"USER"})
	require.NoError(t, err)

	_, err = svc.ValidateToken(token)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expirado")
}

func TestValidateToken_InvalidSignature(t *testing.T) {
	svc := newTestService(t)

	token, err := svc.GenerateAccessToken("userY", "y@example.com", "Y", "", []string{"USER"})
	require.NoError(t, err)

	// Different secret
	otherSvc, err := jwt.NewService("other-secret-key-that-is-at-least-32-bytes!", time.Hour, time.Hour)
	require.NoError(t, err)

	_, err = otherSvc.ValidateToken(token)
	assert.Error(t, err)
}

func TestValidateToken_Malformed(t *testing.T) {
	svc := newTestService(t)
	_, err := svc.ValidateToken("not.a.valid.token")
	assert.Error(t, err)
}

func TestExtractUserID(t *testing.T) {
	svc := newTestService(t)

	token, err := svc.GenerateAccessToken("extractMe", "e@example.com", "E", "", []string{"USER"})
	require.NoError(t, err)

	id, err := svc.ExtractUserID(token)
	require.NoError(t, err)
	assert.Equal(t, "extractMe", id)
}

func TestHasRole(t *testing.T) {
	svc := newTestService(t)

	token, err := svc.GenerateAccessToken("roleUser", "r@example.com", "R", "", []string{"USER", "ADMIN"})
	require.NoError(t, err)

	claims, err := svc.ValidateToken(token)
	require.NoError(t, err)

	assert.True(t, claims.HasRole("USER"))
	assert.True(t, claims.HasRole("ADMIN"))
	assert.False(t, claims.HasRole("MODERATOR"))
}
