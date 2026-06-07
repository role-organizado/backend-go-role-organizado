package notification

import (
	"context"
	"log/slog"
)

// DispatchOverdueNotificationsUseCase sends overdue-installment notifications
// to the users whose installments were just marked as VENCIDO.
//
// TODO(spec-168): implementar nativo Go — atualmente retorna stub (nil).
// A implementação real deve recuperar os usuários afetados e enviar notificações
// via o canal preferido (push/email/in-app) usando o serviço de notificações.
type DispatchOverdueNotificationsUseCase struct{}

// NewDispatchOverdueNotifications constructs the use case stub.
func NewDispatchOverdueNotifications() *DispatchOverdueNotificationsUseCase {
	return &DispatchOverdueNotificationsUseCase{}
}

// Execute dispatches overdue-installment notifications for the given referenceDate and count.
func (uc *DispatchOverdueNotificationsUseCase) Execute(ctx context.Context, referenceDate string, count int) error {
	// TODO(spec-168): implementar nativo Go — atualmente delega ao Java via stub.
	slog.InfoContext(ctx, "DispatchOverdueNotifications stub",
		"referenceDate", referenceDate,
		"count", count,
	)
	return nil
}
