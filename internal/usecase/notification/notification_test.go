package notification_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/notification"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	ucnotif "github.com/role-organizado/backend-go-role-organizado/internal/usecase/notification"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ---- mock ----

type mockNotifRepo struct{ mock.Mock }

func (m *mockNotifRepo) FindByID(ctx context.Context, id string) (*domain.Notificacao, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Notificacao), args.Error(1)
}
func (m *mockNotifRepo) FindByUsuarioID(ctx context.Context, usuarioID string, page, pageSize int) ([]domain.Notificacao, int64, error) {
	args := m.Called(ctx, usuarioID, page, pageSize)
	return args.Get(0).([]domain.Notificacao), args.Get(1).(int64), args.Error(2)
}
func (m *mockNotifRepo) FindUnreadByUsuarioID(ctx context.Context, usuarioID string) ([]domain.Notificacao, error) {
	args := m.Called(ctx, usuarioID)
	return args.Get(0).([]domain.Notificacao), args.Error(1)
}
func (m *mockNotifRepo) Save(ctx context.Context, n *domain.Notificacao) (*domain.Notificacao, error) {
	args := m.Called(ctx, n)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Notificacao), args.Error(1)
}
func (m *mockNotifRepo) Update(ctx context.Context, n *domain.Notificacao) (*domain.Notificacao, error) {
	args := m.Called(ctx, n)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Notificacao), args.Error(1)
}
func (m *mockNotifRepo) MarkAllRead(ctx context.Context, usuarioID string) error {
	args := m.Called(ctx, usuarioID)
	return args.Error(0)
}
func (m *mockNotifRepo) DeleteByID(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// ---- helper ----

func unreadNotif(id, usuarioID string) *domain.Notificacao {
	return &domain.Notificacao{
		ID:        id,
		UsuarioID: usuarioID,
		Tipo:      domain.TipoNotificacaoEvento,
		Status:    domain.StatusNotificacaoNaoLida,
		Titulo:    "Título",
		Mensagem:  "Mensagem",
		CriadoEm: time.Now(),
	}
}

// ---- ListNotificacoes ----

func TestListNotificacoes_Success(t *testing.T) {
	repo := new(mockNotifRepo)
	uc := ucnotif.NewListNotificacoes(repo)

	items := []domain.Notificacao{*unreadNotif("n1", "u1")}
	repo.On("FindByUsuarioID", mock.Anything, "u1", 1, 20).Return(items, int64(1), nil)

	got, total, err := uc.Execute(context.Background(), "u1", 1, 20)
	require.NoError(t, err)
	assert.Len(t, got, 1)
	assert.Equal(t, int64(1), total)
}

// ---- GetNotificacao ----

func TestGetNotificacao_Success(t *testing.T) {
	repo := new(mockNotifRepo)
	uc := ucnotif.NewGetNotificacao(repo)

	n := unreadNotif("n1", "u1")
	repo.On("FindByID", mock.Anything, "n1").Return(n, nil)

	got, err := uc.Execute(context.Background(), "n1", "u1")
	require.NoError(t, err)
	assert.Equal(t, "n1", got.ID)
}

func TestGetNotificacao_Forbidden(t *testing.T) {
	repo := new(mockNotifRepo)
	uc := ucnotif.NewGetNotificacao(repo)

	n := unreadNotif("n1", "u1")
	repo.On("FindByID", mock.Anything, "n1").Return(n, nil)

	_, err := uc.Execute(context.Background(), "n1", "other-user")
	require.Error(t, err)
	ae := err.(*apierr.APIError)
	assert.Equal(t, 403, ae.Status)
}

// ---- CreateNotificacao ----

func TestCreateNotificacao_Success(t *testing.T) {
	repo := new(mockNotifRepo)
	uc := ucnotif.NewCreateNotificacao(repo)

	expected := unreadNotif("n1", "u1")
	repo.On("Save", mock.Anything, mock.AnythingOfType("*notification.Notificacao")).Return(expected, nil)

	got, err := uc.Execute(context.Background(), portin.CreateNotificacaoInput{
		UsuarioID: "u1",
		Tipo:      domain.TipoNotificacaoEvento,
		Titulo:    "Título",
		Mensagem:  "Mensagem",
	})
	require.NoError(t, err)
	assert.Equal(t, "n1", got.ID)
}

// ---- MarcarLida ----

func TestMarcarLida_Success(t *testing.T) {
	repo := new(mockNotifRepo)
	uc := ucnotif.NewMarcarLida(repo)

	n := unreadNotif("n1", "u1")
	marked := *n
	marked.Status = domain.StatusNotificacaoLida
	repo.On("FindByID", mock.Anything, "n1").Return(n, nil)
	repo.On("Update", mock.Anything, mock.AnythingOfType("*notification.Notificacao")).Return(&marked, nil)

	got, err := uc.Execute(context.Background(), "n1", "u1")
	require.NoError(t, err)
	assert.Equal(t, domain.StatusNotificacaoLida, got.Status)
}

func TestMarcarLida_Forbidden(t *testing.T) {
	repo := new(mockNotifRepo)
	uc := ucnotif.NewMarcarLida(repo)

	n := unreadNotif("n1", "u1")
	repo.On("FindByID", mock.Anything, "n1").Return(n, nil)

	_, err := uc.Execute(context.Background(), "n1", "other")
	require.Error(t, err)
	ae := err.(*apierr.APIError)
	assert.Equal(t, 403, ae.Status)
}

func TestMarcarLida_AlreadyRead_Idempotent(t *testing.T) {
	repo := new(mockNotifRepo)
	uc := ucnotif.NewMarcarLida(repo)

	n := unreadNotif("n1", "u1")
	n.Status = domain.StatusNotificacaoLida
	repo.On("FindByID", mock.Anything, "n1").Return(n, nil)
	// Update should NOT be called for idempotent case

	got, err := uc.Execute(context.Background(), "n1", "u1")
	require.NoError(t, err)
	assert.Equal(t, domain.StatusNotificacaoLida, got.Status)
	repo.AssertNotCalled(t, "Update")
}

// ---- MarcarTodasLidas ----

func TestMarcarTodasLidas_Success(t *testing.T) {
	repo := new(mockNotifRepo)
	uc := ucnotif.NewMarcarTodasLidas(repo)

	repo.On("MarkAllRead", mock.Anything, "u1").Return(nil)

	err := uc.Execute(context.Background(), "u1")
	require.NoError(t, err)
}

// ---- DeleteNotificacao ----

func TestDeleteNotificacao_Success(t *testing.T) {
	repo := new(mockNotifRepo)
	uc := ucnotif.NewDeleteNotificacao(repo)

	n := unreadNotif("n1", "u1")
	repo.On("FindByID", mock.Anything, "n1").Return(n, nil)
	repo.On("DeleteByID", mock.Anything, "n1").Return(nil)

	err := uc.Execute(context.Background(), "n1", "u1")
	require.NoError(t, err)
}

// ---- CountUnread ----

func TestCountUnread_Success(t *testing.T) {
	repo := new(mockNotifRepo)
	uc := ucnotif.NewCountUnread(repo)

	items := []domain.Notificacao{*unreadNotif("n1", "u1"), *unreadNotif("n2", "u1")}
	repo.On("FindUnreadByUsuarioID", mock.Anything, "u1").Return(items, nil)

	count, err := uc.Execute(context.Background(), "u1")
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}
