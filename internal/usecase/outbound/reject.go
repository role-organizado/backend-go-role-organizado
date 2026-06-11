package outbound

import (
	"context"
	"strings"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/outbound"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// RejectOutboundRequest implements portin.RejectOutboundRequestUseCase.
// Organizer-only rejection; rejectionReason is mandatory.
type RejectOutboundRequest struct {
	requests     portout.OutboundRequestRepository
	eventos      portout.EventoRepository
	participants portout.ParticipantRepository
	audit        portout.OutboundAuditLogRepository
}

// NewRejectOutboundRequest wires the reject use case.
func NewRejectOutboundRequest(
	requests portout.OutboundRequestRepository,
	eventos portout.EventoRepository,
	participants portout.ParticipantRepository,
	audit portout.OutboundAuditLogRepository,
) *RejectOutboundRequest {
	return &RejectOutboundRequest{requests: requests, eventos: eventos, participants: participants, audit: audit}
}

// Execute rejects the request.
func (uc *RejectOutboundRequest) Execute(ctx context.Context, in portin.RejectOutboundRequestInput) (*domain.OutboundRequest, error) {
	if strings.TrimSpace(in.RejectionReason) == "" {
		return nil, apierr.Unprocessable("rejectionReason é obrigatório")
	}
	req, err := uc.requests.FindByID(ctx, in.RequestID)
	if err != nil {
		return nil, err
	}
	if in.EventID != "" && in.EventID != req.EventID {
		return nil, apierr.NotFoundMsg("solicitação outbound não encontrada para o evento informado")
	}
	ok, err := isOrganizer(ctx, uc.eventos, uc.participants, req.EventID, in.RejecterUserID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, apierr.NotFoundMsg("solicitação outbound não encontrada")
	}
	if err := req.Reject(in.RejecterUserID, in.RejectionReason); err != nil {
		return nil, apierr.Unprocessable(err.Error())
	}
	saved, err := uc.requests.Update(ctx, req)
	if err != nil {
		return nil, err
	}
	appendAudit(ctx, uc.audit, saved, in.RejecterUserID, domain.AuditActionRejected, in.RejectionReason)
	return saved, nil
}

var _ portin.RejectOutboundRequestUseCase = (*RejectOutboundRequest)(nil)
