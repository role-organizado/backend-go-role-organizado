package payment_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	ucpayment "github.com/role-organizado/backend-go-role-organizado/internal/usecase/payment"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ---- mocks ----

type mockPagRepo struct{ mock.Mock }

func (m *mockPagRepo) FindByID(ctx context.Context, id string) (*domain.PagamentoMensal, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.PagamentoMensal), args.Error(1)
}
func (m *mockPagRepo) FindByEventoID(ctx context.Context, eventoID string) ([]domain.PagamentoMensal, error) {
	args := m.Called(ctx, eventoID)
	return args.Get(0).([]domain.PagamentoMensal), args.Error(1)
}
func (m *mockPagRepo) FindByUsuarioID(ctx context.Context, usuarioID string, page, pageSize int) ([]domain.PagamentoMensal, int64, error) {
	args := m.Called(ctx, usuarioID, page, pageSize)
	return args.Get(0).([]domain.PagamentoMensal), args.Get(1).(int64), args.Error(2)
}
func (m *mockPagRepo) FindByEventoIDAndStatus(ctx context.Context, eventoID string, status domain.StatusPagamento) ([]domain.PagamentoMensal, error) {
	args := m.Called(ctx, eventoID, status)
	return args.Get(0).([]domain.PagamentoMensal), args.Error(1)
}
func (m *mockPagRepo) Save(ctx context.Context, p *domain.PagamentoMensal) (*domain.PagamentoMensal, error) {
	args := m.Called(ctx, p)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.PagamentoMensal), args.Error(1)
}
func (m *mockPagRepo) Update(ctx context.Context, p *domain.PagamentoMensal) (*domain.PagamentoMensal, error) {
	args := m.Called(ctx, p)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.PagamentoMensal), args.Error(1)
}
func (m *mockPagRepo) DeleteByID(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

type mockCfgRepo struct{ mock.Mock }

func (m *mockCfgRepo) FindByEventoID(ctx context.Context, eventoID string) (*domain.EventoConfigPagamento, error) {
	args := m.Called(ctx, eventoID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.EventoConfigPagamento), args.Error(1)
}
func (m *mockCfgRepo) Save(ctx context.Context, cfg *domain.EventoConfigPagamento) (*domain.EventoConfigPagamento, error) {
	args := m.Called(ctx, cfg)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.EventoConfigPagamento), args.Error(1)
}
func (m *mockCfgRepo) Update(ctx context.Context, cfg *domain.EventoConfigPagamento) (*domain.EventoConfigPagamento, error) {
	args := m.Called(ctx, cfg)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.EventoConfigPagamento), args.Error(1)
}

// ---- helper ----

func pendingPayment(id, usuarioID string) *domain.PagamentoMensal {
	return &domain.PagamentoMensal{
		ID:              id,
		EventoID:        "evt-1",
		UsuarioID:       usuarioID,
		Valor:           100.0,
		MetodoPagamento: domain.MetodoPagamentoPix,
		Status:          domain.StatusPagamentoPendente,
		DataVencimento:  time.Now().Add(24 * time.Hour),
		CriadoEm:       time.Now(),
		UpdatedAt:       time.Now(),
	}
}

// ---- CreatePagamento ----

func TestCreatePagamento_Success(t *testing.T) {
	repo := new(mockPagRepo)
	uc := ucpayment.NewCreatePagamento(repo)

	expected := pendingPayment("pay-1", "usr-1")
	repo.On("Save", mock.Anything, mock.AnythingOfType("*payment.PagamentoMensal")).Return(expected, nil)

	got, err := uc.Execute(context.Background(), portin.CreatePagamentoInput{
		EventoID:        "evt-1",
		UsuarioID:       "usr-1",
		Valor:           100.0,
		MetodoPagamento: domain.MetodoPagamentoPix,
		DataVencimento:  time.Now().Add(24 * time.Hour),
	})
	require.NoError(t, err)
	assert.Equal(t, "pay-1", got.ID)
}

// ---- GetPagamento ----

func TestGetPagamento_Success(t *testing.T) {
	repo := new(mockPagRepo)
	uc := ucpayment.NewGetPagamento(repo)

	p := pendingPayment("pay-1", "usr-1")
	repo.On("FindByID", mock.Anything, "pay-1").Return(p, nil)

	got, err := uc.Execute(context.Background(), "pay-1", "usr-1")
	require.NoError(t, err)
	assert.Equal(t, "pay-1", got.ID)
}

func TestGetPagamento_Forbidden(t *testing.T) {
	repo := new(mockPagRepo)
	uc := ucpayment.NewGetPagamento(repo)

	p := pendingPayment("pay-1", "usr-owner")
	repo.On("FindByID", mock.Anything, "pay-1").Return(p, nil)

	_, err := uc.Execute(context.Background(), "pay-1", "usr-other")
	require.Error(t, err)
	ae := err.(*apierr.APIError)
	assert.Equal(t, 403, ae.Status)
}

// ---- ListPagamentos ----

func TestListPagamentos_Success(t *testing.T) {
	repo := new(mockPagRepo)
	uc := ucpayment.NewListPagamentos(repo)

	pags := []domain.PagamentoMensal{*pendingPayment("p1", "u1"), *pendingPayment("p2", "u1")}
	repo.On("FindByEventoID", mock.Anything, "evt-1").Return(pags, nil)

	got, err := uc.Execute(context.Background(), "evt-1", "u1")
	require.NoError(t, err)
	assert.Len(t, got, 2)
}

// ---- UpdatePagamento ----

func TestUpdatePagamento_Success(t *testing.T) {
	repo := new(mockPagRepo)
	uc := ucpayment.NewUpdatePagamento(repo)

	p := pendingPayment("pay-1", "usr-1")
	updated := *p
	updated.Descricao = "novo"
	repo.On("FindByID", mock.Anything, "pay-1").Return(p, nil)
	repo.On("Update", mock.Anything, mock.AnythingOfType("*payment.PagamentoMensal")).Return(&updated, nil)

	desc := "novo"
	got, err := uc.Execute(context.Background(), "pay-1", "usr-1", portin.UpdatePagamentoInput{Descricao: &desc})
	require.NoError(t, err)
	assert.Equal(t, "novo", got.Descricao)
}

func TestUpdatePagamento_AlreadyPago(t *testing.T) {
	repo := new(mockPagRepo)
	uc := ucpayment.NewUpdatePagamento(repo)

	p := pendingPayment("pay-1", "usr-1")
	p.Status = domain.StatusPagamentoPago
	repo.On("FindByID", mock.Anything, "pay-1").Return(p, nil)

	_, err := uc.Execute(context.Background(), "pay-1", "usr-1", portin.UpdatePagamentoInput{})
	require.Error(t, err)
	ae := err.(*apierr.APIError)
	assert.Equal(t, 400, ae.Status)
}

// ---- DeletePagamento ----

func TestDeletePagamento_Success(t *testing.T) {
	repo := new(mockPagRepo)
	uc := ucpayment.NewDeletePagamento(repo)

	p := pendingPayment("pay-1", "usr-1")
	repo.On("FindByID", mock.Anything, "pay-1").Return(p, nil)
	repo.On("DeleteByID", mock.Anything, "pay-1").Return(nil)

	err := uc.Execute(context.Background(), "pay-1", "usr-1")
	require.NoError(t, err)
}

// ---- ConfirmarPagamento ----

func TestConfirmarPagamento_Success(t *testing.T) {
	repo := new(mockPagRepo)
	uc := ucpayment.NewConfirmarPagamento(repo)

	p := pendingPayment("pay-1", "usr-1")
	updated := *p
	updated.Status = domain.StatusPagamentoPago
	repo.On("FindByID", mock.Anything, "pay-1").Return(p, nil)
	repo.On("Update", mock.Anything, mock.AnythingOfType("*payment.PagamentoMensal")).Return(&updated, nil)

	got, err := uc.Execute(context.Background(), "pay-1", "usr-1", portin.ConfirmarPagamentoInput{
		DataPagamento: time.Now(),
	})
	require.NoError(t, err)
	assert.Equal(t, domain.StatusPagamentoPago, got.Status)
}

func TestConfirmarPagamento_AlreadyPago(t *testing.T) {
	repo := new(mockPagRepo)
	uc := ucpayment.NewConfirmarPagamento(repo)

	p := pendingPayment("pay-1", "usr-1")
	p.Status = domain.StatusPagamentoPago
	repo.On("FindByID", mock.Anything, "pay-1").Return(p, nil)

	_, err := uc.Execute(context.Background(), "pay-1", "usr-1", portin.ConfirmarPagamentoInput{
		DataPagamento: time.Now(),
	})
	require.Error(t, err)
	ae := err.(*apierr.APIError)
	assert.Equal(t, 409, ae.Status)
}

// ---- UpsertConfigPagamento ----

func TestUpsertConfigPagamento_Create(t *testing.T) {
	cfgRepo := new(mockCfgRepo)
	uc := ucpayment.NewUpsertConfigPagamento(cfgRepo)

	// FindByEventoID returns not-found → create
	cfgRepo.On("FindByEventoID", mock.Anything, "evt-1").Return(nil, apierr.NotFound("config_pagamento", "evt-1"))
	expected := &domain.EventoConfigPagamento{ID: "cfg-1", EventoID: "evt-1"}
	cfgRepo.On("Save", mock.Anything, mock.AnythingOfType("*payment.EventoConfigPagamento")).Return(expected, nil)

	got, err := uc.Execute(context.Background(), portin.UpsertConfigPagamentoInput{
		EventoID:         "evt-1",
		UsuarioID:        "usr-1",
		MetodosPagamento: []domain.MetodoPagamento{domain.MetodoPagamentoPix},
	})
	require.NoError(t, err)
	assert.Equal(t, "cfg-1", got.ID)
}

func TestUpsertConfigPagamento_Update(t *testing.T) {
	cfgRepo := new(mockCfgRepo)
	uc := ucpayment.NewUpsertConfigPagamento(cfgRepo)

	existing := &domain.EventoConfigPagamento{ID: "cfg-1", EventoID: "evt-1", UsuarioID: "usr-1"}
	cfgRepo.On("FindByEventoID", mock.Anything, "evt-1").Return(existing, nil)
	cfgRepo.On("Update", mock.Anything, mock.AnythingOfType("*payment.EventoConfigPagamento")).Return(existing, nil)

	got, err := uc.Execute(context.Background(), portin.UpsertConfigPagamentoInput{
		EventoID:         "evt-1",
		UsuarioID:        "usr-1",
		MetodosPagamento: []domain.MetodoPagamento{domain.MetodoPagamentoBoleto},
	})
	require.NoError(t, err)
	assert.Equal(t, "cfg-1", got.ID)
}

// ---- GetConfigPagamento ----

func TestGetConfigPagamento_Success(t *testing.T) {
	cfgRepo := new(mockCfgRepo)
	uc := ucpayment.NewGetConfigPagamento(cfgRepo)

	cfg := &domain.EventoConfigPagamento{ID: "cfg-1", EventoID: "evt-1"}
	cfgRepo.On("FindByEventoID", mock.Anything, "evt-1").Return(cfg, nil)

	got, err := uc.Execute(context.Background(), "evt-1", "usr-1")
	require.NoError(t, err)
	assert.Equal(t, "cfg-1", got.ID)
}
