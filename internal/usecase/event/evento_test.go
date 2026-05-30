package event_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/event"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	usecase "github.com/role-organizado/backend-go-role-organizado/internal/usecase/event"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ---- Mock ----

type mockEventoRepo struct{ mock.Mock }

func (m *mockEventoRepo) FindByID(ctx context.Context, id string) (*domain.Evento, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Evento), args.Error(1)
}

func (m *mockEventoRepo) FindByUsuarioID(ctx context.Context, usuarioID string, page, pageSize int) ([]domain.Evento, int64, error) {
	args := m.Called(ctx, usuarioID, page, pageSize)
	return args.Get(0).([]domain.Evento), int64(args.Int(1)), args.Error(2)
}

func (m *mockEventoRepo) FindAll(ctx context.Context, page, pageSize int) ([]domain.Evento, int64, error) {
	args := m.Called(ctx, page, pageSize)
	return args.Get(0).([]domain.Evento), int64(args.Int(1)), args.Error(2)
}

func (m *mockEventoRepo) Save(ctx context.Context, e *domain.Evento) (*domain.Evento, error) {
	args := m.Called(ctx, e)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Evento), args.Error(1)
}

func (m *mockEventoRepo) Update(ctx context.Context, e *domain.Evento) (*domain.Evento, error) {
	args := m.Called(ctx, e)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Evento), args.Error(1)
}

func (m *mockEventoRepo) DeleteByID(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// ---- helpers ----

func sampleEvento(id, userID string) *domain.Evento {
	return &domain.Evento{
		ID:        id,
		UsuarioID: userID,
		Nome:      "Festa de Aniversário",
		Tipo:      "festa",
		Data:      time.Now().Add(24 * time.Hour),
		Status:    domain.EventoStatusPublicado,
		CriadoEm: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// ---- CreateEvento tests ----

func TestCreateEvento_Success(t *testing.T) {
	repo := new(mockEventoRepo)
	uc := usecase.NewCreateEvento(repo)

	in := portin.CreateEventoInput{
		UsuarioID: "user-1",
		Nome:      "Festa",
		Tipo:      "festa",
		Data:      time.Now().Add(24 * time.Hour),
	}
	saved := &domain.Evento{ID: "evt-1", UsuarioID: "user-1", Nome: "Festa"}
	repo.On("Save", mock.Anything, mock.AnythingOfType("*event.Evento")).Return(saved, nil)

	result, err := uc.Execute(context.Background(), in)

	require.NoError(t, err)
	assert.Equal(t, "evt-1", result.ID)
	repo.AssertExpectations(t)
}

func TestCreateEvento_RepoError(t *testing.T) {
	repo := new(mockEventoRepo)
	uc := usecase.NewCreateEvento(repo)

	repo.On("Save", mock.Anything, mock.AnythingOfType("*event.Evento")).
		Return(nil, apierr.Internal("db error"))

	_, err := uc.Execute(context.Background(), portin.CreateEventoInput{UsuarioID: "u1", Nome: "X"})
	require.Error(t, err)
}

// ---- GetEvento tests ----

func TestGetEvento_Success(t *testing.T) {
	repo := new(mockEventoRepo)
	uc := usecase.NewGetEvento(repo)

	evt := sampleEvento("evt-1", "user-1")
	repo.On("FindByID", mock.Anything, "evt-1").Return(evt, nil)

	result, err := uc.Execute(context.Background(), "evt-1")
	require.NoError(t, err)
	assert.Equal(t, "evt-1", result.ID)
}

func TestGetEvento_NotFound(t *testing.T) {
	repo := new(mockEventoRepo)
	uc := usecase.NewGetEvento(repo)

	repo.On("FindByID", mock.Anything, "missing").Return(nil, apierr.NotFound("evento", "missing"))

	_, err := uc.Execute(context.Background(), "missing")
	require.Error(t, err)
	var ae *apierr.APIError
	assert.ErrorAs(t, err, &ae)
	assert.Equal(t, 404, ae.Status)
}

// ---- ListEventos tests ----

func TestListEventos_AllEvents(t *testing.T) {
	repo := new(mockEventoRepo)
	uc := usecase.NewListEventos(repo)

	events := []domain.Evento{*sampleEvento("1", "u1"), *sampleEvento("2", "u2")}
	repo.On("FindAll", mock.Anything, 1, 20).Return(events, 2, nil)

	result, total, err := uc.Execute(context.Background(), nil, 1, 20)
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, int64(2), total)
}

func TestListEventos_ByUsuarioID(t *testing.T) {
	repo := new(mockEventoRepo)
	uc := usecase.NewListEventos(repo)

	uid := "user-1"
	events := []domain.Evento{*sampleEvento("1", uid)}
	repo.On("FindByUsuarioID", mock.Anything, uid, 1, 20).Return(events, 1, nil)

	result, total, err := uc.Execute(context.Background(), &uid, 1, 20)
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, int64(1), total)
}

// ---- UpdateEvento tests ----

func TestUpdateEvento_Success(t *testing.T) {
	repo := new(mockEventoRepo)
	uc := usecase.NewUpdateEvento(repo)

	evt := sampleEvento("evt-1", "user-1")
	updated := *evt
	updated.Nome = "Novo Nome"

	repo.On("FindByID", mock.Anything, "evt-1").Return(evt, nil)
	repo.On("Update", mock.Anything, mock.AnythingOfType("*event.Evento")).Return(&updated, nil)

	in := portin.UpdateEventoInput{Nome: "Novo Nome"}
	result, err := uc.Execute(context.Background(), "evt-1", "user-1", in)
	require.NoError(t, err)
	assert.Equal(t, "Novo Nome", result.Nome)
}

func TestUpdateEvento_Forbidden(t *testing.T) {
	repo := new(mockEventoRepo)
	uc := usecase.NewUpdateEvento(repo)

	evt := sampleEvento("evt-1", "owner-id")
	repo.On("FindByID", mock.Anything, "evt-1").Return(evt, nil)

	_, err := uc.Execute(context.Background(), "evt-1", "other-user", portin.UpdateEventoInput{})
	require.Error(t, err)
	var ae *apierr.APIError
	assert.ErrorAs(t, err, &ae)
	assert.Equal(t, 403, ae.Status)
}

// ---- DeleteEvento tests ----

func TestDeleteEvento_Success(t *testing.T) {
	repo := new(mockEventoRepo)
	uc := usecase.NewDeleteEvento(repo)

	evt := sampleEvento("evt-1", "user-1")
	repo.On("FindByID", mock.Anything, "evt-1").Return(evt, nil)
	repo.On("DeleteByID", mock.Anything, "evt-1").Return(nil)

	err := uc.Execute(context.Background(), "evt-1", "user-1")
	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestDeleteEvento_Forbidden(t *testing.T) {
	repo := new(mockEventoRepo)
	uc := usecase.NewDeleteEvento(repo)

	evt := sampleEvento("evt-1", "owner-id")
	repo.On("FindByID", mock.Anything, "evt-1").Return(evt, nil)

	err := uc.Execute(context.Background(), "evt-1", "not-owner")
	require.Error(t, err)
	var ae *apierr.APIError
	assert.ErrorAs(t, err, &ae)
	assert.Equal(t, 403, ae.Status)
}
