// Package server bootstraps and starts the HTTP server.
// Phase 0: minimal chi router with health endpoint, middleware chain, and graceful shutdown.
// Subsequent phases will add domain routers as they are migrated from Java.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/handler"
	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/middleware"
	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/mongodb"
	"github.com/role-organizado/backend-go-role-organizado/internal/config"
	pkgjwt "github.com/role-organizado/backend-go-role-organizado/pkg/jwt"
)

// publicPrefixes are routes that bypass JWT authentication.
var publicPrefixes = []string{
	"/actuator/",
	"/api/v1/auth/",
	"/api/auth/",
	"/api/v1/webhooks/",
	"/swagger/",
	"/docs/",
}

func main() {
	// Load configuration from environment variables.
	cfg, err := config.Load()
	if err != nil {
		slog.Error("loading config", "error", err)
		os.Exit(1)
	}

	// Setup structured JSON logging.
	logLevel := slog.LevelInfo
	if cfg.Server.Env == "local" {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger)

	slog.Info("starting backend-go-role-organizado",
		"env", cfg.Server.Env,
		"port", cfg.Server.Port,
	)

	// Connect to MongoDB.
	ctx := context.Background()
	mongoClient, err := mongodb.Connect(ctx, cfg.MongoDB.URI, cfg.MongoDB.Database)
	if err != nil {
		slog.Error("connecting to mongodb", "error", err)
		os.Exit(1)
	}

	// Build JWT service.
	jwtSvc, err := pkgjwt.NewService(
		cfg.JWT.Secret,
		cfg.JWT.AccessTokenTTL,
		cfg.JWT.RefreshTokenTTL,
	)
	if err != nil {
		slog.Error("creating jwt service", "error", err)
		os.Exit(1)
	}

	// Build chi router.
	r := chi.NewRouter()

	// --- Global middleware (applied to every request) ---
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.ErrorHandler)
	r.Use(middleware.StructuredLogger(logger))
	r.Use(middleware.CORS(cfg.Server.CORSOrigins))
	r.Use(chimiddleware.Recoverer)

	// --- Health endpoint (public, no auth required) ---
	r.Get("/actuator/health", handler.HealthHandler(mongoClient))

	// --- Protected routes (JWT required) ---
	r.Group(func(r chi.Router) {
		r.Use(middleware.JWTAuth(jwtSvc, publicPrefixes))

		// TODO Phase 1+: mount domain routers here
		// r.Mount("/api/v1/events", eventRouter)
		// r.Mount("/api/v1/users", userRouter)
	})

	// --- HTTP server ---
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in background.
	go func() {
		slog.Info("http server listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http server error", "error", err)
			os.Exit(1)
		}
	}()

	// --- Graceful shutdown ---
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down gracefully", "timeout", cfg.Server.ShutdownTimeout)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
	}

	if err := mongoClient.Disconnect(shutdownCtx); err != nil {
		slog.Error("mongodb disconnect error", "error", err)
	}

	slog.Info("server exited")
}
