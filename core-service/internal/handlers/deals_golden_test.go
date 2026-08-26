package handlers

// C2 golden 锚（架构重构任务书 2026-08-26）：
//
// 签约前测算三包（predeal / dealcompare / renttosales）合并为
// services/leasescenario 前先把现状行为钉死。这两个 handler 无任何仓储依赖，
// 可以直接在 HTTP 层抓快照：请求体固定 → 响应体逐字节一致（含 JSON 键序，
// 键序变了就是契约变了）。renttosales 的 HTTP 层依赖真实 PG 仓储，其锚放在
// 服务层 golden：renttosales 包内的 ratio_golden_test.go（现随包居于 leasescenario）。
//
// 本文件只引用 handlers 自己的构造函数，不 import 三个服务包——搬家后无需改动。
// 再生：UPDATE_DEALS_GOLDEN=1 go test ./internal/handlers/ -run TestDealsGolden
// （重构类改动出现 diff = 立即回报，不得顺手再生）。

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

const (
	briefingRequestBody = `{
		"name": "五年期门店租约",
		"commencement_date": "2026-01-01",
		"term_months": 60,
		"monthly_rent": 120000,
		"rent_free_months": 3,
		"annual_escalation_percent": 5,
		"discount_rate": 0.06,
		"currency": "CNY",
		"initial_direct_cost": 150000,
		"early_exit_penalty_months": 3
	}`

	compareRequestBody = `{
		"discount_rate": 0.06,
		"currency": "CNY",
		"offers": [
			{"name": "甲：免租3月+年递增5%", "term_months": 36, "base_monthly_rent": 100000,
			 "rent_free_months": 3, "annual_escalation_percent": 5, "other_monthly_cost": 20000,
			 "upfront_cost": 300000, "landlord_contribution": 0, "area_sqm": 500},
			{"name": "乙：平租无免租", "term_months": 36, "base_monthly_rent": 108000,
			 "rent_free_months": 0, "annual_escalation_percent": 0, "other_monthly_cost": 20000,
			 "upfront_cost": 0, "landlord_contribution": 100000, "area_sqm": 500}
		]
	}`
)

func postJSON(t *testing.T, handler gin.HandlerFunc, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/probe", handler)
	req := httptest.NewRequest(http.MethodPost, "/probe", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// assertGoldenBody compares the response byte-for-byte against
// testdata/golden/<name>.json. Byte-exact on purpose: JSON key order is part
// of the contract (任务书共同验收底线 1).
func assertGoldenBody(t *testing.T, rec *httptest.ResponseRecorder, status int, name string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, status, rec.Body.String())
	}
	got := rec.Body.String()
	goldenPath := filepath.Join("testdata", "golden", name)
	if os.Getenv("UPDATE_DEALS_GOLDEN") != "" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}
	wantBytes, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden %s (seed once with UPDATE_DEALS_GOLDEN=1): %v", goldenPath, err)
	}
	if string(wantBytes) != got {
		t.Fatalf("golden drift in %s (byte-exact contract changed):\nwant:\n%s\ngot:\n%s", name, wantBytes, got)
	}
}

// TestDealsGolden pins POST /deals/briefing and POST /deals/compare.
// The compare case deliberately ranks the two offers differently per measure
// (free-rent months defer cash), so measures_disagree and the conclusion copy
// are pinned together with every number.
func TestDealsGolden(t *testing.T) {
	t.Run("briefing", func(t *testing.T) {
		assertGoldenBody(t, postJSON(t, NewPreDealHandler().Briefing, briefingRequestBody), http.StatusOK, "deals_briefing.json")
	})
	t.Run("compare", func(t *testing.T) {
		assertGoldenBody(t, postJSON(t, NewDealCompareHandler().Compare, compareRequestBody), http.StatusOK, "deals_compare.json")
	})
}
