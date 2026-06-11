package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	paymentdomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// OutboundWebhookHandler handles inbound Asaas outbound-transfer webhooks.
// Public endpoint (no JWT) — the asaas-access-token header is the only auth.
type OutboundWebhookHandler struct {
	callbackUC   portin.HandleOutboundTransferCallbackUseCase
	webhookRepo  portout.ProcessedWebhookEventRepository
	webhookToken string
}

// NewOutboundWebhookHandler creates a new webhook handler.
// webhookToken may be empty in dev mode (token validation is bypassed).
func NewOutboundWebhookHandler(
	callbackUC portin.HandleOutboundTransferCallbackUseCase,
	webhookRepo portout.ProcessedWebhookEventRepository,
	webhookToken string,
) *OutboundWebhookHandler {
	return &OutboundWebhookHandler{
		callbackUC:   callbackUC,
		webhookRepo:  webhookRepo,
		webhookToken: webhookToken,
	}
}

// RegisterOutboundWebhookRoutes mounts the public Asaas outbound webhook route.
// IMPORTANT: This handler must be registered under a public prefix
// (publicPrefixes contains "/api/v1/webhooks/" in main.go).
func (h *OutboundWebhookHandler) RegisterOutboundWebhookRoutes(r chi.Router) {
	r.Post("/api/v1/webhooks/outbound/asaas", h.handleAsaasCallback)
}

// asaasOutboundPayload is a tolerant struct for the Asaas outbound webhook body.
type asaasOutboundPayload struct {
	ID                string `json:"id"`     // event ID (idempotency key)
	Event             string `json:"event"`
	Status            string `json:"status"`
	ExternalReference string `json:"externalReference"`

	Transfer *asaasTransferObject `json:"transfer"`
}

type asaasTransferObject struct {
	ID                string `json:"id"`
	Status            string `json:"status"`
	ExternalReference string `json:"externalReference"`
}

func (p *asaasOutboundPayload) extractRequestID() string {
	if p.ExternalReference != "" {
		return p.ExternalReference
	}
	if p.Transfer != nil && p.Transfer.ExternalReference != "" {
		return p.Transfer.ExternalReference
	}
	return ""
}

func (p *asaasOutboundPayload) extractStatus() string {
	if p.Transfer != nil && p.Transfer.Status != "" {
		return p.Transfer.Status
	}
	return p.Status
}

// handleAsaasCallback handles POST /api/v1/webhooks/outbound/asaas.
//
// Response contract:
//   - 200 OK for any valid, processed, or already-processed payload.
//   - 401 Unauthorized when the asaas-access-token header is wrong.
//   - 400 Bad Request for malformed JSON or missing required fields.
//
// Processing errors are intentionally swallowed (200 OK) so Asaas does not
// retry indefinitely.
func (h *OutboundWebhookHandler) handleAsaasCallback(w http.ResponseWriter, r *http.Request) {
	// Token validation.
	if h.webhookToken != "" {
		token := r.Header.Get("asaas-access-token")
		if token != h.webhookToken {
			slog.WarnContext(r.Context(), "outbound webhook: invalid asaas-access-token received")
			writeError(w, apierr.Unauthorized("invalid asaas-access-token"))
			return
		}
	}

	var payload asaasOutboundPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, apierr.BadRequest("invalid webhook payload"))
		return
	}

	if payload.ID == "" {
		writeError(w, apierr.BadRequest("webhook payload missing required 'id' field"))
		return
	}

	requestID := payload.extractRequestID()
	if requestID == "" {
		writeError(w, apierr.BadRequest("webhook payload missing requestId / externalReference"))
		return
	}
	status := payload.extractStatus()
	if status == "" {
		writeError(w, apierr.BadRequest("webhook payload missing status"))
		return
	}

	// Idempotency: anti-replay via (provider, eventId).
	if h.webhookRepo != nil {
		exists, err := h.webhookRepo.ExistsByProviderAndEventID(r.Context(), "ASAAS_OUTBOUND", payload.ID)
		if err != nil {
			slog.WarnContext(r.Context(), "outbound webhook: idempotency check failed (continuing)", "error", err)
		}
		if exists {
			slog.InfoContext(r.Context(), "outbound webhook: event already processed (no-op)",
				"provider", "ASAAS_OUTBOUND", "eventID", payload.ID)
			writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
			return
		}
		// Record event — race-safe via unique index.
		event := &paymentdomain.ProcessedWebhookEvent{
			ID:                    uuid.New().String(),
			Provider:              "ASAAS_OUTBOUND",
			EventID:               payload.ID,
			ProviderTransactionID: requestID,
			EventType:             payload.Event,
			ProcessedAt:           time.Now().UTC(),
		}
		if err := h.webhookRepo.SaveUnique(r.Context(), event); err != nil {
			if errors.Is(err, portout.ErrAlreadyProcessed) {
				writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
				return
			}
			slog.WarnContext(r.Context(), "outbound webhook: save processed event failed (continuing)", "error", err)
		}
	}

	if _, err := h.callbackUC.Execute(r.Context(), portin.OutboundCallbackInput{
		RequestID:       requestID,
		Provider:        "ASAAS",
		ProviderStatus:  status,
		ProviderEventID: payload.ID,
		Reason:          payload.Event,
	}); err != nil {
		slog.ErrorContext(r.Context(), "outbound webhook: callback processing failed",
			"requestID", requestID, "status", status, "error", err)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
