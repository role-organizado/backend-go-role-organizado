package migrations

import (
	"context"
	"fmt"
	"log/slog"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// RunV082CreateCofrinhoCollection applies the v082 migration:
//  1. Ensures the 'cofrinho_contribuicoes' collection exists.
//  2. Creates a compound index on (evento_id, status).
//  3. Creates a sparse index on guest_id.
func RunV082CreateCofrinhoCollection(ctx context.Context, db *mongo.Database) error {
	slog.Info("running migration v082: create cofrinho_contribuicoes collection")

	if err := ensureCofrinhoCollection(ctx, db); err != nil {
		return fmt.Errorf("v082 ensure cofrinho collection: %w", err)
	}

	if err := ensureCofrinhoIndexes(ctx, db); err != nil {
		return fmt.Errorf("v082 ensure cofrinho indexes: %w", err)
	}

	slog.Info("migration v082 completed")
	return nil
}

func ensureCofrinhoCollection(ctx context.Context, db *mongo.Database) error {
	names, err := db.ListCollectionNames(ctx, bson.D{})
	if err != nil {
		return fmt.Errorf("list collections: %w", err)
	}
	for _, name := range names {
		if name == "cofrinho_contribuicoes" {
			slog.Info("v082: cofrinho_contribuicoes collection already exists")
			return nil
		}
	}
	if err := db.CreateCollection(ctx, "cofrinho_contribuicoes"); err != nil {
		return fmt.Errorf("create collection: %w", err)
	}
	slog.Info("v082: cofrinho_contribuicoes collection created")
	return nil
}

func ensureCofrinhoIndexes(ctx context.Context, db *mongo.Database) error {
	col := db.Collection("cofrinho_contribuicoes")

	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "evento_id", Value: 1},
				{Key: "status", Value: 1},
			},
			Options: options.Index().SetName("evento_id_status"),
		},
		{
			Keys:    bson.D{{Key: "guest_id", Value: 1}},
			Options: options.Index().SetSparse(true).SetName("guest_id_sparse"),
		},
	}

	for _, idx := range indexes {
		name, err := col.Indexes().CreateOne(ctx, idx)
		if err != nil {
			slog.Warn("v082: could not create cofrinho index (may already exist)", "error", err)
			continue
		}
		slog.Info("v082: ensured cofrinho index", "index", name)
	}
	return nil
}
