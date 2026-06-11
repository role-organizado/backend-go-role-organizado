package outbound

import (
	"context"
	"log/slog"
	"strings"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/outbound"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// HandleOutboundTransferCallback implements portin.HandleOutboundTransferCallbackUseCase.
//
// Maps provider terminal statuses to the request lifecycle. Idempotent: a
// callback for an already-terminal request is a no-op. Webhook anti-replay (by
// providerEventId) is handled by the ProcessedWebhookEvent repository at the
// handler boundary.
type HandleOutboundTransferCallback struct {
	requests portout.OutboundRequestRepository
	audit    portout.OutboundAuditLogRepository
}

// NewHandleOutboundTransferCallback wires the callback handler use case.
func NewHandleOutboundTransferCallback(requests portout.OutboundRequestRepository, audit portout.OutboundAuditLogRepository) *HandleOutboundTransferCallback {
	return &HandleOutboundTransferCallback{requests: requests, audit: audit}
}

// terminal-success statuses from the spec (mirrors Java mapping).
var outboundTerminalSuccess = map[string]bool{
	"DONE":      true,
	"CONFIRMED": true,
	"COMPLETED": true,
	"RECEIVED":  true,
}

// terminal-failure statuses from the spec (mirrors Java mapping).
var outboundTerminalFailure = map[string]bool{
	"FAILED":          true,
	"REJECTED":        true,
	"CANCELLED":       true,
	"CANCELED":        true,
	"ERROR":           true,
	"REFUNDED":        true,
	"CHARGEBACK":      true,
	"OVERDUE":         true,
	"TIMEOUT_CALLBACK": true,
}

// Execute applies the provider callback to the request lifecycle.
func (uc *HandleOutboundTransferCallback) Execute(ctx context.Context, in portin.OutboundCallbackInput) (*domain.OutboundRequest, error) {
	if strings.TrimSpace(in.RequestID) == "" {
		return nil, apierr.BadRequest("requestId é obrigatório")
	}
	req, err := uc.requests.FindByID(ctx, in.RequestID)
	if err != nil {
		return nil, err
	}
	// Already terminal — idempotent no-op.
	if req.IsTerminal() {
		slog.InfoContext(ctx, "outbound callback: request already terminal, no-op",
			"requestID", req.ID, "status", req.Status)
		return req, nil
	}

	upper := strings.ToUpper(strings.TrimSpace(in.ProviderStatus))
	switch {
	case outboundTerminalSuccess[upper]:
		// Only PROCESSING transitions to COMPLETED — other states are unexpected.
		if req.Status != domain.StatusProcessing {
			slog.InfoContext(ctx, "outbound callback: success status for non-PROCESSING request, marking COMPLETED",
				"requestID", req.ID, "status", req.Status)
		}
		req.MarkAsCompleted()
		appendAudit(ctx, uc.audit, req, "system", domain.AuditActionCompleted, upper)
	case outboundTerminalFailure[upper]:
		req.MarkAsFailed(strings.TrimSpace(in.Reason + " " + upper))
		appendAudit(ctx, uc.audit, req, "system", domain.AuditActionFailed, upper)
	default:
		// Non-terminal status — no-op for idempotency.
		slog.DebugContext(ctx, "outbound callback: non-terminal status, skipping",
			"requestID", req.ID, "status", upper)
		return req, nil
	}

	return uc.requests.Update(ctx, req)
}

var _ portin.HandleOutboundTransferCallbackUseCase = (*HandleOutboundTransferCallback)(nil)
