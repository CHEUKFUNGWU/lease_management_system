package tools

// F1：fpna.coa.suggest_template 单元测试。
// 反向 #4：AI 生成缺任一保留键 → 拒绝产出（删掉工具内的保留键校验，本
// 文件的 missing-reserved 用例必须变红）。D-F10 合并语义与校验链在此锁定。

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/finmodel/template"
)

type fakeCoaStore struct {
	saved      []template.TemplateDef
	nextID     int
	baseRows   []template.RowDef
	loadCalled bool
}

func (f *fakeCoaStore) SaveCoaTemplateDraft(_ context.Context, _, _ string, def template.TemplateDef) (string, int, error) {
	f.saved = append(f.saved, def)
	f.nextID++
	id := "tpl-" + string(rune('0'+f.nextID))
	return id, def.Version, nil
}

func (f *fakeCoaStore) LoadTemplateRows(_ context.Context, _, _ string) ([]template.RowDef, error) {
	f.loadCalled = true
	return f.baseRows, nil
}

func coaCtx() context.Context {
	return agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{
			UserID: "bp-f1", Permissions: []string{"statement_templates:write"},
			Scope: access.Scope{LegalEntityID: "entity-a"},
		},
		RunID: "run-1", SkillID: "fpna_copilot", SkillVersion: "v1",
	})
}

// f1ReservedRows returns the sixteen reserved rows as minimal subtotal-free
// declarations (kind input keeps Parse happy without children wiring).
func f1ReservedRows() []template.RowDef {
	rows := make([]template.RowDef, 0)
	for _, key := range template.ReservedSheetKeys {
		rows = append(rows, template.RowDef{
			Key: key, Label: key, Kind: template.RowInput,
			Basis: template.BasisIFRS16, Fold: template.FoldStock,
		})
	}
	return rows
}

func TestCoaSuggestHappyPathPersistsDraftWithAISource(t *testing.T) {
	store := &fakeCoaStore{}
	def := NewCoaSuggestTemplateDefinition(store)
	args := map[string]any{"industry": "saas", "name": "SaaS 科目树草稿", "rows": f1ReservedRows()}
	raw, _ := json.Marshal(args)
	result, err := def.Handler(coaCtx(), factsCall(def.Descriptor.Name, string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != agenttools.StatusCompleted {
		t.Fatalf("expected completed, got %s %+v", result.Status, result.Error)
	}
	data := result.Data.(CoaSuggestResult)
	if data.Status != "draft" || data.Source != "ai_suggestion" || !data.SideEffects || !data.Review {
		t.Fatalf("draft/ai_suggestion contract broken: %+v", data)
	}
	if data.ReservedCount != 16 {
		t.Fatalf("reserved count wrong: %d", data.ReservedCount)
	}
	if len(store.saved) != 1 || store.saved[0].Source != "ai_suggestion" {
		t.Fatalf("persisted def wrong: %+v", store.saved)
	}
}

// 反向 #4：生成结果缺任一保留键 → 拒绝产出，措辞带机械原因与缺失键名。
func TestCoaSuggestRejectsMissingReservedKey(t *testing.T) {
	store := &fakeCoaStore{}
	rows := f1ReservedRows()
	for i, row := range rows {
		if row.Key == "total_assets" {
			rows = append(rows[:i], rows[i+1:]...)
			break
		}
	}
	def := NewCoaSuggestTemplateDefinition(store)
	raw, _ := json.Marshal(map[string]any{"industry": "saas", "name": "残缺草稿", "rows": rows})
	result, err := def.Handler(coaCtx(), factsCall(def.Descriptor.Name, string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	if result.Error == nil || result.Error.Code != agenttools.ErrorDataUnavailable {
		t.Fatalf("missing reserved key must be rejected, got %+v", result.Error)
	}
	if !strings.Contains(result.Error.Message, "total_assets") || !strings.Contains(result.Error.Message, "T1–T16") {
		t.Fatalf("rejection must name the missing key and the mechanical reason: %q", result.Error.Message)
	}
	if len(store.saved) != 0 {
		t.Fatal("nothing may be persisted for a rejected generation")
	}
}

// D-F4：自定义行不声明存量/流量 → 拒绝（不给默认值）。
func TestCoaSuggestRequiresFoldOnCustomRows(t *testing.T) {
	store := &fakeCoaStore{}
	rows := append(f1ReservedRows(), template.RowDef{
		Key: "subscription_revenue", Label: "订阅收入", Kind: template.RowInput,
		Basis: template.BasisShared, // fold 缺失
	})
	def := NewCoaSuggestTemplateDefinition(store)
	raw, _ := json.Marshal(map[string]any{"industry": "saas", "name": "n", "rows": rows})
	result, err := def.Handler(coaCtx(), factsCall(def.Descriptor.Name, string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	if result.Error == nil || result.Error.Code != agenttools.ErrorInvalidArguments {
		t.Fatalf("undeclared fold must be rejected, got %+v", result.Error)
	}
	if !strings.Contains(result.Error.Message, "subscription_revenue") {
		t.Fatalf("rejection must name the row: %q", result.Error.Message)
	}
	if len(store.saved) != 0 {
		t.Fatal("rejected generation must not persist")
	}
}


// D-F10：重新生成按 key 合并——base 保留并标冲突，generated 独有追加，
// base 独有（用户自建行）不丢。
func TestCoaSuggestRegenerateMergesWithoutClobbering(t *testing.T) {
	store := &fakeCoaStore{}
	editedReserved := f1ReservedRows()
	for i := range editedReserved {
		if editedReserved[i].Key == "cash" {
			editedReserved[i].Label = "现金（人工改名）" // 用户改过标签——base 赢
		}
	}
	userRow := template.RowDef{Key: "my_custom_kpi", Label: "我的自定义指标", Kind: template.RowInput, Basis: template.BasisShared, Fold: template.FoldFlow}
	editedReserved = append(editedReserved, userRow) // 用户自建行在基线里
	store.baseRows = editedReserved

	generated := f1ReservedRows() // cash 与 base 不同 → 冲突；其余同
	newRow := template.RowDef{Key: "subscription_revenue", Label: "订阅收入", Kind: template.RowInput, Basis: template.BasisShared, Fold: template.FoldFlow}
	generated = append(generated, newRow)

	def := NewCoaSuggestTemplateDefinition(store)
	raw, _ := json.Marshal(map[string]any{
		"industry": "saas", "name": "regen", "rows": generated,
		"base_template_id": "tpl-base",
	})
	result, err := def.Handler(coaCtx(), factsCall(def.Descriptor.Name, string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	data := result.Data.(CoaSuggestResult)
	if !data.Merged || len(data.Conflicts) != 1 || data.Conflicts[0].Key != "cash" || !data.Conflicts[0].BaseKept {
		t.Fatalf("merge conflicts wrong: %+v", data.Conflicts)
	}
	if len(store.saved) != 1 {
		t.Fatalf("merged draft must be persisted once, got %d", len(store.saved))
	}
	saved := store.saved[0]
	keys := map[string]template.RowDef{}
	for _, row := range saved.Rows {
		keys[row.Key] = row
	}
	if _, ok := keys["my_custom_kpi"]; !ok {
		t.Fatal("user-authored custom row must survive regeneration")
	}
	if _, ok := keys["subscription_revenue"]; !ok {
		t.Fatal("generated-only row must be appended")
	}
	if keys["cash"].Label != "现金（人工改名）" {
		t.Fatalf("conflicting key must keep the base (user) version, got %q", keys["cash"].Label)
	}
}
