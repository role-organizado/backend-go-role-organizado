package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/middleware"
	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// PaymentMethodsHandler handles payment-method accounts, PIX validation, and saved cards.
type PaymentMethodsHandler struct {
	manageAccounts portin.ManagePaymentAccountsUseCase
	validatePix    portin.ValidatePixKeyUseCase
	manageSavedCards portin.ManageSavedCardsUseCase
}

// NewPaymentMethodsHandler creates a new PaymentMethodsHandler.
func NewPaymentMethodsHandler(
	manageAccounts portin.ManagePaymentAccountsUseCase,
	validatePix portin.ValidatePixKeyUseCase,
	manageSavedCards portin.ManageSavedCardsUseCase,
) *PaymentMethodsHandler {
	return &PaymentMethodsHandler{
		manageAccounts:   manageAccounts,
		validatePix:      validatePix,
		manageSavedCards: manageSavedCards,
	}
}

// RegisterPaymentMethodsRoutes mounts all payment-methods and saved-cards routes.
func (h *PaymentMethodsHandler) RegisterPaymentMethodsRoutes(r chi.Router) {
	// PIX key validation — static path must come before the parameterised {accountId} routes.
	r.Post("/api/v1/payment-methods/pix/validate", h.validatePixKey)

	// PaymentAccount CRUD
	r.Get("/api/v1/payment-methods", h.listAccounts)
	r.Post("/api/v1/payment-methods", h.createAccount)
	r.Put("/api/v1/payment-methods/{accountId}", h.updateAccount)
	r.Post("/api/v1/payment-methods/{accountId}/set-default", h.setDefaultAccount)
	r.Delete("/api/v1/payment-methods/{accountId}", h.deleteAccount)

	// SavedCreditCard management
	r.Get("/api/v1/saved-cards", h.listSavedCards)
	r.Delete("/api/v1/saved-cards/{cardId}", h.deleteSavedCard)
	r.Post("/api/v1/saved-cards/{cardId}/set-default", h.setDefaultSavedCard)
}

// ─── PaymentAccount DTOs ────────────────────────────────────────────────────

type createAccountRequest struct {
	AccountType           string `json:"accountType"`
	PixKeyType            string `json:"pixKeyType"`
	PixKey                string `json:"pixKey"`
	BankCode              string `json:"bankCode"`
	BankName              string `json:"bankName"`
	Agency                string `json:"agency"`
	AccountNumber         string `json:"accountNumber"`
	AccountDigit          string `json:"accountDigit"`
	AccountHolderName     string `json:"accountHolderName"`
	AccountHolderDocument string `json:"accountHolderDocument"`
}

type updateAccountRequest struct {
	PixKeyType            *string `json:"pixKeyType"`
	PixKey                *string `json:"pixKey"`
	BankCode              *string `json:"bankCode"`
	BankName              *string `json:"bankName"`
	Agency                *string `json:"agency"`
	AccountNumber         *string `json:"accountNumber"`
	AccountDigit          *string `json:"accountDigit"`
	AccountHolderName     *string `json:"accountHolderName"`
	AccountHolderDocument *string `json:"accountHolderDocument"`
}

type paymentAccountResponse struct {
	ID                    string `json:"id"`
	UserID                string `json:"userId"`
	AccountType           string `json:"accountType"`
	PixKeyType            string `json:"pixKeyType,omitempty"`
	PixKey                string `json:"pixKey,omitempty"`
	BankCode              string `json:"bankCode,omitempty"`
	BankName              string `json:"bankName,omitempty"`
	Agency                string `json:"agency,omitempty"`
	AccountNumber         string `json:"accountNumber,omitempty"`
	AccountDigit          string `json:"accountDigit,omitempty"`
	AccountHolderName     string `json:"accountHolderName,omitempty"`
	AccountHolderDocument string `json:"accountHolderDocument,omitempty"`
	IsDefault             bool   `json:"isDefault"`
	IsActive              bool   `json:"isActive"`
	CreatedAt             string `json:"createdAt"`
	UpdatedAt             string `json:"updatedAt"`
}

func accountToResponse(a *domain.PaymentAccount) paymentAccountResponse {
	return paymentAccountResponse{
		ID:                    a.ID,
		UserID:                a.UserID,
		AccountType:           string(a.AccountType),
		PixKeyType:            string(a.PixKeyType),
		PixKey:                a.PixKey,
		BankCode:              a.BankCode,
		BankName:              a.BankName,
		Agency:                a.Agency,
		AccountNumber:         a.AccountNumber,
		AccountDigit:          a.AccountDigit,
		AccountHolderName:     a.AccountHolderName,
		AccountHolderDocument: a.AccountHolderDocument,
		IsDefault:             a.IsDefault,
		IsActive:              a.IsActive,
		CreatedAt:             a.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:             a.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

// ─── PaymentAccount handlers ─────────────────────────────────────────────────

func (h *PaymentMethodsHandler) listAccounts(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("usuário não autenticado"))
		return
	}

	accounts, err := h.manageAccounts.List(r.Context(), userID)
	if err != nil {
		writeError(w, err)
		return
	}

	resp := make([]paymentAccountResponse, len(accounts))
	for i, a := range accounts {
		resp[i] = accountToResponse(a)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *PaymentMethodsHandler) createAccount(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("usuário não autenticado"))
		return
	}

	var req createAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo da requisição inválido"))
		return
	}

	in := portin.CreatePaymentAccountInput{
		UserID:                userID,
		AccountType:           domain.AccountType(req.AccountType),
		PixKeyType:            domain.PixKeyType(req.PixKeyType),
		PixKey:                req.PixKey,
		BankCode:              req.BankCode,
		BankName:              req.BankName,
		Agency:                req.Agency,
		AccountNumber:         req.AccountNumber,
		AccountDigit:          req.AccountDigit,
		AccountHolderName:     req.AccountHolderName,
		AccountHolderDocument: req.AccountHolderDocument,
	}

	acct, err := h.manageAccounts.Create(r.Context(), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, accountToResponse(acct))
}

func (h *PaymentMethodsHandler) updateAccount(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("usuário não autenticado"))
		return
	}
	accountID := chi.URLParam(r, "accountId")

	var req updateAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo da requisição inválido"))
		return
	}

	var pixKeyType *domain.PixKeyType
	if req.PixKeyType != nil {
		kt := domain.PixKeyType(*req.PixKeyType)
		pixKeyType = &kt
	}

	in := portin.UpdatePaymentAccountInput{
		PixKeyType:            pixKeyType,
		PixKey:                req.PixKey,
		BankCode:              req.BankCode,
		BankName:              req.BankName,
		Agency:                req.Agency,
		AccountNumber:         req.AccountNumber,
		AccountDigit:          req.AccountDigit,
		AccountHolderName:     req.AccountHolderName,
		AccountHolderDocument: req.AccountHolderDocument,
	}

	acct, err := h.manageAccounts.Update(r.Context(), accountID, userID, in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, accountToResponse(acct))
}

func (h *PaymentMethodsHandler) setDefaultAccount(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("usuário não autenticado"))
		return
	}
	accountID := chi.URLParam(r, "accountId")

	if err := h.manageAccounts.SetDefault(r.Context(), accountID, userID); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (h *PaymentMethodsHandler) deleteAccount(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("usuário não autenticado"))
		return
	}
	accountID := chi.URLParam(r, "accountId")

	if err := h.manageAccounts.Delete(r.Context(), accountID, userID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ─── PIX Validate handler ─────────────────────────────────────────────────────

type validatePixKeyRequest struct {
	Key string `json:"key"`
}

type validatePixKeyResponse struct {
	Valid   bool   `json:"valid"`
	KeyType string `json:"keyType"`
	Key     string `json:"key"`
	Message string `json:"message"`
}

func (h *PaymentMethodsHandler) validatePixKey(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("usuário não autenticado"))
		return
	}

	var req validatePixKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo da requisição inválido"))
		return
	}

	result, err := h.validatePix.Execute(r.Context(), userID, req.Key)
	if err != nil {
		writeError(w, err)
		return
	}

	msg := ""
	if !result.Valid {
		msg = "chave PIX inválida"
		if result.KeyType != "" {
			msg = "chave PIX inválida para o tipo " + result.KeyType
		}
	} else {
		msg = "chave PIX válida"
	}

	writeJSON(w, http.StatusOK, validatePixKeyResponse{
		Valid:   result.Valid,
		KeyType: result.KeyType,
		Key:     result.Key,
		Message: msg,
	})
}

// ─── SavedCreditCard DTOs & handlers ─────────────────────────────────────────

type savedCardResponse struct {
	ID             string `json:"id"`
	UserID         string `json:"userId"`
	LastFourDigits string `json:"lastFourDigits"`
	Brand          string `json:"brand"`
	HolderName     string `json:"holderName"`
	ExpirationDate string `json:"expirationDate"`
	IsDefault      bool   `json:"isDefault"`
	IsActive       bool   `json:"isActive"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}

func cardToResponse(c *domain.SavedCreditCard) savedCardResponse {
	return savedCardResponse{
		ID:             c.ID,
		UserID:         c.UserID,
		LastFourDigits: c.LastFourDigits,
		Brand:          string(c.Brand),
		HolderName:     c.HolderName,
		ExpirationDate: c.ExpirationDate,
		IsDefault:      c.IsDefault,
		IsActive:       c.IsActive,
		CreatedAt:      c.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:      c.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func (h *PaymentMethodsHandler) listSavedCards(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("usuário não autenticado"))
		return
	}

	cards, err := h.manageSavedCards.List(r.Context(), userID)
	if err != nil {
		writeError(w, err)
		return
	}

	resp := make([]savedCardResponse, len(cards))
	for i, c := range cards {
		resp[i] = cardToResponse(c)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *PaymentMethodsHandler) deleteSavedCard(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("usuário não autenticado"))
		return
	}
	cardID := chi.URLParam(r, "cardId")

	if err := h.manageSavedCards.Delete(r.Context(), cardID, userID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *PaymentMethodsHandler) setDefaultSavedCard(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("usuário não autenticado"))
		return
	}
	cardID := chi.URLParam(r, "cardId")

	if err := h.manageSavedCards.SetDefault(r.Context(), cardID, userID); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}
