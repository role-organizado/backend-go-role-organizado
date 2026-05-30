# AGENTS.md — backend-go-role-organizado

## Contexto

Backend Go 1.23+ em migração Strangler Fig do Java Spring Boot.
**Spec completa**: `role-organizado-workspace/specs/155-backend-go-migration/plan.md`

## Stack

| Componente | Lib |
|------------|-----|
| Router | `github.com/go-chi/chi/v5` |
| MongoDB | `go.mongodb.org/mongo-driver/v2` |
| JWT | `github.com/golang-jwt/jwt/v5` (HMAC-SHA256, interop Java) |
| Config | `github.com/spf13/viper` (prefix `ROLE_`) |
| OTel | `go.opentelemetry.io/otel` v1.35+ OTLP HTTP |
| Temporal | `go.temporal.io/sdk` v1.34 |
| DI | `github.com/google/wire` (compile-time) |
| Validation | `github.com/go-playground/validator/v10` |
| Migrations | `github.com/golang-migrate/migrate/v4` (collection: `_migrations_go`) |
| Tests | `github.com/stretchr/testify` + `testcontainers-go` |

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

| Var | Default | Descrição |
|-----|---------|-----------|
| `ROLE_JWT_SECRET` | obrigatório | Mesmo segredo do Java |
| `ROLE_MONGO_URI` | mongodb://...10.11.12.238 | URI de conexão |
| `ROLE_MONGO_DATABASE` | role_organizado_dev | Database alvo |
| `ROLE_SERVER_PORT` | 8090 | Porta HTTP |
| `ROLE_SERVER_ENV` | local | Ambiente |
| `ROLE_OTEL_ENDPOINT` | http://localhost:4318 | Coletor OTLP |
| `ROLE_OTEL_ENABLED` | false | Habilita OTel |
| `ROLE_TEMPORAL_HOST_PORT` | 10.11.12.244:7233 | Temporal |
| `ROLE_REDIS_ADDR` | localhost:6379 | Redis |

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

## Fases de Migração (Strangler Fig)

BFF roteia via env vars (`DOMINIO_BACKEND_URL`):
- Java `8080` = padrão até a fase ser completada
- Go `8090` = recebe tráfego quando a fase estiver completa

| Fase | Domínio |
|------|---------|
| 0 ✅ | Foundation |
| 1 🔜 | Auth |
| 2+ | Events, Participants, Rateio, Payments... |

## Adicionando um Novo Domínio

1. Criar entidades em `internal/domain/<dominio>/`
2. Criar ports em `internal/port/in/` e `internal/port/out/`
3. Implementar use case em `internal/usecase/<dominio>/`
4. Implementar adapter MongoDB em `internal/adapter/mongodb/<dominio>.go`
5. Implementar handler HTTP em `internal/adapter/http/handler/<dominio>.go`
6. Montar rotas em `cmd/server/main.go` dentro do grupo protegido
7. Criar migração em `migrations/` se necessário
8. Atualizar BFF env var para rotear para `:8090`
