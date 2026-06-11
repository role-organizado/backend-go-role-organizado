package outbound

import (
	"context"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/outbound"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// GetOutboundRequest implements portin.GetOutboundRequestUseCase.
type GetOutboundRequest struct {
	requests     portout.OutboundRequestRepository
	eventos      portout.EventoRepository
	participants portout.ParticipantRepository
}

// NewGetOutboundRequest wires the get use case.
func NewGetOutboundRequest(
	requests portout.OutboundRequestRepository,
	eventos portout.EventoRepository,
	participants portout.ParticipantRepository,
) *GetOutboundRequest {
	return &GetOutboundRequest{requests: requests, eventos: eventos, participants: participants}
}

// Execute fetches the request and validates the user can read it.
func (uc *GetOutboundRequest) Execute(ctx context.Context, userID, requestID string) (*domain.OutboundRequest, error) {
	req, err := uc.requests.FindByID(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if err := requireEventParticipant(ctx, uc.eventos, uc.participants, req.EventID, userID); err != nil {
		// Anti-enumeration: 404 instead of 403.
		if _, ok := err.(*apierr.APIError); ok {
			return nil, err
		}
		return nil, err
	}
	populateAttachmentURL(req)
	return req, nil
}

// ExecuteByEvent enforces both ID and eventID scoping.
func (uc *GetOutboundRequest) ExecuteByEvent(ctx context.Context, userID, requestID, eventID string) (*domain.OutboundRequest, error) {
	req, err := uc.requests.FindByIDAndEventID(ctx, requestID, eventID)
	if err != nil {
		return nil, err
	}
	if err := requireEventParticipant(ctx, uc.eventos, uc.participants, req.EventID, userID); err != nil {
		return nil, err
	}
	populateAttachmentURL(req)
	return req, nil
}

// populateAttachmentURL sets the transient URL on the request's attachment.
func populateAttachmentURL(req *domain.OutboundRequest) {
	if req == nil {
		return
	}
	if req.AttachmentID == "" {
		return
	}
	url := "/api/anexos/" + req.AttachmentID
	if req.Attachment == nil {
		req.Attachment = &domain.Attachment{ID: req.AttachmentID}
	}
	req.Attachment.URL = url
}

var _ portin.GetOutboundRequestUseCase = (*GetOutboundRequest)(nil)
