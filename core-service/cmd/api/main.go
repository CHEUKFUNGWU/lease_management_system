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
	
	// Initialize handlers
	authHandler := handlers.NewAuthHandler(cfg, userRepo)
	contractHandler := handlers.NewContractHandler(contractRepo)
	calcHandler := handlers.NewCalculationHandler(contractRepo)
	
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
	
	// Public routes
	r.POST("/api/v1/auth/register", authHandler.Register)
	r.POST("/api/v1/auth/login", authHandler.Login)
	
	// Protected routes
	api := r.Group("/api/v1")
	api.Use(middleware.JWTAuth(cfg.JWTSecret))
	{
		api.GET("/me", handlers.GetCurrentUser())
		
		// Contracts
		api.POST("/contracts", contractHandler.Create)
		api.GET("/contracts", contractHandler.GetAll)
		api.GET("/contracts/:id", contractHandler.GetByID)
		
		// Calculations
		api.POST("/contracts/:id/calculate", calcHandler.Calculate)
		api.GET("/contracts/:id/schedule", calcHandler.GetAmortizationSchedule)
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
