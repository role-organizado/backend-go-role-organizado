package migrations

import (
	"context"
	"fmt"
	"log/slog"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// RunV083CreateListaPresentesCollection applies the v083 migration:
//  1. Ensures the 'lista_presentes_itens' collection exists.
//  2. Creates a compound index on (evento_id, status).
//  3. Creates an index on owner_user_id.
func RunV083CreateListaPresentesCollection(ctx context.Context, db *mongo.Database) error {
	slog.Info("running migration v083: create lista_presentes_itens collection")

	if err := ensureListaPresentesCollection(ctx, db); err != nil {
		return fmt.Errorf("v083 ensure lista_presentes collection: %w", err)
	}

	if err := ensureListaPresentesIndexes(ctx, db); err != nil {
		return fmt.Errorf("v083 ensure lista_presentes indexes: %w", err)
	}

	slog.Info("migration v083 completed")
	return nil
}

func ensureListaPresentesCollection(ctx context.Context, db *mongo.Database) error {
	names, err := db.ListCollectionNames(ctx, bson.D{})
	if err != nil {
		return fmt.Errorf("list collections: %w", err)
	}
	for _, name := range names {
		if name == "lista_presentes_itens" {
			slog.Info("v083: lista_presentes_itens collection already exists")
			return nil
		}
	}
	if err := db.CreateCollection(ctx, "lista_presentes_itens"); err != nil {
		return fmt.Errorf("create collection: %w", err)
	}
	slog.Info("v083: lista_presentes_itens collection created")
	return nil
}

func ensureListaPresentesIndexes(ctx context.Context, db *mongo.Database) error {
	col := db.Collection("lista_presentes_itens")

	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "evento_id", Value: 1},
				{Key: "status", Value: 1},
			},
			Options: options.Index().SetName("evento_id_status"),
		},
		{
			Keys:    bson.D{{Key: "owner_user_id", Value: 1}},
			Options: options.Index().SetName("owner_user_id"),
		},
	}

	for _, idx := range indexes {
		name, err := col.Indexes().CreateOne(ctx, idx)
		if err != nil {
			slog.Warn("v083: could not create lista_presentes index (may already exist)", "error", err)
			continue
		}
		slog.Info("v083: ensured lista_presentes index", "index", name)
	}
	return nil
}
