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

// Use valid UUID strings — rateioDocument stores evento_id / usuario_id_responsavel
// as bson.Binary subtype 4.  uuidStringToBinary falls back to uuid.New() (random)
// for non-UUID inputs, so each call produces a different binary and queries never match.
const (
	testEventoUUID  = "550e8400-e29b-41d4-a716-446655440000"
	testUsuarioUUID = "6ba7b810-9dad-11d1-80b4-00c04fd430c8"
	testEvtListUUID = "f47ac10b-58cc-4372-a567-0e02b2c3d479"
	testEvtDelUUID  = "a6c8e2d4-1f3b-4e5a-8c7d-9e0f1a2b3c4d"
	testUsrDelUUID  = "b7d9f3e5-2a4c-5f6b-9d8e-0f1a2b3c4d5e"
)

func TestRateioRepository_SaveAndFindByID(t *testing.T) {
	client := testhelper.StartMongo(t)
	repo := mongodb.NewRateioRepository(client)
	ctx := context.Background()

	rat := &rateioDomain.Rateio{
		EventoID:   testEventoUUID,
		UsuarioID:  testUsuarioUUID,
		Tipo:       rateioDomain.TipoRateioDivisao,
		ValorTotal: 150.00,
	}

	saved, err := repo.Save(ctx, rat)
	require.NoError(t, err)
	assert.NotEmpty(t, saved.ID)
	assert.Equal(t, 150.00, saved.ValorTotal)

	found, err := repo.FindByID(ctx, saved.ID)
	require.NoError(t, err)
	// EventoID round-trips through bson.Binary → UUID string correctly
	assert.Equal(t, testEventoUUID, found.EventoID)
	assert.Equal(t, rateioDomain.TipoRateioDivisao, found.Tipo)
	// UsuarioID round-trips correctly
	assert.Equal(t, testUsuarioUUID, found.UsuarioID)
}

func TestRateioRepository_FindByEventoID(t *testing.T) {
	client := testhelper.StartMongo(t)
	repo := mongodb.NewRateioRepository(client)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		_, err := repo.Save(ctx, &rateioDomain.Rateio{
			EventoID:   testEvtListUUID,
			UsuarioID:  testUsuarioUUID,
			Tipo:       rateioDomain.TipoRateioPercentual,
			ValorTotal: 100.0,
		})
		require.NoError(t, err)
	}

	// Save one rateio for a DIFFERENT event — must not appear in results
	_, err := repo.Save(ctx, &rateioDomain.Rateio{
		EventoID:   testEventoUUID,
		UsuarioID:  testUsuarioUUID,
		Tipo:       rateioDomain.TipoRateioDivisao,
		ValorTotal: 50.0,
	})
	require.NoError(t, err)

	rateios, err := repo.FindByEventoID(ctx, testEvtListUUID)
	require.NoError(t, err)
	assert.Len(t, rateios, 3)

	// Verify each returned rateio has the correct EventoID
	for _, r := range rateios {
		assert.Equal(t, testEvtListUUID, r.EventoID)
	}
}

func TestRateioRepository_DeleteByID(t *testing.T) {
	client := testhelper.StartMongo(t)
	repo := mongodb.NewRateioRepository(client)
	ctx := context.Background()

	saved, err := repo.Save(ctx, &rateioDomain.Rateio{
		EventoID:   testEvtDelUUID,
		UsuarioID:  testUsrDelUUID,
		Tipo:       rateioDomain.TipoRateioItens,
		ValorTotal: 200.0,
	})
	require.NoError(t, err)

	err = repo.DeleteByID(ctx, saved.ID)
	require.NoError(t, err)

	_, err = repo.FindByID(ctx, saved.ID)
	assert.True(t, apierr.IsNotFound(err))
}
