package out

import (
	"context"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/accounting"
)

// AccountingAggregation holds the raw aggregated totals read from the data store
// before they are assembled into a persisted Snapshot.
type AccountingAggregation struct {
	TotalEventos    int64
	TotalArrecadado int64
	TotalRepassado  int64
	TotalTaxas      int64
}

// AccountingSnapshotRepository provides the read-side aggregation and write-side
// persistence for accounting snapshots.
type AccountingSnapshotRepository interface {
	// Aggregate computes platform-wide accounting totals for the given window.
	// Empty dataInicio/dataFim mean "unbounded" on that side.
	Aggregate(ctx context.Context, dataInicio, dataFim string) (*AccountingAggregation, error)
	// Save persists a generated snapshot.
	Save(ctx context.Context, snapshot *domain.Snapshot) error
}
