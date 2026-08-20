// Package llm is the Go LLM client (W4). It replaces the former Python
// ai-service chat path. The public surface is shaped so that a Client can be
// used directly as an agentcore.StreamFunc (see stream.go) — the Agent Core
// loop's single LLM I/O seam — while the plain Chat method keeps the existing
// aiagent call sites working without rewiring the event stream.
package llm

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config selects the upstream LLM provider. Field names mirror the existing
// environment variables (LLM_PROVIDER / DEEPSEEK_* / OPENAI_* / LLM_PRICING_*);
// do not invent new names — .env and docker-compose must not be renamed.
type Config struct {
	Provider string
	APIKey   string
	BaseURL  string
	Model    string

	PricingVersion           string
	InputPriceUSDPerMillion  float64
	OutputPriceUSDPerMillion float64
}

// ConfigFromEnv reads the LLM configuration from the process environment,
// matching ai-service/app/config.py defaults.
func ConfigFromEnv() Config {
	provider := strings.ToLower(getEnv("LLM_PROVIDER", "deepseek"))
	cfg := Config{Provider: provider}
	switch provider {
	case "openai":
		cfg.APIKey = os.Getenv("OPENAI_API_KEY")
		cfg.BaseURL = getEnv("OPENAI_BASE_URL", "https://api.openai.com/v1")
		cfg.Model = getEnv("OPENAI_MODEL", "gpt-4o")
	default:
		// Unsupported or unset providers fall back to DeepSeek, matching the
		// Python client's if/elif/else raising on unknown providers only when a
		// typed enum is expected — the HTTP client itself treats deepseek as
		// the default branch.
		cfg.Provider = "deepseek"
		cfg.APIKey = os.Getenv("DEEPSEEK_API_KEY")
		cfg.BaseURL = getEnv("DEEPSEEK_BASE_URL", "https://api.deepseek.com")
		cfg.Model = getEnv("DEEPSEEK_MODEL", "deepseek-v4-flash")
	}
	cfg.PricingVersion = getEnv("LLM_PRICING_VERSION", "unconfigured")
	cfg.InputPriceUSDPerMillion, _ = strconv.ParseFloat(os.Getenv("LLM_INPUT_PRICE_USD_PER_MILLION"), 64)
	cfg.OutputPriceUSDPerMillion, _ = strconv.ParseFloat(os.Getenv("LLM_OUTPUT_PRICE_USD_PER_MILLION"), 64)
	return cfg
}

// Validate mirrors the pydantic model_validator in ai-service/app/config.py:
// an enabled price book needs a version and both positive rates; a version of
// "unconfigured" forbids non-zero prices. A misconfigured book is a startup
// error, not a silent partial-cost fallback.
func (c Config) Validate() error {
	version := strings.TrimSpace(c.PricingVersion)
	if version == "" {
		return fmt.Errorf("llm: LLM_PRICING_VERSION must not be empty")
	}
	if strings.EqualFold(version, "unconfigured") {
		if c.InputPriceUSDPerMillion != 0 || c.OutputPriceUSDPerMillion != 0 {
			return fmt.Errorf("llm: LLM_PRICING_VERSION=unconfigured requires both LLM prices to be 0")
		}
		return nil
	}
	if c.InputPriceUSDPerMillion <= 0 || c.OutputPriceUSDPerMillion <= 0 {
		return fmt.Errorf("llm: configured LLM_PRICING_VERSION requires positive input and output prices")
	}
	return nil
}

// displayModelName is the "provider/model" string the Python client exposes
// through get_model_name().
func (c Config) displayModelName() string {
	return c.Provider + "/" + c.Model
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}