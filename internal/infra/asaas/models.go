// Package asaas provides an HTTP client and mock for the Asaas PSP API.
// Wire types in this file are used only within the infra layer; higher layers
// interact exclusively through the out.PaymentProvider port.
package asaas

import (
	"fmt"
	"math"
)

// ─── Billing type ────────────────────────────────────────────────────────────

// AsaasBillingType identifies the payment method on the Asaas REST API.
type AsaasBillingType string

const (
	AsaasBillingTypePix        AsaasBillingType = "PIX"
	AsaasBillingTypeBoleto     AsaasBillingType = "BOLETO"
	AsaasBillingTypeCreditCard AsaasBillingType = "CREDIT_CARD"
)

// ─── Payment status ──────────────────────────────────────────────────────────

// AsaasPaymentStatus is the charge lifecycle state as defined by the Asaas API.
// Values mirror the Java PaymentTransaction status enum and all documented
// Asaas variants.
type AsaasPaymentStatus string

const (
	AsaasStatusPending              AsaasPaymentStatus = "PENDING"
	AsaasStatusReceived             AsaasPaymentStatus = "RECEIVED"
	AsaasStatusConfirmed            AsaasPaymentStatus = "CONFIRMED"
	AsaasStatusOverdue              AsaasPaymentStatus = "OVERDUE"
	AsaasStatusRefunded             AsaasPaymentStatus = "REFUNDED"
	AsaasStatusCanceled             AsaasPaymentStatus = "CANCELED"
	AsaasStatusAwaitingRiskAnalysis AsaasPaymentStatus = "AWAITING_RISK_ANALYSIS"
	AsaasStatusRefundRequested      AsaasPaymentStatus = "REFUND_REQUESTED"
	AsaasStatusChargebackRequested  AsaasPaymentStatus = "CHARGEBACK_REQUESTED"
	AsaasStatusChargebackDispute    AsaasPaymentStatus = "CHARGEBACK_DISPUTE"
	AsaasStatusAwaitingChargeback   AsaasPaymentStatus = "AWAITING_CHARGEBACK_REVERSAL"
	AsaasStatusDunningRequested     AsaasPaymentStatus = "DUNNING_REQUESTED"
	AsaasStatusDunningReceived      AsaasPaymentStatus = "DUNNING_RECEIVED"
)

// ─── Customer ────────────────────────────────────────────────────────────────

// AsaasCustomerRequest is the payload for POST /customers.
type AsaasCustomerRequest struct {
	Name              string `json:"name"`
	CpfCnpj           string `json:"cpfCnpj"`
	Email             string `json:"email,omitempty"`
	ExternalReference string `json:"externalReference"`
}

// AsaasCustomer is the customer object returned by the Asaas API.
type AsaasCustomer struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	CpfCnpj           string `json:"cpfCnpj"`
	Email             string `json:"email"`
	ExternalReference string `json:"externalReference"`
}

// AsaasCustomerListResponse is the paginated list envelope for GET /customers.
type AsaasCustomerListResponse struct {
	Data       []AsaasCustomer `json:"data"`
	TotalCount int             `json:"totalCount"`
}

// ─── Payment ─────────────────────────────────────────────────────────────────

// AsaasCreatePaymentRequest is the payload for POST /payments.
// Value is in reais (float64) as required by the Asaas wire format; callers
// must use CentavosToReais to convert from the canonical centavos amount.
type AsaasCreatePaymentRequest struct {
	Customer             string           `json:"customer"`
	BillingType          AsaasBillingType `json:"billingType"`
	Value                float64          `json:"value"` // reais
	DueDate              string           `json:"dueDate"` // YYYY-MM-DD
	ExternalReference    string           `json:"externalReference,omitempty"`
	Description          string           `json:"description,omitempty"`
	CreditCardToken      string           `json:"creditCardToken,omitempty"`
	InstallmentCount     int              `json:"installmentCount,omitempty"`
	InstallmentValue     float64          `json:"installmentValue,omitempty"`
}

// AsaasPayment is the charge object returned by the Asaas API.
type AsaasPayment struct {
	ID                string             `json:"id"`
	Customer          string             `json:"customer"`
	BillingType       AsaasBillingType   `json:"billingType"`
	Value             float64            `json:"value"` // reais
	DueDate           string             `json:"dueDate"`
	Status            AsaasPaymentStatus `json:"status"`
	ExternalReference string             `json:"externalReference"`
	InvoiceUrl        string             `json:"invoiceUrl"`
	BankSlipUrl       string             `json:"bankSlipUrl"`
	CreditCardToken   string             `json:"creditCardToken,omitempty"`
	Description       string             `json:"description,omitempty"`
}

// ─── PIX ─────────────────────────────────────────────────────────────────────

// AsaasPixQrCode is the response from GET /payments/{id}/pixQrCode.
type AsaasPixQrCode struct {
	EncodedImage   string `json:"encodedImage"`
	Payload        string `json:"payload"`
	ExpirationDate string `json:"expirationDate"` // ISO-8601 string
}

// ─── Boleto ──────────────────────────────────────────────────────────────────

// AsaasIdentificationField is the response from GET /payments/{id}/identificationField.
type AsaasIdentificationField struct {
	IdentificationField string `json:"identificationField"`
	NossoNumero         string `json:"nossoNumero"`
	BarCode             string `json:"barCode"`
}

// ─── Credit card tokenisation ────────────────────────────────────────────────

// AsaasCardTokenRequest is the payload for POST /creditCard/tokenizeCreditCard.
// NEVER log this struct — it contains sensitive PAN and CVV.
type AsaasCardTokenRequest struct {
	Customer             string              `json:"customer"`
	CreditCard           AsaasCardDetail     `json:"creditCard"`
	CreditCardHolderInfo AsaasCardHolderInfo `json:"creditCardHolderInfo"`
	RemoteIp             string              `json:"remoteIp,omitempty"`
}

// AsaasCardDetail contains the raw card numbers for tokenisation.
// NEVER log this struct.
type AsaasCardDetail struct {
	HolderName  string `json:"holderName"`
	Number      string `json:"number"`      // full PAN
	ExpiryMonth string `json:"expiryMonth"`
	ExpiryYear  string `json:"expiryYear"`
	Ccv         string `json:"ccv"` // CVV
}

// AsaasCardHolderInfo holds card-holder identification data.
type AsaasCardHolderInfo struct {
	Name        string `json:"name"`
	Email       string `json:"email,omitempty"`
	CpfCnpj     string `json:"cpfCnpj,omitempty"`
	PostalCode  string `json:"postalCode,omitempty"`
	AddressNumber string `json:"addressNumber,omitempty"`
	Phone       string `json:"phone,omitempty"`
	MobilePhone string `json:"mobilePhone,omitempty"`
}

// AsaasCardTokenResponse is the result of a tokenisation request.
type AsaasCardTokenResponse struct {
	CreditCardNumber string `json:"creditCardNumber"` // last 4 digits
	CreditCardBrand  string `json:"creditCardBrand"`
	CreditCardToken  string `json:"creditCardToken"`
}

// ─── Sandbox ─────────────────────────────────────────────────────────────────

// AsaasReceiveInCashRequest is the payload for POST /payments/{id}/receiveInCash.
type AsaasReceiveInCashRequest struct {
	PaymentDate string  `json:"paymentDate"` // YYYY-MM-DD
	Value       float64 `json:"value,omitempty"`
}

// ─── Error types ─────────────────────────────────────────────────────────────

// AsaasErrorResponse is the error envelope returned by the Asaas API on 4xx.
type AsaasErrorResponse struct {
	Errors []AsaasErrorDetail `json:"errors"`
}

// AsaasErrorDetail is a single error entry in the Asaas error response.
type AsaasErrorDetail struct {
	Code        string `json:"code"`
	Description string `json:"description"`
}

// AsaasAPIError is a typed error for Asaas 4xx responses.
// It carries the HTTP status code and the first error entry from the body,
// and is safe to unwrap with errors.As.
type AsaasAPIError struct {
	StatusCode  int
	Code        string
	Description string
}

func (e *AsaasAPIError) Error() string {
	return fmt.Sprintf("asaas API error %d: [%s] %s", e.StatusCode, e.Code, e.Description)
}

// ─── Currency helpers ────────────────────────────────────────────────────────

// CentavosToReais converts an integer centavo amount to the float64 reais value
// required by the Asaas wire format. No rounding is needed since cents are
// always integers.
func CentavosToReais(cents int64) float64 {
	return float64(cents) / 100.0
}

// ReaisToCentavos converts a float64 reais value (as returned by the Asaas API)
// to an integer centavo amount. math.Round eliminates IEEE-754 drift, e.g.
//
//	0.1 + 0.2 = 0.30000000000000004  →  round(30.000000000000004)  =  30
//	19.99                             →  round(1998.9999999999998)  =  1999
func ReaisToCentavos(reais float64) int64 {
	return int64(math.Round(reais * 100))
}
