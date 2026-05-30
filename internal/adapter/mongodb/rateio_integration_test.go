//go:build integration

package mongodb_test

import (
	"context"
	"testing"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/mongodb"
	rateioDomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/rateio"
	"github.com/role-organizado/backend-go-role-organizado/internal/testhelper"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRateioRepository_SaveAndFindByID(t *testing.T) {
	client := testhelper.StartMongo(t)
	repo := mongodb.NewRateioRepository(client)
	ctx := context.Background()

	rat := &rateioDomain.Rateio{
		EventoID:   "evt-001",
		UsuarioID:  "usr-001",
		Tipo:       rateioDomain.TipoRateioDivisao,
		ValorTotal: 150.00,
	}

	saved, err := repo.Save(ctx, rat)
	require.NoError(t, err)
	assert.NotEmpty(t, saved.ID)
	assert.Equal(t, 150.00, saved.ValorTotal)

	found, err := repo.FindByID(ctx, saved.ID)
	require.NoError(t, err)
	assert.Equal(t, "evt-001", found.EventoID)
	assert.Equal(t, rateioDomain.TipoRateioDivisao, found.Tipo)
}

func TestRateioRepository_FindByEventoID(t *testing.T) {
	client := testhelper.StartMongo(t)
	repo := mongodb.NewRateioRepository(client)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		_, err := repo.Save(ctx, &rateioDomain.Rateio{
			EventoID:   "evt-list",
			UsuarioID:  "usr-001",
			Tipo:       rateioDomain.TipoRateioPercentual,
			ValorTotal: 100.0,
		})
		require.NoError(t, err)
	}

	rateios, err := repo.FindByEventoID(ctx, "evt-list")
	require.NoError(t, err)
	assert.Len(t, rateios, 3)
}

func TestRateioRepository_DeleteByID(t *testing.T) {
	client := testhelper.StartMongo(t)
	repo := mongodb.NewRateioRepository(client)
	ctx := context.Background()

	saved, err := repo.Save(ctx, &rateioDomain.Rateio{
		EventoID:   "evt-del",
		UsuarioID:  "usr-del",
		Tipo:       rateioDomain.TipoRateioItens,
		ValorTotal: 200.0,
	})
	require.NoError(t, err)

	err = repo.DeleteByID(ctx, saved.ID)
	require.NoError(t, err)

	_, err = repo.FindByID(ctx, saved.ID)
	assert.True(t, apierr.IsNotFound(err))
}
