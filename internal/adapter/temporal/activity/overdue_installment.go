// Package activity contains Temporal activity implementations.
package activity

import (
	"context"
	"fmt"
	"log/slog"
)

// overdueInstallmentFinder is satisfied by payment.FindAndMarkOverdueInstallmentsUseCase.
type overdueInstallmentFinder interface {
	Execute(ctx context.Context, referenceDate string) (int, error)
}

// overdueNotificationDispatcher is satisfied by notification.DispatchOverdueNotificationsUseCase.
type overdueNotificationDispatcher interface {
	Execute(ctx context.Context, referenceDate string, count int) error
}

// OverdueInstallmentActivities contains the activity implementations for the
// OverdueInstallment workflow. Register with worker.RegisterActivity(acts).
type OverdueInstallmentActivities struct {
	finder     overdueInstallmentFinder
	dispatcher overdueNotificationDispatcher
}

// NewOverdueInstallmentActivities constructs OverdueInstallmentActivities.
func NewOverdueInstallmentActivities(
	finder overdueInstallmentFinder,
	dispatcher overdueNotificationDispatcher,
) *OverdueInstallmentActivities {
	return &OverdueInstallmentActivities{
		finder:     finder,
		dispatcher: dispatcher,
	}
}

// FindAndMarkOverdueInstallments marks past-due installments as VENCIDO and returns the count.
// Registered as activity name "FindAndMarkOverdueInstallments".
func (a *OverdueInstallmentActivities) FindAndMarkOverdueInstallments(ctx context.Context, referenceDate string) (int, error) {
	count, err := a.finder.Execute(ctx, referenceDate)
	if err != nil {
		return 0, fmt.Errorf("FindAndMarkOverdueInstallments: %w", err)
	}
	slog.InfoContext(ctx, "overdue installments marked",
		"referenceDate", referenceDate,
		"count", count,
	)
	return count, nil
}

// DispatchNotifications dispatches overdue-installment notifications to the affected users.
// Registered as activity name "DispatchNotifications".
// Only called when count > 0 (enforced by the workflow).
func (a *OverdueInstallmentActivities) DispatchNotifications(ctx context.Context, referenceDate string, count int) error {
	if err := a.dispatcher.Execute(ctx, referenceDate, count); err != nil {
		return fmt.Errorf("DispatchNotifications: %w", err)
	}
	slog.InfoContext(ctx, "overdue notifications dispatched",
		"referenceDate", referenceDate,
		"count", count,
	)
	return nil
}
