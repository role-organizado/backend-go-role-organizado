package notificationtemplate

import (
	"context"
	"sort"
	"time"

	notification "github.com/role-organizado/backend-go-role-organizado/internal/domain/notification"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ManageNotificationStages implements the stage management use cases
// (Listar/Buscar/Upsert/Excluir) plus the TestSendStages orchestrator.
//
// It is the Go port of Java's GerenciarNotificationStagesUseCase and operates
// over the NotificationStageRepository (a full-scan-friendly view of the
// notification_templates collection keyed by the STAGE__... convention).
type ManageNotificationStages struct {
	repo portout.NotificationStageRepository
	now  func() time.Time
}

// NewManageNotificationStages wires the stage management use case.
func NewManageNotificationStages(repo portout.NotificationStageRepository) *ManageNotificationStages {
	return &ManageNotificationStages{repo: repo, now: time.Now}
}

// ---- Listar (ListNotificationStagesUseCase) ----

// Execute lists all stages, grouped by (stageKey, eventType), optionally
// filtered by eventType. Results are sorted by key.
func (uc *ManageNotificationStages) Execute(ctx context.Context, eventType string) ([]notification.NotificationStageConfig, error) {
	all, err := uc.repo.FindAll(ctx)
	if err != nil {
		return nil, err
	}

	grouped := map[string][]notification.NotificationStage{}
	for _, t := range all {
		parsed, ok := notification.NotificationStageKeyCodec.Parse(t.Chave)
		if !ok {
			continue
		}
		if !matchesEventType(t.Chave, eventType) {
			continue
		}
		k := parsed.StageKey + "::" + parsed.EventType
		grouped[k] = append(grouped[k], t)
	}

	stages := make([]notification.NotificationStageConfig, 0, len(grouped))
	for _, rows := range grouped {
		stages = append(stages, notification.ToStageConfig(rows))
	}
	sort.Slice(stages, func(i, j int) bool { return stages[i].Key < stages[j].Key })
	return stages, nil
}

var _ portin.ListNotificationStagesUseCase = (*ManageNotificationStages)(nil)

// ---- Buscar (GetNotificationStageUseCase) ----

// Buscar finds a single stage by key + eventType, returning the rows that match.
func (uc *ManageNotificationStages) buscar(ctx context.Context, stageKey, eventType string) (*notification.NotificationStageConfig, error) {
	normalizedStage := notification.NotificationStageKeyCodec.Normalize(stageKey)
	normalizedEvent := notification.NotificationStageKeyCodec.NormalizeEventType(eventType)

	all, err := uc.repo.FindAll(ctx)
	if err != nil {
		return nil, err
	}

	var rows []notification.NotificationStage
	for _, t := range all {
		parsed, ok := notification.NotificationStageKeyCodec.Parse(t.Chave)
		if ok && parsed.MatchesStage(normalizedStage, normalizedEvent) {
			rows = append(rows, t)
		}
	}
	if len(rows) == 0 {
		return nil, nil
	}
	cfg := notification.ToStageConfig(rows)
	return &cfg, nil
}

// GetExecute satisfies GetNotificationStageUseCase. A missing stage yields 404.
func (uc *ManageNotificationStages) GetExecute(ctx context.Context, stageKey, eventType string) (*notification.NotificationStageConfig, error) {
	cfg, err := uc.buscar(ctx, stageKey, eventType)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, apierr.NotFoundMsg("etapa de notificação não encontrada: " + stageKey)
	}
	return cfg, nil
}

// getStageUseCase adapts ManageNotificationStages to GetNotificationStageUseCase.
type getStageUseCase struct{ uc *ManageNotificationStages }

func (g getStageUseCase) Execute(ctx context.Context, stageKey, eventType string) (*notification.NotificationStageConfig, error) {
	return g.uc.GetExecute(ctx, stageKey, eventType)
}

// AsGetUseCase returns a GetNotificationStageUseCase view.
func (uc *ManageNotificationStages) AsGetUseCase() portin.GetNotificationStageUseCase {
	return getStageUseCase{uc: uc}
}

// ---- Upsert (UpsertNotificationStageUseCase) ----

// upsertStageUseCase adapts ManageNotificationStages to UpsertNotificationStageUseCase.
type upsertStageUseCase struct{ uc *ManageNotificationStages }

func (u upsertStageUseCase) Execute(ctx context.Context, stageKey string, in portin.UpsertNotificationStageInput) (*notification.NotificationStageConfig, error) {
	return u.uc.Upsert(ctx, stageKey, in)
}

// AsUpsertUseCase returns an UpsertNotificationStageUseCase view.
func (uc *ManageNotificationStages) AsUpsertUseCase() portin.UpsertNotificationStageUseCase {
	return upsertStageUseCase{uc: uc}
}

// Upsert replaces all template rows for the (stageKey, eventType) pair.
func (uc *ManageNotificationStages) Upsert(ctx context.Context, stageKey string, in portin.UpsertNotificationStageInput) (*notification.NotificationStageConfig, error) {
	if len(in.Levels) == 0 {
		return nil, apierr.BadRequest("etapa deve conter ao menos um nível")
	}

	normalizedStage := notification.NotificationStageKeyCodec.Normalize(stageKey)
	normalizedEvent := notification.NotificationStageKeyCodec.NormalizeEventType(in.EventType)

	if err := uc.removerStageExistente(ctx, normalizedStage, normalizedEvent); err != nil {
		return nil, err
	}

	policy := in.SuccessPolicy
	if policy == "" {
		policy = notification.SuccessPolicyAtLeastOne
	}
	locale := in.Locale
	if locale == "" {
		locale = "pt-BR"
	}

	now := uc.now()
	for _, level := range in.Levels {
		if len(level.Templates) == 0 {
			return nil, apierr.BadRequest("nível deve conter ao menos um template")
		}
		for _, tr := range level.Templates {
			if !notification.IsValidChannel(tr.Canal) {
				return nil, apierr.BadRequest("canal inválido: " + string(tr.Canal))
			}
			if tr.Nome == "" {
				return nil, apierr.BadRequest("nome do template é obrigatório")
			}
			if tr.Corpo == "" {
				return nil, apierr.BadRequest("corpo do template é obrigatório")
			}

			ativo := true
			if tr.Ativo != nil {
				ativo = *tr.Ativo
			}
			variaveis := tr.Variaveis
			if variaveis == nil {
				variaveis = []string{}
			}

			row := &notification.NotificationStage{
				Chave:        notification.NotificationStageKeyCodec.BuildKey(normalizedStage, normalizedEvent, tr.Canal, level.Order),
				Canal:        tr.Canal,
				Nome:         tr.Nome,
				Assunto:      tr.Assunto,
				Corpo:        tr.Corpo,
				Variaveis:    variaveis,
				Metadados:    notification.BuildStageMetadata(tr.Metadados, locale, policy),
				Ativo:        ativo,
				CriadoEm:     now,
				AtualizadoEm: now,
			}
			if _, err := uc.repo.Save(ctx, row); err != nil {
				return nil, err
			}
		}
	}

	cfg, err := uc.buscar(ctx, normalizedStage, normalizedEvent)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, apierr.Internal("falha ao carregar etapa salva")
	}
	return cfg, nil
}

// ---- Excluir (DeleteNotificationStageUseCase) ----

// deleteStageUseCase adapts ManageNotificationStages to DeleteNotificationStageUseCase.
type deleteStageUseCase struct{ uc *ManageNotificationStages }

func (d deleteStageUseCase) Execute(ctx context.Context, stageKey, eventType string) error {
	return d.uc.Excluir(ctx, stageKey, eventType)
}

// AsDeleteUseCase returns a DeleteNotificationStageUseCase view.
func (uc *ManageNotificationStages) AsDeleteUseCase() portin.DeleteNotificationStageUseCase {
	return deleteStageUseCase{uc: uc}
}

// Excluir removes all template rows of a stage.
func (uc *ManageNotificationStages) Excluir(ctx context.Context, stageKey, eventType string) error {
	normalizedStage := notification.NotificationStageKeyCodec.Normalize(stageKey)
	normalizedEvent := notification.NotificationStageKeyCodec.NormalizeEventType(eventType)
	return uc.removerStageExistente(ctx, normalizedStage, normalizedEvent)
}

func (uc *ManageNotificationStages) removerStageExistente(ctx context.Context, stageKey, eventType string) error {
	all, err := uc.repo.FindAll(ctx)
	if err != nil {
		return err
	}
	for _, t := range all {
		parsed, ok := notification.NotificationStageKeyCodec.Parse(t.Chave)
		if !ok || !parsed.MatchesStage(stageKey, eventType) {
			continue
		}
		if t.ID == "" {
			continue
		}
		if err := uc.repo.DeleteByID(ctx, t.ID); err != nil {
			return err
		}
	}
	return nil
}

// ---- TestSendStages (TestSendStagesUseCase) ----

// testSendStagesUseCase adapts ManageNotificationStages to TestSendStagesUseCase.
type testSendStagesUseCase struct{ uc *ManageNotificationStages }

func (t testSendStagesUseCase) Execute(ctx context.Context, in portin.TestSendStagesInput) (*portin.TestSendStagesResult, error) {
	return t.uc.TestSendStages(ctx, in)
}

// AsTestSendUseCase returns a TestSendStagesUseCase view.
func (uc *ManageNotificationStages) AsTestSendUseCase() portin.TestSendStagesUseCase {
	return testSendStagesUseCase{uc: uc}
}

// TestSendStages resolves the stage (with DEFAULT eventType fallback), then walks
// the escalation levels in order, rendering each active channel template and
// simulating a send. Escalation stops at the first level that "succeeds"
// according to the stage's success policy:
//   - AT_LEAST_ONE: the level succeeds once any channel is rendered+sent.
//   - ALL: the level succeeds only when every channel succeeds.
func (uc *ManageNotificationStages) TestSendStages(ctx context.Context, in portin.TestSendStagesInput) (*portin.TestSendStagesResult, error) {
	if in.Destinatario == "" {
		return nil, apierr.BadRequest("destinatário é obrigatório")
	}

	cfg, resolvedEvent, err := uc.resolveActiveStage(ctx, in.StageKey, in.EventType)
	if err != nil {
		return nil, err
	}

	result := &portin.TestSendStagesResult{
		Key:           cfg.Key,
		ResolvedEvent: resolvedEvent,
		SuccessPolicy: cfg.SuccessPolicy,
		Destinatario:  in.Destinatario,
		Niveis:        []portin.StageLevelResult{},
	}

	for _, level := range cfg.Levels {
		levelResult := portin.StageLevelResult{Order: level.Order, Canais: []portin.StageChannelResult{}}
		sentInLevel := 0
		for _, tmpl := range level.Templates {
			if !tmpl.Ativo {
				continue
			}
			ch := portin.StageChannelResult{
				Canal:   tmpl.Canal,
				Assunto: notification.Render(tmpl.Assunto, in.Variaveis),
				Corpo:   notification.Render(tmpl.Corpo, in.Variaveis),
				Enviado: true,
			}
			levelResult.Canais = append(levelResult.Canais, ch)
			sentInLevel++
			result.TotalEnviados++
		}

		activeCount := countActive(level.Templates)
		levelResult.Sucesso = levelSucceeded(cfg.SuccessPolicy, sentInLevel, activeCount)
		result.Niveis = append(result.Niveis, levelResult)

		if levelResult.Sucesso {
			// Mark the last appended level as the stopping point.
			result.Niveis[len(result.Niveis)-1].Parou = true
			result.Sucesso = true
			break
		}
	}

	return result, nil
}

// resolveActiveStage loads the stage filtered to active templates, applying the
// DEFAULT eventType fallback when the requested eventType yields nothing.
func (uc *ManageNotificationStages) resolveActiveStage(ctx context.Context, stageKey, eventType string) (*notification.NotificationStageConfig, string, error) {
	normalizedStage := notification.NotificationStageKeyCodec.Normalize(stageKey)
	normalizedEvent := notification.NotificationStageKeyCodec.NormalizeEventType(eventType)

	all, err := uc.repo.FindAll(ctx)
	if err != nil {
		return nil, "", err
	}

	rows := filterActiveStage(all, normalizedStage, normalizedEvent)
	resolvedEvent := normalizedEvent
	if len(rows) == 0 && normalizedEvent != "DEFAULT" {
		rows = filterActiveStage(all, normalizedStage, "DEFAULT")
		resolvedEvent = "DEFAULT"
	}
	if len(rows) == 0 {
		return nil, "", apierr.NotFoundMsg("etapa não encontrada: " + stageKey + " (eventType=" + eventType + ")")
	}
	cfg := notification.ToStageConfig(rows)
	return &cfg, resolvedEvent, nil
}

func filterActiveStage(all []notification.NotificationStage, stageKey, eventType string) []notification.NotificationStage {
	var rows []notification.NotificationStage
	for _, t := range all {
		if !t.Ativo {
			continue
		}
		parsed, ok := notification.NotificationStageKeyCodec.Parse(t.Chave)
		if ok && parsed.StageKey == stageKey && parsed.EventType == eventType {
			rows = append(rows, t)
		}
	}
	return rows
}

func countActive(templates []notification.NotificationStageTemplateConfig) int {
	n := 0
	for _, t := range templates {
		if t.Ativo {
			n++
		}
	}
	return n
}

func levelSucceeded(policy notification.NotificationStageSuccessPolicy, sent, active int) bool {
	if active == 0 {
		return false
	}
	if policy == notification.SuccessPolicyAll {
		return sent == active
	}
	return sent >= 1
}

func matchesEventType(chave, eventType string) bool {
	if eventType == "" {
		return true
	}
	parsed, ok := notification.NotificationStageKeyCodec.Parse(chave)
	if !ok {
		return false
	}
	return parsed.EventType == notification.NotificationStageKeyCodec.NormalizeEventType(eventType)
}
