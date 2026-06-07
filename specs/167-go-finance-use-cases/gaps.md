# Gaps — Spec 167: Go Finance Use Cases vs Java

**Criado**: 2026-06-06
**Autor**: QA Agent (parity test)
**Branch**: feature/167-go-finance-use-cases

---

## GAP-001: `installment_allocations` — Feature 045 ausente no Go

### Descrição

No Java, o campo `collected` (dinheiro arrecadado) é calculado somando as **allocations PAID**
da coleção `installment_allocations`, processada via `InstallmentAllocationService` (Feature 045).

No Go, a coleção `installment_allocations` **não existe**. O repositório `PaymentInstallmentRepository`
opera sobre `payment_installments` (parcelas brutas), não sobre as alocações refinadas.

### Impacto

| Use Case | Java | Go (atual) |
|----------|------|-----------|
| `GetFinanceOverview` | `collected = Σ installment_allocations.PAID` | `collected = finance_summary.collected` (leitura passiva) |
| `ListFinanceEvents` | idem | idem |
| `RecalculateFinanceSummary` | via `InstallmentAllocationService` | `collected = Σ payment_installments.PAID` (fallback) |

### Risco

O valor de `collected` no Go pode **divergir do Java** quando:

1. O evento teve pagamentos com alocações parciais (parte de uma parcela alocada em meses diferentes).
2. A `finance_summary` não foi recalculada após novos pagamentos.
3. Reversões ou estornos que atualizam `installment_allocations` mas não `payment_installments`.

Em eventos com pagamentos simples (uma parcela = uma alocação completa), o comportamento é equivalente.

### Fallback utilizado no parity test

O parity test (`internal/usecase/finance/parity_test.go`) pré-popula `finance_summary.collected`
com o valor Java-equivalente hardcoded no seed. Isso garante que o Go retorna o mesmo valor
que o Java retornaria para o mesmo conjunto de dados — mas assume que a `finance_summary` esteja
correta (atualizada pelo Job de recálculo Java).

```go
// Seed:  finance_summaries.collected = 30_000 centavos
// Representa o valor que Java calcularia de installment_allocations.PAID
// para o evento de teste com 2 parcelas PAID de R$150,00 cada.
```

### Solução recomendada (fora do escopo desta spec)

Criar a coleção `installment_allocations` no Go com o mesmo schema do Java,
e atualizar `RecalculateFinanceSummary` para usar `InstallmentAllocationRepository.sumPaidByEventID`
em vez de `PaymentInstallmentRepository.FindByEventID`.

**Spec sugerida**: spec-168 (Go Finance — Feature 045 Installment Allocations)

---

## GAP-002: `SendPaymentReminders` — stub sem enfileiramento Temporal

### Descrição

O UC `SendPaymentReminders` no Go **deduplica** participantes e chama `PaymentReminderDispatcher.Dispatch`.
Em produção, o `dispatcher` é `nil` (Temporal não configurado), então nenhum lembrete é enviado.

O Java enfileira via `PaymentReminderWorkflow` em Temporal.

### Status

Parcialmente implementado — a lógica de deduplicação está correta, mas o dispatcher precisa ser
wired com o cliente Temporal no servidor. Documentado como gap operacional, não de lógica.

---

## GAP-003: Endpoints ausentes no Go (identificados na spec-167)

Os seguintes endpoints do Java `FinanceController` ainda **não existem** no Go:

| Endpoint | Java UC | Status Go |
|----------|---------|-----------|
| `GET /{eventId}/participants-status` | `GetParticipantsStatusUseCase` | ✅ Implementado (handler presente) |
| `GET /{eventId}/ledger/statement` | `GetLedgerStatementUseCase` | ✅ Implementado |
| `GET /{eventId}/summary` | `GetFinanceOverviewUseCase` (reutilizado) | ✅ Implementado |
| `POST /{eventId}/summary/recalculate` | `RecalculateFinanceSummaryUseCase` | ✅ Implementado |
| `GET /{eventId}/audit` | `GetAuditTrailUseCase` | ✅ Implementado |
| `GET /{eventId}/hold-balance` | `CalculateHoldBalanceUseCase` | ✅ Implementado |
| `GET /{eventId}/payment-status` | `GetEventPaymentStatusUseCase` | ✅ Implementado |

> Todos os endpoints foram implementados nesta spec (167). Não há gaps nesta categoria.

---

## Resumo de risco para ativação da flag `strangler.finance`

| Gap | Risco | Bloqueia flag? |
|-----|-------|:-------------:|
| GAP-001: installment_allocations | Médio (divergência em eventos com alocações parciais) | ⚠️ Condicional |
| GAP-002: Temporal dispatcher | Baixo (operacional, não de dados) | ❌ Não |
| GAP-003: endpoints ausentes | Resolvido | ❌ Não |

**Recomendação**: ativar a flag em staging apenas para eventos com pagamentos simples (sem alocações
parciais). Monitorar divergências de `collected` via dashboard de parity por 1 semana antes de
produção.
