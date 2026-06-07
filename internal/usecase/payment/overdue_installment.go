package payment

import (
	"context"
	"log/slog"
)

// FindAndMarkOverdueInstallmentsUseCase marks installments whose due date is before
// referenceDate as VENCIDO and returns the affected count.
//
// TODO(spec-168): implementar nativo Go — atualmente retorna stub (0, nil).
// A implementação real deve consultar a coleção payment_installments no MongoDB,
// marcar como VENCIDO os documentos com due_date < referenceDate e status = PENDENTE,
// e retornar o count de documentos atualizados.
type FindAndMarkOverdueInstallmentsUseCase struct{}

// NewFindAndMarkOverdueInstallments constructs the use case stub.
func NewFindAndMarkOverdueInstallments() *FindAndMarkOverdueInstallmentsUseCase {
	return &FindAndMarkOverdueInstallmentsUseCase{}
}

// Execute marks past-due installments and returns the count of affected records.
func (uc *FindAndMarkOverdueInstallmentsUseCase) Execute(ctx context.Context, referenceDate string) (int, error) {
	// TODO(spec-168): implementar nativo Go — atualmente delega ao Java via stub.
	slog.InfoContext(ctx, "FindAndMarkOverdueInstallments stub",
		"referenceDate", referenceDate,
	)
	return 0, nil
}
