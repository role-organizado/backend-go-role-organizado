package mongodb

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	temporalactivity "github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/activity"
	"github.com/role-organizado/backend-go-role-organizado/internal/domain/admin"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// reconciliationReportDoc is the BSON representation of a ReconciliationReport
// stored in the `reconciliation_reports` collection.
// Shape mirrors the Java ReconciliationReport entity for cross-service readability.
type reconciliationReportDoc struct {
	ID            string                     `bson:"_id"`
	ReferenceDate string                     `bson:"referenceDate"`
	RunAt         time.Time                  `bson:"runAt"`
	CheckedCount  int64                      `bson:"checkedCount"`
	UpdatedCount  int64                      `bson:"updatedCount"`
	FailedCount   int64                      `bson:"failedCount"`
	Updates       []reconciliationUpdateDoc  `bson:"updates,omitempty"`
	Errors        []string                   `bson:"errors,omitempty"`
}

type reconciliationUpdateDoc struct {
	TransactionID  string    `bson:"transactionId"`
	PreviousStatus string    `bson:"previousStatus"`
	NewStatus      string    `bson:"newStatus"`
	ProviderStatus string    `bson:"providerStatus"`
	UpdatedAt      time.Time `bson:"updatedAt"`
}

// ReconciliationReportRepository persists reconciliation run results to MongoDB.
type ReconciliationReportRepository struct {
	coll *mongo.Collection
}

// NewReconciliationReportRepository creates a repository backed by the
// `reconciliation_reports` collection.
func NewReconciliationReportRepository(client *Client) *ReconciliationReportRepository {
	return &ReconciliationReportRepository{
		coll: client.Collection("reconciliation_reports"),
	}
}

// Save inserts a new reconciliation report document.
func (r *ReconciliationReportRepository) Save(ctx context.Context, report *temporalactivity.ReconciliationReport) error {
	doc := reconciliationReportDoc{
		ID:            report.ID,
		ReferenceDate: report.ReferenceDate,
		RunAt:         report.RunAt,
		CheckedCount:  report.CheckedCount,
		UpdatedCount:  report.UpdatedCount,
		FailedCount:   report.FailedCount,
		Errors:        report.Errors,
	}
	for _, u := range report.Updates {
		doc.Updates = append(doc.Updates, reconciliationUpdateDoc{
			TransactionID:  u.TransactionID,
			PreviousStatus: u.PreviousStatus,
			NewStatus:      u.NewStatus,
			ProviderStatus: u.ProviderStatus,
			UpdatedAt:      u.UpdatedAt,
		})
	}

	_, err := r.coll.InsertOne(ctx, bson.M{
		"_id":           doc.ID,
		"referenceDate": doc.ReferenceDate,
		"runAt":         doc.RunAt,
		"checkedCount":  doc.CheckedCount,
		"updatedCount":  doc.UpdatedCount,
		"failedCount":   doc.FailedCount,
		"updates":       doc.Updates,
		"errors":        doc.Errors,
	})
	if err != nil {
		return fmt.Errorf("reconciliation report: insert: %w", err)
	}
	return nil
}

// compile-time interface assertion.
var _ temporalactivity.ReconciliationReportRepository = (*ReconciliationReportRepository)(nil)

// FindRecent returns the most recent reconciliation reports, newest first.
// Implements out.ReconciliationReportReader.
func (r *ReconciliationReportRepository) FindRecent(ctx context.Context, limit int) ([]admin.ReconciliationReport, error) {
	if limit <= 0 {
		limit = 20
	}
	opts := options.Find().SetSort(bson.D{{Key: "runAt", Value: -1}}).SetLimit(int64(limit))
	cursor, err := r.coll.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, fmt.Errorf("reconciliation report: find recent: %w", err)
	}
	defer cursor.Close(ctx)

	var docs []reconciliationReportDoc
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("reconciliation report: decode recent: %w", err)
	}
	out := make([]admin.ReconciliationReport, 0, len(docs))
	for _, d := range docs {
		out = append(out, reconciliationReportToDomain(d))
	}
	return out, nil
}

// FindLatest returns the single most recent reconciliation report.
// Implements out.ReconciliationReportReader.
func (r *ReconciliationReportRepository) FindLatest(ctx context.Context) (*admin.ReconciliationReport, error) {
	opts := options.FindOne().SetSort(bson.D{{Key: "runAt", Value: -1}})
	var doc reconciliationReportDoc
	if err := r.coll.FindOne(ctx, bson.M{}, opts).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apierr.NotFoundMsg("nenhum relatório de reconciliação encontrado")
		}
		return nil, fmt.Errorf("reconciliation report: find latest: %w", err)
	}
	report := reconciliationReportToDomain(doc)
	return &report, nil
}

func reconciliationReportToDomain(d reconciliationReportDoc) admin.ReconciliationReport {
	updates := make([]admin.ReconciliationUpdate, 0, len(d.Updates))
	for _, u := range d.Updates {
		updates = append(updates, admin.ReconciliationUpdate{
			TransactionID:  u.TransactionID,
			PreviousStatus: u.PreviousStatus,
			NewStatus:      u.NewStatus,
			ProviderStatus: u.ProviderStatus,
			UpdatedAt:      u.UpdatedAt,
		})
	}
	return admin.ReconciliationReport{
		ID:            d.ID,
		ReferenceDate: d.ReferenceDate,
		RunAt:         d.RunAt,
		CheckedCount:  d.CheckedCount,
		UpdatedCount:  d.UpdatedCount,
		FailedCount:   d.FailedCount,
		Updates:       updates,
		Errors:        d.Errors,
	}
}
