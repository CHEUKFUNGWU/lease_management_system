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

	JWTSecret                     string
	AccessTokenTTLSeconds         int
	RefreshTokenTTLSeconds        int
	RefreshTokenCleanupSeconds    int
	AgentCapabilitySecret         string
	AgentCapabilityTTLSeconds     int
	AgentCapabilityCleanupSeconds int
	// RT1-L3-C: interval for the expired-lease recovery scheduled job.
	AgentRunLeaseRecoverySeconds int
	// RT1-L3-D: path to the MCP registration manifest; empty = feature off.
	MCPManifestPath string
	LogLevel        string
	Port            string

	// MinIO read seam for agent-side import preview (page-fill). Empty
	// endpoint disables it; the agent then refuses honestly (D-D2).
	MinIOEndpoint  string
	MinIOAccessKey string
	MinIOSecretKey string
	MinIOBucket    string

	// AnyDocBinPath is the pinned anydoc CLI binary shipped in the runtime
	// image (see Dockerfile). Empty disables the office-document parser, which
	// then degrades to parser_unavailable instead of fabricating a parse.
	AnyDocBinPath string

	// IM 网关（Ch3，ADR-0026 §6）：接线但默认关。Enabled=false 或凭据缺失时
	// 对应渠道不启动；记录具名原因，不 panic、不重试刷屏。
	Gateway GatewaySettings
}

// GatewaySettings carries the IM channel switch and credentials. The zero
// value is fully disabled; no environment variable turns it on implicitly.
type GatewaySettings struct {
	Enabled bool
	Feishu  ChannelCredentials
	WeCom   ChannelCredentials
}

type ChannelCredentials struct {
	AppID             string
	AppSecret         string
	EncryptKey        string
	VerificationToken string
	IsLark            bool
	BotID             string
	Secret            string
	WebSocketURL      string
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
	leaseRecovery, _ := strconv.Atoi(os.Getenv("AGENT_RUN_LEASE_RECOVERY_SECONDS"))
	if leaseRecovery <= 0 {
		leaseRecovery = 60
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

		JWTSecret:                     getEnv("JWT_SECRET", "lease_jwt_secret_key"),
		AccessTokenTTLSeconds:         accessTTL,
		RefreshTokenTTLSeconds:        refreshTTL,
		RefreshTokenCleanupSeconds:    refreshCleanup,
		AgentCapabilitySecret:         getEnv("AGENT_CAPABILITY_SECRET", "lease_agent_capability_secret"),
		AgentCapabilityTTLSeconds:     capabilityTTL,
		AgentCapabilityCleanupSeconds: capabilityCleanup,
		AgentRunLeaseRecoverySeconds:  leaseRecovery,
		MCPManifestPath:               getEnv("MCP_MANIFEST_PATH", ""),
		LogLevel:                      getEnv("LOG_LEVEL", "info"),
		Port:                          port,

		MinIOEndpoint:  getEnv("MINIO_ENDPOINT", "minio:9000"),
		MinIOAccessKey: getEnv("MINIO_ACCESS_KEY", "minioadmin"),
		MinIOSecretKey: getEnv("MINIO_SECRET_KEY", "minioadmin"),
		MinIOBucket:    getEnv("MINIO_BUCKET", "lease-uploads"),

		AnyDocBinPath: getEnv("ANYDOC_BIN_PATH", "/usr/local/lib/node_modules/@firecrawl/anydoc/cli.js"),
	}

	// IM 网关：默认关。只有 GATEWAY_ENABLED=true 才读取渠道凭据；凭据本身
	// 只从环境变量来，无默认值（ADR-0026 §6）。
	cfg.Gateway.Enabled = os.Getenv("GATEWAY_ENABLED") == "true"
	if cfg.Gateway.Enabled {
		cfg.Gateway.Feishu = ChannelCredentials{
			AppID:             os.Getenv("GATEWAY_FEISHU_APP_ID"),
			AppSecret:         os.Getenv("GATEWAY_FEISHU_APP_SECRET"),
			EncryptKey:        os.Getenv("GATEWAY_FEISHU_ENCRYPT_KEY"),
			VerificationToken: os.Getenv("GATEWAY_FEISHU_VERIFICATION_TOKEN"),
			IsLark:            os.Getenv("GATEWAY_FEISHU_IS_LARK") == "true",
		}
		cfg.Gateway.WeCom = ChannelCredentials{
			BotID:        os.Getenv("GATEWAY_WECOM_BOT_ID"),
			Secret:       os.Getenv("GATEWAY_WECOM_SECRET"),
			WebSocketURL: os.Getenv("GATEWAY_WECOM_WEBSOCKET_URL"),
		}
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
