//go:build integration

package mongodb_test

import (
	"context"
	"testing"
	"time"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/mongodb"
	"github.com/role-organizado/backend-go-role-organizado/internal/domain/social"
	"github.com/role-organizado/backend-go-role-organizado/internal/testhelper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// UUIDs used across tests — must be valid UUID v4 strings because the participants
// collection stores evento_id as UUID binary (see participant.go / rateio_integration_test.go).
const (
	socialTestEvento1   = "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	socialTestEvento2   = "b2c3d4e5-f6a7-8901-bcde-f12345678901"
	socialTestEvento3   = "c3d4e5f6-a7b8-9012-cdef-123456789012"
	socialTestEvento4   = "d4e5f6a7-b8c9-0123-defa-234567890123"
	socialTestEvento5   = "e5f6a7b8-c9d0-1234-efab-345678901234"
	socialTestEvento6   = "f6a7b8c9-d0e1-2345-fabc-456789012345"
	socialClaimUserID   = "a7b8c9d0-e1f2-3456-abcd-567890123456"
	socialCheckinUserID = "b8c9d0e1-f2a3-4567-bcde-678901234567"
)

// ---- FindOrCreate ----

func TestSocialFeaturesRepository_FindOrCreate_Idempotent(t *testing.T) {
	client := testhelper.StartMongo(t)
	repo := mongodb.NewSocialFeaturesRepository(client)
	ctx := context.Background()

	// First call creates the document.
	first, err := repo.FindOrCreate(ctx, socialTestEvento1)
	require.NoError(t, err)
	require.NotNil(t, first)
	assert.Equal(t, socialTestEvento1, first.EventoID)
	assert.NotEmpty(t, first.ID)
	assert.False(t, first.CheckinHabilitado)
	assert.Empty(t, first.Playlists)
	assert.Empty(t, first.BringList)
	assert.Empty(t, first.Checkins)
	assert.Empty(t, first.AlbumLinks)
	assert.Nil(t, first.DressCode)

	// Second call must return the exact same document (same _id).
	second, err := repo.FindOrCreate(ctx, socialTestEvento1)
	require.NoError(t, err)
	require.NotNil(t, second)
	assert.Equal(t, first.ID, second.ID, "FindOrCreate must be idempotent")
	assert.Equal(t, socialTestEvento1, second.EventoID)
}

// ---- FindByEventoID ----

func TestSocialFeaturesRepository_FindByEventoID_NotFound(t *testing.T) {
	client := testhelper.StartMongo(t)
	repo := mongodb.NewSocialFeaturesRepository(client)
	ctx := context.Background()

	result, err := repo.FindByEventoID(ctx, "nonexistent-event-id")
	require.NoError(t, err)
	assert.Nil(t, result, "FindByEventoID must return nil (no error) when document does not exist")
}

func TestSocialFeaturesRepository_FindByEventoID_Found(t *testing.T) {
	client := testhelper.StartMongo(t)
	repo := mongodb.NewSocialFeaturesRepository(client)
	ctx := context.Background()

	_, err := repo.FindOrCreate(ctx, socialTestEvento2)
	require.NoError(t, err)

	result, err := repo.FindByEventoID(ctx, socialTestEvento2)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, socialTestEvento2, result.EventoID)
}

// ---- DressCode ----

func TestSocialFeaturesRepository_SetDressCode(t *testing.T) {
	client := testhelper.StartMongo(t)
	repo := mongodb.NewSocialFeaturesRepository(client)
	ctx := context.Background()

	dc := &social.DressCode{
		Tipo:              social.DressCodeTematico,
		DescricaoTematico: "Anos 80",
	}

	// SetDressCode should upsert (lazy create).
	err := repo.SetDressCode(ctx, socialTestEvento3, dc)
	require.NoError(t, err)

	sf, err := repo.FindByEventoID(ctx, socialTestEvento3)
	require.NoError(t, err)
	require.NotNil(t, sf)
	require.NotNil(t, sf.DressCode)
	assert.Equal(t, social.DressCodeTematico, sf.DressCode.Tipo)
	assert.Equal(t, "Anos 80", sf.DressCode.DescricaoTematico)

	// Replacing with nil removes the dress code.
	err = repo.SetDressCode(ctx, socialTestEvento3, nil)
	require.NoError(t, err)

	sfAfter, err := repo.FindByEventoID(ctx, socialTestEvento3)
	require.NoError(t, err)
	require.NotNil(t, sfAfter)
	assert.Nil(t, sfAfter.DressCode, "SetDressCode(nil) must clear dress code")
}

// ---- Playlists ----

func TestSocialFeaturesRepository_AddAndRemovePlaylist(t *testing.T) {
	client := testhelper.StartMongo(t)
	repo := mongodb.NewSocialFeaturesRepository(client)
	ctx := context.Background()

	_, err := repo.FindOrCreate(ctx, socialTestEvento4)
	require.NoError(t, err)

	provider, embedURL := social.DetectPlaylistProvider("https://open.spotify.com/playlist/abc123def456")
	playlist := social.PlaylistLink{
		ID:       "pl-001",
		URL:      "https://open.spotify.com/playlist/abc123def456",
		Nome:     "Festa da Firma",
		Provider: provider,
		EmbedURL: embedURL,
	}

	err = repo.AddPlaylist(ctx, socialTestEvento4, playlist)
	require.NoError(t, err)

	sf, err := repo.FindByEventoID(ctx, socialTestEvento4)
	require.NoError(t, err)
	require.Len(t, sf.Playlists, 1)
	assert.Equal(t, "pl-001", sf.Playlists[0].ID)
	assert.Equal(t, social.PlaylistSpotify, sf.Playlists[0].Provider)
	assert.NotEmpty(t, sf.Playlists[0].EmbedURL)

	// RemovePlaylist must be idempotent.
	err = repo.RemovePlaylist(ctx, socialTestEvento4, "pl-001")
	require.NoError(t, err)

	sfAfter, err := repo.FindByEventoID(ctx, socialTestEvento4)
	require.NoError(t, err)
	assert.Empty(t, sfAfter.Playlists)

	// Second remove is a no-op.
	err = repo.RemovePlaylist(ctx, socialTestEvento4, "pl-001")
	require.NoError(t, err)
}

// ---- BringList ----

func TestSocialFeaturesRepository_ClaimBringListItem_AlreadyClaimed(t *testing.T) {
	client := testhelper.StartMongo(t)
	repo := mongodb.NewSocialFeaturesRepository(client)
	ctx := context.Background()

	_, err := repo.FindOrCreate(ctx, socialTestEvento5)
	require.NoError(t, err)

	const itemID = "item-carvao"
	err = repo.AddBringListItem(ctx, socialTestEvento5, social.BringListItem{
		ID:         itemID,
		Nome:       "Carvão",
		Quantidade: "2kg",
	})
	require.NoError(t, err)

	// First claim should succeed.
	now := time.Now()
	err = repo.ClaimBringListItem(ctx, socialTestEvento5, itemID, socialClaimUserID, "João Silva", now)
	require.NoError(t, err)

	// Second claim (different user) must return conflict.
	err = repo.ClaimBringListItem(ctx, socialTestEvento5, itemID, "other-user-id", "Maria", now)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "BRING_LIST_ITEM_ALREADY_CLAIMED",
		"second claim on the same item must return BRING_LIST_ITEM_ALREADY_CLAIMED")

	// Verify the claim was preserved (first user still holds it).
	sf, err := repo.FindByEventoID(ctx, socialTestEvento5)
	require.NoError(t, err)
	require.Len(t, sf.BringList, 1)
	assert.Equal(t, socialClaimUserID, sf.BringList[0].ClaimedBy)
}

func TestSocialFeaturesRepository_UnclaimBringListItem_Idempotent(t *testing.T) {
	client := testhelper.StartMongo(t)
	repo := mongodb.NewSocialFeaturesRepository(client)
	ctx := context.Background()

	_, err := repo.FindOrCreate(ctx, socialTestEvento4)
	require.NoError(t, err)

	const itemID = "item-bebida"
	err = repo.AddBringListItem(ctx, socialTestEvento4, social.BringListItem{
		ID:   itemID,
		Nome: "Bebida",
	})
	require.NoError(t, err)

	now := time.Now()
	err = repo.ClaimBringListItem(ctx, socialTestEvento4, itemID, socialClaimUserID, "João", now)
	require.NoError(t, err)

	// Unclaim clears the claim fields.
	err = repo.UnclaimBringListItem(ctx, socialTestEvento4, itemID)
	require.NoError(t, err)

	sf, err := repo.FindByEventoID(ctx, socialTestEvento4)
	require.NoError(t, err)
	require.Len(t, sf.BringList, 1)
	assert.Empty(t, sf.BringList[0].ClaimedBy, "unclaim must clear claimedBy")

	// Item can now be re-claimed.
	err = repo.ClaimBringListItem(ctx, socialTestEvento4, itemID, "another-user", "Maria", time.Now())
	require.NoError(t, err)

	// Second unclaim must be idempotent (no error).
	err = repo.UnclaimBringListItem(ctx, socialTestEvento4, itemID)
	require.NoError(t, err)
}

func TestSocialFeaturesRepository_RemoveBringListItem_Idempotent(t *testing.T) {
	client := testhelper.StartMongo(t)
	repo := mongodb.NewSocialFeaturesRepository(client)
	ctx := context.Background()

	_, err := repo.FindOrCreate(ctx, socialTestEvento5)
	require.NoError(t, err)

	err = repo.AddBringListItem(ctx, socialTestEvento5, social.BringListItem{
		ID:   "item-gelo",
		Nome: "Gelo",
	})
	require.NoError(t, err)

	err = repo.RemoveBringListItem(ctx, socialTestEvento5, "item-gelo")
	require.NoError(t, err)

	// Second remove must be a no-op.
	err = repo.RemoveBringListItem(ctx, socialTestEvento5, "item-gelo")
	require.NoError(t, err)
}

func TestSocialFeaturesRepository_UpdateBringListItem(t *testing.T) {
	client := testhelper.StartMongo(t)
	repo := mongodb.NewSocialFeaturesRepository(client)
	ctx := context.Background()

	_, err := repo.FindOrCreate(ctx, socialTestEvento3)
	require.NoError(t, err)

	const itemID = "item-salada"
	err = repo.AddBringListItem(ctx, socialTestEvento3, social.BringListItem{
		ID:         itemID,
		Nome:       "Salada",
		Quantidade: "1",
	})
	require.NoError(t, err)

	err = repo.UpdateBringListItem(ctx, socialTestEvento3, itemID, "Salada Caesar", "2 porções")
	require.NoError(t, err)

	sf, err := repo.FindByEventoID(ctx, socialTestEvento3)
	require.NoError(t, err)
	require.Len(t, sf.BringList, 1)
	assert.Equal(t, "Salada Caesar", sf.BringList[0].Nome)
	assert.Equal(t, "2 porções", sf.BringList[0].Quantidade)
}

// ---- Checkin ----

func TestSocialFeaturesRepository_AddCheckin_Duplicate(t *testing.T) {
	client := testhelper.StartMongo(t)
	repo := mongodb.NewSocialFeaturesRepository(client)
	ctx := context.Background()

	_, err := repo.FindOrCreate(ctx, socialTestEvento6)
	require.NoError(t, err)

	checkin := social.Checkin{
		UsuarioID: socialCheckinUserID,
		Nome:      "Maria Souza",
		Timestamp: time.Now(),
	}

	// First check-in must succeed.
	err = repo.AddCheckin(ctx, socialTestEvento6, checkin)
	require.NoError(t, err)

	// Duplicate check-in for same user must fail.
	err = repo.AddCheckin(ctx, socialTestEvento6, checkin)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CHECKIN_ALREADY_REGISTERED",
		"duplicate check-in must return CHECKIN_ALREADY_REGISTERED")

	// Verify only one check-in is stored.
	sf, err := repo.FindByEventoID(ctx, socialTestEvento6)
	require.NoError(t, err)
	assert.Len(t, sf.Checkins, 1)
}

func TestSocialFeaturesRepository_SetCheckinHabilitado(t *testing.T) {
	client := testhelper.StartMongo(t)
	repo := mongodb.NewSocialFeaturesRepository(client)
	ctx := context.Background()

	// Upsert: no prior FindOrCreate needed.
	err := repo.SetCheckinHabilitado(ctx, socialTestEvento2, true)
	require.NoError(t, err)

	sf, err := repo.FindByEventoID(ctx, socialTestEvento2)
	require.NoError(t, err)
	require.NotNil(t, sf)
	assert.True(t, sf.CheckinHabilitado)

	err = repo.SetCheckinHabilitado(ctx, socialTestEvento2, false)
	require.NoError(t, err)

	sfAfter, err := repo.FindByEventoID(ctx, socialTestEvento2)
	require.NoError(t, err)
	assert.False(t, sfAfter.CheckinHabilitado)
}

// ---- AlbumLinks ----

func TestSocialFeaturesRepository_AddAndRemoveAlbumLink(t *testing.T) {
	client := testhelper.StartMongo(t)
	repo := mongodb.NewSocialFeaturesRepository(client)
	ctx := context.Background()

	_, err := repo.FindOrCreate(ctx, socialTestEvento1)
	require.NoError(t, err)

	provider := social.DetectAlbumProvider("https://photos.google.com/share/album123")
	link := social.AlbumLink{
		ID:            "album-001",
		URL:           "https://photos.google.com/share/album123",
		Nome:          "Fotos da Festa",
		Provider:      provider,
		AdicionadoPor: socialClaimUserID,
	}

	err = repo.AddAlbumLink(ctx, socialTestEvento1, link)
	require.NoError(t, err)

	sf, err := repo.FindByEventoID(ctx, socialTestEvento1)
	require.NoError(t, err)
	require.Len(t, sf.AlbumLinks, 1)
	assert.Equal(t, "album-001", sf.AlbumLinks[0].ID)
	assert.Equal(t, social.AlbumGooglePhotos, sf.AlbumLinks[0].Provider)

	// RemoveAlbumLink must be idempotent.
	err = repo.RemoveAlbumLink(ctx, socialTestEvento1, "album-001")
	require.NoError(t, err)

	sfAfter, err := repo.FindByEventoID(ctx, socialTestEvento1)
	require.NoError(t, err)
	assert.Empty(t, sfAfter.AlbumLinks)

	// Second remove is a no-op.
	err = repo.RemoveAlbumLink(ctx, socialTestEvento1, "album-001")
	require.NoError(t, err)
}
