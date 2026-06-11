package migrations

import (
	"context"
	"fmt"
	"log/slog"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// RunV086CreateConviteIndexes applies the v086 migration:
//   - participants: compound index (evento_id, usuario_id), and (tipo_participante, usuario_id), (status).
//   - approval_items: (approver_id, status, created_at), (event_id, type), (target_entity_id, type), (status, expires_at).
//   - guests: (telefone) unique sparse, (email) unique sparse.
//   - participant_credits: (evento_id, participant_id).
//
// Idempotent: index-already-exists errors are logged as warnings and the migration
// continues successfully.
func RunV086CreateConviteIndexes(ctx context.Context, db *mongo.Database) error {
	slog.Info("running migration v086: create convite indexes")

	steps := []struct {
		name string
		fn   func(context.Context, *mongo.Database) error
	}{
		{"participants", ensureParticipantsConviteIndexes},
		{"approval_items", ensureApprovalItemsIndexes},
		{"guests", ensureGuestsIndexes},
		{"participant_credits", ensureParticipantCreditsIndexes},
	}
	for _, s := range steps {
		if err := s.fn(ctx, db); err != nil {
			return fmt.Errorf("v086 %s: %w", s.name, err)
		}
	}

	slog.Info("migration v086 completed")
	return nil
}

func ensureParticipantsConviteIndexes(ctx context.Context, db *mongo.Database) error {
	col := db.Collection("participants")
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "evento_id", Value: 1}, {Key: "usuario_id", Value: 1}},
			Options: options.Index().SetName("evento_usuario_idx"),
		},
		{
			Keys:    bson.D{{Key: "tipo_participante", Value: 1}, {Key: "usuario_id", Value: 1}},
			Options: options.Index().SetName("tipo_usuario_idx"),
		},
		{
			Keys:    bson.D{{Key: "status", Value: 1}},
			Options: options.Index().SetName("status_idx"),
		},
	}
	return ensureIndexes(ctx, col, "participants", indexes)
}

func ensureApprovalItemsIndexes(ctx context.Context, db *mongo.Database) error {
	col := db.Collection("approval_items")
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "approver_id", Value: 1}, {Key: "status", Value: 1}, {Key: "created_at", Value: -1}},
			Options: options.Index().SetName("approver_status_createdAt_idx"),
		},
		{
			Keys:    bson.D{{Key: "event_id", Value: 1}, {Key: "type", Value: 1}},
			Options: options.Index().SetName("event_type_idx"),
		},
		{
			Keys:    bson.D{{Key: "target_entity_id", Value: 1}, {Key: "type", Value: 1}},
			Options: options.Index().SetName("target_type_idx"),
		},
		{
			Keys:    bson.D{{Key: "status", Value: 1}, {Key: "expires_at", Value: 1}},
			Options: options.Index().SetName("status_expires_idx"),
		},
	}
	return ensureIndexes(ctx, col, "approval_items", indexes)
}

func ensureGuestsIndexes(ctx context.Context, db *mongo.Database) error {
	col := db.Collection("guests")
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "telefone", Value: 1}},
			Options: options.Index().SetUnique(true).SetSparse(true).SetName("telefone_unique_sparse"),
		},
		{
			Keys:    bson.D{{Key: "email", Value: 1}},
			Options: options.Index().SetUnique(true).SetSparse(true).SetName("email_unique_sparse"),
		},
	}
	return ensureIndexes(ctx, col, "guests", indexes)
}

func ensureParticipantCreditsIndexes(ctx context.Context, db *mongo.Database) error {
	col := db.Collection("participant_credits")
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "evento_id", Value: 1}, {Key: "participant_id", Value: 1}},
			Options: options.Index().SetName("evento_participant_idx"),
		},
	}
	return ensureIndexes(ctx, col, "participant_credits", indexes)
}

func ensureIndexes(ctx context.Context, col *mongo.Collection, label string, indexes []mongo.IndexModel) error {
	for _, idx := range indexes {
		name, err := col.Indexes().CreateOne(ctx, idx)
		if err != nil {
			slog.Warn("v086: could not create index (may already exist)",
				"collection", label, "error", err)
			continue
		}
		slog.Info("v086: ensured index", "collection", label, "index", name)
	}
	return nil
}
