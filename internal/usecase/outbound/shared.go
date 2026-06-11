// Package outbound implements the Outbound Transfers use cases (supplier payments
// and expense reimbursements with multi-organizer/voting approval).
package outbound

import (
	"context"
	"log/slog"
	"strings"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/outbound"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// votingWindowDays is the default voting window applied to new requests.
// Mirrors the 7-day window used by configureApproval in the Java backend.
const votingWindowDays = 7

// organizadorPapeis lists participant Status values treated as organizer-equivalent.
// finance.Participant carries the legacy 'papel' value in the Status field for the
// read-side projection used by the outbound flow.
var organizadorPapeis = map[string]bool{
	"ORGANIZADOR":    true,
	"CO_ORGANIZADOR": true,
	"ADMIN":          true,
	"ORGANIZACAO":    true,
}

// isOrganizer returns true when the user is the event owner or holds an organizer
// participant role.
func isOrganizer(ctx context.Context, eventos portout.EventoRepository, participants portout.ParticipantRepository, eventID, userID string) (bool, error) {
	evt, err := eventos.FindByID(ctx, eventID)
	if err == nil && evt != nil && evt.UsuarioID == userID {
		return true, nil
	}
	if err != nil && !apierr.IsNotFound(err) {
		return false, err
	}
	p, err := participants.FindByEventIDAndUserID(ctx, eventID, userID)
	if err != nil {
		if apierr.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return organizadorPapeis[strings.ToUpper(p.Status)], nil
}

// requireEventParticipant returns NotFound (anti-enumeration) when the user is not
// a participant of the event.
func requireEventParticipant(ctx context.Context, eventos portout.EventoRepository, participants portout.ParticipantRepository, eventID, userID string) error {
	evt, err := eventos.FindByID(ctx, eventID)
	if err == nil && evt != nil && evt.UsuarioID == userID {
		return nil
	}
	if err != nil && !apierr.IsNotFound(err) {
		return err
	}
	if _, err := participants.FindByEventIDAndUserID(ctx, eventID, userID); err != nil {
		if apierr.IsNotFound(err) {
			return apierr.NotFoundMsg("usuário não é participante do evento")
		}
		return err
	}
	return nil
}

// appendAudit records a structured audit log entry; failures are non-fatal.
func appendAudit(ctx context.Context, audit portout.OutboundAuditLogRepository, req *domain.OutboundRequest, actorID string, action domain.AuditAction, details string) {
	if audit == nil {
		return
	}
	if err := audit.Append(ctx, &domain.AuditLog{
		RequestID: req.ID,
		EventID:   req.EventID,
		ActorID:   actorID,
		Action:    action,
		Details:   details,
	}); err != nil {
		slog.WarnContext(ctx, "outbound: audit append failed (non-fatal)",
			"requestID", req.ID, "action", action, "error", err)
	}
}

// executeProviderAndPersist runs the OutboundTransferProvider for an APPROVED
// request and updates persistence with the result. Failures are encoded in the
// request status (FAILED) so callers can return 200 with a meaningful payload.
func executeProviderAndPersist(
	ctx context.Context,
	repo portout.OutboundRequestRepository,
	provider portout.OutboundTransferProvider,
	audit portout.OutboundAuditLogRepository,
	req *domain.OutboundRequest,
) (*domain.OutboundRequest, error) {
	if provider == nil || !provider.IsEnabled() {
		// No provider — leave APPROVED for manual operator processing.
		slog.InfoContext(ctx, "outbound: provider disabled, request left APPROVED for manual processing",
			"requestID", req.ID)
		return req, nil
	}
	resp, err := provider.ExecuteTransfer(ctx, &portout.OutboundTransferRequest{
		OutboundRequestID: req.ID,
		EventID:           req.EventID,
		RateioID:          req.RateioID,
		AmountCents:       req.AmountCents,
		RecipientName:     req.Recipient.Name,
		RecipientDocument: req.Recipient.Document,
		PixKey:            req.Recipient.PixKey,
		PixKeyType:        req.Recipient.PixKeyType,
		Description:       req.Justification,
	})
	if err != nil || resp == nil || !resp.Success {
		reason := "provider error"
		if err != nil {
			reason = err.Error()
		} else if resp != nil && resp.ErrorMessage != "" {
			reason = resp.ErrorMessage
		}
		req.MarkAsFailed(reason)
		updated, updateErr := repo.Update(ctx, req)
		if updateErr != nil {
			return nil, updateErr
		}
		appendAudit(ctx, audit, updated, "system", domain.AuditActionFailed, reason)
		return updated, nil
	}
	req.MarkAsProcessing(resp.Provider, resp.ProviderTransferID)
	updated, updateErr := repo.Update(ctx, req)
	if updateErr != nil {
		return nil, updateErr
	}
	appendAudit(ctx, audit, updated, "system", domain.AuditActionExecuted, resp.Provider+":"+resp.ProviderTransferID)
	return updated, nil
}

// trimmedRequired returns an Unprocessable error when s is blank after trimming.
func trimmedRequired(s, fieldName string) error {
	if strings.TrimSpace(s) == "" {
		return apierr.Unprocessable(fieldName + " é obrigatório")
	}
	return nil
}

