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
	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/middleware"
)

func TestOutboundRequestHandler_MyRequests_NoAuth_Returns401(t *testing.T) {
	// No JWT context → should return 401 Unauthorized
	h := handler.NewOutboundRequestHandler(nil)
	r := chi.NewRouter()
	h.RegisterOutboundRequestRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/outbound-requests/my-requests", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestOutboundRequestHandler_MyRequests_WithAuth_NilMongo_ReturnsEmptyArray(t *testing.T) {
	// Authenticated user, nil mongo → should return 200 with empty JSON array
	h := handler.NewOutboundRequestHandler(nil)
	r := chi.NewRouter()
	// Inject a user ID into context to simulate JWT auth
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := middleware.ContextWithUserID(r.Context(), "user-test-123")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	h.RegisterOutboundRequestRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/outbound-requests/my-requests", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var result []interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
	assert.NotNil(t, result)
	assert.Empty(t, result)
}
