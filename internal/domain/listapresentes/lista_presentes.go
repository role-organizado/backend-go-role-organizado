package listapresentes

import "time"

// ListaItemStatus represents the lifecycle of a gift list item.
type ListaItemStatus string

const (
	StatusDisponivel ListaItemStatus = "DISPONIVEL"
	StatusReservado  ListaItemStatus = "RESERVADO"
	StatusEntregue   ListaItemStatus = "ENTREGUE"
)

// ListaPresentesItem represents a single item in a collaborative gift list.
type ListaPresentesItem struct {
	ID                  string
	EventoID            string
	OwnerUserID         string
	Nome                string
	Descricao           string
	URLProduto          string
	Valor               int64  // centavos, 0 = qualquer valor
	Quantidade          int    // quantidade desejada
	Reservado           int    // quantidade já reservada
	Status              ListaItemStatus
	ReservadoPorGuestID string // último que reservou
	CriadoEm           time.Time
	UpdatedAt           time.Time
}

// IsOwner returns true if the given userID owns this item.
func (i *ListaPresentesItem) IsOwner(userID string) bool {
	return i.OwnerUserID == userID
}

// CanReserve returns true if there is still available quantity to reserve.
func (i *ListaPresentesItem) CanReserve() bool {
	return i.Reservado < i.Quantidade
}
