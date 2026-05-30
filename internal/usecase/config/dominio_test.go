package config_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/config"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	"github.com/role-organizado/backend-go-role-organizado/internal/usecase/config"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ---- Mock: DominioRepository ----

type mockDominioRepo struct {
	mock.Mock
}

func (m *mockDominioRepo) FindAll(ctx context.Context) ([]domain.Dominio, error) {
	args := m.Called(ctx)
	return args.Get(0).([]domain.Dominio), args.Error(1)
}
func (m *mockDominioRepo) FindByCategoria(ctx context.Context, cat string) ([]domain.Dominio, error) {
	args := m.Called(ctx, cat)
	return args.Get(0).([]domain.Dominio), args.Error(1)
}
func (m *mockDominioRepo) FindByCategoriaAndAtivo(ctx context.Context, cat string, ativo bool) ([]domain.Dominio, error) {
	args := m.Called(ctx, cat, ativo)
	return args.Get(0).([]domain.Dominio), args.Error(1)
}
func (m *mockDominioRepo) FindByCategoriaAndChave(ctx context.Context, cat, chave string) (*domain.Dominio, error) {
	args := m.Called(ctx, cat, chave)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Dominio), args.Error(1)
}
func (m *mockDominioRepo) FindByID(ctx context.Context, id string) (*domain.Dominio, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Dominio), args.Error(1)
}
func (m *mockDominioRepo) Save(ctx context.Context, d *domain.Dominio) (*domain.Dominio, error) {
	args := m.Called(ctx, d)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Dominio), args.Error(1)
}
func (m *mockDominioRepo) DeleteByID(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// ---- Tests: ListDominios ----

func TestListDominios_NoFilter_ReturnsAll(t *testing.T) {
	repo := &mockDominioRepo{}
	uc := config.NewListDominios(repo)
	ctx := context.Background()

	expected := []domain.Dominio{{ID: "1", Categoria: "tipo_evento", Chave: "festa"}}
	repo.On("FindAll", ctx).Return(expected, nil)

	result, err := uc.Execute(ctx, portin.ListDominiosInput{})
	require.NoError(t, err)
	assert.Equal(t, expected, result)
	repo.AssertExpectations(t)
}

func TestListDominios_FilterByCategoria_ReturnsFiltered(t *testing.T) {
	repo := &mockDominioRepo{}
	uc := config.NewListDominios(repo)
	ctx := context.Background()
	cat := "tipo_evento"

	expected := []domain.Dominio{{ID: "1", Categoria: cat, Chave: "festa"}}
	repo.On("FindByCategoria", ctx, cat).Return(expected, nil)

	result, err := uc.Execute(ctx, portin.ListDominiosInput{Categoria: &cat})
	require.NoError(t, err)
	assert.Equal(t, expected, result)
	repo.AssertExpectations(t)
}

func TestListDominios_FilterByCategoriaAndAtivo(t *testing.T) {
	repo := &mockDominioRepo{}
	uc := config.NewListDominios(repo)
	ctx := context.Background()
	cat := "tipo_evento"
	ativo := true

	expected := []domain.Dominio{{ID: "1", Categoria: cat, Ativo: true}}
	repo.On("FindByCategoriaAndAtivo", ctx, cat, ativo).Return(expected, nil)

	result, err := uc.Execute(ctx, portin.ListDominiosInput{Categoria: &cat, Ativo: &ativo})
	require.NoError(t, err)
	assert.Equal(t, expected, result)
	repo.AssertExpectations(t)
}

func TestListDominios_TipoEventoFilter_AppliesFeature008(t *testing.T) {
	repo := &mockDominioRepo{}
	uc := config.NewListDominios(repo)
	ctx := context.Background()
	cat := "politica_cancelamento"
	ativo := true
	tipoEvento := "festa_aniversario"

	allDominios := []domain.Dominio{
		{
			ID:        "1",
			Categoria: cat,
			Ativo:     true,
			Metadata: map[string]any{
				"tiposEventoAplicaveis": []any{"festa_aniversario", "churrasaco"},
			},
		},
		{
			ID:        "2",
			Categoria: cat,
			Ativo:     true,
			Metadata: map[string]any{
				"tiposEventoAplicaveis": []any{"casamento"},
			},
		},
		{
			ID:        "3",
			Categoria: cat,
			Ativo:     true,
			Metadata:  nil, // no restriction → should be included
		},
	}
	repo.On("FindByCategoriaAndAtivo", ctx, cat, ativo).Return(allDominios, nil)

	result, err := uc.Execute(ctx, portin.ListDominiosInput{Categoria: &cat, Ativo: &ativo, TipoEvento: &tipoEvento})
	require.NoError(t, err)
	assert.Len(t, result, 2, "should include dominio 1 (applies) and dominio 3 (no restriction)")
	ids := []string{result[0].ID, result[1].ID}
	assert.Contains(t, ids, "1")
	assert.Contains(t, ids, "3")
	repo.AssertExpectations(t)
}

// ---- Tests: GetDominio ----

func TestGetDominio_Found(t *testing.T) {
	repo := &mockDominioRepo{}
	uc := config.NewGetDominio(repo)
	ctx := context.Background()

	d := &domain.Dominio{ID: "1", Categoria: "cat", Chave: "chave"}
	repo.On("FindByCategoriaAndChave", ctx, "cat", "chave").Return(d, nil)

	result, err := uc.Execute(ctx, "cat", "chave")
	require.NoError(t, err)
	assert.Equal(t, d, result)
}

func TestGetDominio_NotFound(t *testing.T) {
	repo := &mockDominioRepo{}
	uc := config.NewGetDominio(repo)
	ctx := context.Background()

	repo.On("FindByCategoriaAndChave", ctx, "cat", "nope").Return(nil, apierr.NotFound("dominio", "nope"))

	_, err := uc.Execute(ctx, "cat", "nope")
	require.Error(t, err)
	assert.True(t, apierr.IsNotFound(err))
}

// ---- Tests: DeleteDominio ----

func TestDeleteDominio_Success(t *testing.T) {
	repo := &mockDominioRepo{}
	uc := config.NewDeleteDominio(repo)
	ctx := context.Background()

	repo.On("DeleteByID", ctx, "123").Return(nil)

	err := uc.Execute(ctx, "123")
	require.NoError(t, err)
}

func TestDeleteDominio_NotFound(t *testing.T) {
	repo := &mockDominioRepo{}
	uc := config.NewDeleteDominio(repo)
	ctx := context.Background()

	repo.On("DeleteByID", ctx, "999").Return(apierr.NotFound("dominio", "999"))

	err := uc.Execute(ctx, "999")
	assert.True(t, apierr.IsNotFound(err))
}

// ---- Tests: UpsertDominio ----

func TestUpsertDominio_Create_AssignsDefaults(t *testing.T) {
	repo := &mockDominioRepo{}
	uc := config.NewUpsertDominio(repo)
	ctx := context.Background()

	in := portin.UpsertDominioInput{
		Categoria: "tipo_evento",
		Chave:     "festa",
		Valor:     "Festa de Aniversário",
		Ativo:     true,
	}
	repo.On("Save", ctx, mock.AnythingOfType("*config.Dominio")).Return(&domain.Dominio{
		ID:        "new_id",
		Categoria: in.Categoria,
		Chave:     in.Chave,
	}, nil)

	result, err := uc.Execute(ctx, in)
	require.NoError(t, err)
	assert.Equal(t, "new_id", result.ID)
}

func TestUpsertDominio_RepoError_PropagatesError(t *testing.T) {
	repo := &mockDominioRepo{}
	uc := config.NewUpsertDominio(repo)
	ctx := context.Background()

	in := portin.UpsertDominioInput{Categoria: "c", Chave: "k"}
	repo.On("Save", ctx, mock.AnythingOfType("*config.Dominio")).Return((*domain.Dominio)(nil), errors.New("db error"))

	_, err := uc.Execute(ctx, in)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}
