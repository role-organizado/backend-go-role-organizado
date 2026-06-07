package payment

import (
	"context"
	"fmt"
	"log/slog"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
)

// FeePolicyResolver resolves the fee policy snapshot for a payment.
// Mocked in tests; concrete implementation is FeePolicyService.
type FeePolicyResolver interface {
	ResolveSnapshot(ctx context.Context, eventID string) (domain.FeePolicySnapshot, error)
}

// globalPolicy is the default fee configuration used when no event-specific
// policy exists. Mirrors the Java GlobalFeePolicyConfig defaults.
var globalPolicy = domain.FeePolicySnapshot{
	FeePolicySource:       "GLOBAL",
	PlatformFeePercent:    0.0,
	PlatformFeeFixedCents: 0,
	PspFeePercent:         1.99, // Typical Asaas PIX/Boleto fee
	PspFeeFixedCents:      0,
	Version:               "global-v1",
}

// eventFeeConfigRaw holds the Java-written fee fields from evento_config_pagamentos.
// Pointer fields are nil if not present in the document — this distinguishes
// "not set" from "set to 0".
type eventFeeConfigRaw struct {
	PlatformFeePercent    *float64 `bson:"platformFeePercent"`
	PspFeePercent         *float64 `bson:"pspFeePercent"`
	PlatformFeeFixedCents *int64   `bson:"platformFeeFixedCents"`
	PspFeeFixedCents      *int64   `bson:"pspFeeFixedCents"`
	FeePolicyVersion      string   `bson:"feePolicyVersion"`
}

// FeePolicyService resolves fee configuration from the shared evento_config_pagamentos
// collection. If the document has platformFeePercent/pspFeePercent Java fields, those
// are used. Falls back to the GLOBAL policy if the event has no custom fee configuration.
//
// The collection is shared with the Java backend: Java writes camelCase keys (eventoId),
// Go writes snake_case keys (evento_id). Both variants are queried.
//
// Implements FeePolicyResolver.
type FeePolicyService struct {
	col *mongo.Collection
}

// NewFeePolicyService creates a new FeePolicyService targeting the given collection.
// Pass mongoClient.Collection("evento_config_pagamentos") from cmd/server/main.go.
func NewFeePolicyService(col *mongo.Collection) *FeePolicyService {
	return &FeePolicyService{col: col}
}

// ResolveSnapshot reads the evento_config_pagamentos collection for eventID and extracts
// fee fields written by the Java backend. If the document is absent or lacks fee fields,
// the GLOBAL fallback policy is returned.
func (s *FeePolicyService) ResolveSnapshot(ctx context.Context, eventID string) (domain.FeePolicySnapshot, error) {
	// Query both camelCase (Java) and snake_case (Go) key variants.
	filter := bson.D{{Key: "$or", Value: bson.A{
		bson.D{{Key: "eventoId", Value: eventID}},
		bson.D{{Key: "evento_id", Value: eventID}},
	}}}

	var cfg eventFeeConfigRaw
	if err := s.col.FindOne(ctx, filter).Decode(&cfg); err != nil {
		if err == mongo.ErrNoDocuments {
			slog.DebugContext(ctx, "fee policy: event config not found, using global", "eventID", eventID)
			return globalPolicy, nil
		}
		return domain.FeePolicySnapshot{}, fmt.Errorf("fee policy: find event config for %s: %w", eventID, err)
	}

	// If neither fee percentage was set by Java, fall back to global policy.
	if cfg.PlatformFeePercent == nil && cfg.PspFeePercent == nil {
		slog.DebugContext(ctx, "fee policy: event config has no custom fees, using global", "eventID", eventID)
		return globalPolicy, nil
	}

	snap := domain.FeePolicySnapshot{
		FeePolicySource: "EVENT:" + eventID,
		Version:         "event-v1",
	}
	if cfg.FeePolicyVersion != "" {
		snap.Version = cfg.FeePolicyVersion
	}
	if cfg.PlatformFeePercent != nil {
		snap.PlatformFeePercent = *cfg.PlatformFeePercent
	}
	if cfg.PspFeePercent != nil {
		snap.PspFeePercent = *cfg.PspFeePercent
	}
	if cfg.PlatformFeeFixedCents != nil {
		snap.PlatformFeeFixedCents = *cfg.PlatformFeeFixedCents
	}
	if cfg.PspFeeFixedCents != nil {
		snap.PspFeeFixedCents = *cfg.PspFeeFixedCents
	}

	return snap, nil
}

// compile-time assertion: *FeePolicyService implements FeePolicyResolver.
var _ FeePolicyResolver = (*FeePolicyService)(nil)
