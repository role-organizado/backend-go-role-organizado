// Package asaas_test contains black-box tests for the Asaas HTTP client.
// Integration tests use httptest.Server so no real network calls are made.
package asaas_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/role-organizado/backend-go-role-organizado/internal/config"
	"github.com/role-organizado/backend-go-role-organizado/internal/infra/asaas"
	"github.com/role-organizado/backend-go-role-organizado/internal/port/out"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

// newTestClient creates a Client pointing at the given test server URL.
func newTestClient(serverURL string) *asaas.Client {
	return asaas.NewClient(config.AsaasConfig{
		BaseURL: serverURL,
		APIKey:  "test-api-key",
	})
}

// paymentJSON returns a minimal valid Asaas payment JSON string.
func paymentJSON(id string) string {
	return fmt.Sprintf(`{"id":%q,"status":"PENDING","billingType":"PIX","value":10.0,"dueDate":"2026-06-10","customer":"cus_123"}`, id)
}

// ─── Auth header ─────────────────────────────────────────────────────────────

func TestClient_AuthHeaderPresentOnEveryRequest(t *testing.T) {
	tests := []struct {
		name   string
		invoke func(c *asaas.Client, ts *httptest.Server) error
		reply  string
	}{
		{
			name:  "GetPayment",
			reply: paymentJSON("pay_1"),
			invoke: func(c *asaas.Client, ts *httptest.Server) error {
				_, err := c.GetPayment(context.Background(), "pay_1")
				return err
			},
		},
		{
			name:  "GetPixQrCode",
			reply: `{"encodedImage":"abc","payload":"00020126","expirationDate":"2026-06-10T10:00:00Z"}`,
			invoke: func(c *asaas.Client, ts *httptest.Server) error {
				_, err := c.GetPixQrCode(context.Background(), "pay_1")
				return err
			},
		},
		{
			name:  "GetBoletoIdentificationField",
			reply: `{"identificationField":"12345","nossoNumero":"001","barCode":"1111"}`,
			invoke: func(c *asaas.Client, ts *httptest.Server) error {
				_, err := c.GetBoletoIdentificationField(context.Background(), "pay_1")
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotKey string
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotKey = r.Header.Get("access_token")
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, tt.reply)
			}))
			defer ts.Close()

			c := asaas.NewClient(config.AsaasConfig{BaseURL: ts.URL, APIKey: "my-secret-key"})
			_ = tt.invoke(c, ts)
			assert.Equal(t, "my-secret-key", gotKey, "access_token header must be sent")
		})
	}
}

// ─── Retry on 5xx ─────────────────────────────────────────────────────────────

// TestClient_RetryOn503 verifies that the client retries twice on 503 and
// succeeds on the third attempt. Total delay ~1.25 s (250 ms + 1 s back-offs).
func TestClient_RetryOn503(t *testing.T) {
	var attempts int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, paymentJSON("pay_retry"))
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	pay, err := c.GetPayment(context.Background(), "pay_retry")

	require.NoError(t, err, "should succeed on third attempt")
	assert.Equal(t, int32(3), atomic.LoadInt32(&attempts), "exactly 3 HTTP calls expected")
	assert.Equal(t, "pay_retry", pay.ID)
	assert.Equal(t, out.ProviderStatusPending, pay.Status)
}

// TestClient_RetryOn429 verifies that 429 Too Many Requests triggers retry.
func TestClient_RetryOn429(t *testing.T) {
	var attempts int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n < 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, paymentJSON("pay_429"))
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.GetPayment(context.Background(), "pay_429")

	require.NoError(t, err)
	assert.Equal(t, int32(2), atomic.LoadInt32(&attempts))
}

// TestClient_ExhaustsRetriesAndReturnsError verifies that after maxRetries
// consecutive failures the client returns an error.
func TestClient_ExhaustsRetriesAndReturnsError(t *testing.T) {
	var attempts int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.GetPayment(context.Background(), "pay_x")

	require.Error(t, err)
	assert.Equal(t, int32(3), atomic.LoadInt32(&attempts), "should attempt exactly maxRetries=3 times")
}

// ─── No retry on 4xx ─────────────────────────────────────────────────────────

func TestClient_NoRetryOn400(t *testing.T) {
	var attempts int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"errors":[{"code":"INVALID_ACTION","description":"Payment not found"}]}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.GetPayment(context.Background(), "pay_bad")

	require.Error(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&attempts), "4xx must not trigger a retry — only 1 attempt expected")
}

func TestClient_NoRetryOn404(t *testing.T) {
	var attempts int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"errors":[{"code":"NOT_FOUND","description":"not found"}]}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.GetPayment(context.Background(), "pay_404")

	require.Error(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&attempts), "404 must not trigger retry")
}

// ─── AsaasAPIError parsing ────────────────────────────────────────────────────

func TestClient_ParsesAsaasAPIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"errors":[{"code":"INVALID_BILLING_TYPE","description":"billing type is required"}]}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.GetPayment(context.Background(), "pay_err")

	require.Error(t, err)
	var apiErr *asaas.AsaasAPIError
	require.True(t, errors.As(err, &apiErr), "error must be (or wrap) *AsaasAPIError, got: %T — %v", err, err)
	assert.Equal(t, 400, apiErr.StatusCode)
	assert.Equal(t, "INVALID_BILLING_TYPE", apiErr.Code)
	assert.Equal(t, "billing type is required", apiErr.Description)
}

func TestClient_ParsesAsaasAPIError_EmptyBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		// No body
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.GetPayment(context.Background(), "pay_err2")

	require.Error(t, err)
	var apiErr *asaas.AsaasAPIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, http.StatusUnprocessableEntity, apiErr.StatusCode)
	assert.Empty(t, apiErr.Code, "empty body should produce empty code")
}

// ─── SimulateSandboxReceive ───────────────────────────────────────────────────

func TestClient_SimulateSandboxReceive_BlockedOutsideSandbox(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
	}{
		{"production URL", "https://api.asaas.com/api/v3"},
		{"staging URL without sandbox keyword", "https://api-staging.asaas.com/api/v3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := asaas.NewClient(config.AsaasConfig{BaseURL: tt.baseURL, APIKey: "key"})
			err := c.SimulateSandboxReceive(context.Background(), "pay_1")
			require.Error(t, err)
			assert.ErrorIs(t, err, asaas.ErrNotSandbox, "must return ErrNotSandbox for non-sandbox URL")
		})
	}
}

func TestClient_SimulateSandboxReceive_AllowedInSandbox(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "receiveInCash")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, paymentJSON("pay_sim"))
	}))
	defer ts.Close()

	// URL contains "sandbox" to satisfy the guard.
	sandboxURL := ts.URL + "/sandbox"
	c := asaas.NewClient(config.AsaasConfig{BaseURL: sandboxURL, APIKey: "key"})
	err := c.SimulateSandboxReceive(context.Background(), "pay_sim")
	require.NoError(t, err)
}

// ─── Currency conversion ──────────────────────────────────────────────────────

func TestReaisToCentavos(t *testing.T) {
	tests := []struct {
		name  string
		reais float64
		want  int64
	}{
		{"zero", 0, 0},
		{"whole reais", 10.0, 1000},
		{"with cents", 10.99, 1099},
		{"small value", 0.01, 1},
		{"large value", 1999.99, 199999},
		{"250 reais", 250.00, 25000},
		// Floating-point classic: 0.1 + 0.2 = 0.30000000000000004 in IEEE-754.
		// math.Round ensures we get 30, not 29 or 31.
		{"0.1+0.2 float precision", 0.1 + 0.2, 30},
		{"19.99 float precision", 19.99, 1999},
		{"9.90 float precision", 9.90, 990},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := asaas.ReaisToCentavos(tt.reais)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCentavosToReais(t *testing.T) {
	tests := []struct {
		name  string
		cents int64
		want  float64
	}{
		{"zero", 0, 0.0},
		{"100 cents = 1 real", 100, 1.0},
		{"1099 cents = 10.99 reais", 1099, 10.99},
		{"1 cent", 1, 0.01},
		{"25000 cents = 250 reais", 25000, 250.0},
		{"199999 cents = 1999.99 reais", 199999, 1999.99},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := asaas.CentavosToReais(tt.cents)
			assert.InDelta(t, tt.want, got, 1e-9, "CentavosToReais(%d)", tt.cents)
		})
	}
}

// TestCurrencyRoundTrip verifies that cents → reais → cents is lossless for
// typical payment amounts.
func TestCurrencyRoundTrip(t *testing.T) {
	cases := []int64{1, 5, 10, 99, 100, 999, 1000, 1099, 9990, 25000, 199999, 1000000}
	for _, cents := range cases {
		t.Run(fmt.Sprintf("%d cents", cents), func(t *testing.T) {
			reais := asaas.CentavosToReais(cents)
			roundtripped := asaas.ReaisToCentavos(reais)
			assert.Equal(t, cents, roundtripped, "round-trip cents→reais→cents must be lossless")
		})
	}
}

// ─── CreateOrGetCustomer ─────────────────────────────────────────────────────

func TestClient_CreateOrGetCustomer_CreatesWhenNotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			// Empty list → no existing customer.
			fmt.Fprint(w, `{"data":[],"totalCount":0}`)
			return
		}
		// POST → return created customer.
		fmt.Fprint(w, `{"id":"cus_new","name":"João","cpfCnpj":"12345678900","email":"joao@test.com","externalReference":"user-abc"}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	cust, err := c.CreateOrGetCustomer(context.Background(), "user-abc", "João", "12345678900", "joao@test.com")
	require.NoError(t, err)
	assert.Equal(t, "cus_new", cust.ID)
	assert.Equal(t, "user-abc", cust.ExternalReference)
}

func TestClient_CreateOrGetCustomer_ReturnsExistingCustomer(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// List returns one existing customer — POST should not be called.
		if r.Method == http.MethodPost {
			t.Error("POST /customers must not be called when customer already exists")
		}
		fmt.Fprint(w, `{"data":[{"id":"cus_existing","name":"Maria","cpfCnpj":"98765432100","externalReference":"user-xyz"}],"totalCount":1}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	cust, err := c.CreateOrGetCustomer(context.Background(), "user-xyz", "Maria", "98765432100", "")
	require.NoError(t, err)
	assert.Equal(t, "cus_existing", cust.ID)
}

// ─── CreatePayment ────────────────────────────────────────────────────────────

// TestClient_CreatePayment_EndToEnd verifies the full CreatePayment flow.
func TestClient_CreatePayment_EndToEnd(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/payments", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"pay_e2e","status":"PENDING","billingType":"PIX","value":25.00,"customer":"cus_1","externalReference":"ext-ref-1"}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	pay, err := c.CreatePayment(context.Background(), &out.CreatePaymentRequest{
		CustomerID:        "cus_1",
		BillingType:       out.BillingTypePix,
		ValueCents:        2500,
		DueDate:           "2026-06-10",
		ExternalReference: "ext-ref-1",
	})

	require.NoError(t, err)
	assert.Equal(t, "pay_e2e", pay.ID)
	assert.Equal(t, out.ProviderStatusPending, pay.Status)
	assert.Equal(t, int64(2500), pay.ValueCents, "25.00 reais must be converted to 2500 centavos")
	assert.Equal(t, "ext-ref-1", pay.ExternalReference)
}

// ─── MockProvider ────────────────────────────────────────────────────────────

func TestMockProvider_CreatePayment_ReturnsPendingStatus(t *testing.T) {
	mock := asaas.NewMockProvider()
	pay, err := mock.CreatePayment(context.Background(), &out.CreatePaymentRequest{
		CustomerID:  "cus_mock",
		BillingType: out.BillingTypePix,
		ValueCents:  5000,
		DueDate:     "2026-06-15",
	})
	require.NoError(t, err)
	assert.Equal(t, out.ProviderStatusPending, pay.Status)
	assert.Contains(t, pay.ID, "mock", "mock payment ID should contain 'mock'")
	assert.Equal(t, int64(5000), pay.ValueCents)
}

func TestMockProvider_IDs_AreUnique(t *testing.T) {
	mock := asaas.NewMockProvider()
	req := &out.CreatePaymentRequest{BillingType: out.BillingTypePix, ValueCents: 100}
	ids := make(map[string]bool, 50)
	for range 50 {
		pay, err := mock.CreatePayment(context.Background(), req)
		require.NoError(t, err)
		assert.False(t, ids[pay.ID], "duplicate ID: %s", pay.ID)
		ids[pay.ID] = true
	}
}

func TestMockProvider_SimulateSandboxReceive_IsNoOp(t *testing.T) {
	mock := asaas.NewMockProvider()
	err := mock.SimulateSandboxReceive(context.Background(), "pay_any")
	assert.NoError(t, err, "MockProvider.SimulateSandboxReceive should always succeed")
}

func TestMockProvider_ImplementsPaymentProvider(t *testing.T) {
	// Compile-time interface check via assignment.
	var _ out.PaymentProvider = asaas.NewMockProvider()
}

func TestClient_ImplementsPaymentProvider(t *testing.T) {
	// Compile-time interface check via assignment.
	var _ out.PaymentProvider = asaas.NewClient(config.AsaasConfig{})
}
