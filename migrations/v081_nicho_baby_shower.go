// Package migrations contains idempotent Go migration functions run at startup.
package migrations

import (
	"context"
	"fmt"
	"log/slog"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// RunV081NichoBabyShower applies the v081 migration:
//  1. Updates the 'baby_shower' dominio document with a modular template and removes emBreve.
//  2. Ensures a sparse index on 'modulos_ativos' in the 'eventos' collection.
func RunV081NichoBabyShower(ctx context.Context, db *mongo.Database) error {
	slog.Info("running migration v081: nicho baby_shower")

	if err := updateBabyShowerDominio(ctx, db); err != nil {
		return fmt.Errorf("v081 update baby_shower dominio: %w", err)
	}

	if err := ensureModulosAtivosIndex(ctx, db); err != nil {
		return fmt.Errorf("v081 ensure modulos_ativos index: %w", err)
	}

	slog.Info("migration v081 completed")
	return nil
}

func updateBabyShowerDominio(ctx context.Context, db *mongo.Database) error {
	col := db.Collection("dominios")
	filter := bson.D{
		{Key: "categoria", Value: "tipo_evento"},
		{Key: "chave", Value: "baby_shower"},
	}
	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "metadata.template", Value: bson.D{
				{Key: "modulosSuportados", Value: bson.A{"COFRINHO", "LISTA_COLABORATIVA"}},
				{Key: "modulosPadrao", Value: bson.A{"COFRINHO"}},
			}},
		}},
		{Key: "$unset", Value: bson.D{
			{Key: "metadata.emBreve", Value: ""},
		}},
	}
	res, err := col.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("updateOne dominios: %w", err)
	}
	if res.MatchedCount == 0 {
		slog.Warn("v081: baby_shower dominio document not found, skipping update")
	} else {
		slog.Info("v081: baby_shower dominio updated", "modified", res.ModifiedCount)
	}
	return nil
}

func ensureModulosAtivosIndex(ctx context.Context, db *mongo.Database) error {
	col := db.Collection("eventos")
	indexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "modulos_ativos", Value: 1}},
		Options: options.Index().SetSparse(true).SetName("modulos_ativos_sparse"),
	}
	name, err := col.Indexes().CreateOne(ctx, indexModel)
	if err != nil {
		// Index may already exist with same spec — treat as idempotent.
		slog.Warn("v081: could not create modulos_ativos index (may already exist)", "error", err)
		return nil
	}
	slog.Info("v081: ensured index on eventos.modulos_ativos", "index", name)
	return nil
}
