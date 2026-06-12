package mongodb

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/accounting"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
)

// accountingSnapshotDoc is the BSON representation of an accounting snapshot
// stored in the `accounting_snapshots` collection.
type accountingSnapshotDoc struct {
	ID              string    `bson:"_id"`
	CorrelationID   string    `bson:"correlation_id,omitempty"`
	DataInicio      string    `bson:"data_inicio,omitempty"`
	DataFim         string    `bson:"data_fim,omitempty"`
	GeneratedAt     time.Time `bson:"generated_at"`
	TotalEventos    int64     `bson:"total_eventos"`
	TotalArrecadado int64     `bson:"total_arrecadado"`
	TotalRepassado  int64     `bson:"total_repassado"`
	TotalTaxas      int64     `bson:"total_taxas"`
	Status          string    `bson:"status"`
}

// AccountingSnapshotRepository persists and aggregates accounting snapshots in MongoDB.
type AccountingSnapshotRepository struct {
	snapshots *mongo.Collection
	summaries *mongo.Collection
	ledger    *mongo.Collection
}

// NewAccountingSnapshotRepository creates a repository backed by the
// `accounting_snapshots` collection, aggregating from `finance_summaries` and
// `ledger_entries`.
func NewAccountingSnapshotRepository(client *Client) *AccountingSnapshotRepository {
	return &AccountingSnapshotRepository{
		snapshots: client.Collection("accounting_snapshots"),
		summaries: client.Collection("finance_summaries"),
		ledger:    client.Collection("ledger_entries"),
	}
}

// Aggregate computes platform-wide accounting totals for the given window.
func (r *AccountingSnapshotRepository) Aggregate(ctx context.Context, dataInicio, dataFim string) (*portout.AccountingAggregation, error) {
	agg := &portout.AccountingAggregation{}

	// ── finance_summaries: events, collected, transferred ─────────────────────
	summaryPipeline := bson.A{
		bson.M{"$group": bson.M{
			"_id":             nil,
			"totalEventos":    bson.M{"$sum": 1},
			"totalArrecadado": bson.M{"$sum": "$collected"},
			"totalRepassado":  bson.M{"$sum": "$available_for_withdrawal"},
		}},
	}
	summaryCursor, err := r.summaries.Aggregate(ctx, summaryPipeline)
	if err != nil {
		return nil, fmt.Errorf("accounting snapshot: aggregate finance_summaries: %w", err)
	}
	defer summaryCursor.Close(ctx)

	var summaryRows []struct {
		TotalEventos    int64 `bson:"totalEventos"`
		TotalArrecadado int64 `bson:"totalArrecadado"`
		TotalRepassado  int64 `bson:"totalRepassado"`
	}
	if err := summaryCursor.All(ctx, &summaryRows); err != nil {
		return nil, fmt.Errorf("accounting snapshot: decode finance_summaries: %w", err)
	}
	if len(summaryRows) > 0 {
		agg.TotalEventos = summaryRows[0].TotalEventos
		agg.TotalArrecadado = summaryRows[0].TotalArrecadado
		agg.TotalRepassado = summaryRows[0].TotalRepassado
	}

	// ── ledger_entries: platform fees within the window ───────────────────────
	match := bson.M{
		"accounting_classification": bson.M{"$in": bson.A{"TAXA", "FEE", "PLATFORM_FEE"}},
	}
	if occurred := occurredAtFilter(dataInicio, dataFim); occurred != nil {
		match["occurred_at"] = occurred
	}
	feePipeline := bson.A{
		bson.M{"$match": match},
		bson.M{"$group": bson.M{
			"_id":        nil,
			"totalTaxas": bson.M{"$sum": "$amount"},
		}},
	}
	feeCursor, err := r.ledger.Aggregate(ctx, feePipeline)
	if err != nil {
		return nil, fmt.Errorf("accounting snapshot: aggregate ledger_entries: %w", err)
	}
	defer feeCursor.Close(ctx)

	var feeRows []struct {
		TotalTaxas int64 `bson:"totalTaxas"`
	}
	if err := feeCursor.All(ctx, &feeRows); err != nil {
		return nil, fmt.Errorf("accounting snapshot: decode ledger_entries: %w", err)
	}
	if len(feeRows) > 0 {
		agg.TotalTaxas = feeRows[0].TotalTaxas
	}

	return agg, nil
}

// Save inserts a new accounting snapshot document.
func (r *AccountingSnapshotRepository) Save(ctx context.Context, snapshot *domain.Snapshot) error {
	doc := accountingSnapshotDoc{
		ID:              snapshot.ID,
		CorrelationID:   snapshot.CorrelationID,
		DataInicio:      snapshot.DataInicio,
		DataFim:         snapshot.DataFim,
		GeneratedAt:     snapshot.GeneratedAt,
		TotalEventos:    snapshot.TotalEventos,
		TotalArrecadado: snapshot.TotalArrecadado,
		TotalRepassado:  snapshot.TotalRepassado,
		TotalTaxas:      snapshot.TotalTaxas,
		Status:          snapshot.Status,
	}
	if _, err := r.snapshots.InsertOne(ctx, doc); err != nil {
		return fmt.Errorf("accounting snapshot: insert: %w", err)
	}
	return nil
}

// occurredAtFilter builds a BSON range filter for occurred_at from optional
// YYYY-MM-DD bounds. Returns nil when both bounds are empty/unparseable.
func occurredAtFilter(dataInicio, dataFim string) bson.M {
	const layout = "2006-01-02"
	filter := bson.M{}
	if dataInicio != "" {
		if from, err := time.Parse(layout, dataInicio); err == nil {
			filter["$gte"] = from
		}
	}
	if dataFim != "" {
		if to, err := time.Parse(layout, dataFim); err == nil {
			// Inclusive of the whole end day.
			filter["$lt"] = to.AddDate(0, 0, 1)
		}
	}
	if len(filter) == 0 {
		return nil
	}
	return filter
}

// compile-time interface assertion.
var _ portout.AccountingSnapshotRepository = (*AccountingSnapshotRepository)(nil)
