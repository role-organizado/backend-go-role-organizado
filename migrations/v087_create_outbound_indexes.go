package migrations

import (
	"context"
	"fmt"
	"log/slog"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// RunV087CreateOutboundIndexes applies the v087 migration.
//
// Ensures indexes on the outbound_requests and outbound_audit_logs collections.
// Idempotent: pre-existing indexes with the same spec are silently skipped.
func RunV087CreateOutboundIndexes(ctx context.Context, db *mongo.Database) error {
	slog.Info("running migration v087: ensure outbound collection indexes")

	steps := []struct {
		collection string
		indexes    []mongo.IndexModel
	}{
		{
			collection: "outbound_requests",
			indexes: []mongo.IndexModel{
				{
					Keys:    bson.D{{Key: "event_id", Value: 1}, {Key: "status", Value: 1}},
					Options: options.Index().SetName("event_id_status"),
				},
				{
					Keys:    bson.D{{Key: "event_id", Value: 1}, {Key: "created_at", Value: -1}},
					Options: options.Index().SetName("event_id_created_at"),
				},
				{
					Keys:    bson.D{{Key: "requester_user_id", Value: 1}, {Key: "created_at", Value: -1}},
					Options: options.Index().SetName("requester_user_id_created_at"),
				},
				{
					Keys:    bson.D{{Key: "rateio_id", Value: 1}, {Key: "status", Value: 1}},
					Options: options.Index().SetName("rateio_id_status_sparse").SetSparse(true),
				},
				{
					Keys:    bson.D{{Key: "provider_transfer_id", Value: 1}},
					Options: options.Index().SetName("provider_transfer_id_sparse").SetSparse(true),
				},
			},
		},
		{
			collection: "outbound_audit_logs",
			indexes: []mongo.IndexModel{
				{
					Keys:    bson.D{{Key: "request_id", Value: 1}, {Key: "occurred_at", Value: 1}},
					Options: options.Index().SetName("request_id_occurred_at"),
				},
				{
					Keys:    bson.D{{Key: "event_id", Value: 1}, {Key: "occurred_at", Value: -1}},
					Options: options.Index().SetName("event_id_occurred_at"),
				},
			},
		},
	}

	for _, step := range steps {
		if err := ensureOutboundIndexes(ctx, db, step.collection, step.indexes); err != nil {
			return fmt.Errorf("v087 ensure indexes for %s: %w", step.collection, err)
		}
	}

	slog.Info("migration v087 completed")
	return nil
}

func ensureOutboundIndexes(ctx context.Context, db *mongo.Database, colName string, indexes []mongo.IndexModel) error {
	col := db.Collection(colName)
	for _, idx := range indexes {
		name, err := col.Indexes().CreateOne(ctx, idx)
		if err != nil {
			slog.Warn("v087: could not create index (may already exist)",
				"collection", colName, "error", err)
			continue
		}
		slog.Info("v087: ensured index", "collection", colName, "index", name)
	}
	return nil
}
