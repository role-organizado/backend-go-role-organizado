package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/notification"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/middleware"
)

// NotificationHandler handles notification HTTP endpoints.
type NotificationHandler struct {
	listUC          portin.ListNotificacoesUseCase
	getUC           portin.GetNotificacaoUseCase
	createUC        portin.CreateNotificacaoUseCase
	marcarLidaUC    portin.MarcarLidaUseCase
	marcarTodasUC   portin.MarcarTodasLidasUseCase
	deleteUC        portin.DeleteNotificacaoUseCase
	countUnreadUC   portin.CountUnreadUseCase
}

// NewNotificationHandler creates a NotificationHandler.
func NewNotificationHandler(
	listUC portin.ListNotificacoesUseCase,
	getUC portin.GetNotificacaoUseCase,
	createUC portin.CreateNotificacaoUseCase,
	marcarLidaUC portin.MarcarLidaUseCase,
	marcarTodasUC portin.MarcarTodasLidasUseCase,
	deleteUC portin.DeleteNotificacaoUseCase,
	countUnreadUC portin.CountUnreadUseCase,
) *NotificationHandler {
	return &NotificationHandler{
		listUC:        listUC,
		getUC:         getUC,
		createUC:      createUC,
		marcarLidaUC:  marcarLidaUC,
		marcarTodasUC: marcarTodasUC,
		deleteUC:      deleteUC,
		countUnreadUC: countUnreadUC,
	}
}

// RegisterNotificationRoutes registers notification routes on r (protected).
func (h *NotificationHandler) RegisterNotificationRoutes(r chi.Router) {
	r.Route("/api/notificacoes", func(r chi.Router) {
		r.Get("/", h.list)
		r.Get("/unread-count", h.countUnread)
		r.Put("/marcar-todas-lidas", h.marcarTodas)
		r.Post("/", h.create)
		r.Get("/{id}", h.get)
		r.Put("/{id}/lida", h.marcarLida)
		r.Delete("/{id}", h.delete)
	})
	// v1 aliases
	r.Route("/api/v1/notificacoes", func(r chi.Router) {
		r.Get("/", h.list)
		r.Get("/unread-count", h.countUnread)
		r.Put("/marcar-todas-lidas", h.marcarTodas)
		r.Post("/", h.create)
		r.Get("/{id}", h.get)
		r.Put("/{id}/lida", h.marcarLida)
		r.Delete("/{id}", h.delete)
	})
}

func (h *NotificationHandler) list(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	items, total, err := h.listUC.Execute(r.Context(), userID, page, pageSize)
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

func (h *NotificationHandler) get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := middleware.UserIDFromContext(r.Context())
	n, err := h.getUC.Execute(r.Context(), id, userID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, n)
}

func (h *NotificationHandler) create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		UsuarioID string            `json:"usuario_id"`
		Tipo      string            `json:"tipo"`
		Titulo    string            `json:"titulo"`
		Mensagem  string            `json:"mensagem"`
		Dados     map[string]string `json:"dados"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, err)
		return
	}
	n, err := h.createUC.Execute(r.Context(), portin.CreateNotificacaoInput{
		UsuarioID: body.UsuarioID,
		Tipo:      domain.TipoNotificacao(body.Tipo),
		Titulo:    body.Titulo,
		Mensagem:  body.Mensagem,
		Dados:     body.Dados,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, n)
}

func (h *NotificationHandler) marcarLida(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := middleware.UserIDFromContext(r.Context())
	n, err := h.marcarLidaUC.Execute(r.Context(), id, userID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, n)
}

func (h *NotificationHandler) marcarTodas(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if err := h.marcarTodasUC.Execute(r.Context(), userID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *NotificationHandler) delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := middleware.UserIDFromContext(r.Context())
	if err := h.deleteUC.Execute(r.Context(), id, userID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *NotificationHandler) countUnread(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	count, err := h.countUnreadUC.Execute(r.Context(), userID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"unread_count": count})
}
