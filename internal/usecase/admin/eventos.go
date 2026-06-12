package admin

import (
	"context"
	"log/slog"
	"strings"

	"github.com/role-organizado/backend-go-role-organizado/internal/domain/event"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ListEventosAdmin implements portin.ListEventosAdminUseCase.
type ListEventosAdmin struct {
	eventos portout.EventoRepository
}

// NewListEventosAdmin creates a new ListEventosAdmin use case.
func NewListEventosAdmin(e portout.EventoRepository) *ListEventosAdmin {
	return &ListEventosAdmin{eventos: e}
}

// Execute lists events for the admin surface, applying optional filters on the
// requested page. When no filters are supplied TotalCount reflects the full
// collection; with filters it reflects the filtered page.
func (uc *ListEventosAdmin) Execute(ctx context.Context, in portin.ListEventosAdminInput) (portin.EventosAdminPage, error) {
	page := in.Page
	if page < 0 {
		page = 0
	}
	pageSize := in.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 50 {
		pageSize = 50
	}

	eventos, total, err := uc.eventos.FindAll(ctx, page, pageSize)
	if err != nil {
		return portin.EventosAdminPage{}, err
	}

	hasFilter := in.Status != "" || in.Tipo != "" || in.Nome != ""
	if hasFilter {
		filtered := eventos[:0]
		for _, ev := range eventos {
			if in.Status != "" && string(ev.Status) != in.Status {
				continue
			}
			if in.Tipo != "" && ev.Tipo != in.Tipo {
				continue
			}
			if in.Nome != "" && !strings.Contains(strings.ToLower(ev.Nome), strings.ToLower(in.Nome)) {
				continue
			}
			filtered = append(filtered, ev)
		}
		eventos = filtered
		total = int64(len(filtered))
	}

	return portin.EventosAdminPage{
		Eventos:    eventos,
		TotalCount: total,
		HasMore:    !hasFilter && int64((page+1)*pageSize) < total,
		Page:       page,
		PageSize:   pageSize,
	}, nil
}

// GetEventoCompletoAdmin implements portin.GetEventoCompletoAdminUseCase.
type GetEventoCompletoAdmin struct {
	eventos  portout.EventoRepository
	usuarios portout.UsuarioRepository
	finance  portout.FinanceSummaryRepository
	outbound portout.OutboundRequestRepository
}

// NewGetEventoCompletoAdmin creates a new GetEventoCompletoAdmin use case.
func NewGetEventoCompletoAdmin(
	e portout.EventoRepository,
	u portout.UsuarioRepository,
	f portout.FinanceSummaryRepository,
	o portout.OutboundRequestRepository,
) *GetEventoCompletoAdmin {
	return &GetEventoCompletoAdmin{eventos: e, usuarios: u, finance: f, outbound: o}
}

// Execute composes the full admin view of an event. The organizer, finance
// summary, and outbound sections degrade gracefully to nil/empty when absent.
func (uc *GetEventoCompletoAdmin) Execute(ctx context.Context, eventoID string) (*portin.EventoCompletoAdmin, error) {
	ev, err := uc.eventos.FindByID(ctx, eventoID)
	if err != nil {
		return nil, err
	}

	out := &portin.EventoCompletoAdmin{Evento: ev}

	if ev.UsuarioID != "" {
		if org, err := uc.usuarios.FindByID(ctx, ev.UsuarioID); err == nil {
			out.Organizador = org
		}
	}
	if fin, err := uc.finance.FindByEventID(ctx, eventoID); err == nil {
		out.Finance = fin
	}
	if pending, err := uc.outbound.FindPendingByEventID(ctx, eventoID); err == nil {
		out.PendingOutbound = pending
	}

	return out, nil
}

// CancelarEventoAdmin implements portin.CancelarEventoAdminUseCase.
type CancelarEventoAdmin struct {
	eventos portout.EventoRepository
}

// NewCancelarEventoAdmin creates a new CancelarEventoAdmin use case.
func NewCancelarEventoAdmin(e portout.EventoRepository) *CancelarEventoAdmin {
	return &CancelarEventoAdmin{eventos: e}
}

// Execute cancels an event (admin override). Already-cancelled events are rejected.
func (uc *CancelarEventoAdmin) Execute(ctx context.Context, in portin.CancelarEventoAdminInput) (*event.Evento, error) {
	if strings.TrimSpace(in.Motivo) == "" {
		return nil, apierr.BadRequest("motivo é obrigatório")
	}
	slog.InfoContext(ctx, "admin cancelar evento", "eventoId", in.EventoID, "adminUserId", in.AdminUserID)

	ev, err := uc.eventos.FindByID(ctx, in.EventoID)
	if err != nil {
		return nil, err
	}
	if ev.Status == event.EventoStatusCancelado {
		return nil, apierr.Conflict("evento já está cancelado")
	}
	ev.Status = event.EventoStatusCancelado
	if ev.RegrasCustomizadas == "" {
		ev.RegrasCustomizadas = "Cancelado pelo admin: " + in.Motivo
	}
	return uc.eventos.Update(ctx, ev)
}

// FecharFinanceiroAdmin implements portin.FecharFinanceiroAdminUseCase.
type FecharFinanceiroAdmin struct {
	eventos portout.EventoRepository
}

// NewFecharFinanceiroAdmin creates a new FecharFinanceiroAdmin use case.
func NewFecharFinanceiroAdmin(e portout.EventoRepository) *FecharFinanceiroAdmin {
	return &FecharFinanceiroAdmin{eventos: e}
}

// Execute concludes an event's financial cycle by moving it to the FINALIZADO phase.
func (uc *FecharFinanceiroAdmin) Execute(ctx context.Context, eventoID string) (*event.Evento, error) {
	slog.InfoContext(ctx, "admin fechar financeiro", "eventoId", eventoID)
	ev, err := uc.eventos.FindByID(ctx, eventoID)
	if err != nil {
		return nil, err
	}
	ev.Fase = event.FaseFinalizado
	return uc.eventos.Update(ctx, ev)
}
