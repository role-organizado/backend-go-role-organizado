package convite

import (
	"context"
	"time"

	"github.com/google/uuid"

	convitedomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/convite"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ReabrirInviteApproval implements portin.ReabrirInviteApprovalUseCase.
//
// Rules (mirror Java):
//   - Only ORGANIZADOR or CO_ORGANIZADOR roles may reopen.
//   - Only USER-type participants.
//   - Previous invite must exist and must NOT be APPROVED or PENDING.
//   - Creates a new ApprovalItem INVITE in PENDING, copying metadata from the previous one.
type ReabrirInviteApproval struct {
	participants portout.ConviteParticipantRepository
	approvals    portout.ConviteApprovalRepository
}

// NewReabrirInviteApproval wires the use case.
func NewReabrirInviteApproval(
	participants portout.ConviteParticipantRepository,
	approvals portout.ConviteApprovalRepository,
) *ReabrirInviteApproval {
	return &ReabrirInviteApproval{participants: participants, approvals: approvals}
}

// Execute reopens an INVITE approval for the target participant.
func (uc *ReabrirInviteApproval) Execute(ctx context.Context, participantID, requesterID string) (*portin.ApprovalItemResponse, error) {
	if participantID == "" {
		return nil, apierr.BadRequest("participantId é obrigatório")
	}
	if requesterID == "" {
		return nil, apierr.Unauthorized("autenticação necessária")
	}

	target, err := uc.participants.FindByID(ctx, participantID)
	if err != nil {
		return nil, err
	}
	if target == nil {
		return nil, apierr.NotFound("participant", participantID)
	}

	// Authorisation: requester must be ORGANIZADOR or CO_ORGANIZADOR of the event.
	requester, _ := uc.participants.FindByEventoIDAndUsuarioID(ctx, target.EventoID, requesterID)
	if requester == nil || (requester.Papel != convitedomain.PapelOrganizador && requester.Papel != convitedomain.PapelCoOrganizador) {
		return nil, apierr.Forbidden("apenas ORGANIZADOR ou CO_ORGANIZADOR podem reabrir convites")
	}

	if target.TipoParticipante != convitedomain.TipoUser {
		return nil, apierr.BadRequest("reabertura permitida apenas para participantes do tipo USER")
	}

	// No pending duplicate.
	if uc.approvals != nil {
		exists, eerr := uc.approvals.ExistsPendingByTargetEntityID(ctx, target.ID)
		if eerr != nil {
			return nil, eerr
		}
		if exists {
			return nil, apierr.Conflict("já existe convite PENDENTE para este participante")
		}
	}

	// Load previous to copy metadata.
	var prev *convitedomain.ApprovalItem
	if uc.approvals != nil {
		prev, _ = uc.approvals.FindLatestByTargetEntityIDAndType(ctx, target.ID, convitedomain.ApprovalTypeInvite)
	}
	if !convitedomain.CanReopenInvite(prev) {
		return nil, apierr.Conflict("convite anterior não pode ser reaberto neste estado")
	}

	now := time.Now().UTC()
	metadata := map[string]any{}
	if prev != nil && prev.Metadata != nil {
		for k, v := range prev.Metadata {
			metadata[k] = v
		}
	}

	newItem := &convitedomain.ApprovalItem{
		ID:                      uuid.New().String(),
		Type:                    convitedomain.ApprovalTypeInvite,
		Status:                  convitedomain.ApprovalStatusPending,
		ApproverID:              target.UsuarioID,
		EventID:                 target.EventoID,
		TargetEntityID:          target.ID,
		Metadata:                metadata,
		MaterializationStrategy: convitedomain.MaterializationLazyOnApproval,
		CreatedAt:               now,
		UpdatedAt:               now,
	}
	saved, serr := uc.approvals.Save(ctx, newItem)
	if serr != nil {
		return nil, serr
	}
	if saved == nil {
		saved = newItem
	}

	return &portin.ApprovalItemResponse{
		ID:             saved.ID,
		Type:           string(saved.Type),
		Status:         string(saved.Status),
		EventID:        saved.EventID,
		TargetEntityID: saved.TargetEntityID,
		ApproverID:     saved.ApproverID,
		Metadata:       saved.Metadata,
		CreatedAt:      saved.CreatedAt,
	}, nil
}
