package event

import (
	"context"
	"time"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/event"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
	"github.com/role-organizado/backend-go-role-organizado/pkg/tracing"
)

// ---- CreateEvento ----

// CreateEvento implements portin.CreateEventoUseCase.
type CreateEvento struct {
	eventos      portout.EventoRepository
	participantes portout.ParticipanteRepository
}

// NewCreateEvento creates a new CreateEvento use case.
// participantes is used to auto-register the creator as ORGANIZADOR after the event is saved,
// matching Java's behaviour and fixing downstream 403 errors on ownership checks.
func NewCreateEvento(e portout.EventoRepository, p portout.ParticipanteRepository) *CreateEvento {
	return &CreateEvento{eventos: e, participantes: p}
}

// Execute creates a new event owned by the given user and registers the creator
// as an ORGANIZADOR participant in the participants collection.
func (uc *CreateEvento) Execute(ctx context.Context, in portin.CreateEventoInput) (*domain.Evento, error) {
	ctx, span := tracing.StartSpan(ctx, "usecase.event.create", tracing.UserID(in.UsuarioID))
	defer span.End()
	now := time.Now()
	e := &domain.Evento{
		UsuarioID:            in.UsuarioID,
		Nome:                 in.Nome,
		Tipo:                 in.Tipo,
		Data:                 in.Data,
		Descricao:            in.Descricao,
		Local:                in.Local,
		FotoURL:              in.FotoURL,
		Status:               domain.EventoStatusPublicado,
		ConvidadosIDs:        in.ConvidadosIDs,
		PoliticaConvidados:   in.PoliticaConvidados,
		LimiteConvidados:     in.LimiteConvidados,
		RateiosHabilitado:    in.RateiosHabilitado,
		TipoDivisaoRateio:    in.TipoDivisaoRateio,
		PagamentosHabilitado: in.PagamentosHabilitado,
		MetodosPagamento:     in.MetodosPagamento,
		PrazoPagamento:       in.PrazoPagamento,
		RegrasCustomizadas:   in.RegrasCustomizadas,
		PoliticaCancelamento: in.PoliticaCancelamento,
		CriadoEm:             now,
		UpdatedAt:            now,
	}
	saved, err := uc.eventos.Save(ctx, e)
	if err != nil {
		return nil, err
	}

	// Auto-register creator as ORGANIZADOR in the participants collection.
	// Failure here is non-fatal — log via span but don't roll back the event.
	if regErr := uc.participantes.SaveOrganizador(ctx, saved.ID, in.UsuarioID); regErr != nil {
		tracing.RecordError(span, regErr)
	}

	return saved, nil
}

// ---- GetEvento ----

// GetEvento implements portin.GetEventoUseCase.
type GetEvento struct {
	eventos portout.EventoRepository
}

// NewGetEvento creates a new GetEvento use case.
func NewGetEvento(e portout.EventoRepository) *GetEvento {
	return &GetEvento{eventos: e}
}

// Execute retrieves an event by ID.
func (uc *GetEvento) Execute(ctx context.Context, id string) (*domain.Evento, error) {
	return uc.eventos.FindByID(ctx, id)
}

// ---- ListEventos ----

// ListEventos implements portin.ListEventosUseCase.
type ListEventos struct {
	eventos portout.EventoRepository
}

// NewListEventos creates a new ListEventos use case.
func NewListEventos(e portout.EventoRepository) *ListEventos {
	return &ListEventos{eventos: e}
}

// Execute lists events, filtered by usuarioID when provided.
func (uc *ListEventos) Execute(ctx context.Context, usuarioID *string, page, pageSize int) ([]domain.Evento, int64, error) {
	if usuarioID != nil && *usuarioID != "" {
		return uc.eventos.FindByUsuarioID(ctx, *usuarioID, page, pageSize)
	}
	return uc.eventos.FindAll(ctx, page, pageSize)
}

// ---- UpdateEvento ----

// UpdateEvento implements portin.UpdateEventoUseCase.
type UpdateEvento struct {
	eventos portout.EventoRepository
}

// NewUpdateEvento creates a new UpdateEvento use case.
func NewUpdateEvento(e portout.EventoRepository) *UpdateEvento {
	return &UpdateEvento{eventos: e}
}

// Execute updates an event, enforcing ownership.
func (uc *UpdateEvento) Execute(ctx context.Context, id, requesterID string, in portin.UpdateEventoInput) (*domain.Evento, error) {
	evt, err := uc.eventos.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !evt.IsOwner(requesterID) {
		return nil, apierr.Forbidden("acesso negado")
	}
	if !evt.CanEdit() {
		return nil, apierr.BadRequest("evento não pode ser editado no status atual")
	}

	// Apply partial updates
	if in.Nome != "" {
		evt.Nome = in.Nome
	}
	if in.Tipo != "" {
		evt.Tipo = in.Tipo
	}
	if in.Data != nil {
		evt.Data = *in.Data
	}
	if in.Descricao != "" {
		evt.Descricao = in.Descricao
	}
	if in.Local != "" {
		evt.Local = in.Local
	}
	if in.FotoURL != "" {
		evt.FotoURL = in.FotoURL
	}
	if in.PoliticaConvidados != "" {
		evt.PoliticaConvidados = in.PoliticaConvidados
	}
	if in.LimiteConvidados != nil {
		evt.LimiteConvidados = in.LimiteConvidados
	}
	if in.RateiosHabilitado != nil {
		evt.RateiosHabilitado = *in.RateiosHabilitado
	}
	if in.TipoDivisaoRateio != "" {
		evt.TipoDivisaoRateio = in.TipoDivisaoRateio
	}
	if in.PagamentosHabilitado != nil {
		evt.PagamentosHabilitado = *in.PagamentosHabilitado
	}
	if len(in.MetodosPagamento) > 0 {
		evt.MetodosPagamento = in.MetodosPagamento
	}
	if in.PrazoPagamento != nil {
		evt.PrazoPagamento = in.PrazoPagamento
	}
	if in.RegrasCustomizadas != "" {
		evt.RegrasCustomizadas = in.RegrasCustomizadas
	}
	if in.PoliticaCancelamento != "" {
		evt.PoliticaCancelamento = in.PoliticaCancelamento
	}
	evt.UpdatedAt = time.Now()
	return uc.eventos.Update(ctx, evt)
}

// ---- DeleteEvento ----

// DeleteEvento implements portin.DeleteEventoUseCase.
type DeleteEvento struct {
	eventos portout.EventoRepository
}

// NewDeleteEvento creates a new DeleteEvento use case.
func NewDeleteEvento(e portout.EventoRepository) *DeleteEvento {
	return &DeleteEvento{eventos: e}
}

// Execute deletes an event, enforcing ownership.
func (uc *DeleteEvento) Execute(ctx context.Context, id, requesterID string) error {
	ctx, span := tracing.StartSpan(ctx, "usecase.event.delete", tracing.EventID(id), tracing.UserID(requesterID))
	defer span.End()
	evt, err := uc.eventos.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if !evt.IsOwner(requesterID) {
		return apierr.Forbidden("acesso negado")
	}
	return uc.eventos.DeleteByID(ctx, id)
}

// ---- ListEventosByUsuario ----

// ListEventosByUsuario implements portin.ListEventosByUsuarioUseCase.
type ListEventosByUsuario struct {
	eventos portout.EventoRepository
}

// NewListEventosByUsuario creates a new ListEventosByUsuario use case.
func NewListEventosByUsuario(e portout.EventoRepository) *ListEventosByUsuario {
	return &ListEventosByUsuario{eventos: e}
}

// Execute lists events belonging to the given user (cursor pagination).
// Enforces ownership: requester must be the same user unless empty (admin).
func (uc *ListEventosByUsuario) Execute(ctx context.Context, in portin.ListEventosByUsuarioInput) (portout.EventosCursorPage, error) {
	ctx, span := tracing.StartSpan(ctx, "usecase.event.listByUsuario", tracing.UserID(in.UsuarioID))
	defer span.End()
	if in.RequesterID != "" && in.RequesterID != in.UsuarioID {
		return portout.EventosCursorPage{}, apierr.Forbidden("acesso negado: você só pode listar seus próprios eventos")
	}
	filtros := portout.EventoQueryFiltros{
		Status:        in.Status,
		Tipo:          in.Tipo,
		DataInicioGte: in.DataInicioGte,
		DataInicioLte: in.DataInicioLte,
		Cursor:        in.Cursor,
		Limit:         in.Limit,
	}
	return uc.eventos.FindByUsuarioIDCursor(ctx, in.UsuarioID, filtros)
}
