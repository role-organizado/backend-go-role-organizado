FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o bin/server ./cmd/server/

# ---
FROM gcr.io/distroless/static-debian12:nonroot AS runner

COPY --from=builder /app/bin/server /server

EXPOSE 8090

ENTRYPOINT ["/server"]
