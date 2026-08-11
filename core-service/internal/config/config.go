package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	DBHost     string
	DBPort     int
	DBUser     string
	DBPassword string
	DBName     string

	MinioEndpoint  string
	MinioAccessKey string
	MinioSecretKey string

	JWTSecret                     string
	AccessTokenTTLSeconds         int
	RefreshTokenTTLSeconds        int
	RefreshTokenCleanupSeconds    int
	AgentCapabilitySecret         string
	AgentCapabilityTTLSeconds     int
	AgentCapabilityCleanupSeconds int
	LogLevel                      string
	Port                          string
}

func Load() (*Config, error) {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbPort, _ := strconv.Atoi(os.Getenv("DB_PORT"))
	if dbPort == 0 {
		dbPort = 5432
	}

	capabilityTTL, _ := strconv.Atoi(os.Getenv("AGENT_CAPABILITY_TTL_SECONDS"))
	if capabilityTTL <= 0 {
		capabilityTTL = 300
	}
	capabilityCleanup, _ := strconv.Atoi(os.Getenv("AGENT_CAPABILITY_CLEANUP_SECONDS"))
	if capabilityCleanup <= 0 {
		capabilityCleanup = 900
	}
	accessTTL, _ := strconv.Atoi(os.Getenv("ACCESS_TOKEN_TTL_SECONDS"))
	if accessTTL <= 0 {
		accessTTL = 24 * 60 * 60
	}
	refreshTTL, _ := strconv.Atoi(os.Getenv("REFRESH_TOKEN_TTL_SECONDS"))
	if refreshTTL <= 0 {
		refreshTTL = 30 * 24 * 60 * 60
	}
	refreshCleanup, _ := strconv.Atoi(os.Getenv("REFRESH_TOKEN_CLEANUP_SECONDS"))
	if refreshCleanup <= 0 {
		refreshCleanup = 3600
	}

	cfg := &Config{
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     dbPort,
		DBUser:     getEnv("DB_USER", "lease"),
		DBPassword: getEnv("DB_PASSWORD", "lease_secret"),
		DBName:     getEnv("DB_NAME", "lease"),

		MinioEndpoint:  getEnv("MINIO_ENDPOINT", "localhost:9000"),
		MinioAccessKey: getEnv("MINIO_ACCESS_KEY", "minioadmin"),
		MinioSecretKey: getEnv("MINIO_SECRET_KEY", "minioadmin"),

		JWTSecret:                     getEnv("JWT_SECRET", "lease_jwt_secret_key"),
		AccessTokenTTLSeconds:         accessTTL,
		RefreshTokenTTLSeconds:        refreshTTL,
		RefreshTokenCleanupSeconds:    refreshCleanup,
		AgentCapabilitySecret:         getEnv("AGENT_CAPABILITY_SECRET", "lease_agent_capability_secret"),
		AgentCapabilityTTLSeconds:     capabilityTTL,
		AgentCapabilityCleanupSeconds: capabilityCleanup,
		LogLevel:                      getEnv("LOG_LEVEL", "info"),
		Port:                          port,
	}

	return cfg, nil
}

func (c *Config) DatabaseURL() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
