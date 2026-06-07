package payment

import "time"

// PaymentMethod represents the payment method for a real transaction.
type PaymentMethod string

const (
	PaymentMethodPix        PaymentMethod = "PIX"
	PaymentMethodBoleto     PaymentMethod = "BOLETO"
	PaymentMethodCreditCard PaymentMethod = "CREDIT_CARD"
)

// PaymentProvider identifies the payment service provider.
type PaymentProvider string

const (
	PaymentProviderAsaas PaymentProvider = "ASAAS"
	PaymentProviderMock  PaymentProvider = "MOCK"
)

// TransactionStatus represents the lifecycle state of a payment transaction.
type TransactionStatus string

const (
	TransactionStatusPending    TransactionStatus = "PENDING"
	TransactionStatusProcessing TransactionStatus = "PROCESSING"
	TransactionStatusCompleted  TransactionStatus = "COMPLETED"
	TransactionStatusFailed     TransactionStatus = "FAILED"
	TransactionStatusCancelled  TransactionStatus = "CANCELLED"
)

// PaymentMetadata holds provider-specific data returned after a charge is created.
type PaymentMetadata struct {
	// PIX-specific
	PixQrCodeImage string
	PixQrCodeText  string
	PixKey         string
	PixExpiresAt   *time.Time

	// Boleto-specific
	BoletoCode          string
	BoletoDigitableLine string
	BoletoPdfUrl        string
	BoletoDueDate       *time.Time

	// Credit card-specific
	CardLast4              string
	CardBrand              string
	CreditCardInstallments int
	InstallmentAmountCents int64
	TokenizedCard          string

	// Common
	InvoiceUrl  string
	BankSlipUrl string
	Provider    string
	BillingType string
}

// PaymentTransaction is the central entity for real payment processing via Asaas.
// All monetary amounts are stored as int64 cents to avoid floating-point errors.
type PaymentTransaction struct {
	ID                          string
	UserID                      string
	EventID                     string
	InstallmentIDs              []string
	AmountCents                 int64
	Currency                    string
	PaymentMethod               PaymentMethod
	Provider                    PaymentProvider
	Status                      TransactionStatus
	IdempotencyKey              string
	ProviderTransactionID       string
	Metadata                    PaymentMetadata
	FeePolicySnapshotVersion    string
	FeePolicySnapshotCapturedAt *time.Time
	CreatedAt                   time.Time
	UpdatedAt                   time.Time
	CompletedAt                 *time.Time
	ExpiresAt                   *time.Time
	FailureReason               string
}

// IsOwner returns true if userID is the transaction owner.
func (t *PaymentTransaction) IsOwner(userID string) bool {
	return t.UserID == userID
}

// IsTerminal returns true if the transaction has reached a final, immutable state.
func (t *PaymentTransaction) IsTerminal() bool {
	return t.Status == TransactionStatusCompleted ||
		t.Status == TransactionStatusFailed ||
		t.Status == TransactionStatusCancelled
}

// MarkCompleted transitions the transaction to COMPLETED state at the given time.
func (t *PaymentTransaction) MarkCompleted(at time.Time) {
	t.Status = TransactionStatusCompleted
	t.CompletedAt = &at
	t.UpdatedAt = at
}

// MarkFailed transitions the transaction to FAILED state with a descriptive reason.
func (t *PaymentTransaction) MarkFailed(reason string) {
	t.Status = TransactionStatusFailed
	t.FailureReason = reason
	t.UpdatedAt = time.Now()
}
