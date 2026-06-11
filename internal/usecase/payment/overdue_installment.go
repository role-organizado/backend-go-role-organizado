package payment

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
)

// FindAndMarkOverdueInstallmentsUseCase marks past-due installments as OVERDUE
// (flag overdue_notification_sent=true) and returns the count of marked records.
//
// Idempotency:
//   - The repository query (FindOverdueNotNotified) filters by
//     overdue_notification_sent=false, so re-running the activity within the
//     same retry budget will find zero rows after the first successful pass.
//   - MarkOverdueBatch is an UpdateMany; a partial failure leaves the
//     already-updated rows correctly tagged, so the next attempt skips them.
type FindAndMarkOverdueInstallmentsUseCase struct {
	repo portout.PaymentInstallmentRepository
	// Marked is exposed for tests / observability: holds the installments that
	// were marked during the most recent Execute call.
	Marked []*domain.PaymentInstallment
}

// NewFindAndMarkOverdueInstallments wires the native implementation.
func NewFindAndMarkOverdueInstallments(repo portout.PaymentInstallmentRepository) *FindAndMarkOverdueInstallmentsUseCase {
	return &FindAndMarkOverdueInstallmentsUseCase{repo: repo}
}

// Execute marks past-due installments and returns the count.
func (uc *FindAndMarkOverdueInstallmentsUseCase) Execute(ctx context.Context, referenceDate string) (int, error) {
	ref, err := parseReferenceDate(referenceDate)
	if err != nil {
		return 0, fmt.Errorf("FindAndMarkOverdueInstallments: %w", err)
	}

	candidates, err := uc.repo.FindOverdueNotNotified(ctx, ref)
	if err != nil {
		return 0, fmt.Errorf("FindAndMarkOverdueInstallments: query: %w", err)
	}
	if len(candidates) == 0 {
		uc.Marked = nil
		slog.InfoContext(ctx, "FindAndMarkOverdueInstallments: nothing to mark",
			"referenceDate", referenceDate)
		return 0, nil
	}

	ids := make([]string, 0, len(candidates))
	for _, inst := range candidates {
		ids = append(ids, inst.ID)
	}
	updated, err := uc.repo.MarkOverdueBatch(ctx, ids)
	if err != nil {
		return 0, fmt.Errorf("FindAndMarkOverdueInstallments: update: %w", err)
	}

	uc.Marked = candidates
	slog.InfoContext(ctx, "FindAndMarkOverdueInstallments: marked",
		"referenceDate", referenceDate,
		"matched", len(candidates),
		"updated", updated,
	)
	// We return the matched count: it's what downstream notification logic uses
	// to decide whether to dispatch. The DB UpdateMany may have already been
	// applied in a previous attempt (idempotent retries), so trusting `updated`
	// could under-report on retry. The matched count reflects the candidates
	// that still required action at query time.
	return len(candidates), nil
}

// LastMarked exposes the installments transitioned to OVERDUE in the most recent
// Execute call. The OverdueInstallmentWorkflow's DispatchNotifications activity
// uses this to build per-(participant, event) notification batches without
// re-querying the DB or re-deriving the set.
func (uc *FindAndMarkOverdueInstallmentsUseCase) LastMarked() []*domain.PaymentInstallment {
	return uc.Marked
}

// parseReferenceDate parses YYYY-MM-DD or returns time.Now (UTC truncated to day)
// when the input is empty.
func parseReferenceDate(s string) (time.Time, error) {
	if s == "" {
		now := time.Now().UTC()
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC), nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse referenceDate %q: %w", s, err)
	}
	return t, nil
}
