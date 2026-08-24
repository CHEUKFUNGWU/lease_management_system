// First-party replacement for picoclaw pkg/config (AR5b): only the sub-structures
// the vendored agent kernel slice compiles against. NOT upstream code; field
// names and tags mirror upstream so the protocol code reads unchanged.
package config

type SecureString struct {
	resolved string // Decrypted/resolved value returned by String()
	raw      string // Persisted raw value (enc://, file://, or plaintext)
}

type SecureStrings []*SecureString

type ModelStreamingConfig struct {
	Enabled bool `json:"enabled,omitempty"`
}


type StreamingConfig struct {
	Enabled         bool `json:"enabled,omitempty"`
	ThrottleSeconds int  `json:"throttle_seconds,omitempty"`
	MinGrowthChars  int  `json:"min_growth_chars,omitempty"`
}

type GroupTriggerConfig struct {
	MentionOnly bool     `json:"mention_only,omitempty"`
	Prefixes    []string `json:"prefixes,omitempty"`
}

type ModelConfig struct {
	// Required fields
	ModelName string `json:"model_name"` // User-facing alias for the model
	Provider  string `json:"provider"`   // Provider name for routing and selection. When empty, provider resolution infers it from Model.
	Model     string `json:"model"`      // Model identifier, optionally provider-prefixed.

	// HTTP-based providers
	APIBase   string   `json:"api_base,omitempty"`  // API endpoint URL
	Proxy     string   `json:"proxy,omitempty"`     // HTTP proxy URL
	Fallbacks []string `json:"fallbacks,omitempty"` // Fallback model names for failover

	// Special providers (CLI-based, OAuth, etc.)
	AuthMethod  string `json:"auth_method,omitempty"`  // Authentication method: oauth, token
	ConnectMode string `json:"connect_mode,omitempty"` // Connection mode: stdio, grpc
	Workspace   string `json:"workspace,omitempty"`    // Workspace path for CLI-based providers

	// Optional optimizations
	RPM                 int                  `json:"rpm,omitempty"`              // Requests per minute limit
	MaxTokensField      string               `json:"max_tokens_field,omitempty"` // Field name for max tokens (e.g., "max_completion_tokens")
	RequestTimeout      int                  `json:"request_timeout,omitempty"`
	ThinkingLevel       string               `json:"thinking_level,omitempty"`        // Extended thinking: off|low|medium|high|xhigh|adaptive
	ToolSchemaTransform string               `json:"tool_schema_transform,omitempty"` // Optional tool schema compatibility transform (e.g. "simple")
	Streaming           ModelStreamingConfig `json:"streaming,omitzero"`              // Opt-in for provider streaming on this model entry
	ExtraBody           map[string]any       `json:"extra_body,omitempty"`            // Additional fields to inject into request body
	CustomHeaders       map[string]string    `json:"custom_headers,omitempty"`        // Additional headers to inject into every HTTP request

	APIKeys SecureStrings `json:"api_keys,omitzero" yaml:"api_keys,omitempty"` // API authentication keys (multiple keys for failover)

	// Enabled indicates whether this model entry is active. When omitted in
	// existing configs, the field is inferred during load: models with API keys
	// or the reserved "local-model" name are auto-enabled.
	Enabled bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	// UserAgent is the user agent string to use for HTTP requests.
	UserAgent string `json:"user_agent,omitempty" yaml:"-"`

	// isVirtual marks this model as a virtual model generated from multi-key expansion.
	// Virtual models should not be persisted to config files.
	isVirtual bool
}
