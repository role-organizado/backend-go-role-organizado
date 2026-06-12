// Package accounting holds domain types for accounting snapshots — periodic
// aggregations of platform-wide financial totals, mirroring the Java
// AdminAccountingAggregationService output.
package accounting

import "time"

// Snapshot is an immutable aggregation of platform accounting totals for a
// given date range, persisted to the accounting_snapshots collection for audit.
type Snapshot struct {
	// ID is the unique snapshot identifier (correlationId when supplied).
	ID string
	// CorrelationID ties this snapshot to the originating request/run.
	CorrelationID string
	// DataInicio / DataFim bound the aggregation window (YYYY-MM-DD, may be empty
	// to mean "from the beginning" / "until today").
	DataInicio string
	DataFim    string
	// GeneratedAt is the wall-clock time the snapshot was produced.
	GeneratedAt time.Time
	// Aggregated totals (centavos for monetary values).
	TotalEventos    int64
	TotalArrecadado int64
	TotalRepassado  int64
	TotalTaxas      int64
	// Status is "COMPLETED" once the snapshot has been persisted.
	Status string
}
