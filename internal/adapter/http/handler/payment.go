package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/middleware"
	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
)

// PaymentHandler handles payment and payment config HTTP endpoints.
type PaymentHandler struct {
	createUC    portin.CreatePagamentoUseCase
	getUC       portin.GetPagamentoUseCase
	listUC      portin.ListPagamentosUseCase
	updateUC    portin.UpdatePagamentoUseCase
	deleteUC    portin.DeletePagamentoUseCase
	confirmarUC portin.ConfirmarPagamentoUseCase
	upsertCfgUC portin.UpsertConfigPagamentoUseCase
	getCfgUC    portin.GetConfigPagamentoUseCase
}

// NewPaymentHandler creates a new PaymentHandler.
func NewPaymentHandler(
	create portin.CreatePagamentoUseCase,
	get portin.GetPagamentoUseCase,
	list portin.ListPagamentosUseCase,
	update portin.UpdatePagamentoUseCase,
	del portin.DeletePagamentoUseCase,
	confirmar portin.ConfirmarPagamentoUseCase,
	upsertCfg portin.UpsertConfigPagamentoUseCase,
	getCfg portin.GetConfigPagamentoUseCase,
) *PaymentHandler {
	return &PaymentHandler{
		createUC:    create,
		getUC:       get,
		listUC:      list,
		updateUC:    update,
		deleteUC:    del,
		confirmarUC: confirmar,
		upsertCfgUC: upsertCfg,
		getCfgUC:    getCfg,
	}
}

// RegisterPaymentRoutes registers all payment routes on the router.
func (h *PaymentHandler) RegisterPaymentRoutes(r chi.Router) {
	r.Get("/api/payments", h.listPagamentos)
	r.Post("/api/payments", h.createPagamento)
	r.Get("/api/payments/{id}", h.getPagamento)
	r.Put("/api/payments/{id}", h.updatePagamento)
	r.Delete("/api/payments/{id}", h.deletePagamento)
	r.Post("/api/payments/{id}/confirmar", h.confirmarPagamento)
	r.Get("/api/payments/config", h.getConfig)
	r.Put("/api/payments/config", h.upsertConfig)

	r.Get("/api/v1/payments", h.listPagamentos)
	r.Post("/api/v1/payments", h.createPagamento)
	r.Get("/api/v1/payments/{id}", h.getPagamento)
	r.Put("/api/v1/payments/{id}", h.updatePagamento)
	r.Delete("/api/v1/payments/{id}", h.deletePagamento)
	r.Post("/api/v1/payments/{id}/confirmar", h.confirmarPagamento)
	r.Get("/api/v1/payments/config", h.getConfig)
	r.Put("/api/v1/payments/config", h.upsertConfig)
}

// ---- Request types ----

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
}

// ---- Handlers ----

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
		EventoID:         req.EventoID,
		UsuarioID:        userID,
		MetodosPagamento: methods,
		PrazoPagamento:   req.PrazoPagamento,
		ChavePix:         req.ChavePix,
		InstrucoesBoleto: req.InstrucoesBoleto,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

// ---- Response helpers ----

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
