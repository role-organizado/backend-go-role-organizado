package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/middleware"
	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/mongodb"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// financeEventResponse is the FinanceEvent shape expected by the frontend.
type financeEventResponse struct {
	EventID            string  `json:"eventId"`
	EventName          string  `json:"eventName"`
	EventDate          string  `json:"eventDate"`
	Goal               int64   `json:"goal"`
	Collected          int64   `json:"collected"`
	ProgressPercentage float64 `json:"progressPercentage"`
	PendingWithdrawals int64   `json:"pendingWithdrawals"`
}

// FinanceHandler handles finance, payment-methods, saved-cards, and installments endpoints.
type FinanceHandler struct {
	mongo *mongodb.Client
}

// NewFinanceHandler creates a new FinanceHandler.
func NewFinanceHandler(mongo *mongodb.Client) *FinanceHandler {
	return &FinanceHandler{mongo: mongo}
}

// RegisterFinanceRoutes registers all finance routes.
func (h *FinanceHandler) RegisterFinanceRoutes(r chi.Router) {
	// Finance overview
	r.Get("/api/v1/finance", h.ListFinanceEvents)
	r.Get("/api/v1/finance/{eventId}", h.GetFinanceOverview)
	r.Post("/api/v1/finance/{eventId}/send-reminders", h.SendReminders)

	// Payment methods (PIX/Banco)
	r.Get("/api/v1/payment-methods", h.ListPaymentAccounts)
	r.Post("/api/v1/payment-methods", h.CreatePaymentAccount)
	r.Put("/api/v1/payment-methods/{accountId}", h.UpdatePaymentAccount)
	r.Post("/api/v1/payment-methods/{accountId}/set-default", h.SetDefaultAccount)
	r.Delete("/api/v1/payment-methods/{accountId}", h.DeletePaymentAccount)

	// Saved credit cards
	r.Get("/api/v1/saved-cards", h.ListSavedCards)
	r.Post("/api/v1/saved-cards", h.CreateSavedCard)
	r.Get("/api/v1/saved-cards/{cardId}", h.GetSavedCard)
	r.Delete("/api/v1/saved-cards/{cardId}", h.DeleteSavedCard)
	r.Post("/api/v1/saved-cards/{cardId}/set-default", h.SetDefaultSavedCard)

	// Installments query
	r.Get("/api/v1/installments", h.QueryInstallments)
	r.Get("/api/v1/installments/user", h.GetUserInstallments)
}

// ---- finance_summaries ----

func (h *FinanceHandler) ListFinanceEvents(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("usuário não autenticado"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	col := h.mongo.Collection("finance_summaries")

	cursor, err := col.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "last_calculated_at", Value: -1}}).SetLimit(100))
	if err != nil {
		slog.Error("finance: listing summaries", "error", err)
		writeError(w, apierr.Internal("erro ao listar finanças"))
		return
	}
	defer cursor.Close(ctx)

	var rawSummaries []bson.M
	if err := cursor.All(ctx, &rawSummaries); err != nil {
		writeError(w, apierr.Internal("erro ao decodificar finanças"))
		return
	}

	// Build a set of event IDs to look up event names/dates
	eventIDs := make([]string, 0, len(rawSummaries))
	for _, s := range rawSummaries {
		eventID := mongodb.BinaryToUUIDString(s["event_id"])
		if eventID != "" {
			eventIDs = append(eventIDs, eventID)
		}
	}

	// Fetch event names/dates for all event IDs
	// eventos collection uses Binary UUID subtype 4 as _id, so convert strings to Binary.
	eventMap := make(map[string]bson.M)
	if len(eventIDs) > 0 {
		evCol := h.mongo.Collection("eventos")
		idList := make(bson.A, len(eventIDs))
		for i, id := range eventIDs {
			idList[i] = mongodb.UUIDStringToBinary(id)
		}
		evCursor, evErr := evCol.Find(ctx, bson.M{"_id": bson.M{"$in": idList}}, options.Find().SetProjection(bson.M{"_id": 1, "nome": 1, "data": 1, "data_inicio": 1}))
		if evErr == nil {
			defer evCursor.Close(ctx)
			var evDocs []bson.M
			if evCursor.All(ctx, &evDocs) == nil {
				for _, ev := range evDocs {
					// _id is bson.Binary — convert back to UUID string for map key
					idStr := mongodb.BinaryToUUIDString(ev["_id"])
					if idStr != "" {
						eventMap[idStr] = ev
					}
				}
			}
		}
	}

	response := make([]financeEventResponse, 0, len(rawSummaries))
	for _, s := range rawSummaries {
		eventID := mongodb.BinaryToUUIDString(s["event_id"])

		eventName := ""
		eventDate := ""
		if ev, ok := eventMap[eventID]; ok {
			if n, ok := ev["nome"].(string); ok {
				eventName = n
			}
			if d, ok := ev["data"].(bson.DateTime); ok {
				eventDate = d.Time().UTC().Format(time.RFC3339)
			} else if d, ok := ev["data"].(time.Time); ok {
				eventDate = d.UTC().Format(time.RFC3339)
			} else if d, ok := ev["data_inicio"].(bson.DateTime); ok {
				eventDate = d.Time().UTC().Format(time.RFC3339)
			} else if d, ok := ev["data_inicio"].(time.Time); ok {
				eventDate = d.UTC().Format(time.RFC3339)
			}
		}

		goal := mongodb.ToInt64(s["total_goal"])
		collected := mongodb.ToInt64(s["total_collected"])
		progress := 0.0
		if goal > 0 {
			progress = float64(collected) / float64(goal) * 100
		}

		response = append(response, financeEventResponse{
			EventID:            eventID,
			EventName:          eventName,
			EventDate:          eventDate,
			Goal:               goal,
			Collected:          collected,
			ProgressPercentage: progress,
			PendingWithdrawals: mongodb.ToInt64(s["pending_withdrawals"]),
		})
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *FinanceHandler) GetFinanceOverview(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "eventId")

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	col := h.mongo.Collection("finance_summaries")

	var result bson.M
	err := col.FindOne(ctx, bson.M{"event_id": eventID}).Decode(&result)
	if err != nil {
		// return empty summary if not found
		writeJSON(w, http.StatusOK, bson.M{
			"event_id":             eventID,
			"total_goal":           0,
			"total_collected":      0,
			"total_pending":        0,
			"progress":             0,
			"participants_total":   0,
			"participants_paid":    0,
			"participants_pending": 0,
		})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *FinanceHandler) SendReminders(w http.ResponseWriter, r *http.Request) {
	// Stub: reminders are sent via Temporal NotificationFallbackWorkflow
	w.WriteHeader(http.StatusAccepted)
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted", "message": "lembretes enfileirados"})
}

// ---- payment_accounts ----

type paymentAccountDoc struct {
	ID                 string    `bson:"_id" json:"id"`
	UserID             string    `bson:"user_id" json:"userId"`
	AccountType        string    `bson:"account_type" json:"accountType"`
	PixKeyType         string    `bson:"pix_key_type" json:"pixKeyType"`
	PixKey             string    `bson:"pix_key" json:"pixKey"`
	BankName           string    `bson:"bank_name" json:"bankName"`
	AccountHolderName  string    `bson:"account_holder_name" json:"accountHolderName"`
	IsDefault          bool      `bson:"is_default" json:"isDefault"`
	Active             bool      `bson:"active" json:"active"`
	CriadoEm          time.Time `bson:"criado_em" json:"criadoEm"`
	AtualizadoEm      time.Time `bson:"atualizado_em" json:"atualizadoEm"`
}

func (h *FinanceHandler) ListPaymentAccounts(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("usuário não autenticado"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	col := h.mongo.Collection("payment_accounts")
	cursor, err := col.Find(ctx, bson.M{"user_id": userID, "active": bson.M{"$ne": false}})
	if err != nil {
		writeError(w, apierr.Internal("erro ao listar contas"))
		return
	}
	defer cursor.Close(ctx)

	var results []paymentAccountDoc
	if err := cursor.All(ctx, &results); err != nil {
		writeError(w, apierr.Internal("erro ao decodificar contas"))
		return
	}
	if results == nil {
		results = []paymentAccountDoc{}
	}
	writeJSON(w, http.StatusOK, results)
}

func (h *FinanceHandler) CreatePaymentAccount(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("usuário não autenticado"))
		return
	}

	var req map[string]any
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo inválido"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	now := time.Now().UTC()
	doc := bson.M{
		"_id":                 mongodb.GenerateID(),
		"user_id":             userID,
		"account_type":        req["accountType"],
		"pix_key_type":        req["pixKeyType"],
		"pix_key":             req["pixKey"],
		"bank_name":           req["bankName"],
		"account_holder_name": req["accountHolderName"],
		"is_default":          req["isDefault"] == true,
		"active":              true,
		"criado_em":           now,
		"atualizado_em":       now,
	}

	col := h.mongo.Collection("payment_accounts")
	if _, err := col.InsertOne(ctx, doc); err != nil {
		writeError(w, apierr.Internal("erro ao criar conta"))
		return
	}
	writeJSON(w, http.StatusCreated, doc)
}

func (h *FinanceHandler) UpdatePaymentAccount(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "accountId")
	userID := middleware.UserIDFromContext(r.Context())

	var req map[string]any
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo inválido"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	update := bson.M{"atualizado_em": time.Now().UTC()}
	for k, v := range req {
		switch k {
		case "pixKeyType":
			update["pix_key_type"] = v
		case "pixKey":
			update["pix_key"] = v
		case "bankName":
			update["bank_name"] = v
		case "accountHolderName":
			update["account_holder_name"] = v
		}
	}

	col := h.mongo.Collection("payment_accounts")
	result, err := col.UpdateOne(ctx,
		bson.M{"_id": accountID, "user_id": userID},
		bson.M{"$set": update},
	)
	if err != nil || result.MatchedCount == 0 {
		writeError(w, apierr.NotFoundMsg("conta não encontrada"))
		return
	}

	var updated bson.M
	_ = col.FindOne(ctx, bson.M{"_id": accountID}).Decode(&updated)
	writeJSON(w, http.StatusOK, updated)
}

func (h *FinanceHandler) SetDefaultAccount(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "accountId")
	userID := middleware.UserIDFromContext(r.Context())

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	col := h.mongo.Collection("payment_accounts")
	// Clear existing default
	_, _ = col.UpdateMany(ctx, bson.M{"user_id": userID}, bson.M{"$set": bson.M{"is_default": false}})
	// Set new default
	result, err := col.UpdateOne(ctx,
		bson.M{"_id": accountID, "user_id": userID},
		bson.M{"$set": bson.M{"is_default": true, "atualizado_em": time.Now().UTC()}},
	)
	if err != nil || result.MatchedCount == 0 {
		writeError(w, apierr.NotFoundMsg("conta não encontrada"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (h *FinanceHandler) DeletePaymentAccount(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "accountId")
	userID := middleware.UserIDFromContext(r.Context())

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	col := h.mongo.Collection("payment_accounts")
	result, err := col.UpdateOne(ctx,
		bson.M{"_id": accountID, "user_id": userID},
		bson.M{"$set": bson.M{"active": false, "atualizado_em": time.Now().UTC()}},
	)
	if err != nil || result.MatchedCount == 0 {
		writeError(w, apierr.NotFoundMsg("conta não encontrada"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- saved_cards ----

func (h *FinanceHandler) ListSavedCards(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("usuário não autenticado"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	col := h.mongo.Collection("saved_credit_cards")
	cursor, err := col.Find(ctx, bson.M{"user_id": userID, "active": bson.M{"$ne": false}})
	if err != nil {
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

func (h *FinanceHandler) CreateSavedCard(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("usuário não autenticado"))
		return
	}

	var req map[string]any
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo inválido"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	now := time.Now().UTC()
	doc := bson.M{
		"_id":           mongodb.GenerateID(),
		"user_id":       userID,
		"active":        true,
		"is_default":    req["isDefault"] == true,
		"criado_em":     now,
		"atualizado_em": now,
	}
	for k, v := range req {
		doc[mongodb.CamelToSnake(k)] = v
	}

	col := h.mongo.Collection("saved_credit_cards")
	if _, err := col.InsertOne(ctx, doc); err != nil {
		writeError(w, apierr.Internal("erro ao salvar cartão"))
		return
	}
	writeJSON(w, http.StatusCreated, doc)
}

func (h *FinanceHandler) GetSavedCard(w http.ResponseWriter, r *http.Request) {
	cardID := chi.URLParam(r, "cardId")
	userID := middleware.UserIDFromContext(r.Context())

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	col := h.mongo.Collection("saved_credit_cards")
	var result bson.M
	if err := col.FindOne(ctx, bson.M{"_id": cardID, "user_id": userID}).Decode(&result); err != nil {
		writeError(w, apierr.NotFoundMsg("cartão não encontrado"))
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *FinanceHandler) DeleteSavedCard(w http.ResponseWriter, r *http.Request) {
	cardID := chi.URLParam(r, "cardId")
	userID := middleware.UserIDFromContext(r.Context())

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	col := h.mongo.Collection("saved_credit_cards")
	_, _ = col.UpdateOne(ctx,
		bson.M{"_id": cardID, "user_id": userID},
		bson.M{"$set": bson.M{"active": false}},
	)
	w.WriteHeader(http.StatusNoContent)
}

func (h *FinanceHandler) SetDefaultSavedCard(w http.ResponseWriter, r *http.Request) {
	cardID := chi.URLParam(r, "cardId")
	userID := middleware.UserIDFromContext(r.Context())

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	col := h.mongo.Collection("saved_credit_cards")
	_, _ = col.UpdateMany(ctx, bson.M{"user_id": userID}, bson.M{"$set": bson.M{"is_default": false}})
	result, err := col.UpdateOne(ctx,
		bson.M{"_id": cardID, "user_id": userID},
		bson.M{"$set": bson.M{"is_default": true}},
	)
	if err != nil || result.MatchedCount == 0 {
		writeError(w, apierr.NotFoundMsg("cartão não encontrado"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// ---- installments ----

func (h *FinanceHandler) QueryInstallments(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := bson.M{}
	if eventID := q.Get("eventId"); eventID != "" {
		filter["event_id"] = eventID
	}
	if userID := q.Get("userId"); userID != "" {
		filter["user_id"] = userID
	}
	if status := q.Get("status"); status != "" {
		filter["status"] = status
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	col := h.mongo.Collection("payment_installments")
	cursor, err := col.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "due_date", Value: 1}}))
	if err != nil {
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

// GetUserInstallments handles GET /api/v1/installments/user
// Returns all installments for the authenticated user, optionally filtered by status.
func (h *FinanceHandler) GetUserInstallments(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("usuário não autenticado"))
		return
	}

	filter := bson.M{"user_id": userID}
	if status := r.URL.Query().Get("status"); status != "" {
		filter["status"] = status
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	col := h.mongo.Collection("payment_installments")
	cursor, err := col.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "due_date", Value: 1}}))
	if err != nil {
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

