package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/middleware"
	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/mongodb"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ApprovalsHandler handles /api/v1/approvals/* endpoints,
// providing content parity with Java's ApprovalController.
type ApprovalsHandler struct {
	mongo *mongodb.Client
}

// NewApprovalsHandler creates a new ApprovalsHandler.
func NewApprovalsHandler(mongo *mongodb.Client) *ApprovalsHandler {
	return &ApprovalsHandler{mongo: mongo}
}

// RegisterApprovalsRoutes registers approvals routes.
func (h *ApprovalsHandler) RegisterApprovalsRoutes(r chi.Router) {
	r.Get("/api/v1/approvals/count", h.ContarPendentes)
}

// ContarPendentes handles GET /api/v1/approvals/count
// Matches Java's ApprovalController.contarPendentes — returns {"pendingCount": long}.
// approver_id in MongoDB is stored as UUID Binary subtype 4 (Java schema).
func (h *ApprovalsHandler) ContarPendentes(w http.ResponseWriter, r *http.Request) {
	// Prefer X-User-Id header (BFF forwards it, matching Java's @RequestHeader("X-User-Id"))
	userId := r.Header.Get("X-User-Id")
	if userId == "" {
		userId = r.Header.Get("x-user-id")
	}
	// Fallback: extract from JWT context (for direct calls without BFF)
	if userId == "" {
		userId = middleware.UserIDFromContext(r.Context())
	}
	if userId == "" {
		writeError(w, apierr.Unauthorized("usuário não autenticado"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	col := h.mongo.Collection("approval_items")
	// approver_id is stored as UUID Binary — must convert before querying
	count, err := col.CountDocuments(ctx, bson.M{
		"approver_id": uuidStringToBinary(userId),
		"status":      "PENDING",
	})
	if err != nil {
		// On error return 0 — matches Java's defensive behavior
		writeJSON(w, http.StatusOK, map[string]int64{"pendingCount": 0})
		return
	}

	writeJSON(w, http.StatusOK, map[string]int64{"pendingCount": count})
}
