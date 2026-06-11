// Package finance contains use cases for finance domain operations.
package finance

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
)

// Severity classification thresholds (absolute cents). Mirror the Java
// ReconcileFinanceSummaryService classification used during the triple-check.
const (
	severityCriticalCents int64 = 5000
	severityWarningCents  int64 = 500
)

// Diff types emitted in the per-event divergence summary. Kept as strings to
// match the Java enum values used in audit dashboards.
const (
	diffTypeInSync          = "IN_SYNC"
	diffTypeSummaryVsLedger = "SUMMARY_VS_LEDGER"
	diffTypePspVsSummary    = "PSP_VS_SUMMARY"
	diffTypeTripleMismatch  = "TRIPLE_MISMATCH"
)

// ReconciliationReport summarises a single triple-check pass. The repository
// implementation lives in the mongodb adapter to keep this layer pure Go.
type ReconciliationReport struct {
	ID            string
	ReferenceDate string
	RunAt         time.Time
	EventsChecked int64
	Divergences   int64
	Critical      int64
	Warnings      int64
	DurationMs    int64
	Mismatches    []ReconciliationMismatch
}

// ReconciliationMismatch describes a single event whose three balance sources
// (PSP net, finance summary, ledger) disagree.
type ReconciliationMismatch struct {
	EventID         string
	Severity        string // INFO | WARNING | CRITICAL
	DiffType        string
	PspNetCents     int64
	SummaryCents    int64
	LedgerCents     int64
	DeltaCents      int64
}

// ReconciliationReportRepository persists triple-check results for auditability.
type ReconciliationReportRepository interface {
	Save(ctx context.Context, report *ReconciliationReport) error
}

// ReconciliationUseCase performs a single finance reconciliation pass natively
// (no HTTP delegation to Java). It is invoked three times per workflow execution
// (the triple-check pattern) — each pass is read-only and idempotent.
type ReconciliationUseCase struct {
	eventos    portout.EventoRepository
	summaries  portout.FinanceSummaryRepository
	ledger     portout.LedgerEntryRepository
	txs        portout.PaymentTransactionRepository
	reports    ReconciliationReportRepository
	now        func() time.Time
}

// NewReconciliationUseCase wires the native ReconciliationUseCase with its repos.
// reports may be nil for tests.
func NewReconciliationUseCase(
	eventos portout.EventoRepository,
	summaries portout.FinanceSummaryRepository,
	ledger portout.LedgerEntryRepository,
	txs portout.PaymentTransactionRepository,
	reports ReconciliationReportRepository,
) *ReconciliationUseCase {
	return &ReconciliationUseCase{
		eventos:   eventos,
		summaries: summaries,
		ledger:    ledger,
		txs:       txs,
		reports:   reports,
		now:       time.Now,
	}
}

// Execute walks every event, fetches PSP net from completed payment transactions,
// finance summary "collected" value, and the signed sum of ledger entries, then
// classifies divergences and persists a report.
//
// Read-heavy — no writes to FinanceSummary / LedgerEntry. Safe to retry.
func (uc *ReconciliationUseCase) Execute(ctx context.Context, referenceDate string) error {
	start := uc.now()
	if referenceDate == "" {
		referenceDate = start.UTC().AddDate(0, 0, -1).Format("2006-01-02")
	}

	// Walk events page-by-page to bound memory.
	report := &ReconciliationReport{
		ReferenceDate: referenceDate,
		RunAt:         start.UTC(),
	}

	pageSize := 200
	for page := 1; ; page++ {
		events, _, err := uc.eventos.FindAll(ctx, page, pageSize)
		if err != nil {
			return fmt.Errorf("finance-reconciliation: list eventos page %d: %w", page, err)
		}
		if len(events) == 0 {
			break
		}
		for i := range events {
			eventID := events[i].ID
			mismatch, checked := uc.checkEvent(ctx, eventID)
			if !checked {
				continue
			}
			report.EventsChecked++
			if mismatch.DiffType == diffTypeInSync {
				continue
			}
			report.Divergences++
			switch mismatch.Severity {
			case "CRITICAL":
				report.Critical++
			case "WARNING":
				report.Warnings++
			}
			report.Mismatches = append(report.Mismatches, mismatch)
		}
		if len(events) < pageSize {
			break
		}
	}

	report.DurationMs = uc.now().Sub(start).Milliseconds()

	if uc.reports != nil {
		if saveErr := uc.reports.Save(ctx, report); saveErr != nil {
			slog.WarnContext(ctx, "finance-reconciliation: failed to persist report",
				"error", saveErr, "referenceDate", referenceDate)
		}
	}

	slog.InfoContext(ctx, "finance-reconciliation native pass completed",
		"referenceDate", referenceDate,
		"eventsChecked", report.EventsChecked,
		"divergences", report.Divergences,
		"critical", report.Critical,
		"warnings", report.Warnings,
		"durationMs", report.DurationMs,
	)
	return nil
}

// checkEvent computes the three balances for a single event and returns the
// mismatch summary along with a boolean indicating whether the event was
// successfully evaluated (false means the event was skipped due to a transient
// repo error).
func (uc *ReconciliationUseCase) checkEvent(ctx context.Context, eventID string) (ReconciliationMismatch, bool) {
	mismatch := ReconciliationMismatch{EventID: eventID, DiffType: diffTypeInSync, Severity: "INFO"}

	// 1) PSP net — sum of COMPLETED transactions for the event. We use a 0
	//    "since" so the query returns the full history (the triple-check
	//    compares against current finance state, not a rolling window).
	txs, err := uc.txs.FindCompletedByEventID(ctx, eventID, time.Time{})
	if err != nil {
		slog.WarnContext(ctx, "finance-reconciliation: skip event (txs lookup failed)",
			"eventID", eventID, "error", err)
		return mismatch, false
	}
	var pspNet int64
	for _, tx := range txs {
		pspNet += tx.AmountCents
	}
	mismatch.PspNetCents = pspNet

	// 2) Finance summary — collected/available balance.
	if summary, sErr := uc.summaries.FindByEventID(ctx, eventID); sErr == nil && summary != nil {
		mismatch.SummaryCents = summary.Collected
	}

	// 3) Ledger — signed sum of entries (INCOME positive, others negative).
	entries, _, lErr := uc.ledger.FindByEventID(ctx, eventID, nil, nil, nil, 1, 10000)
	if lErr == nil {
		for i := range entries {
			amount := entries[i].Amount
			if entries[i].Type == "INCOME" {
				mismatch.LedgerCents += amount
			} else {
				mismatch.LedgerCents -= amount
			}
		}
	}

	// Classify divergence
	psp := mismatch.PspNetCents
	sum := mismatch.SummaryCents
	led := mismatch.LedgerCents
	maxDelta := absMax3(psp-sum, sum-led, psp-led)
	mismatch.DeltaCents = maxDelta

	switch {
	case psp == sum && sum == led:
		mismatch.DiffType = diffTypeInSync
	case psp != sum && sum != led:
		mismatch.DiffType = diffTypeTripleMismatch
	case psp != sum:
		mismatch.DiffType = diffTypePspVsSummary
	default:
		mismatch.DiffType = diffTypeSummaryVsLedger
	}

	switch {
	case maxDelta >= severityCriticalCents:
		mismatch.Severity = "CRITICAL"
	case maxDelta >= severityWarningCents:
		mismatch.Severity = "WARNING"
	default:
		mismatch.Severity = "INFO"
	}

	return mismatch, true
}

// absMax3 returns the absolute value of the largest-magnitude argument.
func absMax3(a, b, c int64) int64 {
	max := abs64(a)
	if v := abs64(b); v > max {
		max = v
	}
	if v := abs64(c); v > max {
		max = v
	}
	return max
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
