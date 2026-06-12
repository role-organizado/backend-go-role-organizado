package guest

import (
	"context"
	"log/slog"
	"time"

	convitedomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/convite"
	guestdomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/guest"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// VincularGuest implements portin.VincularGuestAUsuarioUseCase.
//
// Go port of Java's VincularGuestAUsuarioUseCase (Feature 016 / 027). It links
// pre-existing guest records to a freshly-created user in one of two modes:
//
//   - Explicit  (ParticipantID set): only the referenced participant is migrated
//     GUEST→USER. Used when the user signs up through an invite link.
//   - Implicit  (no ParticipantID): guests are matched by phone OR email, then
//     their drafts and participations are rewritten to the new userId.
//
// The participant and draft ports are optional (nil → that step is skipped),
// following the codebase's nil-safe optional-dependency idiom.
type VincularGuest struct {
	guests       portout.GuestRepository
	participants portout.VinculacaoParticipantPort // optional
	drafts       portout.VinculacaoDraftPort       // optional
	now          func() time.Time
}

// NewVincularGuest wires the use case. participants and drafts may be nil.
func NewVincularGuest(
	guests portout.GuestRepository,
	participants portout.VinculacaoParticipantPort,
	drafts portout.VinculacaoDraftPort,
) *VincularGuest {
	return &VincularGuest{guests: guests, participants: participants, drafts: drafts, now: time.Now}
}

var _ portin.VincularGuestAUsuarioUseCase = (*VincularGuest)(nil)

// Execute dispatches to the explicit or implicit linking strategy.
func (uc *VincularGuest) Execute(ctx context.Context, in portin.VincularGuestInput) (*portin.VinculacaoResult, error) {
	if in.UsuarioID == "" {
		return nil, apierr.BadRequest("usuarioId é obrigatório")
	}

	if in.ParticipantID != "" {
		return uc.executarExplicita(ctx, in)
	}
	return uc.executarImplicita(ctx, in)
}

// ---- Explicit (invite-link) ----

func (uc *VincularGuest) executarExplicita(ctx context.Context, in portin.VincularGuestInput) (*portin.VinculacaoResult, error) {
	result := &portin.VinculacaoResult{}

	if uc.participants == nil {
		slog.WarnContext(ctx, "vincular guest: explicit mode requested but participant port is disabled",
			"usuarioId", in.UsuarioID, "participantId", in.ParticipantID)
		return result, nil
	}

	participant, err := uc.participants.FindByID(ctx, in.ParticipantID)
	if err != nil {
		// A missing participant is not fatal — mirror Java's empty-result path.
		slog.WarnContext(ctx, "vincular guest: participant not found for explicit linking",
			"participantId", in.ParticipantID, "error", err)
		return result, nil
	}

	// Security: never migrate a participant already bound to a real USER.
	if participant.TipoParticipante != convitedomain.TipoGuest {
		return nil, apierr.Unprocessable("participante já está vinculado a usuário existente (tipo: " + string(participant.TipoParticipante) + ")")
	}

	guestID := participant.UsuarioID // before migration, usuarioId == guestId

	// 1. Migrate the participant GUEST → USER.
	participant.TipoParticipante = convitedomain.TipoUser
	participant.UsuarioID = in.UsuarioID
	participant.AtualizadoEm = uc.now()
	if _, err := uc.participants.Save(ctx, participant); err != nil {
		return nil, err
	}
	result.ParticipantsAtualizados = 1
	result.EventosLinkados = 1
	result.GuestsEncontrados = 1

	// 2. Mark the guest evolved + rewrite drafts.
	if guestID != "" {
		g, ferr := uc.guests.FindByID(ctx, guestID)
		if ferr == nil && g != nil {
			if g.EvoluidoParaUsuarioID == "" {
				g.EvoluidoParaUsuarioID = in.UsuarioID
				if _, uerr := uc.guests.Update(ctx, g); uerr != nil {
					return nil, uerr
				}
			}
			result.DraftsAtualizados += uc.atualizarDrafts(ctx, []guestdomain.Guest{*g}, in.UsuarioID)
		} else {
			slog.WarnContext(ctx, "vincular guest: guest not found for explicit linking", "guestId", guestID)
		}
	}

	return result, nil
}

// ---- Implicit (phone / email) ----

func (uc *VincularGuest) executarImplicita(ctx context.Context, in portin.VincularGuestInput) (*portin.VinculacaoResult, error) {
	result := &portin.VinculacaoResult{}

	matched := uc.buscarGuestsCorrespondentes(ctx, in)
	if len(matched) == 0 {
		return result, nil
	}
	result.GuestsEncontrados = len(matched)

	// 1. Rewrite drafts guestId → userId.
	result.DraftsAtualizados = uc.atualizarDrafts(ctx, matched, in.UsuarioID)

	// 2. Migrate participations GUEST → USER.
	result.ParticipantsAtualizados = uc.atualizarParticipants(ctx, matched, in.UsuarioID)

	// 3. Mark guests evolved (idempotent).
	for i := range matched {
		g := matched[i]
		if g.EvoluidoParaUsuarioID != "" {
			continue
		}
		g.EvoluidoParaUsuarioID = in.UsuarioID
		if _, err := uc.guests.Update(ctx, &g); err != nil {
			return nil, err
		}
	}

	return result, nil
}

// buscarGuestsCorrespondentes returns the deduplicated set of guests whose phone
// or email matches the user.
func (uc *VincularGuest) buscarGuestsCorrespondentes(ctx context.Context, in portin.VincularGuestInput) []guestdomain.Guest {
	seen := map[string]bool{}
	var out []guestdomain.Guest

	add := func(g *guestdomain.Guest) {
		if g == nil || seen[g.ID] {
			return
		}
		seen[g.ID] = true
		out = append(out, *g)
	}

	if in.Telefone != "" {
		if g, err := uc.guests.FindByTelefone(ctx, in.Telefone); err == nil {
			add(g)
		}
	}
	if in.Email != "" {
		if g, err := uc.guests.FindByEmail(ctx, in.Email); err == nil {
			add(g)
		}
	}
	return out
}

// atualizarParticipants migrates every GUEST participation carrying a matched
// guestId as usuarioId over to the new userId, returning the count updated.
func (uc *VincularGuest) atualizarParticipants(ctx context.Context, guests []guestdomain.Guest, novoUsuarioID string) int {
	if uc.participants == nil {
		return 0
	}
	updated := 0
	for _, g := range guests {
		parts, err := uc.participants.FindByTipoParticipanteAndUsuarioID(ctx, convitedomain.TipoGuest, g.ID)
		if err != nil {
			slog.WarnContext(ctx, "vincular guest: failed to load participants for guest", "guestId", g.ID, "error", err)
			continue
		}
		for i := range parts {
			p := parts[i]
			p.TipoParticipante = convitedomain.TipoUser
			p.UsuarioID = novoUsuarioID
			p.AtualizadoEm = uc.now()
			if _, err := uc.participants.Save(ctx, &p); err != nil {
				slog.WarnContext(ctx, "vincular guest: failed to migrate participant", "participantId", p.ID, "guestId", g.ID, "error", err)
				continue
			}
			updated++
		}
	}
	return updated
}

// atualizarDrafts rewrites guestId references to userId across drafts, returning
// the count of drafts updated.
func (uc *VincularGuest) atualizarDrafts(ctx context.Context, guests []guestdomain.Guest, usuarioID string) int {
	if uc.drafts == nil {
		return 0
	}
	updated := 0
	for _, g := range guests {
		ids, err := uc.drafts.FindDraftIDsByConvidadosGuestID(ctx, g.ID)
		if err != nil {
			slog.WarnContext(ctx, "vincular guest: failed to load drafts for guest", "guestId", g.ID, "error", err)
			continue
		}
		for _, draftID := range ids {
			if err := uc.drafts.ConvertGuestToUserInConvidados(ctx, draftID, g.ID, usuarioID); err != nil {
				slog.WarnContext(ctx, "vincular guest: failed to convert draft", "draftId", draftID, "guestId", g.ID, "error", err)
				continue
			}
			updated++
		}
	}
	return updated
}
