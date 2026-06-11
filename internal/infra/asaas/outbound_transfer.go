package asaas

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/role-organizado/backend-go-role-organizado/internal/config"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
)

// OutboundTransferClient implements portout.OutboundTransferProvider against the
// Asaas /transfers endpoint with PIX. It reuses the base HTTP retry/auth pipeline
// of Client.
type OutboundTransferClient struct {
	client *Client
	apiKey string
}

// NewOutboundTransferClient wires a new outbound-transfer provider on top of the
// existing Asaas Client.
func NewOutboundTransferClient(cfg config.AsaasConfig) *OutboundTransferClient {
	return &OutboundTransferClient{
		client: NewClient(cfg),
		apiKey: cfg.APIKey,
	}
}

// Compile-time interface assertion.
var _ portout.OutboundTransferProvider = (*OutboundTransferClient)(nil)

// IsEnabled returns true when the Asaas API key is configured.
func (o *OutboundTransferClient) IsEnabled() bool {
	return o.apiKey != ""
}

// asaasTransferRequest mirrors the Asaas POST /transfers body for PIX.
type asaasTransferRequest struct {
	Value             float64 `json:"value"` // reais
	OperationType     string  `json:"operationType"`
	PixAddressKey     string  `json:"pixAddressKey"`
	PixAddressKeyType string  `json:"pixAddressKeyType"`
	Description       string  `json:"description,omitempty"`
	ExternalReference string  `json:"externalReference,omitempty"`
}

// asaasTransferResponse mirrors the relevant fields of the Asaas /transfers response.
type asaasTransferResponse struct {
	ID            string `json:"id"`
	Status        string `json:"status"`
	OperationType string `json:"operationType"`
}

// ExecuteTransfer POSTs to /transfers and returns the normalised result.
func (o *OutboundTransferClient) ExecuteTransfer(ctx context.Context, req *portout.OutboundTransferRequest) (*portout.OutboundTransferResponse, error) {
	if !o.IsEnabled() {
		return &portout.OutboundTransferResponse{
			Success:      false,
			Provider:     "ASAAS",
			ErrorMessage: "Asaas outbound transfer provider is not configured",
		}, nil
	}

	wire := asaasTransferRequest{
		Value:             CentavosToReais(req.AmountCents),
		OperationType:     "PIX",
		PixAddressKey:     req.PixKey,
		PixAddressKeyType: string(req.PixKeyType),
		Description:       req.Description,
		ExternalReference: req.OutboundRequestID,
	}

	var resp asaasTransferResponse
	if err := o.client.doJSON(ctx, http.MethodPost, "/transfers", wire, &resp); err != nil {
		slog.ErrorContext(ctx, "asaas transfer failed",
			"outboundRequestID", req.OutboundRequestID,
			"error", err,
		)
		// Mirror Java behaviour: failures are returned as Success=false with the message
		// so the use case can mark the request FAILED instead of bubbling the error up.
		return &portout.OutboundTransferResponse{
			Success:      false,
			Provider:     "ASAAS",
			ErrorMessage: err.Error(),
		}, nil
	}

	slog.InfoContext(ctx, "asaas transfer executed",
		"outboundRequestID", req.OutboundRequestID,
		"asaasTransferID", resp.ID,
		"status", resp.Status,
	)

	return &portout.OutboundTransferResponse{
		Success:            true,
		Provider:           "ASAAS",
		ProviderTransferID: resp.ID,
		Status:             resp.Status,
	}, nil
}

