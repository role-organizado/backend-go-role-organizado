package participant_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	eventdomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/event"
	paymentdomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	rateiodomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/rateio"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	ucparticipant "github.com/role-organizado/backend-go-role-organizado/internal/usecase/participant"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ─── fakeEventoRepo ─────────────────────────────────────────────────────────
// Minimal portout.EventoRepository: only FindByID is exercised; the rest satisfy
// the interface and panic if unexpectedly called.

type fakeEventoRepo struct {
	evento *eventdomain.Evento
	err    error
}

func (f *fakeEventoRepo) FindByID(_ context.Context, _ string) (*eventdomain.Evento, error) {
	return f.evento, f.err
}
func (f *fakeEventoRepo) FindByUsuarioID(context.Context, string, int, int) ([]eventdomain.Evento, int64, error) {
	panic("unused")
}
func (f *fakeEventoRepo) FindByUsuarioIDCursor(context.Context, string, portout.EventoQueryFiltros) (portout.EventosCursorPage, error) {
	panic("unused")
}
func (f *fakeEventoRepo) FindAll(context.Context, int, int) ([]eventdomain.Evento, int64, error) {
	panic("unused")
}
func (f *fakeEventoRepo) Save(context.Context, *eventdomain.Evento) (*eventdomain.Evento, error) {
	panic("unused")
}
func (f *fakeEventoRepo) Update(context.Context, *eventdomain.Evento) (*eventdomain.Evento, error) {
	panic("unused")
}
func (f *fakeEventoRepo) DeleteByID(context.Context, string) error { panic("unused") }
func (f *fakeEventoRepo) AddConvidados(context.Context, string, []eventdomain.Convidado) error {
	panic("unused")
}
func (f *fakeEventoRepo) FindAllByIDs(context.Context, []string) ([]eventdomain.Evento, error) {
	panic("unused")
}
func (f *fakeEventoRepo) UpdateFase(context.Context, string, eventdomain.EventoFase) error {
	panic("unused")
}
func (f *fakeEventoRepo) UpdatePoliticaConvidados(context.Context, string, string) error {
	panic("unused")
}
func (f *fakeEventoRepo) AddImagens(context.Context, string, []eventdomain.EventoImagem) error {
	panic("unused")
}
func (f *fakeEventoRepo) UpdateDetalhes(context.Context, *eventdomain.Evento) (*eventdomain.Evento, error) {
	panic("unused")
}

// ─── fakeRateioRepo ─────────────────────────────────────────────────────────

type fakeRateioRepo struct {
	byEvento  []rateiodomain.Rateio
	findErr   error
	updateErr error
	updated   []rateiodomain.Rateio
}

func (f *fakeRateioRepo) FindByID(context.Context, string) (*rateiodomain.Rateio, error) {
	panic("unused")
}
func (f *fakeRateioRepo) FindByEventoID(_ context.Context, _ string) ([]rateiodomain.Rateio, error) {
	return f.byEvento, f.findErr
}
func (f *fakeRateioRepo) FindByUsuarioID(context.Context, string, int, int) ([]rateiodomain.Rateio, int64, error) {
	panic("unused")
}
func (f *fakeRateioRepo) Save(context.Context, *rateiodomain.Rateio) (*rateiodomain.Rateio, error) {
	panic("unused")
}
func (f *fakeRateioRepo) Update(_ context.Context, r *rateiodomain.Rateio) (*rateiodomain.Rateio, error) {
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	f.updated = append(f.updated, *r)
	return r, nil
}
func (f *fakeRateioRepo) DeleteByID(context.Context, string) error { panic("unused") }

// ─── fakeInstallmentRepo ────────────────────────────────────────────────────

type fakeInstallmentRepo struct {
	cancelCount int64
	cancelErr   error
}

func (f *fakeInstallmentRepo) FindByID(context.Context, string) (*paymentdomain.PaymentInstallment, error) {
	panic("unused")
}
func (f *fakeInstallmentRepo) FindByEventAndParticipant(context.Context, string, string) ([]*paymentdomain.PaymentInstallment, error) {
	panic("unused")
}
func (f *fakeInstallmentRepo) FindByUserOrParticipations(context.Context, string, []string, *paymentdomain.InstallmentStatus) ([]*paymentdomain.PaymentInstallment, error) {
	panic("unused")
}
func (f *fakeInstallmentRepo) FindByIDs(context.Context, []string) ([]*paymentdomain.PaymentInstallment, error) {
	panic("unused")
}
func (f *fakeInstallmentRepo) MarkPaidBatch(context.Context, []string, string, time.Time, string, string) error {
	panic("unused")
}
func (f *fakeInstallmentRepo) FindByEvent(context.Context, string, *paymentdomain.InstallmentStatus) ([]*paymentdomain.PaymentInstallment, error) {
	panic("unused")
}
func (f *fakeInstallmentRepo) CancelByParticipant(_ context.Context, _, _ string) (int64, error) {
	return f.cancelCount, f.cancelErr
}
func (f *fakeInstallmentRepo) FindOverdueNotNotified(context.Context, time.Time) ([]*paymentdomain.PaymentInstallment, error) {
	panic("unused")
}
func (f *fakeInstallmentRepo) MarkOverdueBatch(context.Context, []string) (int64, error) {
	panic("unused")
}

// ─── RecalculateRateioAllocations tests ─────────────────────────────────────

func TestRecalculateRateioAllocations_RecalculatesOnlyOpenRateios(t *testing.T) {
	rateios := []rateiodomain.Rateio{
		{
			ID:     "r-open",
			Status: rateiodomain.StatusRateioAberto,
			Itens: []rateiodomain.RateioItem{
				{Descricao: "Bebidas", Valor: 10, Quantidade: 3}, // Total recomputed → 30
			},
		},
		{
			ID:     "r-closed",
			Status: rateiodomain.StatusRateioFechado, // skipped (immutable snapshot)
			Itens:  []rateiodomain.RateioItem{{Valor: 5, Quantidade: 2}},
		},
	}
	rRepo := &fakeRateioRepo{byEvento: rateios}
	eRepo := &fakeEventoRepo{evento: &eventdomain.Evento{ID: "evt-1", UsuarioID: "org-1"}}

	uc := ucparticipant.NewRecalculateRateioAllocations(rRepo, eRepo)
	n, err := uc.Execute(context.Background(), portin.RecalculateRateioAllocationsInput{
		EventID: "evt-1", ParticipantID: "p-1", RequesterID: "org-1",
	})

	require.NoError(t, err)
	assert.Equal(t, int64(1), n)
	require.Len(t, rRepo.updated, 1)
	assert.Equal(t, "r-open", rRepo.updated[0].ID)
	assert.Equal(t, float64(30), rRepo.updated[0].Itens[0].Total)
}

func TestRecalculateRateioAllocations_RejectsNonOwner(t *testing.T) {
	eRepo := &fakeEventoRepo{evento: &eventdomain.Evento{ID: "evt-1", UsuarioID: "someone-else"}}
	uc := ucparticipant.NewRecalculateRateioAllocations(&fakeRateioRepo{}, eRepo)

	_, err := uc.Execute(context.Background(), portin.RecalculateRateioAllocationsInput{
		EventID: "evt-1", RequesterID: "org-1",
	})

	require.Error(t, err)
	var apiErr *apierr.APIError
	require.True(t, errors.As(err, &apiErr), "expected *apierr.APIError, got %v", err)
	assert.Equal(t, http.StatusForbidden, apiErr.Status)
}

func TestRecalculateRateioAllocations_PropagatesEventLookupError(t *testing.T) {
	eRepo := &fakeEventoRepo{err: errors.New("boom")}
	uc := ucparticipant.NewRecalculateRateioAllocations(&fakeRateioRepo{}, eRepo)

	_, err := uc.Execute(context.Background(), portin.RecalculateRateioAllocationsInput{EventID: "evt-1"})
	require.Error(t, err)
}

// ─── CancelParticipantInstallments tests ────────────────────────────────────

func TestCancelParticipantInstallments_CancelsForOwner(t *testing.T) {
	iRepo := &fakeInstallmentRepo{cancelCount: 4}
	eRepo := &fakeEventoRepo{evento: &eventdomain.Evento{ID: "evt-1", UsuarioID: "org-1"}}

	uc := ucparticipant.NewCancelParticipantInstallments(iRepo, eRepo)
	n, err := uc.Execute(context.Background(), portin.CancelParticipantInstallmentsInput{
		EventID: "evt-1", ParticipantID: "p-1", RequesterID: "org-1",
	})

	require.NoError(t, err)
	assert.Equal(t, int64(4), n)
}

func TestCancelParticipantInstallments_RejectsNonOwner(t *testing.T) {
	eRepo := &fakeEventoRepo{evento: &eventdomain.Evento{ID: "evt-1", UsuarioID: "someone-else"}}
	uc := ucparticipant.NewCancelParticipantInstallments(&fakeInstallmentRepo{}, eRepo)

	_, err := uc.Execute(context.Background(), portin.CancelParticipantInstallmentsInput{
		EventID: "evt-1", RequesterID: "org-1",
	})

	require.Error(t, err)
	var apiErr *apierr.APIError
	require.True(t, errors.As(err, &apiErr), "expected *apierr.APIError, got %v", err)
	assert.Equal(t, http.StatusForbidden, apiErr.Status)
}
