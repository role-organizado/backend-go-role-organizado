// Package handler contains HTTP handlers for the finance domain.
// This handler is intentionally lean: it only translates HTTP ↔ use case,
// with ZERO direct MongoDB access.
package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/middleware"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// FinanceHandler handles finance, payment-methods, and audit endpoints.
// It delegates all business logic to injected use cases.
type FinanceHandler struct {
	listEventsUC    portin.ListFinanceEventsUseCase
	overviewUC      portin.GetFinanceOverviewUseCase
	ledgerUC        portin.GetLedgerStatementUseCase
	participantsUC  portin.GetParticipantsStatusUseCase
	recalculateUC   portin.RecalculateFinanceSummaryUseCase
	sendRemindersUC portin.SendPaymentRemindersUseCase
	holdBalanceUC   portin.CalculateHoldBalanceUseCase
	paymentStatusUC portin.GetEventPaymentStatusUseCase
	paymentAccountsUC portin.ManagePaymentAccountsUseCase
	auditTrailUC    portin.GetAuditTrailUseCase
}

// NewFinanceHandler creates a new FinanceHandler with all use cases injected.
func NewFinanceHandler(
	listEventsUC portin.ListFinanceEventsUseCase,
	overviewUC portin.GetFinanceOverviewUseCase,
	ledgerUC portin.GetLedgerStatementUseCase,
	participantsUC portin.GetParticipantsStatusUseCase,
	recalculateUC portin.RecalculateFinanceSummaryUseCase,
	sendRemindersUC portin.SendPaymentRemindersUseCase,
	holdBalanceUC portin.CalculateHoldBalanceUseCase,
	paymentStatusUC portin.GetEventPaymentStatusUseCase,
	paymentAccountsUC portin.ManagePaymentAccountsUseCase,
	auditTrailUC portin.GetAuditTrailUseCase,
) *FinanceHandler {
	return &FinanceHandler{
		listEventsUC:      listEventsUC,
		overviewUC:        overviewUC,
		ledgerUC:          ledgerUC,
		participantsUC:    participantsUC,
		recalculateUC:     recalculateUC,
		sendRemindersUC:   sendRemindersUC,
		holdBalanceUC:     holdBalanceUC,
		paymentStatusUC:   paymentStatusUC,
		paymentAccountsUC: paymentAccountsUC,
		auditTrailUC:      auditTrailUC,
	}
}

// RegisterFinanceRoutes registers all finance and payment-methods routes.
func (h *FinanceHandler) RegisterFinanceRoutes(r chi.Router) {
	// Finance overview endpoints
	r.Get("/api/v1/finance", h.listFinanceEvents)
	r.Get("/api/v1/finance/{eventId}", h.getFinanceOverview)
	r.Post("/api/v1/finance/{eventId}/send-reminders", h.sendPaymentReminders)

	// Finance detail endpoints (new — Java parity)
	r.Get("/api/v1/finance/{eventId}/participants-status", h.getParticipantsStatus)
	r.Get("/api/v1/finance/{eventId}/ledger/statement", h.getLedgerStatement)
	r.Get("/api/v1/finance/{eventId}/summary", h.getFinanceSummary)
	r.Post("/api/v1/finance/{eventId}/summary/recalculate", h.recalculateFinanceSummary)
	r.Get("/api/v1/finance/{eventId}/audit", h.getAuditTrail)
	r.Get("/api/v1/finance/{eventId}/hold-balance", h.getHoldBalance)
	r.Get("/api/v1/finance/{eventId}/payment-status", h.getEventPaymentStatus)

	// Payment methods (PIX/Bank accounts)
	r.Get("/api/v1/payment-methods", h.listPaymentAccounts)
	r.Post("/api/v1/payment-methods", h.createPaymentAccount)
	r.Put("/api/v1/payment-methods/{accountId}", h.updatePaymentAccount)
	r.Post("/api/v1/payment-methods/{accountId}/set-default", h.setDefaultPaymentAccount)
	r.Delete("/api/v1/payment-methods/{accountId}", h.deletePaymentAccount)
}

// ---- helpers ----

// pageParam reads an integer query param, returning defaultVal on parse failure.
func pageParam(r *http.Request, name string, defaultVal int) int {
	if s := r.URL.Query().Get(name); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			return v
		}
	}
	return defaultVal
}

// parseTime parses a RFC3339 query param; returns nil on failure.
func parseTime(r *http.Request, name string) *time.Time {
	if s := r.URL.Query().Get(name); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			return &t
		}
	}
	return nil
}

// ---- Finance overview ----

func (h *FinanceHandler) listFinanceEvents(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("usuário não autenticado"))
		return
	}

	overviews, err := h.listEventsUC.Execute(r.Context(), portin.ListFinanceEventsInput{UserID: userID})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, overviews)
}

func (h *FinanceHandler) getFinanceOverview(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	eventID := chi.URLParam(r, "eventId")

	overview, err := h.overviewUC.Execute(r.Context(), portin.GetFinanceOverviewInput{
		EventID: eventID,
		UserID:  userID,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, overview)
}

func (h *FinanceHandler) sendPaymentReminders(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	eventID := chi.URLParam(r, "eventId")

	if err := h.sendRemindersUC.Execute(r.Context(), portin.SendPaymentRemindersInput{
		EventID: eventID,
		UserID:  userID,
	}); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":  "accepted",
		"message": "lembretes enfileirados",
	})
}

// ---- Finance detail endpoints ----

func (h *FinanceHandler) getParticipantsStatus(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	eventID := chi.URLParam(r, "eventId")
	page := pageParam(r, "page", 0)
	size := pageParam(r, "size", 20)

	statuses, total, err := h.participantsUC.Execute(r.Context(), portin.GetParticipantsStatusInput{
		EventID: eventID,
		UserID:  userID,
		Page:    page,
		Size:    size,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"content":       statuses,
		"totalElements": total,
		"page":          page,
		"size":          size,
	})
}

func (h *FinanceHandler) getLedgerStatement(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	eventID := chi.URLParam(r, "eventId")
	page := pageParam(r, "page", 0)
	size := pageParam(r, "size", 20)
	entryType := r.URL.Query().Get("type")
	from := parseTime(r, "from")
	to := parseTime(r, "to")

	result, err := h.ledgerUC.Execute(r.Context(), portin.GetLedgerStatementInput{
		EventID: eventID,
		UserID:  userID,
		Type:    entryType,
		From:    from,
		To:      to,
		Page:    page,
		Size:    size,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// getFinanceSummary returns the finance summary for an event.
// It reuses the overview UC and returns the same data shape (Java parity).
func (h *FinanceHandler) getFinanceSummary(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	eventID := chi.URLParam(r, "eventId")

	overview, err := h.overviewUC.Execute(r.Context(), portin.GetFinanceOverviewInput{
		EventID: eventID,
		UserID:  userID,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, overview)
}

func (h *FinanceHandler) recalculateFinanceSummary(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	eventID := chi.URLParam(r, "eventId")

	summary, err := h.recalculateUC.Execute(r.Context(), portin.RecalculateFinanceSummaryInput{
		EventID: eventID,
		UserID:  userID,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (h *FinanceHandler) getAuditTrail(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	eventID := chi.URLParam(r, "eventId")
	page := pageParam(r, "page", 0)
	size := pageParam(r, "size", 20)

	entries, total, err := h.auditTrailUC.Execute(r.Context(), portin.GetAuditTrailInput{
		EventID: eventID,
		UserID:  userID,
		Page:    page,
		Size:    size,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"content":       entries,
		"totalElements": total,
		"page":          page,
		"size":          size,
	})
}

func (h *FinanceHandler) getHoldBalance(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	eventID := chi.URLParam(r, "eventId")

	holdBalance, err := h.holdBalanceUC.Execute(r.Context(), portin.CalculateHoldBalanceInput{
		EventID: eventID,
		UserID:  userID,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, holdBalance)
}

func (h *FinanceHandler) getEventPaymentStatus(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	eventID := chi.URLParam(r, "eventId")

	status, err := h.paymentStatusUC.Execute(r.Context(), portin.GetEventPaymentStatusInput{
		EventID: eventID,
		UserID:  userID,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

// ---- Payment methods ----

// createPaymentAccountRequest maps the JSON body to CreatePaymentAccountInput.
type createPaymentAccountRequest struct {
	Type       string `json:"accountType"`
	PixKey     string `json:"pixKey"`
	PixType    string `json:"pixKeyType"`
	BankCode   string `json:"bankCode"`
	AgencyNum  string `json:"agencyNumber"`
	AccountNum string `json:"accountNumber"`
}

// updatePaymentAccountRequest maps the JSON body to UpdatePaymentAccountInput.
type updatePaymentAccountRequest struct {
	Type       string `json:"accountType"`
	PixKey     string `json:"pixKey"`
	PixType    string `json:"pixKeyType"`
	BankCode   string `json:"bankCode"`
	AgencyNum  string `json:"agencyNumber"`
	AccountNum string `json:"accountNumber"`
}

func (h *FinanceHandler) listPaymentAccounts(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("usuário não autenticado"))
		return
	}

	accounts, err := h.paymentAccountsUC.List(r.Context(), portin.ListPaymentAccountsInput{UserID: userID})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, accounts)
}

func (h *FinanceHandler) createPaymentAccount(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("usuário não autenticado"))
		return
	}

	var req createPaymentAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo inválido"))
		return
	}

	account, err := h.paymentAccountsUC.Create(r.Context(), portin.CreatePaymentAccountInput{
		UserID:     userID,
		Type:       req.Type,
		PixKey:     req.PixKey,
		PixType:    req.PixType,
		BankCode:   req.BankCode,
		AgencyNum:  req.AgencyNum,
		AccountNum: req.AccountNum,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, account)
}

func (h *FinanceHandler) updatePaymentAccount(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	accountID := chi.URLParam(r, "accountId")

	var req updatePaymentAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo inválido"))
		return
	}

	account, err := h.paymentAccountsUC.Update(r.Context(), portin.UpdatePaymentAccountInput{
		AccountID:  accountID,
		UserID:     userID,
		Type:       req.Type,
		PixKey:     req.PixKey,
		PixType:    req.PixType,
		BankCode:   req.BankCode,
		AgencyNum:  req.AgencyNum,
		AccountNum: req.AccountNum,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, account)
}

func (h *FinanceHandler) setDefaultPaymentAccount(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	accountID := chi.URLParam(r, "accountId")

	if err := h.paymentAccountsUC.SetDefault(r.Context(), accountID, userID); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (h *FinanceHandler) deletePaymentAccount(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	accountID := chi.URLParam(r, "accountId")

	if err := h.paymentAccountsUC.Delete(r.Context(), accountID, userID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
