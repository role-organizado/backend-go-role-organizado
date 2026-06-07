package payment_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	domain_event "github.com/role-organizado/backend-go-role-organizado/internal/domain/event"
	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	ucpayment "github.com/role-organizado/backend-go-role-organizado/internal/usecase/payment"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ─── Mock: PaymentInstallmentRepository ─────────────────────────────────────

type mockInstallmentRepo struct{ mock.Mock }

func (m *mockInstallmentRepo) FindByID(ctx context.Context, id string) (*domain.PaymentInstallment, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.PaymentInstallment), args.Error(1)
}

func (m *mockInstallmentRepo) FindByEventAndParticipant(ctx context.Context, eventID, participantID string) ([]*domain.PaymentInstallment, error) {
	args := m.Called(ctx, eventID, participantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.PaymentInstallment), args.Error(1)
}

func (m *mockInstallmentRepo) FindByUserOrParticipations(ctx context.Context, userID string, participationIDs []string, statusFilter *domain.InstallmentStatus) ([]*domain.PaymentInstallment, error) {
	args := m.Called(ctx, userID, participationIDs, statusFilter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.PaymentInstallment), args.Error(1)
}

func (m *mockInstallmentRepo) FindByIDs(ctx context.Context, ids []string) ([]*domain.PaymentInstallment, error) {
	args := m.Called(ctx, ids)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.PaymentInstallment), args.Error(1)
}

func (m *mockInstallmentRepo) MarkPaidBatch(ctx context.Context, ids []string, txID string, paidAt time.Time, method, reference string) error {
	args := m.Called(ctx, ids, txID, paidAt, method, reference)
	return args.Error(0)
}

func (m *mockInstallmentRepo) FindByEvent(ctx context.Context, eventID string, statusFilter *domain.InstallmentStatus) ([]*domain.PaymentInstallment, error) {
	args := m.Called(ctx, eventID, statusFilter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.PaymentInstallment), args.Error(1)
}

func (m *mockInstallmentRepo) CancelByParticipant(ctx context.Context, eventID, participantID string) (int64, error) {
	args := m.Called(ctx, eventID, participantID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockInstallmentRepo) Save(ctx context.Context, inst *domain.PaymentInstallment) error {
	args := m.Called(ctx, inst)
	return args.Error(0)
}

// ─── Mock: ParticipanteRepository ────────────────────────────────────────────

type mockInstParticipanteRepo struct{ mock.Mock }

func (m *mockInstParticipanteRepo) SaveOrganizador(ctx context.Context, eventoID, usuarioID string) error {
	args := m.Called(ctx, eventoID, usuarioID)
	return args.Error(0)
}

func (m *mockInstParticipanteRepo) FindParticipationIDsByUserID(ctx context.Context, userID string) ([]string, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *mockInstParticipanteRepo) IsParticipantOfEvent(ctx context.Context, eventID, userID string) (bool, error) {
	args := m.Called(ctx, eventID, userID)
	return args.Bool(0), args.Error(1)
}

// ─── Mock: EventoRepository ──────────────────────────────────────────────────

type mockInstEventoRepo struct{ mock.Mock }

func (m *mockInstEventoRepo) FindByID(ctx context.Context, id string) (*domain_event.Evento, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain_event.Evento), args.Error(1)
}

func (m *mockInstEventoRepo) FindByUsuarioID(ctx context.Context, usuarioID string, page, pageSize int) ([]domain_event.Evento, int64, error) {
	args := m.Called(ctx, usuarioID, page, pageSize)
	return args.Get(0).([]domain_event.Evento), int64(args.Int(1)), args.Error(2)
}

func (m *mockInstEventoRepo) FindByUsuarioIDCursor(ctx context.Context, usuarioID string, filtros portout.EventoQueryFiltros) (portout.EventosCursorPage, error) {
	args := m.Called(ctx, usuarioID, filtros)
	return args.Get(0).(portout.EventosCursorPage), args.Error(1)
}

func (m *mockInstEventoRepo) FindAll(ctx context.Context, page, pageSize int) ([]domain_event.Evento, int64, error) {
	args := m.Called(ctx, page, pageSize)
	return args.Get(0).([]domain_event.Evento), int64(args.Int(1)), args.Error(2)
}

func (m *mockInstEventoRepo) Save(ctx context.Context, e *domain_event.Evento) (*domain_event.Evento, error) {
	args := m.Called(ctx, e)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain_event.Evento), args.Error(1)
}

func (m *mockInstEventoRepo) Update(ctx context.Context, e *domain_event.Evento) (*domain_event.Evento, error) {
	args := m.Called(ctx, e)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain_event.Evento), args.Error(1)
}

func (m *mockInstEventoRepo) DeleteByID(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockInstEventoRepo) AddConvidados(ctx context.Context, eventoID string, convidados []domain_event.Convidado) error {
	args := m.Called(ctx, eventoID, convidados)
	return args.Error(0)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func makeInstallment(id, eventID, participantID string, status domain.InstallmentStatus) *domain.PaymentInstallment {
	return &domain.PaymentInstallment{
		ID:                id,
		EventID:           eventID,
		ParticipantID:     participantID,
		InstallmentNumber: 1,
		TotalInstallments: 3,
		AmountCents:       5000,
		DueDate:           time.Now().Add(30 * 24 * time.Hour),
		Status:            status,
	}
}

func makeEvento(id, ownerID string, status domain_event.EventoStatus) *domain_event.Evento {
	return &domain_event.Evento{
		ID:        id,
		UsuarioID: ownerID,
		Nome:      "Test Event",
		Status:    status,
	}
}

// ─── TestListUserInstallments: BUG5 participationIds ─────────────────────────

// TestListInstallments_BUG5_FindsViaParticipationId verifies the spec-096 fix:
// a user whose installments are stored under a participationId (not userId) still
// sees them returned.
func TestListInstallments_BUG5_FindsViaParticipationId(t *testing.T) {
	installRepo := new(mockInstallmentRepo)
	participRepo := new(mockInstParticipanteRepo)
	eventoRepo := new(mockInstEventoRepo)

	userID := "user-1"
	participationID := "participation-uuid-1"

	// User has one participation record.
	participRepo.On("FindParticipationIDsByUserID", mock.Anything, userID).
		Return([]string{participationID}, nil)

	// Installment is stored under participationID, not userId directly.
	inst := makeInstallment("inst-1", "event-1", participationID, domain.InstallmentStatusPending)
	participationIDs := []string{participationID}
	installRepo.On("FindByUserOrParticipations", mock.Anything, userID, participationIDs, (*domain.InstallmentStatus)(nil)).
		Return([]*domain.PaymentInstallment{inst}, nil)

	// Event is published (not excluded).
	eventoRepo.On("FindByID", mock.Anything, "event-1").
		Return(makeEvento("event-1", "organizer-1", "PUBLICADO"), nil)

	uc := ucpayment.NewListUserInstallments(installRepo, participRepo, eventoRepo)
	result, err := uc.Execute(context.Background(), userID, nil)

	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, participationID, result[0].ParticipantID, "installment stored under participationId must be returned")

	installRepo.AssertExpectations(t)
	participRepo.AssertExpectations(t)
}

// ─── TestListUserInstallments: ORGANIZACAO phase filtered ────────────────────

// TestListInstallments_FiltersOrganizacaoPhase verifies that installments from events
// in Java's ORGANIZACAO planning phase are excluded from the /user listing.
func TestListInstallments_FiltersOrganizacaoPhase(t *testing.T) {
	installRepo := new(mockInstallmentRepo)
	participRepo := new(mockInstParticipanteRepo)
	eventoRepo := new(mockInstEventoRepo)

	userID := "user-1"

	participRepo.On("FindParticipationIDsByUserID", mock.Anything, userID).
		Return([]string{}, nil)

	instExcluded := makeInstallment("inst-excl", "event-planning", userID, domain.InstallmentStatusPending)
	instVisible := makeInstallment("inst-vis", "event-published", userID, domain.InstallmentStatusPending)

	installRepo.On("FindByUserOrParticipations", mock.Anything, userID, []string{}, (*domain.InstallmentStatus)(nil)).
		Return([]*domain.PaymentInstallment{instExcluded, instVisible}, nil)

	// event-planning is in ORGANIZACAO → excluded.
	eventoRepo.On("FindByID", mock.Anything, "event-planning").
		Return(makeEvento("event-planning", "org-1", "ORGANIZACAO"), nil)

	// event-published is fine.
	eventoRepo.On("FindByID", mock.Anything, "event-published").
		Return(makeEvento("event-published", "org-2", "PUBLICADO"), nil)

	uc := ucpayment.NewListUserInstallments(installRepo, participRepo, eventoRepo)
	result, err := uc.Execute(context.Background(), userID, nil)

	require.NoError(t, err)
	require.Len(t, result, 1, "only the non-planning-phase installment should be returned")
	assert.Equal(t, "inst-vis", result[0].ID)
}

// TestListInstallments_FiltersAguardandoAceitePhase verifies AGUARDANDO_ACEITE is also excluded.
func TestListInstallments_FiltersAguardandoAceitePhase(t *testing.T) {
	installRepo := new(mockInstallmentRepo)
	participRepo := new(mockInstParticipanteRepo)
	eventoRepo := new(mockInstEventoRepo)

	userID := "user-2"
	participRepo.On("FindParticipationIDsByUserID", mock.Anything, userID).Return([]string{}, nil)

	inst := makeInstallment("inst-1", "event-aguardando", userID, domain.InstallmentStatusPending)
	installRepo.On("FindByUserOrParticipations", mock.Anything, userID, []string{}, (*domain.InstallmentStatus)(nil)).
		Return([]*domain.PaymentInstallment{inst}, nil)

	eventoRepo.On("FindByID", mock.Anything, "event-aguardando").
		Return(makeEvento("event-aguardando", "org-1", "AGUARDANDO_ACEITE"), nil)

	uc := ucpayment.NewListUserInstallments(installRepo, participRepo, eventoRepo)
	result, err := uc.Execute(context.Background(), userID, nil)

	require.NoError(t, err)
	assert.Empty(t, result, "installment from AGUARDANDO_ACEITE event must be filtered out")
}

// ─── TestListInstallments: access control ────────────────────────────────────

// TestListInstallments_NonParticipantGets403 verifies that a user who is not a
// participant of the event receives a 403 Forbidden.
func TestListInstallments_NonParticipantGets403(t *testing.T) {
	installRepo := new(mockInstallmentRepo)
	participRepo := new(mockInstParticipanteRepo)

	participRepo.On("IsParticipantOfEvent", mock.Anything, "event-1", "outsider").
		Return(false, nil)

	uc := ucpayment.NewListInstallments(installRepo, participRepo)
	_, err := uc.Execute(context.Background(), "outsider", portin.ListInstallmentsFilter{EventID: "event-1"})

	require.Error(t, err)
	var apiErr *apierr.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, 403, apiErr.Status)
}

// TestListInstallments_ParticipantCanList verifies a valid participant gets results.
func TestListInstallments_ParticipantCanList(t *testing.T) {
	installRepo := new(mockInstallmentRepo)
	participRepo := new(mockInstParticipanteRepo)

	participRepo.On("IsParticipantOfEvent", mock.Anything, "event-1", "user-1").Return(true, nil)

	inst := makeInstallment("inst-1", "event-1", "user-1", domain.InstallmentStatusPending)
	installRepo.On("FindByEvent", mock.Anything, "event-1", (*domain.InstallmentStatus)(nil)).
		Return([]*domain.PaymentInstallment{inst}, nil)

	uc := ucpayment.NewListInstallments(installRepo, participRepo)
	result, err := uc.Execute(context.Background(), "user-1", portin.ListInstallmentsFilter{EventID: "event-1"})

	require.NoError(t, err)
	require.Len(t, result, 1)
}

// ─── TestGetInstallment: 404 + 403 ───────────────────────────────────────────

// TestGetInstallment_NotFoundReturns404 verifies a missing installment returns 404.
func TestGetInstallment_NotFoundReturns404(t *testing.T) {
	installRepo := new(mockInstallmentRepo)
	participRepo := new(mockInstParticipanteRepo)

	installRepo.On("FindByID", mock.Anything, "missing").
		Return(nil, apierr.NotFound("payment_installment", "missing"))

	uc := ucpayment.NewGetInstallment(installRepo, participRepo)
	_, err := uc.Execute(context.Background(), "missing", "user-1")

	require.Error(t, err)
	var apiErr *apierr.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, 404, apiErr.Status)
}

// TestGetInstallment_NonParticipantGets403 verifies 403 for non-participants.
func TestGetInstallment_NonParticipantGets403(t *testing.T) {
	installRepo := new(mockInstallmentRepo)
	participRepo := new(mockInstParticipanteRepo)

	inst := makeInstallment("inst-1", "event-1", "owner-1", domain.InstallmentStatusPending)
	installRepo.On("FindByID", mock.Anything, "inst-1").Return(inst, nil)

	participRepo.On("IsParticipantOfEvent", mock.Anything, "event-1", "outsider").Return(false, nil)

	uc := ucpayment.NewGetInstallment(installRepo, participRepo)
	_, err := uc.Execute(context.Background(), "inst-1", "outsider")

	require.Error(t, err)
	var apiErr *apierr.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, 403, apiErr.Status)
}

// ─── TestCancelParticipantInstallments ───────────────────────────────────────

// TestCancelParticipantInstallments_OnlyOrganizerCanCancel verifies non-organizer
// gets 403.
func TestCancelParticipantInstallments_OnlyOrganizerCanCancel(t *testing.T) {
	installRepo := new(mockInstallmentRepo)
	participRepo := new(mockInstParticipanteRepo)
	eventoRepo := new(mockInstEventoRepo)

	eventoRepo.On("FindByID", mock.Anything, "event-1").
		Return(makeEvento("event-1", "organizer-1", "PUBLICADO"), nil)

	uc := ucpayment.NewCancelParticipantInstallments(installRepo, participRepo, eventoRepo)
	_, err := uc.Execute(context.Background(), portin.CancelParticipantInstallmentsInput{
		EventID:       "event-1",
		ParticipantID: "participant-1",
		RequesterID:   "someone-else", // not the organizer
	})

	require.Error(t, err)
	var apiErr *apierr.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, 403, apiErr.Status)
}

// TestCancelParticipantInstallments_OrganizerCancelsSuccessfully verifies the happy path
// and that only PENDING/OVERDUE installments are affected (enforced by the repo mock).
func TestCancelParticipantInstallments_OrganizerCancelsSuccessfully(t *testing.T) {
	installRepo := new(mockInstallmentRepo)
	participRepo := new(mockInstParticipanteRepo)
	eventoRepo := new(mockInstEventoRepo)

	eventoRepo.On("FindByID", mock.Anything, "event-1").
		Return(makeEvento("event-1", "organizer-1", "PUBLICADO"), nil)

	// CancelByParticipant in the repo only updates PENDING/OVERDUE; returns 2.
	installRepo.On("CancelByParticipant", mock.Anything, "event-1", "participant-1").
		Return(int64(2), nil)

	uc := ucpayment.NewCancelParticipantInstallments(installRepo, participRepo, eventoRepo)
	count, err := uc.Execute(context.Background(), portin.CancelParticipantInstallmentsInput{
		EventID:       "event-1",
		ParticipantID: "participant-1",
		RequesterID:   "organizer-1",
	})

	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	// Verify CancelByParticipant was called (only PENDING/OVERDUE at repo level).
	installRepo.AssertCalled(t, "CancelByParticipant", mock.Anything, "event-1", "participant-1")
	installRepo.AssertExpectations(t)
}
