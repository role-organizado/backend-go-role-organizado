package guest

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"log/slog"
	"time"

	authdomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/auth"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
	"github.com/role-organizado/backend-go-role-organizado/pkg/jwt"
)

// BiometricAuthenticate implements portin.BiometricAuthenticateUseCase.
type BiometricAuthenticate struct {
	creds      portout.BiometricCredentialRepository
	challenges portout.BiometricChallengeRepository
	usuarios   portout.UsuarioRepository
	refreshes  portout.RefreshTokenRepository
	jwtSvc     *jwt.Service
}

// NewBiometricAuthenticate wires the use case.
func NewBiometricAuthenticate(
	creds portout.BiometricCredentialRepository,
	challenges portout.BiometricChallengeRepository,
	usuarios portout.UsuarioRepository,
	refreshes portout.RefreshTokenRepository,
	jwtSvc *jwt.Service,
) *BiometricAuthenticate {
	return &BiometricAuthenticate{
		creds:      creds,
		challenges: challenges,
		usuarios:   usuarios,
		refreshes:  refreshes,
		jwtSvc:     jwtSvc,
	}
}

// Execute finds the active credential, validates the most-recent unused non-expired
// challenge, verifies the SHA256withECDSA (or SHA256withRSA) signature against the
// stored public key, marks the challenge used, then issues a fresh JWT pair.
//
// All failures funnel to apierr.Unauthorized to mirror Java's
// InvalidCredentialsException → 401 mapping.
func (uc *BiometricAuthenticate) Execute(
	ctx context.Context,
	in portin.BiometricAuthenticateInput,
	clientIP string,
) (*portin.AuthOutput, error) {
	if in.UsuarioID == "" || in.DeviceID == "" || in.Signature == "" {
		return nil, apierr.BadRequest("usuarioId, deviceId e signature são obrigatórios")
	}

	cred, err := uc.creds.FindByUsuarioIDAndDeviceIDActive(ctx, in.UsuarioID, in.DeviceID)
	if err != nil {
		if apierr.IsNotFound(err) {
			return nil, apierr.Unauthorized("credenciais biométricas inválidas")
		}
		return nil, err
	}

	challenge, err := uc.challenges.FindLatestByDeviceIDUnused(ctx, in.DeviceID)
	if err != nil {
		if apierr.IsNotFound(err) {
			return nil, apierr.Unauthorized("nenhum desafio pendente para o dispositivo")
		}
		return nil, err
	}
	if challenge.IsExpired() {
		return nil, apierr.Unauthorized("desafio expirado")
	}

	challengeBytes, err := base64.StdEncoding.DecodeString(challenge.Challenge)
	if err != nil {
		return nil, apierr.Unauthorized("desafio inválido")
	}
	sigBytes, err := base64.StdEncoding.DecodeString(in.Signature)
	if err != nil {
		return nil, apierr.Unauthorized("assinatura inválida (Base64)")
	}
	pubKey, err := parsePublicKey(cred.PublicKey)
	if err != nil {
		slog.WarnContext(ctx, "biometric public key parse failed", "deviceId", in.DeviceID, "error", err)
		return nil, apierr.Unauthorized("chave pública corrompida")
	}
	if !verifySignature(pubKey, challengeBytes, sigBytes) {
		return nil, apierr.Unauthorized("assinatura biométrica inválida")
	}

	now := time.Now().UTC()
	challenge.Used = true
	challenge.UsedAt = &now
	challenge.UsedFromIP = clientIP
	if _, err := uc.challenges.Update(ctx, challenge); err != nil {
		return nil, err
	}
	cred.LastUsedAt = now
	if _, err := uc.creds.Update(ctx, cred); err != nil {
		return nil, err
	}

	usuario, err := uc.usuarios.FindByID(ctx, in.UsuarioID)
	if err != nil {
		if apierr.IsNotFound(err) {
			return nil, apierr.Unauthorized("usuário não encontrado")
		}
		return nil, err
	}

	pair, err := uc.jwtSvc.GenerateTokenPair(usuario.ID, usuario.Email, usuario.Nome, "", usuario.RoleStrings())
	if err != nil {
		return nil, fmt.Errorf("generating tokens: %w", err)
	}
	rtEntity := &authdomain.RefreshToken{
		UsuarioID: usuario.ID,
		Token:     pair.RefreshToken,
		ExpiresAt: pair.RefreshExpiresAt,
	}
	if _, err := uc.refreshes.Save(ctx, rtEntity); err != nil {
		return nil, fmt.Errorf("saving refresh token: %w", err)
	}

	return &portin.AuthOutput{
		Usuario:      usuario,
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
	}, nil
}

// parsePublicKey accepts either a raw Base64-encoded X.509 SubjectPublicKeyInfo
// (Java parity — the format produced by KeyFactory.getInstance("EC")) or a PEM
// "PUBLIC KEY" block. Returns either *ecdsa.PublicKey or *rsa.PublicKey.
func parsePublicKey(material string) (crypto.PublicKey, error) {
	if material == "" {
		return nil, fmt.Errorf("public key vazia")
	}
	// Try PEM first.
	if block, _ := pem.Decode([]byte(material)); block != nil {
		return x509.ParsePKIXPublicKey(block.Bytes)
	}
	// Fallback to raw Base64-encoded DER (X.509 SubjectPublicKeyInfo).
	der, err := base64.StdEncoding.DecodeString(material)
	if err != nil {
		return nil, fmt.Errorf("public key base64: %w", err)
	}
	return x509.ParsePKIXPublicKey(der)
}

// verifySignature dispatches to ECDSA (preferred — matches Android FIDO defaults)
// or RSA SHA-256 PKCS#1v1.5 verification.
func verifySignature(pub crypto.PublicKey, data, sig []byte) bool {
	hash := sha256.Sum256(data)
	switch k := pub.(type) {
	case *ecdsa.PublicKey:
		return ecdsa.VerifyASN1(k, hash[:], sig)
	case *rsa.PublicKey:
		return rsa.VerifyPKCS1v15(k, crypto.SHA256, hash[:], sig) == nil
	default:
		return false
	}
}
