// Package finance_test — parity tests comparing Go use case outputs against
// Java-equivalent expected values computed from the same canonical seed data.
//
// Each test starts a single MongoDB container via Testcontainers, inserts a
// deterministic seed, wires up real MongoDB adapters (no stubs), and asserts
// field-by-field against hardcoded Java-equivalent expected values.
//
// GAP NOTE — `collected` field:
//
//	In Java, collected = Σ installment_allocations.PAID  (Feature 045)
//	In Go, collected  = finance_summary.collected        (passive read, pre-populated by Java)
//
//	The seed pre-populates finance_summary.collected with the value Java would
//	produce for the same dataset.  See specs/167-go-finance-use-cases/gaps.md.
package finance_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	mongoadapter "github.com/role-organizado/backend-go-role-organizado/internal/adapter/mongodb"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	"github.com/role-organizado/backend-go-role-organizado/internal/testhelper"
	ucfinance "github.com/role-organizado/backend-go-role-organizado/internal/usecase/finance"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─────────────────────────────────────────────────────────────────────────────
// Parity-test constants
// UUIDs use the "a1"/"b1" prefix to avoid collisions with finance_test.go
// which uses the "aa"-"ff" prefix range.
// ─────────────────────────────────────────────────────────────────────────────

const (
	// Event UUIDs
	parityPrimEvtID = "a1000000-0000-4000-a000-000000000001" // main test event (ATIVO participant)
	parityOrgEvtID  = "a1000000-0000-4000-a000-000000000002" // ORGANIZACAO exclusion test
	parityAAEvtID   = "a1000000-0000-4000-a000-000000000003" // AGUARDANDO_ACEITE exclusion test

	// User UUID
	parityUserUUID = "b1000000-0000-4000-a000-000000000001"

	// ── Java-equivalent expected values (hardcoded from seed) ────────
	//
	// Seed rateio: valor_rateado = 500  (R$ 500,00 → int64 domain field)
	// Go UC:       goal = int64(500.0 * 100) = 50_000 centavos
	parityExpectedGoal int64 = 50_000

	// finance_summary.collected pre-seeded to simulate Java's installment_allocations PAID.
	// GAP-001: Go does not have installment_allocations; uses finance_summary.collected instead.
	// See specs/167-go-finance-use-cases/gaps.md.
	parityExpectedCollected int64 = 30_000

	// pending_withdrawals pre-seeded in finance_summary
	parityExpectedPendingWithdrawals int64 = 5_000

	// available_for_withdrawal = max(0, collected − pendingWithdrawals) = 30_000 − 5_000
	parityExpectedAvailable int64 = 25_000

	// progressPercentage = CalculateProgress(30_000, 50_000) = 30000/50000*100 = 60.0000
	// math.Round(60.0 * 10_000) / 10_000 = 60.0  (HALF_UP, 4 decimal places)
	parityExpectedProgress float64 = 60.0

	// Ledger seed counts (CREDIT=5, DEBIT=2 — total 7)
	parityLedgerTotal   int64 = 7
	parityLedgerCredit  int64 = 5
	parityLedgerDebit   int64 = 2
)

// ─────────────────────────────────────────────────────────────────────────────
// seedParityData inserts the canonical seed used by all parity subtests.
// ─────────────────────────────────────────────────────────────────────────────

func seedParityData(t *testing.T, client *mongoadapter.Client) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()

	// ── Evento principal (PUBLICADO) ──────────────────────────────────
	evtDate := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	_, err := client.Collection("eventos").InsertOne(ctx, bson.D{
		{Key: "_id", Value: mongoadapter.UUIDStringToBinary(parityPrimEvtID)},
		{Key: "nome", Value: "Parity Test Event"},
		{Key: "tipo", Value: "CONFRATERNIZACAO"},
		{Key: "data_inicio", Value: evtDate},
		{Key: "usuario_id_responsavel", Value: mongoadapter.UUIDStringToBinary(parityUserUUID)},
		{Key: "status", Value: "PUBLICADO"},
		{Key: "rateios_habilitado", Value: true},
		{Key: "pagamentos_habilitado", Value: true},
		{Key: "criado_em", Value: now},
		{Key: "atualizado_em", Value: now},
	})
	require.NoError(t, err, "seed: insert evento principal")

	// ── Participante ATIVO → deve aparecer em ListFinanceEvents ───────
	_, err = client.Collection("participants").InsertOne(ctx, bson.D{
		{Key: "evento_id", Value: mongoadapter.UUIDStringToBinary(parityPrimEvtID)},
		{Key: "usuario_id", Value: mongoadapter.UUIDStringToBinary(parityUserUUID)},
		{Key: "papel", Value: "PARTICIPANTE"},
		{Key: "status", Value: "ATIVO"},
		{Key: "nome", Value: "Parity User"},
		{Key: "email", Value: "parity@test.com"},
	})
	require.NoError(t, err, "seed: insert participante ATIVO")

	// ── Participante ORGANIZACAO → deve ser excluído por ListFinanceEvents ──
	// A UC filtra antes de buscar o evento, então não é necessário inserir o evento.
	_, err = client.Collection("participants").InsertOne(ctx, bson.D{
		{Key: "evento_id", Value: mongoadapter.UUIDStringToBinary(parityOrgEvtID)},
		{Key: "usuario_id", Value: mongoadapter.UUIDStringToBinary(parityUserUUID)},
		{Key: "papel", Value: "ORGANIZADOR"},
		{Key: "status", Value: "ORGANIZACAO"},
		{Key: "nome", Value: "Parity User"},
		{Key: "email", Value: "parity@test.com"},
	})
	require.NoError(t, err, "seed: insert participante ORGANIZACAO")

	// ── Participante AGUARDANDO_ACEITE → deve ser excluído por ListFinanceEvents ──
	_, err = client.Collection("participants").InsertOne(ctx, bson.D{
		{Key: "evento_id", Value: mongoadapter.UUIDStringToBinary(parityAAEvtID)},
		{Key: "usuario_id", Value: mongoadapter.UUIDStringToBinary(parityUserUUID)},
		{Key: "papel", Value: "PARTICIPANTE"},
		{Key: "status", Value: "AGUARDANDO_ACEITE"},
		{Key: "nome", Value: "Parity User"},
		{Key: "email", Value: "parity@test.com"},
	})
	require.NoError(t, err, "seed: insert participante AGUARDANDO_ACEITE")

	// ── Rateio ATIVO (= StatusRateioAberto = "ABERTO" no domínio Go) ─
	// MongoDB status: "ATIVO" → statusFromMongo → domain.StatusRateioAberto ("ABERTO")
	// valor_rateado = 500 → domain.ValorTotal = float64(500) = 500.0
	// UC: goal += int64(500.0 * 100) = 50_000 centavos
	_, err = client.Collection("rateios").InsertOne(ctx, bson.D{
		{Key: "evento_id", Value: mongoadapter.UUIDStringToBinary(parityPrimEvtID)},
		{Key: "status", Value: "ATIVO"},
		{Key: "valor_rateado", Value: int64(500)},
		{Key: "tipo_cobranca", Value: "DIVISAO"},
		{Key: "pendente_recalculo", Value: false},
		{Key: "criado_em", Value: now},
		{Key: "atualizado_em", Value: now},
	})
	require.NoError(t, err, "seed: insert rateio ATIVO")

	// ── Finance Summary ───────────────────────────────────────────────
	// Pre-populated with Java-equivalent values.
	// GAP-001: In Java, collected = Σ installment_allocations.PAID.
	//          In Go, collected is read directly from this document.
	//          See specs/167-go-finance-use-cases/gaps.md.
	_, err = client.Collection("finance_summaries").InsertOne(ctx, bson.D{
		{Key: "event_id", Value: mongoadapter.UUIDStringToBinary(parityPrimEvtID)},
		{Key: "goal", Value: parityExpectedGoal},
		{Key: "collected", Value: parityExpectedCollected},                 // from Java installment_allocations PAID
		{Key: "progress_percentage", Value: parityExpectedProgress},
		{Key: "available_for_withdrawal", Value: parityExpectedAvailable},
		{Key: "pending_withdrawals", Value: parityExpectedPendingWithdrawals},
		{Key: "last_calculated_at", Value: now},
	})
	require.NoError(t, err, "seed: insert finance summary")

	// ── Payment Installments (fallback Go — não são installment_allocations) ─
	// GAP-001: Go's RecalculateFinanceSummary uses payment_installments.PAID
	//          as a fallback. Two PAID installments = 15_000 each = 30_000 total,
	//          matching the pre-seeded finance_summary.collected.
	paidAt := now.Add(-24 * time.Hour)
	for _, amount := range []int64{15_000, 15_000} {
		_, err = client.Collection("payment_installments").InsertOne(ctx, bson.D{
			{Key: "event_id", Value: mongoadapter.UUIDStringToBinary(parityPrimEvtID)},
			{Key: "amount", Value: amount},
			{Key: "status", Value: "PAID"},
			{Key: "payment_method", Value: "PIX"},
			{Key: "paid_at", Value: paidAt},
		})
		require.NoError(t, err, "seed: insert installment PAID")
	}
	_, err = client.Collection("payment_installments").InsertOne(ctx, bson.D{
		{Key: "event_id", Value: mongoadapter.UUIDStringToBinary(parityPrimEvtID)},
		{Key: "amount", Value: int64(5_000)},
		{Key: "status", Value: "PENDING"},
		{Key: "payment_method", Value: "BOLETO"},
	})
	require.NoError(t, err, "seed: insert installment PENDING")

	// ── Ledger Entries (CREDIT×5, DEBIT×2 — total 7) ─────────────────
	// Timestamps spread over 7 hours so sort order is deterministic.
	baseTime := now.Truncate(time.Millisecond)
	ledgerRows := []struct {
		typ    string
		amount int64
		offset time.Duration
	}{
		{"CREDIT", 10_000, 0},
		{"CREDIT", 20_000, -1 * time.Hour},
		{"CREDIT", 30_000, -2 * time.Hour},
		{"CREDIT", 40_000, -3 * time.Hour},
		{"DEBIT", 5_000, -4 * time.Hour},
		{"DEBIT", 3_000, -5 * time.Hour},
		{"CREDIT", 50_000, -6 * time.Hour},
	}
	for i, row := range ledgerRows {
		_, err = client.Collection("ledger_entries").InsertOne(ctx, bson.D{
			{Key: "event_id", Value: mongoadapter.UUIDStringToBinary(parityPrimEvtID)},
			{Key: "type", Value: row.typ},
			{Key: "amount", Value: row.amount},
			{Key: "description", Value: fmt.Sprintf("parity entry %d", i+1)},
			{Key: "occurred_at", Value: baseTime.Add(row.offset)},
			{Key: "accounting_classification", Value: "REVENUE"},
		})
		require.NoError(t, err, "seed: insert ledger entry %d", i+1)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestParityFinanceDomain — main parity test suite
//
// A single Testcontainers MongoDB is shared across all subtests to avoid
// the overhead of starting multiple containers.
// ─────────────────────────────────────────────────────────────────────────────

func TestParityFinanceDomain(t *testing.T) {
	client := testhelper.StartMongo(t)
	seedParityData(t, client)
	ctx := context.Background()

	// Wire up real MongoDB adapters — no stubs.
	summaryRepo     := mongoadapter.NewFinanceSummaryRepository(client)
	ledgerRepo      := mongoadapter.NewLedgerEntryRepository(client)
	participantRepo := mongoadapter.NewParticipantRepository(client)
	rateioRepo      := mongoadapter.NewRateioRepository(client)
	eventoRepo      := mongoadapter.NewEventoRepository(client)

	// ─────────────────────────────────────────────────────────────────
	// UC 2: GetFinanceOverview — campo-a-campo vs Java
	// ─────────────────────────────────────────────────────────────────

	t.Run("GetFinanceOverview_goal_campo_a_campo", func(t *testing.T) {
		uc := ucfinance.NewGetFinanceOverview(eventoRepo, participantRepo, rateioRepo, summaryRepo)
		got, err := uc.Execute(ctx, portin.GetFinanceOverviewInput{
			EventID: parityPrimEvtID,
			UserID:  parityUserUUID,
		})
		require.NoError(t, err)
		require.NotNil(t, got)

		// goal = Σ rateios ATIVO × 100 centavos
		assert.Equal(t, parityExpectedGoal, got.Goal,
			"goal deve ser = Σ rateios ATIVO * 100 (valor_rateado=500 → 50000 centavos)")
	})

	t.Run("GetFinanceOverview_collected_campo_a_campo", func(t *testing.T) {
		uc := ucfinance.NewGetFinanceOverview(eventoRepo, participantRepo, rateioRepo, summaryRepo)
		got, err := uc.Execute(ctx, portin.GetFinanceOverviewInput{
			EventID: parityPrimEvtID,
			UserID:  parityUserUUID,
		})
		require.NoError(t, err)
		require.NotNil(t, got)

		// collected vem de finance_summary.collected — mesmo valor que Java calcularia
		// de installment_allocations.PAID (GAP-001: Go lê passivamente do summary)
		assert.Equal(t, parityExpectedCollected, got.Collected,
			"collected deve ser = finance_summary.collected (pré-populado com Java-equivalente)")
	})

	t.Run("GetFinanceOverview_progressPercentage_HALF_UP_4casas", func(t *testing.T) {
		uc := ucfinance.NewGetFinanceOverview(eventoRepo, participantRepo, rateioRepo, summaryRepo)
		got, err := uc.Execute(ctx, portin.GetFinanceOverviewInput{
			EventID: parityPrimEvtID,
			UserID:  parityUserUUID,
		})
		require.NoError(t, err)
		require.NotNil(t, got)

		// progressPercentage = CalculateProgress(30_000, 50_000)
		// = math.Round(60.0 * 10_000) / 10_000 = 60.0000 (HALF_UP, 4 decimal places)
		assert.InDelta(t, parityExpectedProgress, got.ProgressPercentage, 0.00001,
			"progressPercentage deve ser calculado com HALF_UP 4 casas decimais")
	})

	t.Run("GetFinanceOverview_availableForWithdrawal_campo_a_campo", func(t *testing.T) {
		uc := ucfinance.NewGetFinanceOverview(eventoRepo, participantRepo, rateioRepo, summaryRepo)
		got, err := uc.Execute(ctx, portin.GetFinanceOverviewInput{
			EventID: parityPrimEvtID,
			UserID:  parityUserUUID,
		})
		require.NoError(t, err)
		require.NotNil(t, got)

		// availableForWithdrawal = max(0, collected − pendingWithdrawals)
		//                        = max(0, 30_000 − 5_000) = 25_000
		assert.Equal(t, parityExpectedAvailable, got.AvailableForWithdrawal,
			"availableForWithdrawal = max(0, collected − pendingWithdrawals)")
	})

	t.Run("GetFinanceOverview_acesso_negado_usuario_sem_participacao", func(t *testing.T) {
		uc := ucfinance.NewGetFinanceOverview(eventoRepo, participantRepo, rateioRepo, summaryRepo)
		_, err := uc.Execute(ctx, portin.GetFinanceOverviewInput{
			EventID: parityPrimEvtID,
			UserID:  "00000000-0000-0000-0000-999999999999", // sem participação
		})
		require.Error(t, err, "deve retornar erro para usuário sem participação")
		ae, ok := err.(*apierr.APIError)
		require.True(t, ok, "erro deve ser *apierr.APIError")
		assert.Equal(t, 403, ae.Status, "deve retornar 403 Forbidden")
	})

	// ─────────────────────────────────────────────────────────────────
	// UC 1: ListFinanceEvents — filtro de fases + valores campo-a-campo
	// ─────────────────────────────────────────────────────────────────

	t.Run("ListFinanceEvents_exclui_ORGANIZACAO", func(t *testing.T) {
		uc := ucfinance.NewListFinanceEvents(participantRepo, eventoRepo, rateioRepo, summaryRepo)
		got, err := uc.Execute(ctx, portin.ListFinanceEventsInput{UserID: parityUserUUID})
		require.NoError(t, err)

		// Usuário tem 3 participações: ATIVO + ORGANIZACAO + AGUARDANDO_ACEITE.
		// ORGANIZACAO deve ser excluído → não contar o parityOrgEvtID.
		eventIDs := make([]string, len(got))
		for i, ev := range got {
			eventIDs[i] = ev.EventID
		}
		assert.NotContains(t, eventIDs, parityOrgEvtID,
			"evento com participação ORGANIZACAO deve ser excluído de ListFinanceEvents")
	})

	t.Run("ListFinanceEvents_exclui_AGUARDANDO_ACEITE", func(t *testing.T) {
		uc := ucfinance.NewListFinanceEvents(participantRepo, eventoRepo, rateioRepo, summaryRepo)
		got, err := uc.Execute(ctx, portin.ListFinanceEventsInput{UserID: parityUserUUID})
		require.NoError(t, err)

		eventIDs := make([]string, len(got))
		for i, ev := range got {
			eventIDs[i] = ev.EventID
		}
		assert.NotContains(t, eventIDs, parityAAEvtID,
			"evento com participação AGUARDANDO_ACEITE deve ser excluído de ListFinanceEvents")
	})

	t.Run("ListFinanceEvents_inclui_apenas_ATIVO_retorna_1_evento", func(t *testing.T) {
		uc := ucfinance.NewListFinanceEvents(participantRepo, eventoRepo, rateioRepo, summaryRepo)
		got, err := uc.Execute(ctx, portin.ListFinanceEventsInput{UserID: parityUserUUID})
		require.NoError(t, err)

		require.Len(t, got, 1,
			"apenas o evento com participação ATIVO deve ser retornado (3 participações no seed, 2 excluídas)")
		assert.Equal(t, parityPrimEvtID, got[0].EventID,
			"o evento retornado deve ser o evento principal (ATIVO)")
	})

	t.Run("ListFinanceEvents_goal_campo_a_campo", func(t *testing.T) {
		uc := ucfinance.NewListFinanceEvents(participantRepo, eventoRepo, rateioRepo, summaryRepo)
		got, err := uc.Execute(ctx, portin.ListFinanceEventsInput{UserID: parityUserUUID})
		require.NoError(t, err)
		require.Len(t, got, 1)

		assert.Equal(t, parityExpectedGoal, got[0].Goal,
			"goal no ListFinanceEvents deve corresponder ao rateio ATIVO do seed")
	})

	t.Run("ListFinanceEvents_collected_campo_a_campo", func(t *testing.T) {
		uc := ucfinance.NewListFinanceEvents(participantRepo, eventoRepo, rateioRepo, summaryRepo)
		got, err := uc.Execute(ctx, portin.ListFinanceEventsInput{UserID: parityUserUUID})
		require.NoError(t, err)
		require.Len(t, got, 1)

		// GAP-001: collected lido de finance_summary.collected
		assert.Equal(t, parityExpectedCollected, got[0].Collected,
			"collected deve vir de finance_summary.collected (Java-equivalente via seed)")
	})

	t.Run("ListFinanceEvents_progressPercentage_campo_a_campo", func(t *testing.T) {
		uc := ucfinance.NewListFinanceEvents(participantRepo, eventoRepo, rateioRepo, summaryRepo)
		got, err := uc.Execute(ctx, portin.ListFinanceEventsInput{UserID: parityUserUUID})
		require.NoError(t, err)
		require.Len(t, got, 1)

		assert.InDelta(t, parityExpectedProgress, got[0].ProgressPercentage, 0.00001,
			"progressPercentage deve ser calculado com HALF_UP 4 casas decimais")
	})

	// ─────────────────────────────────────────────────────────────────
	// UC 3: GetLedgerStatement — paginação, ordem, filtros
	// ─────────────────────────────────────────────────────────────────

	t.Run("GetLedgerStatement_paginacao_page1_size3_retorna_3", func(t *testing.T) {
		uc := ucfinance.NewGetLedgerStatement(ledgerRepo, participantRepo)
		got, err := uc.Execute(ctx, portin.GetLedgerStatementInput{
			EventID: parityPrimEvtID,
			UserID:  parityUserUUID,
			Type:    "all",
			Page:    1,
			Size:    3,
		})
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Len(t, got.Entries, 3, "página 1 com size=3 deve retornar 3 entradas")
		assert.Equal(t, parityLedgerTotal, got.TotalElements,
			"total deve ser 7 entradas no seed (5 CREDIT + 2 DEBIT)")
	})

	t.Run("GetLedgerStatement_paginacao_page2_size3_retorna_3", func(t *testing.T) {
		uc := ucfinance.NewGetLedgerStatement(ledgerRepo, participantRepo)
		got, err := uc.Execute(ctx, portin.GetLedgerStatementInput{
			EventID: parityPrimEvtID,
			UserID:  parityUserUUID,
			Type:    "all",
			Page:    2,
			Size:    3,
		})
		require.NoError(t, err)
		assert.Len(t, got.Entries, 3, "página 2 com size=3 deve retornar 3 entradas")
	})

	t.Run("GetLedgerStatement_paginacao_page3_size3_retorna_1", func(t *testing.T) {
		uc := ucfinance.NewGetLedgerStatement(ledgerRepo, participantRepo)
		got, err := uc.Execute(ctx, portin.GetLedgerStatementInput{
			EventID: parityPrimEvtID,
			UserID:  parityUserUUID,
			Type:    "all",
			Page:    3,
			Size:    3,
		})
		require.NoError(t, err)
		assert.Len(t, got.Entries, 1, "página 3 com size=3 deve retornar 1 entrada (resto)")
	})

	t.Run("GetLedgerStatement_ordem_occurredAt_desc", func(t *testing.T) {
		uc := ucfinance.NewGetLedgerStatement(ledgerRepo, participantRepo)
		got, err := uc.Execute(ctx, portin.GetLedgerStatementInput{
			EventID: parityPrimEvtID,
			UserID:  parityUserUUID,
			Type:    "all",
			Page:    1,
			Size:    10,
		})
		require.NoError(t, err)
		require.True(t, len(got.Entries) > 1, "deve haver mais de uma entrada para testar ordem")

		for i := 1; i < len(got.Entries); i++ {
			assert.False(t,
				got.Entries[i].OccurredAt.After(got.Entries[i-1].OccurredAt),
				"entry[%d].OccurredAt (%v) deve ser ≤ entry[%d].OccurredAt (%v) — ordem DESC esperada",
				i, got.Entries[i].OccurredAt,
				i-1, got.Entries[i-1].OccurredAt,
			)
		}
	})

	t.Run("GetLedgerStatement_filtro_type_CREDIT", func(t *testing.T) {
		uc := ucfinance.NewGetLedgerStatement(ledgerRepo, participantRepo)
		got, err := uc.Execute(ctx, portin.GetLedgerStatementInput{
			EventID: parityPrimEvtID,
			UserID:  parityUserUUID,
			Type:    "CREDIT",
			Page:    1,
			Size:    10,
		})
		require.NoError(t, err)
		assert.Equal(t, parityLedgerCredit, got.TotalElements,
			"filtro type=CREDIT deve retornar somente as 5 entradas CREDIT do seed")
		for _, e := range got.Entries {
			assert.Equal(t, "CREDIT", e.Type,
				"todas as entradas com filtro CREDIT devem ter type=CREDIT")
		}
	})

	t.Run("GetLedgerStatement_filtro_type_DEBIT", func(t *testing.T) {
		uc := ucfinance.NewGetLedgerStatement(ledgerRepo, participantRepo)
		got, err := uc.Execute(ctx, portin.GetLedgerStatementInput{
			EventID: parityPrimEvtID,
			UserID:  parityUserUUID,
			Type:    "DEBIT",
			Page:    1,
			Size:    10,
		})
		require.NoError(t, err)
		assert.Equal(t, parityLedgerDebit, got.TotalElements,
			"filtro type=DEBIT deve retornar somente as 2 entradas DEBIT do seed")
		for _, e := range got.Entries {
			assert.Equal(t, "DEBIT", e.Type,
				"todas as entradas com filtro DEBIT devem ter type=DEBIT")
		}
	})

	t.Run("GetLedgerStatement_acesso_negado_usuario_sem_participacao", func(t *testing.T) {
		uc := ucfinance.NewGetLedgerStatement(ledgerRepo, participantRepo)
		_, err := uc.Execute(ctx, portin.GetLedgerStatementInput{
			EventID: parityPrimEvtID,
			UserID:  "00000000-0000-0000-0000-999999999999",
			Type:    "all",
			Page:    1,
			Size:    10,
		})
		require.Error(t, err)
		ae, ok := err.(*apierr.APIError)
		require.True(t, ok)
		assert.Equal(t, 403, ae.Status, "deve retornar 403 para usuário sem participação")
	})
}
