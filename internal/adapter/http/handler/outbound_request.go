package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/middleware"
	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/outbound"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// OutboundRequestHandler handles /api/v1/outbound-requests/* endpoints.
// Provides parity with Java's OutboundRequestController.
type OutboundRequestHandler struct {
	createUC  portin.CreateOutboundRequestUseCase
	listUC    portin.ListOutboundRequestsUseCase
	getUC     portin.GetOutboundRequestUseCase
	detailsUC portin.GetOutboundRequestDetailsUseCase
	approveUC portin.ApproveOutboundRequestUseCase
	rejectUC  portin.RejectOutboundRequestUseCase
	cancelUC  portin.CancelOutboundRequestUseCase
	voteUC    portin.VoteOnOutboundRequestUseCase
}

// NewOutboundRequestHandler wires the OutboundRequestHandler.
func NewOutboundRequestHandler(
	create portin.CreateOutboundRequestUseCase,
	list portin.ListOutboundRequestsUseCase,
	get portin.GetOutboundRequestUseCase,
	details portin.GetOutboundRequestDetailsUseCase,
	approve portin.ApproveOutboundRequestUseCase,
	reject portin.RejectOutboundRequestUseCase,
	cancel portin.CancelOutboundRequestUseCase,
	vote portin.VoteOnOutboundRequestUseCase,
) *OutboundRequestHandler {
	return &OutboundRequestHandler{
		createUC:  create,
		listUC:    list,
		getUC:     get,
		detailsUC: details,
		approveUC: approve,
		rejectUC:  reject,
		cancelUC:  cancel,
		voteUC:    vote,
	}
}

// RegisterOutboundRequestRoutes mounts the JWT-protected outbound routes.
func (h *OutboundRequestHandler) RegisterOutboundRequestRoutes(r chi.Router) {
	// Static routes first to avoid wildcard shadowing on /{requestId}.
	r.Get("/api/v1/outbound-requests/my-requests", h.listMyRequests)
	r.Get("/api/v1/outbound-requests/by-event/{eventId}", h.listByEvent)
	r.Get("/api/v1/outbound-requests/by-event/{eventId}/pending", h.listPendingByEvent)
	r.Get("/api/v1/outbound-requests/by-event/{eventId}/count-pending", h.countPendingByEvent)
	r.Post("/api/v1/outbound-requests", h.createRequest)
	r.Get("/api/v1/outbound-requests/{requestId}/details", h.getDetails)
	r.Get("/api/v1/outbound-requests/{requestId}/by-event/{eventId}", h.getByEvent)
	r.Get("/api/v1/outbound-requests/{requestId}", h.getRequest)
	r.Post("/api/v1/outbound-requests/{requestId}/approve", h.approve)
	r.Post("/api/v1/outbound-requests/{requestId}/reject", h.reject)
	r.Post("/api/v1/outbound-requests/{requestId}/cancel", h.cancel)
	r.Post("/api/v1/outbound-requests/{requestId}/vote", h.vote)
}

// ─── DTOs ─────────────────────────────────────────────────────────────────────

type createOutboundRequestBody struct {
	EventID               string `json:"eventId"`
	Type                  string `json:"type"`
	AmountCents           int64  `json:"amountCents"`
	Justification         string `json:"justification"`
	RateioID              string `json:"rateioId"`
	PaymentAccountID      string `json:"paymentAccountId"`
	RecipientName         string `json:"recipientName"`
	RecipientDocument     string `json:"recipientDocument"`
	PixKeyType            string `json:"pixKeyType"`
	PixKey                string `json:"pixKey"`
	AttachmentID          string `json:"attachmentId"`
	AttachmentFilename    string `json:"attachmentFilename"`
	AttachmentContentType string `json:"attachmentContentType"`
	AttachmentSize        int64  `json:"attachmentSize"`
}

type approveBody struct {
	EventID       string `json:"eventId"`
	ApprovalNotes string `json:"approvalNotes"`
}

type rejectBody struct {
	EventID         string `json:"eventId"`
	RejectionReason string `json:"rejectionReason"`
}

type cancelBody struct {
	CancellationReason string `json:"cancellationReason"`
}

type voteBody struct {
	EventID string `json:"eventId"`
	Approve bool   `json:"approve"`
	Comment string `json:"comment"`
}

type voteResponse struct {
	Request        any                   `json:"request"`
	TotalVotes     int                   `json:"totalVotes"`
	ApproveVotes   int                   `json:"approveVotes"`
	RejectVotes    int                   `json:"rejectVotes"`
	RequiredVotes  int                   `json:"requiredVotes"`
	VotingComplete bool                  `json:"votingComplete"`
	FinalStatus    domain.OutboundStatus `json:"finalStatus"`
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

func (h *OutboundRequestHandler) listMyRequests(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("usuário não autenticado"))
		return
	}
	list, err := h.listUC.ListMyRequests(r.Context(), userID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toOutboundResponseList(list))
}

func (h *OutboundRequestHandler) listByEvent(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("usuário não autenticado"))
		return
	}
	eventID := chi.URLParam(r, "eventId")
	status := strings.ToUpper(r.URL.Query().Get("status"))
	typ := strings.ToUpper(r.URL.Query().Get("type"))
	if status == "REQUESTED" {
		status = string(domain.StatusPending)
	}

	switch {
	case status != "":
		if !isValidOutboundStatus(status) {
			writeError(w, apierr.BadRequest("status inválido: "+status))
			return
		}
		list, err := h.listUC.ListByEventAndStatus(r.Context(), userID, eventID, domain.OutboundStatus(status))
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, toOutboundResponseList(list))
	case typ != "":
		list, err := h.listUC.ListByEventAndType(r.Context(), userID, eventID, domain.OutboundType(typ))
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, toOutboundResponseList(list))
	default:
		list, err := h.listUC.ListByEvent(r.Context(), userID, eventID)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, toOutboundResponseList(list))
	}
}

func (h *OutboundRequestHandler) listPendingByEvent(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("usuário não autenticado"))
		return
	}
	eventID := chi.URLParam(r, "eventId")
	list, err := h.listUC.ListPendingByEvent(r.Context(), userID, eventID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toOutboundResponseList(list))
}

func (h *OutboundRequestHandler) countPendingByEvent(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("usuário não autenticado"))
		return
	}
	eventID := chi.URLParam(r, "eventId")
	count, err := h.listUC.CountPendingByEvent(r.Context(), userID, eventID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"pendingCount": count})
}

func (h *OutboundRequestHandler) createRequest(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("usuário não autenticado"))
		return
	}
	var body createOutboundRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, apierr.BadRequest("corpo da requisição inválido"))
		return
	}
	req, err := h.createUC.Execute(r.Context(), portin.CreateOutboundRequestInput{
		UserID:                userID,
		EventID:               body.EventID,
		Type:                  domain.OutboundType(body.Type),
		AmountCents:           body.AmountCents,
		Justification:         body.Justification,
		RateioID:              body.RateioID,
		PaymentAccountID:      body.PaymentAccountID,
		RecipientName:         body.RecipientName,
		RecipientDocument:     body.RecipientDocument,
		PixKeyType:            domain.PixKeyType(strings.ToUpper(body.PixKeyType)),
		PixKey:                body.PixKey,
		AttachmentID:          body.AttachmentID,
		AttachmentFilename:    body.AttachmentFilename,
		AttachmentContentType: body.AttachmentContentType,
		AttachmentSize:        body.AttachmentSize,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toOutboundResponse(req))
}

func (h *OutboundRequestHandler) getRequest(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	requestID := chi.URLParam(r, "requestId")
	req, err := h.getUC.Execute(r.Context(), userID, requestID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toOutboundResponse(req))
}

func (h *OutboundRequestHandler) getByEvent(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	requestID := chi.URLParam(r, "requestId")
	eventID := chi.URLParam(r, "eventId")
	req, err := h.getUC.ExecuteByEvent(r.Context(), userID, requestID, eventID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toOutboundResponse(req))
}

func (h *OutboundRequestHandler) getDetails(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	requestID := chi.URLParam(r, "requestId")
	res, err := h.detailsUC.Execute(r.Context(), userID, requestID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"outboundRequest":         toOutboundResponse(res.Request),
		"rateioParticipants":      res.RateioParticipants,
		"canVote":                 res.CanVote,
		"hasUserVoted":            res.HasUserVoted,
		"isOrganizer":             res.IsOrganizer,
		"totalVotes":              res.TotalVotes,
		"approvalsCount":          res.ApprovalsCount,
		"rejectionsCount":         res.RejectionsCount,
		"totalRateioParticipants": res.TotalRateioParticipants,
		"requiredVotes":           res.RequiredVotes,
	})
}

func (h *OutboundRequestHandler) approve(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	requestID := chi.URLParam(r, "requestId")
	var body approveBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err.Error() != "EOF" {
		writeError(w, apierr.BadRequest("corpo da requisição inválido"))
		return
	}
	req, err := h.approveUC.Execute(r.Context(), portin.ApproveOutboundRequestInput{
		RequestID:      requestID,
		EventID:        body.EventID,
		ApproverUserID: userID,
		ApprovalNotes:  body.ApprovalNotes,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toOutboundResponse(req))
}

func (h *OutboundRequestHandler) reject(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	requestID := chi.URLParam(r, "requestId")
	var body rejectBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, apierr.BadRequest("corpo da requisição inválido"))
		return
	}
	req, err := h.rejectUC.Execute(r.Context(), portin.RejectOutboundRequestInput{
		RequestID:       requestID,
		EventID:         body.EventID,
		RejecterUserID:  userID,
		RejectionReason: body.RejectionReason,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toOutboundResponse(req))
}

func (h *OutboundRequestHandler) cancel(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	requestID := chi.URLParam(r, "requestId")
	var body cancelBody
	_ = json.NewDecoder(r.Body).Decode(&body) // body is optional
	req, err := h.cancelUC.Execute(r.Context(), portin.CancelOutboundRequestInput{
		RequestID:          requestID,
		UserID:             userID,
		CancellationReason: body.CancellationReason,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toOutboundResponse(req))
}

func (h *OutboundRequestHandler) vote(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	requestID := chi.URLParam(r, "requestId")
	var body voteBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, apierr.BadRequest("corpo da requisição inválido"))
		return
	}
	res, err := h.voteUC.Execute(r.Context(), portin.VoteOnOutboundRequestInput{
		RequestID: requestID,
		EventID:   body.EventID,
		UserID:    userID,
		Approve:   body.Approve,
		Comment:   body.Comment,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, voteResponse{
		Request:        toOutboundResponse(res.Request),
		TotalVotes:     res.TotalVotes,
		ApproveVotes:   res.ApproveVotes,
		RejectVotes:    res.RejectVotes,
		RequiredVotes:  res.RequiredVotes,
		VotingComplete: res.VotingComplete,
		FinalStatus:    res.FinalStatus,
	})
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func isValidOutboundStatus(s string) bool {
	switch domain.OutboundStatus(s) {
	case domain.StatusPending, domain.StatusApproved, domain.StatusRejected,
		domain.StatusProcessing, domain.StatusCompleted, domain.StatusFailed,
		domain.StatusCancelled, domain.StatusExpired:
		return true
	}
	return false
}

type outboundResponse struct {
	ID                 string                `json:"id"`
	EventID            string                `json:"eventId"`
	RequesterUserID    string                `json:"requesterUserId"`
	RateioID           string                `json:"rateioId,omitempty"`
	Type               domain.OutboundType   `json:"type"`
	AmountCents        int64                 `json:"amountCents"`
	Justification      string                `json:"justification,omitempty"`
	Recipient          *recipientResponse    `json:"recipient,omitempty"`
	AttachmentID       string                `json:"attachmentId,omitempty"`
	AttachmentURL      string                `json:"attachmentUrl,omitempty"`
	Status             domain.OutboundStatus `json:"status"`
	Approvals          int                   `json:"approvals"`
	Rejections         int                   `json:"rejections"`
	RequiredVotes      int                   `json:"requiredVotes"`
	RequiresVoting     bool                  `json:"requiresVoting"`
	ApprovalMode       domain.ApprovalMode   `json:"approvalMode,omitempty"`
	ExpiresAt          string                `json:"expiresAt,omitempty"`
	ApprovedBy         string                `json:"approvedBy,omitempty"`
	RejectedBy         string                `json:"rejectedBy,omitempty"`
	RejectionReason    string                `json:"rejectionReason,omitempty"`
	Provider           string                `json:"provider,omitempty"`
	ProviderTransferID string                `json:"providerTransferId,omitempty"`
	CreatedAt          string                `json:"createdAt"`
	UpdatedAt          string                `json:"updatedAt"`
	CompletedAt        string                `json:"completedAt,omitempty"`
	Votes              []voteResponseItem    `json:"votes,omitempty"`
}

type recipientResponse struct {
	Name       string            `json:"name,omitempty"`
	Document   string            `json:"document,omitempty"`
	PixKey     string            `json:"pixKey,omitempty"`
	PixKeyType domain.PixKeyType `json:"pixKeyType,omitempty"`
}

type voteResponseItem struct {
	UserID  string `json:"userId"`
	Vote    string `json:"vote"`
	VotedAt string `json:"votedAt"`
	Comment string `json:"comment,omitempty"`
}

func toOutboundResponse(req *domain.OutboundRequest) *outboundResponse {
	if req == nil {
		return nil
	}
	resp := &outboundResponse{
		ID:                 req.ID,
		EventID:            req.EventID,
		RequesterUserID:    req.RequesterUserID,
		RateioID:           req.RateioID,
		Type:               req.Type,
		AmountCents:        req.AmountCents,
		Justification:      req.Justification,
		AttachmentID:       req.AttachmentID,
		Status:             req.Status,
		Approvals:          req.Approvals,
		Rejections:         req.Rejections,
		RequiredVotes:      req.RequiredVotes,
		RequiresVoting:     req.RequiresVoting,
		ApprovalMode:       req.ApprovalMode,
		ApprovedBy:         req.ApprovedBy,
		RejectedBy:         req.RejectedBy,
		RejectionReason:    req.RejectionReason,
		Provider:           req.Provider,
		ProviderTransferID: req.ProviderTransferID,
		CreatedAt:          req.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:          req.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if req.Recipient.Name != "" || req.Recipient.PixKey != "" {
		resp.Recipient = &recipientResponse{
			Name:       req.Recipient.Name,
			Document:   req.Recipient.Document,
			PixKey:     req.Recipient.PixKey,
			PixKeyType: req.Recipient.PixKeyType,
		}
	}
	if req.Attachment != nil {
		resp.AttachmentURL = req.Attachment.URL
	}
	if req.ExpiresAt != nil {
		resp.ExpiresAt = req.ExpiresAt.UTC().Format(time.RFC3339)
	}
	if req.CompletedAt != nil {
		resp.CompletedAt = req.CompletedAt.UTC().Format(time.RFC3339)
	}
	if len(req.Votes) > 0 {
		resp.Votes = make([]voteResponseItem, len(req.Votes))
		for i, v := range req.Votes {
			resp.Votes[i] = voteResponseItem{
				UserID:  v.UserID,
				Vote:    string(v.Vote),
				VotedAt: v.VotedAt.UTC().Format(time.RFC3339),
				Comment: v.Comment,
			}
		}
	}
	return resp
}

func toOutboundResponseList(list []domain.OutboundRequest) []*outboundResponse {
	out := make([]*outboundResponse, len(list))
	for i := range list {
		r := list[i]
		out[i] = toOutboundResponse(&r)
	}
	return out
}
