package event_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	authdomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/auth"
	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/event"
	financedomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/finance"
	paymentdomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	rateiodomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/rateio"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	usecase "github.com/role-organizado/backend-go-role-organizado/internal/usecase/event"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ---- additional mocks (advanced use cases) ----

type mockPaymentTxRepo struct{ mock.Mock }

func (m *mockPaymentTxRepo) Save(ctx context.Context, tx *paymentdomain.PaymentTransaction) error {
	return m.Called(ctx, tx).Error(0)
}
func (m *mockPaymentTxRepo) Update(ctx context.Context, tx *paymentdomain.PaymentTransaction) error {
	return m.Called(ctx, tx).Error(0)
}
func (m *mockPaymentTxRepo) FindByID(ctx context.Context, id string) (*paymentdomain.PaymentTransaction, error) {
	a := m.Called(ctx, id)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).(*paymentdomain.PaymentTransaction), a.Error(1)
}
func (m *mockPaymentTxRepo) FindByIdempotencyKey(ctx context.Context, key string) (*paymentdomain.PaymentTransaction, error) {
	return nil, nil
}
func (m *mockPaymentTxRepo) FindByProviderTransactionID(ctx context.Context, k string) (*paymentdomain.PaymentTransaction, error) {
	return nil, nil
}
func (m *mockPaymentTxRepo) FindByUserID(ctx context.Context, uid string, f portout.TransactionFilter) ([]*paymentdomain.PaymentTransaction, int64, error) {
	return nil, 0, nil
}
func (m *mockPaymentTxRepo) FindPendingOlderThan(ctx context.Context, t time.Time) ([]*paymentdomain.PaymentTransaction, error) {
	return nil, nil
}
func (m *mockPaymentTxRepo) FindByEventID(ctx context.Context, eventID string) ([]*paymentdomain.PaymentTransaction, error) {
	a := m.Called(ctx, eventID)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).([]*paymentdomain.PaymentTransaction), a.Error(1)
}

// mockParticipantReadRepo implements portout.ParticipantRepository (read-side).
type mockParticipantReadRepo struct{ mock.Mock }

func (m *mockParticipantReadRepo) FindByUserID(ctx context.Context, uid string) ([]financedomain.Participant, error) {
	a := m.Called(ctx, uid)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).([]financedomain.Participant), a.Error(1)
}
func (m *mockParticipantReadRepo) FindByEventID(ctx context.Context, eid string, p, s int) ([]financedomain.Participant, int64, error) {
	a := m.Called(ctx, eid, p, s)
	if a.Get(0) == nil {
		return nil, int64(a.Int(1)), a.Error(2)
	}
	return a.Get(0).([]financedomain.Participant), int64(a.Int(1)), a.Error(2)
}
func (m *mockParticipantReadRepo) FindByEventIDAndUserID(ctx context.Context, eid, uid string) (*financedomain.Participant, error) {
	a := m.Called(ctx, eid, uid)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).(*financedomain.Participant), a.Error(1)
}
func (m *mockParticipantReadRepo) FindAllByEventID(ctx context.Context, eid string) ([]financedomain.Participant, error) {
	a := m.Called(ctx, eid)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).([]financedomain.Participant), a.Error(1)
}

// mockRateioRepo implements portout.RateioRepository.
type mockRateioRepo struct{ mock.Mock }

func (m *mockRateioRepo) FindByID(ctx context.Context, id string) (*rateiodomain.Rateio, error) {
	a := m.Called(ctx, id)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).(*rateiodomain.Rateio), a.Error(1)
}
func (m *mockRateioRepo) FindByEventoID(ctx context.Context, eid string) ([]rateiodomain.Rateio, error) {
	a := m.Called(ctx, eid)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).([]rateiodomain.Rateio), a.Error(1)
}
func (m *mockRateioRepo) FindByUsuarioID(ctx context.Context, uid string, p, s int) ([]rateiodomain.Rateio, int64, error) {
	return nil, 0, nil
}
func (m *mockRateioRepo) Save(ctx context.Context, r *rateiodomain.Rateio) (*rateiodomain.Rateio, error) {
	return r, nil
}
func (m *mockRateioRepo) Update(ctx context.Context, r *rateiodomain.Rateio) (*rateiodomain.Rateio, error) {
	return r, nil
}
func (m *mockRateioRepo) DeleteByID(ctx context.Context, id string) error { return nil }

// mockUsuarioRepo implements portout.UsuarioRepository.
type mockUsuarioRepo struct{ mock.Mock }

func (m *mockUsuarioRepo) FindByID(ctx context.Context, id string) (*authdomain.Usuario, error) {
	a := m.Called(ctx, id)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).(*authdomain.Usuario), a.Error(1)
}
func (m *mockUsuarioRepo) FindByEmail(ctx context.Context, e string) (*authdomain.Usuario, error) {
	return nil, nil
}
func (m *mockUsuarioRepo) FindByProviderID(ctx context.Context, p, pid string) (*authdomain.Usuario, error) {
	return nil, nil
}
func (m *mockUsuarioRepo) Save(ctx context.Context, u *authdomain.Usuario) (*authdomain.Usuario, error) {
	return u, nil
}
func (m *mockUsuarioRepo) Update(ctx context.Context, u *authdomain.Usuario) (*authdomain.Usuario, error) {
	return u, nil
}
func (m *mockUsuarioRepo) FindAll(ctx context.Context, p, ps int) ([]authdomain.Usuario, int64, error) {
	return nil, 0, nil
}
func (m *mockUsuarioRepo) DeleteByID(ctx context.Context, id string) error { return nil }

// fakeImageStorage implements usecase.ImagemStorage.
type fakeImageStorage struct {
	calls int
	fail  bool
}

func (f *fakeImageStorage) UploadImage(ctx context.Context, filename, contentType string, data []byte) (string, string, error) {
	f.calls++
	if f.fail {
		return "", "", errors.New("upload boom")
	}
	return "fid", "/api/eventos/imagens/fid", nil
}

// ---- AlterarFase ----

func TestAlterarFase_Success_AdvanceToPreparacao(t *testing.T) {
	er := new(mockEventoRepo)
	pr := new(mockParticipanteRepo)
	tr := new(mockPaymentTxRepo)
	uc := usecase.NewAlterarFase(er, pr, tr)

	evt := &domain.Evento{
		ID:        "evt-1",
		UsuarioID: "owner",
		Fase:      domain.FaseAguardandoAceite,
	}
	er.On("FindByID", mock.Anything, "evt-1").Return(evt, nil)
	pr.On("CountNonOrganizadorByEventID", mock.Anything, "evt-1").Return(3, nil)
	er.On("UpdateFase", mock.Anything, "evt-1", domain.FasePreparacao).Return(nil)

	res, err := uc.Execute(context.Background(), portin.AlterarFaseInput{
		EventoID:    "evt-1",
		RequesterID: "owner",
		FaseDestino: string(domain.FasePreparacao),
	})
	require.NoError(t, err)
	assert.Equal(t, string(domain.FaseAguardandoAceite), res.FaseAnterior)
	assert.Equal(t, string(domain.FasePreparacao), res.FaseAtual)
}

func TestAlterarFase_Forbidden_NotOrganizer(t *testing.T) {
	er := new(mockEventoRepo)
	pr := new(mockParticipanteRepo)
	tr := new(mockPaymentTxRepo)
	uc := usecase.NewAlterarFase(er, pr, tr)

	evt := &domain.Evento{ID: "evt-1", UsuarioID: "owner", Fase: domain.FaseAguardandoAceite}
	er.On("FindByID", mock.Anything, "evt-1").Return(evt, nil)
	pr.On("HasOrganizadorPapel", mock.Anything, "evt-1", "intruder").Return(false, nil)

	_, err := uc.Execute(context.Background(), portin.AlterarFaseInput{
		EventoID:    "evt-1",
		RequesterID: "intruder",
		FaseDestino: string(domain.FasePreparacao),
	})
	require.Error(t, err)
	var ae *apierr.APIError
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, 403, ae.Status)
}

func TestAlterarFase_InvalidTransition(t *testing.T) {
	er := new(mockEventoRepo)
	pr := new(mockParticipanteRepo)
	tr := new(mockPaymentTxRepo)
	uc := usecase.NewAlterarFase(er, pr, tr)

	evt := &domain.Evento{ID: "evt-1", UsuarioID: "owner", Fase: domain.FaseExecucao}
	er.On("FindByID", mock.Anything, "evt-1").Return(evt, nil)

	_, err := uc.Execute(context.Background(), portin.AlterarFaseInput{
		EventoID:    "evt-1",
		RequesterID: "owner",
		FaseDestino: string(domain.FaseAguardandoAceite),
	})
	var ae *apierr.APIError
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, 400, ae.Status)
}

func TestAlterarFase_BlockedRollback_CompletedPayments(t *testing.T) {
	er := new(mockEventoRepo)
	pr := new(mockParticipanteRepo)
	tr := new(mockPaymentTxRepo)
	uc := usecase.NewAlterarFase(er, pr, tr)

	evt := &domain.Evento{ID: "evt-1", UsuarioID: "owner", Fase: domain.FaseColetaPagamentos}
	er.On("FindByID", mock.Anything, "evt-1").Return(evt, nil)
	tr.On("FindByEventID", mock.Anything, "evt-1").Return([]*paymentdomain.PaymentTransaction{
		{ID: "tx1", Status: paymentdomain.TransactionStatusCompleted},
	}, nil)

	_, err := uc.Execute(context.Background(), portin.AlterarFaseInput{
		EventoID:    "evt-1",
		RequesterID: "owner",
		FaseDestino: string(domain.FaseAguardandoAceite),
	})
	var ae *apierr.APIError
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, 400, ae.Status)
}

// ---- UploadImagens ----

func TestUploadImagens_Success_FirstIsCapa(t *testing.T) {
	er := new(mockEventoRepo)
	store := &fakeImageStorage{}
	uc := usecase.NewUploadImagens(er, store)

	evt := &domain.Evento{ID: "evt-1", UsuarioID: "owner", Imagens: nil}
	er.On("FindByID", mock.Anything, "evt-1").Return(evt, nil)
	er.On("AddImagens", mock.Anything, "evt-1", mock.AnythingOfType("[]event.EventoImagem")).Return(nil)

	res, err := uc.Execute(context.Background(), portin.UploadImagensInput{
		EventoID:    "evt-1",
		RequesterID: "owner",
		Tipo:        "galeria",
		Imagens: []portin.UploadImagemInput{
			{Filename: "a.jpg", ContentType: "image/jpeg", Size: 1024, Data: []byte("x")},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, 1, len(res.Imagens))
	assert.Equal(t, "capa", res.Imagens[0].Tipo)
}

func TestUploadImagens_Forbidden(t *testing.T) {
	er := new(mockEventoRepo)
	store := &fakeImageStorage{}
	uc := usecase.NewUploadImagens(er, store)

	evt := &domain.Evento{ID: "evt-1", UsuarioID: "owner"}
	er.On("FindByID", mock.Anything, "evt-1").Return(evt, nil)

	_, err := uc.Execute(context.Background(), portin.UploadImagensInput{
		EventoID:    "evt-1",
		RequesterID: "intruder",
		Imagens:     []portin.UploadImagemInput{{Filename: "a.jpg", ContentType: "image/jpeg", Size: 1, Data: []byte("a")}},
	})
	var ae *apierr.APIError
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, 403, ae.Status)
}

func TestUploadImagens_RejectInvalidContentType(t *testing.T) {
	er := new(mockEventoRepo)
	store := &fakeImageStorage{}
	uc := usecase.NewUploadImagens(er, store)

	evt := &domain.Evento{ID: "evt-1", UsuarioID: "owner"}
	er.On("FindByID", mock.Anything, "evt-1").Return(evt, nil)

	_, err := uc.Execute(context.Background(), portin.UploadImagensInput{
		EventoID:    "evt-1",
		RequesterID: "owner",
		Imagens:     []portin.UploadImagemInput{{Filename: "a.gif", ContentType: "image/gif", Size: 1, Data: []byte("a")}},
	})
	var ae *apierr.APIError
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, 400, ae.Status)
}

// ---- BuscarSummaries ----

func TestBuscarSummaries_Success_FallbackImageURL(t *testing.T) {
	er := new(mockEventoRepo)
	uc := usecase.NewBuscarSummaries(er)

	er.On("FindAllByIDs", mock.Anything, []string{"a", "b"}).Return([]domain.Evento{
		{
			ID:   "a",
			Nome: "ev a",
			Data: time.Now(),
			Imagens: []domain.EventoImagem{
				{URL: "https://cdn/x.jpg", Ordem: 0, Tipo: "capa"},
			},
		},
		{
			ID:       "b",
			Nome:     "ev b",
			Data:     time.Now(),
			ImageURL: "https://cdn/legacy.jpg",
		},
	}, nil)

	out, err := uc.Execute(context.Background(), []string{"a", "b"})
	require.NoError(t, err)
	require.Len(t, out, 2)
	assert.Equal(t, "https://cdn/x.jpg", out[0].ImageURL)
	assert.Equal(t, "https://cdn/legacy.jpg", out[1].ImageURL)
}

func TestBuscarSummaries_RejectEmpty(t *testing.T) {
	er := new(mockEventoRepo)
	uc := usecase.NewBuscarSummaries(er)
	_, err := uc.Execute(context.Background(), nil)
	var ae *apierr.APIError
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, 400, ae.Status)
}

// ---- AtualizarPoliticaConvidados ----

func TestAtualizarPolitica_Success(t *testing.T) {
	er := new(mockEventoRepo)
	pr := new(mockParticipanteRepo)
	uc := usecase.NewAtualizarPoliticaConvidados(er, pr)

	evt := &domain.Evento{ID: "evt-1", UsuarioID: "owner"}
	er.On("FindByID", mock.Anything, "evt-1").Return(evt, nil)
	er.On("UpdatePoliticaConvidados", mock.Anything, "evt-1", "public").Return(nil)

	res, err := uc.Execute(context.Background(), portin.AtualizarPoliticaConvidadosInput{
		EventoID:    "evt-1",
		RequesterID: "owner",
		Politica:    "public",
	})
	require.NoError(t, err)
	assert.Equal(t, "public", res.PoliticaConvidados)
}

func TestAtualizarPolitica_InvalidValue(t *testing.T) {
	er := new(mockEventoRepo)
	pr := new(mockParticipanteRepo)
	uc := usecase.NewAtualizarPoliticaConvidados(er, pr)

	_, err := uc.Execute(context.Background(), portin.AtualizarPoliticaConvidadosInput{
		EventoID:    "evt-1",
		RequesterID: "owner",
		Politica:    "bogus",
	})
	var ae *apierr.APIError
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, 400, ae.Status)
}

// ---- AtualizarDetalhes ----

func TestAtualizarDetalhes_Success(t *testing.T) {
	er := new(mockEventoRepo)
	pr := new(mockParticipanteRepo)
	uc := usecase.NewAtualizarDetalhes(er, pr)

	evt := &domain.Evento{ID: "evt-1", UsuarioID: "owner", Nome: "old"}
	er.On("FindByID", mock.Anything, "evt-1").Return(evt, nil)
	er.On("UpdateDetalhes", mock.Anything, mock.AnythingOfType("*event.Evento")).Return(evt, nil)

	nome := "Nova Festa"
	res, err := uc.Execute(context.Background(), portin.AtualizarDetalhesInput{
		EventoID:    "evt-1",
		RequesterID: "owner",
		Nome:        &nome,
	})
	require.NoError(t, err)
	assert.Equal(t, "Nova Festa", res.Nome)
}

func TestAtualizarDetalhes_RejectShortNome(t *testing.T) {
	er := new(mockEventoRepo)
	pr := new(mockParticipanteRepo)
	uc := usecase.NewAtualizarDetalhes(er, pr)

	short := "ab"
	_, err := uc.Execute(context.Background(), portin.AtualizarDetalhesInput{
		EventoID:    "evt-1",
		RequesterID: "owner",
		Nome:        &short,
	})
	var ae *apierr.APIError
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, 400, ae.Status)
}

// ---- GerenciarEvento ----

func TestGerenciarEvento_Success(t *testing.T) {
	er := new(mockEventoRepo)
	pr := new(mockParticipanteRepo)
	tr := new(mockPaymentTxRepo)
	uc := usecase.NewGerenciarEvento(er, pr, tr)

	evt := &domain.Evento{ID: "evt-1", UsuarioID: "owner", Nome: "ev", Fase: domain.FaseColetaPagamentos}
	er.On("FindByID", mock.Anything, "evt-1").Return(evt, nil)
	pr.On("CountByEventIDAndStatus", mock.Anything, "evt-1", "CONFIRMADO").Return(2, nil)
	pr.On("CountByEventIDAndStatus", mock.Anything, "evt-1", "PENDENTE").Return(1, nil)
	pr.On("CountByEventIDAndStatus", mock.Anything, "evt-1", "RECUSADO").Return(0, nil)
	tr.On("FindByEventID", mock.Anything, "evt-1").Return([]*paymentdomain.PaymentTransaction{
		{ID: "tx1", Status: paymentdomain.TransactionStatusCompleted},
	}, nil)

	res, err := uc.Execute(context.Background(), portin.GerenciarEventoInput{
		EventoID:    "evt-1",
		RequesterID: "owner",
	})
	require.NoError(t, err)
	assert.Equal(t, 3, res.ParticipantSummary.Total)
	assert.Equal(t, 2, res.ParticipantSummary.Confirmed)
	assert.True(t, res.HasCompletedPayments)
	assert.Equal(t, "NOT_STARTED", res.WorkflowStatus)
}

func TestGerenciarEvento_Forbidden(t *testing.T) {
	er := new(mockEventoRepo)
	pr := new(mockParticipanteRepo)
	tr := new(mockPaymentTxRepo)
	uc := usecase.NewGerenciarEvento(er, pr, tr)

	evt := &domain.Evento{ID: "evt-1", UsuarioID: "owner"}
	er.On("FindByID", mock.Anything, "evt-1").Return(evt, nil)
	pr.On("HasOrganizadorPapel", mock.Anything, "evt-1", "intruder").Return(false, nil)

	_, err := uc.Execute(context.Background(), portin.GerenciarEventoInput{
		EventoID:    "evt-1",
		RequesterID: "intruder",
	})
	var ae *apierr.APIError
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, 403, ae.Status)
}

// ---- GetPublicInfo ----

func TestGetPublicInfo_Success(t *testing.T) {
	er := new(mockEventoRepo)
	pr := new(mockParticipanteRepo)
	ur := new(mockUsuarioRepo)
	uc := usecase.NewGetPublicInfo(er, pr, ur)

	evt := &domain.Evento{
		ID:                 "evt-1",
		UsuarioID:          "owner",
		Nome:               "Festa Aberta",
		PoliticaConvidados: domain.PoliticaPublic,
	}
	er.On("FindByID", mock.Anything, "evt-1").Return(evt, nil)
	ur.On("FindByID", mock.Anything, "owner").Return(&authdomain.Usuario{ID: "owner", Nome: "Maria"}, nil)
	pr.On("CountConfirmedByEventID", mock.Anything, "evt-1").Return(5, nil)

	res, err := uc.Execute(context.Background(), "evt-1")
	require.NoError(t, err)
	assert.Equal(t, "Festa Aberta", res.Nome)
	require.NotNil(t, res.OrganizadorNome)
	assert.Equal(t, "Maria", *res.OrganizadorNome)
	assert.Equal(t, int64(5), res.TotalConfirmados)
	assert.Equal(t, "public", res.PoliticaConvidados)
}

func TestGetPublicInfo_NotFound(t *testing.T) {
	er := new(mockEventoRepo)
	pr := new(mockParticipanteRepo)
	ur := new(mockUsuarioRepo)
	uc := usecase.NewGetPublicInfo(er, pr, ur)

	er.On("FindByID", mock.Anything, "missing").Return(nil, apierr.NotFound("evento", "missing"))

	_, err := uc.Execute(context.Background(), "missing")
	var ae *apierr.APIError
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, 404, ae.Status)
}

// ---- JoinEvento ----

func TestJoin_PublicPolicy_Confirmado_200(t *testing.T) {
	er := new(mockEventoRepo)
	pr := new(mockParticipanteRepo)
	uc := usecase.NewJoinEvento(er, pr)

	evt := &domain.Evento{ID: "evt-1", PoliticaConvidados: domain.PoliticaPublic}
	er.On("FindByID", mock.Anything, "evt-1").Return(evt, nil)
	pr.On("IsParticipantOfEvent", mock.Anything, "evt-1", "user-x").Return(false, nil)
	pr.On("CreateParticipant", mock.Anything, mock.AnythingOfType("out.NewParticipant")).Return("part-1", nil)

	res, err := uc.Execute(context.Background(), portin.JoinEventoInput{EventoID: "evt-1", UserID: "user-x"})
	require.NoError(t, err)
	assert.Equal(t, "CONFIRMADO", res.Status)
	assert.Equal(t, 200, res.HTTPStatusCode)
}

func TestJoin_ApprovalPolicy_Pendente_202(t *testing.T) {
	er := new(mockEventoRepo)
	pr := new(mockParticipanteRepo)
	uc := usecase.NewJoinEvento(er, pr)

	evt := &domain.Evento{ID: "evt-1", PoliticaConvidados: domain.PoliticaApproval}
	er.On("FindByID", mock.Anything, "evt-1").Return(evt, nil)
	pr.On("IsParticipantOfEvent", mock.Anything, "evt-1", "user-x").Return(false, nil)
	pr.On("CreateParticipant", mock.Anything, mock.AnythingOfType("out.NewParticipant")).Return("part-1", nil)

	res, err := uc.Execute(context.Background(), portin.JoinEventoInput{EventoID: "evt-1", UserID: "user-x"})
	require.NoError(t, err)
	assert.Equal(t, "PENDENTE", res.Status)
	assert.Equal(t, 202, res.HTTPStatusCode)
}

func TestJoin_InviteOnly_Forbidden(t *testing.T) {
	er := new(mockEventoRepo)
	pr := new(mockParticipanteRepo)
	uc := usecase.NewJoinEvento(er, pr)

	evt := &domain.Evento{ID: "evt-1", PoliticaConvidados: domain.PoliticaInviteOnly}
	er.On("FindByID", mock.Anything, "evt-1").Return(evt, nil)

	_, err := uc.Execute(context.Background(), portin.JoinEventoInput{EventoID: "evt-1", UserID: "user-x"})
	var ae *apierr.APIError
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, 403, ae.Status)
}

func TestJoin_Duplicate_Conflict(t *testing.T) {
	er := new(mockEventoRepo)
	pr := new(mockParticipanteRepo)
	uc := usecase.NewJoinEvento(er, pr)

	evt := &domain.Evento{ID: "evt-1", PoliticaConvidados: domain.PoliticaPublic}
	er.On("FindByID", mock.Anything, "evt-1").Return(evt, nil)
	pr.On("IsParticipantOfEvent", mock.Anything, "evt-1", "user-x").Return(true, nil)

	_, err := uc.Execute(context.Background(), portin.JoinEventoInput{EventoID: "evt-1", UserID: "user-x"})
	var ae *apierr.APIError
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, 409, ae.Status)
}

func TestJoin_CapacityReached_422(t *testing.T) {
	er := new(mockEventoRepo)
	pr := new(mockParticipanteRepo)
	uc := usecase.NewJoinEvento(er, pr)

	limit := 2
	evt := &domain.Evento{
		ID:                 "evt-1",
		PoliticaConvidados: domain.PoliticaPublic,
		LimiteConvidados:   &limit,
	}
	er.On("FindByID", mock.Anything, "evt-1").Return(evt, nil)
	pr.On("IsParticipantOfEvent", mock.Anything, "evt-1", "user-x").Return(false, nil)
	pr.On("CountConfirmedByEventID", mock.Anything, "evt-1").Return(2, nil)

	_, err := uc.Execute(context.Background(), portin.JoinEventoInput{EventoID: "evt-1", UserID: "user-x"})
	var ae *apierr.APIError
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, 422, ae.Status)
}

// ---- GetEventoCompleto ----

func TestGetEventoCompleto_Success(t *testing.T) {
	er := new(mockEventoRepo)
	pr := new(mockParticipantReadRepo)
	rr := new(mockRateioRepo)
	uc := usecase.NewGetEventoCompleto(er, pr, rr)

	evt := &domain.Evento{ID: "evt-1", Nome: "ev", Status: domain.EventoStatusPublicado, Imagens: []domain.EventoImagem{
		{URL: "u1", Ordem: 1}, {URL: "u0", Ordem: 0},
	}}
	er.On("FindByID", mock.Anything, "evt-1").Return(evt, nil)
	pr.On("FindAllByEventID", mock.Anything, "evt-1").Return([]financedomain.Participant{
		{ID: "p1", UserID: "u1", Status: "CONFIRMADO"},
	}, nil)
	rr.On("FindByEventoID", mock.Anything, "evt-1").Return([]rateiodomain.Rateio{
		{ID: "r1", Descricao: "Bar", Tipo: rateiodomain.TipoRateioDivisao, Status: rateiodomain.StatusRateioAberto, ValorTotal: 100},
	}, nil)

	res, err := uc.Execute(context.Background(), "evt-1")
	require.NoError(t, err)
	assert.Equal(t, "evt-1", res.ID)
	require.Len(t, res.Imagens, 2)
	assert.Equal(t, 0, res.Imagens[0].Ordem) // sorted ascending
	assert.Equal(t, "u0", res.Imagens[0].URL)
	require.Len(t, res.Usuarios, 1)
	require.Len(t, res.Rateios, 1)
}
