package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/agentcapability"
	"github.com/lease-management-system/core-service/internal/agentreaders"
	"github.com/lease-management-system/core-service/internal/agenttools"
	agenttooldefs "github.com/lease-management-system/core-service/internal/agenttools/tools"
	"github.com/lease-management-system/core-service/internal/aiagent"
	"github.com/lease-management-system/core-service/internal/config"
	"github.com/lease-management-system/core-service/internal/contextassembler"
	"github.com/lease-management-system/core-service/internal/db"
	"github.com/lease-management-system/core-service/internal/docparse"
	"github.com/lease-management-system/core-service/internal/gateway"
	"github.com/lease-management-system/core-service/internal/handlers"
	"github.com/lease-management-system/core-service/internal/llm"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/miniostore"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/scheduler"
	"github.com/lease-management-system/core-service/internal/services/agentguard"
	"github.com/lease-management-system/core-service/internal/services/audit"
	"github.com/lease-management-system/core-service/internal/services/closecontrol"
	"github.com/lease-management-system/core-service/internal/services/closereadiness"
	"github.com/lease-management-system/core-service/internal/services/draftapp"
	"github.com/lease-management-system/core-service/internal/services/eventaccounting"
	"github.com/lease-management-system/core-service/internal/services/monthend"
	"github.com/lease-management-system/core-service/internal/services/reporting"
	"github.com/lease-management-system/core-service/internal/sessionmanager"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database
	database, err := db.New(cfg.DatabaseURL())
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	log.Println("Connected to PostgreSQL")

	// Initialize repositories
	userRepo := repository.NewUserRepository(database.Pool)
	contractRepo := repository.NewContractRepository(database.Pool)
	roleRepo := repository.NewRoleRepository(database.Pool)
	authRefreshRepo := repository.NewAuthRefreshRepository(database.Pool)
	approvalRepo := repository.NewApprovalRepository(database.Pool)
	psRepo := repository.NewPaymentScheduleRepository(database.Pool)
	eventRepo := repository.NewEventRepository(database.Pool)
	mcRepo := repository.NewMonthlyClosingRepository(database.Pool)
	auditRepo := repository.NewAuditRepository(database.Pool)
	systemSettingRepo := repository.NewSystemSettingRepository(database.Pool)
	leaseAdminRepo := repository.NewLeaseAdminRepository(database.Pool)
	aiChatRuntimeRepo := repository.NewAIChatRuntimeRepository(database.Pool)
	aiRunQueueRepo := repository.NewAgentRunQueueRepository(database.Pool)
	masterDataRepo := repository.NewMasterDataRepository(database.Pool)
	accessPolicyRepo := repository.NewAccessPolicyRepository(database.Pool)
	exchangeRateRepo := repository.NewExchangeRateRepository(database.Pool)
	workQueueRepo := repository.NewWorkQueueRepository(database.Pool)
	closeReadinessRepo := repository.NewCloseReadinessRepository(database.Pool)
	closeControlRepo := repository.NewCloseControlRepository(database.Pool)
	budgetRepo := repository.NewBudgetRepository(database.Pool)
	storeMetricsRepo := repository.NewStoreMetricsRepository(database.Pool)
	renewalDecisionRepo := repository.NewRenewalDecisionRepository(database.Pool)
	operatingFactsRepo := repository.NewOperatingFactsRepository(database.Pool)
	retailSimulationRepo := repository.NewRetailSimulationRepository(database.Pool)
	retailKPIRepo := repository.NewRetailKPIRepository(database.Pool)
	fpnaGovernanceRepo := repository.NewFPnAGovernanceRepository(database.Pool)

	// Initialize audit logger
	auditLogger := audit.NewLogger(auditRepo)

	// Initialize services
	closeService := monthend.NewService(database.Pool, mcRepo, contractRepo, psRepo, systemSettingRepo, exchangeRateRepo, masterDataRepo, auditLogger)
	closeReadinessService := closereadiness.NewService(closeReadinessRepo, systemSettingRepo, closeControlRepo)
	closeControlService := closecontrol.NewService(closeReadinessRepo, systemSettingRepo, closeControlRepo)
	eventPersistence := eventaccounting.NewPersistenceService(database.Pool, mcRepo, eventRepo, auditLogger)
	draftService := draftapp.NewPostgresService(database.Pool, contractRepo, psRepo, eventRepo)
	// Ch2 草稿复核工作台：读走 contractRepo（Reader），写复用 draftapp 的
	// 幂等 UoW（agent_draft_idempotency + lease_contracts 同一事务）。
	draftReviewUOW := handlers.DraftReviewUOW{Inner: draftapp.NewPostgresUnitOfWork(database.Pool, contractRepo, psRepo, eventRepo)}
	draftReviewHandler := handlers.NewContractDraftReviewHandler(contractRepo, draftReviewUOW)
	capabilityIssuer, err := agentcapability.NewIssuer(cfg.AgentCapabilitySecret, "lease-agent-gateway", time.Duration(cfg.AgentCapabilityTTLSeconds)*time.Second)
	if err != nil {
		log.Fatalf("Failed to initialize agent capability issuer: %v", err)
	}
	capabilityStore := agentcapability.NewPostgresStore(database.Pool)
	capabilityIssuer = capabilityIssuer.WithRevocationStore(capabilityStore)
	// RT1-L3-C: all in-process scheduled jobs live in internal/scheduler —
	// the single home that replaced three same-shaped hand-written tickers
	// (lease recovery + capability cleanup + auth-refresh cleanup). Type A
	// (system maintenance) jobs produce no runs and call no tools, so they
	// never cross the governance chain; the package is stdlib-only by import
	// guard. Lease recovery closes the L3-B registered debt: before this job
	// existed, a dead worker's runs stayed leased until expiry with nothing
	// to requeue them.
	maintenanceJobs := []scheduler.Registration{
		scheduler.LeaseRecovery(aiRunQueueRepo, time.Duration(cfg.AgentRunLeaseRecoverySeconds)*time.Second),
		{
			Name:     "agent-capability-cleanup",
			Interval: time.Duration(cfg.AgentCapabilityCleanupSeconds) * time.Second,
			Auth:     scheduler.AuthSystemMaintenance,
			Run: func(ctx context.Context) error {
				deleted, err := capabilityStore.CleanupExpired(ctx, time.Now().UTC())
				if err != nil {
					return err
				}
				stats, err := capabilityStore.Stats(ctx, time.Now().UTC())
				if err != nil {
					return err
				}
				log.Printf("Agent capability maintenance: deleted=%d active=%d revoked=%d expired=%d", deleted, stats.Active, stats.Revoked, stats.Expired)
				return nil
			},
		},
		{
			Name:     "auth-refresh-cleanup",
			Interval: time.Duration(cfg.RefreshTokenCleanupSeconds) * time.Second,
			Auth:     scheduler.AuthSystemMaintenance,
			Run: func(ctx context.Context) error {
				deleted, err := authRefreshRepo.CleanupExpired(ctx, time.Now().UTC())
				if err != nil {
					return err
				}
				log.Printf("Auth refresh session maintenance: deleted=%d", deleted)
				return nil
			},
		},
	}
	maintenanceScheduler, schedulerErr := scheduler.New(maintenanceJobs)
	if schedulerErr != nil {
		log.Fatalf("Failed to initialize scheduler: %v", schedulerErr)
	}
	schedulerCtx, stopScheduler := context.WithCancel(context.Background())
	maintenanceScheduler.Start(schedulerCtx)

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(cfg, userRepo, roleRepo).WithRefreshTokenStore(authRefreshRepo)
	contractHandler := handlers.NewContractHandler(contractRepo, auditLogger)
	calcHandler := handlers.NewCalculationHandler(contractRepo, psRepo, systemSettingRepo)
	approvalHandler := handlers.NewApprovalHandler(approvalRepo, contractRepo, auditLogger)
	psHandler := handlers.NewPaymentScheduleHandler(psRepo, contractRepo)
	dealCompareHandler := handlers.NewDealCompareHandler()
	preDealHandler := handlers.NewPreDealHandler()
	cashflowScenarioHandler := handlers.NewCashflowScenarioHandler(contractRepo, psRepo)
	renewalCardHandler := handlers.NewRenewalCardHandler(contractRepo, psRepo, storeMetricsRepo, mcRepo, renewalDecisionRepo, auditLogger, systemSettingRepo)
	reportHandler := handlers.NewReportHandler(contractRepo, psRepo, mcRepo, systemSettingRepo, masterDataRepo, closeControlRepo)
	eventHandler := handlers.NewEventHandler(eventRepo, contractRepo, mcRepo, psRepo, systemSettingRepo, eventPersistence, auditLogger)
	monthlyClosingHandler := handlers.NewMonthlyClosingHandler(mcRepo, contractRepo, closeService, closeReadinessService, auditLogger)
	closeExceptionHandler := handlers.NewCloseExceptionHandler(closeControlService, auditLogger)
	controlReaders := &agenttooldefs.ControlReaders{
		Budget:   agentreaders.NewBudgetVarianceReader(budgetRepo, systemSettingRepo),
		Cashflow: agentreaders.NewCashflowScenarioReader(contractRepo, psRepo),
		Renewal:  agentreaders.NewRenewalDecisionReader(contractRepo, renewalDecisionRepo),
	}
	// B-4：报表快照构建器只建一次，HTTP 报表与 Agent 工具共用同一条投影路径。
	reportSnapshotBuilder := reporting.NewSnapshotBuilder(contractRepo, psRepo, systemSettingRepo, mcRepo).WithStores(masterDataRepo)
	sensitivityReader := agenttooldefs.NewReportingSensitivityReader(reportSnapshotBuilder)
	minioClient, minioErr := miniostore.New(miniostore.Config{
		Endpoint: cfg.MinIOEndpoint, AccessKey: cfg.MinIOAccessKey, SecretKey: cfg.MinIOSecretKey, Bucket: cfg.MinIOBucket,
	})
	if minioErr != nil {
		log.Fatalf("Failed to build MinIO client: %v", minioErr)
	}
	var fillReader agenttooldefs.IngestFileReader = miniostore.NewIngestReader(minioClient)
	finModelRepo := repository.NewFinModelRepository(database.Pool)
	// B-1: the Agent Tool seam rides the same S1 adapters as /stores/:id/pnl;
	// built here (before the Agent handler) so fpna.store_pnl.read is registered in
	// the production runtime exactly when the HTTP projection is wired.
	storePnlKPIAdapter := handlers.NewStorePnlKPIAdapter(retailKPIRepo)
	storePnlHandler := handlers.NewStorePnlHandler(storePnlKPIAdapter, nil, fpnaGovernanceRepo).WithPeer(storePnlKPIAdapter).WithMasterData(masterDataRepo).WithOccupancy(handlers.NewStorePnlOccupancyAdapter(psRepo)).WithLease(handlers.NewStorePnlLeaseAdapter(mcRepo)).WithTemplates(finModelRepo)
	storePnlAgentReader := handlers.NewStorePnlAgentReader(storePnlHandler)
	// SI1 Part B: 生产 chat 的会话创建/加载经过 sessionmanager（AR2）——
	// 独占租约让同一会话的两条消息串行。gateway/runner 平面仍挂 G9。
	sessionManager := sessionmanager.New(sessionmanager.NewPostgresStore(database.Pool), sessionmanager.Policy{})
	// C1（架构重构任务书 2026-08-26）：构造参数归组为 AgentPorts，每个仓库只传一次。
	// 旧位置参数签名曾让 retailKPIRepo / fpnaGovernanceRepo / operatingFactsRepo 各传两遍。
	aiChatHandler := handlers.NewAIChatHandler(aiChatRuntimeRepo, contractRepo, aiagent.AgentPorts{
		Contracts:       contractRepo,
		Closing:         mcRepo,
		Events:          eventRepo,
		DraftServices:   []*draftapp.Service{draftService},
		FileIngest:      fillReader,
		CloseReadiness:  closeReadinessService,
		Reports:         agenttooldefs.NewReportSnapshotReader(reportSnapshotBuilder, fpnaGovernanceRepo, operatingFactsRepo, closeControlRepo, mcRepo),
		RateSensitivity: sensitivityReader,
		Governance:      fpnaGovernanceRepo,
		Plans:           fpnaGovernanceRepo,
		FinModelRepo:    finModelRepo,
		Controls:        controlReaders,
		StorePnl:        storePnlAgentReader,
		OperatingFacts:  operatingFactsRepo,
		RetailKPI:       retailKPIRepo,
	}).WithSessionOwner(sessionManager).WithAuditRepository(auditRepo).WithWorkerRunStore(aiRunQueueRepo).WithGuard(agentguard.New(repository.NewAgentUsageStore(database.Pool, 12, 2.0), agentguard.Config{}))
	// W5-1: document parser seam (ADR-0024 routing). AnyDoc binary is pinned
	// in the runtime image (Dockerfile); OCR is enabled when a PaddleOCR token is configured.
	ocrParser := docparse.NewPaddleOCR(docparse.PaddleOCRConfig{
		APIURL: os.Getenv("PADDLEOCR_API_URL"), AccessToken: os.Getenv("PADDLEOCR_ACCESS_TOKEN"), Model: os.Getenv("PADDLEOCR_MODEL"),
	})
	docParser := docparse.NewRouter(docparse.CSV(), docparse.AnyDoc(cfg.AnyDocBinPath, 60*time.Second), ocrParser, docparse.OCREnabled(ocrParser))
	aiChatHandler.SetDocumentParser(docParser)
	// RT1-L3-D: if an MCP manifest is configured, register its tools into the
	// shared runtime registry BEFORE any traffic is served. Fail-fast: a
	// manifest that cannot fully register (unreachable server, missing tool,
	// non-read entry) refuses startup — honest absence, never a silent skip.
	if cfg.MCPManifestPath != "" {
		if err := aiChatHandler.RegisterMCPTools(cfg.MCPManifestPath); err != nil {
			log.Fatalf("Failed to register MCP tools: %v", err)
		}
	}
	// W5-3: the intake parse endpoints read uploaded files through MinIO.
	aiChatHandler.SetFileBytesReader(func(ctx context.Context, objectName string) ([]byte, error) {
		return minioClient.GetObject(ctx, objectName)
	})
	uploadHandler := handlers.UploadAIFile(minioClient)
	auditHandler := handlers.NewAuditHandler(auditRepo)

	// AR3 wiring (RT1-A two-level switch, D-C10): CONTEXT_ASSEMBLER_MODE is
	// off (default) / count / on. off keeps the legacy path byte-for-byte;
	// count runs the budget geometry and reports occupancy but never compacts
	// (production traffic data before the first behavior change); on adds
	// compaction. The old CONTEXT_ASSEMBLER_ENABLED=true maps to on.
	contextMode := contextassembler.ParseMode(
		os.Getenv("CONTEXT_ASSEMBLER_MODE"),
	)
	if os.Getenv("CONTEXT_ASSEMBLER_ENABLED") == "true" && contextMode == contextassembler.ModeOff {
		contextMode = contextassembler.ModeOn
	}
	if contextMode != contextassembler.ModeOff {
		specs := contextassembler.BudgetSpecsFromEnv(os.Getenv)
		// AF1-c fail-fast: a model actually running MUST have budget geometry —
		// in count mode too, because occupancy needs a denominator (the budget).
		if err := contextassembler.ValidateModelCoverage(specs, llm.ConfigFromEnv().Model); err != nil {
			log.Fatalf("context assembler mode %q: %v", contextMode, err)
		}
		assembler := contextassembler.NewAssembler(
			contextassembler.NewPgHistorySource(database.Pool),
			contextassembler.WithMode(contextMode),
		)
		for model, spec := range specs {
			if err := contextassembler.RegisterBudget(assembler, model, spec); err != nil {
				log.Fatalf("Invalid context budget for %q: %v", model, err)
			}
		}
		contextMetrics := agenttools.NewContextMetrics()
		aiChatHandler.SetContextAssembler(assembler)
		aiChatHandler.WithContextMetrics(contextMetrics)
		log.Printf("AR3 context assembler enabled (mode %s)", contextMode)
	}
	settingsHandler := handlers.NewSettingsHandler(systemSettingRepo)
	leaseAdminHandler := handlers.NewLeaseAdminHandler(leaseAdminRepo, contractRepo, auditLogger)
	masterDataHandler := handlers.NewMasterDataHandler(masterDataRepo)
	exchangeRateHandler := handlers.NewExchangeRateHandler(exchangeRateRepo, auditLogger)
	workQueueHandler := handlers.NewWorkQueueHandler(workQueueRepo)
	budgetHandler := handlers.NewBudgetHandler(budgetRepo, contractRepo, psRepo, systemSettingRepo)
	storeMetricsHandler := handlers.NewStoreMetricsHandler(storeMetricsRepo, auditLogger, systemSettingRepo)
	operatingFactsHandler := handlers.NewOperatingFactsHandler(operatingFactsRepo, auditLogger, fpnaGovernanceRepo)
	retailStoreDayFactsHandler := handlers.NewRetailStoreDayFactsHandler(operatingFactsRepo, auditLogger)
	finModelHandler := handlers.NewFinModelHandlerWithAudit(finModelRepo, auditLogger).WithExchangeRates(exchangeRateRepo).WithPlanGovernance(fpnaGovernanceRepo).WithFacts(retailKPIRepo).WithProductionSources(mcRepo, operatingFactsRepo)
	savedViewHandler := handlers.NewSavedViewHandler(finModelRepo)
	retailIngestHandler := handlers.NewRetailIngestHandler(retailKPIRepo, operatingFactsRepo, auditLogger)
	retailExportHandler := handlers.NewRetailExportHandler()
	retailSimulationHandler := handlers.NewRetailSimulationHandler(retailSimulationRepo, auditLogger)
	retailKPIHandler := handlers.NewRetailKPIHandler(retailKPIRepo)
	planReader := handlers.NewRetailPlanReader(fpnaGovernanceRepo)
	planMateriality := func(ctx context.Context) float64 {
		return systemSettingRepo.GetFloat64(ctx, "retail_plan_variance_materiality_pct", 5)
	}
	retailPulseHandler := handlers.NewRetailPulseHandler(retailKPIRepo).WithPlanReader(planReader).WithPlanMateriality(planMateriality)
	retailStoreDiagnosticsHandler := handlers.NewRetailStoreDiagnosticsHandler(retailKPIRepo).WithPlanReader(planReader).WithPlanMateriality(planMateriality)
	retailScenarioHandler := handlers.NewRetailScenarioHandler(retailKPIRepo, operatingFactsRepo)
	fpnaGovernanceHandler := handlers.NewFPnAGovernanceHandler(fpnaGovernanceRepo, operatingFactsRepo, auditLogger).WithExchangeRateRepo(exchangeRateRepo)
	cashPlanRepo := repository.NewCashPlanRepository(database.Pool)
	cashPlanHandler := handlers.NewCashPlanHandler(cashPlanRepo)
	categoryRepo := repository.NewCategoryRepository(database.Pool)
	categoryHandler := handlers.NewCategoryHandler(categoryRepo)
	promotionRepo := repository.NewPromotionRepository(database.Pool)
	promotionHandler := handlers.NewPromotionHandler(promotionRepo)
	varianceAttributionHandler := handlers.NewVarianceAttributionHandler(retailKPIRepo)
	newStoreFeasibilityHandler := handlers.NewNewStoreFeasibilityHandler(database.Pool)
	machineCredRepo := repository.NewMachineCredentialRepository(database.Pool)
	sourceFeedHandler := handlers.NewSourceFeedHandler(machineCredRepo, retailKPIRepo, operatingFactsRepo, auditLogger)
	inventoryRepo := repository.NewInventoryRepository(database.Pool)
	inventoryMetricsHandler := handlers.NewInventoryMetricsHandler(inventoryRepo, retailKPIRepo)
	masterDataResolutionHandler := handlers.NewMasterDataResolutionHandler(database.Pool)
	competitorRepo := repository.NewCompetitorRepository(database.Pool)
	competitorHandler := handlers.NewCompetitorObservationsHandler(competitorRepo)
	exchangeRateVersionHandler := handlers.NewExchangeRateVersionHandler(exchangeRateRepo)
	fpnaPlanImportHandler := handlers.NewFPnAPlanImportHandler(retailKPIRepo, fpnaGovernanceRepo)
	trialBalanceHandler := handlers.NewTrialBalanceHandler(operatingFactsRepo)
	decisionScenarioHandler := handlers.NewDecisionScenarioHandler(draftService)
	gatewayAudit := handlers.NewAgentToolAuditRecorder(auditLogger)
	// RT1-D-1: gateway 工具执行走新链（governance.Assembly 九控制），经
	// AgentGovernedToolRuntime（共享 chat 的 registry，只换 guard）。
	agentGatewayHandler := handlers.NewAgentGatewayHandler(aiChatHandler.AgentGovernedToolRuntime(), gatewayAudit).WithCapabilityIssuer(capabilityIssuer).WithSkillRegistry(aiChatHandler.AgentSkillRegistry()).WithSessionStore(aiChatHandler.AgentSessionStore()).WithContractScopeReader(contractRepo).WithRunStore(aiChatHandler.AgentRunStore()).WithCheckpointStore(aiChatHandler.AgentRunCheckpointStore()).WithQueueStore(aiRunQueueRepo).WithWorkerRunStore(aiRunQueueRepo).WithTerminalAlertStore(aiChatRuntimeRepo).WithUsageStore(aiChatRuntimeRepo).WithContextMetrics(aiChatHandler.ContextMetrics()).WithSessionOwner(aiChatHandler.SessionOwner())

	if cfg.LogLevel == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(corsMiddleware())

	// RT1-L3-B: honest health surface. The previous inline handler answered
	// 200 + "status":"ok" even when the postgres probe failed and pasted the
	// raw error into the body — a control that existed but could never be
	// false. /live is liveness (dependency-blind: restarting cannot heal
	// postgres); /ready gates on postgres+minio; /health keeps its URL for
	// existing callers but now carries the readiness contract. LLM provider
	// state comes from cached real-call outcomes (never actively probed) and
	// does not gate.
	healthHandler := handlers.NewHealthHandler(database.HealthCheck)
	if minioClient != nil {
		healthHandler = healthHandler.WithMinioProbe(minioClient.HealthCheck)
	}
	r.GET("/health", healthHandler.Ready)
	r.GET("/live", healthHandler.Live)
	r.GET("/ready", healthHandler.Ready)

	// Public routes - registration disabled, only login is public
	r.POST("/api/v1/auth/register", authHandler.Register)
	r.POST("/api/v1/auth/login", authHandler.Login)
	r.POST("/api/v1/auth/refresh", authHandler.Refresh)
	r.POST("/api/v1/auth/logout", authHandler.Logout)
	r.POST("/api/v1/retail/push/facts", sourceFeedHandler.PushFacts)

	// Protected routes
	api := r.Group("/api/v1")
	api.Use(middleware.JWTAuth(cfg.JWTSecret))
	api.Use(middleware.LoadUserPermissions(roleRepo))
	api.Use(middleware.DataScopeMiddleware())
	api.Use(middleware.TenantMiddleware())
	api.Use(middleware.RequireTenant())
	{
		protected := middleware.NewProtectedRouter(api)
		permission := func(resource, action string) middleware.Permission {
			return middleware.Permission{Resource: resource, Action: action}
		}
		contractScope := middleware.RequireContractScope(contractRepo, "id")
		contractApprovalSeparation := middleware.RequireApprovalSeparation(accessPolicyRepo, "contract", "id")
		eventApprovalSeparation := middleware.RequireApprovalSeparation(accessPolicyRepo, "event", "eventId")
		entryApprovalSeparation := middleware.RequireApprovalSeparation(accessPolicyRepo, "journal_entry", "id")
		batchApprovalSeparation := middleware.RequireApprovalSeparation(accessPolicyRepo, "monthly_batch", "id")
		templateApprovalSeparation := middleware.RequireApprovalSeparation(accessPolicyRepo, "statement_template", "id")
		runPublishSeparation := middleware.RequireApprovalSeparation(accessPolicyRepo, "fin_model_run", "id")
		protected.Handle(http.MethodGet, "/me", permission("identity", "read"), handlers.GetCurrentUser())
		protected.Handle(http.MethodGet, "/auth/sessions", permission("identity", "read"), authHandler.ListSessions)
		protected.Handle(http.MethodDelete, "/auth/sessions/:id", permission("identity", "read"), authHandler.RevokeSession)
		protected.Handle(http.MethodPost, "/auth/logout-all", permission("identity", "read"), authHandler.LogoutAll)
		protected.Handle(http.MethodGet, "/me/work-queue", permission("identity", "read"), workQueueHandler.Get)

		// Contracts
		protected.Handle(http.MethodPost, "/contracts", permission("contracts", "create"), contractHandler.Create)
		protected.Handle(http.MethodPost, "/contracts/batch", permission("contracts", "create"), contractHandler.CreateBatch)
		protected.Handle(http.MethodGet, "/contracts", permission("contracts", "read"), contractHandler.GetAll)
		protected.Handle(http.MethodGet, "/contracts/:id", permission("contracts", "read"), contractScope, contractHandler.GetByID)
		protected.Handle(http.MethodPut, "/contracts/:id", permission("contracts", "update"), contractScope, contractHandler.Update)

		// Calculations
		protected.Handle(http.MethodPost, "/contracts/:id/calculate", permission("calculations", "trigger"), contractScope, calcHandler.Calculate)
		protected.Handle(http.MethodGet, "/contracts/:id/schedule", permission("calculations", "read"), contractScope, calcHandler.GetAmortizationSchedule)

		// Approval workflow
		protected.Handle(http.MethodPost, "/contracts/:id/submit", permission("contracts", "submit"), contractScope, approvalHandler.SubmitForReview)
		protected.Handle(http.MethodPost, "/contracts/:id/review", permission("contracts", "review"), contractScope, approvalHandler.Review)
		protected.Handle(http.MethodPost, "/contracts/:id/approve", permission("contracts", "approve"), contractScope, contractApprovalSeparation, approvalHandler.Approve)
		protected.Handle(http.MethodPost, "/contracts/:id/reject", permission("contracts", "approve"), contractScope, contractApprovalSeparation, approvalHandler.Reject)
		protected.Handle(http.MethodGet, "/contracts/:id/approval-status", permission("contracts", "read"), contractScope, approvalHandler.GetStatus)
		protected.Handle(http.MethodGet, "/contracts/:id/renewal-card", permission("reports", "read"), contractScope, renewalCardHandler.Card)
		protected.Handle(http.MethodPost, "/contracts/:id/renewal-decisions", permission("renewal_decisions", "write"), contractScope, renewalCardHandler.CreateDecision)
		protected.Handle(http.MethodGet, "/contracts/:id/renewal-decisions", permission("reports", "read"), contractScope, renewalCardHandler.ListDecisions)
		protected.Handle(http.MethodGet, "/contracts-by-status", permission("contracts", "read"), approvalHandler.ListByStatus)

		// Ch2 草稿复核工作台：草稿的生命周期长于会话，按草稿自身的状态与归属组织（D-B6）。
		protected.Handle(http.MethodGet, "/contracts/drafts", permission("contracts", "read"), draftReviewHandler.ListDrafts)
		protected.Handle(http.MethodPost, "/contracts/drafts/decide", permission("contracts", "approve"), draftReviewHandler.DecideDrafts)
		protected.Handle(http.MethodGet, "/contracts/drafts/:id", permission("contracts", "read"), draftReviewHandler.GetDraft)
		protected.Handle(http.MethodPut, "/contracts/drafts/:id", permission("contracts", "update"), draftReviewHandler.ReviseDraft)

		// Discount Rate
		protected.Handle(http.MethodGet, "/contracts/:id/discount-rate-status", permission("calculations", "read"), contractScope, handlers.CheckDiscountRate(contractRepo))
		protected.Handle(http.MethodPost, "/contracts/:id/confirm-discount-rate", permission("calculations", "trigger"), contractScope, handlers.ConfirmDiscountRate(contractRepo, auditLogger))

		// Payment Schedules
		protected.Handle(http.MethodPost, "/contracts/:id/payment-schedules", permission("payment_schedules", "create"), contractScope, psHandler.Create)
		protected.Handle(http.MethodGet, "/contracts/:id/payment-schedules", permission("payment_schedules", "read"), contractScope, psHandler.ListByContract)

		// Events
		protected.Handle(http.MethodPost, "/contracts/:id/events", permission("events", "create"), contractScope, eventHandler.Create)
		protected.Handle(http.MethodGet, "/contracts/:id/events", permission("events", "read"), contractScope, eventHandler.ListByContract)
		// Deriving a draft writes nothing, so it needs only read permission.
		protected.Handle(http.MethodPost, "/contracts/:id/events/preview-payments", permission("events", "read"), contractScope, eventHandler.PreviewRevisedPayments)

		// Event approval workflow
		protected.Handle(http.MethodPost, "/contracts/:id/events/:eventId/submit", permission("events", "submit"), contractScope, eventHandler.SubmitForReview)
		protected.Handle(http.MethodPost, "/contracts/:id/events/:eventId/review", permission("events", "review"), contractScope, eventHandler.Review)
		protected.Handle(http.MethodPost, "/contracts/:id/events/:eventId/approve", permission("events", "approve"), contractScope, eventApprovalSeparation, eventHandler.Approve)
		protected.Handle(http.MethodPost, "/contracts/:id/events/:eventId/reject", permission("events", "approve"), contractScope, eventApprovalSeparation, eventHandler.Reject)

		// Event IFRS 16 recalculation
		protected.Handle(http.MethodPost, "/contracts/:id/events/:eventId/recalculate", permission("calculations", "trigger"), contractScope, eventHandler.RecalculateEvent)
		protected.Handle(http.MethodPost, "/contracts/:id/events/:eventId/preview", permission("calculations", "read"), contractScope, eventHandler.PreviewEventAdjustment)
		protected.Handle(http.MethodGet, "/contracts/:id/events/:eventId/adjustment", permission("calculations", "read"), contractScope, eventHandler.GetEventAdjustment)

		// Lease administration
		protected.Handle(http.MethodGet, "/lease-admin/critical-dates/upcoming", permission("lease_admin", "read"), leaseAdminHandler.ListUpcomingCriticalDates)
		protected.Handle(http.MethodPost, "/contracts/:id/critical-dates", permission("lease_admin", "create"), contractScope, leaseAdminHandler.CreateCriticalDate)
		protected.Handle(http.MethodGet, "/contracts/:id/critical-dates", permission("lease_admin", "read"), contractScope, leaseAdminHandler.ListCriticalDates)
		protected.Handle(http.MethodPatch, "/contracts/:id/critical-dates/:dateId/status", permission("lease_admin", "update"), contractScope, leaseAdminHandler.UpdateCriticalDateStatus)
		protected.Handle(http.MethodPost, "/contracts/:id/documents", permission("lease_admin", "create"), contractScope, leaseAdminHandler.CreateDocument)
		protected.Handle(http.MethodGet, "/contracts/:id/documents", permission("lease_admin", "read"), contractScope, leaseAdminHandler.ListDocuments)
		protected.Handle(http.MethodPost, "/contracts/:id/obligations", permission("lease_admin", "create"), contractScope, leaseAdminHandler.CreateObligation)
		protected.Handle(http.MethodGet, "/contracts/:id/obligations", permission("lease_admin", "read"), contractScope, leaseAdminHandler.ListObligations)
		protected.Handle(http.MethodPatch, "/contracts/:id/obligations/:obligationId/status", permission("lease_admin", "update"), contractScope, leaseAdminHandler.UpdateObligationStatus)

		// Reports
		protected.Handle(http.MethodGet, "/reports/liability-rolling", permission("reports", "read"), reportHandler.LiabilityRolling)
		protected.Handle(http.MethodGet, "/reports/liability-rolling/export", permission("reports", "export"), reportHandler.ExportLiabilityRolling)
		protected.Handle(http.MethodGet, "/reports/contract-summary", permission("reports", "read"), reportHandler.ContractSummary)
		protected.Handle(http.MethodGet, "/reports/portfolio-summary", permission("reports", "read"), reportHandler.PortfolioSummary)
		protected.Handle(http.MethodGet, "/reports/sensitivity", permission("reports", "read"), reportHandler.SensitivityAnalysis)

		// Store revenue is commercially sensitive, so writing it needs master
		// data rights rather than report rights, and every read goes through
		// the caller's brand and region slice.
		protected.Handle(http.MethodPost, "/store-metrics", permission("master_data", "manage"), storeMetricsHandler.Upsert)
		protected.Handle(http.MethodGet, "/store-metrics", permission("reports", "read"), storeMetricsHandler.List)
		protected.Handle(http.MethodGet, "/reports/rent-to-sales", permission("reports", "read"), storeMetricsHandler.RentToSales)
		// Operating facts and the unified decision surface. Operating data is
		// versioned and remains separate from lease contracts and official close.
		protected.Handle(http.MethodGet, "/performance/overview", permission("reports", "read"), operatingFactsHandler.Overview)
		protected.Handle(http.MethodGet, "/performance/brief", permission("reports", "read"), operatingFactsHandler.ManagementBrief)
		protected.Handle(http.MethodGet, "/performance/actions", permission("reports", "read"), operatingFactsHandler.ListActions)
		protected.Handle(http.MethodGet, "/performance/actions/:id/realizations", permission("reports", "read"), fpnaGovernanceHandler.ListRealizations)
		protected.Handle(http.MethodPost, "/performance/actions/:id/realizations", permission("fpna_actions", "write"), fpnaGovernanceHandler.CreateRealization)
		protected.Handle(http.MethodGet, "/performance/assumptions", permission("reports", "read"), operatingFactsHandler.ListAssumptions)
		protected.Handle(http.MethodPost, "/performance/assumptions", permission("fpna_actions", "write"), operatingFactsHandler.CreateAssumption)
		protected.Handle(http.MethodPost, "/performance/actions", permission("fpna_actions", "write"), operatingFactsHandler.CreateAction)
		protected.Handle(http.MethodPost, "/performance/actions/bulk", permission("fpna_actions", "write"), operatingFactsHandler.BulkUpdateActions)
		protected.Handle(http.MethodGet, "/performance/actions/export", permission("reports", "export"), operatingFactsHandler.ExportActions)
		protected.Handle(http.MethodPatch, "/performance/actions/:id", permission("fpna_actions", "write"), operatingFactsHandler.UpdateAction)
		protected.Handle(http.MethodPost, "/operating-facts/stores", permission("master_data", "manage"), operatingFactsHandler.UpsertStores)
		protected.Handle(http.MethodPost, "/operating-facts/stores/import", permission("master_data", "manage"), operatingFactsHandler.ImportStoresCSV)
		protected.Handle(http.MethodPost, "/operating-facts/stores/import-xlsx", permission("master_data", "manage"), operatingFactsHandler.ImportStoresXLSX)
		protected.Handle(http.MethodGet, "/operating-facts/stores/template", permission("reports", "read"), operatingFactsHandler.StoreCSVTemplate)
		protected.Handle(http.MethodGet, "/operating-facts/stores", permission("reports", "read"), operatingFactsHandler.ListStores)
		protected.Handle(http.MethodPost, "/retail/operating-facts/store-days", permission("master_data", "manage"), retailStoreDayFactsHandler.Upsert)
		protected.Handle(http.MethodPost, "/retail/operating-facts/store-days/import/preview", permission("master_data", "manage"), retailIngestHandler.Preview)
		protected.Handle(http.MethodPost, "/retail/operating-facts/store-days/import/commit", permission("master_data", "manage"), retailIngestHandler.Commit)
		protected.Handle(http.MethodGet, "/retail/operating-facts/store-days", permission("reports", "read"), retailStoreDayFactsHandler.List)
		protected.Handle(http.MethodGet, "/stores/:id/pnl", permission("reports", "read"), storePnlHandler.Projection)
		protected.Handle(http.MethodGet, "/store-pnl/aggregate", permission("reports", "read"), storePnlHandler.AggregateProjection)
		protected.Handle(http.MethodPost, "/financial-model/templates", permission("statement_templates", "write"), finModelHandler.CreateTemplate)
		protected.Handle(http.MethodPost, "/financial-model/templates/validate", permission("statement_templates", "read"), finModelHandler.ValidateTemplate)
		protected.Handle(http.MethodGet, "/financial-model/templates", permission("statement_templates", "read"), finModelHandler.ListTemplates)
		protected.Handle(http.MethodPost, "/financial-model/templates/:id/review", permission("statement_templates", "review"), finModelHandler.ReviewTemplate)
		protected.Handle(http.MethodPost, "/financial-model/templates/:id/approve", permission("statement_templates", "approve"), templateApprovalSeparation, finModelHandler.ApproveTemplate)
		protected.Handle(http.MethodPost, "/financial-model/templates/:id/copy", permission("statement_templates", "write"), finModelHandler.CopyTemplate)
		protected.Handle(http.MethodDelete, "/financial-model/templates/:id", permission("statement_templates", "write"), finModelHandler.DeleteTemplate)
		protected.Handle(http.MethodPost, "/financial-model/opening/validate", permission("fin_models", "write"), finModelHandler.ValidateOpening)
		protected.Handle(http.MethodPost, "/financial-model/definitions/:id/runs", permission("fin_models", "write"), finModelHandler.RunDefinition)
		protected.Handle(http.MethodPost, "/financial-model/runs/:id/publish", permission("fin_models", "approve"), runPublishSeparation, finModelHandler.PublishRun)
		protected.Handle(http.MethodGet, "/financial-model/runs/:id/export", permission("fin_models", "read"), finModelHandler.ExportRun)
		protected.Handle(http.MethodGet, "/financial-model/runs/:id", permission("fin_models", "read"), finModelHandler.GetRun)
		protected.Handle(http.MethodPost, "/financial-model/runs/:id/cancel", permission("fin_models", "write"), finModelHandler.CancelRun)
		protected.Handle(http.MethodGet, "/financial-model/definitions", permission("fin_models", "write"), finModelHandler.ListDefinitions)
		protected.Handle(http.MethodGet, "/financial-model/group", permission("reports", "read"), finModelHandler.GroupRuns)
		protected.Handle(http.MethodGet, "/financial-model/saved-views", permission("fin_views", "read"), savedViewHandler.List)
		protected.Handle(http.MethodPost, "/financial-model/saved-views", permission("fin_views", "write"), savedViewHandler.Create)
		protected.Handle(http.MethodPut, "/financial-model/saved-views/:id", permission("fin_views", "write"), savedViewHandler.Update)
		protected.Handle(http.MethodDelete, "/financial-model/saved-views/:id", permission("fin_views", "write"), savedViewHandler.Delete)
		protected.Handle(http.MethodPost, "/financial-model/saved-views/:id/default", permission("fin_views", "write"), savedViewHandler.SetDefault)
		protected.Handle(http.MethodPost, "/financial-model/saved-views/:id/share", permission("fin_views", "share"), savedViewHandler.Share)
		protected.Handle(http.MethodGet, "/retail/kpis/definitions", permission("reports", "read"), retailKPIHandler.Definitions)
		protected.Handle(http.MethodGet, "/retail/exports/descriptors", permission("reports", "read"), retailExportHandler.Descriptors)
		protected.Handle(http.MethodGet, "/retail/kpis/store-days", permission("reports", "read"), retailKPIHandler.StoreDays)
		protected.Handle(http.MethodGet, "/retail/operating-pulse", permission("reports", "read"), retailPulseHandler.OperatingPulse)
		protected.Handle(http.MethodGet, "/retail/store-options", permission("reports", "read"), retailStoreDiagnosticsHandler.StoreOptions)
		protected.Handle(http.MethodGet, "/retail/stores/:store_id/diagnostics", permission("reports", "read"), retailStoreDiagnosticsHandler.Diagnostics)
		protected.Handle(http.MethodGet, "/retail/stores/:store_id/pl-flow", permission("reports", "read"), retailStoreDiagnosticsHandler.PlFlow)
		protected.Handle(http.MethodPost, "/retail/stores/:store_id/scenarios/evaluate", permission("reports", "read"), retailScenarioHandler.Evaluate)
		protected.Handle(http.MethodPost, "/retail/stores/:store_id/scenario-action-drafts", permission("fpna_actions", "write"), retailScenarioHandler.SaveAction)
		protected.Handle(http.MethodPost, "/retail/simulations/store-days/generate", permission("master_data", "manage"), retailSimulationHandler.GenerateStoreDays)
		protected.Handle(http.MethodGet, "/retail/simulations/store-days/latest", permission("reports", "read"), retailSimulationHandler.LatestStoreDays)
		protected.Handle(http.MethodGet, "/operating-facts/batches", permission("reports", "read"), operatingFactsHandler.ListBatches)
		protected.Handle(http.MethodGet, "/reports/store-performance", permission("reports", "read"), operatingFactsHandler.StorePerformance)
		protected.Handle(http.MethodGet, "/reports/store-performance/benchmarks", permission("reports", "read"), operatingFactsHandler.StoreBenchmarks)
		protected.Handle(http.MethodGet, "/reports/store-performance/cohorts", permission("reports", "read"), operatingFactsHandler.StoreCohorts)
		protected.Handle(http.MethodPost, "/reports/store-promotion-roi", permission("reports", "read"), operatingFactsHandler.StorePromotionROI)
		protected.Handle(http.MethodPost, "/operating-facts/equipment", permission("master_data", "manage"), operatingFactsHandler.UpsertEquipment)
		protected.Handle(http.MethodGet, "/operating-facts/equipment", permission("reports", "read"), operatingFactsHandler.ListEquipment)
		protected.Handle(http.MethodPost, "/operating-facts/equipment-facts", permission("master_data", "manage"), operatingFactsHandler.UpsertEquipmentFact)
		protected.Handle(http.MethodGet, "/reports/equipment-performance", permission("reports", "read"), operatingFactsHandler.EquipmentPerformance)
		protected.Handle(http.MethodGet, "/reports/equipment-candidates", permission("reports", "read"), operatingFactsHandler.EquipmentCandidates)
		protected.Handle(http.MethodPost, "/reports/store-decision-scenario", permission("reports", "read"), decisionScenarioHandler.Store)
		protected.Handle(http.MethodPost, "/reports/store-decision-event-draft", permission("events", "create"), decisionScenarioHandler.StoreDecisionEventDraft)
		protected.Handle(http.MethodPost, "/reports/equipment-decision-scenario", permission("reports", "read"), decisionScenarioHandler.Equipment)
		// Governed FP&A plan versions, effective-dated mappings, data quality,
		// report artifacts and decision memos.  Frozen/Official transitions are
		// explicit and never overwrite an earlier version.
		protected.Handle(http.MethodGet, "/performance/plan-versions", permission("reports", "read"), fpnaGovernanceHandler.ListPlanVersions)
		protected.Handle(http.MethodPost, "/performance/plan-versions", permission("fpna_actions", "write"), fpnaGovernanceHandler.CreatePlanVersion)
		protected.Handle(http.MethodPost, "/performance/plan-versions/:id/freeze", permission("fpna_actions", "write"), fpnaGovernanceHandler.FreezePlanVersion)
		protected.Handle(http.MethodGet, "/performance/plan-versions/compare", permission("reports", "read"), fpnaGovernanceHandler.ComparePlanVersions)
		protected.Handle(http.MethodGet, "/performance/forecast-accuracy", permission("reports", "read"), fpnaGovernanceHandler.ForecastAccuracy)
		protected.Handle(http.MethodGet, "/performance/forecast-accuracy/trend", permission("reports", "read"), fpnaGovernanceHandler.ForecastAccuracyTrend)
		protected.Handle(http.MethodPost, "/performance/forecast/hybrid", permission("fpna_actions", "write"), fpnaGovernanceHandler.HybridForecast)
		protected.Handle(http.MethodGet, "/performance/mappings", permission("reports", "read"), fpnaGovernanceHandler.ListMappings)
		protected.Handle(http.MethodPost, "/performance/mappings", permission("fpna_mappings", "write"), fpnaGovernanceHandler.CreateMapping)
		protected.Handle(http.MethodGet, "/performance/metrics", permission("reports", "read"), fpnaGovernanceHandler.ListMetricDefinitions)
		protected.Handle(http.MethodPost, "/performance/metrics", permission("fpna_mappings", "write"), fpnaGovernanceHandler.CreateMetricDefinition)
		protected.Handle(http.MethodGet, "/performance/agent-signals", permission("reports", "read"), fpnaGovernanceHandler.ListAgentSignals)
		protected.Handle(http.MethodPost, "/performance/agent-signals", permission("fpna_actions", "write"), fpnaGovernanceHandler.CreateAgentSignal)
		protected.Handle(http.MethodGet, "/performance/data-quality", permission("fpna_data_quality", "read"), fpnaGovernanceHandler.ListDataQuality)
		protected.Handle(http.MethodPost, "/performance/data-quality", permission("fpna_data_quality", "write"), fpnaGovernanceHandler.CreateDataQuality)
		protected.Handle(http.MethodPatch, "/performance/data-quality/:id/status", permission("fpna_data_quality", "write"), fpnaGovernanceHandler.UpdateDataQualityStatus)
		protected.Handle(http.MethodGet, "/performance/decision-memos", permission("fpna_memos", "read"), fpnaGovernanceHandler.ListMemos)
		protected.Handle(http.MethodPost, "/performance/decision-memos", permission("fpna_memos", "write"), fpnaGovernanceHandler.CreateMemo)
		protected.Handle(http.MethodPatch, "/performance/decision-memos/:id/status", permission("fpna_memos", "write"), fpnaGovernanceHandler.UpdateMemoStatus)
		protected.Handle(http.MethodGet, "/performance/report-packs", permission("fpna_reports", "read"), fpnaGovernanceHandler.ListReportPacks)
		protected.Handle(http.MethodPost, "/performance/report-packs", permission("fpna_reports", "write"), fpnaGovernanceHandler.GenerateReportPack)
		protected.Handle(http.MethodGet, "/performance/report-packs/:id/download", permission("fpna_reports", "read"), fpnaGovernanceHandler.DownloadReportPack)

		// Offer comparison reads nothing and writes nothing: the terms come in
		// with the request. It sits behind report permission because it is an
		// analysis tool, not because it touches report data.
		protected.Handle(http.MethodPost, "/deals/compare", permission("reports", "read"), dealCompareHandler.Compare)
		protected.Handle(http.MethodPost, "/deals/briefing", permission("reports", "read"), preDealHandler.Briefing)
		protected.Handle(http.MethodPost, "/reports/cashflow-scenario", permission("reports", "read"), cashflowScenarioHandler.Scenario)
		protected.Handle(http.MethodGet, "/reports/standard-comparison", permission("reports", "read"), reportHandler.StandardComparison)
		protected.Handle(http.MethodGet, "/reports/amortization", permission("reports", "read"), reportHandler.Amortization)
		protected.Handle(http.MethodGet, "/reports/tags", permission("reports", "read"), reportHandler.Tags)
		protected.Handle(http.MethodGet, "/reports/tags/summary", permission("reports", "read"), reportHandler.TagSummary)
		protected.Handle(http.MethodGet, "/reports/cashflow-forecast", permission("reports", "read"), reportHandler.CashflowForecast)
		protected.Handle(http.MethodGet, "/reports/disclosure", permission("reports", "read"), reportHandler.Disclosure)
		protected.Handle(http.MethodGet, "/reports/close-pack", permission("reports", "read"), reportHandler.ClosePack)
		protected.Handle(http.MethodGet, "/reports/close-pack/export", permission("reports", "export"), reportHandler.ExportClosePack)
		protected.Handle(http.MethodGet, "/reports/unit-price", permission("reports", "read"), reportHandler.UnitPrice)

		// Exchange rates: settings-grade master data used to translate
		// foreign-currency leases into the entity's functional currency.
		protected.Handle(http.MethodGet, "/exchange-rates", permission("settings", "read"), exchangeRateHandler.List)
		protected.Handle(http.MethodPost, "/exchange-rates", permission("settings", "update"), exchangeRateHandler.Upsert)
		protected.Handle(http.MethodGet, "/exchange-rates/versions", permission("settings", "read"), exchangeRateVersionHandler.ListVersions)
		protected.Handle(http.MethodPost, "/exchange-rates/versions", permission("settings", "update"), exchangeRateVersionHandler.CreateVersion)
		protected.Handle(http.MethodPost, "/cashflow/plan/compose", permission("reports", "read"), cashPlanHandler.Compose)
		protected.Handle(http.MethodGet, "/retail/categories", permission("master_data", "read"), categoryHandler.ListCategories)
		protected.Handle(http.MethodPost, "/retail/categories", permission("master_data", "manage"), categoryHandler.CreateCategory)
		protected.Handle(http.MethodGet, "/retail/store-day-category-facts", permission("operating_facts", "read"), categoryHandler.ListCategoryFacts)
		protected.Handle(http.MethodPost, "/retail/category-facts/reconcile", permission("operating_facts", "read"), categoryHandler.ReconcileCategoryFacts)
		protected.Handle(http.MethodPost, "/retail/margin-decomposition", permission("operating_facts", "read"), categoryHandler.GetMarginDecomposition)
		protected.Handle(http.MethodGet, "/retail/promotions", permission("operating_facts", "read"), promotionHandler.ListPromotions)
		protected.Handle(http.MethodPost, "/retail/promotions", permission("operating_facts", "write"), promotionHandler.CreatePromotion)
		protected.Handle(http.MethodGet, "/retail/promotions/:id", permission("operating_facts", "read"), promotionHandler.GetPromotion)
		protected.Handle(http.MethodPut, "/retail/promotions/:id", permission("operating_facts", "write"), promotionHandler.UpdatePromotion)
		protected.Handle(http.MethodGet, "/retail/promotions/:id/costs", permission("operating_facts", "read"), promotionHandler.ListPromotionCosts)
		protected.Handle(http.MethodPost, "/retail/promotions/:id/costs", permission("operating_facts", "write"), promotionHandler.AddPromotionCost)
		protected.Handle(http.MethodGet, "/retail/promotions/:id/roi", permission("operating_facts", "read"), promotionHandler.EvaluateROI)
		protected.Handle(http.MethodPost, "/retail/promotions/breakeven", permission("operating_facts", "read"), promotionHandler.EvaluateBreakeven)
		protected.Handle(http.MethodGet, "/retail/store-variance-attribution", permission("operating_facts", "read"), varianceAttributionHandler.StoreVarianceAttribution)
		protected.Handle(http.MethodPost, "/retail/new-store-feasibility", permission("operating_facts", "read"), newStoreFeasibilityHandler.Evaluate)
		protected.Handle(http.MethodGet, "/retail/inventory/summary", permission("operating_facts", "read"), inventoryMetricsHandler.GetInventorySummary)
		protected.Handle(http.MethodPost, "/retail/inventory/facts", permission("operating_facts", "write"), inventoryMetricsHandler.UpsertInventoryFact)
		protected.Handle(http.MethodPost, "/retail/master-data/resolve", permission("master_data", "read"), masterDataResolutionHandler.Resolve)
		protected.Handle(http.MethodPost, "/retail/master-data/confirm-mapping", permission("master_data", "manage"), masterDataResolutionHandler.ConfirmMapping)
		protected.Handle(http.MethodGet, "/retail/competitor-observations", permission("operating_facts", "read"), competitorHandler.ListObservations)
		protected.Handle(http.MethodPost, "/retail/competitor-observations", permission("operating_facts", "write"), competitorHandler.AddObservation)

		// Budget versions freeze the measured forward schedule so later actuals
		// can be explained against a stable plan.
		protected.Handle(http.MethodGet, "/budget-versions", permission("reports", "read"), budgetHandler.ListVersions)
		protected.Handle(http.MethodPost, "/fpna/plan-versions/import", permission("master_data", "manage"), fpnaPlanImportHandler.Import)
		protected.Handle(http.MethodPost, "/gl/trial-balances/import", permission("master_data", "manage"), trialBalanceHandler.Import)
		protected.Handle(http.MethodGet, "/gl/trial-balances", permission("reports", "read"), trialBalanceHandler.List)
		protected.Handle(http.MethodPost, "/budget-versions", permission("reports", "read"), budgetHandler.CreateVersion)
		protected.Handle(http.MethodGet, "/budget-versions/compare", permission("reports", "read"), budgetHandler.CompareVersions)
		protected.Handle(http.MethodGet, "/budget-versions/management-brief", permission("reports", "read"), budgetHandler.ManagementBrief)
		protected.Handle(http.MethodGet, "/budget-versions/:id/variance", permission("reports", "read"), budgetHandler.Variance)
		protected.Handle(http.MethodPut, "/budget-versions/:id/variance-actions", permission("variance_actions", "write"), budgetHandler.VarianceActions)

		// Monthly Closing
		protected.Handle(http.MethodPost, "/monthly-closing/generate", permission("monthly_closing", "generate"), monthlyClosingHandler.Generate)
		protected.Handle(http.MethodGet, "/monthly-closing/batches", permission("monthly_closing", "read"), monthlyClosingHandler.ListBatches)
		protected.Handle(http.MethodGet, "/monthly-closing/readiness", permission("monthly_closing", "read"), monthlyClosingHandler.Readiness)
		protected.Handle(http.MethodGet, "/monthly-closing/entries", permission("monthly_closing", "read"), monthlyClosingHandler.GetJournalEntries)
		protected.Handle(http.MethodGet, "/monthly-closing/periods", permission("monthly_closing", "read"), monthlyClosingHandler.ListEntryPeriods)
		protected.Handle(http.MethodGet, "/contracts/:id/measurement-results", permission("calculations", "read"), contractScope, monthlyClosingHandler.GetMeasurementResults)

		// Monthly Closing - Approval & Posting
		protected.Handle(http.MethodPost, "/monthly-closing/entries/:id/approve", permission("monthly_closing", "approve"), entryApprovalSeparation, monthlyClosingHandler.ApproveEntry)
		protected.Handle(http.MethodPost, "/monthly-closing/entries/:id/post", permission("monthly_closing", "post"), monthlyClosingHandler.PostEntry)
		protected.Handle(http.MethodPost, "/monthly-closing/entries/:id/reject", permission("monthly_closing", "approve"), entryApprovalSeparation, monthlyClosingHandler.RejectEntry)
		protected.Handle(http.MethodPost, "/monthly-closing/entries/:id/reverse", permission("monthly_closing", "reverse"), entryApprovalSeparation, monthlyClosingHandler.ReverseEntry)
		protected.Handle(http.MethodGet, "/monthly-closing/entries/export", permission("monthly_closing", "export"), monthlyClosingHandler.ExportJournalEntries)
		protected.Handle(http.MethodPost, "/monthly-closing/erp-writeback", permission("monthly_closing", "writeback"), monthlyClosingHandler.ApplyERPWriteback)
		protected.Handle(http.MethodPost, "/monthly-closing/batches/:id/approve", permission("monthly_closing", "approve"), batchApprovalSeparation, monthlyClosingHandler.ApproveBatch)
		protected.Handle(http.MethodPost, "/monthly-closing/batches/:id/post", permission("monthly_closing", "post"), monthlyClosingHandler.PostBatch)

		// Monthly Closing - Period Locking
		protected.Handle(http.MethodPost, "/monthly-closing/periods/:period/lock", permission("monthly_closing", "lock"), middleware.RequireLegalEntityWideScope(), monthlyClosingHandler.LockPeriod)
		protected.Handle(http.MethodPost, "/monthly-closing/periods/:period/unlock", permission("monthly_closing", "unlock"), middleware.RequireLegalEntityWideScope(), monthlyClosingHandler.UnlockPeriod)
		protected.Handle(http.MethodGet, "/monthly-closing/periods/:period/lock-status", permission("monthly_closing", "read"), monthlyClosingHandler.GetPeriodLockStatus)

		// Close Exception Governance
		protected.Handle(http.MethodGet, "/monthly-closing/periods/:period/exceptions", permission("monthly_closing", "exception_read"), closeExceptionHandler.List)
		protected.Handle(http.MethodPost, "/monthly-closing/periods/:period/exceptions/detect", permission("monthly_closing", "exception_detect"), middleware.RequireLegalEntityWideScope(), closeExceptionHandler.Detect)
		protected.Handle(http.MethodPost, "/close-exceptions/:id/actions", permission("monthly_closing", "exception_manage"), middleware.RequireLegalEntityWideScope(), closeExceptionHandler.ApplyAction)

		// AI Chat
		protected.Handle(http.MethodPost, "/ai/chat", permission("ai_chat", "use"), aiChatHandler.Chat)
		protected.Handle(http.MethodPost, "/ai/chat/sessions", permission("ai_chat", "use"), aiChatHandler.CreateSession)
		protected.Handle(http.MethodGet, "/ai/chat/sessions", permission("ai_chat", "use"), aiChatHandler.ListSessions)
		protected.Handle(http.MethodGet, "/ai/chat/sessions/:id", permission("ai_chat", "use"), aiChatHandler.GetSession)
		protected.Handle(http.MethodPost, "/ai/chat/sessions/:id/runs", permission("ai_chat", "use"), aiChatHandler.CreateRun)
		protected.Handle(http.MethodGet, "/ai/chat/sessions/:id/runs", permission("ai_chat", "use"), aiChatHandler.ListRuns)
		protected.Handle(http.MethodPost, "/ai/chat/continuations", permission("ai_chat", "use"), aiChatHandler.CreateContinuation)
		protected.Handle(http.MethodGet, "/ai/chat/draft-batches/:id", permission("ai_chat", "use"), aiChatHandler.GetDraftBatch)
		protected.Handle(http.MethodPost, "/ai/chat/draft-batches/:id/retry", permission("ai_chat", "use"), aiChatHandler.RetryDraftBatch)
		protected.Handle(http.MethodGet, "/ai/chat/runs/:id/events", permission("ai_chat", "use"), aiChatHandler.ListRunEvents)
		protected.Handle(http.MethodGet, "/ai/chat/runs/:id/trace", permission("ai_chat", "use"), aiChatHandler.GetAgentRunTrace)
		protected.Handle(http.MethodGet, "/ai/chat/runs/:id/stream", permission("ai_chat", "use"), aiChatHandler.StreamRunEvents)
		protected.Handle(http.MethodPost, "/ai/chat/artifacts/:id/actions", permission("ai_chat", "use"), aiChatHandler.CreateReviewAction)
		protected.Handle(http.MethodGet, "/ai/chat/artifacts/:id", permission("ai_chat", "use"), aiChatHandler.GetArtifact)
		protected.Handle(http.MethodGet, "/ai/chat/artifacts/:id/export", permission("ai_chat", "use"), aiChatHandler.ExportArtifact)
		// W5-5: file upload moved to core-service (MinIO write seam). The
		// front-end posts /api/ai/files/upload which now lands here.
		protected.Handle(http.MethodPost, "/ai/files/upload", permission("ai_chat", "use"), uploadHandler)

		// Agent Gateway: individual Tools enforce their own permissions. Do not
		// put a broad ai_chat permission in front of this group, otherwise CLI
		// and external Agent callers could not use permitted read Tools such as
		// contracts:read or calculations:read.
		api.GET("/agent/tools", agentGatewayHandler.Describe)
		api.GET("/agent/skills", agentGatewayHandler.Skills)
		api.GET("/agent/metrics/prometheus", agentGatewayHandler.MetricsPrometheus)
		api.GET("/agent/usage", agentGatewayHandler.Usage)
		api.POST("/agent/sessions", agentGatewayHandler.CreateSession)
		api.POST("/agent/tools/execute", agentGatewayHandler.Execute)
		api.POST("/agent/capabilities", agentGatewayHandler.IssueCapability)
		api.POST("/agent/capabilities/revoke", agentGatewayHandler.RevokeCapability)
		api.POST("/agent/runs", agentGatewayHandler.CreateRun)
		api.POST("/agent/runs/claim", agentGatewayHandler.ClaimRun)
		api.POST("/agent/runs/recover-leases", agentGatewayHandler.RecoverRunLeases)
		api.GET("/agent/runs/:id/events", agentGatewayHandler.ListRunEvents)
		protected.Handle(http.MethodGet, "/agent/runs/:id/trace", permission("ai_chat", "use"), aiChatHandler.GetAgentRunTrace)
		api.GET("/agent/runs/:id/checkpoint", agentGatewayHandler.GetRunCheckpoint)
		api.GET("/agent/runs/:id/stream", aiChatHandler.StreamRunEvents)
		api.GET("/agent/alerts/terminal", agentGatewayHandler.ListTerminalAlerts)
		api.POST("/agent/alerts/terminal/:id/ack", agentGatewayHandler.AcknowledgeTerminalAlert)
		api.POST("/agent/runs/:id/events", agentGatewayHandler.AppendRunEvent)
		api.POST("/agent/runs/:id/checkpoint", agentGatewayHandler.SaveRunCheckpoint)
		api.POST("/agent/runs/:id/cancel", agentGatewayHandler.CancelRun)
		api.POST("/agent/runs/:id/steer", agentGatewayHandler.SteerRun)
		api.POST("/agent/runs/:id/follow-up", agentGatewayHandler.FollowUpRun)
		api.POST("/agent/runs/:id/branch", agentGatewayHandler.BranchRun)
		api.POST("/agent/runs/:id/lease/heartbeat", agentGatewayHandler.HeartbeatRunLease)
		api.POST("/agent/runs/:id/lease/release", agentGatewayHandler.ReleaseRunLease)
		api.POST("/agent/artifacts/:id/actions", aiChatHandler.CreateReviewAction)

		// Audit Logs
		protected.Handle(http.MethodGet, "/audit-logs", permission("audit_logs", "read"), auditHandler.List)

		// Admin: user management
		protected.Handle(http.MethodGet, "/admin/users", permission("users", "read"), authHandler.AdminListUsers)
		protected.Handle(http.MethodPost, "/admin/users", permission("users", "create"), authHandler.AdminCreateUser)

		// Roles & Permissions
		protected.Handle(http.MethodGet, "/roles", permission("roles", "read"), handlers.ListRoles(roleRepo))
		protected.Handle(http.MethodGet, "/my-permissions", permission("identity", "read"), handlers.GetMyPermissions(roleRepo))

		// Master data
		protected.Handle(http.MethodGet, "/master-data/legal-entities", permission("master_data", "read"), masterDataHandler.ListLegalEntities)
		protected.Handle(http.MethodGet, "/master-data/stores", permission("master_data", "read"), masterDataHandler.ListStores)
		protected.Handle(http.MethodGet, "/master-data/landlords", permission("master_data", "read"), masterDataHandler.ListLandlords)

		// Global Settings
		protected.Handle(http.MethodGet, "/settings/global", permission("settings", "read"), settingsHandler.GetGlobal)
		protected.Handle(http.MethodPut, "/settings/global", permission("settings", "update"), settingsHandler.UpdateGlobal)
		protected.Handle(http.MethodGet, "/admin/machine-credentials", permission("settings", "read"), sourceFeedHandler.ListMachineCredentials)
		protected.Handle(http.MethodPost, "/admin/machine-credentials", permission("settings", "update"), sourceFeedHandler.IssueMachineCredential)
		protected.Handle(http.MethodPost, "/admin/machine-credentials/:id/revoke", permission("settings", "update"), sourceFeedHandler.RevokeMachineCredential)
	}

	// Ch3 IM 网关：接线但默认关（ADR-0026 §6）。Enabled=false 时 Start 是
	// no-op 且零渠道连接；渠道凭据缺失时该渠道被跳过并记录具名原因。
	gatewayRunner := gateway.NewRunner(gateway.GatewaySettingsFromConfig(cfg.Gateway), gateway.NewIdentityResolver(
		repository.NewChannelIdentityBindingRepository(database.Pool),
		userRepo,
		roleRepo,
	), gateway.NoopDispatcher{})
	if cfg.Gateway.Enabled {
		if err := gatewayRunner.Start(context.Background()); err != nil {
			log.Printf("Gateway start failed: %v", err)
		} else {
			log.Printf("Gateway started with %d channel(s)", gatewayRunner.ChannelsStarted())
		}
	}

	port := cfg.Port
	if port == "" {
		port = "8080"
	}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("Core service starting on port %s", port)
		if err := r.Run(":" + port); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	<-quit
	log.Println("Shutting down server...")
	stopScheduler()
	maintenanceScheduler.Wait() // let in-flight job executions settle
	if err := gatewayRunner.Stop(context.Background()); err != nil {
		log.Printf("Gateway stop failed: %v", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := database.HealthCheck(shutdownCtx); err != nil {
		log.Printf("Database health check during shutdown: %v", err)
	}

	log.Println("Server exited")
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, X-Admin-Override-Reason, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, PATCH, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
