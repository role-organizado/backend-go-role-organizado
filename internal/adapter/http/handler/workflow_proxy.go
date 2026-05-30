package handler

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// WorkflowProxyHandler proxies Temporal workflow status/control requests to the Java backend.
// This implements the BFF Temporal Workflow Routes pattern (E1-E4) described in agents.md.
type WorkflowProxyHandler struct {
	javaBackendURL string
	httpClient     *http.Client
}

// NewWorkflowProxyHandler creates a new WorkflowProxyHandler.
// javaBackendURL is the base URL of the Java Spring Boot backend (e.g. http://localhost:8080).
func NewWorkflowProxyHandler(javaBackendURL string) *WorkflowProxyHandler {
	return &WorkflowProxyHandler{
		javaBackendURL: strings.TrimRight(javaBackendURL, "/"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// RegisterWorkflowRoutes registers Temporal workflow proxy routes.
// These routes map 1:1 to the Java backend endpoints.
func (h *WorkflowProxyHandler) RegisterWorkflowRoutes(r chi.Router) {
	r.Route("/api/v1/workflows", func(r chi.Router) {
		// E1 — Participant lifecycle workflow status
		r.Get("/participant/{participantId}/status", h.proxy)

		// E2 — Invite lifecycle workflow status
		r.Get("/invite/{approvalId}/status", h.proxy)

		// E3 — PSP Reconciliation workflow control
		r.Post("/reconciliation/psp/trigger", h.proxy)
		r.Get("/reconciliation/psp/status", h.proxy)
		r.Post("/reconciliation/psp/pause", h.proxy)
		r.Post("/reconciliation/psp/resume", h.proxy)
		r.Post("/reconciliation/psp/cancel", h.proxy)

		// E4 — Accounting snapshot workflow
		r.Post("/accounting/snapshot/trigger", h.proxy)
		r.Get("/accounting/snapshot/status", h.proxy)
	})
}

// proxy forwards the request to the Java backend preserving path, query, headers, and body.
func (h *WorkflowProxyHandler) proxy(w http.ResponseWriter, r *http.Request) {
	targetURL, err := url.Parse(fmt.Sprintf("%s%s", h.javaBackendURL, r.RequestURI))
	if err != nil {
		http.Error(w, "invalid target URL", http.StatusInternalServerError)
		return
	}

	proxyReq, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL.String(), r.Body)
	if err != nil {
		http.Error(w, "failed to create proxy request", http.StatusInternalServerError)
		return
	}

	// Forward relevant headers
	for key, values := range r.Header {
		switch strings.ToLower(key) {
		case "authorization", "content-type", "accept", "x-correlation-id", "x-request-id":
			for _, v := range values {
				proxyReq.Header.Add(key, v)
			}
		}
	}

	resp, err := h.httpClient.Do(proxyReq)
	if err != nil {
		slog.Error("workflow proxy request failed", "url", targetURL.String(), "error", err)
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Forward response headers
	for key, values := range resp.Header {
		for _, v := range values {
			w.Header().Add(key, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body) //nolint:errcheck
}
