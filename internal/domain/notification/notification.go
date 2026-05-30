package notification

import "time"

// TipoNotificacao represents the category of notification.
type TipoNotificacao string

const (
	TipoNotificacaoEvento     TipoNotificacao = "EVENTO"
	TipoNotificacaoRateio     TipoNotificacao = "RATEIO"
	TipoNotificacaoPagamento  TipoNotificacao = "PAGAMENTO"
	TipoNotificacaoConvite    TipoNotificacao = "CONVITE"
	TipoNotificacaoSistema    TipoNotificacao = "SISTEMA"
)

// StatusNotificacao represents read/unread lifecycle.
type StatusNotificacao string

const (
	StatusNotificacaoNaoLida StatusNotificacao = "NAO_LIDA"
	StatusNotificacaoLida    StatusNotificacao = "LIDA"
)

// Notificacao is an in-app notification for a user.
type Notificacao struct {
	ID          string
	UsuarioID   string
	Tipo        TipoNotificacao
	Status      StatusNotificacao
	Titulo      string
	Mensagem    string
	Dados       map[string]string // extra context (eventId, rateioId, etc.)
	CriadoEm   time.Time
	LidoEm     *time.Time
}

// IsOwner returns true when userID matches notification owner.
func (n *Notificacao) IsOwner(userID string) bool {
	return n.UsuarioID == userID
}

// IsLida returns true when the notification has been read.
func (n *Notificacao) IsLida() bool {
	return n.Status == StatusNotificacaoLida
}

// Marcar marks the notification as read.
func (n *Notificacao) Marcar() {
	now := time.Now()
	n.Status = StatusNotificacaoLida
	n.LidoEm = &now
}
