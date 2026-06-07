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
var _ portout.ProcessedWebhookEventRepository = (*ProcessedWebhookEventRepository)(nil)

const colProcessedWebhookEvents = "processed_webhook_events"

// processedWebhookEventDocument matches the processed_webhook_events schema.
// The Java uses camelCase for eventId and providerTransactionId.
type processedWebhookEventDocument struct {
	ID                    string    `bson:"_id,omitempty"`
	Provider              string    `bson:"provider"`
	EventID               string    `bson:"eventId"`
	ProviderTransactionID string    `bson:"providerTransactionId,omitempty"`
	EventType             string    `bson:"eventType,omitempty"`
	ProcessedAt           time.Time `bson:"processedAt"`
}

// ProcessedWebhookEventRepository implements portout.ProcessedWebhookEventRepository.
type ProcessedWebhookEventRepository struct {
	col *mongo.Collection
}

// NewProcessedWebhookEventRepository creates a new ProcessedWebhookEventRepository.
func NewProcessedWebhookEventRepository(client *Client) *ProcessedWebhookEventRepository {
	return &ProcessedWebhookEventRepository{col: client.Collection(colProcessedWebhookEvents)}
}

// ExistsByProviderAndEventID returns true if the (provider, eventID) pair has been recorded.
func (r *ProcessedWebhookEventRepository) ExistsByProviderAndEventID(ctx context.Context, provider, eventID string) (bool, error) {
	count, err := r.col.CountDocuments(ctx,
		bson.D{
			{Key: "provider", Value: provider},
			{Key: "eventId", Value: eventID},
		},
		options.Count().SetLimit(1),
	)
	if err != nil {
		return false, apierr.Internal(fmt.Sprintf("check processed webhook event: %s", err.Error()))
	}
	return count > 0, nil
}

// SaveUnique persists the webhook event record. Returns portout.ErrAlreadyProcessed
// if the (provider, eventID) unique index fires — the caller treats this as a no-op.
func (r *ProcessedWebhookEventRepository) SaveUnique(ctx context.Context, e *domain.ProcessedWebhookEvent) error {
	if e.ID == "" {
		e.ID = uuid.New().String()
	}
	doc := processedWebhookEventDocument{
		ID:                    e.ID,
		Provider:              e.Provider,
		EventID:               e.EventID,
		ProviderTransactionID: e.ProviderTransactionID,
		EventType:             e.EventType,
		ProcessedAt:           e.ProcessedAt,
	}
	if _, err := r.col.InsertOne(ctx, doc); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return portout.ErrAlreadyProcessed
		}
		return apierr.Internal(fmt.Sprintf("save processed webhook event: %s", err.Error()))
	}
	return nil
}
