package admin

import (
	"context"
	"log/slog"
	"sort"

	domainadmin "github.com/role-organizado/backend-go-role-organizado/internal/domain/admin"
	"github.com/role-organizado/backend-go-role-organizado/internal/domain/config"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// categoriaPoliticaCancelamento is the dominios.categoria that holds cancellation policies.
const categoriaPoliticaCancelamento = "politica_cancelamento"

// GetDominioByID implements portin.GetDominioByIDUseCase.
type GetDominioByID struct {
	repo portout.DominioRepository
}

// NewGetDominioByID creates a new GetDominioByID use case.
func NewGetDominioByID(r portout.DominioRepository) *GetDominioByID {
	return &GetDominioByID{repo: r}
}

// Execute returns a Dominio by ID.
func (uc *GetDominioByID) Execute(ctx context.Context, id string) (*config.Dominio, error) {
	return uc.repo.FindByID(ctx, id)
}

// ToggleDominio implements portin.ToggleDominioUseCase.
type ToggleDominio struct {
	repo portout.DominioRepository
}

// NewToggleDominio creates a new ToggleDominio use case.
func NewToggleDominio(r portout.DominioRepository) *ToggleDominio {
	return &ToggleDominio{repo: r}
}

// Execute flips the ativo flag of a Dominio.
func (uc *ToggleDominio) Execute(ctx context.Context, id string) (*config.Dominio, error) {
	slog.InfoContext(ctx, "toggle dominio", "id", id)
	d, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	d.Ativo = !d.Ativo
	return uc.repo.Save(ctx, d)
}

// ListDominioCategorias implements portin.ListDominioCategoriasUseCase.
type ListDominioCategorias struct {
	repo portout.DominioRepository
}

// NewListDominioCategorias creates a new ListDominioCategorias use case.
func NewListDominioCategorias(r portout.DominioRepository) *ListDominioCategorias {
	return &ListDominioCategorias{repo: r}
}

// Execute returns the distinct, sorted categorias across all dominios.
func (uc *ListDominioCategorias) Execute(ctx context.Context) ([]string, error) {
	all, err := uc.repo.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(all))
	cats := make([]string, 0, len(all))
	for i := range all {
		c := all[i].Categoria
		if c == "" {
			continue
		}
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		cats = append(cats, c)
	}
	sort.Strings(cats)
	return cats, nil
}

// ListCancelamentoPolicies implements portin.ListCancelamentoPoliciesUseCase.
type ListCancelamentoPolicies struct {
	repo portout.DominioRepository
}

// NewListCancelamentoPolicies creates a new ListCancelamentoPolicies use case.
func NewListCancelamentoPolicies(r portout.DominioRepository) *ListCancelamentoPolicies {
	return &ListCancelamentoPolicies{repo: r}
}

// Execute lists the politica_cancelamento dominios.
func (uc *ListCancelamentoPolicies) Execute(ctx context.Context) ([]config.Dominio, error) {
	return uc.repo.FindByCategoria(ctx, categoriaPoliticaCancelamento)
}

// UpdateCancelamentoTiers implements portin.UpdateCancelamentoTiersUseCase.
type UpdateCancelamentoTiers struct {
	repo portout.DominioRepository
}

// NewUpdateCancelamentoTiers creates a new UpdateCancelamentoTiers use case.
func NewUpdateCancelamentoTiers(r portout.DominioRepository) *UpdateCancelamentoTiers {
	return &UpdateCancelamentoTiers{repo: r}
}

// Execute validates and replaces metadata.tiers of a cancellation policy.
func (uc *UpdateCancelamentoTiers) Execute(ctx context.Context, id string, tiers []domainadmin.CancellationTier) (*config.Dominio, error) {
	slog.InfoContext(ctx, "update cancelamento tiers", "id", id, "tiers", len(tiers))
	if len(tiers) == 0 {
		return nil, apierr.BadRequest("tiers não pode ser vazio")
	}
	for i, t := range tiers {
		if !t.Valid() {
			return nil, apierr.BadRequestWithDetails("tier inválido", map[string]any{"index": i})
		}
	}

	d, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if d.Categoria != categoriaPoliticaCancelamento {
		return nil, apierr.Unprocessable("dominio não é uma política de cancelamento")
	}

	if d.Metadata == nil {
		d.Metadata = map[string]any{}
	}
	tierMaps := make([]map[string]any, 0, len(tiers))
	for _, t := range tiers {
		tierMaps = append(tierMaps, map[string]any{
			"triggerType":   t.TriggerType,
			"threshold":     t.Threshold,
			"refundPercent": t.RefundPercent,
			"label":         t.Label,
		})
	}
	d.Metadata["tiers"] = tierMaps
	return uc.repo.Save(ctx, d)
}
