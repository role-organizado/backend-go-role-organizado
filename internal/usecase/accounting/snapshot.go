// Package accounting holds use cases for accounting snapshot generation.
package accounting

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/accounting"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
)

// GenerateSnapshot implements portin.GenerateAccountingSnapshotUseCase.
//
// It aggregates platform-wide accounting totals for the requested window and
// persists the result as an immutable Snapshot, mirroring the Java
// AdminAccountingAggregationService.aggregate operation.
type GenerateSnapshot struct {
	repo  portout.AccountingSnapshotRepository
	clock func() time.Time
}

// NewGenerateSnapshot creates a new GenerateSnapshot use case.
func NewGenerateSnapshot(repo portout.AccountingSnapshotRepository) *GenerateSnapshot {
	return &GenerateSnapshot{repo: repo, clock: time.Now}
}

// Execute aggregates totals for the window and persists the snapshot.
func (uc *GenerateSnapshot) Execute(ctx context.Context, in portin.GenerateAccountingSnapshotInput) (*domain.Snapshot, error) {
	agg, err := uc.repo.Aggregate(ctx, in.DataInicio, in.DataFim)
	if err != nil {
		return nil, fmt.Errorf("accounting snapshot: aggregate: %w", err)
	}

	id := in.CorrelationID
	if id == "" {
		id = uuid.NewString()
	}

	snapshot := &domain.Snapshot{
		ID:              id,
		CorrelationID:   in.CorrelationID,
		DataInicio:      in.DataInicio,
		DataFim:         in.DataFim,
		GeneratedAt:     uc.clock().UTC(),
		TotalEventos:    agg.TotalEventos,
		TotalArrecadado: agg.TotalArrecadado,
		TotalRepassado:  agg.TotalRepassado,
		TotalTaxas:      agg.TotalTaxas,
		Status:          "COMPLETED",
	}

	if err := uc.repo.Save(ctx, snapshot); err != nil {
		return nil, fmt.Errorf("accounting snapshot: save: %w", err)
	}

	return snapshot, nil
}

// compile-time assertion: *GenerateSnapshot implements the use case interface.
var _ portin.GenerateAccountingSnapshotUseCase = (*GenerateSnapshot)(nil)
