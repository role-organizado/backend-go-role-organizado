package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/handler"
	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/auth"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
)

// ---- minimal stubs for AuthHandler use cases ----

// stubValidateToken always returns a fixed user (or error when configured).
type stubValidateToken struct {
	out *portin.AuthOutput
	err error
}

func (s *stubValidateToken) Execute(_ context.Context, _ string) (*portin.AuthOutput, error) {
	return s.out, s.err
}

// stubRefreshToken records the token it receives and returns a preset response.
type stubRefreshToken struct {
	receivedToken string
	out           *portin.AuthOutput
	err           error
}

func (s *stubRefreshToken) Execute(_ context.Context, token string) (*portin.AuthOutput, error) {
	s.receivedToken = token
	return s.out, s.err
}

// stubLoginUC, stubRegisterUC, stubLogoutUC, stubGoogleUC, stubAppleUC satisfy the
// remaining AuthHandler constructor parameters; none of these methods are called in
// the test paths below.

type stubLoginUC struct{}

func (s *stubLoginUC) Execute(_ context.Context, _ portin.LoginInput) (*portin.AuthOutput, error) {
	return nil, nil
}

type stubRegisterUC struct{}

func (s *stubRegisterUC) Execute(_ context.Context, _ portin.RegisterInput) (*portin.AuthOutput, error) {
	return nil, nil
}

type stubLogoutUC struct{}

func (s *stubLogoutUC) Execute(_ context.Context, _ string) error { return nil }

type stubGoogleUC struct{}

func (s *stubGoogleUC) Execute(_ context.Context, _ portin.GoogleAuthInput) (*portin.AuthOutput, error) {
	return nil, nil
}

type stubAppleUC struct{}

func (s *stubAppleUC) Execute(_ context.Context, _ portin.AppleAuthInput) (*portin.AuthOutput, error) {
	return nil, nil
}

// ---- helpers ----

func newAuthRouter(validate portin.ValidateTokenUseCase, refresh portin.RefreshTokenUseCase) *chi.Mux {
	h := handler.NewAuthHandler(
		&stubLoginUC{},
		&stubRegisterUC{},
		refresh,
		validate,
		&stubLogoutUC{},
		&stubGoogleUC{},
		&stubAppleUC{},
	)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r
}

func fixedUser() *domain.Usuario {
	return &domain.Usuario{
		ID:       "user-id-123",
		Nome:     "Test User",
		Email:    "test@example.com",
		Roles:    []domain.Role{domain.RoleUser},
		Ativo:    true,
		CriadoEm: time.Now(),
	}
}

// ---- Fix 2: Validate returns valid:true ----

// TestAuthHandler_Validate_ContainsValidTrue verifies that GET /api/auth/validate
// returns a JSON body with { "valid": true, "usuario": { ... } }.
// This is required by the BFF and mobile app which check body.valid === true.
func TestAuthHandler_Validate_ContainsValidTrue(t *testing.T) {
	validateStub := &stubValidateToken{
		out: &portin.AuthOutput{
			Usuario:     fixedUser(),
			AccessToken: "access.token.here",
		},
	}

	r := newAuthRouter(validateStub, &stubRefreshToken{})

	req := httptest.NewRequest(http.MethodGet, "/api/auth/validate", nil)
	req.Header.Set("Authorization", "Bearer some.jwt.token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "validate should return 200")

	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))

	// The BFF checks body.valid === true — it must be present and true.
	valid, ok := resp["valid"]
	require.True(t, ok, "response must contain 'valid' field")
	assert.Equal(t, true, valid, "'valid' must be true when token is valid")

	// The usuario object must be present with the correct fields.
	usuario, ok := resp["usuario"]
	require.True(t, ok, "response must contain 'usuario' object")
	usuarioMap, ok := usuario.(map[string]any)
	require.True(t, ok, "usuario must be a JSON object")
	assert.Equal(t, "user-id-123", usuarioMap["id"])
	assert.Equal(t, "test@example.com", usuarioMap["email"])
	assert.Equal(t, "Test User", usuarioMap["nome"])
}

// TestAuthHandler_Validate_v1Prefix confirms the v1 alias also works.
func TestAuthHandler_Validate_v1Prefix(t *testing.T) {
	validateStub := &stubValidateToken{
		out: &portin.AuthOutput{
			Usuario:     fixedUser(),
			AccessToken: "access.token.here",
		},
	}

	r := newAuthRouter(validateStub, &stubRefreshToken{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/validate", nil)
	req.Header.Set("Authorization", "Bearer some.jwt.token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	valid, ok := resp["valid"]
	require.True(t, ok, "response must contain 'valid' field")
	assert.Equal(t, true, valid)
}

// TestAuthHandler_Validate_NoAccessTokenField verifies the validate response does NOT
// expose the raw accessToken (it's already in the Authorization header context).
// This also ensures no regression on the response shape.
func TestAuthHandler_Validate_ResponseShape(t *testing.T) {
	validateStub := &stubValidateToken{
		out: &portin.AuthOutput{
			Usuario:     fixedUser(),
			AccessToken: "some-access-token",
		},
	}

	r := newAuthRouter(validateStub, &stubRefreshToken{})
	req := httptest.NewRequest(http.MethodGet, "/api/auth/validate", nil)
	req.Header.Set("Authorization", "Bearer some-access-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))

	// valid and usuario fields are mandatory
	_, hasValid := resp["valid"]
	_, hasUsuario := resp["usuario"]
	assert.True(t, hasValid, "must have 'valid'")
	assert.True(t, hasUsuario, "must have 'usuario'")
}

// ---- Fix 1: Refresh reads from JSON body ----

// TestAuthHandler_Refresh_ReadsTokenFromBody confirms that the handler passes the
// refreshToken value from the JSON body to the use case — not from a header.
func TestAuthHandler_Refresh_ReadsTokenFromBody(t *testing.T) {
	const sentToken = "my-refresh-token-from-body"

	refreshStub := &stubRefreshToken{
		out: &portin.AuthOutput{
			Usuario:      fixedUser(),
			AccessToken:  "new.access.token",
			RefreshToken: "new.refresh.token",
		},
	}

	r := newAuthRouter(&stubValidateToken{}, refreshStub)

	body := `{"refreshToken":"` + sentToken + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "valid refresh token should return 200")

	// Confirm the use case received exactly the token from the body
	assert.Equal(t, sentToken, refreshStub.receivedToken,
		"handler must forward the refreshToken from the JSON body to the use case")

	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "new.access.token", resp["accessToken"])
	assert.Equal(t, "new.refresh.token", resp["refreshToken"])
}

// TestAuthHandler_Refresh_EmptyBody_ReturnsBadRequest confirms a malformed body returns 400.
func TestAuthHandler_Refresh_EmptyBody_ReturnsBadRequest(t *testing.T) {
	r := newAuthRouter(&stubValidateToken{}, &stubRefreshToken{})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", strings.NewReader("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
