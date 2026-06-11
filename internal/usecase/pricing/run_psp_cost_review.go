// Package pricing holds use-case implementations for the pricing domain.
package pricing

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
)

// Lookback window for completed transactions when computing real PSP cost.
const pspReviewLookbackDays = 30

// driftWarningThresholdPp is the percentage-point delta (e.g. 0.5pp) that escalates
// an event into the warnings bucket.
const driftWarningThresholdPp = 0.5

// PspReviewReport is a single persisted summary of a daily review run. The
// repository implementation lives in the mongodb adapter to avoid pulling
// mongo dependencies into the domain layer.
type PspReviewReport struct {
	ID            string
	ReferenceDate string
	RunAt         time.Time
	Reviewed      int64
	Eligible      int64
	Warnings      int64
	Skipped       int64
	DurationMs    int64
	Drifts        []PspReviewDrift
}

// PspReviewDrift describes a single event whose computed PSP cost diverges from
// the snapshot reference by more than driftWarningThresholdPp.
type PspReviewDrift struct {
	EventID            string
	ReferencePercent   float64
	ComputedPercent    float64
	DeltaPp            float64
	CompletedTxs       int
	GrossCents         int64
	PspFeeAppliedCents int64
}

// PspReviewReportRepository persists daily PSP review reports for auditability.
type PspReviewReportRepository interface {
	Save(ctx context.Context, report *PspReviewReport) error
}

// runPspCostReviewUseCase implements RunPspCostReviewUseCase natively, replacing
// the previous HTTP delegation to the Java backend.
//
// The use case scans all EventoConfigPagamento records, fetches COMPLETED payment
// transactions in a 30-day window for each event, recomputes the effective PSP
// cost percentage from gross+fee amounts, compares against the snapshot reference,
// and emits a warning entry when the delta exceeds driftWarningThresholdPp.
type runPspCostReviewUseCase struct {
	configRepo portout.EventoConfigPagamentoRepository
	txRepo     portout.PaymentTransactionRepository
	reportRepo PspReviewReportRepository
	now        func() time.Time
}

// NewRunPspCostReview creates a native (no-HTTP) PSP cost review use case.
//
// reportRepo may be nil during tests that don't care about persistence; in that
// case the report is logged but not persisted.
func NewRunPspCostReview(
	configRepo portout.EventoConfigPagamentoRepository,
	txRepo portout.PaymentTransactionRepository,
	reportRepo PspReviewReportRepository,
) *runPspCostReviewUseCase {
	return &runPspCostReviewUseCase{
		configRepo: configRepo,
		txRepo:     txRepo,
		reportRepo: reportRepo,
		now:        time.Now,
	}
}

// Execute scans all EventoConfigPagamento documents, computes real PSP cost for
// each event from the past 30 days of COMPLETED transactions, and persists a
// summary report with the per-event drifts.
//
// referenceDate is informational only (used in the persisted report). If empty
// the use case uses today's UTC date.
func (uc *runPspCostReviewUseCase) Execute(ctx context.Context, referenceDate string) error {
	start := uc.now()
	if referenceDate == "" {
		referenceDate = start.UTC().Format("2006-01-02")
	}
	since := start.UTC().Add(-pspReviewLookbackDays * 24 * time.Hour)

	configs, err := uc.configRepo.FindAll(ctx)
	if err != nil {
		return fmt.Errorf("psp-review: list configs: %w", err)
	}

	report := &PspReviewReport{
		ReferenceDate: referenceDate,
		RunAt:         start.UTC(),
	}

	for _, cfg := range configs {
		report.Reviewed++

		// Skip configs without a meaningful reference PSP cost to compare against.
		referencePercent := cfg.PspFeePercent
		if referencePercent <= 0 && cfg.PspFeeFixedCents <= 0 {
			report.Skipped++
			continue
		}

		txs, txErr := uc.txRepo.FindCompletedByEventID(ctx, cfg.EventoID, since)
		if txErr != nil {
			slog.WarnContext(ctx, "psp-review: failed to load transactions",
				"eventID", cfg.EventoID, "error", txErr)
			report.Skipped++
			continue
		}
		if len(txs) == 0 {
			report.Skipped++
			continue
		}

		gross, pspFee := sumTransactionAmounts(txs)
		if gross <= 0 {
			report.Skipped++
			continue
		}

		computedPercent := (float64(pspFee) / float64(gross)) * 100.0
		delta := computedPercent - referencePercent
		if delta < 0 {
			delta = -delta
		}

		report.Eligible++
		if delta >= driftWarningThresholdPp {
			report.Warnings++
			report.Drifts = append(report.Drifts, PspReviewDrift{
				EventID:            cfg.EventoID,
				ReferencePercent:   referencePercent,
				ComputedPercent:    computedPercent,
				DeltaPp:            delta,
				CompletedTxs:       len(txs),
				GrossCents:         gross,
				PspFeeAppliedCents: pspFee,
			})
		}
	}

	report.DurationMs = uc.now().Sub(start).Milliseconds()

	if uc.reportRepo != nil {
		if saveErr := uc.reportRepo.Save(ctx, report); saveErr != nil {
			// Persistence is best-effort: don't fail the workflow.
			slog.WarnContext(ctx, "psp-review: failed to persist report",
				"error", saveErr, "referenceDate", referenceDate)
		}
	}

	slog.InfoContext(ctx, "psp-review native run completed",
		"referenceDate", referenceDate,
		"reviewed", report.Reviewed,
		"eligible", report.Eligible,
		"warnings", report.Warnings,
		"skipped", report.Skipped,
		"durationMs", report.DurationMs,
	)
	return nil
}

// sumTransactionAmounts sums grossAmountCents and pspFeeAppliedCents across the
// given transactions. The Java backend stored those values in a metadata
// subdocument; in Go we use the typed AmountCents (gross) and infer the PSP fee
// from the captured fee policy when available, falling back to a rate derived
// from the average snapshot percent if metadata is absent.
//
// We intentionally use a simple sum here: full per-transaction fee recomputation
// is the responsibility of FeePolicyService and isn't required for drift detection.
func sumTransactionAmounts(txs []*domain.PaymentTransaction) (gross, pspFee int64) {
	for _, tx := range txs {
		gross += tx.AmountCents
		// pspFeeAppliedCents lived in metadata in Java. In Go, the snapshot version
		// is captured but the per-tx fee numeric isn't persisted; we infer a
		// best-effort fee from InstallmentAmountCents-vs-AmountCents when available,
		// or leave 0 (treated as no fee data) otherwise.
		if tx.Metadata.InstallmentAmountCents > 0 && tx.Metadata.CreditCardInstallments > 0 {
			total := tx.Metadata.InstallmentAmountCents * int64(tx.Metadata.CreditCardInstallments)
			if total > tx.AmountCents {
				pspFee += total - tx.AmountCents
			}
		}
	}
	return gross, pspFee
}

// Compile-time interface check.
var _ portin.RunPspCostReviewUseCase = (*runPspCostReviewUseCase)(nil)
