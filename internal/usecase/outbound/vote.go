package outbound

import (
	"context"
	"time"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/outbound"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// VoteOnOutboundRequest implements portin.VoteOnOutboundRequestUseCase.
//
// Voting rules (mirroring Java VoteOnOutboundRequestUseCase):
//   - Request must be PENDING and require voting.
//   - Requester cannot vote on their own request.
//   - Only event participants can vote (rateio-allocation participants in Java; we
//     use the event participant set as the equivalent universe in Go).
//   - A user may vote only once per request.
//   - Voting window expiry is enforced — expired requests are transitioned to
//     EXPIRED and the call returns 422.
//   - ALL_PARTICIPANTS mode: any rejection rejects the request; reaching the
//     required approvals approves it.
//   - QUORUM_50 mode: once total votes ≥ required, majority wins; ties → REJECTED.
type VoteOnOutboundRequest struct {
	requests     portout.OutboundRequestRepository
	eventos      portout.EventoRepository
	participants portout.ParticipantRepository
	provider     portout.OutboundTransferProvider
	audit        portout.OutboundAuditLogRepository
}

// NewVoteOnOutboundRequest wires the vote use case.
func NewVoteOnOutboundRequest(
	requests portout.OutboundRequestRepository,
	eventos portout.EventoRepository,
	participants portout.ParticipantRepository,
	provider portout.OutboundTransferProvider,
	audit portout.OutboundAuditLogRepository,
) *VoteOnOutboundRequest {
	return &VoteOnOutboundRequest{
		requests:     requests,
		eventos:      eventos,
		participants: participants,
		provider:     provider,
		audit:        audit,
	}
}

// Execute records a vote and applies quorum.
func (uc *VoteOnOutboundRequest) Execute(ctx context.Context, in portin.VoteOnOutboundRequestInput) (*portin.VoteResult, error) {
	req, err := uc.requests.FindByID(ctx, in.RequestID)
	if err != nil {
		return nil, err
	}
	if in.EventID != "" && in.EventID != req.EventID {
		return nil, apierr.NotFoundMsg("solicitação outbound não encontrada para o evento informado")
	}
	if req.Status != domain.StatusPending {
		return nil, apierr.Unprocessable("apenas solicitações PENDING podem receber votos")
	}
	if !req.RequiresVoting {
		return nil, apierr.Unprocessable("esta solicitação não está sujeita a votação")
	}
	if req.IsVotingExpired(time.Now().UTC()) {
		req.MarkAsExpired()
		_, _ = uc.requests.Update(ctx, req)
		appendAudit(ctx, uc.audit, req, "system", domain.AuditActionExpired, "voting window expired")
		return nil, apierr.Unprocessable("janela de votação expirada")
	}
	if req.RequesterUserID == in.UserID {
		return nil, apierr.Unprocessable("solicitante não pode votar em sua própria solicitação")
	}
	if req.HasUserVoted(in.UserID) {
		return nil, apierr.Unprocessable("usuário já votou nesta solicitação")
	}
	// Verify the voter is an event participant. Anti-enumeration: 404 when not.
	if err := requireEventParticipant(ctx, uc.eventos, uc.participants, req.EventID, in.UserID); err != nil {
		return nil, err
	}

	voteValue := domain.VoteReject
	if in.Approve {
		voteValue = domain.VoteApprove
	}
	if err := req.AddVote(in.UserID, voteValue, in.Comment); err != nil {
		return nil, apierr.Unprocessable(err.Error())
	}

	// Apply quorum decisions.
	if req.HasReachedRejectionQuorum() {
		req.RejectImmediately("voting threshold reached: REJECTED")
	} else if req.HasReachedApprovalQuorum() {
		_ = req.Approve(in.UserID, "auto-approved via voting")
	}

	saved, err := uc.requests.Update(ctx, req)
	if err != nil {
		return nil, err
	}
	appendAudit(ctx, uc.audit, saved, in.UserID, domain.AuditActionVoted, string(voteValue))

	// Trigger provider execution if voting resolved to APPROVED.
	finalRequest := saved
	if saved.Status == domain.StatusApproved {
		executed, execErr := executeProviderAndPersist(ctx, uc.requests, uc.provider, uc.audit, saved)
		if execErr != nil {
			return nil, execErr
		}
		finalRequest = executed
	}

	result := &portin.VoteResult{
		Request:        finalRequest,
		TotalVotes:     len(finalRequest.Votes),
		ApproveVotes:   finalRequest.Approvals,
		RejectVotes:    finalRequest.Rejections,
		RequiredVotes:  finalRequest.RequiredVotes,
		VotingComplete: finalRequest.Status != domain.StatusPending,
		FinalStatus:    finalRequest.Status,
	}
	return result, nil
}

var _ portin.VoteOnOutboundRequestUseCase = (*VoteOnOutboundRequest)(nil)
