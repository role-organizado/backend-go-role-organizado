package payment

import (
	"context"
	"time"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ---- CreatePagamento ----

// CreatePagamento implements portin.CreatePagamentoUseCase.
type CreatePagamento struct {
	pagamentos portout.PagamentoMensalRepository
}

// NewCreatePagamento creates a new CreatePagamento use case.
func NewCreatePagamento(r portout.PagamentoMensalRepository) *CreatePagamento {
	return &CreatePagamento{pagamentos: r}
}

// Execute creates a new recurring payment.
func (uc *CreatePagamento) Execute(ctx context.Context, in portin.CreatePagamentoInput) (*domain.PagamentoMensal, error) {
	now := time.Now()
	p := &domain.PagamentoMensal{
		EventoID:        in.EventoID,
		UsuarioID:       in.UsuarioID,
		Descricao:       in.Descricao,
		Valor:           in.Valor,
		MetodoPagamento: in.MetodoPagamento,
		Status:          domain.StatusPagamentoPendente,
		DataVencimento:  in.DataVencimento,
		Observacao:      in.Observacao,
		CriadoEm:       now,
		UpdatedAt:       now,
	}
	return uc.pagamentos.Save(ctx, p)
}

// ---- GetPagamento ----

// GetPagamento implements portin.GetPagamentoUseCase.
type GetPagamento struct {
	pagamentos portout.PagamentoMensalRepository
}

// NewGetPagamento creates a new GetPagamento use case.
func NewGetPagamento(r portout.PagamentoMensalRepository) *GetPagamento {
	return &GetPagamento{pagamentos: r}
}

// Execute returns a payment by ID, enforcing ownership.
func (uc *GetPagamento) Execute(ctx context.Context, id, requesterID string) (*domain.PagamentoMensal, error) {
	p, err := uc.pagamentos.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !p.IsOwner(requesterID) {
		return nil, apierr.Forbidden("acesso negado")
	}
	return p, nil
}

// ---- ListPagamentos ----

// ListPagamentos implements portin.ListPagamentosUseCase.
type ListPagamentos struct {
	pagamentos portout.PagamentoMensalRepository
}

// NewListPagamentos creates a new ListPagamentos use case.
func NewListPagamentos(r portout.PagamentoMensalRepository) *ListPagamentos {
	return &ListPagamentos{pagamentos: r}
}

// Execute returns all payments for an event.
func (uc *ListPagamentos) Execute(ctx context.Context, eventoID, _ string) ([]domain.PagamentoMensal, error) {
	return uc.pagamentos.FindByEventoID(ctx, eventoID)
}

// ---- UpdatePagamento ----

// UpdatePagamento implements portin.UpdatePagamentoUseCase.
type UpdatePagamento struct {
	pagamentos portout.PagamentoMensalRepository
}

// NewUpdatePagamento creates a new UpdatePagamento use case.
func NewUpdatePagamento(r portout.PagamentoMensalRepository) *UpdatePagamento {
	return &UpdatePagamento{pagamentos: r}
}

// Execute applies partial updates to a payment.
func (uc *UpdatePagamento) Execute(ctx context.Context, id, requesterID string, in portin.UpdatePagamentoInput) (*domain.PagamentoMensal, error) {
	p, err := uc.pagamentos.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !p.IsOwner(requesterID) {
		return nil, apierr.Forbidden("acesso negado")
	}
	if p.IsPago() {
		return nil, apierr.BadRequest("pagamento já confirmado — não pode ser editado")
	}
	if in.Descricao != nil {
		p.Descricao = *in.Descricao
	}
	if in.Valor != nil {
		p.Valor = *in.Valor
	}
	if in.DataVencimento != nil {
		p.DataVencimento = *in.DataVencimento
	}
	if in.Observacao != nil {
		p.Observacao = *in.Observacao
	}
	p.UpdatedAt = time.Now()
	return uc.pagamentos.Update(ctx, p)
}

// ---- DeletePagamento ----

// DeletePagamento implements portin.DeletePagamentoUseCase.
type DeletePagamento struct {
	pagamentos portout.PagamentoMensalRepository
}

// NewDeletePagamento creates a new DeletePagamento use case.
func NewDeletePagamento(r portout.PagamentoMensalRepository) *DeletePagamento {
	return &DeletePagamento{pagamentos: r}
}

// Execute deletes a payment, enforcing ownership.
func (uc *DeletePagamento) Execute(ctx context.Context, id, requesterID string) error {
	p, err := uc.pagamentos.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if !p.IsOwner(requesterID) {
		return apierr.Forbidden("acesso negado")
	}
	return uc.pagamentos.DeleteByID(ctx, id)
}

// ---- ConfirmarPagamento ----

// ConfirmarPagamento implements portin.ConfirmarPagamentoUseCase.
type ConfirmarPagamento struct {
	pagamentos portout.PagamentoMensalRepository
}

// NewConfirmarPagamento creates a new ConfirmarPagamento use case.
func NewConfirmarPagamento(r portout.PagamentoMensalRepository) *ConfirmarPagamento {
	return &ConfirmarPagamento{pagamentos: r}
}

// Execute marks a payment as settled.
func (uc *ConfirmarPagamento) Execute(ctx context.Context, id, requesterID string, in portin.ConfirmarPagamentoInput) (*domain.PagamentoMensal, error) {
	p, err := uc.pagamentos.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !p.IsOwner(requesterID) {
		return nil, apierr.Forbidden("acesso negado")
	}
	if !p.CanPay() {
		return nil, apierr.Conflict("pagamento não pode ser confirmado nesse estado")
	}
	p.Status = domain.StatusPagamentoPago
	p.DataPagamento = &in.DataPagamento
	if in.Comprovante != "" {
		p.Comprovante = in.Comprovante
	}
	p.UpdatedAt = time.Now()
	return uc.pagamentos.Update(ctx, p)
}

// ---- UpsertConfigPagamento ----

// UpsertConfigPagamento implements portin.UpsertConfigPagamentoUseCase.
type UpsertConfigPagamento struct {
	configs portout.EventoConfigPagamentoRepository
}

// NewUpsertConfigPagamento creates a new UpsertConfigPagamento use case.
func NewUpsertConfigPagamento(r portout.EventoConfigPagamentoRepository) *UpsertConfigPagamento {
	return &UpsertConfigPagamento{configs: r}
}

// Execute creates or updates the payment config for an event.
func (uc *UpsertConfigPagamento) Execute(ctx context.Context, in portin.UpsertConfigPagamentoInput) (*domain.EventoConfigPagamento, error) {
	now := time.Now()

	existing, err := uc.configs.FindByEventoID(ctx, in.EventoID)
	if err == nil && existing != nil {
		// update existing
		existing.MetodosPagamento = in.MetodosPagamento
		existing.PrazoPagamento = in.PrazoPagamento
		existing.ChavePix = in.ChavePix
		existing.InstrucoesBoleto = in.InstrucoesBoleto
		existing.UpdatedAt = now
		return uc.configs.Update(ctx, existing)
	}

	// create new
	cfg := &domain.EventoConfigPagamento{
		EventoID:         in.EventoID,
		UsuarioID:        in.UsuarioID,
		MetodosPagamento: in.MetodosPagamento,
		PrazoPagamento:   in.PrazoPagamento,
		ChavePix:         in.ChavePix,
		InstrucoesBoleto: in.InstrucoesBoleto,
		CriadoEm:        now,
		UpdatedAt:        now,
	}
	return uc.configs.Save(ctx, cfg)
}

// ---- GetConfigPagamento ----

// GetConfigPagamento implements portin.GetConfigPagamentoUseCase.
type GetConfigPagamento struct {
	configs portout.EventoConfigPagamentoRepository
}

// NewGetConfigPagamento creates a new GetConfigPagamento use case.
func NewGetConfigPagamento(r portout.EventoConfigPagamentoRepository) *GetConfigPagamento {
	return &GetConfigPagamento{configs: r}
}

// Execute returns the payment config for an event.
func (uc *GetConfigPagamento) Execute(ctx context.Context, eventoID, _ string) (*domain.EventoConfigPagamento, error) {
	return uc.configs.FindByEventoID(ctx, eventoID)
}

// ---- ManageSavedCards ----

// ManageSavedCards implements portin.ManageSavedCardsUseCase.
type ManageSavedCards struct {
	cards portout.SavedCardRepository
}

// NewManageSavedCards creates a new ManageSavedCards use case.
func NewManageSavedCards(cards portout.SavedCardRepository) *ManageSavedCards {
	return &ManageSavedCards{cards: cards}
}

// List returns all active saved cards for the user.
func (uc *ManageSavedCards) List(ctx context.Context, userID string) ([]domain.SavedCard, error) {
	return uc.cards.FindByUserID(ctx, userID)
}

// Create saves a new credit card for the user.
func (uc *ManageSavedCards) Create(ctx context.Context, in portin.CreateSavedCardInput) (*domain.SavedCard, error) {
	now := time.Now()
	card := &domain.SavedCard{
		UserID:      in.UserID,
		LastFour:    in.LastFour,
		Brand:       in.Brand,
		HolderName:  in.HolderName,
		ExpiryMonth: in.ExpiryMonth,
		ExpiryYear:  in.ExpiryYear,
		IsDefault:   in.IsDefault,
		Active:      true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	return uc.cards.Save(ctx, card)
}

// Get retrieves a saved card by ID, enforcing ownership.
func (uc *ManageSavedCards) Get(ctx context.Context, id, userID string) (*domain.SavedCard, error) {
	return uc.cards.FindByID(ctx, id, userID)
}

// Delete soft-deletes a saved card, enforcing ownership.
func (uc *ManageSavedCards) Delete(ctx context.Context, id, userID string) error {
	return uc.cards.SoftDelete(ctx, id, userID)
}

// SetDefault designates a saved card as the user's default.
func (uc *ManageSavedCards) SetDefault(ctx context.Context, id, userID string) error {
	// Verify ownership first.
	if _, err := uc.cards.FindByID(ctx, id, userID); err != nil {
		return err
	}
	if err := uc.cards.ClearDefault(ctx, userID); err != nil {
		return err
	}
	card, err := uc.cards.FindByID(ctx, id, userID)
	if err != nil {
		return err
	}
	card.IsDefault = true
	card.UpdatedAt = time.Now()
	_, err = uc.cards.Save(ctx, card)
	return err
}

// ---- QueryInstallments ----

// QueryInstallments implements portin.QueryInstallmentsUseCase.
type QueryInstallments struct {
	installments portout.InstallmentQueryRepository
}

// NewQueryInstallments creates a new QueryInstallments use case.
func NewQueryInstallments(installments portout.InstallmentQueryRepository) *QueryInstallments {
	return &QueryInstallments{installments: installments}
}

// Query returns installments filtered by eventID, userID, and/or status.
func (uc *QueryInstallments) Query(ctx context.Context, in portin.QueryInstallmentsInput) ([]domain.Installment, error) {
	return uc.installments.FindByFilters(ctx, in.EventID, in.UserID, in.Status)
}

// GetForUser returns all installments for a user, optionally filtered by status.
func (uc *QueryInstallments) GetForUser(ctx context.Context, in portin.GetUserInstallmentsInput) ([]domain.Installment, error) {
	return uc.installments.FindByUserID(ctx, in.UserID, in.Status)
}
