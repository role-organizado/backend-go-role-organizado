package config

import (
	"context"
	"log/slog"

	"github.com/role-organizado/backend-go-role-organizado/internal/domain/config"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
)

// GetConfigSistema implements portin.GetConfigSistemaUseCase.
type GetConfigSistema struct {
	repo portout.ConfigSistemaRepository
}

// NewGetConfigSistema creates a new GetConfigSistema use case.
func NewGetConfigSistema(repo portout.ConfigSistemaRepository) *GetConfigSistema {
	return &GetConfigSistema{repo: repo}
}

// Execute returns a ConfiguracaoSistema by chave.
func (uc *GetConfigSistema) Execute(ctx context.Context, chave string) (*config.ConfiguracaoSistema, error) {
	slog.InfoContext(ctx, "getting config sistema", "chave", chave)
	return uc.repo.FindByChave(ctx, chave)
}

// UpsertConfigSistema implements portin.UpsertConfigSistemaUseCase.
type UpsertConfigSistema struct {
	repo portout.ConfigSistemaRepository
}

// NewUpsertConfigSistema creates a new UpsertConfigSistema use case.
func NewUpsertConfigSistema(repo portout.ConfigSistemaRepository) *UpsertConfigSistema {
	return &UpsertConfigSistema{repo: repo}
}

// Execute creates or updates a ConfiguracaoSistema.
func (uc *UpsertConfigSistema) Execute(ctx context.Context, in portin.UpsertConfigSistemaInput) (*config.ConfiguracaoSistema, error) {
	slog.InfoContext(ctx, "upserting config sistema", "chave", in.Chave)
	c := &config.ConfiguracaoSistema{
		Chave:     in.Chave,
		Valor:     in.Valor,
		Descricao: in.Descricao,
		Ativo:     in.Ativo,
	}
	return uc.repo.Save(ctx, c)
}
