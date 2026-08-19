package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lease-management-system/core-service/internal/repository"
)

// TestGroupRunsTranslatedSecondViewPostgres locks S5-2 end to end: two runs
// in different currencies, an explicit closing-rate version, and the
// /financial-model/group handler must serve the translated second view —
// totals computed ONLY from translated values, the version banner and its
// type on the response, and ties_out verified against member contributions.
// The T14 refusal and the rate_type conflict guard get locked in the same
// fixture.
func TestGroupRunsTranslatedSecondViewPostgres(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to Postgres: %v", err)
	}
	t.Cleanup(pool.Close)
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("fixture exec: %v", err)
		}
	}

	suffix := uuid.NewString()[:8]
	entity := uuid.NewString()
	exec(`INSERT INTO legal_entities (id, code, name, country, currency) VALUES ($1,$2,$3,'CN','CNY')`,
		entity, "GRP-E-"+suffix, "Group "+suffix)
	tmplID := uuid.NewString()
	exec(`INSERT INTO fin_statement_templates (id, legal_entity_id, name, version, status, rows)
		VALUES ($1,$2,$3,1,'approved','{"rows":[]}'::jsonb)`, tmplID, entity, "GRP-TPL-"+suffix)
	defID := uuid.NewString()
	exec(`INSERT INTO fin_model_definitions (id, legal_entity_id, name, version, template_id, policy, source_bindings)
		VALUES ($1,$2,$3,1,$4,'{}'::jsonb,'{}'::jsonb)`, defID, entity, "GRP-DEF-"+suffix, tmplID)
	runCNY := uuid.NewString()
	runUSD := uuid.NewString()
	exec(`INSERT INTO fin_model_runs (id, legal_entity_id, model_definition_id, model_definition_version, status, tie_out_status, input_snapshot, idempotency_key)
		VALUES ($1,$2,$3,1,'completed','passed','{"currency":"CNY"}'::jsonb,$4),
		       ($5,$2,$3,1,'completed','passed','{"currency":"USD"}'::jsonb,$6)`,
		runCNY, entity, defID, "grp-cny-"+suffix, runUSD, "grp-usd-"+suffix)
	exec(`INSERT INTO fin_model_run_lines (run_id, row_key, period, value, provenance) VALUES
		($1,'rev','2026-01',100,'{}'::jsonb), ($2,'rev','2026-01',80,'{}'::jsonb)`, runCNY, runUSD)

	fxID := uuid.NewString()
	exec(`INSERT INTO exchange_rate_versions (id, name, version_type, effective_from, source, status)
		VALUES ($1,$2,'closing',NOW(),'integration-test','approved')`, fxID, "GRP-FX-"+suffix)
	// exchange_rates 唯一键含 rate_date：以 suffix 派生唯一日期，避免同一
	// 集成库内重复运行互相撞 key。
	seed, _ := strconv.ParseUint(suffix, 16, 32)
	rateDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, int(seed%30000)).Format("2006-01-02")
	exec(`INSERT INTO exchange_rates (from_currency, to_currency, rate_date, rate_type, rate, version_id, source)
		VALUES ('USD','CNY',$2,'closing',7.0,$1,'integration-test')`, fxID, rateDate)

	handler := NewFinModelHandler(repository.NewFinModelRepository(pool)).
		WithExchangeRates(repository.NewExchangeRateRepository(pool))
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/financial-model/group", func(c *gin.Context) {
		c.Set("legal_entity_id", "") // 全局 admin：所有成员 authorized
		handler.GroupRuns(c)
	})

	get := func(query string) (int, string) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/financial-model/group?"+query, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w.Code, w.Body.String()
	}

	type groupBody struct {
		Group struct {
			Totals              map[string]*float64 `json:"totals"`
			TotalsCurrency      string              `json:"totals_currency"`
			ExchangeRateVersion string              `json:"exchange_rate_version"`
			ExchangeRateType    string              `json:"exchange_rate_type"`
			TiesOut             bool                `json:"ties_out"`
			Note                string              `json:"note"`
		} `json:"group"`
	}

	// 1. 折算第二视图：cross-currency totals 只来自折算值。
	code, raw := get("run_ids=" + runCNY + "," + runUSD + "&exchange_rate_version=GRP-FX-" + suffix + "&target_currency=CNY")
	if code != http.StatusOK {
		t.Fatalf("translated view status %d: %s", code, raw)
	}
	var body groupBody
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		t.Fatalf("parse body: %v (%s)", err, raw)
	}
	if got := body.Group.Totals["rev@2026-01"]; got == nil || *got != 660 {
		t.Fatalf("translated total = %v, want 660 (100 + 80×7.0); body=%s", got, raw)
	}
	if body.Group.TotalsCurrency != "CNY" || body.Group.ExchangeRateVersion != "GRP-FX-"+suffix || body.Group.ExchangeRateType != "closing" {
		t.Fatalf("banner fields = %+v; the translated view must carry version + type everywhere", body.Group)
	}
	if !body.Group.TiesOut || !strings.Contains(body.Group.Note, "未抵销") {
		t.Fatalf("ties_out=%v note=%q", body.Group.TiesOut, body.Group.Note)
	}

	// 2. T14：未选汇率版本，跨币种不出现任何合计数字。
	code, raw = get("run_ids=" + runCNY + "," + runUSD)
	if code != http.StatusOK {
		t.Fatalf("default view status %d: %s", code, raw)
	}
	var plain groupBody
	if err := json.Unmarshal([]byte(raw), &plain); err != nil {
		t.Fatal(err)
	}
	if len(plain.Group.Totals) != 0 {
		t.Fatalf("cross-currency totals must not exist without an exchange_rate_version, got %+v", plain.Group.Totals)
	}

	// 3. rate_type 与版本类型冲突：显式 closing / average 是 S5-2 的要求。
	code, raw = get("run_ids=" + runCNY + "," + runUSD + "&exchange_rate_version=GRP-FX-" + suffix + "&rate_type=average")
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("rate_type conflict status %d, want 422: %s", code, raw)
	}
}
