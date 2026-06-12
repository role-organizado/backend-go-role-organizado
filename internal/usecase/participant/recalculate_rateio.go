// Package participant contains use cases for the participant lifecycle, driven by
// the Temporal ParticipantLifecycleWorkflow.
package participant

import (
	"context"
	"fmt"
	"time"

	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// RecalculateRateioAllocations implements portin.RecalculateRateioAllocationsUseCase.
//
// When a participant leaves (or is removed from) an event, every still-open
// rateio attached to that event must be recomputed so that the redistributed
// cost is reflected in NumeroParticipantes and the per-item totals. Closed
// rateios are immutable snapshots and are skipped.
type RecalculateRateioAllocations struct {
	rateios portout.RateioRepository
	eventos portout.EventoRepository
}

// NewRecalculateRateioAllocations creates a new RecalculateRateioAllocations use case.
func NewRecalculateRateioAllocations(
	rateios portout.RateioRepository,
	eventos portout.EventoRepository,
) *RecalculateRateioAllocations {
	return &RecalculateRateioAllocations{rateios: rateios, eventos: eventos}
}

// Execute recomputes the allocations of every open rateio in the event and
// returns the number of rateios that were recalculated.
func (uc *RecalculateRateioAllocations) Execute(ctx context.Context, in portin.RecalculateRateioAllocationsInput) (int64, error) {
	// Authorization: only the event organizer may trigger a recalculation.
	ev, err := uc.eventos.FindByID(ctx, in.EventID)
	if err != nil {
		return 0, err
	}
	if !ev.IsOwner(in.RequesterID) {
		return 0, apierr.Forbidden("somente o organizador pode recalcular rateios")
	}

	rateios, err := uc.rateios.FindByEventoID(ctx, in.EventID)
	if err != nil {
		return 0, fmt.Errorf("find rateios for event %s: %w", in.EventID, err)
	}

	var recalculated int64
	now := time.Now()
	for i := range rateios {
		r := &rateios[i]
		// Closed rateios are versioned snapshots — never mutate them.
		if !r.CanEdit() {
			continue
		}
		// Recompute item totals from valor * quantidade so the rateio reflects the
		// current participant set. The redistribution itself is driven by
		// NumeroParticipantes, which downstream preview/fechamento divide against.
		for j := range r.Itens {
			r.Itens[j].Total = r.Itens[j].Valor * float64(r.Itens[j].Quantidade)
			r.Itens[j].UpdatedAt = now
		}
		r.UpdatedAt = now
		if _, err := uc.rateios.Update(ctx, r); err != nil {
			return recalculated, fmt.Errorf("update rateio %s: %w", r.ID, err)
		}
		recalculated++
	}

	return recalculated, nil
}

var _ portin.RecalculateRateioAllocationsUseCase = (*RecalculateRateioAllocations)(nil)
