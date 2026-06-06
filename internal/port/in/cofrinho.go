package in

import (
	"context"

	"github.com/role-organizado/backend-go-role-organizado/internal/domain/cofrinho"
)

// CreateContribuicaoInput holds data needed to create a new cofrinho contribution.
type CreateContribuicaoInput struct {
	EventoID string
	GuestID  string
	Nome     string
	Mensagem string
	Valor    int64 // in centavos
}

// ConfirmarContribuicaoInput holds data needed to confirm a contribution via payment webhook.
type ConfirmarContribuicaoInput struct {
	ContribuicaoID   string
	WebhookPaymentID string
}

// CreateContribuicaoUseCase creates a new cofrinho contribution.
type CreateContribuicaoUseCase interface {
	Execute(ctx context.Context, in CreateContribuicaoInput) (*cofrinho.CofrinhoContribuicao, error)
}

// GetContribuicaoUseCase retrieves a single contribution by ID.
type GetContribuicaoUseCase interface {
	Execute(ctx context.Context, id string) (*cofrinho.CofrinhoContribuicao, error)
}

// ListContribuicoesUseCase lists all contributions for a given event.
type ListContribuicoesUseCase interface {
	Execute(ctx context.Context, eventoID string) ([]*cofrinho.CofrinhoContribuicao, error)
}

// ConfirmarContribuicaoUseCase confirms a pending contribution via payment webhook.
type ConfirmarContribuicaoUseCase interface {
	Execute(ctx context.Context, in ConfirmarContribuicaoInput) (*cofrinho.CofrinhoContribuicao, error)
}

// RemoverContribuicaoUseCase removes a PENDENTE contribution by ID.
// Only contributions in PENDENTE status can be removed; CONFIRMADO ones require a refund flow.
type RemoverContribuicaoUseCase interface {
	Execute(ctx context.Context, id string) error
}
