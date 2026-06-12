package notification

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ============================================================================
// Notification Stages
//
// A "stage" is a multi-level notification escalation configuration. Each stage
// is persisted as a set of NotificationStage rows whose key encodes the stage
// identity, the event type, the delivery channel and the escalation level using
// the deterministic format:
//
//	STAGE__<KEY>__<EVENT>__<CHANNEL>__L<N>
//
// Mirrors Java's NotificationStageKeyCodec + the notification.* domain models
// (NotificationStageConfig / LevelConfig / TemplateConfig / SuccessPolicy).
// ============================================================================

// NotificationChannel is the delivery channel of a stage template.
type NotificationChannel string

const (
	ChannelEmail    NotificationChannel = "EMAIL"
	ChannelSMS      NotificationChannel = "SMS"
	ChannelWhatsApp NotificationChannel = "WHATSAPP"
	ChannelPush     NotificationChannel = "PUSH"
)

// IsValidChannel reports whether c is one of the four supported channels.
func IsValidChannel(c NotificationChannel) bool {
	switch c {
	case ChannelEmail, ChannelSMS, ChannelWhatsApp, ChannelPush:
		return true
	default:
		return false
	}
}

// NotificationStageSuccessPolicy controls how a stage's channels are considered
// "delivered". AT_LEAST_ONE succeeds if any channel succeeds; ALL requires every
// channel of the level to succeed.
type NotificationStageSuccessPolicy string

const (
	SuccessPolicyAtLeastOne NotificationStageSuccessPolicy = "AT_LEAST_ONE"
	SuccessPolicyAll        NotificationStageSuccessPolicy = "ALL"
)

// ParseSuccessPolicy returns the policy for s, defaulting to AT_LEAST_ONE.
func ParseSuccessPolicy(s string) NotificationStageSuccessPolicy {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case string(SuccessPolicyAll):
		return SuccessPolicyAll
	default:
		return SuccessPolicyAtLeastOne
	}
}

// Metadata keys used to persist stage-level configuration on each template row.
// They live in NotificationStage.Metadados and are stripped from the public view.
const (
	StageLocaleMetadataKey        = "__stageLocale"
	StageSuccessPolicyMetadataKey = "__stageSuccessPolicy"
	defaultStageLocale            = "pt-BR"
)

// NotificationStage is a single persisted stage-template row. The Chave field
// carries the STAGE__... key that encodes its stage/eventType/channel/level.
type NotificationStage struct {
	ID           string
	Chave        string
	Canal        NotificationChannel
	Nome         string
	Assunto      string
	Corpo        string
	Variaveis    []string
	Metadados    map[string]any
	Ativo        bool
	CriadoEm     time.Time
	AtualizadoEm time.Time
}

// NotificationStageTemplateConfig is the public view of a single channel template
// within a level (metadata cleaned of internal __stage* keys).
type NotificationStageTemplateConfig struct {
	Chave        string              `json:"chave"`
	Canal        NotificationChannel `json:"canal"`
	Nome         string              `json:"nome"`
	Assunto      string              `json:"assunto,omitempty"`
	Corpo        string              `json:"corpo"`
	Variaveis    []string            `json:"variaveis"`
	Metadados    map[string]any      `json:"metadados,omitempty"`
	Ativo        bool                `json:"ativo"`
	CriadoEm     time.Time           `json:"criadoEm"`
	AtualizadoEm time.Time           `json:"atualizadoEm"`
}

// NotificationStageLevelConfig groups the templates of an escalation level.
type NotificationStageLevelConfig struct {
	Order     int                               `json:"order"`
	Templates []NotificationStageTemplateConfig `json:"templates"`
}

// NotificationStageConfig is the aggregated, API-facing view of a stage.
type NotificationStageConfig struct {
	Key           string                         `json:"key"`
	Locale        string                         `json:"locale"`
	EventType     string                         `json:"eventType,omitempty"`
	SuccessPolicy NotificationStageSuccessPolicy `json:"successPolicy"`
	Levels        []NotificationStageLevelConfig `json:"levels"`
}

// ParsedStageKey is the decoded form of a STAGE__... chave.
type ParsedStageKey struct {
	StageKey   string
	EventType  string
	Canal      NotificationChannel
	LevelOrder int
}

// MatchesStage reports whether the parsed key targets the given stage/event pair.
func (p ParsedStageKey) MatchesStage(expectedStageKey, expectedEventType string) bool {
	return p.StageKey == NotificationStageKeyCodec.Normalize(expectedStageKey) &&
		p.EventType == NotificationStageKeyCodec.NormalizeEventType(expectedEventType)
}

// stageKeyCodec implements the encode/decode/normalize helpers for stage keys.
type stageKeyCodec struct{}

// NotificationStageKeyCodec is the singleton codec for stage keys.
var NotificationStageKeyCodec = stageKeyCodec{}

// keyPattern matches STAGE__<KEY>__<EVENT>__<CHANNEL>__L<N>.
var keyPattern = regexp.MustCompile(`^STAGE__([A-Z0-9_]+)__([A-Z0-9_]+)__(EMAIL|SMS|WHATSAPP|PUSH)__L([0-9]+)$`)

var nonAlnum = regexp.MustCompile(`[^A-Z0-9]+`)
var multiUnderscore = regexp.MustCompile(`_+`)

// BuildKey assembles a STAGE__... key for the given parts. A nil/<1 level
// defaults to 1.
func (stageKeyCodec) BuildKey(stageKey, eventType string, canal NotificationChannel, levelOrder int) string {
	normalizedStage := NotificationStageKeyCodec.Normalize(stageKey)
	normalizedEventType := NotificationStageKeyCodec.NormalizeEventType(eventType)
	if levelOrder < 1 {
		levelOrder = 1
	}
	return "STAGE__" + normalizedStage + "__" + normalizedEventType + "__" + string(canal) + "__L" + strconv.Itoa(levelOrder)
}

// Parse decodes a chave, returning ok=false when it is not a stage key.
func (stageKeyCodec) Parse(chave string) (ParsedStageKey, bool) {
	if strings.TrimSpace(chave) == "" {
		return ParsedStageKey{}, false
	}
	m := keyPattern.FindStringSubmatch(strings.ToUpper(chave))
	if m == nil {
		return ParsedStageKey{}, false
	}
	level, _ := strconv.Atoi(m[4])
	return ParsedStageKey{
		StageKey:   m[1],
		EventType:  m[2],
		Canal:      NotificationChannel(m[3]),
		LevelOrder: level,
	}, true
}

// Normalize upper-cases and collapses non-alphanumerics to single underscores,
// trimming leading/trailing underscores. Empty input becomes "DEFAULT".
func (stageKeyCodec) Normalize(value string) string {
	if strings.TrimSpace(value) == "" {
		return "DEFAULT"
	}
	v := strings.ToUpper(strings.TrimSpace(value))
	v = nonAlnum.ReplaceAllString(v, "_")
	v = multiUnderscore.ReplaceAllString(v, "_")
	v = strings.Trim(v, "_")
	if v == "" {
		return "DEFAULT"
	}
	return v
}

// NormalizeEventType is Normalize with an explicit DEFAULT fallback.
func (stageKeyCodec) NormalizeEventType(eventType string) string {
	normalized := NotificationStageKeyCodec.Normalize(eventType)
	if normalized == "" {
		return "DEFAULT"
	}
	return normalized
}

// metadataString reads a string metadata value, falling back to def when absent.
func metadataString(metadata map[string]any, key, def string) string {
	if metadata == nil {
		return def
	}
	if v, ok := metadata[key]; ok && v != nil {
		if s, isStr := v.(string); isStr {
			return s
		}
	}
	return def
}

// cleanMetadata returns a copy of metadata with the internal __stage* keys removed.
func cleanMetadata(metadata map[string]any) map[string]any {
	if len(metadata) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(metadata))
	for k, v := range metadata {
		if k == StageLocaleMetadataKey || k == StageSuccessPolicyMetadataKey {
			continue
		}
		out[k] = v
	}
	return out
}

// BuildStageMetadata merges per-template metadata with the stage-level locale and
// success policy keys.
func BuildStageMetadata(templateMetadata map[string]any, locale string, policy NotificationStageSuccessPolicy) map[string]any {
	out := map[string]any{}
	for k, v := range templateMetadata {
		out[k] = v
	}
	if strings.TrimSpace(locale) == "" {
		locale = defaultStageLocale
	}
	if policy == "" {
		policy = SuccessPolicyAtLeastOne
	}
	out[StageLocaleMetadataKey] = locale
	out[StageSuccessPolicyMetadataKey] = string(policy)
	return out
}

// ToStageConfig aggregates a set of stage-template rows (all sharing the same
// stage/eventType) into the public NotificationStageConfig view. The templates
// slice must be non-empty.
func ToStageConfig(templates []NotificationStage) NotificationStageConfig {
	first, _ := NotificationStageKeyCodec.Parse(templates[0].Chave)

	byLevel := map[int][]NotificationStage{}
	for _, t := range templates {
		parsed, ok := NotificationStageKeyCodec.Parse(t.Chave)
		order := 1
		if ok {
			order = parsed.LevelOrder
		}
		byLevel[order] = append(byLevel[order], t)
	}

	orders := make([]int, 0, len(byLevel))
	for order := range byLevel {
		orders = append(orders, order)
	}
	sort.Ints(orders)

	levels := make([]NotificationStageLevelConfig, 0, len(orders))
	for _, order := range orders {
		rows := byLevel[order]
		sort.Slice(rows, func(i, j int) bool {
			return string(rows[i].Canal) < string(rows[j].Canal)
		})
		tmpls := make([]NotificationStageTemplateConfig, 0, len(rows))
		for _, r := range rows {
			tmpls = append(tmpls, toTemplateConfig(r))
		}
		levels = append(levels, NotificationStageLevelConfig{Order: order, Templates: tmpls})
	}

	firstMeta := templates[0].Metadados
	locale := metadataString(firstMeta, StageLocaleMetadataKey, defaultStageLocale)
	policy := ParseSuccessPolicy(metadataString(firstMeta, StageSuccessPolicyMetadataKey, string(SuccessPolicyAtLeastOne)))

	eventType := first.EventType
	if eventType == "DEFAULT" {
		eventType = ""
	}

	return NotificationStageConfig{
		Key:           first.StageKey,
		Locale:        locale,
		EventType:     eventType,
		SuccessPolicy: policy,
		Levels:        levels,
	}
}

func toTemplateConfig(t NotificationStage) NotificationStageTemplateConfig {
	variaveis := t.Variaveis
	if variaveis == nil {
		variaveis = []string{}
	}
	return NotificationStageTemplateConfig{
		Chave:        t.Chave,
		Canal:        t.Canal,
		Nome:         t.Nome,
		Assunto:      t.Assunto,
		Corpo:        t.Corpo,
		Variaveis:    variaveis,
		Metadados:    cleanMetadata(t.Metadados),
		Ativo:        t.Ativo,
		CriadoEm:     t.CriadoEm,
		AtualizadoEm: t.AtualizadoEm,
	}
}

// Render substitutes {{key}} placeholders in s using vars (string values).
func Render(s string, vars map[string]string) string {
	for k, v := range vars {
		s = strings.ReplaceAll(s, "{{"+k+"}}", v)
	}
	return s
}
