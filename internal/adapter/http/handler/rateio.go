package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/middleware"
	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/rateio"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
)

// RateioHandler handles cost split (rateio) HTTP endpoints.
type RateioHandler struct {
	createUC    portin.CreateRateioUseCase
	getUC       portin.GetRateioUseCase
	listUC      portin.ListRateiosUseCase
	updateUC    portin.UpdateRateioUseCase
	deleteUC    portin.DeleteRateioUseCase
	previewUC   portin.PreviewRateioUseCase
	fecharUC    portin.FecharRateioUseCase
	fechsUC     portin.GetFechamentosUseCase
}

// NewRateioHandler creates a new RateioHandler.
func NewRateioHandler(
	create portin.CreateRateioUseCase,
	get portin.GetRateioUseCase,
	list portin.ListRateiosUseCase,
	update portin.UpdateRateioUseCase,
	del portin.DeleteRateioUseCase,
	preview portin.PreviewRateioUseCase,
	fechar portin.FecharRateioUseCase,
	fechs portin.GetFechamentosUseCase,
) *RateioHandler {
	return &RateioHandler{
		createUC:  create,
		getUC:     get,
		listUC:    list,
		updateUC:  update,
		deleteUC:  del,
		previewUC: preview,
		fecharUC:  fechar,
		fechsUC:   fechs,
	}
}

// RegisterRateioRoutes registers all rateio routes on the router.
func (h *RateioHandler) RegisterRateioRoutes(r chi.Router) {
	// Mounted under protected JWT group
	r.Get("/api/rateios", h.listRateios)
	r.Post("/api/rateios", h.createRateio)
	r.Get("/api/rateios/{id}", h.getRateio)
	r.Put("/api/rateios/{id}", h.updateRateio)
	r.Delete("/api/rateios/{id}", h.deleteRateio)
	r.Get("/api/rateios/{id}/preview", h.previewRateio)
	r.Get("/api/rateios/{id}/previa", h.previewRateio)  // Java-compat alias
	r.Post("/api/rateios/{id}/fechar", h.fecharRateio)
	r.Get("/api/rateios/{id}/fechamentos", h.getFechamentos)

	// BFF-compatible paths
	r.Get("/api/v1/rateios", h.listRateios)
	r.Post("/api/v1/rateios", h.createRateio)
	r.Get("/api/v1/rateios/by-evento/{eventoId}", h.listRateiosByEvento)
	r.Get("/api/v1/rateios/{id}", h.getRateio)
	r.Put("/api/v1/rateios/{id}", h.updateRateio)
	r.Delete("/api/v1/rateios/{id}", h.deleteRateio)
	r.Get("/api/v1/rateios/{id}/preview", h.previewRateio)
	r.Get("/api/v1/rateios/{id}/previa", h.previewRateio)  // Java-compat alias
	r.Post("/api/v1/rateios/{id}/fechar", h.fecharRateio)
	r.Get("/api/v1/rateios/{id}/fechamentos", h.getFechamentos)
	// Legacy paths (Java-compat)
	r.Get("/api/rateios/by-evento/{eventoId}", h.listRateiosByEvento)
	r.Get("/api/rateios/v1/evento/{eventoId}", h.listRateiosByEvento)
}

// ---- Request/Response types ----

type createRateioRequest struct {
	EventoID            string                    `json:"eventoId"`
	Tipo                string                    `json:"tipo"`
	Nome                string                    `json:"nome"`      // Java parity: accepts nome as primary name field
	Descricao           string                    `json:"descricao"`
	ValorTotal          float64                   `json:"valorTotal"`
	NumeroParticipantes int                       `json:"numeroParticipantes"`
	Itens               []createRateioItemRequest `json:"itens"`
}

type createRateioItemRequest struct {
	Descricao  string  `json:"descricao"`
	Valor      float64 `json:"valor"`
	Quantidade int     `json:"quantidade"`
}

type updateRateioRequest struct {
	Descricao           *string                   `json:"descricao"`
	ValorTotal          *float64                  `json:"valorTotal"`
	NumeroParticipantes *int                      `json:"numeroParticipantes"`
	Itens               []createRateioItemRequest `json:"itens"`
}

type previewQueryRequest struct {
	Participantes []string `json:"participantes"`
}

type fecharRateioRequest struct {
	Participantes []fecharPartRequest `json:"participantes"`
}

type fecharPartRequest struct {
	UsuarioID  string  `json:"usuarioId"`
	Valor      float64 `json:"valor"`
	Percentual float64 `json:"percentual"`
}

// ---- Handlers ----

func (h *RateioHandler) createRateio(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	var req createRateioRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, err)
		return
	}
	itens := make([]portin.CreateRateioItemInput, len(req.Itens))
	for i, it := range req.Itens {
		itens[i] = portin.CreateRateioItemInput{
			Descricao:  it.Descricao,
			Valor:      it.Valor,
			Quantidade: it.Quantidade,
		}
	}
	in := portin.CreateRateioInput{
		EventoID:            req.EventoID,
		UsuarioID:           userID,
		Tipo:                domain.TipoRateio(req.Tipo),
		Descricao:           func() string { if req.Nome != "" { return req.Nome }; return req.Descricao }(),
		ValorTotal:          req.ValorTotal,
		NumeroParticipantes: req.NumeroParticipantes,
		Itens:               itens,
	}
	rat, err := h.createUC.Execute(r.Context(), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, rateioToResponse(rat))
}

func (h *RateioHandler) getRateio(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")
	rat, err := h.getUC.Execute(r.Context(), id, userID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rateioToResponse(rat))
}

func (h *RateioHandler) listRateios(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	eventoID := r.URL.Query().Get("eventoId")
	rats, err := h.listUC.Execute(r.Context(), eventoID, userID)
	if err != nil {
		writeError(w, err)
		return
	}
	resp := make([]rateioResponse, len(rats))
	for i, rat := range rats {
		r2 := rat // copy
		resp[i] = rateioToResponse(&r2)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *RateioHandler) listRateiosByEvento(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	eventoID := chi.URLParam(r, "eventoId")
	rats, err := h.listUC.Execute(r.Context(), eventoID, userID)
	if err != nil {
		writeError(w, err)
		return
	}

	rateioItems := make([]rateioResponse, len(rats))
	for i, rat := range rats {
		r2 := rat // copy
		rateioItems[i] = rateioToResponse(&r2)
	}
	writeJSON(w, http.StatusOK, rateioItems)
}

func (h *RateioHandler) updateRateio(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")
	var req updateRateioRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, err)
		return
	}
	var itens []portin.CreateRateioItemInput
	for _, it := range req.Itens {
		itens = append(itens, portin.CreateRateioItemInput{
			Descricao:  it.Descricao,
			Valor:      it.Valor,
			Quantidade: it.Quantidade,
		})
	}
	in := portin.UpdateRateioInput{
		Descricao:           req.Descricao,
		ValorTotal:          req.ValorTotal,
		NumeroParticipantes: req.NumeroParticipantes,
		Itens:               itens,
	}
	rat, err := h.updateUC.Execute(r.Context(), id, userID, in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rateioToResponse(rat))
}

func (h *RateioHandler) deleteRateio(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")
	if err := h.deleteUC.Execute(r.Context(), id, userID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *RateioHandler) previewRateio(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")
	var req previewQueryRequest
	_ = json.NewDecoder(r.Body).Decode(&req) // optional body
	result, err := h.previewUC.Execute(r.Context(), id, userID, req.Participantes)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *RateioHandler) fecharRateio(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")
	var req fecharRateioRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, err)
		return
	}
	parts := make([]portin.FecharParticipanteInput, len(req.Participantes))
	for i, p := range req.Participantes {
		parts[i] = portin.FecharParticipanteInput{
			UsuarioID:  p.UsuarioID,
			Valor:      p.Valor,
			Percentual: p.Percentual,
		}
	}
	f, err := h.fecharUC.Execute(r.Context(), id, userID, portin.FecharRateioInput{Participantes: parts})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, f)
}

func (h *RateioHandler) getFechamentos(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")
	fechs, err := h.fechsUC.Execute(r.Context(), id, userID)
	if err != nil {
		writeError(w, err)
		return
	}
	// Ensure non-null response (nil slice serializes to null in Go)
	if fechs == nil {
		fechs = []domain.RateioFechamento{}
	}
	writeJSON(w, http.StatusOK, fechs)
}

// ---- Response helpers ----

type rateioItemResponse struct {
	ID         string  `json:"id"`
	Descricao  string  `json:"descricao"`
	Valor      float64 `json:"valor"`
	Quantidade int     `json:"quantidade"`
	Total      float64 `json:"total"`
}

type rateioResponse struct {
	ID                  string               `json:"id"`
	EventoID            string               `json:"eventoId"`
	UsuarioID           string               `json:"usuarioId"`
	Nome                string               `json:"nome"`
	Tipo                string               `json:"tipo"`
	Status              string               `json:"status"`
	Descricao           string               `json:"descricao"`
	ValorTotal          float64              `json:"valorTotal"`
	NumeroParticipantes int                  `json:"numeroParticipantes"`
	Itens               []rateioItemResponse `json:"itens"`
	CriadoEm            string               `json:"criadoEm"`
	UpdatedAt           string               `json:"updatedAt"`
}

func rateioToResponse(r *domain.Rateio) rateioResponse {
	itens := make([]rateioItemResponse, len(r.Itens))
	for i, it := range r.Itens {
		itens[i] = rateioItemResponse{
			ID:         it.ID,
			Descricao:  it.Descricao,
			Valor:      it.Valor,
			Quantidade: it.Quantidade,
			Total:      it.Total,
		}
	}
	return rateioResponse{
		ID:                  r.ID,
		EventoID:            r.EventoID,
		UsuarioID:           r.UsuarioID,
		Nome:                r.Descricao, // Java parity: nome field aliases descricao
		Tipo:                string(r.Tipo),
		Status:              string(r.Status),
		Descricao:           r.Descricao,
		ValorTotal:          r.ValorTotal,
		NumeroParticipantes: r.NumeroParticipantes,
		Itens:               itens,
		CriadoEm:            r.CriadoEm.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:           r.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
