package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/middleware"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ============================================================
// ConviteHandler — Java parity for:
//   - ConviteController      (/api/v1/convites)
//   - ApprovalController     (POST /api/v1/approvals/invite/{id}/reopen)
//   - DesistenciaController  (/api/v1/participantes/{id}/desistir[/preview])
//   - AdminEventosController (POST /api/v1/admin/eventos/{id}/convites/reenviar-todos)
// Public routes (no JWT): GET /{id}, GET /{id}/registration-data,
//   POST /{id}/confirmar, POST /{id}/recusar.
// All other routes require JWT and are registered via the protected group.
// ============================================================

// ConviteHandler wires the 7 convite use cases to chi routes.
type ConviteHandler struct {
	buscarUC         portin.BuscarConviteUseCase
	enviarUC         portin.EnviarConviteUseCase
	confirmarUC      portin.ConfirmarConviteUseCase
	recusarUC        portin.RecusarConviteUseCase
	desistirUC       portin.DesistirEventoUseCase
	reabrirUC        portin.ReabrirInviteApprovalUseCase
	reenviarMassaUC  portin.ReenviarConvitesMassaAdminUseCase
}

// NewConviteHandler builds the handler with all 7 use cases injected.
func NewConviteHandler(
	buscar portin.BuscarConviteUseCase,
	enviar portin.EnviarConviteUseCase,
	confirmar portin.ConfirmarConviteUseCase,
	recusar portin.RecusarConviteUseCase,
	desistir portin.DesistirEventoUseCase,
	reabrir portin.ReabrirInviteApprovalUseCase,
	reenviarMassa portin.ReenviarConvitesMassaAdminUseCase,
) *ConviteHandler {
	return &ConviteHandler{
		buscarUC:        buscar,
		enviarUC:        enviar,
		confirmarUC:     confirmar,
		recusarUC:       recusar,
		desistirUC:      desistir,
		reabrirUC:       reabrir,
		reenviarMassaUC: reenviarMassa,
	}
}

// RegisterPublicConviteRoutes mounts the public (NO JWT) convite routes.
func (h *ConviteHandler) RegisterPublicConviteRoutes(r chi.Router) {
	r.Get("/api/v1/convites/{participantId}", h.buscar)
	r.Get("/api/v1/convites/{participantId}/registration-data", h.registrationData)
	r.Post("/api/v1/convites/{participantId}/confirmar", h.confirmar)
	r.Post("/api/v1/convites/{participantId}/recusar", h.recusar)
}

// RegisterProtectedConviteRoutes mounts the JWT-protected convite routes.
func (h *ConviteHandler) RegisterProtectedConviteRoutes(r chi.Router) {
	r.Post("/api/v1/convites/enviar", h.enviar)
	r.Get("/api/v1/participantes/{participantId}/desistir/preview", h.desistirPreview)
	r.Post("/api/v1/participantes/{participantId}/desistir", h.desistir)
	r.Post("/api/v1/approvals/invite/{participantId}/reopen", h.reabrir)
	r.Post("/api/v1/admin/eventos/{eventoId}/convites/reenviar-todos", h.reenviarTodos)
}

// ---- request DTOs ----

type enviarConviteRequest struct {
	ParticipantID string `json:"participantId"`
	ForcarReenvio bool   `json:"forcarReenvio"`
}

// ---- handlers ----

func (h *ConviteHandler) buscar(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "participantId")
	res, err := h.buscarUC.Execute(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// registrationData returns the data used to pre-fill the registration form.
// Java rule: 400 BAD_REQUEST when the participant is not a GUEST (link re-use prevention).
func (h *ConviteHandler) registrationData(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "participantId")
	res, err := h.buscarUC.Execute(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	if !strings.EqualFold(res.TipoParticipante, "GUEST") {
		writeError(w, apierr.BadRequest("registration-data disponível apenas para convites do tipo GUEST"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"participantId":  res.ParticipantID,
		"nome":           res.ConvidadoNome,
		"email":          res.ConvidadoEmail,
		"telefone":       res.ConvidadoTelefone,
		"eventoId":       res.EventoID,
		"eventoNome":     res.EventoNome,
	})
}

func (h *ConviteHandler) confirmar(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "participantId")
	res, err := h.confirmarUC.Execute(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *ConviteHandler) recusar(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "participantId")
	res, err := h.recusarUC.Execute(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *ConviteHandler) enviar(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("autenticação necessária"))
		return
	}
	var req enviarConviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo de requisição inválido"))
		return
	}
	if strings.TrimSpace(req.ParticipantID) == "" {
		writeError(w, apierr.BadRequest("participantId é obrigatório"))
		return
	}
	res, err := h.enviarUC.Execute(r.Context(), portin.EnviarConviteInput{
		ParticipantID: req.ParticipantID,
		ForcarReenvio: req.ForcarReenvio,
		OrganizadorID: userID,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, res)
}

func (h *ConviteHandler) desistir(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("autenticação necessária"))
		return
	}
	id := chi.URLParam(r, "participantId")
	res, err := h.desistirUC.Execute(r.Context(), id, userID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *ConviteHandler) desistirPreview(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("autenticação necessária"))
		return
	}
	id := chi.URLParam(r, "participantId")
	res, err := h.desistirUC.Preview(r.Context(), id, userID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *ConviteHandler) reabrir(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("autenticação necessária"))
		return
	}
	id := chi.URLParam(r, "participantId")
	res, err := h.reabrirUC.Execute(r.Context(), id, userID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, res)
}

// reenviarTodos requires ROLE_ADMIN. The route is registered inside the
// protected group; the ADMIN check happens here by inspecting the JWT claims.
func (h *ConviteHandler) reenviarTodos(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("autenticação necessária"))
		return
	}
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil || !claims.HasRole("ADMIN") {
		writeError(w, apierr.Forbidden("ROLE_ADMIN obrigatório"))
		return
	}
	eventoID := chi.URLParam(r, "eventoId")
	res, err := h.reenviarMassaUC.Execute(r.Context(), eventoID, userID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}
