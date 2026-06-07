package payment

import "time"

// CardBrand identifies the credit card network / brand.
type CardBrand string

const (
	CardBrandVisa       CardBrand = "VISA"
	CardBrandMastercard CardBrand = "MASTERCARD"
	CardBrandElo        CardBrand = "ELO"
	CardBrandAmex       CardBrand = "AMEX"
	CardBrandHipercard  CardBrand = "HIPERCARD"
	CardBrandDiners     CardBrand = "DINERS"
	CardBrandDiscover   CardBrand = "DISCOVER"
	CardBrandJCB        CardBrand = "JCB"
	CardBrandAura       CardBrand = "AURA"
	CardBrandOther      CardBrand = "OTHER"
)

// SavedCreditCard is a tokenized credit card stored for use in future payments.
// The actual card number is never persisted — only the provider token reference.
type SavedCreditCard struct {
	ID             string
	UserID         string
	LastFourDigits string
	Brand          CardBrand
	HolderName     string
	ExpirationDate string // formatted as MM/YYYY
	TokenRef       string // provider tokenisation reference (e.g. Asaas token)
	IsDefault      bool
	IsActive       bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
