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
	"github.com/lease-management-system/core-service/internal/config"
	"github.com/lease-management-system/core-service/internal/db"
	"github.com/lease-management-system/core-service/internal/handlers"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/audit"
	"github.com/lease-management-system/core-service/internal/services/eventaccounting"
	"github.com/lease-management-system/core-service/internal/services/monthend"
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
	approvalRepo := repository.NewApprovalRepository(database.Pool)
	psRepo := repository.NewPaymentScheduleRepository(database.Pool)
	eventRepo := repository.NewEventRepository(database.Pool)
	mcRepo := repository.NewMonthlyClosingRepository(database.Pool)
	auditRepo := repository.NewAuditRepository(database.Pool)
	systemSettingRepo := repository.NewSystemSettingRepository(database.Pool)
	leaseAdminRepo := repository.NewLeaseAdminRepository(database.Pool)
	aiChatRuntimeRepo := repository.NewAIChatRuntimeRepository(database.Pool)
	masterDataRepo := repository.NewMasterDataRepository(database.Pool)
	accessPolicyRepo := repository.NewAccessPolicyRepository(database.Pool)
	exchangeRateRepo := repository.NewExchangeRateRepository(database.Pool)
	workQueueRepo := repository.NewWorkQueueRepository(database.Pool)
	budgetRepo := repository.NewBudgetRepository(database.Pool)

	// Initialize audit logger
	auditLogger := audit.NewLogger(auditRepo)

	// Initialize services
	closeService := monthend.NewService(database.Pool, mcRepo, contractRepo, psRepo, systemSettingRepo, exchangeRateRepo, masterDataRepo, auditLogger)
	eventPersistence := eventaccounting.NewPersistenceService(database.Pool, mcRepo, eventRepo, auditLogger)

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(cfg, userRepo, roleRepo)
	contractHandler := handlers.NewContractHandler(contractRepo, auditLogger)
	calcHandler := handlers.NewCalculationHandler(contractRepo, psRepo, systemSettingRepo)
	approvalHandler := handlers.NewApprovalHandler(approvalRepo, contractRepo, auditLogger)
	psHandler := handlers.NewPaymentScheduleHandler(psRepo, contractRepo)
	dealCompareHandler := handlers.NewDealCompareHandler()
	reportHandler := handlers.NewReportHandler(contractRepo, psRepo, mcRepo, systemSettingRepo, masterDataRepo)
	eventHandler := handlers.NewEventHandler(eventRepo, contractRepo, mcRepo, psRepo, systemSettingRepo, eventPersistence, auditLogger)
	monthlyClosingHandler := handlers.NewMonthlyClosingHandler(mcRepo, contractRepo, closeService, auditLogger)
	aiChatHandler := handlers.NewAIChatHandler(contractRepo, mcRepo, eventRepo, aiChatRuntimeRepo)
	auditHandler := handlers.NewAuditHandler(auditRepo)
	settingsHandler := handlers.NewSettingsHandler(systemSettingRepo)
	leaseAdminHandler := handlers.NewLeaseAdminHandler(leaseAdminRepo, contractRepo, auditLogger)
	masterDataHandler := handlers.NewMasterDataHandler(masterDataRepo)
	exchangeRateHandler := handlers.NewExchangeRateHandler(exchangeRateRepo, auditLogger)
	workQueueHandler := handlers.NewWorkQueueHandler(workQueueRepo)
	budgetHandler := handlers.NewBudgetHandler(budgetRepo, contractRepo, psRepo, systemSettingRepo)

	if cfg.LogLevel == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(corsMiddleware())

	// Health check
	r.GET("/health", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		dbStatus := "ok"
		if err := database.HealthCheck(ctx); err != nil {
			dbStatus = "error: " + err.Error()
		}

		c.JSON(200, gin.H{
			"status":   "ok",
			"service":  "core-service",
			"version":  "0.1.0",
			"database": dbStatus,
		})
	})

	// Public routes - registration disabled, only login is public
	r.POST("/api/v1/auth/register", authHandler.Register)
	r.POST("/api/v1/auth/login", authHandler.Login)

	// Protected routes
	api := r.Group("/api/v1")
	api.Use(middleware.JWTAuth(cfg.JWTSecret))
	api.Use(middleware.LoadUserPermissions(roleRepo))
	api.Use(middleware.DataScopeMiddleware())
	api.Use(middleware.TenantMiddleware())
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
		protected.Handle(http.MethodGet, "/me", permission("identity", "read"), handlers.GetCurrentUser())
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
		protected.Handle(http.MethodGet, "/contracts-by-status", permission("contracts", "read"), approvalHandler.ListByStatus)

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

		// Offer comparison reads nothing and writes nothing: the terms come in
		// with the request. It sits behind report permission because it is an
		// analysis tool, not because it touches report data.
		protected.Handle(http.MethodPost, "/deals/compare", permission("reports", "read"), dealCompareHandler.Compare)
		protected.Handle(http.MethodGet, "/reports/standard-comparison", permission("reports", "read"), reportHandler.StandardComparison)
		protected.Handle(http.MethodGet, "/reports/amortization", permission("reports", "read"), reportHandler.Amortization)
		protected.Handle(http.MethodGet, "/reports/tags", permission("reports", "read"), reportHandler.Tags)
		protected.Handle(http.MethodGet, "/reports/tags/summary", permission("reports", "read"), reportHandler.TagSummary)
		protected.Handle(http.MethodGet, "/reports/cashflow-forecast", permission("reports", "read"), reportHandler.CashflowForecast)
		protected.Handle(http.MethodGet, "/reports/disclosure", permission("reports", "read"), reportHandler.Disclosure)
		protected.Handle(http.MethodGet, "/reports/unit-price", permission("reports", "read"), reportHandler.UnitPrice)

		// Exchange rates: settings-grade master data used to translate
		// foreign-currency leases into the entity's functional currency.
		protected.Handle(http.MethodGet, "/exchange-rates", permission("settings", "read"), exchangeRateHandler.List)
		protected.Handle(http.MethodPost, "/exchange-rates", permission("settings", "update"), exchangeRateHandler.Upsert)

		// Budget versions freeze the measured forward schedule so later actuals
		// can be explained against a stable plan.
		protected.Handle(http.MethodGet, "/budget-versions", permission("reports", "read"), budgetHandler.ListVersions)
		protected.Handle(http.MethodPost, "/budget-versions", permission("reports", "read"), budgetHandler.CreateVersion)
		protected.Handle(http.MethodGet, "/budget-versions/:id/variance", permission("reports", "read"), budgetHandler.Variance)

		// Monthly Closing
		protected.Handle(http.MethodPost, "/monthly-closing/generate", permission("monthly_closing", "generate"), monthlyClosingHandler.Generate)
		protected.Handle(http.MethodGet, "/monthly-closing/batches", permission("monthly_closing", "read"), monthlyClosingHandler.ListBatches)
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

		// AI Chat
		protected.Handle(http.MethodPost, "/ai/chat", permission("ai_chat", "use"), aiChatHandler.Chat)
		protected.Handle(http.MethodPost, "/ai/chat/sessions", permission("ai_chat", "use"), aiChatHandler.CreateSession)
		protected.Handle(http.MethodGet, "/ai/chat/sessions", permission("ai_chat", "use"), aiChatHandler.ListSessions)
		protected.Handle(http.MethodGet, "/ai/chat/sessions/:id", permission("ai_chat", "use"), aiChatHandler.GetSession)
		protected.Handle(http.MethodPost, "/ai/chat/sessions/:id/runs", permission("ai_chat", "use"), aiChatHandler.CreateRun)
		protected.Handle(http.MethodGet, "/ai/chat/sessions/:id/runs", permission("ai_chat", "use"), aiChatHandler.ListRuns)
		protected.Handle(http.MethodPost, "/ai/chat/continuations", permission("ai_chat", "use"), aiChatHandler.CreateContinuation)
		protected.Handle(http.MethodGet, "/ai/chat/runs/:id/events", permission("ai_chat", "use"), aiChatHandler.ListRunEvents)
		protected.Handle(http.MethodGet, "/ai/chat/runs/:id/stream", permission("ai_chat", "use"), aiChatHandler.StreamRunEvents)
		protected.Handle(http.MethodPost, "/ai/chat/artifacts/:id/actions", permission("ai_chat", "use"), aiChatHandler.CreateReviewAction)

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
