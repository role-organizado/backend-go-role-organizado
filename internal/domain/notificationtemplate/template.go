// Package notificationtemplate defines the domain for reusable notification templates.
// Templates support variable substitution using {{varName}} syntax.
package notificationtemplate

import (
	"strings"
	"time"
)

// TemplateType classifies the delivery channel of a notification template.
type TemplateType string

const (
	TemplateTypeEmail    TemplateType = "EMAIL"
	TemplateTypePush     TemplateType = "PUSH"
	TemplateTypeSMS      TemplateType = "SMS"
	TemplateTypeInApp    TemplateType = "IN_APP"
	TemplateTypeWhatsApp TemplateType = "WHATSAPP"
)

// TemplateCategoria classifies the business purpose of a notification template.
type TemplateCategoria string

const (
	TemplateCategoriaEvento    TemplateCategoria = "EVENTO"
	TemplateCategoriaRateio    TemplateCategoria = "RATEIO"
	TemplateCategoriaPagamento TemplateCategoria = "PAGAMENTO"
	TemplateCategoriaConvite   TemplateCategoria = "CONVITE"
	TemplateCategoriaMarketing TemplateCategoria = "MARKETING"
	TemplateCategoriaSistema   TemplateCategoria = "SISTEMA"
)

// NotificationTemplate is a reusable message template with variable placeholders.
// Variables follow the {{varName}} syntax and are substituted at render time.
type NotificationTemplate struct {
	ID               string
	Nome             string
	Tipo             TemplateType
	Categoria        TemplateCategoria
	Assunto          string            // subject line (email/push)
	Corpo            string            // body content — may contain {{varName}} placeholders
	VariaveisEsperadas []string        // list of expected variable names (documentation)
	Ativo            bool
	CriadoEm        time.Time
	AtualizadoEm    time.Time
}

// RenderRequest carries the variable map used to render a template.
type RenderRequest struct {
	Variaveis map[string]string // key = varName (without braces), value = replacement
}

// RenderResponse holds the rendered output after variable substitution.
type RenderResponse struct {
	TemplateID string
	Assunto    string
	Corpo      string
}

// Render substitutes all {{key}} placeholders in Assunto and Corpo with the provided values.
// Unknown variables are left as-is. Returns a RenderResponse with the substituted content.
func (t *NotificationTemplate) Render(req RenderRequest) RenderResponse {
	assunto := substituir(t.Assunto, req.Variaveis)
	corpo := substituir(t.Corpo, req.Variaveis)
	return RenderResponse{
		TemplateID: t.ID,
		Assunto:    assunto,
		Corpo:      corpo,
	}
}

// substituir replaces all {{key}} occurrences in s with the matching value from vars.
func substituir(s string, vars map[string]string) string {
	for k, v := range vars {
		s = strings.ReplaceAll(s, "{{"+k+"}}", v)
	}
	return s
}

// IsAtivo reports whether the template is active.
func (t *NotificationTemplate) IsAtivo() bool {
	return t.Ativo
}
