// Package view is the S3-5 saved-view value lint. A saved view is
// presentation config only — period range, version lines, basis mode,
// grain, row show/hide and region/brand/store filters. The render path
// keeps its data rows and its permissions from the caller's scope; the
// view object must therefore be able to carry neither, and Lint enforces
// that fail-closed: any key outside the whitelist and any illegal value
// rejects the config before it persists.
package view

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Kind names the reporting surface a view is bound to.
type Kind string

const (
	KindStorePnl       Kind = "store_pnl"
	KindFinancialModel Kind = "financial_model"
	KindGroup          Kind = "group_view"
)

// ValidKind reports whether s is a surface saved views exist for.
func ValidKind(s string) bool {
	switch Kind(strings.TrimSpace(s)) {
	case KindStorePnl, KindFinancialModel, KindGroup:
		return true
	}
	return false
}

// Config is the validated presentation config of one saved view. Every
// field is opt-in; the zero value is a legal default view (all periods,
// default versions, everything visible, month grain).
type Config struct {
	PeriodFrom string              `json:"period_from,omitempty"` // ^YYYY-MM, <= PeriodTo when both set
	PeriodTo   string              `json:"period_to,omitempty"`
	BasisMode  string              `json:"basis_mode,omitempty"`  // working | official
	Grain      string              `json:"grain,omitempty"`       // day | week | month | quarter | year
	Versions   map[string]string   `json:"versions,omitempty"`    // the five version lines
	RowsHidden []string            `json:"rows_hidden,omitempty"` // rows the view hides
	RowsFold   []string            `json:"rows_fold,omitempty"`   // groups the view renders folded
	Filters    map[string][]string `json:"filters,omitempty"`     // region | brand | store (store_pnl only)
}

// The whitelist is the fail-closed half of the contract: anything else is
// rejected, so keys like "sql" or "base_values" cannot smuggle logic or
// data into what is supposed to be a presentation object.
var allowedKeys = map[string]bool{
	"period_from": true, "period_to": true, "basis_mode": true, "grain": true,
	"versions": true, "rows_hidden": true, "rows_fold": true, "filters": true,
}

var allowedBasisModes = map[string]bool{"": true, "working": true, "official": true}

var allowedGrains = map[string]bool{
	"": true, "day": true, "week": true, "month": true, "quarter": true, "year": true,
}

// allowedVersionLines is the five-line set in the PRD: template, data,
// assumption, exchange rate and metric definition. A typo here would
// silently dereference nothing at render time, so it is an error instead.
var allowedVersionLines = map[string]bool{
	"template": true, "data": true, "assumption": true,
	"exchange_rate": true, "metric_definition": true,
}

var allowedFilterDims = map[string]bool{"region": true, "brand": true, "store": true}

var periodRe = regexp.MustCompile(`^[0-9]{4}-(0[1-9]|1[0-2])$`)

const (
	maxNameLen      = 100 // row keys and filter values: sanity bound, not a charset contract
	maxFilterValues = 200 // total across all dimensions
	maxRowKeys      = 500
)

// Lint parses raw and returns the validated config, or an error naming the
// first violation found.
func Lint(kind Kind, raw json.RawMessage) (Config, error) {
	// Pass 1: key whitelist over the raw payload so unknown keys fail even
	// when the typed struct would have ignored them.
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payload); err != nil {
		return Config{}, fmt.Errorf("saved view: config is not a JSON object: %w", err)
	}
	unknown := make([]string, 0, len(payload))
	for key := range payload {
		if !allowedKeys[key] {
			unknown = append(unknown, key)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return Config{}, fmt.Errorf("saved view: unknown config key(s) %s — a saved view holds presentation config only", strings.Join(unknown, ", "))
	}

	// Pass 2: typed parse (payload was whitelisted, so this cannot miss keys).
	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("saved view: invalid config: %w", err)
	}
	if err := check(cfg); err != nil {
		return Config{}, err
	}

	// Pass 3: filters are a store-dimension concept; a surface without a
	// store dimension must not silently ignore them.
	if len(cfg.Filters) > 0 && kind != KindStorePnl {
		return Config{}, fmt.Errorf("saved view: filters are only valid for %s views", KindStorePnl)
	}
	return cfg, nil
}

func check(cfg Config) error {
	if !allowedBasisModes[cfg.BasisMode] {
		return fmt.Errorf("saved view: illegal basis_mode %q (working | official)", cfg.BasisMode)
	}
	if !allowedGrains[cfg.Grain] {
		return fmt.Errorf("saved view: illegal grain %q (day | week | month | quarter | year)", cfg.Grain)
	}

	switch {
	case cfg.PeriodFrom == "" && cfg.PeriodTo == "":
		// full range — fine
	case cfg.PeriodFrom == "" || cfg.PeriodTo == "":
		return fmt.Errorf("saved view: period_from and period_to must be set together")
	default:
		if !periodRe.MatchString(cfg.PeriodFrom) || !periodRe.MatchString(cfg.PeriodTo) {
			return fmt.Errorf("saved view: periods must be YYYY-MM")
		}
		if cfg.PeriodFrom > cfg.PeriodTo {
			return fmt.Errorf("saved view: period_from %s is after period_to %s", cfg.PeriodFrom, cfg.PeriodTo)
		}
	}

	for line, value := range cfg.Versions {
		if !allowedVersionLines[line] {
			return fmt.Errorf("saved view: unknown version line %q", line)
		}
		if strings.TrimSpace(value) == "" || len(value) > 200 {
			return fmt.Errorf("saved view: version line %q must be a non-empty value <= 200 chars", line)
		}
	}

	if len(cfg.RowsHidden)+len(cfg.RowsFold) > maxRowKeys {
		return fmt.Errorf("saved view: too many row keys (max %d)", maxRowKeys)
	}
	if err := checkRowKeys(cfg.RowsHidden, "rows_hidden"); err != nil {
		return err
	}
	if err := checkRowKeys(cfg.RowsFold, "rows_fold"); err != nil {
		return err
	}
	hidden := map[string]bool{}
	for _, key := range cfg.RowsHidden {
		hidden[key] = true
	}
	for _, key := range cfg.RowsFold {
		if hidden[key] {
			return fmt.Errorf("saved view: row %q is both hidden and folded", key)
		}
	}

	total := 0
	dims := make([]string, 0, len(cfg.Filters))
	for dim := range cfg.Filters {
		dims = append(dims, dim)
	}
	sort.Strings(dims)
	for _, dim := range dims {
		if !allowedFilterDims[dim] {
			return fmt.Errorf("saved view: unknown filter dimension %q (region | brand | store)", dim)
		}
		values := cfg.Filters[dim]
		if len(values) == 0 {
			return fmt.Errorf("saved view: filter %q has no values", dim)
		}
		total += len(values)
		if total > maxFilterValues {
			return fmt.Errorf("saved view: too many filter values (max %d)", maxFilterValues)
		}
		seen := map[string]bool{}
		for _, value := range values {
			value = strings.TrimSpace(value)
			if value == "" || len(value) > maxNameLen {
				return fmt.Errorf("saved view: filter %q has an illegal value", dim)
			}
			if seen[value] {
				return fmt.Errorf("saved view: filter %q repeats value %q", dim, value)
			}
			seen[value] = true
		}
	}
	return nil
}

func checkRowKeys(keys []string, field string) error {
	seen := map[string]bool{}
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" || len(key) > maxNameLen {
			return fmt.Errorf("saved view: %s contains an illegal row key", field)
		}
		if seen[key] {
			return fmt.Errorf("saved view: %s repeats row key %q", field, key)
		}
		seen[key] = true
	}
	return nil
}
