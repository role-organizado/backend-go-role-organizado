// Package auth holds the Identity & Authentication domain entities.
package auth

import "time"

// Role represents a user authorization role.
type Role string

const (
	RoleUser      Role = "USER"
	RoleAdmin     Role = "ADMIN"
	RoleModerator Role = "MODERATOR"
)

// ProviderLogin represents a linked social login provider.
type ProviderLogin struct {
	Provider      string
	ProviderUserID string
	Nome          string
	Email         string
	FotoPerfil    string
}

// Telefone represents a phone number.
type Telefone struct {
	DDI    string
	DDD    string
	Numero string
	Tipo   string
}

// Endereco represents a postal address.
type Endereco struct {
	Rua        string
	Numero     string
	Complemento string
	Bairro     string
	Cidade     string
	Estado     string
	CEP        string
}

// Usuario is the core user entity.
// Collection: usuarios
type Usuario struct {
	ID             string
	Nome           string
	Email          string
	CPF            string
	SenhaHash      string
	DataNascimento *time.Time
	FotoPerfil     string
	Telefone       *Telefone
	Endereco       *Endereco
	ProviderLogin  []ProviderLogin
	Roles          []Role
	Ativo          bool
	CriadoEm      time.Time
	UpdatedAt      time.Time
}

// HasRole returns true if the user has the given role.
func (u *Usuario) HasRole(r Role) bool {
	for _, role := range u.Roles {
		if role == r {
			return true
		}
	}
	return false
}

// RoleStrings returns roles as string slice (for JWT claims).
func (u *Usuario) RoleStrings() []string {
	out := make([]string, len(u.Roles))
	for i, r := range u.Roles {
		out[i] = string(r)
	}
	return out
}
