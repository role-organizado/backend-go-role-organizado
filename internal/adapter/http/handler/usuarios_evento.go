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
	// participants-summary must be registered before /{id} to avoid route conflict
	r.Get("/api/usuarios-evento/{eventoId}/participants-summary", h.GetParticipantsSummary)
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

// participantSummaryDTO matches Java's ParticipantController.ParticipantSummaryDTO.
type participantSummaryDTO struct {
	UserID       string    `json:"userId"`
	UserName     string    `json:"userName"`
	Installments []bson.M  `json:"installments"`
}

// GetParticipantsSummary handles GET /api/usuarios-evento/{eventoId}/participants-summary
// Returns confirmed participants enriched with user name and their installments.
// Matches Java's ParticipantController.getParticipantsSummary.
func (h *UsuariosEventoHandler) GetParticipantsSummary(w http.ResponseWriter, r *http.Request) {
	eventoId := chi.URLParam(r, "eventoId")
	if eventoId == "" {
		writeJSON(w, http.StatusOK, []participantSummaryDTO{})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// 1. Find confirmed participants for the event
	partCol := h.mongo.Collection("participants")
	partCursor, err := partCol.Find(ctx, bson.M{
		"evento_id": uuidStringToBinary(eventoId),
		"status":    bson.M{"$in": bson.A{"CONFIRMADO", "ATIVO"}},
	})
	if err != nil {
		writeJSON(w, http.StatusOK, []participantSummaryDTO{})
		return
	}
	defer partCursor.Close(ctx)

	var participants []bson.M
	if err := partCursor.All(ctx, &participants); err != nil || len(participants) == 0 {
		writeJSON(w, http.StatusOK, []participantSummaryDTO{})
		return
	}

	// 2. Collect unique user IDs to batch-fetch names
	userIDs := make([]any, 0, len(participants))
	for _, p := range participants {
		uid := binaryToUUIDString(p["usuario_id"])
		if uid != "" {
			userIDs = append(userIDs, uuidStringToBinary(uid))
		}
	}

	// 3. Fetch user names
	userNames := make(map[string]string, len(userIDs))
	if len(userIDs) > 0 {
		uCol := h.mongo.Collection("usuarios")
		uCursor, uErr := uCol.Find(ctx,
			bson.M{"_id": bson.M{"$in": userIDs}},
			options.Find().SetProjection(bson.M{"_id": 1, "nome": 1}),
		)
		if uErr == nil {
			defer uCursor.Close(ctx)
			var users []bson.M
			if uCursor.All(ctx, &users) == nil {
				for _, u := range users {
					uid := binaryToUUIDString(u["_id"])
					if nome, ok := u["nome"].(string); ok && uid != "" {
						userNames[uid] = nome
					}
				}
			}
		}
	}

	// 4. Fetch installments for this event grouped by participant
	installmentsMap := make(map[string][]bson.M)
	instCol := h.mongo.Collection("payment_installments")
	instCursor, instErr := instCol.Find(ctx, bson.M{
		"event_id": uuidStringToBinary(eventoId),
	})
	if instErr == nil {
		defer instCursor.Close(ctx)
		var installments []bson.M
		if instCursor.All(ctx, &installments) == nil {
			for _, inst := range installments {
				// Java uses participant_id to group (UUID string key)
				pid := binaryToUUIDString(inst["participant_id"])
				if pid != "" {
					installmentsMap[pid] = append(installmentsMap[pid], inst)
				}
			}
		}
	}

	// 5. Build response
	results := make([]participantSummaryDTO, 0, len(participants))
	for _, p := range participants {
		uid := binaryToUUIDString(p["usuario_id"])
		pid := binaryToUUIDString(p["_id"])
		userName := userNames[uid]
		if userName == "" {
			userName = "Unknown"
		}
		insts := installmentsMap[pid]
		if insts == nil {
			insts = []bson.M{}
		}
		results = append(results, participantSummaryDTO{
			UserID:       uid,
			UserName:     userName,
			Installments: insts,
		})
	}
	writeJSON(w, http.StatusOK, results)
}
