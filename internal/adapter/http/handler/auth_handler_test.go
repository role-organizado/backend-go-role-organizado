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
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/auth"
	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/handler"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ---- mocks for auth handler tests ----

type mockLoginUC struct{ mock.Mock }

func (m *mockLoginUC) Execute(ctx context.Context, in portin.LoginInput) (*portin.AuthOutput, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*portin.AuthOutput), args.Error(1)
}

type mockRegisterUC struct{ mock.Mock }

func (m *mockRegisterUC) Execute(ctx context.Context, in portin.RegisterInput) (*portin.AuthOutput, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*portin.AuthOutput), args.Error(1)
}

type mockRefreshUC struct{ mock.Mock }

func (m *mockRefreshUC) Execute(ctx context.Context, token string) (*portin.AuthOutput, error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*portin.AuthOutput), args.Error(1)
}

type mockValidateUC struct{ mock.Mock }

func (m *mockValidateUC) Execute(ctx context.Context, token string) (*portin.AuthOutput, error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*portin.AuthOutput), args.Error(1)
}

type mockLogoutUC struct{ mock.Mock }

func (m *mockLogoutUC) Execute(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

type mockGoogleUC struct{ mock.Mock }

func (m *mockGoogleUC) Execute(ctx context.Context, in portin.GoogleAuthInput) (*portin.AuthOutput, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*portin.AuthOutput), args.Error(1)
}

type mockAppleUC struct{ mock.Mock }

func (m *mockAppleUC) Execute(ctx context.Context, in portin.AppleAuthInput) (*portin.AuthOutput, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*portin.AuthOutput), args.Error(1)
}

// newTestAuthHandler creates a chi router with the AuthHandler mounted.
func newTestAuthRouter(
	loginUC portin.LoginUseCase,
	registerUC portin.RegisterUseCase,
	refreshUC portin.RefreshTokenUseCase,
	validateUC portin.ValidateTokenUseCase,
	logoutUC portin.LogoutUseCase,
) *chi.Mux {
	h := handler.NewAuthHandler(loginUC, registerUC, refreshUC, validateUC, logoutUC, &mockGoogleUC{}, &mockAppleUC{})
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r
}

// ---- Tests: GET /api/auth/validate ----

// TestValidate_ResponseContainsValidTrue ensures the validate endpoint
// returns { "valid": true, "usuario": {...} } as expected by the BFF and mobile apps.
func TestValidate_ResponseContainsValidTrue(t *testing.T) {
	validateUC := &mockValidateUC{}

	usuario := &domain.Usuario{
		ID:       "user-abc",
		Nome:     "Alice",
		Email:    "alice@example.com",
		Ativo:    true,
		CriadoEm: time.Now(),
		Roles:    []domain.Role{domain.RoleUser},
	}
	validateUC.On("Execute", mock.Anything, "some-jwt-token").Return(&portin.AuthOutput{
		Usuario:     usuario,
		AccessToken: "some-jwt-token",
	}, nil)

	r := newTestAuthRouter(&mockLoginUC{}, &mockRegisterUC{}, &mockRefreshUC{}, validateUC, &mockLogoutUC{})

	req := httptest.NewRequest(http.MethodGet, "/api/auth/validate", nil)
	req.Header.Set("Authorization", "Bearer some-jwt-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, true, body["valid"], "expected valid=true in validate response")
	assert.NotNil(t, body["usuario"], "expected usuario object in validate response")

	usuario_obj, ok := body["usuario"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "user-abc", usuario_obj["id"])
	assert.Equal(t, "alice@example.com", usuario_obj["email"])

	validateUC.AssertExpectations(t)
}

// TestValidate_LegacyPath ensures /api/v1/auth/validate also works.
func TestValidate_LegacyPath_ReturnsValidTrue(t *testing.T) {
	validateUC := &mockValidateUC{}
	usuario := &domain.Usuario{
		ID:       "u1",
		Nome:     "Bob",
		Email:    "bob@example.com",
		Ativo:    true,
		Roles:    []domain.Role{domain.RoleUser},
		CriadoEm: time.Now(),
	}
	validateUC.On("Execute", mock.Anything, "token-v1").Return(&portin.AuthOutput{
		Usuario:     usuario,
		AccessToken: "token-v1",
	}, nil)

	r := newTestAuthRouter(&mockLoginUC{}, &mockRegisterUC{}, &mockRefreshUC{}, validateUC, &mockLogoutUC{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/validate", nil)
	req.Header.Set("Authorization", "Bearer token-v1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, true, body["valid"])
}

// TestValidate_InvalidToken_Returns401 ensures that an invalid token yields 401.
func TestValidate_InvalidToken_Returns401(t *testing.T) {
	validateUC := &mockValidateUC{}
	validateUC.On("Execute", mock.Anything, "bad-token").Return(nil, apierr.Unauthorized("token inválido"))

	r := newTestAuthRouter(&mockLoginUC{}, &mockRegisterUC{}, &mockRefreshUC{}, validateUC, &mockLogoutUC{})

	req := httptest.NewRequest(http.MethodGet, "/api/auth/validate", nil)
	req.Header.Set("Authorization", "Bearer bad-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// ---- Tests: POST /api/auth/refresh ----

// TestRefresh_ReadsTokenFromBody ensures the refresh handler reads the token
// from the JSON body (not from a header), matching the client contract.
func TestRefresh_ReadsTokenFromBody_Returns200(t *testing.T) {
	refreshUC := &mockRefreshUC{}
	usuario := &domain.Usuario{
		ID:       "user-xyz",
		Nome:     "Carlos",
		Email:    "carlos@example.com",
		Ativo:    true,
		Roles:    []domain.Role{domain.RoleUser},
		CriadoEm: time.Now(),
	}
	refreshUC.On("Execute", mock.Anything, "my-refresh-token").Return(&portin.AuthOutput{
		Usuario:      usuario,
		AccessToken:  "new-access-token",
		RefreshToken: "new-refresh-token",
	}, nil)

	r := newTestAuthRouter(&mockLoginUC{}, &mockRegisterUC{}, refreshUC, &mockValidateUC{}, &mockLogoutUC{})

	body := strings.NewReader(`{"refreshToken":"my-refresh-token"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "new-access-token", resp["accessToken"])
	assert.Equal(t, "new-refresh-token", resp["refreshToken"])

	refreshUC.AssertExpectations(t)
}

// TestRefresh_InvalidToken_Returns401 ensures a bad refresh token yields 401.
func TestRefresh_InvalidToken_Returns401(t *testing.T) {
	refreshUC := &mockRefreshUC{}
	refreshUC.On("Execute", mock.Anything, "invalid-token").Return(nil, apierr.Unauthorized("refresh token inválido"))

	r := newTestAuthRouter(&mockLoginUC{}, &mockRegisterUC{}, refreshUC, &mockValidateUC{}, &mockLogoutUC{})

	body := strings.NewReader(`{"refreshToken":"invalid-token"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// ---- Tests: mocks for usuario handler ----

type mockGetUsuarioUC struct{ mock.Mock }

func (m *mockGetUsuarioUC) Execute(ctx context.Context, id string) (*domain.Usuario, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Usuario), args.Error(1)
}

type mockUpdateUsuarioUC struct{ mock.Mock }

func (m *mockUpdateUsuarioUC) Execute(ctx context.Context, id string, in portin.UpdateUsuarioInput) (*domain.Usuario, error) {
	args := m.Called(ctx, id, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Usuario), args.Error(1)
}

type mockListUsuariosUC struct{ mock.Mock }

func (m *mockListUsuariosUC) Execute(ctx context.Context, page, size int) ([]domain.Usuario, int64, error) {
	args := m.Called(ctx, page, size)
	return args.Get(0).([]domain.Usuario), args.Get(1).(int64), args.Error(2)
}

type mockUpdateRoleUC struct{ mock.Mock }

func (m *mockUpdateRoleUC) Execute(ctx context.Context, in portin.UpdateUserRoleInput) (*domain.Usuario, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Usuario), args.Error(1)
}

// ---- Tests: PATCH /api/usuarios/{id} ----

// TestUsuarioHandler_PATCH_RouteRegistered ensures the PATCH method is registered
// for /api/usuarios/{id} and does not return 405 Method Not Allowed.
func TestUsuarioHandler_PATCH_RouteRegistered(t *testing.T) {
	getUC := &mockGetUsuarioUC{}
	updateUC := &mockUpdateUsuarioUC{}
	listUC := &mockListUsuariosUC{}
	roleUC := &mockUpdateRoleUC{}

	updated := &domain.Usuario{
		ID:       "u1",
		Nome:     "Novo Nome",
		Email:    "u@example.com",
		Ativo:    true,
		Roles:    []domain.Role{domain.RoleUser},
		CriadoEm: time.Now(),
	}
	updateUC.On("Execute", mock.Anything, "u1", mock.Anything).Return(updated, nil)

	h := handler.NewUsuarioHandler(getUC, updateUC, listUC, roleUC, nil)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	body := strings.NewReader(`{"nome":"Novo Nome"}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/usuarios/u1", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.NotEqual(t, http.StatusMethodNotAllowed, w.Code, "PATCH /api/usuarios/{id} must not return 405")
	assert.Equal(t, http.StatusOK, w.Code)
	updateUC.AssertExpectations(t)
}

// TestUsuarioHandler_PUT_StillWorks ensures the existing PUT route still works after adding PATCH.
func TestUsuarioHandler_PUT_StillWorks(t *testing.T) {
	getUC := &mockGetUsuarioUC{}
	updateUC := &mockUpdateUsuarioUC{}
	listUC := &mockListUsuariosUC{}
	roleUC := &mockUpdateRoleUC{}

	updated := &domain.Usuario{
		ID:       "u2",
		Nome:     "Updated",
		Email:    "u2@example.com",
		Ativo:    true,
		Roles:    []domain.Role{domain.RoleUser},
		CriadoEm: time.Now(),
	}
	updateUC.On("Execute", mock.Anything, "u2", mock.Anything).Return(updated, nil)

	h := handler.NewUsuarioHandler(getUC, updateUC, listUC, roleUC, nil)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	body := strings.NewReader(`{"nome":"Updated"}`)
	req := httptest.NewRequest(http.MethodPut, "/api/usuarios/u2", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
