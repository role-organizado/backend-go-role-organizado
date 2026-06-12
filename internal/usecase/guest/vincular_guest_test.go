package guest_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	convitedomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/convite"
	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/guest"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	ucguest "github.com/role-organizado/backend-go-role-organizado/internal/usecase/guest"
)

// ---- fakes (embed the interface so only used methods are implemented) ----

type fakeVincGuestRepo struct {
	portout.GuestRepository
	byTel   map[string]*domain.Guest
	byEmail map[string]*domain.Guest
	byID    map[string]*domain.Guest
	updated []domain.Guest
}

func newFakeVincGuestRepo() *fakeVincGuestRepo {
	return &fakeVincGuestRepo{
		byTel:   map[string]*domain.Guest{},
		byEmail: map[string]*domain.Guest{},
		byID:    map[string]*domain.Guest{},
	}
}

func (f *fakeVincGuestRepo) add(g *domain.Guest) {
	f.byID[g.ID] = g
	if g.Telefone != "" {
		f.byTel[g.Telefone] = g
	}
	if g.Email != "" {
		f.byEmail[g.Email] = g
	}
}

func (f *fakeVincGuestRepo) FindByTelefone(_ context.Context, tel string) (*domain.Guest, error) {
	if g, ok := f.byTel[tel]; ok {
		return g, nil
	}
	return nil, errors.New("not found")
}

func (f *fakeVincGuestRepo) FindByEmail(_ context.Context, email string) (*domain.Guest, error) {
	if g, ok := f.byEmail[email]; ok {
		return g, nil
	}
	return nil, errors.New("not found")
}

func (f *fakeVincGuestRepo) FindByID(_ context.Context, id string) (*domain.Guest, error) {
	if g, ok := f.byID[id]; ok {
		return g, nil
	}
	return nil, errors.New("not found")
}

func (f *fakeVincGuestRepo) Update(_ context.Context, g *domain.Guest) (*domain.Guest, error) {
	f.updated = append(f.updated, *g)
	f.add(g) // refresh all indexes so subsequent lookups see the mutation
	return g, nil
}

type fakeVincParticipantPort struct {
	byID    map[string]*convitedomain.Participant
	byGuest map[string][]convitedomain.Participant
	saved   []convitedomain.Participant
}

func newFakeVincParticipantPort() *fakeVincParticipantPort {
	return &fakeVincParticipantPort{
		byID:    map[string]*convitedomain.Participant{},
		byGuest: map[string][]convitedomain.Participant{},
	}
}

func (f *fakeVincParticipantPort) FindByID(_ context.Context, id string) (*convitedomain.Participant, error) {
	if p, ok := f.byID[id]; ok {
		return p, nil
	}
	return nil, errors.New("not found")
}

func (f *fakeVincParticipantPort) FindByTipoParticipanteAndUsuarioID(_ context.Context, tipo convitedomain.TipoParticipante, usuarioID string) ([]convitedomain.Participant, error) {
	var out []convitedomain.Participant
	for _, p := range f.byGuest[usuarioID] {
		if p.TipoParticipante == tipo {
			out = append(out, p)
		}
	}
	return out, nil
}

func (f *fakeVincParticipantPort) Save(_ context.Context, p *convitedomain.Participant) (*convitedomain.Participant, error) {
	f.saved = append(f.saved, *p)
	f.byID[p.ID] = p
	return p, nil
}

type fakeVincDraftPort struct {
	byGuest    map[string][]string
	converted  int
	failLookup bool
}

func (f *fakeVincDraftPort) FindDraftIDsByConvidadosGuestID(_ context.Context, guestID string) ([]string, error) {
	if f.failLookup {
		return nil, errors.New("boom")
	}
	return f.byGuest[guestID], nil
}

func (f *fakeVincDraftPort) ConvertGuestToUserInConvidados(_ context.Context, _, _, _ string) error {
	f.converted++
	return nil
}

// ---- Execute validation ----

func TestVincularGuest_RequiresUsuarioID(t *testing.T) {
	uc := ucguest.NewVincularGuest(newFakeVincGuestRepo(), nil, nil)
	_, err := uc.Execute(context.Background(), portin.VincularGuestInput{})
	require.Error(t, err)
}

// ---- Implicit mode ----

func TestVincularGuest_Implicito(t *testing.T) {
	tests := []struct {
		name                 string
		seedGuests           []domain.Guest
		seedParticipants     map[string][]convitedomain.Participant
		seedDrafts           map[string][]string
		withParticipantPort  bool
		withDraftPort        bool
		in                   portin.VincularGuestInput
		wantGuests           int
		wantParticipants     int
		wantDrafts           int
		wantGuestUpdates     int
	}{
		{
			name:       "no matching guest yields empty result",
			in:         portin.VincularGuestInput{UsuarioID: "u1", Email: "ghost@x.com"},
			wantGuests: 0,
		},
		{
			name:       "matches by email and marks evolved",
			seedGuests: []domain.Guest{{ID: "g1", Email: "a@b.com"}},
			in:         portin.VincularGuestInput{UsuarioID: "u1", Email: "a@b.com"},
			wantGuests: 1, wantGuestUpdates: 1,
		},
		{
			name:       "matches by phone and email deduplicates same guest",
			seedGuests: []domain.Guest{{ID: "g1", Telefone: "+5511999999999", Email: "a@b.com"}},
			in:         portin.VincularGuestInput{UsuarioID: "u1", Telefone: "+5511999999999", Email: "a@b.com"},
			wantGuests: 1, wantGuestUpdates: 1,
		},
		{
			name:       "already-evolved guest is not updated again",
			seedGuests: []domain.Guest{{ID: "g1", Email: "a@b.com", EvoluidoParaUsuarioID: "old"}},
			in:         portin.VincularGuestInput{UsuarioID: "u1", Email: "a@b.com"},
			wantGuests: 1, wantGuestUpdates: 0,
		},
		{
			name:                "migrates GUEST participations when port present",
			seedGuests:          []domain.Guest{{ID: "g1", Email: "a@b.com"}},
			seedParticipants:    map[string][]convitedomain.Participant{"g1": {{ID: "p1", TipoParticipante: convitedomain.TipoGuest, UsuarioID: "g1"}}},
			withParticipantPort: true,
			in:                  portin.VincularGuestInput{UsuarioID: "u1", Email: "a@b.com"},
			wantGuests:          1, wantParticipants: 1, wantGuestUpdates: 1,
		},
		{
			name:          "rewrites drafts when draft port present",
			seedGuests:    []domain.Guest{{ID: "g1", Email: "a@b.com"}},
			seedDrafts:    map[string][]string{"g1": {"d1", "d2"}},
			withDraftPort: true,
			in:            portin.VincularGuestInput{UsuarioID: "u1", Email: "a@b.com"},
			wantGuests:    1, wantDrafts: 2, wantGuestUpdates: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeVincGuestRepo()
			for i := range tc.seedGuests {
				g := tc.seedGuests[i]
				repo.add(&g)
			}

			var partPort portout.VinculacaoParticipantPort
			if tc.withParticipantPort {
				fp := newFakeVincParticipantPort()
				fp.byGuest = tc.seedParticipants
				partPort = fp
			}
			var draftPort portout.VinculacaoDraftPort
			if tc.withDraftPort {
				draftPort = &fakeVincDraftPort{byGuest: tc.seedDrafts}
			}

			uc := ucguest.NewVincularGuest(repo, partPort, draftPort)
			res, err := uc.Execute(context.Background(), tc.in)
			require.NoError(t, err)
			require.NotNil(t, res)

			assert.Equal(t, tc.wantGuests, res.GuestsEncontrados, "guests")
			assert.Equal(t, tc.wantParticipants, res.ParticipantsAtualizados, "participants")
			assert.Equal(t, tc.wantDrafts, res.DraftsAtualizados, "drafts")
			assert.Len(t, repo.updated, tc.wantGuestUpdates, "guest updates")
		})
	}
}

func TestVincularGuest_Implicito_Idempotente(t *testing.T) {
	repo := newFakeVincGuestRepo()
	repo.add(&domain.Guest{ID: "g1", Email: "a@b.com"})
	uc := ucguest.NewVincularGuest(repo, nil, nil)

	_, err := uc.Execute(context.Background(), portin.VincularGuestInput{UsuarioID: "u1", Email: "a@b.com"})
	require.NoError(t, err)
	// Second call: guest now evolved, so no further update.
	_, err = uc.Execute(context.Background(), portin.VincularGuestInput{UsuarioID: "u1", Email: "a@b.com"})
	require.NoError(t, err)
	assert.Len(t, repo.updated, 1, "guest should only be updated once across repeated calls")
}

// ---- Explicit mode ----

func TestVincularGuest_Explicito(t *testing.T) {
	t.Run("migrates the referenced GUEST participant and evolves its guest", func(t *testing.T) {
		repo := newFakeVincGuestRepo()
		repo.add(&domain.Guest{ID: "g1", Email: "a@b.com"})

		fp := newFakeVincParticipantPort()
		fp.byID["p1"] = &convitedomain.Participant{ID: "p1", EventoID: "e1", TipoParticipante: convitedomain.TipoGuest, UsuarioID: "g1"}

		uc := ucguest.NewVincularGuest(repo, fp, nil)
		res, err := uc.Execute(context.Background(), portin.VincularGuestInput{UsuarioID: "u1", ParticipantID: "p1"})
		require.NoError(t, err)

		assert.Equal(t, 1, res.ParticipantsAtualizados)
		assert.Equal(t, 1, res.EventosLinkados)
		assert.Equal(t, 1, res.GuestsEncontrados)
		require.Len(t, fp.saved, 1)
		assert.Equal(t, convitedomain.TipoUser, fp.saved[0].TipoParticipante)
		assert.Equal(t, "u1", fp.saved[0].UsuarioID)
		require.Len(t, repo.updated, 1)
		assert.Equal(t, "u1", repo.updated[0].EvoluidoParaUsuarioID)
	})

	t.Run("rejects participant already bound to a USER", func(t *testing.T) {
		fp := newFakeVincParticipantPort()
		fp.byID["p1"] = &convitedomain.Participant{ID: "p1", TipoParticipante: convitedomain.TipoUser, UsuarioID: "someone"}

		uc := ucguest.NewVincularGuest(newFakeVincGuestRepo(), fp, nil)
		_, err := uc.Execute(context.Background(), portin.VincularGuestInput{UsuarioID: "u1", ParticipantID: "p1"})
		require.Error(t, err)
	})

	t.Run("missing participant returns empty result (non-fatal)", func(t *testing.T) {
		fp := newFakeVincParticipantPort()
		uc := ucguest.NewVincularGuest(newFakeVincGuestRepo(), fp, nil)
		res, err := uc.Execute(context.Background(), portin.VincularGuestInput{UsuarioID: "u1", ParticipantID: "ghost"})
		require.NoError(t, err)
		assert.Equal(t, 0, res.ParticipantsAtualizados)
	})

	t.Run("nil participant port disables explicit linking gracefully", func(t *testing.T) {
		uc := ucguest.NewVincularGuest(newFakeVincGuestRepo(), nil, nil)
		res, err := uc.Execute(context.Background(), portin.VincularGuestInput{UsuarioID: "u1", ParticipantID: "p1"})
		require.NoError(t, err)
		assert.Equal(t, 0, res.GuestsEncontrados)
	})
}
