package asaas

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/role-organizado/backend-go-role-organizado/internal/config"
	"github.com/role-organizado/backend-go-role-organizado/internal/port/out"
)

const (
	clientTimeout = 30 * time.Second
	maxRetries    = 3
)

// retryBackoffs defines the wait durations before each retry attempt.
// retryBackoffs[0] is applied before attempt 1 (first retry),
// retryBackoffs[1] before attempt 2 (second retry).
var retryBackoffs = [2]time.Duration{250 * time.Millisecond, 1 * time.Second}

// ErrNotSandbox is returned by SimulateSandboxReceive when the client is not
// pointing at the Asaas sandbox URL.
var ErrNotSandbox = errors.New("asaas: SimulateSandboxReceive is only available in sandbox mode")

// Client is the HTTP client for the Asaas PSP API.
// It implements out.PaymentProvider; use cases should depend on the port
// interface, never on this concrete type.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new Asaas HTTP client with a fixed 30 s per-request
// timeout (not counting retry back-off waits).
func NewClient(cfg config.AsaasConfig) *Client {
	return &Client{
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		apiKey:  cfg.APIKey,
		httpClient: &http.Client{
			Timeout: clientTimeout,
		},
	}
}

// ─── out.PaymentProvider ─────────────────────────────────────────────────────

// CreateOrGetCustomer returns the Asaas customer for the given userID, creating
// one if it does not yet exist. userID is used as the Asaas externalReference
// so the lookup is idempotent across restarts.
func (c *Client) CreateOrGetCustomer(ctx context.Context, userID, name, cpf, email string) (*out.ProviderCustomer, error) {
	params := url.Values{}
	params.Set("externalReference", userID)

	var listResp AsaasCustomerListResponse
	if err := c.doJSON(ctx, http.MethodGet, "/customers?"+params.Encode(), nil, &listResp); err != nil {
		return nil, fmt.Errorf("asaas: list customers: %w", err)
	}

	if len(listResp.Data) > 0 {
		cust := listResp.Data[0]
		return &out.ProviderCustomer{
			ID:                cust.ID,
			Name:              cust.Name,
			ExternalReference: cust.ExternalReference,
		}, nil
	}

	// Customer not found — create it.
	reqBody := AsaasCustomerRequest{
		Name:              name,
		CpfCnpj:           cpf,
		Email:             email,
		ExternalReference: userID,
	}

	var created AsaasCustomer
	if err := c.doJSON(ctx, http.MethodPost, "/customers", reqBody, &created); err != nil {
		return nil, fmt.Errorf("asaas: create customer: %w", err)
	}

	slog.InfoContext(ctx, "asaas customer created", "asaasCustomerID", created.ID, "externalRef", userID)
	return &out.ProviderCustomer{
		ID:                created.ID,
		Name:              created.Name,
		ExternalReference: created.ExternalReference,
	}, nil
}

// CreatePayment submits a new charge to Asaas.
// req.ValueCents is converted from centavos to reais at the wire boundary.
func (c *Client) CreatePayment(ctx context.Context, req *out.CreatePaymentRequest) (*out.ProviderPayment, error) {
	wire := AsaasCreatePaymentRequest{
		Customer:          req.CustomerID,
		BillingType:       AsaasBillingType(req.BillingType),
		Value:             CentavosToReais(req.ValueCents),
		DueDate:           req.DueDate,
		ExternalReference: req.ExternalReference,
		Description:       req.Description,
		CreditCardToken:   req.CreditCardToken,
		InstallmentCount:  req.InstallmentCount,
	}

	var payment AsaasPayment
	if err := c.doJSON(ctx, http.MethodPost, "/payments", wire, &payment); err != nil {
		return nil, fmt.Errorf("asaas: create payment: %w", err)
	}

	slog.InfoContext(ctx, "asaas payment created",
		"asaasPaymentID", payment.ID,
		"status", payment.Status,
		"billingType", payment.BillingType)
	return toProviderPayment(&payment), nil
}

// GetPayment retrieves the current state of a charge by its Asaas ID.
func (c *Client) GetPayment(ctx context.Context, providerID string) (*out.ProviderPayment, error) {
	var payment AsaasPayment
	if err := c.doJSON(ctx, http.MethodGet, "/payments/"+providerID, nil, &payment); err != nil {
		return nil, fmt.Errorf("asaas: get payment %s: %w", providerID, err)
	}
	return toProviderPayment(&payment), nil
}

// GetPixQrCode fetches QR-code data for a PIX charge.
func (c *Client) GetPixQrCode(ctx context.Context, providerID string) (*out.ProviderPixQrCode, error) {
	var qr AsaasPixQrCode
	if err := c.doJSON(ctx, http.MethodGet, "/payments/"+providerID+"/pixQrCode", nil, &qr); err != nil {
		return nil, fmt.Errorf("asaas: get pix qr code %s: %w", providerID, err)
	}

	expTime, _ := time.Parse(time.RFC3339, qr.ExpirationDate)

	return &out.ProviderPixQrCode{
		EncodedImage:   qr.EncodedImage,
		Payload:        qr.Payload,
		ExpirationDate: expTime,
	}, nil
}

// GetBoletoIdentificationField fetches the digitável line for a Boleto charge.
func (c *Client) GetBoletoIdentificationField(ctx context.Context, providerID string) (*out.ProviderIdentificationField, error) {
	var field AsaasIdentificationField
	if err := c.doJSON(ctx, http.MethodGet, "/payments/"+providerID+"/identificationField", nil, &field); err != nil {
		return nil, fmt.Errorf("asaas: get boleto identification field %s: %w", providerID, err)
	}
	return &out.ProviderIdentificationField{
		IdentificationField: field.IdentificationField,
		NossoNumero:         field.NossoNumero,
		BarCode:             field.BarCode,
	}, nil
}

// TokenizeCreditCard stores a card in the Asaas vault and returns the token.
// The request struct is never logged to avoid leaking PAN or CVV.
func (c *Client) TokenizeCreditCard(ctx context.Context, req *out.TokenizeCreditCardRequest) (*out.ProviderCardToken, error) {
	wire := AsaasCardTokenRequest{
		Customer: req.CustomerID,
		CreditCard: AsaasCardDetail{
			HolderName:  req.HolderName,
			Number:      req.Number,
			ExpiryMonth: req.ExpiryMonth,
			ExpiryYear:  req.ExpiryYear,
			Ccv:         req.CVV,
		},
		CreditCardHolderInfo: AsaasCardHolderInfo{
			Name: req.HolderName,
		},
	}

	var tokenResp AsaasCardTokenResponse
	// Use sensitive variant — body is never logged.
	if err := c.doJSONSensitive(ctx, http.MethodPost, "/creditCard/tokenizeCreditCard", wire, &tokenResp); err != nil {
		return nil, fmt.Errorf("asaas: tokenize credit card: %w", err)
	}

	return &out.ProviderCardToken{
		Token:    tokenResp.CreditCardToken,
		Brand:    tokenResp.CreditCardBrand,
		LastFour: tokenResp.CreditCardNumber,
	}, nil
}

// SimulateSandboxReceive marks a sandbox charge as received via receiveInCash.
// Returns ErrNotSandbox when the base URL does not contain "sandbox".
func (c *Client) SimulateSandboxReceive(ctx context.Context, providerID string) error {
	if !strings.Contains(c.baseURL, "sandbox") {
		return ErrNotSandbox
	}

	reqBody := AsaasReceiveInCashRequest{
		PaymentDate: time.Now().Format("2006-01-02"),
	}

	if err := c.doJSON(ctx, http.MethodPost, "/payments/"+providerID+"/receiveInCash", reqBody, nil); err != nil {
		return fmt.Errorf("asaas: simulate sandbox receive %s: %w", providerID, err)
	}

	slog.InfoContext(ctx, "asaas sandbox receive simulated", "providerID", providerID)
	return nil
}

// ─── Internal helpers ─────────────────────────────────────────────────────────

// doJSON marshals body (nil for GET), executes the request with retry,
// and unmarshals the successful response into result (nil to discard).
func (c *Client) doJSON(ctx context.Context, method, path string, body, result any) error {
	return c.doJSONInternal(ctx, method, path, body, result, false)
}

// doJSONSensitive is like doJSON but marks the request as sensitive so
// the body is never included in log output (use for card tokenisation).
func (c *Client) doJSONSensitive(ctx context.Context, method, path string, body, result any) error {
	return c.doJSONInternal(ctx, method, path, body, result, true)
}

func (c *Client) doJSONInternal(ctx context.Context, method, path string, body, result any, sensitive bool) error {
	var bodyReader io.ReadSeeker
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("asaas: marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	resp, err := c.doWithRetry(ctx, method, path, bodyReader)
	if err != nil {
		if sensitive {
			// Do not log body — it may contain card numbers or CVV.
			slog.ErrorContext(ctx, "asaas sensitive request failed", "method", method, "path", path, "error", err)
		}
		return err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("asaas: read response: %w", err)
	}

	// 4xx → parse Asaas error body and return typed error.
	if resp.StatusCode >= 400 {
		var errResp AsaasErrorResponse
		_ = json.Unmarshal(respBytes, &errResp)
		apiErr := &AsaasAPIError{StatusCode: resp.StatusCode}
		if len(errResp.Errors) > 0 {
			apiErr.Code = errResp.Errors[0].Code
			apiErr.Description = errResp.Errors[0].Description
		}
		return apiErr
	}

	if result != nil && len(respBytes) > 0 {
		if err := json.Unmarshal(respBytes, result); err != nil {
			return fmt.Errorf("asaas: unmarshal response: %w", err)
		}
	}
	return nil
}

// doWithRetry executes an HTTP request with up to maxRetries attempts.
//
//   - Network errors and 5xx / 429 responses → retry with exponential back-off.
//   - 4xx responses → return immediately (no retry).
//   - Context cancellation → return immediately at any point.
//
// The body must be an io.ReadSeeker so it can be rewound between attempts.
// A nil body is fine for methods that have no request body (GET, DELETE).
func (c *Client) doWithRetry(ctx context.Context, method, path string, body io.ReadSeeker) (*http.Response, error) {
	fullURL := c.baseURL + path
	var lastErr error

	for attempt := range maxRetries {
		// Before each retry (not the first attempt): rewind body and sleep.
		if attempt > 0 {
			if body != nil {
				if _, seekErr := body.Seek(0, io.SeekStart); seekErr != nil {
					return nil, fmt.Errorf("asaas: seek body for retry: %w", seekErr)
				}
			}

			idx := attempt - 1
			if idx >= len(retryBackoffs) {
				idx = len(retryBackoffs) - 1
			}
			delay := retryBackoffs[idx]

			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("asaas: context cancelled during retry wait: %w", ctx.Err())
			case <-time.After(delay):
			}
		}

		req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
		if err != nil {
			return nil, fmt.Errorf("asaas: build request: %w", err)
		}
		req.Header.Set("access_token", c.apiKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, fmt.Errorf("asaas: request cancelled: %w", ctx.Err())
			}
			lastErr = fmt.Errorf("asaas: http do: %w", err)
			slog.WarnContext(ctx, "asaas request network error, retrying",
				"attempt", attempt+1, "maxAttempts", maxRetries,
				"method", method, "path", path, "error", lastErr)
			continue
		}

		if retryableStatus(resp.StatusCode) {
			resp.Body.Close()
			lastErr = fmt.Errorf("asaas: retryable server error %d", resp.StatusCode)
			slog.WarnContext(ctx, "asaas server error, retrying",
				"attempt", attempt+1, "maxAttempts", maxRetries,
				"method", method, "path", path, "status", resp.StatusCode)
			continue
		}

		// Success (2xx/3xx) or non-retryable client error (4xx) — return as-is.
		return resp, nil
	}

	return nil, fmt.Errorf("asaas: all %d attempts failed for %s %s: %w", maxRetries, method, path, lastErr)
}

// retryableStatus returns true for status codes that should trigger a retry.
func retryableStatus(code int) bool {
	return code == http.StatusTooManyRequests || (code >= 500 && code <= 599)
}

// toProviderPayment maps an Asaas wire payment to the port ProviderPayment type,
// converting the wire float64 reais value to int64 centavos at the boundary.
func toProviderPayment(p *AsaasPayment) *out.ProviderPayment {
	return &out.ProviderPayment{
		ID:                p.ID,
		CustomerID:        p.Customer,
		BillingType:       out.PaymentBillingType(p.BillingType),
		Status:            out.PaymentProviderStatus(p.Status),
		ValueCents:        ReaisToCentavos(p.Value),
		DueDate:           p.DueDate,
		ExternalReference: p.ExternalReference,
		InvoiceUrl:        p.InvoiceUrl,
		BankSlipUrl:       p.BankSlipUrl,
	}
}
