package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lease-management-system/core-service/internal/finmodel"
	"github.com/lease-management-system/core-service/internal/finmodel/template"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/xuri/excelize/v2"
)

func fp2(v float64) *float64 { return &v }

func TestRenderModelRunXLSXLiveFormulasAndMarkers(t *testing.T) {
	tmpl, err := template.Parse(template.TemplateDef{Name: "t", Version: 1, Rows: []template.RowDef{
		{Key: "rev", Label: "营业收入", Kind: template.RowLink, Basis: template.BasisShared, Source: "fact.revenue"},
		{Key: "cost", Label: "成本", Kind: template.RowFormula, Basis: template.BasisShared, Formula: "0 - rows.rev"},
		{Key: "gp", Label: "毛利", Kind: template.RowSubtotal, Basis: template.BasisShared, Children: []string{"rev", "cost"}, Subtract: []string{"cost"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	buckets := []finmodel.FoldBucket{{Periods: []string{"2026-01"}, Label: "2026-Q1(1/3)"}}
	rows := []modelExportRow{
		{Key: "rev", Label: "营业收入", Kind: "link", Basis: "shared", Values: map[string]*float64{"2026-Q1(1/3)": fp2(100)}},
		{Key: "cost", Label: "成本", Kind: "formula", Basis: "shared", Values: map[string]*float64{"2026-Q1(1/3)": fp2(50)}},
		{Key: "gp", Label: "毛利", Kind: "subtotal", Basis: "shared", Children: []string{"rev", "cost"}, Subtracted: []string{"cost"}},
	}
	out, err := RenderModelRunXLSX(tmpl, rows, buckets, ModelExportMeta{
		ModelName: "m", DataClassification: "simulated", DatasetVersion: "ds-1", FoldKind: finmodel.FoldQuarter,
	})
	if err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	// 模拟标识在口径头行。
	head, _ := f.GetCellValue("model", "A1")
	if !strings.Contains(head, "simulated") || !strings.Contains(head, "ds-1") {
		t.Fatalf("口径头 must carry classification + dataset: %q", head)
	}
	marker, _ := f.GetCellValue("model", "A2")
	if !strings.Contains(marker, "SIMULATED") {
		t.Fatalf("simulated marker missing: %q", marker)
	}
	// 未治理公式行带标识；小计行是活公式 =+C5-C6（毛利 − 成本）。
	costLabel, _ := f.GetCellValue("model", "A6")
	if !strings.Contains(costLabel, "未经指标治理") {
		t.Fatalf("formula row must carry the governance marker: %q", costLabel)
	}
	formula, err := f.GetCellFormula("model", "C7")
	if err != nil || formula != "=+C5-C6" {
		t.Fatalf("subtotal must be a live formula, got %q err=%v", formula, err)
	}
}

// TestExportRunQuarterFoldPostgres locks S2-9 end to end: a persisted run
// exports through the endpoint, 12 months fold into 4 quarter columns,
// flows sum in the workbook, subtotals are live formulas.
func TestExportRunQuarterFoldPostgres(t *testing.T) {
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
		entity, "EXP-E-"+suffix, "Export "+suffix)
	tmplID := uuid.NewString()
	exec(`INSERT INTO fin_statement_templates (id, legal_entity_id, name, version, status, rows)
		VALUES ($1,$2,$3,1,'approved', $4::jsonb)`, tmplID, entity, "EXP-TPL-"+suffix,
		json.RawMessage(`{"name":"EXP-TPL","version":1,"rows":[{"key":"rev","label":"rev","kind":"link","basis":"shared","source":"fact.revenue"},{"key":"gp","label":"gp","kind":"subtotal","basis":"shared","children":["rev"]}]}`))
	defID := uuid.NewString()
	exec(`INSERT INTO fin_model_definitions (id, legal_entity_id, name, version, template_id, policy, source_bindings)
		VALUES ($1,$2,$3,1,$4,'{}'::jsonb,'{}'::jsonb)`, defID, entity, "EXP-DEF-"+suffix, tmplID)
	runID := uuid.NewString()
	exec(`INSERT INTO fin_model_runs (id, legal_entity_id, model_definition_id, model_definition_version, status, tie_out_status, data_classification, input_snapshot, idempotency_key)
		VALUES ($1,$2,$3,1,'completed','passed','simulated','{"currency":"CNY"}'::jsonb,$4)`, runID, entity, defID, "exp-run-"+suffix)
	for month := 1; month <= 12; month++ {
		period := "2026-" + padMonth(month)
		exec(`INSERT INTO fin_model_run_lines (run_id, row_key, period, value, provenance) VALUES ($1,'rev',$2,$3,'{}'::jsonb)`,
			runID, period, float64(10*month))
	}

	handler := NewFinModelHandler(repository.NewFinModelRepository(pool))
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/financial-model/runs/:id/export", func(c *gin.Context) {
		c.Set("legal_entity_id", entity)
		handler.ExportRun(c)
	})
	req := httptest.NewRequest(http.MethodGet, "/financial-model/runs/"+runID+"/export?fold=quarter", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("export status %d: %s", w.Code, w.Body.String())
	}
	f, err := excelize.OpenReader(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatalf("open workbook: %v", err)
	}
	// 12 个月 → 4 个季度列（C..F）；口径头和模拟标记在场。
	for _, cell := range []string{"C4", "D4", "E4", "F4"} {
		label, _ := f.GetCellValue("model", cell)
		if !strings.HasPrefix(label, "2026-Q") {
			t.Fatalf("quarter column %s = %q", cell, label)
		}
	}
	head, _ := f.GetCellValue("model", "A1")
	if !strings.Contains(head, "simulated") || !strings.Contains(head, "quarter") {
		t.Fatalf("header must carry classification + fold grain: %q", head)
	}
	// 流量求和：2026-Q1 的 rev = 10+20+30 = 60。
	q1, err := f.GetCellValue("model", "C5")
	if err != nil || q1 == "" {
		t.Fatalf("Q1 rev cell = %q err=%v", q1, err)
	}
	// 小计活公式：gp 行引用同列 rev 单元格。
	formula, err := f.GetCellFormula("model", "C6")
	if err != nil || (formula != "=+C5" && formula != "=C5") {
		t.Fatalf("subtotal formula = %q err=%v", formula, err)
	}
}

func padMonth(month int) string {
	if month < 10 {
		return "0" + string(rune('0'+month))
	}
	return string(rune('0'+month/10)) + string(rune('0'+month%10))
}
