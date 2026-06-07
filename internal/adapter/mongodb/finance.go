package mongodb

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/finance"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ===================================================================
// FinanceSummary — collection: finance_summaries
// Mirrors Java's FinanceSummary document schema.
// event_id is stored as UUID Binary subtype 4 (Java-compatible).
// ===================================================================

type financeSummaryDocument struct {
	ID                     interface{} `bson:"_id,omitempty"`
	EventID                bson.Binary `bson:"event_id"`
	Goal                   int64       `bson:"goal"`
	Collected              int64       `bson:"collected"`
	ProgressPercentage     float64     `bson:"progress_percentage"`
	AvailableForWithdrawal int64       `bson:"available_for_withdrawal"`
	PendingWithdrawals     int64       `bson:"pending_withdrawals"`
	LastCalculatedAt       time.Time   `bson:"last_calculated_at"`
}

func financeSummaryDocFromDomain(s *domain.FinanceSummary) financeSummaryDocument {
	return financeSummaryDocument{
		EventID:                UUIDStringToBinary(s.EventID),
		Goal:                   s.Goal,
		Collected:              s.Collected,
		ProgressPercentage:     s.ProgressPercentage,
		AvailableForWithdrawal: s.AvailableForWithdrawal,
		LastCalculatedAt:       s.LastCalculatedAt,
	}
}

func financeSummaryDocToDomain(doc financeSummaryDocument) *domain.FinanceSummary {
	return &domain.FinanceSummary{
		ID:                     rawIDToString(doc.ID),
		EventID:                uuidBinaryToString(doc.EventID),
		Goal:                   doc.Goal,
		Collected:              doc.Collected,
		ProgressPercentage:     doc.ProgressPercentage,
		AvailableForWithdrawal: doc.AvailableForWithdrawal,
		LastCalculatedAt:       doc.LastCalculatedAt,
	}
}

// ---- FinanceSummaryMongoRepository ----

// FinanceSummaryMongoRepository implements portout.FinanceSummaryRepository.
type FinanceSummaryMongoRepository struct {
	col *mongo.Collection
}

// NewFinanceSummaryRepository creates a FinanceSummaryRepository backed by MongoDB.
func NewFinanceSummaryRepository(client *Client) portout.FinanceSummaryRepository {
	return &FinanceSummaryMongoRepository{col: client.Collection("finance_summaries")}
}

// FindByEventID returns the financial summary for the given event ID (UUID string).
// Returns apierr.NotFound if no summary exists.
func (r *FinanceSummaryMongoRepository) FindByEventID(ctx context.Context, eventID string) (*domain.FinanceSummary, error) {
	filter := bson.D{{Key: "event_id", Value: UUIDStringToBinary(eventID)}}
	var doc financeSummaryDocument
	if err := r.col.FindOne(ctx, filter).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apierr.NotFound("finance_summary", eventID)
		}
		return nil, apierr.Internal(err.Error())
	}
	return financeSummaryDocToDomain(doc), nil
}

// Save inserts a new FinanceSummary document, generating a new ObjectID.
func (r *FinanceSummaryMongoRepository) Save(ctx context.Context, s *domain.FinanceSummary) (*domain.FinanceSummary, error) {
	doc := financeSummaryDocFromDomain(s)
	newOID := bson.NewObjectID()
	doc.ID = newOID
	if doc.LastCalculatedAt.IsZero() {
		doc.LastCalculatedAt = time.Now().UTC()
	}
	if _, err := r.col.InsertOne(ctx, doc); err != nil {
		return nil, apierr.Internal("saving finance_summary: " + err.Error())
	}
	s.ID = newOID.Hex()
	return s, nil
}

// Update patches the mutable fields of an existing summary and refreshes last_calculated_at.
func (r *FinanceSummaryMongoRepository) Update(ctx context.Context, s *domain.FinanceSummary) (*domain.FinanceSummary, error) {
	filter := parseIDToFilter(s.ID)
	now := time.Now().UTC()
	s.LastCalculatedAt = now
	setDoc := bson.D{
		{Key: "goal", Value: s.Goal},
		{Key: "collected", Value: s.Collected},
		{Key: "progress_percentage", Value: s.ProgressPercentage},
		{Key: "available_for_withdrawal", Value: s.AvailableForWithdrawal},
		{Key: "last_calculated_at", Value: now},
	}
	update := bson.D{{Key: "$set", Value: setDoc}}
	res, err := r.col.UpdateOne(ctx, filter, update)
	if err != nil {
		return nil, apierr.Internal(err.Error())
	}
	if res.MatchedCount == 0 {
		return nil, apierr.NotFound("finance_summary", s.ID)
	}
	return s, nil
}

// ===================================================================
// LedgerEntry — collection: ledger_entries
// Represents a single financial transaction in the event ledger.
// event_id stored as UUID Binary subtype 4.
// ===================================================================

type ledgerEntryDocument struct {
	ID              interface{} `bson:"_id,omitempty"`
	EventID         bson.Binary `bson:"event_id"`
	Type            string      `bson:"type"`
	Amount          int64       `bson:"amount"`
	Description     string      `bson:"description"`
	OccurredAt      time.Time   `bson:"occurred_at"`
	AccountingClass string      `bson:"accounting_classification"`
}

func ledgerEntryDocToDomain(doc ledgerEntryDocument) domain.LedgerEntry {
	return domain.LedgerEntry{
		ID:              rawIDToString(doc.ID),
		EventID:         uuidBinaryToString(doc.EventID),
		Type:            doc.Type,
		Amount:          doc.Amount,
		Description:     doc.Description,
		OccurredAt:      doc.OccurredAt,
		AccountingClass: doc.AccountingClass,
	}
}

// ---- LedgerEntryMongoRepository ----

// LedgerEntryMongoRepository implements portout.LedgerEntryRepository.
type LedgerEntryMongoRepository struct {
	col *mongo.Collection
}

// NewLedgerEntryRepository creates a LedgerEntryRepository backed by MongoDB.
func NewLedgerEntryRepository(client *Client) portout.LedgerEntryRepository {
	return &LedgerEntryMongoRepository{col: client.Collection("ledger_entries")}
}

// FindByEventID returns a paginated list of ledger entries for the given event.
// entryType, from, and to are optional filters. Results are sorted by occurred_at DESC.
func (r *LedgerEntryMongoRepository) FindByEventID(
	ctx context.Context,
	eventID string,
	entryType *string,
	from, to *time.Time,
	page, size int,
) ([]domain.LedgerEntry, int64, error) {
	filter := bson.D{{Key: "event_id", Value: UUIDStringToBinary(eventID)}}

	if entryType != nil {
		filter = append(filter, bson.E{Key: "type", Value: *entryType})
	}

	if from != nil || to != nil {
		rangeFilter := bson.D{}
		if from != nil {
			rangeFilter = append(rangeFilter, bson.E{Key: "$gte", Value: *from})
		}
		if to != nil {
			rangeFilter = append(rangeFilter, bson.E{Key: "$lte", Value: *to})
		}
		filter = append(filter, bson.E{Key: "occurred_at", Value: rangeFilter})
	}

	total, err := r.col.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, apierr.Internal(err.Error())
	}

	skip := int64((page - 1) * size)
	opts := options.Find().
		SetSort(bson.D{{Key: "occurred_at", Value: -1}}).
		SetSkip(skip).
		SetLimit(int64(size))

	cur, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, apierr.Internal(err.Error())
	}
	defer cur.Close(ctx)

	var result []domain.LedgerEntry
	for cur.Next(ctx) {
		var doc ledgerEntryDocument
		if err := cur.Decode(&doc); err != nil {
			return nil, 0, apierr.Internal(err.Error())
		}
		result = append(result, ledgerEntryDocToDomain(doc))
	}
	if err := cur.Err(); err != nil {
		return nil, 0, apierr.Internal(err.Error())
	}
	return result, total, nil
}

// ===================================================================
// PaymentAccount — collection: payment_accounts
// Holds a user's PIX or bank account for receiving payments.
// user_id stored as UUID Binary subtype 4 (Java-compatible).
// _id is flexible (interface{}) to accommodate both ObjectID (Go) and
// UUID Binary (Java) documents.
// ===================================================================

type paymentAccountDocument struct {
	ID         interface{} `bson:"_id,omitempty"`
	UserID     bson.Binary `bson:"user_id"`
	Type       string      `bson:"type"`
	PixKey     string      `bson:"pix_key,omitempty"`
	PixType    string      `bson:"pix_type,omitempty"`
	BankCode   string      `bson:"bank_code,omitempty"`
	AgencyNum  string      `bson:"agency_number,omitempty"`
	AccountNum string      `bson:"account_number,omitempty"`
	IsDefault  bool        `bson:"is_default"`
	Active     bool        `bson:"active"`
	CreatedAt  time.Time   `bson:"created_at"`
	UpdatedAt  time.Time   `bson:"updated_at"`
}

func paymentAccountDocFromDomain(a *domain.PaymentAccount) paymentAccountDocument {
	return paymentAccountDocument{
		UserID:     UUIDStringToBinary(a.UserID),
		Type:       a.Type,
		PixKey:     a.PixKey,
		PixType:    a.PixType,
		BankCode:   a.BankCode,
		AgencyNum:  a.AgencyNum,
		AccountNum: a.AccountNum,
		IsDefault:  a.IsDefault,
		Active:     a.Active,
		CreatedAt:  a.CreatedAt,
		UpdatedAt:  a.UpdatedAt,
	}
}

func paymentAccountDocToDomain(doc paymentAccountDocument) *domain.PaymentAccount {
	return &domain.PaymentAccount{
		ID:         rawIDToString(doc.ID),
		UserID:     uuidBinaryToString(doc.UserID),
		Type:       doc.Type,
		PixKey:     doc.PixKey,
		PixType:    doc.PixType,
		BankCode:   doc.BankCode,
		AgencyNum:  doc.AgencyNum,
		AccountNum: doc.AccountNum,
		IsDefault:  doc.IsDefault,
		Active:     doc.Active,
		CreatedAt:  doc.CreatedAt,
		UpdatedAt:  doc.UpdatedAt,
	}
}

// ---- PaymentAccountMongoRepository ----

// PaymentAccountMongoRepository implements portout.PaymentAccountRepository.
type PaymentAccountMongoRepository struct {
	col *mongo.Collection
}

// NewPaymentAccountRepository creates a PaymentAccountRepository backed by MongoDB.
func NewPaymentAccountRepository(client *Client) portout.PaymentAccountRepository {
	return &PaymentAccountMongoRepository{col: client.Collection("payment_accounts")}
}

// FindByUserID returns all active payment accounts for a given user.
// Accounts with active=false are excluded (soft-deleted accounts).
func (r *PaymentAccountMongoRepository) FindByUserID(ctx context.Context, userID string) ([]domain.PaymentAccount, error) {
	filter := bson.D{
		{Key: "user_id", Value: UUIDStringToBinary(userID)},
		{Key: "active", Value: bson.D{{Key: "$ne", Value: false}}},
	}
	cur, err := r.col.Find(ctx, filter)
	if err != nil {
		return nil, apierr.Internal(err.Error())
	}
	defer cur.Close(ctx)

	var result []domain.PaymentAccount
	for cur.Next(ctx) {
		var doc paymentAccountDocument
		if err := cur.Decode(&doc); err != nil {
			return nil, apierr.Internal(err.Error())
		}
		result = append(result, *paymentAccountDocToDomain(doc))
	}
	if err := cur.Err(); err != nil {
		return nil, apierr.Internal(err.Error())
	}
	return result, nil
}

// FindByID returns a single payment account by ID, verifying ownership via userID.
// Returns apierr.NotFound if the account does not exist or does not belong to the user.
func (r *PaymentAccountMongoRepository) FindByID(ctx context.Context, id, userID string) (*domain.PaymentAccount, error) {
	filter := parseIDToFilter(id)
	filter = append(filter, bson.E{Key: "user_id", Value: UUIDStringToBinary(userID)})

	var doc paymentAccountDocument
	if err := r.col.FindOne(ctx, filter).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apierr.NotFound("payment_account", id)
		}
		return nil, apierr.Internal(err.Error())
	}
	return paymentAccountDocToDomain(doc), nil
}

// Save inserts a new PaymentAccount, setting active=true and timestamps.
func (r *PaymentAccountMongoRepository) Save(ctx context.Context, a *domain.PaymentAccount) (*domain.PaymentAccount, error) {
	now := time.Now().UTC()
	if a.CreatedAt.IsZero() {
		a.CreatedAt = now
	}
	a.UpdatedAt = now
	a.Active = true

	doc := paymentAccountDocFromDomain(a)
	res, err := r.col.InsertOne(ctx, doc)
	if err != nil {
		return nil, apierr.Internal("saving payment_account: " + err.Error())
	}
	a.ID = rawIDToString(res.InsertedID)
	return a, nil
}

// Update patches the mutable fields of an existing account.
// The filter includes user_id to ensure ownership.
func (r *PaymentAccountMongoRepository) Update(ctx context.Context, a *domain.PaymentAccount) (*domain.PaymentAccount, error) {
	filter := parseIDToFilter(a.ID)
	filter = append(filter, bson.E{Key: "user_id", Value: UUIDStringToBinary(a.UserID)})

	now := time.Now().UTC()
	a.UpdatedAt = now
	setDoc := bson.D{
		{Key: "type", Value: a.Type},
		{Key: "pix_key", Value: a.PixKey},
		{Key: "pix_type", Value: a.PixType},
		{Key: "bank_code", Value: a.BankCode},
		{Key: "agency_number", Value: a.AgencyNum},
		{Key: "account_number", Value: a.AccountNum},
		{Key: "is_default", Value: a.IsDefault},
		{Key: "updated_at", Value: now},
	}
	update := bson.D{{Key: "$set", Value: setDoc}}
	res, err := r.col.UpdateOne(ctx, filter, update)
	if err != nil {
		return nil, apierr.Internal(err.Error())
	}
	if res.MatchedCount == 0 {
		return nil, apierr.NotFound("payment_account", a.ID)
	}
	return a, nil
}

// ClearDefault removes the is_default flag from ALL payment accounts belonging to userID.
// Call this before setting a new default account to avoid multiple defaults.
func (r *PaymentAccountMongoRepository) ClearDefault(ctx context.Context, userID string) error {
	filter := bson.D{{Key: "user_id", Value: UUIDStringToBinary(userID)}}
	update := bson.D{{Key: "$set", Value: bson.D{{Key: "is_default", Value: false}}}}
	if _, err := r.col.UpdateMany(ctx, filter, update); err != nil {
		return apierr.Internal(err.Error())
	}
	return nil
}

// SoftDelete marks the account as inactive (active=false).
// The filter includes user_id to ensure ownership.
func (r *PaymentAccountMongoRepository) SoftDelete(ctx context.Context, id, userID string) error {
	filter := parseIDToFilter(id)
	filter = append(filter, bson.E{Key: "user_id", Value: UUIDStringToBinary(userID)})

	update := bson.D{{Key: "$set", Value: bson.D{{Key: "active", Value: false}}}}
	res, err := r.col.UpdateOne(ctx, filter, update)
	if err != nil {
		return apierr.Internal(err.Error())
	}
	if res.MatchedCount == 0 {
		return apierr.NotFound("payment_account", id)
	}
	return nil
}
