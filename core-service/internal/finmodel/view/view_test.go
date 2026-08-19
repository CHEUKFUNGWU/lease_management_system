package view

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

func lintRaw(t *testing.T, kind Kind, config string) (Config, error) {
	t.Helper()
	return Lint(kind, json.RawMessage(config))
}

func TestLintRejects(t *testing.T) {
	cases := []struct {
		name    string
		kind    Kind
		config  string
		wantSub string
	}{
		{"unknown key smuggles data", KindStorePnl,
			`{"base_values":{"cash":100}}`, "unknown config key"},
		{"sql key rejected", KindStorePnl,
			`{"sql":"select 1"}`, "unknown config key"},
		{"not an object", KindStorePnl, `[1,2]`, "not a JSON object"},
		{"illegal basis mode", KindFinancialModel,
			`{"basis_mode":"debug"}`, "illegal basis_mode"},
		{"illegal grain", KindStorePnl, `{"grain":"fortnight"}`, "illegal grain"},
		{"one-sided period pair", KindFinancialModel,
			`{"period_from":"2026-01"}`, "set together"},
		{"bad period format", KindFinancialModel,
			`{"period_from":"2026-13","period_to":"2026-02"}`, "YYYY-MM"},
		{"from after to", KindFinancialModel,
			`{"period_from":"2026-06","period_to":"2026-01"}`, "after"},
		{"unknown version line", KindFinancialModel,
			`{"versions":{"fx":"2026Q1"}}`, "unknown version line"},
		{"empty version value", KindFinancialModel,
			`{"versions":{"data":""}}`, "non-empty"},
		{"unknown filter dimension", KindStorePnl,
			`{"filters":{"city":["x"]}}`, "unknown filter dimension"},
		{"filters on a non-store surface", KindFinancialModel,
			`{"filters":{"region":["华东"]}}`, "only valid for store_pnl"},
		{"filters on group surface", KindGroup,
			`{"filters":{"region":["华东"]}}`, "only valid for store_pnl"},
		{"empty filter values", KindStorePnl,
			`{"filters":{"brand":[]}}`, "no values"},
		{"duplicate filter value", KindStorePnl,
			`{"filters":{"brand":["A","A"]}}`, "repeats value"},
		{"duplicate hidden row", KindStorePnl,
			`{"rows_hidden":["cost","cost"]}`, "repeats row key"},
		{"hidden and folded overlap", KindStorePnl,
			`{"rows_hidden":["cost"],"rows_fold":["cost"]}`, "both hidden and folded"},
		{"empty row key", KindStorePnl, `{"rows_fold":["  "]}`, "illegal row key"},
		{"too many row keys", KindStorePnl,
			"{\"rows_hidden\":" + manyRowKeys(501) + "}", "too many row keys"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := lintRaw(t, tc.kind, tc.config)
			if err == nil {
				t.Fatalf("expected rejection, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("error %q does not mention %q", err.Error(), tc.wantSub)
			}
		})
	}
}

func TestLintAccepts(t *testing.T) {
	cases := []struct {
		name   string
		kind   Kind
		config string
	}{
		{"empty view is a legal default", KindStorePnl, `{}`},
		{"full store view", KindStorePnl, `{
			"period_from":"2026-01","period_to":"2026-12",
			"basis_mode":"working","grain":"week",
			"versions":{"template":"2","data":"2026-08-01","assumption":"a7"},
			"rows_hidden":["depreciation"],"rows_fold":["opexp_group"],
			"filters":{"region":["华东"],"brand":["B1","B2"],"store":["S-001","S-002"]}
		}`},
		{"official month group view", KindGroup,
			`{"basis_mode":"official","versions":{"exchange_rate":"fx-2026Q2","metric_definition":"m3"}}`},
		{"financial model quarter grain", KindFinancialModel,
			`{"period_from":"2025-01","period_to":"2026-12","grain":"quarter"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := lintRaw(t, tc.kind, tc.config)
			if err != nil {
				t.Fatalf("expected acceptance, got %v", err)
			}
			if tc.name == "full store view" && cfg.BasisMode != "working" {
				t.Fatalf("basis mode lost: %+v", cfg)
			}
		})
	}
}

func TestKindValidation(t *testing.T) {
	for _, kind := range []string{"store_pnl", "financial_model", "group_view"} {
		if !ValidKind(kind) {
			t.Fatalf("%s should be a valid kind", kind)
		}
	}
	for _, kind := range []string{"", "store", "sql", "store_pnl; drop"} {
		if ValidKind(kind) {
			t.Fatalf("%q should not be a valid kind", kind)
		}
	}
}

func manyRowKeys(n int) string {
	keys := make([]string, n)
	for i := range keys {
		keys[i] = strconv.Quote("r" + strconv.Itoa(i))
	}
	return "[" + strings.Join(keys, ",") + "]"
}
