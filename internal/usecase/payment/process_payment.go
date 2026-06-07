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

// ProcessPayment implements portin.ProcessPaymentUseCase.
// It orchestrates the full charge lifecycle:
//  1. Idempotency check
//  2. Validation (amount >= R$1, CPF required for PIX/Boleto/CC)
//  3. CreateOrGetCustomer in Asaas (cached via AsaasCustomerLink)
//  4. ResolveFeePolicySnapshot + CalculateFees
//  5. Persist PaymentTransaction as PENDING
//  6. SubledgerDualWrite PAYMENT_COMMITMENT
//  7. CreatePayment at provider (tokenize CC if needed)
//  8. Fetch method-specific data (PIX QR code, Boleto identification field)
//  9. Update transaction with provider data; CC immediately confirmed → COMPLETED
//  10. Persist SavedCreditCard if SaveCard=true
type ProcessPayment struct {
	txRepo           portout.PaymentTransactionRepository
	customerLinkRepo portout.AsaasCustomerLinkRepository
	usuarioRepo      portout.UsuarioRepository
	savedCardRepo    portout.SavedCreditCardRepository
	provider         portout.PaymentProvider
	providerName     domain.PaymentProvider // ASAAS or MOCK
	feePolicy        FeePolicyResolver
	subledger        PaymentCommitmentWriter
}

// NewProcessPayment creates a new ProcessPayment use case.
func NewProcessPayment(
	txRepo portout.PaymentTransactionRepository,
	customerLinkRepo portout.AsaasCustomerLinkRepository,
	usuarioRepo portout.UsuarioRepository,
	savedCardRepo portout.SavedCreditCardRepository,
	provider portout.PaymentProvider,
	providerName domain.PaymentProvider,
	feePolicy FeePolicyResolver,
	subledger PaymentCommitmentWriter,
) *ProcessPayment {
	return &ProcessPayment{
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

// Execute processes a payment transaction.
func (uc *ProcessPayment) Execute(ctx context.Context, in portin.ProcessPaymentInput) (*domain.PaymentTransaction, error) {
	// ── Step 1: Idempotency ─────────────────────────────────────────────────────
	if in.IdempotencyKey != "" {
		existing, err := uc.txRepo.FindByIdempotencyKey(ctx, in.IdempotencyKey)
		if err == nil {
			slog.InfoContext(ctx, "process payment: idempotent request, returning existing",
				"idempotencyKey", in.IdempotencyKey, "transactionID", existing.ID)
			return existing, nil
		}
		if !apierr.IsNotFound(err) {
			return nil, fmt.Errorf("process payment: idempotency check: %w", err)
		}
	}

	// ── Step 2: Validation ──────────────────────────────────────────────────────
	if in.AmountCents < 100 {
		return nil, apierr.BadRequest("valor mínimo é R$1,00 (100 centavos)")
	}
	switch in.Method {
	case domain.PaymentMethodPix, domain.PaymentMethodBoleto, domain.PaymentMethodCreditCard:
		if in.CPF == "" {
			return nil, apierr.BadRequest("CPF é obrigatório para este método de pagamento")
		}
	}

	// ── Step 2b: Resolve saved card token ──────────────────────────────────────
	// If savedCardId is provided, look up the saved card and populate CreditCard.TokenRef.
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

	// ── Step 3: CreateOrGetCustomer ─────────────────────────────────────────────
	customerID, err := uc.getOrCreateCustomer(ctx, in.UserID, in.CPF)
	if err != nil {
		return nil, fmt.Errorf("process payment: customer: %w", err)
	}

	// ── Step 4: Fee snapshot ────────────────────────────────────────────────────
	snapshot, err := uc.feePolicy.ResolveSnapshot(ctx, in.EventID)
	if err != nil {
		return nil, fmt.Errorf("process payment: fee policy: %w", err)
	}
	fees := domain.CalculateFees(in.AmountCents, snapshot)
	capturedAt := time.Now()

	// ── Step 5: Persist PENDING transaction ─────────────────────────────────────
	tx := &domain.PaymentTransaction{
		ID:                          uuid.New().String(),
		UserID:                      in.UserID,
		EventID:                     in.EventID,
		InstallmentIDs:              in.InstallmentIDs,
		AmountCents:                 in.AmountCents,
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
		return nil, fmt.Errorf("process payment: save transaction: %w", err)
	}

	// ── Step 6: SubledgerDualWrite ──────────────────────────────────────────────
	if subledgerErr := uc.subledger.AppendPaymentCommitment(ctx, tx, fees, snapshot); subledgerErr != nil {
		// Non-fatal: subledger is best-effort for now; log and continue.
		slog.WarnContext(ctx, "process payment: subledger dual write failed (non-fatal)",
			"transactionID", tx.ID, "error", subledgerErr)
	}

	// ── Step 7: Create payment at provider ──────────────────────────────────────
	billingType := methodToBillingType(in.Method)
	dueDate := time.Now().AddDate(0, 0, 1).Format("2006-01-02")

	var cardToken string
	var cardLast4, cardBrand string

	if in.Method == domain.PaymentMethodCreditCard && in.CreditCard != nil {
		if in.CreditCard.TokenRef != "" {
			// Re-use a provider-tokenised saved card.
			cardToken = in.CreditCard.TokenRef
		} else if in.CreditCard.Number != "" {
			// Tokenise the raw card — never store PAN/CVV.
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
				return tx, nil //nolint:nilerr // failure is encoded in the transaction status
			}
			cardToken = tokenResult.Token
			cardLast4 = tokenResult.LastFour
			cardBrand = tokenResult.Brand
		}
	}

	providerReq := &portout.CreatePaymentRequest{
		CustomerID:        customerID,
		BillingType:       billingType,
		ValueCents:        in.AmountCents,
		DueDate:           dueDate,
		ExternalReference: tx.ID,
		Description:       "Pagamento Rolê Organizado",
		CreditCardToken:   cardToken,
	}

	providerPayment, providerErr := uc.provider.CreatePayment(ctx, providerReq)
	if providerErr != nil {
		tx.MarkFailed("provider error: " + providerErr.Error())
		_ = uc.txRepo.Update(ctx, tx)
		return tx, nil //nolint:nilerr // failure is encoded in the transaction status
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

	// ── Step 8: Method-specific data ────────────────────────────────────────────
	switch in.Method {
	case domain.PaymentMethodPix:
		qr, qrErr := uc.provider.GetPixQrCode(ctx, providerPayment.ID)
		if qrErr != nil {
			slog.WarnContext(ctx, "process payment: get pix qr code failed (non-fatal)",
				"providerID", providerPayment.ID, "error", qrErr)
		} else {
			tx.Metadata.PixQrCodeImage = qr.EncodedImage
			tx.Metadata.PixQrCodeText = qr.Payload
			expAt := qr.ExpirationDate
			tx.ExpiresAt = &expAt
			tx.Metadata.PixExpiresAt = &expAt
		}

	case domain.PaymentMethodBoleto:
		field, fieldErr := uc.provider.GetBoletoIdentificationField(ctx, providerPayment.ID)
		if fieldErr != nil {
			slog.WarnContext(ctx, "process payment: get boleto identification field failed (non-fatal)",
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

	case domain.PaymentMethodCreditCard:
		// CC charges may be confirmed immediately (no 3DS or instant approval).
		if providerPayment.Status == portout.ProviderStatusConfirmed ||
			providerPayment.Status == portout.ProviderStatusReceived {
			tx.MarkCompleted(time.Now())
		}
	}

	// ── Step 9: Persist updated transaction ─────────────────────────────────────
	if updateErr := uc.txRepo.Update(ctx, tx); updateErr != nil {
		return nil, fmt.Errorf("process payment: update transaction: %w", updateErr)
	}

	// ── Step 10: Persist saved card ─────────────────────────────────────────────
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
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		if cardSaveErr := uc.savedCardRepo.Save(ctx, savedCard); cardSaveErr != nil {
			// Non-fatal: card save failure should not fail the payment.
			slog.WarnContext(ctx, "process payment: failed to save credit card token (non-fatal)",
				"transactionID", tx.ID, "error", cardSaveErr)
		}
	}

	slog.InfoContext(ctx, "process payment: completed",
		"transactionID", tx.ID,
		"providerTransactionID", tx.ProviderTransactionID,
		"status", tx.Status,
		"method", tx.PaymentMethod)

	return tx, nil
}

// getOrCreateCustomer retrieves the cached Asaas customer ID for userID,
// or creates a new customer at Asaas and caches the link.
func (uc *ProcessPayment) getOrCreateCustomer(ctx context.Context, userID, cpf string) (string, error) {
	// Fast path: cached link.
	link, err := uc.customerLinkRepo.FindByUserID(ctx, userID)
	if err == nil {
		return link.AsaasCustomerID, nil
	}
	if !apierr.IsNotFound(err) {
		return "", fmt.Errorf("find customer link: %w", err)
	}

	// Resolve user profile for name and email.
	usuario, userErr := uc.usuarioRepo.FindByID(ctx, userID)
	if userErr != nil {
		return "", fmt.Errorf("find user: %w", userErr)
	}

	// CPF from the payment request takes precedence over the profile CPF.
	resolvedCPF := cpf
	if resolvedCPF == "" {
		resolvedCPF = usuario.CPF
	}

	customer, createErr := uc.provider.CreateOrGetCustomer(ctx, userID, usuario.Nome, resolvedCPF, usuario.Email)
	if createErr != nil {
		return "", fmt.Errorf("create asaas customer: %w", createErr)
	}

	// Cache the link — non-fatal on failure (next request will re-create).
	now := time.Now()
	cacheLink := &domain.AsaasCustomerLink{
		UserID:          userID,
		AsaasCustomerID: customer.ID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if saveErr := uc.customerLinkRepo.Save(ctx, cacheLink); saveErr != nil {
		slog.WarnContext(ctx, "process payment: failed to cache asaas customer link (non-fatal)",
			"userID", userID, "error", saveErr)
	}

	return customer.ID, nil
}

// methodToBillingType converts a domain.PaymentMethod to the provider billing type.
func methodToBillingType(method domain.PaymentMethod) portout.PaymentBillingType {
	switch method {
	case domain.PaymentMethodPix:
		return portout.BillingTypePix
	case domain.PaymentMethodBoleto:
		return portout.BillingTypeBoleto
	case domain.PaymentMethodCreditCard:
		return portout.BillingTypeCreditCard
	default:
		return portout.BillingTypePix
	}
}

// compile-time assertion: *ProcessPayment implements portin.ProcessPaymentUseCase.
var _ portin.ProcessPaymentUseCase = (*ProcessPayment)(nil)
