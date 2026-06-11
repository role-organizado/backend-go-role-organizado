package convite

import (
	"context"
	"time"

	convitedomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/convite"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// RecusarConvite implements portin.RecusarConviteUseCase.
//
// Idempotency, GUEST-only guard, approval delegation, Temporal ACTION_REMOVE signal.
type RecusarConvite struct {
	participants portout.ConviteParticipantRepository
	approvals    portout.ConviteApprovalRepository
	buscar       portin.BuscarConviteUseCase
	temporal     portout.TemporalWorkflowStarter
}

// NewRecusarConvite wires the use case.
func NewRecusarConvite(
	participants portout.ConviteParticipantRepository,
	approvals portout.ConviteApprovalRepository,
	buscar portin.BuscarConviteUseCase,
	temporal portout.TemporalWorkflowStarter,
) *RecusarConvite {
	return &RecusarConvite{
		participants: participants,
		approvals:    approvals,
		buscar:       buscar,
		temporal:     temporal,
	}
}

// Execute records the refusal (idempotent, with approval delegation).
func (uc *RecusarConvite) Execute(ctx context.Context, participantID string) (*portin.ConviteResponse, error) {
	if participantID == "" {
		return nil, apierr.BadRequest("participantId é obrigatório")
	}

	p, err := uc.participants.FindByID(ctx, participantID)
	if err == nil && p != nil {
		return uc.handleParticipant(ctx, p)
	}
	if err != nil && !apierr.IsNotFound(err) {
		return nil, err
	}

	if uc.approvals != nil {
		appr, _ := uc.approvals.FindByID(ctx, participantID)
		if appr != nil {
			if appr.Type != convitedomain.ApprovalTypeInvite ||
				appr.MaterializationStrategy != convitedomain.MaterializationLazyOnApproval {
				return nil, apierr.NotFoundMsg("convite não encontrado")
			}
			return uc.rejectApproval(ctx, appr, "")
		}
	}

	return nil, apierr.NotFoundMsg("convite não encontrado")
}

func (uc *RecusarConvite) handleParticipant(ctx context.Context, p *convitedomain.Participant) (*portin.ConviteResponse, error) {
	if p.Status == convitedomain.StatusRecusado {
		return uc.lookup(ctx, p.ID)
	}
	if p.Status == convitedomain.StatusCancelado {
		return nil, apierr.Unprocessable("convite cancelado não pode ser recusado")
	}
	if p.TipoParticipante == convitedomain.TipoUser {
		return nil, apierr.Forbidden("PARTICIPANT_MUST_USE_APPROVAL_CENTER")
	}

	if uc.approvals != nil {
		appr, _ := uc.approvals.FindPendingByTargetEntityID(ctx, p.ID)
		if appr != nil {
			return uc.rejectApproval(ctx, appr, p.UsuarioID)
		}
	}

	now := time.Now().UTC()
	if err := uc.participants.UpdateStatus(ctx, p.ID, convitedomain.StatusRecusado, &now); err != nil {
		return nil, err
	}
	uc.startTemporal(ctx, p.ID, "ACTION_REMOVE")
	return uc.lookup(ctx, p.ID)
}

func (uc *RecusarConvite) rejectApproval(ctx context.Context, appr *convitedomain.ApprovalItem, resolvedBy string) (*portin.ConviteResponse, error) {
	if appr.Status == convitedomain.ApprovalStatusRejected {
		return uc.lookup(ctx, appr.ID)
	}
	if appr.Status != convitedomain.ApprovalStatusPending {
		return nil, apierr.Unprocessable("approval não está pendente")
	}
	now := time.Now().UTC()
	if err := uc.approvals.UpdateStatus(ctx, appr.ID, convitedomain.ApprovalStatusRejected, resolvedBy, "", now); err != nil {
		return nil, err
	}
	uc.startTemporal(ctx, appr.ID, "ACTION_REMOVE")
	return uc.lookup(ctx, appr.ID)
}

func (uc *RecusarConvite) startTemporal(ctx context.Context, id, action string) {
	if uc.temporal == nil {
		return
	}
	_ = uc.temporal.StartWorkflow(ctx, portout.WorkflowStartOptions{
		WorkflowID: "participant-lifecycle-" + id + "-" + action,
		TaskQueue:  "participant-lifecycle",
	}, "ParticipantLifecycleWorkflow", id, action)
}

func (uc *RecusarConvite) lookup(ctx context.Context, id string) (*portin.ConviteResponse, error) {
	if uc.buscar != nil {
		return uc.buscar.Execute(ctx, id)
	}
	return &portin.ConviteResponse{ParticipantID: id, Status: string(convitedomain.StatusRecusado)}, nil
}
