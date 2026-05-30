package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/mongodb"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// AdminHandler handles admin-only endpoints: dashboard stats, feature flags.
type AdminHandler struct {
	mongo *mongodb.Client
}

// NewAdminHandler creates a new AdminHandler.
func NewAdminHandler(mongo *mongodb.Client) *AdminHandler {
	return &AdminHandler{mongo: mongo}
}

// RegisterAdminRoutes registers admin routes.
func (h *AdminHandler) RegisterAdminRoutes(r chi.Router) {
	r.Get("/api/v1/admin/dashboard/stats", h.GetDashboardStats)
	r.Get("/api/v1/admin/dashboard/health", h.GetDashboardHealth)
	r.Get("/api/v1/admin/dashboard/finance", h.GetDashboardFinance)

	r.Get("/api/v1/admin/feature-flags", h.ListFeatureFlags)
	r.Put("/api/v1/admin/feature-flags/{chave}", h.UpdateFeatureFlag)
	r.Patch("/api/v1/admin/feature-flags/{chave}", h.UpdateFeatureFlag)
}

// ---- Dashboard stats ----

func (h *AdminHandler) GetDashboardStats(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	totalUsuarios, _ := h.mongo.Collection("usuarios").CountDocuments(ctx, bson.M{})
	totalEventos, _ := h.mongo.Collection("eventos").CountDocuments(ctx, bson.M{})
	totalDrafts, _ := h.mongo.Collection("eventos_draft").CountDocuments(ctx, bson.M{})
	totalPagamentos, _ := h.mongo.Collection("pagamentos_mensais").CountDocuments(ctx, bson.M{})
	totalNotificacoes, _ := h.mongo.Collection("notificacoes").CountDocuments(ctx, bson.M{})

	stats := map[string]any{
		"bigNumbers": map[string]any{
			"totalUsuarios":   totalUsuarios,
			"totalEventos":    totalEventos,
			"totalDrafts":     totalDrafts,
			"totalPagamentos": totalPagamentos,
			"totalNotificacoes": totalNotificacoes,
		},
		"serviceHealth": map[string]any{
			"database": "UP",
			"backend":  "UP",
		},
		"recentActivity": []any{},
	}
	writeJSON(w, http.StatusOK, stats)
}

func (h *AdminHandler) GetDashboardHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	dbStatus := "UP"
	if err := h.mongo.Ping(ctx); err != nil {
		dbStatus = "DOWN"
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":    dbStatus,
		"database":  dbStatus,
		"backend":   "UP",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func (h *AdminHandler) GetDashboardFinance(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Aggregate finance_summaries to get totals
	pipeline := bson.A{
		bson.M{"$group": bson.M{
			"_id":               nil,
			"totalEventos":      bson.M{"$sum": 1},
		}},
	}
	cursor, err := h.mongo.Collection("finance_summaries").Aggregate(ctx, pipeline)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"totalEventos": 0})
		return
	}
	defer cursor.Close(ctx)

	var results []bson.M
	_ = cursor.All(ctx, &results)

	if len(results) > 0 {
		writeJSON(w, http.StatusOK, results[0])
	} else {
		writeJSON(w, http.StatusOK, map[string]any{"totalEventos": 0})
	}
}

// ---- Feature Flags ----

type featureFlagDoc struct {
	ID           string         `bson:"_id" json:"id"`
	Chave        string         `bson:"chave" json:"chave"`
	Nome         string         `bson:"nome" json:"nome"`
	Enabled      bool           `bson:"enabled" json:"enabled"`
	Descricao    string         `bson:"descricao" json:"descricao"`
	Categoria    string         `bson:"categoria" json:"categoria"`
	Metadata     map[string]any `bson:"metadata" json:"metadata"`
	CriadoEm    string         `bson:"criado_em" json:"criadoEm"`
	AtualizadoEm string        `bson:"atualizado_em" json:"atualizadoEm"`
}

func (h *AdminHandler) ListFeatureFlags(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	cursor, err := h.mongo.Collection("feature_flags").Find(ctx, bson.M{})
	if err != nil {
		writeJSON(w, http.StatusOK, []featureFlagDoc{})
		return
	}
	defer cursor.Close(ctx)

	var results []featureFlagDoc
	if err := cursor.All(ctx, &results); err != nil || results == nil {
		results = []featureFlagDoc{}
	}
	writeJSON(w, http.StatusOK, results)
}

func (h *AdminHandler) UpdateFeatureFlag(w http.ResponseWriter, r *http.Request) {
	chave := chi.URLParam(r, "chave")

	var req map[string]any
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo inválido"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	update := bson.M{"atualizado_em": time.Now().UTC().Format(time.RFC3339)}
	if enabled, ok := req["enabled"]; ok {
		update["enabled"] = enabled
	}
	if nome, ok := req["nome"]; ok {
		update["nome"] = nome
	}
	if descricao, ok := req["descricao"]; ok {
		update["descricao"] = descricao
	}

	col := h.mongo.Collection("feature_flags")
	result, err := col.UpdateOne(ctx,
		bson.M{"chave": chave},
		bson.M{"$set": update},
	)
	if err != nil || result.MatchedCount == 0 {
		writeError(w, apierr.NotFoundMsg("feature flag não encontrada"))
		return
	}

	var updated featureFlagDoc
	_ = col.FindOne(ctx, bson.M{"chave": chave}).Decode(&updated)
	writeJSON(w, http.StatusOK, updated)
}

// ---- Admin Usuarios ----

func (h *AdminHandler) ListUsuarios(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	cursor, err := h.mongo.Collection("usuarios").Find(ctx, bson.M{})
	if err != nil {
		writeError(w, apierr.Internal("erro ao listar usuários"))
		return
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err := cursor.All(ctx, &results); err != nil || results == nil {
		results = []bson.M{}
	}
	// Remove sensitive fields
	for i := range results {
		delete(results[i], "senha")
		delete(results[i], "password")
		delete(results[i], "hash_senha")
	}
	writeJSON(w, http.StatusOK, results)
}

func (h *AdminHandler) UpdateUsuarioRole(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req map[string]any
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo inválido"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	update := bson.M{"atualizado_em": time.Now().UTC()}
	if roles, ok := req["roles"]; ok {
		update["roles"] = roles
	}
	if role, ok := req["role"]; ok {
		update["role"] = role
	}

	col := h.mongo.Collection("usuarios")
	result, err := col.UpdateOne(ctx,
		bson.M{"_id": id},
		bson.M{"$set": update},
	)
	if err != nil || result.MatchedCount == 0 {
		writeError(w, apierr.NotFoundMsg("usuário não encontrado"))
		return
	}

	var updated bson.M
	_ = col.FindOne(ctx, bson.M{"_id": id}).Decode(&updated)
	delete(updated, "senha")
	delete(updated, "password")
	delete(updated, "hash_senha")
	writeJSON(w, http.StatusOK, updated)
}
