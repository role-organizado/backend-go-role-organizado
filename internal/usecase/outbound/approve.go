package outbound

import (
	"context"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/outbound"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ApproveOutboundRequest implements portin.ApproveOutboundRequestUseCase.
// Direct organizer approval — bypasses voting. Validates organizer role and
// PENDING status; triggers the OutboundTransferProvider on success.
type ApproveOutboundRequest struct {
	requests     portout.OutboundRequestRepository
	eventos      portout.EventoRepository
	participants portout.ParticipantRepository
	provider     portout.OutboundTransferProvider
	audit        portout.OutboundAuditLogRepository
}

// NewApproveOutboundRequest wires the approve use case.
func NewApproveOutboundRequest(
	requests portout.OutboundRequestRepository,
	eventos portout.EventoRepository,
	participants portout.ParticipantRepository,
	provider portout.OutboundTransferProvider,
	audit portout.OutboundAuditLogRepository,
) *ApproveOutboundRequest {
	return &ApproveOutboundRequest{
		requests:     requests,
		eventos:      eventos,
		participants: participants,
		provider:     provider,
		audit:        audit,
	}
}

// Execute approves the request and runs the provider.
func (uc *ApproveOutboundRequest) Execute(ctx context.Context, in portin.ApproveOutboundRequestInput) (*domain.OutboundRequest, error) {
	req, err := uc.requests.FindByID(ctx, in.RequestID)
	if err != nil {
		return nil, err
	}
	// Optional event scoping when supplied.
	if in.EventID != "" && in.EventID != req.EventID {
		return nil, apierr.NotFoundMsg("solicitação outbound não encontrada para o evento informado")
	}

	// Anti-enumeration: non-organizers see 404 instead of 403.
	ok, err := isOrganizer(ctx, uc.eventos, uc.participants, req.EventID, in.ApproverUserID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, apierr.NotFoundMsg("solicitação outbound não encontrada")
	}

	if err := req.Approve(in.ApproverUserID, in.ApprovalNotes); err != nil {
		return nil, apierr.Unprocessable(err.Error())
	}
	saved, err := uc.requests.Update(ctx, req)
	if err != nil {
		return nil, err
	}
	appendAudit(ctx, uc.audit, saved, in.ApproverUserID, domain.AuditActionApproved, in.ApprovalNotes)
	return executeProviderAndPersist(ctx, uc.requests, uc.provider, uc.audit, saved)
}

var _ portin.ApproveOutboundRequestUseCase = (*ApproveOutboundRequest)(nil)
