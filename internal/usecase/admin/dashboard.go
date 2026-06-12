// Package admin implements the admin-surface use cases (dashboard, feature flags,
// approvals, dominios extras, cancellation policies, events admin, reconciliation
// reports). All persistence is delegated to output ports — zero direct Mongo here.
package admin

import (
	"context"
	"log/slog"
	"sort"
	"time"

	"github.com/role-organizado/backend-go-role-organizado/internal/domain/admin"
	"github.com/role-organizado/backend-go-role-organizado/internal/domain/outbound"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
)

// GetDashboardStats implements portin.GetDashboardStatsUseCase.
type GetDashboardStats struct {
	metrics portout.AdminMetricsRepository
}

// NewGetDashboardStats creates a new GetDashboardStats use case.
func NewGetDashboardStats(m portout.AdminMetricsRepository) *GetDashboardStats {
	return &GetDashboardStats{metrics: m}
}

// Execute returns the aggregate dashboard counts.
func (uc *GetDashboardStats) Execute(ctx context.Context) (admin.DashboardCounts, error) {
	slog.InfoContext(ctx, "admin dashboard stats")
	return uc.metrics.DashboardCounts(ctx)
}

// GetDashboardHealth implements portin.GetDashboardHealthUseCase.
type GetDashboardHealth struct {
	metrics portout.AdminMetricsRepository
}

// NewGetDashboardHealth creates a new GetDashboardHealth use case.
func NewGetDashboardHealth(m portout.AdminMetricsRepository) *GetDashboardHealth {
	return &GetDashboardHealth{metrics: m}
}

// Execute reports whether the database is reachable.
func (uc *GetDashboardHealth) Execute(ctx context.Context) (bool, error) {
	return uc.metrics.Ping(ctx) == nil, nil
}

// GetDashboardFinance implements portin.GetDashboardFinanceUseCase.
type GetDashboardFinance struct {
	metrics portout.AdminMetricsRepository
}

// NewGetDashboardFinance creates a new GetDashboardFinance use case.
func NewGetDashboardFinance(m portout.AdminMetricsRepository) *GetDashboardFinance {
	return &GetDashboardFinance{metrics: m}
}

// Execute aggregates the finance summaries. On error it returns a zeroed map
// (Java's defensive behaviour) rather than failing the request.
func (uc *GetDashboardFinance) Execute(ctx context.Context) (map[string]any, error) {
	totals, err := uc.metrics.FinanceSummaryTotals(ctx)
	if err != nil {
		slog.WarnContext(ctx, "dashboard finance aggregation failed", "error", err)
		return map[string]any{"totalEventos": 0}, nil
	}
	return totals, nil
}

// GetPendingOutbound implements portin.GetPendingOutboundUseCase.
type GetPendingOutbound struct {
	eventos  portout.EventoRepository
	outbound portout.OutboundRequestRepository
}

// NewGetPendingOutbound creates a new GetPendingOutbound use case.
func NewGetPendingOutbound(e portout.EventoRepository, o portout.OutboundRequestRepository) *GetPendingOutbound {
	return &GetPendingOutbound{eventos: e, outbound: o}
}

// Execute walks all events, collects their pending outbound requests, and returns
// a summary sorted by pending amount desc then count desc. Java parity:
// AdminDashboardController.getPendingOutbound.
func (uc *GetPendingOutbound) Execute(ctx context.Context, limit int) (admin.PendingOutboundSummary, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}

	// Page through all events (admin surface; volume is bounded in practice).
	const pageSize = 200
	var items []admin.PendingOutboundItem
	summary := admin.PendingOutboundSummary{Timestamp: time.Now().UTC()}

	for page := 0; ; page++ {
		eventos, total, err := uc.eventos.FindAll(ctx, page, pageSize)
		if err != nil {
			return summary, err
		}
		for i := range eventos {
			ev := &eventos[i]
			pending, err := uc.outbound.FindPendingByEventID(ctx, ev.ID)
			if err != nil {
				return summary, err
			}
			if len(pending) == 0 {
				continue
			}
			item := admin.PendingOutboundItem{
				EventID:      ev.ID,
				EventName:    ev.Nome,
				PendingCount: len(pending),
			}
			item.OldestPendingAt, item.PendingAmountCents = oldestAndSum(pending)
			items = append(items, item)
			summary.TotalPendingRequests += len(pending)
			summary.TotalPendingAmountCents += item.PendingAmountCents
		}
		if int64((page+1)*pageSize) >= total || len(eventos) == 0 {
			break
		}
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].PendingAmountCents != items[j].PendingAmountCents {
			return items[i].PendingAmountCents > items[j].PendingAmountCents
		}
		return items[i].PendingCount > items[j].PendingCount
	})

	summary.TotalEventsWithPending = len(items)
	if len(items) > limit {
		items = items[:limit]
	}
	summary.Items = items
	return summary, nil
}

// oldestAndSum returns the earliest CreatedAt and the total AmountCents of the
// given requests.
func oldestAndSum(reqs []outbound.OutboundRequest) (*time.Time, int64) {
	var oldest *time.Time
	var sum int64
	for i := range reqs {
		sum += reqs[i].AmountCents
		created := reqs[i].CreatedAt
		if oldest == nil || created.Before(*oldest) {
			c := created
			oldest = &c
		}
	}
	return oldest, sum
}
