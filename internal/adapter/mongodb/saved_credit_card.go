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
var _ portout.SavedCreditCardRepository = (*SavedCreditCardRepository)(nil)

const colSavedCreditCards = "saved_credit_cards"

// savedCreditCardDocument matches the saved_credit_cards collection schema (snake_case).
type savedCreditCardDocument struct {
	ID             string    `bson:"_id,omitempty"`
	UserID         string    `bson:"user_id"`
	LastFourDigits string    `bson:"last_four_digits"`
	Brand          string    `bson:"brand"`
	HolderName     string    `bson:"holder_name"`
	ExpirationDate string    `bson:"expiration_date"`
	TokenRef       string    `bson:"token_ref"`
	IsDefault      bool      `bson:"is_default"`
	IsActive       bool      `bson:"is_active"`
	CreatedAt      time.Time `bson:"created_at"`
	UpdatedAt      time.Time `bson:"updated_at"`
}

// SavedCreditCardRepository implements portout.SavedCreditCardRepository using MongoDB.
type SavedCreditCardRepository struct {
	col *mongo.Collection
}

// NewSavedCreditCardRepository creates a new SavedCreditCardRepository.
func NewSavedCreditCardRepository(client *Client) *SavedCreditCardRepository {
	return &SavedCreditCardRepository{col: client.Collection(colSavedCreditCards)}
}

// Save persists a new saved credit card. ID is generated if empty.
func (r *SavedCreditCardRepository) Save(ctx context.Context, card *domain.SavedCreditCard) error {
	if card.ID == "" {
		card.ID = uuid.New().String()
	}
	doc := savedCardToDoc(card)
	if _, err := r.col.InsertOne(ctx, doc); err != nil {
		return apierr.Internal(fmt.Sprintf("save credit card: %s", err.Error()))
	}
	return nil
}

// Update replaces an existing saved credit card document.
func (r *SavedCreditCardRepository) Update(ctx context.Context, card *domain.SavedCreditCard) error {
	card.UpdatedAt = time.Now()
	doc := savedCardToDoc(card)
	res, err := r.col.ReplaceOne(ctx, bson.D{{Key: "_id", Value: card.ID}}, doc)
	if err != nil {
		return apierr.Internal(fmt.Sprintf("update credit card: %s", err.Error()))
	}
	if res.MatchedCount == 0 {
		return apierr.NotFound("saved_credit_card", card.ID)
	}
	return nil
}

// FindByID retrieves a saved credit card by its ID.
func (r *SavedCreditCardRepository) FindByID(ctx context.Context, id string) (*domain.SavedCreditCard, error) {
	var doc savedCreditCardDocument
	if err := r.col.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apierr.NotFound("saved_credit_card", id)
		}
		return nil, apierr.Internal(fmt.Sprintf("find credit card by id: %s", err.Error()))
	}
	card := savedCardFromDoc(doc)
	return &card, nil
}

// FindByUserID retrieves all active saved credit cards for the user, default first.
func (r *SavedCreditCardRepository) FindByUserID(ctx context.Context, userID string) ([]*domain.SavedCreditCard, error) {
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
		return nil, apierr.Internal(fmt.Sprintf("find credit cards by user: %s", err.Error()))
	}
	defer cursor.Close(ctx)

	var results []*domain.SavedCreditCard
	for cursor.Next(ctx) {
		var doc savedCreditCardDocument
		if err := cursor.Decode(&doc); err != nil {
			return nil, apierr.Internal(fmt.Sprintf("decode credit card: %s", err.Error()))
		}
		card := savedCardFromDoc(doc)
		results = append(results, &card)
	}
	if err := cursor.Err(); err != nil {
		return nil, apierr.Internal(fmt.Sprintf("cursor credit cards: %s", err.Error()))
	}
	if results == nil {
		results = []*domain.SavedCreditCard{}
	}
	return results, nil
}

// FindDefaultByUserID retrieves the default active saved credit card for the user.
func (r *SavedCreditCardRepository) FindDefaultByUserID(ctx context.Context, userID string) (*domain.SavedCreditCard, error) {
	var doc savedCreditCardDocument
	filter := bson.D{
		{Key: "user_id", Value: userID},
		{Key: "is_default", Value: true},
		{Key: "is_active", Value: true},
	}
	if err := r.col.FindOne(ctx, filter).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apierr.NotFound("saved_credit_card (default)", userID)
		}
		return nil, apierr.Internal(fmt.Sprintf("find default credit card: %s", err.Error()))
	}
	card := savedCardFromDoc(doc)
	return &card, nil
}

// SetDefault atomically clears is_default on all cards for the user, then sets
// it on the target card.
func (r *SavedCreditCardRepository) SetDefault(ctx context.Context, userID, cardID string) error {
	now := time.Now()

	// Step 1: clear existing defaults for this user.
	if _, err := r.col.UpdateMany(ctx,
		bson.D{{Key: "user_id", Value: userID}},
		bson.D{{Key: "$set", Value: bson.D{
			{Key: "is_default", Value: false},
			{Key: "updated_at", Value: now},
		}}},
	); err != nil {
		return apierr.Internal(fmt.Sprintf("set default credit card (clear): %s", err.Error()))
	}

	// Step 2: mark the target as default.
	res, err := r.col.UpdateOne(ctx,
		bson.D{{Key: "_id", Value: cardID}},
		bson.D{{Key: "$set", Value: bson.D{
			{Key: "is_default", Value: true},
			{Key: "updated_at", Value: now},
		}}},
	)
	if err != nil {
		return apierr.Internal(fmt.Sprintf("set default credit card (set): %s", err.Error()))
	}
	if res.MatchedCount == 0 {
		return apierr.NotFound("saved_credit_card", cardID)
	}
	return nil
}

// DeleteByID soft-deletes a saved credit card by setting is_active = false.
func (r *SavedCreditCardRepository) DeleteByID(ctx context.Context, id string) error {
	res, err := r.col.UpdateOne(ctx,
		bson.D{{Key: "_id", Value: id}},
		bson.D{{Key: "$set", Value: bson.D{
			{Key: "is_active", Value: false},
			{Key: "updated_at", Value: time.Now()},
		}}},
	)
	if err != nil {
		return apierr.Internal(fmt.Sprintf("soft-delete credit card: %s", err.Error()))
	}
	if res.MatchedCount == 0 {
		return apierr.NotFound("saved_credit_card", id)
	}
	return nil
}

// ---- mapping helpers ----

func savedCardToDoc(card *domain.SavedCreditCard) savedCreditCardDocument {
	return savedCreditCardDocument{
		ID:             card.ID,
		UserID:         card.UserID,
		LastFourDigits: card.LastFourDigits,
		Brand:          string(card.Brand),
		HolderName:     card.HolderName,
		ExpirationDate: card.ExpirationDate,
		TokenRef:       card.TokenRef,
		IsDefault:      card.IsDefault,
		IsActive:       card.IsActive,
		CreatedAt:      card.CreatedAt,
		UpdatedAt:      card.UpdatedAt,
	}
}

func savedCardFromDoc(doc savedCreditCardDocument) domain.SavedCreditCard {
	return domain.SavedCreditCard{
		ID:             doc.ID,
		UserID:         doc.UserID,
		LastFourDigits: doc.LastFourDigits,
		Brand:          domain.CardBrand(doc.Brand),
		HolderName:     doc.HolderName,
		ExpirationDate: doc.ExpirationDate,
		TokenRef:       doc.TokenRef,
		IsDefault:      doc.IsDefault,
		IsActive:       doc.IsActive,
		CreatedAt:      doc.CreatedAt,
		UpdatedAt:      doc.UpdatedAt,
	}
}
