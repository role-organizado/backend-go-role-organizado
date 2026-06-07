package payment

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
)

// installmentAllocationDocument mirrors the installment_allocations collection schema
// shared with the Java service. All field names are snake_case to match the Java
// JPA/Mongo convention for this collection.
type installmentAllocationDocument struct {
	ID            string    `bson:"_id,omitempty"`
	InstallmentID string    `bson:"installment_id"`
	LiabilityID   string    `bson:"liability_id"`
	AmountCents   int64     `bson:"amount_cents"`
	ProportionPct float64   `bson:"proportion_pct"`
	CreatedAt     time.Time `bson:"created_at"`
}

// InstallmentAllocationService creates allocation records in the shared
// installment_allocations MongoDB collection when an installment is paid.
//
// Allocation rules (mirrors Java InstallmentAllocationService):
//   - installment.LiabilityID != "": one allocation at 100% to that liability.
//   - installment.LiabilityID == "": proportional allocation across sibling
//     installments (same event+participant) that have a LiabilityID set,
//     weighted by their AmountCents. The arithmetic remainder (rounding) is
//     added to the largest liability. Falls back to a self-allocation (installmentID
//     as liabilityID) when no sibling liabilities are found.
//
// Idempotent: a second call for the same installment is a silent no-op.
// Race-safe: concurrent duplicate inserts are caught as duplicate-key errors
// and treated as no-ops.
type InstallmentAllocationService struct {
	allocCol        *mongo.Collection
	installmentRepo portout.PaymentInstallmentRepository
}

// NewInstallmentAllocationService creates a new InstallmentAllocationService.
// Pass mongoClient.Collection("installment_allocations") from cmd/server/main.go.
func NewInstallmentAllocationService(
	allocCol *mongo.Collection,
	installmentRepo portout.PaymentInstallmentRepository,
) *InstallmentAllocationService {
	return &InstallmentAllocationService{
		allocCol:        allocCol,
		installmentRepo: installmentRepo,
	}
}

// Allocate creates allocation record(s) for the given paid installment.
//
// Pre-conditions (validated before writing):
//   - inst.Status == PAID
//   - inst.AmountCents > 0
//   - no existing allocation for inst.ID (idempotency guard via CountDocuments)
func (s *InstallmentAllocationService) Allocate(ctx context.Context, inst *domain.PaymentInstallment) error {
	if inst.Status != domain.InstallmentStatusPaid {
		return fmt.Errorf("installment allocation: installment %s is not PAID (got %s)", inst.ID, inst.Status)
	}
	if inst.AmountCents <= 0 {
		return fmt.Errorf("installment allocation: installment %s has non-positive amount %d", inst.ID, inst.AmountCents)
	}

	// Idempotency guard: skip if allocations already exist.
	count, err := s.allocCol.CountDocuments(ctx,
		bson.D{{Key: "installment_id", Value: inst.ID}},
		options.Count().SetLimit(1),
	)
	if err != nil {
		return fmt.Errorf("installment allocation: check existing: %w", err)
	}
	if count > 0 {
		slog.DebugContext(ctx, "installment allocation: already exists, skipping (idempotent)",
			"installmentID", inst.ID)
		return nil
	}

	if inst.LiabilityID != "" {
		// Simple case: single liability → 100%.
		return s.insertOne(ctx, inst.ID, inst.LiabilityID, inst.AmountCents, 100.0)
	}
	return s.allocateProportional(ctx, inst)
}

// allocateProportional distributes the installment amount proportionally across
// sibling installments (same event+participant) that have a LiabilityID set.
func (s *InstallmentAllocationService) allocateProportional(ctx context.Context, inst *domain.PaymentInstallment) error {
	siblings, err := s.installmentRepo.FindByEventAndParticipant(ctx, inst.EventID, inst.ParticipantID)
	if err != nil {
		return fmt.Errorf("installment allocation: find siblings: %w", err)
	}

	// Collect only siblings that carry a LiabilityID, excluding the current installment.
	var liabilities []*domain.PaymentInstallment
	for _, sib := range siblings {
		if sib.LiabilityID != "" && sib.ID != inst.ID {
			liabilities = append(liabilities, sib)
		}
	}

	if len(liabilities) == 0 {
		// No sibling liabilities found — self-allocation fallback.
		slog.WarnContext(ctx, "installment allocation: no sibling liabilities, using self-allocation",
			"installmentID", inst.ID, "eventID", inst.EventID)
		return s.insertOne(ctx, inst.ID, inst.ID, inst.AmountCents, 100.0)
	}

	var totalSiblingCents int64
	for _, lib := range liabilities {
		totalSiblingCents += lib.AmountCents
	}
	if totalSiblingCents == 0 {
		// Guard against divide-by-zero; allocate 100% to the first liability.
		return s.insertOne(ctx, inst.ID, liabilities[0].LiabilityID, inst.AmountCents, 100.0)
	}

	// Compute per-liability allocations. Track the index of the largest for remainder.
	type alloc struct {
		liabilityID string
		cents       int64
		pct         float64
	}
	allocs := make([]alloc, len(liabilities))
	var allocated int64
	var largestIdx int
	var largestCents int64

	for i, lib := range liabilities {
		proportion := float64(lib.AmountCents) / float64(totalSiblingCents)
		c := int64(math.Round(float64(inst.AmountCents) * proportion))
		allocs[i] = alloc{
			liabilityID: lib.LiabilityID,
			cents:       c,
			pct:         proportion * 100.0,
		}
		allocated += c

		if lib.AmountCents > largestCents {
			largestCents = lib.AmountCents
			largestIdx = i
		}
	}

	// Add rounding remainder to the largest liability.
	remainder := inst.AmountCents - allocated
	if remainder != 0 {
		allocs[largestIdx].cents += remainder
	}

	// Bulk-insert all allocation documents.
	docs := make([]any, len(allocs))
	for i, a := range allocs {
		docs[i] = installmentAllocationDocument{
			ID:            uuid.New().String(),
			InstallmentID: inst.ID,
			LiabilityID:   a.liabilityID,
			AmountCents:   a.cents,
			ProportionPct: a.pct,
			CreatedAt:     time.Now(),
		}
	}

	if _, insertErr := s.allocCol.InsertMany(ctx, docs); insertErr != nil {
		if mongo.IsDuplicateKeyError(insertErr) {
			// Concurrent insert by another goroutine — treat as no-op.
			slog.DebugContext(ctx, "installment allocation: concurrent insert detected, skipping",
				"installmentID", inst.ID)
			return nil
		}
		return fmt.Errorf("installment allocation: insert many: %w", insertErr)
	}
	return nil
}

// insertOne persists a single allocation record for the given installment.
func (s *InstallmentAllocationService) insertOne(
	ctx context.Context,
	installmentID, liabilityID string,
	amountCents int64,
	pct float64,
) error {
	doc := installmentAllocationDocument{
		ID:            uuid.New().String(),
		InstallmentID: installmentID,
		LiabilityID:   liabilityID,
		AmountCents:   amountCents,
		ProportionPct: pct,
		CreatedAt:     time.Now(),
	}
	if _, err := s.allocCol.InsertOne(ctx, doc); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return nil // idempotent
		}
		return fmt.Errorf("installment allocation: insert: %w", err)
	}
	return nil
}
