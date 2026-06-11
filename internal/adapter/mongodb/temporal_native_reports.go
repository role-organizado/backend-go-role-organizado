// Package mongodb provides MongoDB-backed adapters.
//
// This file holds report persistence adapters for the native (no-Java)
// Temporal activities: PricingPspReview and FinanceReconciliation.
//
// Collections (created lazily on first insert):
//   - pricing_psp_review_reports  — daily PSP cost drift summaries
//   - finance_reconciliation_reports — triple-check pass summaries
package mongodb

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/mongo"

	ucfinance "github.com/role-organizado/backend-go-role-organizado/internal/usecase/finance"
	ucpricing "github.com/role-organizado/backend-go-role-organizado/internal/usecase/pricing"
)

// ===================================================================
// pricing_psp_review_reports
// ===================================================================

const colPspReviewReports = "pricing_psp_review_reports"

type pspReviewReportDoc struct {
	ID            string              `bson:"_id"`
	ReferenceDate string              `bson:"referenceDate"`
	RunAt         time.Time           `bson:"runAt"`
	Reviewed      int64               `bson:"reviewed"`
	Eligible      int64               `bson:"eligible"`
	Warnings      int64               `bson:"warnings"`
	Skipped       int64               `bson:"skipped"`
	DurationMs    int64               `bson:"durationMs"`
	Drifts        []pspReviewDriftDoc `bson:"drifts,omitempty"`
}

type pspReviewDriftDoc struct {
	EventID            string  `bson:"eventId"`
	ReferencePercent   float64 `bson:"referencePercent"`
	ComputedPercent    float64 `bson:"computedPercent"`
	DeltaPp            float64 `bson:"deltaPp"`
	CompletedTxs       int     `bson:"completedTxs"`
	GrossCents         int64   `bson:"grossCents"`
	PspFeeAppliedCents int64   `bson:"pspFeeAppliedCents"`
}

// PspReviewReportRepository persists pricing PSP review summaries.
type PspReviewReportRepository struct {
	col *mongo.Collection
}

// NewPspReviewReportRepository creates a MongoDB-backed PspReviewReportRepository.
func NewPspReviewReportRepository(client *Client) *PspReviewReportRepository {
	return &PspReviewReportRepository{col: client.Collection(colPspReviewReports)}
}

// Save inserts a new PSP review report. ID is generated when absent.
func (r *PspReviewReportRepository) Save(ctx context.Context, report *ucpricing.PspReviewReport) error {
	if report.ID == "" {
		report.ID = uuid.New().String()
	}
	doc := pspReviewReportDoc{
		ID:            report.ID,
		ReferenceDate: report.ReferenceDate,
		RunAt:         report.RunAt,
		Reviewed:      report.Reviewed,
		Eligible:      report.Eligible,
		Warnings:      report.Warnings,
		Skipped:       report.Skipped,
		DurationMs:    report.DurationMs,
	}
	for _, d := range report.Drifts {
		doc.Drifts = append(doc.Drifts, pspReviewDriftDoc{
			EventID:            d.EventID,
			ReferencePercent:   d.ReferencePercent,
			ComputedPercent:    d.ComputedPercent,
			DeltaPp:            d.DeltaPp,
			CompletedTxs:       d.CompletedTxs,
			GrossCents:         d.GrossCents,
			PspFeeAppliedCents: d.PspFeeAppliedCents,
		})
	}
	if _, err := r.col.InsertOne(ctx, doc); err != nil {
		return fmt.Errorf("psp review report: insert: %w", err)
	}
	return nil
}

// Compile-time assertion that the adapter implements the use-case repository.
var _ ucpricing.PspReviewReportRepository = (*PspReviewReportRepository)(nil)

// ===================================================================
// finance_reconciliation_reports
// ===================================================================

const colFinanceReconReports = "finance_reconciliation_reports"

type financeReconReportDoc struct {
	ID            string                       `bson:"_id"`
	ReferenceDate string                       `bson:"referenceDate"`
	RunAt         time.Time                    `bson:"runAt"`
	EventsChecked int64                        `bson:"eventsChecked"`
	Divergences   int64                        `bson:"divergences"`
	Critical      int64                        `bson:"critical"`
	Warnings      int64                        `bson:"warnings"`
	DurationMs    int64                        `bson:"durationMs"`
	Mismatches    []financeReconMismatchDoc    `bson:"mismatches,omitempty"`
}

type financeReconMismatchDoc struct {
	EventID      string `bson:"eventId"`
	Severity     string `bson:"severity"`
	DiffType     string `bson:"diffType"`
	PspNetCents  int64  `bson:"pspNetCents"`
	SummaryCents int64  `bson:"summaryCents"`
	LedgerCents  int64  `bson:"ledgerCents"`
	DeltaCents   int64  `bson:"deltaCents"`
}

// FinanceReconciliationReportRepository persists triple-check pass summaries.
type FinanceReconciliationReportRepository struct {
	col *mongo.Collection
}

// NewFinanceReconciliationReportRepository creates a MongoDB-backed adapter.
func NewFinanceReconciliationReportRepository(client *Client) *FinanceReconciliationReportRepository {
	return &FinanceReconciliationReportRepository{col: client.Collection(colFinanceReconReports)}
}

// Save inserts a new triple-check pass report.
func (r *FinanceReconciliationReportRepository) Save(ctx context.Context, report *ucfinance.ReconciliationReport) error {
	if report.ID == "" {
		report.ID = uuid.New().String()
	}
	doc := financeReconReportDoc{
		ID:            report.ID,
		ReferenceDate: report.ReferenceDate,
		RunAt:         report.RunAt,
		EventsChecked: report.EventsChecked,
		Divergences:   report.Divergences,
		Critical:      report.Critical,
		Warnings:      report.Warnings,
		DurationMs:    report.DurationMs,
	}
	for _, m := range report.Mismatches {
		doc.Mismatches = append(doc.Mismatches, financeReconMismatchDoc{
			EventID:      m.EventID,
			Severity:     m.Severity,
			DiffType:     m.DiffType,
			PspNetCents:  m.PspNetCents,
			SummaryCents: m.SummaryCents,
			LedgerCents:  m.LedgerCents,
			DeltaCents:   m.DeltaCents,
		})
	}
	if _, err := r.col.InsertOne(ctx, doc); err != nil {
		return fmt.Errorf("finance reconciliation report: insert: %w", err)
	}
	return nil
}

// Compile-time assertion that the adapter implements the use-case repository.
var _ ucfinance.ReconciliationReportRepository = (*FinanceReconciliationReportRepository)(nil)
