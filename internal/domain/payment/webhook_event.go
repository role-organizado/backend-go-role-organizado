package payment

import "time"

// ProcessedWebhookEvent records a webhook event that has already been handled,
// ensuring idempotent processing of provider callbacks.
type ProcessedWebhookEvent struct {
	ID                    string
	Provider              string
	EventID               string // provider-assigned webhook event ID
	ProviderTransactionID string
	EventType             string
	ProcessedAt           time.Time
}
