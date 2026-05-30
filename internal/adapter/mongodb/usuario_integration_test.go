//go:build integration

// Package mongodb_integration contains integration tests for MongoDB adapters.
// Run with: go test -tags=integration ./internal/adapter/mongodb/... -v
package mongodb_test

import (
	"context"
	"testing"

	authDomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/auth"
	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/mongodb"
	"github.com/role-organizado/backend-go-role-organizado/internal/testhelper"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUsuarioRepository_SaveAndFindByEmail(t *testing.T) {
	client := testhelper.StartMongo(t)
	repo := mongodb.NewUsuarioRepository(client)
	ctx := context.Background()

	u := &authDomain.Usuario{
		Nome:      "Test User",
		Email:     "test@example.com",
		SenhaHash: "hash",
		Ativo:     true,
		Roles:     []authDomain.Role{authDomain.RoleUser},
	}

	saved, err := repo.Save(ctx, u)
	require.NoError(t, err)
	assert.NotEmpty(t, saved.ID)
	assert.Equal(t, "test@example.com", saved.Email)

	found, err := repo.FindByEmail(ctx, "test@example.com")
	require.NoError(t, err)
	assert.Equal(t, saved.ID, found.ID)
	assert.Equal(t, "Test User", found.Nome)
}

func TestUsuarioRepository_FindByEmail_NotFound(t *testing.T) {
	client := testhelper.StartMongo(t)
	repo := mongodb.NewUsuarioRepository(client)
	ctx := context.Background()

	_, err := repo.FindByEmail(ctx, "nonexistent@example.com")
	require.Error(t, err)
	assert.True(t, apierr.IsNotFound(err))
}

func TestUsuarioRepository_FindByID(t *testing.T) {
	client := testhelper.StartMongo(t)
	repo := mongodb.NewUsuarioRepository(client)
	ctx := context.Background()

	u := &authDomain.Usuario{
		Nome:      "ID Test",
		Email:     "id@test.com",
		SenhaHash: "hash",
		Ativo:     true,
		Roles:     []authDomain.Role{authDomain.RoleUser},
	}
	saved, err := repo.Save(ctx, u)
	require.NoError(t, err)

	found, err := repo.FindByID(ctx, saved.ID)
	require.NoError(t, err)
	assert.Equal(t, "ID Test", found.Nome)
}

func TestUsuarioRepository_Update(t *testing.T) {
	client := testhelper.StartMongo(t)
	repo := mongodb.NewUsuarioRepository(client)
	ctx := context.Background()

	u := &authDomain.Usuario{
		Nome:      "Old Name",
		Email:     "update@test.com",
		SenhaHash: "hash",
		Ativo:     true,
		Roles:     []authDomain.Role{authDomain.RoleUser},
	}
	saved, err := repo.Save(ctx, u)
	require.NoError(t, err)

	saved.Nome = "New Name"
	updated, err := repo.Update(ctx, saved)
	require.NoError(t, err)
	assert.Equal(t, "New Name", updated.Nome)
}
