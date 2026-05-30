package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/auth"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
	"github.com/role-organizado/backend-go-role-organizado/pkg/jwt"
)

// AppleAuth implements portin.AppleAuthUseCase.
type AppleAuth struct {
	usuarios  portout.UsuarioRepository
	refreshes portout.RefreshTokenRepository
	jwtSvc    *jwt.Service
}

// NewAppleAuth creates a new AppleAuth use case.
func NewAppleAuth(u portout.UsuarioRepository, rt portout.RefreshTokenRepository, j *jwt.Service) *AppleAuth {
	return &AppleAuth{usuarios: u, refreshes: rt, jwtSvc: j}
}

// appleJWKSet holds Apple's public keys from their JWKS endpoint.
type appleJWKSet struct {
	Keys []appleJWK `json:"keys"`
}

type appleJWK struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// appleClaims holds the identity token claims we care about.
type appleClaims struct {
	Sub   string `json:"sub"`
	Email string `json:"email"`
	gojwt.RegisteredClaims
}

const appleJWKSURL = "https://appleid.apple.com/auth/keys"
const appleIssuer = "https://appleid.apple.com"

// Execute validates the Apple identity token and upserts the user.
func (uc *AppleAuth) Execute(ctx context.Context, in portin.AppleAuthInput) (*portin.AuthOutput, error) {
	slog.InfoContext(ctx, "apple auth")

	claims, err := verifyAppleIdentityToken(ctx, in.IdentityToken)
	if err != nil {
		return nil, apierr.Unauthorized("token Apple inválido: " + err.Error())
	}

	usuario, err := uc.usuarios.FindByProviderID(ctx, "apple", claims.Sub)
	if err != nil && !apierr.IsNotFound(err) {
		return nil, err
	}

	if usuario == nil && claims.Email != "" {
		usuario, err = uc.usuarios.FindByEmail(ctx, claims.Email)
		if err != nil && !apierr.IsNotFound(err) {
			return nil, err
		}
	}

	if usuario == nil {
		nome := in.Nome
		if nome == "" {
			nome = claims.Email
		}
		usuario = &domain.Usuario{
			Nome:  nome,
			Email: claims.Email,
			Ativo: true,
			Roles: []domain.Role{domain.RoleUser},
			ProviderLogin: []domain.ProviderLogin{
				{Provider: "apple", ProviderUserID: claims.Sub, Email: claims.Email},
			},
		}
		usuario, err = uc.usuarios.Save(ctx, usuario)
		if err != nil {
			return nil, err
		}
	} else {
		found := false
		for _, p := range usuario.ProviderLogin {
			if p.Provider == "apple" && p.ProviderUserID == claims.Sub {
				found = true
				break
			}
		}
		if !found {
			usuario.ProviderLogin = append(usuario.ProviderLogin, domain.ProviderLogin{
				Provider: "apple", ProviderUserID: claims.Sub, Email: claims.Email,
			})
			usuario, err = uc.usuarios.Update(ctx, usuario)
			if err != nil {
				return nil, err
			}
		}
	}

	return issueTokens(ctx, usuario, uc.jwtSvc, uc.refreshes)
}

// verifyAppleIdentityToken fetches Apple JWKS and validates the identity token.
func verifyAppleIdentityToken(ctx context.Context, identityToken string) (*appleClaims, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(reqCtx, http.MethodGet, appleJWKSURL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching Apple JWKS: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var jwks appleJWKSet
	if err := json.Unmarshal(body, &jwks); err != nil {
		return nil, fmt.Errorf("parsing Apple JWKS: %w", err)
	}

	keyFunc := func(token *gojwt.Token) (any, error) {
		if _, ok := token.Method.(*gojwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		kid, _ := token.Header["kid"].(string)
		for _, k := range jwks.Keys {
			if k.Kid == kid {
				return jwkToRSAPublicKey(k)
			}
		}
		return nil, fmt.Errorf("matching key not found for kid=%s", kid)
	}

	var claims appleClaims
	parsed, err := gojwt.ParseWithClaims(identityToken, &claims, keyFunc,
		gojwt.WithIssuer(appleIssuer),
		gojwt.WithExpirationRequired(),
	)
	if err != nil || !parsed.Valid {
		return nil, fmt.Errorf("token validation failed: %w", err)
	}
	return &claims, nil
}

func jwkToRSAPublicKey(k appleJWK) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, err
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, err
	}
	e := int(new(big.Int).SetBytes(eBytes).Int64())
	return &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: e}, nil
}
