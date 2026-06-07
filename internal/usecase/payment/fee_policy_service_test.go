package payment_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	ucpayment "github.com/role-organizado/backend-go-role-organizado/internal/usecase/payment"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// TestFeePolicyService_ResolveSnapshot_GlobalFallback_WhenNoConfig verifies that
// the global default policy is returned when no event config exists.
func TestFeePolicyService_ResolveSnapshot_GlobalFallback_WhenNoConfig(t *testing.T) {
	repo := new(mockCfgRepo)
	svc := ucpayment.NewFeePolicyService(repo)

	repo.On("FindByEventoID", mock.Anything, "evt-missing").
		Return(nil, apierr.NotFound("config_pagamento", "evt-missing"))

	snap, err := svc.ResolveSnapshot(context.Background(), "evt-missing")
	require.NoError(t, err)
	assert.Equal(t, "GLOBAL", snap.FeePolicySource)
	assert.Equal(t, 1.99, snap.PspFeePercent)
	assert.Equal(t, 0.0, snap.PlatformFeePercent)
}

// TestFeePolicyService_ResolveSnapshot_GlobalFallback_WhenConfigHasNoFees verifies
// that configs with zero fee values and no FeePolicyVersion fall back to global.
func TestFeePolicyService_ResolveSnapshot_GlobalFallback_WhenConfigHasNoFees(t *testing.T) {
	repo := new(mockCfgRepo)
	svc := ucpayment.NewFeePolicyService(repo)

	// Config exists but has no fee configuration (old doc — backward compat).
	cfg := &domain.EventoConfigPagamento{
		ID:       "cfg-1",
		EventoID: "evt-old",
		// PlatformFeePercent, PspFeePercent are zero values — fee not configured.
		// FeePolicyVersion is empty.
	}
	repo.On("FindByEventoID", mock.Anything, "evt-old").Return(cfg, nil)

	snap, err := svc.ResolveSnapshot(context.Background(), "evt-old")
	require.NoError(t, err)
	assert.Equal(t, "GLOBAL", snap.FeePolicySource, "should fall back to global when no fees configured")
	assert.Equal(t, 1.99, snap.PspFeePercent)
}

// TestFeePolicyService_ResolveSnapshot_EventSpecific_WhenFeePolicyVersionSet
// verifies that typed fee fields are read from the config when FeePolicyVersion is set.
func TestFeePolicyService_ResolveSnapshot_EventSpecific_WhenFeePolicyVersionSet(t *testing.T) {
	repo := new(mockCfgRepo)
	svc := ucpayment.NewFeePolicyService(repo)

	cfg := &domain.EventoConfigPagamento{
		ID:                    "cfg-2",
		EventoID:              "evt-custom",
		PlatformFeePercent:    3.0,
		PspFeePercent:         1.5,
		PlatformFeeFixedCents: 50,
		PspFeeFixedCents:      25,
		FeePolicyVersion:      "pricing-policy:v1:evt-custom:2026-01-01T00:00:00Z",
	}
	repo.On("FindByEventoID", mock.Anything, "evt-custom").Return(cfg, nil)

	snap, err := svc.ResolveSnapshot(context.Background(), "evt-custom")
	require.NoError(t, err)
	assert.Equal(t, "EVENT:evt-custom", snap.FeePolicySource)
	assert.Equal(t, 3.0, snap.PlatformFeePercent)
	assert.Equal(t, 1.5, snap.PspFeePercent)
	assert.Equal(t, int64(50), snap.PlatformFeeFixedCents)
	assert.Equal(t, int64(25), snap.PspFeeFixedCents)
	assert.Equal(t, "pricing-policy:v1:evt-custom:2026-01-01T00:00:00Z", snap.Version)
}

// TestFeePolicyService_ResolveSnapshot_EventSpecific_WhenOnlyPercentsSet verifies
// backward compat: Java docs may have fee percents without a version field.
func TestFeePolicyService_ResolveSnapshot_EventSpecific_WhenOnlyPercentsSet(t *testing.T) {
	repo := new(mockCfgRepo)
	svc := ucpayment.NewFeePolicyService(repo)

	// Old-style Java doc: has fee percents but no feePolicyVersion.
	cfg := &domain.EventoConfigPagamento{
		ID:                 "cfg-3",
		EventoID:           "evt-java",
		PlatformFeePercent: 0.0, // platform takes nothing
		PspFeePercent:      2.5, // custom PSP rate
		// FeePolicyVersion empty (old Java doc)
	}
	repo.On("FindByEventoID", mock.Anything, "evt-java").Return(cfg, nil)

	snap, err := svc.ResolveSnapshot(context.Background(), "evt-java")
	require.NoError(t, err)
	// Non-zero PspFeePercent → custom policy used even without version.
	assert.Equal(t, "EVENT:evt-java", snap.FeePolicySource)
	assert.Equal(t, 2.5, snap.PspFeePercent)
	assert.Equal(t, 0.0, snap.PlatformFeePercent)
	assert.Equal(t, "event-v1", snap.Version, "version defaults to event-v1 when field is empty")
}

// TestFeePolicyService_ResolveSnapshot_ZeroPlatformFee_WithVersion ensures that
// PlatformFeePercent=0.0 + FeePolicyVersion != "" is treated as custom (not global).
func TestFeePolicyService_ResolveSnapshot_ZeroPlatformFee_WithVersion(t *testing.T) {
	repo := new(mockCfgRepo)
	svc := ucpayment.NewFeePolicyService(repo)

	cfg := &domain.EventoConfigPagamento{
		ID:                 "cfg-4",
		EventoID:           "evt-zero",
		PlatformFeePercent: 0.0, // explicitly no platform fee
		PspFeePercent:      0.0, // explicitly no PSP fee
		FeePolicyVersion:   "pricing-policy:v2:evt-zero:2026-01-01T00:00:00Z",
	}
	repo.On("FindByEventoID", mock.Anything, "evt-zero").Return(cfg, nil)

	snap, err := svc.ResolveSnapshot(context.Background(), "evt-zero")
	require.NoError(t, err)
	// FeePolicyVersion is set → use custom policy (both fees genuinely 0).
	assert.Equal(t, "EVENT:evt-zero", snap.FeePolicySource,
		"non-empty FeePolicyVersion must force custom policy even when both fees are 0")
	assert.Equal(t, 0.0, snap.PlatformFeePercent)
	assert.Equal(t, 0.0, snap.PspFeePercent)
}
