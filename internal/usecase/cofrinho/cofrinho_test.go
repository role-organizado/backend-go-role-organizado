package cofrinho_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/cofrinho"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	uccofrinho "github.com/role-organizado/backend-go-role-organizado/internal/usecase/cofrinho"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ---- mock repo ----

type mockCofrinhoRepo struct{ mock.Mock }

func (m *mockCofrinhoRepo) Save(ctx context.Context, c *domain.CofrinhoContribuicao) (*domain.CofrinhoContribuicao, error) {
	args := m.Called(ctx, c)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.CofrinhoContribuicao), args.Error(1)
}

func (m *mockCofrinhoRepo) FindByID(ctx context.Context, id string) (*domain.CofrinhoContribuicao, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.CofrinhoContribuicao), args.Error(1)
}

func (m *mockCofrinhoRepo) FindByEventoID(ctx context.Context, eventoID string) ([]*domain.CofrinhoContribuicao, error) {
	args := m.Called(ctx, eventoID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.CofrinhoContribuicao), args.Error(1)
}

func (m *mockCofrinhoRepo) UpdateStatus(ctx context.Context, id string, status domain.CofrinhoStatus, webhookPaymentID string) (*domain.CofrinhoContribuicao, error) {
	args := m.Called(ctx, id, status, webhookPaymentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.CofrinhoContribuicao), args.Error(1)
}

func (m *mockCofrinhoRepo) DeleteByID(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// ---- helpers ----

func pendente(id, eventoID string) *domain.CofrinhoContribuicao {
	return &domain.CofrinhoContribuicao{
		ID:        id,
		EventoID:  eventoID,
		GuestID:   "guest-1",
		Nome:      "Fulano",
		Valor:     5000,
		Status:    domain.StatusPendente,
		PIXQRCode: "PIX-PLACEHOLDER-XXX",
		CriadoEm: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func confirmado(id, eventoID string) *domain.CofrinhoContribuicao {
	c := pendente(id, eventoID)
	c.Status = domain.StatusConfirmado
	return c
}

// ---- CreateContribuicao tests ----

func TestCreateContribuicao_Execute(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		in        portin.CreateContribuicaoInput
		mockSetup func(*mockCofrinhoRepo)
		wantErr   bool
		errMsg    string
	}{
		{
			name: "success",
			in: portin.CreateContribuicaoInput{
				EventoID: "evt-1",
				GuestID:  "guest-1",
				Nome:     "Fulano",
				Mensagem: "Parabéns!",
				Valor:    10000,
			},
			mockSetup: func(r *mockCofrinhoRepo) {
				r.On("Save", ctx, mock.MatchedBy(func(c *domain.CofrinhoContribuicao) bool {
					return c.EventoID == "evt-1" && c.Valor == 10000 && c.Status == domain.StatusPendente
				})).Return(pendente("new-id", "evt-1"), nil)
			},
			wantErr: false,
		},
		{
			name:      "missing eventoId returns bad request",
			in:        portin.CreateContribuicaoInput{Nome: "Fulano", Valor: 100},
			mockSetup: func(r *mockCofrinhoRepo) {},
			wantErr:   true,
			errMsg:    "eventoId",
		},
		{
			name:      "missing nome returns bad request",
			in:        portin.CreateContribuicaoInput{EventoID: "evt-1", Valor: 100},
			mockSetup: func(r *mockCofrinhoRepo) {},
			wantErr:   true,
			errMsg:    "nome",
		},
		{
			name:      "zero valor returns bad request",
			in:        portin.CreateContribuicaoInput{EventoID: "evt-1", Nome: "Fulano", Valor: 0},
			mockSetup: func(r *mockCofrinhoRepo) {},
			wantErr:   true,
			errMsg:    "valor",
		},
		{
			name: "repo error propagated",
			in: portin.CreateContribuicaoInput{
				EventoID: "evt-1",
				Nome:     "Fulano",
				Valor:    100,
			},
			mockSetup: func(r *mockCofrinhoRepo) {
				r.On("Save", ctx, mock.Anything).Return(nil, errors.New("db down"))
			},
			wantErr: true,
			errMsg:  "db down",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockCofrinhoRepo{}
			tt.mockSetup(repo)
			uc := uccofrinho.NewCreateContribuicao(repo)
			c, err := uc.Execute(ctx, tt.in)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				assert.Nil(t, c)
			} else {
				require.NoError(t, err)
				require.NotNil(t, c)
				assert.Equal(t, domain.StatusPendente, c.Status)
			}
			repo.AssertExpectations(t)
		})
	}
}

// ---- ConfirmarContribuicao tests ----

func TestConfirmarContribuicao_Execute(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		in        portin.ConfirmarContribuicaoInput
		mockSetup func(*mockCofrinhoRepo)
		wantErr   bool
		errMsg    string
	}{
		{
			name: "success — PENDENTE → CONFIRMADO",
			in:   portin.ConfirmarContribuicaoInput{ContribuicaoID: "c1", WebhookPaymentID: "pay-1"},
			mockSetup: func(r *mockCofrinhoRepo) {
				r.On("FindByID", ctx, "c1").Return(pendente("c1", "evt-1"), nil)
				r.On("UpdateStatus", ctx, "c1", domain.StatusConfirmado, "pay-1").Return(confirmado("c1", "evt-1"), nil)
			},
		},
		{
			name: "already CONFIRMADO returns unprocessable",
			in:   portin.ConfirmarContribuicaoInput{ContribuicaoID: "c2", WebhookPaymentID: "pay-2"},
			mockSetup: func(r *mockCofrinhoRepo) {
				r.On("FindByID", ctx, "c2").Return(confirmado("c2", "evt-1"), nil)
			},
			wantErr: true,
			errMsg:  "pendente",
		},
		{
			name: "not found propagated",
			in:   portin.ConfirmarContribuicaoInput{ContribuicaoID: "c3", WebhookPaymentID: "pay-3"},
			mockSetup: func(r *mockCofrinhoRepo) {
				r.On("FindByID", ctx, "c3").Return(nil, apierr.NotFound("cofrinho_contribuicao", "c3"))
			},
			wantErr: true,
			errMsg:  "c3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockCofrinhoRepo{}
			tt.mockSetup(repo)
			uc := uccofrinho.NewConfirmarContribuicao(repo)
			c, err := uc.Execute(ctx, tt.in)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				assert.Nil(t, c)
			} else {
				require.NoError(t, err)
				require.NotNil(t, c)
				assert.Equal(t, domain.StatusConfirmado, c.Status)
			}
			repo.AssertExpectations(t)
		})
	}
}

// ---- RemoverContribuicao tests ----

func TestRemoverContribuicao_Execute(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		id        string
		mockSetup func(*mockCofrinhoRepo)
		wantErr   bool
		errMsg    string
	}{
		{
			name: "success — removes PENDENTE contribution",
			id:   "c1",
			mockSetup: func(r *mockCofrinhoRepo) {
				r.On("FindByID", ctx, "c1").Return(pendente("c1", "evt-1"), nil)
				r.On("DeleteByID", ctx, "c1").Return(nil)
			},
		},
		{
			name: "CONFIRMADO cannot be removed",
			id:   "c2",
			mockSetup: func(r *mockCofrinhoRepo) {
				r.On("FindByID", ctx, "c2").Return(confirmado("c2", "evt-1"), nil)
			},
			wantErr: true,
			errMsg:  "PENDENTE",
		},
		{
			name: "EXPIRADO cannot be removed",
			id:   "c3",
			mockSetup: func(r *mockCofrinhoRepo) {
				exp := pendente("c3", "evt-1")
				exp.Status = domain.StatusExpirado
				r.On("FindByID", ctx, "c3").Return(exp, nil)
			},
			wantErr: true,
			errMsg:  "PENDENTE",
		},
		{
			name: "not found propagated from repo",
			id:   "c4",
			mockSetup: func(r *mockCofrinhoRepo) {
				r.On("FindByID", ctx, "c4").Return(nil, apierr.NotFound("cofrinho_contribuicao", "c4"))
			},
			wantErr: true,
			errMsg:  "c4",
		},
		{
			name:      "empty id returns bad request",
			id:        "",
			mockSetup: func(r *mockCofrinhoRepo) {},
			wantErr:   true,
			errMsg:    "obrigatório",
		},
		{
			name: "delete repo error propagated",
			id:   "c5",
			mockSetup: func(r *mockCofrinhoRepo) {
				r.On("FindByID", ctx, "c5").Return(pendente("c5", "evt-1"), nil)
				r.On("DeleteByID", ctx, "c5").Return(errors.New("db unavailable"))
			},
			wantErr: true,
			errMsg:  "db unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockCofrinhoRepo{}
			tt.mockSetup(repo)
			uc := uccofrinho.NewRemoverContribuicao(repo)
			err := uc.Execute(ctx, tt.id)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
			repo.AssertExpectations(t)
		})
	}
}

// ---- ListContribuicoes tests ----

func TestListContribuicoes_Execute(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		eventoID  string
		mockSetup func(*mockCofrinhoRepo)
		wantLen   int
		wantErr   bool
		errMsg    string
	}{
		{
			name:     "success — returns list",
			eventoID: "evt-1",
			mockSetup: func(r *mockCofrinhoRepo) {
				r.On("FindByEventoID", ctx, "evt-1").Return([]*domain.CofrinhoContribuicao{
					pendente("c1", "evt-1"),
					confirmado("c2", "evt-1"),
				}, nil)
			},
			wantLen: 2,
		},
		{
			name:     "empty list returned as empty slice",
			eventoID: "evt-2",
			mockSetup: func(r *mockCofrinhoRepo) {
				r.On("FindByEventoID", ctx, "evt-2").Return([]*domain.CofrinhoContribuicao{}, nil)
			},
			wantLen: 0,
		},
		{
			name:      "missing eventoId returns bad request",
			eventoID:  "",
			mockSetup: func(r *mockCofrinhoRepo) {},
			wantErr:   true,
			errMsg:    "eventoId",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockCofrinhoRepo{}
			tt.mockSetup(repo)
			uc := uccofrinho.NewListContribuicoes(repo)
			result, err := uc.Execute(ctx, tt.eventoID)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
				assert.Len(t, result, tt.wantLen)
			}
			repo.AssertExpectations(t)
		})
	}
}
