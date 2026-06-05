package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/handler"
)

// newTestUsuarioHandler creates a minimal UsuarioHandler suitable for tests.
// Use-case ports are nil — only the routes that do NOT call use cases should be tested.
func newTestUsuarioHandler() *handler.UsuarioHandler {
	return handler.NewUsuarioHandler(nil, nil, nil, nil, nil)
}

func TestUsuarioHandler_GetDevices_NilMongo_ReturnsEmptyArray(t *testing.T) {
	// nil mongo → handler should return 200 with empty JSON array
	h := newTestUsuarioHandler()
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/usuarios/user-123/devices", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var result []interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
	assert.NotNil(t, result)
	assert.Empty(t, result)
}

func TestUsuarioHandler_GetDevices_RouteIsRegistered(t *testing.T) {
	// Verify the route is registered (vs returning 404)
	h := newTestUsuarioHandler()
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/usuarios/abc/devices", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should NOT be 404 — the route must exist
	assert.NotEqual(t, http.StatusNotFound, w.Code)
}
