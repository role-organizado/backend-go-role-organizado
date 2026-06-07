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
	// temporalSignaler signals the PaymentExpirationWorkflow when payment is confirmed.
	// nil-safe: disables Temporal signalling when not set.
	temporalSignaler portout.TemporalWorkflowSignaler
	// temporalStarter starts the PaymentConfirmationWorkflow for async retry semantics.
	// nil-safe: disables Temporal workflow start when not set.
	temporalStarter portout.TemporalWorkflowStarter
}

// WithTemporalSignaler attaches a Temporal signaler so the expiration workflow
// receives a paymentCompleted signal when a webhook confirms the payment.
func (uc *HandlePaymentCallback) WithTemporalSignaler(s portout.TemporalWorkflowSignaler) *HandlePaymentCallback {
	uc.temporalSignaler = s
	return uc
}

// WithTemporalStarter attaches a Temporal starter so the PaymentConfirmationWorkflow
// is launched for async retry semantics when a webhook arrives.
func (uc *HandlePaymentCallback) WithTemporalStarter(s portout.TemporalWorkflowStarter) *HandlePaymentCallback {
	uc.temporalStarter = s
	return uc
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
	action := mapCallbackStatus(in.NewStatus)
	var actionErr error
	switch action {
	case callbackActionApproved:
		actionErr = uc.executeApproved(ctx, in)
	case callbackActionFailed:
		actionErr = uc.executeFailed(ctx, in)
	case callbackActionCancelled:
		actionErr = uc.executeCancelled(ctx, in)
	default:
		// Unknown/future status — log and treat as no-op.
		slog.WarnContext(ctx, "handle payment callback: unrecognised status, treating as no-op",
			"status", in.NewStatus,
			"eventID", in.ProviderEventID,
		)
		return nil
	}
	if actionErr != nil {
		return actionErr
	}

	// ── Step 5 (optional): Temporal integration ─────────────────────────────────
	// Signal the PaymentExpirationWorkflow so it exits cleanly (avoids expiring a
	// payment that was just confirmed). Best-effort: non-fatal if Temporal is down.
	if uc.temporalSignaler != nil {
		uc.signalExpirationWorkflow(ctx, in)
	}
	// Start the PaymentConfirmationWorkflow for audit trail and retry semantics.
	if uc.temporalStarter != nil {
		uc.startConfirmationWorkflow(ctx, in, action)
	}

	return nil
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

// ── Temporal helpers ──────────────────────────────────────────────────────────

// signalExpirationWorkflow sends a paymentCompleted signal to the
// PaymentExpirationWorkflow identified by the transaction ID.
// Non-fatal: errors are logged at WARN level.
func (uc *HandlePaymentCallback) signalExpirationWorkflow(ctx context.Context, in portin.PaymentCallbackPayload) {
	// Look up the transaction to get the platform ID (needed for workflow ID).
	tx, err := uc.txRepo.FindByProviderTransactionID(ctx, in.ProviderTransactionID)
	if err != nil {
		slog.WarnContext(ctx, "handle payment callback: cannot signal expiration workflow — transaction not found",
			"providerTransactionID", in.ProviderTransactionID, "error", err)
		return
	}

	workflowID := "payment-expiration-real-" + tx.ID
	if signalErr := uc.temporalSignaler.SignalWorkflow(ctx, workflowID, "paymentCompleted", in.NewStatus); signalErr != nil {
		// Non-fatal: workflow may have already completed (timer fired before webhook).
		slog.WarnContext(ctx, "handle payment callback: signal expiration workflow failed (non-fatal)",
			"workflowID", workflowID, "error", signalErr)
	}
}

// startConfirmationWorkflow starts a PaymentConfirmationWorkflow for async retry semantics.
// Non-fatal: errors are logged at WARN level.
func (uc *HandlePaymentCallback) startConfirmationWorkflow(ctx context.Context, in portin.PaymentCallbackPayload, action callbackAction) {
	callbackType := string(action)
	switch action {
	case callbackActionApproved:
		callbackType = "APPROVED"
	case callbackActionFailed:
		callbackType = "FAILED"
	case callbackActionCancelled:
		callbackType = "CANCELLED"
	}

	workflowID := "payment-confirmation-real-" + in.ProviderTransactionID + "-" + strings.ToLower(callbackType)

	type confirmationInput struct {
		ProviderTransactionID string `json:"providerTransactionId"`
		ProviderName          string `json:"providerName"`
		CallbackType          string `json:"callbackType"`
		FailureReason         string `json:"failureReason,omitempty"`
		ProviderEventID       string `json:"providerEventId"`
		EventType             string `json:"eventType"`
	}

	input := confirmationInput{
		ProviderTransactionID: in.ProviderTransactionID,
		ProviderName:          string(in.Provider),
		CallbackType:          callbackType,
		FailureReason:         in.NewStatus,
		ProviderEventID:       in.ProviderEventID,
		EventType:             in.EventType,
	}

	startErr := uc.temporalStarter.StartWorkflow(ctx, portout.WorkflowStartOptions{
		WorkflowID: workflowID,
		TaskQueue:  "payment-queue",
	}, "PaymentConfirmationWorkflow", input)
	if startErr != nil {
		slog.WarnContext(ctx, "handle payment callback: start confirmation workflow failed (non-fatal)",
			"workflowID", workflowID, "error", startErr)
	}
}

// compile-time assertion.
var _ portin.HandlePaymentCallbackUseCase = (*HandlePaymentCallback)(nil)
