package in

import (
	"context"

	notification "github.com/role-organizado/backend-go-role-organizado/internal/domain/notification"
)

// HandleWhatsAppWebhookUseCase processes an inbound webhook payload from Meta's
// WhatsApp Business Cloud API.
//
// Contract (mirrors Java's WhatsAppWebhookController.handleWhatsAppWebhook):
//   - The HTTP handler MUST always respond 200 to Meta; this use case therefore
//     returns nil for "successfully processed", "duplicate (idempotent skip)",
//     and "ignored (object not supported / empty entries)" outcomes alike.
//   - It returns a non-nil error only for unrecoverable internal failures
//     (e.g. database connectivity loss) — the handler logs the error but still
//     replies 200 so Meta does not retry indefinitely.
type HandleWhatsAppWebhookUseCase interface {
	Execute(ctx context.Context, payload notification.WhatsAppWebhookPayload) error
}
