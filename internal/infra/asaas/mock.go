package asaas

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/role-organizado/backend-go-role-organizado/internal/port/out"
)

// mockIDCounter is shared across all MockProvider instances; each call to
// nextMockID increments it atomically for globally unique IDs within a process.
var mockIDCounter atomic.Int64

// MockProvider is a fake out.PaymentProvider for local development and testing.
// It never calls the real Asaas API and requires no credentials.
// Enable by setting ROLE_ASAAS_USE_MOCK=true (default: true).
//
// Mirrors the Java MockPaymentProvider behaviour: all payments are created
// with status PENDING; SimulateSandboxReceive is a no-op.
type MockProvider struct{}

// NewMockProvider returns a ready-to-use MockProvider.
func NewMockProvider() *MockProvider {
	return &MockProvider{}
}

// CreateOrGetCustomer returns a deterministic fake customer record keyed to
// userID. No network call is made.
func (m *MockProvider) CreateOrGetCustomer(_ context.Context, userID, name, _, _ string) (*out.ProviderCustomer, error) {
	suffix := userID
	if len(suffix) > 8 {
		suffix = suffix[:8]
	}
	return &out.ProviderCustomer{
		ID:                "cus_mock_" + suffix,
		Name:              name,
		ExternalReference: userID,
	}, nil
}

// CreatePayment returns a fake PENDING charge. The returned ID has the form
// pay_mock_000001 to make mock payments easy to identify in logs.
func (m *MockProvider) CreatePayment(_ context.Context, req *out.CreatePaymentRequest) (*out.ProviderPayment, error) {
	id := nextMockID("pay")
	return &out.ProviderPayment{
		ID:                id,
		CustomerID:        req.CustomerID,
		BillingType:       req.BillingType,
		Status:            out.ProviderStatusPending,
		ValueCents:        req.ValueCents,
		DueDate:           req.DueDate,
		ExternalReference: req.ExternalReference,
		InvoiceUrl:        "https://mock.asaas.com/invoices/" + id,
		BankSlipUrl:       "",
	}, nil
}

// GetPayment returns a fake PENDING charge with the given providerID.
func (m *MockProvider) GetPayment(_ context.Context, providerID string) (*out.ProviderPayment, error) {
	return &out.ProviderPayment{
		ID:     providerID,
		Status: out.ProviderStatusPending,
	}, nil
}

// GetPixQrCode returns a fake QR-code valid for 30 minutes.
func (m *MockProvider) GetPixQrCode(_ context.Context, providerID string) (*out.ProviderPixQrCode, error) {
	// Minimal valid 1×1 PNG, base64-encoded — safe to render without decoding.
	const fakeImg = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="
	return &out.ProviderPixQrCode{
		EncodedImage:   fakeImg,
		Payload:        "mock_pix_payload_" + providerID,
		ExpirationDate: time.Now().Add(30 * time.Minute),
	}, nil
}

// GetBoletoIdentificationField returns a plausible (but fake) boleto digitável line.
func (m *MockProvider) GetBoletoIdentificationField(_ context.Context, providerID string) (*out.ProviderIdentificationField, error) {
	return &out.ProviderIdentificationField{
		IdentificationField: "23790.00100 01234.567890 12345.678901 1 97010000010000",
		NossoNumero:         "mock-" + providerID,
		BarCode:             "23791970100000100002379000100012345678901234567890",
	}, nil
}

// TokenizeCreditCard returns a fake card token without touching any network.
// The last 4 digits of the provided card number are echoed back.
func (m *MockProvider) TokenizeCreditCard(_ context.Context, req *out.TokenizeCreditCardRequest) (*out.ProviderCardToken, error) {
	last4 := "0000"
	if len(req.Number) >= 4 {
		last4 = req.Number[len(req.Number)-4:]
	}
	return &out.ProviderCardToken{
		Token:    nextMockID("tok"),
		Brand:    "VISA",
		LastFour: last4,
	}, nil
}

// SimulateSandboxReceive is a no-op for the mock provider.
func (m *MockProvider) SimulateSandboxReceive(_ context.Context, _ string) error {
	return nil
}

// nextMockID generates a monotonically increasing fake provider ID with the
// given prefix, e.g. "pay_mock_000001".
func nextMockID(prefix string) string {
	n := mockIDCounter.Add(1)
	return fmt.Sprintf("%s_mock_%06d", prefix, n)
}
