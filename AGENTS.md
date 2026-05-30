# AGENTS.md — backend-go-role-organizado

## Contexto

Backend Go 1.23+ em migração Strangler Fig do Java Spring Boot.
**Spec completa**: `role-organizado-workspace/specs/155-backend-go-migration/plan.md`

## Stack

| Componente | Lib                                                                   |
| ---------- | --------------------------------------------------------------------- |
| Router     | `github.com/go-chi/chi/v5`                                            |
| MongoDB    | `go.mongodb.org/mongo-driver/v2`                                      |
| JWT        | `github.com/golang-jwt/jwt/v5` (HMAC-SHA256, interop Java)            |
| Config     | `github.com/spf13/viper` (prefix `ROLE_`)                             |
| OTel       | `go.opentelemetry.io/otel` v1.35+ OTLP HTTP                           |
| Temporal   | `go.temporal.io/sdk` v1.34                                            |
| DI         | `github.com/google/wire` (compile-time)                               |
| Validation | `github.com/go-playground/validator/v10`                              |
| Migrations | `github.com/golang-migrate/migrate/v4` (collection: `_migrations_go`) |
| Tests      | `github.com/stretchr/testify` + `testcontainers-go`                   |

## Arquitetura

- **Hexagonal**: `domain/` → `port/` → `usecase/` → `adapter/`
- **Handler pattern**: `handler.New(deps...)` retorna `*Handler`; rotas montadas via `router.Mount("/path", h.Routes())`
- **Erros HTTP**: sempre `*apierr.APIError` — middleware converte para JSON
- **Contexto de usuário**: `middleware.UserIDFromContext(ctx)` e `middleware.ClaimsFromContext(ctx)`
- **Port**: `8090` (Java fica em `8080` durante coexistência)

## Convenções Go

- Pacotes: `snake_case`
- Tipos/Funções exportadas: `PascalCase`
- Arquivos: `snake_case.go`
- Erros: sempre `fmt.Errorf("contexto: %w", err)` — wrap completo
- `context.Context` é sempre o primeiro parâmetro
- Tests: `*_test.go` com `package xxx_test` (caixa preta), tabela-driven com `t.Run`
- Mocks: interfaces nos ports — use `testify/mock` ou implementações fakes

## Env Vars (prefixo `ROLE_`)

| Var                       | Default                   | Descrição             |
| ------------------------- | ------------------------- | --------------------- |
| `ROLE_JWT_SECRET`         | obrigatório               | Mesmo segredo do Java |
| `ROLE_MONGO_URI`          | mongodb://...10.11.12.238 | URI de conexão        |
| `ROLE_MONGO_DATABASE`     | role_organizado_dev       | Database alvo         |
| `ROLE_SERVER_PORT`        | 8090                      | Porta HTTP            |
| `ROLE_SERVER_ENV`         | local                     | Ambiente              |
| `ROLE_OTEL_ENDPOINT`      | http://localhost:4318     | Coletor OTLP          |
| `ROLE_OTEL_ENABLED`       | false                     | Habilita OTel         |
| `ROLE_TEMPORAL_HOST_PORT` | 10.11.12.244:7233         | Temporal              |
| `ROLE_REDIS_ADDR`         | localhost:6379            | Redis                 |

## Comandos Essenciais

```bash
# Rodar local
go run ./cmd/server/

# Build binário
go build -o bin/server ./cmd/server/

# Testes (sem integration)
go test ./pkg/... ./internal/...

# Testes com tags de integração (Testcontainers)
go test -tags integration ./...

# Lint
golangci-lint run

# Dep tidy
go mod tidy
```

## Health Check

```
GET http://localhost:8090/actuator/health
→ {"status":"UP","components":{"mongo":{"status":"UP"}}}
```

## Status da Migração (Spec 155)

**Todas as 13 fases concluídas.** Backend Go está feature-completo e pronto para receber tráfego via BFF Strangler Fig.

| Fase | Status | Domínio                                                         |
| ---- | ------ | --------------------------------------------------------------- |
| 0    | ✅     | Foundation — chi, MongoDB, JWT, OTel, health check              |
| 1    | ✅     | Configuration Domain (Dominio, FeatureFlag)                     |
| 2    | ✅     | Identity & Auth (Usuario, JWT, Google/Apple OAuth, Biometric)   |
| 3    | ✅     | Events & Drafts (Evento, EventoDraft, wizard, Regra)            |
| 4    | ✅     | Rateios/Cost Splits (Rateio, Itens, Fechamento)                 |
| 5    | ✅     | Payments (Transaction, Installment, PIX/Boleto/Card)            |
| 6    | ✅     | Notifications (Notificacao, templates, SQS)                     |
| 7    | ✅     | File Storage (GridFS para comprovantes)                         |
| 8    | ✅     | Temporal Workflow Proxies (todos os 13 workflows)               |
| 9    | ✅     | Admin Routes (handler/config.go + handler/usuario.go)           |
| 10   | ✅     | Observability Enrichment (OTel spans, structured logging)       |
| 11   | ✅     | Integration Tests (Testcontainers, MongoDB 7.0, 11 testes)      |
| 12   | ✅     | Strangler Fig BFF Routing (per-domain flags, zero impacto prod) |
| 13   | ✅     | Java Decommission (documentado, plano de cutover pronto)        |

## Ativando Tráfego por Domínio (Cutover)

No BFF, ativar via env var:

```bash
STRANGLER_AUTH_ENABLED=true       # roteia auth → Go:8090
STRANGLER_EVENTOS_ENABLED=true    # roteia eventos → Go:8090
# ... (ver bff/docs/STRANGLER-FIG-CUTOVER.md para ordem recomendada)
```

Guia completo: `bff-role-organizado/docs/STRANGLER-FIG-CUTOVER.md`

## Adicionando um Novo Domínio

1. Criar entidades em `internal/domain/<dominio>/`
2. Criar ports em `internal/port/in/` e `internal/port/out/`
3. Implementar use case em `internal/usecase/<dominio>/`
4. Implementar adapter MongoDB em `internal/adapter/mongodb/<dominio>.go`
5. Implementar handler HTTP em `internal/adapter/http/handler/<dominio>.go`
6. Montar rotas em `cmd/server/main.go` dentro do grupo protegido
7. Criar migração em `migrations/` se necessário
8. Adicionar flag `STRANGLER_<DOMAIN>_ENABLED` no BFF `env.ts`
