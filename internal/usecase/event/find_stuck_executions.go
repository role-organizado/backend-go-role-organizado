package event

import (
	"context"
	"fmt"
	"sort"
	"time"

	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
)

// defaultStuckThresholdMinutes is applied when the caller passes a non-positive
// StuckThresholdMinutes.
const defaultStuckThresholdMinutes = 30

// defaultStuckMaxResults caps a scan when the caller passes a non-positive MaxResults.
const defaultStuckMaxResults = 50

// FindStuckExecutions implements portin.FindStuckExecutionsUseCase.
//
// It treats PENDING/PROCESSING payment transactions older than the configured
// threshold as "stuck" executions, leveraging the repository's
// FindPendingOlderThan query. The scan is read-only and idempotent.
type FindStuckExecutions struct {
	transactions portout.PaymentTransactionRepository
}

// NewFindStuckExecutions creates a new FindStuckExecutions use case.
func NewFindStuckExecutions(transactions portout.PaymentTransactionRepository) *FindStuckExecutions {
	return &FindStuckExecutions{transactions: transactions}
}

// Execute scans for stuck executions and returns them ordered oldest-first,
// capped at MaxResults.
func (uc *FindStuckExecutions) Execute(ctx context.Context, in portin.FindStuckExecutionsInput) (*portin.FindStuckExecutionsResult, error) {
	thresholdMin := in.StuckThresholdMinutes
	if thresholdMin <= 0 {
		thresholdMin = defaultStuckThresholdMinutes
	}
	maxResults := in.MaxResults
	if maxResults <= 0 {
		maxResults = defaultStuckMaxResults
	}

	now := time.Now()
	threshold := now.Add(-time.Duration(thresholdMin) * time.Minute)

	txs, err := uc.transactions.FindPendingOlderThan(ctx, threshold)
	if err != nil {
		return nil, fmt.Errorf("find pending transactions older than %s: %w", threshold.Format(time.RFC3339), err)
	}

	stuck := make([]portin.StuckExecution, 0, len(txs))
	for _, tx := range txs {
		if tx == nil {
			continue
		}
		stuck = append(stuck, portin.StuckExecution{
			TransactionID: tx.ID,
			EventID:       tx.EventID,
			Status:        string(tx.Status),
			CreatedAt:     tx.CreatedAt,
			AgeMinutes:    int(now.Sub(tx.CreatedAt).Minutes()),
		})
	}

	// Oldest first — the most stuck executions are the most urgent to surface.
	sort.Slice(stuck, func(i, j int) bool {
		return stuck[i].CreatedAt.Before(stuck[j].CreatedAt)
	})

	if len(stuck) > maxResults {
		stuck = stuck[:maxResults]
	}

	return &portin.FindStuckExecutionsResult{
		StuckCount: len(stuck),
		Executions: stuck,
	}, nil
}

var _ portin.FindStuckExecutionsUseCase = (*FindStuckExecutions)(nil)
