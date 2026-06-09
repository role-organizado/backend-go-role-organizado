package mongodb

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// Compile-time interface assertion.
var _ portout.AsaasCustomerLinkRepository = (*AsaasCustomerLinkRepository)(nil)

const colAsaasCustomerLinks = "asaas_customer_links"

// asaasCustomerLinkDocument matches the asaas_customer_links schema.
// _id equals the platform userID for O(1) lookup — no secondary index needed.
type asaasCustomerLinkDocument struct {
	UserID          string    `bson:"_id"`
	AsaasCustomerID string    `bson:"asaasCustomerId"`
	CreatedAt       time.Time `bson:"createdAt"`
	UpdatedAt       time.Time `bson:"updatedAt"`
}

// AsaasCustomerLinkRepository implements portout.AsaasCustomerLinkRepository using MongoDB.
type AsaasCustomerLinkRepository struct {
	col *mongo.Collection
}

// NewAsaasCustomerLinkRepository creates a new AsaasCustomerLinkRepository.
func NewAsaasCustomerLinkRepository(client *Client) *AsaasCustomerLinkRepository {
	return &AsaasCustomerLinkRepository{col: client.Collection(colAsaasCustomerLinks)}
}

// FindByUserID retrieves the Asaas customer link for the given platform user.
func (r *AsaasCustomerLinkRepository) FindByUserID(ctx context.Context, userID string) (*domain.AsaasCustomerLink, error) {
	var doc asaasCustomerLinkDocument
	if err := r.col.FindOne(ctx, bson.D{{Key: "_id", Value: userID}}).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apierr.NotFound("asaas_customer_link", userID)
		}
		return nil, apierr.Internal(fmt.Sprintf("find asaas customer link: %s", err.Error()))
	}
	link := asaasLinkFromDoc(doc)
	return &link, nil
}

// Save upserts a new Asaas customer link keyed by userID.
// Using upsert avoids duplicate-key errors on race conditions (two requests
// creating the same customer concurrently).
func (r *AsaasCustomerLinkRepository) Save(ctx context.Context, link *domain.AsaasCustomerLink) error {
	doc := asaasLinkToDoc(link)
	_, err := r.col.ReplaceOne(ctx,
		bson.D{{Key: "_id", Value: link.UserID}},
		doc,
		options.Replace().SetUpsert(true),
	)
	if err != nil {
		return apierr.Internal(fmt.Sprintf("save asaas customer link: %s", err.Error()))
	}
	return nil
}

// Update replaces an existing Asaas customer link.
func (r *AsaasCustomerLinkRepository) Update(ctx context.Context, link *domain.AsaasCustomerLink) error {
	link.UpdatedAt = time.Now()
	doc := asaasLinkToDoc(link)
	res, err := r.col.ReplaceOne(ctx, bson.D{{Key: "_id", Value: link.UserID}}, doc)
	if err != nil {
		return apierr.Internal(fmt.Sprintf("update asaas customer link: %s", err.Error()))
	}
	if res.MatchedCount == 0 {
		return apierr.NotFound("asaas_customer_link", link.UserID)
	}
	return nil
}

// ---- mapping helpers ----

func asaasLinkToDoc(link *domain.AsaasCustomerLink) asaasCustomerLinkDocument {
	return asaasCustomerLinkDocument{
		UserID:          link.UserID,
		AsaasCustomerID: link.AsaasCustomerID,
		CreatedAt:       link.CreatedAt,
		UpdatedAt:       link.UpdatedAt,
	}
}

func asaasLinkFromDoc(doc asaasCustomerLinkDocument) domain.AsaasCustomerLink {
	return domain.AsaasCustomerLink{
		UserID:          doc.UserID,
		AsaasCustomerID: doc.AsaasCustomerID,
		CreatedAt:       doc.CreatedAt,
		UpdatedAt:       doc.UpdatedAt,
	}
}
