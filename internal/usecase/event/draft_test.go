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
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	usecase "github.com/role-organizado/backend-go-role-organizado/internal/usecase/event"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ---- Mocks ----

type mockDraftRepo struct{ mock.Mock }

func (m *mockDraftRepo) FindByID(ctx context.Context, id string) (*domain.EventoDraft, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.EventoDraft), args.Error(1)
}

func (m *mockDraftRepo) FindByUsuarioID(ctx context.Context, usuarioID string) ([]domain.EventoDraft, error) {
	args := m.Called(ctx, usuarioID)
	return args.Get(0).([]domain.EventoDraft), args.Error(1)
}

func (m *mockDraftRepo) Save(ctx context.Context, d *domain.EventoDraft) (*domain.EventoDraft, error) {
	args := m.Called(ctx, d)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.EventoDraft), args.Error(1)
}

func (m *mockDraftRepo) Update(ctx context.Context, d *domain.EventoDraft) (*domain.EventoDraft, error) {
	args := m.Called(ctx, d)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.EventoDraft), args.Error(1)
}

func (m *mockDraftRepo) DeleteByID(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// mockEventoRepoForDraft - reuses fields from evento_test.go but declared here to avoid conflict
type mockEventoRepoForPublish struct{ mock.Mock }

func (m *mockEventoRepoForPublish) FindByID(ctx context.Context, id string) (*domain.Evento, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Evento), args.Error(1)
}

func (m *mockEventoRepoForPublish) FindByUsuarioID(ctx context.Context, uid string, page, ps int) ([]domain.Evento, int64, error) {
	return nil, 0, nil
}

func (m *mockEventoRepoForPublish) FindAll(ctx context.Context, page, ps int) ([]domain.Evento, int64, error) {
	return nil, 0, nil
}

func (m *mockEventoRepoForPublish) Save(ctx context.Context, e *domain.Evento) (*domain.Evento, error) {
	args := m.Called(ctx, e)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Evento), args.Error(1)
}

func (m *mockEventoRepoForPublish) FindByUsuarioIDCursor(ctx context.Context, usuarioID string, filtros portout.EventoQueryFiltros) (portout.EventosCursorPage, error) {
	return portout.EventosCursorPage{}, nil
}

func (m *mockEventoRepoForPublish) Update(ctx context.Context, e *domain.Evento) (*domain.Evento, error) {
	return nil, nil
}

func (m *mockEventoRepoForPublish) DeleteByID(ctx context.Context, id string) error {
	return nil
}

// ---- helpers ----

func sampleDraft(id, userID string) *domain.EventoDraft {
	return &domain.EventoDraft{
		ID:              id,
		UsuarioID:       userID,
		Nome:            "Festa",
		EtapaAtual:      0,
		EtapasCompletas: []int{},
		CriadoEm:        time.Now(),
		UpdatedAt:       time.Now(),
	}
}

// ---- CreateDraft ----

func TestCreateDraft_Success(t *testing.T) {
	draftRepo := new(mockDraftRepo)
	uc := usecase.NewCreateDraft(draftRepo)

	saved := sampleDraft("d-1", "user-1")
	draftRepo.On("Save", mock.Anything, mock.AnythingOfType("*event.EventoDraft")).Return(saved, nil)

	result, err := uc.Execute(context.Background(), "user-1")
	require.NoError(t, err)
	assert.Equal(t, "d-1", result.ID)
	draftRepo.AssertExpectations(t)
}

// ---- GetDraft ----

func TestGetDraft_Success(t *testing.T) {
	draftRepo := new(mockDraftRepo)
	uc := usecase.NewGetDraft(draftRepo)

	d := sampleDraft("d-1", "user-1")
	draftRepo.On("FindByID", mock.Anything, "d-1").Return(d, nil)

	result, err := uc.Execute(context.Background(), "d-1", "user-1")
	require.NoError(t, err)
	assert.Equal(t, "d-1", result.ID)
}

func TestGetDraft_Forbidden(t *testing.T) {
	draftRepo := new(mockDraftRepo)
	uc := usecase.NewGetDraft(draftRepo)

	d := sampleDraft("d-1", "owner-id")
	draftRepo.On("FindByID", mock.Anything, "d-1").Return(d, nil)

	_, err := uc.Execute(context.Background(), "d-1", "other-user")
	require.Error(t, err)
	var ae *apierr.APIError
	assert.ErrorAs(t, err, &ae)
	assert.Equal(t, 403, ae.Status)
}

// ---- ListDrafts ----

func TestListDrafts_Success(t *testing.T) {
	draftRepo := new(mockDraftRepo)
	uc := usecase.NewListDrafts(draftRepo)

	drafts := []domain.EventoDraft{*sampleDraft("d-1", "user-1")}
	draftRepo.On("FindByUsuarioID", mock.Anything, "user-1").Return(drafts, nil)

	result, err := uc.Execute(context.Background(), "user-1")
	require.NoError(t, err)
	assert.Len(t, result, 1)
}

// ---- UpdateDraft ----

func TestUpdateDraft_Success(t *testing.T) {
	draftRepo := new(mockDraftRepo)
	uc := usecase.NewUpdateDraft(draftRepo)

	d := sampleDraft("d-1", "user-1")
	nome := "Novo Nome"
	in := portin.UpsertDraftInput{Nome: &nome}

	updated := *d
	updated.Nome = nome
	draftRepo.On("FindByID", mock.Anything, "d-1").Return(d, nil)
	draftRepo.On("Update", mock.Anything, mock.AnythingOfType("*event.EventoDraft")).Return(&updated, nil)

	result, err := uc.Execute(context.Background(), "d-1", "user-1", in)
	require.NoError(t, err)
	assert.Equal(t, "Novo Nome", result.Nome)
}

func TestUpdateDraft_Conflict(t *testing.T) {
	draftRepo := new(mockDraftRepo)
	uc := usecase.NewUpdateDraft(draftRepo)

	// Draft was updated 10 seconds ago; client last read 20 seconds ago → conflict
	updatedAt := time.Now().Add(-10 * time.Second)
	lastRead := time.Now().Add(-20 * time.Second)

	d := &domain.EventoDraft{
		ID:        "d-1",
		UsuarioID: "user-1",
		UpdatedAt: updatedAt,
	}
	in := portin.UpsertDraftInput{LastReadAt: &lastRead}

	draftRepo.On("FindByID", mock.Anything, "d-1").Return(d, nil)

	_, err := uc.Execute(context.Background(), "d-1", "user-1", in)
	require.Error(t, err)
	var ae *apierr.APIError
	assert.ErrorAs(t, err, &ae)
	assert.Equal(t, 409, ae.Status)
}

func TestUpdateDraft_Forbidden(t *testing.T) {
	draftRepo := new(mockDraftRepo)
	uc := usecase.NewUpdateDraft(draftRepo)

	d := sampleDraft("d-1", "owner-id")
	draftRepo.On("FindByID", mock.Anything, "d-1").Return(d, nil)

	_, err := uc.Execute(context.Background(), "d-1", "other-user", portin.UpsertDraftInput{})
	require.Error(t, err)
	var ae *apierr.APIError
	assert.ErrorAs(t, err, &ae)
	assert.Equal(t, 403, ae.Status)
}

// ---- DeleteDraft ----

func TestDeleteDraft_Success(t *testing.T) {
	draftRepo := new(mockDraftRepo)
	uc := usecase.NewDeleteDraft(draftRepo)

	d := sampleDraft("d-1", "user-1")
	draftRepo.On("FindByID", mock.Anything, "d-1").Return(d, nil)
	draftRepo.On("DeleteByID", mock.Anything, "d-1").Return(nil)

	err := uc.Execute(context.Background(), "d-1", "user-1")
	require.NoError(t, err)
	draftRepo.AssertExpectations(t)
}

// ---- PublishDraft ----

func TestPublishDraft_Success(t *testing.T) {
	draftRepo := new(mockDraftRepo)
	eventoRepo := new(mockEventoRepoForPublish)
	uc := usecase.NewPublishDraft(draftRepo, eventoRepo)

	d := sampleDraft("d-1", "user-1")
	savedEvt := &domain.Evento{ID: "evt-from-draft", UsuarioID: "user-1", Nome: "Festa"}

	draftRepo.On("FindByID", mock.Anything, "d-1").Return(d, nil)
	eventoRepo.On("Save", mock.Anything, mock.AnythingOfType("*event.Evento")).Return(savedEvt, nil)
	draftRepo.On("DeleteByID", mock.Anything, "d-1").Return(nil)

	result, err := uc.Execute(context.Background(), "d-1", "user-1")
	require.NoError(t, err)
	assert.Equal(t, "evt-from-draft", result.ID)
	draftRepo.AssertExpectations(t)
}

func TestPublishDraft_Forbidden(t *testing.T) {
	draftRepo := new(mockDraftRepo)
	eventoRepo := new(mockEventoRepoForPublish)
	uc := usecase.NewPublishDraft(draftRepo, eventoRepo)

	d := sampleDraft("d-1", "owner-id")
	draftRepo.On("FindByID", mock.Anything, "d-1").Return(d, nil)

	_, err := uc.Execute(context.Background(), "d-1", "intruder")
	require.Error(t, err)
	var ae *apierr.APIError
	assert.ErrorAs(t, err, &ae)
	assert.Equal(t, 403, ae.Status)
}
