package notification

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	notifdomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/notification"
	paymentdomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
)

// overdueInstallmentSource exposes the installments marked as OVERDUE by the
// FindAndMarkOverdueInstallments use case. It is satisfied by
// payment.FindAndMarkOverdueInstallmentsUseCase, decoupled here to avoid a
// circular import between the payment and notification packages.
type overdueInstallmentSource interface {
	// LastMarked returns the installments freshly transitioned to OVERDUE in the
	// most recent successful run. Implementations should return nil when no run
	// has executed yet or when the previous run found no candidates.
	LastMarked() []*paymentdomain.PaymentInstallment
}

// notificationCreator is satisfied by *CreateNotificacao.
type notificationCreator interface {
	Execute(ctx context.Context, in portin.CreateNotificacaoInput) (*notifdomain.Notificacao, error)
}

// DispatchOverdueNotificationsUseCase emits one notification per
// (participantID, eventID) pair using the most recent OVERDUE batch from the
// FindAndMark UC.
//
// Native implementation — no HTTP delegation to Java. Notifications include
// the participant total overdue cents, count, and oldest due date so the
// recipient can prioritise.
type DispatchOverdueNotificationsUseCase struct {
	source  overdueInstallmentSource
	creator notificationCreator
}

// NewDispatchOverdueNotifications wires the native dispatcher.
//
// source may be nil for tests that don't need a real upstream marker; in that
// case Execute is a no-op (count param is purely informational).
func NewDispatchOverdueNotifications(source overdueInstallmentSource, creator notificationCreator) *DispatchOverdueNotificationsUseCase {
	return &DispatchOverdueNotificationsUseCase{source: source, creator: creator}
}

// Execute dispatches one notification per (participantID, eventID) pair for all
// installments marked OVERDUE in the most recent FindAndMark run.
//
// Idempotency: if the same installments are seen twice (e.g. retry after a
// partial failure), the worst case is a duplicate notification — better than a
// missed one. The FindAndMark UC already prevents re-emitting the same overdue
// row in a subsequent scheduled run via the overdue_notification_sent flag.
func (uc *DispatchOverdueNotificationsUseCase) Execute(ctx context.Context, referenceDate string, count int) error {
	if uc.source == nil || uc.creator == nil {
		slog.InfoContext(ctx, "DispatchOverdueNotifications: no source/creator wired (no-op)",
			"referenceDate", referenceDate, "count", count)
		return nil
	}

	installments := uc.source.LastMarked()
	if len(installments) == 0 {
		slog.InfoContext(ctx, "DispatchOverdueNotifications: nothing to dispatch",
			"referenceDate", referenceDate)
		return nil
	}

	type aggKey struct {
		ParticipantID string
		EventID       string
	}
	type aggValue struct {
		Total     int64
		Count     int
		OldestDue time.Time
	}
	agg := make(map[aggKey]*aggValue, len(installments))
	for _, inst := range installments {
		k := aggKey{ParticipantID: inst.ParticipantID, EventID: inst.EventID}
		v, ok := agg[k]
		if !ok {
			v = &aggValue{OldestDue: inst.DueDate}
			agg[k] = v
		}
		v.Total += inst.AmountCents
		v.Count++
		if inst.DueDate.Before(v.OldestDue) {
			v.OldestDue = inst.DueDate
		}
	}

	for k, v := range agg {
		in := portin.CreateNotificacaoInput{
			UsuarioID: k.ParticipantID,
			Tipo:      notifdomain.TipoNotificacaoPagamento,
			Titulo:    "Parcelas em atraso",
			Mensagem:  fmt.Sprintf("Você possui %d parcela(s) em atraso totalizando R$ %.2f.", v.Count, float64(v.Total)/100.0),
			Dados: map[string]string{
				"eventId":             k.EventID,
				"overdueInstallments": strconv.Itoa(v.Count),
				"totalOverdueCents":   strconv.FormatInt(v.Total, 10),
				"oldestDueDate":       v.OldestDue.Format("2006-01-02"),
				"reason":              "LembretePagamentoOverdue",
				"sourceReferenceDate": referenceDate,
			},
		}
		if _, err := uc.creator.Execute(ctx, in); err != nil {
			slog.WarnContext(ctx, "DispatchOverdueNotifications: failed to create notification",
				"participantID", k.ParticipantID, "eventID", k.EventID, "error", err)
			return fmt.Errorf("DispatchOverdueNotifications: %w", err)
		}
	}

	slog.InfoContext(ctx, "DispatchOverdueNotifications: dispatched",
		"referenceDate", referenceDate,
		"installments", len(installments),
		"recipients", len(agg),
	)
	return nil
}
