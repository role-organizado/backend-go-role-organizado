// Package convite implements the Convites (invitations) domain use cases.
// Mirrors Java's BuscarConviteUseCase, EnviarConviteUseCase, ConfirmarConviteUseCase,
// RecusarConviteUseCase, DesistirEventoUseCase, ReabrirInviteApprovalUseCase
// and ReenviarConvitesMassaAdminUseCase.
package convite

import (
	"context"
	"errors"
	"time"

	convitedomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/convite"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// BuscarConvite implements portin.BuscarConviteUseCase.
//
// Flow:
//  1. FindByID on participants — if found, build response from that.
//  2. Else fallback to approval_items by id (LAZY_ON_APPROVAL strategy).
//  3. Else 404.
type BuscarConvite struct {
	participants portout.ConviteParticipantRepository
	approvals    portout.ConviteApprovalRepository
	eventos      portout.EventoRepository
	usuarios     portout.UsuarioRepository
	guests       portout.ConviteGuestRepository
}

// NewBuscarConvite wires the use case dependencies.
func NewBuscarConvite(
	participants portout.ConviteParticipantRepository,
	approvals portout.ConviteApprovalRepository,
	eventos portout.EventoRepository,
	usuarios portout.UsuarioRepository,
	guests portout.ConviteGuestRepository,
) *BuscarConvite {
	return &BuscarConvite{
		participants: participants,
		approvals:    approvals,
		eventos:      eventos,
		usuarios:     usuarios,
		guests:       guests,
	}
}

// Execute returns the convite details for the given participantId.
func (uc *BuscarConvite) Execute(ctx context.Context, participantID string) (*portin.ConviteResponse, error) {
	if participantID == "" {
		return nil, apierr.BadRequest("participantId é obrigatório")
	}

	p, err := uc.participants.FindByID(ctx, participantID)
	if err == nil && p != nil {
		return uc.buildFromParticipant(ctx, p)
	}
	if err != nil && !apierr.IsNotFound(err) {
		// non-not-found errors bubble up
		return nil, err
	}

	// LAZY_ON_APPROVAL fallback: try approval_items by id.
	if uc.approvals != nil {
		appr, aerr := uc.approvals.FindByID(ctx, participantID)
		if aerr == nil && appr != nil {
			if appr.Type != convitedomain.ApprovalTypeInvite ||
				appr.MaterializationStrategy != convitedomain.MaterializationLazyOnApproval {
				return nil, apierr.NotFoundMsg("convite não encontrado")
			}
			return uc.buildFromApproval(ctx, appr)
		}
		if aerr != nil && !apierr.IsNotFound(aerr) {
			return nil, aerr
		}
	}

	return nil, apierr.NotFoundMsg("convite não encontrado")
}

func (uc *BuscarConvite) buildFromParticipant(ctx context.Context, p *convitedomain.Participant) (*portin.ConviteResponse, error) {
	resp := &portin.ConviteResponse{
		ParticipantID:     p.ID,
		Status:            string(p.Status),
		EventoID:          p.EventoID,
		ConvidadoNome:     p.Nome,
		ConvidadoEmail:    p.Email,
		ConvidadoTelefone: p.Telefone,
		TipoParticipante:  string(p.TipoParticipante),
		DataResposta:      p.DataResposta,
	}
	uc.enrichEvento(ctx, p.EventoID, resp)
	return resp, nil
}

func (uc *BuscarConvite) buildFromApproval(ctx context.Context, a *convitedomain.ApprovalItem) (*portin.ConviteResponse, error) {
	resp := &portin.ConviteResponse{
		ParticipantID:    a.ID,
		Status:           mapApprovalToConviteStatus(a.Status),
		EventoID:         a.EventID,
		TipoParticipante: string(convitedomain.TipoGuest),
	}
	if a.Metadata != nil {
		if v, ok := a.Metadata["nome"].(string); ok {
			resp.ConvidadoNome = v
		}
		if v, ok := a.Metadata["email"].(string); ok {
			resp.ConvidadoEmail = v
		}
		if v, ok := a.Metadata["telefone"].(string); ok {
			resp.ConvidadoTelefone = v
		}
	}
	uc.enrichEvento(ctx, a.EventID, resp)
	return resp, nil
}

func (uc *BuscarConvite) enrichEvento(ctx context.Context, eventoID string, resp *portin.ConviteResponse) {
	if eventoID == "" || uc.eventos == nil {
		return
	}
	evt, err := uc.eventos.FindByID(ctx, eventoID)
	if err != nil || evt == nil {
		return
	}
	resp.EventoNome = evt.Nome
	resp.EventoLocal = evt.Local
	resp.EventoDescricao = evt.Descricao
	if !evt.Data.IsZero() {
		d := evt.Data
		resp.EventoData = &d
		resp.EventoPassado = time.Now().UTC().After(evt.Data)
	}
	if uc.usuarios != nil && evt.UsuarioID != "" {
		if u, uerr := uc.usuarios.FindByID(ctx, evt.UsuarioID); uerr == nil && u != nil {
			resp.OrganizadorNome = u.Nome
		}
	}
}

// mapApprovalToConviteStatus converts ApprovalItem.Status to the convite status
// expected by the API.
func mapApprovalToConviteStatus(s convitedomain.ApprovalItemStatus) string {
	switch s {
	case convitedomain.ApprovalStatusApproved:
		return string(convitedomain.StatusConfirmado)
	case convitedomain.ApprovalStatusRejected:
		return string(convitedomain.StatusRecusado)
	case convitedomain.ApprovalStatusExpired, convitedomain.ApprovalStatusCancelled:
		return string(convitedomain.StatusCancelado)
	default:
		return string(convitedomain.StatusPendente)
	}
}

// Guards-rails for stub repos so callers can pass nil safely in tests.
var _ = errors.New
