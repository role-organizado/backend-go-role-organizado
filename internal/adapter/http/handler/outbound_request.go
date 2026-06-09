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

// OutboundRequestHandler handles /api/v1/outbound-requests/* endpoints.
// Provides parity with Java's OutboundRequestController.
type OutboundRequestHandler struct {
	mongo *mongodb.Client
}

// NewOutboundRequestHandler creates a new OutboundRequestHandler.
func NewOutboundRequestHandler(mongo *mongodb.Client) *OutboundRequestHandler {
	return &OutboundRequestHandler{mongo: mongo}
}

// RegisterOutboundRequestRoutes registers outbound request routes (protected by JWT).
func (h *OutboundRequestHandler) RegisterOutboundRequestRoutes(r chi.Router) {
	r.Get("/api/v1/outbound-requests/my-requests", h.MyRequests)
}

// MyRequests handles GET /api/v1/outbound-requests/my-requests.
// Returns all outbound requests (reimbursements/supplier payments) belonging to the
// authenticated user, filtered by usuario_id from the JWT context.
// Returns an empty array if the collection is empty or does not exist.
func (h *OutboundRequestHandler) MyRequests(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("usuário não autenticado"))
		return
	}

	// Graceful degradation: if mongo is not wired (e.g. in tests), return empty array.
	if h.mongo == nil {
		writeJSON(w, http.StatusOK, []bson.M{})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	col := h.mongo.Collection("outbound_requests")
	cursor, err := col.Find(ctx, bson.M{"usuario_id": userID})
	if err != nil {
		// Collection may not exist yet — return empty array rather than 500.
		writeJSON(w, http.StatusOK, []bson.M{})
		return
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err := cursor.All(ctx, &results); err != nil || results == nil {
		results = []bson.M{}
	}
	writeJSON(w, http.StatusOK, results)
}
