package auth

import "time"

// RefreshToken represents an opaque refresh token stored in MongoDB.
// Collection: refresh_tokens
type RefreshToken struct {
	ID        string
	UsuarioID string
	Token     string
	ExpiresAt time.Time
	Used      bool
	CreatedAt time.Time
}

// IsExpired returns true if the token has passed its expiry time.
func (rt *RefreshToken) IsExpired() bool {
	return time.Now().After(rt.ExpiresAt)
}
