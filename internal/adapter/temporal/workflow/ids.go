// Package workflow contains Temporal workflow definitions and ID helpers.
package workflow

import "fmt"

// PricingPspReviewPrimaryID returns the deterministic workflow ID for a
// manual/triggered PricingPspReview run, scoped to a specific reference date.
//
// Example: pricing-psp-review-real-2026-06-06
func PricingPspReviewPrimaryID(referenceDate string) string {
	return fmt.Sprintf("pricing-psp-review-real-%s", referenceDate)
}
