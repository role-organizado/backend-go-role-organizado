package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/mongodb"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// UsuariosEventoHandler handles /api/usuarios-evento/* endpoints,
// providing content parity with Java's ParticipantController.
type UsuariosEventoHandler struct {
	mongo *mongodb.Client
}

// NewUsuariosEventoHandler creates a new UsuariosEventoHandler.
func NewUsuariosEventoHandler(mongo *mongodb.Client) *UsuariosEventoHandler {
	return &UsuariosEventoHandler{mongo: mongo}
}

// RegisterUsuariosEventoRoutes registers routes matching Java's ParticipantController.
func (h *UsuariosEventoHandler) RegisterUsuariosEventoRoutes(r chi.Router) {
	// NOTE: No /v1 prefix — matches Java @RequestMapping("/api/usuarios-evento")
	r.Get("/api/usuarios-evento/usuario/{usuarioId}", h.GetByUsuario)
	r.Get("/api/usuarios-evento/by-evento/{eventoId}", h.GetByEvento)
	r.Get("/api/usuarios-evento/{id}", h.GetByID)
}

// bsonDateToISO converts a BSON DateTime or time.Time to an ISO 8601 RFC3339 string.
// Returns nil for nil or unrecognised types.
func bsonDateToISO(v interface{}) interface{} {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case bson.DateTime:
		return val.Time().UTC().Format(time.RFC3339)
	case time.Time:
		return val.UTC().Format(time.RFC3339)
	}
	return nil
}

// participantDocToResponse maps a MongoDB bson.M document to the JSON structure
// matching Java's ParticipantResponse record exactly (camelCase fields, UUID strings, ISO dates).
func participantDocToResponse(doc bson.M) map[string]interface{} {
	return map[string]interface{}{
		"id":                    binaryToUUIDString(doc["_id"]),
		"eventoId":              binaryToUUIDString(doc["evento_id"]),
		"usuarioId":             binaryToUUIDString(doc["usuario_id"]),
		"tipoParticipante":      doc["tipo_participante"],
		"papel":                 doc["papel"],
		"status":                doc["status"],
		"agrupamentoUsuario":    doc["agrupamento_usuario"],
		"dataConfirmacao":       bsonDateToISO(doc["data_confirmacao"]),
		"diasPermanenciaEvento": doc["dias_permanencia_evento"],
		"observacoes":           doc["observacoes"],
		"faixaEtaria":           doc["faixa_etaria"],
		"criadoEm":              bsonDateToISO(doc["criado_em"]),
		"atualizadoEm":          bsonDateToISO(doc["atualizado_em"]),
	}
}

// GetByUsuario handles GET /api/usuarios-evento/usuario/{usuarioId}
// Matches Java's ParticipantController.getByUsuario
func (h *UsuariosEventoHandler) GetByUsuario(w http.ResponseWriter, r *http.Request) {
	usuarioId := chi.URLParam(r, "usuarioId")
	if usuarioId == "" {
		writeError(w, apierr.NotFoundMsg("usuarioId não informado"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	col := h.mongo.Collection("participants")
	cursor, err := col.Find(ctx,
		bson.M{"usuario_id": uuidStringToBinary(usuarioId)},
		options.Find().SetSort(bson.D{{Key: "criado_em", Value: -1}}),
	)
	if err != nil {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}
	defer cursor.Close(ctx)

	var docs []bson.M
	if err := cursor.All(ctx, &docs); err != nil || docs == nil {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}

	results := make([]map[string]interface{}, 0, len(docs))
	for _, doc := range docs {
		results = append(results, participantDocToResponse(doc))
	}
	writeJSON(w, http.StatusOK, results)
}

// GetByEvento handles GET /api/usuarios-evento/by-evento/{eventoId}
// Matches Java's ParticipantController.getByEvento
func (h *UsuariosEventoHandler) GetByEvento(w http.ResponseWriter, r *http.Request) {
	eventoId := chi.URLParam(r, "eventoId")
	if eventoId == "" {
		writeError(w, apierr.NotFoundMsg("eventoId não informado"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	col := h.mongo.Collection("participants")
	cursor, err := col.Find(ctx,
		bson.M{"evento_id": uuidStringToBinary(eventoId)},
		options.Find().SetSort(bson.D{{Key: "criado_em", Value: -1}}),
	)
	if err != nil {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}
	defer cursor.Close(ctx)

	var docs []bson.M
	if err := cursor.All(ctx, &docs); err != nil || docs == nil {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}

	results := make([]map[string]interface{}, 0, len(docs))
	for _, doc := range docs {
		results = append(results, participantDocToResponse(doc))
	}
	writeJSON(w, http.StatusOK, results)
}

// GetByID handles GET /api/usuarios-evento/{id}
// Matches Java's BaseCrudController.get
func (h *UsuariosEventoHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, apierr.NotFoundMsg("id não informado"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	col := h.mongo.Collection("participants")
	var doc bson.M
	err := col.FindOne(ctx, bson.M{"_id": uuidStringToBinary(id)}).Decode(&doc)
	if err != nil {
		writeError(w, apierr.NotFoundMsg("participante não encontrado"))
		return
	}

	writeJSON(w, http.StatusOK, participantDocToResponse(doc))
}
