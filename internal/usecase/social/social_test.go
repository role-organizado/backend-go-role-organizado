package social_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/social"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	ucsocial "github.com/role-organizado/backend-go-role-organizado/internal/usecase/social"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ============================================================
// Mock implementations
// ============================================================

// mockSocialRepo mocks portout.SocialFeaturesRepository.
type mockSocialRepo struct{ mock.Mock }

func (m *mockSocialRepo) FindByEventoID(ctx context.Context, eventoID string) (*domain.EventoSocialFeatures, error) {
	args := m.Called(ctx, eventoID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.EventoSocialFeatures), args.Error(1)
}

func (m *mockSocialRepo) FindOrCreate(ctx context.Context, eventoID string) (*domain.EventoSocialFeatures, error) {
	args := m.Called(ctx, eventoID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.EventoSocialFeatures), args.Error(1)
}

func (m *mockSocialRepo) SetDressCode(ctx context.Context, eventoID string, dc *domain.DressCode) error {
	args := m.Called(ctx, eventoID, dc)
	return args.Error(0)
}

func (m *mockSocialRepo) AddPlaylist(ctx context.Context, eventoID string, p domain.PlaylistLink) error {
	args := m.Called(ctx, eventoID, p)
	return args.Error(0)
}

func (m *mockSocialRepo) RemovePlaylist(ctx context.Context, eventoID, playlistID string) error {
	args := m.Called(ctx, eventoID, playlistID)
	return args.Error(0)
}

func (m *mockSocialRepo) AddBringListItem(ctx context.Context, eventoID string, item domain.BringListItem) error {
	args := m.Called(ctx, eventoID, item)
	return args.Error(0)
}

func (m *mockSocialRepo) UpdateBringListItem(ctx context.Context, eventoID, itemID, nome, quantidade string) error {
	args := m.Called(ctx, eventoID, itemID, nome, quantidade)
	return args.Error(0)
}

func (m *mockSocialRepo) RemoveBringListItem(ctx context.Context, eventoID, itemID string) error {
	args := m.Called(ctx, eventoID, itemID)
	return args.Error(0)
}

func (m *mockSocialRepo) ClaimBringListItem(ctx context.Context, eventoID, itemID, usuarioID, usuarioNome string, claimedAt time.Time) error {
	args := m.Called(ctx, eventoID, itemID, usuarioID, usuarioNome, claimedAt)
	return args.Error(0)
}

func (m *mockSocialRepo) UnclaimBringListItem(ctx context.Context, eventoID, itemID string) error {
	args := m.Called(ctx, eventoID, itemID)
	return args.Error(0)
}

func (m *mockSocialRepo) SetCheckinHabilitado(ctx context.Context, eventoID string, habilitado bool) error {
	args := m.Called(ctx, eventoID, habilitado)
	return args.Error(0)
}

func (m *mockSocialRepo) AddCheckin(ctx context.Context, eventoID string, c domain.Checkin) error {
	args := m.Called(ctx, eventoID, c)
	return args.Error(0)
}

func (m *mockSocialRepo) AddAlbumLink(ctx context.Context, eventoID string, link domain.AlbumLink) error {
	args := m.Called(ctx, eventoID, link)
	return args.Error(0)
}

func (m *mockSocialRepo) RemoveAlbumLink(ctx context.Context, eventoID, linkID string) error {
	args := m.Called(ctx, eventoID, linkID)
	return args.Error(0)
}

// mockAuthPort mocks portout.EventoAuthPort.
type mockAuthPort struct{ mock.Mock }

func (m *mockAuthPort) FaseAtLeast(ctx context.Context, eventoID string, fase domain.EventoFase) error {
	args := m.Called(ctx, eventoID, fase)
	return args.Error(0)
}

func (m *mockAuthPort) IsOrganizador(ctx context.Context, eventoID, usuarioID string) (bool, error) {
	args := m.Called(ctx, eventoID, usuarioID)
	return args.Bool(0), args.Error(1)
}

func (m *mockAuthPort) IsParticipanteConfirmadoOuOrganizador(ctx context.Context, eventoID, usuarioID string) (bool, error) {
	args := m.Called(ctx, eventoID, usuarioID)
	return args.Bool(0), args.Error(1)
}

// ============================================================
// Helpers
// ============================================================

const (
	evtID  = "evt-123"
	userID = "usr-456"
)

func emptyDoc() *domain.EventoSocialFeatures {
	return &domain.EventoSocialFeatures{EventoID: evtID}
}

func docWithPlaylists(n int) *domain.EventoSocialFeatures {
	doc := emptyDoc()
	for range n {
		doc.Playlists = append(doc.Playlists, domain.PlaylistLink{ID: "pl", URL: "http://x.com", Provider: domain.PlaylistOther})
	}
	return doc
}

func docWithAlbums(n int) *domain.EventoSocialFeatures {
	doc := emptyDoc()
	for range n {
		doc.AlbumLinks = append(doc.AlbumLinks, domain.AlbumLink{ID: "al", URL: "http://x.com", Provider: domain.AlbumOther})
	}
	return doc
}

func docCheckinEnabled() *domain.EventoSocialFeatures {
	doc := emptyDoc()
	doc.CheckinHabilitado = true
	return doc
}

var errNotFound = apierr.NotFound("evento", evtID)
var errFase = apierr.Unprocessable("FASE_INSUFICIENTE")
var errDB = errors.New("db error")

// ============================================================
// GetSocialFeatures
// ============================================================

func TestGetSocialFeatures_Execute(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		in        portin.GetSocialFeaturesInput
		setupRepo func(*mockSocialRepo)
		setupAuth func(*mockAuthPort)
		wantErr   string
		wantEmpty bool // expect EventoID set but no doc in mongo
	}{
		{
			name: "success — existing doc",
			in:   portin.GetSocialFeaturesInput{EventoID: evtID, UsuarioID: userID},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(nil)
				a.On("IsParticipanteConfirmadoOuOrganizador", ctx, evtID, userID).Return(true, nil)
			},
			setupRepo: func(r *mockSocialRepo) {
				r.On("FindByEventoID", ctx, evtID).Return(emptyDoc(), nil)
			},
		},
		{
			name: "success — doc not found returns empty struct",
			in:   portin.GetSocialFeaturesInput{EventoID: evtID, UsuarioID: userID},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(nil)
				a.On("IsParticipanteConfirmadoOuOrganizador", ctx, evtID, userID).Return(true, nil)
			},
			setupRepo: func(r *mockSocialRepo) {
				r.On("FindByEventoID", ctx, evtID).Return(nil, nil)
			},
			wantEmpty: true,
		},
		{
			name: "fase check fails — event not found",
			in:   portin.GetSocialFeaturesInput{EventoID: evtID, UsuarioID: userID},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(errNotFound)
			},
			setupRepo: func(r *mockSocialRepo) {},
			wantErr:   "NOT_FOUND",
		},
		{
			name: "fase check fails — wrong phase",
			in:   portin.GetSocialFeaturesInput{EventoID: evtID, UsuarioID: userID},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(errFase)
			},
			setupRepo: func(r *mockSocialRepo) {},
			wantErr:   "FASE_INSUFICIENTE",
		},
		{
			name: "not authorized",
			in:   portin.GetSocialFeaturesInput{EventoID: evtID, UsuarioID: userID},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(nil)
				a.On("IsParticipanteConfirmadoOuOrganizador", ctx, evtID, userID).Return(false, nil)
			},
			setupRepo: func(r *mockSocialRepo) {},
			wantErr:   "FORBIDDEN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockSocialRepo{}
			auth := &mockAuthPort{}
			tt.setupRepo(repo)
			tt.setupAuth(auth)

			uc := ucsocial.NewGetSocialFeatures(repo, auth)
			doc, err := uc.Execute(ctx, tt.in)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, doc)
			} else {
				require.NoError(t, err)
				require.NotNil(t, doc)
				assert.Equal(t, evtID, doc.EventoID)
			}
			repo.AssertExpectations(t)
			auth.AssertExpectations(t)
		})
	}
}

// ============================================================
// SetDressCode
// ============================================================

func TestSetDressCode_Execute(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		in        portin.SetDressCodeInput
		setupRepo func(*mockSocialRepo)
		setupAuth func(*mockAuthPort)
		wantErr   string
	}{
		{
			name: "success — CASUAL",
			in:   portin.SetDressCodeInput{EventoID: evtID, UsuarioID: userID, Tipo: "CASUAL"},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(nil)
				a.On("IsOrganizador", ctx, evtID, userID).Return(true, nil)
			},
			setupRepo: func(r *mockSocialRepo) {
				r.On("FindOrCreate", ctx, evtID).Return(emptyDoc(), nil)
				r.On("SetDressCode", ctx, evtID, mock.MatchedBy(func(dc *domain.DressCode) bool {
					return dc != nil && dc.Tipo == domain.DressCodeCasual
				})).Return(nil)
			},
		},
		{
			name: "success — TEMATICO with descricao",
			in:   portin.SetDressCodeInput{EventoID: evtID, UsuarioID: userID, Tipo: "TEMATICO", DescricaoTematico: "Havaiano"},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(nil)
				a.On("IsOrganizador", ctx, evtID, userID).Return(true, nil)
			},
			setupRepo: func(r *mockSocialRepo) {
				r.On("FindOrCreate", ctx, evtID).Return(emptyDoc(), nil)
				r.On("SetDressCode", ctx, evtID, mock.MatchedBy(func(dc *domain.DressCode) bool {
					return dc != nil && dc.Tipo == domain.DressCodeTematico && dc.DescricaoTematico == "Havaiano"
				})).Return(nil)
			},
		},
		{
			name:      "missing tipo returns 400",
			in:        portin.SetDressCodeInput{EventoID: evtID, UsuarioID: userID},
			setupRepo: func(r *mockSocialRepo) {},
			setupAuth: func(a *mockAuthPort) {},
			wantErr:   "tipo",
		},
		{
			name:      "TEMATICO without descricao returns 400",
			in:        portin.SetDressCodeInput{EventoID: evtID, UsuarioID: userID, Tipo: "TEMATICO"},
			setupRepo: func(r *mockSocialRepo) {},
			setupAuth: func(a *mockAuthPort) {},
			wantErr:   "descricaoTematico",
		},
		{
			name: "fase check fails",
			in:   portin.SetDressCodeInput{EventoID: evtID, UsuarioID: userID, Tipo: "CASUAL"},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(errFase)
			},
			setupRepo: func(r *mockSocialRepo) {},
			wantErr:   "FASE_INSUFICIENTE",
		},
		{
			name: "not organizer returns 403",
			in:   portin.SetDressCodeInput{EventoID: evtID, UsuarioID: userID, Tipo: "CASUAL"},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(nil)
				a.On("IsOrganizador", ctx, evtID, userID).Return(false, nil)
			},
			setupRepo: func(r *mockSocialRepo) {},
			wantErr:   "FORBIDDEN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockSocialRepo{}
			auth := &mockAuthPort{}
			tt.setupRepo(repo)
			tt.setupAuth(auth)

			uc := ucsocial.NewSetDressCode(repo, auth)
			doc, err := uc.Execute(ctx, tt.in)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, doc)
			} else {
				require.NoError(t, err)
				require.NotNil(t, doc)
			}
			repo.AssertExpectations(t)
			auth.AssertExpectations(t)
		})
	}
}

// ============================================================
// RemoveDressCode
// ============================================================

func TestRemoveDressCode_Execute(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		in        portin.RemoveDressCodeInput
		setupRepo func(*mockSocialRepo)
		setupAuth func(*mockAuthPort)
		wantErr   string
	}{
		{
			name: "success — idempotent remove",
			in:   portin.RemoveDressCodeInput{EventoID: evtID, UsuarioID: userID},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(nil)
				a.On("IsOrganizador", ctx, evtID, userID).Return(true, nil)
			},
			setupRepo: func(r *mockSocialRepo) {
				r.On("SetDressCode", ctx, evtID, (*domain.DressCode)(nil)).Return(nil)
			},
		},
		{
			name: "fase check fails",
			in:   portin.RemoveDressCodeInput{EventoID: evtID, UsuarioID: userID},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(errNotFound)
			},
			setupRepo: func(r *mockSocialRepo) {},
			wantErr:   "NOT_FOUND",
		},
		{
			name: "not organizer returns 403",
			in:   portin.RemoveDressCodeInput{EventoID: evtID, UsuarioID: userID},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(nil)
				a.On("IsOrganizador", ctx, evtID, userID).Return(false, nil)
			},
			setupRepo: func(r *mockSocialRepo) {},
			wantErr:   "FORBIDDEN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockSocialRepo{}
			auth := &mockAuthPort{}
			tt.setupRepo(repo)
			tt.setupAuth(auth)

			uc := ucsocial.NewRemoveDressCode(repo, auth)
			err := uc.Execute(ctx, tt.in)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			repo.AssertExpectations(t)
			auth.AssertExpectations(t)
		})
	}
}

// ============================================================
// AddPlaylist
// ============================================================

func TestAddPlaylist_Execute(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		in        portin.AddPlaylistInput
		setupRepo func(*mockSocialRepo)
		setupAuth func(*mockAuthPort)
		wantErr   string
		wantProv  domain.PlaylistProvider
	}{
		{
			name: "success — spotify URL",
			in:   portin.AddPlaylistInput{EventoID: evtID, UsuarioID: userID, URL: "https://open.spotify.com/playlist/37i9dQZF1DXcBWIGoYBM5M", Nome: "Funk"},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(nil)
				a.On("IsOrganizador", ctx, evtID, userID).Return(true, nil)
			},
			setupRepo: func(r *mockSocialRepo) {
				r.On("FindOrCreate", ctx, evtID).Return(emptyDoc(), nil)
				r.On("AddPlaylist", ctx, evtID, mock.MatchedBy(func(p domain.PlaylistLink) bool {
					return p.Provider == domain.PlaylistSpotify && p.ID != ""
				})).Return(nil)
			},
			wantProv: domain.PlaylistSpotify,
		},
		{
			name: "success — other URL",
			in:   portin.AddPlaylistInput{EventoID: evtID, UsuarioID: userID, URL: "https://soundcloud.com/playlist/x", Nome: "SoundCloud"},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(nil)
				a.On("IsOrganizador", ctx, evtID, userID).Return(true, nil)
			},
			setupRepo: func(r *mockSocialRepo) {
				r.On("FindOrCreate", ctx, evtID).Return(emptyDoc(), nil)
				r.On("AddPlaylist", ctx, evtID, mock.MatchedBy(func(p domain.PlaylistLink) bool {
					return p.Provider == domain.PlaylistOther
				})).Return(nil)
			},
			wantProv: domain.PlaylistOther,
		},
		{
			name: "max playlists reached — returns 400",
			in:   portin.AddPlaylistInput{EventoID: evtID, UsuarioID: userID, URL: "https://x.com", Nome: "X"},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(nil)
				a.On("IsOrganizador", ctx, evtID, userID).Return(true, nil)
			},
			setupRepo: func(r *mockSocialRepo) {
				r.On("FindOrCreate", ctx, evtID).Return(docWithPlaylists(domain.MaxPlaylists), nil)
			},
			wantErr: "MAX_PLAYLISTS_REACHED",
		},
		{
			name: "fase check fails",
			in:   portin.AddPlaylistInput{EventoID: evtID, UsuarioID: userID, URL: "https://x.com", Nome: "X"},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(errFase)
			},
			setupRepo: func(r *mockSocialRepo) {},
			wantErr:   "FASE_INSUFICIENTE",
		},
		{
			name: "not organizer returns 403",
			in:   portin.AddPlaylistInput{EventoID: evtID, UsuarioID: userID, URL: "https://x.com", Nome: "X"},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(nil)
				a.On("IsOrganizador", ctx, evtID, userID).Return(false, nil)
			},
			setupRepo: func(r *mockSocialRepo) {},
			wantErr:   "FORBIDDEN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockSocialRepo{}
			auth := &mockAuthPort{}
			tt.setupRepo(repo)
			tt.setupAuth(auth)

			uc := ucsocial.NewAddPlaylist(repo, auth)
			pl, err := uc.Execute(ctx, tt.in)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, pl)
			} else {
				require.NoError(t, err)
				require.NotNil(t, pl)
				assert.NotEmpty(t, pl.ID)
				assert.Equal(t, tt.wantProv, pl.Provider)
			}
			repo.AssertExpectations(t)
			auth.AssertExpectations(t)
		})
	}
}

// ============================================================
// RemovePlaylist
// ============================================================

func TestRemovePlaylist_Execute(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		in        portin.RemovePlaylistInput
		setupRepo func(*mockSocialRepo)
		setupAuth func(*mockAuthPort)
		wantErr   string
	}{
		{
			name: "success — idempotent remove",
			in:   portin.RemovePlaylistInput{EventoID: evtID, UsuarioID: userID, PlaylistID: "pl-1"},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(nil)
				a.On("IsOrganizador", ctx, evtID, userID).Return(true, nil)
			},
			setupRepo: func(r *mockSocialRepo) {
				r.On("RemovePlaylist", ctx, evtID, "pl-1").Return(nil)
			},
		},
		{
			name: "not organizer returns 403",
			in:   portin.RemovePlaylistInput{EventoID: evtID, UsuarioID: userID, PlaylistID: "pl-1"},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(nil)
				a.On("IsOrganizador", ctx, evtID, userID).Return(false, nil)
			},
			setupRepo: func(r *mockSocialRepo) {},
			wantErr:   "FORBIDDEN",
		},
		{
			name: "fase check fails",
			in:   portin.RemovePlaylistInput{EventoID: evtID, UsuarioID: userID, PlaylistID: "pl-1"},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(errFase)
			},
			setupRepo: func(r *mockSocialRepo) {},
			wantErr:   "FASE_INSUFICIENTE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockSocialRepo{}
			auth := &mockAuthPort{}
			tt.setupRepo(repo)
			tt.setupAuth(auth)

			uc := ucsocial.NewRemovePlaylist(repo, auth)
			err := uc.Execute(ctx, tt.in)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			repo.AssertExpectations(t)
			auth.AssertExpectations(t)
		})
	}
}

// ============================================================
// AddBringListItem
// ============================================================

func TestAddBringListItem_Execute(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		in        portin.AddBringListItemInput
		setupRepo func(*mockSocialRepo)
		setupAuth func(*mockAuthPort)
		wantErr   string
	}{
		{
			name: "success",
			in:   portin.AddBringListItemInput{EventoID: evtID, UsuarioID: userID, Nome: "Cerveja", Quantidade: "2 caixas"},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(nil)
				a.On("IsOrganizador", ctx, evtID, userID).Return(true, nil)
			},
			setupRepo: func(r *mockSocialRepo) {
				r.On("FindOrCreate", ctx, evtID).Return(emptyDoc(), nil)
				r.On("AddBringListItem", ctx, evtID, mock.MatchedBy(func(item domain.BringListItem) bool {
					return item.Nome == "Cerveja" && item.ID != ""
				})).Return(nil)
			},
		},
		{
			name:      "missing nome returns 400",
			in:        portin.AddBringListItemInput{EventoID: evtID, UsuarioID: userID},
			setupRepo: func(r *mockSocialRepo) {},
			setupAuth: func(a *mockAuthPort) {},
			wantErr:   "nome",
		},
		{
			name: "not organizer returns 403",
			in:   portin.AddBringListItemInput{EventoID: evtID, UsuarioID: userID, Nome: "Gelo"},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(nil)
				a.On("IsOrganizador", ctx, evtID, userID).Return(false, nil)
			},
			setupRepo: func(r *mockSocialRepo) {},
			wantErr:   "FORBIDDEN",
		},
		{
			name: "repo error propagated",
			in:   portin.AddBringListItemInput{EventoID: evtID, UsuarioID: userID, Nome: "Salgado"},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(nil)
				a.On("IsOrganizador", ctx, evtID, userID).Return(true, nil)
			},
			setupRepo: func(r *mockSocialRepo) {
				r.On("FindOrCreate", ctx, evtID).Return(emptyDoc(), nil)
				r.On("AddBringListItem", ctx, evtID, mock.Anything).Return(errDB)
			},
			wantErr: "db error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockSocialRepo{}
			auth := &mockAuthPort{}
			tt.setupRepo(repo)
			tt.setupAuth(auth)

			uc := ucsocial.NewAddBringListItem(repo, auth)
			item, err := uc.Execute(ctx, tt.in)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, item)
			} else {
				require.NoError(t, err)
				require.NotNil(t, item)
				assert.NotEmpty(t, item.ID)
				assert.Equal(t, tt.in.Nome, item.Nome)
			}
			repo.AssertExpectations(t)
			auth.AssertExpectations(t)
		})
	}
}

// ============================================================
// UpdateBringListItem
// ============================================================

func TestUpdateBringListItem_Execute(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		in        portin.UpdateBringListItemInput
		setupRepo func(*mockSocialRepo)
		setupAuth func(*mockAuthPort)
		wantErr   string
	}{
		{
			name: "success",
			in:   portin.UpdateBringListItemInput{EventoID: evtID, UsuarioID: userID, ItemID: "item-1", Nome: "Cerveja gelada", Quantidade: "3 caixas"},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(nil)
				a.On("IsOrganizador", ctx, evtID, userID).Return(true, nil)
			},
			setupRepo: func(r *mockSocialRepo) {
				r.On("UpdateBringListItem", ctx, evtID, "item-1", "Cerveja gelada", "3 caixas").Return(nil)
			},
		},
		{
			name: "not organizer returns 403",
			in:   portin.UpdateBringListItemInput{EventoID: evtID, UsuarioID: userID, ItemID: "item-1", Nome: "X"},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(nil)
				a.On("IsOrganizador", ctx, evtID, userID).Return(false, nil)
			},
			setupRepo: func(r *mockSocialRepo) {},
			wantErr:   "FORBIDDEN",
		},
		{
			name: "fase check fails",
			in:   portin.UpdateBringListItemInput{EventoID: evtID, UsuarioID: userID, ItemID: "item-1"},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(errFase)
			},
			setupRepo: func(r *mockSocialRepo) {},
			wantErr:   "FASE_INSUFICIENTE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockSocialRepo{}
			auth := &mockAuthPort{}
			tt.setupRepo(repo)
			tt.setupAuth(auth)

			uc := ucsocial.NewUpdateBringListItem(repo, auth)
			err := uc.Execute(ctx, tt.in)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			repo.AssertExpectations(t)
			auth.AssertExpectations(t)
		})
	}
}

// ============================================================
// RemoveBringListItem
// ============================================================

func TestRemoveBringListItem_Execute(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		in        portin.RemoveBringListItemInput
		setupRepo func(*mockSocialRepo)
		setupAuth func(*mockAuthPort)
		wantErr   string
	}{
		{
			name: "success — idempotent",
			in:   portin.RemoveBringListItemInput{EventoID: evtID, UsuarioID: userID, ItemID: "item-1"},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(nil)
				a.On("IsOrganizador", ctx, evtID, userID).Return(true, nil)
			},
			setupRepo: func(r *mockSocialRepo) {
				r.On("RemoveBringListItem", ctx, evtID, "item-1").Return(nil)
			},
		},
		{
			name: "not organizer returns 403",
			in:   portin.RemoveBringListItemInput{EventoID: evtID, UsuarioID: userID, ItemID: "item-1"},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(nil)
				a.On("IsOrganizador", ctx, evtID, userID).Return(false, nil)
			},
			setupRepo: func(r *mockSocialRepo) {},
			wantErr:   "FORBIDDEN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockSocialRepo{}
			auth := &mockAuthPort{}
			tt.setupRepo(repo)
			tt.setupAuth(auth)

			uc := ucsocial.NewRemoveBringListItem(repo, auth)
			err := uc.Execute(ctx, tt.in)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			repo.AssertExpectations(t)
			auth.AssertExpectations(t)
		})
	}
}

// ============================================================
// ClaimBringListItem
// ============================================================

func TestClaimBringListItem_Execute(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		in        portin.ClaimBringListItemInput
		setupRepo func(*mockSocialRepo)
		setupAuth func(*mockAuthPort)
		wantErr   string
	}{
		{
			name: "success",
			in:   portin.ClaimBringListItemInput{EventoID: evtID, UsuarioID: userID, ItemID: "item-1", UsuarioNome: "Fulano"},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(nil)
				a.On("IsParticipanteConfirmadoOuOrganizador", ctx, evtID, userID).Return(true, nil)
			},
			setupRepo: func(r *mockSocialRepo) {
				r.On("ClaimBringListItem", ctx, evtID, "item-1", userID, "Fulano", mock.AnythingOfType("time.Time")).Return(nil)
			},
		},
		{
			name: "already claimed — returns 409 BRING_LIST_ITEM_ALREADY_CLAIMED",
			in:   portin.ClaimBringListItemInput{EventoID: evtID, UsuarioID: userID, ItemID: "item-2", UsuarioNome: "Fulano"},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(nil)
				a.On("IsParticipanteConfirmadoOuOrganizador", ctx, evtID, userID).Return(true, nil)
			},
			setupRepo: func(r *mockSocialRepo) {
				r.On("ClaimBringListItem", ctx, evtID, "item-2", userID, "Fulano", mock.AnythingOfType("time.Time")).
					Return(apierr.Conflict("BRING_LIST_ITEM_ALREADY_CLAIMED"))
			},
			wantErr: "BRING_LIST_ITEM_ALREADY_CLAIMED",
		},
		{
			name: "not participant returns 403",
			in:   portin.ClaimBringListItemInput{EventoID: evtID, UsuarioID: userID, ItemID: "item-1"},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(nil)
				a.On("IsParticipanteConfirmadoOuOrganizador", ctx, evtID, userID).Return(false, nil)
			},
			setupRepo: func(r *mockSocialRepo) {},
			wantErr:   "FORBIDDEN",
		},
		{
			name: "fase check fails",
			in:   portin.ClaimBringListItemInput{EventoID: evtID, UsuarioID: userID, ItemID: "item-1"},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(errNotFound)
			},
			setupRepo: func(r *mockSocialRepo) {},
			wantErr:   "NOT_FOUND",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockSocialRepo{}
			auth := &mockAuthPort{}
			tt.setupRepo(repo)
			tt.setupAuth(auth)

			uc := ucsocial.NewClaimBringListItem(repo, auth)
			err := uc.Execute(ctx, tt.in)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			repo.AssertExpectations(t)
			auth.AssertExpectations(t)
		})
	}
}

// ============================================================
// UnclaimBringListItem
// ============================================================

func TestUnclaimBringListItem_Execute(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		in        portin.UnclaimBringListItemInput
		setupRepo func(*mockSocialRepo)
		setupAuth func(*mockAuthPort)
		wantErr   string
	}{
		{
			name: "success — participant unclaims",
			in:   portin.UnclaimBringListItemInput{EventoID: evtID, UsuarioID: userID, ItemID: "item-1"},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(nil)
				a.On("IsParticipanteConfirmadoOuOrganizador", ctx, evtID, userID).Return(true, nil)
			},
			setupRepo: func(r *mockSocialRepo) {
				r.On("UnclaimBringListItem", ctx, evtID, "item-1").Return(nil)
			},
		},
		{
			name: "not authorized returns 403",
			in:   portin.UnclaimBringListItemInput{EventoID: evtID, UsuarioID: userID, ItemID: "item-1"},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(nil)
				a.On("IsParticipanteConfirmadoOuOrganizador", ctx, evtID, userID).Return(false, nil)
			},
			setupRepo: func(r *mockSocialRepo) {},
			wantErr:   "FORBIDDEN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockSocialRepo{}
			auth := &mockAuthPort{}
			tt.setupRepo(repo)
			tt.setupAuth(auth)

			uc := ucsocial.NewUnclaimBringListItem(repo, auth)
			err := uc.Execute(ctx, tt.in)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			repo.AssertExpectations(t)
			auth.AssertExpectations(t)
		})
	}
}

// ============================================================
// SetCheckinHabilitado
// ============================================================

func TestSetCheckinHabilitado_Execute(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		in        portin.SetCheckinHabilitadoInput
		setupRepo func(*mockSocialRepo)
		setupAuth func(*mockAuthPort)
		wantErr   string
	}{
		{
			name: "success — enable",
			in:   portin.SetCheckinHabilitadoInput{EventoID: evtID, UsuarioID: userID, Habilitado: true},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(nil)
				a.On("IsOrganizador", ctx, evtID, userID).Return(true, nil)
			},
			setupRepo: func(r *mockSocialRepo) {
				r.On("SetCheckinHabilitado", ctx, evtID, true).Return(nil)
			},
		},
		{
			name: "success — disable",
			in:   portin.SetCheckinHabilitadoInput{EventoID: evtID, UsuarioID: userID, Habilitado: false},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(nil)
				a.On("IsOrganizador", ctx, evtID, userID).Return(true, nil)
			},
			setupRepo: func(r *mockSocialRepo) {
				r.On("SetCheckinHabilitado", ctx, evtID, false).Return(nil)
			},
		},
		{
			name: "not organizer returns 403",
			in:   portin.SetCheckinHabilitadoInput{EventoID: evtID, UsuarioID: userID, Habilitado: true},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(nil)
				a.On("IsOrganizador", ctx, evtID, userID).Return(false, nil)
			},
			setupRepo: func(r *mockSocialRepo) {},
			wantErr:   "FORBIDDEN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockSocialRepo{}
			auth := &mockAuthPort{}
			tt.setupRepo(repo)
			tt.setupAuth(auth)

			uc := ucsocial.NewSetCheckinHabilitado(repo, auth)
			err := uc.Execute(ctx, tt.in)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			repo.AssertExpectations(t)
			auth.AssertExpectations(t)
		})
	}
}

// ============================================================
// DoCheckin
// ============================================================

func TestDoCheckin_Execute(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		in        portin.DoCheckinInput
		setupRepo func(*mockSocialRepo)
		setupAuth func(*mockAuthPort)
		wantErr   string
	}{
		{
			name: "success",
			in:   portin.DoCheckinInput{EventoID: evtID, UsuarioID: userID, UsuarioNome: "Fulano"},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(nil)
				a.On("IsParticipanteConfirmadoOuOrganizador", ctx, evtID, userID).Return(true, nil)
			},
			setupRepo: func(r *mockSocialRepo) {
				r.On("FindByEventoID", ctx, evtID).Return(docCheckinEnabled(), nil)
				r.On("AddCheckin", ctx, evtID, mock.MatchedBy(func(c domain.Checkin) bool {
					return c.UsuarioID == userID && c.Nome == "Fulano"
				})).Return(nil)
			},
		},
		{
			name: "checkin disabled — doc exists with flag false",
			in:   portin.DoCheckinInput{EventoID: evtID, UsuarioID: userID, UsuarioNome: "Fulano"},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(nil)
				a.On("IsParticipanteConfirmadoOuOrganizador", ctx, evtID, userID).Return(true, nil)
			},
			setupRepo: func(r *mockSocialRepo) {
				r.On("FindByEventoID", ctx, evtID).Return(emptyDoc(), nil) // CheckinHabilitado = false
			},
			wantErr: "CHECKIN_NOT_ENABLED",
		},
		{
			name: "checkin disabled — doc does not exist",
			in:   portin.DoCheckinInput{EventoID: evtID, UsuarioID: userID, UsuarioNome: "Fulano"},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(nil)
				a.On("IsParticipanteConfirmadoOuOrganizador", ctx, evtID, userID).Return(true, nil)
			},
			setupRepo: func(r *mockSocialRepo) {
				r.On("FindByEventoID", ctx, evtID).Return(nil, nil)
			},
			wantErr: "CHECKIN_NOT_ENABLED",
		},
		{
			name: "already checked in — returns 409 CHECKIN_ALREADY_REGISTERED",
			in:   portin.DoCheckinInput{EventoID: evtID, UsuarioID: userID, UsuarioNome: "Fulano"},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(nil)
				a.On("IsParticipanteConfirmadoOuOrganizador", ctx, evtID, userID).Return(true, nil)
			},
			setupRepo: func(r *mockSocialRepo) {
				r.On("FindByEventoID", ctx, evtID).Return(docCheckinEnabled(), nil)
				r.On("AddCheckin", ctx, evtID, mock.Anything).
					Return(apierr.Conflict("CHECKIN_ALREADY_REGISTERED"))
			},
			wantErr: "CHECKIN_ALREADY_REGISTERED",
		},
		{
			name: "not participant returns 403",
			in:   portin.DoCheckinInput{EventoID: evtID, UsuarioID: userID},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(nil)
				a.On("IsParticipanteConfirmadoOuOrganizador", ctx, evtID, userID).Return(false, nil)
			},
			setupRepo: func(r *mockSocialRepo) {},
			wantErr:   "FORBIDDEN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockSocialRepo{}
			auth := &mockAuthPort{}
			tt.setupRepo(repo)
			tt.setupAuth(auth)

			uc := ucsocial.NewDoCheckin(repo, auth)
			checkin, err := uc.Execute(ctx, tt.in)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, checkin)
			} else {
				require.NoError(t, err)
				require.NotNil(t, checkin)
				assert.Equal(t, userID, checkin.UsuarioID)
				assert.Equal(t, "Fulano", checkin.Nome)
				assert.False(t, checkin.Timestamp.IsZero())
			}
			repo.AssertExpectations(t)
			auth.AssertExpectations(t)
		})
	}
}

// ============================================================
// AddAlbumLink
// ============================================================

func TestAddAlbumLink_Execute(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		in        portin.AddAlbumLinkInput
		setupRepo func(*mockSocialRepo)
		setupAuth func(*mockAuthPort)
		wantErr   string
		wantProv  domain.AlbumProvider
	}{
		{
			name: "success — google photos URL — adicionadoPor from JWT",
			in:   portin.AddAlbumLinkInput{EventoID: evtID, UsuarioID: userID, URL: "https://photos.google.com/album/123", Nome: "Fotos"},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(nil)
				a.On("IsParticipanteConfirmadoOuOrganizador", ctx, evtID, userID).Return(true, nil)
			},
			setupRepo: func(r *mockSocialRepo) {
				r.On("FindOrCreate", ctx, evtID).Return(emptyDoc(), nil)
				r.On("AddAlbumLink", ctx, evtID, mock.MatchedBy(func(l domain.AlbumLink) bool {
					return l.AdicionadoPor == userID && l.Provider == domain.AlbumGooglePhotos && l.ID != ""
				})).Return(nil)
			},
			wantProv: domain.AlbumGooglePhotos,
		},
		{
			name: "success — iCloud URL",
			in:   portin.AddAlbumLinkInput{EventoID: evtID, UsuarioID: userID, URL: "https://www.icloud.com/photos/album123", Nome: "Fotos iCloud"},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(nil)
				a.On("IsParticipanteConfirmadoOuOrganizador", ctx, evtID, userID).Return(true, nil)
			},
			setupRepo: func(r *mockSocialRepo) {
				r.On("FindOrCreate", ctx, evtID).Return(emptyDoc(), nil)
				r.On("AddAlbumLink", ctx, evtID, mock.MatchedBy(func(l domain.AlbumLink) bool {
					return l.Provider == domain.AlbumICloud
				})).Return(nil)
			},
			wantProv: domain.AlbumICloud,
		},
		{
			name: "max album links reached — returns 400",
			in:   portin.AddAlbumLinkInput{EventoID: evtID, UsuarioID: userID, URL: "https://x.com", Nome: "X"},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(nil)
				a.On("IsParticipanteConfirmadoOuOrganizador", ctx, evtID, userID).Return(true, nil)
			},
			setupRepo: func(r *mockSocialRepo) {
				r.On("FindOrCreate", ctx, evtID).Return(docWithAlbums(domain.MaxAlbumLinks), nil)
			},
			wantErr: "MAX_ALBUM_LINKS_REACHED",
		},
		{
			name: "not participant returns 403",
			in:   portin.AddAlbumLinkInput{EventoID: evtID, UsuarioID: userID, URL: "https://x.com", Nome: "X"},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(nil)
				a.On("IsParticipanteConfirmadoOuOrganizador", ctx, evtID, userID).Return(false, nil)
			},
			setupRepo: func(r *mockSocialRepo) {},
			wantErr:   "FORBIDDEN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockSocialRepo{}
			auth := &mockAuthPort{}
			tt.setupRepo(repo)
			tt.setupAuth(auth)

			uc := ucsocial.NewAddAlbumLink(repo, auth)
			link, err := uc.Execute(ctx, tt.in)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, link)
			} else {
				require.NoError(t, err)
				require.NotNil(t, link)
				assert.NotEmpty(t, link.ID)
				assert.Equal(t, tt.wantProv, link.Provider)
				assert.Equal(t, userID, link.AdicionadoPor) // must come from JWT
			}
			repo.AssertExpectations(t)
			auth.AssertExpectations(t)
		})
	}
}

// ============================================================
// RemoveAlbumLink — ORGANIZADOR ONLY (assimetria com add)
// ============================================================

func TestRemoveAlbumLink_Execute(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		in        portin.RemoveAlbumLinkInput
		setupRepo func(*mockSocialRepo)
		setupAuth func(*mockAuthPort)
		wantErr   string
	}{
		{
			name: "success — organizador removes",
			in:   portin.RemoveAlbumLinkInput{EventoID: evtID, UsuarioID: userID, LinkID: "link-1"},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(nil)
				a.On("IsOrganizador", ctx, evtID, userID).Return(true, nil)
			},
			setupRepo: func(r *mockSocialRepo) {
				r.On("RemoveAlbumLink", ctx, evtID, "link-1").Return(nil)
			},
		},
		{
			name: "participant cannot remove — returns 403 (assimetria)",
			in:   portin.RemoveAlbumLinkInput{EventoID: evtID, UsuarioID: userID, LinkID: "link-1"},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(nil)
				a.On("IsOrganizador", ctx, evtID, userID).Return(false, nil) // not organizer
			},
			setupRepo: func(r *mockSocialRepo) {},
			wantErr:   "FORBIDDEN",
		},
		{
			name: "fase check fails",
			in:   portin.RemoveAlbumLinkInput{EventoID: evtID, UsuarioID: userID, LinkID: "link-1"},
			setupAuth: func(a *mockAuthPort) {
				a.On("FaseAtLeast", ctx, evtID, domain.FaseAguardandoAceite).Return(errFase)
			},
			setupRepo: func(r *mockSocialRepo) {},
			wantErr:   "FASE_INSUFICIENTE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockSocialRepo{}
			auth := &mockAuthPort{}
			tt.setupRepo(repo)
			tt.setupAuth(auth)

			uc := ucsocial.NewRemoveAlbumLink(repo, auth)
			err := uc.Execute(ctx, tt.in)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			repo.AssertExpectations(t)
			auth.AssertExpectations(t)
		})
	}
}
