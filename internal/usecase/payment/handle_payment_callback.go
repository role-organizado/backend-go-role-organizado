package payment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	notificationdomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/notification"
	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
)

// PaymentConfirmationWriter appends a PAYMENT_CONFIRMATION entry to the subledger.
// Implemented by SubledgerDualWriteService.
type PaymentConfirmationWriter interface {
	AppendPaymentConfirmation(ctx context.Context, tx *domain.PaymentTransaction) error
}

// HandlePaymentCallback processes inbound payment-provider webhook callbacks.
// It is the Go mirror of the Java HandlePaymentCallbackUseCase.
//
// Idempotency is enforced at two layers:
//  1. ExistsByProviderAndEventID — cheap pre-check (avoids duplicate work).
//  2. SaveUnique — DB-level unique (provider, eventId) constraint handles the race
//     where two concurrent deliveries both pass the pre-check.
//
// Status routing (mirrors Java HandlePaymentCallbackUseCase):
//   - PENDING / AWAITING_* → silent no-op (not a final outcome)
//   - APPROVED / RECEIVED / CONFIRMED / COMPLETED → executeApproved
//   - REJECTED / FAILED / OVERDUE / CHARGEBACK* → executeFailed
//   - CANCELLED / REFUNDED → executeCancelled
//
// Implements portin.HandlePaymentCallbackUseCase.
type HandlePaymentCallback struct {
	txRepo          portout.PaymentTransactionRepository
	installmentRepo portout.PaymentInstallmentRepository
	webhookRepo     portout.ProcessedWebhookEventRepository
	allocationSvc   *InstallmentAllocationService  // nil-safe
	subledger       PaymentConfirmationWriter       // nil-safe
	createNotifUC   portin.CreateNotificacaoUseCase // nil-safe
}

// NewHandlePaymentCallback creates a new HandlePaymentCallback use case.
// allocationSvc, subledger, and createNotifUC are optional (nil → feature disabled).
func NewHandlePaymentCallback(
	txRepo portout.PaymentTransactionRepository,
	installmentRepo portout.PaymentInstallmentRepository,
	webhookRepo portout.ProcessedWebhookEventRepository,
	allocationSvc *InstallmentAllocationService,
	subledger PaymentConfirmationWriter,
	createNotifUC portin.CreateNotificacaoUseCase,
) *HandlePaymentCallback {
	return &HandlePaymentCallback{
		txRepo:          txRepo,
		installmentRepo: installmentRepo,
		webhookRepo:     webhookRepo,
		allocationSvc:   allocationSvc,
		subledger:       subledger,
		createNotifUC:   createNotifUC,
	}
}

// Execute processes a normalised webhook event from the payment provider.
func (uc *HandlePaymentCallback) Execute(ctx context.Context, in portin.PaymentCallbackPayload) error {
	provider := string(in.Provider)

	// ── Step 1: Idempotency pre-check ───────────────────────────────────────────
	exists, err := uc.webhookRepo.ExistsByProviderAndEventID(ctx, provider, in.ProviderEventID)
	if err != nil {
		return fmt.Errorf("handle payment callback: idempotency check: %w", err)
	}
	if exists {
		slog.InfoContext(ctx, "handle payment callback: event already processed (no-op)",
			"provider", provider,
			"eventID", in.ProviderEventID,
		)
		return nil
	}

	// ── Step 2: No-op for non-terminal statuses ─────────────────────────────────
	if isNonTerminalStatus(in.NewStatus) {
		slog.DebugContext(ctx, "handle payment callback: non-terminal status, skipping",
			"status", in.NewStatus,
			"eventID", in.ProviderEventID,
		)
		return nil
	}

	// ── Step 3: Record processed event (race-safe via unique index) ─────────────
	webhookEvent := &domain.ProcessedWebhookEvent{
		ID:                    uuid.New().String(),
		Provider:              provider,
		EventID:               in.ProviderEventID,
		ProviderTransactionID: in.ProviderTransactionID,
		EventType:             in.EventType,
		ProcessedAt:           time.Now(),
	}
	if saveErr := uc.webhookRepo.SaveUnique(ctx, webhookEvent); saveErr != nil {
		if errors.Is(saveErr, portout.ErrAlreadyProcessed) {
			// Another goroutine won the race — treat as no-op.
			slog.InfoContext(ctx, "handle payment callback: concurrent delivery detected, no-op",
				"provider", provider,
				"eventID", in.ProviderEventID,
			)
			return nil
		}
		return fmt.Errorf("handle payment callback: save webhook event: %w", saveErr)
	}

	slog.InfoContext(ctx, "handle payment callback: processing",
		"provider", provider,
		"eventID", in.ProviderEventID,
		"providerTransactionID", in.ProviderTransactionID,
		"status", in.NewStatus,
		"eventType", in.EventType,
	)

	// ── Step 4: Route to action ─────────────────────────────────────────────────
	switch mapCallbackStatus(in.NewStatus) {
	case callbackActionApproved:
		return uc.executeApproved(ctx, in)
	case callbackActionFailed:
		return uc.executeFailed(ctx, in)
	case callbackActionCancelled:
		return uc.executeCancelled(ctx, in)
	default:
		// Unknown/future status — log and treat as no-op.
		slog.WarnContext(ctx, "handle payment callback: unrecognised status, treating as no-op",
			"status", in.NewStatus,
			"eventID", in.ProviderEventID,
		)
		return nil
	}
}

// ── Status classification ─────────────────────────────────────────────────────

type callbackAction string

const (
	callbackActionApproved  callbackAction = "approved"
	callbackActionFailed    callbackAction = "failed"
	callbackActionCancelled callbackAction = "cancelled"
	callbackActionNoop      callbackAction = "noop"
)

// mapCallbackStatus classifies a provider status string into a callbackAction.
// Mirrors the Java HandlePaymentCallbackUseCase status routing.
func mapCallbackStatus(status string) callbackAction {
	switch strings.ToUpper(status) {
	case "APPROVED", "RECEIVED", "CONFIRMED", "COMPLETED":
		return callbackActionApproved
	case "REJECTED", "FAILED", "OVERDUE",
		"CHARGEBACK_REQUESTED", "CHARGEBACK_DISPUTE", "AWAITING_CHARGEBACK_REVERSAL",
		"DUNNING_RECEIVED", "DUNNING_REQUESTED":
		return callbackActionFailed
	case "CANCELLED", "REFUNDED":
		return callbackActionCancelled
	default:
		return callbackActionNoop
	}
}

// isNonTerminalStatus returns true for Asaas statuses that do not represent a
// final payment outcome and should be silently ignored.
func isNonTerminalStatus(status string) bool {
	switch strings.ToUpper(status) {
	case "PENDING", "AWAITING_RISK_ANALYSIS", "AWAITING_PAYMENT",
		"IN_ANALYSIS", "PROCESSING", "":
		return true
	}
	return false
}

// ── Action handlers ───────────────────────────────────────────────────────────

// executeApproved handles an approved/received/confirmed payment callback:
//  1. Fetches and validates the transaction.
//  2. Marks it COMPLETED.
//  3. Marks all linked installments PAID.
//  4. Runs installment allocation.
//  5. Appends PAYMENT_CONFIRMATION subledger entry (best-effort).
//  6. Sends an in-app notification to the participant (best-effort).
//  7. Writes a structured audit log entry.
func (uc *HandlePaymentCallback) executeApproved(ctx context.Context, in portin.PaymentCallbackPayload) error {
	tx, err := uc.txRepo.FindByProviderTransactionID(ctx, in.ProviderTransactionID)
	if err != nil {
		slog.WarnContext(ctx, "handle payment callback: transaction not found for approved event",
			"providerTransactionID", in.ProviderTransactionID,
			"error", err,
		)
		return fmt.Errorf("handle payment callback: find transaction (approved): %w", err)
	}

	// Skip if already terminal (idempotent re-delivery of approved event).
	if tx.IsTerminal() {
		slog.InfoContext(ctx, "handle payment callback: transaction already terminal, skipping",
			"transactionID", tx.ID,
			"status", tx.Status,
		)
		return nil
	}

	now := time.Now()
	tx.MarkCompleted(now)

	if updateErr := uc.txRepo.Update(ctx, tx); updateErr != nil {
		return fmt.Errorf("handle payment callback: update transaction to COMPLETED: %w", updateErr)
	}

	slog.InfoContext(ctx, "handle payment callback: transaction marked COMPLETED",
		"transactionID", tx.ID,
		"providerTransactionID", in.ProviderTransactionID,
	)

	// ── Mark installments PAID ──────────────────────────────────────────────────
	if len(tx.InstallmentIDs) > 0 {
		markErr := uc.installmentRepo.MarkPaidBatch(
			ctx,
			tx.InstallmentIDs,
			tx.ID,
			now,
			string(tx.PaymentMethod),
			in.ProviderTransactionID,
		)
		if markErr != nil {
			// Non-fatal: transaction is already COMPLETED; installment state can be
			// corrected by reconciliation.
			slog.WarnContext(ctx, "handle payment callback: mark paid batch failed (non-fatal)",
				"transactionID", tx.ID, "error", markErr)
		} else {
			uc.runAllocation(ctx, tx)
		}
	}

	// ── Subledger dual-write (PAYMENT_CONFIRMATION) ─────────────────────────────
	if uc.subledger != nil {
		if subledgerErr := uc.subledger.AppendPaymentConfirmation(ctx, tx); subledgerErr != nil {
			slog.WarnContext(ctx, "handle payment callback: subledger confirmation failed (non-fatal)",
				"transactionID", tx.ID, "error", subledgerErr)
		}
	}

	// ── Notify participant ───────────────────────────────────────────────────────
	if uc.createNotifUC != nil && tx.UserID != "" {
		if notifErr := uc.notifyParticipant(ctx, tx); notifErr != nil {
			slog.WarnContext(ctx, "handle payment callback: notification failed (non-fatal)",
				"transactionID", tx.ID, "userID", tx.UserID, "error", notifErr)
		}
	}

	// ── Audit trail ─────────────────────────────────────────────────────────────
	slog.InfoContext(ctx, "handle payment callback: audit — APPROVED",
		"entityType", "PAYMENT_TRANSACTION",
		"entityID", tx.ID,
		"action", "APPROVED",
		"providerTransactionID", in.ProviderTransactionID,
		"eventType", in.EventType,
		"completedAt", now.Format(time.RFC3339),
	)
	return nil
}

// executeFailed handles a rejected/failed/overdue/chargeback* payment callback.
func (uc *HandlePaymentCallback) executeFailed(ctx context.Context, in portin.PaymentCallbackPayload) error {
	tx, err := uc.txRepo.FindByProviderTransactionID(ctx, in.ProviderTransactionID)
	if err != nil {
		slog.WarnContext(ctx, "handle payment callback: transaction not found for failed event",
			"providerTransactionID", in.ProviderTransactionID, "error", err)
		return fmt.Errorf("handle payment callback: find transaction (failed): %w", err)
	}

	if tx.IsTerminal() {
		return nil
	}

	tx.MarkFailed(in.NewStatus)

	if updateErr := uc.txRepo.Update(ctx, tx); updateErr != nil {
		return fmt.Errorf("handle payment callback: update transaction to FAILED: %w", updateErr)
	}

	slog.InfoContext(ctx, "handle payment callback: audit — FAILED",
		"entityType", "PAYMENT_TRANSACTION",
		"entityID", tx.ID,
		"action", "FAILED",
		"reason", in.NewStatus,
	)
	return nil
}

// executeCancelled handles a cancelled/refunded payment callback.
func (uc *HandlePaymentCallback) executeCancelled(ctx context.Context, in portin.PaymentCallbackPayload) error {
	tx, err := uc.txRepo.FindByProviderTransactionID(ctx, in.ProviderTransactionID)
	if err != nil {
		slog.WarnContext(ctx, "handle payment callback: transaction not found for cancelled event",
			"providerTransactionID", in.ProviderTransactionID, "error", err)
		return fmt.Errorf("handle payment callback: find transaction (cancelled): %w", err)
	}

	if tx.IsTerminal() {
		return nil
	}

	tx.Status = domain.TransactionStatusCancelled
	tx.FailureReason = in.NewStatus
	tx.UpdatedAt = time.Now()

	if updateErr := uc.txRepo.Update(ctx, tx); updateErr != nil {
		return fmt.Errorf("handle payment callback: update transaction to CANCELLED: %w", updateErr)
	}

	slog.InfoContext(ctx, "handle payment callback: audit — CANCELLED",
		"entityType", "PAYMENT_TRANSACTION",
		"entityID", tx.ID,
		"action", "CANCELLED",
	)
	return nil
}

// ── Internal helpers ──────────────────────────────────────────────────────────

// runAllocation fetches the freshly-paid installments and creates allocation records.
// All allocation errors are non-fatal (logged as warnings).
func (uc *HandlePaymentCallback) runAllocation(ctx context.Context, tx *domain.PaymentTransaction) {
	if uc.allocationSvc == nil || len(tx.InstallmentIDs) == 0 {
		return
	}
	installments, fetchErr := uc.installmentRepo.FindByIDs(ctx, tx.InstallmentIDs)
	if fetchErr != nil {
		slog.WarnContext(ctx, "handle payment callback: fetch installments for allocation failed (non-fatal)",
			"transactionID", tx.ID, "error", fetchErr)
		return
	}
	for _, inst := range installments {
		if allocErr := uc.allocationSvc.Allocate(ctx, inst); allocErr != nil {
			slog.WarnContext(ctx, "handle payment callback: allocation failed (non-fatal)",
				"installmentID", inst.ID, "error", allocErr)
		}
	}
}

// notifyParticipant sends an in-app PAGAMENTO notification to the transaction owner.
func (uc *HandlePaymentCallback) notifyParticipant(ctx context.Context, tx *domain.PaymentTransaction) error {
	amountBRL := float64(tx.AmountCents) / 100.0
	_, err := uc.createNotifUC.Execute(ctx, portin.CreateNotificacaoInput{
		UsuarioID: tx.UserID,
		Tipo:      notificationdomain.TipoNotificacaoPagamento,
		Titulo:    "Pagamento Confirmado",
		Mensagem:  fmt.Sprintf("Seu pagamento de R$ %.2f foi confirmado.", amountBRL),
		Dados: map[string]string{
			"transactionId": tx.ID,
			"eventId":       tx.EventID,
			"amountCents":   fmt.Sprintf("%d", tx.AmountCents),
		},
	})
	return err
}

// compile-time assertion.
var _ portin.HandlePaymentCallbackUseCase = (*HandlePaymentCallback)(nil)
