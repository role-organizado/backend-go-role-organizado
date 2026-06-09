package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/handler"
	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/middleware"
	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/social"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
)

// ---- mock use cases ----

type mockGetSocialFeaturesUC struct{ mock.Mock }

func (m *mockGetSocialFeaturesUC) Execute(ctx context.Context, in portin.GetSocialFeaturesInput) (*domain.EventoSocialFeatures, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.EventoSocialFeatures), args.Error(1)
}

type mockSetDressCodeUC struct{ mock.Mock }

func (m *mockSetDressCodeUC) Execute(ctx context.Context, in portin.SetDressCodeInput) (*domain.EventoSocialFeatures, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.EventoSocialFeatures), args.Error(1)
}

type mockRemoveDressCodeUC struct{ mock.Mock }

func (m *mockRemoveDressCodeUC) Execute(ctx context.Context, in portin.RemoveDressCodeInput) error {
	args := m.Called(ctx, in)
	return args.Error(0)
}

type mockAddPlaylistUC struct{ mock.Mock }

func (m *mockAddPlaylistUC) Execute(ctx context.Context, in portin.AddPlaylistInput) (*domain.PlaylistLink, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.PlaylistLink), args.Error(1)
}

type mockRemovePlaylistUC struct{ mock.Mock }

func (m *mockRemovePlaylistUC) Execute(ctx context.Context, in portin.RemovePlaylistInput) error {
	args := m.Called(ctx, in)
	return args.Error(0)
}

type mockAddBringListItemUC struct{ mock.Mock }

func (m *mockAddBringListItemUC) Execute(ctx context.Context, in portin.AddBringListItemInput) (*domain.BringListItem, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.BringListItem), args.Error(1)
}

type mockUpdateBringListItemUC struct{ mock.Mock }

func (m *mockUpdateBringListItemUC) Execute(ctx context.Context, in portin.UpdateBringListItemInput) error {
	args := m.Called(ctx, in)
	return args.Error(0)
}

type mockRemoveBringListItemUC struct{ mock.Mock }

func (m *mockRemoveBringListItemUC) Execute(ctx context.Context, in portin.RemoveBringListItemInput) error {
	args := m.Called(ctx, in)
	return args.Error(0)
}

type mockClaimBringListItemUC struct{ mock.Mock }

func (m *mockClaimBringListItemUC) Execute(ctx context.Context, in portin.ClaimBringListItemInput) error {
	args := m.Called(ctx, in)
	return args.Error(0)
}

type mockUnclaimBringListItemUC struct{ mock.Mock }

func (m *mockUnclaimBringListItemUC) Execute(ctx context.Context, in portin.UnclaimBringListItemInput) error {
	args := m.Called(ctx, in)
	return args.Error(0)
}

type mockSetCheckinHabilitadoUC struct{ mock.Mock }

func (m *mockSetCheckinHabilitadoUC) Execute(ctx context.Context, in portin.SetCheckinHabilitadoInput) error {
	args := m.Called(ctx, in)
	return args.Error(0)
}

type mockDoCheckinUC struct{ mock.Mock }

func (m *mockDoCheckinUC) Execute(ctx context.Context, in portin.DoCheckinInput) (*domain.Checkin, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Checkin), args.Error(1)
}

type mockAddAlbumLinkUC struct{ mock.Mock }

func (m *mockAddAlbumLinkUC) Execute(ctx context.Context, in portin.AddAlbumLinkInput) (*domain.AlbumLink, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.AlbumLink), args.Error(1)
}

type mockRemoveAlbumLinkUC struct{ mock.Mock }

func (m *mockRemoveAlbumLinkUC) Execute(ctx context.Context, in portin.RemoveAlbumLinkInput) error {
	args := m.Called(ctx, in)
	return args.Error(0)
}

// ---- helpers ----

// socialMocks bundles all 14 mock use cases for convenient test setup.
type socialMocks struct {
	get                  *mockGetSocialFeaturesUC
	setDressCode         *mockSetDressCodeUC
	removeDressCode      *mockRemoveDressCodeUC
	addPlaylist          *mockAddPlaylistUC
	removePlaylist       *mockRemovePlaylistUC
	addBringListItem     *mockAddBringListItemUC
	updateBringListItem  *mockUpdateBringListItemUC
	removeBringListItem  *mockRemoveBringListItemUC
	claimBringListItem   *mockClaimBringListItemUC
	unclaimBringListItem *mockUnclaimBringListItemUC
	setCheckinHabilitado *mockSetCheckinHabilitadoUC
	doCheckin            *mockDoCheckinUC
	addAlbumLink         *mockAddAlbumLinkUC
	removeAlbumLink      *mockRemoveAlbumLinkUC
}

func newSocialMocks() *socialMocks {
	return &socialMocks{
		get:                  new(mockGetSocialFeaturesUC),
		setDressCode:         new(mockSetDressCodeUC),
		removeDressCode:      new(mockRemoveDressCodeUC),
		addPlaylist:          new(mockAddPlaylistUC),
		removePlaylist:       new(mockRemovePlaylistUC),
		addBringListItem:     new(mockAddBringListItemUC),
		updateBringListItem:  new(mockUpdateBringListItemUC),
		removeBringListItem:  new(mockRemoveBringListItemUC),
		claimBringListItem:   new(mockClaimBringListItemUC),
		unclaimBringListItem: new(mockUnclaimBringListItemUC),
		setCheckinHabilitado: new(mockSetCheckinHabilitadoUC),
		doCheckin:            new(mockDoCheckinUC),
		addAlbumLink:         new(mockAddAlbumLinkUC),
		removeAlbumLink:      new(mockRemoveAlbumLinkUC),
	}
}

func newSocialRouter(m *socialMocks) *chi.Mux {
	h := handler.NewSocialHandler(
		m.get,
		m.setDressCode,
		m.removeDressCode,
		m.addPlaylist,
		m.removePlaylist,
		m.addBringListItem,
		m.updateBringListItem,
		m.removeBringListItem,
		m.claimBringListItem,
		m.unclaimBringListItem,
		m.setCheckinHabilitado,
		m.doCheckin,
		m.addAlbumLink,
		m.removeAlbumLink,
	)
	r := chi.NewRouter()
	h.RegisterSocialRoutes(r)
	return r
}

const testEventoID = "evento-test-123"

// ---- TestSocial_RoutesRegistered ----

// TestSocial_RoutesRegistered verifies all 14 social routes are mounted.
// chi returns 404 for unregistered paths; any other status confirms the route exists.
func TestSocial_RoutesRegistered(t *testing.T) {
	m := newSocialMocks()
	r := newSocialRouter(m)

	routes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/eventos/" + testEventoID + "/social/"},
		{http.MethodPut, "/api/v1/eventos/" + testEventoID + "/social/dress-code"},
		{http.MethodDelete, "/api/v1/eventos/" + testEventoID + "/social/dress-code"},
		{http.MethodPost, "/api/v1/eventos/" + testEventoID + "/social/playlists"},
		{http.MethodDelete, "/api/v1/eventos/" + testEventoID + "/social/playlists/pl-1"},
		{http.MethodPost, "/api/v1/eventos/" + testEventoID + "/social/bring-list"},
		{http.MethodPut, "/api/v1/eventos/" + testEventoID + "/social/bring-list/item-1"},
		{http.MethodDelete, "/api/v1/eventos/" + testEventoID + "/social/bring-list/item-1"},
		{http.MethodPost, "/api/v1/eventos/" + testEventoID + "/social/bring-list/item-1/claim"},
		{http.MethodDelete, "/api/v1/eventos/" + testEventoID + "/social/bring-list/item-1/claim"},
		{http.MethodPut, "/api/v1/eventos/" + testEventoID + "/social/checkin/habilitado"},
		{http.MethodPost, "/api/v1/eventos/" + testEventoID + "/social/checkin"},
		{http.MethodPost, "/api/v1/eventos/" + testEventoID + "/social/album-links"},
		{http.MethodDelete, "/api/v1/eventos/" + testEventoID + "/social/album-links/link-1"},
	}

	for _, tc := range routes {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.NotEqual(t, http.StatusNotFound, w.Code,
				"route %s %s should be registered", tc.method, tc.path)
			assert.NotEqual(t, http.StatusMethodNotAllowed, w.Code,
				"route %s %s should accept the correct HTTP method", tc.method, tc.path)
		})
	}
}

// ---- TestSocial_RequiresAuth ----

// TestSocial_RequiresAuth confirms every endpoint returns 401 when no userID is in context.
func TestSocial_RequiresAuth(t *testing.T) {
	m := newSocialMocks()
	r := newSocialRouter(m)

	routes := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/v1/eventos/" + testEventoID + "/social/", ""},
		{http.MethodPut, "/api/v1/eventos/" + testEventoID + "/social/dress-code", `{"tipo":"CASUAL"}`},
		{http.MethodDelete, "/api/v1/eventos/" + testEventoID + "/social/dress-code", ""},
		{http.MethodPost, "/api/v1/eventos/" + testEventoID + "/social/playlists", `{"url":"https://spotify.com/playlist/abc"}`},
		{http.MethodDelete, "/api/v1/eventos/" + testEventoID + "/social/playlists/pl-1", ""},
		{http.MethodPost, "/api/v1/eventos/" + testEventoID + "/social/bring-list", `{"nome":"Cerveja"}`},
		{http.MethodPut, "/api/v1/eventos/" + testEventoID + "/social/bring-list/item-1", `{"nome":"Cerveja","quantidade":"2"}`},
		{http.MethodDelete, "/api/v1/eventos/" + testEventoID + "/social/bring-list/item-1", ""},
		{http.MethodPost, "/api/v1/eventos/" + testEventoID + "/social/bring-list/item-1/claim", ""},
		{http.MethodDelete, "/api/v1/eventos/" + testEventoID + "/social/bring-list/item-1/claim", ""},
		{http.MethodPut, "/api/v1/eventos/" + testEventoID + "/social/checkin/habilitado", `{"checkinHabilitado":true}`},
		{http.MethodPost, "/api/v1/eventos/" + testEventoID + "/social/checkin", ""},
		{http.MethodPost, "/api/v1/eventos/" + testEventoID + "/social/album-links", `{"url":"https://photos.google.com/share/abc"}`},
		{http.MethodDelete, "/api/v1/eventos/" + testEventoID + "/social/album-links/link-1", ""},
	}

	for _, tc := range routes {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			var body *bytes.Reader
			if tc.body != "" {
				body = bytes.NewReader([]byte(tc.body))
			} else {
				body = bytes.NewReader(nil)
			}
			req := httptest.NewRequest(tc.method, tc.path, body)
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusUnauthorized, w.Code,
				"route %s %s should return 401 without auth", tc.method, tc.path)
		})
	}
}

// ---- TestSocial_GetSocialFeatures_Success ----

func TestSocial_GetSocialFeatures_Success(t *testing.T) {
	m := newSocialMocks()
	r := newSocialRouter(m)

	expectedDoc := &domain.EventoSocialFeatures{
		ID:                "doc-1",
		EventoID:          testEventoID,
		Playlists:         []domain.PlaylistLink{},
		BringList:         []domain.BringListItem{},
		Checkins:          []domain.Checkin{},
		AlbumLinks:        []domain.AlbumLink{},
		CheckinHabilitado: false,
	}

	m.get.On("Execute", mock.Anything, portin.GetSocialFeaturesInput{
		EventoID:  testEventoID,
		UsuarioID: "user-abc",
	}).Return(expectedDoc, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/eventos/"+testEventoID+"/social/", nil)
	req = req.WithContext(middleware.WithUserIDContext(req.Context(), "user-abc"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, testEventoID, resp["eventoId"])
	assert.NotNil(t, resp["playlists"])
	assert.NotNil(t, resp["bringList"])
	assert.NotNil(t, resp["checkins"])
	assert.NotNil(t, resp["albumLinks"])

	m.get.AssertExpectations(t)
}

// ---- TestSocial_SetDressCode_Success ----

func TestSocial_SetDressCode_Success(t *testing.T) {
	m := newSocialMocks()
	r := newSocialRouter(m)

	dc := &domain.DressCode{Tipo: domain.DressCodeCasual}
	returnedDoc := &domain.EventoSocialFeatures{
		EventoID:  testEventoID,
		DressCode: dc,
		Playlists: []domain.PlaylistLink{},
		BringList: []domain.BringListItem{},
		Checkins:  []domain.Checkin{},
		AlbumLinks: []domain.AlbumLink{},
	}

	m.setDressCode.On("Execute", mock.Anything, portin.SetDressCodeInput{
		EventoID:  testEventoID,
		UsuarioID: "org-user",
		Tipo:      "CASUAL",
	}).Return(returnedDoc, nil)

	body := `{"tipo":"CASUAL"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/eventos/"+testEventoID+"/social/dress-code",
		bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(middleware.WithUserIDContext(req.Context(), "org-user"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, testEventoID, resp["eventoId"])
	m.setDressCode.AssertExpectations(t)
}

// ---- TestSocial_RemoveDressCode_NoContent ----

func TestSocial_RemoveDressCode_NoContent(t *testing.T) {
	m := newSocialMocks()
	r := newSocialRouter(m)

	m.removeDressCode.On("Execute", mock.Anything, portin.RemoveDressCodeInput{
		EventoID:  testEventoID,
		UsuarioID: "org-user",
	}).Return(nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/eventos/"+testEventoID+"/social/dress-code", nil)
	req = req.WithContext(middleware.WithUserIDContext(req.Context(), "org-user"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	m.removeDressCode.AssertExpectations(t)
}

// ---- TestSocial_AddPlaylist_Created ----

func TestSocial_AddPlaylist_Created(t *testing.T) {
	m := newSocialMocks()
	r := newSocialRouter(m)

	playlistURL := "https://open.spotify.com/playlist/37i9dQZF1DX0XUsuxWHRQd"
	returnedPlaylist := &domain.PlaylistLink{
		ID:       "pl-new-1",
		URL:      playlistURL,
		Nome:     "Minha Playlist",
		Provider: domain.PlaylistSpotify,
		EmbedURL: "https://open.spotify.com/embed/playlist/37i9dQZF1DX0XUsuxWHRQd?utm_source=generator&theme=0",
	}

	m.addPlaylist.On("Execute", mock.Anything, portin.AddPlaylistInput{
		EventoID:  testEventoID,
		UsuarioID: "org-user",
		URL:       playlistURL,
		Nome:      "Minha Playlist",
	}).Return(returnedPlaylist, nil)

	body, _ := json.Marshal(map[string]string{"url": playlistURL, "nome": "Minha Playlist"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/eventos/"+testEventoID+"/social/playlists",
		bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(middleware.WithUserIDContext(req.Context(), "org-user"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "pl-new-1", resp["id"])
	assert.Equal(t, "SPOTIFY", resp["provider"])
	m.addPlaylist.AssertExpectations(t)
}

// ---- TestSocial_DoCheckin_Success ----

func TestSocial_DoCheckin_Success(t *testing.T) {
	m := newSocialMocks()
	r := newSocialRouter(m)

	now := time.Now()
	returnedCheckin := &domain.Checkin{
		UsuarioID: "participant-1",
		Nome:      "João Silva",
		Timestamp: now,
	}

	m.doCheckin.On("Execute", mock.Anything, portin.DoCheckinInput{
		EventoID:    testEventoID,
		UsuarioID:   "participant-1",
		UsuarioNome: "", // no JWT claims in test ctx
	}).Return(returnedCheckin, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/eventos/"+testEventoID+"/social/checkin", nil)
	req = req.WithContext(middleware.WithUserIDContext(req.Context(), "participant-1"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "participant-1", resp["usuarioId"])
	m.doCheckin.AssertExpectations(t)
}

// ---- TestSocial_AddBringListItem_Created ----

func TestSocial_AddBringListItem_Created(t *testing.T) {
	m := newSocialMocks()
	r := newSocialRouter(m)

	returnedItem := &domain.BringListItem{
		ID:         "item-new-1",
		Nome:       "Cerveja",
		Quantidade: "2 caixas",
	}

	m.addBringListItem.On("Execute", mock.Anything, portin.AddBringListItemInput{
		EventoID:   testEventoID,
		UsuarioID:  "org-user",
		Nome:       "Cerveja",
		Quantidade: "2 caixas",
	}).Return(returnedItem, nil)

	body, _ := json.Marshal(map[string]string{"nome": "Cerveja", "quantidade": "2 caixas"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/eventos/"+testEventoID+"/social/bring-list",
		bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(middleware.WithUserIDContext(req.Context(), "org-user"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "item-new-1", resp["id"])
	assert.Equal(t, "Cerveja", resp["nome"])
	m.addBringListItem.AssertExpectations(t)
}

// ---- TestSocial_AddAlbumLink_Created ----

func TestSocial_AddAlbumLink_Created(t *testing.T) {
	m := newSocialMocks()
	r := newSocialRouter(m)

	albumURL := "https://photos.google.com/share/album-abc"
	returnedLink := &domain.AlbumLink{
		ID:            "link-new-1",
		URL:           albumURL,
		Nome:          "Fotos do Evento",
		Provider:      domain.AlbumGooglePhotos,
		AdicionadoPor: "participant-1",
	}

	m.addAlbumLink.On("Execute", mock.Anything, portin.AddAlbumLinkInput{
		EventoID:  testEventoID,
		UsuarioID: "participant-1",
		URL:       albumURL,
		Nome:      "Fotos do Evento",
	}).Return(returnedLink, nil)

	body, _ := json.Marshal(map[string]string{"url": albumURL, "nome": "Fotos do Evento"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/eventos/"+testEventoID+"/social/album-links",
		bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(middleware.WithUserIDContext(req.Context(), "participant-1"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "link-new-1", resp["id"])
	assert.Equal(t, "GOOGLE_PHOTOS", resp["provider"])
	assert.Equal(t, "participant-1", resp["adicionadoPor"])
	m.addAlbumLink.AssertExpectations(t)
}

// ---- TestSocial_UpdateBringListItem_Accepted ----

func TestSocial_UpdateBringListItem_Accepted(t *testing.T) {
	m := newSocialMocks()
	r := newSocialRouter(m)

	m.updateBringListItem.On("Execute", mock.Anything, portin.UpdateBringListItemInput{
		EventoID:   testEventoID,
		ItemID:     "item-1",
		UsuarioID:  "org-user",
		Nome:       "Cerveja Gelada",
		Quantidade: "3 caixas",
	}).Return(nil)

	body, _ := json.Marshal(map[string]string{"nome": "Cerveja Gelada", "quantidade": "3 caixas"})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/eventos/"+testEventoID+"/social/bring-list/item-1",
		bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(middleware.WithUserIDContext(req.Context(), "org-user"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
	m.updateBringListItem.AssertExpectations(t)
}

// ---- TestSocial_SetCheckinHabilitado_Accepted ----

func TestSocial_SetCheckinHabilitado_Accepted(t *testing.T) {
	m := newSocialMocks()
	r := newSocialRouter(m)

	m.setCheckinHabilitado.On("Execute", mock.Anything, portin.SetCheckinHabilitadoInput{
		EventoID:   testEventoID,
		UsuarioID:  "org-user",
		Habilitado: true,
	}).Return(nil)

	body, _ := json.Marshal(map[string]bool{"checkinHabilitado": true})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/eventos/"+testEventoID+"/social/checkin/habilitado",
		bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(middleware.WithUserIDContext(req.Context(), "org-user"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
	m.setCheckinHabilitado.AssertExpectations(t)
}
