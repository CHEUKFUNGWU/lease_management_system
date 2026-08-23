package tools

// F1：fpna.coa.suggest_template —— AI 生成科目树草稿。
//
// 形状与 fpna.assumptions.suggest 完全同形（决策 2026-08-23）：模型在 chat
// plane 组好 rows JSON 作为 tool arguments 传入，本工具不做任何 LLM 调用，
// 只做校验与落库——LLM 调用若发生在工具内部，token 成本绕过 agentguard
// 记账、结果不可重放、治理链也管不到。工具是手，不是脑子。
//
// 校验链（任一失败拒绝产出，绝不产出残缺模板）：
//  1. template.Parse 通过（结构、公式编译、basis 混合、循环）
//  2. 全部 16 个保留键齐全（D-F1：缺任一键 T1–T16 无法运行）
//  3. 每个自定义行声明了存量/流量（D-F4：不给默认值）
//
// 落库恒为模板 draft 状态，source=ai_suggestion；AI 无 approved 路径。
// D-F10 重新生成：给 base_template_id 时按 key 合并——base 已有的行保留
// base 版本并标冲突，generated 独有的行追加，base 独有的行保留。

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/google/uuid"
	"github.com/lease-management-system/core-service/internal/finmodel/template"
	"github.com/lease-management-system/core-service/internal/repository"
)

// CoaSuggestArguments is the strict input. Rows are the model-authored
// subject tree; industry rides along as provenance metadata.
type CoaSuggestArguments struct {
	Industry       string             `json:"industry"`
	Name           string             `json:"name"`
	Rows           []template.RowDef  `json:"rows"`
	BaseTemplateID string             `json:"base_template_id,omitempty"` // D-F10 重新生成的基线
}

// CoaTemplateDraftStore is the persistence seam: drafts only, never an
// approved path (人工确认接口在外，不在工具里).
type CoaTemplateDraftStore interface {
	SaveCoaTemplateDraft(ctx context.Context, legalEntityID, createdBy string, def template.TemplateDef) (id string, version int, err error)
	LoadTemplateRows(ctx context.Context, callerLegalEntityID, templateID string) ([]template.RowDef, error)
}

// CoaSuggestResult reports what was persisted and what the regeneration
// merge decided.
type CoaSuggestResult struct {
	TemplateID    string           `json:"template_id"`
	Name          string           `json:"name"`
	Version       int              `json:"version"`
	Status        string           `json:"status"`
	Source        string           `json:"source"`
	Industry      string           `json:"industry"`
	Merged        bool             `json:"merged"`
	Conflicts     []CoaRowConflict `json:"conflicts,omitempty"`
	ReservedCount int              `json:"reserved_key_count"`
	SideEffects   bool             `json:"side_effects"`
	Review        bool             `json:"review_required"`
}

// CoaRowConflict marks a key present on both sides of a regeneration: the
// base row was kept and the user decides.
type CoaRowConflict struct {
	Key         string `json:"key"`
	BaseKept    bool   `json:"base_kept"`
	Description string `json:"description,omitempty"`
}

func NewCoaSuggestTemplateDefinition(store CoaTemplateDraftStore) agenttools.ToolDefinition {
	return agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name: "fpna.coa.suggest_template", Version: "v1",
			DisplayName: "生成科目树草稿",
			Description: "把模型按行业组出的科目树 rows 校验后落为模板草稿（status=draft，source=ai_suggestion），人确认后才可用。生成必须包含全部 16 个保留键（勾稽读取点），每个自定义行必须声明存量/流量；可给 base_template_id 在已有草稿上重新生成而不覆盖人工改动。",
			Level:       agenttools.LevelDraft, ReadOnly: false,
			Permissions: []agenttools.Permission{{Resource: "statement_templates", Action: "write"}},
			InputSchema: json.RawMessage(`{
  "type":"object",
  "additionalProperties":false,
  "required":["industry","name","rows"],
  "properties":{
    "industry":{"type":"string","minLength":1,"description":"行业类型，如 saas、跨境电商；作为 provenance 元数据回显"},
    "name":{"type":"string","minLength":1,"description":"草稿模板名"},
    "rows":{"type":"array","minItems":1,"items":{"type":"object","required":["key","label","kind","basis"],"properties":{
      "key":{"type":"string"},"label":{"type":"string"},
      "kind":{"type":"string","enum":["input","link","formula","subtotal","check"]},
      "basis":{"type":"string","enum":["operating_basis","ifrs16_basis","shared"]},
      "source":{"type":"string"},"formula":{"type":"string"},
      "children":{"type":"array","items":{"type":"string"}},
      "subtract":{"type":"array","items":{"type":"string"}},
      "fold":{"type":"string","enum":["stock","flow"]},
      "actual_source":{"type":"string"}
    }}},
    "base_template_id":{"type":"string"}
  }
}`),
			Review: agenttools.ReviewPolicy{
				Required: true, Reasons: []string{"ai_suggestion_draft", "coa_human_confirmation"},
				AllowedRoles: []string{"reviewer", "approver"}, ConfirmAction: "confirm_coa_template",
			},
			Retry:               agenttools.RetryPolicy{Retryable: true, MaxAttempts: 2},
			SupportsDryRun:      true,
			SupportsIdempotency: true,
			MaxRows:             5000,
			TimeoutSeconds:      30,
		},
		SkillIDs: []string{"fpna_copilot"},
		Handler: func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
			if store == nil {
				return rejected(call.CallID, agenttools.ErrorDataUnavailable, "coa template store unavailable"), nil
			}
			execution, err := agenttools.RequireExecutionContext(ctx)
			if err != nil {
				return rejected(call.CallID, agenttools.ErrorUnauthenticated, "authenticated tool context is required"), nil
			}
			legalEntityID := strings.TrimSpace(execution.Principal.Scope.LegalEntityID)
			if legalEntityID == "" {
				return rejected(call.CallID, agenttools.ErrorScopeDenied, "legal entity scope is required"), nil
			}
			args, err := decodeStrict[CoaSuggestArguments](call.Arguments)
			if err != nil {
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, "arguments contain unsupported or invalid fields"), nil
			}
			args.Industry = strings.TrimSpace(args.Industry)
			args.Name = strings.TrimSpace(args.Name)
			if args.Industry == "" || args.Name == "" || len(args.Rows) == 0 {
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, "industry, name and rows are required"), nil
			}
			def := template.TemplateDef{Name: args.Name, Version: 1, Rows: args.Rows}
			merged := false
			var conflicts []CoaRowConflict
			if strings.TrimSpace(args.BaseTemplateID) != "" {
				baseRows, loadErr := store.LoadTemplateRows(ctx, legalEntityID, strings.TrimSpace(args.BaseTemplateID))
				if loadErr != nil {
					return rejected(call.CallID, reportErrorCode(loadErr), reportPublicError(loadErr)), nil
				}
				def.Rows, conflicts, merged = mergeCoaRows(baseRows, args.Rows)
				def.Version = 1 // 新草稿从 v1 起步；版本冻结语义由存储层保证
			}
			def.Source = "ai_suggestion"

			// 校验链 1：结构 + 公式编译 + basis 混合 + 循环。
			if _, err := template.Parse(def); err != nil {
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, fmt.Sprintf("template invalid: %v", err)), nil
			}
			// 校验链 2：保留键齐全（D-F1/D-F9）——缺任一键拒绝产出。
			keys := make([]string, 0, len(def.Rows))
			for _, row := range def.Rows {
				keys = append(keys, row.Key)
			}
			if missing := template.MissingReservedSheetKeys(keys); len(missing) > 0 {
				return rejected(call.CallID, agenttools.ErrorDataUnavailable,
					fmt.Sprintf("%s；AI 生成缺失保留键：%s", template.ReservedKeysReason, strings.Join(missing, ", "))), nil
			}
			// 校验链 3：自定义行存量/流量必填（D-F4）。保留键继承默认存量语义；
			// 自定义行不给默认值——替用户选一个大概率错的答案等于编造。
			for _, row := range def.Rows {
				if template.IsReservedSheetKey(row.Key) {
					continue
				}
				if row.Fold != template.FoldStock && row.Fold != template.FoldFlow {
					return rejected(call.CallID, agenttools.ErrorInvalidArguments,
						fmt.Sprintf("自定义科目 %q 必须声明 fold（stock 存量 / flow 流量），不给默认值", row.Key)), nil
				}
			}

			id, version, saveErr := store.SaveCoaTemplateDraft(ctx, legalEntityID, execution.Principal.UserID, def)
			if saveErr != nil {
				return rejected(call.CallID, agenttools.ErrorBusinessFailure, "failed to save coa template draft"), nil
			}
			data := CoaSuggestResult{
				TemplateID: id, Name: def.Name, Version: version,
				Status: "draft", Source: "ai_suggestion", Industry: args.Industry,
				Merged: merged, Conflicts: conflicts,
				ReservedCount: len(template.ReservedSheetKeys),
				SideEffects:   true, Review: true,
			}
			out := completed(call.CallID, data)
			out.Review = agenttools.ReviewResult{Required: true, Reasons: []string{"ai_suggestion_draft", "coa_human_confirmation"}}
			return out, nil
		},
	}
}

// mergeCoaRows implements D-F10: base rows keep their position and win
// conflicts (marked), generated-only rows append in order, base-only rows
// stay. Reserved keys always survive because both sides must carry them.
func mergeCoaRows(base, generated []template.RowDef) ([]template.RowDef, []CoaRowConflict, bool) {
	generatedByKey := make(map[string]template.RowDef, len(generated))
	for _, row := range generated {
		generatedByKey[strings.TrimSpace(row.Key)] = row
	}
	baseByKey := make(map[string]template.RowDef, len(base))
	merged := make([]template.RowDef, 0, len(base)+len(generated))
	conflicts := make([]CoaRowConflict, 0)
	for _, row := range base {
		key := strings.TrimSpace(row.Key)
		baseByKey[key] = row
		if generatedRow, exists := generatedByKey[key]; exists && !rowsEqual(row, generatedRow) {
			conflicts = append(conflicts, CoaRowConflict{
				Key: key, BaseKept: true,
				Description: "基线行与生成行不同，已保留基线版本；如需采用生成版本请在编辑器中手工修改",
			})
		}
		merged = append(merged, row)
	}
	for _, row := range generated {
		key := strings.TrimSpace(row.Key)
		if _, exists := baseByKey[key]; !exists {
			merged = append(merged, row)
		}
	}
	return merged, conflicts, len(conflicts) > 0
}

func rowsEqual(a, b template.RowDef) bool {
	aJSON, _ := json.Marshal(a)
	bJSON, _ := json.Marshal(b)
	return string(aJSON) == string(bJSON)
}

// finModelCoaStore is the production persistence adapter over the statement
// template repository: drafts only.
type finModelCoaStore struct{ repo *repository.FinModelRepository }

// NewCoaTemplateStore builds the production store.
func NewCoaTemplateStore(repo *repository.FinModelRepository) CoaTemplateDraftStore {
	return finModelCoaStore{repo: repo}
}

func (s finModelCoaStore) SaveCoaTemplateDraft(ctx context.Context, legalEntityID, createdBy string, def template.TemplateDef) (string, int, error) {
	id := uuid.NewString()
	entity, createdByPtr := legalEntityID, createdBy
	tmpl, err := s.repo.SaveStatementTemplate(ctx, def, &entity, &createdByPtr, id)
	if err != nil {
		return "", 0, err
	}
	return id, tmpl.Major, nil
}

// LoadTemplateRows returns the base rows for a regeneration. The template
// must belong to the caller's legal entity — a foreign template is
// indistinguishable from a missing one (no existence leak, 底线 1).
func (s finModelCoaStore) LoadTemplateRows(ctx context.Context, callerLegalEntityID, templateID string) ([]template.RowDef, error) {
	head, err := s.repo.GetStatementTemplate(ctx, templateID)
	if err != nil || head == nil || head.LegalEntityID == nil || *head.LegalEntityID != callerLegalEntityID {
		return nil, fmt.Errorf("base template not found")
	}
	if head.Status != "draft" {
		return nil, fmt.Errorf("base template is not a draft (status=%s); regeneration layers onto drafts only", head.Status)
	}
	tmpl, err := s.repo.LoadStatementTemplate(ctx, templateID)
	if err != nil {
		return nil, err
	}
	rows := make([]template.RowDef, 0, len(tmpl.Rows))
	for _, row := range tmpl.Rows {
		rows = append(rows, template.RowDef{
			Key: row.Key, Label: row.Label, Kind: row.Kind, Basis: row.Basis,
			Source: row.Source, Formula: row.FormulaText, Children: row.Children,
			Subtract: row.Subtract, Format: row.Format, ActualSource: row.ActualSource,
			Fold: row.Fold,
		})
	}
	return rows, nil
}
