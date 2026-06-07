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
var _ portout.PaymentAccountRepository = (*PaymentAccountRepository)(nil)

const colPaymentAccounts = "payment_accounts"

// paymentAccountDocument matches the payment_accounts collection schema (snake_case).
type paymentAccountDocument struct {
	ID                    string    `bson:"_id,omitempty"`
	UserID                string    `bson:"user_id"`
	AccountType           string    `bson:"account_type"`
	PixKeyType            string    `bson:"pix_key_type,omitempty"`
	PixKey                string    `bson:"pix_key,omitempty"`
	BankCode              string    `bson:"bank_code,omitempty"`
	BankName              string    `bson:"bank_name,omitempty"`
	Agency                string    `bson:"agency,omitempty"`
	AccountNumber         string    `bson:"account_number,omitempty"`
	AccountDigit          string    `bson:"account_digit,omitempty"`
	AccountHolderName     string    `bson:"account_holder_name,omitempty"`
	AccountHolderDocument string    `bson:"account_holder_document,omitempty"`
	IsDefault             bool      `bson:"is_default"`
	IsActive              bool      `bson:"is_active"`
	CreatedAt             time.Time `bson:"created_at"`
	UpdatedAt             time.Time `bson:"updated_at"`
}

// PaymentAccountRepository implements portout.PaymentAccountRepository using MongoDB.
type PaymentAccountRepository struct {
	col *mongo.Collection
}

// NewPaymentAccountRepository creates a new PaymentAccountRepository.
func NewPaymentAccountRepository(client *Client) *PaymentAccountRepository {
	return &PaymentAccountRepository{col: client.Collection(colPaymentAccounts)}
}

// Save persists a new payment account. ID is generated if empty.
func (r *PaymentAccountRepository) Save(ctx context.Context, acct *domain.PaymentAccount) error {
	if acct.ID == "" {
		acct.ID = uuid.New().String()
	}
	doc := paymentAccountToDoc(acct)
	if _, err := r.col.InsertOne(ctx, doc); err != nil {
		return apierr.Internal(fmt.Sprintf("save payment account: %s", err.Error()))
	}
	return nil
}

// Update replaces an existing payment account document.
func (r *PaymentAccountRepository) Update(ctx context.Context, acct *domain.PaymentAccount) error {
	acct.UpdatedAt = time.Now()
	doc := paymentAccountToDoc(acct)
	res, err := r.col.ReplaceOne(ctx, bson.D{{Key: "_id", Value: acct.ID}}, doc)
	if err != nil {
		return apierr.Internal(fmt.Sprintf("update payment account: %s", err.Error()))
	}
	if res.MatchedCount == 0 {
		return apierr.NotFound("payment_account", acct.ID)
	}
	return nil
}

// FindByID retrieves a payment account by its ID.
func (r *PaymentAccountRepository) FindByID(ctx context.Context, id string) (*domain.PaymentAccount, error) {
	var doc paymentAccountDocument
	if err := r.col.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apierr.NotFound("payment_account", id)
		}
		return nil, apierr.Internal(fmt.Sprintf("find payment account by id: %s", err.Error()))
	}
	acct := paymentAccountFromDoc(doc)
	return &acct, nil
}

// FindByUserID retrieves all active payment accounts for the user, ordered by
// is_default desc so the default account comes first.
func (r *PaymentAccountRepository) FindByUserID(ctx context.Context, userID string) ([]*domain.PaymentAccount, error) {
	filter := bson.D{
		{Key: "user_id", Value: userID},
		{Key: "is_active", Value: true},
	}
	cursor, err := r.col.Find(ctx, filter,
		options.Find().SetSort(bson.D{
			{Key: "is_default", Value: -1},
			{Key: "created_at", Value: 1},
		}),
	)
	if err != nil {
		return nil, apierr.Internal(fmt.Sprintf("find payment accounts by user: %s", err.Error()))
	}
	defer cursor.Close(ctx)

	var results []*domain.PaymentAccount
	for cursor.Next(ctx) {
		var doc paymentAccountDocument
		if err := cursor.Decode(&doc); err != nil {
			return nil, apierr.Internal(fmt.Sprintf("decode payment account: %s", err.Error()))
		}
		acct := paymentAccountFromDoc(doc)
		results = append(results, &acct)
	}
	if err := cursor.Err(); err != nil {
		return nil, apierr.Internal(fmt.Sprintf("cursor payment accounts: %s", err.Error()))
	}
	if results == nil {
		results = []*domain.PaymentAccount{}
	}
	return results, nil
}

// FindDefaultByUserID retrieves the default active payment account for the user.
func (r *PaymentAccountRepository) FindDefaultByUserID(ctx context.Context, userID string) (*domain.PaymentAccount, error) {
	var doc paymentAccountDocument
	filter := bson.D{
		{Key: "user_id", Value: userID},
		{Key: "is_default", Value: true},
		{Key: "is_active", Value: true},
	}
	if err := r.col.FindOne(ctx, filter).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apierr.NotFound("payment_account (default)", userID)
		}
		return nil, apierr.Internal(fmt.Sprintf("find default payment account: %s", err.Error()))
	}
	acct := paymentAccountFromDoc(doc)
	return &acct, nil
}

// SetDefault atomically clears is_default on all accounts for the user, then sets
// it on the target account (two-step update — idempotent and race-safe enough for
// this use case).
func (r *PaymentAccountRepository) SetDefault(ctx context.Context, userID, accountID string) error {
	now := time.Now()

	// Step 1: clear existing defaults for this user.
	if _, err := r.col.UpdateMany(ctx,
		bson.D{{Key: "user_id", Value: userID}},
		bson.D{{Key: "$set", Value: bson.D{
			{Key: "is_default", Value: false},
			{Key: "updated_at", Value: now},
		}}},
	); err != nil {
		return apierr.Internal(fmt.Sprintf("set default payment account (clear): %s", err.Error()))
	}

	// Step 2: mark the target as default.
	res, err := r.col.UpdateOne(ctx,
		bson.D{{Key: "_id", Value: accountID}},
		bson.D{{Key: "$set", Value: bson.D{
			{Key: "is_default", Value: true},
			{Key: "updated_at", Value: now},
		}}},
	)
	if err != nil {
		return apierr.Internal(fmt.Sprintf("set default payment account (set): %s", err.Error()))
	}
	if res.MatchedCount == 0 {
		return apierr.NotFound("payment_account", accountID)
	}
	return nil
}

// DeleteByID soft-deletes a payment account by setting is_active = false.
func (r *PaymentAccountRepository) DeleteByID(ctx context.Context, id string) error {
	res, err := r.col.UpdateOne(ctx,
		bson.D{{Key: "_id", Value: id}},
		bson.D{{Key: "$set", Value: bson.D{
			{Key: "is_active", Value: false},
			{Key: "updated_at", Value: time.Now()},
		}}},
	)
	if err != nil {
		return apierr.Internal(fmt.Sprintf("soft-delete payment account: %s", err.Error()))
	}
	if res.MatchedCount == 0 {
		return apierr.NotFound("payment_account", id)
	}
	return nil
}

// ---- mapping helpers ----

func paymentAccountToDoc(acct *domain.PaymentAccount) paymentAccountDocument {
	return paymentAccountDocument{
		ID:                    acct.ID,
		UserID:                acct.UserID,
		AccountType:           string(acct.AccountType),
		PixKeyType:            string(acct.PixKeyType),
		PixKey:                acct.PixKey,
		BankCode:              acct.BankCode,
		BankName:              acct.BankName,
		Agency:                acct.Agency,
		AccountNumber:         acct.AccountNumber,
		AccountDigit:          acct.AccountDigit,
		AccountHolderName:     acct.AccountHolderName,
		AccountHolderDocument: acct.AccountHolderDocument,
		IsDefault:             acct.IsDefault,
		IsActive:              acct.IsActive,
		CreatedAt:             acct.CreatedAt,
		UpdatedAt:             acct.UpdatedAt,
	}
}

func paymentAccountFromDoc(doc paymentAccountDocument) domain.PaymentAccount {
	return domain.PaymentAccount{
		ID:                    doc.ID,
		UserID:                doc.UserID,
		AccountType:           domain.AccountType(doc.AccountType),
		PixKeyType:            domain.PixKeyType(doc.PixKeyType),
		PixKey:                doc.PixKey,
		BankCode:              doc.BankCode,
		BankName:              doc.BankName,
		Agency:                doc.Agency,
		AccountNumber:         doc.AccountNumber,
		AccountDigit:          doc.AccountDigit,
		AccountHolderName:     doc.AccountHolderName,
		AccountHolderDocument: doc.AccountHolderDocument,
		IsDefault:             doc.IsDefault,
		IsActive:              doc.IsActive,
		CreatedAt:             doc.CreatedAt,
		UpdatedAt:             doc.UpdatedAt,
	}
}
