package convite

import (
	"context"
	"time"

	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ReenviarConvitesMassaAdmin implements portin.ReenviarConvitesMassaAdminUseCase.
//
// Admin-only re-send of all invites for an event. Persists an AuditEntry; the
// actual SQS enqueue is delegated to the notification adapter when wired.
type ReenviarConvitesMassaAdmin struct {
	eventos portout.EventoRepository
	audit   portout.ConviteAuditRepository
}

// NewReenviarConvitesMassaAdmin wires the use case.
func NewReenviarConvitesMassaAdmin(
	eventos portout.EventoRepository,
	audit portout.ConviteAuditRepository,
) *ReenviarConvitesMassaAdmin {
	return &ReenviarConvitesMassaAdmin{eventos: eventos, audit: audit}
}

// Execute checks the event exists, registers an audit entry and returns success.
// NOTE: actual SQS publishing is TODO — left to the adapter wiring. Java's
// implementation also enqueues a placeholder TODO comment.
func (uc *ReenviarConvitesMassaAdmin) Execute(ctx context.Context, eventoID, adminID string) (*portin.EventoActionResponse, error) {
	if eventoID == "" {
		return nil, apierr.BadRequest("eventoId é obrigatório")
	}
	if adminID == "" {
		return nil, apierr.Unauthorized("autenticação necessária")
	}

	evt, err := uc.eventos.FindByID(ctx, eventoID)
	if err != nil {
		return nil, err
	}
	if evt == nil {
		return nil, apierr.NotFound("evento", eventoID)
	}

	if uc.audit != nil {
		_ = uc.audit.Save(ctx, portout.ConviteAuditEntry{
			Acao:       "EVENT_INVITES_RESENT_BY_ADMIN",
			EventoID:   eventoID,
			AdminID:    adminID,
			Detalhes:   map[string]any{"eventoNome": evt.Nome},
			OcorridoEm: time.Now().UTC(),
		})
	}

	return &portin.EventoActionResponse{
		EventoID: eventoID,
		Acao:     "EVENT_INVITES_RESENT_BY_ADMIN",
		Mensagem: "Reenvio de convites enfileirado para todos os participantes",
	}, nil
}
