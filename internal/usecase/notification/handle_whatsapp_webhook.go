package notification

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	notifdomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/notification"
	paymentdomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
)

// HandleWhatsAppWebhook implements portin.HandleWhatsAppWebhookUseCase.
//
// The ProcessedWebhookEventRepository is the same one used by the Asaas payment
// webhook flow — we differentiate WhatsApp events purely via the `provider`
// column ("WHATSAPP") on the persisted record, matching the Java backend's
// idempotency contract.
type HandleWhatsAppWebhook struct {
	webhooks portout.ProcessedWebhookEventRepository
	now      func() time.Time
}

// Compile-time assertion.
var _ portin.HandleWhatsAppWebhookUseCase = (*HandleWhatsAppWebhook)(nil)

// NewHandleWhatsAppWebhook builds the use case.
func NewHandleWhatsAppWebhook(repo portout.ProcessedWebhookEventRepository) *HandleWhatsAppWebhook {
	return &HandleWhatsAppWebhook{
		webhooks: repo,
		now:      func() time.Time { return time.Now().UTC() },
	}
}

// WithClock overrides the timestamp source (used by tests for deterministic
// ProcessedAt assertions).
func (uc *HandleWhatsAppWebhook) WithClock(now func() time.Time) *HandleWhatsAppWebhook {
	uc.now = now
	return uc
}

// Execute applies the Meta-required processing rules:
//
//  1. If object != "whatsapp_business_account", ignore silently (no persist).
//  2. If entry[] is absent or empty, treat as a verification ping (no persist).
//  3. For every status and message event, build the idempotency key
//     "<entryId>:<id>:<status|message>" and try to persist a
//     ProcessedWebhookEvent record. Duplicates are no-ops.
//
// Every branch returns nil — Meta requires a 200 response for any payload, so
// the use case never blocks the HTTP handler from replying success. Persistence
// errors are logged but not propagated (transient DB issues will resolve on the
// next event; the unique index is the final guard).
func (uc *HandleWhatsAppWebhook) Execute(ctx context.Context, payload notifdomain.WhatsAppWebhookPayload) error {
	if payload.Object != "" && payload.Object != "whatsapp_business_account" {
		slog.WarnContext(ctx, "whatsapp webhook: ignoring unsupported object",
			"object", payload.Object,
		)
		return nil
	}

	if len(payload.Entry) == 0 {
		slog.DebugContext(ctx, "whatsapp webhook: empty entry — likely verification ping")
		return nil
	}

	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			uc.processStatuses(ctx, entry.ID, change.Value.Statuses)
			uc.processMessages(ctx, entry.ID, change.Value.Messages)
		}
	}
	return nil
}

func (uc *HandleWhatsAppWebhook) processStatuses(ctx context.Context, entryID string, statuses []notifdomain.WhatsAppWebhookStatus) {
	for _, st := range statuses {
		if st.ID == "" {
			slog.WarnContext(ctx, "whatsapp webhook: status missing id — skipping")
			continue
		}
		eventID := fmt.Sprintf("%s:%s:status", entryID, st.ID)
		eventType := "STATUS_" + strings.ToUpper(st.Status)
		uc.persist(ctx, eventID, st.ID, eventType)
	}
}

func (uc *HandleWhatsAppWebhook) processMessages(ctx context.Context, entryID string, messages []notifdomain.WhatsAppWebhookMessage) {
	for _, msg := range messages {
		if msg.ID == "" {
			slog.WarnContext(ctx, "whatsapp webhook: inbound message missing id — skipping")
			continue
		}
		eventID := fmt.Sprintf("%s:%s:message", entryID, msg.ID)
		eventType := "MESSAGE_" + strings.ToUpper(msg.Type)
		uc.persist(ctx, eventID, msg.ID, eventType)
	}
}

// persist runs the dedup-then-save sequence, swallowing ErrAlreadyProcessed and
// logging any unexpected error.
func (uc *HandleWhatsAppWebhook) persist(ctx context.Context, eventID, providerTxID, eventType string) {
	exists, err := uc.webhooks.ExistsByProviderAndEventID(ctx, notifdomain.WhatsAppProvider, eventID)
	if err != nil {
		slog.ErrorContext(ctx, "whatsapp webhook: dedup check failed",
			"eventId", eventID,
			"error", err,
		)
		return
	}
	if exists {
		slog.DebugContext(ctx, "whatsapp webhook: duplicate event — skipping",
			"eventId", eventID,
		)
		return
	}

	saveErr := uc.webhooks.SaveUnique(ctx, &paymentdomain.ProcessedWebhookEvent{
		Provider:              notifdomain.WhatsAppProvider,
		EventID:               eventID,
		ProviderTransactionID: providerTxID,
		EventType:             eventType,
		ProcessedAt:           uc.now(),
	})
	if saveErr == nil {
		slog.InfoContext(ctx, "whatsapp webhook: event recorded",
			"eventId", eventID,
			"eventType", eventType,
		)
		return
	}
	if errors.Is(saveErr, portout.ErrAlreadyProcessed) {
		// Race condition between Exists* check and SaveUnique — the unique
		// index caught the duplicate, which is the contract we rely on.
		slog.DebugContext(ctx, "whatsapp webhook: race-duplicate caught by unique index",
			"eventId", eventID,
		)
		return
	}
	slog.ErrorContext(ctx, "whatsapp webhook: persist failed",
		"eventId", eventID,
		"error", saveErr,
	)
}
