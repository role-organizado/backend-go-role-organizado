package auth_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/auth"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	"github.com/role-organizado/backend-go-role-organizado/internal/usecase/auth"
)

func TestAddUserRole_AppendsNewRole(t *testing.T) {
	repo := &mockUsuarioRepo{}
	u := &domain.Usuario{ID: "u1", Roles: []domain.Role{domain.RoleUser}}
	repo.On("FindByID", mock.Anything, "u1").Return(u, nil)
	repo.On("Update", mock.Anything, mock.Anything).Return(u, nil)

	got, err := auth.NewAddUserRole(repo).Execute(context.Background(), portin.ModifyUserRoleInput{
		UsuarioID: "u1", Role: domain.RoleAdmin,
	})
	require.NoError(t, err)
	assert.True(t, got.HasRole(domain.RoleAdmin))
	repo.AssertCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestAddUserRole_Idempotent(t *testing.T) {
	repo := &mockUsuarioRepo{}
	u := &domain.Usuario{ID: "u1", Roles: []domain.Role{domain.RoleUser, domain.RoleAdmin}}
	repo.On("FindByID", mock.Anything, "u1").Return(u, nil)

	got, err := auth.NewAddUserRole(repo).Execute(context.Background(), portin.ModifyUserRoleInput{
		UsuarioID: "u1", Role: domain.RoleAdmin,
	})
	require.NoError(t, err)
	assert.Len(t, got.Roles, 2)
	repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestAddUserRole_InvalidRole(t *testing.T) {
	repo := &mockUsuarioRepo{}
	_, err := auth.NewAddUserRole(repo).Execute(context.Background(), portin.ModifyUserRoleInput{
		UsuarioID: "u1", Role: domain.Role("SUPERUSER"),
	})
	require.Error(t, err)
	repo.AssertNotCalled(t, "FindByID", mock.Anything, mock.Anything)
}

func TestRemoveUserRole_RemovesRole(t *testing.T) {
	repo := &mockUsuarioRepo{}
	u := &domain.Usuario{ID: "u1", Roles: []domain.Role{domain.RoleUser, domain.RoleAdmin}}
	repo.On("FindByID", mock.Anything, "u1").Return(u, nil)
	repo.On("Update", mock.Anything, mock.Anything).Return(u, nil)

	got, err := auth.NewRemoveUserRole(repo).Execute(context.Background(), portin.ModifyUserRoleInput{
		UsuarioID: "u1", Role: domain.RoleAdmin,
	})
	require.NoError(t, err)
	assert.False(t, got.HasRole(domain.RoleAdmin))
	assert.True(t, got.HasRole(domain.RoleUser))
}

func TestRemoveUserRole_LastRoleRejected(t *testing.T) {
	repo := &mockUsuarioRepo{}
	u := &domain.Usuario{ID: "u1", Roles: []domain.Role{domain.RoleUser}}
	repo.On("FindByID", mock.Anything, "u1").Return(u, nil)

	_, err := auth.NewRemoveUserRole(repo).Execute(context.Background(), portin.ModifyUserRoleInput{
		UsuarioID: "u1", Role: domain.RoleUser,
	})
	require.Error(t, err)
	repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestRemoveUserRole_AbsentRoleNoop(t *testing.T) {
	repo := &mockUsuarioRepo{}
	u := &domain.Usuario{ID: "u1", Roles: []domain.Role{domain.RoleUser, domain.RoleAdmin}}
	repo.On("FindByID", mock.Anything, "u1").Return(u, nil)

	got, err := auth.NewRemoveUserRole(repo).Execute(context.Background(), portin.ModifyUserRoleInput{
		UsuarioID: "u1", Role: domain.RoleModerator,
	})
	require.NoError(t, err)
	assert.Len(t, got.Roles, 2)
	repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}
