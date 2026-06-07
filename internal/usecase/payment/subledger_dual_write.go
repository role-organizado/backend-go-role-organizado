package payment

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
)

// PaymentCommitmentWriter appends a payment commitment entry to the subledger.
// Mocked in tests; concrete implementation is SubledgerDualWriteService.
type PaymentCommitmentWriter interface {
	AppendPaymentCommitment(
		ctx context.Context,
		tx *domain.PaymentTransaction,
		fees domain.FeeCalculationResult,
		snapshot domain.FeePolicySnapshot,
	) error
}

// ledgerEntryDocument mirrors the ledger_entries collection schema shared with
// the Java SubledgerDualWriteService. Field names match the Java camelCase convention.
type ledgerEntryDocument struct {
	SourceEventID   string    `bson:"sourceEventId"`
	SourceType      string    `bson:"sourceType"`
	EntryType       string    `bson:"entryType"`
	CommittedDelta  int64     `bson:"committedDeltaCents"`
	Metadata        bson.M    `bson:"metadata"`
	CompetenceMonth string    `bson:"competenceMonth"` // YYYY-MM
	CreatedAt       time.Time `bson:"createdAt"`
}

// ledgerSnapshotEventDocument mirrors the ledger_snapshot_events collection schema.
type ledgerSnapshotEventDocument struct {
	ID          string    `bson:"_id"`
	LastEventID string    `bson:"lastEventId"`
	EventID     string    `bson:"eventId"`
	ProcessedAt time.Time `bson:"processedAt"`
	UpdatedAt   time.Time `bson:"updatedAt"`
}

// SubledgerDualWriteService writes PAYMENT_COMMITMENT ledger entries to the shared
// ledger_entries collection and keeps ledger_snapshot_events in sync.
//
// This mirrors the Java SubledgerDualWriteService for cross-service reconciliation:
//   - sourceEventId "payment:commitment:{idempotencyKey}" carries a unique-index guarantee;
//     duplicate insertion is treated as no-op (idempotent).
//   - ledger_snapshot_events is updated via upsert for the event's payment stream.
//
// Implements PaymentCommitmentWriter.
type SubledgerDualWriteService struct {
	ledgerEntries        *mongo.Collection
	ledgerSnapshotEvents *mongo.Collection
}

// NewSubledgerDualWriteService creates a new SubledgerDualWriteService.
// Pass mongoClient.Collection("ledger_entries") and
// mongoClient.Collection("ledger_snapshot_events") from cmd/server/main.go.
func NewSubledgerDualWriteService(
	ledgerEntriesCol *mongo.Collection,
	ledgerSnapshotEventsCol *mongo.Collection,
) *SubledgerDualWriteService {
	return &SubledgerDualWriteService{
		ledgerEntries:        ledgerEntriesCol,
		ledgerSnapshotEvents: ledgerSnapshotEventsCol,
	}
}

// AppendPaymentCommitment writes a PAYMENT_COMMITMENT entry to ledger_entries.
//
// The sourceEventId "payment:commitment:{idempotencyKey}" must be unique — the
// ledger_entries collection should have a unique sparse index on sourceEventId
// (created by the Java migration). A duplicate-key error is treated as a no-op.
//
// After writing the entry, ledger_snapshot_events is upserted to track the last
// processed event for the event's payment stream.
func (s *SubledgerDualWriteService) AppendPaymentCommitment(
	ctx context.Context,
	tx *domain.PaymentTransaction,
	fees domain.FeeCalculationResult,
	snapshot domain.FeePolicySnapshot,
) error {
	// Derive a stable idempotency key for the sourceEventId.
	key := tx.IdempotencyKey
	if key == "" {
		key = tx.ID
	}
	sourceEventID := "payment:commitment:" + key
	reconciliationKey := tx.EventID + ":" + tx.ID

	competenceMonth := tx.CreatedAt.Format("2006-01")

	entry := ledgerEntryDocument{
		SourceEventID:  sourceEventID,
		SourceType:     "PAYMENT",
		EntryType:      "PAYMENT_COMMITMENT",
		CommittedDelta: fees.GrossAmountCents,
		Metadata: bson.M{
			"transactionId":           tx.ID,
			"provider":                string(tx.Provider),
			"paymentMethod":           string(tx.PaymentMethod),
			"grossAmountCents":        fees.GrossAmountCents,
			"pspFeeAppliedCents":      fees.PspFeeAppliedCents,
			"platformFeeAppliedCents": fees.PlatformFeeAppliedCents,
			"netAmountCents":          fees.NetAmountCents,
			"feePolicySource":         snapshot.FeePolicySource,
			"reconciliationKey":       reconciliationKey,
		},
		CompetenceMonth: competenceMonth,
		CreatedAt:       tx.CreatedAt,
	}

	_, err := s.ledgerEntries.InsertOne(ctx, entry)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			slog.DebugContext(ctx, "subledger: commitment already exists, skipping (idempotent)",
				"sourceEventId", sourceEventID)
			return nil
		}
		return fmt.Errorf("subledger: insert ledger entry: %w", err)
	}

	// Upsert ledger_snapshot_events to track the latest commitment for this event.
	snapshotKey := "payment:" + tx.EventID
	now := time.Now()
	snapshotDoc := ledgerSnapshotEventDocument{
		ID:          snapshotKey,
		LastEventID: sourceEventID,
		EventID:     tx.EventID,
		ProcessedAt: now,
		UpdatedAt:   now,
	}
	_, err = s.ledgerSnapshotEvents.ReplaceOne(
		ctx,
		bson.D{{Key: "_id", Value: snapshotKey}},
		snapshotDoc,
		options.Replace().SetUpsert(true),
	)
	if err != nil {
		// Non-fatal: the ledger entry was already persisted; snapshot is best-effort.
		slog.WarnContext(ctx, "subledger: update snapshot event failed (non-fatal)",
			"snapshotKey", snapshotKey, "error", err)
	}

	slog.DebugContext(ctx, "subledger: payment commitment appended",
		"sourceEventId", sourceEventID,
		"committedDeltaCents", fees.GrossAmountCents,
		"netAmountCents", fees.NetAmountCents)

	return nil
}

// compile-time assertion: *SubledgerDualWriteService implements PaymentCommitmentWriter.
var _ PaymentCommitmentWriter = (*SubledgerDualWriteService)(nil)
