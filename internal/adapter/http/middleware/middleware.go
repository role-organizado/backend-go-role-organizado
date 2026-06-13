// Package middleware provides HTTP middleware for the chi router.
// The middleware chain order (applied outermost-first in main.go) is:
//   OTel → CorrelationID → Logging (slog) → CORS → RateLimit → JWT Auth (protected routes)
package middleware

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
	pkgjwt "github.com/role-organizado/backend-go-role-organizado/pkg/jwt"
)

// contextKey is used for context values set by middleware to avoid collisions.
type contextKey string

const (
	contextKeyUserID contextKey = "userId"
	contextKeyClaims contextKey = "claims"
)

// StructuredLogger returns a chi middleware that logs each request with slog.
// Logs include method, path, status, latency, and correlation-id.
func StructuredLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			defer func() {
				logger.InfoContext(r.Context(), "http request",
					"method", r.Method,
					"path", r.URL.Path,
					"status", ww.Status(),
					"latency_ms", time.Since(start).Milliseconds(),
					"correlation_id", middleware.GetReqID(r.Context()),
					"remote_addr", r.RemoteAddr,
					"user_agent", r.UserAgent(),
				)
			}()

			next.ServeHTTP(ww, r)
		})
	}
}

// CORS returns a middleware that sets CORS headers.
// allowedOrigins is a list of allowed origins (e.g. ["http://localhost:3000"]).
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[strings.TrimSpace(o)] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if allowed[origin] || allowed["*"] {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Correlation-ID")
			w.Header().Set("Access-Control-Max-Age", "86400")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// JWTAuth returns a middleware that validates JWT tokens in the Authorization header.
// Routes that start with any of the publicPrefixes are passed through without authentication.
func JWTAuth(jwtSvc *pkgjwt.Service, publicPrefixes []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip authentication for public routes (webhooks, health, auth endpoints)
			for _, prefix := range publicPrefixes {
				if strings.HasPrefix(r.URL.Path, prefix) {
					next.ServeHTTP(w, r)
					return
				}
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeError(w, apierr.Unauthorized("missing Authorization header"))
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				writeError(w, apierr.Unauthorized("invalid Authorization header format"))
				return
			}

			claims, err := jwtSvc.ValidateToken(parts[1])
			if err != nil {
				writeError(w, apierr.Unauthorized(err.Error()))
				return
			}

			ctx := context.WithValue(r.Context(), contextKeyUserID, claims.Sub)
			ctx = context.WithValue(ctx, contextKeyClaims, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole returns a middleware that requires the authenticated user to have the given role.
// Must be used after JWTAuth.
func RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := r.Context().Value(contextKeyClaims).(*pkgjwt.Claims)
			if !ok || claims == nil {
				writeError(w, apierr.Unauthorized("unauthenticated"))
				return
			}
			if !claims.HasRole(role) {
				writeError(w, apierr.Forbidden("role "+role+" required"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireAnyRole returns a middleware that requires the authenticated user to have
// at least one of the given roles. Must be used after JWTAuth.
func RequireAnyRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := r.Context().Value(contextKeyClaims).(*pkgjwt.Claims)
			if !ok || claims == nil {
				writeError(w, apierr.Unauthorized("unauthenticated"))
				return
			}
			for _, role := range roles {
				if claims.HasRole(role) {
					next.ServeHTTP(w, r)
					return
				}
			}
			writeError(w, apierr.Forbidden("one of roles "+strings.Join(roles, ", ")+" required"))
		})
	}
}

// WithClaimsContext injects JWT claims into ctx. Primarily used in tests to
// simulate an authenticated request without a full JWT stack.
func WithClaimsContext(ctx context.Context, claims *pkgjwt.Claims) context.Context {
	return context.WithValue(ctx, contextKeyClaims, claims)
}

// UserIDFromContext extracts the authenticated user ID from the request context.
// Returns empty string if not authenticated.
func UserIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(contextKeyUserID).(string)
	return v
}

// WithUserIDContext injects userID into ctx as if the user were authenticated.
// Primarily used in tests to simulate authenticated requests without a full JWT stack.
func WithUserIDContext(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, contextKeyUserID, userID)
}

// ContextWithUserID is an alias for WithUserIDContext.
func ContextWithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, contextKeyUserID, userID)
}

// ClaimsFromContext extracts the JWT claims from the request context.
// Returns nil if not authenticated.
func ClaimsFromContext(ctx context.Context) *pkgjwt.Claims {
	v, _ := ctx.Value(contextKeyClaims).(*pkgjwt.Claims)
	return v
}

// writeError writes an API error as a JSON response.
func writeError(w http.ResponseWriter, err *apierr.APIError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(err.Status)
	_ = json.NewEncoder(w).Encode(err)
}

// ErrorHandler returns a middleware that recovers from panics and converts *apierr.APIError
// returned via panic (or via the request context) into JSON HTTP responses.
func ErrorHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.ErrorContext(r.Context(), "panic recovered",
					"error", rec,
					"path", r.URL.Path,
				)
				writeError(w, apierr.Internal("unexpected server error"))
			}
		}()
		next.ServeHTTP(w, r)
	})
}
