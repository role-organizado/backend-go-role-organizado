package rateio_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/rateio"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	ucrateio "github.com/role-organizado/backend-go-role-organizado/internal/usecase/rateio"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ---- mocks ----

type mockRateioRepo struct{ mock.Mock }

func (m *mockRateioRepo) FindByID(ctx context.Context, id string) (*domain.Rateio, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Rateio), args.Error(1)
}
func (m *mockRateioRepo) FindByEventoID(ctx context.Context, eventoID string) ([]domain.Rateio, error) {
	args := m.Called(ctx, eventoID)
	return args.Get(0).([]domain.Rateio), args.Error(1)
}
func (m *mockRateioRepo) FindByUsuarioID(ctx context.Context, usuarioID string, page, pageSize int) ([]domain.Rateio, int64, error) {
	args := m.Called(ctx, usuarioID, page, pageSize)
	return args.Get(0).([]domain.Rateio), args.Get(1).(int64), args.Error(2)
}
func (m *mockRateioRepo) Save(ctx context.Context, r *domain.Rateio) (*domain.Rateio, error) {
	args := m.Called(ctx, r)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Rateio), args.Error(1)
}
func (m *mockRateioRepo) Update(ctx context.Context, r *domain.Rateio) (*domain.Rateio, error) {
	args := m.Called(ctx, r)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Rateio), args.Error(1)
}
func (m *mockRateioRepo) DeleteByID(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

type mockFechamentoRepo struct{ mock.Mock }

func (m *mockFechamentoRepo) FindByRateioID(ctx context.Context, rateioID string) ([]domain.RateioFechamento, error) {
	args := m.Called(ctx, rateioID)
	return args.Get(0).([]domain.RateioFechamento), args.Error(1)
}
func (m *mockFechamentoRepo) FindLatestByRateioID(ctx context.Context, rateioID string) (*domain.RateioFechamento, error) {
	args := m.Called(ctx, rateioID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.RateioFechamento), args.Error(1)
}
func (m *mockFechamentoRepo) Save(ctx context.Context, f *domain.RateioFechamento) (*domain.RateioFechamento, error) {
	args := m.Called(ctx, f)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.RateioFechamento), args.Error(1)
}

// ---- helpers ----

func openRateio(id, usuarioID string) *domain.Rateio {
	return &domain.Rateio{
		ID:                  id,
		EventoID:            "evento-1",
		UsuarioID:           usuarioID,
		Tipo:                domain.TipoRateioDivisao,
		Status:              domain.StatusRateioAberto,
		ValorTotal:          300.0,
		NumeroParticipantes: 3,
		CriadoEm:            time.Now(),
		UpdatedAt:           time.Now(),
	}
}

// ---- CreateRateio ----

func TestCreateRateio_Success(t *testing.T) {
	repo := new(mockRateioRepo)
	uc := ucrateio.NewCreateRateio(repo)

	in := portin.CreateRateioInput{
		EventoID:            "evt-1",
		UsuarioID:           "usr-1",
		Tipo:                domain.TipoRateioDivisao,
		ValorTotal:          150.0,
		NumeroParticipantes: 3,
	}
	expected := &domain.Rateio{ID: "rat-1", EventoID: "evt-1", UsuarioID: "usr-1"}
	repo.On("Save", mock.Anything, mock.AnythingOfType("*rateio.Rateio")).Return(expected, nil)

	got, err := uc.Execute(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, "rat-1", got.ID)
	repo.AssertExpectations(t)
}

func TestCreateRateio_RepoError(t *testing.T) {
	repo := new(mockRateioRepo)
	uc := ucrateio.NewCreateRateio(repo)

	repo.On("Save", mock.Anything, mock.AnythingOfType("*rateio.Rateio")).Return(nil, apierr.Internal("db error"))

	_, err := ucrateio.NewCreateRateio(repo).Execute(context.Background(), portin.CreateRateioInput{
		Tipo: domain.TipoRateioDivisao,
	})
	require.Error(t, err)
	_ = uc // keep reference
}

// ---- GetRateio ----

func TestGetRateio_Success(t *testing.T) {
	repo := new(mockRateioRepo)
	uc := ucrateio.NewGetRateio(repo)

	r := openRateio("rat-1", "usr-1")
	repo.On("FindByID", mock.Anything, "rat-1").Return(r, nil)

	got, err := uc.Execute(context.Background(), "rat-1", "usr-1")
	require.NoError(t, err)
	assert.Equal(t, "rat-1", got.ID)
}

func TestGetRateio_Forbidden(t *testing.T) {
	repo := new(mockRateioRepo)
	uc := ucrateio.NewGetRateio(repo)

	r := openRateio("rat-1", "usr-owner")
	repo.On("FindByID", mock.Anything, "rat-1").Return(r, nil)

	_, err := uc.Execute(context.Background(), "rat-1", "usr-other")
	require.Error(t, err)
	ae, ok := err.(*apierr.APIError)
	require.True(t, ok)
	assert.Equal(t, 403, ae.Status)
}

// ---- ListRateios ----

func TestListRateios_Success(t *testing.T) {
	repo := new(mockRateioRepo)
	uc := ucrateio.NewListRateios(repo)

	rats := []domain.Rateio{*openRateio("r1", "u1"), *openRateio("r2", "u1")}
	repo.On("FindByEventoID", mock.Anything, "evt-1").Return(rats, nil)

	got, err := uc.Execute(context.Background(), "evt-1", "u1")
	require.NoError(t, err)
	assert.Len(t, got, 2)
}

// ---- UpdateRateio ----

func TestUpdateRateio_Success(t *testing.T) {
	repo := new(mockRateioRepo)
	uc := ucrateio.NewUpdateRateio(repo)

	r := openRateio("rat-1", "usr-1")
	updated := *r
	updated.Descricao = "nova desc"
	repo.On("FindByID", mock.Anything, "rat-1").Return(r, nil)
	repo.On("Update", mock.Anything, mock.AnythingOfType("*rateio.Rateio")).Return(&updated, nil)

	desc := "nova desc"
	got, err := uc.Execute(context.Background(), "rat-1", "usr-1", portin.UpdateRateioInput{Descricao: &desc})
	require.NoError(t, err)
	assert.Equal(t, "nova desc", got.Descricao)
}

func TestUpdateRateio_Forbidden(t *testing.T) {
	repo := new(mockRateioRepo)
	uc := ucrateio.NewUpdateRateio(repo)

	r := openRateio("rat-1", "usr-owner")
	repo.On("FindByID", mock.Anything, "rat-1").Return(r, nil)

	_, err := uc.Execute(context.Background(), "rat-1", "usr-other", portin.UpdateRateioInput{})
	require.Error(t, err)
	ae := err.(*apierr.APIError)
	assert.Equal(t, 403, ae.Status)
}

func TestUpdateRateio_AlreadyFechado(t *testing.T) {
	repo := new(mockRateioRepo)
	uc := ucrateio.NewUpdateRateio(repo)

	r := openRateio("rat-1", "usr-1")
	r.Status = domain.StatusRateioFechado
	repo.On("FindByID", mock.Anything, "rat-1").Return(r, nil)

	_, err := uc.Execute(context.Background(), "rat-1", "usr-1", portin.UpdateRateioInput{})
	require.Error(t, err)
	ae := err.(*apierr.APIError)
	assert.Equal(t, 400, ae.Status)
}

// ---- DeleteRateio ----

func TestDeleteRateio_Success(t *testing.T) {
	repo := new(mockRateioRepo)
	uc := ucrateio.NewDeleteRateio(repo)

	r := openRateio("rat-1", "usr-1")
	repo.On("FindByID", mock.Anything, "rat-1").Return(r, nil)
	repo.On("DeleteByID", mock.Anything, "rat-1").Return(nil)

	err := uc.Execute(context.Background(), "rat-1", "usr-1")
	require.NoError(t, err)
}

func TestDeleteRateio_Forbidden(t *testing.T) {
	repo := new(mockRateioRepo)
	uc := ucrateio.NewDeleteRateio(repo)

	r := openRateio("rat-1", "usr-owner")
	repo.On("FindByID", mock.Anything, "rat-1").Return(r, nil)

	err := uc.Execute(context.Background(), "rat-1", "usr-other")
	require.Error(t, err)
	ae := err.(*apierr.APIError)
	assert.Equal(t, 403, ae.Status)
}

// ---- PreviewRateio ----

func TestPreviewRateio_Divisao(t *testing.T) {
	repo := new(mockRateioRepo)
	uc := ucrateio.NewPreviewRateio(repo)

	r := openRateio("rat-1", "usr-1")
	r.ValorTotal = 300.0
	r.NumeroParticipantes = 3
	repo.On("FindByID", mock.Anything, "rat-1").Return(r, nil)

	result, err := uc.Execute(context.Background(), "rat-1", "usr-1", []string{"u1", "u2", "u3"})
	require.NoError(t, err)
	assert.Equal(t, 300.0, result.TotalGeral)
	assert.Len(t, result.Participantes, 3)
	assert.InDelta(t, 100.0, result.Participantes[0].Valor, 0.01)
}

func TestPreviewRateio_Itens(t *testing.T) {
	repo := new(mockRateioRepo)
	uc := ucrateio.NewPreviewRateio(repo)

	r := openRateio("rat-1", "usr-1")
	r.Tipo = domain.TipoRateioItens
	r.Itens = []domain.RateioItem{
		{Descricao: "A", Valor: 50, Quantidade: 2, Total: 100},
		{Descricao: "B", Valor: 25, Quantidade: 4, Total: 100},
	}
	repo.On("FindByID", mock.Anything, "rat-1").Return(r, nil)

	result, err := uc.Execute(context.Background(), "rat-1", "usr-1", []string{"u1", "u2"})
	require.NoError(t, err)
	assert.Equal(t, 200.0, result.TotalGeral)
	assert.InDelta(t, 100.0, result.Participantes[0].Valor, 0.01)
}

// ---- FecharRateio ----

func TestFecharRateio_Success(t *testing.T) {
	rateioRepo := new(mockRateioRepo)
	fechRepo := new(mockFechamentoRepo)
	uc := ucrateio.NewFecharRateio(rateioRepo, fechRepo)

	r := openRateio("rat-1", "usr-1")
	rateioRepo.On("FindByID", mock.Anything, "rat-1").Return(r, nil)
	fechRepo.On("FindByRateioID", mock.Anything, "rat-1").Return([]domain.RateioFechamento{}, nil)

	expected := &domain.RateioFechamento{ID: "fech-1", RateioID: "rat-1", Versao: 1}
	fechRepo.On("Save", mock.Anything, mock.AnythingOfType("*rateio.RateioFechamento")).Return(expected, nil)
	rateioRepo.On("Update", mock.Anything, mock.AnythingOfType("*rateio.Rateio")).Return(r, nil)

	in := portin.FecharRateioInput{
		Participantes: []portin.FecharParticipanteInput{
			{UsuarioID: "u1", Valor: 150.0, Percentual: 50},
			{UsuarioID: "u2", Valor: 150.0, Percentual: 50},
		},
	}
	f, err := uc.Execute(context.Background(), "rat-1", "usr-1", in)
	require.NoError(t, err)
	assert.Equal(t, "fech-1", f.ID)
}

func TestFecharRateio_AlreadyFechado(t *testing.T) {
	rateioRepo := new(mockRateioRepo)
	fechRepo := new(mockFechamentoRepo)
	uc := ucrateio.NewFecharRateio(rateioRepo, fechRepo)

	r := openRateio("rat-1", "usr-1")
	r.Status = domain.StatusRateioFechado
	rateioRepo.On("FindByID", mock.Anything, "rat-1").Return(r, nil)

	_, err := uc.Execute(context.Background(), "rat-1", "usr-1", portin.FecharRateioInput{})
	require.Error(t, err)
	ae := err.(*apierr.APIError)
	assert.Equal(t, 409, ae.Status)
}

// ---- GetFechamentos ----

func TestGetFechamentos_Success(t *testing.T) {
	rateioRepo := new(mockRateioRepo)
	fechRepo := new(mockFechamentoRepo)
	uc := ucrateio.NewGetFechamentos(rateioRepo, fechRepo)

	r := openRateio("rat-1", "usr-1")
	rateioRepo.On("FindByID", mock.Anything, "rat-1").Return(r, nil)

	fechs := []domain.RateioFechamento{{ID: "f1", Versao: 1}, {ID: "f2", Versao: 2}}
	fechRepo.On("FindByRateioID", mock.Anything, "rat-1").Return(fechs, nil)

	got, err := uc.Execute(context.Background(), "rat-1", "usr-1")
	require.NoError(t, err)
	assert.Len(t, got, 2)
}

func TestGetFechamentos_Forbidden(t *testing.T) {
	rateioRepo := new(mockRateioRepo)
	fechRepo := new(mockFechamentoRepo)
	uc := ucrateio.NewGetFechamentos(rateioRepo, fechRepo)

	r := openRateio("rat-1", "usr-owner")
	rateioRepo.On("FindByID", mock.Anything, "rat-1").Return(r, nil)

	_, err := uc.Execute(context.Background(), "rat-1", "usr-other")
	require.Error(t, err)
	ae := err.(*apierr.APIError)
	assert.Equal(t, 403, ae.Status)
}
