package out

import (
	"context"

	"github.com/role-organizado/backend-go-role-organizado/internal/domain/cofrinho"
)

// CofrinhoRepository defines persistence operations for cofrinho contributions.
type CofrinhoRepository interface {
	Save(ctx context.Context, c *cofrinho.CofrinhoContribuicao) (*cofrinho.CofrinhoContribuicao, error)
	FindByID(ctx context.Context, id string) (*cofrinho.CofrinhoContribuicao, error)
	FindByEventoID(ctx context.Context, eventoID string) ([]*cofrinho.CofrinhoContribuicao, error)
	UpdateStatus(ctx context.Context, id string, status cofrinho.CofrinhoStatus, webhookPaymentID string) (*cofrinho.CofrinhoContribuicao, error)
	// DeleteByID permanently removes a contribution by ID.
	DeleteByID(ctx context.Context, id string) error
}
