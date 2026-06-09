package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/middleware"
	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/event"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// EventHandler handles event-related HTTP requests.
type EventHandler struct {
	createUC        portin.CreateEventoUseCase
	getUC           portin.GetEventoUseCase
	listUC          portin.ListEventosUseCase
	updateUC        portin.UpdateEventoUseCase
	deleteUC        portin.DeleteEventoUseCase
	listByUsuarioUC portin.ListEventosByUsuarioUseCase
	addConvidadosUC portin.AddConvidadosUseCase
}

// NewEventHandler creates a new EventHandler.
func NewEventHandler(
	create portin.CreateEventoUseCase,
	get portin.GetEventoUseCase,
	list portin.ListEventosUseCase,
	update portin.UpdateEventoUseCase,
	del portin.DeleteEventoUseCase,
	listByUsuario portin.ListEventosByUsuarioUseCase,
	addConvidados portin.AddConvidadosUseCase,
) *EventHandler {
	return &EventHandler{
		createUC:        create,
		getUC:           get,
		listUC:          list,
		updateUC:        update,
		deleteUC:        del,
		listByUsuarioUC: listByUsuario,
		addConvidadosUC: addConvidados,
	}
}

// RegisterEventRoutes mounts event routes onto the given router.
func (h *EventHandler) RegisterEventRoutes(r chi.Router) {
	r.Get("/api/eventos", h.listEvento)
	r.Post("/api/eventos", h.createEvento)
	// /usuario/{usuarioId} must be registered before /{id} to avoid chi matching conflict
	r.Get("/api/eventos/usuario/{usuarioId}", h.listEventoByUsuario)
	// v1 versioned alias — same handler, identical behaviour
	r.Get("/api/eventos/v1/eventos/usuario/{userId}", h.listEventoByUsuarioV1)
	r.Get("/api/eventos/{id}", h.getEvento)
	r.Put("/api/eventos/{id}", h.updateEvento)
	r.Delete("/api/eventos/{id}", h.deleteEvento)
	// Convidados
	r.Post("/api/eventos/v1/eventos/{eventoId}/convidados", h.addConvidados)
}

// ---- request/response DTOs ----

type createEventoRequest struct {
	Nome                 string     `json:"nome"`
	Tipo                 string     `json:"tipo"`
	Data                 time.Time  `json:"data"`
	Descricao            string     `json:"descricao"`
	Local                string     `json:"local"`
	FotoURL              string     `json:"fotoUrl"`
	ConvidadosIDs        []string   `json:"convidadosIds"`
	PoliticaConvidados   string     `json:"politicaConvidados"`
	LimiteConvidados     *int       `json:"limiteConvidados"`
	RateiosHabilitado    bool       `json:"rateiosHabilitado"`
	TipoDivisaoRateio    string     `json:"tipoDivisaoRateio"`
	PagamentosHabilitado bool       `json:"pagamentosHabilitado"`
	MetodosPagamento     []string   `json:"metodosPagamento"`
	PrazoPagamento       *time.Time `json:"prazoPagamento"`
	RegrasCustomizadas   string     `json:"regrasCustomizadas"`
	PoliticaCancelamento string     `json:"politicaCancelamento"`
}

type updateEventoRequest struct {
	Nome                 string     `json:"nome"`
	Tipo                 string     `json:"tipo"`
	Data                 *time.Time `json:"data"`
	Descricao            string     `json:"descricao"`
	Local                string     `json:"local"`
	FotoURL              string     `json:"fotoUrl"`
	PoliticaConvidados   string     `json:"politicaConvidados"`
	LimiteConvidados     *int       `json:"limiteConvidados"`
	RateiosHabilitado    *bool      `json:"rateiosHabilitado"`
	TipoDivisaoRateio    string     `json:"tipoDivisaoRateio"`
	PagamentosHabilitado *bool      `json:"pagamentosHabilitado"`
	MetodosPagamento     []string   `json:"metodosPagamento"`
	PrazoPagamento       *time.Time `json:"prazoPagamento"`
	RegrasCustomizadas   string     `json:"regrasCustomizadas"`
	PoliticaCancelamento string     `json:"politicaCancelamento"`
}

type eventoResponse struct {
	ID                   string     `json:"id"`
	UsuarioID            string     `json:"usuarioId"`
	Nome                 string     `json:"nome"`
	Tipo                 string     `json:"tipo"`
	Data                 time.Time  `json:"data"`
	Descricao            string     `json:"descricao"`
	Local                string     `json:"local"`
	FotoURL              string     `json:"fotoUrl"`
	Status               string     `json:"status"`
	ConvidadosIDs        []string   `json:"convidadosIds"`
	PoliticaConvidados   string     `json:"politicaConvidados"`
	LimiteConvidados     *int       `json:"limiteConvidados"`
	RateiosHabilitado    bool       `json:"rateiosHabilitado"`
	TipoDivisaoRateio    string     `json:"tipoDivisaoRateio"`
	PagamentosHabilitado bool       `json:"pagamentosHabilitado"`
	MetodosPagamento     []string   `json:"metodosPagamento"`
	PrazoPagamento       *time.Time `json:"prazoPagamento"`
	RegrasCustomizadas   string     `json:"regrasCustomizadas"`
	PoliticaCancelamento string     `json:"politicaCancelamento"`
	CriadoEm             time.Time  `json:"criadoEm"`
	UpdatedAt            time.Time  `json:"updatedAt"`
}

type addConvidadosRequest struct {
	Convidados []convidadoRequest `json:"convidados"`
}

type convidadoRequest struct {
	Telefone string `json:"telefone"`
	Nome     string `json:"nome"`
}

// ---- handlers ----

func (h *EventHandler) listEvento(w http.ResponseWriter, r *http.Request) {
	page := queryIntParam(r, "page", 1)
	pageSize := queryIntParam(r, "pageSize", 20)
	// Security: ignore usuarioId query param — use authenticated user from JWT.
	// Use /api/eventos/usuario/{usuarioId} for user-filtered listing.
	eventos, total, err := h.listUC.Execute(r.Context(), nil, page, pageSize)
	if err != nil {
		writeError(w, err)
		return
	}
	resp := make([]eventoResponse, len(eventos))
	for i, e := range eventos {
		resp[i] = eventoToResponse(&e)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": resp, "total": total, "page": page, "pageSize": pageSize})
}

func (h *EventHandler) listEventoByUsuario(w http.ResponseWriter, r *http.Request) {
	requesterID := middleware.UserIDFromContext(r.Context())
	if requesterID == "" {
		writeError(w, apierr.Unauthorized("autenticação necessária"))
		return
	}
	usuarioID := chi.URLParam(r, "usuarioId")
	if usuarioID == "" {
		writeError(w, apierr.BadRequest("usuarioId é obrigatório"))
		return
	}

	var cursor *string
	if c := r.URL.Query().Get("cursor"); c != "" {
		cursor = &c
	}
	var status, tipo *string
	if s := r.URL.Query().Get("status"); s != "" {
		status = &s
	}
	if t := r.URL.Query().Get("tipo"); t != "" {
		tipo = &t
	}
	var dataInicioGte, dataInicioLte *time.Time
	if v := r.URL.Query().Get("dataInicioGte"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			dataInicioGte = &t
		}
	}
	if v := r.URL.Query().Get("dataInicioLte"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			dataInicioLte = &t
		}
	}
	limit := queryIntParam(r, "limit", 20)

	in := portin.ListEventosByUsuarioInput{
		UsuarioID:     usuarioID,
		RequesterID:   requesterID,
		Status:        status,
		Tipo:          tipo,
		DataInicioGte: dataInicioGte,
		DataInicioLte: dataInicioLte,
		Cursor:        cursor,
		Limit:         limit,
	}
	page, err := h.listByUsuarioUC.Execute(r.Context(), in)
	if err != nil {
		writeError(w, err)
		return
	}
	resp := make([]eventoResponse, len(page.Eventos))
	for i, e := range page.Eventos {
		e2 := e
		resp[i] = eventoToResponse(&e2)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"eventos":     resp,
		"total":       page.Total,
		"nextCursor":  page.NextCursor,
		"hasNextPage": page.HasNextPage,
		"limit":       page.Limit,
	})
}

func (h *EventHandler) createEvento(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("autenticação necessária"))
		return
	}
	var req createEventoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo da requisição inválido"))
		return
	}
	in := portin.CreateEventoInput{
		UsuarioID:            userID,
		Nome:                 req.Nome,
		Tipo:                 req.Tipo,
		Data:                 req.Data,
		Descricao:            req.Descricao,
		Local:                req.Local,
		FotoURL:              req.FotoURL,
		ConvidadosIDs:        req.ConvidadosIDs,
		PoliticaConvidados:   req.PoliticaConvidados,
		LimiteConvidados:     req.LimiteConvidados,
		RateiosHabilitado:    req.RateiosHabilitado,
		TipoDivisaoRateio:    req.TipoDivisaoRateio,
		PagamentosHabilitado: req.PagamentosHabilitado,
		MetodosPagamento:     req.MetodosPagamento,
		PrazoPagamento:       req.PrazoPagamento,
		RegrasCustomizadas:   req.RegrasCustomizadas,
		PoliticaCancelamento: req.PoliticaCancelamento,
	}
	evt, err := h.createUC.Execute(r.Context(), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, eventoToResponse(evt))
}

func (h *EventHandler) getEvento(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	evt, err := h.getUC.Execute(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, eventoToResponse(evt))
}

func (h *EventHandler) updateEvento(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("autenticação necessária"))
		return
	}
	var req updateEventoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo da requisição inválido"))
		return
	}
	in := portin.UpdateEventoInput{
		Nome:                 req.Nome,
		Tipo:                 req.Tipo,
		Data:                 req.Data,
		Descricao:            req.Descricao,
		Local:                req.Local,
		FotoURL:              req.FotoURL,
		PoliticaConvidados:   req.PoliticaConvidados,
		LimiteConvidados:     req.LimiteConvidados,
		RateiosHabilitado:    req.RateiosHabilitado,
		TipoDivisaoRateio:    req.TipoDivisaoRateio,
		PagamentosHabilitado: req.PagamentosHabilitado,
		MetodosPagamento:     req.MetodosPagamento,
		PrazoPagamento:       req.PrazoPagamento,
		RegrasCustomizadas:   req.RegrasCustomizadas,
		PoliticaCancelamento: req.PoliticaCancelamento,
	}
	evt, err := h.updateUC.Execute(r.Context(), id, userID, in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, eventoToResponse(evt))
}

func (h *EventHandler) deleteEvento(w http.ResponseWriter, r *http.Request) {
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

// listEventoByUsuarioV1 handles GET /api/eventos/v1/eventos/usuario/{userId}.
// It is an alias for listEventoByUsuario that reads from the {userId} chi param.
func (h *EventHandler) listEventoByUsuarioV1(w http.ResponseWriter, r *http.Request) {
	requesterID := middleware.UserIDFromContext(r.Context())
	if requesterID == "" {
		writeError(w, apierr.Unauthorized("autenticação necessária"))
		return
	}
	usuarioID := chi.URLParam(r, "userId")
	if usuarioID == "" {
		writeError(w, apierr.BadRequest("userId é obrigatório"))
		return
	}

	var cursor *string
	if c := r.URL.Query().Get("cursor"); c != "" {
		cursor = &c
	}
	var status, tipo *string
	if s := r.URL.Query().Get("status"); s != "" {
		status = &s
	}
	if t := r.URL.Query().Get("tipo"); t != "" {
		tipo = &t
	}
	var dataInicioGte, dataInicioLte *time.Time
	if v := r.URL.Query().Get("dataInicioGte"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			dataInicioGte = &t
		}
	}
	if v := r.URL.Query().Get("dataInicioLte"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			dataInicioLte = &t
		}
	}
	limit := queryIntParam(r, "limit", 20)

	in := portin.ListEventosByUsuarioInput{
		UsuarioID:     usuarioID,
		RequesterID:   requesterID,
		Status:        status,
		Tipo:          tipo,
		DataInicioGte: dataInicioGte,
		DataInicioLte: dataInicioLte,
		Cursor:        cursor,
		Limit:         limit,
	}
	page, err := h.listByUsuarioUC.Execute(r.Context(), in)
	if err != nil {
		writeError(w, err)
		return
	}
	resp := make([]eventoResponse, len(page.Eventos))
	for i, e := range page.Eventos {
		e2 := e
		resp[i] = eventoToResponse(&e2)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"eventos":     resp,
		"total":       page.Total,
		"nextCursor":  page.NextCursor,
		"hasNextPage": page.HasNextPage,
		"limit":       page.Limit,
	})
}

// addConvidados handles POST /api/eventos/v1/eventos/{eventoId}/convidados.
// Header X-Usuario-Id is required and identifies the actor performing the operation.
func (h *EventHandler) addConvidados(w http.ResponseWriter, r *http.Request) {
	eventoID := chi.URLParam(r, "eventoId")
	if eventoID == "" {
		writeError(w, apierr.BadRequest("eventoId é obrigatório"))
		return
	}

	usuarioID := r.Header.Get("X-Usuario-Id")
	if usuarioID == "" {
		writeError(w, apierr.BadRequest("header X-Usuario-Id é obrigatório"))
		return
	}

	var req addConvidadosRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo da requisição inválido"))
		return
	}
	if len(req.Convidados) == 0 {
		writeError(w, apierr.BadRequest("lista de convidados não pode ser vazia"))
		return
	}

	convidados := make([]domain.Convidado, len(req.Convidados))
	for i, c := range req.Convidados {
		convidados[i] = domain.Convidado{Telefone: c.Telefone, Nome: c.Nome}
	}

	in := portin.AddConvidadosInput{
		EventoID:   eventoID,
		UsuarioID:  usuarioID,
		Convidados: convidados,
	}
	if err := h.addConvidadosUC.Execute(r.Context(), in); err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"message":    "convidados adicionados com sucesso",
		"eventoId":   eventoID,
		"quantidade": len(convidados),
	})
}

// ---- helpers ----

func eventoToResponse(e *domain.Evento) eventoResponse {
	return eventoResponse{
		ID:                   e.ID,
		UsuarioID:            e.UsuarioID,
		Nome:                 e.Nome,
		Tipo:                 e.Tipo,
		Data:                 e.Data,
		Descricao:            e.Descricao,
		Local:                e.Local,
		FotoURL:              e.FotoURL,
		Status:               string(e.Status),
		ConvidadosIDs:        e.ConvidadosIDs,
		PoliticaConvidados:   e.PoliticaConvidados,
		LimiteConvidados:     e.LimiteConvidados,
		RateiosHabilitado:    e.RateiosHabilitado,
		TipoDivisaoRateio:    e.TipoDivisaoRateio,
		PagamentosHabilitado: e.PagamentosHabilitado,
		MetodosPagamento:     e.MetodosPagamento,
		PrazoPagamento:       e.PrazoPagamento,
		RegrasCustomizadas:   e.RegrasCustomizadas,
		PoliticaCancelamento: e.PoliticaCancelamento,
		CriadoEm:             e.CriadoEm,
		UpdatedAt:            e.UpdatedAt,
	}
}

func queryIntParam(r *http.Request, key string, defaultVal int) int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return v
}
