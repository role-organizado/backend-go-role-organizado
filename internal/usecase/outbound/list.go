package outbound

import (
	"context"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/outbound"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
)

// ListOutboundRequests implements portin.ListOutboundRequestsUseCase.
type ListOutboundRequests struct {
	requests     portout.OutboundRequestRepository
	eventos      portout.EventoRepository
	participants portout.ParticipantRepository
}

// NewListOutboundRequests wires the list use case.
func NewListOutboundRequests(
	requests portout.OutboundRequestRepository,
	eventos portout.EventoRepository,
	participants portout.ParticipantRepository,
) *ListOutboundRequests {
	return &ListOutboundRequests{requests: requests, eventos: eventos, participants: participants}
}

// ListByEvent returns all outbound requests for an event after participant check.
func (uc *ListOutboundRequests) ListByEvent(ctx context.Context, userID, eventID string) ([]domain.OutboundRequest, error) {
	if err := requireEventParticipant(ctx, uc.eventos, uc.participants, eventID, userID); err != nil {
		return nil, err
	}
	return uc.requests.FindByEventID(ctx, eventID)
}

// ListByEventAndStatus filters by status.
func (uc *ListOutboundRequests) ListByEventAndStatus(ctx context.Context, userID, eventID string, status domain.OutboundStatus) ([]domain.OutboundRequest, error) {
	if err := requireEventParticipant(ctx, uc.eventos, uc.participants, eventID, userID); err != nil {
		return nil, err
	}
	return uc.requests.FindByEventIDAndStatus(ctx, eventID, status)
}

// ListByEventAndType filters by type.
func (uc *ListOutboundRequests) ListByEventAndType(ctx context.Context, userID, eventID string, t domain.OutboundType) ([]domain.OutboundRequest, error) {
	if err := requireEventParticipant(ctx, uc.eventos, uc.participants, eventID, userID); err != nil {
		return nil, err
	}
	return uc.requests.FindByEventIDAndType(ctx, eventID, t)
}

// ListPendingByEvent filters by PENDING status.
func (uc *ListOutboundRequests) ListPendingByEvent(ctx context.Context, userID, eventID string) ([]domain.OutboundRequest, error) {
	if err := requireEventParticipant(ctx, uc.eventos, uc.participants, eventID, userID); err != nil {
		return nil, err
	}
	return uc.requests.FindPendingByEventID(ctx, eventID)
}

// CountPendingByEvent returns the pending count.
func (uc *ListOutboundRequests) CountPendingByEvent(ctx context.Context, userID, eventID string) (int64, error) {
	if err := requireEventParticipant(ctx, uc.eventos, uc.participants, eventID, userID); err != nil {
		return 0, err
	}
	return uc.requests.CountPendingByEventID(ctx, eventID)
}

// ListMyRequests returns the JWT user's own requests across all events.
func (uc *ListOutboundRequests) ListMyRequests(ctx context.Context, userID string) ([]domain.OutboundRequest, error) {
	return uc.requests.FindByRequesterUserID(ctx, userID)
}

var _ portin.ListOutboundRequestsUseCase = (*ListOutboundRequests)(nil)
