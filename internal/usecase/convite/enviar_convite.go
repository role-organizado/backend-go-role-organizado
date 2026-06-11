package convite

import (
	"context"
	"strings"
	"time"

	convitedomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/convite"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// EnviarConvite implements portin.EnviarConviteUseCase.
//
// Validates ownership (organizador), at-least-one-contact-channel, normalises
// the phone to E.164, picks the delivery channel, and enqueues the notification
// payload. Returns 202-style EnviarConviteResponse with the message id.
type EnviarConvite struct {
	participants portout.ConviteParticipantRepository
	approvals    portout.ConviteApprovalRepository
	eventos      portout.EventoRepository
	usuarios     portout.UsuarioRepository
	notif        portout.ConviteNotificationPort
}

// NewEnviarConvite wires the use case.
func NewEnviarConvite(
	participants portout.ConviteParticipantRepository,
	approvals portout.ConviteApprovalRepository,
	eventos portout.EventoRepository,
	usuarios portout.UsuarioRepository,
	notif portout.ConviteNotificationPort,
) *EnviarConvite {
	return &EnviarConvite{
		participants: participants,
		approvals:    approvals,
		eventos:      eventos,
		usuarios:     usuarios,
		notif:        notif,
	}
}

// Execute resolves the convite (Participant first, ApprovalItem fallback), checks
// organizador authorisation, normalises phone and dispatches via the notification port.
func (uc *EnviarConvite) Execute(ctx context.Context, in portin.EnviarConviteInput) (*portin.EnviarConviteResponse, error) {
	if in.ParticipantID == "" {
		return nil, apierr.BadRequest("participantId é obrigatório")
	}
	if in.OrganizadorID == "" {
		return nil, apierr.Unauthorized("autenticação necessária")
	}

	// Resolve target participant or lazy approval item.
	target, errResolve := uc.resolveTarget(ctx, in.ParticipantID)
	if errResolve != nil {
		return nil, errResolve
	}

	// Load evento and check organizador authorization.
	evt, eerr := uc.eventos.FindByID(ctx, target.eventoID)
	if eerr != nil {
		return nil, eerr
	}
	if evt == nil {
		return nil, apierr.NotFound("evento", target.eventoID)
	}
	if !evt.IsOwner(in.OrganizadorID) {
		return nil, apierr.Forbidden("apenas o organizador pode enviar convites para este evento")
	}

	// At least one contact channel.
	if strings.TrimSpace(target.telefone) == "" && strings.TrimSpace(target.email) == "" {
		return nil, apierr.Unprocessable("participante sem canal de contato (telefone ou email)")
	}

	telefoneNormalized := convitedomain.NormalizePhoneE164(target.telefone)
	canal := convitedomain.SelectCanal(telefoneNormalized, target.email)

	// Organizer name for the notification payload (best effort).
	organizadorNome := ""
	if uc.usuarios != nil && evt.UsuarioID != "" {
		if u, _ := uc.usuarios.FindByID(ctx, evt.UsuarioID); u != nil {
			organizadorNome = u.Nome
		}
	}

	pubIn := portout.ConvitePublishInput{
		ParticipantID:   in.ParticipantID,
		Canal:           string(canal),
		Telefone:        telefoneNormalized,
		Email:           target.email,
		Nome:            target.nome,
		EventoID:        evt.ID,
		EventoNome:      evt.Nome,
		EventoLocal:     evt.Local,
		OrganizadorNome: organizadorNome,
	}
	if !evt.Data.IsZero() {
		d := evt.Data
		pubIn.EventoData = &d
	}

	messageID := ""
	if uc.notif != nil {
		mid, perr := uc.notif.PublicarConvite(ctx, pubIn)
		if perr != nil {
			return nil, apierr.ServiceUnavailable("falha ao enfileirar convite: " + perr.Error())
		}
		messageID = mid
	}

	return &portin.EnviarConviteResponse{
		ParticipantID: in.ParticipantID,
		Aceito:        true,
		MessageID:     messageID,
		Canal:         string(canal),
		Mensagem:      "convite enfileirado para envio",
	}, nil
}

// resolveTargetData is the small projection the use case uses internally.
type resolvedTarget struct {
	eventoID string
	nome     string
	email    string
	telefone string
}

func (uc *EnviarConvite) resolveTarget(ctx context.Context, id string) (*resolvedTarget, error) {
	p, err := uc.participants.FindByID(ctx, id)
	if err == nil && p != nil {
		return &resolvedTarget{
			eventoID: p.EventoID,
			nome:     p.Nome,
			email:    p.Email,
			telefone: p.Telefone,
		}, nil
	}
	if err != nil && !apierr.IsNotFound(err) {
		return nil, err
	}
	if uc.approvals != nil {
		appr, aerr := uc.approvals.FindByID(ctx, id)
		if aerr == nil && appr != nil {
			if appr.Type != convitedomain.ApprovalTypeInvite ||
				appr.MaterializationStrategy != convitedomain.MaterializationLazyOnApproval {
				return nil, apierr.NotFoundMsg("convite não encontrado")
			}
			out := &resolvedTarget{eventoID: appr.EventID}
			if appr.Metadata != nil {
				if v, ok := appr.Metadata["nome"].(string); ok {
					out.nome = v
				}
				if v, ok := appr.Metadata["email"].(string); ok {
					out.email = v
				}
				if v, ok := appr.Metadata["telefone"].(string); ok {
					out.telefone = v
				}
			}
			return out, nil
		}
		if aerr != nil && !apierr.IsNotFound(aerr) {
			return nil, aerr
		}
	}
	return nil, apierr.NotFoundMsg("convite não encontrado")
}

// Time helper kept here so test files don't need to import time directly.
var _ = time.Time{}
