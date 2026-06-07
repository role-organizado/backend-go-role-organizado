// Package activity contains Temporal activity implementations.
package activity

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/activity"

	"github.com/role-organizado/backend-go-role-organizado/internal/port/in"
)

// PricingPspReviewActivity wraps the RunPspCostReviewUseCase as a Temporal activity.
type PricingPspReviewActivity struct {
	useCase in.RunPspCostReviewUseCase
}

// NewPricingPspReviewActivity creates a new PricingPspReviewActivity with the given use case.
func NewPricingPspReviewActivity(uc in.RunPspCostReviewUseCase) *PricingPspReviewActivity {
	return &PricingPspReviewActivity{useCase: uc}
}

// RunPspCostReview executes the PSP cost review for the given reference date.
// It is registered in Temporal as the activity "RunPspCostReview".
func (a *PricingPspReviewActivity) RunPspCostReview(ctx context.Context, referenceDate string) error {
	logger := activity.GetLogger(ctx)
	logger.Info("starting psp cost review activity", "referenceDate", referenceDate)

	if err := a.useCase.Execute(ctx, referenceDate); err != nil {
		return fmt.Errorf("run psp cost review: %w", err)
	}

	logger.Info("psp cost review activity completed", "referenceDate", referenceDate)
	return nil
}
