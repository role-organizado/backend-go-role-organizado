package migrations

import (
	"context"
	"fmt"
	"log/slog"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// RunV088CreateGuestsBiometricIndexes applies the v088 migration:
//
//  1. 'guests': unique sparse index on telefone, unique sparse index on email.
//  2. 'biometric_credentials': compound partial unique index on
//     (usuario_id, device_id) WHERE is_active=true; plus secondary indexes.
//  3. 'biometric_challenges': TTL index on expires_at, index on device_id,
//     compound index on (device_id, used).
//
// Idempotent: index-already-exists errors are logged as warnings and the
// migration continues — mirrors the style of v085.
func RunV088CreateGuestsBiometricIndexes(ctx context.Context, db *mongo.Database) error {
	slog.Info("running migration v088: create guests + biometric indexes")

	if err := ensureGuestsIndexes(ctx, db); err != nil {
		return fmt.Errorf("v088 ensure guests indexes: %w", err)
	}
	if err := ensureBiometricCredentialsIndexes(ctx, db); err != nil {
		return fmt.Errorf("v088 ensure biometric_credentials indexes: %w", err)
	}
	if err := ensureBiometricChallengesIndexes(ctx, db); err != nil {
		return fmt.Errorf("v088 ensure biometric_challenges indexes: %w", err)
	}

	slog.Info("migration v088 completed")
	return nil
}

func ensureGuestsIndexes(ctx context.Context, db *mongo.Database) error {
	col := db.Collection("guests")
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "telefone", Value: 1}},
			Options: options.Index().
				SetUnique(true).
				SetSparse(true).
				SetName("guests_telefone_unique_sparse"),
		},
		{
			Keys: bson.D{{Key: "email", Value: 1}},
			Options: options.Index().
				SetUnique(true).
				SetSparse(true).
				SetName("guests_email_unique_sparse"),
		},
	}
	createIndexes(ctx, col, "guests", indexes)
	return nil
}

func ensureBiometricCredentialsIndexes(ctx context.Context, db *mongo.Database) error {
	col := db.Collection("biometric_credentials")
	partial := bson.D{{Key: "is_active", Value: true}}
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "usuario_id", Value: 1},
				{Key: "device_id", Value: 1},
			},
			Options: options.Index().
				SetUnique(true).
				SetPartialFilterExpression(partial).
				SetName("biometric_credentials_user_device_active_unique"),
		},
		{
			Keys:    bson.D{{Key: "usuario_id", Value: 1}},
			Options: options.Index().SetName("biometric_credentials_usuario_id"),
		},
		{
			Keys:    bson.D{{Key: "device_id", Value: 1}},
			Options: options.Index().SetName("biometric_credentials_device_id"),
		},
		{
			Keys:    bson.D{{Key: "is_active", Value: 1}},
			Options: options.Index().SetName("biometric_credentials_is_active"),
		},
	}
	createIndexes(ctx, col, "biometric_credentials", indexes)
	return nil
}

func ensureBiometricChallengesIndexes(ctx context.Context, db *mongo.Database) error {
	col := db.Collection("biometric_challenges")
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "expires_at", Value: 1}},
			Options: options.Index().
				SetExpireAfterSeconds(0).
				SetName("biometric_challenges_expires_at_ttl"),
		},
		{
			Keys:    bson.D{{Key: "device_id", Value: 1}},
			Options: options.Index().SetName("biometric_challenges_device_id"),
		},
		{
			Keys: bson.D{
				{Key: "device_id", Value: 1},
				{Key: "used", Value: 1},
			},
			Options: options.Index().SetName("biometric_challenges_device_used"),
		},
	}
	createIndexes(ctx, col, "biometric_challenges", indexes)
	return nil
}

// createIndexes is the shared idempotent helper for the v088 migration.
// Logs warnings (does not abort) on already-exists errors.
func createIndexes(ctx context.Context, col *mongo.Collection, label string, indexes []mongo.IndexModel) {
	for _, idx := range indexes {
		name, err := col.Indexes().CreateOne(ctx, idx)
		if err != nil {
			slog.Warn("v088: could not create index (may already exist)",
				"collection", label,
				"error", err)
			continue
		}
		slog.Info("v088: ensured index", "collection", label, "index", name)
	}
}
