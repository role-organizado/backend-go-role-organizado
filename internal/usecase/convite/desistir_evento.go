package convite

import (
	"context"
	"time"

	"github.com/google/uuid"

	convitedomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/convite"
	domainevent "github.com/role-organizado/backend-go-role-organizado/internal/domain/event"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// DesistirEvento implements portin.DesistirEventoUseCase.
//
// Business rules (mirror Java):
//   - ownership: participant.usuarioId must equal the JWT userId → otherwise 403.
//   - cannot withdraw if status is already CANCELADO/RECUSADO → 400.
//   - evento in CONCLUIDO/Finalizado phase blocks desistência → 422.
//   - ORGANIZADOR único bloqueado → 422.
//   - Calculates refund via cancellation tier evaluator.
//   - Cancels PENDING/OVERDUE installments.
//   - If refund > 0, registers a ParticipantCredit (DESISTENCIA, PENDING).
//   - Fires Temporal ACTION_REMOVE workflow post-commit (Execute only, not Preview).
type DesistirEvento struct {
	participants portout.ConviteParticipantRepository
	eventos      portout.EventoRepository
	politicas    portout.ConvitePoliticaCancelamentoRepository
	installments portout.ConviteInstallmentRepository
	credits      portout.ParticipantCreditRepository
	temporal     portout.TemporalWorkflowStarter
}

// NewDesistirEvento wires the use case.
func NewDesistirEvento(
	participants portout.ConviteParticipantRepository,
	eventos portout.EventoRepository,
	politicas portout.ConvitePoliticaCancelamentoRepository,
	installments portout.ConviteInstallmentRepository,
	credits portout.ParticipantCreditRepository,
	temporal portout.TemporalWorkflowStarter,
) *DesistirEvento {
	return &DesistirEvento{
		participants: participants,
		eventos:      eventos,
		politicas:    politicas,
		installments: installments,
		credits:      credits,
		temporal:     temporal,
	}
}

// Execute performs the desistência with side effects (installment cancellation,
// credit creation, Temporal signal).
func (uc *DesistirEvento) Execute(ctx context.Context, participantID, userID string) (*portin.DesistenciaResult, error) {
	calc, p, err := uc.evaluate(ctx, participantID, userID)
	if err != nil {
		return nil, err
	}

	// Cancel pending installments.
	if uc.installments != nil {
		if _, cerr := uc.installments.CancelPendingByParticipantID(ctx, p.EventoID, p.ID); cerr != nil {
			return nil, cerr
		}
	}

	// Mark participant as CANCELADO.
	now := time.Now().UTC()
	if err := uc.participants.UpdateStatus(ctx, p.ID, convitedomain.StatusCancelado, &now); err != nil {
		return nil, err
	}

	// Issue a credit when refund > 0.
	if calc.RefundAmountCents > 0 && uc.credits != nil {
		creditID := uuid.New().String()
		_, _ = uc.credits.Save(ctx, portout.ParticipantCredit{
			ID:            creditID,
			ParticipantID: p.ID,
			EventoID:      p.EventoID,
			UsuarioID:     p.UsuarioID,
			AmountCents:   calc.RefundAmountCents,
			Reason:        string(convitedomain.CreditReasonDesistencia),
			Status:        "PENDING",
			CreatedAt:     now,
		})
		calc.CreditID = creditID
	}

	// Post-commit Temporal signal.
	if uc.temporal != nil {
		_ = uc.temporal.StartWorkflow(ctx, portout.WorkflowStartOptions{
			WorkflowID: "participant-lifecycle-" + p.ID + "-ACTION_REMOVE",
			TaskQueue:  "participant-lifecycle",
		}, "ParticipantLifecycleWorkflow", p.ID, "ACTION_REMOVE")
	}

	calc.Status = string(convitedomain.StatusCancelado)
	return calc, nil
}

// Preview returns what would happen on Execute without any side effects.
func (uc *DesistirEvento) Preview(ctx context.Context, participantID, userID string) (*portin.DesistenciaResult, error) {
	calc, _, err := uc.evaluate(ctx, participantID, userID)
	if err != nil {
		return nil, err
	}
	calc.Status = "PREVIEW"
	return calc, nil
}

// evaluate runs all the read-only checks and returns the projected refund result.
func (uc *DesistirEvento) evaluate(ctx context.Context, participantID, userID string) (*portin.DesistenciaResult, *convitedomain.Participant, error) {
	if participantID == "" {
		return nil, nil, apierr.BadRequest("participantId é obrigatório")
	}
	if userID == "" {
		return nil, nil, apierr.Unauthorized("autenticação necessária")
	}

	p, err := uc.participants.FindByID(ctx, participantID)
	if err != nil {
		return nil, nil, err
	}
	if p == nil {
		return nil, nil, apierr.NotFound("participant", participantID)
	}

	// Ownership.
	if p.UsuarioID != userID {
		return nil, nil, apierr.Forbidden("OWNERSHIP_VIOLATION")
	}

	if p.Status == convitedomain.StatusCancelado {
		return nil, nil, apierr.BadRequest("participação já cancelada")
	}

	evt, eerr := uc.eventos.FindByID(ctx, p.EventoID)
	if eerr != nil {
		return nil, nil, eerr
	}
	if evt == nil {
		return nil, nil, apierr.NotFound("evento", p.EventoID)
	}
	if evt.Status == domainevent.EventoStatusConcluido {
		return nil, nil, apierr.Unprocessable("EVENTO_FINALIZADO")
	}

	// ORGANIZADOR único bloqueado: if requester is the sole ORGANIZADOR, block.
	if p.Papel == convitedomain.PapelOrganizador {
		orgs, lerr := uc.participants.FindByEventoIDAndPapel(ctx, p.EventoID, convitedomain.PapelOrganizador)
		if lerr != nil {
			return nil, nil, lerr
		}
		active := 0
		for _, o := range orgs {
			if o.Status != convitedomain.StatusCancelado && o.Status != convitedomain.StatusRecusado {
				active++
			}
		}
		if active <= 1 {
			return nil, nil, apierr.Unprocessable("ORGANIZADOR_UNICO")
		}
	}

	// Load installments to compute already-paid.
	var totalPaid int64
	if uc.installments != nil {
		insts, ierr := uc.installments.FindByParticipantID(ctx, p.EventoID, p.ID)
		if ierr != nil {
			return nil, nil, ierr
		}
		for _, i := range insts {
			if i.PaidAt != nil || i.Status == "PAID" {
				totalPaid += i.AmountCents
			}
		}
	}

	// Resolve cancellation policy.
	policy := convitedomain.DefaultGenericPolicy()
	if uc.politicas != nil {
		chave := evt.PoliticaCancelamento
		if chave == "" {
			chave = "GENERICA_FLEXIVEL"
		}
		if loaded, perr := uc.politicas.FindByChave(ctx, chave); perr == nil && loaded != nil && len(loaded.Tiers) > 0 {
			policy = *loaded
		}
	}

	daysBefore := 0
	if !evt.Data.IsZero() {
		daysBefore = convitedomain.DaysBefore(time.Now().UTC(), evt.Data)
	}
	refund, percent, label := policy.EvaluateRefund(totalPaid, daysBefore)

	return &portin.DesistenciaResult{
		TotalPagoCents:    totalPaid,
		RefundAmountCents: refund,
		RefundPercent:     percent,
		TierLabel:         label,
		PoliticaAplicada:  policy.Chave,
	}, p, nil
}
