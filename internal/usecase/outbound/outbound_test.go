package outbound_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	eventdomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/event"
	financedomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/finance"
	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/outbound"
	paymentdomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	usecase "github.com/role-organizado/backend-go-role-organizado/internal/usecase/outbound"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ─── Fakes ────────────────────────────────────────────────────────────────────

type fakeOutboundRepo struct {
	byID         map[string]*domain.OutboundRequest
	activeRateio map[string]bool
	saveCalls    int
	updateCalls  int
}

func newFakeOutboundRepo() *fakeOutboundRepo {
	return &fakeOutboundRepo{
		byID:         map[string]*domain.OutboundRequest{},
		activeRateio: map[string]bool{},
	}
}

func (f *fakeOutboundRepo) Save(ctx context.Context, r *domain.OutboundRequest) (*domain.OutboundRequest, error) {
	f.saveCalls++
	if r.ID == "" {
		r.ID = "req-test-1"
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now().UTC()
	}
	r.UpdatedAt = time.Now().UTC()
	copy := *r
	f.byID[r.ID] = &copy
	return r, nil
}

func (f *fakeOutboundRepo) Update(ctx context.Context, r *domain.OutboundRequest) (*domain.OutboundRequest, error) {
	f.updateCalls++
	if _, ok := f.byID[r.ID]; !ok {
		return nil, apierr.NotFound("outbound_request", r.ID)
	}
	r.UpdatedAt = time.Now().UTC()
	copy := *r
	f.byID[r.ID] = &copy
	return r, nil
}

func (f *fakeOutboundRepo) FindByID(ctx context.Context, id string) (*domain.OutboundRequest, error) {
	if r, ok := f.byID[id]; ok {
		c := *r
		return &c, nil
	}
	return nil, apierr.NotFound("outbound_request", id)
}

func (f *fakeOutboundRepo) FindByIDAndEventID(ctx context.Context, id, eventID string) (*domain.OutboundRequest, error) {
	r, err := f.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if r.EventID != eventID {
		return nil, apierr.NotFound("outbound_request", id)
	}
	return r, nil
}

func (f *fakeOutboundRepo) FindByEventID(ctx context.Context, eventID string) ([]domain.OutboundRequest, error) {
	out := []domain.OutboundRequest{}
	for _, r := range f.byID {
		if r.EventID == eventID {
			out = append(out, *r)
		}
	}
	return out, nil
}

func (f *fakeOutboundRepo) FindByEventIDAndStatus(ctx context.Context, eventID string, s domain.OutboundStatus) ([]domain.OutboundRequest, error) {
	out := []domain.OutboundRequest{}
	for _, r := range f.byID {
		if r.EventID == eventID && r.Status == s {
			out = append(out, *r)
		}
	}
	return out, nil
}

func (f *fakeOutboundRepo) FindByEventIDAndType(ctx context.Context, eventID string, t domain.OutboundType) ([]domain.OutboundRequest, error) {
	return nil, nil
}

func (f *fakeOutboundRepo) FindByRequesterUserID(ctx context.Context, userID string) ([]domain.OutboundRequest, error) {
	out := []domain.OutboundRequest{}
	for _, r := range f.byID {
		if r.RequesterUserID == userID {
			out = append(out, *r)
		}
	}
	return out, nil
}

func (f *fakeOutboundRepo) FindPendingByEventID(ctx context.Context, eventID string) ([]domain.OutboundRequest, error) {
	return f.FindByEventIDAndStatus(ctx, eventID, domain.StatusPending)
}

func (f *fakeOutboundRepo) CountPendingByEventID(ctx context.Context, eventID string) (int64, error) {
	count := int64(0)
	for _, r := range f.byID {
		if r.EventID == eventID && r.Status == domain.StatusPending {
			count++
		}
	}
	return count, nil
}

func (f *fakeOutboundRepo) ExistsActiveByRateioID(ctx context.Context, rateioID string) (bool, error) {
	return f.activeRateio[rateioID], nil
}

func (f *fakeOutboundRepo) FindByProviderTransferID(ctx context.Context, providerTransferID string) (*domain.OutboundRequest, error) {
	for _, r := range f.byID {
		if r.ProviderTransferID == providerTransferID {
			c := *r
			return &c, nil
		}
	}
	return nil, apierr.NotFound("outbound_request", providerTransferID)
}

func (f *fakeOutboundRepo) DeleteByID(ctx context.Context, id string) error {
	delete(f.byID, id)
	return nil
}

// ── audit log fake ──

type fakeAudit struct {
	entries []domain.AuditLog
}

func (f *fakeAudit) Append(ctx context.Context, e *domain.AuditLog) error {
	f.entries = append(f.entries, *e)
	return nil
}

func (f *fakeAudit) FindByRequestID(ctx context.Context, reqID string) ([]domain.AuditLog, error) {
	out := []domain.AuditLog{}
	for _, e := range f.entries {
		if e.RequestID == reqID {
			out = append(out, e)
		}
	}
	return out, nil
}

// ── evento fake ──

type fakeEventoRepo struct {
	byID map[string]*eventdomain.Evento
}

func (f *fakeEventoRepo) FindByID(ctx context.Context, id string) (*eventdomain.Evento, error) {
	if e, ok := f.byID[id]; ok {
		return e, nil
	}
	return nil, apierr.NotFound("evento", id)
}
func (f *fakeEventoRepo) FindByUsuarioID(ctx context.Context, uid string, p, s int) ([]eventdomain.Evento, int64, error) {
	return nil, 0, nil
}
func (f *fakeEventoRepo) FindByUsuarioIDCursor(ctx context.Context, uid string, fil portout.EventoQueryFiltros) (portout.EventosCursorPage, error) {
	return portout.EventosCursorPage{}, nil
}
func (f *fakeEventoRepo) FindAll(ctx context.Context, p, s int) ([]eventdomain.Evento, int64, error) {
	return nil, 0, nil
}
func (f *fakeEventoRepo) Save(ctx context.Context, e *eventdomain.Evento) (*eventdomain.Evento, error) {
	return e, nil
}
func (f *fakeEventoRepo) Update(ctx context.Context, e *eventdomain.Evento) (*eventdomain.Evento, error) {
	return e, nil
}
func (f *fakeEventoRepo) DeleteByID(ctx context.Context, id string) error                          { return nil }
func (f *fakeEventoRepo) AddConvidados(ctx context.Context, id string, c []eventdomain.Convidado) error { return nil }
func (f *fakeEventoRepo) FindAllByIDs(ctx context.Context, ids []string) ([]eventdomain.Evento, error) {
	return nil, nil
}
func (f *fakeEventoRepo) UpdateFase(ctx context.Context, id string, fase eventdomain.EventoFase) error {
	return nil
}
func (f *fakeEventoRepo) UpdatePoliticaConvidados(ctx context.Context, id, politica string) error {
	return nil
}
func (f *fakeEventoRepo) AddImagens(ctx context.Context, id string, imagens []eventdomain.EventoImagem) error {
	return nil
}
func (f *fakeEventoRepo) UpdateDetalhes(ctx context.Context, e *eventdomain.Evento) (*eventdomain.Evento, error) {
	return e, nil
}

// ── participant fake ──

type fakeParticipantRepo struct {
	byEventUser map[string]*financedomain.Participant
	byEvent     map[string][]financedomain.Participant
}

func (f *fakeParticipantRepo) FindByUserID(ctx context.Context, userID string) ([]financedomain.Participant, error) {
	return nil, nil
}

func (f *fakeParticipantRepo) FindByEventID(ctx context.Context, eventID string, page, size int) ([]financedomain.Participant, int64, error) {
	parts := f.byEvent[eventID]
	return parts, int64(len(parts)), nil
}

func (f *fakeParticipantRepo) FindByEventIDAndUserID(ctx context.Context, eventID, userID string) (*financedomain.Participant, error) {
	if p, ok := f.byEventUser[eventID+":"+userID]; ok {
		return p, nil
	}
	return nil, apierr.NotFound("participant", userID)
}

func (f *fakeParticipantRepo) FindAllByEventID(ctx context.Context, eventID string) ([]financedomain.Participant, error) {
	return f.byEvent[eventID], nil
}

// ── finance summary fake ──

type fakeFinanceSummaryRepo struct {
	byEvent map[string]*financedomain.FinanceSummary
}

func (f *fakeFinanceSummaryRepo) FindByEventID(ctx context.Context, eventID string) (*financedomain.FinanceSummary, error) {
	if s, ok := f.byEvent[eventID]; ok {
		return s, nil
	}
	return nil, apierr.NotFound("finance_summary", eventID)
}
func (f *fakeFinanceSummaryRepo) Save(ctx context.Context, s *financedomain.FinanceSummary) (*financedomain.FinanceSummary, error) {
	return s, nil
}
func (f *fakeFinanceSummaryRepo) Update(ctx context.Context, s *financedomain.FinanceSummary) (*financedomain.FinanceSummary, error) {
	return s, nil
}

// ── payment account fake ──

type fakePaymentAccountRepo struct {
	byID map[string]*paymentdomain.PaymentAccount
}

func (f *fakePaymentAccountRepo) FindByID(ctx context.Context, id string) (*paymentdomain.PaymentAccount, error) {
	if a, ok := f.byID[id]; ok {
		return a, nil
	}
	return nil, apierr.NotFound("payment_account", id)
}
func (f *fakePaymentAccountRepo) FindByUserID(ctx context.Context, userID string) ([]*paymentdomain.PaymentAccount, error) {
	return nil, nil
}
func (f *fakePaymentAccountRepo) FindDefaultByUserID(ctx context.Context, userID string) (*paymentdomain.PaymentAccount, error) {
	return nil, nil
}
func (f *fakePaymentAccountRepo) Save(ctx context.Context, a *paymentdomain.PaymentAccount) error   { return nil }
func (f *fakePaymentAccountRepo) Update(ctx context.Context, a *paymentdomain.PaymentAccount) error { return nil }
func (f *fakePaymentAccountRepo) SetDefault(ctx context.Context, userID, accountID string) error    { return nil }
func (f *fakePaymentAccountRepo) DeleteByID(ctx context.Context, id string) error                   { return nil }

// ── outbound transfer provider fake ──

type fakeProvider struct {
	enabled   bool
	fail      bool
	failError error
	calls     int
}

func (f *fakeProvider) IsEnabled() bool { return f.enabled }
func (f *fakeProvider) ExecuteTransfer(ctx context.Context, req *portout.OutboundTransferRequest) (*portout.OutboundTransferResponse, error) {
	f.calls++
	if f.fail {
		return &portout.OutboundTransferResponse{
			Success:      false,
			Provider:     "TEST",
			ErrorMessage: "boom",
		}, nil
	}
	if f.failError != nil {
		return nil, f.failError
	}
	return &portout.OutboundTransferResponse{
		Success:            true,
		Provider:           "TEST",
		ProviderTransferID: "tx-fake-1",
		Status:             "PENDING",
	}, nil
}

// ─── Test fixtures ────────────────────────────────────────────────────────────

func makeStandardFixtures(eventID, ownerID string) (*fakeOutboundRepo, *fakeEventoRepo, *fakeParticipantRepo, *fakeAudit) {
	repo := newFakeOutboundRepo()
	events := &fakeEventoRepo{byID: map[string]*eventdomain.Evento{
		eventID: {ID: eventID, UsuarioID: ownerID},
	}}
	parts := &fakeParticipantRepo{
		byEventUser: map[string]*financedomain.Participant{
			eventID + ":" + ownerID: {ID: "p-owner", EventID: eventID, UserID: ownerID, Status: "ORGANIZADOR"},
		},
		byEvent: map[string][]financedomain.Participant{
			eventID: {{ID: "p-owner", UserID: ownerID, Status: "ORGANIZADOR"}},
		},
	}
	audit := &fakeAudit{}
	return repo, events, parts, audit
}

// ─── Tests ────────────────────────────────────────────────────────────────────

func TestCreateOutbound_HappyPath_AutoApproved_SoleParticipant(t *testing.T) {
	repo, events, parts, audit := makeStandardFixtures("evt-1", "owner")
	provider := &fakeProvider{enabled: true}
	uc := usecase.NewCreateOutboundRequest(
		repo, events, parts,
		&fakeFinanceSummaryRepo{}, &fakePaymentAccountRepo{}, provider, audit,
	)
	res, err := uc.Execute(context.Background(), portin.CreateOutboundRequestInput{
		UserID:        "owner",
		EventID:       "evt-1",
		Type:          domain.TypeSupplierPayment,
		AmountCents:   5000,
		RateioID:      "rateio-1",
		Justification: "Pagamento fornecedor X",
		RecipientName: "Forn X",
		PixKeyType:    domain.PixKeyTypeEmail,
		PixKey:        "fx@example.com",
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	// Sole participant → ConfigureApproval set AUTO; provider was called → PROCESSING.
	assert.Equal(t, domain.ApprovalModeAuto, res.ApprovalMode)
	assert.False(t, res.RequiresVoting)
	assert.Equal(t, domain.StatusProcessing, res.Status)
	assert.Equal(t, 1, provider.calls)
}

func TestCreateOutbound_MissingRateio_Returns422(t *testing.T) {
	repo, events, parts, audit := makeStandardFixtures("evt-1", "owner")
	uc := usecase.NewCreateOutboundRequest(
		repo, events, parts,
		&fakeFinanceSummaryRepo{}, &fakePaymentAccountRepo{}, &fakeProvider{enabled: true}, audit,
	)
	_, err := uc.Execute(context.Background(), portin.CreateOutboundRequestInput{
		UserID:        "owner",
		EventID:       "evt-1",
		Type:          domain.TypeSupplierPayment,
		AmountCents:   5000,
		RecipientName: "Forn X",
		PixKeyType:    domain.PixKeyTypeEmail,
		PixKey:        "fx@example.com",
	})
	require.Error(t, err)
	var ae *apierr.APIError
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, 422, ae.Status)
}

func TestCreateOutbound_ActiveDuplicate_Returns422(t *testing.T) {
	repo, events, parts, audit := makeStandardFixtures("evt-1", "owner")
	repo.activeRateio["rateio-1"] = true
	uc := usecase.NewCreateOutboundRequest(
		repo, events, parts,
		&fakeFinanceSummaryRepo{}, &fakePaymentAccountRepo{}, &fakeProvider{enabled: true}, audit,
	)
	_, err := uc.Execute(context.Background(), portin.CreateOutboundRequestInput{
		UserID:        "owner",
		EventID:       "evt-1",
		Type:          domain.TypeSupplierPayment,
		AmountCents:   100,
		RateioID:      "rateio-1",
		RecipientName: "Forn",
		PixKeyType:    domain.PixKeyTypeEmail,
		PixKey:        "fx@example.com",
	})
	require.Error(t, err)
	var ae *apierr.APIError
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, 422, ae.Status)
}

func TestCreateOutbound_VotingRequired_WithMultipleParticipants(t *testing.T) {
	repo, events, parts, audit := makeStandardFixtures("evt-1", "owner")
	// Add 2 other participants → eligible voters = 2 → ALL_PARTICIPANTS mode.
	parts.byEvent["evt-1"] = []financedomain.Participant{
		{UserID: "owner", Status: "ORGANIZADOR"},
		{UserID: "voter-a", Status: "ORGANIZADOR"},
		{UserID: "voter-b", Status: "ORGANIZADOR"},
	}
	provider := &fakeProvider{enabled: true}
	uc := usecase.NewCreateOutboundRequest(
		repo, events, parts,
		&fakeFinanceSummaryRepo{}, &fakePaymentAccountRepo{}, provider, audit,
	)
	res, err := uc.Execute(context.Background(), portin.CreateOutboundRequestInput{
		UserID:        "owner",
		EventID:       "evt-1",
		Type:          domain.TypeSupplierPayment,
		AmountCents:   500,
		RateioID:      "rateio-1",
		RecipientName: "Forn",
		PixKeyType:    domain.PixKeyTypeEmail,
		PixKey:        "fx@example.com",
	})
	require.NoError(t, err)
	assert.True(t, res.RequiresVoting)
	assert.Equal(t, domain.ApprovalModeAllParticipants, res.ApprovalMode)
	assert.Equal(t, 2, res.RequiredVotes)
	assert.Equal(t, domain.StatusPending, res.Status)
	assert.Equal(t, 0, provider.calls, "provider should not run until voting resolves")
}

func TestVote_AllParticipantsMode_BothApprove_ApprovesAndExecutes(t *testing.T) {
	repo, events, parts, audit := makeStandardFixtures("evt-1", "owner")
	parts.byEvent["evt-1"] = []financedomain.Participant{
		{UserID: "owner", Status: "ORGANIZADOR"},
		{UserID: "voter-a", Status: "ORGANIZADOR"},
		{UserID: "voter-b", Status: "ORGANIZADOR"},
	}
	parts.byEventUser["evt-1:voter-a"] = &financedomain.Participant{UserID: "voter-a", Status: "ORGANIZADOR"}
	parts.byEventUser["evt-1:voter-b"] = &financedomain.Participant{UserID: "voter-b", Status: "ORGANIZADOR"}

	createUC := usecase.NewCreateOutboundRequest(
		repo, events, parts,
		&fakeFinanceSummaryRepo{}, &fakePaymentAccountRepo{}, &fakeProvider{}, audit,
	)
	created, err := createUC.Execute(context.Background(), portin.CreateOutboundRequestInput{
		UserID:        "owner",
		EventID:       "evt-1",
		Type:          domain.TypeSupplierPayment,
		AmountCents:   500,
		RateioID:      "rateio-1",
		RecipientName: "Forn",
		PixKeyType:    domain.PixKeyTypeEmail,
		PixKey:        "fx@example.com",
	})
	require.NoError(t, err)

	provider := &fakeProvider{enabled: true}
	voteUC := usecase.NewVoteOnOutboundRequest(repo, events, parts, provider, audit)

	// First approval — still PENDING.
	res, err := voteUC.Execute(context.Background(), portin.VoteOnOutboundRequestInput{
		RequestID: created.ID, EventID: created.EventID, UserID: "voter-a", Approve: true,
	})
	require.NoError(t, err)
	assert.Equal(t, domain.StatusPending, res.FinalStatus)
	assert.Equal(t, 1, res.ApproveVotes)

	// Second approval — quorum reached → APPROVED → PROCESSING via provider.
	res, err = voteUC.Execute(context.Background(), portin.VoteOnOutboundRequestInput{
		RequestID: created.ID, EventID: created.EventID, UserID: "voter-b", Approve: true,
	})
	require.NoError(t, err)
	assert.Equal(t, domain.StatusProcessing, res.FinalStatus)
	assert.Equal(t, 1, provider.calls)
}

func TestVote_AllParticipantsMode_AnyReject_ImmediateRejection(t *testing.T) {
	repo, events, parts, audit := makeStandardFixtures("evt-1", "owner")
	parts.byEvent["evt-1"] = []financedomain.Participant{
		{UserID: "owner", Status: "ORGANIZADOR"},
		{UserID: "voter-a", Status: "ORGANIZADOR"},
		{UserID: "voter-b", Status: "ORGANIZADOR"},
	}
	parts.byEventUser["evt-1:voter-a"] = &financedomain.Participant{UserID: "voter-a", Status: "ORGANIZADOR"}

	createUC := usecase.NewCreateOutboundRequest(
		repo, events, parts,
		&fakeFinanceSummaryRepo{}, &fakePaymentAccountRepo{}, &fakeProvider{}, audit,
	)
	created, err := createUC.Execute(context.Background(), portin.CreateOutboundRequestInput{
		UserID:        "owner",
		EventID:       "evt-1",
		Type:          domain.TypeSupplierPayment,
		AmountCents:   500,
		RateioID:      "rateio-1",
		RecipientName: "Forn",
		PixKeyType:    domain.PixKeyTypeEmail,
		PixKey:        "fx@example.com",
	})
	require.NoError(t, err)

	voteUC := usecase.NewVoteOnOutboundRequest(repo, events, parts, &fakeProvider{enabled: true}, audit)
	res, err := voteUC.Execute(context.Background(), portin.VoteOnOutboundRequestInput{
		RequestID: created.ID, EventID: created.EventID, UserID: "voter-a", Approve: false,
	})
	require.NoError(t, err)
	assert.Equal(t, domain.StatusRejected, res.FinalStatus)
}

func TestVote_RequesterCannotVoteOnOwnRequest(t *testing.T) {
	repo, events, parts, audit := makeStandardFixtures("evt-1", "owner")
	parts.byEvent["evt-1"] = []financedomain.Participant{
		{UserID: "owner", Status: "ORGANIZADOR"},
		{UserID: "voter-a", Status: "ORGANIZADOR"},
	}
	createUC := usecase.NewCreateOutboundRequest(
		repo, events, parts,
		&fakeFinanceSummaryRepo{}, &fakePaymentAccountRepo{}, &fakeProvider{}, audit,
	)
	created, _ := createUC.Execute(context.Background(), portin.CreateOutboundRequestInput{
		UserID:        "owner",
		EventID:       "evt-1",
		Type:          domain.TypeSupplierPayment,
		AmountCents:   500,
		RateioID:      "rateio-1",
		RecipientName: "Forn",
		PixKeyType:    domain.PixKeyTypeEmail,
		PixKey:        "fx@example.com",
	})
	voteUC := usecase.NewVoteOnOutboundRequest(repo, events, parts, &fakeProvider{enabled: true}, audit)
	_, err := voteUC.Execute(context.Background(), portin.VoteOnOutboundRequestInput{
		RequestID: created.ID, EventID: created.EventID, UserID: "owner", Approve: true,
	})
	require.Error(t, err)
	var ae *apierr.APIError
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, 422, ae.Status)
}

func TestCancel_OnlyRequesterCanCancel(t *testing.T) {
	repo := newFakeOutboundRepo()
	now := time.Now()
	req := &domain.OutboundRequest{
		ID:              "r1",
		EventID:         "evt-1",
		RequesterUserID: "owner",
		Status:          domain.StatusPending,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	repo.byID["r1"] = req
	uc := usecase.NewCancelOutboundRequest(repo, &fakeAudit{})

	// Non-owner → 404 (anti-enumeration).
	_, err := uc.Execute(context.Background(), portin.CancelOutboundRequestInput{
		RequestID: "r1", UserID: "intruder",
	})
	require.Error(t, err)
	var ae *apierr.APIError
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, 404, ae.Status)

	// Owner → success.
	updated, err := uc.Execute(context.Background(), portin.CancelOutboundRequestInput{
		RequestID: "r1", UserID: "owner", CancellationReason: "mudei de ideia",
	})
	require.NoError(t, err)
	assert.Equal(t, domain.StatusCancelled, updated.Status)
}

func TestHandleCallback_Idempotent_AlreadyTerminal(t *testing.T) {
	repo := newFakeOutboundRepo()
	now := time.Now()
	repo.byID["r1"] = &domain.OutboundRequest{
		ID:        "r1",
		EventID:   "evt-1",
		Status:    domain.StatusCompleted, // already terminal
		CreatedAt: now,
		UpdatedAt: now,
		CompletedAt: &now,
	}
	uc := usecase.NewHandleOutboundTransferCallback(repo, &fakeAudit{})
	res, err := uc.Execute(context.Background(), portin.OutboundCallbackInput{
		RequestID:      "r1",
		Provider:       "ASAAS",
		ProviderStatus: "DONE",
	})
	require.NoError(t, err)
	assert.Equal(t, domain.StatusCompleted, res.Status)
	assert.Equal(t, 0, repo.updateCalls, "should not write again for already-terminal requests")
}

func TestHandleCallback_TerminalFailure_TransitionsFailed(t *testing.T) {
	repo := newFakeOutboundRepo()
	now := time.Now()
	repo.byID["r1"] = &domain.OutboundRequest{
		ID:        "r1",
		EventID:   "evt-1",
		Status:    domain.StatusProcessing,
		CreatedAt: now,
		UpdatedAt: now,
	}
	uc := usecase.NewHandleOutboundTransferCallback(repo, &fakeAudit{})
	res, err := uc.Execute(context.Background(), portin.OutboundCallbackInput{
		RequestID:      "r1",
		Provider:       "ASAAS",
		ProviderStatus: "FAILED",
		Reason:         "insufficient funds",
	})
	require.NoError(t, err)
	assert.Equal(t, domain.StatusFailed, res.Status)
}

func TestHandleCallback_NonTerminalStatus_NoOp(t *testing.T) {
	repo := newFakeOutboundRepo()
	now := time.Now()
	repo.byID["r1"] = &domain.OutboundRequest{
		ID:        "r1",
		EventID:   "evt-1",
		Status:    domain.StatusProcessing,
		CreatedAt: now,
		UpdatedAt: now,
	}
	uc := usecase.NewHandleOutboundTransferCallback(repo, &fakeAudit{})
	res, err := uc.Execute(context.Background(), portin.OutboundCallbackInput{
		RequestID:      "r1",
		Provider:       "ASAAS",
		ProviderStatus: "PENDING",
	})
	require.NoError(t, err)
	assert.Equal(t, domain.StatusProcessing, res.Status)
	assert.Equal(t, 0, repo.updateCalls)
}

func TestApprove_NonOrganizer_Returns404(t *testing.T) {
	repo, events, parts, audit := makeStandardFixtures("evt-1", "owner")
	now := time.Now()
	repo.byID["r1"] = &domain.OutboundRequest{
		ID: "r1", EventID: "evt-1", Status: domain.StatusPending,
		CreatedAt: now, UpdatedAt: now,
	}
	uc := usecase.NewApproveOutboundRequest(repo, events, parts, &fakeProvider{enabled: true}, audit)
	_, err := uc.Execute(context.Background(), portin.ApproveOutboundRequestInput{
		RequestID: "r1", ApproverUserID: "stranger",
	})
	require.Error(t, err)
	var ae *apierr.APIError
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, 404, ae.Status, "anti-enumeration: non-organizer must see 404")
}

func TestReject_RequiresReason(t *testing.T) {
	repo, events, parts, audit := makeStandardFixtures("evt-1", "owner")
	now := time.Now()
	repo.byID["r1"] = &domain.OutboundRequest{
		ID: "r1", EventID: "evt-1", Status: domain.StatusPending,
		CreatedAt: now, UpdatedAt: now,
	}
	uc := usecase.NewRejectOutboundRequest(repo, events, parts, audit)
	_, err := uc.Execute(context.Background(), portin.RejectOutboundRequestInput{
		RequestID: "r1", RejecterUserID: "owner", RejectionReason: "",
	})
	require.Error(t, err)
	var ae *apierr.APIError
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, 422, ae.Status)
}

// Sanity: silence linter on unused identifiers, e.g. errors.Is.
var _ = errors.Is
