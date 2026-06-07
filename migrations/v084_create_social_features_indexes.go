package migrations

import (
	"context"
	"fmt"
	"log/slog"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// RunV084CreateSocialFeaturesIndexes applies the v084 migration:
//  1. Does NOT create the 'evento_social_features' collection — it already exists in Java.
//  2. Ensures a unique index on eventoId (one social-features doc per event).
//  3. Ensures a descending index on atualizadoEm for ordered queries.
//
// The migration is idempotent: index-already-exists errors are logged as warnings
// and the migration continues successfully.
func RunV084CreateSocialFeaturesIndexes(ctx context.Context, db *mongo.Database) error {
	slog.Info("running migration v084: create evento_social_features indexes")

	if err := ensureSocialFeaturesIndexes(ctx, db); err != nil {
		return fmt.Errorf("v084 ensure social_features indexes: %w", err)
	}

	slog.Info("migration v084 completed")
	return nil
}

func ensureSocialFeaturesIndexes(ctx context.Context, db *mongo.Database) error {
	col := db.Collection("evento_social_features")

	indexes := []mongo.IndexModel{
		{
			// Unique index on eventoId — one document per event (shared with Java).
			Keys: bson.D{{Key: "eventoId", Value: 1}},
			Options: options.Index().
				SetUnique(true).
				SetName("eventoId_unique"),
		},
		{
			// Descending index on atualizadoEm for ordered queries / change tracking.
			Keys:    bson.D{{Key: "atualizadoEm", Value: -1}},
			Options: options.Index().SetName("atualizadoEm_desc"),
		},
	}

	for _, idx := range indexes {
		name, err := col.Indexes().CreateOne(ctx, idx)
		if err != nil {
			// Idempotente: ignore IndexOptionsConflict (index already exists with same options)
			// or NamespaceExists (collection not yet visible). Log and continue.
			slog.Warn("v084: could not create social_features index (may already exist)",
				"error", err)
			continue
		}
		slog.Info("v084: ensured social_features index", "index", name)
	}
	return nil
}
