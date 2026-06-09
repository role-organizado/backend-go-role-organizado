// Package in defines input ports (use-case interfaces) for the pricing domain.
package in

import "context"

// RunPspCostReviewUseCase drives the daily PSP cost review process.
// Implementations may delegate to the Java backend (stub) or run natively.
type RunPspCostReviewUseCase interface {
	Execute(ctx context.Context, referenceDate string) error
}
