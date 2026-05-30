// Package in defines input ports (use-case interfaces) for all domains.
package in

import (
	"context"

	"github.com/role-organizado/backend-go-role-organizado/internal/domain/config"
)

// ListDominiosInput carries optional filter params for listing Dominio entries.
type ListDominiosInput struct {
	Categoria   *string
	Ativo       *bool
	TipoEvento  *string // Feature 008: filter cancellation policies by event type
}

// ListDominiosUseCase lists Dominio entries applying optional filters.
type ListDominiosUseCase interface {
	Execute(ctx context.Context, in ListDominiosInput) ([]config.Dominio, error)
}

// GetDominioUseCase returns a single Dominio by categoria+chave.
type GetDominioUseCase interface {
	Execute(ctx context.Context, categoria, chave string) (*config.Dominio, error)
}

// UpsertDominioInput is the payload for admin create/update of a Dominio.
type UpsertDominioInput struct {
	ID        *string
	Categoria string
	Chave     string
	Valor     string
	Descricao string
	Icone     string
	Ordem     int
	Ativo     bool
	Metadata  map[string]any
}

// UpsertDominioUseCase creates or updates a Dominio (admin).
type UpsertDominioUseCase interface {
	Execute(ctx context.Context, in UpsertDominioInput) (*config.Dominio, error)
}

// DeleteDominioUseCase removes a Dominio by ID (admin).
type DeleteDominioUseCase interface {
	Execute(ctx context.Context, id string) error
}

// UpsertConfigSistemaInput is the payload for admin upsert of a ConfiguracaoSistema.
type UpsertConfigSistemaInput struct {
	Chave     string
	Valor     map[string]any
	Descricao string
	Ativo     bool
}

// GetConfigSistemaUseCase retrieves a ConfiguracaoSistema by chave.
type GetConfigSistemaUseCase interface {
	Execute(ctx context.Context, chave string) (*config.ConfiguracaoSistema, error)
}

// UpsertConfigSistemaUseCase creates or updates a ConfiguracaoSistema (admin).
type UpsertConfigSistemaUseCase interface {
	Execute(ctx context.Context, in UpsertConfigSistemaInput) (*config.ConfiguracaoSistema, error)
}
