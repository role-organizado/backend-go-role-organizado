package payment

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ProcessBatchPayment implements portin.ProcessBatchPaymentUseCase.
//
// It charges multiple installments atomically in a single PaymentTransaction:
//  1. Idempotency check via IdempotencyKey
//  2. Fetch all installments by IDs — 404 if any not found
//  3. Validate ownership — 403 if any installment belongs to another user
//  4. Validate status — 400 if any installment is PAID / PROCESSING / CANCELLED
//  5. Sum AmountCents from the installment records
//  6. CreateOrGetCustomer in Asaas (cached via AsaasCustomerLink)
//  7. ResolveFeePolicySnapshot + CalculateFees
//  8. Persist ONE PaymentTransaction as PENDING with InstallmentIDs[]
//  9. SubledgerDualWrite PAYMENT_COMMITMENT (best-effort, non-fatal)
//  10. CreatePayment at provider (tokenize CC if needed)
//  11. CC approved immediately → MarkPaidBatch atomically; PIX/Boleto stay PENDING
//  12. Provider failure → transaction FAILED, no installments changed
type ProcessBatchPayment struct {
	installmentRepo  portout.PaymentInstallmentRepository
	participantes    portout.ParticipanteRepository
	txRepo           portout.PaymentTransactionRepository
	customerLinkRepo portout.AsaasCustomerLinkRepository
	usuarioRepo      portout.UsuarioRepository
	savedCardRepo    portout.SavedCreditCardRepository
	provider         portout.PaymentProvider
	providerName     domain.PaymentProvider
	feePolicy        FeePolicyResolver
	subledger        PaymentCommitmentWriter
}

// NewProcessBatchPayment creates a new ProcessBatchPayment use case.
func NewProcessBatchPayment(
	installmentRepo portout.PaymentInstallmentRepository,
	participantes portout.ParticipanteRepository,
	txRepo portout.PaymentTransactionRepository,
	customerLinkRepo portout.AsaasCustomerLinkRepository,
	usuarioRepo portout.UsuarioRepository,
	savedCardRepo portout.SavedCreditCardRepository,
	provider portout.PaymentProvider,
	providerName domain.PaymentProvider,
	feePolicy FeePolicyResolver,
	subledger PaymentCommitmentWriter,
) *ProcessBatchPayment {
	return &ProcessBatchPayment{
		installmentRepo:  installmentRepo,
		participantes:    participantes,
		txRepo:           txRepo,
		customerLinkRepo: customerLinkRepo,
		usuarioRepo:      usuarioRepo,
		savedCardRepo:    savedCardRepo,
		provider:         provider,
		providerName:     providerName,
		feePolicy:        feePolicy,
		subledger:        subledger,
	}
}

// Execute processes a batch payment atomically.
func (uc *ProcessBatchPayment) Execute(ctx context.Context, in portin.ProcessBatchPaymentInput) (*portin.BatchPaymentResponse, error) {
	// ── Step 1: Idempotency ─────────────────────────────────────────────────────
	if in.IdempotencyKey != "" {
		existing, err := uc.txRepo.FindByIdempotencyKey(ctx, in.IdempotencyKey)
		if err == nil {
			slog.InfoContext(ctx, "process batch payment: idempotent request, returning existing",
				"idempotencyKey", in.IdempotencyKey, "transactionID", existing.ID)
			return batchResponseFromTx(existing), nil
		}
		if !apierr.IsNotFound(err) {
			return nil, fmt.Errorf("process batch payment: idempotency check: %w", err)
		}
	}

	// ── Step 2: Validate input ──────────────────────────────────────────────────
	if len(in.InstallmentIDs) == 0 {
		return nil, apierr.BadRequest("installmentIds não pode ser vazio")
	}

	// ── Step 3: Fetch installments ───────────────────────────────────────────────
	installments, err := uc.installmentRepo.FindByIDs(ctx, in.InstallmentIDs)
	if err != nil {
		return nil, fmt.Errorf("process batch payment: fetch installments: %w", err)
	}

	// 404 if any requested ID was not found.
	foundIDs := make(map[string]struct{}, len(installments))
	for _, inst := range installments {
		foundIDs[inst.ID] = struct{}{}
	}
	for _, id := range in.InstallmentIDs {
		if _, ok := foundIDs[id]; !ok {
			return nil, apierr.NotFound("installment", id)
		}
	}

	// ── Step 4: Validate ownership ───────────────────────────────────────────────
	// Installments may be stored under a participationId rather than the userID
	// directly (BUG5/spec-096). Resolve all participation IDs for the user.
	participationIDs, err := uc.participantes.FindParticipationIDsByUserID(ctx, in.UserID)
	if err != nil {
		return nil, fmt.Errorf("process batch payment: fetch participations: %w", err)
	}
	participationSet := make(map[string]struct{}, len(participationIDs)+1)
	participationSet[in.UserID] = struct{}{} // userID itself is always valid
	for _, pid := range participationIDs {
		participationSet[pid] = struct{}{}
	}
	for _, inst := range installments {
		if _, ok := participationSet[inst.ParticipantID]; !ok {
			return nil, apierr.Forbidden("installment não pertence ao usuário autenticado: " + inst.ID)
		}
	}

	// ── Step 5: Validate status ──────────────────────────────────────────────────
	for _, inst := range installments {
		switch inst.Status {
		case domain.InstallmentStatusPaid,
			domain.InstallmentStatusProcessing,
			domain.InstallmentStatusCancelled:
			return nil, apierr.BadRequest(fmt.Sprintf(
				"installment %s possui status inválido para pagamento: %s", inst.ID, inst.Status))
		}
	}

	// ── Step 6: Sum amounts ──────────────────────────────────────────────────────
	var totalAmountCents int64
	for _, inst := range installments {
		totalAmountCents += inst.AmountCents
	}
	if totalAmountCents < 100 {
		return nil, apierr.BadRequest("valor total mínimo é R$1,00 (100 centavos)")
	}

	// ── Step 6b: CPF validation ──────────────────────────────────────────────────
	switch in.Method {
	case domain.PaymentMethodPix, domain.PaymentMethodBoleto, domain.PaymentMethodCreditCard:
		if in.CPF == "" {
			return nil, apierr.BadRequest("CPF é obrigatório para este método de pagamento")
		}
	}

	// ── Step 7: Resolve saved card token ─────────────────────────────────────────
	if in.SavedCardID != "" && in.Method == domain.PaymentMethodCreditCard {
		savedCard, scErr := uc.savedCardRepo.FindByID(ctx, in.SavedCardID)
		if scErr != nil {
			return nil, apierr.BadRequest("cartão salvo não encontrado: " + in.SavedCardID)
		}
		if !savedCard.IsActive {
			return nil, apierr.BadRequest("cartão salvo está inativo")
		}
		if in.CreditCard == nil {
			in.CreditCard = &portin.CreditCardInput{}
		}
		in.CreditCard.TokenRef = savedCard.TokenRef
	}

	// ── Step 8: CreateOrGetCustomer ─────────────────────────────────────────────
	customerID, err := uc.getOrCreateCustomer(ctx, in.UserID, in.CPF)
	if err != nil {
		return nil, fmt.Errorf("process batch payment: customer: %w", err)
	}

	// ── Step 9: Fee snapshot ─────────────────────────────────────────────────────
	// Use the first installment's event ID for fee policy resolution.
	eventID := installments[0].EventID
	snapshot, err := uc.feePolicy.ResolveSnapshot(ctx, eventID)
	if err != nil {
		return nil, fmt.Errorf("process batch payment: fee policy: %w", err)
	}
	fees := domain.CalculateFees(totalAmountCents, snapshot)
	capturedAt := time.Now()

	// Collect ordered installment IDs for the transaction record.
	installmentIDs := make([]string, len(installments))
	for i, inst := range installments {
		installmentIDs[i] = inst.ID
	}

	// ── Step 10: Persist PENDING transaction ─────────────────────────────────────
	tx := &domain.PaymentTransaction{
		ID:                          uuid.New().String(),
		UserID:                      in.UserID,
		EventID:                     eventID,
		InstallmentIDs:              installmentIDs,
		AmountCents:                 totalAmountCents,
		Currency:                    "BRL",
		PaymentMethod:               in.Method,
		Provider:                    uc.providerName,
		Status:                      domain.TransactionStatusPending,
		IdempotencyKey:              in.IdempotencyKey,
		FeePolicySnapshotVersion:    snapshot.Version,
		FeePolicySnapshotCapturedAt: &capturedAt,
		CreatedAt:                   capturedAt,
		UpdatedAt:                   capturedAt,
	}

	if err := uc.txRepo.Save(ctx, tx); err != nil {
		return nil, fmt.Errorf("process batch payment: save transaction: %w", err)
	}

	// ── Step 11: SubledgerDualWrite ─────────────────────────────────────────────
	if subledgerErr := uc.subledger.AppendPaymentCommitment(ctx, tx, fees, snapshot); subledgerErr != nil {
		// Non-fatal: log and continue.
		slog.WarnContext(ctx, "process batch payment: subledger dual write failed (non-fatal)",
			"transactionID", tx.ID, "error", subledgerErr)
	}

	// ── Step 12: Credit card tokenisation ───────────────────────────────────────
	billingType := methodToBillingType(in.Method)
	dueDate := time.Now().AddDate(0, 0, 1).Format("2006-01-02")

	var cardToken string
	var cardLast4, cardBrand string

	if in.Method == domain.PaymentMethodCreditCard && in.CreditCard != nil {
		if in.CreditCard.TokenRef != "" {
			cardToken = in.CreditCard.TokenRef
		} else if in.CreditCard.Number != "" {
			tokenResult, tokenErr := uc.provider.TokenizeCreditCard(ctx, &portout.TokenizeCreditCardRequest{
				CustomerID:  customerID,
				HolderName:  in.CreditCard.HolderName,
				Number:      in.CreditCard.Number,
				ExpiryMonth: in.CreditCard.ExpiryMonth,
				ExpiryYear:  in.CreditCard.ExpiryYear,
				CVV:         in.CreditCard.CVV,
			})
			if tokenErr != nil {
				tx.MarkFailed("card tokenisation failed: " + tokenErr.Error())
				_ = uc.txRepo.Update(ctx, tx)
				return batchFailedResponse(tx, totalAmountCents, len(installments)), nil //nolint:nilerr
			}
			cardToken = tokenResult.Token
			cardLast4 = tokenResult.LastFour
			cardBrand = tokenResult.Brand
		}
	}

	// ── Step 13: Create payment at provider ─────────────────────────────────────
	providerReq := &portout.CreatePaymentRequest{
		CustomerID:        customerID,
		BillingType:       billingType,
		ValueCents:        totalAmountCents,
		DueDate:           dueDate,
		ExternalReference: tx.ID,
		Description:       "Pagamento em Lote Rolê Organizado",
		CreditCardToken:   cardToken,
	}

	providerPayment, providerErr := uc.provider.CreatePayment(ctx, providerReq)
	if providerErr != nil {
		tx.MarkFailed("provider error: " + providerErr.Error())
		_ = uc.txRepo.Update(ctx, tx)
		// Provider failure: NO installments altered.
		return batchFailedResponse(tx, totalAmountCents, len(installments)), nil //nolint:nilerr
	}

	tx.ProviderTransactionID = providerPayment.ID
	tx.Metadata.Provider = string(uc.providerName)
	tx.Metadata.BillingType = string(billingType)
	tx.Metadata.InvoiceUrl = providerPayment.InvoiceUrl
	tx.Metadata.BankSlipUrl = providerPayment.BankSlipUrl
	if cardLast4 != "" {
		tx.Metadata.CardLast4 = cardLast4
	}
	if cardBrand != "" {
		tx.Metadata.CardBrand = cardBrand
	}
	if cardToken != "" {
		tx.Metadata.TokenizedCard = cardToken
	}

	// ── Step 14: Method-specific data + installment state transitions ────────────
	now := time.Now()
	switch in.Method {
	case domain.PaymentMethodPix:
		qr, qrErr := uc.provider.GetPixQrCode(ctx, providerPayment.ID)
		if qrErr != nil {
			slog.WarnContext(ctx, "process batch payment: get pix qr code failed (non-fatal)",
				"providerID", providerPayment.ID, "error", qrErr)
		} else {
			tx.Metadata.PixQrCodeImage = qr.EncodedImage
			tx.Metadata.PixQrCodeText = qr.Payload
			expAt := qr.ExpirationDate
			tx.ExpiresAt = &expAt
			tx.Metadata.PixExpiresAt = &expAt
		}
		// PIX: installments stay PENDING — webhook (T007) will MarkPaidBatch on confirmation.

	case domain.PaymentMethodBoleto:
		field, fieldErr := uc.provider.GetBoletoIdentificationField(ctx, providerPayment.ID)
		if fieldErr != nil {
			slog.WarnContext(ctx, "process batch payment: get boleto identification field failed (non-fatal)",
				"providerID", providerPayment.ID, "error", fieldErr)
		} else {
			tx.Metadata.BoletoDigitableLine = field.IdentificationField
			tx.Metadata.BoletoCode = field.NossoNumero
			if providerPayment.DueDate != "" {
				if dueTime, parseErr := time.Parse("2006-01-02", providerPayment.DueDate); parseErr == nil {
					tx.Metadata.BoletoDueDate = &dueTime
					tx.ExpiresAt = &dueTime
				}
			}
		}
		// Boleto: installments stay PENDING — webhook (T007) will MarkPaidBatch on confirmation.

	case domain.PaymentMethodCreditCard:
		// CC charges may be confirmed immediately (no 3DS / instant approval).
		if providerPayment.Status == portout.ProviderStatusConfirmed ||
			providerPayment.Status == portout.ProviderStatusReceived {
			tx.MarkCompleted(now)
			// Atomic: mark all installments PAID in one DB operation.
			if markErr := uc.installmentRepo.MarkPaidBatch(
				ctx, installmentIDs, tx.ID, now,
				string(in.Method), providerPayment.ID,
			); markErr != nil {
				// Critical: the charge succeeded but we couldn't record it on the installments.
				// Log as error; the transaction is still COMPLETED for the customer.
				slog.ErrorContext(ctx, "process batch payment: MarkPaidBatch failed after CC completion — manual reconciliation required",
					"transactionID", tx.ID, "error", markErr)
			}
		}
	}

	// ── Step 15: Persist updated transaction ─────────────────────────────────────
	if updateErr := uc.txRepo.Update(ctx, tx); updateErr != nil {
		return nil, fmt.Errorf("process batch payment: update transaction: %w", updateErr)
	}

	// ── Step 16: Persist saved card (non-fatal) ──────────────────────────────────
	if in.SaveCard && cardToken != "" && in.Method == domain.PaymentMethodCreditCard && in.CreditCard != nil {
		expirationDate := ""
		if in.CreditCard.ExpiryMonth != "" && in.CreditCard.ExpiryYear != "" {
			expirationDate = in.CreditCard.ExpiryMonth + "/" + in.CreditCard.ExpiryYear
		}
		savedCard := &domain.SavedCreditCard{
			ID:             uuid.New().String(),
			UserID:         in.UserID,
			LastFourDigits: cardLast4,
			Brand:          domain.CardBrand(cardBrand),
			HolderName:     in.CreditCard.HolderName,
			ExpirationDate: expirationDate,
			TokenRef:       cardToken,
			IsActive:       true,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		if cardSaveErr := uc.savedCardRepo.Save(ctx, savedCard); cardSaveErr != nil {
			slog.WarnContext(ctx, "process batch payment: failed to save credit card token (non-fatal)",
				"transactionID", tx.ID, "error", cardSaveErr)
		}
	}

	slog.InfoContext(ctx, "process batch payment: completed",
		"transactionID", tx.ID,
		"installmentCount", len(installments),
		"totalAmountCents", totalAmountCents,
		"status", tx.Status,
		"method", tx.PaymentMethod)

	return batchSuccessResponse(tx, totalAmountCents, len(installments)), nil
}

// getOrCreateCustomer retrieves the cached Asaas customer ID for the user,
// or creates a new customer at Asaas and caches the link.
func (uc *ProcessBatchPayment) getOrCreateCustomer(ctx context.Context, userID, cpf string) (string, error) {
	link, err := uc.customerLinkRepo.FindByUserID(ctx, userID)
	if err == nil {
		return link.AsaasCustomerID, nil
	}
	if !apierr.IsNotFound(err) {
		return "", fmt.Errorf("find customer link: %w", err)
	}

	usuario, userErr := uc.usuarioRepo.FindByID(ctx, userID)
	if userErr != nil {
		return "", fmt.Errorf("find user: %w", userErr)
	}

	resolvedCPF := cpf
	if resolvedCPF == "" {
		resolvedCPF = usuario.CPF
	}

	customer, createErr := uc.provider.CreateOrGetCustomer(ctx, userID, usuario.Nome, resolvedCPF, usuario.Email)
	if createErr != nil {
		return "", fmt.Errorf("create asaas customer: %w", createErr)
	}

	now := time.Now()
	cacheLink := &domain.AsaasCustomerLink{
		UserID:          userID,
		AsaasCustomerID: customer.ID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if saveErr := uc.customerLinkRepo.Save(ctx, cacheLink); saveErr != nil {
		slog.WarnContext(ctx, "process batch payment: failed to cache customer link (non-fatal)",
			"userID", userID, "error", saveErr)
	}

	return customer.ID, nil
}

// ── Response helpers ──────────────────────────────────────────────────────────

func batchSuccessResponse(tx *domain.PaymentTransaction, totalAmountCents int64, count int) *portin.BatchPaymentResponse {
	return &portin.BatchPaymentResponse{
		Success:          true,
		ProcessedCount:   count,
		TotalAmountCents: totalAmountCents,
		TransactionID:    tx.ID,
		Message:          "Lote processado com sucesso",
	}
}

func batchFailedResponse(tx *domain.PaymentTransaction, totalAmountCents int64, _ int) *portin.BatchPaymentResponse {
	return &portin.BatchPaymentResponse{
		Success:          false,
		ProcessedCount:   0,
		TotalAmountCents: totalAmountCents,
		TransactionID:    tx.ID,
		Error:            tx.FailureReason,
		Message:          "Falha ao processar lote de pagamento",
	}
}

func batchResponseFromTx(tx *domain.PaymentTransaction) *portin.BatchPaymentResponse {
	success := tx.Status != domain.TransactionStatusFailed &&
		tx.Status != domain.TransactionStatusCancelled
	msg := "Requisição idempotente: retornando transação existente"
	if !success {
		msg = "Falha ao processar lote de pagamento"
	}
	return &portin.BatchPaymentResponse{
		Success:          success,
		ProcessedCount:   len(tx.InstallmentIDs),
		TotalAmountCents: tx.AmountCents,
		TransactionID:    tx.ID,
		Error:            tx.FailureReason,
		Message:          msg,
	}
}

// compile-time assertion.
var _ portin.ProcessBatchPaymentUseCase = (*ProcessBatchPayment)(nil)
