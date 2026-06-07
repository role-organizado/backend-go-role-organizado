package payment_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
)

// ─── CalculateFees ───────────────────────────────────────────────────────────

func TestCalculateFees(t *testing.T) {
	tests := []struct {
		name       string
		grossCents int64
		policy     payment.FeePolicySnapshot
		want       payment.FeeCalculationResult
	}{
		{
			name:       "zero fee — no psp, no platform",
			grossCents: 1000,
			policy: payment.FeePolicySnapshot{
				PspFeePercent: 0, PspFeeFixedCents: 0,
				PlatformFeePercent: 0, PlatformFeeFixedCents: 0,
			},
			want: payment.FeeCalculationResult{
				GrossAmountCents:        1000,
				PspFeeAppliedCents:      0,
				PlatformFeeAppliedCents: 0,
				NetAmountCents:          1000,
			},
		},
		{
			name:       "percent only — psp 2.5% + platform 1%",
			grossCents: 1000,
			policy: payment.FeePolicySnapshot{
				PspFeePercent: 2.5, PspFeeFixedCents: 0,
				PlatformFeePercent: 1.0, PlatformFeeFixedCents: 0,
			},
			// psp:      round(1000 * 2.5 / 100) = round(25.0) = 25
			// platform: round(1000 * 1.0 / 100) = round(10.0) = 10
			want: payment.FeeCalculationResult{
				GrossAmountCents:        1000,
				PspFeeAppliedCents:      25,
				PlatformFeeAppliedCents: 10,
				NetAmountCents:          965,
			},
		},
		{
			name:       "fixed only — psp 100¢ + platform 50¢",
			grossCents: 1000,
			policy: payment.FeePolicySnapshot{
				PspFeePercent: 0, PspFeeFixedCents: 100,
				PlatformFeePercent: 0, PlatformFeeFixedCents: 50,
			},
			want: payment.FeeCalculationResult{
				GrossAmountCents:        1000,
				PspFeeAppliedCents:      100,
				PlatformFeeAppliedCents: 50,
				NetAmountCents:          850,
			},
		},
		{
			name:       "percent plus fixed combined",
			grossCents: 1000,
			policy: payment.FeePolicySnapshot{
				PspFeePercent: 2.5, PspFeeFixedCents: 100,
				PlatformFeePercent: 1.0, PlatformFeeFixedCents: 50,
			},
			// psp:      round(25) + 100 = 125
			// platform: round(10) + 50  = 60
			// net:      1000 - 125 - 60 = 815
			want: payment.FeeCalculationResult{
				GrossAmountCents:        1000,
				PspFeeAppliedCents:      125,
				PlatformFeeAppliedCents: 60,
				NetAmountCents:          815,
			},
		},
		{
			name:       "rounding half-up at exactly .5 cents",
			grossCents: 100,
			policy: payment.FeePolicySnapshot{
				PspFeePercent: 2.5, PspFeeFixedCents: 0,
				PlatformFeePercent: 0, PlatformFeeFixedCents: 0,
			},
			// psp: round(100 * 2.5 / 100) = round(2.5) = 3  ← half-up
			want: payment.FeeCalculationResult{
				GrossAmountCents:        100,
				PspFeeAppliedCents:      3,
				PlatformFeeAppliedCents: 0,
				NetAmountCents:          97,
			},
		},
		{
			name:       "rounding down when below .5",
			grossCents: 333,
			policy: payment.FeePolicySnapshot{
				PspFeePercent: 0, PspFeeFixedCents: 0,
				PlatformFeePercent: 1.0, PlatformFeeFixedCents: 0,
			},
			// platform: round(333 * 1.0 / 100) = round(3.33) = 3  ← rounds down
			want: payment.FeeCalculationResult{
				GrossAmountCents:        333,
				PspFeeAppliedCents:      0,
				PlatformFeeAppliedCents: 3,
				NetAmountCents:          330,
			},
		},
		{
			name:       "rounding up at .5 for platform",
			grossCents: 350,
			policy: payment.FeePolicySnapshot{
				PspFeePercent: 0, PspFeeFixedCents: 0,
				PlatformFeePercent: 1.0, PlatformFeeFixedCents: 0,
			},
			// platform: round(350 * 1.0 / 100) = round(3.5) = 4  ← half-up
			want: payment.FeeCalculationResult{
				GrossAmountCents:        350,
				PspFeeAppliedCents:      0,
				PlatformFeeAppliedCents: 4,
				NetAmountCents:          346,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := payment.CalculateFees(tt.grossCents, tt.policy)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ─── PaymentTransaction.IsTerminal ──────────────────────────────────────────

func TestPaymentTransaction_IsTerminal(t *testing.T) {
	tests := []struct {
		status payment.TransactionStatus
		want   bool
	}{
		{payment.TransactionStatusPending, false},
		{payment.TransactionStatusProcessing, false},
		{payment.TransactionStatusCompleted, true},
		{payment.TransactionStatusFailed, true},
		{payment.TransactionStatusCancelled, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			tx := &payment.PaymentTransaction{Status: tt.status}
			assert.Equal(t, tt.want, tx.IsTerminal())
		})
	}
}

// ─── PaymentTransaction.MarkCompleted ───────────────────────────────────────

func TestPaymentTransaction_MarkCompleted(t *testing.T) {
	tx := &payment.PaymentTransaction{
		Status: payment.TransactionStatusProcessing,
	}
	at := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)

	tx.MarkCompleted(at)

	assert.Equal(t, payment.TransactionStatusCompleted, tx.Status)
	assert.NotNil(t, tx.CompletedAt)
	assert.Equal(t, at, *tx.CompletedAt)
	assert.Equal(t, at, tx.UpdatedAt)
	assert.True(t, tx.IsTerminal(), "completed transaction must be terminal")
}

// ─── PaymentTransaction.MarkFailed ──────────────────────────────────────────

func TestPaymentTransaction_MarkFailed(t *testing.T) {
	tx := &payment.PaymentTransaction{
		Status: payment.TransactionStatusProcessing,
	}
	before := time.Now()

	tx.MarkFailed("insufficient funds")

	assert.Equal(t, payment.TransactionStatusFailed, tx.Status)
	assert.Equal(t, "insufficient funds", tx.FailureReason)
	assert.False(t, tx.UpdatedAt.Before(before), "UpdatedAt must not be before the call")
	assert.True(t, tx.IsTerminal(), "failed transaction must be terminal")
}

// ─── PaymentTransaction.IsOwner ─────────────────────────────────────────────

func TestPaymentTransaction_IsOwner(t *testing.T) {
	tx := &payment.PaymentTransaction{UserID: "user-123"}

	assert.True(t, tx.IsOwner("user-123"))
	assert.False(t, tx.IsOwner("user-456"))
	assert.False(t, tx.IsOwner(""))
}
