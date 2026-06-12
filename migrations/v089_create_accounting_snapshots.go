package migrations

import (
	"context"
	"log/slog"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// RunV089CreateAccountingSnapshots applies the v089 migration.
//
// Ensures indexes on the accounting_snapshots collection produced by the
// AccountingSnapshotWorkflow. Idempotent: pre-existing indexes with the same
// spec are silently skipped.
func RunV089CreateAccountingSnapshots(ctx context.Context, db *mongo.Database) error {
	slog.Info("running migration v089: ensure accounting_snapshots indexes")

	col := db.Collection("accounting_snapshots")
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "generated_at", Value: -1}},
			Options: options.Index().SetName("generated_at_desc"),
		},
		{
			Keys:    bson.D{{Key: "correlation_id", Value: 1}},
			Options: options.Index().SetName("correlation_id_sparse").SetSparse(true),
		},
		{
			Keys:    bson.D{{Key: "data_inicio", Value: 1}, {Key: "data_fim", Value: 1}},
			Options: options.Index().SetName("data_inicio_data_fim"),
		},
	}

	for _, idx := range indexes {
		name, err := col.Indexes().CreateOne(ctx, idx)
		if err != nil {
			slog.Warn("v089: could not create index (may already exist)",
				"collection", "accounting_snapshots", "error", err)
			continue
		}
		slog.Info("v089: ensured index", "collection", "accounting_snapshots", "index", name)
	}

	slog.Info("migration v089 completed")
	return nil
}
