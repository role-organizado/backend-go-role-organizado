package asaas

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
)

// NoopOutboundTransferProvider is a dev/test stub that always succeeds without
// hitting any external service. Used when ROLE_OUTBOUND_PROVIDER=noop or when
// the Asaas token is not configured.
type NoopOutboundTransferProvider struct{}

// NewNoopOutboundTransferProvider returns a new no-op provider.
func NewNoopOutboundTransferProvider() *NoopOutboundTransferProvider {
	return &NoopOutboundTransferProvider{}
}

// Compile-time interface assertion.
var _ portout.OutboundTransferProvider = (*NoopOutboundTransferProvider)(nil)

// IsEnabled always returns true; the noop provider is the safe default for dev.
func (NoopOutboundTransferProvider) IsEnabled() bool { return true }

// ExecuteTransfer logs the attempt and returns a fake provider transfer ID.
func (NoopOutboundTransferProvider) ExecuteTransfer(ctx context.Context, req *portout.OutboundTransferRequest) (*portout.OutboundTransferResponse, error) {
	fakeID := "noop-" + uuid.New().String()
	slog.InfoContext(ctx, "noop outbound transfer executed",
		"outboundRequestID", req.OutboundRequestID,
		"amountCents", req.AmountCents,
		"providerTransferID", fakeID,
	)
	return &portout.OutboundTransferResponse{
		Success:            true,
		Provider:           "NOOP",
		ProviderTransferID: fakeID,
		Status:             "PENDING",
	}, nil
}
