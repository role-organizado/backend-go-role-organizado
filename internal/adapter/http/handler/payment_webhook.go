package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// PaymentWebhookHandler handles inbound payment-provider webhook callbacks.
// Routes are public: /api/v1/webhooks/ is in publicPrefixes and bypasses JWT.
type PaymentWebhookHandler struct {
	callbackUC   portin.HandlePaymentCallbackUseCase
	webhookToken string // expected asaas-access-token header value
	// empty string → dev/permissive mode (accept any token, mirrors Java behaviour)
}

// NewPaymentWebhookHandler creates a new PaymentWebhookHandler.
// webhookToken is the value of ROLE_ASAAS_WEBHOOK_TOKEN; pass "" for dev mode.
func NewPaymentWebhookHandler(
	callbackUC portin.HandlePaymentCallbackUseCase,
	webhookToken string,
) *PaymentWebhookHandler {
	return &PaymentWebhookHandler{
		callbackUC:   callbackUC,
		webhookToken: webhookToken,
	}
}

// RegisterWebhookRoutes mounts the Asaas webhook endpoint on the router.
// The /api/v1/webhooks/ prefix is in publicPrefixes (no JWT required).
func (h *PaymentWebhookHandler) RegisterWebhookRoutes(r chi.Router) {
	r.Post("/api/v1/webhooks/payment/asaas", h.HandleAsaasCallback)
}

// asaasWebhookPayload is a tolerant struct for the Asaas inbound webhook body.
// All fields are optional to accommodate future Asaas payload changes.
//
// Asaas event structure (typical):
//
//	{
//	  "id": "evt_...",
//	  "event": "PAYMENT_RECEIVED",
//	  "payment": {
//	    "id": "pay_...",
//	    "status": "RECEIVED",
//	    "externalReference": "..."
//	  }
//	}
type asaasWebhookPayload struct {
	ID     string `json:"id"`
	Event  string `json:"event"`
	Status string `json:"status"` // top-level status (fallback)

	Payment *asaasPaymentObject `json:"payment"`

	// Legacy/direct field used by some Asaas event types.
	ProviderTransactionID string `json:"providerTransactionId"`
}

type asaasPaymentObject struct {
	ID                string `json:"id"`
	Status            string `json:"status"`
	ExternalReference string `json:"externalReference"`
}

// extractStatus returns the most specific available status.
// Primary source: payment.status; fallback: top-level status field.
func (p *asaasWebhookPayload) extractStatus() string {
	if p.Payment != nil && p.Payment.Status != "" {
		return p.Payment.Status
	}
	return p.Status
}

// extractProviderTransactionID returns the provider transaction ID using the
// Java-identical fallback chain:
//  1. payment.id
//  2. payment.externalReference
//  3. top-level providerTransactionId
func (p *asaasWebhookPayload) extractProviderTransactionID() string {
	if p.Payment != nil {
		if p.Payment.ID != "" {
			return p.Payment.ID
		}
		if p.Payment.ExternalReference != "" {
			return p.Payment.ExternalReference
		}
	}
	return p.ProviderTransactionID
}

// HandleAsaasCallback handles POST /api/v1/webhooks/payment/asaas.
//
// Response contract (mirrors Java PaymentWebhookController):
//   - 200 OK for any valid, processed, or already-processed payload.
//   - 401 Unauthorized when the asaas-access-token header is wrong.
//   - 400 Bad Request for malformed JSON or missing required fields.
//
// Processing errors (e.g. DB transient failure) are logged but still return 200
// to prevent Asaas from retrying indefinitely; reconciliation handles corrections.
func (h *PaymentWebhookHandler) HandleAsaasCallback(w http.ResponseWriter, r *http.Request) {
	// ── Token validation ────────────────────────────────────────────────────────
	// When webhookToken is empty the server is in dev mode — accept all calls.
	if h.webhookToken != "" {
		token := r.Header.Get("asaas-access-token")
		if token != h.webhookToken {
			slog.WarnContext(r.Context(), "payment webhook: invalid asaas-access-token received")
			writeError(w, apierr.Unauthorized("invalid asaas-access-token"))
			return
		}
	}

	// ── Parse payload ───────────────────────────────────────────────────────────
	var payload asaasWebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, apierr.BadRequest("invalid webhook payload"))
		return
	}

	if payload.ID == "" {
		writeError(w, apierr.BadRequest("webhook payload missing required 'id' field"))
		return
	}

	// ── Build normalised callback input ─────────────────────────────────────────
	callbackIn := portin.PaymentCallbackPayload{
		Provider:              domain.PaymentProviderAsaas,
		ProviderEventID:       payload.ID,
		ProviderTransactionID: payload.extractProviderTransactionID(),
		EventType:             payload.Event,
		NewStatus:             payload.extractStatus(),
	}

	// ── Execute use case ────────────────────────────────────────────────────────
	if err := h.callbackUC.Execute(r.Context(), callbackIn); err != nil {
		// Processing errors are intentionally non-fatal at the HTTP layer:
		// returning 200 prevents Asaas from infinite retries; reconciliation
		// will correct any resulting inconsistency.
		slog.ErrorContext(r.Context(), "payment webhook: callback processing failed",
			"eventID", payload.ID,
			"event", payload.Event,
			"providerTransactionID", callbackIn.ProviderTransactionID,
			"error", err,
		)
	}

	// Always 200 for processed or already-processed payloads.
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
