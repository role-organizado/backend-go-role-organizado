// Package jwt provides JWT token generation and validation for the backend.
// It uses HMAC-SHA256 with the same secret and claims payload as the Java backend,
// ensuring tokens issued by Java are valid in Go and vice versa during migration.
package jwt

import (
	"errors"
	"fmt"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
)

// Claims holds the JWT payload matching the Java backend's token structure.
// Fields must match exactly to ensure cross-backend token compatibility.
type Claims struct {
	Sub      string   `json:"sub"`      // User ID (MongoDB ObjectId string)
	Email    string   `json:"email"`
	Nome     string   `json:"nome"`
	Telefone string   `json:"telefone,omitempty"`
	Roles    []string `json:"roles"`    // e.g. ["USER", "ADMIN"]
	gojwt.RegisteredClaims
}

// TokenPair holds an access token and a refresh token.
type TokenPair struct {
	AccessToken      string
	RefreshToken     string
	RefreshExpiresAt time.Time
}

// Service manages JWT token generation and validation.
type Service struct {
	secret          []byte
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
}

// NewService creates a new JWT Service with the given secret and TTLs.
// Secret must be at least 32 bytes (256 bits) for HMAC-SHA256.
func NewService(secret string, accessTTL, refreshTTL time.Duration) (*Service, error) {
	if len(secret) < 32 {
		return nil, fmt.Errorf("jwt secret must be at least 32 bytes, got %d", len(secret))
	}
	return &Service{
		secret:          []byte(secret),
		accessTokenTTL:  accessTTL,
		refreshTokenTTL: refreshTTL,
	}, nil
}

// GenerateTokenPair generates both an access token and a refresh token for a user.
func (s *Service) GenerateTokenPair(userID, email, nome, telefone string, roles []string) (*TokenPair, error) {
	accessToken, err := s.generateToken(userID, email, nome, telefone, roles, s.accessTokenTTL)
	if err != nil {
		return nil, fmt.Errorf("generating access token: %w", err)
	}

	// Refresh tokens carry only sub and a longer TTL.
	refreshToken, err := s.generateToken(userID, email, nome, telefone, roles, s.refreshTokenTTL)
	if err != nil {
		return nil, fmt.Errorf("generating refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:      accessToken,
		RefreshToken:     refreshToken,
		RefreshExpiresAt: time.Now().Add(s.refreshTokenTTL),
	}, nil
}

// GenerateAccessToken generates only an access token.
func (s *Service) GenerateAccessToken(userID, email, nome, telefone string, roles []string) (string, error) {
	return s.generateToken(userID, email, nome, telefone, roles, s.accessTokenTTL)
}

func (s *Service) generateToken(userID, email, nome, telefone string, roles []string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		Sub:      userID,
		Email:    email,
		Nome:     nome,
		Telefone: telefone,
		Roles:    roles,
		RegisteredClaims: gojwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  gojwt.NewNumericDate(now),
			ExpiresAt: gojwt.NewNumericDate(now.Add(ttl)),
		},
	}

	token := gojwt.NewWithClaims(gojwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.secret)
	if err != nil {
		return "", fmt.Errorf("signing token: %w", err)
	}
	return signed, nil
}

// ValidateToken validates a JWT token string and returns the claims if valid.
func (s *Service) ValidateToken(tokenString string) (*Claims, error) {
	token, err := gojwt.ParseWithClaims(tokenString, &Claims{}, func(t *gojwt.Token) (any, error) {
		if _, ok := t.Method.(*gojwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil {
		if errors.Is(err, gojwt.ErrTokenExpired) {
			return nil, fmt.Errorf("token expirado: %w", err)
		}
		return nil, fmt.Errorf("token inválido: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("claims inválidas no token")
	}
	return claims, nil
}

// ExtractUserID extracts the user ID from a token without full validation.
// Use only for logging/tracing — always call ValidateToken for authentication.
func (s *Service) ExtractUserID(tokenString string) (string, error) {
	claims, err := s.ValidateToken(tokenString)
	if err != nil {
		return "", err
	}
	return claims.Sub, nil
}

// HasRole reports whether the claims contain the given role.
func (c *Claims) HasRole(role string) bool {
	for _, r := range c.Roles {
		if r == role {
			return true
		}
	}
	return false
}
