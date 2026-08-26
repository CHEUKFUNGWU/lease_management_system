package handlers

// 电商独立站模式 handler 层 golden（spec §6 测试缝 1：最高缝）。
//
// 四个新页面的 API 各一组：site-pulse / site-diagnostics / site-pnl /
// settlement-runs。用 fake DBTX 在 handler 边界逐字节锁响应形状与降级语义；
// 挂钟字段（generated_at / created_at 等）scrub 后比较，其余任何字节
// 漂了就是漂了。
//
// 再生：UPDATE_ECOM_GOLDEN=1 go test ./internal/handlers/ -run TestEcomGolden

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/lease-management-system/core-service/internal/repository"
)

// ---------------------------------------------------------------------------
// fake DBTX：路由到预置行集。列序与 repository 的 Scan 严格一致。

type ecomFakeRows struct {
	rows [][]any
	idx  int
}

func (r *ecomFakeRows) Next() bool {
	r.idx++
	return r.idx <= len(r.rows)
}

func (r *ecomFakeRows) Scan(dest ...any) error {
	if r.idx <= 0 || r.idx > len(r.rows) {
		return pgx.ErrNoRows
	}
	row := r.rows[r.idx-1]
	if len(row) != len(dest) {
		return nil // 列数不匹配由字段扫描报错更自然；这里直接复制可复制的部分
	}
	for i := range dest {
		if row[i] == nil {
			continue
		}
		rv := reflect.ValueOf(dest[i])
		if rv.Kind() != reflect.Ptr || rv.IsNil() {
			continue
		}
		sv := reflect.ValueOf(row[i])
		if sv.Type().AssignableTo(rv.Elem().Type()) {
			rv.Elem().Set(sv)
		} else if sv.Type().ConvertibleTo(rv.Elem().Type()) {
			rv.Elem().Set(sv.Convert(rv.Elem().Type()))
		}
	}
	return nil
}

func (r *ecomFakeRows) Close()                                  {}
func (r *ecomFakeRows) Err() error                              { return nil }
func (r *ecomFakeRows) CommandTag() pgconn.CommandTag           { return pgconn.CommandTag{} }
func (r *ecomFakeRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *ecomFakeRows) Values() ([]any, error)                  { return nil, nil }
func (r *ecomFakeRows) RawValues() [][]byte                     { return nil }
func (r *ecomFakeRows) Conn() *pgx.Conn                         { return nil }

type ecomFakeRow struct{ rows *ecomFakeRows }

func (r *ecomFakeRow) Scan(dest ...any) error {
	if !r.rows.Next() {
		return pgx.ErrNoRows
	}
	return r.rows.Scan(dest...)
}

// ecomFakeDB 是 EcommerceRepository 的桩库。每个查询按 SQL 关键词路由到 fixture。
type ecomFakeDB struct {
	storefronts   [][]any
	dayFacts      [][]any
	campaignFacts [][]any
	glRevenue     [][]any
	fixedCost     [][]any
	payouts       [][]any
	banks         [][]any
	receivables   [][]any
	reserveEvents [][]any
	settlementRun [][]any
	insertedRun   *repository.SettlementRun
}

func (f *ecomFakeDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(sql, "INSERT INTO settlement_runs") {
		f.captureSettlementRun(args...)
	}
	return pgconn.CommandTag{}, nil
}

// captureSettlementRun 从 INSERT 参数构造成返列（列序 = settlementRunColumns）。
func (f *ecomFakeDB) captureSettlementRun(args ...any) {
	run := &repository.SettlementRun{}
	assignStr := func(i int, dst *string) {
		if i < len(args) && args[i] != nil {
			if s, ok := args[i].(string); ok {
				*dst = s
			}
		}
	}
	assignStr(0, &run.ID)
	assignStr(1, &run.LegalEntityID)
	assignStr(2, &run.StorefrontID)
	assignStr(3, &run.Period)
	assignStr(4, &run.Currency)
	assignStr(5, &run.PolicyVersion)
	// INSERT 列序（gate 修复后）：...policy_version, gate_verdict, matched_count,
	// difference_count, total_difference_amount, results, differences, created_by, idempotency_key
	if raw, ok := args[10].(json.RawMessage); ok {
		run.Results = raw
	}
	if raw, ok := args[11].(json.RawMessage); ok {
		run.Differences = raw
	}
	if v, ok := args[6].(string); ok {
		run.GateVerdict = &v
	}
	run.Status = "draft"
	run.CreatedAt = time.Now().UTC()
	run.UpdatedAt = time.Now().UTC()
	f.insertedRun = run
	now := run.CreatedAt
	var verdict any
	if v := run.GateVerdict; v != nil {
		verdict = *v
	}
	f.settlementRun = [][]any{{
		run.ID, run.LegalEntityID, run.StorefrontID, run.Period, run.Currency, run.Status,
		run.PolicyVersion, verdict, run.MatchedCount, run.DifferenceCount, run.TotalDifferenceAmount,
		run.Results, run.Differences, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		run.CreatedBy, run.IdempotencyKey, now, now,
	}}
}

func (f *ecomFakeDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return &ecomFakeRows{rows: f.route(sql)}, nil
}

func (f *ecomFakeDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	rows := f.route(sql, args...)
	return &ecomFakeRow{rows: &ecomFakeRows{rows: rows}}
}


func (f *ecomFakeDB) route(sql string, args ...any) [][]any {
	switch {
	case strings.Contains(sql, "INSERT INTO settlement_runs"):
		if len(f.settlementRun) == 0 {
			if len(args) == 0 {
				println("INSERT INTO settlement_runs WITH NO ARGS:", sql)
			}
			f.captureSettlementRun(args...)
		}
		return f.settlementRun
	case strings.Contains(sql, "FROM operating_fact_batches WHERE idempotency_key"):
		return [][]any{{"batch-1", "completed", 0, 0, 0}}
	case strings.Contains(sql, "FROM storefronts") && (strings.Contains(sql, "ORDER BY code") || strings.Contains(sql, "WHERE id::text")):
		return f.storefronts
	case strings.Contains(sql, "DISTINCT ON (storefront_id::text, business_date, channel, sku, source_system)"):
		return f.dayFacts
	case strings.Contains(sql, "DISTINCT ON (storefront_id::text, campaign_id, business_date, source_system)"):
		return f.campaignFacts
	case strings.Contains(sql, "FROM storefront_gl_revenues"):
		return f.glRevenue
	case strings.Contains(sql, "FROM storefront_fixed_costs"):
		return f.fixedCost
	case strings.Contains(sql, "WITH latest AS"):
		return f.receivables
	case strings.Contains(sql, "DISTINCT ON (provider, payout_id)"):
		return f.payouts
	case strings.Contains(sql, "DISTINCT ON (bank_ref)"):
		return f.banks
	case strings.Contains(sql, "FROM rolling_reserve_events"):
		return f.reserveEvents
	case strings.Contains(sql, "SELECT " + settlementRunColumnsSQL()):
		if len(f.settlementRun) > 0 {
			return f.settlementRun
		}
		return [][]any{}
	default:
		return [][]any{}
	}
}

func settlementRunColumnsSQL() string {
	// 从 repository 常量镜像列序（测试锁定形状，漂了会 red）
	return `id, legal_entity_id::text, storefront_id::text, period, currency, status,
	policy_version, gate_verdict, matched_count, difference_count, total_difference_amount,
	results, differences, prepared_by::text, prepared_at, submitted_by::text, submitted_at,
	approved_by::text, approved_at, rejected_by::text, rejected_at, rejection_reason,
	created_by::text, idempotency_key, created_at, updated_at`
}

// ---------------------------------------------------------------------------
// fixture：固定构造（2026-08 窗口）

func ecomGoldenFixture() *ecomFakeDB {
	day := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	now := time.Now().UTC()
	db := &ecomFakeDB{}
	db.storefronts = [][]any{
		{"sf-us", "LE-1", "US", "美国独立站", "US", "USD", "shopify", "active", now, now},
		{"sf-eu", "LE-1", "EU", "欧洲独立站", "DE", "EUR", "shopify", "active", now, now},
	}
	// 站点日事实（scanStorefrontDay 列序）
	nullAny := any(nil)
	db.dayFacts = [][]any{
		{"sf-us", "LE-1", day, "direct", "", "USD", 10000.0, 1000.0, 500.0, 100.0, 200, 60, 3000.0, 500.0, 300.0, 500.0,
			"shopify", "batch-1", 1, day.Add(24 * time.Hour), "production", nullAny, false},
		{"sf-eu", "LE-1", day, "direct", "", "EUR", 8000.0, 800.0, 400.0, 80.0, 160, 50, 2400.0, 400.0, 240.0, 400.0,
			"shopify", "batch-1", 1, day.Add(24 * time.Hour), "production", nullAny, false},
	}
	// campaign 日事实（paid）
	db.campaignFacts = [][]any{
		{"sf-us", "LE-1", "camp_direct", "camp_direct", day, "paid", "direct", 2000.0, int64(60000), int64(6000), 200.0, "INV-1", "USD",
			"ad_invoice", "batch-1", 1, day.Add(24 * time.Hour), "production", nullAny, false},
		{"sf-eu", "LE-1", "camp_direct", "camp_direct", day, "paid", "direct", 1600.0, int64(48000), int64(4800), 160.0, "INV-2", "EUR",
			"ad_invoice", "batch-1", 1, day.Add(24 * time.Hour), "production", nullAny, false},
	}
	db.glRevenue = [][]any{{8600.0, "USD", "gl_revenue", "batch-1", 1, now}}
	db.fixedCost = [][]any{{1500.0, "USD", "overhead"}}
	// 对账三方：1 payout 干净匹配 + 1 payout 在途（无银行行）
	db.payouts = [][]any{
		{"shopify_payments", "PO-1", day, "USD", 5000.0, 150.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 4850.0, "production"},
		{"shopify_payments", "PO-2", day, "USD", 3000.0, 90.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 2910.0, "production"},
	}
	db.banks = [][]any{{"BNK-PO-1", day.Add(24 * time.Hour), "USD", 4850.0, "production"}}
	db.receivables = [][]any{{"PO-1", "USD", 5000.0}, {"PO-2", "USD", 3000.0}}
	db.reserveEvents = [][]any{}
	return db
}

// ---------------------------------------------------------------------------
// 测试主体

func TestEcomGolden(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := ecomGoldenFixture()
	repo := repository.NewEcommerceRepository(db)
	handler := NewEcommerceHandler(repo, nil)

	routes := gin.New()
	routes.Use(func(c *gin.Context) {
		c.Set("legal_entity_id", "LE-1")
		c.Set("user_id", "usr-1")
	})
	// 临时路由（golden 只测 handler 形状，不走权限中间件）
	p := routes.Group("/api/v1")
	p.GET("/ecom/site-pulse", handler.SitePulse)
	p.GET("/ecom/sites/:id/diagnostics", handler.SiteDiagnostics)
	p.GET("/ecom/sites/:id/pnl", handler.SitePnl)
	p.POST("/ecom/settlement/runs", handler.CreateSettlementRun)

	cases := []struct {
		name    string
		method  string
		path    string
		body    string
		golden  string
		scrub   func(t *testing.T, body []byte) []byte
	}{
		{
			name: "site-pulse", method: http.MethodGet,
			path:   "/api/v1/ecom/site-pulse?data_classification=production&as_of=2026-08-02&window_days=7",
			golden: "ecom_site_pulse.json",
			scrub:  scrubEcomEnvelope,
		},
		{
			name: "site-diagnostics", method: http.MethodGet,
			path:   "/api/v1/ecom/sites/sf-us/diagnostics?data_classification=production&as_of=2026-08-02&window_days=7",
			golden: "ecom_site_diagnostics.json",
			scrub:  scrubEcomEnvelope,
		},
		{
			name: "site-pnl", method: http.MethodGet,
			path:   "/api/v1/ecom/sites/sf-us/pnl?period=2026-08&data_classification=production",
			golden: "ecom_site_pnl.json",
			scrub:  scrubEcomEnvelope,
		},
		{
			name: "settlement-run", method: http.MethodPost,
			path:   "/api/v1/ecom/settlement/runs",
			body:   `{"storefront_id":"sf-us","period":"2026-08"}`,
			golden: "ecom_settlement_run.json",
			scrub:  scrubSettlementRun,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var req *http.Request
			if tc.method == http.MethodPost {
				req = httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Idempotency-Key", "golden-req-1")
			} else {
				req = httptest.NewRequest(tc.method, tc.path, nil)
			}
			w := httptest.NewRecorder()
			routes.ServeHTTP(w, req)
			if w.Code != http.StatusOK && w.Code != http.StatusCreated {
				t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
			}
			body := tc.scrub(t, w.Body.Bytes())
			goldenPath := filepath.Join("testdata", "golden", tc.golden)
			if os.Getenv("UPDATE_ECOM_GOLDEN") == "1" {
				if err := os.WriteFile(goldenPath, body, 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
				return
			}
			expected, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden: %v（UPDATE_ECOM_GOLDEN=1 生成）", err)
			}
			if string(body) != string(expected) {
				t.Fatalf("golden mismatch:\n got: %s\nwant: %s", body, expected)
			}
		})
	}
}

// scrubEcomEnvelope 删除挂钟字段（generated_at / highest_as_of / created_at）与
// 会话相关的 created_by。
func scrubEcomEnvelope(t *testing.T, body []byte) []byte {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	var walk func(any)
	walk = func(v any) {
		switch node := v.(type) {
		case map[string]any:
			for k, child := range node {
				switch k {
				case "generated_at", "highest_as_of", "created_at", "updated_at":
					delete(node, k)
				default:
					walk(child)
				}
			}
		case []any:
			for _, child := range node {
				walk(child)
			}
		}
	}
	walk(doc)
	normalized, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("re-marshal: %v", err)
	}
	return normalized
}

func scrubSettlementRun(t *testing.T, body []byte) []byte {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	for _, k := range []string{"created_at", "updated_at", "created_by", "id", "legal_entity_id", "storefront_id"} {
		delete(doc, k)
	}
	normalized, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("re-marshal: %v", err)
	}
	return normalized
}
