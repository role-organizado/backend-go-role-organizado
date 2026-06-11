package guest

import "time"

// ChallengeTTL is the lifespan of a biometric challenge: exactly 5 minutes (Java parity).
const ChallengeTTL = 5 * time.Minute

// BiometricChallenge is a single-use random nonce that the client signs with its
// device-bound private key to prove possession during biometric authentication.
// Collection: biometric_challenges (TTL index on ExpiresAt drops expired docs).
type BiometricChallenge struct {
	ID         string
	DeviceID   string
	Challenge  string // 32-byte SecureRandom nonce, Base64-encoded
	CreatedAt  time.Time
	ExpiresAt  time.Time
	Used       bool
	UsedAt     *time.Time
	UsedFromIP string
}

// IsExpired returns true when the challenge has passed its 5-minute window.
func (c *BiometricChallenge) IsExpired() bool {
	return time.Now().After(c.ExpiresAt)
}
