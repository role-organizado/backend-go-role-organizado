package mongodb

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

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
