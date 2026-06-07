package payment

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
)

// ReaplicarFeePolicySnapshot implements portin.ReaplicarFeePolicyNasConfigsUseCase.
//
// Admin-only use case that reloads the vigente (current) fee policy and bulk-applies
// platformFeePercent / pspFeePercent to ALL EventoConfigPagamento documents.
//
// This mirrors the Java ReaplicarFeePolicySnapshotUseCase endpoint (admin).
// Route: POST /api/v1/admin/payments/reaplicar-fee-policy (ADMIN role required).
//
// Snapshot version format: pricing-policy:{versionId}:ALL:{effectiveFrom}
// Individual event configs saved via UpsertConfigPagamento use the format:
//   pricing-policy:{versionId}:{eventId}:{effectiveFrom}
type ReaplicarFeePolicySnapshot struct {
	configs portout.EventoConfigPagamentoRepository
}

// NewReaplicarFeePolicySnapshot creates a new ReaplicarFeePolicySnapshot use case.
func NewReaplicarFeePolicySnapshot(configs portout.EventoConfigPagamentoRepository) *ReaplicarFeePolicySnapshot {
	return &ReaplicarFeePolicySnapshot{configs: configs}
}

// Execute reapplies the supplied fee values to every EventoConfigPagamento document
// and stamps a new feePolicyVersion.
//
// The version is built as: pricing-policy:{versionId}:ALL:{effectiveFrom}
// where versionId comes from in.VersionID (or a generated UUID if empty).
func (uc *ReaplicarFeePolicySnapshot) Execute(ctx context.Context, in portin.ReaplicarFeePolicyNasConfigsInput) (*portin.ReaplicarFeePolicyResult, error) {
	versionID := in.VersionID
	if versionID == "" {
		versionID = uuid.New().String()
	}
	effectiveFrom := time.Now().UTC().Format(time.RFC3339)
	version := fmt.Sprintf("pricing-policy:%s:ALL:%s", versionID, effectiveFrom)

	count, err := uc.configs.BulkUpdateFeeFields(ctx, in.PlatformFeePercent, in.PspFeePercent, version)
	if err != nil {
		return nil, fmt.Errorf("reaplicar fee policy nas configs: %w", err)
	}

	return &portin.ReaplicarFeePolicyResult{UpdatedCount: count}, nil
}

// compile-time assertion: *ReaplicarFeePolicySnapshot implements the interface.
var _ portin.ReaplicarFeePolicyNasConfigsUseCase = (*ReaplicarFeePolicySnapshot)(nil)
