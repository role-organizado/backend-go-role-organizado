// Package config holds the Configuration domain entities:
// Dominio (configurable lookup values) and ConfiguracaoSistema (system settings).
package config

import "time"

// Dominio represents a configurable lookup value of the system.
// Examples: event types, payment methods, cancellation policies, etc.
// Collection: dominios
type Dominio struct {
	ID        string
	Categoria string
	Chave     string
	Valor     string
	Descricao string
	Icone     string
	Ordem     int
	Ativo     bool
	Metadata  map[string]any
	CriadoEm  time.Time
	UpdatedAt time.Time
}

// IsApplicableToEventType returns true if the Dominio is applicable to the given event type.
// Used to filter politica_cancelamento by tipoEvento (Feature 008).
// When Metadata has no "tiposEventoAplicaveis" key the entry is considered applicable to all types.
func (d *Dominio) IsApplicableToEventType(tipoEvento string) bool {
	if d.Metadata == nil {
		return true
	}
	raw, ok := d.Metadata["tiposEventoAplicaveis"]
	if !ok {
		return true
	}
	switch tipos := raw.(type) {
	case []string:
		for _, t := range tipos {
			if t == tipoEvento {
				return true
			}
		}
		return false
	case []any:
		for _, v := range tipos {
			if s, ok := v.(string); ok && s == tipoEvento {
				return true
			}
		}
		return false
	default:
		return true // unknown structure — include as fallback
	}
}
