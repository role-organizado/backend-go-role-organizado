// Package finance contains use cases for finance domain operations.
package finance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ReconciliationUseCase performs a single finance reconciliation pass.
// TODO(spec-168): implementar nativo Go — atualmente delega ao Java via HTTP POST.
type ReconciliationUseCase struct {
	javaBackendURL string
	httpClient     *http.Client
}

// NewReconciliationUseCase creates a new ReconciliationUseCase delegating to the
// Java backend at javaBackendURL.
func NewReconciliationUseCase(javaBackendURL string) *ReconciliationUseCase {
	return &ReconciliationUseCase{
		javaBackendURL: javaBackendURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Minute,
		},
	}
}

type reconciliationRequest struct {
	ReferenceDate string `json:"referenceDate"`
}

// Execute delegates a single finance reconciliation pass to the Java backend.
// TODO(spec-168): implementar nativo Go
func (uc *ReconciliationUseCase) Execute(ctx context.Context, referenceDate string) error {
	payload, err := json.Marshal(reconciliationRequest{ReferenceDate: referenceDate})
	if err != nil {
		return fmt.Errorf("finance reconciliation: marshal request: %w", err)
	}

	url := uc.javaBackendURL + "/api/v1/admin/finance/reconciliation"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("finance reconciliation: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := uc.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("finance reconciliation: http request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("finance reconciliation: java backend returned %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
