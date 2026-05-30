//go:build integration

package mongodb_test

import (
	"context"
	"testing"
	"time"

	eventDomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/event"
	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/mongodb"
	"github.com/role-organizado/backend-go-role-organizado/internal/testhelper"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventoRepository_SaveAndFindByID(t *testing.T) {
	client := testhelper.StartMongo(t)
	repo := mongodb.NewEventoRepository(client)
	ctx := context.Background()

	e := &eventDomain.Evento{
		UsuarioID: "user-001",
		Nome:      "Test Event",
		Tipo:      "festa",
		Data:      time.Now().Add(24 * time.Hour),
		Status:    eventDomain.EventoStatusPublicado,
	}

	saved, err := repo.Save(ctx, e)
	require.NoError(t, err)
	assert.NotEmpty(t, saved.ID)
	assert.Equal(t, "Test Event", saved.Nome)

	found, err := repo.FindByID(ctx, saved.ID)
	require.NoError(t, err)
	assert.Equal(t, "user-001", found.UsuarioID)
}

func TestEventoRepository_FindByID_NotFound(t *testing.T) {
	client := testhelper.StartMongo(t)
	repo := mongodb.NewEventoRepository(client)
	ctx := context.Background()

	_, err := repo.FindByID(ctx, "000000000000000000000001")
	require.Error(t, err)
	assert.True(t, apierr.IsNotFound(err))
}

func TestEventoRepository_DeleteByID(t *testing.T) {
	client := testhelper.StartMongo(t)
	repo := mongodb.NewEventoRepository(client)
	ctx := context.Background()

	e := &eventDomain.Evento{
		UsuarioID: "user-del",
		Nome:      "To Delete",
		Tipo:      "churras",
		Data:      time.Now().Add(48 * time.Hour),
		Status:    eventDomain.EventoStatusPublicado,
	}
	saved, err := repo.Save(ctx, e)
	require.NoError(t, err)

	err = repo.DeleteByID(ctx, saved.ID)
	require.NoError(t, err)

	_, err = repo.FindByID(ctx, saved.ID)
	assert.True(t, apierr.IsNotFound(err))
}

func TestEventoRepository_FindAll_Pagination(t *testing.T) {
	client := testhelper.StartMongo(t)
	repo := mongodb.NewEventoRepository(client)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		e := &eventDomain.Evento{
			UsuarioID: "user-page",
			Nome:      "Event",
			Tipo:      "rolê",
			Data:      time.Now().Add(time.Duration(i) * 24 * time.Hour),
			Status:    eventDomain.EventoStatusPublicado,
		}
		_, err := repo.Save(ctx, e)
		require.NoError(t, err)
	}

	eventos, total, err := repo.FindAll(ctx, 1, 3)
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, eventos, 3)
}
