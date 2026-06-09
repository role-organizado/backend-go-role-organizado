package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/handler"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
)

// ─── Mock: HandlePaymentCallbackUseCase ──────────────────────────────────────

type mockHandleCallbackUC struct {
	executeFn func(ctx context.Context, in portin.PaymentCallbackPayload) error
}

func (m *mockHandleCallbackUC) Execute(ctx context.Context, in portin.PaymentCallbackPayload) error {
	if m.executeFn != nil {
		return m.executeFn(ctx, in)
	}
	return nil
}

var _ portin.HandlePaymentCallbackUseCase = (*mockHandleCallbackUC)(nil)

// ─── Helpers ─────────────────────────────────────────────────────────────────

// buildAsaasBody builds a typical Asaas webhook JSON payload.
func buildAsaasBody(id, event, paymentID, paymentStatus string) []byte {
	body := map[string]interface{}{
		"id":    id,
		"event": event,
		"payment": map[string]string{
			"id":     paymentID,
			"status": paymentStatus,
		},
	}
	data, _ := json.Marshal(body)
	return data
}

// doWebhookRequest sends a POST to the handler and returns the response.
func doWebhookRequest(t *testing.T, h *handler.PaymentWebhookHandler, body []byte, token string) *httptest.ResponseRecorder {
	t.Helper()
	r := chi.NewRouter()
	h.RegisterWebhookRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/payment/asaas", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("asaas-access-token", token)
	}

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	return rr
}

// ─── Tests ────────────────────────────────────────────────────────────────────

// TestWebhookHandler_InvalidToken_Returns401 verifies that a request with a
// mismatched asaas-access-token header receives 401 Unauthorized.
func TestWebhookHandler_InvalidToken_Returns401(t *testing.T) {
	uc := &mockHandleCallbackUC{}
	h := handler.NewPaymentWebhookHandler(uc, "secret-token")

	body := buildAsaasBody("evt-1", "PAYMENT_RECEIVED", "pay-1", "RECEIVED")
	rr := doWebhookRequest(t, h, body, "wrong-token")

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// TestWebhookHandler_ValidToken_Returns200 verifies that a request with the
// correct asaas-access-token returns 200 OK.
func TestWebhookHandler_ValidToken_Returns200(t *testing.T) {
	called := false
	uc := &mockHandleCallbackUC{executeFn: func(_ context.Context, in portin.PaymentCallbackPayload) error {
		called = true
		return nil
	}}
	h := handler.NewPaymentWebhookHandler(uc, "secret-token")

	body := buildAsaasBody("evt-2", "PAYMENT_RECEIVED", "pay-2", "RECEIVED")
	rr := doWebhookRequest(t, h, body, "secret-token")

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.True(t, called, "use case must have been called")
}

// TestWebhookHandler_EmptyToken_DevMode_Returns200 verifies that when the server
// is configured without a webhook token (dev mode), any request is accepted.
func TestWebhookHandler_EmptyToken_DevMode_Returns200(t *testing.T) {
	uc := &mockHandleCallbackUC{}
	h := handler.NewPaymentWebhookHandler(uc, "") // empty token → dev mode

	body := buildAsaasBody("evt-3", "PAYMENT_RECEIVED", "pay-3", "RECEIVED")
	// No token header sent — should still return 200 in dev mode.
	rr := doWebhookRequest(t, h, body, "")

	assert.Equal(t, http.StatusOK, rr.Code)
}

// TestWebhookHandler_MalformedBody_Returns400 verifies that invalid JSON returns
// 400 Bad Request.
func TestWebhookHandler_MalformedBody_Returns400(t *testing.T) {
	uc := &mockHandleCallbackUC{}
	h := handler.NewPaymentWebhookHandler(uc, "")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/payment/asaas",
		bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	r := chi.NewRouter()
	h.RegisterWebhookRoutes(r)
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestWebhookHandler_MissingID_Returns400 verifies that a payload without the
// required 'id' field returns 400 Bad Request.
func TestWebhookHandler_MissingID_Returns400(t *testing.T) {
	uc := &mockHandleCallbackUC{}
	h := handler.NewPaymentWebhookHandler(uc, "")

	body, _ := json.Marshal(map[string]string{"event": "PAYMENT_RECEIVED"})
	rr := doWebhookRequest(t, h, body, "")

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestWebhookHandler_ProcessingError_StillReturns200 verifies that even when
// the use case returns an error, the handler returns 200 (prevents Asaas retries).
func TestWebhookHandler_ProcessingError_StillReturns200(t *testing.T) {
	uc := &mockHandleCallbackUC{executeFn: func(_ context.Context, _ portin.PaymentCallbackPayload) error {
		return errors.New("database connection lost")
	}}
	h := handler.NewPaymentWebhookHandler(uc, "")

	body := buildAsaasBody("evt-4", "PAYMENT_RECEIVED", "pay-4", "RECEIVED")
	rr := doWebhookRequest(t, h, body, "")

	// Processing errors must NOT propagate as HTTP errors — always 200.
	assert.Equal(t, http.StatusOK, rr.Code)
}

// TestWebhookHandler_ProviderTransactionID_Fallback verifies the three-level
// fallback chain for providerTransactionId extraction:
//   1. payment.id
//   2. payment.externalReference
//   3. top-level providerTransactionId
func TestWebhookHandler_ProviderTransactionID_Fallback(t *testing.T) {
	t.Run("from_payment_id", func(t *testing.T) {
		var captured portin.PaymentCallbackPayload
		uc := &mockHandleCallbackUC{executeFn: func(_ context.Context, in portin.PaymentCallbackPayload) error {
			captured = in
			return nil
		}}
		h := handler.NewPaymentWebhookHandler(uc, "")

		body := buildAsaasBody("evt-fallback-1", "PAYMENT_RECEIVED", "asaas-pay-id", "RECEIVED")
		doWebhookRequest(t, h, body, "")

		assert.Equal(t, "asaas-pay-id", captured.ProviderTransactionID)
	})

	t.Run("from_payment_externalReference", func(t *testing.T) {
		var captured portin.PaymentCallbackPayload
		uc := &mockHandleCallbackUC{executeFn: func(_ context.Context, in portin.PaymentCallbackPayload) error {
			captured = in
			return nil
		}}
		h := handler.NewPaymentWebhookHandler(uc, "")

		// payment.id is empty; externalReference should be used.
		body, _ := json.Marshal(map[string]interface{}{
			"id":    "evt-fallback-2",
			"event": "PAYMENT_RECEIVED",
			"payment": map[string]string{
				"id":                "",
				"externalReference": "ext-ref-123",
				"status":            "RECEIVED",
			},
		})
		doWebhookRequest(t, h, body, "")

		assert.Equal(t, "ext-ref-123", captured.ProviderTransactionID)
	})

	t.Run("from_top_level_providerTransactionId", func(t *testing.T) {
		var captured portin.PaymentCallbackPayload
		uc := &mockHandleCallbackUC{executeFn: func(_ context.Context, in portin.PaymentCallbackPayload) error {
			captured = in
			return nil
		}}
		h := handler.NewPaymentWebhookHandler(uc, "")

		// No payment object — fall back to top-level providerTransactionId.
		body, _ := json.Marshal(map[string]interface{}{
			"id":                    "evt-fallback-3",
			"event":                 "PAYMENT_RECEIVED",
			"status":                "RECEIVED",
			"providerTransactionId": "legacy-pay-456",
		})
		doWebhookRequest(t, h, body, "")

		assert.Equal(t, "legacy-pay-456", captured.ProviderTransactionID)
	})
}

// TestWebhookHandler_StatusExtraction verifies that status is extracted from
// payment.status first, falling back to top-level status.
func TestWebhookHandler_StatusExtraction(t *testing.T) {
	t.Run("from_payment_status", func(t *testing.T) {
		var captured portin.PaymentCallbackPayload
		uc := &mockHandleCallbackUC{executeFn: func(_ context.Context, in portin.PaymentCallbackPayload) error {
			captured = in
			return nil
		}}
		h := handler.NewPaymentWebhookHandler(uc, "")

		body := buildAsaasBody("evt-status-1", "PAYMENT_RECEIVED", "pay-s1", "CONFIRMED")
		doWebhookRequest(t, h, body, "")

		assert.Equal(t, "CONFIRMED", captured.NewStatus)
	})

	t.Run("from_top_level_status", func(t *testing.T) {
		var captured portin.PaymentCallbackPayload
		uc := &mockHandleCallbackUC{executeFn: func(_ context.Context, in portin.PaymentCallbackPayload) error {
			captured = in
			return nil
		}}
		h := handler.NewPaymentWebhookHandler(uc, "")

		// No payment object — use top-level status.
		body, _ := json.Marshal(map[string]interface{}{
			"id":                    "evt-status-2",
			"event":                 "PAYMENT_OVERDUE",
			"status":                "OVERDUE",
			"providerTransactionId": "pay-s2",
		})
		doWebhookRequest(t, h, body, "")

		assert.Equal(t, "OVERDUE", captured.NewStatus)
	})
}

// TestWebhookHandler_IdempotencyViaHTTP_SameEventID_TwoRequests tests that
// sending the same event ID twice returns 200 both times. The actual idempotency
// de-duplication happens inside the use case (tested separately with Testcontainers),
// but the HTTP contract (always 200) must hold.
func TestWebhookHandler_IdempotencyViaHTTP_SameEventID_TwoRequests(t *testing.T) {
	callCount := 0
	uc := &mockHandleCallbackUC{executeFn: func(_ context.Context, in portin.PaymentCallbackPayload) error {
		callCount++
		return nil
	}}
	h := handler.NewPaymentWebhookHandler(uc, "")

	body := buildAsaasBody("evt-dup-http", "PAYMENT_RECEIVED", "pay-dup", "RECEIVED")

	rr1 := doWebhookRequest(t, h, body, "")
	rr2 := doWebhookRequest(t, h, body, "")

	require.Equal(t, http.StatusOK, rr1.Code)
	require.Equal(t, http.StatusOK, rr2.Code)
	assert.Equal(t, 2, callCount, "use case must be called once per HTTP request (idempotency is inside the UC)")
}
