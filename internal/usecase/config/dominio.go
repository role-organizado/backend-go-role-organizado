// Package config holds the Configuration domain use cases.
package config

import (
	"context"
	"log/slog"

	"github.com/role-organizado/backend-go-role-organizado/internal/domain/config"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
)

// ListDominios implements portin.ListDominiosUseCase.
type ListDominios struct {
	repo portout.DominioRepository
}

// NewListDominios creates a new ListDominios use case.
func NewListDominios(repo portout.DominioRepository) *ListDominios {
	return &ListDominios{repo: repo}
}

// Execute returns dominios matching the optional filters.
func (uc *ListDominios) Execute(ctx context.Context, in portin.ListDominiosInput) ([]config.Dominio, error) {
	slog.InfoContext(ctx, "listing dominios",
		"categoria", in.Categoria,
		"ativo", in.Ativo,
		"tipoEvento", in.TipoEvento,
	)

	var (
		dominios []config.Dominio
		err      error
	)

	switch {
	case in.Categoria == nil && in.Ativo == nil:
		dominios, err = uc.repo.FindAll(ctx)
	case in.Categoria != nil && in.Ativo == nil:
		dominios, err = uc.repo.FindByCategoria(ctx, *in.Categoria)
	case in.Categoria != nil && in.Ativo != nil:
		dominios, err = uc.repo.FindByCategoriaAndAtivo(ctx, *in.Categoria, *in.Ativo)
	default:
		// Only ativo specified — fetch all and filter in memory
		all, e := uc.repo.FindAll(ctx)
		if e != nil {
			return nil, e
		}
		for _, d := range all {
			if d.Ativo == *in.Ativo {
				dominios = append(dominios, d)
			}
		}
	}

	if err != nil {
		return nil, err
	}

	// Feature 008: additional filter by tipoEvento
	if in.TipoEvento != nil && *in.TipoEvento != "" {
		filtered := make([]config.Dominio, 0, len(dominios))
		for _, d := range dominios {
			if d.IsApplicableToEventType(*in.TipoEvento) {
				filtered = append(filtered, d)
			}
		}
		dominios = filtered
	}

	slog.InfoContext(ctx, "dominios listed", "count", len(dominios))
	return dominios, nil
}

// GetDominio implements portin.GetDominioUseCase.
type GetDominio struct {
	repo portout.DominioRepository
}

// NewGetDominio creates a new GetDominio use case.
func NewGetDominio(repo portout.DominioRepository) *GetDominio {
	return &GetDominio{repo: repo}
}

// Execute returns a single Dominio by categoria + chave.
func (uc *GetDominio) Execute(ctx context.Context, categoria, chave string) (*config.Dominio, error) {
	slog.InfoContext(ctx, "getting dominio", "categoria", categoria, "chave", chave)
	return uc.repo.FindByCategoriaAndChave(ctx, categoria, chave)
}

// UpsertDominio implements portin.UpsertDominioUseCase.
type UpsertDominio struct {
	repo portout.DominioRepository
}

// NewUpsertDominio creates a new UpsertDominio use case.
func NewUpsertDominio(repo portout.DominioRepository) *UpsertDominio {
	return &UpsertDominio{repo: repo}
}

// Execute creates or updates a Dominio.
func (uc *UpsertDominio) Execute(ctx context.Context, in portin.UpsertDominioInput) (*config.Dominio, error) {
	d := &config.Dominio{
		Categoria: in.Categoria,
		Chave:     in.Chave,
		Valor:     in.Valor,
		Descricao: in.Descricao,
		Icone:     in.Icone,
		Ordem:     in.Ordem,
		Ativo:     in.Ativo,
		Metadata:  in.Metadata,
	}
	if in.ID != nil {
		d.ID = *in.ID
	}
	slog.InfoContext(ctx, "upserting dominio", "categoria", d.Categoria, "chave", d.Chave)
	return uc.repo.Save(ctx, d)
}

// DeleteDominio implements portin.DeleteDominioUseCase.
type DeleteDominio struct {
	repo portout.DominioRepository
}

// NewDeleteDominio creates a new DeleteDominio use case.
func NewDeleteDominio(repo portout.DominioRepository) *DeleteDominio {
	return &DeleteDominio{repo: repo}
}

// Execute removes a Dominio by ID.
func (uc *DeleteDominio) Execute(ctx context.Context, id string) error {
	slog.InfoContext(ctx, "deleting dominio", "id", id)
	return uc.repo.DeleteByID(ctx, id)
}
