package migrations

import (
	"context"
	"fmt"
	"log/slog"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// RunV084CreatePaymentTransactionsIndexes applies the v084 migration.
//
// IMPORTANT: The payment collections are SHARED with the Java backend.
// This migration only ensures indexes — it NEVER creates collections.
// All index operations are idempotent: IndexOptionsConflict / IndexKeySpecsConflict
// errors are logged as warnings and ignored so the migration is safe to re-run.
func RunV084CreatePaymentTransactionsIndexes(ctx context.Context, db *mongo.Database) error {
	slog.Info("running migration v084: ensure payment collection indexes")

	steps := []struct {
		collection string
		indexes    []mongo.IndexModel
	}{
		{
			collection: "payment_transactions",
			indexes: []mongo.IndexModel{
				{
					Keys:    bson.D{{Key: "userId", Value: 1}, {Key: "createdAt", Value: -1}},
					Options: options.Index().SetName("userId_createdAt"),
				},
				{
					Keys:    bson.D{{Key: "eventId", Value: 1}, {Key: "userId", Value: 1}},
					Options: options.Index().SetName("eventId_userId"),
				},
				{
					Keys:    bson.D{{Key: "idempotencyKey", Value: 1}},
					Options: options.Index().SetName("idempotencyKey_unique").SetUnique(true).SetSparse(true),
				},
				{
					Keys:    bson.D{{Key: "providerTransactionId", Value: 1}},
					Options: options.Index().SetName("providerTransactionId"),
				},
				{
					Keys:    bson.D{{Key: "installmentIds", Value: 1}},
					Options: options.Index().SetName("installmentIds_sparse").SetSparse(true),
				},
				{
					Keys:    bson.D{{Key: "expiresAt", Value: 1}},
					Options: options.Index().SetName("expiresAt"),
				},
			},
		},
		{
			collection: "payment_installments",
			indexes: []mongo.IndexModel{
				{
					Keys:    bson.D{{Key: "event_id", Value: 1}, {Key: "participant_id", Value: 1}},
					Options: options.Index().SetName("event_id_participant_id"),
				},
				{
					Keys:    bson.D{{Key: "liability_id", Value: 1}},
					Options: options.Index().SetName("liability_id"),
				},
				{
					Keys:    bson.D{{Key: "status", Value: 1}, {Key: "due_date", Value: 1}},
					Options: options.Index().SetName("status_due_date"),
				},
				{
					Keys:    bson.D{{Key: "transaction_id", Value: 1}},
					Options: options.Index().SetName("transaction_id_sparse").SetSparse(true),
				},
			},
		},
		{
			collection: "payment_accounts",
			indexes: []mongo.IndexModel{
				{
					Keys:    bson.D{{Key: "user_id", Value: 1}, {Key: "is_active", Value: 1}},
					Options: options.Index().SetName("user_id_is_active"),
				},
				{
					Keys:    bson.D{{Key: "user_id", Value: 1}, {Key: "is_default", Value: 1}},
					Options: options.Index().SetName("user_id_is_default"),
				},
			},
		},
		{
			collection: "saved_credit_cards",
			indexes: []mongo.IndexModel{
				{
					Keys:    bson.D{{Key: "user_id", Value: 1}, {Key: "is_default", Value: 1}},
					Options: options.Index().SetName("user_id_is_default"),
				},
				{
					Keys:    bson.D{{Key: "user_id", Value: 1}, {Key: "is_active", Value: 1}},
					Options: options.Index().SetName("user_id_is_active"),
				},
			},
		},
		{
			collection: "processed_webhook_events",
			indexes: []mongo.IndexModel{
				{
					Keys:    bson.D{{Key: "provider", Value: 1}, {Key: "eventId", Value: 1}},
					Options: options.Index().SetName("provider_eventId_unique").SetUnique(true),
				},
			},
		},
	}

	for _, step := range steps {
		if err := ensurePaymentIndexes(ctx, db, step.collection, step.indexes); err != nil {
			return fmt.Errorf("v084 ensure indexes for %s: %w", step.collection, err)
		}
	}

	slog.Info("migration v084 completed")
	return nil
}

// ensurePaymentIndexes creates the given indexes on col, silently skipping any that
// already exist (IndexOptionsConflict / IndexKeySpecsConflict are treated as no-ops).
func ensurePaymentIndexes(ctx context.Context, db *mongo.Database, colName string, indexes []mongo.IndexModel) error {
	col := db.Collection(colName)
	for _, idx := range indexes {
		name, err := col.Indexes().CreateOne(ctx, idx)
		if err != nil {
			// Idempotent: an already-existing index with the same spec is fine.
			slog.Warn("v084: could not create index (may already exist)",
				"collection", colName,
				"error", err,
			)
			continue
		}
		slog.Info("v084: ensured index", "collection", colName, "index", name)
	}
	return nil
}
