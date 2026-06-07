// Package out defines the output-port (driven-side) interfaces.
package out

import (
	"context"
	"time"
)

// ─── Billing type ────────────────────────────────────────────────────────────

// PaymentBillingType is the payment method offered to the payer.
type PaymentBillingType string

const (
	BillingTypePix        PaymentBillingType = "PIX"
	BillingTypeBoleto     PaymentBillingType = "BOLETO"
	BillingTypeCreditCard PaymentBillingType = "CREDIT_CARD"
)

// ─── Provider status ─────────────────────────────────────────────────────────

// PaymentProviderStatus is the charge lifecycle state as reported by the provider.
type PaymentProviderStatus string

const (
	ProviderStatusPending              PaymentProviderStatus = "PENDING"
	ProviderStatusReceived             PaymentProviderStatus = "RECEIVED"
	ProviderStatusConfirmed            PaymentProviderStatus = "CONFIRMED"
	ProviderStatusOverdue              PaymentProviderStatus = "OVERDUE"
	ProviderStatusRefunded             PaymentProviderStatus = "REFUNDED"
	ProviderStatusCanceled             PaymentProviderStatus = "CANCELED"
	ProviderStatusAwaitingRiskAnalysis PaymentProviderStatus = "AWAITING_RISK_ANALYSIS"
	ProviderStatusRefundRequested      PaymentProviderStatus = "REFUND_REQUESTED"
	ProviderStatusChargebackRequested  PaymentProviderStatus = "CHARGEBACK_REQUESTED"
	ProviderStatusChargebackDispute    PaymentProviderStatus = "CHARGEBACK_DISPUTE"
	ProviderStatusAwaitingChargeback   PaymentProviderStatus = "AWAITING_CHARGEBACK_REVERSAL"
	ProviderStatusDunningRequested     PaymentProviderStatus = "DUNNING_REQUESTED"
	ProviderStatusDunningReceived      PaymentProviderStatus = "DUNNING_RECEIVED"
)

// IsTerminal returns true when no further status transitions are expected from
// the provider. Terminal statuses are: RECEIVED, CONFIRMED, OVERDUE, REFUNDED,
// CANCELED.
func (s PaymentProviderStatus) IsTerminal() bool {
	switch s {
	case ProviderStatusReceived, ProviderStatusConfirmed,
		ProviderStatusOverdue, ProviderStatusRefunded, ProviderStatusCanceled:
		return true
	}
	return false
}

// ─── Provider DTO types ───────────────────────────────────────────────────────

// ProviderCustomer represents a customer record at the payment provider.
type ProviderCustomer struct {
	ID                string
	Name              string
	ExternalReference string
}

// CreatePaymentRequest holds parameters for creating a charge at the provider.
// ValueCents is the canonical amount in centavos; the adapter converts to the
// wire format (e.g. Asaas uses float64 reais).
type CreatePaymentRequest struct {
	CustomerID        string
	BillingType       PaymentBillingType
	ValueCents        int64
	DueDate           string // YYYY-MM-DD
	ExternalReference string
	Description       string
	// Credit-card specific — leave zero for PIX/Boleto.
	CreditCardToken  string
	InstallmentCount int
}

// ProviderPayment is a charge record as returned or updated by the provider.
type ProviderPayment struct {
	ID                string
	CustomerID        string
	BillingType       PaymentBillingType
	Status            PaymentProviderStatus
	ValueCents        int64
	DueDate           string
	ExternalReference string
	InvoiceUrl        string
	BankSlipUrl       string
}

// ProviderPixQrCode contains QR-code data for a PIX charge.
type ProviderPixQrCode struct {
	EncodedImage   string    // base64-encoded PNG
	Payload        string    // copia-e-cola EMV string
	ExpirationDate time.Time
}

// ProviderIdentificationField contains boleto payment line data.
type ProviderIdentificationField struct {
	IdentificationField string // linha digitável
	NossoNumero         string
	BarCode             string
}

// TokenizeCreditCardRequest holds raw card data for tokenisation.
// NEVER log this struct — it contains sensitive PAN and CVV.
type TokenizeCreditCardRequest struct {
	CustomerID  string
	HolderName  string
	Number      string // full PAN — never persisted
	ExpiryMonth string // MM
	ExpiryYear  string // YYYY
	CVV         string // 3-4 digits — never persisted
}

// ProviderCardToken is the tokenisation result returned by the provider.
type ProviderCardToken struct {
	Token    string // provider vault token — safe to persist
	Brand    string // e.g. VISA, MASTERCARD
	LastFour string
}

// ─── Port interface ───────────────────────────────────────────────────────────

// PaymentProvider is the output port for PSP (Payment Service Provider)
// integration. The canonical live implementation is infra/asaas.Client; for
// local development without Asaas credentials use infra/asaas.MockProvider.
// Use cases depend solely on this interface — never on the concrete client.
type PaymentProvider interface {
	// CreateOrGetCustomer lazily creates a customer record at the provider
	// keyed by userID as the external reference, and returns it.
	// If a customer with that externalReference already exists it is returned
	// without creating a duplicate.
	CreateOrGetCustomer(ctx context.Context, userID, name, cpf, email string) (*ProviderCustomer, error)

	// CreatePayment submits a new charge to the provider.
	CreatePayment(ctx context.Context, req *CreatePaymentRequest) (*ProviderPayment, error)

	// GetPayment retrieves the current state of an existing charge.
	GetPayment(ctx context.Context, providerID string) (*ProviderPayment, error)

	// GetPixQrCode fetches QR-code data for a PIX charge.
	GetPixQrCode(ctx context.Context, providerID string) (*ProviderPixQrCode, error)

	// GetBoletoIdentificationField fetches the digitável line for a Boleto charge.
	GetBoletoIdentificationField(ctx context.Context, providerID string) (*ProviderIdentificationField, error)

	// TokenizeCreditCard stores a card in the provider vault and returns the token.
	TokenizeCreditCard(ctx context.Context, req *TokenizeCreditCardRequest) (*ProviderCardToken, error)

	// SimulateSandboxReceive marks a sandbox charge as received.
	// Implementations MUST return an error when not operating in sandbox mode.
	SimulateSandboxReceive(ctx context.Context, providerID string) error
}
