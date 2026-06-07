package payment

import (
	"context"
	"fmt"
	"time"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// managePaymentAccountsUseCase implements portin.ManagePaymentAccountsUseCase.
type managePaymentAccountsUseCase struct {
	repo portout.PaymentAccountRepository
}

// NewManagePaymentAccounts creates a new ManagePaymentAccountsUseCase.
func NewManagePaymentAccounts(repo portout.PaymentAccountRepository) portin.ManagePaymentAccountsUseCase {
	return &managePaymentAccountsUseCase{repo: repo}
}

// List returns active accounts for the user ordered by is_default desc, created_at asc.
func (uc *managePaymentAccountsUseCase) List(ctx context.Context, userID string) ([]*domain.PaymentAccount, error) {
	accounts, err := uc.repo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("listar contas de pagamento: %w", err)
	}
	return accounts, nil
}

// Create validates and persists a new payment account.
//
// Validation rules:
//   - PIX accounts require PixKeyType and PixKey.
//   - BANK_ACCOUNT accounts require AccountHolderName and AccountNumber.
//
// The first active account for a user is automatically set as default,
// regardless of the IsDefault field in the input.
func (uc *managePaymentAccountsUseCase) Create(ctx context.Context, in portin.CreatePaymentAccountInput) (*domain.PaymentAccount, error) {
	if in.AccountType == "" {
		return nil, apierr.BadRequest("accountType é obrigatório")
	}

	switch in.AccountType {
	case domain.AccountTypePix:
		if in.PixKeyType == "" || in.PixKey == "" {
			return nil, apierr.BadRequest("pixKeyType e pixKey são obrigatórios para contas PIX")
		}
	case domain.AccountTypeBankAccount:
		if in.AccountHolderName == "" || in.AccountNumber == "" {
			return nil, apierr.BadRequest("accountHolderName e accountNumber são obrigatórios para contas bancárias")
		}
	default:
		return nil, apierr.BadRequest(fmt.Sprintf("accountType inválido: %s", in.AccountType))
	}

	// First account for this user is automatically set as default.
	existing, err := uc.repo.FindByUserID(ctx, in.UserID)
	if err != nil {
		return nil, fmt.Errorf("verificar contas existentes: %w", err)
	}
	isDefault := len(existing) == 0

	now := time.Now()
	acct := &domain.PaymentAccount{
		UserID:                in.UserID,
		AccountType:           in.AccountType,
		PixKeyType:            in.PixKeyType,
		PixKey:                in.PixKey,
		BankCode:              in.BankCode,
		BankName:              in.BankName,
		Agency:                in.Agency,
		AccountNumber:         in.AccountNumber,
		AccountDigit:          in.AccountDigit,
		AccountHolderName:     in.AccountHolderName,
		AccountHolderDocument: in.AccountHolderDocument,
		IsDefault:             isDefault,
		IsActive:              true,
		CreatedAt:             now,
		UpdatedAt:             now,
	}

	if err := uc.repo.Save(ctx, acct); err != nil {
		return nil, fmt.Errorf("criar conta de pagamento: %w", err)
	}
	return acct, nil
}

// Update applies partial updates to an existing account.
// Returns 403 Forbidden if accountID does not belong to userID.
func (uc *managePaymentAccountsUseCase) Update(ctx context.Context, accountID, userID string, in portin.UpdatePaymentAccountInput) (*domain.PaymentAccount, error) {
	acct, err := uc.repo.FindByID(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("buscar conta para atualização: %w", err)
	}

	if acct.UserID != userID {
		return nil, apierr.Forbidden("acesso negado: conta não pertence ao usuário")
	}

	if in.PixKeyType != nil {
		acct.PixKeyType = *in.PixKeyType
	}
	if in.PixKey != nil {
		acct.PixKey = *in.PixKey
	}
	if in.BankCode != nil {
		acct.BankCode = *in.BankCode
	}
	if in.BankName != nil {
		acct.BankName = *in.BankName
	}
	if in.Agency != nil {
		acct.Agency = *in.Agency
	}
	if in.AccountNumber != nil {
		acct.AccountNumber = *in.AccountNumber
	}
	if in.AccountDigit != nil {
		acct.AccountDigit = *in.AccountDigit
	}
	if in.AccountHolderName != nil {
		acct.AccountHolderName = *in.AccountHolderName
	}
	if in.AccountHolderDocument != nil {
		acct.AccountHolderDocument = *in.AccountHolderDocument
	}
	acct.UpdatedAt = time.Now()

	if err := uc.repo.Update(ctx, acct); err != nil {
		return nil, fmt.Errorf("atualizar conta de pagamento: %w", err)
	}
	return acct, nil
}

// SetDefault atomically marks the given account as default and clears all others.
// Returns 403 Forbidden if accountID does not belong to userID.
func (uc *managePaymentAccountsUseCase) SetDefault(ctx context.Context, accountID, userID string) error {
	acct, err := uc.repo.FindByID(ctx, accountID)
	if err != nil {
		return fmt.Errorf("buscar conta para definir padrão: %w", err)
	}

	if acct.UserID != userID {
		return apierr.Forbidden("acesso negado: conta não pertence ao usuário")
	}

	if err := uc.repo.SetDefault(ctx, userID, accountID); err != nil {
		return fmt.Errorf("definir conta padrão: %w", err)
	}
	return nil
}

// Delete soft-deletes a payment account (sets is_active = false).
// Returns 403 Forbidden if accountID does not belong to userID.
func (uc *managePaymentAccountsUseCase) Delete(ctx context.Context, accountID, userID string) error {
	acct, err := uc.repo.FindByID(ctx, accountID)
	if err != nil {
		return fmt.Errorf("buscar conta para remoção: %w", err)
	}

	if acct.UserID != userID {
		return apierr.Forbidden("acesso negado: conta não pertence ao usuário")
	}

	if err := uc.repo.DeleteByID(ctx, accountID); err != nil {
		return fmt.Errorf("remover conta de pagamento: %w", err)
	}
	return nil
}
