package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.temporal.io/sdk/client"

	temporalworkflow "github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/workflow"
)

// WorkflowNativeHandler serves Temporal workflow status/control requests for the
// workflows that have been migrated to native Go (Onda 3/4) by talking to the
// Temporal frontend directly via the Go SDK (DescribeWorkflowExecution,
// QueryWorkflow, SignalWorkflow).
//
// For not-yet-migrated control routes (PSP reconciliation, accounting snapshot),
// and whenever the native lookup is unavailable (no Temporal client, or the
// workflow execution cannot be described), it falls back to the Java proxy.
type WorkflowNativeHandler struct {
	// client is the Temporal client; nil when TEMPORAL_WORKER_ENABLED=false, in
	// which case every route falls back to the Java proxy.
	client client.Client
	// fallback proxies to the Java backend.
	fallback *WorkflowProxyHandler
}

// NewWorkflowNativeHandler creates a new WorkflowNativeHandler. A nil client is
// valid and routes all requests to the fallback proxy.
func NewWorkflowNativeHandler(c client.Client, fallback *WorkflowProxyHandler) *WorkflowNativeHandler {
	return &WorkflowNativeHandler{client: c, fallback: fallback}
}

// nativeStatusResponse is the JSON shape returned by the native status endpoints.
type nativeStatusResponse struct {
	WorkflowID string `json:"workflowId"`
	// ExecutionStatus is the Temporal-side execution status (RUNNING, COMPLETED…).
	ExecutionStatus string `json:"executionStatus"`
	// WorkflowStatus is the workflow's own getWorkflowStatus query result, when available.
	WorkflowStatus string `json:"workflowStatus,omitempty"`
	// Source is always "native" for these responses, distinguishing them from proxied ones.
	Source string `json:"source"`
}

// RegisterWorkflowRoutes registers the workflow status/control routes, serving the
// 6 migrated workflows natively and proxying the rest to the Java backend.
func (h *WorkflowNativeHandler) RegisterWorkflowRoutes(r chi.Router) {
	r.Route("/api/v1/workflows", func(r chi.Router) {
		// ── Natively-served (Go SDK) workflow status ──────────────────────────
		r.Get("/participant/{participantId}/status", h.participantStatus)
		r.Get("/invite/{approvalId}/status", h.inviteStatus)
		r.Get("/outbound/{outboundRequestId}/status", h.outboundStatus)
		r.Get("/event/{eventoId}/lifecycle/status", h.eventLifecycleStatus)
		r.Get("/event-publication/{draftId}/status", h.eventPublicationStatus)
		r.Get("/event-publication/monitoring/status", h.eventPublicationMonitoringStatus)

		// Generic native signal dispatch for the migrated workflows.
		r.Post("/{workflowId}/signal/{signalName}", h.signal)

		// ── Java proxy fallback for not-yet-migrated control routes ───────────
		r.Post("/reconciliation/psp/trigger", h.fallback.proxy)
		r.Get("/reconciliation/psp/status", h.fallback.proxy)
		r.Post("/reconciliation/psp/pause", h.fallback.proxy)
		r.Post("/reconciliation/psp/resume", h.fallback.proxy)
		r.Post("/reconciliation/psp/cancel", h.fallback.proxy)
		r.Post("/accounting/snapshot/trigger", h.fallback.proxy)
		r.Get("/accounting/snapshot/status", h.fallback.proxy)
	})
}

// ─── Status handlers ────────────────────────────────────────────────────────

func (h *WorkflowNativeHandler) participantStatus(w http.ResponseWriter, r *http.Request) {
	participantID := chi.URLParam(r, "participantId")
	// The participant-lifecycle workflow ID is keyed on (eventId, participantId).
	// The eventId is supplied as a query parameter; without it we cannot build the
	// canonical ID, so we defer to the Java proxy.
	eventID := r.URL.Query().Get("eventId")
	if eventID == "" {
		h.fallback.proxy(w, r)
		return
	}
	h.nativeStatus(w, r, temporalworkflow.ParticipantLifecyclePrimaryID(eventID, participantID))
}

func (h *WorkflowNativeHandler) inviteStatus(w http.ResponseWriter, r *http.Request) {
	h.nativeStatus(w, r, temporalworkflow.InviteLifecyclePrimaryID(chi.URLParam(r, "approvalId")))
}

func (h *WorkflowNativeHandler) outboundStatus(w http.ResponseWriter, r *http.Request) {
	h.nativeStatus(w, r, temporalworkflow.OutboundPrimaryID(chi.URLParam(r, "outboundRequestId")))
}

func (h *WorkflowNativeHandler) eventLifecycleStatus(w http.ResponseWriter, r *http.Request) {
	h.nativeStatus(w, r, temporalworkflow.EventLifecyclePrimaryID(chi.URLParam(r, "eventoId")))
}

func (h *WorkflowNativeHandler) eventPublicationStatus(w http.ResponseWriter, r *http.Request) {
	h.nativeStatus(w, r, temporalworkflow.EventPublicationPrimaryID(chi.URLParam(r, "draftId")))
}

func (h *WorkflowNativeHandler) eventPublicationMonitoringStatus(w http.ResponseWriter, r *http.Request) {
	h.nativeStatus(w, r, temporalworkflow.EventPublicationMonitoringPrimaryID())
}

// nativeStatus describes the workflow execution via the Go SDK and returns its
// status, falling back to the Java proxy when the lookup is not possible.
func (h *WorkflowNativeHandler) nativeStatus(w http.ResponseWriter, r *http.Request, workflowID string) {
	if h.client == nil {
		h.fallback.proxy(w, r)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	desc, err := h.client.DescribeWorkflowExecution(ctx, workflowID, "")
	if err != nil {
		// Execution not found / Temporal unavailable — defer to Java.
		slog.DebugContext(ctx, "workflow native describe failed, falling back to proxy",
			"workflowId", workflowID, "error", err)
		h.fallback.proxy(w, r)
		return
	}

	resp := nativeStatusResponse{
		WorkflowID:      workflowID,
		ExecutionStatus: desc.GetWorkflowExecutionInfo().GetStatus().String(),
		Source:          "native",
	}

	// Best-effort: query the workflow's own status string.
	if qv, qerr := h.client.QueryWorkflow(ctx, workflowID, "", "getWorkflowStatus"); qerr == nil {
		var s string
		if getErr := qv.Get(&s); getErr == nil {
			resp.WorkflowStatus = s
		}
	}

	writeNativeJSON(w, http.StatusOK, resp)
}

// signal dispatches a signal to a migrated workflow via the Go SDK. The request
// body, if any, is forwarded verbatim as the signal payload.
func (h *WorkflowNativeHandler) signal(w http.ResponseWriter, r *http.Request) {
	if h.client == nil {
		h.fallback.proxy(w, r)
		return
	}

	workflowID := chi.URLParam(r, "workflowId")
	signalName := chi.URLParam(r, "signalName")

	var payload json.RawMessage
	if r.Body != nil {
		// Ignore decode errors — an empty/absent body is a valid (nil-payload) signal.
		_ = json.NewDecoder(r.Body).Decode(&payload)
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := h.client.SignalWorkflow(ctx, workflowID, "", signalName, payload); err != nil {
		slog.WarnContext(ctx, "workflow native signal failed, falling back to proxy",
			"workflowId", workflowID, "signal", signalName, "error", err)
		h.fallback.proxy(w, r)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func writeNativeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
