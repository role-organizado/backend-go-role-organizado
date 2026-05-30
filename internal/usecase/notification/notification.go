package notification

import (
	"context"
	"time"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/notification"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ---- ListNotificacoes ----

// ListNotificacoes lists paginated notifications for a user.
type ListNotificacoes struct {
	repo portout.NotificacaoRepository
}

// NewListNotificacoes creates a ListNotificacoes use case.
func NewListNotificacoes(repo portout.NotificacaoRepository) *ListNotificacoes {
	return &ListNotificacoes{repo: repo}
}

func (uc *ListNotificacoes) Execute(ctx context.Context, usuarioID string, page, pageSize int) ([]domain.Notificacao, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return uc.repo.FindByUsuarioID(ctx, usuarioID, page, pageSize)
}

// Ensure interface compliance.
var _ portin.ListNotificacoesUseCase = (*ListNotificacoes)(nil)

// ---- GetNotificacao ----

// GetNotificacao retrieves a notification by ID and validates ownership.
type GetNotificacao struct {
	repo portout.NotificacaoRepository
}

// NewGetNotificacao creates a GetNotificacao use case.
func NewGetNotificacao(repo portout.NotificacaoRepository) *GetNotificacao {
	return &GetNotificacao{repo: repo}
}

func (uc *GetNotificacao) Execute(ctx context.Context, id, requesterID string) (*domain.Notificacao, error) {
	n, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !n.IsOwner(requesterID) {
		return nil, apierr.Forbidden("acesso negado à notificação")
	}
	return n, nil
}

var _ portin.GetNotificacaoUseCase = (*GetNotificacao)(nil)

// ---- CreateNotificacao ----

// CreateNotificacao creates a new in-app notification.
type CreateNotificacao struct {
	repo portout.NotificacaoRepository
}

// NewCreateNotificacao creates a CreateNotificacao use case.
func NewCreateNotificacao(repo portout.NotificacaoRepository) *CreateNotificacao {
	return &CreateNotificacao{repo: repo}
}

func (uc *CreateNotificacao) Execute(ctx context.Context, in portin.CreateNotificacaoInput) (*domain.Notificacao, error) {
	n := &domain.Notificacao{
		UsuarioID: in.UsuarioID,
		Tipo:      in.Tipo,
		Titulo:    in.Titulo,
		Mensagem:  in.Mensagem,
		Dados:     in.Dados,
		Status:    domain.StatusNotificacaoNaoLida,
		CriadoEm: time.Now(),
	}
	return uc.repo.Save(ctx, n)
}

var _ portin.CreateNotificacaoUseCase = (*CreateNotificacao)(nil)

// ---- MarcarLida ----

// MarcarLida marks a single notification as read.
type MarcarLida struct {
	repo portout.NotificacaoRepository
}

// NewMarcarLida creates a MarcarLida use case.
func NewMarcarLida(repo portout.NotificacaoRepository) *MarcarLida {
	return &MarcarLida{repo: repo}
}

func (uc *MarcarLida) Execute(ctx context.Context, id, requesterID string) (*domain.Notificacao, error) {
	n, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !n.IsOwner(requesterID) {
		return nil, apierr.Forbidden("acesso negado à notificação")
	}
	if n.IsLida() {
		return n, nil // idempotent
	}
	n.Marcar()
	return uc.repo.Update(ctx, n)
}

var _ portin.MarcarLidaUseCase = (*MarcarLida)(nil)

// ---- MarcarTodasLidas ----

// MarcarTodasLidas marks all notifications of a user as read.
type MarcarTodasLidas struct {
	repo portout.NotificacaoRepository
}

// NewMarcarTodasLidas creates a MarcarTodasLidas use case.
func NewMarcarTodasLidas(repo portout.NotificacaoRepository) *MarcarTodasLidas {
	return &MarcarTodasLidas{repo: repo}
}

func (uc *MarcarTodasLidas) Execute(ctx context.Context, usuarioID string) error {
	return uc.repo.MarkAllRead(ctx, usuarioID)
}

var _ portin.MarcarTodasLidasUseCase = (*MarcarTodasLidas)(nil)

// ---- DeleteNotificacao ----

// DeleteNotificacao removes a notification by ID with ownership check.
type DeleteNotificacao struct {
	repo portout.NotificacaoRepository
}

// NewDeleteNotificacao creates a DeleteNotificacao use case.
func NewDeleteNotificacao(repo portout.NotificacaoRepository) *DeleteNotificacao {
	return &DeleteNotificacao{repo: repo}
}

func (uc *DeleteNotificacao) Execute(ctx context.Context, id, requesterID string) error {
	n, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if !n.IsOwner(requesterID) {
		return apierr.Forbidden("acesso negado à notificação")
	}
	return uc.repo.DeleteByID(ctx, id)
}

var _ portin.DeleteNotificacaoUseCase = (*DeleteNotificacao)(nil)

// ---- CountUnread ----

// CountUnread returns the count of unread notifications for a user.
type CountUnread struct {
	repo portout.NotificacaoRepository
}

// NewCountUnread creates a CountUnread use case.
func NewCountUnread(repo portout.NotificacaoRepository) *CountUnread {
	return &CountUnread{repo: repo}
}

func (uc *CountUnread) Execute(ctx context.Context, usuarioID string) (int, error) {
	items, err := uc.repo.FindUnreadByUsuarioID(ctx, usuarioID)
	if err != nil {
		return 0, err
	}
	return len(items), nil
}

var _ portin.CountUnreadUseCase = (*CountUnread)(nil)
