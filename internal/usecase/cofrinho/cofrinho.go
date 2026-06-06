package cofrinho

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/cofrinho"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ---- CreateContribuicao ----

// CreateContribuicao implements portin.CreateContribuicaoUseCase.
type CreateContribuicao struct {
	repo portout.CofrinhoRepository
}

// NewCreateContribuicao creates a new CreateContribuicao use case.
func NewCreateContribuicao(repo portout.CofrinhoRepository) *CreateContribuicao {
	return &CreateContribuicao{repo: repo}
}

// Execute creates a new cofrinho contribution with a placeholder PIX QR code.
func (uc *CreateContribuicao) Execute(ctx context.Context, in portin.CreateContribuicaoInput) (*domain.CofrinhoContribuicao, error) {
	if in.EventoID == "" {
		return nil, apierr.BadRequest("eventoId é obrigatório")
	}
	if in.Nome == "" {
		return nil, apierr.BadRequest("nome é obrigatório")
	}
	if in.Valor <= 0 {
		return nil, apierr.BadRequest("valor deve ser maior que zero")
	}

	now := time.Now()
	c := &domain.CofrinhoContribuicao{
		EventoID:  in.EventoID,
		GuestID:   in.GuestID,
		Nome:      in.Nome,
		Mensagem:  in.Mensagem,
		Valor:     in.Valor,
		Status:    domain.StatusPendente,
		PIXQRCode: fmt.Sprintf("PIX-PLACEHOLDER-%s-%d", uuid.New().String(), in.Valor),
		CriadoEm: now,
		UpdatedAt: now,
	}
	return uc.repo.Save(ctx, c)
}

// ---- GetContribuicao ----

// GetContribuicao implements portin.GetContribuicaoUseCase.
type GetContribuicao struct {
	repo portout.CofrinhoRepository
}

// NewGetContribuicao creates a new GetContribuicao use case.
func NewGetContribuicao(repo portout.CofrinhoRepository) *GetContribuicao {
	return &GetContribuicao{repo: repo}
}

// Execute retrieves a contribution by ID.
func (uc *GetContribuicao) Execute(ctx context.Context, id string) (*domain.CofrinhoContribuicao, error) {
	return uc.repo.FindByID(ctx, id)
}

// ---- ListContribuicoes ----

// ListContribuicoes implements portin.ListContribuicoesUseCase.
type ListContribuicoes struct {
	repo portout.CofrinhoRepository
}

// NewListContribuicoes creates a new ListContribuicoes use case.
func NewListContribuicoes(repo portout.CofrinhoRepository) *ListContribuicoes {
	return &ListContribuicoes{repo: repo}
}

// Execute lists all contributions for the given event.
func (uc *ListContribuicoes) Execute(ctx context.Context, eventoID string) ([]*domain.CofrinhoContribuicao, error) {
	if eventoID == "" {
		return nil, apierr.BadRequest("eventoId é obrigatório")
	}
	return uc.repo.FindByEventoID(ctx, eventoID)
}

// ---- ConfirmarContribuicao ----

// ConfirmarContribuicao implements portin.ConfirmarContribuicaoUseCase.
type ConfirmarContribuicao struct {
	repo portout.CofrinhoRepository
}

// NewConfirmarContribuicao creates a new ConfirmarContribuicao use case.
func NewConfirmarContribuicao(repo portout.CofrinhoRepository) *ConfirmarContribuicao {
	return &ConfirmarContribuicao{repo: repo}
}

// Execute confirms a pending contribution via payment webhook.
// Returns an error if the contribution is not in PENDENTE status.
func (uc *ConfirmarContribuicao) Execute(ctx context.Context, in portin.ConfirmarContribuicaoInput) (*domain.CofrinhoContribuicao, error) {
	c, err := uc.repo.FindByID(ctx, in.ContribuicaoID)
	if err != nil {
		return nil, err
	}
	if c.Status != domain.StatusPendente {
		return nil, apierr.Unprocessable(fmt.Sprintf("contribuição não está pendente (status: %s)", c.Status))
	}
	return uc.repo.UpdateStatus(ctx, in.ContribuicaoID, domain.StatusConfirmado, in.WebhookPaymentID)
}

// ---- RemoverContribuicao ----

// RemoverContribuicao implements portin.RemoverContribuicaoUseCase.
// Only PENDENTE contributions can be removed. CONFIRMADO contributions require
// a refund flow via the payment provider (Asaas) and cannot be deleted directly.
type RemoverContribuicao struct {
	repo portout.CofrinhoRepository
}

// NewRemoverContribuicao creates a new RemoverContribuicao use case.
func NewRemoverContribuicao(repo portout.CofrinhoRepository) *RemoverContribuicao {
	return &RemoverContribuicao{repo: repo}
}

// Execute removes a cofrinho contribution by ID, but only if its status is PENDENTE.
func (uc *RemoverContribuicao) Execute(ctx context.Context, id string) error {
	if id == "" {
		return apierr.BadRequest("id da contribuição é obrigatório")
	}
	c, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if c.Status != domain.StatusPendente {
		return apierr.Unprocessable(
			fmt.Sprintf("apenas contribuições com status PENDENTE podem ser removidas. Status atual: %s", c.Status),
		)
	}
	return uc.repo.DeleteByID(ctx, id)
}
