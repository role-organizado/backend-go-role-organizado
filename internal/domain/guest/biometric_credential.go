package guest

import "time"

// BiometricCredential is a FIDO2-like credential bound to a (UsuarioID, DeviceID) pair.
// Collection: biometric_credentials
//
// PublicKey is stored as a Base64-encoded X.509 SubjectPublicKeyInfo blob (EC P-256 or
// RSA), matching the Java backend's X509EncodedKeySpec format.
type BiometricCredential struct {
	ID             string
	UsuarioID      string
	DeviceID       string
	PublicKey      string // Base64-encoded X.509 SubjectPublicKeyInfo
	DeviceName     string
	DeviceModel    string
	AndroidVersion string
	CreatedAt      time.Time
	LastUsedAt     time.Time
	IsActive       bool
	RevokedAt      *time.Time
	RevokedReason  string
}

// IsRevoked is the inverse of IsActive (true once revoked).
func (c *BiometricCredential) IsRevoked() bool {
	return !c.IsActive
}
