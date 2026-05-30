package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/middleware"
	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/event"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// DraftHandler handles event draft HTTP requests.
type DraftHandler struct {
	createUC  portin.CreateDraftUseCase
	getUC     portin.GetDraftUseCase
	listUC    portin.ListDraftsUseCase
	updateUC  portin.UpdateDraftUseCase
	deleteUC  portin.DeleteDraftUseCase
	publishUC portin.PublishDraftUseCase
}

// NewDraftHandler creates a new DraftHandler.
func NewDraftHandler(
	create portin.CreateDraftUseCase,
	get portin.GetDraftUseCase,
	list portin.ListDraftsUseCase,
	update portin.UpdateDraftUseCase,
	del portin.DeleteDraftUseCase,
	publish portin.PublishDraftUseCase,
) *DraftHandler {
	return &DraftHandler{
		createUC:  create,
		getUC:     get,
		listUC:    list,
		updateUC:  update,
		deleteUC:  del,
		publishUC: publish,
	}
}

// RegisterDraftRoutes mounts draft routes onto the given router.
func (h *DraftHandler) RegisterDraftRoutes(r chi.Router) {
	// Both path prefixes for compatibility with Java BFF expectations
	r.Get("/api/v1/drafts", h.listDrafts)
	r.Post("/api/v1/drafts", h.createDraft)
	r.Get("/api/v1/drafts/{id}", h.getDraft)
	r.Put("/api/v1/drafts/{id}", h.updateDraft)
	r.Delete("/api/v1/drafts/{id}", h.deleteDraft)
	r.Post("/api/v1/drafts/{id}/publish", h.publishDraft)

	// Alias used by BFF
	r.Get("/api/v1/eventos-draft", h.listDrafts)
	r.Post("/api/v1/eventos-draft", h.createDraft)
	r.Get("/api/v1/eventos-draft/{id}", h.getDraft)
	r.Put("/api/v1/eventos-draft/{id}", h.updateDraft)
	r.Delete("/api/v1/eventos-draft/{id}", h.deleteDraft)
	r.Post("/api/v1/eventos-draft/{id}/publish", h.publishDraft)
}

// ---- DTOs ----

type draftRateioItemRequest struct {
	Descricao  string  `json:"descricao"`
	Valor      float64 `json:"valor"`
	Quantidade int     `json:"quantidade"`
}

type updateDraftRequest struct {
	Nome      *string    `json:"nome"`
	Tipo      *string    `json:"tipo"`
	Data      *time.Time `json:"data"`
	Descricao *string    `json:"descricao"`
	Local     *string    `json:"local"`

	ConvidadosIDs      []string `json:"convidadosIds"`
	PoliticaConvidados *string  `json:"politicaConvidados"`
	LimiteConvidados   *int     `json:"limiteConvidados"`

	RateiosHabilitado *bool                    `json:"rateiosHabilitado"`
	RateiosItens      []draftRateioItemRequest  `json:"rateiosItens"`
	TipoDivisaoRateio *string                  `json:"tipoDivisaoRateio"`

	PagamentosHabilitado *bool      `json:"pagamentosHabilitado"`
	MetodosPagamento     []string   `json:"metodosPagamento"`
	PrazoPagamento       *time.Time `json:"prazoPagamento"`

	RegrasCustomizadas   *string `json:"regrasCustomizadas"`
	PoliticaCancelamento *string `json:"politicaCancelamento"`

	EtapaAtual      *int  `json:"etapaAtual"`
	EtapasCompletas []int `json:"etapasCompletas"`

	LastReadAt *time.Time `json:"lastReadAt"`
}

type draftRateioItemResponse struct {
	Descricao  string  `json:"descricao"`
	Valor      float64 `json:"valor"`
	Quantidade int     `json:"quantidade"`
}

type draftResponse struct {
	ID        string `json:"id"`
	UsuarioID string `json:"usuarioId"`

	Nome      string     `json:"nome"`
	Tipo      string     `json:"tipo"`
	Data      *time.Time `json:"data"`
	Descricao string     `json:"descricao"`
	Local     string     `json:"local"`

	ConvidadosIDs      []string `json:"convidadosIds"`
	PoliticaConvidados string   `json:"politicaConvidados"`
	LimiteConvidados   *int     `json:"limiteConvidados"`

	RateiosHabilitado bool                      `json:"rateiosHabilitado"`
	RateiosItens      []draftRateioItemResponse  `json:"rateiosItens"`
	TipoDivisaoRateio string                    `json:"tipoDivisaoRateio"`

	PagamentosHabilitado bool       `json:"pagamentosHabilitado"`
	MetodosPagamento     []string   `json:"metodosPagamento"`
	PrazoPagamento       *time.Time `json:"prazoPagamento"`

	RegrasCustomizadas   string `json:"regrasCustomizadas"`
	PoliticaCancelamento string `json:"politicaCancelamento"`

	EtapaAtual      int       `json:"etapaAtual"`
	EtapasCompletas []int     `json:"etapasCompletas"`
	CriadoEm        time.Time `json:"criadoEm"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

// ---- handlers ----

func (h *DraftHandler) listDrafts(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("autenticação necessária"))
		return
	}
	drafts, err := h.listUC.Execute(r.Context(), userID)
	if err != nil {
		writeError(w, err)
		return
	}
	resp := make([]draftResponse, len(drafts))
	for i, d := range drafts {
		resp[i] = draftToResponse(&d)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *DraftHandler) createDraft(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("autenticação necessária"))
		return
	}
	d, err := h.createUC.Execute(r.Context(), userID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, draftToResponse(d))
}

func (h *DraftHandler) getDraft(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("autenticação necessária"))
		return
	}
	d, err := h.getUC.Execute(r.Context(), id, userID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, draftToResponse(d))
}

func (h *DraftHandler) updateDraft(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("autenticação necessária"))
		return
	}
	var req updateDraftRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo da requisição inválido"))
		return
	}

	itens := make([]portin.DraftRateioItem, len(req.RateiosItens))
	for i, ri := range req.RateiosItens {
		itens[i] = portin.DraftRateioItem{
			Descricao:  ri.Descricao,
			Valor:      ri.Valor,
			Quantidade: ri.Quantidade,
		}
	}

	in := portin.UpsertDraftInput{
		Nome:                 req.Nome,
		Tipo:                 req.Tipo,
		Data:                 req.Data,
		Descricao:            req.Descricao,
		Local:                req.Local,
		ConvidadosIDs:        req.ConvidadosIDs,
		PoliticaConvidados:   req.PoliticaConvidados,
		LimiteConvidados:     req.LimiteConvidados,
		RateiosHabilitado:    req.RateiosHabilitado,
		RateiosItens:         itens,
		TipoDivisaoRateio:    req.TipoDivisaoRateio,
		PagamentosHabilitado: req.PagamentosHabilitado,
		MetodosPagamento:     req.MetodosPagamento,
		PrazoPagamento:       req.PrazoPagamento,
		RegrasCustomizadas:   req.RegrasCustomizadas,
		PoliticaCancelamento: req.PoliticaCancelamento,
		EtapaAtual:           req.EtapaAtual,
		EtapasCompletas:      req.EtapasCompletas,
		LastReadAt:           req.LastReadAt,
	}
	d, err := h.updateUC.Execute(r.Context(), id, userID, in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, draftToResponse(d))
}

func (h *DraftHandler) deleteDraft(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("autenticação necessária"))
		return
	}
	if err := h.deleteUC.Execute(r.Context(), id, userID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *DraftHandler) publishDraft(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("autenticação necessária"))
		return
	}
	evt, err := h.publishUC.Execute(r.Context(), id, userID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, eventoToResponse(evt))
}

// ---- helpers ----

func draftToResponse(d *domain.EventoDraft) draftResponse {
	itens := make([]draftRateioItemResponse, len(d.RateiosItens))
	for i, ri := range d.RateiosItens {
		itens[i] = draftRateioItemResponse{
			Descricao:  ri.Descricao,
			Valor:      ri.Valor,
			Quantidade: ri.Quantidade,
		}
	}
	return draftResponse{
		ID:                   d.ID,
		UsuarioID:            d.UsuarioID,
		Nome:                 d.Nome,
		Tipo:                 d.Tipo,
		Data:                 d.Data,
		Descricao:            d.Descricao,
		Local:                d.Local,
		ConvidadosIDs:        d.ConvidadosIDs,
		PoliticaConvidados:   d.PoliticaConvidados,
		LimiteConvidados:     d.LimiteConvidados,
		RateiosHabilitado:    d.RateiosHabilitado,
		RateiosItens:         itens,
		TipoDivisaoRateio:    d.TipoDivisaoRateio,
		PagamentosHabilitado: d.PagamentosHabilitado,
		MetodosPagamento:     d.MetodosPagamento,
		PrazoPagamento:       d.PrazoPagamento,
		RegrasCustomizadas:   d.RegrasCustomizadas,
		PoliticaCancelamento: d.PoliticaCancelamento,
		EtapaAtual:           d.EtapaAtual,
		EtapasCompletas:      d.EtapasCompletas,
		CriadoEm:             d.CriadoEm,
		UpdatedAt:            d.UpdatedAt,
	}
}
