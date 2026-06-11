package out

import (
	"context"
	"errors"
	"time"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
)

// ErrAlreadyProcessed is returned by ProcessedWebhookEventRepository.SaveUnique when
// the (provider, eventID) pair has already been recorded — the caller should treat
// this as a no-op, not an error.
var ErrAlreadyProcessed = errors.New("webhook event already processed")

// TransactionFilter holds optional query parameters for paginated transaction
// listing. Zero values are ignored (no filter applied for that field).
type TransactionFilter struct {
	Status   *domain.TransactionStatus
	EventoID string
	From     *time.Time
	To       *time.Time
	Page     int
	PageSize int
}

// PaymentTransactionRepository is the persistence contract for PaymentTransaction.
type PaymentTransactionRepository interface {
	// Save persists a new transaction. ID must be pre-populated by the caller.
	Save(ctx context.Context, tx *domain.PaymentTransaction) error
	// Update replaces an existing transaction document.
	Update(ctx context.Context, tx *domain.PaymentTransaction) error
	// FindByID retrieves a transaction by its platform ID.
	FindByID(ctx context.Context, id string) (*domain.PaymentTransaction, error)
	// FindByIdempotencyKey looks up a transaction by its idempotency key.
	FindByIdempotencyKey(ctx context.Context, key string) (*domain.PaymentTransaction, error)
	// FindByProviderTransactionID looks up a transaction by the provider's ID.
	FindByProviderTransactionID(ctx context.Context, providerTxID string) (*domain.PaymentTransaction, error)
	// FindByUserID returns paginated transactions for a user with optional filters.
	// Returns the matching slice, the total count before pagination, and any error.
	FindByUserID(ctx context.Context, userID string, filter TransactionFilter) ([]*domain.PaymentTransaction, int64, error)
	// FindPendingOlderThan returns PENDING/PROCESSING transactions created before
	// threshold, used by expiration and reconciliation workflows.
	FindPendingOlderThan(ctx context.Context, threshold time.Time) ([]*domain.PaymentTransaction, error)
	// FindByEventID returns all transactions for the given event regardless of
	// status. Used by alterarFase and gerenciar to detect COMPLETED payments.
	FindByEventID(ctx context.Context, eventID string) ([]*domain.PaymentTransaction, error)
}

// PaymentInstallmentRepository is the persistence contract for PaymentInstallment.
type PaymentInstallmentRepository interface {
	// FindByID retrieves a single installment by its platform ID.
	FindByID(ctx context.Context, id string) (*domain.PaymentInstallment, error)
	// FindByEventAndParticipant returns all installments for a participant in an event.
	FindByEventAndParticipant(ctx context.Context, eventID, participantID string) ([]*domain.PaymentInstallment, error)
	// FindByUserOrParticipations retrieves installments owned by the user directly
	// or via any of the given participation IDs. An optional status filter is applied
	// when non-nil.
	FindByUserOrParticipations(ctx context.Context, userID string, participationIDs []string, statusFilter *domain.InstallmentStatus) ([]*domain.PaymentInstallment, error)
	// FindByIDs fetches multiple installments by their IDs in a single query.
	FindByIDs(ctx context.Context, ids []string) ([]*domain.PaymentInstallment, error)
	// MarkPaidBatch atomically marks a set of installments as PAID, associating
	// them with the given transaction ID and recording the payment details.
	MarkPaidBatch(ctx context.Context, ids []string, txID string, paidAt time.Time, method, reference string) error
	// FindByEvent returns all installments for an event with an optional status
	// filter, ordered by due_date ascending. Used by ListInstallments when no
	// specific userId filter is applied.
	FindByEvent(ctx context.Context, eventID string, statusFilter *domain.InstallmentStatus) ([]*domain.PaymentInstallment, error)
	// CancelByParticipant cancels all non-terminal installments for a participant
	// within an event. Returns the number of records updated.
	CancelByParticipant(ctx context.Context, eventID, participantID string) (int64, error)
}

// PaymentAccountRepository is the persistence contract for PaymentAccount.
type PaymentAccountRepository interface {
	FindByID(ctx context.Context, id string) (*domain.PaymentAccount, error)
	FindByUserID(ctx context.Context, userID string) ([]*domain.PaymentAccount, error)
	FindDefaultByUserID(ctx context.Context, userID string) (*domain.PaymentAccount, error)
	Save(ctx context.Context, acct *domain.PaymentAccount) error
	Update(ctx context.Context, acct *domain.PaymentAccount) error
	// SetDefault atomically clears is_default on all accounts of the user and sets
	// it on the target account. Callers use this instead of two separate Updates.
	SetDefault(ctx context.Context, userID, accountID string) error
	// DeleteByID performs a soft delete (sets is_active = false).
	DeleteByID(ctx context.Context, id string) error
}

// SavedCreditCardRepository is the persistence contract for SavedCreditCard.
type SavedCreditCardRepository interface {
	FindByID(ctx context.Context, id string) (*domain.SavedCreditCard, error)
	FindByUserID(ctx context.Context, userID string) ([]*domain.SavedCreditCard, error)
	FindDefaultByUserID(ctx context.Context, userID string) (*domain.SavedCreditCard, error)
	Save(ctx context.Context, card *domain.SavedCreditCard) error
	Update(ctx context.Context, card *domain.SavedCreditCard) error
	// SetDefault atomically clears is_default on all cards of the user and sets
	// it on the target card.
	SetDefault(ctx context.Context, userID, cardID string) error
	// DeleteByID performs a soft delete (sets is_active = false).
	DeleteByID(ctx context.Context, id string) error
}

// ProcessedWebhookEventRepository prevents double-processing of provider webhooks
// by persisting a record of every handled event.
type ProcessedWebhookEventRepository interface {
	// ExistsByProviderAndEventID returns true if the event has already been processed.
	ExistsByProviderAndEventID(ctx context.Context, provider, eventID string) (bool, error)
	// SaveUnique persists the event record. Implementations must enforce uniqueness
	// on (provider, eventID) and return an error on duplicate.
	SaveUnique(ctx context.Context, e *domain.ProcessedWebhookEvent) error
}

// AsaasCustomerLinkRepository maps platform users to Asaas customer identifiers.
type AsaasCustomerLinkRepository interface {
	FindByUserID(ctx context.Context, userID string) (*domain.AsaasCustomerLink, error)
	Save(ctx context.Context, link *domain.AsaasCustomerLink) error
	Update(ctx context.Context, link *domain.AsaasCustomerLink) error
}
