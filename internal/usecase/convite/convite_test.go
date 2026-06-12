package convite_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authdomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/auth"
	convitedomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/convite"
	domainevent "github.com/role-organizado/backend-go-role-organizado/internal/domain/event"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	ucconvite "github.com/role-organizado/backend-go-role-organizado/internal/usecase/convite"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// =================================================================
// Lightweight stub repositories — no external mock framework.
// =================================================================

type stubParticipantRepo struct {
	byID                map[string]*convitedomain.Participant
	byCompositeKey      map[string]*convitedomain.Participant
	byPapel             map[string][]convitedomain.Participant
	updateStatusErr     error
	updateStatusCalls   int
	updateStatusLast    convitedomain.ParticipantStatus
	bindUsuarioErr      error
	findByIDErr         error
	saveCalls           int
}

func (s *stubParticipantRepo) FindByID(_ context.Context, id string) (*convitedomain.Participant, error) {
	if s.findByIDErr != nil {
		return nil, s.findByIDErr
	}
	if p, ok := s.byID[id]; ok {
		return p, nil
	}
	return nil, apierr.NotFound("participant", id)
}
func (s *stubParticipantRepo) FindByEventoIDAndUsuarioID(_ context.Context, eventoID, usuarioID string) (*convitedomain.Participant, error) {
	key := eventoID + ":" + usuarioID
	if p, ok := s.byCompositeKey[key]; ok {
		return p, nil
	}
	return nil, nil
}
func (s *stubParticipantRepo) FindByEventoIDAndPapel(_ context.Context, eventoID string, papel convitedomain.Papel) ([]convitedomain.Participant, error) {
	if s.byPapel == nil {
		return nil, nil
	}
	return s.byPapel[eventoID+":"+string(papel)], nil
}
func (s *stubParticipantRepo) Save(_ context.Context, p *convitedomain.Participant) (*convitedomain.Participant, error) {
	s.saveCalls++
	return p, nil
}
func (s *stubParticipantRepo) UpdateStatus(_ context.Context, _ string, status convitedomain.ParticipantStatus, _ *time.Time) error {
	s.updateStatusCalls++
	s.updateStatusLast = status
	return s.updateStatusErr
}
func (s *stubParticipantRepo) BindUsuarioID(_ context.Context, _, _ string) error {
	return s.bindUsuarioErr
}

type stubApprovalRepo struct {
	byID            map[string]*convitedomain.ApprovalItem
	pendingByTarget map[string]*convitedomain.ApprovalItem
	existsPending   map[string]bool
	saveCalls       int
	saveErr         error
	updateStatusErr error
	updateStatusLast convitedomain.ApprovalItemStatus
}

func (s *stubApprovalRepo) FindByID(_ context.Context, id string) (*convitedomain.ApprovalItem, error) {
	if a, ok := s.byID[id]; ok {
		return a, nil
	}
	return nil, apierr.NotFound("approval", id)
}
func (s *stubApprovalRepo) FindLatestByTargetEntityIDAndType(_ context.Context, t string, _ convitedomain.ApprovalItemType) (*convitedomain.ApprovalItem, error) {
	return s.pendingByTarget[t], nil
}
func (s *stubApprovalRepo) FindPendingByTargetEntityID(_ context.Context, t string) (*convitedomain.ApprovalItem, error) {
	return s.pendingByTarget[t], nil
}
func (s *stubApprovalRepo) ExistsPendingByTargetEntityID(_ context.Context, t string) (bool, error) {
	return s.existsPending[t], nil
}
func (s *stubApprovalRepo) Save(_ context.Context, a *convitedomain.ApprovalItem) (*convitedomain.ApprovalItem, error) {
	s.saveCalls++
	return a, s.saveErr
}
func (s *stubApprovalRepo) UpdateStatus(_ context.Context, _ string, st convitedomain.ApprovalItemStatus, _, _ string, _ time.Time) error {
	s.updateStatusLast = st
	return s.updateStatusErr
}

type stubEventoRepo struct {
	byID map[string]*domainevent.Evento
}

func (s *stubEventoRepo) FindByID(_ context.Context, id string) (*domainevent.Evento, error) {
	if e, ok := s.byID[id]; ok {
		return e, nil
	}
	return nil, apierr.NotFound("evento", id)
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
func (s *stubEventoRepo) UpdatePoliticaConvidados(_ context.Context, _, _ string) error { return nil }
func (s *stubEventoRepo) AddImagens(_ context.Context, _ string, _ []domainevent.EventoImagem) error {
	return nil
}
func (s *stubEventoRepo) UpdateDetalhes(_ context.Context, e *domainevent.Evento) (*domainevent.Evento, error) {
	return e, nil
}

type stubUsuarioRepo struct {
	byID map[string]*authdomain.Usuario
}

func (s *stubUsuarioRepo) FindByID(_ context.Context, id string) (*authdomain.Usuario, error) {
	if u, ok := s.byID[id]; ok {
		return u, nil
	}
	return nil, apierr.NotFound("usuario", id)
}
func (s *stubUsuarioRepo) FindByEmail(_ context.Context, _ string) (*authdomain.Usuario, error) {
	return nil, nil
}
func (s *stubUsuarioRepo) FindByProviderID(_ context.Context, _, _ string) (*authdomain.Usuario, error) {
	return nil, nil
}
func (s *stubUsuarioRepo) Save(_ context.Context, u *authdomain.Usuario) (*authdomain.Usuario, error) {
	return u, nil
}
func (s *stubUsuarioRepo) Update(_ context.Context, u *authdomain.Usuario) (*authdomain.Usuario, error) {
	return u, nil
}
func (s *stubUsuarioRepo) FindAll(_ context.Context, _, _ int) ([]authdomain.Usuario, int64, error) {
	return nil, 0, nil
}
func (s *stubUsuarioRepo) DeleteByID(_ context.Context, _ string) error { return nil }

type stubGuestRepo struct{}

func (s *stubGuestRepo) FindByID(_ context.Context, _ string) (*convitedomain.Guest, error) {
	return nil, nil
}
func (s *stubGuestRepo) FindByTelefoneOrEmail(_ context.Context, _, _ string) (*convitedomain.Guest, error) {
	return nil, nil
}
func (s *stubGuestRepo) Save(_ context.Context, g *convitedomain.Guest) (*convitedomain.Guest, error) {
	return g, nil
}

type stubCreditRepo struct{ id string }

func (s *stubCreditRepo) Save(_ context.Context, c portout.ParticipantCredit) (string, error) {
	if s.id == "" {
		s.id = c.ID
	}
	return s.id, nil
}

type stubInstallmentRepo struct {
	items     []portout.ConviteInstallment
	cancelled int64
}

func (s *stubInstallmentRepo) FindByParticipantID(_ context.Context, _, _ string) ([]portout.ConviteInstallment, error) {
	return s.items, nil
}
func (s *stubInstallmentRepo) CancelPendingByParticipantID(_ context.Context, _, _ string) (int64, error) {
	s.cancelled++
	return s.cancelled, nil
}

type stubAuditRepo struct{ count int }

func (s *stubAuditRepo) Save(_ context.Context, _ portout.ConviteAuditEntry) error {
	s.count++
	return nil
}

type stubNotifPort struct {
	called     int
	messageID  string
	err        error
	lastCanal  string
}

func (s *stubNotifPort) PublicarConvite(_ context.Context, in portout.ConvitePublishInput) (string, error) {
	s.called++
	s.lastCanal = in.Canal
	return s.messageID, s.err
}

type stubTemporal struct{ calls int }

func (s *stubTemporal) StartWorkflow(_ context.Context, _ portout.WorkflowStartOptions, _ interface{}, _ ...interface{}) error {
	s.calls++
	return nil
}

// =================================================================
// BuscarConvite
// =================================================================

func TestBuscarConvite_FromParticipant_Success(t *testing.T) {
	p := &convitedomain.Participant{
		ID:               "p-1",
		EventoID:         "e-1",
		Nome:             "Alice",
		Email:            "a@x.io",
		TipoParticipante: convitedomain.TipoGuest,
		Status:           convitedomain.StatusPendente,
	}
	pr := &stubParticipantRepo{byID: map[string]*convitedomain.Participant{"p-1": p}}
	er := &stubEventoRepo{byID: map[string]*domainevent.Evento{"e-1": {
		ID: "e-1", Nome: "Festa", UsuarioID: "u-org", Data: time.Now().Add(24 * time.Hour),
	}}}
	ur := &stubUsuarioRepo{byID: map[string]*authdomain.Usuario{"u-org": {ID: "u-org", Nome: "Bob"}}}
	uc := ucconvite.NewBuscarConvite(pr, &stubApprovalRepo{}, er, ur, &stubGuestRepo{})

	res, err := uc.Execute(context.Background(), "p-1")
	require.NoError(t, err)
	assert.Equal(t, "p-1", res.ParticipantID)
	assert.Equal(t, "Alice", res.ConvidadoNome)
	assert.Equal(t, "Festa", res.EventoNome)
	assert.Equal(t, "Bob", res.OrganizadorNome)
	assert.False(t, res.EventoPassado)
}

func TestBuscarConvite_FallbackToApproval_Lazy(t *testing.T) {
	pr := &stubParticipantRepo{byID: map[string]*convitedomain.Participant{}}
	ar := &stubApprovalRepo{
		byID: map[string]*convitedomain.ApprovalItem{
			"a-1": {
				ID:                      "a-1",
				Type:                    convitedomain.ApprovalTypeInvite,
				Status:                  convitedomain.ApprovalStatusPending,
				EventID:                 "e-1",
				MaterializationStrategy: convitedomain.MaterializationLazyOnApproval,
				Metadata:                map[string]any{"nome": "Carol"},
			},
		},
	}
	er := &stubEventoRepo{byID: map[string]*domainevent.Evento{"e-1": {ID: "e-1", Nome: "Festa"}}}
	uc := ucconvite.NewBuscarConvite(pr, ar, er, &stubUsuarioRepo{}, &stubGuestRepo{})

	res, err := uc.Execute(context.Background(), "a-1")
	require.NoError(t, err)
	assert.Equal(t, "a-1", res.ParticipantID)
	assert.Equal(t, "Carol", res.ConvidadoNome)
	assert.Equal(t, string(convitedomain.StatusPendente), res.Status)
}

func TestBuscarConvite_NotFound(t *testing.T) {
	uc := ucconvite.NewBuscarConvite(&stubParticipantRepo{byID: map[string]*convitedomain.Participant{}},
		&stubApprovalRepo{byID: map[string]*convitedomain.ApprovalItem{}},
		&stubEventoRepo{}, &stubUsuarioRepo{}, &stubGuestRepo{})
	_, err := uc.Execute(context.Background(), "missing")
	require.Error(t, err)
	assert.True(t, apierr.IsNotFound(err))
}

// =================================================================
// EnviarConvite
// =================================================================

func TestEnviarConvite_Success(t *testing.T) {
	p := &convitedomain.Participant{
		ID: "p-1", EventoID: "e-1", Nome: "Alice", Email: "a@x.io", Telefone: "11 99999-0000",
	}
	pr := &stubParticipantRepo{byID: map[string]*convitedomain.Participant{"p-1": p}}
	er := &stubEventoRepo{byID: map[string]*domainevent.Evento{"e-1": {
		ID: "e-1", UsuarioID: "u-org", Nome: "Festa", Data: time.Now().Add(24 * time.Hour),
	}}}
	notif := &stubNotifPort{messageID: "msg-99"}
	uc := ucconvite.NewEnviarConvite(pr, &stubApprovalRepo{}, er, &stubUsuarioRepo{}, notif)

	res, err := uc.Execute(context.Background(), portin.EnviarConviteInput{
		ParticipantID: "p-1", OrganizadorID: "u-org",
	})
	require.NoError(t, err)
	assert.True(t, res.Aceito)
	assert.Equal(t, "msg-99", res.MessageID)
	assert.Equal(t, string(convitedomain.CanalWhatsAppFallbackEmail), res.Canal)
	assert.Equal(t, 1, notif.called)
	assert.Equal(t, string(convitedomain.CanalWhatsAppFallbackEmail), notif.lastCanal)
}

func TestEnviarConvite_NoChannelsRaisesUnprocessable(t *testing.T) {
	p := &convitedomain.Participant{ID: "p-1", EventoID: "e-1", Nome: "Alice"}
	pr := &stubParticipantRepo{byID: map[string]*convitedomain.Participant{"p-1": p}}
	er := &stubEventoRepo{byID: map[string]*domainevent.Evento{"e-1": {ID: "e-1", UsuarioID: "u-org"}}}
	uc := ucconvite.NewEnviarConvite(pr, &stubApprovalRepo{}, er, &stubUsuarioRepo{}, &stubNotifPort{})

	_, err := uc.Execute(context.Background(), portin.EnviarConviteInput{
		ParticipantID: "p-1", OrganizadorID: "u-org",
	})
	require.Error(t, err)
	var apiErr *apierr.APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, 422, apiErr.Status)
}

func TestEnviarConvite_NotOrganizerForbidden(t *testing.T) {
	p := &convitedomain.Participant{ID: "p-1", EventoID: "e-1", Email: "a@x.io"}
	pr := &stubParticipantRepo{byID: map[string]*convitedomain.Participant{"p-1": p}}
	er := &stubEventoRepo{byID: map[string]*domainevent.Evento{"e-1": {ID: "e-1", UsuarioID: "u-org"}}}
	uc := ucconvite.NewEnviarConvite(pr, &stubApprovalRepo{}, er, &stubUsuarioRepo{}, &stubNotifPort{})

	_, err := uc.Execute(context.Background(), portin.EnviarConviteInput{
		ParticipantID: "p-1", OrganizadorID: "u-stranger",
	})
	require.Error(t, err)
	var apiErr *apierr.APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, 403, apiErr.Status)
}

// =================================================================
// ConfirmarConvite
// =================================================================

func TestConfirmarConvite_Idempotent(t *testing.T) {
	p := &convitedomain.Participant{
		ID: "p-1", EventoID: "e-1", TipoParticipante: convitedomain.TipoGuest,
		Status: convitedomain.StatusConfirmado,
	}
	pr := &stubParticipantRepo{byID: map[string]*convitedomain.Participant{"p-1": p}}
	er := &stubEventoRepo{byID: map[string]*domainevent.Evento{"e-1": {ID: "e-1"}}}
	buscar := ucconvite.NewBuscarConvite(pr, &stubApprovalRepo{}, er, &stubUsuarioRepo{}, &stubGuestRepo{})
	temp := &stubTemporal{}
	uc := ucconvite.NewConfirmarConvite(pr, &stubApprovalRepo{}, buscar, temp)

	res, err := uc.Execute(context.Background(), "p-1")
	require.NoError(t, err)
	assert.Equal(t, string(convitedomain.StatusConfirmado), res.Status)
	assert.Equal(t, 0, pr.updateStatusCalls) // idempotent — no update
	assert.Equal(t, 0, temp.calls)
}

func TestConfirmarConvite_GuestSuccess_StartsTemporal(t *testing.T) {
	p := &convitedomain.Participant{
		ID: "p-1", EventoID: "e-1", TipoParticipante: convitedomain.TipoGuest,
		Status: convitedomain.StatusPendente,
	}
	pr := &stubParticipantRepo{byID: map[string]*convitedomain.Participant{"p-1": p}}
	er := &stubEventoRepo{byID: map[string]*domainevent.Evento{"e-1": {ID: "e-1"}}}
	buscar := ucconvite.NewBuscarConvite(pr, &stubApprovalRepo{}, er, &stubUsuarioRepo{}, &stubGuestRepo{})
	temp := &stubTemporal{}
	uc := ucconvite.NewConfirmarConvite(pr, &stubApprovalRepo{}, buscar, temp)

	_, err := uc.Execute(context.Background(), "p-1")
	require.NoError(t, err)
	assert.Equal(t, 1, pr.updateStatusCalls)
	assert.Equal(t, convitedomain.StatusConfirmado, pr.updateStatusLast)
	assert.Equal(t, 1, temp.calls)
}

func TestConfirmarConvite_UserMustUseApprovalCenter(t *testing.T) {
	p := &convitedomain.Participant{
		ID: "p-1", EventoID: "e-1", TipoParticipante: convitedomain.TipoUser,
		Status: convitedomain.StatusPendente,
	}
	pr := &stubParticipantRepo{byID: map[string]*convitedomain.Participant{"p-1": p}}
	er := &stubEventoRepo{byID: map[string]*domainevent.Evento{"e-1": {ID: "e-1"}}}
	buscar := ucconvite.NewBuscarConvite(pr, &stubApprovalRepo{}, er, &stubUsuarioRepo{}, &stubGuestRepo{})
	uc := ucconvite.NewConfirmarConvite(pr, &stubApprovalRepo{}, buscar, &stubTemporal{})
	_, err := uc.Execute(context.Background(), "p-1")
	require.Error(t, err)
	var apiErr *apierr.APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, 403, apiErr.Status)
}

// =================================================================
// RecusarConvite
// =================================================================

func TestRecusarConvite_GuestSuccess(t *testing.T) {
	p := &convitedomain.Participant{
		ID: "p-1", EventoID: "e-1", TipoParticipante: convitedomain.TipoGuest,
		Status: convitedomain.StatusPendente,
	}
	pr := &stubParticipantRepo{byID: map[string]*convitedomain.Participant{"p-1": p}}
	er := &stubEventoRepo{byID: map[string]*domainevent.Evento{"e-1": {ID: "e-1"}}}
	buscar := ucconvite.NewBuscarConvite(pr, &stubApprovalRepo{}, er, &stubUsuarioRepo{}, &stubGuestRepo{})
	temp := &stubTemporal{}
	uc := ucconvite.NewRecusarConvite(pr, &stubApprovalRepo{}, buscar, temp)

	_, err := uc.Execute(context.Background(), "p-1")
	require.NoError(t, err)
	assert.Equal(t, convitedomain.StatusRecusado, pr.updateStatusLast)
	assert.Equal(t, 1, temp.calls)
}

func TestRecusarConvite_CancelladoUnprocessable(t *testing.T) {
	p := &convitedomain.Participant{
		ID: "p-1", EventoID: "e-1", TipoParticipante: convitedomain.TipoGuest,
		Status: convitedomain.StatusCancelado,
	}
	pr := &stubParticipantRepo{byID: map[string]*convitedomain.Participant{"p-1": p}}
	uc := ucconvite.NewRecusarConvite(pr, &stubApprovalRepo{}, nil, &stubTemporal{})
	_, err := uc.Execute(context.Background(), "p-1")
	require.Error(t, err)
	var apiErr *apierr.APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, 422, apiErr.Status)
}

// =================================================================
// DesistirEvento
// =================================================================

func TestDesistirEvento_PreviewComputesRefund(t *testing.T) {
	p := &convitedomain.Participant{
		ID: "p-1", EventoID: "e-1", UsuarioID: "u-1",
		Status: convitedomain.StatusConfirmado, Papel: convitedomain.PapelConvidado,
	}
	pr := &stubParticipantRepo{byID: map[string]*convitedomain.Participant{"p-1": p}}
	er := &stubEventoRepo{byID: map[string]*domainevent.Evento{"e-1": {
		ID: "e-1", Status: domainevent.EventoStatusPublicado, Data: time.Now().Add(40 * 24 * time.Hour),
	}}}
	inst := &stubInstallmentRepo{items: []portout.ConviteInstallment{
		{Status: "PAID", AmountCents: 10_000, PaidAt: ptrTime(time.Now())},
		{Status: "PENDING", AmountCents: 5_000},
	}}
	uc := ucconvite.NewDesistirEvento(pr, er, nil, inst, &stubCreditRepo{}, &stubTemporal{})

	res, err := uc.Preview(context.Background(), "p-1", "u-1")
	require.NoError(t, err)
	assert.Equal(t, "PREVIEW", res.Status)
	assert.Equal(t, int64(10_000), res.TotalPagoCents)
	// 40 days before → tier 30 (100% refund) per default policy
	assert.Equal(t, int64(10_000), res.RefundAmountCents)
	assert.Equal(t, 1.0, res.RefundPercent)
	assert.Equal(t, "GENERICA_FLEXIVEL", res.PoliticaAplicada)
}

func TestDesistirEvento_OwnershipViolation(t *testing.T) {
	p := &convitedomain.Participant{ID: "p-1", EventoID: "e-1", UsuarioID: "u-owner"}
	pr := &stubParticipantRepo{byID: map[string]*convitedomain.Participant{"p-1": p}}
	er := &stubEventoRepo{byID: map[string]*domainevent.Evento{"e-1": {ID: "e-1"}}}
	uc := ucconvite.NewDesistirEvento(pr, er, nil, &stubInstallmentRepo{}, &stubCreditRepo{}, &stubTemporal{})
	_, err := uc.Execute(context.Background(), "p-1", "u-stranger")
	require.Error(t, err)
	var apiErr *apierr.APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, 403, apiErr.Status)
}

func TestDesistirEvento_OrganizadorUnicoBlocked(t *testing.T) {
	p := &convitedomain.Participant{
		ID: "p-1", EventoID: "e-1", UsuarioID: "u-1",
		Papel: convitedomain.PapelOrganizador, Status: convitedomain.StatusConfirmado,
	}
	pr := &stubParticipantRepo{
		byID: map[string]*convitedomain.Participant{"p-1": p},
		byPapel: map[string][]convitedomain.Participant{
			"e-1:ORGANIZADOR": {*p}, // only one organiser
		},
	}
	er := &stubEventoRepo{byID: map[string]*domainevent.Evento{"e-1": {ID: "e-1"}}}
	uc := ucconvite.NewDesistirEvento(pr, er, nil, &stubInstallmentRepo{}, &stubCreditRepo{}, &stubTemporal{})
	_, err := uc.Execute(context.Background(), "p-1", "u-1")
	require.Error(t, err)
	var apiErr *apierr.APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, 422, apiErr.Status)
}

func TestDesistirEvento_ExecuteCancelsAndIssuesCredit(t *testing.T) {
	p := &convitedomain.Participant{
		ID: "p-1", EventoID: "e-1", UsuarioID: "u-1",
		Papel: convitedomain.PapelConvidado, Status: convitedomain.StatusConfirmado,
	}
	pr := &stubParticipantRepo{byID: map[string]*convitedomain.Participant{"p-1": p}}
	er := &stubEventoRepo{byID: map[string]*domainevent.Evento{"e-1": {
		ID: "e-1", Status: domainevent.EventoStatusPublicado, Data: time.Now().Add(40 * 24 * time.Hour),
	}}}
	inst := &stubInstallmentRepo{items: []portout.ConviteInstallment{
		{Status: "PAID", AmountCents: 20_000, PaidAt: ptrTime(time.Now())},
	}}
	credit := &stubCreditRepo{}
	temp := &stubTemporal{}
	uc := ucconvite.NewDesistirEvento(pr, er, nil, inst, credit, temp)

	res, err := uc.Execute(context.Background(), "p-1", "u-1")
	require.NoError(t, err)
	assert.Equal(t, int64(20_000), res.RefundAmountCents)
	assert.NotEmpty(t, res.CreditID)
	assert.Equal(t, 1, pr.updateStatusCalls)
	assert.Equal(t, convitedomain.StatusCancelado, pr.updateStatusLast)
	assert.Equal(t, int64(1), inst.cancelled)
	assert.Equal(t, 1, temp.calls)
}

// =================================================================
// ReabrirInviteApproval
// =================================================================

func TestReabrirInviteApproval_Success_CopiesMetadata(t *testing.T) {
	target := &convitedomain.Participant{
		ID: "p-1", EventoID: "e-1", UsuarioID: "u-target",
		TipoParticipante: convitedomain.TipoUser,
	}
	requester := &convitedomain.Participant{
		EventoID: "e-1", UsuarioID: "u-org", Papel: convitedomain.PapelOrganizador,
	}
	pr := &stubParticipantRepo{
		byID: map[string]*convitedomain.Participant{"p-1": target},
		byCompositeKey: map[string]*convitedomain.Participant{
			"e-1:u-org": requester,
		},
	}
	prev := &convitedomain.ApprovalItem{
		ID:       "old", Type: convitedomain.ApprovalTypeInvite,
		Status:   convitedomain.ApprovalStatusRejected,
		Metadata: map[string]any{"nome": "Carol"},
	}
	ar := &stubApprovalRepo{
		pendingByTarget: map[string]*convitedomain.ApprovalItem{"p-1": prev},
		existsPending:   map[string]bool{"p-1": false},
	}
	uc := ucconvite.NewReabrirInviteApproval(pr, ar)

	res, err := uc.Execute(context.Background(), "p-1", "u-org")
	require.NoError(t, err)
	assert.Equal(t, "INVITE", res.Type)
	assert.Equal(t, "PENDING", res.Status)
	assert.Equal(t, "Carol", res.Metadata["nome"])
	assert.Equal(t, 1, ar.saveCalls)
}

func TestReabrirInviteApproval_ForbiddenWhenNotOrganizer(t *testing.T) {
	target := &convitedomain.Participant{
		ID: "p-1", EventoID: "e-1", TipoParticipante: convitedomain.TipoUser,
	}
	pr := &stubParticipantRepo{
		byID: map[string]*convitedomain.Participant{"p-1": target},
	}
	uc := ucconvite.NewReabrirInviteApproval(pr, &stubApprovalRepo{})
	_, err := uc.Execute(context.Background(), "p-1", "u-random")
	require.Error(t, err)
	var apiErr *apierr.APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, 403, apiErr.Status)
}

func TestReabrirInviteApproval_ConflictWhenPendingExists(t *testing.T) {
	target := &convitedomain.Participant{
		ID: "p-1", EventoID: "e-1", UsuarioID: "u-target", TipoParticipante: convitedomain.TipoUser,
	}
	requester := &convitedomain.Participant{Papel: convitedomain.PapelOrganizador}
	pr := &stubParticipantRepo{
		byID:           map[string]*convitedomain.Participant{"p-1": target},
		byCompositeKey: map[string]*convitedomain.Participant{"e-1:u-org": requester},
	}
	ar := &stubApprovalRepo{
		existsPending: map[string]bool{"p-1": true},
	}
	uc := ucconvite.NewReabrirInviteApproval(pr, ar)
	_, err := uc.Execute(context.Background(), "p-1", "u-org")
	require.Error(t, err)
	var apiErr *apierr.APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, 409, apiErr.Status)
}

// =================================================================
// ReenviarConvitesMassaAdmin
// =================================================================

func TestReenviarMassa_SuccessRegistersAudit(t *testing.T) {
	er := &stubEventoRepo{byID: map[string]*domainevent.Evento{"e-1": {ID: "e-1", Nome: "Festa"}}}
	audit := &stubAuditRepo{}
	uc := ucconvite.NewReenviarConvitesMassaAdmin(er, audit)

	res, err := uc.Execute(context.Background(), "e-1", "u-admin")
	require.NoError(t, err)
	assert.Equal(t, "EVENT_INVITES_RESENT_BY_ADMIN", res.Acao)
	assert.Equal(t, 1, audit.count)
}

func TestReenviarMassa_NotFound(t *testing.T) {
	uc := ucconvite.NewReenviarConvitesMassaAdmin(&stubEventoRepo{byID: map[string]*domainevent.Evento{}}, &stubAuditRepo{})
	_, err := uc.Execute(context.Background(), "missing", "u-admin")
	require.Error(t, err)
	assert.True(t, apierr.IsNotFound(err))
}

// =================================================================
// helpers
// =================================================================

func ptrTime(t time.Time) *time.Time { return &t }
