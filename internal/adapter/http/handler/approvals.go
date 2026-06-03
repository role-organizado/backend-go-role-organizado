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
	r.Get("/api/v1/approvals/pending", h.GetPendingApprovals)
	r.Get("/api/v1/approvals/history", h.GetApprovalHistory)
}

// resolveUserID extracts the user ID from the request, preferring the X-User-Id header
// (forwarded by the BFF, matching Java's @RequestHeader("X-User-Id")) then the JWT context.
func (h *ApprovalsHandler) resolveUserID(r *http.Request) string {
	if id := r.Header.Get("X-User-Id"); id != "" {
		return id
	}
	if id := r.Header.Get("x-user-id"); id != "" {
		return id
	}
	return middleware.UserIDFromContext(r.Context())
}

// ContarPendentes handles GET /api/v1/approvals/count
// Matches Java's ApprovalController.contarPendentes — returns {"pendingCount": long}.
// approver_id in MongoDB is stored as UUID Binary subtype 4 (Java schema).
func (h *ApprovalsHandler) ContarPendentes(w http.ResponseWriter, r *http.Request) {
	userId := h.resolveUserID(r)
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

// GetPendingApprovals handles GET /api/v1/approvals/pending
// Returns approval items with status=PENDING for the authenticated user (as approver).
// Matches Java's ApprovalController.getPendingApprovals.
func (h *ApprovalsHandler) GetPendingApprovals(w http.ResponseWriter, r *http.Request) {
	userId := h.resolveUserID(r)
	if userId == "" {
		writeError(w, apierr.Unauthorized("usuário não autenticado"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	col := h.mongo.Collection("approval_items")
	cursor, err := col.Find(ctx, bson.M{
		"approver_id": uuidStringToBinary(userId),
		"status":      "PENDING",
	})
	if err != nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	defer cursor.Close(ctx)

	var items []bson.M
	if err := cursor.All(ctx, &items); err != nil || len(items) == 0 {
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	writeJSON(w, http.StatusOK, toApprovalResponses(items))
}

// GetApprovalHistory handles GET /api/v1/approvals/history
// Returns resolved/cancelled approval items for the authenticated user (as approver).
// Matches Java's ApprovalController.getApprovalHistory.
func (h *ApprovalsHandler) GetApprovalHistory(w http.ResponseWriter, r *http.Request) {
	userId := h.resolveUserID(r)
	if userId == "" {
		writeError(w, apierr.Unauthorized("usuário não autenticado"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	col := h.mongo.Collection("approval_items")
	cursor, err := col.Find(ctx, bson.M{
		"approver_id": uuidStringToBinary(userId),
		"status":      bson.M{"$in": bson.A{"APROVADO", "REJEITADO", "CANCELADO"}},
	})
	if err != nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	defer cursor.Close(ctx)

	var items []bson.M
	if err := cursor.All(ctx, &items); err != nil || len(items) == 0 {
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	writeJSON(w, http.StatusOK, toApprovalResponses(items))
}

// toApprovalResponses maps raw MongoDB approval_items documents to the API response shape.
// Field names match Java's ApprovalItemDTO: id, tipo, eventoId, solicitanteId, status, criadoEm.
func toApprovalResponses(items []bson.M) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		resp := map[string]any{
			"id":            binaryToUUIDString(item["_id"]),
			"tipo":          item["tipo"],
			"eventoId":      binaryToUUIDString(item["evento_id"]),
			"solicitanteId": binaryToUUIDString(item["solicitante_id"]),
			"status":        item["status"],
			"criadoEm":      item["criado_em"],
		}
		result = append(result, resp)
	}
	return result
}
