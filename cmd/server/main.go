// Package server bootstraps and starts the HTTP server.
// Phase 0: minimal chi router with health endpoint, middleware chain, and graceful shutdown.
// Phase 1: Config domain (Dominios + ConfigSistema).
// Phase 2: Auth + Usuarios domain.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	sdkclient "go.temporal.io/sdk/client"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/handler"
	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/middleware"
	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/mongodb"
	temporaladapter "github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal"
	sqsadapter "github.com/role-organizado/backend-go-role-organizado/internal/adapter/sqs"
	temporalactivity "github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/activity"
	temporalworker "github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/worker"
	"github.com/role-organizado/backend-go-role-organizado/internal/config"
	"github.com/role-organizado/backend-go-role-organizado/migrations"
	pkgjwt "github.com/role-organizado/backend-go-role-organizado/pkg/jwt"
	pkgotel "github.com/role-organizado/backend-go-role-organizado/pkg/otel"

	// Phase 1
	ucconfig "github.com/role-organizado/backend-go-role-organizado/internal/usecase/config"
	// Phase 2
	ucauth "github.com/role-organizado/backend-go-role-organizado/internal/usecase/auth"
	// Phase 3
	ucevent "github.com/role-organizado/backend-go-role-organizado/internal/usecase/event"
	// Phase 4
	ucrateio "github.com/role-organizado/backend-go-role-organizado/internal/usecase/rateio"
	// Onda 3/4 — participant lifecycle use cases
	ucparticipant "github.com/role-organizado/backend-go-role-organizado/internal/usecase/participant"
	// Phase 5
	ucpayment "github.com/role-organizado/backend-go-role-organizado/internal/usecase/payment"
	ucasaas "github.com/role-organizado/backend-go-role-organizado/internal/infra/asaas"
	paymentdomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	// Phase 5b: Finance hexagonal
	ucfinance "github.com/role-organizado/backend-go-role-organizado/internal/usecase/finance"
	ucaccounting "github.com/role-organizado/backend-go-role-organizado/internal/usecase/accounting"
	// Phase 6
	ucnotification "github.com/role-organizado/backend-go-role-organizado/internal/usecase/notification"
	// Phase 6b: Notification Templates
	ucnotiftemplate "github.com/role-organizado/backend-go-role-organizado/internal/usecase/notificationtemplate"
	// Phase 7
	ucstorage "github.com/role-organizado/backend-go-role-organizado/internal/usecase/storage"
	// Cofrinho
	uccofrinho "github.com/role-organizado/backend-go-role-organizado/internal/usecase/cofrinho"
	// Lista Presentes
	uclistapresentes "github.com/role-organizado/backend-go-role-organizado/internal/usecase/listapresentes"
	// Social Features
	ucsocial "github.com/role-organizado/backend-go-role-organizado/internal/usecase/social"
	// Pricing — PSP cost review
	ucpricing "github.com/role-organizado/backend-go-role-organizado/internal/usecase/pricing"
	// Guest + Biometric Auth (Java parity: GuestController + BiometricAuthController)
	ucguest "github.com/role-organizado/backend-go-role-organizado/internal/usecase/guest"
	// Convites domain
	ucconvite "github.com/role-organizado/backend-go-role-organizado/internal/usecase/convite"
	// Outbound Transfers (withdrawals + voting approval)
	ucoutbound "github.com/role-organizado/backend-go-role-organizado/internal/usecase/outbound"
	// Admin surface (Trilha C: 9 Java admin controllers → hexagonal)
	ucadmin "github.com/role-organizado/backend-go-role-organizado/internal/usecase/admin"
)

// publicPrefixes are routes that bypass JWT authentication.
var publicPrefixes = []string{
	"/actuator/",
	"/api/v1/auth/",
	"/api/auth/",
	"/api/v1/webhooks/",
	"/swagger/",
	"/docs/",
}

func main() {
	// Load configuration from environment variables.
	cfg, err := config.Load()
	if err != nil {
		slog.Error("loading config", "error", err)
		os.Exit(1)
	}

	// Setup structured JSON logging.
	logLevel := slog.LevelInfo
	if cfg.Server.Env == "local" {
		logLevel = slog.LevelDebug
	}
	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})

	// Sempre wrappear com TraceContextHandler para injetar trace_id/span_id no JSON stdout.
	// Quando OTel está habilitado, o TeeHandler encaminha também para o LoggerProvider.
	tracedJSON := pkgotel.NewTraceContextHandler(jsonHandler)
	logHandler := tracedJSON

	ctx := context.Background()

	if cfg.OTel.Enabled {
		providers, err := pkgotel.Init(ctx, pkgotel.Config{
			OTLPEndpoint:   cfg.OTel.Endpoint,
			ServiceName:    cfg.OTel.ServiceName,
			ServiceVersion: cfg.OTel.ServiceVersion,
			Environment:    cfg.Server.Env,
		})
		if err != nil {
			slog.Error("initializing otel", "error", err)
			os.Exit(1)
		}
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := providers.Shutdown(shutdownCtx); err != nil {
				slog.Error("otel shutdown", "error", err)
			}
		}()
		logHandler = pkgotel.NewTeeHandler(providers.LoggerProvider, tracedJSON)
	}

	logger := slog.New(logHandler)
	slog.SetDefault(logger)

	slog.Info("starting backend-go-role-organizado",
		"env", cfg.Server.Env,
		"port", cfg.Server.Port,
	)

	// Connect to MongoDB.
	mongoClient, err := mongodb.Connect(ctx, cfg.MongoDB.URI, cfg.MongoDB.Database)
	if err != nil {
		slog.Error("connecting to mongodb", "error", err)
		os.Exit(1)
	}

	// Run Go migrations at startup.
	if err := migrations.RunV081NichoBabyShower(ctx, mongoClient.DB()); err != nil {
		slog.Error("migration v081 failed", "error", err)
		os.Exit(1)
	}
	if err := migrations.RunV082CreateCofrinhoCollection(ctx, mongoClient.DB()); err != nil {
		slog.Error("migration v082 failed", "error", err)
		os.Exit(1)
	}
	if err := migrations.RunV083CreateListaPresentesCollection(ctx, mongoClient.DB()); err != nil {
		slog.Error("migration v083 failed", "error", err)
		os.Exit(1)
	}
	if err := migrations.RunV084CreatePaymentTransactionsIndexes(ctx, mongoClient.DB()); err != nil {
		slog.Error("migration v084 failed", "error", err)
		os.Exit(1)
	}
	if err := migrations.RunV085CreateSocialFeaturesIndexes(ctx, mongoClient.DB()); err != nil {
		slog.Error("migration v085 failed", "error", err)
		os.Exit(1)
	}
	if err := migrations.RunV088CreateGuestsBiometricIndexes(ctx, mongoClient.DB()); err != nil {
		slog.Error("migration v088 failed", "error", err)
		os.Exit(1)
	}
	if err := migrations.RunV086CreateConviteIndexes(ctx, mongoClient.DB()); err != nil {
		slog.Error("migration v086 failed", "error", err)
		os.Exit(1)
	}
	if err := migrations.RunV087CreateOutboundIndexes(ctx, mongoClient.DB()); err != nil {
		slog.Error("migration v087 failed", "error", err)
		os.Exit(1)
	}
	if err := migrations.RunV089CreateAccountingSnapshots(ctx, mongoClient.DB()); err != nil {
		slog.Error("migration v089 failed", "error", err)
		os.Exit(1)
	}

	// Build JWT service.
	jwtSvc, err := pkgjwt.NewService(
		cfg.JWT.Secret,
		cfg.JWT.AccessTokenTTL,
		cfg.JWT.RefreshTokenTTL,
	)
	if err != nil {
		slog.Error("creating jwt service", "error", err)
		os.Exit(1)
	}

	// --- Phase 1: Config domain repositories ---
	dominioRepo := mongodb.NewDominioRepository(mongoClient)
	configSistemaRepo := mongodb.NewConfigSistemaRepository(mongoClient)

	// --- Phase 1: Config domain use cases ---
	listDominiosUC := ucconfig.NewListDominios(dominioRepo)
	getDominioUC := ucconfig.NewGetDominio(dominioRepo)
	upsertDominioUC := ucconfig.NewUpsertDominio(dominioRepo)
	deleteDominioUC := ucconfig.NewDeleteDominio(dominioRepo)
	getConfigUC := ucconfig.NewGetConfigSistema(configSistemaRepo)
	upsertConfigUC := ucconfig.NewUpsertConfigSistema(configSistemaRepo)

	// --- Phase 2: Auth domain repositories ---
	usuarioRepo := mongodb.NewUsuarioRepository(mongoClient)
	refreshTokenRepo := mongodb.NewRefreshTokenRepository(mongoClient)

	// --- Phase 2: Auth domain use cases ---
	loginUC := ucauth.NewLogin(usuarioRepo, refreshTokenRepo, jwtSvc)
	registerUC := ucauth.NewRegister(usuarioRepo, refreshTokenRepo, jwtSvc)
	refreshUC := ucauth.NewRefreshToken(refreshTokenRepo, usuarioRepo, jwtSvc)
	validateUC := ucauth.NewValidateToken(usuarioRepo, jwtSvc)
	logoutUC := ucauth.NewLogout(refreshTokenRepo)
	googleUC := ucauth.NewGoogleAuth(usuarioRepo, refreshTokenRepo, jwtSvc)
	appleUC := ucauth.NewAppleAuth(usuarioRepo, refreshTokenRepo, jwtSvc)
	getUsuarioUC := ucauth.NewGetUsuario(usuarioRepo)
	updateUsuarioUC := ucauth.NewUpdateUsuario(usuarioRepo)
	listUsuariosUC := ucauth.NewListUsuarios(usuarioRepo)
	updateRoleUC := ucauth.NewUpdateUserRole(usuarioRepo)

	// --- HTTP Handlers ---
	configHandler := handler.NewConfigHandler(
		listDominiosUC, getDominioUC, upsertDominioUC, deleteDominioUC,
		getConfigUC, upsertConfigUC,
	)
	authHandler := handler.NewAuthHandler(
		loginUC, registerUC, refreshUC, validateUC, logoutUC, googleUC, appleUC,
	)
	usuarioHandler := handler.NewUsuarioHandler(
		getUsuarioUC, updateUsuarioUC, listUsuariosUC, updateRoleUC, mongoClient,
	)

	// --- Phase 3: Events & Drafts domain repositories ---
	eventoRepo := mongodb.NewEventoRepository(mongoClient)
	draftRepo := mongodb.NewEventoDraftRepository(mongoClient)
	participanteRepo := mongodb.NewParticipanteRepository(mongoClient)

	// --- Phase 3: Events domain use cases ---
	createEventoUC := ucevent.NewCreateEvento(eventoRepo, participanteRepo)
	getEventoUC := ucevent.NewGetEvento(eventoRepo)
	listEventosUC := ucevent.NewListEventos(eventoRepo)
	updateEventoUC := ucevent.NewUpdateEvento(eventoRepo)
	deleteEventoUC := ucevent.NewDeleteEvento(eventoRepo)
	listEventosByUsuarioUC := ucevent.NewListEventosByUsuario(eventoRepo)
	addConvidadosUC := ucevent.NewAddConvidados(eventoRepo)

	// --- Phase 3: Drafts domain use cases ---
	createDraftUC := ucevent.NewCreateDraft(draftRepo)
	getDraftUC := ucevent.NewGetDraft(draftRepo)
	listDraftsUC := ucevent.NewListDrafts(draftRepo)
	updateDraftUC := ucevent.NewUpdateDraft(draftRepo)
	deleteDraftUC := ucevent.NewDeleteDraft(draftRepo)
	publishDraftUC := ucevent.NewPublishDraft(draftRepo, eventoRepo)
	validateDraftUC := ucevent.NewValidateDraft(draftRepo)

	// --- Phase 3: HTTP Handlers ---
	eventoHandler := handler.NewEventHandler(
		createEventoUC, getEventoUC, listEventosUC, updateEventoUC, deleteEventoUC,
		listEventosByUsuarioUC, addConvidadosUC,
	)

	// --- CSE_014: Eventos Advanced (fase/imagens/summaries/politica/detalhes/gerenciar/public-info/join/completo) ---
	// txRepo is created later in Phase 5 (line ~283). We will wire after it exists.
	// participantRepo (read-side) for completo is created later in Phase Finance (line ~519).
	// We defer construction below where all repos exist.
	draftHandler := handler.NewDraftHandler(
		createDraftUC, getDraftUC, listDraftsUC, updateDraftUC, deleteDraftUC, publishDraftUC,
		validateDraftUC,
	)

	// --- Phase 4: Rateios domain ---
	rateioRepo := mongodb.NewRateioRepository(mongoClient)
	fechamentoRepo := mongodb.NewRateioFechamentoRepository(mongoClient)

	createRateioUC := ucrateio.NewCreateRateio(rateioRepo)
	getRateioUC := ucrateio.NewGetRateio(rateioRepo)
	listRateiosUC := ucrateio.NewListRateios(rateioRepo)
	updateRateioUC := ucrateio.NewUpdateRateio(rateioRepo)
	deleteRateioUC := ucrateio.NewDeleteRateio(rateioRepo)
	previewRateioUC := ucrateio.NewPreviewRateio(rateioRepo)
	fecharRateioUC := ucrateio.NewFecharRateio(rateioRepo, fechamentoRepo)
	getFechamentosUC := ucrateio.NewGetFechamentos(rateioRepo, fechamentoRepo)

	rateioHandler := handler.NewRateioHandler(
		createRateioUC, getRateioUC, listRateiosUC, updateRateioUC, deleteRateioUC,
		previewRateioUC, fecharRateioUC, getFechamentosUC,
	)

	// --- Phase 5: Payments domain ---
	pagamentoRepo := mongodb.NewPagamentoRepository(mongoClient)
	configPagRepo := mongodb.NewConfigPagamentoRepository(mongoClient)
	installmentRepo := mongodb.NewPaymentInstallmentRepository(mongoClient)

	createPayUC := ucpayment.NewCreatePagamento(pagamentoRepo)
	getPayUC := ucpayment.NewGetPagamento(pagamentoRepo)
	listPayUC := ucpayment.NewListPagamentos(pagamentoRepo)
	updatePayUC := ucpayment.NewUpdatePagamento(pagamentoRepo)
	deletePayUC := ucpayment.NewDeletePagamento(pagamentoRepo)
	confirmarPayUC := ucpayment.NewConfirmarPagamento(pagamentoRepo)
	upsertCfgPayUC := ucpayment.NewUpsertConfigPagamento(configPagRepo)
	getCfgPayUC := ucpayment.NewGetConfigPagamento(configPagRepo)

	// Payment provider (Asaas real vs mock, controlled by ROLE_ASAAS_USE_MOCK).
	var paymentProvider portout.PaymentProvider
	var providerName paymentdomain.PaymentProvider
	if cfg.Asaas.UseMock {
		paymentProvider = ucasaas.NewMockProvider()
		providerName = paymentdomain.PaymentProviderMock
	} else {
		paymentProvider = ucasaas.NewClient(cfg.Asaas)
		providerName = paymentdomain.PaymentProviderAsaas
	}

	// Real-payment repos.
	txRepo := mongodb.NewPaymentTransactionRepository(mongoClient)
	customerLinkRepo := mongodb.NewAsaasCustomerLinkRepository(mongoClient)
	savedCardRepo := mongodb.NewSavedCreditCardRepository(mongoClient)

	// Fee policy snapshot + subledger dual-write services.
	// FeePolicyService now uses the typed repository (refactor: typed fields vs raw BSON).
	feePolicySvc := ucpayment.NewFeePolicyService(configPagRepo)
	subledgerSvc := ucpayment.NewSubledgerDualWriteService(
		mongoClient.Collection("ledger_entries"),
		mongoClient.Collection("ledger_snapshot_events"),
	)

	// ProcessPayment / GetTransaction / ListUserPayments use cases.
	processPaymentUC := ucpayment.NewProcessPayment(
		txRepo, customerLinkRepo, usuarioRepo, savedCardRepo,
		paymentProvider, providerName, feePolicySvc, subledgerSvc,
	)
	processBatchPaymentUC := ucpayment.NewProcessBatchPayment(
		installmentRepo, participanteRepo,
		txRepo, customerLinkRepo, usuarioRepo, savedCardRepo,
		paymentProvider, providerName, feePolicySvc, subledgerSvc,
	)
	getTransactionUC := ucpayment.NewGetPaymentTransaction(txRepo)
	listUserPaymentsUC := ucpayment.NewListUserPayments(txRepo)

	listUserInstallmentsUC := ucpayment.NewListUserInstallments(installmentRepo, participanteRepo, eventoRepo)
	listInstallmentsUC := ucpayment.NewListInstallments(installmentRepo, participanteRepo)
	getInstallmentUC := ucpayment.NewGetInstallment(installmentRepo, participanteRepo)
	cancelInstallmentsUC := ucpayment.NewCancelParticipantInstallments(installmentRepo, participanteRepo, eventoRepo)

	reaplicarFeeUC := ucpayment.NewReaplicarFeePolicySnapshot(configPagRepo)

	paymentHandler := handler.NewPaymentHandler(
		createPayUC, getPayUC, listPayUC, updatePayUC, deletePayUC,
		confirmarPayUC, upsertCfgPayUC, getCfgPayUC,
		processPaymentUC, processBatchPaymentUC, getTransactionUC, listUserPaymentsUC, paymentProvider,
		reaplicarFeeUC,
	)
	installmentHandler := handler.NewInstallmentHandler(
		listUserInstallmentsUC, listInstallmentsUC, getInstallmentUC, cancelInstallmentsUC,
	)

	// --- Phase 6: Notifications domain ---
	notifTemplateRepo := mongodb.NewNotificationTemplateRepository(mongoClient)

	createTemplateUC := ucnotiftemplate.NewCreateNotificationTemplate(notifTemplateRepo)
	getTemplateUC := ucnotiftemplate.NewGetNotificationTemplate(notifTemplateRepo)
	listTemplatesUC := ucnotiftemplate.NewListNotificationTemplates(notifTemplateRepo)
	updateTemplateUC := ucnotiftemplate.NewUpdateNotificationTemplate(notifTemplateRepo)
	deleteTemplateUC := ucnotiftemplate.NewDeleteNotificationTemplate(notifTemplateRepo)
	renderTemplateUC := ucnotiftemplate.NewRenderNotificationTemplate(notifTemplateRepo)
	testSendTemplateUC := ucnotiftemplate.NewTestSendNotificationTemplate(notifTemplateRepo)
	getByTypeTemplateUC := ucnotiftemplate.NewGetByTypeNotificationTemplate(notifTemplateRepo)
	listCategoriaTemplateUC := ucnotiftemplate.NewListByCategoriaNotificationTemplate(notifTemplateRepo)

	// Notification stages — reuse the notification_templates collection (Java parity).
	notifStageRepo := mongodb.NewNotificationStageRepository(mongoClient)
	manageStagesUC := ucnotiftemplate.NewManageNotificationStages(notifStageRepo)

	notifTemplateHandler := handler.NewNotificationTemplateHandler(
		createTemplateUC, getTemplateUC, listTemplatesUC, updateTemplateUC, deleteTemplateUC,
		renderTemplateUC, testSendTemplateUC, getByTypeTemplateUC, listCategoriaTemplateUC,
		manageStagesUC, manageStagesUC.AsGetUseCase(), manageStagesUC.AsUpsertUseCase(),
		manageStagesUC.AsDeleteUseCase(), manageStagesUC.AsTestSendUseCase(),
	)

	notificacaoRepo := mongodb.NewNotificacaoRepository(mongoClient)

	listNotifUC := ucnotification.NewListNotificacoes(notificacaoRepo)
	getNotifUC := ucnotification.NewGetNotificacao(notificacaoRepo)
	createNotifUC := ucnotification.NewCreateNotificacao(notificacaoRepo)
	marcarLidaUC := ucnotification.NewMarcarLida(notificacaoRepo)
	marcarTodasUC := ucnotification.NewMarcarTodasLidas(notificacaoRepo)
	deleteNotifUC := ucnotification.NewDeleteNotificacao(notificacaoRepo)
	countUnreadUC := ucnotification.NewCountUnread(notificacaoRepo)

	notificationHandler := handler.NewNotificationHandler(
		listNotifUC, getNotifUC, createNotifUC, marcarLidaUC, marcarTodasUC, deleteNotifUC, countUnreadUC,
	)

	// --- Phase 5c: Webhook callback (wired here — depends on createNotifUC from Phase 6) ---
	webhookRepo := mongodb.NewProcessedWebhookEventRepository(mongoClient)
	allocationSvc := ucpayment.NewInstallmentAllocationService(
		mongoClient.Collection("installment_allocations"),
		installmentRepo,
	)
	handlePaymentCallbackUC := ucpayment.NewHandlePaymentCallback(
		txRepo, installmentRepo, webhookRepo, allocationSvc, subledgerSvc, createNotifUC,
	)
	paymentWebhookHandler := handler.NewPaymentWebhookHandler(handlePaymentCallbackUC, cfg.Asaas.WebhookToken)

	// === WHATSAPP WEBHOOK ===
	// Backfill: Meta WhatsApp Business Cloud API webhook (paridade Java
	// WhatsAppWebhookController) — POST callback + GET hub.challenge verify.
	// Reuses the same ProcessedWebhookEventRepository (provider="WHATSAPP") for
	// idempotency.
	handleWhatsAppWebhookUC := ucnotification.NewHandleWhatsAppWebhook(webhookRepo)
	whatsappWebhookHandler := handler.NewWhatsAppWebhookHandler(
		handleWhatsAppWebhookUC,
		cfg.WhatsApp.WebhookVerifyToken,
	)

	// --- Phase 7: File Storage (GridFS) ---
	arquivoRepo := mongodb.NewArquivoRepository(mongoClient)
	gridfsStorage := mongodb.NewGridFSStorageAdapter(mongoClient)
	uploadUC := ucstorage.NewUploadArquivo(arquivoRepo, gridfsStorage)
	downloadUC := ucstorage.NewDownloadArquivo(arquivoRepo, gridfsStorage)
	deleteArquivoUC := ucstorage.NewDeleteArquivo(arquivoRepo, gridfsStorage)
	storageHandler := handler.NewStorageHandler(uploadUC, downloadUC, deleteArquivoUC)

	// --- Cofrinho domain ---
	cofrinhoRepo := mongodb.NewCofrinhoRepository(mongoClient)
	createContribuicaoUC := uccofrinho.NewCreateContribuicao(cofrinhoRepo)
	listContribuicoesUC := uccofrinho.NewListContribuicoes(cofrinhoRepo)
	confirmarContribuicaoUC := uccofrinho.NewConfirmarContribuicao(cofrinhoRepo)
	removerContribuicaoUC := uccofrinho.NewRemoverContribuicao(cofrinhoRepo)
	cofrinhoHandler := handler.NewCofrinhoHandler(createContribuicaoUC, listContribuicoesUC, confirmarContribuicaoUC, removerContribuicaoUC)

	// --- Lista Presentes domain ---
	listaPresentesRepo := mongodb.NewListaPresentesRepository(mongoClient)
	addItemUC := uclistapresentes.NewAddItem(listaPresentesRepo, eventoRepo)
	getItemUC := uclistapresentes.NewGetItem(listaPresentesRepo)
	listItemsUC := uclistapresentes.NewListItems(listaPresentesRepo)
	reservarItemUC := uclistapresentes.NewReservarItem(listaPresentesRepo)
	removeItemUC := uclistapresentes.NewRemoveItem(listaPresentesRepo)
	listaPresentesHandler := handler.NewListaPresentesHandler(addItemUC, getItemUC, listItemsUC, reservarItemUC, removeItemUC)

	// --- Social Features domain ---
	socialRepo := mongodb.NewSocialFeaturesRepository(mongoClient)
	eventoAuthPort := mongodb.NewEventoAuthAdapter(mongoClient)
	getSocialFeaturesUC := ucsocial.NewGetSocialFeatures(socialRepo, eventoAuthPort)
	setDressCodeUC := ucsocial.NewSetDressCode(socialRepo, eventoAuthPort)
	removeDressCodeUC := ucsocial.NewRemoveDressCode(socialRepo, eventoAuthPort)
	addPlaylistUC := ucsocial.NewAddPlaylist(socialRepo, eventoAuthPort)
	removePlaylistUC := ucsocial.NewRemovePlaylist(socialRepo, eventoAuthPort)
	addBringListItemUC := ucsocial.NewAddBringListItem(socialRepo, eventoAuthPort)
	updateBringListItemUC := ucsocial.NewUpdateBringListItem(socialRepo, eventoAuthPort)
	removeBringListItemUC := ucsocial.NewRemoveBringListItem(socialRepo, eventoAuthPort)
	claimBringListItemUC := ucsocial.NewClaimBringListItem(socialRepo, eventoAuthPort)
	unclaimBringListItemUC := ucsocial.NewUnclaimBringListItem(socialRepo, eventoAuthPort)
	setCheckinHabilitadoUC := ucsocial.NewSetCheckinHabilitado(socialRepo, eventoAuthPort)
	doCheckinUC := ucsocial.NewDoCheckin(socialRepo, eventoAuthPort)
	addAlbumLinkUC := ucsocial.NewAddAlbumLink(socialRepo, eventoAuthPort)
	removeAlbumLinkUC := ucsocial.NewRemoveAlbumLink(socialRepo, eventoAuthPort)
	socialHandler := handler.NewSocialHandler(
		getSocialFeaturesUC,
		setDressCodeUC,
		removeDressCodeUC,
		addPlaylistUC,
		removePlaylistUC,
		addBringListItemUC,
		updateBringListItemUC,
		removeBringListItemUC,
		claimBringListItemUC,
		unclaimBringListItemUC,
		setCheckinHabilitadoUC,
		doCheckinUC,
		addAlbumLinkUC,
		removeAlbumLinkUC,
	)

	// === GUESTS DOMAIN ===
	// Java parity: GuestController + CriarOuBuscarGuestUseCase (collection: guests).
	// All endpoints are PUBLIC — registered outside the JWT-protected group below.
	guestRepo := mongodb.NewGuestRepository(mongoClient)
	createOrFindGuestUC := ucguest.NewCreateOrFindGuest(guestRepo)
	getGuestUC := ucguest.NewGetGuest(guestRepo)
	getGuestByTelefoneUC := ucguest.NewGetGuestByTelefone(guestRepo)
	getGuestByEmailUC := ucguest.NewGetGuestByEmail(guestRepo)
	listGuestsUC := ucguest.NewListGuests(guestRepo)
	batchGetGuestsUC := ucguest.NewBatchGetGuests(guestRepo)
	guestHandler := handler.NewGuestHandler(
		createOrFindGuestUC, getGuestUC, getGuestByTelefoneUC, getGuestByEmailUC,
		listGuestsUC, batchGetGuestsUC,
	)

	// === BIOMETRIC AUTH DOMAIN ===
	// Java parity: BiometricAuthController + BiometricAuthUseCase.
	// Collections: biometric_credentials, biometric_challenges (TTL on expires_at).
	// Challenge / authenticate / status are PUBLIC; register / devices / revoke require JWT.
	biometricCredRepo := mongodb.NewBiometricCredentialRepository(mongoClient)
	biometricChallengeRepo := mongodb.NewBiometricChallengeRepository(mongoClient)
	generateChallengeUC := ucguest.NewGenerateBiometricChallenge(biometricCredRepo, biometricChallengeRepo)
	biometricAuthUC := ucguest.NewBiometricAuthenticate(
		biometricCredRepo, biometricChallengeRepo, usuarioRepo, refreshTokenRepo, jwtSvc,
	)
	registerCredentialUC := ucguest.NewRegisterBiometricCredential(biometricCredRepo, usuarioRepo)
	listDevicesUC := ucguest.NewListBiometricDevices(biometricCredRepo)
	revokeDeviceUC := ucguest.NewRevokeBiometricDevice(biometricCredRepo, biometricChallengeRepo)
	checkBiometricStatusUC := ucguest.NewCheckBiometricStatus(biometricCredRepo)
	biometricHandler := handler.NewBiometricHandler(
		generateChallengeUC, biometricAuthUC, registerCredentialUC,
		listDevicesUC, revokeDeviceUC, checkBiometricStatusUC,
	)

	// === CONVITES DOMAIN ===
	// Repos shared with Java (participants, guests, approval_items, participant_credits,
	// audit_entries, payment_installments, dominios). All UUIDs stored as Binary subtype 4.
	conviteParticipantRepo := mongodb.NewConviteParticipantRepository(mongoClient)
	conviteGuestRepo := mongodb.NewConviteGuestRepository(mongoClient)

	// Guest → user linking (Feature 016/027), invoked after /register + OAuth.
	// Draft rewriting is left disabled (nil) until an EventoDraft convidados
	// conversion port is implemented; participant migration is wired live.
	vincularGuestUC := ucguest.NewVincularGuest(guestRepo, conviteParticipantRepo, nil)
	authHandler.WithVincularGuest(vincularGuestUC)
	conviteApprovalRepo := mongodb.NewConviteApprovalRepository(mongoClient)
	conviteCreditRepo := mongodb.NewConviteParticipantCreditRepository(mongoClient)
	conviteAuditRepo := mongodb.NewConviteAuditRepository(mongoClient)
	convitePoliticaRepo := mongodb.NewConvitePoliticaRepository(mongoClient)
	conviteInstallmentRepo := mongodb.NewConviteInstallmentRepository(mongoClient)
	// Convite notification port: real SQS publisher when enabled, no-op otherwise.
	var conviteNotifPort portout.ConviteNotificationPort = mongodb.NewNoopConviteNotificationAdapter()
	if cfg.SQS.Enabled {
		sqsPublisher, sqsErr := sqsadapter.NewConvitePublisher(ctx, cfg.SQS)
		if sqsErr != nil {
			slog.Error("failed to initialise SQS convite publisher; falling back to no-op", "error", sqsErr)
		} else {
			conviteNotifPort = sqsPublisher
			slog.Info("SQS convite publisher enabled", "region", cfg.SQS.Region, "queueURL", cfg.SQS.QueueURL)
		}
	}

	// Use cases — buscar must be wired first because confirmar/recusar reuse it.
	buscarConviteUC := ucconvite.NewBuscarConvite(
		conviteParticipantRepo, conviteApprovalRepo, eventoRepo, usuarioRepo, conviteGuestRepo,
	)
	enviarConviteUC := ucconvite.NewEnviarConvite(
		conviteParticipantRepo, conviteApprovalRepo, eventoRepo, usuarioRepo, conviteNotifPort,
	)
	reabrirInviteUC := ucconvite.NewReabrirInviteApproval(conviteParticipantRepo, conviteApprovalRepo)
	reenviarMassaUC := ucconvite.NewReenviarConvitesMassaAdmin(eventoRepo, conviteAuditRepo)

	// --- Phase 8: Temporal Worker (payment workflows) ---
	// Enabled via TEMPORAL_WORKER_ENABLED=true (default: false during Strangler Fig migration).
	// When disabled, no-op starters/signalers are used so the payment use cases compile and
	// run correctly without a live Temporal cluster.
	var temporalStarter portout.TemporalWorkflowStarter = &temporaladapter.NoopWorkflowStarter{}
	var temporalSignaler portout.TemporalWorkflowSignaler = &temporaladapter.NoopWorkflowSignaler{}
	// temporalQueryClient backs the native workflow status/signal handler. It stays
	// nil when the worker is disabled, in which case the handler falls back to the
	// Java proxy. Set in the Phase 9 worker block below.
	var temporalQueryClient sdkclient.Client

	if cfg.Temporal.WorkerEnabled {
		temporalClient, temporalErr := temporaladapter.NewClient(cfg.Temporal)
		if temporalErr != nil {
			slog.Error("failed to connect to Temporal", "error", temporalErr)
			os.Exit(1)
		}
		defer temporalClient.Close()

		temporalStarter = temporaladapter.NewClientWorkflowStarter(temporalClient)
		temporalSignaler = temporaladapter.NewClientWorkflowSignaler(temporalClient)

		// Build activities and register workers.
		reconcileReportRepo := mongodb.NewReconciliationReportRepository(mongoClient)
		expireUC := ucpayment.NewExpireTransaction(txRepo)
		reconcileUC := ucpayment.NewReconcilePspTransactions(txRepo, paymentProvider)

		paymentActs := temporalactivity.NewPaymentActivities(
			txRepo, expireUC, handlePaymentCallbackUC, reconcileUC, reconcileReportRepo,
		)

		temporalRegistry := temporalworker.NewRegistry(temporalClient)
		temporalRegistry.RegisterPaymentWorker(paymentActs)
		temporalRegistry.RegisterReconciliationWorker(paymentActs)
		temporalRegistry.RegisterSandboxWorker(temporalactivity.NewSandboxActivity())
		// Native Go finance reconciliation (no HTTP delegation to Java).
		financeSummaryRepoEarly := mongodb.NewFinanceSummaryRepository(mongoClient)
		ledgerEntryRepoEarly := mongodb.NewLedgerEntryRepository(mongoClient)
		financeReconReportRepo := mongodb.NewFinanceReconciliationReportRepository(mongoClient)
		reconUC := ucfinance.NewReconciliationUseCase(
			eventoRepo,
			financeSummaryRepoEarly,
			ledgerEntryRepoEarly,
			txRepo,
			financeReconReportRepo,
		)
		financeReconActs := temporalactivity.NewFinanceReconciliationActivities(reconUC)
		temporalRegistry.RegisterFinanceReconciliationWorker(financeReconActs)

		// Native Go PSP cost review (no HTTP delegation to Java).
		pspReviewReportRepo := mongodb.NewPspReviewReportRepository(mongoClient)
		pspReviewUC := ucpricing.NewRunPspCostReview(configPagRepo, txRepo, pspReviewReportRepo)
		pspReviewActivity := temporalactivity.NewPricingPspReviewActivity(pspReviewUC)
		temporalRegistry.RegisterPricingPspReviewWorker(pspReviewActivity)

		// Native Go overdue installment marker + dispatcher (no HTTP delegation).
		findMarkUC := ucpayment.NewFindAndMarkOverdueInstallments(installmentRepo)
		dispatchUC := ucnotification.NewDispatchOverdueNotifications(findMarkUC, createNotifUC)
		overdueActs := temporalactivity.NewOverdueInstallmentActivities(findMarkUC, dispatchUC)
		temporalRegistry.RegisterOverdueInstallmentWorker(overdueActs)

		// --- Temporal Onda 2: ParticipantRecalculation (native Go) ---
		financeInstallmentRepoEarly := mongodb.NewFinanceInstallmentRepository(mongoClient)
		recalcFinanceUC := ucfinance.NewRecalculateFinanceSummary(
			financeSummaryRepoEarly, rateioRepo, financeInstallmentRepoEarly,
		)
		participantRecalcActs := temporalactivity.NewParticipantRecalculationActivities(recalcFinanceUC)
		temporalRegistry.RegisterParticipantRecalculationWorker(participantRecalcActs)

		// --- Temporal Onda 2: AccountingSnapshot (native Go) ---
		accountingSnapshotRepo := mongodb.NewAccountingSnapshotRepository(mongoClient)
		generateSnapshotUC := ucaccounting.NewGenerateSnapshot(accountingSnapshotRepo)
		accountingSnapshotActs := temporalactivity.NewAccountingSnapshotActivities(generateSnapshotUC)
		temporalRegistry.RegisterAccountingSnapshotWorker(accountingSnapshotActs)

		// --- Temporal Onda 2: PspReconciliation (reuses reconcile UC + report repo) ---
		pspReconActs := temporalactivity.NewPspReconciliationActivities(reconcileUC, reconcileReportRepo)
		temporalRegistry.RegisterPspReconciliationWorker(pspReconActs)

		if startErr := temporalRegistry.Start(); startErr != nil {
			slog.Error("failed to start Temporal workers", "error", startErr)
			os.Exit(1)
		}
		defer temporalRegistry.Stop()

		schedCtx, schedCancel := context.WithTimeout(ctx, 15*time.Second)
		defer schedCancel()
		if schedErr := temporalRegistry.InitPricingPspReviewSchedule(schedCtx); schedErr != nil {
			slog.Warn("temporal: failed to init pricing-psp-review schedule", "error", schedErr)
		}
		if schedErr := temporalworker.InitFinanceReconciliationSchedule(schedCtx, temporalClient); schedErr != nil {
			slog.Warn("finance reconciliation schedule init failed", "error", schedErr)
		}
		if schedErr := temporalworker.InitOverdueInstallmentSchedule(schedCtx, temporalClient); schedErr != nil {
			slog.Warn("overdue-installment schedule init failed", "error", schedErr)
		}
		if schedErr := temporalworker.InitAccountingSnapshotSchedule(schedCtx, temporalClient); schedErr != nil {
			slog.Warn("accounting-snapshot schedule init failed", "error", schedErr)
		}
		if schedErr := temporalworker.InitPspReconciliationSchedule(schedCtx, temporalClient); schedErr != nil {
			slog.Warn("psp-reconciliation schedule init failed", "error", schedErr)
		}

		slog.Info("temporal workers started",
			"hostPort", cfg.Temporal.HostPort,
			"namespace", cfg.Temporal.Namespace,
		)
	} else {
		slog.Info("temporal workers disabled (TEMPORAL_WORKER_ENABLED=false)")
	}

	// Wire Temporal starters/signalers into payment use cases (nil-safe, non-breaking).
	// ProcessBatchPayment: TODO add WithTemporalStarter when batch expiration is required.
	processPaymentUC.WithTemporalStarter(temporalStarter)
	handlePaymentCallbackUC.
		WithTemporalSignaler(temporalSignaler).
		WithTemporalStarter(temporalStarter)

	// === CONVITES DOMAIN — temporal-dependent UCs + handler ===
	confirmarConviteUC := ucconvite.NewConfirmarConvite(
		conviteParticipantRepo, conviteApprovalRepo, buscarConviteUC, temporalStarter,
	)
	recusarConviteUC := ucconvite.NewRecusarConvite(
		conviteParticipantRepo, conviteApprovalRepo, buscarConviteUC, temporalStarter,
	)
	desistirEventoUC := ucconvite.NewDesistirEvento(
		conviteParticipantRepo, eventoRepo, convitePoliticaRepo, conviteInstallmentRepo, conviteCreditRepo, temporalStarter,
	)
	conviteHandler := handler.NewConviteHandler(
		buscarConviteUC, enviarConviteUC, confirmarConviteUC, recusarConviteUC,
		desistirEventoUC, reabrirInviteUC, reenviarMassaUC,
	)

	// --- Phase 8b: Temporal Workflow Proxies (Java fallback) ---
	// The native handler serves the 6 migrated (Go) workflows directly via the
	// Temporal Go SDK and falls back to this proxy for everything else.
	workflowProxyHandler := handler.NewWorkflowProxyHandler(cfg.Server.JavaBackendURL)

	// --- Phase 5b: Payment Methods + PIX validate + Saved Cards (hexagonal) ---
	// savedCardRepo is reused from Phase 5 (already declared above).
	paymentAccountRepo := mongodb.NewPaymentAccountRepository(mongoClient)

	manageAccountsUC := ucpayment.NewManagePaymentAccounts(paymentAccountRepo)
	validatePixUC := ucpayment.NewValidatePixKey()
	manageSavedCardsUC := ucpayment.NewManageSavedCards(savedCardRepo)

	paymentMethodsHandler := handler.NewPaymentMethodsHandler(manageAccountsUC, validatePixUC, manageSavedCardsUC)

	// --- Finance domain (hexagonal — zero direct Mongo in handler) ---
	financeSummaryRepo := mongodb.NewFinanceSummaryRepository(mongoClient)
	ledgerEntryRepo := mongodb.NewLedgerEntryRepository(mongoClient)
	auditTrailRepo := mongodb.NewAuditTrailRepository(mongoClient)
	participantRepo := mongodb.NewParticipantRepository(mongoClient)
	financeInstallmentRepo := mongodb.NewFinanceInstallmentRepository(mongoClient)
	financeAccountRepo := mongodb.NewFinanceAccountRepository(mongoClient)

	// --- CSE_014: Eventos Advanced wiring (after all repos exist) ---
	eventoImagemStorage := mongodb.NewEventoImagemStorageAdapter(mongoClient)
	alterarFaseUC := ucevent.NewAlterarFase(eventoRepo, participanteRepo, txRepo)
	uploadImagensUC := ucevent.NewUploadImagens(eventoRepo, eventoImagemStorage)
	buscarSummariesUC := ucevent.NewBuscarSummaries(eventoRepo)
	atualizarPoliticaUC := ucevent.NewAtualizarPoliticaConvidados(eventoRepo, participanteRepo)
	atualizarDetalhesUC := ucevent.NewAtualizarDetalhes(eventoRepo, participanteRepo)
	gerenciarUC := ucevent.NewGerenciarEvento(eventoRepo, participanteRepo, txRepo)
	getCompletoUC := ucevent.NewGetEventoCompleto(eventoRepo, participantRepo, rateioRepo)
	getPublicInfoUC := ucevent.NewGetPublicInfo(eventoRepo, participanteRepo, usuarioRepo)
	joinEventoUC := ucevent.NewJoinEvento(eventoRepo, participanteRepo)
	eventoHandler = eventoHandler.WithAdvanced(
		alterarFaseUC,
		uploadImagensUC,
		buscarSummariesUC,
		atualizarPoliticaUC,
		atualizarDetalhesUC,
		gerenciarUC,
		getCompletoUC,
		getPublicInfoUC,
		joinEventoUC,
	)

	listEventsUC := ucfinance.NewListFinanceEvents(participantRepo, eventoRepo, rateioRepo, financeSummaryRepo)
	overviewUC := ucfinance.NewGetFinanceOverview(eventoRepo, participantRepo, rateioRepo, financeSummaryRepo)
	ledgerUC := ucfinance.NewGetLedgerStatement(ledgerEntryRepo, participantRepo)
	participantsStatusUC := ucfinance.NewGetParticipantsStatus(participantRepo, financeInstallmentRepo)
	recalculateUC := ucfinance.NewRecalculateFinanceSummary(financeSummaryRepo, rateioRepo, financeInstallmentRepo)
	sendRemindersUC := ucfinance.NewSendPaymentReminders(participantRepo, financeInstallmentRepo, nil)
	holdBalanceUC := ucfinance.NewCalculateHoldBalance(financeInstallmentRepo, configSistemaRepo)
	paymentStatusUC := ucfinance.NewGetEventPaymentStatus(financeInstallmentRepo, participantRepo)
	paymentAccountsUC := ucfinance.NewManagePaymentAccounts(financeAccountRepo)
	auditTrailUC := ucfinance.NewGetAuditTrail(auditTrailRepo)

	financeHandler := handler.NewFinanceHandler(
		listEventsUC, overviewUC, ledgerUC, participantsStatusUC, recalculateUC,
		sendRemindersUC, holdBalanceUC, paymentStatusUC, paymentAccountsUC, auditTrailUC,
	)

	// --- Participantes handlers (direct MongoDB) ---
	participantesHandler := handler.NewParticipantesHandler(mongoClient)
	usuariosEventoHandler := handler.NewUsuariosEventoHandler(mongoClient)

	// --- Misc handlers (Bloco 3d parity) ---
	cardapioHandler := handler.NewCardapioHandler(mongoClient)

	// === OUTBOUND DOMAIN ===
	// Outbound Transfers (organizer withdrawals + voting approval) — Java parity.
	// Provider is chosen via ROLE_OUTBOUND_PROVIDER: "asaas" (real) or "noop" (default).
	outboundRequestRepo := mongodb.NewOutboundRequestRepository(mongoClient)
	outboundAuditLogRepo := mongodb.NewOutboundAuditLogRepository(mongoClient)

	var outboundTransferProvider portout.OutboundTransferProvider
	switch strings.ToLower(os.Getenv("ROLE_OUTBOUND_PROVIDER")) {
	case "asaas":
		outboundTransferProvider = ucasaas.NewOutboundTransferClient(cfg.Asaas)
	default:
		outboundTransferProvider = ucasaas.NewNoopOutboundTransferProvider()
	}

	createOutboundUC := ucoutbound.NewCreateOutboundRequest(
		outboundRequestRepo, eventoRepo, participantRepo,
		financeSummaryRepo, paymentAccountRepo,
		outboundTransferProvider, outboundAuditLogRepo,
	)
	listOutboundUC := ucoutbound.NewListOutboundRequests(outboundRequestRepo, eventoRepo, participantRepo)
	getOutboundUC := ucoutbound.NewGetOutboundRequest(outboundRequestRepo, eventoRepo, participantRepo)
	getOutboundDetailsUC := ucoutbound.NewGetOutboundRequestDetails(outboundRequestRepo, eventoRepo, participantRepo)
	approveOutboundUC := ucoutbound.NewApproveOutboundRequest(
		outboundRequestRepo, eventoRepo, participantRepo,
		outboundTransferProvider, outboundAuditLogRepo,
	).WithTemporalStarter(temporalStarter)
	rejectOutboundUC := ucoutbound.NewRejectOutboundRequest(
		outboundRequestRepo, eventoRepo, participantRepo, outboundAuditLogRepo,
	)
	cancelOutboundUC := ucoutbound.NewCancelOutboundRequest(outboundRequestRepo, outboundAuditLogRepo)
	voteOutboundUC := ucoutbound.NewVoteOnOutboundRequest(
		outboundRequestRepo, eventoRepo, participantRepo,
		outboundTransferProvider, outboundAuditLogRepo,
	)
	handleOutboundCallbackUC := ucoutbound.NewHandleOutboundTransferCallback(outboundRequestRepo, outboundAuditLogRepo)

	outboundRequestHandler := handler.NewOutboundRequestHandler(
		createOutboundUC, listOutboundUC, getOutboundUC, getOutboundDetailsUC,
		approveOutboundUC, rejectOutboundUC, cancelOutboundUC, voteOutboundUC,
	)
	outboundWebhookHandler := handler.NewOutboundWebhookHandler(
		handleOutboundCallbackUC, webhookRepo, cfg.Asaas.WebhookToken,
	)

	// === ADMIN SURFACE (hexagonal) ===
	// Trilha C: 9 Java admin controllers migrated to hexagonal use cases.
	// Repos reused: dominioRepo, usuarioRepo, eventoRepo, financeSummaryRepo,
	// outboundRequestRepo. New repos: admin metrics, feature flags, approvals,
	// reconciliation report reader.
	adminMetricsRepo := mongodb.NewAdminMetricsRepository(mongoClient)
	featureFlagRepo := mongodb.NewFeatureFlagRepository(mongoClient)
	approvalRepo := mongodb.NewApprovalRepository(mongoClient)
	reconReportReader := mongodb.NewReconciliationReportRepository(mongoClient)

	addUserRoleUC := ucauth.NewAddUserRole(usuarioRepo)
	removeUserRoleUC := ucauth.NewRemoveUserRole(usuarioRepo)

	adminHandler := handler.NewAdminHandler(handler.AdminHandlerDeps{
		Stats:           ucadmin.NewGetDashboardStats(adminMetricsRepo),
		Health:          ucadmin.NewGetDashboardHealth(adminMetricsRepo),
		Finance:         ucadmin.NewGetDashboardFinance(adminMetricsRepo),
		PendingOutbound: ucadmin.NewGetPendingOutbound(eventoRepo, outboundRequestRepo),
		ListFlags:       ucadmin.NewListFeatureFlags(featureFlagRepo),
		UpdateFlag:      ucadmin.NewUpdateFeatureFlag(featureFlagRepo),
		GetDominio:      ucadmin.NewGetDominioByID(dominioRepo),
		ToggleDominio:   ucadmin.NewToggleDominio(dominioRepo),
		ListCategorias:  ucadmin.NewListDominioCategorias(dominioRepo),
		ListPolicies:    ucadmin.NewListCancelamentoPolicies(dominioRepo),
		UpdateTiers:     ucadmin.NewUpdateCancelamentoTiers(dominioRepo),
		ListEventos:     ucadmin.NewListEventosAdmin(eventoRepo),
		Completo:        ucadmin.NewGetEventoCompletoAdmin(eventoRepo, usuarioRepo, financeSummaryRepo, outboundRequestRepo),
		Cancelar:        ucadmin.NewCancelarEventoAdmin(eventoRepo),
		Fechar:          ucadmin.NewFecharFinanceiroAdmin(eventoRepo),
		AddRole:         addUserRoleUC,
		RemoveRole:      removeUserRoleUC,
		ListReports:     ucadmin.NewListReconciliationReports(reconReportReader),
		LatestReport:    ucadmin.NewGetLatestReconciliationReport(reconReportReader),
	})
	approvalsHandler := handler.NewApprovalsHandler(
		ucadmin.NewCountPendingApprovals(approvalRepo),
		ucadmin.NewListPendingApprovals(approvalRepo),
		ucadmin.NewListApprovalHistory(approvalRepo),
	)

	// --- Phase 9: Temporal Workers ---
	if cfg.Temporal.WorkerEnabled {
		temporalClient, err := temporaladapter.NewClient(cfg.Temporal)
		if err != nil {
			slog.Error("failed to connect to Temporal", "error", err)
			os.Exit(1)
		}
		defer temporalClient.Close()
		// Expose the client to the native workflow status/signal handler.
		temporalQueryClient = temporalClient

		temporalRegistry := temporalworker.NewRegistry(temporalClient)

		// ── Onda 3/4 native workflows ─────────────────────────────────────────
		// 1. Participant lifecycle.
		participantCancelUC := ucparticipant.NewCancelParticipantInstallments(installmentRepo, eventoRepo)
		participantRecalcUC := ucparticipant.NewRecalculateRateioAllocations(rateioRepo, eventoRepo)
		participantLifecycleActs := temporalactivity.NewParticipantLifecycleActivities(participantCancelUC, participantRecalcUC)
		temporalRegistry.RegisterParticipantLifecycleWorker(participantLifecycleActs)

		// 2. Invite lifecycle (expiration auto-declines via RecusarConvite).
		inviteLifecycleActs := temporalactivity.NewInviteLifecycleActivities(recusarConviteUC)
		temporalRegistry.RegisterInviteLifecycleWorker(inviteLifecycleActs)

		// 3. Outbound execution.
		outboundExecActs := temporalactivity.NewOutboundExecutionActivities(approveOutboundUC, handleOutboundCallbackUC)
		temporalRegistry.RegisterOutboundExecutionWorker(outboundExecActs)

		// 4. Event lifecycle.
		eventLifecycleActs := temporalactivity.NewEventLifecycleActivities(alterarFaseUC)
		temporalRegistry.RegisterEventLifecycleWorker(eventLifecycleActs)

		// 5. Event publication monitoring (stuck-execution scan).
		findStuckUC := ucevent.NewFindStuckExecutions(txRepo)
		eventPubMonitoringActs := temporalactivity.NewEventPublicationMonitoringActivities(findStuckUC)
		temporalRegistry.RegisterEventPublicationMonitoringWorker(eventPubMonitoringActs)

		// 6. Event publication execution.
		eventPubExecActs := temporalactivity.NewEventPublicationExecutionActivities(publishDraftUC)
		temporalRegistry.RegisterEventPublicationExecutionWorker(eventPubExecActs)

		if err := temporalRegistry.Start(); err != nil {
			slog.Error("failed to start Temporal workers", "error", err)
			os.Exit(1)
		}
		defer temporalRegistry.Stop()
	}

	// Native Temporal workflow handler (serves the 6 migrated workflows via the Go
	// SDK; falls back to the Java proxy when the client is nil or lookup fails).
	workflowNativeHandler := handler.NewWorkflowNativeHandler(temporalQueryClient, workflowProxyHandler)

	// Build chi router.
	r := chi.NewRouter()

	// --- Global middleware (applied to every request) ---
	// OTel deve ser o primeiro para criar o span raiz antes que os demais middlewares executem.
	if cfg.OTel.Enabled {
		r.Use(otelhttp.NewMiddleware(cfg.OTel.ServiceName))
	}
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.ErrorHandler)
	r.Use(middleware.StructuredLogger(logger))
	r.Use(middleware.Metrics)
	r.Use(middleware.CORS(cfg.Server.CORSOrigins))
	r.Use(chimiddleware.Recoverer)

	// --- Health endpoint (public, no auth required) ---
	r.Get("/actuator/health", handler.HealthHandler(mongoClient))

	// --- Public routes (no JWT required) ---
	authHandler.RegisterRoutes(r)
	configHandler.RegisterRoutes(r)          // GET /api/v1/dominios (public read) + admin write
	cardapioHandler.RegisterCardapioRoutes(r) // GET /api/cardapios (public — Java parity)
	paymentWebhookHandler.RegisterWebhookRoutes(r) // POST /api/v1/webhooks/payment/asaas (no JWT)
	whatsappWebhookHandler.RegisterWebhookRoutes(r) // POST + GET /api/v1/webhooks/notifications/whatsapp (no JWT)
	eventoHandler.RegisterPublicEventRoutes(r) // CSE_014 — GET /api/v1/eventos/{eventId}/public-info (no JWT)
	guestHandler.RegisterGuestRoutes(r)              // Java parity: GuestController is fully public
	biometricHandler.RegisterPublicRoutes(r)          // challenge / authenticate / status (public)
	conviteHandler.RegisterPublicConviteRoutes(r)  // GET /{id}, /registration-data, POST /confirmar, /recusar
	outboundWebhookHandler.RegisterOutboundWebhookRoutes(r) // POST /api/v1/webhooks/outbound/asaas (no JWT)

	// --- Protected routes (JWT required) ---
	r.Group(func(r chi.Router) {
		r.Use(middleware.JWTAuth(jwtSvc, publicPrefixes))
		usuarioHandler.RegisterRoutes(r)
		eventoHandler.RegisterEventRoutes(r)
		draftHandler.RegisterDraftRoutes(r)
		rateioHandler.RegisterRateioRoutes(r)
		paymentHandler.RegisterPaymentRoutes(r)
		notificationHandler.RegisterNotificationRoutes(r)
		notifTemplateHandler.RegisterNotificationTemplateRoutes(r)
		storageHandler.RegisterStorageRoutes(r)
		workflowNativeHandler.RegisterWorkflowRoutes(r)
		workflowProxyHandler.RegisterAdminWorkflowRoutes(r)
		cofrinhoHandler.RegisterCofrinhoRoutes(r)
		listaPresentesHandler.RegisterListaPresentesRoutes(r)
		socialHandler.RegisterSocialRoutes(r)
		financeHandler.RegisterFinanceRoutes(r)
		paymentMethodsHandler.RegisterPaymentMethodsRoutes(r)
		installmentHandler.RegisterInstallmentRoutes(r)
		adminHandler.RegisterAdminRoutes(r)
		participantesHandler.RegisterParticipantesRoutes(r)
		usuariosEventoHandler.RegisterUsuariosEventoRoutes(r)
		approvalsHandler.RegisterApprovalsRoutes(r)
		outboundRequestHandler.RegisterOutboundRequestRoutes(r)
		conviteHandler.RegisterProtectedConviteRoutes(r)
	})

	// --- Biometric protected routes (own JWT middleware — sits under /api/auth/* which
	// the global JWTAuth treats as public). Must be registered AFTER the public auth
	// handler to keep more specific paths matched first by chi.
	biometricHandler.RegisterProtectedRoutes(r, jwtSvc) // register / devices / revoke (JWT)

	// --- HTTP server ---
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in background.
	go func() {
		slog.Info("http server listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http server error", "error", err)
			os.Exit(1)
		}
	}()

	// --- Graceful shutdown ---
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down gracefully", "timeout", cfg.Server.ShutdownTimeout)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
	}

	if err := mongoClient.Disconnect(shutdownCtx); err != nil {
		slog.Error("mongodb disconnect error", "error", err)
	}

	slog.Info("server exited")
}
