package rateio

import (
	"context"
	"time"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/rateio"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ---- CreateRateio ----

// CreateRateio implements portin.CreateRateioUseCase.
type CreateRateio struct {
	rateios portout.RateioRepository
}

// NewCreateRateio creates a new CreateRateio use case.
func NewCreateRateio(r portout.RateioRepository) *CreateRateio {
	return &CreateRateio{rateios: r}
}

// Execute creates and persists a new rateio.
func (uc *CreateRateio) Execute(ctx context.Context, in portin.CreateRateioInput) (*domain.Rateio, error) {
	now := time.Now()
	itens := make([]domain.RateioItem, len(in.Itens))
	for i, it := range in.Itens {
		itens[i] = domain.RateioItem{
			Descricao:  it.Descricao,
			Valor:      it.Valor,
			Quantidade: it.Quantidade,
			Total:      it.Valor * float64(it.Quantidade),
			CriadoEm:  now,
			UpdatedAt:  now,
		}
	}
	r := &domain.Rateio{
		EventoID:            in.EventoID,
		UsuarioID:           in.UsuarioID,
		Tipo:                in.Tipo,
		Status:              domain.StatusRateioAberto,
		Descricao:           in.Descricao,
		ValorTotal:          in.ValorTotal,
		NumeroParticipantes: in.NumeroParticipantes,
		Itens:               itens,
		CriadoEm:            now,
		UpdatedAt:           now,
	}
	return uc.rateios.Save(ctx, r)
}

// ---- GetRateio ----

// GetRateio implements portin.GetRateioUseCase.
type GetRateio struct {
	rateios portout.RateioRepository
}

// NewGetRateio creates a new GetRateio use case.
func NewGetRateio(r portout.RateioRepository) *GetRateio {
	return &GetRateio{rateios: r}
}

// Execute retrieves a rateio by ID, enforcing ownership.
func (uc *GetRateio) Execute(ctx context.Context, id, requesterID string) (*domain.Rateio, error) {
	r, err := uc.rateios.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !r.IsOwner(requesterID) {
		return nil, apierr.Forbidden("acesso negado")
	}
	return r, nil
}

// ---- ListRateios ----

// ListRateios implements portin.ListRateiosUseCase.
type ListRateios struct {
	rateios portout.RateioRepository
}

// NewListRateios creates a new ListRateios use case.
func NewListRateios(r portout.RateioRepository) *ListRateios {
	return &ListRateios{rateios: r}
}

// Execute returns all rateios for an event. Access is implicitly scoped by eventoID.
func (uc *ListRateios) Execute(ctx context.Context, eventoID, _ string) ([]domain.Rateio, error) {
	return uc.rateios.FindByEventoID(ctx, eventoID)
}

// ---- UpdateRateio ----

// UpdateRateio implements portin.UpdateRateioUseCase.
type UpdateRateio struct {
	rateios portout.RateioRepository
}

// NewUpdateRateio creates a new UpdateRateio use case.
func NewUpdateRateio(r portout.RateioRepository) *UpdateRateio {
	return &UpdateRateio{rateios: r}
}

// Execute applies partial updates to an open rateio.
func (uc *UpdateRateio) Execute(ctx context.Context, id, requesterID string, in portin.UpdateRateioInput) (*domain.Rateio, error) {
	r, err := uc.rateios.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !r.IsOwner(requesterID) {
		return nil, apierr.Forbidden("acesso negado")
	}
	if !r.CanEdit() {
		return nil, apierr.BadRequest("rateio não pode ser editado — já foi fechado")
	}

	if in.Descricao != nil {
		r.Descricao = *in.Descricao
	}
	if in.ValorTotal != nil {
		r.ValorTotal = *in.ValorTotal
	}
	if in.NumeroParticipantes != nil {
		r.NumeroParticipantes = *in.NumeroParticipantes
	}
	if in.Itens != nil {
		now := time.Now()
		itens := make([]domain.RateioItem, len(in.Itens))
		for i, it := range in.Itens {
			itens[i] = domain.RateioItem{
				Descricao:  it.Descricao,
				Valor:      it.Valor,
				Quantidade: it.Quantidade,
				Total:      it.Valor * float64(it.Quantidade),
				UpdatedAt:  now,
			}
		}
		r.Itens = itens
	}

	r.UpdatedAt = time.Now()
	return uc.rateios.Update(ctx, r)
}

// ---- DeleteRateio ----

// DeleteRateio implements portin.DeleteRateioUseCase.
type DeleteRateio struct {
	rateios portout.RateioRepository
}

// NewDeleteRateio creates a new DeleteRateio use case.
func NewDeleteRateio(r portout.RateioRepository) *DeleteRateio {
	return &DeleteRateio{rateios: r}
}

// Execute removes a rateio, enforcing ownership.
func (uc *DeleteRateio) Execute(ctx context.Context, id, requesterID string) error {
	r, err := uc.rateios.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if !r.IsOwner(requesterID) {
		return apierr.Forbidden("acesso negado")
	}
	return uc.rateios.DeleteByID(ctx, id)
}

// ---- PreviewRateio ----

// PreviewRateio implements portin.PreviewRateioUseCase.
type PreviewRateio struct {
	rateios portout.RateioRepository
}

// NewPreviewRateio creates a new PreviewRateio use case.
func NewPreviewRateio(r portout.RateioRepository) *PreviewRateio {
	return &PreviewRateio{rateios: r}
}

// Execute calculates how costs are split among participants without persisting.
func (uc *PreviewRateio) Execute(ctx context.Context, id, requesterID string, participantes []string) (*domain.PreviewResult, error) {
	r, err := uc.rateios.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !r.IsOwner(requesterID) {
		return nil, apierr.Forbidden("acesso negado")
	}

	n := len(participantes)
	if n == 0 {
		n = r.NumeroParticipantes
	}
	if n == 0 {
		return nil, apierr.Unprocessable("número de participantes não definido")
	}

	total := r.ValorTotal
	if r.Tipo == domain.TipoRateioItens {
		total = 0
		for _, it := range r.Itens {
			total += it.Total
		}
	}

	result := &domain.PreviewResult{
		RateioID:   r.ID,
		TotalGeral: total,
	}

	switch r.Tipo {
	case domain.TipoRateioDivisao, domain.TipoRateioItens:
		valorPorPessoa := total / float64(n)
		pct := 100.0 / float64(n)
		for _, uid := range participantes {
			result.Participantes = append(result.Participantes, domain.PreviewParticipante{
				UsuarioID:  uid,
				Valor:      valorPorPessoa,
				Percentual: pct,
			})
		}
	case domain.TipoRateioPercentual:
		// For PERCENTUAL type, return equal split as default preview
		pct := 100.0 / float64(n)
		valorPorPessoa := total * (pct / 100)
		for _, uid := range participantes {
			result.Participantes = append(result.Participantes, domain.PreviewParticipante{
				UsuarioID:  uid,
				Valor:      valorPorPessoa,
				Percentual: pct,
			})
		}
	}

	return result, nil
}

// ---- FecharRateio ----

// FecharRateio implements portin.FecharRateioUseCase.
type FecharRateio struct {
	rateios     portout.RateioRepository
	fechamentos portout.RateioFechamentoRepository
}

// NewFecharRateio creates a new FecharRateio use case.
func NewFecharRateio(r portout.RateioRepository, f portout.RateioFechamentoRepository) *FecharRateio {
	return &FecharRateio{rateios: r, fechamentos: f}
}

// Execute closes a rateio and creates a versioned snapshot.
func (uc *FecharRateio) Execute(ctx context.Context, id, requesterID string, in portin.FecharRateioInput) (*domain.RateioFechamento, error) {
	r, err := uc.rateios.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !r.IsOwner(requesterID) {
		return nil, apierr.Forbidden("acesso negado")
	}
	if !r.CanEdit() {
		return nil, apierr.Conflict("rateio já foi fechado")
	}

	// Determine next version
	existing, _ := uc.fechamentos.FindByRateioID(ctx, id)
	versao := len(existing) + 1

	participantes := make([]domain.FechamentoParticipante, len(in.Participantes))
	total := 0.0
	for i, p := range in.Participantes {
		participantes[i] = domain.FechamentoParticipante{
			UsuarioID:  p.UsuarioID,
			Valor:      p.Valor,
			Percentual: p.Percentual,
		}
		total += p.Valor
	}

	f := &domain.RateioFechamento{
		RateioID:      r.ID,
		EventoID:      r.EventoID,
		Versao:        versao,
		ValorTotal:    total,
		Participantes: participantes,
		CriadoEm:     time.Now(),
	}

	saved, err := uc.fechamentos.Save(ctx, f)
	if err != nil {
		return nil, err
	}

	// Mark rateio as fechado
	r.Status = domain.StatusRateioFechado
	r.UpdatedAt = time.Now()
	if _, err := uc.rateios.Update(ctx, r); err != nil {
		return nil, err
	}

	return saved, nil
}

// ---- GetFechamentos ----

// GetFechamentos implements portin.GetFechamentosUseCase.
type GetFechamentos struct {
	rateios     portout.RateioRepository
	fechamentos portout.RateioFechamentoRepository
}

// NewGetFechamentos creates a new GetFechamentos use case.
func NewGetFechamentos(r portout.RateioRepository, f portout.RateioFechamentoRepository) *GetFechamentos {
	return &GetFechamentos{rateios: r, fechamentos: f}
}

// Execute returns all closings for a rateio, enforcing ownership.
func (uc *GetFechamentos) Execute(ctx context.Context, rateioID, requesterID string) ([]domain.RateioFechamento, error) {
	r, err := uc.rateios.FindByID(ctx, rateioID)
	if err != nil {
		return nil, err
	}
	if !r.IsOwner(requesterID) {
		return nil, apierr.Forbidden("acesso negado")
	}
	return uc.fechamentos.FindByRateioID(ctx, rateioID)
}
