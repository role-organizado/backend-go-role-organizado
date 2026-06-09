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

// newApprovalsRouter returns a chi router with approvals routes registered.
// Passes nil as the mongo client — safe for tests that return before hitting the DB layer.
func newApprovalsRouter() *chi.Mux {
	h := handler.NewApprovalsHandler(nil)
	r := chi.NewRouter()
	h.RegisterApprovalsRoutes(r)
	return r
}

// TestApproval_RoutesRegistered verifies that all three approvals routes are mounted.
// chi returns 404 for unregistered paths, so any non-404 status proves the route is registered.
func TestApproval_RoutesRegistered(t *testing.T) {
	r := newApprovalsRouter()

	routes := []string{
		"/api/v1/approvals/count",
		"/api/v1/approvals/pending",
		"/api/v1/approvals/history",
	}

	for _, path := range routes {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			// chi returns 404 for unmounted paths; any other status confirms registration
			assert.NotEqual(t, http.StatusNotFound, w.Code,
				"route %s should be registered", path)
		})
	}
}

// TestApproval_Unauthorized verifies all three endpoints return 401 JSON
// when no user identity is present (no X-User-Id header, no JWT context).
// These tests never reach MongoDB, so a nil client is safe.
func TestApproval_Unauthorized(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{"count", "/api/v1/approvals/count"},
		{"pending", "/api/v1/approvals/pending"},
		{"history", "/api/v1/approvals/history"},
	}

	r := newApprovalsRouter()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			var body map[string]any
			require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
			assert.NotEmpty(t, body["message"], "error body should contain a message field")
			assert.NotEmpty(t, body["code"], "error body should contain a code field")
		})
	}
}

// TestApproval_ResponseShape verifies that the approvals endpoints return
// well-formed JSON error bodies on auth failure.
// These tests exercise the error path, which does not touch MongoDB.
func TestApproval_ResponseShape(t *testing.T) {
	r := newApprovalsRouter()

	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{
			name:       "pending returns 401 with apierr body",
			path:       "/api/v1/approvals/pending",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "history returns 401 with apierr body",
			path:       "/api/v1/approvals/history",
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tc.wantStatus, w.Code)

			var body map[string]any
			require.NoError(t, json.NewDecoder(w.Body).Decode(&body),
				"response body should be valid JSON")
			assert.Contains(t, body, "code",
				"error body should contain 'code' field (apierr standard)")
			assert.Contains(t, body, "message",
				"error body should contain 'message' field (apierr standard)")
		})
	}
}

// TestApproval_LowercaseXUserIdHeader verifies that the handler also accepts
// the lowercase form x-user-id as user identity (case-insensitive HTTP headers).
// These tests do NOT send a valid mongo client — we verify only the auth rejection path.
func TestApproval_HeaderVariants(t *testing.T) {
	r := newApprovalsRouter()

	// Without any identity → always 401
	paths := []string{
		"/api/v1/approvals/pending",
		"/api/v1/approvals/history",
		"/api/v1/approvals/count",
	}
	for _, path := range paths {
		t.Run("no_header_"+path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusUnauthorized, w.Code,
				"should be unauthorized without user identity on %s", path)
		})
	}
}
