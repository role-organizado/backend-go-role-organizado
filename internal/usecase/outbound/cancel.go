package outbound

import (
	"context"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/outbound"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// CancelOutboundRequest implements portin.CancelOutboundRequestUseCase.
// Only the original requester can cancel, and only while the request is PENDING.
type CancelOutboundRequest struct {
	requests portout.OutboundRequestRepository
	audit    portout.OutboundAuditLogRepository
}

// NewCancelOutboundRequest wires the cancel use case.
func NewCancelOutboundRequest(requests portout.OutboundRequestRepository, audit portout.OutboundAuditLogRepository) *CancelOutboundRequest {
	return &CancelOutboundRequest{requests: requests, audit: audit}
}

// Execute cancels the request.
func (uc *CancelOutboundRequest) Execute(ctx context.Context, in portin.CancelOutboundRequestInput) (*domain.OutboundRequest, error) {
	req, err := uc.requests.FindByID(ctx, in.RequestID)
	if err != nil {
		return nil, err
	}
	// Anti-enumeration: non-owners see 404.
	if req.RequesterUserID != in.UserID {
		return nil, apierr.NotFoundMsg("solicitação outbound não encontrada")
	}
	if err := req.Cancel(in.CancellationReason); err != nil {
		return nil, apierr.Unprocessable(err.Error())
	}
	saved, err := uc.requests.Update(ctx, req)
	if err != nil {
		return nil, err
	}
	appendAudit(ctx, uc.audit, saved, in.UserID, domain.AuditActionCancelled, in.CancellationReason)
	return saved, nil
}

var _ portin.CancelOutboundRequestUseCase = (*CancelOutboundRequest)(nil)
