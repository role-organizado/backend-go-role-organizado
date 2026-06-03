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

// ---- stubs for UsuarioHandler ----

type stubGetUsuario struct {
	out *domain.Usuario
	err error
}

func (s *stubGetUsuario) Execute(_ context.Context, _ string) (*domain.Usuario, error) {
	return s.out, s.err
}

type stubUpdateUsuario struct {
	out *domain.Usuario
	err error
}

func (s *stubUpdateUsuario) Execute(_ context.Context, _ string, _ portin.UpdateUsuarioInput) (*domain.Usuario, error) {
	return s.out, s.err
}

type stubListUsuarios struct{}

func (s *stubListUsuarios) Execute(_ context.Context, _, _ int) ([]domain.Usuario, int64, error) {
	return nil, 0, nil
}

type stubUpdateRole struct{}

func (s *stubUpdateRole) Execute(_ context.Context, _ portin.UpdateUserRoleInput) (*domain.Usuario, error) {
	return nil, nil
}

func newUsuarioRouter() (*chi.Mux, *stubUpdateUsuario) {
	updateStub := &stubUpdateUsuario{
		out: &domain.Usuario{
			ID:        "user-abc",
			Nome:      "Updated Name",
			Email:     "user@example.com",
			Roles:     []domain.Role{domain.RoleUser},
			Ativo:     true,
			CriadoEm: time.Now(),
		},
	}
	h := handler.NewUsuarioHandler(
		&stubGetUsuario{out: &domain.Usuario{ID: "user-abc", CriadoEm: time.Now()}},
		updateStub,
		&stubListUsuarios{},
		&stubUpdateRole{},
	)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r, updateStub
}

// ---- Fix 3: PATCH /api/usuarios/{id} must not return 405 ----

// TestUsuarioHandler_PatchRoute_Returns200 is the direct regression test for bug 1.3.
// Before the fix, PATCH /api/usuarios/{id} returned 405 Method Not Allowed because only
// PUT was registered. After the fix, PATCH must return 200 OK.
func TestUsuarioHandler_PatchRoute_Returns200(t *testing.T) {
	r, _ := newUsuarioRouter()

	body := `{"nome":"Novo Nome"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/usuarios/user-abc", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code,
		"PATCH /api/usuarios/{id} should return 200 — 405 means the PATCH route is not registered")
}

// TestUsuarioHandler_PatchRoute_NotReturning405 is an alias assertion for clarity.
func TestUsuarioHandler_PatchRoute_NotReturning405(t *testing.T) {
	r, _ := newUsuarioRouter()

	req := httptest.NewRequest(http.MethodPatch, "/api/usuarios/user-abc",
		strings.NewReader(`{"nome":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.NotEqual(t, http.StatusMethodNotAllowed, w.Code,
		"PATCH route must be registered; 405 means only PUT is registered")
}

// TestUsuarioHandler_PutRoute_StillWorks confirms the existing PUT route is unaffected.
func TestUsuarioHandler_PutRoute_StillWorks(t *testing.T) {
	r, _ := newUsuarioRouter()

	req := httptest.NewRequest(http.MethodPut, "/api/usuarios/user-abc",
		strings.NewReader(`{"nome":"updated"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "existing PUT route must still work")
}

// TestUsuarioHandler_PatchRoute_ReturnsUpdatedUsuario verifies response body shape.
func TestUsuarioHandler_PatchRoute_ReturnsUpdatedUsuario(t *testing.T) {
	r, _ := newUsuarioRouter()

	body := `{"nome":"Novo Nome Atualizado"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/usuarios/user-abc", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "user-abc", resp["id"], "response must include user ID")
	_, hasEmail := resp["email"]
	assert.True(t, hasEmail, "response must include email field")
}
