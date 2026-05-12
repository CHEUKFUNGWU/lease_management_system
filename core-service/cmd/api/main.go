package main

import (
	"log"
	"os"
	
	"github.com/gin-gonic/gin"
	"github.com/ifrs16/core-service/internal/config"
	"github.com/ifrs16/core-service/internal/handlers"
	"github.com/ifrs16/core-service/internal/middleware"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	
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
		c.JSON(200, gin.H{
			"status":  "ok",
			"service": "core-service",
			"version": "0.1.0",
		})
	})
	
	// Public routes
	r.POST("/api/v1/auth/register", handlers.Register(cfg))
	r.POST("/api/v1/auth/login", handlers.Login(cfg))
	
	// Protected routes
	api := r.Group("/api/v1")
	api.Use(middleware.JWTAuth(cfg.JWTSecret))
	{
		api.GET("/me", handlers.GetCurrentUser())
		
		// Contracts
		api.POST("/contracts", handlers.CreateContract())
		api.GET("/contracts", handlers.GetContracts())
		api.GET("/contracts/:id", handlers.GetContract())
	}
	
	port := cfg.Port
	if port == "" {
		port = "8080"
	}
	
	log.Printf("Core service starting on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
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
