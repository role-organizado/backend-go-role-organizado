package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/middleware"
	"github.com/role-organizado/backend-go-role-organizado/internal/domain/admin"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ApprovalsHandler handles /api/v1/approvals/* endpoints, providing content
// parity with Java's ApprovalController. Fully hexagonal — all reads go through
// use-case ports (no direct MongoDB access).
type ApprovalsHandler struct {
	countUC   portin.CountPendingApprovalsUseCase
	pendingUC portin.ListPendingApprovalsUseCase
	historyUC portin.ListApprovalHistoryUseCase
}

// NewApprovalsHandler creates a new ApprovalsHandler.
func NewApprovalsHandler(
	count portin.CountPendingApprovalsUseCase,
	pending portin.ListPendingApprovalsUseCase,
	history portin.ListApprovalHistoryUseCase,
) *ApprovalsHandler {
	return &ApprovalsHandler{countUC: count, pendingUC: pending, historyUC: history}
}

// RegisterApprovalsRoutes registers approvals routes.
func (h *ApprovalsHandler) RegisterApprovalsRoutes(r chi.Router) {
	r.Get("/api/v1/approvals/count", h.ContarPendentes)
	r.Get("/api/v1/approvals/pending", h.GetPendingApprovals)
	r.Get("/api/v1/approvals/history", h.GetApprovalHistory)
}

// resolveUserID extracts the user ID from the request, preferring the X-User-Id
// header (forwarded by the BFF, matching Java's @RequestHeader("X-User-Id")) then
// the JWT context.
func (h *ApprovalsHandler) resolveUserID(r *http.Request) string {
	if id := r.Header.Get("X-User-Id"); id != "" {
		return id
	}
	if id := r.Header.Get("x-user-id"); id != "" {
		return id
	}
	return middleware.UserIDFromContext(r.Context())
}

// ContarPendentes handles GET /api/v1/approvals/count.
// Matches Java's ApprovalController.contarPendentes — returns {"pendingCount": long}.
func (h *ApprovalsHandler) ContarPendentes(w http.ResponseWriter, r *http.Request) {
	userID := h.resolveUserID(r)
	if userID == "" {
		writeError(w, apierr.Unauthorized("usuário não autenticado"))
		return
	}
	count, err := h.countUC.Execute(r.Context(), userID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"pendingCount": count})
}

// GetPendingApprovals handles GET /api/v1/approvals/pending.
func (h *ApprovalsHandler) GetPendingApprovals(w http.ResponseWriter, r *http.Request) {
	userID := h.resolveUserID(r)
	if userID == "" {
		writeError(w, apierr.Unauthorized("usuário não autenticado"))
		return
	}
	items, err := h.pendingUC.Execute(r.Context(), userID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toApprovalResponses(items))
}

// GetApprovalHistory handles GET /api/v1/approvals/history.
func (h *ApprovalsHandler) GetApprovalHistory(w http.ResponseWriter, r *http.Request) {
	userID := h.resolveUserID(r)
	if userID == "" {
		writeError(w, apierr.Unauthorized("usuário não autenticado"))
		return
	}
	items, err := h.historyUC.Execute(r.Context(), userID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toApprovalResponses(items))
}

// toApprovalResponses maps approval read models to the API response shape.
// Field names match Java's ApprovalItemDTO.
func toApprovalResponses(items []admin.ApprovalItem) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{
			"id":            item.ID,
			"tipo":          item.Tipo,
			"eventoId":      item.EventoID,
			"solicitanteId": item.SolicitanteID,
			"status":        item.Status,
			"criadoEm":      item.CriadoEm,
		})
	}
	return result
}
