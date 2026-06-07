package mongodb

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/finance"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
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

// ===================================================================
// ParticipantMongoRepository — read-side, implements portout.ParticipantRepository
// Queries the 'participants' collection for the finance domain use cases.
// ===================================================================

type participantDocument struct {
	ID      interface{} `bson:"_id,omitempty"`
	EventID bson.Binary `bson:"evento_id"`
	UserID  interface{} `bson:"usuario_id"`
	Papel   string      `bson:"papel"`
	Status  string      `bson:"status"`
	Nome    string      `bson:"nome,omitempty"`
	Email   string      `bson:"email,omitempty"`
}

func participantDocToDomain(doc participantDocument) domain.Participant {
	return domain.Participant{
		ID:      rawIDToString(doc.ID),
		EventID: uuidBinaryToString(doc.EventID),
		UserID:  rawIDToString(doc.UserID),
		Name:    doc.Nome,
		Email:   doc.Email,
		Status:  doc.Status,
	}
}

// ParticipantMongoRepository implements portout.ParticipantRepository (read-side).
type ParticipantMongoRepository struct {
	col *mongo.Collection
}

// NewParticipantRepository creates a read-side ParticipantRepository.
func NewParticipantRepository(client *Client) portout.ParticipantRepository {
	return &ParticipantMongoRepository{col: client.Collection("participants")}
}

// FindByUserID returns all participations for the given user.
func (r *ParticipantMongoRepository) FindByUserID(ctx context.Context, userID string) ([]domain.Participant, error) {
	filter := bson.D{{Key: "usuario_id", Value: userIDValue(userID)}}
	cur, err := r.col.Find(ctx, filter)
	if err != nil {
		return nil, apierr.Internal(err.Error())
	}
	defer cur.Close(ctx)

	var result []domain.Participant
	for cur.Next(ctx) {
		var doc participantDocument
		if err := cur.Decode(&doc); err != nil {
			return nil, apierr.Internal(err.Error())
		}
		result = append(result, participantDocToDomain(doc))
	}
	if err := cur.Err(); err != nil {
		return nil, apierr.Internal(err.Error())
	}
	return result, nil
}

// FindByEventID returns a paginated list of participants for the given event.
func (r *ParticipantMongoRepository) FindByEventID(ctx context.Context, eventID string, page, size int) ([]domain.Participant, int64, error) {
	filter := bson.D{{Key: "evento_id", Value: UUIDStringToBinary(eventID)}}

	total, err := r.col.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, apierr.Internal(err.Error())
	}

	skip := int64(page * size)
	opts := options.Find().SetSkip(skip).SetLimit(int64(size))
	cur, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, apierr.Internal(err.Error())
	}
	defer cur.Close(ctx)

	var result []domain.Participant
	for cur.Next(ctx) {
		var doc participantDocument
		if err := cur.Decode(&doc); err != nil {
			return nil, 0, apierr.Internal(err.Error())
		}
		result = append(result, participantDocToDomain(doc))
	}
	if err := cur.Err(); err != nil {
		return nil, 0, apierr.Internal(err.Error())
	}
	return result, total, nil
}

// FindByEventIDAndUserID returns the participation for a specific user in a specific event.
func (r *ParticipantMongoRepository) FindByEventIDAndUserID(ctx context.Context, eventID, userID string) (*domain.Participant, error) {
	filter := bson.D{
		{Key: "evento_id", Value: UUIDStringToBinary(eventID)},
		{Key: "usuario_id", Value: userIDValue(userID)},
	}
	var doc participantDocument
	if err := r.col.FindOne(ctx, filter).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apierr.NotFound("participant", userID)
		}
		return nil, apierr.Internal(err.Error())
	}
	p := participantDocToDomain(doc)
	return &p, nil
}

// ===================================================================
// PaymentInstallmentMongoRepository — implements portout.PaymentInstallmentRepository
// Queries the 'payment_installments' collection for finance domain use cases.
// ===================================================================

type installmentDocument struct {
	ID            interface{} `bson:"_id,omitempty"`
	EventID       bson.Binary `bson:"event_id"`
	UserID        interface{} `bson:"user_id,omitempty"`
	ParticipantID bson.Binary `bson:"participant_id,omitempty"`
	Amount        int64       `bson:"amount"`
	Status        string      `bson:"status"`
	PaymentMethod string      `bson:"payment_method,omitempty"`
	DueDate       time.Time   `bson:"due_date,omitempty"`
	PaidAt        *time.Time  `bson:"paid_at,omitempty"`
}

func installmentDocToDomain(doc installmentDocument) domain.PaymentInstallment {
	return domain.PaymentInstallment{
		ID:            rawIDToString(doc.ID),
		EventID:       uuidBinaryToString(doc.EventID),
		ParticipantID: uuidBinaryToString(doc.ParticipantID),
		Amount:        doc.Amount,
		Status:        doc.Status,
		PaymentMethod: doc.PaymentMethod,
		PaidAt:        doc.PaidAt,
	}
}

// PaymentInstallmentMongoRepository implements portout.PaymentInstallmentRepository.
type FinanceInstallmentMongoRepository struct {
	col *mongo.Collection
}

// NewPaymentInstallmentRepository creates a PaymentInstallmentRepository backed by MongoDB.
func NewFinanceInstallmentRepository(client *Client) portout.FinanceInstallmentRepository {
	return &FinanceInstallmentMongoRepository{col: client.Collection("payment_installments")}
}

// FindByEventID returns all installments for the given event.
func (r *FinanceInstallmentMongoRepository) FindByEventID(ctx context.Context, eventID string) ([]domain.PaymentInstallment, error) {
	filter := bson.D{{Key: "event_id", Value: UUIDStringToBinary(eventID)}}
	cur, err := r.col.Find(ctx, filter)
	if err != nil {
		return nil, apierr.Internal(err.Error())
	}
	defer cur.Close(ctx)

	var result []domain.PaymentInstallment
	for cur.Next(ctx) {
		var doc installmentDocument
		if err := cur.Decode(&doc); err != nil {
			return nil, apierr.Internal(err.Error())
		}
		result = append(result, installmentDocToDomain(doc))
	}
	return result, cur.Err()
}

// FindByParticipantID returns all installments for a participant in a specific event.
func (r *FinanceInstallmentMongoRepository) FindByParticipantID(ctx context.Context, eventID, participantID string) ([]domain.PaymentInstallment, error) {
	filter := bson.D{
		{Key: "event_id", Value: UUIDStringToBinary(eventID)},
		{Key: "participant_id", Value: UUIDStringToBinary(participantID)},
	}
	cur, err := r.col.Find(ctx, filter)
	if err != nil {
		return nil, apierr.Internal(err.Error())
	}
	defer cur.Close(ctx)

	var result []domain.PaymentInstallment
	for cur.Next(ctx) {
		var doc installmentDocument
		if err := cur.Decode(&doc); err != nil {
			return nil, apierr.Internal(err.Error())
		}
		result = append(result, installmentDocToDomain(doc))
	}
	return result, cur.Err()
}

// FindPendingByEventID returns all PENDING or OVERDUE installments for the given event.
func (r *FinanceInstallmentMongoRepository) FindPendingByEventID(ctx context.Context, eventID string) ([]domain.PaymentInstallment, error) {
	filter := bson.D{
		{Key: "event_id", Value: UUIDStringToBinary(eventID)},
		{Key: "status", Value: bson.D{{Key: "$in", Value: bson.A{"PENDING", "OVERDUE"}}}},
	}
	cur, err := r.col.Find(ctx, filter)
	if err != nil {
		return nil, apierr.Internal(err.Error())
	}
	defer cur.Close(ctx)

	var result []domain.PaymentInstallment
	for cur.Next(ctx) {
		var doc installmentDocument
		if err := cur.Decode(&doc); err != nil {
			return nil, apierr.Internal(err.Error())
		}
		result = append(result, installmentDocToDomain(doc))
	}
	return result, cur.Err()
}

// ===================================================================
// ParticipanteMongoRepository — write-side, implements portout.ParticipanteRepository
// ===================================================================

// FindParticipationIDsByUserID returns the _id (as UUID strings) of all participation
// records for the given user.
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
		{Key: "evento_id", Value: UUIDStringToBinary(eventID)},
		{Key: "usuario_id", Value: userIDValue(userID)},
	})
	if err != nil {
		return false, fmt.Errorf("is participant of event: %w", err)
	}
	return count > 0, nil
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
		{Key: "evento_id", Value: UUIDStringToBinary(eventoID)}, // eventoID is always UUID format from event Save
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
