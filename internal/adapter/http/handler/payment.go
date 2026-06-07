package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/middleware"
	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// PaymentHandler handles payment and payment config HTTP endpoints.
//
// Route architecture decision (Strangler Fig):
//   - /api/v1/payments/* → PaymentTransaction (real Asaas charges) — Java-compat routes
//   - /api/payments/*    → PagamentoMensal (internal recurring payments) — legacy Go CRUD
//
// The conflict between GET /api/v1/payments/{id} (PagamentoMensal) and
// GET /api/v1/payments/{transactionId} (PaymentTransaction) is resolved in favour of
// PaymentTransaction: the v1 prefix is exclusively Java-compat and therefore
// PaymentTransaction takes precedence. PagamentoMensal CRUD remains accessible
// under the non-v1 /api/payments/ prefix.
type PaymentHandler struct {
	// ── PagamentoMensal CRUD (legacy internal payments) ─────────────────────────
	createUC    portin.CreatePagamentoUseCase
	getUC       portin.GetPagamentoUseCase
	listUC      portin.ListPagamentosUseCase
	updateUC    portin.UpdatePagamentoUseCase
	deleteUC    portin.DeletePagamentoUseCase
	confirmarUC portin.ConfirmarPagamentoUseCase
	upsertCfgUC portin.UpsertConfigPagamentoUseCase
	getCfgUC    portin.GetConfigPagamentoUseCase

	// ── PaymentTransaction (real Asaas charges) ──────────────────────────────────
	processPaymentUC      portin.ProcessPaymentUseCase
	processBatchPaymentUC portin.ProcessBatchPaymentUseCase
	getTransactionUC      portin.GetPaymentTransactionUseCase
	listUserPaymentsUC    portin.ListUserPaymentsUseCase
	provider              portout.PaymentProvider // used by sandbox-simulate

	// ── Admin: fee policy reapplication ─────────────────────────────────────────
	reaplicarFeeUC portin.ReaplicarFeePolicyNasConfigsUseCase
}

// NewPaymentHandler creates a PaymentHandler with all use cases wired in.
func NewPaymentHandler(
	create portin.CreatePagamentoUseCase,
	get portin.GetPagamentoUseCase,
	list portin.ListPagamentosUseCase,
	update portin.UpdatePagamentoUseCase,
	del portin.DeletePagamentoUseCase,
	confirmar portin.ConfirmarPagamentoUseCase,
	upsertCfg portin.UpsertConfigPagamentoUseCase,
	getCfg portin.GetConfigPagamentoUseCase,
	processPayment portin.ProcessPaymentUseCase,
	processBatchPayment portin.ProcessBatchPaymentUseCase,
	getTransaction portin.GetPaymentTransactionUseCase,
	listUserPayments portin.ListUserPaymentsUseCase,
	provider portout.PaymentProvider,
	reaplicarFee portin.ReaplicarFeePolicyNasConfigsUseCase,
) *PaymentHandler {
	return &PaymentHandler{
		createUC:              create,
		getUC:                 get,
		listUC:                list,
		updateUC:              update,
		deleteUC:              del,
		confirmarUC:           confirmar,
		upsertCfgUC:           upsertCfg,
		getCfgUC:              getCfg,
		processPaymentUC:      processPayment,
		processBatchPaymentUC: processBatchPayment,
		getTransactionUC:      getTransaction,
		listUserPaymentsUC:    listUserPayments,
		provider:              provider,
		reaplicarFeeUC:        reaplicarFee,
	}
}

// RegisterPaymentRoutes mounts all payment routes onto the chi router.
func (h *PaymentHandler) RegisterPaymentRoutes(r chi.Router) {
	// ── Legacy PagamentoMensal CRUD (no /v1 prefix) ─────────────────────────────
	r.Get("/api/payments", h.listPagamentos)
	r.Post("/api/payments", h.createPagamento)
	r.Get("/api/payments/{id}", h.getPagamento)
	r.Put("/api/payments/{id}", h.updatePagamento)
	r.Delete("/api/payments/{id}", h.deletePagamento)
	r.Post("/api/payments/{id}/confirmar", h.confirmarPagamento)
	r.Get("/api/payments/config", h.getConfig)
	r.Put("/api/payments/config", h.upsertConfig)

	// ── PaymentTransaction — Java-compat v1 routes ───────────────────────────────
	// Static routes must be registered before the wildcard {transactionId}.
	// chi handles specificity correctly, but explicit ordering makes the intent clear.

	// POST /api/v1/payments/process — real Asaas payment (replaces stub)
	r.Post("/api/v1/payments/process", h.processPayment)

	// POST /api/v1/payments/batch — atomic batch charge for multiple installments
	r.Post("/api/v1/payments/batch", h.processBatch)

	// GET /api/v1/payments/user — list transactions for JWT user
	r.Get("/api/v1/payments/user", h.listUserPayments)

	// GET /api/v1/payments/user/{userId} — list for explicit user (ownership enforced)
	r.Get("/api/v1/payments/user/{userId}", h.listUserPaymentsByUserID)

	// GET /api/v1/payments/config — event payment config (backward compat)
	r.Get("/api/v1/payments/config", h.getConfig)
	r.Put("/api/v1/payments/config", h.upsertConfig)

	// GET /api/v1/payments/{transactionId} — PaymentTransaction detail (takes precedence)
	// NOTE: PagamentoMensal GET /{id} was removed from /api/v1/ prefix here;
	// it remains accessible under /api/payments/{id}.
	r.Get("/api/v1/payments/{transactionId}", h.getTransaction)

	// POST /api/v1/payments/{transactionId}/sandbox-simulate — admin/moderator only
	r.With(middleware.RequireRole("ADMIN")).Post("/api/v1/payments/{transactionId}/sandbox-simulate", h.sandboxSimulate)

	// POST /api/v1/admin/payments/reaplicar-fee-policy — ADMIN only
	// Mirrors Java ReaplicarFeePolicySnapshotUseCase endpoint.
	r.With(middleware.RequireRole("ADMIN")).Post("/api/v1/admin/payments/reaplicar-fee-policy", h.reaplicarFeePolicy)

	// PagamentoMensal list/create remain on the v1 prefix for backward compat
	// with existing mobile clients that use /api/v1/payments (non-process).
	r.Get("/api/v1/payments", h.listPagamentos)
	r.Post("/api/v1/payments", h.createPagamento)
}

// ─── PagamentoMensal handlers ─────────────────────────────────────────────────

type createPagamentoRequest struct {
	EventoID        string    `json:"eventoId"`
	Descricao       string    `json:"descricao"`
	Valor           float64   `json:"valor"`
	MetodoPagamento string    `json:"metodoPagamento"`
	DataVencimento  time.Time `json:"dataVencimento"`
	Observacao      string    `json:"observacao"`
}

type updatePagamentoRequest struct {
	Descricao      *string    `json:"descricao"`
	Valor          *float64   `json:"valor"`
	DataVencimento *time.Time `json:"dataVencimento"`
	Observacao     *string    `json:"observacao"`
}

type confirmarPagamentoRequest struct {
	DataPagamento time.Time `json:"dataPagamento"`
	Comprovante   string    `json:"comprovante"`
}

type upsertConfigRequest struct {
	EventoID         string     `json:"eventoId"`
	MetodosPagamento []string   `json:"metodosPagamento"`
	PrazoPagamento   *time.Time `json:"prazoPagamento"`
	ChavePix         string     `json:"chavePix"`
	InstrucoesBoleto string     `json:"instrucoesBoleto"`

	// Fee policy snapshot fields (gap report §6.3 — Java parity).
	PlatformFeePercent    float64 `json:"platformFeePercent"`
	PspFeePercent         float64 `json:"pspFeePercent"`
	PaymentProvider       string  `json:"paymentProvider"`
	PaymentFrequency      string  `json:"paymentFrequency"`
	PaymentReleaseTrigger string  `json:"paymentReleaseTrigger"`
}

// reaplicarFeePolicyRequest is the body for POST /api/v1/admin/payments/reaplicar-fee-policy.
type reaplicarFeePolicyRequest struct {
	PlatformFeePercent float64 `json:"platformFeePercent"`
	PspFeePercent      float64 `json:"pspFeePercent"`
	VersionID          string  `json:"versionId,omitempty"`
}

func (h *PaymentHandler) createPagamento(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	var req createPagamentoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, err)
		return
	}
	p, err := h.createUC.Execute(r.Context(), portin.CreatePagamentoInput{
		EventoID:        req.EventoID,
		UsuarioID:       userID,
		Descricao:       req.Descricao,
		Valor:           req.Valor,
		MetodoPagamento: domain.MetodoPagamento(req.MetodoPagamento),
		DataVencimento:  req.DataVencimento,
		Observacao:      req.Observacao,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, pagamentoToResponse(p))
}

func (h *PaymentHandler) getPagamento(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")
	p, err := h.getUC.Execute(r.Context(), id, userID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, pagamentoToResponse(p))
}

func (h *PaymentHandler) listPagamentos(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	eventoID := r.URL.Query().Get("eventoId")
	pags, err := h.listUC.Execute(r.Context(), eventoID, userID)
	if err != nil {
		writeError(w, err)
		return
	}
	resp := make([]pagamentoResponse, len(pags))
	for i, p := range pags {
		p2 := p
		resp[i] = pagamentoToResponse(&p2)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *PaymentHandler) updatePagamento(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")
	var req updatePagamentoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, err)
		return
	}
	p, err := h.updateUC.Execute(r.Context(), id, userID, portin.UpdatePagamentoInput{
		Descricao:      req.Descricao,
		Valor:          req.Valor,
		DataVencimento: req.DataVencimento,
		Observacao:     req.Observacao,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, pagamentoToResponse(p))
}

func (h *PaymentHandler) deletePagamento(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")
	if err := h.deleteUC.Execute(r.Context(), id, userID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *PaymentHandler) confirmarPagamento(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")
	var req confirmarPagamentoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, err)
		return
	}
	p, err := h.confirmarUC.Execute(r.Context(), id, userID, portin.ConfirmarPagamentoInput{
		DataPagamento: req.DataPagamento,
		Comprovante:   req.Comprovante,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, pagamentoToResponse(p))
}

func (h *PaymentHandler) getConfig(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	eventoID := r.URL.Query().Get("eventoId")
	cfg, err := h.getCfgUC.Execute(r.Context(), eventoID, userID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (h *PaymentHandler) upsertConfig(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	var req upsertConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, err)
		return
	}
	methods := make([]domain.MetodoPagamento, len(req.MetodosPagamento))
	for i, m := range req.MetodosPagamento {
		methods[i] = domain.MetodoPagamento(m)
	}
	cfg, err := h.upsertCfgUC.Execute(r.Context(), portin.UpsertConfigPagamentoInput{
		EventoID:              req.EventoID,
		UsuarioID:             userID,
		MetodosPagamento:      methods,
		PrazoPagamento:        req.PrazoPagamento,
		ChavePix:              req.ChavePix,
		InstrucoesBoleto:      req.InstrucoesBoleto,
		PlatformFeePercent:    req.PlatformFeePercent,
		PspFeePercent:         req.PspFeePercent,
		PaymentProvider:       req.PaymentProvider,
		PaymentFrequency:      req.PaymentFrequency,
		PaymentReleaseTrigger: req.PaymentReleaseTrigger,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

// reaplicarFeePolicy handles POST /api/v1/admin/payments/reaplicar-fee-policy.
// Requires ADMIN role (enforced by RequireRole middleware in route registration).
// Mirrors Java ReaplicarFeePolicySnapshotUseCase.
func (h *PaymentHandler) reaplicarFeePolicy(w http.ResponseWriter, r *http.Request) {
	requesterID := middleware.UserIDFromContext(r.Context())

	var req reaplicarFeePolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo da requisição inválido"))
		return
	}

	result, err := h.reaplicarFeeUC.Execute(r.Context(), portin.ReaplicarFeePolicyNasConfigsInput{
		PlatformFeePercent: req.PlatformFeePercent,
		PspFeePercent:      req.PspFeePercent,
		VersionID:          req.VersionID,
		RequesterID:        requesterID,
	})
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"updatedCount": result.UpdatedCount,
		"message":      "Fee policy reapplied successfully",
	})
}

// ─── PagamentoMensal response helpers ────────────────────────────────────────

type pagamentoResponse struct {
	ID              string  `json:"id"`
	EventoID        string  `json:"eventoId"`
	UsuarioID       string  `json:"usuarioId"`
	Descricao       string  `json:"descricao"`
	Valor           float64 `json:"valor"`
	MetodoPagamento string  `json:"metodoPagamento"`
	Status          string  `json:"status"`
	DataVencimento  string  `json:"dataVencimento"`
	DataPagamento   *string `json:"dataPagamento,omitempty"`
	Observacao      string  `json:"observacao,omitempty"`
	Comprovante     string  `json:"comprovante,omitempty"`
	CriadoEm        string  `json:"criadoEm"`
	UpdatedAt        string  `json:"updatedAt"`
}

func pagamentoToResponse(p *domain.PagamentoMensal) pagamentoResponse {
	resp := pagamentoResponse{
		ID:              p.ID,
		EventoID:        p.EventoID,
		UsuarioID:       p.UsuarioID,
		Descricao:       p.Descricao,
		Valor:           p.Valor,
		MetodoPagamento: string(p.MetodoPagamento),
		Status:          string(p.Status),
		DataVencimento:  p.DataVencimento.Format("2006-01-02T15:04:05Z07:00"),
		Observacao:      p.Observacao,
		Comprovante:     p.Comprovante,
		CriadoEm:        p.CriadoEm.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:        p.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if p.DataPagamento != nil {
		s := p.DataPagamento.Format("2006-01-02T15:04:05Z07:00")
		resp.DataPagamento = &s
	}
	return resp
}

// ─── PaymentTransaction handlers ─────────────────────────────────────────────

// ─── Batch payment request / response ────────────────────────────────────────

// batchPaymentRequest is the body for POST /api/v1/payments/batch.
// Field names match the Java BatchPaymentRequest exactly.
type batchPaymentRequest struct {
	InstallmentIDs []string           `json:"installmentIds"`
	PaymentMethod  string             `json:"paymentMethod"`
	IdempotencyKey string             `json:"idempotencyKey"`
	CPF            string             `json:"cpf"`
	CreditCard     *creditCardRequest `json:"creditCard,omitempty"`
	SaveCard       bool               `json:"saveCard"`
	SavedCardID    string             `json:"savedCardId,omitempty"`
}

// processBatch handles POST /api/v1/payments/batch.
// Atomically charges multiple installments in one PaymentTransaction.
func (h *PaymentHandler) processBatch(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("autenticação necessária"))
		return
	}

	var req batchPaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo da requisição inválido"))
		return
	}

	if len(req.InstallmentIDs) == 0 {
		writeError(w, apierr.BadRequest("installmentIds não pode ser vazio"))
		return
	}

	in := portin.ProcessBatchPaymentInput{
		UserID:         userID,
		InstallmentIDs: req.InstallmentIDs,
		Method:         domain.PaymentMethod(req.PaymentMethod),
		IdempotencyKey: req.IdempotencyKey,
		CPF:            req.CPF,
		ClientIP:       clientIP(r),
		SaveCard:       req.SaveCard,
		SavedCardID:    req.SavedCardID,
	}

	if req.CreditCard != nil {
		in.CreditCard = &portin.CreditCardInput{
			HolderName:   req.CreditCard.HolderName,
			Number:       req.CreditCard.Number,
			ExpiryMonth:  req.CreditCard.ExpiryMonth,
			ExpiryYear:   req.CreditCard.ExpiryYear,
			CVV:          req.CreditCard.CVV,
			Installments: req.CreditCard.Installments,
			TokenRef:     req.CreditCard.TokenRef,
		}
	}

	resp, err := h.processBatchPaymentUC.Execute(r.Context(), in)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, resp)
}

// ─── Single payment request ───────────────────────────────────────────────────

// processPaymentRequest is the body for POST /api/v1/payments/process.
// Field names match the Java PaymentProcessRequest exactly.
type processPaymentRequest struct {
	EventID        string             `json:"eventId"`
	AmountCents    int64              `json:"amountCents"`
	PaymentMethod  string             `json:"paymentMethod"`
	IdempotencyKey string             `json:"idempotencyKey"`
	CPF            string             `json:"cpf"`
	CreditCard     *creditCardRequest `json:"creditCard,omitempty"`
	SaveCard       bool               `json:"saveCard"`
	SavedCardID    string             `json:"savedCardId,omitempty"`
}

type creditCardRequest struct {
	HolderName   string `json:"holderName"`
	Number       string `json:"number"`
	ExpiryMonth  string `json:"expiryMonth"`
	ExpiryYear   string `json:"expiryYear"`
	CVV          string `json:"cvv"`
	Installments int    `json:"installments"`
	TokenRef     string `json:"tokenRef,omitempty"`
}

// paymentMetadataResponse mirrors the Java PaymentResponse.metadata field names exactly.
// Note: snake_case JSON keys required for Java/BFF compatibility.
type paymentMetadataResponse struct {
	QrCodeText          string `json:"qr_code_text,omitempty"`
	QrCode              string `json:"qr_code,omitempty"`
	ExpiresAt           string `json:"expires_at,omitempty"`
	InvoiceURL          string `json:"invoice_url,omitempty"`
	LinhaDigitavel      string `json:"linha_digitavel,omitempty"`
	BoletoNossoNumero   string `json:"boleto_nosso_numero,omitempty"`
	PdfURL              string `json:"pdf_url,omitempty"`
	DueDate             string `json:"due_date,omitempty"`
	CardLast4           string `json:"card_last4,omitempty"`
	CardBrand           string `json:"card_brand,omitempty"`
	CreditCardToken     string `json:"credit_card_token,omitempty"`
	ReceiptURL          string `json:"receipt_url,omitempty"`
	Provider            string `json:"provider,omitempty"`
	BillingType         string `json:"billing_type,omitempty"`
}

// paymentResponse mirrors the Java PaymentResponse JSON exactly.
type paymentResponse struct {
	TransactionID         string                   `json:"transactionId"`
	ProviderTransactionID string                   `json:"providerTransactionId,omitempty"`
	Status                string                   `json:"status"`
	Method                string                   `json:"method"`
	Metadata              *paymentMetadataResponse `json:"metadata,omitempty"`
	ErrorMessage          string                   `json:"errorMessage,omitempty"`
}

// processPayment handles POST /api/v1/payments/process — creates a real Asaas charge.
func (h *PaymentHandler) processPayment(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("autenticação necessária"))
		return
	}

	var req processPaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo da requisição inválido"))
		return
	}

	in := portin.ProcessPaymentInput{
		UserID:         userID,
		EventID:        req.EventID,
		AmountCents:    req.AmountCents,
		Method:         domain.PaymentMethod(req.PaymentMethod),
		IdempotencyKey: req.IdempotencyKey,
		CPF:            req.CPF,
		ClientIP:       clientIP(r),
		SaveCard:       req.SaveCard,
		SavedCardID:    req.SavedCardID,
	}

	if req.CreditCard != nil {
		in.CreditCard = &portin.CreditCardInput{
			HolderName:   req.CreditCard.HolderName,
			Number:       req.CreditCard.Number,
			ExpiryMonth:  req.CreditCard.ExpiryMonth,
			ExpiryYear:   req.CreditCard.ExpiryYear,
			CVV:          req.CreditCard.CVV,
			Installments: req.CreditCard.Installments,
			TokenRef:     req.CreditCard.TokenRef,
		}
	}

	tx, err := h.processPaymentUC.Execute(r.Context(), in)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, transactionToResponse(tx))
}

// getTransaction handles GET /api/v1/payments/{transactionId}.
func (h *PaymentHandler) getTransaction(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	transactionID := chi.URLParam(r, "transactionId")

	tx, err := h.getTransactionUC.Execute(r.Context(), transactionID, userID)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, transactionToResponse(tx))
}

// listUserPayments handles GET /api/v1/payments/user — list for the JWT user.
func (h *PaymentHandler) listUserPayments(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("autenticação necessária"))
		return
	}

	filter := parseTransactionFilter(r)
	txs, err := h.listUserPaymentsUC.Execute(r.Context(), userID, filter)
	if err != nil {
		writeError(w, err)
		return
	}

	resp := make([]paymentResponse, len(txs))
	for i, tx := range txs {
		resp[i] = transactionToResponse(tx)
	}
	writeJSON(w, http.StatusOK, resp)
}

// listUserPaymentsByUserID handles GET /api/v1/payments/user/{userId}.
// Enforces ownership: requester must match the userId in the path (Java-compat).
func (h *PaymentHandler) listUserPaymentsByUserID(w http.ResponseWriter, r *http.Request) {
	requesterID := middleware.UserIDFromContext(r.Context())
	targetUserID := chi.URLParam(r, "userId")

	if requesterID != targetUserID {
		writeError(w, apierr.Forbidden("acesso negado: você só pode visualizar seus próprios pagamentos"))
		return
	}

	filter := parseTransactionFilter(r)
	txs, err := h.listUserPaymentsUC.Execute(r.Context(), targetUserID, filter)
	if err != nil {
		writeError(w, err)
		return
	}

	resp := make([]paymentResponse, len(txs))
	for i, tx := range txs {
		resp[i] = transactionToResponse(tx)
	}
	writeJSON(w, http.StatusOK, resp)
}

// sandboxSimulate handles POST /api/v1/payments/{transactionId}/sandbox-simulate.
// Requires ADMIN role (enforced by RequireRole middleware in route registration).
func (h *PaymentHandler) sandboxSimulate(w http.ResponseWriter, r *http.Request) {
	transactionID := chi.URLParam(r, "transactionId")
	userID := middleware.UserIDFromContext(r.Context())

	// Fetch transaction to get providerTransactionID and enforce basic ownership.
	tx, err := h.getTransactionUC.Execute(r.Context(), transactionID, userID)
	if err != nil {
		// Admin may simulate any transaction — skip ownership if 403.
		// Try direct fetch via admin path (currently no admin-level GetTransaction,
		// so we relax the ownership check for sandbox-simulate since admins are validated
		// by the middleware RequireRole("ADMIN") in the route registration).
		writeError(w, err)
		return
	}

	if tx.ProviderTransactionID == "" {
		writeError(w, apierr.BadRequest("transação não tem ID do provedor — não pode ser simulada"))
		return
	}

	if err := h.provider.SimulateSandboxReceive(r.Context(), tx.ProviderTransactionID); err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "SIMULATED",
		"message": "Sandbox receive simulated successfully",
	})
}

// ─── Mapping helpers ──────────────────────────────────────────────────────────

func transactionToResponse(tx *domain.PaymentTransaction) paymentResponse {
	resp := paymentResponse{
		TransactionID:         tx.ID,
		ProviderTransactionID: tx.ProviderTransactionID,
		Status:                string(tx.Status),
		Method:                string(tx.PaymentMethod),
	}

	if tx.FailureReason != "" {
		resp.ErrorMessage = tx.FailureReason
	}

	meta := &paymentMetadataResponse{
		Provider:    tx.Metadata.Provider,
		BillingType: tx.Metadata.BillingType,
		InvoiceURL:  tx.Metadata.InvoiceUrl,
	}

	// PIX
	meta.QrCodeText = tx.Metadata.PixQrCodeText
	meta.QrCode = tx.Metadata.PixQrCodeImage
	if tx.Metadata.PixExpiresAt != nil {
		meta.ExpiresAt = tx.Metadata.PixExpiresAt.Format(time.RFC3339)
	}

	// Boleto
	meta.LinhaDigitavel = tx.Metadata.BoletoDigitableLine
	meta.BoletoNossoNumero = tx.Metadata.BoletoCode
	meta.PdfURL = tx.Metadata.BoletoPdfUrl
	if tx.Metadata.BoletoDueDate != nil {
		meta.DueDate = tx.Metadata.BoletoDueDate.Format("2006-01-02")
	}

	// Credit card
	meta.CardLast4 = tx.Metadata.CardLast4
	meta.CardBrand = tx.Metadata.CardBrand
	meta.CreditCardToken = tx.Metadata.TokenizedCard

	// ExpiresAt (fallback from tx level)
	if meta.ExpiresAt == "" && tx.ExpiresAt != nil {
		meta.ExpiresAt = tx.ExpiresAt.Format(time.RFC3339)
	}

	// Only attach metadata if there's any meaningful content.
	if hasMetadata(meta) {
		resp.Metadata = meta
	}

	return resp
}

func hasMetadata(m *paymentMetadataResponse) bool {
	return m.QrCodeText != "" || m.LinhaDigitavel != "" || m.CardLast4 != "" ||
		m.InvoiceURL != "" || m.Provider != "" || m.BillingType != ""
}

// parseTransactionFilter reads optional query parameters for listing.
func parseTransactionFilter(r *http.Request) portin.ListUserPaymentsFilter {
	var filter portin.ListUserPaymentsFilter

	if s := r.URL.Query().Get("status"); s != "" {
		status := domain.TransactionStatus(s)
		filter.Status = &status
	}
	filter.EventoID = r.URL.Query().Get("eventoId")

	return filter
}

// clientIP extracts the real client IP from X-Forwarded-For or RemoteAddr,
// mirroring the Java ClientIpUtils behaviour.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For may contain multiple IPs; take the leftmost (original client).
		parts := strings.SplitN(xff, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	// Fall back to RemoteAddr (strips port).
	ip := r.RemoteAddr
	if colon := strings.LastIndex(ip, ":"); colon != -1 {
		ip = ip[:colon]
	}
	return ip
}
