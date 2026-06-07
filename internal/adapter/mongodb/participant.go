package mongodb

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
)

// compile-time interface assertion.
var _ portout.ParticipanteRepository = (*ParticipanteMongoRepository)(nil)

// ParticipanteMongoRepository persists participant documents to MongoDB.
// The 'participants' collection schema matches Java's ParticipantEntity.
type ParticipanteMongoRepository struct {
	col *mongo.Collection
}

// NewParticipanteRepository creates a new ParticipanteMongoRepository.
func NewParticipanteRepository(client *Client) *ParticipanteMongoRepository {
	return &ParticipanteMongoRepository{col: client.Collection("participants")}
}

// SaveOrganizador creates a participant document for the event creator with
// papel=ORGANIZADOR and status=CONFIRMADO, matching Java's auto-registration behaviour.
func (r *ParticipanteMongoRepository) SaveOrganizador(ctx context.Context, eventoID, usuarioID string) error {
	newUUID := uuid.New()
	b := [16]byte(newUUID)
	docID := bson.Binary{Subtype: 0x04, Data: b[:]}

	now := time.Now().UTC()
	doc := bson.D{
		{Key: "_id", Value: docID},
		{Key: "evento_id", Value: uuidStringToBinary(eventoID)}, // eventoID is always UUID format from event Save
		{Key: "usuario_id", Value: userIDValue(usuarioID)},
		{Key: "papel", Value: "ORGANIZADOR"},
		{Key: "status", Value: "CONFIRMADO"},
		{Key: "criado_em", Value: now},
		{Key: "atualizado_em", Value: now},
	}

	if _, err := r.col.InsertOne(ctx, doc); err != nil {
		return fmt.Errorf("salvar participante organizador: %w", err)
	}
	return nil
}

// FindParticipationIDsByUserID returns the _id (as UUID strings) of all participation
// records for the given user. Used by the BUG5/spec-096 dual-search fix so that
// installments stored under a participation UUID are also returned.
func (r *ParticipanteMongoRepository) FindParticipationIDsByUserID(ctx context.Context, userID string) ([]string, error) {
	cursor, err := r.col.Find(ctx, bson.D{{Key: "usuario_id", Value: userIDValue(userID)}},
		options.Find().SetProjection(bson.D{{Key: "_id", Value: 1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("find participation ids by user: %w", err)
	}
	defer cursor.Close(ctx)

	var ids []string
	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		if id := rawIDToString(doc["_id"]); id != "" {
			ids = append(ids, id)
		}
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor participation ids: %w", err)
	}
	return ids, nil
}

// IsParticipantOfEvent reports whether the user has any participation record in the event.
func (r *ParticipanteMongoRepository) IsParticipantOfEvent(ctx context.Context, eventID, userID string) (bool, error) {
	count, err := r.col.CountDocuments(ctx, bson.D{
		{Key: "evento_id", Value: uuidStringToBinary(eventID)},
		{Key: "usuario_id", Value: userIDValue(userID)},
	})
	if err != nil {
		return false, fmt.Errorf("is participant of event: %w", err)
	}
	return count > 0, nil
}
