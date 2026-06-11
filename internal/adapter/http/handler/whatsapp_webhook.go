package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	notifdomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/notification"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
)

// WhatsAppWebhookHandler handles Meta WhatsApp Business Cloud API webhooks.
//
// Routes (registered as PUBLIC — bypass JWT via /api/v1/webhooks/ prefix):
//
//   - POST /api/v1/webhooks/notifications/whatsapp
//     Receives event callbacks (messages + statuses). Always returns 200 so
//     Meta never retries; processing errors are logged but never surfaced.
//
//   - GET /api/v1/webhooks/notifications/whatsapp
//     Meta hub.challenge verification handshake. When hub.mode=subscribe and
//     hub.verify_token matches the configured token, echoes hub.challenge as
//     text/plain with 200. Otherwise returns 403.
type WhatsAppWebhookHandler struct {
	uc          portin.HandleWhatsAppWebhookUseCase
	verifyToken string // expected hub.verify_token; empty disables verification (dev mode)
}

// NewWhatsAppWebhookHandler creates a new WhatsAppWebhookHandler.
// verifyToken is the value of ROLE_WHATSAPP_WEBHOOK_VERIFY_TOKEN. Pass "" to
// accept any verify_token (dev/local mode) — mirrors the Asaas webhook pattern.
func NewWhatsAppWebhookHandler(uc portin.HandleWhatsAppWebhookUseCase, verifyToken string) *WhatsAppWebhookHandler {
	return &WhatsAppWebhookHandler{uc: uc, verifyToken: verifyToken}
}

// RegisterWebhookRoutes mounts both the POST callback and the GET verification
// route. The /api/v1/webhooks/ prefix is in publicPrefixes so no JWT is needed.
func (h *WhatsAppWebhookHandler) RegisterWebhookRoutes(r chi.Router) {
	r.Post("/api/v1/webhooks/notifications/whatsapp", h.HandleWebhook)
	r.Get("/api/v1/webhooks/notifications/whatsapp", h.HandleVerify)
}

// HandleVerify implements the Meta GET handshake described at
// https://developers.facebook.com/docs/graph-api/webhooks/getting-started.
//
// Query parameters:
//
//   - hub.mode         — must be exactly "subscribe"
//   - hub.verify_token — must match h.verifyToken (when non-empty)
//   - hub.challenge    — opaque value to be echoed back in the response body
//
// Successful response: 200 with the raw hub.challenge as text/plain.
// Any mismatch: 403 with an empty body.
func (h *WhatsAppWebhookHandler) HandleVerify(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	mode := q.Get("hub.mode")
	token := q.Get("hub.verify_token")
	challenge := q.Get("hub.challenge")

	if mode != "subscribe" {
		slog.WarnContext(r.Context(), "whatsapp verify: invalid hub.mode",
			"hub.mode", mode,
		)
		w.WriteHeader(http.StatusForbidden)
		return
	}

	// Empty verifyToken means dev mode — accept any token (but mode must still
	// be "subscribe" as required by the Meta contract).
	if h.verifyToken != "" && token != h.verifyToken {
		slog.WarnContext(r.Context(), "whatsapp verify: token mismatch")
		w.WriteHeader(http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(challenge))
}

// HandleWebhook receives the POST event callback. Per Meta's contract this
// endpoint MUST always reply 200 (any 4xx/5xx triggers infinite retries).
// Both malformed JSON and downstream processing errors are logged but still
// surface as a 200 {"status":"ok"} body.
func (h *WhatsAppWebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	var payload notifdomain.WhatsAppWebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		slog.WarnContext(r.Context(), "whatsapp webhook: malformed json — still returning 200",
			"error", err,
		)
		respondOK(w)
		return
	}

	if err := h.uc.Execute(r.Context(), payload); err != nil {
		// Use case never returns errors today, but if a future implementation
		// surfaces one we still want to reply 200 — log + carry on.
		slog.ErrorContext(r.Context(), "whatsapp webhook: processing failed — still returning 200",
			"object", payload.Object,
			"error", err,
		)
	}

	respondOK(w)
}

// respondOK is the canonical 200 reply shape used by every WhatsApp branch.
func respondOK(w http.ResponseWriter) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
