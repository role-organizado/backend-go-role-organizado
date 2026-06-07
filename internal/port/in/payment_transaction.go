package in

import (
	"context"
	"time"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
)

// ─── Process Payment ────────────────────────────────────────────────────────

// CreditCardInput holds raw card details or a saved-card token for a charge.
type CreditCardInput struct {
	HolderName   string
	Number       string
	ExpiryMonth  string
	ExpiryYear   string
	CVV          string
	Installments int
	TokenRef     string // set when reusing a provider-tokenised saved card
}

// ProcessPaymentInput is the command for creating a single payment transaction.
type ProcessPaymentInput struct {
	UserID         string
	EventID        string
	AmountCents    int64
	Method         domain.PaymentMethod
	IdempotencyKey string
	CPF            string
	ClientIP       string
	CreditCard     *CreditCardInput // required when Method == CREDIT_CARD
	SaveCard       bool
	SavedCardID    string   // optional: ID of a previously saved credit card (sets CreditCard.TokenRef)
	InstallmentIDs []string // optional: links transaction to specific installments
}

// ProcessPaymentUseCase creates a charge via the payment provider for a single
// transaction, enforcing idempotency via IdempotencyKey.
type ProcessPaymentUseCase interface {
	Execute(ctx context.Context, in ProcessPaymentInput) (*domain.PaymentTransaction, error)
}

// ─── Batch Payment ──────────────────────────────────────────────────────────

// ProcessBatchPaymentInput is the command for charging multiple installments
// atomically (all-or-nothing semantics).
// InstallmentIDs are resolved to amounts by fetching the installment records;
// the caller does NOT supply amounts — they come from the DB.
type ProcessBatchPaymentInput struct {
	UserID         string
	InstallmentIDs []string // IDs of installments to pay; amounts fetched from DB
	Method         domain.PaymentMethod
	IdempotencyKey string
	CPF            string
	ClientIP       string
	CreditCard     *CreditCardInput // required when Method == CREDIT_CARD
	SaveCard       bool
	SavedCardID    string // optional: ID of a previously saved credit card
}

// BatchPaymentResponse is the result of an atomic batch payment operation.
// Field names mirror the Java BatchPaymentResponse exactly.
type BatchPaymentResponse struct {
	Success          bool   `json:"success"`
	ProcessedCount   int    `json:"processedCount"`
	TotalAmountCents int64  `json:"totalAmountCents"`
	TransactionID    string `json:"transactionId"`
	Error            string `json:"error,omitempty"`
	Message          string `json:"message"`
}

// ProcessBatchPaymentUseCase charges multiple installments in a single atomic
// operation. On failure all charges are rolled back and no installments are modified.
type ProcessBatchPaymentUseCase interface {
	Execute(ctx context.Context, in ProcessBatchPaymentInput) (*BatchPaymentResponse, error)
}

// ─── Get / List Transactions ────────────────────────────────────────────────

// GetPaymentTransactionUseCase retrieves a transaction by ID, enforcing that
// the requester is the transaction owner.
type GetPaymentTransactionUseCase interface {
	Execute(ctx context.Context, transactionID, requesterID string) (*domain.PaymentTransaction, error)
}

// ListUserPaymentsFilter holds optional query parameters for listing transactions.
type ListUserPaymentsFilter struct {
	Status   *domain.TransactionStatus
	EventoID string
	From     *time.Time
	To       *time.Time
}

// ListUserPaymentsUseCase lists payment transactions belonging to a user,
// with optional filtering by status, event, and date range.
type ListUserPaymentsUseCase interface {
	Execute(ctx context.Context, userID string, filter ListUserPaymentsFilter) ([]*domain.PaymentTransaction, error)
}

// ─── Webhook / Callback ─────────────────────────────────────────────────────

// PaymentCallbackPayload carries a normalised inbound webhook event from the
// payment provider. ProviderEventID is used for idempotency.
type PaymentCallbackPayload struct {
	Provider              domain.PaymentProvider
	ProviderEventID       string // unique webhook event ID from the provider
	ProviderTransactionID string
	EventType             string
	NewStatus             string
	RawPayload            []byte
}

// HandlePaymentCallbackUseCase processes an inbound webhook from the payment
// provider, updates the corresponding transaction, and publishes domain events.
type HandlePaymentCallbackUseCase interface {
	Execute(ctx context.Context, in PaymentCallbackPayload) error
}

// ─── Installments ───────────────────────────────────────────────────────────

// ListUserInstallmentsUseCase lists installments for the authenticated user,
// searching by both userId and participationIds (BUG5/spec-096 fix), and
// filtering out installments from events still in a planning phase
// (ORGANIZACAO / AGUARDANDO_ACEITE).
type ListUserInstallmentsUseCase interface {
	Execute(ctx context.Context, userID string, statusFilter *domain.InstallmentStatus) ([]*domain.PaymentInstallment, error)
}

// CancelParticipantInstallmentsInput identifies the participant whose open
// installments should be cancelled.
type CancelParticipantInstallmentsInput struct {
	EventID       string
	ParticipantID string
	RequesterID   string
}

// CancelParticipantInstallmentsUseCase cancels all non-terminal installments
// for a participant within an event. Returns the number of records cancelled.
type CancelParticipantInstallmentsUseCase interface {
	Execute(ctx context.Context, in CancelParticipantInstallmentsInput) (int64, error)
}

// ListInstallmentsFilter holds optional query parameters for listing installments.
type ListInstallmentsFilter struct {
	EventID string
	UserID  string
	Status  *domain.InstallmentStatus
}

// ListInstallmentsUseCase lists installments matching the given filter.
// requesterID is used for access-control checks.
type ListInstallmentsUseCase interface {
	Execute(ctx context.Context, requesterID string, filter ListInstallmentsFilter) ([]*domain.PaymentInstallment, error)
}

// GetInstallmentUseCase retrieves a single installment by ID, enforcing that
// the requester has access to the associated event.
type GetInstallmentUseCase interface {
	Execute(ctx context.Context, installmentID, requesterID string) (*domain.PaymentInstallment, error)
}

// ─── Payment Accounts (PIX / Bank) ─────────────────────────────────────────

// CreatePaymentAccountInput carries data to register a new payment account
// (PIX key or bank account) for a user.
type CreatePaymentAccountInput struct {
	UserID                string
	AccountType           domain.AccountType
	PixKeyType            domain.PixKeyType
	PixKey                string
	BankCode              string
	BankName              string
	Agency                string
	AccountNumber         string
	AccountDigit          string
	AccountHolderName     string
	AccountHolderDocument string
	IsDefault             bool
}

// UpdatePaymentAccountInput holds optional fields to update an existing account.
type UpdatePaymentAccountInput struct {
	PixKeyType            *domain.PixKeyType
	PixKey                *string
	BankCode              *string
	BankName              *string
	Agency                *string
	AccountNumber         *string
	AccountDigit          *string
	AccountHolderName     *string
	AccountHolderDocument *string
}

// ManagePaymentAccountsUseCase provides CRUD and default-setting operations
// over a user's registered payment accounts.
type ManagePaymentAccountsUseCase interface {
	List(ctx context.Context, userID string) ([]*domain.PaymentAccount, error)
	Create(ctx context.Context, in CreatePaymentAccountInput) (*domain.PaymentAccount, error)
	Update(ctx context.Context, id, userID string, in UpdatePaymentAccountInput) (*domain.PaymentAccount, error)
	SetDefault(ctx context.Context, id, userID string) error
	Delete(ctx context.Context, id, userID string) error
}

// ─── PIX Key Validation ─────────────────────────────────────────────────────

// ValidatePixKeyResult is returned by the provider after validating a PIX key.
type ValidatePixKeyResult struct {
	Valid      bool
	HolderName string
	KeyType    string
	Key        string
}

// ValidatePixKeyUseCase validates a PIX key against the payment provider and
// returns account holder information when the key is valid.
type ValidatePixKeyUseCase interface {
	Execute(ctx context.Context, userID, pixKey string) (*ValidatePixKeyResult, error)
}

// ─── Fee Policy Snapshot ────────────────────────────────────────────────────

// ReaplicarFeePolicySnapshotInput identifies the transaction to re-snapshot.
type ReaplicarFeePolicySnapshotInput struct {
	TransactionID string
	RequesterID   string
}

// ReaplicarFeePolicySnapshotUseCase reaplicates the current fee policy snapshot
// onto an existing transaction (e.g. after an admin policy correction).
type ReaplicarFeePolicySnapshotUseCase interface {
	Execute(ctx context.Context, in ReaplicarFeePolicySnapshotInput) error
}

// ─── Expiration ─────────────────────────────────────────────────────────────

// ExpireTransactionUseCase marks a PENDING/PROCESSING payment transaction as
// CANCELLED with failure reason TIMEOUT. Called by the PaymentExpirationWorkflow
// activity after the expiry timer fires. Returns an error whose message is
// "transaction already terminal" when the transaction is already in a final state
// (safe to convert to Temporal non-retryable error).
type ExpireTransactionUseCase interface {
	Execute(ctx context.Context, transactionID string) error
}

// ─── PSP Reconciliation ─────────────────────────────────────────────────────

// ReconcileFilter specifies the time window for a PSP reconciliation run.
type ReconcileFilter struct {
	From time.Time
	To   time.Time
}

// ReconcileResult summarises the outcome of a reconciliation pass.
type ReconcileResult struct {
	Checked int64
	Updated int64
	Failed  int64
}

// ReconcilePspTransactionsUseCase reconciles local transaction states against
// the PSP, correcting any divergence. Typically invoked by a Temporal workflow.
type ReconcilePspTransactionsUseCase interface {
	Execute(ctx context.Context, filter ReconcileFilter) (*ReconcileResult, error)
}
