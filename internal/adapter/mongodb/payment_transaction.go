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
var _ portout.PaymentTransactionRepository = (*PaymentTransactionRepository)(nil)

const colPaymentTransactions = "payment_transactions"

// paymentMetadataDocument matches the nested metadata object stored in payment_transactions.
// The Java stores these as camelCase fields inside the metadata subdocument.
type paymentMetadataDocument struct {
	PixQrCodeImage         string     `bson:"pixQrCodeImage,omitempty"`
	PixQrCodeText          string     `bson:"pixQrCodeText,omitempty"`
	PixKey                 string     `bson:"pixKey,omitempty"`
	PixExpiresAt           *time.Time `bson:"pixExpiresAt,omitempty"`
	BoletoCode             string     `bson:"boletoCode,omitempty"`
	BoletoDigitableLine    string     `bson:"boletoDigitableLine,omitempty"`
	BoletoPdfUrl           string     `bson:"boletoPdfUrl,omitempty"`
	BoletoDueDate          *time.Time `bson:"boletoDueDate,omitempty"`
	CardLast4              string     `bson:"cardLast4,omitempty"`
	CardBrand              string     `bson:"cardBrand,omitempty"`
	CreditCardInstallments int        `bson:"creditCardInstallments,omitempty"`
	InstallmentAmountCents int64      `bson:"installmentAmountCents,omitempty"`
	TokenizedCard          string     `bson:"tokenizedCard,omitempty"`
	InvoiceUrl             string     `bson:"invoiceUrl,omitempty"`
	BankSlipUrl            string     `bson:"bankSlipUrl,omitempty"`
	Provider               string     `bson:"provider,omitempty"`
	BillingType            string     `bson:"billingType,omitempty"`
}

// paymentTransactionDocument matches the payment_transactions collection schema.
// IMPORTANT: the Java uses camelCase for most fields but snake_case for amount_cents.
type paymentTransactionDocument struct {
	ID                          string                  `bson:"_id,omitempty"`
	UserID                      string                  `bson:"userId"`
	EventID                     string                  `bson:"eventId,omitempty"`
	InstallmentIDs              []string                `bson:"installmentIds,omitempty"`
	AmountCents                 int64                   `bson:"amount_cents"`
	Currency                    string                  `bson:"currency,omitempty"`
	PaymentMethod               string                  `bson:"paymentMethod"`
	Provider                    string                  `bson:"provider"`
	Status                      string                  `bson:"status"`
	IdempotencyKey              string                  `bson:"idempotencyKey,omitempty"`
	ProviderTransactionID       string                  `bson:"providerTransactionId,omitempty"`
	Metadata                    paymentMetadataDocument `bson:"metadata,omitempty"`
	FeePolicySnapshotVersion    string                  `bson:"feePolicySnapshotVersion,omitempty"`
	FeePolicySnapshotCapturedAt *time.Time              `bson:"feePolicySnapshotCapturedAt,omitempty"`
	CreatedAt                   time.Time               `bson:"createdAt"`
	UpdatedAt                   time.Time               `bson:"updatedAt"`
	CompletedAt                 *time.Time              `bson:"completedAt,omitempty"`
	ExpiresAt                   *time.Time              `bson:"expiresAt,omitempty"`
	FailureReason               string                  `bson:"failureReason,omitempty"`
}

// PaymentTransactionRepository implements portout.PaymentTransactionRepository using MongoDB.
type PaymentTransactionRepository struct {
	col *mongo.Collection
}

// NewPaymentTransactionRepository creates a new PaymentTransactionRepository.
func NewPaymentTransactionRepository(client *Client) *PaymentTransactionRepository {
	return &PaymentTransactionRepository{col: client.Collection(colPaymentTransactions)}
}

// Save persists a new transaction. If a duplicate-key error occurs on idempotencyKey,
// the existing transaction is looked up and its data is copied back into tx (Java-compatible
// idempotency behaviour). The caller must pre-populate tx.ID.
func (r *PaymentTransactionRepository) Save(ctx context.Context, tx *domain.PaymentTransaction) error {
	if tx.ID == "" {
		tx.ID = uuid.New().String()
	}
	doc := paymentTxToDoc(tx)
	if _, err := r.col.InsertOne(ctx, doc); err != nil {
		if mongo.IsDuplicateKeyError(err) && tx.IdempotencyKey != "" {
			existing, findErr := r.FindByIdempotencyKey(ctx, tx.IdempotencyKey)
			if findErr != nil {
				return fmt.Errorf("save payment transaction (idempotency lookup): %w", findErr)
			}
			// Copy existing data back so the caller sees the idempotent result.
			*tx = *existing
			return nil
		}
		return apierr.Internal(fmt.Sprintf("save payment transaction: %s", err.Error()))
	}
	return nil
}

// Update replaces an existing transaction document.
func (r *PaymentTransactionRepository) Update(ctx context.Context, tx *domain.PaymentTransaction) error {
	tx.UpdatedAt = time.Now()
	doc := paymentTxToDoc(tx)
	res, err := r.col.ReplaceOne(ctx, bson.D{{Key: "_id", Value: tx.ID}}, doc)
	if err != nil {
		return apierr.Internal(fmt.Sprintf("update payment transaction: %s", err.Error()))
	}
	if res.MatchedCount == 0 {
		return apierr.NotFound("payment_transaction", tx.ID)
	}
	return nil
}

// FindByID retrieves a transaction by its platform ID.
func (r *PaymentTransactionRepository) FindByID(ctx context.Context, id string) (*domain.PaymentTransaction, error) {
	var doc paymentTransactionDocument
	if err := r.col.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apierr.NotFound("payment_transaction", id)
		}
		return nil, apierr.Internal(fmt.Sprintf("find payment transaction by id: %s", err.Error()))
	}
	tx := paymentTxFromDoc(doc)
	return &tx, nil
}

// FindByIdempotencyKey looks up a transaction by its idempotency key.
func (r *PaymentTransactionRepository) FindByIdempotencyKey(ctx context.Context, key string) (*domain.PaymentTransaction, error) {
	var doc paymentTransactionDocument
	if err := r.col.FindOne(ctx, bson.D{{Key: "idempotencyKey", Value: key}}).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apierr.NotFound("payment_transaction", key)
		}
		return nil, apierr.Internal(fmt.Sprintf("find payment transaction by idempotency key: %s", err.Error()))
	}
	tx := paymentTxFromDoc(doc)
	return &tx, nil
}

// FindByProviderTransactionID looks up a transaction by the provider's ID.
// Falls back to searching inside the installmentIds array (Java-compatible
// externalReference/installment reference fallback).
func (r *PaymentTransactionRepository) FindByProviderTransactionID(ctx context.Context, providerTxID string) (*domain.PaymentTransaction, error) {
	// Primary lookup: exact match on providerTransactionId field.
	var doc paymentTransactionDocument
	err := r.col.FindOne(ctx, bson.D{{Key: "providerTransactionId", Value: providerTxID}}).Decode(&doc)
	if err == nil {
		tx := paymentTxFromDoc(doc)
		return &tx, nil
	}
	if err != mongo.ErrNoDocuments {
		return nil, apierr.Internal(fmt.Sprintf("find payment transaction by provider id: %s", err.Error()))
	}

	// Fallback: search inside installmentIds array (Java stores Asaas installment
	// references here when processing split payments).
	err = r.col.FindOne(ctx, bson.D{{Key: "installmentIds", Value: providerTxID}}).Decode(&doc)
	if err == nil {
		tx := paymentTxFromDoc(doc)
		return &tx, nil
	}
	if err != mongo.ErrNoDocuments {
		return nil, apierr.Internal(fmt.Sprintf("find payment transaction by installment ref: %s", err.Error()))
	}

	return nil, apierr.NotFound("payment_transaction", providerTxID)
}

// FindByUserID returns paginated transactions for a user with optional filters,
// ordered by createdAt descending.
func (r *PaymentTransactionRepository) FindByUserID(ctx context.Context, userID string, filter portout.TransactionFilter) ([]*domain.PaymentTransaction, int64, error) {
	query := bson.D{{Key: "userId", Value: userID}}

	if filter.Status != nil {
		query = append(query, bson.E{Key: "status", Value: string(*filter.Status)})
	}
	if filter.EventoID != "" {
		query = append(query, bson.E{Key: "eventId", Value: filter.EventoID})
	}
	if filter.From != nil || filter.To != nil {
		dateFilter := bson.D{}
		if filter.From != nil {
			dateFilter = append(dateFilter, bson.E{Key: "$gte", Value: *filter.From})
		}
		if filter.To != nil {
			dateFilter = append(dateFilter, bson.E{Key: "$lte", Value: *filter.To})
		}
		query = append(query, bson.E{Key: "createdAt", Value: dateFilter})
	}

	total, err := r.col.CountDocuments(ctx, query)
	if err != nil {
		return nil, 0, apierr.Internal(fmt.Sprintf("count payment transactions: %s", err.Error()))
	}

	findOpts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}})
	if filter.PageSize > 0 {
		findOpts.SetLimit(int64(filter.PageSize))
		if filter.Page > 1 {
			findOpts.SetSkip(int64((filter.Page - 1) * filter.PageSize))
		}
	}

	cursor, err := r.col.Find(ctx, query, findOpts)
	if err != nil {
		return nil, 0, apierr.Internal(fmt.Sprintf("find payment transactions by user: %s", err.Error()))
	}
	defer cursor.Close(ctx)

	var results []*domain.PaymentTransaction
	for cursor.Next(ctx) {
		var doc paymentTransactionDocument
		if err := cursor.Decode(&doc); err != nil {
			return nil, 0, apierr.Internal(fmt.Sprintf("decode payment transaction: %s", err.Error()))
		}
		tx := paymentTxFromDoc(doc)
		results = append(results, &tx)
	}
	if err := cursor.Err(); err != nil {
		return nil, 0, apierr.Internal(fmt.Sprintf("cursor payment transactions: %s", err.Error()))
	}
	if results == nil {
		results = []*domain.PaymentTransaction{}
	}
	return results, total, nil
}

// FindPendingOlderThan returns PENDING and PROCESSING transactions created before
// the given threshold, used by expiration and reconciliation workflows.
func (r *PaymentTransactionRepository) FindPendingOlderThan(ctx context.Context, threshold time.Time) ([]*domain.PaymentTransaction, error) {
	filter := bson.D{
		{Key: "status", Value: bson.D{{Key: "$in", Value: bson.A{
			string(domain.TransactionStatusPending),
			string(domain.TransactionStatusProcessing),
		}}}},
		{Key: "createdAt", Value: bson.D{{Key: "$lt", Value: threshold}}},
	}
	cursor, err := r.col.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}}))
	if err != nil {
		return nil, apierr.Internal(fmt.Sprintf("find pending transactions: %s", err.Error()))
	}
	defer cursor.Close(ctx)

	var results []*domain.PaymentTransaction
	for cursor.Next(ctx) {
		var doc paymentTransactionDocument
		if err := cursor.Decode(&doc); err != nil {
			return nil, apierr.Internal(fmt.Sprintf("decode pending transaction: %s", err.Error()))
		}
		tx := paymentTxFromDoc(doc)
		results = append(results, &tx)
	}
	if err := cursor.Err(); err != nil {
		return nil, apierr.Internal(fmt.Sprintf("cursor pending transactions: %s", err.Error()))
	}
	if results == nil {
		results = []*domain.PaymentTransaction{}
	}
	return results, nil
}

// FindCompletedByEventID returns COMPLETED transactions whose completedAt OR createdAt
// (fallback when completedAt is nil) is >= since. The OR semantics on completedAt|createdAt
// mirror the Java PricingPspReviewService 30-day lookback rule.
func (r *PaymentTransactionRepository) FindCompletedByEventID(ctx context.Context, eventID string, since time.Time) ([]*domain.PaymentTransaction, error) {
	filter := bson.D{
		{Key: "eventId", Value: eventID},
		{Key: "status", Value: string(domain.TransactionStatusCompleted)},
		{Key: "$or", Value: bson.A{
			bson.D{{Key: "completedAt", Value: bson.D{{Key: "$gte", Value: since}}}},
			bson.D{
				{Key: "completedAt", Value: nil},
				{Key: "createdAt", Value: bson.D{{Key: "$gte", Value: since}}},
			},
		}},
	}
	cursor, err := r.col.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}}))
	if err != nil {
		return nil, apierr.Internal(fmt.Sprintf("find completed transactions by event: %s", err.Error()))
	}
	defer cursor.Close(ctx)

	var results []*domain.PaymentTransaction
	for cursor.Next(ctx) {
		var doc paymentTransactionDocument
		if err := cursor.Decode(&doc); err != nil {
			return nil, apierr.Internal(fmt.Sprintf("decode completed transaction: %s", err.Error()))
		}
		tx := paymentTxFromDoc(doc)
		results = append(results, &tx)
	}
	if err := cursor.Err(); err != nil {
		return nil, apierr.Internal(fmt.Sprintf("cursor completed transactions: %s", err.Error()))
	}
	if results == nil {
		results = []*domain.PaymentTransaction{}
	}
	return results, nil
}

// ---- mapping helpers ----

func paymentTxToDoc(tx *domain.PaymentTransaction) paymentTransactionDocument {
	return paymentTransactionDocument{
		ID:                          tx.ID,
		UserID:                      tx.UserID,
		EventID:                     tx.EventID,
		InstallmentIDs:              tx.InstallmentIDs,
		AmountCents:                 tx.AmountCents,
		Currency:                    tx.Currency,
		PaymentMethod:               string(tx.PaymentMethod),
		Provider:                    string(tx.Provider),
		Status:                      string(tx.Status),
		IdempotencyKey:              tx.IdempotencyKey,
		ProviderTransactionID:       tx.ProviderTransactionID,
		Metadata:                    paymentMetaToDoc(tx.Metadata),
		FeePolicySnapshotVersion:    tx.FeePolicySnapshotVersion,
		FeePolicySnapshotCapturedAt: tx.FeePolicySnapshotCapturedAt,
		CreatedAt:                   tx.CreatedAt,
		UpdatedAt:                   tx.UpdatedAt,
		CompletedAt:                 tx.CompletedAt,
		ExpiresAt:                   tx.ExpiresAt,
		FailureReason:               tx.FailureReason,
	}
}

func paymentTxFromDoc(doc paymentTransactionDocument) domain.PaymentTransaction {
	return domain.PaymentTransaction{
		ID:                          doc.ID,
		UserID:                      doc.UserID,
		EventID:                     doc.EventID,
		InstallmentIDs:              doc.InstallmentIDs,
		AmountCents:                 doc.AmountCents,
		Currency:                    doc.Currency,
		PaymentMethod:               domain.PaymentMethod(doc.PaymentMethod),
		Provider:                    domain.PaymentProvider(doc.Provider),
		Status:                      domain.TransactionStatus(doc.Status),
		IdempotencyKey:              doc.IdempotencyKey,
		ProviderTransactionID:       doc.ProviderTransactionID,
		Metadata:                    paymentMetaFromDoc(doc.Metadata),
		FeePolicySnapshotVersion:    doc.FeePolicySnapshotVersion,
		FeePolicySnapshotCapturedAt: doc.FeePolicySnapshotCapturedAt,
		CreatedAt:                   doc.CreatedAt,
		UpdatedAt:                   doc.UpdatedAt,
		CompletedAt:                 doc.CompletedAt,
		ExpiresAt:                   doc.ExpiresAt,
		FailureReason:               doc.FailureReason,
	}
}

func paymentMetaToDoc(m domain.PaymentMetadata) paymentMetadataDocument {
	return paymentMetadataDocument{
		PixQrCodeImage:         m.PixQrCodeImage,
		PixQrCodeText:          m.PixQrCodeText,
		PixKey:                 m.PixKey,
		PixExpiresAt:           m.PixExpiresAt,
		BoletoCode:             m.BoletoCode,
		BoletoDigitableLine:    m.BoletoDigitableLine,
		BoletoPdfUrl:           m.BoletoPdfUrl,
		BoletoDueDate:          m.BoletoDueDate,
		CardLast4:              m.CardLast4,
		CardBrand:              m.CardBrand,
		CreditCardInstallments: m.CreditCardInstallments,
		InstallmentAmountCents: m.InstallmentAmountCents,
		TokenizedCard:          m.TokenizedCard,
		InvoiceUrl:             m.InvoiceUrl,
		BankSlipUrl:            m.BankSlipUrl,
		Provider:               m.Provider,
		BillingType:            m.BillingType,
	}
}

func paymentMetaFromDoc(doc paymentMetadataDocument) domain.PaymentMetadata {
	return domain.PaymentMetadata{
		PixQrCodeImage:         doc.PixQrCodeImage,
		PixQrCodeText:          doc.PixQrCodeText,
		PixKey:                 doc.PixKey,
		PixExpiresAt:           doc.PixExpiresAt,
		BoletoCode:             doc.BoletoCode,
		BoletoDigitableLine:    doc.BoletoDigitableLine,
		BoletoPdfUrl:           doc.BoletoPdfUrl,
		BoletoDueDate:          doc.BoletoDueDate,
		CardLast4:              doc.CardLast4,
		CardBrand:              doc.CardBrand,
		CreditCardInstallments: doc.CreditCardInstallments,
		InstallmentAmountCents: doc.InstallmentAmountCents,
		TokenizedCard:          doc.TokenizedCard,
		InvoiceUrl:             doc.InvoiceUrl,
		BankSlipUrl:            doc.BankSlipUrl,
		Provider:               doc.Provider,
		BillingType:            doc.BillingType,
	}
}
