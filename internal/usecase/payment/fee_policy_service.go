package payment

import (
	"context"
	"fmt"
	"log/slog"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// FeePolicyResolver resolves the fee policy snapshot for a payment.
// Mocked in tests; concrete implementation is FeePolicyService.
type FeePolicyResolver interface {
	ResolveSnapshot(ctx context.Context, eventID string) (domain.FeePolicySnapshot, error)
}

// globalPolicy is the default fee configuration used when no event-specific
// policy exists. Mirrors the Java GlobalFeePolicyConfig defaults.
var globalPolicy = domain.FeePolicySnapshot{
	FeePolicySource:       "GLOBAL",
	PlatformFeePercent:    0.0,
	PlatformFeeFixedCents: 0,
	PspFeePercent:         1.99, // Typical Asaas PIX/Boleto fee
	PspFeeFixedCents:      0,
	Version:               "global-v1",
}

// FeePolicyService resolves fee configuration from the shared evento_config_pagamentos
// collection via the EventoConfigPagamentoRepository (typed domain fields).
//
// If the document has platformFeePercent/pspFeePercent or a feePolicyVersion set,
// those values are used. Falls back to the GLOBAL policy otherwise.
//
// The repository FindByEventoID queries both camelCase (eventoId — Java) and
// snake_case (evento_id — Go) to support the shared collection.
//
// Implements FeePolicyResolver.
type FeePolicyService struct {
	repo portout.EventoConfigPagamentoRepository
}

// NewFeePolicyService creates a new FeePolicyService backed by the given repository.
func NewFeePolicyService(repo portout.EventoConfigPagamentoRepository) *FeePolicyService {
	return &FeePolicyService{repo: repo}
}

// ResolveSnapshot reads the evento_config_pagamentos collection for eventID and returns
// a FeePolicySnapshot built from typed domain fields. If the document is absent or lacks
// fee configuration, the GLOBAL fallback policy is returned.
func (s *FeePolicyService) ResolveSnapshot(ctx context.Context, eventID string) (domain.FeePolicySnapshot, error) {
	cfg, err := s.repo.FindByEventoID(ctx, eventID)
	if err != nil {
		// NotFound → use global policy.
		if ae, ok := err.(*apierr.APIError); ok && ae.Status == 404 {
			slog.DebugContext(ctx, "fee policy: event config not found, using global", "eventID", eventID)
			return globalPolicy, nil
		}
		return domain.FeePolicySnapshot{}, fmt.Errorf("fee policy: find event config for %s: %w", eventID, err)
	}

	// Determine whether a custom fee policy was configured.
	// Primary discriminator: non-empty FeePolicyVersion (set by UpsertConfigPagamento
	// or ReaplicarFeePolicySnapshotUseCase when fees were explicitly configured).
	// Secondary: non-zero fee percents (for old Java docs without version field).
	hasCustomPolicy := cfg.FeePolicyVersion != "" || cfg.PlatformFeePercent != 0 || cfg.PspFeePercent != 0

	if !hasCustomPolicy {
		slog.DebugContext(ctx, "fee policy: event config has no custom fees, using global", "eventID", eventID)
		return globalPolicy, nil
	}

	snap := domain.FeePolicySnapshot{
		FeePolicySource:       "EVENT:" + eventID,
		PlatformFeePercent:    cfg.PlatformFeePercent,
		PspFeePercent:         cfg.PspFeePercent,
		PlatformFeeFixedCents: cfg.PlatformFeeFixedCents,
		PspFeeFixedCents:      cfg.PspFeeFixedCents,
		Version:               cfg.FeePolicyVersion,
	}
	if snap.Version == "" {
		snap.Version = "event-v1"
	}

	return snap, nil
}

// compile-time assertion: *FeePolicyService implements FeePolicyResolver.
var _ FeePolicyResolver = (*FeePolicyService)(nil)
