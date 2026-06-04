package event

import (
	"context"
	"time"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/event"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ---- CreateDraft ----

// CreateDraft implements portin.CreateDraftUseCase.
type CreateDraft struct {
	drafts portout.EventoDraftRepository
}

// NewCreateDraft creates a new CreateDraft use case.
func NewCreateDraft(d portout.EventoDraftRepository) *CreateDraft {
	return &CreateDraft{drafts: d}
}

// Execute creates an empty draft owned by the given user.
func (uc *CreateDraft) Execute(ctx context.Context, usuarioID string) (*domain.EventoDraft, error) {
	now := time.Now()
	d := &domain.EventoDraft{
		UsuarioID:       usuarioID,
		EtapaAtual:      0,
		EtapasCompletas: []int{},
		CriadoEm:        now,
		UpdatedAt:       now,
	}
	return uc.drafts.Save(ctx, d)
}

// ---- GetDraft ----

// GetDraft implements portin.GetDraftUseCase.
type GetDraft struct {
	drafts portout.EventoDraftRepository
}

// NewGetDraft creates a new GetDraft use case.
func NewGetDraft(d portout.EventoDraftRepository) *GetDraft {
	return &GetDraft{drafts: d}
}

// Execute retrieves a draft by ID, enforcing ownership.
func (uc *GetDraft) Execute(ctx context.Context, id, requesterID string) (*domain.EventoDraft, error) {
	d, err := uc.drafts.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !d.IsOwner(requesterID) {
		return nil, apierr.Forbidden("acesso negado")
	}
	return d, nil
}

// ---- ListDrafts ----

// ListDrafts implements portin.ListDraftsUseCase.
type ListDrafts struct {
	drafts portout.EventoDraftRepository
}

// NewListDrafts creates a new ListDrafts use case.
func NewListDrafts(d portout.EventoDraftRepository) *ListDrafts {
	return &ListDrafts{drafts: d}
}

// Execute returns all drafts for the given user.
func (uc *ListDrafts) Execute(ctx context.Context, usuarioID string) ([]domain.EventoDraft, error) {
	return uc.drafts.FindByUsuarioID(ctx, usuarioID)
}

// ---- UpdateDraft ----

// UpdateDraft implements portin.UpdateDraftUseCase (auto-save).
type UpdateDraft struct {
	drafts portout.EventoDraftRepository
}

// NewUpdateDraft creates a new UpdateDraft use case.
func NewUpdateDraft(d portout.EventoDraftRepository) *UpdateDraft {
	return &UpdateDraft{drafts: d}
}

// Execute applies partial updates to a draft, checking for conflicts.
func (uc *UpdateDraft) Execute(ctx context.Context, id, requesterID string, in portin.UpsertDraftInput) (*domain.EventoDraft, error) {
	d, err := uc.drafts.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !d.IsOwner(requesterID) {
		return nil, apierr.Forbidden("acesso negado")
	}
	// Optimistic concurrency check
	if in.LastReadAt != nil && d.HasConflict(*in.LastReadAt) {
		return nil, apierr.Conflict("draft foi modificado por outro processo. Recarregue antes de salvar.")
	}

	// Apply partial updates — etapa 0
	if in.Nome != nil {
		d.Nome = *in.Nome
	}
	if in.Tipo != nil {
		d.Tipo = *in.Tipo
	}
	if in.Data != nil {
		d.Data = in.Data
	}
	if in.Descricao != nil {
		d.Descricao = *in.Descricao
	}
	if in.Local != nil {
		d.Local = *in.Local
	}

	// Etapa 1
	if in.ConvidadosIDs != nil {
		d.ConvidadosIDs = in.ConvidadosIDs
	}
	if in.PoliticaConvidados != nil {
		d.PoliticaConvidados = *in.PoliticaConvidados
	}
	if in.LimiteConvidados != nil {
		d.LimiteConvidados = in.LimiteConvidados
	}

	// Etapa 2
	if in.RateiosHabilitado != nil {
		d.RateiosHabilitado = *in.RateiosHabilitado
	}
	if in.RateiosItens != nil {
		itens := make([]domain.RateioItem, len(in.RateiosItens))
		for i, ri := range in.RateiosItens {
			itens[i] = domain.RateioItem{
				Descricao:  ri.Descricao,
				Valor:      ri.Valor,
				Quantidade: ri.Quantidade,
			}
		}
		d.RateiosItens = itens
	}
	if in.TipoDivisaoRateio != nil {
		d.TipoDivisaoRateio = *in.TipoDivisaoRateio
	}

	// Etapa 3
	if in.PagamentosHabilitado != nil {
		d.PagamentosHabilitado = *in.PagamentosHabilitado
	}
	if in.MetodosPagamento != nil {
		d.MetodosPagamento = in.MetodosPagamento
	}
	if in.PrazoPagamento != nil {
		d.PrazoPagamento = in.PrazoPagamento
	}

	// Etapa 4
	if in.RegrasCustomizadas != nil {
		d.RegrasCustomizadas = *in.RegrasCustomizadas
	}
	if in.PoliticaCancelamento != nil {
		d.PoliticaCancelamento = *in.PoliticaCancelamento
	}

	// Wizard state
	if in.EtapaAtual != nil {
		d.EtapaAtual = *in.EtapaAtual
	}
	if in.EtapasCompletas != nil {
		d.EtapasCompletas = in.EtapasCompletas
	}

	d.UpdatedAt = time.Now()
	return uc.drafts.Update(ctx, d)
}

// ---- DeleteDraft ----

// DeleteDraft implements portin.DeleteDraftUseCase.
type DeleteDraft struct {
	drafts portout.EventoDraftRepository
}

// NewDeleteDraft creates a new DeleteDraft use case.
func NewDeleteDraft(d portout.EventoDraftRepository) *DeleteDraft {
	return &DeleteDraft{drafts: d}
}

// Execute deletes a draft, enforcing ownership.
func (uc *DeleteDraft) Execute(ctx context.Context, id, requesterID string) error {
	d, err := uc.drafts.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if !d.IsOwner(requesterID) {
		return apierr.Forbidden("acesso negado")
	}
	return uc.drafts.DeleteByID(ctx, id)
}

// ---- ValidateDraft ----

// ValidateDraft implements portin.ValidateDraftUseCase.
type ValidateDraft struct {
	drafts portout.EventoDraftRepository
}

// NewValidateDraft creates a new ValidateDraft use case.
func NewValidateDraft(d portout.EventoDraftRepository) *ValidateDraft {
	return &ValidateDraft{drafts: d}
}

// Execute validates whether a draft is complete enough to be published.
// Always returns the full list of DraftValidationResult (one per required field).
// Returns a non-nil error only for system-level failures (not found, forbidden).
func (uc *ValidateDraft) Execute(ctx context.Context, draftID, requesterID string) ([]portin.DraftValidationResult, error) {
	d, err := uc.drafts.FindByID(ctx, draftID)
	if err != nil {
		return nil, err
	}
	if !d.IsOwner(requesterID) {
		return nil, apierr.Forbidden("acesso negado")
	}

	results := []portin.DraftValidationResult{
		validateCampo("nome", d.Nome != "", "Título do evento é obrigatório"),
		validateCampo("tipo", d.Tipo != "", "Tipo do evento é obrigatório"),
		validateCampo("data", d.Data != nil, "Data do evento é obrigatória"),
		validateCampo("local", d.Local != "", "Local do evento é obrigatório"),
	}

	return results, nil
}

// validateCampo builds a DraftValidationResult for a single required field.
func validateCampo(campo string, valid bool, msgErro string) portin.DraftValidationResult {
	if valid {
		return portin.DraftValidationResult{Campo: campo, Valido: true}
	}
	return portin.DraftValidationResult{Campo: campo, Valido: false, Mensagem: msgErro}
}

// ---- PublishDraft ----

// PublishDraft implements portin.PublishDraftUseCase.
type PublishDraft struct {
	drafts  portout.EventoDraftRepository
	eventos portout.EventoRepository
}

// NewPublishDraft creates a new PublishDraft use case.
func NewPublishDraft(d portout.EventoDraftRepository, e portout.EventoRepository) *PublishDraft {
	return &PublishDraft{drafts: d, eventos: e}
}

// Execute converts a completed draft into a published event.
func (uc *PublishDraft) Execute(ctx context.Context, draftID, requesterID string) (*domain.Evento, error) {
	d, err := uc.drafts.FindByID(ctx, draftID)
	if err != nil {
		return nil, err
	}
	if !d.IsOwner(requesterID) {
		return nil, apierr.Forbidden("acesso negado")
	}

	var data time.Time
	if d.Data != nil {
		data = *d.Data
	}

	now := time.Now()
	evt := &domain.Evento{
		UsuarioID:            d.UsuarioID,
		Nome:                 d.Nome,
		Tipo:                 d.Tipo,
		Data:                 data,
		Descricao:            d.Descricao,
		Local:                d.Local,
		Status:               domain.EventoStatusPublicado,
		ConvidadosIDs:        d.ConvidadosIDs,
		PoliticaConvidados:   d.PoliticaConvidados,
		LimiteConvidados:     d.LimiteConvidados,
		RateiosHabilitado:    d.RateiosHabilitado,
		TipoDivisaoRateio:    d.TipoDivisaoRateio,
		PagamentosHabilitado: d.PagamentosHabilitado,
		MetodosPagamento:     d.MetodosPagamento,
		PrazoPagamento:       d.PrazoPagamento,
		RegrasCustomizadas:   d.RegrasCustomizadas,
		PoliticaCancelamento: d.PoliticaCancelamento,
		CriadoEm:             now,
		UpdatedAt:            now,
	}
	saved, err := uc.eventos.Save(ctx, evt)
	if err != nil {
		return nil, err
	}
	// Remove the draft after publishing
	_ = uc.drafts.DeleteByID(ctx, draftID)
	return saved, nil
}
