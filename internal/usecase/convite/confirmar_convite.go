package convite

import (
	"context"
	"time"

	convitedomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/convite"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ConfirmarConvite implements portin.ConfirmarConviteUseCase.
//
// Business rules (mirror Java):
//   - idempotent if status is already CONFIRMADO.
//   - GUEST-type only: USER must use the Approval Center → 403.
//   - If a PENDING ApprovalItem INVITE exists → delegate via approval Update (approve).
//   - On CANCELADO/RECUSADO terminal → 422.
//   - Fires a Temporal ParticipantLifecycle workflow (ACTION_CONFIRM) post-commit.
type ConfirmarConvite struct {
	participants portout.ConviteParticipantRepository
	approvals    portout.ConviteApprovalRepository
	buscar       portin.BuscarConviteUseCase
	temporal     portout.TemporalWorkflowStarter
}

// NewConfirmarConvite wires the use case.
func NewConfirmarConvite(
	participants portout.ConviteParticipantRepository,
	approvals portout.ConviteApprovalRepository,
	buscar portin.BuscarConviteUseCase,
	temporal portout.TemporalWorkflowStarter,
) *ConfirmarConvite {
	return &ConfirmarConvite{
		participants: participants,
		approvals:    approvals,
		buscar:       buscar,
		temporal:     temporal,
	}
}

// Execute records the confirmation (idempotent, with approval delegation).
func (uc *ConfirmarConvite) Execute(ctx context.Context, participantID string) (*portin.ConviteResponse, error) {
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

	// LAZY_ON_APPROVAL fallback.
	if uc.approvals != nil {
		appr, aerr := uc.approvals.FindByID(ctx, participantID)
		if aerr == nil && appr != nil {
			if appr.Type != convitedomain.ApprovalTypeInvite ||
				appr.MaterializationStrategy != convitedomain.MaterializationLazyOnApproval {
				return nil, apierr.NotFoundMsg("convite não encontrado")
			}
			return uc.approveApproval(ctx, appr, "")
		}
	}

	return nil, apierr.NotFoundMsg("convite não encontrado")
}

func (uc *ConfirmarConvite) handleParticipant(ctx context.Context, p *convitedomain.Participant) (*portin.ConviteResponse, error) {
	// Idempotency: already confirmed → return current state.
	if p.Status == convitedomain.StatusConfirmado {
		return uc.lookup(ctx, p.ID)
	}
	if p.Status == convitedomain.StatusCancelado {
		return nil, apierr.Unprocessable("convite cancelado não pode ser confirmado")
	}
	// USER must use Approval Center.
	if p.TipoParticipante == convitedomain.TipoUser {
		return nil, apierr.Forbidden("PARTICIPANT_MUST_USE_APPROVAL_CENTER")
	}

	// If a pending INVITE approval exists, delegate to approval flow.
	if uc.approvals != nil {
		appr, _ := uc.approvals.FindPendingByTargetEntityID(ctx, p.ID)
		if appr != nil {
			return uc.approveApproval(ctx, appr, p.UsuarioID)
		}
	}

	now := time.Now().UTC()
	if err := uc.participants.UpdateStatus(ctx, p.ID, convitedomain.StatusConfirmado, &now); err != nil {
		return nil, err
	}
	uc.startTemporal(ctx, p.ID, "ACTION_CONFIRM")
	return uc.lookup(ctx, p.ID)
}

func (uc *ConfirmarConvite) approveApproval(ctx context.Context, appr *convitedomain.ApprovalItem, resolvedBy string) (*portin.ConviteResponse, error) {
	if appr.Status == convitedomain.ApprovalStatusApproved {
		return uc.lookup(ctx, appr.ID)
	}
	if appr.Status != convitedomain.ApprovalStatusPending {
		return nil, apierr.Unprocessable("approval não está pendente")
	}
	now := time.Now().UTC()
	if err := uc.approvals.UpdateStatus(ctx, appr.ID, convitedomain.ApprovalStatusApproved, resolvedBy, "", now); err != nil {
		return nil, err
	}
	uc.startTemporal(ctx, appr.ID, "ACTION_CONFIRM")
	return uc.lookup(ctx, appr.ID)
}

func (uc *ConfirmarConvite) startTemporal(ctx context.Context, id, action string) {
	if uc.temporal == nil {
		return
	}
	_ = uc.temporal.StartWorkflow(ctx, portout.WorkflowStartOptions{
		WorkflowID: "participant-lifecycle-" + id + "-" + action,
		TaskQueue:  "participant-lifecycle",
	}, "ParticipantLifecycleWorkflow", id, action)
}

func (uc *ConfirmarConvite) lookup(ctx context.Context, id string) (*portin.ConviteResponse, error) {
	if uc.buscar != nil {
		return uc.buscar.Execute(ctx, id)
	}
	return &portin.ConviteResponse{ParticipantID: id, Status: string(convitedomain.StatusConfirmado)}, nil
}
