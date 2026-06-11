package notification

// Provider constants used when persisting ProcessedWebhookEvent records and
// when issuing dedup lookups. Keep the value in UPPER_SNAKE to mirror the Java
// backend's WHATSAPP literal (so cross-stack dedup queries hit the same docs).
const (
	WhatsAppProvider = "WHATSAPP"
)

// WhatsAppWebhookPayload is the top-level body Meta sends to the Business
// Cloud API webhook endpoint. We only care about object + entry[]; every other
// field is intentionally ignored to keep the surface forward-compatible.
//
// Reference shape:
//
//	{
//	  "object": "whatsapp_business_account",
//	  "entry": [{
//	    "id": "<waba_id>",
//	    "changes": [{
//	      "field": "messages",
//	      "value": {
//	        "messages": [{ "id": "wamid.HBg…", "type": "text", "from": "55…" }],
//	        "statuses": [{ "id": "wamid.HBg…", "status": "delivered" }]
//	      }
//	    }]
//	  }]
//	}
type WhatsAppWebhookPayload struct {
	Object string                  `json:"object"`
	Entry  []WhatsAppWebhookEntry  `json:"entry"`
}

// WhatsAppWebhookEntry is one element of the top-level entry array.
type WhatsAppWebhookEntry struct {
	ID      string                   `json:"id"`
	Changes []WhatsAppWebhookChange  `json:"changes"`
}

// WhatsAppWebhookChange is one element of entry[].changes.
type WhatsAppWebhookChange struct {
	Field string                  `json:"field"`
	Value WhatsAppWebhookValue    `json:"value"`
}

// WhatsAppWebhookValue is the actual payload carried inside a change.
// It holds either inbound messages, status updates, or both.
type WhatsAppWebhookValue struct {
	MessagingProduct string                   `json:"messaging_product"`
	Messages         []WhatsAppWebhookMessage `json:"messages"`
	Statuses         []WhatsAppWebhookStatus  `json:"statuses"`
}

// WhatsAppWebhookMessage represents an inbound message from a user.
type WhatsAppWebhookMessage struct {
	ID        string `json:"id"`
	From      string `json:"from"`
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
}

// WhatsAppWebhookStatus represents a delivery/read status update for a
// previously-sent message.
type WhatsAppWebhookStatus struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	Timestamp    string `json:"timestamp"`
	RecipientID  string `json:"recipient_id"`
}
