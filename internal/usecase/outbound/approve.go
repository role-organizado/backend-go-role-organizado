package outbound

import (
	"context"
	"log/slog"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/outbound"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// outboundExecutionTaskQueue and outboundExecutionWorkflowName must match the
// Java backend (TemporalWorkerRegistry.OUTBOUND_EXECUTION_TASK_QUEUE +
// OutboundExecutionWorkflow) so workflow IDs/signals interoperate cross-backend.
const (
	outboundExecutionTaskQueue    = "OUTBOUND_EXECUTION_QUEUE"
	outboundExecutionWorkflowName = "OutboundExecutionWorkflow"
)

// ApproveOutboundRequest implements portin.ApproveOutboundRequestUseCase.
// Direct organizer approval — bypasses voting. Validates organizer role and
// PENDING status; triggers the OutboundTransferProvider on success.
type ApproveOutboundRequest struct {
	requests     portout.OutboundRequestRepository
	eventos      portout.EventoRepository
	participants portout.ParticipantRepository
	provider     portout.OutboundTransferProvider
	audit        portout.OutboundAuditLogRepository
	// temporalStarter optionally starts the OutboundExecutionWorkflow for
	// real-execution tracking. nil-safe: when unset, no workflow is started.
	temporalStarter portout.TemporalWorkflowStarter
}

// WithTemporalStarter attaches a Temporal starter so the OutboundExecutionWorkflow
// (Trilha B) is launched after a successful approval. Returns the use case for
// chaining. Mirrors Java's TemporalOutboundExecutionStarter wiring.
func (uc *ApproveOutboundRequest) WithTemporalStarter(s portout.TemporalWorkflowStarter) *ApproveOutboundRequest {
	uc.temporalStarter = s
	return uc
}

// NewApproveOutboundRequest wires the approve use case.
func NewApproveOutboundRequest(
	requests portout.OutboundRequestRepository,
	eventos portout.EventoRepository,
	participants portout.ParticipantRepository,
	provider portout.OutboundTransferProvider,
	audit portout.OutboundAuditLogRepository,
) *ApproveOutboundRequest {
	return &ApproveOutboundRequest{
		requests:     requests,
		eventos:      eventos,
		participants: participants,
		provider:     provider,
		audit:        audit,
	}
}

// Execute approves the request and runs the provider.
func (uc *ApproveOutboundRequest) Execute(ctx context.Context, in portin.ApproveOutboundRequestInput) (*domain.OutboundRequest, error) {
	req, err := uc.requests.FindByID(ctx, in.RequestID)
	if err != nil {
		return nil, err
	}
	// Optional event scoping when supplied.
	if in.EventID != "" && in.EventID != req.EventID {
		return nil, apierr.NotFoundMsg("solicitação outbound não encontrada para o evento informado")
	}

	// Anti-enumeration: non-organizers see 404 instead of 403.
	ok, err := isOrganizer(ctx, uc.eventos, uc.participants, req.EventID, in.ApproverUserID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, apierr.NotFoundMsg("solicitação outbound não encontrada")
	}

	if err := req.Approve(in.ApproverUserID, in.ApprovalNotes); err != nil {
		return nil, apierr.Unprocessable(err.Error())
	}
	saved, err := uc.requests.Update(ctx, req)
	if err != nil {
		return nil, err
	}
	appendAudit(ctx, uc.audit, saved, in.ApproverUserID, domain.AuditActionApproved, in.ApprovalNotes)

	// Best-effort: start the OutboundExecutionWorkflow for real-execution tracking.
	// A failure here must never break the approval/provider flow (Java parity).
	uc.startExecutionTracking(ctx, saved, in.ApproverUserID, in.ApprovalNotes)

	return executeProviderAndPersist(ctx, uc.requests, uc.provider, uc.audit, saved)
}

// startExecutionTracking launches the OutboundExecutionWorkflow when a Temporal
// starter is wired and the request carries the ids needed for the workflow.
func (uc *ApproveOutboundRequest) startExecutionTracking(ctx context.Context, req *domain.OutboundRequest, actorUserID, reason string) {
	if uc.temporalStarter == nil || req == nil || req.ID == "" || req.EventID == "" {
		return
	}

	triggeredBy := actorUserID
	if triggeredBy == "" {
		triggeredBy = "SYSTEM"
	}

	// Workflow ID mirrors Java's outboundPrimary("outbound-real-<id>"). The
	// workflow name is passed as a string to avoid an import cycle with the
	// Temporal adapter (same pattern as the payment use cases).
	type outboundExecutionInput struct {
		OutboundRequestID string `json:"outboundRequestId"`
		EventID           string `json:"eventId"`
		TriggeredBy       string `json:"triggeredBy"`
		Reason            string `json:"reason"`
	}

	err := uc.temporalStarter.StartWorkflow(ctx, portout.WorkflowStartOptions{
		WorkflowID: "outbound-real-" + req.ID,
		TaskQueue:  outboundExecutionTaskQueue,
	}, outboundExecutionWorkflowName, outboundExecutionInput{
		OutboundRequestID: req.ID,
		EventID:           req.EventID,
		TriggeredBy:       triggeredBy,
		Reason:            reason,
	})
	if err != nil {
		slog.WarnContext(ctx, "failed to start OutboundExecutionWorkflow (non-fatal)",
			"requestId", req.ID, "eventId", req.EventID, "error", err)
	}
}

var _ portin.ApproveOutboundRequestUseCase = (*ApproveOutboundRequest)(nil)
