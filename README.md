# backend-go-role-organizado

Go 1.23+ backend — migração gradual do Java Spring Boot via **Strangler Fig Pattern**.

## Visão Geral

- **Port**: `8090` durante coexistência com Java (Java mantém `8080`)
- **Padrão**: Strangler Fig — BFF roteia domínio a domínio de Java→Go
- **DB**: MongoDB 6.0+ compartilhado com Java (`role_organizado_dev`)
- **Temporal**: `10.11.12.244:7233` — mesmas task queues do Java
- **JWT**: HMAC-SHA256 idêntico ao Java → tokens intercambiáveis

## Estrutura

```
cmd/server/         → main.go (chi router + graceful shutdown)
internal/
  adapter/
    http/handler/   → HTTP handlers por domínio
    http/middleware/ → Middleware chain (OTel, JWT, CORS, logging)
    mongodb/        → MongoDB client + adapters por domínio
    temporal/       → Workers e workflow stubs
    sqs/            → AWS SQS producer/consumer
    redis/          → Redis cache adapter
  config/           → Viper config (env vars)
  domain/           → Entidades de domínio puras
  port/             → Ports de entrada (in) e saída (out)
  usecase/          → Use cases (application layer)
pkg/
  apierr/           → Tipos de erro HTTP tipados
  jwt/              → Serviço JWT (compatível com Java)
  otel/             → Bootstrap OpenTelemetry SDK
migrations/         → Migrações MongoDB (collection: _migrations_go)
.github/workflows/  → CI: lint + test + build + docker
```

## Setup Local

```bash
# Variáveis de ambiente obrigatórias
export ROLE_JWT_SECRET="<mesmo JWT_SECRET do Java>"
export ROLE_MONGO_URI="mongodb://admin:<senha>@10.11.12.238:27017/role_organizado_dev?authSource=admin&replicaSet=rs0"

# Rodar
go run ./cmd/server/

# Verificar saúde
curl http://localhost:8090/actuator/health
```

## Comandos

```bash
# Build
go build ./cmd/server/...

# Testes
go test ./...

# Lint
golangci-lint run

# Go mod
go mod tidy
```

## Fases de Migração

| Fase | Domínio | Status |
|------|---------|--------|
| 0    | Foundation (scaffold, health, JWT, OTel) | ✅ Done |
| 1    | Auth (/api/v1/auth/*) | 🔜 |
| 2    | Events (/api/v1/events/*) | 🔜 |
| 3    | Participants | 🔜 |
| 4    | Rateio | 🔜 |
| 5    | Payments | 🔜 |
| 6    | Finance | 🔜 |
| 7    | Notifications | 🔜 |
| 8    | Config/Admin | 🔜 |
| 9    | Temporal workers | 🔜 |
| 10   | Traffic cutover | 🔜 |
| 11   | Java decommission | 🔜 |

Spec completa: `role-organizado-workspace/specs/155-backend-go-migration/plan.md`
