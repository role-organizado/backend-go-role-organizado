// Package pricing holds use-case implementations for the pricing domain.
package pricing

import (
	"context"
	"fmt"
	"net/http"

	"github.com/role-organizado/backend-go-role-organizado/internal/config"
)

// runPspCostReviewUseCase delegates PSP cost review to the Java backend.
// TODO(spec-168): implementar nativo Go
type runPspCostReviewUseCase struct {
	cfg        *config.AppConfig
	httpClient *http.Client
}

// NewRunPspCostReview creates a new RunPspCostReviewUseCase backed by the Java endpoint.
func NewRunPspCostReview(cfg *config.AppConfig, httpClient *http.Client) *runPspCostReviewUseCase {
	return &runPspCostReviewUseCase{cfg: cfg, httpClient: httpClient}
}

// Execute triggers the PSP cost review via the Java backend.
// If referenceDate is empty the Java backend uses its own current-date logic.
func (uc *runPspCostReviewUseCase) Execute(ctx context.Context, referenceDate string) error {
	// TODO(spec-168): implementar nativo Go
	url := uc.cfg.Server.JavaBackendURL + "/api/v1/admin/pricing/psp-review"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("criar requisição psp-review: %w", err)
	}

	if referenceDate != "" {
		q := req.URL.Query()
		q.Set("referenceDate", referenceDate)
		req.URL.RawQuery = q.Encode()
	}

	resp, err := uc.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executar psp-review via java: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("java backend retornou status %d para psp-review", resp.StatusCode)
	}

	return nil
}
