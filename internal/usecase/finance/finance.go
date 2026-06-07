// Package finance implements the 9 finance use cases following the hexagonal architecture.
// Each UC is a struct with injected output-port dependencies and an Execute method.
// No mongo/bson imports — all I/O is via interfaces defined in port/out.
package finance

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/finance"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ---- UC 1: ListFinanceEvents ----

// ListFinanceEvents lists finance overviews for all events where the user is an active participant.
// It excludes events where the participation status is ORGANIZACAO or AGUARDANDO_ACEITE.
type ListFinanceEvents struct {
	participants portout.ParticipantRepository
	eventos      portout.EventoRepository
	rateios      portout.RateioRepository
	summaries    portout.FinanceSummaryRepository
}

// NewListFinanceEvents creates a new ListFinanceEvents use case.
func NewListFinanceEvents(
	participants portout.ParticipantRepository,
	eventos portout.EventoRepository,
	rateios portout.RateioRepository,
	summaries portout.FinanceSummaryRepository,
) *ListFinanceEvents {
	return &ListFinanceEvents{
		participants: participants,
		eventos:      eventos,
		rateios:      rateios,
		summaries:    summaries,
	}
}

// eventEntry is a local helper for sorting before returning FinanceOverview slices.
type eventEntry struct {
	overview  domain.FinanceOverview
	eventDate time.Time
}

// Execute returns finance overviews for all events where the user is an active participant,
// ordered by event date descending.
func (uc *ListFinanceEvents) Execute(ctx context.Context, in portin.ListFinanceEventsInput) ([]domain.FinanceOverview, error) {
	participations, err := uc.participants.FindByUserID(ctx, in.UserID)
	if err != nil {
		return nil, fmt.Errorf("buscar participações do usuário: %w", err)
	}

	var entries []eventEntry
	for _, p := range participations {
		// Excluir fases de organização e convite pendente
		if p.Status == "ORGANIZACAO" || p.Status == "AGUARDANDO_ACEITE" {
			continue
		}

		evt, err := uc.eventos.FindByID(ctx, p.EventID)
		if err != nil {
			// evento inacessível: pular silenciosamente
			continue
		}

		// goal = Σ rateios com status ABERTO (ativo) em centavos
		rateioList, _ := uc.rateios.FindByEventoID(ctx, p.EventID)
		var goal int64
		for _, r := range rateioList {
			if r.Status == "ABERTO" { // StatusRateioAberto
				goal += int64(r.ValorTotal * 100)
			}
		}

		// collected vem do finance_summary
		var collected int64
		if summary, err := uc.summaries.FindByEventID(ctx, p.EventID); err == nil && summary != nil {
			collected = summary.Collected
		}

		overview := domain.FinanceOverview{
			EventID:            p.EventID,
			EventName:          evt.Nome,
			Goal:               goal,
			Collected:          collected,
			ProgressPercentage: domain.CalculateProgress(collected, goal),
		}
		if s, err2 := uc.summaries.FindByEventID(ctx, p.EventID); err2 == nil && s != nil {
			overview.AvailableForWithdrawal = s.AvailableForWithdrawal
			overview.PendingWithdrawals = s.PendingWithdrawals
		}

		entries = append(entries, eventEntry{overview: overview, eventDate: evt.Data})
	}

	// Ordenar por data do evento desc
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].eventDate.After(entries[j].eventDate)
	})

	overviews := make([]domain.FinanceOverview, len(entries))
	for i, e := range entries {
		overviews[i] = e.overview
	}
	return overviews, nil
}

// ---- UC 2: GetFinanceOverview ----

// GetFinanceOverview returns the detailed finance overview for a single event.
// Returns Forbidden if the caller is not a participant of the event.
type GetFinanceOverview struct {
	eventos      portout.EventoRepository
	participants portout.ParticipantRepository
	rateios      portout.RateioRepository
	summaries    portout.FinanceSummaryRepository
}

// NewGetFinanceOverview creates a new GetFinanceOverview use case.
func NewGetFinanceOverview(
	eventos portout.EventoRepository,
	participants portout.ParticipantRepository,
	rateios portout.RateioRepository,
	summaries portout.FinanceSummaryRepository,
) *GetFinanceOverview {
	return &GetFinanceOverview{
		eventos:      eventos,
		participants: participants,
		rateios:      rateios,
		summaries:    summaries,
	}
}

// Execute validates access and returns the finance overview for the event.
func (uc *GetFinanceOverview) Execute(ctx context.Context, in portin.GetFinanceOverviewInput) (*domain.FinanceOverview, error) {
	// Validar que o usuário é participante do evento
	_, err := uc.participants.FindByEventIDAndUserID(ctx, in.EventID, in.UserID)
	if err != nil {
		return nil, apierr.Forbidden("acesso negado ao overview financeiro do evento")
	}

	evt, err := uc.eventos.FindByID(ctx, in.EventID)
	if err != nil {
		return nil, fmt.Errorf("buscar evento: %w", err)
	}

	// goal = Σ rateios aprovados (status ABERTO/ATIVO) em centavos
	rateioList, _ := uc.rateios.FindByEventoID(ctx, in.EventID)
	var goal int64
	for _, r := range rateioList {
		if r.Status == "ABERTO" {
			goal += int64(r.ValorTotal * 100)
		}
	}

	// collected e pendingWithdrawals vêm do finance_summary
	var collected, pendingWithdrawals int64
	if summary, err := uc.summaries.FindByEventID(ctx, in.EventID); err == nil && summary != nil {
		collected = summary.Collected
		pendingWithdrawals = summary.PendingWithdrawals
	}

	availableForWithdrawal := max(int64(0), collected-pendingWithdrawals)
	progressPercentage := domain.CalculateProgress(collected, goal)

	return &domain.FinanceOverview{
		EventID:                in.EventID,
		EventName:              evt.Nome,
		Goal:                   goal,
		Collected:              collected,
		ProgressPercentage:     progressPercentage,
		AvailableForWithdrawal: availableForWithdrawal,
		PendingWithdrawals:     pendingWithdrawals,
	}, nil
}

// ---- UC 3: GetLedgerStatement ----

// GetLedgerStatement returns a paginated, optionally filtered ledger statement for an event.
type GetLedgerStatement struct {
	ledger       portout.LedgerEntryRepository
	participants portout.ParticipantRepository
}

// NewGetLedgerStatement creates a new GetLedgerStatement use case.
func NewGetLedgerStatement(
	ledger portout.LedgerEntryRepository,
	participants portout.ParticipantRepository,
) *GetLedgerStatement {
	return &GetLedgerStatement{ledger: ledger, participants: participants}
}

// Execute validates access and returns a paginated ledger for the event.
// Input normalization: type "all" → nil filter; size clamped to [1,100]; page clamped to >= 0.
func (uc *GetLedgerStatement) Execute(ctx context.Context, in portin.GetLedgerStatementInput) (*domain.LedgerStatementPage, error) {
	// Validar acesso
	if _, err := uc.participants.FindByEventIDAndUserID(ctx, in.EventID, in.UserID); err != nil {
		return nil, apierr.Forbidden("acesso negado ao extrato do evento")
	}

	// Normalizar type
	var entryType *string
	if t := strings.ToUpper(strings.TrimSpace(in.Type)); t != "" && t != "ALL" {
		entryType = &t
	}

	// Clamp pagination
	size := max(1, min(100, in.Size))
	page := max(0, in.Page)

	entries, total, err := uc.ledger.FindByEventID(ctx, in.EventID, entryType, in.From, in.To, page, size)
	if err != nil {
		return nil, fmt.Errorf("buscar extrato do evento: %w", err)
	}

	totalPages := 0
	if total > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(size)))
	}

	return &domain.LedgerStatementPage{
		Entries:       entries,
		TotalElements: total,
		TotalPages:    totalPages,
		Page:          page,
		Size:          size,
	}, nil
}

// ---- UC 4: GetParticipantsStatus ----

// GetParticipantsStatus returns the payment status for each participant of an event.
type GetParticipantsStatus struct {
	participants portout.ParticipantRepository
	installments portout.FinanceInstallmentRepository
}

// NewGetParticipantsStatus creates a new GetParticipantsStatus use case.
func NewGetParticipantsStatus(
	participants portout.ParticipantRepository,
	installments portout.FinanceInstallmentRepository,
) *GetParticipantsStatus {
	return &GetParticipantsStatus{participants: participants, installments: installments}
}

// Execute returns paged participant payment statuses and the total count.
func (uc *GetParticipantsStatus) Execute(ctx context.Context, in portin.GetParticipantsStatusInput) ([]domain.ParticipantPaymentStatus, int64, error) {
	size := max(1, min(100, in.Size))
	page := max(0, in.Page)

	participants, total, err := uc.participants.FindByEventID(ctx, in.EventID, page, size)
	if err != nil {
		return nil, 0, fmt.Errorf("buscar participantes: %w", err)
	}

	statuses := make([]domain.ParticipantPaymentStatus, 0, len(participants))
	for _, p := range participants {
		insts, _ := uc.installments.FindByParticipantID(ctx, in.EventID, p.ID)

		var paid, pending int64
		hasOverdue := false
		allPaid := len(insts) > 0

		for _, inst := range insts {
			switch inst.Status {
			case "PAID":
				paid += inst.Amount
			case "PENDING":
				pending += inst.Amount
				allPaid = false
			case "OVERDUE":
				pending += inst.Amount
				hasOverdue = true
				allPaid = false
			}
		}

		participantStatus := "PENDING"
		switch {
		case hasOverdue:
			participantStatus = "OVERDUE"
		case allPaid && len(insts) > 0:
			participantStatus = "PAID"
		}

		statuses = append(statuses, domain.ParticipantPaymentStatus{
			ParticipantID: p.ID,
			Name:          p.Name,
			Email:         p.Email,
			Status:        participantStatus,
			PaidAmount:    paid,
			PendingAmount: pending,
		})
	}

	return statuses, total, nil
}

// ---- UC 5: RecalculateFinanceSummary ----

// RecalculateFinanceSummary recalculates and persists the finance summary for an event.
// It is idempotent: it finds or creates the summary before updating.
type RecalculateFinanceSummary struct {
	summaries    portout.FinanceSummaryRepository
	rateios      portout.RateioRepository
	installments portout.FinanceInstallmentRepository
}

// NewRecalculateFinanceSummary creates a new RecalculateFinanceSummary use case.
func NewRecalculateFinanceSummary(
	summaries portout.FinanceSummaryRepository,
	rateios portout.RateioRepository,
	installments portout.FinanceInstallmentRepository,
) *RecalculateFinanceSummary {
	return &RecalculateFinanceSummary{
		summaries:    summaries,
		rateios:      rateios,
		installments: installments,
	}
}

// Execute recalculates goal and collected for the event and persists the result.
func (uc *RecalculateFinanceSummary) Execute(ctx context.Context, in portin.RecalculateFinanceSummaryInput) (*domain.FinanceSummary, error) {
	// goal = Σ rateios ABERTO em centavos
	rateioList, _ := uc.rateios.FindByEventoID(ctx, in.EventID)
	var goal int64
	for _, r := range rateioList {
		if r.Status == "ABERTO" {
			goal += int64(r.ValorTotal * 100)
		}
	}

	// collected = Σ installments PAID
	instList, _ := uc.installments.FindByEventID(ctx, in.EventID)
	var collected int64
	for _, inst := range instList {
		if inst.Status == "PAID" {
			collected += inst.Amount
		}
	}

	progress := domain.CalculateProgress(collected, goal)
	now := time.Now()

	// Idempotente: buscar summary existente ou criar novo
	existing, err := uc.summaries.FindByEventID(ctx, in.EventID)
	if err != nil || existing == nil {
		// Criar novo summary
		s := &domain.FinanceSummary{
			EventID:            in.EventID,
			Goal:               goal,
			Collected:          collected,
			ProgressPercentage: progress,
			LastCalculatedAt:   now,
		}
		saved, err := uc.summaries.Save(ctx, s)
		if err != nil {
			return nil, fmt.Errorf("salvar finance summary: %w", err)
		}
		return saved, nil
	}

	// Atualizar summary existente
	existing.Goal = goal
	existing.Collected = collected
	existing.ProgressPercentage = progress
	existing.LastCalculatedAt = now

	updated, err := uc.summaries.Update(ctx, existing)
	if err != nil {
		return nil, fmt.Errorf("atualizar finance summary: %w", err)
	}
	return updated, nil
}

// ---- UC 6: SendPaymentReminders ----

// PaymentReminderDispatcher abstracts how payment reminders are dispatched.
// In production this is backed by a Temporal workflow signal; in tests it can be stubbed.
type PaymentReminderDispatcher interface {
	Dispatch(ctx context.Context, eventID, participantID string) error
}

// SendPaymentReminders dispatches payment reminders to participants with pending/overdue installments.
type SendPaymentReminders struct {
	participants portout.ParticipantRepository
	installments portout.FinanceInstallmentRepository
	dispatcher   PaymentReminderDispatcher // nil → log only (Temporal unavailable)
}

// NewSendPaymentReminders creates a new SendPaymentReminders use case.
// dispatcher may be nil — in that case reminders are logged but not dispatched.
func NewSendPaymentReminders(
	participants portout.ParticipantRepository,
	installments portout.FinanceInstallmentRepository,
	dispatcher PaymentReminderDispatcher,
) *SendPaymentReminders {
	return &SendPaymentReminders{
		participants: participants,
		installments: installments,
		dispatcher:   dispatcher,
	}
}

// Execute sends payment reminders to all participants with PENDING or OVERDUE installments.
func (uc *SendPaymentReminders) Execute(ctx context.Context, in portin.SendPaymentRemindersInput) error {
	pending, err := uc.installments.FindPendingByEventID(ctx, in.EventID)
	if err != nil {
		return fmt.Errorf("buscar parcelas pendentes: %w", err)
	}

	// Deduplica por participantID para não enviar múltiplos lembretes
	seen := make(map[string]bool)
	for _, inst := range pending {
		if seen[inst.ParticipantID] {
			continue
		}
		seen[inst.ParticipantID] = true

		if uc.dispatcher == nil {
			// Temporal não configurado — apenas loga
			continue
		}
		if err := uc.dispatcher.Dispatch(ctx, in.EventID, inst.ParticipantID); err != nil {
			return fmt.Errorf("enviar lembrete para participante %s: %w", inst.ParticipantID, err)
		}
	}
	return nil
}

// ---- UC 7: CalculateHoldBalance ----

// defaultHoldDays defines the hold period in days by payment method (fallback hardcoded).
var defaultHoldDays = map[string]int{
	"PIX":         0,
	"BOLETO":      1,
	"CREDIT_CARD": 30,
}

// CalculateHoldBalance calculates the blocked and available balance breakdown for an event.
type CalculateHoldBalance struct {
	installments portout.FinanceInstallmentRepository
	configRepo   portout.ConfiguracaoSistemaRepository
}

// NewCalculateHoldBalance creates a new CalculateHoldBalance use case.
func NewCalculateHoldBalance(
	installments portout.FinanceInstallmentRepository,
	configRepo portout.ConfiguracaoSistemaRepository,
) *CalculateHoldBalance {
	return &CalculateHoldBalance{installments: installments, configRepo: configRepo}
}

// Execute computes blocked vs available balance using hold periods per payment method.
func (uc *CalculateHoldBalance) Execute(ctx context.Context, in portin.CalculateHoldBalanceInput) (*domain.HoldBalance, error) {
	// Carregar hold settings do config sistema (fallback para defaults)
	holdDays := make(map[string]int)
	for k, v := range defaultHoldDays {
		holdDays[k] = v
	}
	if cfg, err := uc.configRepo.FindByChave(ctx, "PAYMENT_SETTINGS"); err == nil && cfg != nil {
		if hs, ok := cfg.Valor["holdSettings"].(map[string]any); ok {
			for method, v := range hs {
				if days, ok := v.(float64); ok {
					holdDays[strings.ToUpper(method)] = int(days)
				}
			}
		}
	}

	// Buscar todos os installments do evento
	allInsts, err := uc.installments.FindByEventID(ctx, in.EventID)
	if err != nil {
		return nil, fmt.Errorf("buscar parcelas do evento: %w", err)
	}

	now := time.Now()
	var blockedBalance int64
	var nextReleaseDate *time.Time
	breakdown := map[string]int64{"PIX": 0, "BOLETO": 0, "CREDIT_CARD": 0}
	var collected int64

	for _, inst := range allInsts {
		if inst.Status != "PAID" || inst.PaidAt == nil {
			continue
		}
		collected += inst.Amount

		method := strings.ToUpper(inst.PaymentMethod)
		days := holdDays[method] // defaults to 0 for unknown methods

		releaseDate := inst.PaidAt.Add(time.Duration(days) * 24 * time.Hour)
		if releaseDate.After(now) {
			// Still within hold period — blocked
			blockedBalance += inst.Amount
			breakdown[method] += inst.Amount

			if nextReleaseDate == nil || releaseDate.Before(*nextReleaseDate) {
				rd := releaseDate
				nextReleaseDate = &rd
			}
		}
	}

	availableForOutbound := max(int64(0), collected-blockedBalance)

	return &domain.HoldBalance{
		BlockedBalance:       blockedBalance,
		AvailableForOutbound: availableForOutbound,
		NextReleaseDate:      nextReleaseDate,
		BreakdownByMethod:    breakdown,
	}, nil
}

// ---- UC 8: GetEventPaymentStatus ----

// GetEventPaymentStatus aggregates installment counts and amounts by status for an event.
type GetEventPaymentStatus struct {
	installments portout.FinanceInstallmentRepository
	participants portout.ParticipantRepository
}

// NewGetEventPaymentStatus creates a new GetEventPaymentStatus use case.
func NewGetEventPaymentStatus(
	installments portout.FinanceInstallmentRepository,
	participants portout.ParticipantRepository,
) *GetEventPaymentStatus {
	return &GetEventPaymentStatus{installments: installments, participants: participants}
}

// Execute returns aggregated payment totals and counts for the event.
func (uc *GetEventPaymentStatus) Execute(ctx context.Context, in portin.GetEventPaymentStatusInput) (*domain.EventPaymentStatus, error) {
	allInsts, err := uc.installments.FindByEventID(ctx, in.EventID)
	if err != nil {
		return nil, fmt.Errorf("buscar parcelas: %w", err)
	}

	status := &domain.EventPaymentStatus{EventID: in.EventID}
	for _, inst := range allInsts {
		switch inst.Status {
		case "PAID":
			status.TotalPaid += inst.Amount
			status.PaidCount++
		case "PENDING":
			status.TotalPending += inst.Amount
			status.PendingCount++
		case "OVERDUE":
			status.TotalOverdue += inst.Amount
			status.PendingCount++ // OVERDUE conta como pendente no total de contagem
		}
	}
	return status, nil
}

// ---- UC 9: ManagePaymentAccounts ----

// ManagePaymentAccounts handles CRUD operations for user PIX/bank payment accounts.
type ManagePaymentAccounts struct {
	accounts portout.FinanceAccountRepository
}

// NewManagePaymentAccounts creates a new ManagePaymentAccounts use case.
func NewManagePaymentAccounts(accounts portout.FinanceAccountRepository) *ManagePaymentAccounts {
	return &ManagePaymentAccounts{accounts: accounts}
}

// List returns all active payment accounts belonging to the user.
func (uc *ManagePaymentAccounts) List(ctx context.Context, in portin.ListPaymentAccountsInput) ([]domain.PaymentAccount, error) {
	accounts, err := uc.accounts.FindByUserID(ctx, in.UserID)
	if err != nil {
		return nil, fmt.Errorf("listar contas de pagamento: %w", err)
	}
	return accounts, nil
}

// Create validates and persists a new payment account.
func (uc *ManagePaymentAccounts) Create(ctx context.Context, in portin.FinanceCreateAccountInput) (*domain.PaymentAccount, error) {
	// Validar chave PIX quando tipo for PIX
	if strings.ToUpper(in.Type) == "PIX" {
		if err := domain.ValidatePixKey(in.PixType, in.PixKey); err != nil {
			return nil, apierr.BadRequest(fmt.Sprintf("chave PIX inválida: %s", err.Error()))
		}
	}

	now := time.Now()
	account := &domain.PaymentAccount{
		UserID:     in.UserID,
		Type:       strings.ToUpper(in.Type),
		PixKey:     in.PixKey,
		PixType:    strings.ToUpper(in.PixType),
		BankCode:   in.BankCode,
		AgencyNum:  in.AgencyNum,
		AccountNum: in.AccountNum,
		IsDefault:  false,
		Active:     true,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	saved, err := uc.accounts.Save(ctx, account)
	if err != nil {
		return nil, fmt.Errorf("salvar conta de pagamento: %w", err)
	}
	return saved, nil
}

// Update validates and updates an existing payment account, enforcing ownership.
func (uc *ManagePaymentAccounts) Update(ctx context.Context, in portin.FinanceUpdateAccountInput) (*domain.PaymentAccount, error) {
	account, err := uc.accounts.FindByID(ctx, in.AccountID, in.UserID)
	if err != nil {
		return nil, fmt.Errorf("buscar conta de pagamento: %w", err)
	}

	// Validar chave PIX quando tipo for PIX
	newType := in.Type
	if newType == "" {
		newType = account.Type
	}
	newPixType := in.PixType
	if newPixType == "" {
		newPixType = account.PixType
	}
	newPixKey := in.PixKey
	if newPixKey == "" {
		newPixKey = account.PixKey
	}

	if strings.ToUpper(newType) == "PIX" && newPixKey != "" {
		if err := domain.ValidatePixKey(newPixType, newPixKey); err != nil {
			return nil, apierr.BadRequest(fmt.Sprintf("chave PIX inválida: %s", err.Error()))
		}
	}

	account.Type = strings.ToUpper(newType)
	account.PixKey = newPixKey
	account.PixType = strings.ToUpper(newPixType)
	account.BankCode = in.BankCode
	account.AgencyNum = in.AgencyNum
	account.AccountNum = in.AccountNum
	account.UpdatedAt = time.Now()

	updated, err := uc.accounts.Update(ctx, account)
	if err != nil {
		return nil, fmt.Errorf("atualizar conta de pagamento: %w", err)
	}
	return updated, nil
}

// SetDefault designates a payment account as the user's default.
// Uses a sequential "clear then set" pattern (sem transação MongoDB).
func (uc *ManagePaymentAccounts) SetDefault(ctx context.Context, accountID, userID string) error {
	// Verificar ownership antes de alterar
	if _, err := uc.accounts.FindByID(ctx, accountID, userID); err != nil {
		return fmt.Errorf("conta de pagamento não encontrada: %w", err)
	}

	// Passo 1: remover default de todas as contas do usuário
	if err := uc.accounts.ClearDefault(ctx, userID); err != nil {
		return fmt.Errorf("limpar default das contas: %w", err)
	}

	// Passo 2: definir a conta selecionada como default
	account, err := uc.accounts.FindByID(ctx, accountID, userID)
	if err != nil {
		return fmt.Errorf("recarregar conta de pagamento: %w", err)
	}
	account.IsDefault = true
	account.UpdatedAt = time.Now()

	if _, err := uc.accounts.Update(ctx, account); err != nil {
		return fmt.Errorf("setar conta como default: %w", err)
	}
	return nil
}

// Delete soft-deletes a payment account, enforcing ownership.
func (uc *ManagePaymentAccounts) Delete(ctx context.Context, accountID, userID string) error {
	// Verificar ownership
	if _, err := uc.accounts.FindByID(ctx, accountID, userID); err != nil {
		return fmt.Errorf("conta de pagamento não encontrada: %w", err)
	}

	if err := uc.accounts.SoftDelete(ctx, accountID, userID); err != nil {
		return fmt.Errorf("desativar conta de pagamento: %w", err)
	}
	return nil
}

// ---- UC 10: GetAuditTrail ----

// GetAuditTrail returns the paginated audit trail for an event.
// Returns an empty list gracefully if the audit_trail collection does not exist or is empty.
type GetAuditTrail struct {
	auditTrail portout.AuditTrailRepository
}

// NewGetAuditTrail creates a new GetAuditTrail use case.
func NewGetAuditTrail(auditTrail portout.AuditTrailRepository) *GetAuditTrail {
	return &GetAuditTrail{auditTrail: auditTrail}
}

// Execute returns paged audit entries for the event.
func (uc *GetAuditTrail) Execute(ctx context.Context, in portin.GetAuditTrailInput) ([]domain.AuditEntry, int64, error) {
	size := max(1, min(100, in.Size))
	page := max(0, in.Page)

	entries, total, err := uc.auditTrail.FindByEventID(ctx, in.EventID, page, size)
	if err != nil {
		return []domain.AuditEntry{}, 0, fmt.Errorf("buscar audit trail: %w", err)
	}
	if entries == nil {
		entries = []domain.AuditEntry{}
	}
	return entries, total, nil
}
