package payment_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	ucpayment "github.com/role-organizado/backend-go-role-organizado/internal/usecase/payment"
)

// TestReaplicarFeePolicySnapshot_Execute_BulkUpdatesAllConfigs verifies that the
// use case calls BulkUpdateFeeFields with the correct values and returns the
// count reported by the repository.
func TestReaplicarFeePolicySnapshot_Execute_BulkUpdatesAllConfigs(t *testing.T) {
	cfgRepo := new(mockCfgRepo)
	uc := ucpayment.NewReaplicarFeePolicySnapshot(cfgRepo)

	// Repository reports 5 documents updated.
	cfgRepo.On("BulkUpdateFeeFields",
		mock.Anything,
		5.0,  // platformFeePercent
		2.0,  // pspFeePercent
		mock.MatchedBy(func(v string) bool {
			// Version must follow the format: pricing-policy:{uuid}:ALL:{timestamp}
			return len(v) > len("pricing-policy::ALL:") &&
				v[:len("pricing-policy:")] == "pricing-policy:" &&
				containsSubstr(v, ":ALL:")
		}),
	).Return(int64(5), nil)

	result, err := uc.Execute(context.Background(), portin.ReaplicarFeePolicyNasConfigsInput{
		PlatformFeePercent: 5.0,
		PspFeePercent:      2.0,
		RequesterID:        "admin-1",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int64(5), result.UpdatedCount)

	cfgRepo.AssertExpectations(t)
}

// TestReaplicarFeePolicySnapshot_Execute_CustomVersionID verifies that when the
// caller supplies a VersionID it is embedded in the snapshot version string.
func TestReaplicarFeePolicySnapshot_Execute_CustomVersionID(t *testing.T) {
	cfgRepo := new(mockCfgRepo)
	uc := ucpayment.NewReaplicarFeePolicySnapshot(cfgRepo)

	cfgRepo.On("BulkUpdateFeeFields",
		mock.Anything,
		0.5, // platformFeePercent
		1.99, // pspFeePercent
		mock.MatchedBy(func(v string) bool {
			return containsSubstr(v, "my-custom-version-id")
		}),
	).Return(int64(12), nil)

	result, err := uc.Execute(context.Background(), portin.ReaplicarFeePolicyNasConfigsInput{
		PlatformFeePercent: 0.5,
		PspFeePercent:      1.99,
		VersionID:          "my-custom-version-id",
		RequesterID:        "admin-2",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(12), result.UpdatedCount)

	cfgRepo.AssertExpectations(t)
}

// TestReaplicarFeePolicySnapshot_Execute_RepositoryError propagates repository errors.
func TestReaplicarFeePolicySnapshot_Execute_RepositoryError(t *testing.T) {
	cfgRepo := new(mockCfgRepo)
	uc := ucpayment.NewReaplicarFeePolicySnapshot(cfgRepo)

	cfgRepo.On("BulkUpdateFeeFields",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	).Return(int64(0), assert.AnError)

	_, err := uc.Execute(context.Background(), portin.ReaplicarFeePolicyNasConfigsInput{
		PlatformFeePercent: 1.0,
		PspFeePercent:      2.0,
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "reaplicar fee policy nas configs")
}

// containsSubstr is a helper to check substring without importing strings in test.
func containsSubstr(s, sub string) bool {
	return len(s) >= len(sub) && func() bool {
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	}()
}
