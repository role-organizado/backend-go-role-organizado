package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/notificationtemplate"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
)

// NotificationTemplateHandler handles HTTP endpoints for notification templates.
type NotificationTemplateHandler struct {
	createUC        portin.CreateNotificationTemplateUseCase
	getUC           portin.GetNotificationTemplateUseCase
	listUC          portin.ListNotificationTemplatesUseCase
	updateUC        portin.UpdateNotificationTemplateUseCase
	deleteUC        portin.DeleteNotificationTemplateUseCase
	renderUC        portin.RenderNotificationTemplateUseCase
	testSendUC      portin.TestSendNotificationTemplateUseCase
	getByTypeUC     portin.GetByTypeNotificationTemplateUseCase
	listCategoriaUC portin.ListByCategoriaNotificationTemplateUseCase
}

// NewNotificationTemplateHandler creates a NotificationTemplateHandler.
func NewNotificationTemplateHandler(
	createUC portin.CreateNotificationTemplateUseCase,
	getUC portin.GetNotificationTemplateUseCase,
	listUC portin.ListNotificationTemplatesUseCase,
	updateUC portin.UpdateNotificationTemplateUseCase,
	deleteUC portin.DeleteNotificationTemplateUseCase,
	renderUC portin.RenderNotificationTemplateUseCase,
	testSendUC portin.TestSendNotificationTemplateUseCase,
	getByTypeUC portin.GetByTypeNotificationTemplateUseCase,
	listCategoriaUC portin.ListByCategoriaNotificationTemplateUseCase,
) *NotificationTemplateHandler {
	return &NotificationTemplateHandler{
		createUC:        createUC,
		getUC:           getUC,
		listUC:          listUC,
		updateUC:        updateUC,
		deleteUC:        deleteUC,
		renderUC:        renderUC,
		testSendUC:      testSendUC,
		getByTypeUC:     getByTypeUC,
		listCategoriaUC: listCategoriaUC,
	}
}

// RegisterNotificationTemplateRoutes registers notification template routes (protected).
// Mirrors the Java API: /api/v1/notification-templates/...
func (h *NotificationTemplateHandler) RegisterNotificationTemplateRoutes(r chi.Router) {
	// CRUD
	r.Post("/api/v1/notification-templates", h.create)
	r.Get("/api/v1/notification-templates", h.list)
	r.Get("/api/v1/notification-templates/{id}", h.get)
	r.Put("/api/v1/notification-templates/{id}", h.update)
	r.Delete("/api/v1/notification-templates/{id}", h.deleteTemplate)

	// Query routes — must appear before /{id} to avoid shadowing
	r.Get("/api/v1/notification-templates/by-type/{type}", h.getByType)
	r.Get("/api/v1/notification-templates/by-categoria/{categoria}", h.listByCategoria)

	// Action routes
	r.Post("/api/v1/notification-templates/{id}/render", h.render)
	r.Post("/api/v1/notification-templates/{id}/test-send", h.testSend)

	// Non-versioned aliases (Java parity)
	r.Post("/api/notification-templates", h.create)
	r.Get("/api/notification-templates", h.list)
	r.Get("/api/notification-templates/{id}", h.get)
	r.Put("/api/notification-templates/{id}", h.update)
	r.Delete("/api/notification-templates/{id}", h.deleteTemplate)
	r.Get("/api/notification-templates/by-type/{type}", h.getByType)
	r.Get("/api/notification-templates/by-categoria/{categoria}", h.listByCategoria)
	r.Post("/api/notification-templates/{id}/render", h.render)
	r.Post("/api/notification-templates/{id}/test-send", h.testSend)
}

// createTemplateRequest is the JSON body for POST /notification-templates.
type createTemplateRequest struct {
	Nome               string   `json:"nome"`
	Tipo               string   `json:"tipo"`
	Categoria          string   `json:"categoria"`
	Assunto            string   `json:"assunto"`
	Corpo              string   `json:"corpo"`
	VariaveisEsperadas []string `json:"variaveis_esperadas"`
	Ativo              bool     `json:"ativo"`
}

// updateTemplateRequest is the JSON body for PUT /notification-templates/{id}.
type updateTemplateRequest struct {
	Nome               string   `json:"nome"`
	Tipo               string   `json:"tipo"`
	Categoria          string   `json:"categoria"`
	Assunto            string   `json:"assunto"`
	Corpo              string   `json:"corpo"`
	VariaveisEsperadas []string `json:"variaveis_esperadas"`
	Ativo              bool     `json:"ativo"`
}

// renderRequest is the JSON body for POST /notification-templates/{id}/render.
type renderRequest struct {
	Variaveis map[string]string `json:"variaveis"`
}

// testSendRequest is the JSON body for POST /notification-templates/{id}/test-send.
type testSendRequest struct {
	Destinatario string            `json:"destinatario"`
	Variaveis    map[string]string `json:"variaveis"`
}

func (h *NotificationTemplateHandler) create(w http.ResponseWriter, r *http.Request) {
	var body createTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, err)
		return
	}
	t, err := h.createUC.Execute(r.Context(), portin.CreateNotificationTemplateInput{
		Nome:               body.Nome,
		Tipo:               domain.TemplateType(body.Tipo),
		Categoria:          domain.TemplateCategoria(body.Categoria),
		Assunto:            body.Assunto,
		Corpo:              body.Corpo,
		VariaveisEsperadas: body.VariaveisEsperadas,
		Ativo:              body.Ativo,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func (h *NotificationTemplateHandler) list(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	items, total, err := h.listUC.Execute(r.Context(), page, pageSize)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":      items,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func (h *NotificationTemplateHandler) get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	t, err := h.getUC.Execute(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (h *NotificationTemplateHandler) update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body updateTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, err)
		return
	}
	t, err := h.updateUC.Execute(r.Context(), portin.UpdateNotificationTemplateInput{
		ID:                 id,
		Nome:               body.Nome,
		Tipo:               domain.TemplateType(body.Tipo),
		Categoria:          domain.TemplateCategoria(body.Categoria),
		Assunto:            body.Assunto,
		Corpo:              body.Corpo,
		VariaveisEsperadas: body.VariaveisEsperadas,
		Ativo:              body.Ativo,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (h *NotificationTemplateHandler) deleteTemplate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.deleteUC.Execute(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *NotificationTemplateHandler) render(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body renderRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, err)
		return
	}
	resp, err := h.renderUC.Execute(r.Context(), id, domain.RenderRequest{
		Variaveis: body.Variaveis,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *NotificationTemplateHandler) testSend(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body testSendRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, err)
		return
	}
	resp, err := h.testSendUC.Execute(r.Context(), portin.TestSendInput{
		TemplateID:   id,
		Destinatario: body.Destinatario,
		Variaveis:    body.Variaveis,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *NotificationTemplateHandler) getByType(w http.ResponseWriter, r *http.Request) {
	tipo := chi.URLParam(r, "type")
	t, err := h.getByTypeUC.Execute(r.Context(), domain.TemplateType(tipo))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (h *NotificationTemplateHandler) listByCategoria(w http.ResponseWriter, r *http.Request) {
	categoria := chi.URLParam(r, "categoria")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	items, total, err := h.listCategoriaUC.Execute(r.Context(), domain.TemplateCategoria(categoria), page, pageSize)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":      items,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}
