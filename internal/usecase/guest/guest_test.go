package guest_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/guest"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	ucguest "github.com/role-organizado/backend-go-role-organizado/internal/usecase/guest"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ============================================================
// Mocks
// ============================================================

type mockGuestRepo struct{ mock.Mock }

func (m *mockGuestRepo) Save(ctx context.Context, g *domain.Guest) (*domain.Guest, error) {
	args := m.Called(ctx, g)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Guest), args.Error(1)
}

func (m *mockGuestRepo) Update(ctx context.Context, g *domain.Guest) (*domain.Guest, error) {
	args := m.Called(ctx, g)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Guest), args.Error(1)
}

func (m *mockGuestRepo) FindByID(ctx context.Context, id string) (*domain.Guest, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Guest), args.Error(1)
}

func (m *mockGuestRepo) FindByTelefone(ctx context.Context, telefone string) (*domain.Guest, error) {
	args := m.Called(ctx, telefone)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Guest), args.Error(1)
}

func (m *mockGuestRepo) FindByEmail(ctx context.Context, email string) (*domain.Guest, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Guest), args.Error(1)
}

func (m *mockGuestRepo) FindByTelefoneOrEmail(ctx context.Context, telefone, email string) (*domain.Guest, error) {
	args := m.Called(ctx, telefone, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Guest), args.Error(1)
}

func (m *mockGuestRepo) FindAll(ctx context.Context, limit int) ([]domain.Guest, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.Guest), args.Error(1)
}

func (m *mockGuestRepo) FindAllByIDs(ctx context.Context, ids []string) ([]domain.Guest, error) {
	args := m.Called(ctx, ids)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.Guest), args.Error(1)
}

func (m *mockGuestRepo) ExistsByTelefone(ctx context.Context, telefone string) (bool, error) {
	args := m.Called(ctx, telefone)
	return args.Bool(0), args.Error(1)
}

func (m *mockGuestRepo) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	args := m.Called(ctx, email)
	return args.Bool(0), args.Error(1)
}

// ============================================================
// CreateOrFind
// ============================================================

func TestCreateOrFindGuest_Execute(t *testing.T) {
	ctx := context.Background()
	tel := "+5511999999999"

	tests := []struct {
		name      string
		in        portin.CreateOrFindGuestInput
		setupRepo func(*mockGuestRepo)
		wantErr   string
		wantNome  string
	}{
		{
			name: "validation — nome too short",
			in:   portin.CreateOrFindGuestInput{Nome: "A", Telefone: tel},
			setupRepo: func(_ *mockGuestRepo) {},
			wantErr:   "nome",
		},
		{
			name: "validation — missing both telefone and email",
			in:   portin.CreateOrFindGuestInput{Nome: "Maria"},
			setupRepo: func(_ *mockGuestRepo) {},
			wantErr:   "telefone ou email",
		},
		{
			name: "validation — invalid E.164 telefone",
			in:   portin.CreateOrFindGuestInput{Nome: "Maria", Telefone: "0abc"},
			setupRepo: func(_ *mockGuestRepo) {},
			wantErr:   "telefone inválido",
		},
		{
			name: "create new — phone normalization preserves leading +",
			in:   portin.CreateOrFindGuestInput{Nome: "Maria Silva", Telefone: "+55 11 99999-9999"},
			setupRepo: func(r *mockGuestRepo) {
				r.On("FindByTelefoneOrEmail", ctx, "+5511999999999", "").
					Return(nil, apierr.NotFound("guest", ""))
				r.On("Save", ctx, mock.MatchedBy(func(g *domain.Guest) bool {
					return g.Telefone == "+5511999999999" && g.Nome == "Maria Silva"
				})).Return(&domain.Guest{ID: "new-id", Nome: "Maria Silva", Telefone: "+5511999999999"}, nil)
			},
			wantNome: "Maria Silva",
		},
		{
			name: "existing guest — nome unchanged returns as-is",
			in:   portin.CreateOrFindGuestInput{Nome: "Joao", Email: "j@x.com"},
			setupRepo: func(r *mockGuestRepo) {
				existing := &domain.Guest{ID: "g1", Nome: "Joao", Email: "j@x.com"}
				r.On("FindByTelefoneOrEmail", ctx, "", "j@x.com").Return(existing, nil)
			},
			wantNome: "Joao",
		},
		{
			name: "existing guest — nome updated",
			in:   portin.CreateOrFindGuestInput{Nome: "Joao Pedro", Email: "j@x.com"},
			setupRepo: func(r *mockGuestRepo) {
				existing := &domain.Guest{ID: "g1", Nome: "Joao", Email: "j@x.com"}
				r.On("FindByTelefoneOrEmail", ctx, "", "j@x.com").Return(existing, nil)
				r.On("Update", ctx, mock.MatchedBy(func(g *domain.Guest) bool {
					return g.Nome == "Joao Pedro"
				})).Return(&domain.Guest{ID: "g1", Nome: "Joao Pedro", Email: "j@x.com"}, nil)
			},
			wantNome: "Joao Pedro",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockGuestRepo{}
			tt.setupRepo(repo)
			uc := ucguest.NewCreateOrFindGuest(repo)
			out, err := uc.Execute(ctx, tt.in)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, out)
			} else {
				require.NoError(t, err)
				require.NotNil(t, out)
				assert.Equal(t, tt.wantNome, out.Nome)
			}
			repo.AssertExpectations(t)
		})
	}
}

// ============================================================
// ListGuests / BatchGetGuests
// ============================================================

func TestListGuests_Execute_HardCap100(t *testing.T) {
	ctx := context.Background()
	repo := &mockGuestRepo{}
	repo.On("FindAll", ctx, 100).Return([]domain.Guest{{ID: "g1"}}, nil)
	uc := ucguest.NewListGuests(repo)
	out, err := uc.Execute(ctx)
	require.NoError(t, err)
	assert.Len(t, out, 1)
	repo.AssertExpectations(t)
}

func TestBatchGetGuests_Execute_EmptyReturnsEmpty(t *testing.T) {
	ctx := context.Background()
	repo := &mockGuestRepo{}
	uc := ucguest.NewBatchGetGuests(repo)
	out, err := uc.Execute(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, out)
	repo.AssertNotCalled(t, "FindAllByIDs", mock.Anything, mock.Anything)
}

func TestBatchGetGuests_Execute_NonEmptyDelegates(t *testing.T) {
	ctx := context.Background()
	ids := []string{"a", "b"}
	repo := &mockGuestRepo{}
	repo.On("FindAllByIDs", ctx, ids).Return([]domain.Guest{{ID: "a"}, {ID: "b"}}, nil)
	uc := ucguest.NewBatchGetGuests(repo)
	out, err := uc.Execute(ctx, ids)
	require.NoError(t, err)
	assert.Len(t, out, 2)
	repo.AssertExpectations(t)
}

// ============================================================
// NormalizeTelefone / IsValidE164 — pure helpers
// ============================================================

func TestNormalizeTelefone(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"  ", ""},
		{"+55 11 99999-9999", "+5511999999999"},
		{"5511999999999", "5511999999999"},
		{"(11) 9 9999-9999", "11999999999"},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, domain.NormalizeTelefone(c.in), c.in)
	}
}

func TestIsValidE164(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", false},
		{"+5511999999999", true},
		{"5511999999999", true},
		{"+0123", false},   // leading digit cannot be 0
		{"+", false},
		{"+12", true},
		{"+1234567890123456", false}, // 16 digits — too long
	}
	for _, c := range cases {
		assert.Equal(t, c.want, domain.IsValidE164(c.in), c.in)
	}
}
