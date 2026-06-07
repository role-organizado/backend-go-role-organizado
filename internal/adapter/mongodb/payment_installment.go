package mongodb

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// Compile-time interface assertion.
var _ portout.PaymentInstallmentRepository = (*PaymentInstallmentRepository)(nil)

const colPaymentInstallments = "payment_installments"

// paymentInstallmentDocument matches the payment_installments collection schema.
// The Java uses snake_case throughout this collection.
type paymentInstallmentDocument struct {
	ID                      string     `bson:"_id,omitempty"`
	EventID                 string     `bson:"event_id"`
	LiabilityID             string     `bson:"liability_id,omitempty"`
	ParticipantID           string     `bson:"participant_id"`
	InstallmentNumber       int        `bson:"installment_number"`
	TotalInstallments       int        `bson:"total_installments"`
	AmountCents             int64      `bson:"amount_cents"`
	DueDate                 time.Time  `bson:"due_date"`
	Status                  string     `bson:"status"`
	TransactionID           string     `bson:"transaction_id,omitempty"`
	PaidAt                  *time.Time `bson:"paid_at,omitempty"`
	PaymentMethod           string     `bson:"payment_method,omitempty"`
	PaymentReference        string     `bson:"payment_reference,omitempty"`
	OverdueNotificationSent bool       `bson:"overdue_notification_sent"`
	CreatedAt               time.Time  `bson:"created_at"`
	UpdatedAt               time.Time  `bson:"updated_at"`
}

// PaymentInstallmentRepository implements portout.PaymentInstallmentRepository using MongoDB.
type PaymentInstallmentRepository struct {
	col *mongo.Collection
}

// NewPaymentInstallmentRepository creates a new PaymentInstallmentRepository.
func NewPaymentInstallmentRepository(client *Client) *PaymentInstallmentRepository {
	return &PaymentInstallmentRepository{col: client.Collection(colPaymentInstallments)}
}

// FindByID retrieves a single installment by its platform ID.
func (r *PaymentInstallmentRepository) FindByID(ctx context.Context, id string) (*domain.PaymentInstallment, error) {
	var doc paymentInstallmentDocument
	if err := r.col.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apierr.NotFound("payment_installment", id)
		}
		return nil, apierr.Internal(fmt.Sprintf("find installment by id: %s", err.Error()))
	}
	inst := installmentFromDoc(doc)
	return &inst, nil
}

// FindByEventAndParticipant returns all installments for a participant in an event,
// ordered by installment_number ascending.
func (r *PaymentInstallmentRepository) FindByEventAndParticipant(ctx context.Context, eventID, participantID string) ([]*domain.PaymentInstallment, error) {
	filter := bson.D{
		{Key: "event_id", Value: eventID},
		{Key: "participant_id", Value: participantID},
	}
	cursor, err := r.col.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "installment_number", Value: 1}}))
	if err != nil {
		return nil, apierr.Internal(fmt.Sprintf("find installments by event and participant: %s", err.Error()))
	}
	return installmentCursorToSlice(ctx, cursor)
}

// FindByUserOrParticipations retrieves installments owned by the user directly or via
// any of the given participation IDs. This fixes BUG5/spec-096 where the Java had to
// search both by userId and by participationIds to get the full set.
func (r *PaymentInstallmentRepository) FindByUserOrParticipations(ctx context.Context, userID string, participationIDs []string, statusFilter *domain.InstallmentStatus) ([]*domain.PaymentInstallment, error) {
	allIDs := make([]string, 0, len(participationIDs)+1)
	allIDs = append(allIDs, userID)
	allIDs = append(allIDs, participationIDs...)

	filter := bson.D{{Key: "participant_id", Value: bson.D{{Key: "$in", Value: allIDs}}}}
	if statusFilter != nil {
		filter = append(filter, bson.E{Key: "status", Value: string(*statusFilter)})
	}
	cursor, err := r.col.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "due_date", Value: 1}}))
	if err != nil {
		return nil, apierr.Internal(fmt.Sprintf("find installments by user or participations: %s", err.Error()))
	}
	return installmentCursorToSlice(ctx, cursor)
}

// FindByIDs fetches multiple installments by their IDs in a single query.
func (r *PaymentInstallmentRepository) FindByIDs(ctx context.Context, ids []string) ([]*domain.PaymentInstallment, error) {
	filter := bson.D{{Key: "_id", Value: bson.D{{Key: "$in", Value: ids}}}}
	cursor, err := r.col.Find(ctx, filter)
	if err != nil {
		return nil, apierr.Internal(fmt.Sprintf("find installments by ids: %s", err.Error()))
	}
	return installmentCursorToSlice(ctx, cursor)
}

// MarkPaidBatch atomically marks a set of installments as PAID in a single UpdateMany.
func (r *PaymentInstallmentRepository) MarkPaidBatch(ctx context.Context, ids []string, txID string, paidAt time.Time, method, reference string) error {
	now := time.Now()
	_, err := r.col.UpdateMany(ctx,
		bson.D{{Key: "_id", Value: bson.D{{Key: "$in", Value: ids}}}},
		bson.D{{Key: "$set", Value: bson.D{
			{Key: "status", Value: string(domain.InstallmentStatusPaid)},
			{Key: "transaction_id", Value: txID},
			{Key: "paid_at", Value: paidAt},
			{Key: "payment_method", Value: method},
			{Key: "payment_reference", Value: reference},
			{Key: "updated_at", Value: now},
		}}},
	)
	if err != nil {
		return fmt.Errorf("mark paid batch: %w", err)
	}
	return nil
}

// FindByEvent returns all installments for an event with an optional status filter,
// ordered by due_date ascending. Used by ListInstallments when no userId is provided.
func (r *PaymentInstallmentRepository) FindByEvent(ctx context.Context, eventID string, statusFilter *domain.InstallmentStatus) ([]*domain.PaymentInstallment, error) {
	filter := bson.D{{Key: "event_id", Value: eventID}}
	if statusFilter != nil {
		filter = append(filter, bson.E{Key: "status", Value: string(*statusFilter)})
	}
	cursor, err := r.col.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "due_date", Value: 1}}))
	if err != nil {
		return nil, apierr.Internal(fmt.Sprintf("find installments by event: %s", err.Error()))
	}
	return installmentCursorToSlice(ctx, cursor)
}

// CancelByParticipant cancels all PENDING and OVERDUE installments for a participant
// within an event. Returns the number of records updated.
func (r *PaymentInstallmentRepository) CancelByParticipant(ctx context.Context, eventID, participantID string) (int64, error) {
	result, err := r.col.UpdateMany(ctx,
		bson.D{
			{Key: "event_id", Value: eventID},
			{Key: "participant_id", Value: participantID},
			{Key: "status", Value: bson.D{{Key: "$in", Value: bson.A{
				string(domain.InstallmentStatusPending),
				string(domain.InstallmentStatusOverdue),
			}}}},
		},
		bson.D{{Key: "$set", Value: bson.D{
			{Key: "status", Value: string(domain.InstallmentStatusCancelled)},
			{Key: "updated_at", Value: time.Now()},
		}}},
	)
	if err != nil {
		return 0, fmt.Errorf("cancel installments by participant: %w", err)
	}
	return result.ModifiedCount, nil
}

// Save persists a new installment. ID is generated if empty.
func (r *PaymentInstallmentRepository) Save(ctx context.Context, inst *domain.PaymentInstallment) error {
	if inst.ID == "" {
		inst.ID = uuid.New().String()
	}
	doc := installmentToDoc(inst)
	if _, err := r.col.InsertOne(ctx, doc); err != nil {
		return apierr.Internal(fmt.Sprintf("save installment: %s", err.Error()))
	}
	return nil
}

// ---- internal helpers ----

func installmentCursorToSlice(ctx context.Context, cursor *mongo.Cursor) ([]*domain.PaymentInstallment, error) {
	defer cursor.Close(ctx)

	var results []*domain.PaymentInstallment
	for cursor.Next(ctx) {
		var doc paymentInstallmentDocument
		if err := cursor.Decode(&doc); err != nil {
			return nil, apierr.Internal(fmt.Sprintf("decode installment: %s", err.Error()))
		}
		inst := installmentFromDoc(doc)
		results = append(results, &inst)
	}
	if err := cursor.Err(); err != nil {
		return nil, apierr.Internal(fmt.Sprintf("cursor installments: %s", err.Error()))
	}
	if results == nil {
		results = []*domain.PaymentInstallment{}
	}
	return results, nil
}

// ---- mapping helpers ----

func installmentToDoc(inst *domain.PaymentInstallment) paymentInstallmentDocument {
	return paymentInstallmentDocument{
		ID:                      inst.ID,
		EventID:                 inst.EventID,
		LiabilityID:             inst.LiabilityID,
		ParticipantID:           inst.ParticipantID,
		InstallmentNumber:       inst.InstallmentNumber,
		TotalInstallments:       inst.TotalInstallments,
		AmountCents:             inst.AmountCents,
		DueDate:                 inst.DueDate,
		Status:                  string(inst.Status),
		TransactionID:           inst.TransactionID,
		PaidAt:                  inst.PaidAt,
		PaymentMethod:           inst.PaymentMethod,
		PaymentReference:        inst.PaymentReference,
		OverdueNotificationSent: inst.OverdueNotificationSent,
		CreatedAt:               inst.CreatedAt,
		UpdatedAt:               inst.UpdatedAt,
	}
}

func installmentFromDoc(doc paymentInstallmentDocument) domain.PaymentInstallment {
	return domain.PaymentInstallment{
		ID:                      doc.ID,
		EventID:                 doc.EventID,
		LiabilityID:             doc.LiabilityID,
		ParticipantID:           doc.ParticipantID,
		InstallmentNumber:       doc.InstallmentNumber,
		TotalInstallments:       doc.TotalInstallments,
		AmountCents:             doc.AmountCents,
		DueDate:                 doc.DueDate,
		Status:                  domain.InstallmentStatus(doc.Status),
		TransactionID:           doc.TransactionID,
		PaidAt:                  doc.PaidAt,
		PaymentMethod:           doc.PaymentMethod,
		PaymentReference:        doc.PaymentReference,
		OverdueNotificationSent: doc.OverdueNotificationSent,
		CreatedAt:               doc.CreatedAt,
		UpdatedAt:               doc.UpdatedAt,
	}
}
