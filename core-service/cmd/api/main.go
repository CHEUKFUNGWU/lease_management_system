package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
	
	"github.com/gin-gonic/gin"
	"github.com/ifrs16/core-service/internal/config"
	"github.com/ifrs16/core-service/internal/db"
	"github.com/ifrs16/core-service/internal/handlers"
	"github.com/ifrs16/core-service/internal/middleware"
	"github.com/ifrs16/core-service/internal/repository"
	"github.com/ifrs16/core-service/internal/services/audit"
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
	
	// Initialize audit logger
	auditLogger := audit.NewLogger(auditRepo)
	
	// Initialize handlers
	authHandler := handlers.NewAuthHandler(cfg, userRepo)
	contractHandler := handlers.NewContractHandler(contractRepo, auditLogger)
	calcHandler := handlers.NewCalculationHandler(contractRepo, psRepo, systemSettingRepo)
	approvalHandler := handlers.NewApprovalHandler(approvalRepo, contractRepo, auditLogger)
	psHandler := handlers.NewPaymentScheduleHandler(psRepo, contractRepo)
	reportHandler := handlers.NewReportHandler(contractRepo, psRepo, mcRepo, systemSettingRepo)
	eventHandler := handlers.NewEventHandler(eventRepo, contractRepo, mcRepo, psRepo, systemSettingRepo, auditLogger)
	monthlyClosingHandler := handlers.NewMonthlyClosingHandler(mcRepo, contractRepo, psRepo, systemSettingRepo, auditLogger)
	aiChatHandler := handlers.NewAIChatHandler(contractRepo, mcRepo, eventRepo)
	auditHandler := handlers.NewAuditHandler(auditRepo)
	settingsHandler := handlers.NewSettingsHandler(systemSettingRepo)
	
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
	
	// Public: list active legal entities for registration
	r.GET("/api/v1/legal-entities", func(c *gin.Context) {
		rows, err := database.Pool.Query(c.Request.Context(), 
			`SELECT id, code, name, country, currency FROM legal_entities WHERE is_active = true ORDER BY code`)
		if err != nil {
			c.JSON(500, gin.H{"error": "failed to fetch legal entities"})
			return
		}
		defer rows.Close()
		
		type LegalEntity struct {
			ID       string `json:"id"`
			Code     string `json:"code"`
			Name     string `json:"name"`
			Country  string `json:"country"`
			Currency string `json:"currency"`
		}
		
		var entities []LegalEntity
		for rows.Next() {
			var e LegalEntity
			if err := rows.Scan(&e.ID, &e.Code, &e.Name, &e.Country, &e.Currency); err != nil {
				continue
			}
			entities = append(entities, e)
		}
		c.JSON(200, gin.H{"legal_entities": entities})
	})
	
	// Protected routes
	api := r.Group("/api/v1")
	api.Use(middleware.JWTAuth(cfg.JWTSecret))
	api.Use(middleware.TenantMiddleware())
	{
		api.GET("/me", handlers.GetCurrentUser())
		
		// Contracts
		api.POST("/contracts", contractHandler.Create)
		api.GET("/contracts", contractHandler.GetAll)
		api.GET("/contracts/:id", contractHandler.GetByID)
		api.PUT("/contracts/:id", contractHandler.Update)
		
		// Calculations
		api.POST("/contracts/:id/calculate", calcHandler.Calculate)
		api.GET("/contracts/:id/schedule", calcHandler.GetAmortizationSchedule)
		
		// Approval workflow
		api.POST("/contracts/:id/submit", approvalHandler.SubmitForReview)
		api.POST("/contracts/:id/review", approvalHandler.Review)
		api.POST("/contracts/:id/approve", approvalHandler.Approve)
		api.POST("/contracts/:id/reject", approvalHandler.Reject)
		api.GET("/contracts/:id/approval-status", approvalHandler.GetStatus)
		api.GET("/contracts-by-status", approvalHandler.ListByStatus)
		
		// Discount Rate
		api.GET("/contracts/:id/discount-rate-status", handlers.CheckDiscountRate(contractRepo))
		api.POST("/contracts/:id/confirm-discount-rate", handlers.ConfirmDiscountRate(contractRepo))
		
		// Payment Schedules
		api.POST("/contracts/:id/payment-schedules", psHandler.Create)
		api.GET("/contracts/:id/payment-schedules", psHandler.ListByContract)
		
		// Events
		api.POST("/contracts/:id/events", eventHandler.Create)
		api.GET("/contracts/:id/events", eventHandler.ListByContract)
		
		// Event approval workflow
		api.POST("/contracts/:id/events/:eventId/submit", eventHandler.SubmitForReview)
		api.POST("/contracts/:id/events/:eventId/review", eventHandler.Review)
		api.POST("/contracts/:id/events/:eventId/approve", eventHandler.Approve)
		api.POST("/contracts/:id/events/:eventId/reject", eventHandler.Reject)
		
		// Event IFRS 16 recalculation
		api.POST("/contracts/:id/events/:eventId/recalculate", eventHandler.RecalculateEvent)
		api.POST("/contracts/:id/events/:eventId/preview", eventHandler.PreviewEventAdjustment)
		api.GET("/contracts/:id/events/:eventId/adjustment", eventHandler.GetEventAdjustment)
		
		// Reports
		api.GET("/reports/liability-rolling", reportHandler.LiabilityRolling)
		api.GET("/reports/liability-rolling/export", reportHandler.ExportLiabilityRolling)
		api.GET("/reports/contract-summary", reportHandler.ContractSummary)
		api.GET("/reports/amortization", reportHandler.Amortization)
		api.GET("/reports/tags", reportHandler.Tags)
		api.GET("/reports/tags/summary", reportHandler.TagSummary)
		api.GET("/reports/cashflow-forecast", reportHandler.CashflowForecast)
		
		// Monthly Closing
		api.POST("/monthly-closing/generate", monthlyClosingHandler.Generate)
		api.GET("/monthly-closing/batches", monthlyClosingHandler.ListBatches)
		api.GET("/monthly-closing/entries", monthlyClosingHandler.GetJournalEntries)
		api.GET("/contracts/:id/measurement-results", monthlyClosingHandler.GetMeasurementResults)
		
		// Monthly Closing - Approval & Posting
		api.POST("/monthly-closing/entries/:id/approve", monthlyClosingHandler.ApproveEntry)
		api.POST("/monthly-closing/entries/:id/post", monthlyClosingHandler.PostEntry)
		api.POST("/monthly-closing/batches/:id/approve", monthlyClosingHandler.ApproveBatch)
		api.POST("/monthly-closing/batches/:id/post", monthlyClosingHandler.PostBatch)
		
		// Monthly Closing - Period Locking
		api.POST("/monthly-closing/periods/:period/lock", monthlyClosingHandler.LockPeriod)
		api.POST("/monthly-closing/periods/:period/unlock", monthlyClosingHandler.UnlockPeriod)
		api.GET("/monthly-closing/periods/:period/lock-status", monthlyClosingHandler.GetPeriodLockStatus)
		
		// AI Chat
		api.POST("/ai/chat", aiChatHandler.Chat)
		
		// Audit Logs
		api.GET("/audit-logs", auditHandler.List)
		
		// Admin: user management
		api.GET("/admin/users", authHandler.AdminListUsers)
		api.POST("/admin/users", authHandler.AdminCreateUser)

		// Roles & Permissions
		api.GET("/roles", handlers.ListRoles(roleRepo))
		api.GET("/my-permissions", handlers.GetMyPermissions(roleRepo))

		// Global Settings
		api.GET("/settings/global", settingsHandler.GetGlobal)
		api.PUT("/settings/global", settingsHandler.UpdateGlobal)
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
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, PATCH, DELETE")
		
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		
		c.Next()
	}
}
