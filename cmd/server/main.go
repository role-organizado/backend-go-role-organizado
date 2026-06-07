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
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/handler"
	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/middleware"
	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/mongodb"
	temporaladapter "github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal"
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
	// Phase 5
	ucpayment "github.com/role-organizado/backend-go-role-organizado/internal/usecase/payment"
	ucasaas "github.com/role-organizado/backend-go-role-organizado/internal/infra/asaas"
	paymentdomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	// Phase 5b: Finance hexagonal
	ucfinance "github.com/role-organizado/backend-go-role-organizado/internal/usecase/finance"
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

	var logHandler slog.Handler = jsonHandler

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
		logHandler = pkgotel.NewTeeHandler(providers.LoggerProvider, jsonHandler)
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

	notifTemplateHandler := handler.NewNotificationTemplateHandler(
		createTemplateUC, getTemplateUC, listTemplatesUC, updateTemplateUC, deleteTemplateUC,
		renderTemplateUC, testSendTemplateUC, getByTypeTemplateUC, listCategoriaTemplateUC,
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

	// --- Phase 8: Temporal Worker (payment workflows) ---
	// Enabled via TEMPORAL_WORKER_ENABLED=true (default: false during Strangler Fig migration).
	// When disabled, no-op starters/signalers are used so the payment use cases compile and
	// run correctly without a live Temporal cluster.
	var temporalStarter portout.TemporalWorkflowStarter = &temporaladapter.NoopWorkflowStarter{}
	var temporalSignaler portout.TemporalWorkflowSignaler = &temporaladapter.NoopWorkflowSignaler{}

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

		pspReviewUC := ucpricing.NewRunPspCostReview(cfg, &http.Client{Timeout: 5 * time.Minute})
		pspReviewActivity := temporalactivity.NewPricingPspReviewActivity(pspReviewUC)
		temporalRegistry.RegisterPricingPspReviewWorker(pspReviewActivity)

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

	// --- Phase 8b: Temporal Workflow Proxies (Java fallback) ---
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

	// --- Admin, Participantes handlers (direct MongoDB) ---
	adminHandler := handler.NewAdminHandler(mongoClient)
	participantesHandler := handler.NewParticipantesHandler(mongoClient)
	usuariosEventoHandler := handler.NewUsuariosEventoHandler(mongoClient)
	approvalsHandler := handler.NewApprovalsHandler(mongoClient)

	// --- Misc handlers (Bloco 3d parity) ---
	cardapioHandler := handler.NewCardapioHandler(mongoClient)
	outboundRequestHandler := handler.NewOutboundRequestHandler(mongoClient)

	// --- Phase 9: Temporal Workers ---
	temporalClient, err := temporaladapter.NewClient(cfg.Temporal)
	if err != nil {
		slog.Error("failed to connect to Temporal", "error", err)
		os.Exit(1)
	}
	defer temporalClient.Close()
	temporalRegistry := temporalworker.NewRegistry(temporalClient)
	// Workers are registered per migration wave (T003, T004, T005, T006).
	if err := temporalRegistry.Start(); err != nil {
		slog.Error("failed to start Temporal workers", "error", err)
		os.Exit(1)
	}
	defer temporalRegistry.Stop()

	// Build chi router.
	r := chi.NewRouter()

	// --- Global middleware (applied to every request) ---
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
		workflowProxyHandler.RegisterWorkflowRoutes(r)
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
	})

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
