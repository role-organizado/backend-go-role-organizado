package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/guest"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// GuestHandler handles HTTP requests for the Guest domain.
// All endpoints are PUBLIC (no JWT required) — matches the Java GuestController.
type GuestHandler struct {
	createOrFindUC portin.CreateOrFindGuestUseCase
	getUC          portin.GetGuestUseCase
	getByTelUC     portin.GetGuestByTelefoneUseCase
	getByEmailUC   portin.GetGuestByEmailUseCase
	listUC         portin.ListGuestsUseCase
	batchUC        portin.BatchGetGuestsUseCase
}

// NewGuestHandler builds the handler.
func NewGuestHandler(
	createOrFind portin.CreateOrFindGuestUseCase,
	get portin.GetGuestUseCase,
	getByTel portin.GetGuestByTelefoneUseCase,
	getByEmail portin.GetGuestByEmailUseCase,
	list portin.ListGuestsUseCase,
	batch portin.BatchGetGuestsUseCase,
) *GuestHandler {
	return &GuestHandler{
		createOrFindUC: createOrFind,
		getUC:          get,
		getByTelUC:     getByTel,
		getByEmailUC:   getByEmail,
		listUC:         list,
		batchUC:        batch,
	}
}

// RegisterGuestRoutes mounts the 6 Guest routes. All routes are public — register
// them outside the JWT-protected group in main.go.
func (h *GuestHandler) RegisterGuestRoutes(r chi.Router) {
	r.Post("/api/guests/batch", h.batchGuests)
	r.Post("/api/guests", h.createOrFind)
	r.Get("/api/guests", h.list)
	r.Get("/api/guests/{id}", h.getByID)
	r.Get("/api/guests/telefone/{telefone}", h.getByTelefone)
	r.Get("/api/guests/email/{email}", h.getByEmail)
}

// ---- DTOs ----

type createGuestRequest struct {
	Nome     string `json:"nome"`
	Telefone string `json:"telefone"`
	Email    string `json:"email"`
}

type batchGuestRequest struct {
	IDs []string `json:"ids"`
}

type guestResponse struct {
	ID                    string     `json:"id"`
	Nome                  string     `json:"nome"`
	Telefone              string     `json:"telefone,omitempty"`
	Email                 string     `json:"email,omitempty"`
	CriadoEm              time.Time  `json:"criadoEm"`
	AtualizadoEm          time.Time  `json:"atualizadoEm"`
	EvoluidoParaUsuarioID string     `json:"evoluidoParaUsuarioId,omitempty"`
	EvoluidoEm            *time.Time `json:"evoluidoEm,omitempty"`
}

func toGuestResponse(g domain.Guest) guestResponse {
	return guestResponse{
		ID:                    g.ID,
		Nome:                  g.Nome,
		Telefone:              g.Telefone,
		Email:                 g.Email,
		CriadoEm:              g.CriadoEm,
		AtualizadoEm:          g.AtualizadoEm,
		EvoluidoParaUsuarioID: g.EvoluidoParaUsuarioID,
		EvoluidoEm:            g.EvoluidoEm,
	}
}

// ---- Handlers ----

func (h *GuestHandler) createOrFind(w http.ResponseWriter, r *http.Request) {
	var req createGuestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo da requisição inválido"))
		return
	}
	g, err := h.createOrFindUC.Execute(r.Context(), portin.CreateOrFindGuestInput{
		Nome:     req.Nome,
		Telefone: req.Telefone,
		Email:    req.Email,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toGuestResponse(*g))
}

func (h *GuestHandler) getByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	g, err := h.getUC.Execute(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toGuestResponse(*g))
}

func (h *GuestHandler) getByTelefone(w http.ResponseWriter, r *http.Request) {
	telefone := chi.URLParam(r, "telefone")
	g, err := h.getByTelUC.Execute(r.Context(), telefone)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toGuestResponse(*g))
}

func (h *GuestHandler) getByEmail(w http.ResponseWriter, r *http.Request) {
	email := chi.URLParam(r, "email")
	g, err := h.getByEmailUC.Execute(r.Context(), email)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toGuestResponse(*g))
}

func (h *GuestHandler) list(w http.ResponseWriter, r *http.Request) {
	guests, err := h.listUC.Execute(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]guestResponse, len(guests))
	for i, g := range guests {
		out[i] = toGuestResponse(g)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *GuestHandler) batchGuests(w http.ResponseWriter, r *http.Request) {
	var req batchGuestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo da requisição inválido"))
		return
	}
	guests, err := h.batchUC.Execute(r.Context(), req.IDs)
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]guestResponse, len(guests))
	for i, g := range guests {
		out[i] = toGuestResponse(g)
	}
	writeJSON(w, http.StatusOK, out)
}
