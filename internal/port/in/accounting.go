package in

import (
	"context"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/accounting"
)

// GenerateAccountingSnapshotInput is the command for producing an accounting snapshot.
type GenerateAccountingSnapshotInput struct {
	// DataInicio / DataFim bound the aggregation window (YYYY-MM-DD, optional).
	DataInicio string
	DataFim    string
	// CorrelationID ties the snapshot to the originating request/run (optional).
	CorrelationID string
}

// GenerateAccountingSnapshotUseCase aggregates platform-wide accounting totals
// for the given window and persists a Snapshot. Mirrors the Java
// AdminAccountingAggregationService.aggregate operation.
type GenerateAccountingSnapshotUseCase interface {
	Execute(ctx context.Context, in GenerateAccountingSnapshotInput) (*domain.Snapshot, error)
}
