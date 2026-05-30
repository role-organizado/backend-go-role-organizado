package handler_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/handler"
)

func TestWorkflowProxy_ParticipantStatus(t *testing.T) {
	// Java backend stub
	javaBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/workflows/participant/p123/status", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"RUNNING"}`)) //nolint:errcheck
	}))
	defer javaBackend.Close()

	h := handler.NewWorkflowProxyHandler(javaBackend.URL)
	r := chi.NewRouter()
	h.RegisterWorkflowRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/participant/p123/status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "RUNNING")
}

func TestWorkflowProxy_InviteStatus(t *testing.T) {
	javaBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/workflows/invite/inv456/status", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"COMPLETED"}`)) //nolint:errcheck
	}))
	defer javaBackend.Close()

	h := handler.NewWorkflowProxyHandler(javaBackend.URL)
	r := chi.NewRouter()
	h.RegisterWorkflowRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/invite/inv456/status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "COMPLETED")
}

func TestWorkflowProxy_ReconciliationTrigger(t *testing.T) {
	javaBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v1/workflows/reconciliation/psp/trigger", r.URL.Path)
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{"workflowId":"wf-001"}`)) //nolint:errcheck
	}))
	defer javaBackend.Close()

	h := handler.NewWorkflowProxyHandler(javaBackend.URL)
	r := chi.NewRouter()
	h.RegisterWorkflowRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/reconciliation/psp/trigger",
		strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
}

func TestWorkflowProxy_AccountingSnapshotTrigger(t *testing.T) {
	javaBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v1/workflows/accounting/snapshot/trigger", r.URL.Path)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer javaBackend.Close()

	h := handler.NewWorkflowProxyHandler(javaBackend.URL)
	r := chi.NewRouter()
	h.RegisterWorkflowRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/accounting/snapshot/trigger",
		strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
}

func TestWorkflowProxy_ForwardsAuthorizationHeader(t *testing.T) {
	var receivedAuth string
	javaBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer javaBackend.Close()

	h := handler.NewWorkflowProxyHandler(javaBackend.URL)
	r := chi.NewRouter()
	h.RegisterWorkflowRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/participant/p1/status", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, "Bearer test-token", receivedAuth)
}

func TestWorkflowProxy_BackendUnavailable_Returns502(t *testing.T) {
	h := handler.NewWorkflowProxyHandler("http://localhost:19999") // nothing listening
	r := chi.NewRouter()
	h.RegisterWorkflowRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/participant/p1/status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)
}
