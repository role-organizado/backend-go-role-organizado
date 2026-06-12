package notificationtemplate_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	notification "github.com/role-organizado/backend-go-role-organizado/internal/domain/notification"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	uctemplate "github.com/role-organizado/backend-go-role-organizado/internal/usecase/notificationtemplate"
)

// ---- in-memory fake stage repository ----

type fakeStageRepo struct {
	rows   []notification.NotificationStage
	nextID int
	failOn string // "find" | "save" | "delete" to inject errors
}

func (f *fakeStageRepo) FindAll(ctx context.Context) ([]notification.NotificationStage, error) {
	if f.failOn == "find" {
		return nil, assertErr
	}
	out := make([]notification.NotificationStage, len(f.rows))
	copy(out, f.rows)
	return out, nil
}

func (f *fakeStageRepo) Save(ctx context.Context, s *notification.NotificationStage) (*notification.NotificationStage, error) {
	if f.failOn == "save" {
		return nil, assertErr
	}
	f.nextID++
	s.ID = "id-" + strconv.Itoa(f.nextID)
	f.rows = append(f.rows, *s)
	return s, nil
}

func (f *fakeStageRepo) DeleteByID(ctx context.Context, id string) error {
	if f.failOn == "delete" {
		return assertErr
	}
	kept := f.rows[:0]
	for _, r := range f.rows {
		if r.ID != id {
			kept = append(kept, r)
		}
	}
	f.rows = kept
	return nil
}

var assertErr = &stubErr{}

type stubErr struct{}

func (*stubErr) Error() string { return "boom" }

func seedRow(chave string, canal notification.NotificationChannel, ativo bool, policy notification.NotificationStageSuccessPolicy) notification.NotificationStage {
	return notification.NotificationStage{
		ID:    chave,
		Chave: chave,
		Canal: canal,
		Nome:  "n-" + string(canal),
		Corpo: "olá {{nome}}",
		Ativo: ativo,
		Metadados: map[string]any{
			notification.StageSuccessPolicyMetadataKey: string(policy),
			notification.StageLocaleMetadataKey:        "pt-BR",
		},
	}
}

func boolPtr(b bool) *bool { return &b }

// ---- Listar ----

func TestManageStages_Listar(t *testing.T) {
	tests := []struct {
		name      string
		rows      []notification.NotificationStage
		eventType string
		wantKeys  []string
	}{
		{
			name:     "empty repo yields no stages",
			rows:     nil,
			wantKeys: []string{},
		},
		{
			name: "groups rows by stage+event and sorts by key",
			rows: []notification.NotificationStage{
				seedRow("STAGE__RESET_SENHA__DEFAULT__EMAIL__L1", notification.ChannelEmail, true, notification.SuccessPolicyAtLeastOne),
				seedRow("STAGE__CONVITE__DEFAULT__SMS__L1", notification.ChannelSMS, true, notification.SuccessPolicyAtLeastOne),
				seedRow("STAGE__CONVITE__DEFAULT__EMAIL__L1", notification.ChannelEmail, true, notification.SuccessPolicyAtLeastOne),
			},
			wantKeys: []string{"CONVITE", "RESET_SENHA"},
		},
		{
			name: "ignores non-stage templates",
			rows: []notification.NotificationStage{
				seedRow("STAGE__CONVITE__DEFAULT__EMAIL__L1", notification.ChannelEmail, true, notification.SuccessPolicyAtLeastOne),
				{ID: "plain", Chave: "welcome-email", Canal: notification.ChannelEmail, Ativo: true},
			},
			wantKeys: []string{"CONVITE"},
		},
		{
			name: "filters by eventType",
			rows: []notification.NotificationStage{
				seedRow("STAGE__CONVITE__APROVADO__EMAIL__L1", notification.ChannelEmail, true, notification.SuccessPolicyAtLeastOne),
				seedRow("STAGE__CONVITE__DEFAULT__EMAIL__L1", notification.ChannelEmail, true, notification.SuccessPolicyAtLeastOne),
			},
			eventType: "aprovado",
			wantKeys:  []string{"CONVITE"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeStageRepo{rows: tc.rows}
			uc := uctemplate.NewManageNotificationStages(repo)

			stages, err := uc.Execute(context.Background(), tc.eventType)
			require.NoError(t, err)

			gotKeys := make([]string, 0, len(stages))
			for _, s := range stages {
				gotKeys = append(gotKeys, s.Key)
			}
			assert.Equal(t, tc.wantKeys, gotKeys)
		})
	}
}

func TestManageStages_Listar_RepoError(t *testing.T) {
	repo := &fakeStageRepo{failOn: "find"}
	uc := uctemplate.NewManageNotificationStages(repo)
	_, err := uc.Execute(context.Background(), "")
	require.Error(t, err)
}

// ---- Buscar (Get) ----

func TestManageStages_Get(t *testing.T) {
	rows := []notification.NotificationStage{
		seedRow("STAGE__CONVITE__DEFAULT__EMAIL__L1", notification.ChannelEmail, true, notification.SuccessPolicyAll),
		seedRow("STAGE__CONVITE__DEFAULT__SMS__L2", notification.ChannelSMS, true, notification.SuccessPolicyAll),
	}

	tests := []struct {
		name      string
		key       string
		eventType string
		wantErr   bool
		wantLevel int
	}{
		{name: "found with two levels", key: "convite", wantLevel: 2},
		{name: "normalizes key", key: "  Convite  ", wantLevel: 2},
		{name: "missing stage is 404", key: "inexistente", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeStageRepo{rows: rows}
			uc := uctemplate.NewManageNotificationStages(repo)

			cfg, err := uc.AsGetUseCase().Execute(context.Background(), tc.key, tc.eventType)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, cfg)
			assert.Equal(t, "CONVITE", cfg.Key)
			assert.Len(t, cfg.Levels, tc.wantLevel)
			assert.Equal(t, notification.SuccessPolicyAll, cfg.SuccessPolicy)
		})
	}
}

// ---- Upsert ----

func TestManageStages_Upsert(t *testing.T) {
	tests := []struct {
		name    string
		in      portin.UpsertNotificationStageInput
		wantErr bool
		wantRow int
	}{
		{
			name:    "empty levels is rejected",
			in:      portin.UpsertNotificationStageInput{},
			wantErr: true,
		},
		{
			name: "invalid channel is rejected",
			in: portin.UpsertNotificationStageInput{
				Levels: []portin.NotificationStageLevelInput{{
					Order: 1,
					Templates: []portin.NotificationStageTemplateInput{{
						Canal: "TELEPATHY", Nome: "x", Corpo: "y",
					}},
				}},
			},
			wantErr: true,
		},
		{
			name: "missing corpo is rejected",
			in: portin.UpsertNotificationStageInput{
				Levels: []portin.NotificationStageLevelInput{{
					Order: 1,
					Templates: []portin.NotificationStageTemplateInput{{
						Canal: notification.ChannelEmail, Nome: "x",
					}},
				}},
			},
			wantErr: true,
		},
		{
			name: "creates two rows across one level",
			in: portin.UpsertNotificationStageInput{
				EventType:     "aprovado",
				SuccessPolicy: notification.SuccessPolicyAll,
				Levels: []portin.NotificationStageLevelInput{{
					Order: 1,
					Templates: []portin.NotificationStageTemplateInput{
						{Canal: notification.ChannelEmail, Nome: "e", Corpo: "c1"},
						{Canal: notification.ChannelSMS, Nome: "s", Corpo: "c2"},
					},
				}},
			},
			wantRow: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeStageRepo{}
			uc := uctemplate.NewManageNotificationStages(repo)

			cfg, err := uc.AsUpsertUseCase().Execute(context.Background(), "convite", tc.in)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, cfg)
			assert.Len(t, repo.rows, tc.wantRow)
			assert.Equal(t, "CONVITE", cfg.Key)
			assert.Equal(t, "APROVADO", cfg.EventType)
		})
	}
}

func TestManageStages_Upsert_ReplacesExisting(t *testing.T) {
	repo := &fakeStageRepo{
		rows: []notification.NotificationStage{
			seedRow("STAGE__CONVITE__DEFAULT__EMAIL__L1", notification.ChannelEmail, true, notification.SuccessPolicyAtLeastOne),
		},
	}
	uc := uctemplate.NewManageNotificationStages(repo)

	_, err := uc.AsUpsertUseCase().Execute(context.Background(), "convite", portin.UpsertNotificationStageInput{
		Levels: []portin.NotificationStageLevelInput{{
			Order: 1,
			Templates: []portin.NotificationStageTemplateInput{
				{Canal: notification.ChannelPush, Nome: "p", Corpo: "novo"},
			},
		}},
	})
	require.NoError(t, err)

	// Old EMAIL row removed, single PUSH row remains.
	require.Len(t, repo.rows, 1)
	assert.Equal(t, notification.ChannelPush, repo.rows[0].Canal)
}

// ---- Excluir ----

func TestManageStages_Excluir(t *testing.T) {
	repo := &fakeStageRepo{
		rows: []notification.NotificationStage{
			seedRow("STAGE__CONVITE__DEFAULT__EMAIL__L1", notification.ChannelEmail, true, notification.SuccessPolicyAtLeastOne),
			seedRow("STAGE__RESET__DEFAULT__EMAIL__L1", notification.ChannelEmail, true, notification.SuccessPolicyAtLeastOne),
		},
	}
	uc := uctemplate.NewManageNotificationStages(repo)

	err := uc.AsDeleteUseCase().Execute(context.Background(), "convite", "")
	require.NoError(t, err)
	require.Len(t, repo.rows, 1)
	assert.Equal(t, "STAGE__RESET__DEFAULT__EMAIL__L1", repo.rows[0].Chave)
}

// ---- TestSendStages ----

func TestManageStages_TestSendStages(t *testing.T) {
	tests := []struct {
		name          string
		rows          []notification.NotificationStage
		in            portin.TestSendStagesInput
		wantErr       bool
		wantSuccess   bool
		wantSent      int
		wantStopLevel int // order at which Parou is expected, 0 = none
	}{
		{
			name:    "missing destinatario rejected",
			in:      portin.TestSendStagesInput{StageKey: "convite"},
			wantErr: true,
		},
		{
			name:    "unknown stage is 404",
			in:      portin.TestSendStagesInput{StageKey: "ghost", Destinatario: "a@b.com"},
			wantErr: true,
		},
		{
			name: "AT_LEAST_ONE stops at first level",
			rows: []notification.NotificationStage{
				seedRow("STAGE__CONVITE__DEFAULT__EMAIL__L1", notification.ChannelEmail, true, notification.SuccessPolicyAtLeastOne),
				seedRow("STAGE__CONVITE__DEFAULT__SMS__L2", notification.ChannelSMS, true, notification.SuccessPolicyAtLeastOne),
			},
			in:            portin.TestSendStagesInput{StageKey: "convite", Destinatario: "a@b.com", Variaveis: map[string]string{"nome": "Rod"}},
			wantSuccess:   true,
			wantSent:      1,
			wantStopLevel: 1,
		},
		{
			name: "ALL policy requires every channel in the level",
			rows: []notification.NotificationStage{
				seedRow("STAGE__CONVITE__DEFAULT__EMAIL__L1", notification.ChannelEmail, true, notification.SuccessPolicyAll),
				seedRow("STAGE__CONVITE__DEFAULT__SMS__L1", notification.ChannelSMS, true, notification.SuccessPolicyAll),
			},
			in:            portin.TestSendStagesInput{StageKey: "convite", Destinatario: "a@b.com"},
			wantSuccess:   true,
			wantSent:      2,
			wantStopLevel: 1,
		},
		{
			name: "falls back to DEFAULT eventType when specific is absent",
			rows: []notification.NotificationStage{
				seedRow("STAGE__CONVITE__DEFAULT__EMAIL__L1", notification.ChannelEmail, true, notification.SuccessPolicyAtLeastOne),
			},
			in:            portin.TestSendStagesInput{StageKey: "convite", EventType: "aprovado", Destinatario: "a@b.com"},
			wantSuccess:   true,
			wantSent:      1,
			wantStopLevel: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeStageRepo{rows: tc.rows}
			uc := uctemplate.NewManageNotificationStages(repo)

			res, err := uc.AsTestSendUseCase().Execute(context.Background(), tc.in)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, res)
			assert.Equal(t, tc.wantSuccess, res.Sucesso)
			assert.Equal(t, tc.wantSent, res.TotalEnviados)

			if tc.wantStopLevel > 0 {
				var stopped int
				for _, lvl := range res.Niveis {
					if lvl.Parou {
						stopped = lvl.Order
					}
				}
				assert.Equal(t, tc.wantStopLevel, stopped)
			}
		})
	}
}

func TestManageStages_TestSendStages_RendersVariables(t *testing.T) {
	repo := &fakeStageRepo{
		rows: []notification.NotificationStage{
			seedRow("STAGE__CONVITE__DEFAULT__EMAIL__L1", notification.ChannelEmail, true, notification.SuccessPolicyAtLeastOne),
		},
	}
	uc := uctemplate.NewManageNotificationStages(repo)

	res, err := uc.AsTestSendUseCase().Execute(context.Background(), portin.TestSendStagesInput{
		StageKey:     "convite",
		Destinatario: "a@b.com",
		Variaveis:    map[string]string{"nome": "Rodrigo"},
	})
	require.NoError(t, err)
	require.Len(t, res.Niveis, 1)
	require.Len(t, res.Niveis[0].Canais, 1)
	assert.Equal(t, "olá Rodrigo", res.Niveis[0].Canais[0].Corpo)
}

// ensure the Upsert validation also rejects a level with a disabled-only template count
func TestManageStages_Upsert_RespectsAtivoFlag(t *testing.T) {
	repo := &fakeStageRepo{}
	uc := uctemplate.NewManageNotificationStages(repo)

	cfg, err := uc.AsUpsertUseCase().Execute(context.Background(), "convite", portin.UpsertNotificationStageInput{
		Levels: []portin.NotificationStageLevelInput{{
			Order: 1,
			Templates: []portin.NotificationStageTemplateInput{
				{Canal: notification.ChannelEmail, Nome: "e", Corpo: "c", Ativo: boolPtr(false)},
			},
		}},
	})
	require.NoError(t, err)
	require.Len(t, cfg.Levels, 1)
	assert.False(t, cfg.Levels[0].Templates[0].Ativo)
}
