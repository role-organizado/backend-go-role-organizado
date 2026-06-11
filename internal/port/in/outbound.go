package in

import (
	"context"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/outbound"
)

// ─── Inputs ───────────────────────────────────────────────────────────────────

// CreateOutboundRequestInput carries the data needed to create a new request.
type CreateOutboundRequestInput struct {
	UserID                string
	EventID               string
	Type                  domain.OutboundType
	AmountCents           int64
	Justification         string
	RateioID              string
	PaymentAccountID      string
	RecipientName         string
	RecipientDocument     string
	PixKeyType            domain.PixKeyType
	PixKey                string
	AttachmentID          string
	AttachmentFilename    string
	AttachmentContentType string
	AttachmentSize        int64
}

// ApproveOutboundRequestInput is the body for the approve endpoint.
type ApproveOutboundRequestInput struct {
	RequestID      string
	EventID        string
	ApproverUserID string
	ApprovalNotes  string
}

// RejectOutboundRequestInput is the body for the reject endpoint.
type RejectOutboundRequestInput struct {
	RequestID       string
	EventID         string
	RejecterUserID  string
	RejectionReason string
}

// CancelOutboundRequestInput is the body for the cancel endpoint.
type CancelOutboundRequestInput struct {
	RequestID          string
	UserID             string
	CancellationReason string
}

// VoteOnOutboundRequestInput is the body for the vote endpoint.
type VoteOnOutboundRequestInput struct {
	RequestID string
	EventID   string
	UserID    string
	Approve   bool
	Comment   string
}

// OutboundCallbackInput is the normalized input for the Asaas outbound webhook.
type OutboundCallbackInput struct {
	RequestID      string
	Provider       string
	ProviderStatus string
	ProviderEventID string
	Reason         string
}

// ListOutboundRequestsInput filters the list query.
type ListOutboundRequestsInput struct {
	UserID  string
	EventID string
	Status  *domain.OutboundStatus
	Type    *domain.OutboundType
}

// ─── Outputs ──────────────────────────────────────────────────────────────────

// VoteResult is the response of VoteOnOutboundRequestUseCase.
type VoteResult struct {
	Request         *domain.OutboundRequest
	TotalVotes      int
	ApproveVotes    int
	RejectVotes     int
	RequiredVotes   int
	VotingComplete  bool
	FinalStatus     domain.OutboundStatus
}

// OutboundParticipant enriches rateio participants with display info for the
// details endpoint.
type OutboundParticipant struct {
	UserID    string
	Name      string
	Email     string
	IsGuest   bool
	HasVoted  bool
}

// OutboundDetailsResult is the rich detail view returned by the details endpoint.
type OutboundDetailsResult struct {
	Request                  *domain.OutboundRequest
	RateioParticipants       []OutboundParticipant
	CanVote                  bool
	HasUserVoted             bool
	IsOrganizer              bool
	TotalVotes               int
	ApprovalsCount           int
	RejectionsCount          int
	TotalRateioParticipants  int
	RequiredVotes            int
}

// ─── Use-case interfaces ──────────────────────────────────────────────────────

// CreateOutboundRequestUseCase creates a new outbound request.
type CreateOutboundRequestUseCase interface {
	Execute(ctx context.Context, in CreateOutboundRequestInput) (*domain.OutboundRequest, error)
}

// GetOutboundRequestUseCase fetches a single request.
type GetOutboundRequestUseCase interface {
	Execute(ctx context.Context, userID, requestID string) (*domain.OutboundRequest, error)
	ExecuteByEvent(ctx context.Context, userID, requestID, eventID string) (*domain.OutboundRequest, error)
}

// GetOutboundRequestDetailsUseCase returns the rich detail view.
type GetOutboundRequestDetailsUseCase interface {
	Execute(ctx context.Context, userID, requestID string) (*OutboundDetailsResult, error)
}

// ListOutboundRequestsUseCase exposes the list variants.
type ListOutboundRequestsUseCase interface {
	ListByEvent(ctx context.Context, userID, eventID string) ([]domain.OutboundRequest, error)
	ListByEventAndStatus(ctx context.Context, userID, eventID string, status domain.OutboundStatus) ([]domain.OutboundRequest, error)
	ListByEventAndType(ctx context.Context, userID, eventID string, t domain.OutboundType) ([]domain.OutboundRequest, error)
	ListPendingByEvent(ctx context.Context, userID, eventID string) ([]domain.OutboundRequest, error)
	CountPendingByEvent(ctx context.Context, userID, eventID string) (int64, error)
	ListMyRequests(ctx context.Context, userID string) ([]domain.OutboundRequest, error)
}

// ApproveOutboundRequestUseCase performs organizer approval.
type ApproveOutboundRequestUseCase interface {
	Execute(ctx context.Context, in ApproveOutboundRequestInput) (*domain.OutboundRequest, error)
}

// RejectOutboundRequestUseCase performs organizer rejection.
type RejectOutboundRequestUseCase interface {
	Execute(ctx context.Context, in RejectOutboundRequestInput) (*domain.OutboundRequest, error)
}

// CancelOutboundRequestUseCase cancels a request (requester-only).
type CancelOutboundRequestUseCase interface {
	Execute(ctx context.Context, in CancelOutboundRequestInput) (*domain.OutboundRequest, error)
}

// VoteOnOutboundRequestUseCase records a vote and applies quorum.
type VoteOnOutboundRequestUseCase interface {
	Execute(ctx context.Context, in VoteOnOutboundRequestInput) (*VoteResult, error)
}

// HandleOutboundTransferCallbackUseCase processes inbound provider callbacks.
type HandleOutboundTransferCallbackUseCase interface {
	Execute(ctx context.Context, in OutboundCallbackInput) (*domain.OutboundRequest, error)
}
