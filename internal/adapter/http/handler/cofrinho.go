package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/middleware"
	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/cofrinho"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// CofrinhoHandler handles cofrinho contribution HTTP endpoints.
type CofrinhoHandler struct {
	createUC    portin.CreateContribuicaoUseCase
	listUC      portin.ListContribuicoesUseCase
	confirmarUC portin.ConfirmarContribuicaoUseCase
	removerUC   portin.RemoverContribuicaoUseCase
}

// NewCofrinhoHandler creates a new CofrinhoHandler.
func NewCofrinhoHandler(
	create portin.CreateContribuicaoUseCase,
	list portin.ListContribuicoesUseCase,
	confirmar portin.ConfirmarContribuicaoUseCase,
	remover portin.RemoverContribuicaoUseCase,
) *CofrinhoHandler {
	return &CofrinhoHandler{
		createUC:    create,
		listUC:      list,
		confirmarUC: confirmar,
		removerUC:   remover,
	}
}

// RegisterCofrinhoRoutes mounts cofrinho routes onto the given router.
func (h *CofrinhoHandler) RegisterCofrinhoRoutes(r chi.Router) {
	// Guest can submit a contribution (auth optional)
	r.Post("/api/v1/eventos/{eventoId}/cofrinho", h.createContribuicao)
	// Event owner lists contributions + saldo
	r.Get("/api/v1/eventos/{eventoId}/cofrinho", h.listContribuicoes)
	// Payment webhook confirms a contribution
	r.Post("/api/v1/cofrinho/{id}/confirmar", h.confirmarContribuicao)
	// Event owner removes a PENDENTE contribution
	r.Delete("/api/v1/baby-shower/cofrinho/{id}", h.removerContribuicao)
}

// ---- Request / Response DTOs ----

type createContribuicaoRequest struct {
	GuestID  string `json:"guestId"`
	Nome     string `json:"nome"`
	Mensagem string `json:"mensagem"`
	Valor    int64  `json:"valor"` // centavos
}

type confirmarContribuicaoRequest struct {
	WebhookPaymentID string `json:"webhookPaymentId"`
}

type cofrinhoContribuicaoResponse struct {
	ID               string    `json:"id"`
	EventoID         string    `json:"eventoId"`
	GuestID          string    `json:"guestId"`
	Nome             string    `json:"nome"`
	Mensagem         string    `json:"mensagem"`
	Valor            int64     `json:"valor"`
	Status           string    `json:"status"`
	PIXQRCode        string    `json:"pixQrCode,omitempty"`
	WebhookPaymentID string    `json:"webhookPaymentId,omitempty"`
	CriadoEm        time.Time `json:"criadoEm"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

// saldoCofrinhoResponse mirrors Java's SaldoCofrinhoResponse record.
// Only CONFIRMADO contributions are counted in totals.
type saldoCofrinhoResponse struct {
	EventoID                string `json:"eventoId"`
	TotalConfirmadoCents    int64  `json:"totalConfirmadoCents"`
	QuantidadeContribuicoes int64  `json:"quantidadeContribuicoes"`
}

// cofrinhoSummaryResponse mirrors Java's CofrinhoSummaryResponse record.
// GET /api/v1/eventos/{eventoId}/cofrinho returns this combined view.
type cofrinhoSummaryResponse struct {
	Saldo         saldoCofrinhoResponse          `json:"saldo"`
	Contribuicoes []cofrinhoContribuicaoResponse `json:"contribuicoes"`
}

func toContribuicaoResponse(c *domain.CofrinhoContribuicao) cofrinhoContribuicaoResponse {
	return cofrinhoContribuicaoResponse{
		ID:               c.ID,
		EventoID:         c.EventoID,
		GuestID:          c.GuestID,
		Nome:             c.Nome,
		Mensagem:         c.Mensagem,
		Valor:            c.Valor,
		Status:           string(c.Status),
		PIXQRCode:        c.PIXQRCode,
		WebhookPaymentID: c.WebhookPaymentID,
		CriadoEm:        c.CriadoEm,
		UpdatedAt:        c.UpdatedAt,
	}
}

// ---- Handlers ----

// createContribuicao handles POST /api/v1/eventos/{eventoId}/cofrinho
// Auth is optional: logged-in users have their ID used as guestID by default.
func (h *CofrinhoHandler) createContribuicao(w http.ResponseWriter, r *http.Request) {
	eventoID := chi.URLParam(r, "eventoId")

	var req createContribuicaoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo de requisição inválido"))
		return
	}

	// If authenticated, prefer the authenticated user ID as guestID.
	if userID := middleware.UserIDFromContext(r.Context()); userID != "" && req.GuestID == "" {
		req.GuestID = userID
	}

	in := portin.CreateContribuicaoInput{
		EventoID: eventoID,
		GuestID:  req.GuestID,
		Nome:     req.Nome,
		Mensagem: req.Mensagem,
		Valor:    req.Valor,
	}

	c, err := h.createUC.Execute(r.Context(), in)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, toContribuicaoResponse(c))
}

// listContribuicoes handles GET /api/v1/eventos/{eventoId}/cofrinho
// Returns a summary matching Java's CofrinhoSummaryResponse: saldo + full list.
// Only CONFIRMADO contributions are counted toward saldo.totalConfirmadoCents.
func (h *CofrinhoHandler) listContribuicoes(w http.ResponseWriter, r *http.Request) {
	eventoID := chi.URLParam(r, "eventoId")

	contribuicoes, err := h.listUC.Execute(r.Context(), eventoID)
	if err != nil {
		writeError(w, err)
		return
	}

	// Compute saldo inline — mirrors BuscarSaldoCofrinhoUseCaseImpl behaviour.
	var totalConfirmadoCents, qtdConfirmadas int64
	for _, c := range contribuicoes {
		if c.Status == domain.StatusConfirmado {
			totalConfirmadoCents += c.Valor
			qtdConfirmadas++
		}
	}

	items := make([]cofrinhoContribuicaoResponse, len(contribuicoes))
	for i, c := range contribuicoes {
		items[i] = toContribuicaoResponse(c)
	}

	writeJSON(w, http.StatusOK, cofrinhoSummaryResponse{
		Saldo: saldoCofrinhoResponse{
			EventoID:                eventoID,
			TotalConfirmadoCents:    totalConfirmadoCents,
			QuantidadeContribuicoes: qtdConfirmadas,
		},
		Contribuicoes: items,
	})
}

// confirmarContribuicao handles POST /api/v1/cofrinho/{id}/confirmar
func (h *CofrinhoHandler) confirmarContribuicao(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req confirmarContribuicaoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo de requisição inválido"))
		return
	}

	in := portin.ConfirmarContribuicaoInput{
		ContribuicaoID:   id,
		WebhookPaymentID: req.WebhookPaymentID,
	}

	c, err := h.confirmarUC.Execute(r.Context(), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toContribuicaoResponse(c))
}

// removerContribuicao handles DELETE /api/v1/baby-shower/cofrinho/{id}
// Only contributions with status PENDENTE can be removed.
// CONFIRMADO contributions require a refund flow via the payment provider.
func (h *CofrinhoHandler) removerContribuicao(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.removerUC.Execute(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
