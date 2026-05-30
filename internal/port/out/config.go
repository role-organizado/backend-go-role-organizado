// Package out defines output ports (repository interfaces) for all domains.
package out

import (
	"context"

	"github.com/role-organizado/backend-go-role-organizado/internal/domain/config"
)

// DominioRepository is the output port for persisting and querying Dominio entities.
type DominioRepository interface {
	FindAll(ctx context.Context) ([]config.Dominio, error)
	FindByCategoria(ctx context.Context, categoria string) ([]config.Dominio, error)
	FindByCategoriaAndAtivo(ctx context.Context, categoria string, ativo bool) ([]config.Dominio, error)
	FindByCategoriaAndChave(ctx context.Context, categoria, chave string) (*config.Dominio, error)
	FindByID(ctx context.Context, id string) (*config.Dominio, error)
	Save(ctx context.Context, d *config.Dominio) (*config.Dominio, error)
	DeleteByID(ctx context.Context, id string) error
}

// ConfigSistemaRepository is the output port for persisting ConfiguracaoSistema.
type ConfigSistemaRepository interface {
	FindAll(ctx context.Context) ([]config.ConfiguracaoSistema, error)
	FindByChave(ctx context.Context, chave string) (*config.ConfiguracaoSistema, error)
	Save(ctx context.Context, c *config.ConfiguracaoSistema) (*config.ConfiguracaoSistema, error)
}
