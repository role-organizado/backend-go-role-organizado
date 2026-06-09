package notificationtemplate_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/notificationtemplate"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	uctemplate "github.com/role-organizado/backend-go-role-organizado/internal/usecase/notificationtemplate"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ---- mock repository ----

type mockRepo struct{ mock.Mock }

func (m *mockRepo) Save(ctx context.Context, t *domain.NotificationTemplate) (*domain.NotificationTemplate, error) {
	args := m.Called(ctx, t)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.NotificationTemplate), args.Error(1)
}

func (m *mockRepo) FindByID(ctx context.Context, id string) (*domain.NotificationTemplate, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.NotificationTemplate), args.Error(1)
}

func (m *mockRepo) FindAll(ctx context.Context, page, pageSize int) ([]domain.NotificationTemplate, int64, error) {
	args := m.Called(ctx, page, pageSize)
	return args.Get(0).([]domain.NotificationTemplate), args.Get(1).(int64), args.Error(2)
}

func (m *mockRepo) Update(ctx context.Context, t *domain.NotificationTemplate) (*domain.NotificationTemplate, error) {
	args := m.Called(ctx, t)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.NotificationTemplate), args.Error(1)
}

func (m *mockRepo) DeleteByID(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockRepo) FindByType(ctx context.Context, tipo domain.TemplateType) (*domain.NotificationTemplate, error) {
	args := m.Called(ctx, tipo)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.NotificationTemplate), args.Error(1)
}

func (m *mockRepo) FindByCategoria(ctx context.Context, categoria domain.TemplateCategoria, page, pageSize int) ([]domain.NotificationTemplate, int64, error) {
	args := m.Called(ctx, categoria, page, pageSize)
	return args.Get(0).([]domain.NotificationTemplate), args.Get(1).(int64), args.Error(2)
}

// ---- helpers ----

func activeTemplate(id string) *domain.NotificationTemplate {
	return &domain.NotificationTemplate{
		ID:               id,
		Nome:             "Convite para {{evento}}",
		Tipo:             domain.TemplateTypeEmail,
		Categoria:        domain.TemplateCategoriaEvento,
		Assunto:          "Você foi convidado para {{evento}}",
		Corpo:            "Olá {{nome}}, você foi convidado para {{evento}}!",
		VariaveisEsperadas: []string{"nome", "evento"},
		Ativo:            true,
		CriadoEm:        time.Now(),
		AtualizadoEm:    time.Now(),
	}
}

func inactiveTemplate(id string) *domain.NotificationTemplate {
	t := activeTemplate(id)
	t.Ativo = false
	return t
}

// ---- CreateNotificationTemplate ----

func TestCreate_Success(t *testing.T) {
	repo := new(mockRepo)
	uc := uctemplate.NewCreateNotificationTemplate(repo)

	expected := activeTemplate("t1")
	repo.On("Save", mock.Anything, mock.AnythingOfType("*notificationtemplate.NotificationTemplate")).Return(expected, nil)

	got, err := uc.Execute(context.Background(), portin.CreateNotificationTemplateInput{
		Nome:      "Convite para {{evento}}",
		Tipo:      domain.TemplateTypeEmail,
		Categoria: domain.TemplateCategoriaEvento,
		Assunto:   "Você foi convidado para {{evento}}",
		Corpo:     "Olá {{nome}}, você foi convidado para {{evento}}!",
		Ativo:     true,
	})
	require.NoError(t, err)
	assert.Equal(t, "t1", got.ID)
	assert.Equal(t, domain.TemplateTypeEmail, got.Tipo)
}

func TestCreate_MissingNome_Error(t *testing.T) {
	repo := new(mockRepo)
	uc := uctemplate.NewCreateNotificationTemplate(repo)

	_, err := uc.Execute(context.Background(), portin.CreateNotificationTemplateInput{
		Tipo:  domain.TemplateTypeEmail,
		Corpo: "corpo",
	})
	require.Error(t, err)
	ae := err.(*apierr.APIError)
	assert.Equal(t, 400, ae.Status)
}

func TestCreate_MissingCorpo_Error(t *testing.T) {
	repo := new(mockRepo)
	uc := uctemplate.NewCreateNotificationTemplate(repo)

	_, err := uc.Execute(context.Background(), portin.CreateNotificationTemplateInput{
		Nome: "Template",
		Tipo: domain.TemplateTypeEmail,
	})
	require.Error(t, err)
	ae := err.(*apierr.APIError)
	assert.Equal(t, 400, ae.Status)
}

func TestCreate_MissingTipo_Error(t *testing.T) {
	repo := new(mockRepo)
	uc := uctemplate.NewCreateNotificationTemplate(repo)

	_, err := uc.Execute(context.Background(), portin.CreateNotificationTemplateInput{
		Nome:  "Template",
		Corpo: "corpo",
	})
	require.Error(t, err)
	ae := err.(*apierr.APIError)
	assert.Equal(t, 400, ae.Status)
}

// ---- GetNotificationTemplate ----

func TestGet_Success(t *testing.T) {
	repo := new(mockRepo)
	uc := uctemplate.NewGetNotificationTemplate(repo)

	expected := activeTemplate("t1")
	repo.On("FindByID", mock.Anything, "t1").Return(expected, nil)

	got, err := uc.Execute(context.Background(), "t1")
	require.NoError(t, err)
	assert.Equal(t, "t1", got.ID)
}

func TestGet_NotFound_Error(t *testing.T) {
	repo := new(mockRepo)
	uc := uctemplate.NewGetNotificationTemplate(repo)

	repo.On("FindByID", mock.Anything, "unknown").Return(nil, apierr.NotFound("template", "unknown"))

	_, err := uc.Execute(context.Background(), "unknown")
	require.Error(t, err)
	assert.True(t, apierr.IsNotFound(err))
}

// ---- ListNotificationTemplates ----

func TestList_Success(t *testing.T) {
	repo := new(mockRepo)
	uc := uctemplate.NewListNotificationTemplates(repo)

	items := []domain.NotificationTemplate{*activeTemplate("t1"), *activeTemplate("t2")}
	repo.On("FindAll", mock.Anything, 1, 20).Return(items, int64(2), nil)

	got, total, err := uc.Execute(context.Background(), 1, 20)
	require.NoError(t, err)
	assert.Len(t, got, 2)
	assert.Equal(t, int64(2), total)
}

func TestList_DefaultPagination(t *testing.T) {
	repo := new(mockRepo)
	uc := uctemplate.NewListNotificationTemplates(repo)

	repo.On("FindAll", mock.Anything, 1, 20).Return([]domain.NotificationTemplate{}, int64(0), nil)

	// page=0 and pageSize=0 should be normalised to 1 and 20
	_, _, err := uc.Execute(context.Background(), 0, 0)
	require.NoError(t, err)
	repo.AssertCalled(t, "FindAll", mock.Anything, 1, 20)
}

// ---- UpdateNotificationTemplate ----

func TestUpdate_Success(t *testing.T) {
	repo := new(mockRepo)
	uc := uctemplate.NewUpdateNotificationTemplate(repo)

	existing := activeTemplate("t1")
	updated := *existing
	updated.Nome = "Novo Nome"
	repo.On("FindByID", mock.Anything, "t1").Return(existing, nil)
	repo.On("Update", mock.Anything, mock.AnythingOfType("*notificationtemplate.NotificationTemplate")).Return(&updated, nil)

	got, err := uc.Execute(context.Background(), portin.UpdateNotificationTemplateInput{
		ID:   "t1",
		Nome: "Novo Nome",
	})
	require.NoError(t, err)
	assert.Equal(t, "Novo Nome", got.Nome)
}

func TestUpdate_NotFound_Error(t *testing.T) {
	repo := new(mockRepo)
	uc := uctemplate.NewUpdateNotificationTemplate(repo)

	repo.On("FindByID", mock.Anything, "unknown").Return(nil, apierr.NotFound("template", "unknown"))

	_, err := uc.Execute(context.Background(), portin.UpdateNotificationTemplateInput{ID: "unknown"})
	require.Error(t, err)
	assert.True(t, apierr.IsNotFound(err))
}

// ---- DeleteNotificationTemplate ----

func TestDelete_Success(t *testing.T) {
	repo := new(mockRepo)
	uc := uctemplate.NewDeleteNotificationTemplate(repo)

	existing := activeTemplate("t1")
	repo.On("FindByID", mock.Anything, "t1").Return(existing, nil)
	repo.On("DeleteByID", mock.Anything, "t1").Return(nil)

	err := uc.Execute(context.Background(), "t1")
	require.NoError(t, err)
}

func TestDelete_NotFound_Error(t *testing.T) {
	repo := new(mockRepo)
	uc := uctemplate.NewDeleteNotificationTemplate(repo)

	repo.On("FindByID", mock.Anything, "unknown").Return(nil, apierr.NotFound("template", "unknown"))

	err := uc.Execute(context.Background(), "unknown")
	require.Error(t, err)
	assert.True(t, apierr.IsNotFound(err))
	repo.AssertNotCalled(t, "DeleteByID")
}

// ---- RenderNotificationTemplate ----

func TestRender_Success(t *testing.T) {
	repo := new(mockRepo)
	uc := uctemplate.NewRenderNotificationTemplate(repo)

	tmpl := activeTemplate("t1")
	repo.On("FindByID", mock.Anything, "t1").Return(tmpl, nil)

	resp, err := uc.Execute(context.Background(), "t1", domain.RenderRequest{
		Variaveis: map[string]string{
			"nome":   "João",
			"evento": "Churrasco do João",
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "t1", resp.TemplateID)
	assert.Contains(t, resp.Corpo, "João")
	assert.Contains(t, resp.Corpo, "Churrasco do João")
	assert.NotContains(t, resp.Corpo, "{{nome}}")
	assert.NotContains(t, resp.Corpo, "{{evento}}")
}

func TestRender_PartialVariables(t *testing.T) {
	repo := new(mockRepo)
	uc := uctemplate.NewRenderNotificationTemplate(repo)

	tmpl := activeTemplate("t1")
	repo.On("FindByID", mock.Anything, "t1").Return(tmpl, nil)

	// Only provide one variable — {{evento}} should remain in output
	resp, err := uc.Execute(context.Background(), "t1", domain.RenderRequest{
		Variaveis: map[string]string{
			"nome": "Maria",
		},
	})
	require.NoError(t, err)
	assert.Contains(t, resp.Corpo, "Maria")
	assert.Contains(t, resp.Corpo, "{{evento}}") // unreplaced
}

func TestRender_NotFound_Error(t *testing.T) {
	repo := new(mockRepo)
	uc := uctemplate.NewRenderNotificationTemplate(repo)

	repo.On("FindByID", mock.Anything, "unknown").Return(nil, apierr.NotFound("template", "unknown"))

	_, err := uc.Execute(context.Background(), "unknown", domain.RenderRequest{})
	require.Error(t, err)
	assert.True(t, apierr.IsNotFound(err))
}

// ---- TestSendNotificationTemplate ----

func TestTestSend_Success(t *testing.T) {
	repo := new(mockRepo)
	uc := uctemplate.NewTestSendNotificationTemplate(repo)

	tmpl := activeTemplate("t1")
	repo.On("FindByID", mock.Anything, "t1").Return(tmpl, nil)

	resp, err := uc.Execute(context.Background(), portin.TestSendInput{
		TemplateID:   "t1",
		Destinatario: "teste@example.com",
		Variaveis: map[string]string{
			"nome":   "João",
			"evento": "Churrasco",
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "t1", resp.TemplateID)
	assert.Contains(t, resp.Corpo, "João")
}

func TestTestSend_InactiveTemplate_Error(t *testing.T) {
	repo := new(mockRepo)
	uc := uctemplate.NewTestSendNotificationTemplate(repo)

	tmpl := inactiveTemplate("t1")
	repo.On("FindByID", mock.Anything, "t1").Return(tmpl, nil)

	_, err := uc.Execute(context.Background(), portin.TestSendInput{
		TemplateID:   "t1",
		Destinatario: "teste@example.com",
	})
	require.Error(t, err)
	ae := err.(*apierr.APIError)
	assert.Equal(t, 422, ae.Status)
}

func TestTestSend_MissingDestinatario_Error(t *testing.T) {
	repo := new(mockRepo)
	uc := uctemplate.NewTestSendNotificationTemplate(repo)

	tmpl := activeTemplate("t1")
	repo.On("FindByID", mock.Anything, "t1").Return(tmpl, nil)

	_, err := uc.Execute(context.Background(), portin.TestSendInput{
		TemplateID: "t1",
	})
	require.Error(t, err)
	ae := err.(*apierr.APIError)
	assert.Equal(t, 400, ae.Status)
}

// ---- GetByTypeNotificationTemplate ----

func TestGetByType_Success(t *testing.T) {
	repo := new(mockRepo)
	uc := uctemplate.NewGetByTypeNotificationTemplate(repo)

	expected := activeTemplate("t1")
	repo.On("FindByType", mock.Anything, domain.TemplateTypeEmail).Return(expected, nil)

	got, err := uc.Execute(context.Background(), domain.TemplateTypeEmail)
	require.NoError(t, err)
	assert.Equal(t, "t1", got.ID)
	assert.Equal(t, domain.TemplateTypeEmail, got.Tipo)
}

func TestGetByType_NotFound_Error(t *testing.T) {
	repo := new(mockRepo)
	uc := uctemplate.NewGetByTypeNotificationTemplate(repo)

	repo.On("FindByType", mock.Anything, domain.TemplateTypeSMS).Return(nil, apierr.NotFoundMsg("nenhum template encontrado para o tipo SMS"))

	_, err := uc.Execute(context.Background(), domain.TemplateTypeSMS)
	require.Error(t, err)
	assert.True(t, apierr.IsNotFound(err))
}

// ---- ListByCategoriaNotificationTemplate ----

func TestListByCategoria_Success(t *testing.T) {
	repo := new(mockRepo)
	uc := uctemplate.NewListByCategoriaNotificationTemplate(repo)

	items := []domain.NotificationTemplate{*activeTemplate("t1"), *activeTemplate("t2")}
	repo.On("FindByCategoria", mock.Anything, domain.TemplateCategoriaEvento, 1, 20).Return(items, int64(2), nil)

	got, total, err := uc.Execute(context.Background(), domain.TemplateCategoriaEvento, 1, 20)
	require.NoError(t, err)
	assert.Len(t, got, 2)
	assert.Equal(t, int64(2), total)
}

func TestListByCategoria_DefaultPagination(t *testing.T) {
	repo := new(mockRepo)
	uc := uctemplate.NewListByCategoriaNotificationTemplate(repo)

	repo.On("FindByCategoria", mock.Anything, domain.TemplateCategoriaRateio, 1, 20).Return([]domain.NotificationTemplate{}, int64(0), nil)

	_, _, err := uc.Execute(context.Background(), domain.TemplateCategoriaRateio, 0, 0)
	require.NoError(t, err)
	repo.AssertCalled(t, "FindByCategoria", mock.Anything, domain.TemplateCategoriaRateio, 1, 20)
}
