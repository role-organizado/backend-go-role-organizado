// Package config loads and validates application configuration from environment variables.
// It uses Viper for env var binding with sensible defaults for local development.
package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// AppConfig holds all runtime configuration for the backend.
type AppConfig struct {
	Server   ServerConfig
	MongoDB  MongoDBConfig
	JWT      JWTConfig
	OTel     OTelConfig
	Temporal TemporalConfig
	Redis    RedisConfig
	SQS      SQSConfig
	Asaas    AsaasConfig
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	// Port is the HTTP listen port (default 8090 during Java coexistence).
	Port    int
	// ReadTimeout is the maximum duration for reading the entire request.
	ReadTimeout time.Duration
	// WriteTimeout is the maximum duration before timing out writes of the response.
	WriteTimeout time.Duration
	// ShutdownTimeout is how long to wait for in-flight requests during graceful shutdown.
	ShutdownTimeout time.Duration
	// CORSOrigins is a comma-separated list of allowed CORS origins.
	CORSOrigins []string
	// Env is the deployment environment (local, staging, production).
	Env string
	// JavaBackendURL is the base URL of the Java Spring Boot backend for Strangler Fig proxying.
	JavaBackendURL string
}

// MongoDBConfig holds MongoDB connection settings.
type MongoDBConfig struct {
	URI      string
	Database string
}

// JWTConfig holds JWT signing settings.
type JWTConfig struct {
	// Secret must match the Java backend JWT_SECRET exactly for token interoperability.
	Secret          string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

// OTelConfig holds OpenTelemetry exporter settings.
type OTelConfig struct {
	// Endpoint is the OTLP HTTP collector URL (e.g. "http://otel-staging.rolds.dev:4318").
	Endpoint       string
	ServiceName    string
	ServiceVersion string
	Enabled        bool
}

// TemporalConfig holds Temporal.io connection settings.
type TemporalConfig struct {
	// HostPort is the Temporal frontend address (e.g. "10.11.12.244:7233").
	HostPort  string
	Namespace string
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

// SQSConfig holds AWS SQS settings.
type SQSConfig struct {
	Region    string
	QueueURL  string
}

// AsaasConfig holds Asaas PSP API settings.
type AsaasConfig struct {
	BaseURL string
	APIKey  string
	// WebhookToken is the secret used to validate incoming Asaas webhook calls.
	// Set via ROLE_ASAAS_WEBHOOK_TOKEN.
	WebhookToken string
	// UseMock controls whether to use the in-memory MockProvider instead of the real
	// Asaas HTTP client. Defaults to true so local dev never calls the real API.
	// Set via ROLE_ASAAS_USE_MOCK.
	UseMock bool
}

// loadDotEnv reads a .env file and injects each ROLE_* variable into the process
// environment (only when the variable is not already set, so real env vars always
// take precedence). This makes Viper's AutomaticEnv pick them up correctly.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return // file absent — silently skip
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}
}

// Load reads configuration from environment variables using Viper.
// All env vars follow the pattern ROLE_* (e.g. ROLE_SERVER_PORT, ROLE_MONGO_URI).
// If a .env file exists in the working directory it is loaded before env vars
// (real env vars always take precedence over the file).
func Load() (*AppConfig, error) {
	loadDotEnv(".env")

	v := viper.New()
	v.SetEnvPrefix("ROLE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	setDefaults(v)

	cfg := &AppConfig{
		Server: ServerConfig{
			Port:            v.GetInt("server.port"),
			ReadTimeout:     v.GetDuration("server.read_timeout"),
			WriteTimeout:    v.GetDuration("server.write_timeout"),
			ShutdownTimeout: v.GetDuration("server.shutdown_timeout"),
			CORSOrigins:     strings.Split(v.GetString("server.cors_origins"), ","),
			Env:             v.GetString("server.env"),
			JavaBackendURL:  v.GetString("server.java_backend_url"),
		},
		MongoDB: MongoDBConfig{
			URI:      v.GetString("mongo.uri"),
			Database: v.GetString("mongo.database"),
		},
		JWT: JWTConfig{
			Secret:          v.GetString("jwt.secret"),
			AccessTokenTTL:  v.GetDuration("jwt.access_token_ttl"),
			RefreshTokenTTL: v.GetDuration("jwt.refresh_token_ttl"),
		},
		OTel: OTelConfig{
			Endpoint:       v.GetString("otel.endpoint"),
			ServiceName:    v.GetString("otel.service_name"),
			ServiceVersion: v.GetString("otel.service_version"),
			Enabled:        v.GetBool("otel.enabled"),
		},
		Temporal: TemporalConfig{
			HostPort:  v.GetString("temporal.host_port"),
			Namespace: v.GetString("temporal.namespace"),
		},
		Redis: RedisConfig{
			Addr:     v.GetString("redis.addr"),
			Password: v.GetString("redis.password"),
			DB:       v.GetInt("redis.db"),
		},
		SQS: SQSConfig{
			Region:   v.GetString("sqs.region"),
			QueueURL: v.GetString("sqs.queue_url"),
		},
		Asaas: AsaasConfig{
			BaseURL:      v.GetString("asaas.base_url"),
			APIKey:       v.GetString("asaas.api_key"),
			WebhookToken: v.GetString("asaas.webhook_token"),
			UseMock:      v.GetBool("asaas.use_mock"),
		},
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.port", 8090)
	v.SetDefault("server.read_timeout", "15s")
	v.SetDefault("server.write_timeout", "15s")
	v.SetDefault("server.shutdown_timeout", "30s")
	v.SetDefault("server.cors_origins", "http://localhost:3000,http://localhost:4300")
	v.SetDefault("server.env", "local")
	v.SetDefault("server.java_backend_url", "http://localhost:8080")

	v.SetDefault("mongo.uri", "mongodb://admin:e2952c6f90af7aee28f401c4ffc030b9@10.11.12.238:27017/role_organizado_dev?authSource=admin&replicaSet=rs0")
	v.SetDefault("mongo.database", "role_organizado_dev")

	v.SetDefault("jwt.access_token_ttl", "1h")
	v.SetDefault("jwt.refresh_token_ttl", "168h") // 7 days

	v.SetDefault("otel.endpoint", "http://localhost:4318")
	v.SetDefault("otel.service_name", "backend-go-role-organizado")
	v.SetDefault("otel.service_version", "0.1.0")
	v.SetDefault("otel.enabled", false)

	v.SetDefault("temporal.host_port", "10.11.12.244:7233")
	v.SetDefault("temporal.namespace", "default")

	v.SetDefault("redis.addr", "localhost:6379")
	v.SetDefault("redis.db", 0)

	v.SetDefault("sqs.region", "us-east-1")

	v.SetDefault("asaas.base_url", "https://sandbox.asaas.com/api/v3")
	// UseMock defaults to true so local dev never calls the real Asaas API.
	// Override with ROLE_ASAAS_USE_MOCK=false (or PAYMENT_USE_MOCK=false via BindEnv).
	v.SetDefault("asaas.use_mock", true)
	// Support the legacy env-var name used in some Java deployments.
	_ = v.BindEnv("asaas.use_mock", "PAYMENT_USE_MOCK")
}

func (c *AppConfig) validate() error {
	if c.JWT.Secret == "" {
		return fmt.Errorf("ROLE_JWT_SECRET is required")
	}
	if len(c.JWT.Secret) < 32 {
		return fmt.Errorf("ROLE_JWT_SECRET must be at least 32 characters")
	}
	if c.MongoDB.URI == "" {
		return fmt.Errorf("ROLE_MONGO_URI is required")
	}
	return nil
}
