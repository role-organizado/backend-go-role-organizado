package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/middleware"
	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/mongodb"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ParticipantesHandler handles participantes/sugestões endpoints.
type ParticipantesHandler struct {
	mongo *mongodb.Client
}

// NewParticipantesHandler creates a new ParticipantesHandler.
func NewParticipantesHandler(mongo *mongodb.Client) *ParticipantesHandler {
	return &ParticipantesHandler{mongo: mongo}
}

// RegisterParticipantesRoutes registers routes.
func (h *ParticipantesHandler) RegisterParticipantesRoutes(r chi.Router) {
	r.Get("/api/v1/participantes/recentes", h.BuscarRecentes)
	r.Get("/api/v1/participantes/verificar", h.VerificarExistente)
}

// BuscarRecentes returns recent participant suggestions based on user's last 2 events.
// GET /api/v1/participantes/recentes
func (h *ParticipantesHandler) BuscarRecentes(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("usuário não autenticado"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Fetch user's last 2 events (as organizer)
	eventosCol := h.mongo.Collection("eventos")
	eventsCursor, err := eventosCol.Find(ctx,
		bson.M{"usuario_id_responsavel": mongodb.UUIDStringToBinary(userID)},
		options.Find().SetSort(bson.D{{Key: "data_inicio", Value: -1}}).SetLimit(2).SetProjection(bson.M{"_id": 1}),
	)
	if err != nil {
		writeJSON(w, http.StatusOK, []bson.M{})
		return
	}
	defer eventsCursor.Close(ctx)

	var eventos []bson.M
	if err := eventsCursor.All(ctx, &eventos); err != nil || len(eventos) == 0 {
		writeJSON(w, http.StatusOK, []bson.M{})
		return
	}

	// Collect event IDs
	eventIDs := make([]any, 0, len(eventos))
	for _, e := range eventos {
		if id, ok := e["_id"]; ok {
			eventIDs = append(eventIDs, id)
		}
	}

	// Find participants from those events (excluding the organizer)
	participantesCol := h.mongo.Collection("participants")
	partCursor, err := participantesCol.Find(ctx, bson.M{
		"evento_id":  bson.M{"$in": eventIDs},
		"usuario_id": bson.M{"$ne": mongodb.UUIDStringToBinary(userID)},
	})
	if err != nil {
		writeJSON(w, http.StatusOK, []bson.M{})
		return
	}
	defer partCursor.Close(ctx)

	var participantes []bson.M
	if err := partCursor.All(ctx, &participantes); err != nil || len(participantes) == 0 {
		writeJSON(w, http.StatusOK, []bson.M{})
		return
	}

	// Collect unique user IDs from participants
	userIDCounts := make(map[string]int)
	for _, p := range participantes {
		uid := mongodb.BinaryToUUIDString(p["usuario_id"])
		if uid != "" {
			userIDCounts[uid]++
		}
	}

	// Fetch user details for each participant
	usuariosCol := h.mongo.Collection("usuarios")
	var uniqueUserIDs []any
	for uid := range userIDCounts {
		uniqueUserIDs = append(uniqueUserIDs, mongodb.UUIDStringToBinary(uid))
	}

	usersCursor, err := usuariosCol.Find(ctx,
		bson.M{"_id": bson.M{"$in": uniqueUserIDs}},
		options.Find().SetProjection(bson.M{"senha": 0, "password": 0, "hash_senha": 0}),
	)
	if err != nil {
		writeJSON(w, http.StatusOK, []bson.M{})
		return
	}
	defer usersCursor.Close(ctx)

	var usuarios []bson.M
	_ = usersCursor.All(ctx, &usuarios)

	// Build response with sugestão format — matches Java's ParticipanteRecenteDTO
	// Java fields: nome, telefone, email, quantidadeEventosRecentes (camelCase), jaCadastrado (camelCase)
	sugestoes := make([]bson.M, 0, len(usuarios))
	for _, u := range usuarios {
		uid := mongodb.BinaryToUUIDString(u["_id"])
		count := userIDCounts[uid]

		sugestao := bson.M{
			"nome":                    u["nome"],
			"telefone":                u["telefone"],
			"email":                   u["email"],
			"quantidadeEventosRecentes": count,
			"jaCadastrado":            true,
		}
		sugestoes = append(sugestoes, sugestao)
	}

	writeJSON(w, http.StatusOK, sugestoes)
}

// VerificarExistente checks if a contact (phone/email) is already registered.
// GET /api/v1/participantes/verificar?telefone=X&email=Y
func (h *ParticipantesHandler) VerificarExistente(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	telefone := q.Get("telefone")
	email := q.Get("email")

	if telefone == "" && email == "" {
		writeError(w, apierr.BadRequest("telefone ou email é obrigatório"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	filter := bson.A{}
	if telefone != "" {
		filter = append(filter, bson.M{"telefone": telefone})
	}
	if email != "" {
		filter = append(filter, bson.M{"email": email})
	}

	var result bson.M
	err := h.mongo.Collection("usuarios").FindOne(ctx, bson.M{"$or": filter}).Decode(&result)
	jaCadastrado := err == nil && result != nil

	response := bson.M{"ja_cadastrado": jaCadastrado}
	if jaCadastrado {
		response["nome"] = result["nome"]
		response["email"] = result["email"]
	}

	writeJSON(w, http.StatusOK, response)
}

// LookupParticipante looks up participant details by phone/email.
// GET /api/v1/participantes/lookup?telefone=X&email=Y
func (h *ParticipantesHandler) LookupParticipante(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	telefone := q.Get("telefone")
	email := q.Get("email")

	filter := bson.A{}
	if telefone != "" {
		filter = append(filter, bson.M{"telefone": telefone})
	}
	if email != "" {
		filter = append(filter, bson.M{"email": email})
	}
	if len(filter) == 0 {
		writeError(w, apierr.BadRequest("telefone ou email é obrigatório"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var result bson.M
	err := h.mongo.Collection("usuarios").FindOne(ctx, bson.M{"$or": filter}).Decode(&result)
	if err != nil {
		writeError(w, apierr.NotFoundMsg("participante não encontrado"))
		return
	}
	response := bson.M{
		"id":        mongodb.BinaryToUUIDString(result["_id"]),
		"nome":      result["nome"],
		"email":     result["email"],
		"telefone":  result["telefone"],
		"fotoPerfil": result["foto_perfil"],
		"ja_cadastrado": true,
	}
	writeJSON(w, http.StatusOK, response)
}
