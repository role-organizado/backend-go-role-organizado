package out

import (
	"context"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/outbound"
)

// OutboundRequestRepository defines persistence for OutboundRequest entities.
type OutboundRequestRepository interface {
	Save(ctx context.Context, r *domain.OutboundRequest) (*domain.OutboundRequest, error)
	Update(ctx context.Context, r *domain.OutboundRequest) (*domain.OutboundRequest, error)
	FindByID(ctx context.Context, id string) (*domain.OutboundRequest, error)
	FindByIDAndEventID(ctx context.Context, id, eventID string) (*domain.OutboundRequest, error)
	FindByEventID(ctx context.Context, eventID string) ([]domain.OutboundRequest, error)
	FindByEventIDAndStatus(ctx context.Context, eventID string, status domain.OutboundStatus) ([]domain.OutboundRequest, error)
	FindByEventIDAndType(ctx context.Context, eventID string, t domain.OutboundType) ([]domain.OutboundRequest, error)
	FindByRequesterUserID(ctx context.Context, userID string) ([]domain.OutboundRequest, error)
	FindPendingByEventID(ctx context.Context, eventID string) ([]domain.OutboundRequest, error)
	CountPendingByEventID(ctx context.Context, eventID string) (int64, error)
	ExistsActiveByRateioID(ctx context.Context, rateioID string) (bool, error)
	FindByProviderTransferID(ctx context.Context, providerTransferID string) (*domain.OutboundRequest, error)
	DeleteByID(ctx context.Context, id string) error
}

// OutboundAuditLogRepository persists structured audit entries for the outbound flow.
type OutboundAuditLogRepository interface {
	Append(ctx context.Context, entry *domain.AuditLog) error
	FindByRequestID(ctx context.Context, requestID string) ([]domain.AuditLog, error)
}

// OutboundTransferRequest is the contract handed to the OutboundTransferProvider.
type OutboundTransferRequest struct {
	OutboundRequestID string
	EventID           string
	RateioID          string
	AmountCents       int64
	RecipientName     string
	RecipientDocument string
	PixKey            string
	PixKeyType        domain.PixKeyType
	Description       string
}

// OutboundTransferResponse is the result returned by the provider.
type OutboundTransferResponse struct {
	Success            bool
	Provider           string
	ProviderTransferID string
	Status             string
	ErrorMessage       string
}

// OutboundTransferProvider executes the actual money movement at the PSP.
// IsEnabled controls whether the provider should be used by the approval flow.
type OutboundTransferProvider interface {
	IsEnabled() bool
	ExecuteTransfer(ctx context.Context, req *OutboundTransferRequest) (*OutboundTransferResponse, error)
}
