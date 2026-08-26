package llm

import (
	"strings"
	"testing"
)

func TestConfigFromEnvDeepSeekDefault(t *testing.T) {
	t.Setenv("LLM_PROVIDER", "deepseek")
	t.Setenv("DEEPSEEK_API_KEY", "sk-deepseek")
	t.Setenv("DEEPSEEK_BASE_URL", "https://api.example.deepseek.com")
	t.Setenv("DEEPSEEK_MODEL", "deepseek-v4-pro")
	t.Setenv("OPENAI_API_KEY", "sk-openai")

	cfg := ConfigFromEnv()
	if cfg.Provider != "deepseek" || cfg.APIKey != "sk-deepseek" ||
		cfg.BaseURL != "https://api.example.deepseek.com" || cfg.Model != "deepseek-v4-pro" {
		t.Fatalf("deepseek branch misread: %#v", cfg)
	}
	if cfg.PricingVersion != "unconfigured" {
		t.Fatalf("pricing default = %q, want unconfigured", cfg.PricingVersion)
	}
	if got := cfg.displayModelName(); got != "deepseek/deepseek-v4-pro" {
		t.Fatalf("display model = %q", got)
	}
}

func TestConfigFromEnvOpenAIBranch(t *testing.T) {
	t.Setenv("LLM_PROVIDER", "openai")
	t.Setenv("OPENAI_API_KEY", "sk-openai")
	t.Setenv("OPENAI_BASE_URL", "https://api.openai.proxy.local/v1")
	t.Setenv("OPENAI_MODEL", "gpt-4o")

	cfg := ConfigFromEnv()
	if cfg.Provider != "openai" || cfg.APIKey != "sk-openai" ||
		cfg.BaseURL != "https://api.openai.proxy.local/v1" || cfg.Model != "gpt-4o" {
		t.Fatalf("openai branch misread: %#v", cfg)
	}
	if got := cfg.displayModelName(); got != "openai/gpt-4o" {
		t.Fatalf("display model = %q", got)
	}
}

func TestConfigFromEnvFallbackToDeepSeek(t *testing.T) {
	t.Setenv("LLM_PROVIDER", "anthropic") // unsupported -> deepseek branch
	t.Setenv("OPENAI_API_KEY", "sk-openai")
	t.Setenv("DEEPSEEK_API_KEY", "sk-deepseek")

	cfg := ConfigFromEnv()
	if cfg.Provider != "deepseek" || cfg.APIKey != "sk-deepseek" {
		t.Fatalf("unsupported provider must fall back to deepseek: %#v", cfg)
	}
}

func TestConfigFromEnvEmptyProviderFallsBack(t *testing.T) {
	t.Setenv("LLM_PROVIDER", "")
	t.Setenv("DEEPSEEK_API_KEY", "sk-deepseek")
	cfg := ConfigFromEnv()
	if cfg.Provider != "deepseek" || cfg.APIKey != "sk-deepseek" {
		t.Fatalf("empty provider must default to deepseek: %#v", cfg)
	}
}

func TestConfigValidatePricingBook(t *testing.T) {
	// valid defaults
	if err := (Config{Provider: "deepseek", PricingVersion: "unconfigured"}).Validate(); err != nil {
		t.Fatalf("unconfigured empty book must be valid: %v", err)
	}
	// empty version rejected
	if err := (Config{Provider: "deepseek", PricingVersion: "  "}).Validate(); err == nil {
		t.Fatal("blank version must be rejected")
	}
	// unconfigured with nonzero prices rejected
	if err := (Config{Provider: "deepseek", PricingVersion: "unconfigured", InputPriceUSDPerMillion: 1}).Validate(); err == nil {
		t.Fatal("unconfigured + non-zero price must be rejected")
	}
	// configured book requires both positive rates
	if err := (Config{Provider: "deepseek", PricingVersion: "v1", InputPriceUSDPerMillion: 1}).Validate(); err == nil {
		t.Fatal("configured book with one zero rate must be rejected")
	}
	if err := (Config{Provider: "deepseek", PricingVersion: "v1", InputPriceUSDPerMillion: 1, OutputPriceUSDPerMillion: 2}).Validate(); err != nil {
		t.Fatalf("full book must be valid: %v", err)
	}
}

func TestNewClientRejectsInvalidPricingBook(t *testing.T) {
	if _, err := NewClient(Config{Provider: "deepseek", APIKey: "k", PricingVersion: "unconfigured", InputPriceUSDPerMillion: 0.5}); err == nil {
		t.Fatal("NewClient must reject a half-configured price book at construction")
	}
}

func TestChatWithoutAPIKeyFailsClosed(t *testing.T) {
	cfg := Config{Provider: "deepseek", BaseURL: "https://example.invalid", Model: "m"}

	// ConfigFromEnv would leave APIKey empty when the env var is absent.
	client, err := NewClient(Config{Provider: "deepseek", BaseURL: "https://example.invalid", Model: "m", PricingVersion: "unconfigured"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	_, err = client.Chat(t.Context(), ChatRequest{Messages: []Message{{Role: "user", Content: "hi"}}})
	if err == nil || !strings.EqualFold(err.Error(), ErrNotConfigured.Error()) {
		t.Fatalf("missing key must fail closed with ErrNotConfigured, got %v", err)
	}
	_ = cfg
}
