package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/middleware"
	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// InstallmentHandler handles payment installment HTTP endpoints.
// It replaces the raw MongoDB installment queries previously in FinanceHandler.
type InstallmentHandler struct {
	listUserUC portin.ListUserInstallmentsUseCase
	listUC     portin.ListInstallmentsUseCase
	getUC      portin.GetInstallmentUseCase
	cancelUC   portin.CancelParticipantInstallmentsUseCase
}

// NewInstallmentHandler creates a new InstallmentHandler.
func NewInstallmentHandler(
	listUserUC portin.ListUserInstallmentsUseCase,
	listUC portin.ListInstallmentsUseCase,
	getUC portin.GetInstallmentUseCase,
	cancelUC portin.CancelParticipantInstallmentsUseCase,
) *InstallmentHandler {
	return &InstallmentHandler{
		listUserUC: listUserUC,
		listUC:     listUC,
		getUC:      getUC,
		cancelUC:   cancelUC,
	}
}

// RegisterInstallmentRoutes mounts installment routes onto the given router.
// NOTE: /user and /cancel must be registered BEFORE /{installmentId} to prevent
// chi from routing them as path parameters.
func (h *InstallmentHandler) RegisterInstallmentRoutes(r chi.Router) {
	r.Get("/api/v1/installments/user", h.ListUserInstallments)
	r.Post("/api/v1/installments/cancel", h.CancelParticipantInstallments)
	r.Get("/api/v1/installments", h.ListInstallments)
	r.Get("/api/v1/installments/{installmentId}", h.GetInstallment)
}

// installmentResponse mirrors Java's PaymentInstallmentResponse DTO with camelCase fields.
type installmentResponse struct {
	ID                string  `json:"id"`
	EventID           string  `json:"eventId"`
	ParticipantID     string  `json:"participantId"`
	InstallmentNumber int     `json:"installmentNumber"`
	TotalInstallments int     `json:"totalInstallments"`
	AmountCents       int64   `json:"amountCents"`
	DueDate           string  `json:"dueDate"`
	Status            string  `json:"status"`
	TransactionID     string  `json:"transactionId,omitempty"`
	PaidAt            *string `json:"paidAt,omitempty"`
	PaymentMethod     string  `json:"paymentMethod,omitempty"`
}

func toInstallmentResponse(inst *domain.PaymentInstallment) installmentResponse {
	resp := installmentResponse{
		ID:                inst.ID,
		EventID:           inst.EventID,
		ParticipantID:     inst.ParticipantID,
		InstallmentNumber: inst.InstallmentNumber,
		TotalInstallments: inst.TotalInstallments,
		AmountCents:       inst.AmountCents,
		DueDate:           inst.DueDate.UTC().Format(time.RFC3339),
		Status:            string(inst.Status),
		TransactionID:     inst.TransactionID,
		PaymentMethod:     inst.PaymentMethod,
	}
	if inst.PaidAt != nil {
		s := inst.PaidAt.UTC().Format(time.RFC3339)
		resp.PaidAt = &s
	}
	return resp
}

func toInstallmentSlice(insts []*domain.PaymentInstallment) []installmentResponse {
	resp := make([]installmentResponse, len(insts))
	for i, inst := range insts {
		resp[i] = toInstallmentResponse(inst)
	}
	return resp
}

// GET /api/v1/installments/user
// Lists installments for the authenticated user, using the BUG5 dual-search
// (userId + participationIds) and filtering events in planning phases.
func (h *InstallmentHandler) ListUserInstallments(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("usuário não autenticado"))
		return
	}

	var statusFilter *domain.InstallmentStatus
	if s := r.URL.Query().Get("status"); s != "" {
		st := domain.InstallmentStatus(s)
		statusFilter = &st
	}

	insts, err := h.listUserUC.Execute(r.Context(), userID, statusFilter)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toInstallmentSlice(insts))
}

// GET /api/v1/installments
// Lists installments for an event. eventId is required (400 if absent).
// Requester must be a participant (403 otherwise).
func (h *InstallmentHandler) ListInstallments(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("usuário não autenticado"))
		return
	}

	q := r.URL.Query()
	eventID := q.Get("eventId")
	if eventID == "" {
		writeError(w, apierr.BadRequest("eventId é obrigatório"))
		return
	}

	filter := portin.ListInstallmentsFilter{
		EventID: eventID,
		UserID:  q.Get("userId"),
	}
	if s := q.Get("status"); s != "" {
		st := domain.InstallmentStatus(s)
		filter.Status = &st
	}

	insts, err := h.listUC.Execute(r.Context(), userID, filter)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toInstallmentSlice(insts))
}

// GET /api/v1/installments/{installmentId}
// Returns a single installment. 404 if not found; 403 if not a participant.
func (h *InstallmentHandler) GetInstallment(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("usuário não autenticado"))
		return
	}

	installmentID := chi.URLParam(r, "installmentId")
	inst, err := h.getUC.Execute(r.Context(), installmentID, userID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toInstallmentResponse(inst))
}

// cancelInstallmentsRequest is the JSON body for POST /api/v1/installments/cancel.
type cancelInstallmentsRequest struct {
	EventID       string `json:"eventId"`
	ParticipantID string `json:"participantId"`
}

// POST /api/v1/installments/cancel
// Cancels PENDING/OVERDUE installments for a participant. Only the event organizer
// may call this endpoint.
func (h *InstallmentHandler) CancelParticipantInstallments(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("usuário não autenticado"))
		return
	}

	var req cancelInstallmentsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo inválido"))
		return
	}
	if req.EventID == "" || req.ParticipantID == "" {
		writeError(w, apierr.BadRequest("eventId e participantId são obrigatórios"))
		return
	}

	count, err := h.cancelUC.Execute(r.Context(), portin.CancelParticipantInstallmentsInput{
		EventID:       req.EventID,
		ParticipantID: req.ParticipantID,
		RequesterID:   userID,
	})
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"cancelled":     count,
		"eventId":       req.EventID,
		"participantId": req.ParticipantID,
	})
}
