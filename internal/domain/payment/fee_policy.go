package payment

import "math"

// FeePolicySnapshot captures the fee configuration at the exact moment a charge
// is created. Storing a snapshot prevents retroactive fee changes from affecting
// already-created transactions.
type FeePolicySnapshot struct {
	FeePolicySource       string
	PlatformFeePercent    float64
	PlatformFeeFixedCents int64
	PspFeePercent         float64
	PspFeeFixedCents      int64
	Version               string
}

// FeeCalculationResult holds the fee breakdown for a given charge amount.
// All amounts are in integer cents.
type FeeCalculationResult struct {
	GrossAmountCents        int64
	PspFeeAppliedCents      int64
	PlatformFeeAppliedCents int64
	NetAmountCents          int64
}

// CalculateFees computes PSP and platform fees for grossCents using the provided
// policy snapshot.
//
// Rounding rule (mirrors Java FeePolicyService): percentage fees are rounded
// half-up to the nearest cent using math.Round (rounds half away from zero,
// equivalent to half-up for positive amounts). Fixed fees are added after
// rounding and never rounded themselves.
func CalculateFees(grossCents int64, s FeePolicySnapshot) FeeCalculationResult {
	pspPercentFee := int64(math.Round(float64(grossCents) * s.PspFeePercent / 100.0))
	pspFeeApplied := pspPercentFee + s.PspFeeFixedCents

	platformPercentFee := int64(math.Round(float64(grossCents) * s.PlatformFeePercent / 100.0))
	platformFeeApplied := platformPercentFee + s.PlatformFeeFixedCents

	return FeeCalculationResult{
		GrossAmountCents:        grossCents,
		PspFeeAppliedCents:      pspFeeApplied,
		PlatformFeeAppliedCents: platformFeeApplied,
		NetAmountCents:          grossCents - pspFeeApplied - platformFeeApplied,
	}
}
