package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/mongodb"
)

// CardapioHandler handles GET /api/cardapios.
// Provides parity with Java's CardapioController.
type CardapioHandler struct {
	mongo *mongodb.Client
}

// NewCardapioHandler creates a new CardapioHandler.
func NewCardapioHandler(mongo *mongodb.Client) *CardapioHandler {
	return &CardapioHandler{mongo: mongo}
}

// RegisterCardapioRoutes registers the cardapio routes.
// The endpoint is registered as public (no JWT required) to match the Java behavior.
func (h *CardapioHandler) RegisterCardapioRoutes(r chi.Router) {
	r.Get("/api/cardapios", h.ListCardapios)
}

// ListCardapios handles GET /api/cardapios.
// Returns all cardapios from the cardapios collection.
// Returns an empty array if the collection is empty or does not exist.
func (h *CardapioHandler) ListCardapios(w http.ResponseWriter, r *http.Request) {
	// Graceful degradation: if mongo is not wired (e.g. in tests), return empty array.
	if h.mongo == nil {
		writeJSON(w, http.StatusOK, []bson.M{})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	col := h.mongo.Collection("cardapios")
	cursor, err := col.Find(ctx, bson.M{})
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
