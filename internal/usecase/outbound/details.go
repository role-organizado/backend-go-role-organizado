package outbound

import (
	"context"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/outbound"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
)

// GetOutboundRequestDetails implements portin.GetOutboundRequestDetailsUseCase.
// Returns a rich detail view: request, enriched participants list, vote counts,
// and helper flags for the frontend (canVote, hasUserVoted, isOrganizer).
type GetOutboundRequestDetails struct {
	requests     portout.OutboundRequestRepository
	eventos      portout.EventoRepository
	participants portout.ParticipantRepository
}

// NewGetOutboundRequestDetails wires the details use case.
func NewGetOutboundRequestDetails(
	requests portout.OutboundRequestRepository,
	eventos portout.EventoRepository,
	participants portout.ParticipantRepository,
) *GetOutboundRequestDetails {
	return &GetOutboundRequestDetails{requests: requests, eventos: eventos, participants: participants}
}

// Execute returns the detail view.
func (uc *GetOutboundRequestDetails) Execute(ctx context.Context, userID, requestID string) (*portin.OutboundDetailsResult, error) {
	req, err := uc.requests.FindByID(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if err := requireEventParticipant(ctx, uc.eventos, uc.participants, req.EventID, userID); err != nil {
		return nil, err
	}
	populateAttachmentURL(req)

	orgFlag, _ := isOrganizer(ctx, uc.eventos, uc.participants, req.EventID, userID)

	// Build the participants view directly from event participants — this codebase
	// does not yet expose dedicated rateio-allocation participants, so we mirror the
	// event participant set as the eligible voter universe (excludes requester).
	parts, _, _ := uc.participants.FindByEventID(ctx, req.EventID, 0, 100)
	resultParts := make([]portin.OutboundParticipant, 0, len(parts))
	for _, p := range parts {
		if p.UserID == "" || p.UserID == req.RequesterUserID {
			continue
		}
		resultParts = append(resultParts, portin.OutboundParticipant{
			UserID:   p.UserID,
			Name:     p.Name,
			Email:    p.Email,
			IsGuest:  false,
			HasVoted: req.HasUserVoted(p.UserID),
		})
	}

	canVote := req.Status == domain.StatusPending &&
		req.RequiresVoting &&
		userID != req.RequesterUserID &&
		!req.HasUserVoted(userID)

	result := &portin.OutboundDetailsResult{
		Request:                 req,
		RateioParticipants:      resultParts,
		CanVote:                 canVote,
		HasUserVoted:            req.HasUserVoted(userID),
		IsOrganizer:             orgFlag,
		TotalVotes:              len(req.Votes),
		ApprovalsCount:          req.Approvals,
		RejectionsCount:         req.Rejections,
		TotalRateioParticipants: len(resultParts),
		RequiredVotes:           req.RequiredVotes,
	}
	return result, nil
}

var _ portin.GetOutboundRequestDetailsUseCase = (*GetOutboundRequestDetails)(nil)
