package config

import "time"

// ConfiguracaoSistema represents a global key-value system configuration.
// Examples: PAYMENT_SETTINGS, NOTIFICATION_SETTINGS, FEATURE_FLAGS.
// Collection: configuracao_sistema
type ConfiguracaoSistema struct {
	ID        string
	Chave     string
	Valor     map[string]any
	Descricao string
	Ativo     bool
	CriadoEm  time.Time
	UpdatedAt time.Time
}
