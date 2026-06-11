package finance_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainconfig "github.com/role-organizado/backend-go-role-organizado/internal/domain/config"
	domainevent "github.com/role-organizado/backend-go-role-organizado/internal/domain/event"
	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/finance"
	domainrateio "github.com/role-organizado/backend-go-role-organizado/internal/domain/rateio"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	ucfinance "github.com/role-organizado/backend-go-role-organizado/internal/usecase/finance"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ============================================================
// Simple stub repositories — sem framework de mock
// ============================================================

// stubParticipantRepo implements portout.ParticipantRepository.
type stubParticipantRepo struct {
	byUser    []domain.Participant
	byEvent   []domain.Participant
	eventTotal int64
	single    *domain.Participant
	userErr   error
	eventErr  error
	singleErr error
}

func (s *stubParticipantRepo) FindByUserID(_ context.Context, _ string) ([]domain.Participant, error) {
	return s.byUser, s.userErr
}
func (s *stubParticipantRepo) FindByEventID(_ context.Context, _ string, _, _ int) ([]domain.Participant, int64, error) {
	return s.byEvent, s.eventTotal, s.eventErr
}
func (s *stubParticipantRepo) FindByEventIDAndUserID(_ context.Context, _, _ string) (*domain.Participant, error) {
	return s.single, s.singleErr
}
func (s *stubParticipantRepo) FindAllByEventID(_ context.Context, _ string) ([]domain.Participant, error) {
	return s.byEvent, s.eventErr
}

// stubEventoRepo implements portout.EventoRepository.
type stubEventoRepo struct {
	evento *domainevent.Evento
	err    error
}

func (s *stubEventoRepo) FindByID(_ context.Context, _ string) (*domainevent.Evento, error) {
	return s.evento, s.err
}
func (s *stubEventoRepo) FindByUsuarioID(_ context.Context, _ string, _, _ int) ([]domainevent.Evento, int64, error) {
	return nil, 0, nil
}
func (s *stubEventoRepo) FindByUsuarioIDCursor(_ context.Context, _ string, _ portout.EventoQueryFiltros) (portout.EventosCursorPage, error) {
	return portout.EventosCursorPage{}, nil
}
func (s *stubEventoRepo) FindAll(_ context.Context, _, _ int) ([]domainevent.Evento, int64, error) {
	return nil, 0, nil
}
func (s *stubEventoRepo) Save(_ context.Context, e *domainevent.Evento) (*domainevent.Evento, error) {
	return e, nil
}
func (s *stubEventoRepo) Update(_ context.Context, e *domainevent.Evento) (*domainevent.Evento, error) {
	return e, nil
}
func (s *stubEventoRepo) DeleteByID(_ context.Context, _ string) error { return nil }
func (s *stubEventoRepo) AddConvidados(_ context.Context, _ string, _ []domainevent.Convidado) error {
	return nil
}
func (s *stubEventoRepo) FindAllByIDs(_ context.Context, _ []string) ([]domainevent.Evento, error) {
	return nil, nil
}
func (s *stubEventoRepo) UpdateFase(_ context.Context, _ string, _ domainevent.EventoFase) error {
	return nil
}
func (s *stubEventoRepo) UpdatePoliticaConvidados(_ context.Context, _, _ string) error {
	return nil
}
func (s *stubEventoRepo) AddImagens(_ context.Context, _ string, _ []domainevent.EventoImagem) error {
	return nil
}
func (s *stubEventoRepo) UpdateDetalhes(_ context.Context, e *domainevent.Evento) (*domainevent.Evento, error) {
	return e, nil
}

// stubRateioRepo implements portout.RateioRepository.
type stubRateioRepo struct {
	byEventoID []domainrateio.Rateio
	err        error
}

func (s *stubRateioRepo) FindByID(_ context.Context, _ string) (*domainrateio.Rateio, error) {
	return nil, nil
}
func (s *stubRateioRepo) FindByEventoID(_ context.Context, _ string) ([]domainrateio.Rateio, error) {
	return s.byEventoID, s.err
}
func (s *stubRateioRepo) FindByUsuarioID(_ context.Context, _ string, _, _ int) ([]domainrateio.Rateio, int64, error) {
	return nil, 0, nil
}
func (s *stubRateioRepo) Save(_ context.Context, r *domainrateio.Rateio) (*domainrateio.Rateio, error) {
	return r, nil
}
func (s *stubRateioRepo) Update(_ context.Context, r *domainrateio.Rateio) (*domainrateio.Rateio, error) {
	return r, nil
}
func (s *stubRateioRepo) DeleteByID(_ context.Context, _ string) error { return nil }

// stubSummaryRepo implements portout.FinanceSummaryRepository.
type stubSummaryRepo struct {
	found   *domain.FinanceSummary
	findErr error
	saved   *domain.FinanceSummary
	saveErr error
}

func (s *stubSummaryRepo) FindByEventID(_ context.Context, _ string) (*domain.FinanceSummary, error) {
	return s.found, s.findErr
}
func (s *stubSummaryRepo) Save(_ context.Context, sm *domain.FinanceSummary) (*domain.FinanceSummary, error) {
	if s.saveErr != nil {
		return nil, s.saveErr
	}
	if s.saved != nil {
		return s.saved, nil
	}
	return sm, nil
}
func (s *stubSummaryRepo) Update(_ context.Context, sm *domain.FinanceSummary) (*domain.FinanceSummary, error) {
	if s.saveErr != nil {
		return nil, s.saveErr
	}
	return sm, nil
}

// stubLedgerRepo implements portout.LedgerEntryRepository with arg capture for clamping tests.
type stubLedgerRepo struct {
	entries    []domain.LedgerEntry
	total      int64
	err        error
	lastFilter *string // captures the entryType passed by the UC
	lastPage   int
	lastSize   int
}

func (s *stubLedgerRepo) FindByEventID(_ context.Context, _ string, entryType *string, _, _ *time.Time, page, size int) ([]domain.LedgerEntry, int64, error) {
	s.lastFilter = entryType
	s.lastPage = page
	s.lastSize = size
	return s.entries, s.total, s.err
}

// stubInstallmentRepo implements portout.PaymentInstallmentRepository.
type stubInstallmentRepo struct {
	all     []domain.PaymentInstallment
	byPart  []domain.PaymentInstallment
	pending []domain.PaymentInstallment
	err     error
}

func (s *stubInstallmentRepo) FindByEventID(_ context.Context, _ string) ([]domain.PaymentInstallment, error) {
	return s.all, s.err
}
func (s *stubInstallmentRepo) FindByParticipantID(_ context.Context, _, _ string) ([]domain.PaymentInstallment, error) {
	return s.byPart, s.err
}
func (s *stubInstallmentRepo) FindPendingByEventID(_ context.Context, _ string) ([]domain.PaymentInstallment, error) {
	return s.pending, s.err
}

// stubConfigRepo implements portout.ConfiguracaoSistemaRepository (= ConfigSistemaRepository).
type stubConfigRepo struct {
	cfg *domainconfig.ConfiguracaoSistema
	err error
}

func (s *stubConfigRepo) FindAll(_ context.Context) ([]domainconfig.ConfiguracaoSistema, error) {
	return nil, nil
}
func (s *stubConfigRepo) FindByChave(_ context.Context, _ string) (*domainconfig.ConfiguracaoSistema, error) {
	return s.cfg, s.err
}
func (s *stubConfigRepo) Save(_ context.Context, c *domainconfig.ConfiguracaoSistema) (*domainconfig.ConfiguracaoSistema, error) {
	return c, nil
}

// stubAccountRepo implements portout.PaymentAccountRepository.
type stubAccountRepo struct {
	accounts      []domain.PaymentAccount
	single        *domain.PaymentAccount
	findErr       error
	saveErr       error
	clearDefaultN int // how many times ClearDefault was called
}

func (s *stubAccountRepo) FindByUserID(_ context.Context, _ string) ([]domain.PaymentAccount, error) {
	return s.accounts, s.findErr
}
func (s *stubAccountRepo) FindByID(_ context.Context, _, _ string) (*domain.PaymentAccount, error) {
	return s.single, s.findErr
}
func (s *stubAccountRepo) Save(_ context.Context, a *domain.PaymentAccount) (*domain.PaymentAccount, error) {
	if s.saveErr != nil {
		return nil, s.saveErr
	}
	a.ID = "acc-generated"
	return a, nil
}
func (s *stubAccountRepo) Update(_ context.Context, a *domain.PaymentAccount) (*domain.PaymentAccount, error) {
	return a, s.saveErr
}
func (s *stubAccountRepo) ClearDefault(_ context.Context, _ string) error {
	s.clearDefaultN++
	return nil
}
func (s *stubAccountRepo) SoftDelete(_ context.Context, _, _ string) error {
	return s.saveErr
}

// stubDispatcher implements ucfinance.PaymentReminderDispatcher.
type stubDispatcher struct {
	dispatched []string // participantIDs dispatched
	err        error
}

func (s *stubDispatcher) Dispatch(_ context.Context, _, participantID string) error {
	if s.err != nil {
		return s.err
	}
	s.dispatched = append(s.dispatched, participantID)
	return nil
}

// ============================================================
// Helpers
// ============================================================

func makeParticipant(eventID, userID, status string) domain.Participant {
	return domain.Participant{
		ID:      "part-" + userID,
		EventID: eventID,
		UserID:  userID,
		Status:  status,
		Name:    "Test User",
		Email:   "test@example.com",
	}
}

func makeEvento(id, nome string, date time.Time) *domainevent.Evento {
	return &domainevent.Evento{
		ID:   id,
		Nome: nome,
		Data: date,
	}
}

func makeRateio(eventoID string, valorTotal float64, status domainrateio.StatusRateio) domainrateio.Rateio {
	return domainrateio.Rateio{
		ID:         "rat-1",
		EventoID:   eventoID,
		ValorTotal: valorTotal,
		Status:     status,
	}
}

func makeInstallment(eventID, participantID, status, method string, amount int64, paidAt *time.Time) domain.PaymentInstallment {
	return domain.PaymentInstallment{
		ID:            "inst-" + participantID,
		EventID:       eventID,
		ParticipantID: participantID,
		Amount:        amount,
		Status:        status,
		PaymentMethod: method,
		PaidAt:        paidAt,
	}
}

// ============================================================
// UC 1: ListFinanceEvents
// ============================================================

func TestListFinanceEvents(t *testing.T) {
	now := time.Now()
	evt := makeEvento("evt-1", "Churrasco", now)

	tests := []struct {
		name         string
		participants []domain.Participant
		evento       *domainevent.Evento
		rateios      []domainrateio.Rateio
		summary      *domain.FinanceSummary
		wantLen      int
		wantGoal     int64
		wantCollected int64
	}{
		{
			name:     "usuário sem eventos retorna lista vazia",
			wantLen:  0,
		},
		{
			name:         "exclui participação com status ORGANIZACAO",
			participants: []domain.Participant{makeParticipant("evt-1", "u1", "ORGANIZACAO")},
			evento:       evt,
			wantLen:      0,
		},
		{
			name:         "exclui participação com status AGUARDANDO_ACEITE",
			participants: []domain.Participant{makeParticipant("evt-1", "u1", "AGUARDANDO_ACEITE")},
			evento:       evt,
			wantLen:      0,
		},
		{
			name:         "inclui participação ativa e calcula goal",
			participants: []domain.Participant{makeParticipant("evt-1", "u1", "ATIVO")},
			evento:       evt,
			rateios:      []domainrateio.Rateio{makeRateio("evt-1", 100.00, domainrateio.StatusRateioAberto)},
			summary:      &domain.FinanceSummary{EventID: "evt-1", Collected: 5000},
			wantLen:      1,
			wantGoal:     10000, // 100.00 * 100 = 10000 centavos
			wantCollected: 5000,
		},
		{
			name:         "rateio FECHADO não conta para goal",
			participants: []domain.Participant{makeParticipant("evt-1", "u1", "ATIVO")},
			evento:       evt,
			rateios: []domainrateio.Rateio{
				makeRateio("evt-1", 100.00, domainrateio.StatusRateioAberto),
				makeRateio("evt-1", 50.00, domainrateio.StatusRateioFechado),
			},
			wantLen:  1,
			wantGoal: 10000, // só o ABERTO conta
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			partRepo := &stubParticipantRepo{byUser: tt.participants}
			eventoRepo := &stubEventoRepo{evento: tt.evento}
			rateioRepo := &stubRateioRepo{byEventoID: tt.rateios}
			summaryRepo := &stubSummaryRepo{found: tt.summary}

			uc := ucfinance.NewListFinanceEvents(partRepo, eventoRepo, rateioRepo, summaryRepo)
			got, err := uc.Execute(context.Background(), portin.ListFinanceEventsInput{UserID: "u1"})

			require.NoError(t, err)
			assert.Len(t, got, tt.wantLen)
			if tt.wantLen > 0 {
				assert.Equal(t, tt.wantGoal, got[0].Goal)
				assert.Equal(t, tt.wantCollected, got[0].Collected)
			}
		})
	}
}

// ============================================================
// UC 2: GetFinanceOverview
// ============================================================

func TestGetFinanceOverview(t *testing.T) {
	baseEvento := makeEvento("evt-1", "Aniversário", time.Now())
	activeParticipant := makeParticipant("evt-1", "u1", "ATIVO")

	tests := []struct {
		name               string
		participant        *domain.Participant
		participantErr     error
		evento             *domainevent.Evento
		rateios            []domainrateio.Rateio
		summary            *domain.FinanceSummary
		wantErr            bool
		wantStatus         int
		wantGoal           int64
		wantProgress       float64
		wantAvailable      int64
	}{
		{
			name:           "usuário sem acesso retorna 403",
			participantErr: apierr.NotFound("participant", "u-none"),
			evento:         baseEvento,
			wantErr:        true,
			wantStatus:     403,
		},
		{
			name:        "goal zero retorna progress zero",
			participant: &activeParticipant,
			evento:      baseEvento,
			rateios:     nil, // sem rateios
			wantErr:     false,
			wantGoal:    0,
			wantProgress: 0,
		},
		{
			name:        "calcula availableForWithdrawal corretamente",
			participant: &activeParticipant,
			evento:      baseEvento,
			rateios:     []domainrateio.Rateio{makeRateio("evt-1", 100.00, domainrateio.StatusRateioAberto)},
			summary: &domain.FinanceSummary{
				EventID:            "evt-1",
				Collected:          8000,
				PendingWithdrawals: 2000,
			},
			wantErr:       false,
			wantGoal:      10000,
			wantProgress:  80.0, // 8000/10000 * 100
			wantAvailable: 6000, // 8000 - 2000
		},
		{
			name:        "pendingWithdrawals maior que collected → available zero",
			participant: &activeParticipant,
			evento:      baseEvento,
			summary: &domain.FinanceSummary{
				EventID:            "evt-1",
				Collected:          1000,
				PendingWithdrawals: 5000,
			},
			wantErr:       false,
			wantAvailable: 0, // max(0, 1000-5000) = 0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			partRepo := &stubParticipantRepo{
				single:    tt.participant,
				singleErr: tt.participantErr,
			}
			eventoRepo := &stubEventoRepo{evento: tt.evento}
			rateioRepo := &stubRateioRepo{byEventoID: tt.rateios}
			summaryRepo := &stubSummaryRepo{found: tt.summary}

			uc := ucfinance.NewGetFinanceOverview(eventoRepo, partRepo, rateioRepo, summaryRepo)
			got, err := uc.Execute(context.Background(), portin.GetFinanceOverviewInput{
				EventID: "evt-1",
				UserID:  "u1",
			})

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantStatus != 0 {
					ae, ok := err.(*apierr.APIError)
					require.True(t, ok)
					assert.Equal(t, tt.wantStatus, ae.Status)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tt.wantGoal, got.Goal)
			assert.InDelta(t, tt.wantProgress, got.ProgressPercentage, 0.0001)
			assert.Equal(t, tt.wantAvailable, got.AvailableForWithdrawal)
		})
	}
}

// ============================================================
// UC 3: GetLedgerStatement
// ============================================================

func TestGetLedgerStatement(t *testing.T) {
	activeParticipant := makeParticipant("evt-1", "u1", "ATIVO")

	tests := []struct {
		name       string
		input      portin.GetLedgerStatementInput
		wantSize   int
		wantFilter *string // nil = "all"; non-nil = exact value expected
		wantErr    bool
	}{
		{
			name:     "size 0 é clampado para 1",
			input:    portin.GetLedgerStatementInput{EventID: "evt-1", UserID: "u1", Type: "all", Size: 0},
			wantSize: 1,
		},
		{
			name:     "size 200 é clampado para 100",
			input:    portin.GetLedgerStatementInput{EventID: "evt-1", UserID: "u1", Type: "all", Size: 200},
			wantSize: 100,
		},
		{
			name:     "size 50 permanece 50",
			input:    portin.GetLedgerStatementInput{EventID: "evt-1", UserID: "u1", Type: "all", Size: 50},
			wantSize: 50,
		},
		{
			name:       `type "all" resulta em filtro nil`,
			input:      portin.GetLedgerStatementInput{EventID: "evt-1", UserID: "u1", Type: "all", Size: 10},
			wantFilter: nil,
		},
		{
			name:       `type "income" é normalizado para "INCOME"`,
			input:      portin.GetLedgerStatementInput{EventID: "evt-1", UserID: "u1", Type: "income", Size: 10},
			wantFilter: strPtr("INCOME"),
		},
		{
			name:       `type "EXPENSE" permanece "EXPENSE"`,
			input:      portin.GetLedgerStatementInput{EventID: "evt-1", UserID: "u1", Type: "EXPENSE", Size: 10},
			wantFilter: strPtr("EXPENSE"),
		},
		{
			name:    "usuário sem acesso retorna 403",
			input:   portin.GetLedgerStatementInput{EventID: "evt-1", UserID: "u-none", Size: 10},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ledgerRepo := &stubLedgerRepo{total: 5}
			var partRepo *stubParticipantRepo
			if tt.input.UserID == "u1" {
				partRepo = &stubParticipantRepo{single: &activeParticipant}
			} else {
				partRepo = &stubParticipantRepo{singleErr: apierr.NotFound("participant", tt.input.UserID)}
			}

			uc := ucfinance.NewGetLedgerStatement(ledgerRepo, partRepo)
			got, err := uc.Execute(context.Background(), tt.input)

			if tt.wantErr {
				require.Error(t, err)
				ae, ok := err.(*apierr.APIError)
				require.True(t, ok)
				assert.Equal(t, 403, ae.Status)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, got)

			if tt.wantSize > 0 {
				assert.Equal(t, tt.wantSize, got.Size, "size clamp incorreto")
				assert.Equal(t, tt.wantSize, ledgerRepo.lastSize, "size passado ao repo incorreto")
			}
			if tt.input.Type != "" {
				if tt.wantFilter == nil {
					assert.Nil(t, ledgerRepo.lastFilter, "esperava filtro nil para type=all")
				} else {
					require.NotNil(t, ledgerRepo.lastFilter)
					assert.Equal(t, *tt.wantFilter, *ledgerRepo.lastFilter)
				}
			}
		})
	}
}

func strPtr(s string) *string { return &s }

// ============================================================
// UC 4: GetParticipantsStatus
// ============================================================

func TestGetParticipantsStatus(t *testing.T) {
	part := makeParticipant("evt-1", "u1", "ATIVO")
	paidAt := time.Now().Add(-24 * time.Hour)

	tests := []struct {
		name         string
		participants []domain.Participant
		total        int64
		installments []domain.PaymentInstallment
		wantLen      int
		wantStatus   string
	}{
		{
			name:     "sem participantes retorna lista vazia",
			wantLen:  0,
		},
		{
			name:         "parcela PAID → participante PAID",
			participants: []domain.Participant{part},
			total:        1,
			installments: []domain.PaymentInstallment{makeInstallment("evt-1", part.ID, "PAID", "PIX", 1000, &paidAt)},
			wantLen:      1,
			wantStatus:   "PAID",
		},
		{
			name:         "parcela OVERDUE → participante OVERDUE",
			participants: []domain.Participant{part},
			total:        1,
			installments: []domain.PaymentInstallment{makeInstallment("evt-1", part.ID, "OVERDUE", "PIX", 1000, nil)},
			wantLen:      1,
			wantStatus:   "OVERDUE",
		},
		{
			name:         "sem parcelas → participante PENDING",
			participants: []domain.Participant{part},
			total:        1,
			installments: nil,
			wantLen:      1,
			wantStatus:   "PENDING",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			partRepo := &stubParticipantRepo{
				byEvent:    tt.participants,
				eventTotal: tt.total,
			}
			instRepo := &stubInstallmentRepo{byPart: tt.installments}

			uc := ucfinance.NewGetParticipantsStatus(partRepo, instRepo)
			got, total, err := uc.Execute(context.Background(), portin.GetParticipantsStatusInput{
				EventID: "evt-1",
				UserID:  "u1",
				Size:    10,
			})

			require.NoError(t, err)
			assert.Len(t, got, tt.wantLen)
			assert.Equal(t, tt.total, total)
			if tt.wantLen > 0 {
				assert.Equal(t, tt.wantStatus, got[0].Status)
			}
		})
	}
}

// ============================================================
// UC 5: RecalculateFinanceSummary
// ============================================================

func TestRecalculateFinanceSummary(t *testing.T) {
	paidAt := time.Now().Add(-time.Hour)

	tests := []struct {
		name         string
		existingSummary *domain.FinanceSummary
		rateios      []domainrateio.Rateio
		installments []domain.PaymentInstallment
		wantGoal     int64
		wantCollected int64
	}{
		{
			name:          "cria novo summary quando não existe",
			existingSummary: nil,
			rateios:       []domainrateio.Rateio{makeRateio("evt-1", 50.00, domainrateio.StatusRateioAberto)},
			installments:  []domain.PaymentInstallment{makeInstallment("evt-1", "p1", "PAID", "PIX", 3000, &paidAt)},
			wantGoal:      5000,
			wantCollected: 3000,
		},
		{
			name:            "atualiza summary existente",
			existingSummary: &domain.FinanceSummary{ID: "sum-1", EventID: "evt-1"},
			rateios:         []domainrateio.Rateio{makeRateio("evt-1", 200.00, domainrateio.StatusRateioAberto)},
			installments:    []domain.PaymentInstallment{makeInstallment("evt-1", "p1", "PAID", "PIX", 15000, &paidAt)},
			wantGoal:        20000,
			wantCollected:   15000,
		},
		{
			name:          "rateio FECHADO não conta para goal",
			rateios:       []domainrateio.Rateio{makeRateio("evt-1", 100.00, domainrateio.StatusRateioFechado)},
			wantGoal:      0,
			wantCollected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findErr := errors.New("not found")
			if tt.existingSummary != nil {
				findErr = nil
			}
			summaryRepo := &stubSummaryRepo{found: tt.existingSummary, findErr: findErr}
			rateioRepo := &stubRateioRepo{byEventoID: tt.rateios}
			instRepo := &stubInstallmentRepo{all: tt.installments}

			uc := ucfinance.NewRecalculateFinanceSummary(summaryRepo, rateioRepo, instRepo)
			got, err := uc.Execute(context.Background(), portin.RecalculateFinanceSummaryInput{
				EventID: "evt-1",
				UserID:  "u1",
			})

			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tt.wantGoal, got.Goal)
			assert.Equal(t, tt.wantCollected, got.Collected)
			assert.False(t, got.LastCalculatedAt.IsZero())
		})
	}
}

// ============================================================
// UC 6: SendPaymentReminders
// ============================================================

func TestSendPaymentReminders(t *testing.T) {
	tests := []struct {
		name            string
		pending         []domain.PaymentInstallment
		dispatcherErr   error
		nilDispatcher   bool
		wantErr         bool
		wantDispatched  int
	}{
		{
			name:           "sem pendências não envia lembretes",
			pending:        nil,
			wantDispatched: 0,
		},
		{
			name: "envia lembretes para participantes únicos",
			pending: []domain.PaymentInstallment{
				makeInstallment("evt-1", "p1", "PENDING", "PIX", 1000, nil),
				makeInstallment("evt-1", "p2", "OVERDUE", "BOLETO", 2000, nil),
				makeInstallment("evt-1", "p1", "PENDING", "PIX", 500, nil), // duplicado p1
			},
			wantDispatched: 2, // p1 e p2 (p1 deduplicado)
		},
		{
			name:          "dispatcher nil não falha (loga)",
			pending:       []domain.PaymentInstallment{makeInstallment("evt-1", "p1", "PENDING", "PIX", 1000, nil)},
			nilDispatcher: true,
			wantErr:       false,
		},
		{
			name:    "erro no dispatcher retorna erro wrappado",
			pending: []domain.PaymentInstallment{makeInstallment("evt-1", "p1", "PENDING", "PIX", 1000, nil)},
			dispatcherErr: errors.New("temporal unavailable"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instRepo := &stubInstallmentRepo{pending: tt.pending}
			partRepo := &stubParticipantRepo{}

			var dispatcher ucfinance.PaymentReminderDispatcher
			if !tt.nilDispatcher {
				d := &stubDispatcher{err: tt.dispatcherErr}
				dispatcher = d
				defer func() {
					if !tt.wantErr && tt.wantDispatched > 0 {
						assert.Len(t, d.dispatched, tt.wantDispatched)
					}
				}()
			}

			uc := ucfinance.NewSendPaymentReminders(partRepo, instRepo, dispatcher)
			err := uc.Execute(context.Background(), portin.SendPaymentRemindersInput{
				EventID: "evt-1",
				UserID:  "u1",
			})

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// ============================================================
// UC 7: CalculateHoldBalance
// ============================================================

func TestCalculateHoldBalance(t *testing.T) {
	now := time.Now()
	recent := now.Add(-1 * time.Hour)              // 1 hora atrás
	old31days := now.Add(-31 * 24 * time.Hour)     // 31 dias atrás (passou hold de 30 dias)
	old2days := now.Add(-2 * 24 * time.Hour)       // 2 dias atrás (passou hold de BOLETO=1 dia)

	tests := []struct {
		name             string
		installments     []domain.PaymentInstallment
		wantBlocked      int64
		wantAvailable    int64
		wantNextRelease  bool // true se deve haver nextReleaseDate
	}{
		{
			name: "installment PIX recente não é bloqueado (hold=0 dias)",
			installments: []domain.PaymentInstallment{
				makeInstallment("evt-1", "p1", "PAID", "PIX", 5000, &recent),
			},
			wantBlocked:   0,
			wantAvailable: 5000,
		},
		{
			name: "installment CREDIT_CARD recente é bloqueado (hold=30 dias)",
			installments: []domain.PaymentInstallment{
				makeInstallment("evt-1", "p1", "PAID", "CREDIT_CARD", 5000, &recent),
			},
			wantBlocked:    5000,
			wantAvailable:  0,
			wantNextRelease: true,
		},
		{
			name: "installment CREDIT_CARD com 31 dias não é mais bloqueado",
			installments: []domain.PaymentInstallment{
				makeInstallment("evt-1", "p1", "PAID", "CREDIT_CARD", 5000, &old31days),
			},
			wantBlocked:   0,
			wantAvailable: 5000,
		},
		{
			name: "installment BOLETO com 2 dias não é mais bloqueado (hold=1 dia)",
			installments: []domain.PaymentInstallment{
				makeInstallment("evt-1", "p1", "PAID", "BOLETO", 3000, &old2days),
			},
			wantBlocked:   0,
			wantAvailable: 3000,
		},
		{
			name: "mix: PIX liberado + CREDIT_CARD bloqueado",
			installments: []domain.PaymentInstallment{
				makeInstallment("evt-1", "p1", "PAID", "PIX", 2000, &recent),
				makeInstallment("evt-1", "p2", "PAID", "CREDIT_CARD", 3000, &recent),
			},
			wantBlocked:    3000,
			wantAvailable:  2000, // 5000 total - 3000 bloqueado
			wantNextRelease: true,
		},
		{
			name: "installment PENDING é ignorado",
			installments: []domain.PaymentInstallment{
				makeInstallment("evt-1", "p1", "PENDING", "PIX", 5000, nil),
			},
			wantBlocked:   0,
			wantAvailable: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instRepo := &stubInstallmentRepo{all: tt.installments}
			configRepo := &stubConfigRepo{err: errors.New("not configured")} // usar defaults

			uc := ucfinance.NewCalculateHoldBalance(instRepo, configRepo)
			got, err := uc.Execute(context.Background(), portin.CalculateHoldBalanceInput{
				EventID: "evt-1",
				UserID:  "u1",
			})

			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tt.wantBlocked, got.BlockedBalance)
			assert.Equal(t, tt.wantAvailable, got.AvailableForOutbound)
			if tt.wantNextRelease {
				assert.NotNil(t, got.NextReleaseDate)
			} else {
				assert.Nil(t, got.NextReleaseDate)
			}
			assert.NotNil(t, got.BreakdownByMethod)
		})
	}
}

// ============================================================
// UC 8: GetEventPaymentStatus
// ============================================================

func TestGetEventPaymentStatus(t *testing.T) {
	paidAt := time.Now().Add(-time.Hour)

	tests := []struct {
		name         string
		installments []domain.PaymentInstallment
		wantPaid     int64
		wantPending  int64
		wantOverdue  int64
		wantPaidCnt  int
		wantPendCnt  int
	}{
		{
			name:     "sem parcelas retorna zeros",
			wantPaid: 0, wantPending: 0, wantOverdue: 0,
		},
		{
			name: "agrega por status corretamente",
			installments: []domain.PaymentInstallment{
				makeInstallment("evt-1", "p1", "PAID", "PIX", 5000, &paidAt),
				makeInstallment("evt-1", "p2", "PENDING", "BOLETO", 3000, nil),
				makeInstallment("evt-1", "p3", "OVERDUE", "PIX", 2000, nil),
			},
			wantPaid:    5000,
			wantPending: 3000,
			wantOverdue: 2000,
			wantPaidCnt: 1,
			wantPendCnt: 2, // PENDING + OVERDUE
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instRepo := &stubInstallmentRepo{all: tt.installments}
			partRepo := &stubParticipantRepo{}

			uc := ucfinance.NewGetEventPaymentStatus(instRepo, partRepo)
			got, err := uc.Execute(context.Background(), portin.GetEventPaymentStatusInput{
				EventID: "evt-1",
				UserID:  "u1",
			})

			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tt.wantPaid, got.TotalPaid)
			assert.Equal(t, tt.wantPending, got.TotalPending)
			assert.Equal(t, tt.wantOverdue, got.TotalOverdue)
			assert.Equal(t, tt.wantPaidCnt, got.PaidCount)
			assert.Equal(t, tt.wantPendCnt, got.PendingCount)
		})
	}
}

// ============================================================
// UC 9: ManagePaymentAccounts
// ============================================================

func TestManagePaymentAccounts_Create(t *testing.T) {
	tests := []struct {
		name    string
		input   portin.FinanceCreateAccountInput
		wantErr bool
		errMsg  string
	}{
		{
			name: "PIX CPF inválido com 10 dígitos retorna erro",
			input: portin.FinanceCreateAccountInput{
				UserID:  "u1",
				Type:    "PIX",
				PixType: "CPF",
				PixKey:  "1234567890", // 10 dígitos — inválido (deve ter 11)
			},
			wantErr: true,
			errMsg:  "11 dígitos",
		},
		{
			name: "PIX CPF válido com 11 dígitos é salvo",
			input: portin.FinanceCreateAccountInput{
				UserID:  "u1",
				Type:    "PIX",
				PixType: "CPF",
				PixKey:  "12345678901", // 11 dígitos — válido
			},
			wantErr: false,
		},
		{
			name: "PIX EMAIL válido é aceito",
			input: portin.FinanceCreateAccountInput{
				UserID:  "u1",
				Type:    "PIX",
				PixType: "EMAIL",
				PixKey:  "user@example.com",
			},
			wantErr: false,
		},
		{
			name: "PIX EMAIL sem @ retorna erro",
			input: portin.FinanceCreateAccountInput{
				UserID:  "u1",
				Type:    "PIX",
				PixType: "EMAIL",
				PixKey:  "emailsemaroba",
			},
			wantErr: true,
		},
		{
			name: "conta BANK não valida chave PIX",
			input: portin.FinanceCreateAccountInput{
				UserID:   "u1",
				Type:     "BANK",
				BankCode: "001",
				AgencyNum: "1234",
				AccountNum: "567890",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accountRepo := &stubAccountRepo{}
			uc := ucfinance.NewManagePaymentAccounts(accountRepo)
			got, err := uc.Create(context.Background(), tt.input)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				ae, ok := err.(*apierr.APIError)
				require.True(t, ok)
				assert.Equal(t, 400, ae.Status)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tt.input.UserID, got.UserID)
			assert.True(t, got.Active)
		})
	}
}

func TestManagePaymentAccounts_SetDefault(t *testing.T) {
	account := &domain.PaymentAccount{
		ID:        "acc-1",
		UserID:    "u1",
		Type:      "PIX",
		IsDefault: false,
		Active:    true,
	}

	t.Run("set-default limpa e marca corretamente", func(t *testing.T) {
		repo := &stubAccountRepo{single: account}
		uc := ucfinance.NewManagePaymentAccounts(repo)

		err := uc.SetDefault(context.Background(), "acc-1", "u1")
		require.NoError(t, err)
		assert.Equal(t, 1, repo.clearDefaultN, "ClearDefault deve ser chamado uma vez")
	})

	t.Run("set-default é idempotente (segunda chamada não falha)", func(t *testing.T) {
		repo := &stubAccountRepo{single: account}
		uc := ucfinance.NewManagePaymentAccounts(repo)

		err1 := uc.SetDefault(context.Background(), "acc-1", "u1")
		require.NoError(t, err1)
		err2 := uc.SetDefault(context.Background(), "acc-1", "u1")
		require.NoError(t, err2, "segunda chamada também deve ser sem erro")
		assert.Equal(t, 2, repo.clearDefaultN, "ClearDefault chamado duas vezes (uma por SetDefault)")
	})

	t.Run("conta inexistente retorna erro", func(t *testing.T) {
		repo := &stubAccountRepo{findErr: apierr.NotFound("account", "acc-999")}
		uc := ucfinance.NewManagePaymentAccounts(repo)

		err := uc.SetDefault(context.Background(), "acc-999", "u1")
		require.Error(t, err)
	})
}

func TestManagePaymentAccounts_Update(t *testing.T) {
	existing := &domain.PaymentAccount{
		ID:      "acc-1",
		UserID:  "u1",
		Type:    "PIX",
		PixType: "CPF",
		PixKey:  "12345678901",
		Active:  true,
	}

	tests := []struct {
		name    string
		account *domain.PaymentAccount
		findErr error
		input   portin.FinanceUpdateAccountInput
		wantErr bool
		errCode int
	}{
		{
			name:    "atualiza conta existente com email válido",
			account: existing,
			input: portin.FinanceUpdateAccountInput{
				AccountID: "acc-1",
				UserID:    "u1",
				Type:      "PIX",
				PixType:   "EMAIL",
				PixKey:    "novo@email.com",
			},
			wantErr: false,
		},
		{
			name:    "PIX com email inválido retorna 400",
			account: existing,
			input: portin.FinanceUpdateAccountInput{
				AccountID: "acc-1",
				UserID:    "u1",
				Type:      "PIX",
				PixType:   "EMAIL",
				PixKey:    "invalido",
			},
			wantErr: true,
			errCode: 400,
		},
		{
			name:    "conta não encontrada retorna erro",
			findErr: apierr.NotFound("account", "acc-999"),
			input:   portin.FinanceUpdateAccountInput{AccountID: "acc-999", UserID: "u1"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &stubAccountRepo{single: tt.account, findErr: tt.findErr}
			uc := ucfinance.NewManagePaymentAccounts(repo)

			got, err := uc.Update(context.Background(), tt.input)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errCode != 0 {
					ae, ok := err.(*apierr.APIError)
					require.True(t, ok)
					assert.Equal(t, tt.errCode, ae.Status)
				}
				return
			}
			require.NoError(t, err)
			require.NotNil(t, got)
		})
	}
}

func TestManagePaymentAccounts_List(t *testing.T) {
	t.Run("lista contas do usuário", func(t *testing.T) {
		accounts := []domain.PaymentAccount{
			{ID: "acc-1", UserID: "u1", Active: true},
			{ID: "acc-2", UserID: "u1", Active: true},
		}
		repo := &stubAccountRepo{accounts: accounts}
		uc := ucfinance.NewManagePaymentAccounts(repo)

		got, err := uc.List(context.Background(), portin.ListPaymentAccountsInput{UserID: "u1"})
		require.NoError(t, err)
		assert.Len(t, got, 2)
	})
}

func TestManagePaymentAccounts_Delete(t *testing.T) {
	t.Run("deleta conta com ownership", func(t *testing.T) {
		account := &domain.PaymentAccount{ID: "acc-1", UserID: "u1", Active: true}
		repo := &stubAccountRepo{single: account}
		uc := ucfinance.NewManagePaymentAccounts(repo)

		err := uc.Delete(context.Background(), "acc-1", "u1")
		require.NoError(t, err)
	})

	t.Run("conta inexistente retorna erro", func(t *testing.T) {
		repo := &stubAccountRepo{findErr: apierr.NotFound("account", "acc-999")}
		uc := ucfinance.NewManagePaymentAccounts(repo)

		err := uc.Delete(context.Background(), "acc-999", "u1")
		require.Error(t, err)
	})
}
