package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/middleware"
	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/listapresentes"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ListaPresentesHandler handles gift list HTTP endpoints.
type ListaPresentesHandler struct {
	addUC      portin.AddItemUseCase
	getUC      portin.GetItemUseCase
	listUC     portin.ListItemsUseCase
	reservarUC portin.ReservarItemUseCase
	removeUC   portin.RemoveItemUseCase
}

// NewListaPresentesHandler creates a new ListaPresentesHandler.
func NewListaPresentesHandler(
	add portin.AddItemUseCase,
	get portin.GetItemUseCase,
	list portin.ListItemsUseCase,
	reservar portin.ReservarItemUseCase,
	remove portin.RemoveItemUseCase,
) *ListaPresentesHandler {
	return &ListaPresentesHandler{
		addUC:      add,
		getUC:      get,
		listUC:     list,
		reservarUC: reservar,
		removeUC:   remove,
	}
}

// RegisterListaPresentesRoutes mounts gift list routes onto the given router.
func (h *ListaPresentesHandler) RegisterListaPresentesRoutes(r chi.Router) {
	r.Post("/api/v1/eventos/{eventoId}/lista-presentes", h.addItem)
	r.Get("/api/v1/eventos/{eventoId}/lista-presentes", h.listItems)
	r.Get("/api/v1/eventos/{eventoId}/lista-presentes/{itemId}", h.getItem)
	r.Post("/api/v1/eventos/{eventoId}/lista-presentes/{itemId}/reservar", h.reservarItem)
	r.Delete("/api/v1/eventos/{eventoId}/lista-presentes/{itemId}", h.removeItem)
}

// ---- Request / Response DTOs ----

type addItemRequest struct {
	Nome       string `json:"nome"`
	Descricao  string `json:"descricao"`
	URLProduto string `json:"urlProduto"`
	Valor      int64  `json:"valor"` // centavos, 0 = qualquer valor
	Quantidade int    `json:"quantidade"`
}

type reservarItemRequest struct {
	GuestID string `json:"guestId"`
}

type listaPresentesItemResponse struct {
	ID                  string    `json:"id"`
	EventoID            string    `json:"eventoId"`
	OwnerUserID         string    `json:"ownerUserId"`
	Nome                string    `json:"nome"`
	Descricao           string    `json:"descricao,omitempty"`
	URLProduto          string    `json:"urlProduto,omitempty"`
	Valor               int64     `json:"valor"`
	Quantidade          int       `json:"quantidade"`
	Reservado           int       `json:"reservado"`
	Status              string    `json:"status"`
	ReservadoPorGuestID string    `json:"reservadoPorGuestId,omitempty"`
	CriadoEm           time.Time `json:"criadoEm"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

func toListaPresentesResponse(item *domain.ListaPresentesItem) listaPresentesItemResponse {
	return listaPresentesItemResponse{
		ID:                  item.ID,
		EventoID:            item.EventoID,
		OwnerUserID:         item.OwnerUserID,
		Nome:                item.Nome,
		Descricao:           item.Descricao,
		URLProduto:          item.URLProduto,
		Valor:               item.Valor,
		Quantidade:          item.Quantidade,
		Reservado:           item.Reservado,
		Status:              string(item.Status),
		ReservadoPorGuestID: item.ReservadoPorGuestID,
		CriadoEm:           item.CriadoEm,
		UpdatedAt:           item.UpdatedAt,
	}
}

// ---- Handlers ----

// addItem handles POST /api/v1/eventos/{eventoId}/lista-presentes
func (h *ListaPresentesHandler) addItem(w http.ResponseWriter, r *http.Request) {
	eventoID := chi.URLParam(r, "eventoId")
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("autenticação necessária"))
		return
	}

	var req addItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo de requisição inválido"))
		return
	}

	in := portin.AddItemInput{
		EventoID:    eventoID,
		OwnerUserID: userID,
		Nome:        req.Nome,
		Descricao:   req.Descricao,
		URLProduto:  req.URLProduto,
		Valor:       req.Valor,
		Quantidade:  req.Quantidade,
	}

	item, err := h.addUC.Execute(r.Context(), in)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, toListaPresentesResponse(item))
}

// listItems handles GET /api/v1/eventos/{eventoId}/lista-presentes
func (h *ListaPresentesHandler) listItems(w http.ResponseWriter, r *http.Request) {
	eventoID := chi.URLParam(r, "eventoId")

	items, err := h.listUC.Execute(r.Context(), eventoID)
	if err != nil {
		writeError(w, err)
		return
	}

	resp := make([]listaPresentesItemResponse, len(items))
	for i, item := range items {
		resp[i] = toListaPresentesResponse(item)
	}
	writeJSON(w, http.StatusOK, resp)
}

// getItem handles GET /api/v1/eventos/{eventoId}/lista-presentes/{itemId}
func (h *ListaPresentesHandler) getItem(w http.ResponseWriter, r *http.Request) {
	itemID := chi.URLParam(r, "itemId")

	item, err := h.getUC.Execute(r.Context(), itemID)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toListaPresentesResponse(item))
}

// reservarItem handles POST /api/v1/eventos/{eventoId}/lista-presentes/{itemId}/reservar
func (h *ListaPresentesHandler) reservarItem(w http.ResponseWriter, r *http.Request) {
	itemID := chi.URLParam(r, "itemId")

	var req reservarItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo de requisição inválido"))
		return
	}

	// Prefer authenticated user ID as guestID if not provided.
	if req.GuestID == "" {
		if userID := middleware.UserIDFromContext(r.Context()); userID != "" {
			req.GuestID = userID
		}
	}

	in := portin.ReservarItemInput{
		ItemID:  itemID,
		GuestID: req.GuestID,
	}

	item, err := h.reservarUC.Execute(r.Context(), in)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toListaPresentesResponse(item))
}

// removeItem handles DELETE /api/v1/eventos/{eventoId}/lista-presentes/{itemId}
func (h *ListaPresentesHandler) removeItem(w http.ResponseWriter, r *http.Request) {
	itemID := chi.URLParam(r, "itemId")
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("autenticação necessária"))
		return
	}

	if err := h.removeUC.Execute(r.Context(), itemID, userID); err != nil {
		writeError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
