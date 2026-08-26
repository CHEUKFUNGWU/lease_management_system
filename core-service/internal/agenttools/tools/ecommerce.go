// 电商独立站模式的 Agent 工具面（spec §5 一次定全七个）。
//
// 命名判定（Agent_Tool_包装规范「看数据属于哪个域」）：
//   - retail.site_pulse.read / retail.site_diagnostics.read / retail.site.scenario.evaluate
//     读的是经营域（站点脉搏/诊断/情景）——归 retail.；
//   - fpna.site_pnl.read / fpna.settlement.read / fpna.settlement_recon_draft.create /
//     fpna.ecom_assumption.suggest 读的是财务报表面与 GL 对账域——归 fpna.。
//
// 读类只读（不落任何表）；写类一律只写草稿层（source=ai_suggestion / memo draft），
// approved-only 读取永不回采草稿；scope_denied 的原因原样透传，不软化。
package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/finmodel/memo"
	"github.com/lease-management-system/core-service/internal/finmodel/suggestion"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/ecomfact"
	"github.com/lease-management-system/core-service/internal/services/ecomkpi"
	"github.com/lease-management-system/core-service/internal/services/ecompulse"
	"github.com/lease-management-system/core-service/internal/services/ecomsim"
	"github.com/lease-management-system/core-service/internal/services/sitepnl"
	"github.com/lease-management-system/core-service/internal/services/unitecon"
)

const ecommerceSkill = "retail_operations" // 与既有零售运营工具同一技能族（模型可预测地找到）

// EcomReader 电商域工具的应用接缝（*repository.EcommerceRepository 满足）。
// 保留这里而不让 runtime 拿到 db 句柄或 HTTP handler。
type EcomReader interface {
	ecomfact.FactReader
	ListStorefronts(ctx context.Context, entity access.EntityFilter) ([]*repository.Storefront, error)
	GetStorefront(ctx context.Context, entity access.EntityFilter, id string) (*repository.Storefront, error)
	LatestGLRevenue(ctx context.Context, entity access.EntityFilter, storefrontID, period string) (*repository.GLRevenueRow, error)
	LatestFixedCost(ctx context.Context, entity access.EntityFilter, storefrontID, period string) (*repository.FixedCostRow, error)
	ListSettlementRuns(ctx context.Context, entity access.EntityFilter, storefrontID, period string) ([]*repository.SettlementRun, error)
	GetSettlementRun(ctx context.Context, entity access.EntityFilter, id string) (*repository.SettlementRun, error)
	ListPayoutLines(ctx context.Context, entity access.EntityFilter, storefrontID string, from, to time.Time) ([]repository.PayoutLineRow, error)
	ListBankLines(ctx context.Context, entity access.EntityFilter, storefrontID string, from, to time.Time) ([]repository.BankLineRow, error)
	ListReceivablesByPayout(ctx context.Context, entity access.EntityFilter, storefrontID string, from, to time.Time) ([]repository.ReceivableRow, error)
	ListReserveEvents(ctx context.Context, entity access.EntityFilter, storefrontID string) ([]repository.ReserveEventRow, error)
}

// ecomWindowProps 表格化窗口参数（无外层花括号，供 properties 对象内嵌）。
var ecomWindowProps = `
	"as_of":{"type":"string","description":"YYYY-MM-DD；缺省截至昨天"},
	"window_days":{"type":"integer","minimum":1,"maximum":366,"description":"窗口天数，缺省 7"},
	"data_classification":{"type":"string","enum":["production","simulated"],"description":"显式指定读哪类数据（两类永不混读）"},
	"dataset_version":{"type":"string","description":"simulated 数据必带模拟数据集版本"}`

func entityFromContext(execution agenttools.ExecutionContext) (access.EntityFilter, error) {
	entity, err := access.FromScope(execution.Principal.Scope)
	if err != nil {
		return access.EntityFilter{}, err
	}
	return entity, nil
}

// NewSitePulseDefinition retail.site_pulse.read：三站一页脉搏（与 /ecom/site-pulse 同源组装）。
func NewSitePulseDefinition(reader EcomReader) agenttools.ToolDefinition {
	return ecomReadDefinition("retail.site_pulse.read", "站点脉搏（电商）",
		"各站点窗口内净收入 / CM1 率 / MER / 退款率 vs 前窗的并列与差值、前 3 差异因子、被重述期间；来源信封与 Decision Ready 判定随行。",
		json.RawMessage(fmt.Sprintf(`{"type":"object","required":["data_classification"],"properties":{%s},"additionalProperties":false}`, ecomWindowProps)),
		[]string{ecommerceSkill}, func(ctx context.Context, call agenttools.ToolCall, execution agenttools.ExecutionContext, raw json.RawMessage) (any, []agenttools.ToolSource, error) {
			var args struct {
				AsOf              string `json:"as_of"`
				WindowDays        int    `json:"window_days"`
				DataClassification string `json:"data_classification"`
				DatasetVersion    string `json:"dataset_version"`
			}
			if err := decodeStrictAny(raw, &args); err != nil {
				return nil, nil, err
			}
			if args.DataClassification != "production" && args.DataClassification != "simulated" {
				return nil, nil, errors.New("data_classification 仅允许 production|simulated")
			}
			entity, err := entityFromContext(execution)
			if err != nil {
				return nil, nil, err
			}
			cur, prev, err := windows(ecomWindow{asOf: args.AsOf, windowDays: args.WindowDays})
			if err != nil {
				return nil, nil, err
			}
			result, err := ecompulse.Compute(ctx, reader, entity, args.DataClassification, args.DatasetVersion, cur, prev)
			if err != nil {
				return nil, nil, err
			}
			return result, nil, nil
		})
}

// NewSiteDiagnosticsDefinition retail.site_diagnostics.read：站点诊断（KPI 全集 + CAC + 保本）。
func NewSiteDiagnosticsDefinition(reader EcomReader) agenttools.ToolDefinition {
	return ecomReadDefinition("retail.site_diagnostics.read", "站点诊断（电商）",
		"单站点窗口内 KPI 全集（GMV/净收入/CM/退款率/AOV）、付费新客与混合 CAC 双口径（分子分母标明）、保本 MER/ROAS；缺数显示为具名 unavailable 并给出原因。",
		json.RawMessage(fmt.Sprintf(`{"type":"object","required":["storefront_id","data_classification"],"properties":{"storefront_id":{"type":"string"},%s},"additionalProperties":false}`, ecomWindowProps)),
		[]string{ecommerceSkill}, func(ctx context.Context, call agenttools.ToolCall, execution agenttools.ExecutionContext, raw json.RawMessage) (any, []agenttools.ToolSource, error) {
			args, err := decodeSiteArgs(raw)
			if err != nil {
				return nil, nil, err
			}
			entity, err := entityFromContext(execution)
			if err != nil {
				return nil, nil, err
			}
			site, err := reader.GetStorefront(ctx, entity, args.StorefrontID)
			if err != nil {
				return nil, nil, err
			}
			cur, _, err := windows(ecomWindow{asOf: args.AsOf, windowDays: args.WindowDays})
			if err != nil {
				return nil, nil, err
			}
			filter := ecomfact.StorefrontFilter{Entity: entity, StorefrontIDs: []string{site.ID}}
			facts, err := reader.StorefrontDays(ctx, filter, cur)
			if err != nil {
				return nil, nil, err
			}
			facts = ecompulse.FilterStorefrontClassification(facts, args.DataClassification, args.DatasetVersion)
			ads, err := reader.CampaignDays(ctx, filter, cur, ecomfact.AdBasisPaid)
			if err != nil {
				return nil, nil, err
			}
			ads = ecompulse.FilterCampaignClassification(ads, args.DataClassification, args.DatasetVersion)
			codes := []string{"gmv", "net_revenue", "cm1", "cm1_rate", "cm2", "cm2_rate", "aov", "order_count",
				"new_customer_orders", "mer", "roas", "refund_rate", "ad_spend_paid", "tax_collected", "landed_cost",
				"fulfillment_cost", "payment_fee"}
			partitions, coverage := ecomkpi.EvaluateByCurrency(codes, facts, ads, cur)
			cac, breakEven := siteEconomics(facts, ads, site)
			return map[string]any{
				"storefront":     site,
				"window":         map[string]string{"from": cur.From.Format(time.DateOnly), "to": cur.To.Format(time.DateOnly)},
				"kpis":           partitions,
				"coverage":       coverage,
				"decision_ready": !ecomkpi.CoverageIncomplete(coverage) && len(facts) > 0,
				"cac":            cac,
				"break_even":     breakEven,
			}, []agenttools.ToolSource{{Type: "storefront", ID: site.ID, Title: site.Name}}, nil
		})
}

// NewSiteScenarioEvaluateDefinition retail.site.scenario.evaluate：大促保本情景（评估不落库）。
func NewSiteScenarioEvaluateDefinition(reader EcomReader) agenttools.ToolDefinition {
	return ecomReadDefinition("retail.site.scenario.evaluate", "大促保本情景（电商）",
		"输入广告预算 + 预期 CPM/CPC/CVR/AOV 与 CM1 率，输出 GMV / CM / 保本 MER 与 ROAS（CM1 率 ≤ 0 报 unachievable）+ 现金缺口静态提示；输出顶层 data_classification=simulated，永不落正式表。",
		json.RawMessage(`{"type":"object","required":["input"],"properties":{"input":{"type":"object","required":["ad_budget","cm1_rate"],"properties":{
			"ad_budget":{"type":"number"},"cpm":{"type":"number"},"cpc":{"type":"number"},"cvr":{"type":"number"},
			"aov":{"type":"number"},"cm1_rate":{"type":"number"},"fixed_cost":{"type":"number"},"target_profit":{"type":"number"},
			"currency":{"type":"string"},"inventory_outlay":{"type":"number"},"payout_lag_days":{"type":"integer"},"reserve_hold_pct":{"type":"number"}
		}}},"additionalProperties":false}`),
		[]string{ecommerceSkill}, func(ctx context.Context, call agenttools.ToolCall, execution agenttools.ExecutionContext, raw json.RawMessage) (any, []agenttools.ToolSource, error) {
			var args struct {
				Input ecomsim.BFCMInput `json:"input"`
			}
			if err := decodeStrictAny(raw, &args); err != nil {
				return nil, nil, err
			}
			out := ecomsim.EvaluateBFCM(ctx, args.Input, nil)
			return out, nil, nil
		})
}

// NewSitePnlDefinition fpna.site_pnl.read：单站利润表（纯函数投影 + GL 会计块）。
func NewSitePnlDefinition(reader EcomReader) agenttools.ToolDefinition {
	return ecomReadDefinition("fpna.site_pnl.read", "单站利润表（电商）",
		"站点月度/周度利润表：GMV→净收入→落地→履约→支付费→CM1→广告(实付)→CM2→分摊固定费→经营利润；会计口径收入块只读来自 GL 导入；子合计由子行推导；下钻维度 channel/campaign/sku。",
		json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{
			"storefront_id":{"type":"string"},"period":{"type":"string","description":"YYYY-MM（月度）"},
			"from":{"type":"string","description":"周度起 YYYY-MM-DD"},"to":{"type":"string","description":"周度止 YYYY-MM-DD"},
			"breakdown":{"type":"string","enum":["none","channel","campaign","sku"]},
			"currency":{"type":"string","description":"目标币种分区；缺省全部分区"},
			"target_profit":{"type":"number"}
		}}`),
		[]string{"fpna_copilot", ecommerceSkill}, func(ctx context.Context, call agenttools.ToolCall, execution agenttools.ExecutionContext, raw json.RawMessage) (any, []agenttools.ToolSource, error) {
			var args struct {
				StorefrontID string  `json:"storefront_id"`
				Period       string  `json:"period"`
				From         string  `json:"from"`
				To           string  `json:"to"`
				Breakdown    string  `json:"breakdown"`
				Currency     string  `json:"currency"`
				TargetProfit *float64 `json:"target_profit"`
			}
			if err := decodeStrictAny(raw, &args); err != nil {
				return nil, nil, err
			}
			entity, err := entityFromContext(execution)
			if err != nil {
				return nil, nil, err
			}
			site, err := reader.GetStorefront(ctx, entity, args.StorefrontID)
			if err != nil {
				return nil, nil, err
			}
			period, err := sitePeriod(args.Period, args.From, args.To)
			if err != nil {
				return nil, nil, err
			}
			stmt, err := sitepnl.Project(ctx, sitepnl.SitePnlRequest{
				Storefront: ecomfact.StorefrontRef{LegalEntityID: site.LegalEntityID, StorefrontID: site.ID},
				Currency:   args.Currency,
				Period:     period,
				Breakdown:  sitepnl.Breakdown(args.Breakdown),
				TargetProfit: args.TargetProfit,
			}, sitepnl.Readers{
				Facts: reader,
				GL:    &ecomAgentGLReader{reader: reader, entity: entity},
				Fixed: &ecomAgentFixedReader{reader: reader, entity: entity},
			})
			if err != nil {
				return nil, nil, err
			}
			return stmt, []agenttools.ToolSource{{Type: "storefront", ID: site.ID, Title: site.Name}}, nil
		})
}

type ecomAgentGLReader struct {
	reader EcomReader
	entity access.EntityFilter
}

func (a *ecomAgentGLReader) GLRevenue(ctx context.Context, ref ecomfact.StorefrontRef, period sitepnl.Period) (*sitepnl.GLRevenue, error) {
	if period.Kind != sitepnl.PeriodMonthly && period.Kind != "" {
		return nil, nil
	}
	row, err := a.reader.LatestGLRevenue(ctx, a.entity, ref.StorefrontID, period.Month)
	if err != nil || row == nil {
		return nil, err
	}
	return &sitepnl.GLRevenue{Amount: row.Amount, Currency: row.Currency, SourceSystem: row.SourceSystem,
		ImportBatchID: row.ImportBatchID, FactVersion: row.FactVersion, AsOfAt: row.AsOfAt}, nil
}

type ecomAgentFixedReader struct {
	reader EcomReader
	entity access.EntityFilter
}

func (a *ecomAgentFixedReader) FixedCost(ctx context.Context, ref ecomfact.StorefrontRef, period sitepnl.Period) (*sitepnl.FixedCost, error) {
	if period.Kind != sitepnl.PeriodMonthly && period.Kind != "" {
		return nil, nil
	}
	row, err := a.reader.LatestFixedCost(ctx, a.entity, ref.StorefrontID, period.Month)
	if err != nil || row == nil {
		return nil, err
	}
	return &sitepnl.FixedCost{Amount: row.Amount, Currency: row.Currency, SourceSystem: row.SourceSystem}, nil
}

// NewSettlementReadDefinition fpna.settlement.read：对账 run 与门禁（GL 对账域）。
func NewSettlementReadDefinition(reader EcomReader) agenttools.ToolDefinition {
	return ecomReadDefinition("fpna.settlement.read", "收款对账（电商）",
		"站点某期间的对账 run：三方匹配结果（六类具名差异：手续费/汇兑/拒付/在途/调整/准备金）、口径门禁裁决（allow/deny）、签认状态（Draft→Prepared→Pending→Approved）与滚动准备金头寸。",
		json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{
			"storefront_id":{"type":"string"},"period":{"type":"string","description":"YYYY-MM"},
			"run_id":{"type":"string","description":"指定 run；缺省按期间查最新"}
		}}`),
		[]string{"fpna_copilot", ecommerceSkill}, func(ctx context.Context, call agenttools.ToolCall, execution agenttools.ExecutionContext, raw json.RawMessage) (any, []agenttools.ToolSource, error) {
			var args struct {
				StorefrontID string `json:"storefront_id"`
				Period       string `json:"period"`
				RunID        string `json:"run_id"`
			}
			if err := decodeStrictAny(raw, &args); err != nil {
				return nil, nil, err
			}
			entity, err := entityFromContext(execution)
			if err != nil {
				return nil, nil, err
			}
			if args.RunID != "" {
				run, err := reader.GetSettlementRun(ctx, entity, args.RunID)
				if err != nil {
					return nil, nil, err
				}
				return run, settlementSource(run), nil
			}
			runs, err := reader.ListSettlementRuns(ctx, entity, args.StorefrontID, args.Period)
			if err != nil {
				return nil, nil, err
			}
			if len(runs) == 0 {
				return map[string]any{"runs": []*repository.SettlementRun{}, "note": "该期间尚无对账 run（差异不会静默消失：未对平期间不得进入 Approved）"}, nil, nil
			}
			return map[string]any{"runs": runs}, settlementSource(runs[0]), nil
		})
}

func settlementSource(run *repository.SettlementRun) []agenttools.ToolSource {
	return []agenttools.ToolSource{{Type: "settlement_run", ID: run.ID, Title: run.StorefrontID + " " + run.Period}}
}

// NewSettlementReconDraftDefinition fpna.settlement_recon_draft.create：对账签认建议落草稿。
func NewSettlementReconDraftDefinition(writer DecisionMemoDraftWriter) agenttools.ToolDefinition {
	return agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name:        "fpna.settlement_recon_draft.create",
			Version:     "v1",
			DisplayName: "对账签认建议（草稿）",
			Description: "对 settlement run 的差异归因与签认建议写为备忘草稿（assist_mode，Review Gate 必过）；approved-only 读取永不回采该草稿。",
			Level:       agenttools.LevelDraft,
			ReadOnly:    false,
			Permissions: []agenttools.Permission{{Resource: "fpna_memos", Action: "write"}},
			InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["run_id","title","narrative"],
				"properties":{
					"run_id":{"type":"string"},"title":{"type":"string"},
					"narrative":{"type":"string","description":"AI 叙述：只允许认领 run 差异清单里已具名的金额与类别"},
					"source_references":{"type":"array"},
					"system_facts":{"type":"object"}
				}}`),
			OutputSchema: json.RawMessage(`{"type":"object","required":["draft","formal_state"]}`),
			Review:       agenttools.ReviewPolicy{Required: true, Reasons: []string{"assist_mode", "settlement_recon_draft"}, AllowedRoles: []string{"reviewer", "approver"}, ConfirmAction: "confirm_settlement_recon"},
			SupportsDryRun:      true,
			SupportsIdempotency: true,
			MaxRows:             1,
			TimeoutSeconds:      30,
		},
		SkillIDs: []string{"fpna_copilot", ecommerceSkill},
		Handler: func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
			if writer == nil {
				return agenttools.ToolResult{}, errors.New("decision memo writer unavailable")
			}
			execution, err := agenttools.RequireExecutionContext(ctx)
			if err != nil {
				return agenttools.ToolResult{}, err
			}
			var args struct {
				RunID     string          `json:"run_id"`
				Title     string          `json:"title"`
				Narrative string          `json:"narrative"`
				Refs      json.RawMessage `json:"source_references"`
				Facts     json.RawMessage `json:"system_facts"`
			}
			dec := json.NewDecoder(strings.NewReader(string(call.Arguments)))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&args); err != nil || strings.TrimSpace(args.RunID) == "" || strings.TrimSpace(args.Title) == "" {
				return agenttools.ToolResult{}, errors.New("run_id 与 title 必填")
			}
			if call.DryRun {
				return agenttools.ToolResult{CallID: call.CallID, Status: agenttools.StatusCompleted, Data: map[string]any{
					"draft": map[string]any{"run_id": args.RunID, "title": args.Title}, "side_effects": false, "dry_run": true,
				}}, nil
			}
			if call.IdempotencyKey == "" {
				return agenttools.ToolResult{}, errors.New("idempotency_key is required for draft writes")
			}
			facts := json.RawMessage(`{}`)
			if len(args.Facts) > 0 {
				facts = args.Facts
			}
			refs := json.RawMessage(`[]`)
			if len(args.Refs) > 0 {
				refs = args.Refs
			}
			legal := execution.Principal.Scope.LegalEntityID
			var entity *string
			if legal != "" {
				entity = &legal
			}
			memoNarr, _ := json.Marshal([]memo.NarrativeItem{{Key: "settlement_recon", Explanation: args.Narrative}})
			item, err := writer.CreateMemo(ctx, &repository.FPnADecisionMemo{
				LegalEntityID: entity, MemoType: "settlement_recon", Title: args.Title,
				Basis: "Draft", Status: "draft",
				SystemFacts:               facts,
				DeterministicCalculations: json.RawMessage(`{"deltas":{},"residuals":{}}`),
				HumanInputs:               json.RawMessage(`{}`),
				AINarrative:               memoNarr,
				SourceReferences:          refs,
				IdempotencyKey:            call.IdempotencyKey,
				CreatedBy:                 &execution.Principal.UserID,
			})
			if err != nil {
				return agenttools.ToolResult{}, errors.New("failed to create settlement recon draft: " + err.Error())
			}
			return agenttools.ToolResult{CallID: call.CallID, Status: agenttools.StatusCompleted, Data: map[string]any{
				"draft": item, "review_required": true, "formal_state": "settlement_recon_draft", "side_effects": true,
			}, Review: agenttools.ReviewResult{Required: true, Reasons: []string{"assist_mode", "settlement_recon_draft"}},
				Sources: []agenttools.ToolSource{{Type: "fpna_decision_memo", ID: item.ID, Title: item.Title, Locator: "assist_draft"}}}, nil
		},
	}
}

// NewEcomAssumptionSuggestionDefinition fpna.ecom_assumption.suggest：预算/假设建议只落草稿。
func NewEcomAssumptionSuggestionDefinition(writer AssumptionDraftWriter) agenttools.ToolDefinition {
	return agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name:        "fpna.ecom_assumption.suggest",
			Version:     "v1",
			DisplayName: "电商预算假设建议（草稿）",
			Description: "对站点预算与经营假设（目标 MER、CM1 率、广告预算占比等）只提交建议草稿（source=ai_suggestion）；财务批准后才生效，AI 无 approved 路径（Growth 角色提交建议的流程化入口）。",
			Level:       agenttools.LevelDraft,
			ReadOnly:    false,
			Permissions: []agenttools.Permission{{Resource: "fin_models", Action: "write"}},
			InputSchema: json.RawMessage(`{"type":"object","required":["suggestions"],"properties":{
				"suggestions":{"type":"array","minItems":1,"items":{"type":"object","required":["assumption_key","category","value","basis","confidence"],"properties":{
					"assumption_key":{"type":"string"},"category":{"type":"string"},"value":{},"unit":{"type":"string"},
					"basis":{"type":"array","minItems":1,"items":{"type":"object","required":["tool_call_id","scope"],"properties":{
						"tool_call_id":{"type":"string"},"scope":{"type":"string"},"period":{"type":"string"}}}},
					"confidence":{"type":"number"},"source_tag":{"type":"string"}
				}}}
			}}`),
			OutputSchema:        json.RawMessage(`{"type":"object","required":["draft_ids","status"]}`),
			Review:              agenttools.ReviewPolicy{Required: true, Reasons: []string{"ai_suggestion_draft", "assumptions_unconfirmed"}, ConfirmAction: "confirm"},
			SupportsDryRun:      true,
			SupportsIdempotency: true,
			MaxRows:             100,
			TimeoutSeconds:      30,
		},
		SkillIDs: []string{"fpna_copilot", ecommerceSkill},
		Handler: func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
			if writer == nil {
				return agenttools.ToolResult{}, errors.New("assumption draft writer unavailable")
			}
			execution, err := agenttools.RequireExecutionContext(ctx)
			if err != nil {
				return agenttools.ToolResult{}, err
			}
			var args AssumptionSuggestionArguments
			dec := json.NewDecoder(strings.NewReader(string(call.Arguments)))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&args); err != nil || len(args.Suggestions) == 0 {
				return agenttools.ToolResult{}, errors.New("at least one suggestion is required")
			}
			if call.IdempotencyKey == "" {
				return agenttools.ToolResult{}, errors.New("idempotency_key is required for draft writes")
			}
			drafts := make([]suggestion.SuggestionDraft, 0, len(args.Suggestions))
			for _, draft := range args.Suggestions {
				if draft.SourceTag == "" {
					draft.SourceTag = "ai_suggestion"
				}
				if err := draft.Validate(); err != nil {
					return agenttools.ToolResult{}, err
				}
				drafts = append(drafts, draft)
			}
			ids, err := writer.SaveDrafts(ctx, execution.Principal.Scope.LegalEntityID, drafts, call.IdempotencyKey)
			if err != nil {
				return agenttools.ToolResult{}, err
			}
			return agenttools.ToolResult{CallID: call.CallID, Status: agenttools.StatusCompleted,
				Data: map[string]any{"draft_ids": ids, "draft_count": len(ids), "status": "draft", "side_effects": true},
				Review: agenttools.ReviewResult{Required: true, Reasons: []string{"ai_suggestion_draft"}}}, nil
		},
	}
}

// ---------------------------------------------------------------------------
// 小工具

type ecomWindow struct {
	asOf       string
	windowDays int
}

func windows(w ecomWindow) (cur, prev ecomfact.Window, err error) {
	days := w.windowDays
	if days <= 0 {
		days = 7
	}
	end := time.Now().UTC().Truncate(24 * time.Hour).AddDate(0, 0, -1)
	if w.asOf != "" {
		parsed, perr := time.Parse(time.DateOnly, w.asOf)
		if perr != nil {
			return cur, prev, errors.New("as_of 需要 YYYY-MM-DD")
		}
		end = parsed.UTC()
	}
	start := end.AddDate(0, 0, -(days - 1))
	cur = ecomfact.Window{From: start, To: end}
	prev = ecomfact.Window{From: start.AddDate(0, 0, -days), To: start.AddDate(0, 0, -1)}
	return cur, prev, nil
}

type siteDiagnosticsArgs struct {
	StorefrontID      string `json:"storefront_id"`
	AsOf              string `json:"as_of"`
	WindowDays        int    `json:"window_days"`
	DataClassification string `json:"data_classification"`
	DatasetVersion    string `json:"dataset_version"`
}

func decodeSiteArgs(raw json.RawMessage) (siteDiagnosticsArgs, error) {
	var args siteDiagnosticsArgs
	if err := decodeStrictAny(raw, &args); err != nil {
		return args, err
	}
	if strings.TrimSpace(args.StorefrontID) == "" {
		return args, errors.New("storefront_id 必填")
	}
	if args.DataClassification != "production" && args.DataClassification != "simulated" {
		return args, errors.New("data_classification 仅允许 production|simulated")
	}
	return args, nil
}

func sitePeriod(period, from, to string) (sitepnl.Period, error) {
	if period != "" {
		if len(period) != 7 {
			return sitepnl.Period{}, errors.New("period 需要 YYYY-MM")
		}
		return sitepnl.Period{Kind: sitepnl.PeriodMonthly, Month: period}, nil
	}
	if from != "" && to != "" {
		f, err1 := time.Parse(time.DateOnly, from)
		t, err2 := time.Parse(time.DateOnly, to)
		if err1 != nil || err2 != nil {
			return sitepnl.Period{}, errors.New("from/to 需要 YYYY-MM-DD")
		}
		return sitepnl.Period{Kind: sitepnl.PeriodWeekly, From: f, To: t}, nil
	}
	return sitepnl.Period{Kind: sitepnl.PeriodMonthly, Month: time.Now().UTC().Format("2006-01")}, nil
}

// siteEconomics CAC 双口径 + 保本（与 HTTP diagnostics 同一算法）。
func siteEconomics(facts []ecomfact.StorefrontDayFact, ads []ecomfact.CampaignDayFact, site *repository.Storefront) (unitecon.CACReport, unitecon.BreakEvenResult) {
	currency := site.Currency
	spend := 0.0
	seen := false
	for _, a := range ecomfact.HighestCampaignDays(ads) {
		if a.Currency == currency {
			spend += a.SpendAmount
			seen = true
		}
	}
	input := unitecon.CACInput{}
	if seen {
		s := mathRound2(spend)
		input.AdSpendPaid = &s
	}
	newCustomers, orders := 0, 0
	newOK, orderOK := len(facts) > 0, len(facts) > 0
	netRevenue, cm1 := 0.0, 0.0
	cm1OK := true
	for _, f := range ecomfact.HighestStorefrontDays(facts) {
		if f.NewCustomerOrders == nil {
			newOK = false
		} else {
			newCustomers += *f.NewCustomerOrders
		}
		if f.OrderCount == nil {
			orderOK = false
		} else {
			orders += *f.OrderCount
		}
		if f.GMVAmount == nil || f.DiscountAmount == nil || f.RefundAmount == nil || f.ChargebackLoss == nil ||
			f.LandedCostAmount == nil || f.FulfillmentAmount == nil || f.PaymentFeeAmount == nil {
			cm1OK = false
			continue
		}
		nr := *f.GMVAmount - *f.DiscountAmount - *f.RefundAmount - *f.ChargebackLoss
		netRevenue += nr
		cm1 += nr - *f.LandedCostAmount - *f.FulfillmentAmount - *f.PaymentFeeAmount
	}
	if newOK {
		input.PayingNewCustomers = &newCustomers
	}
	if orderOK {
		input.TotalOrders = &orders
	}
	if !cm1OK || netRevenue == 0 {
		return unitecon.CACView(input), unitecon.BreakEvenResult{Status: unitecon.StatusUnachievable, Reason: "cm1_components_missing_or_zero_net_revenue"}
	}
	return unitecon.CACView(input), unitecon.BreakEven(cm1/netRevenue, 0, 0) // 保本固定费为空时仅报 ROAS 可算部分
}

func mathRound2(v float64) float64 { return math.Round(v*100) / 100 }


// decodeStrictAny 严格解码（未知字段拒绝）。
func decodeStrictAny(raw json.RawMessage, target any) error {
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		return errors.New("arguments contain unsupported or invalid fields")
	}
	return nil
}

// ecomReadDefinition 读类工具的公共装配（照 retailReadDefinition 形状）。
func ecomReadDefinition(name, display, description string, schema json.RawMessage, skills []string, read func(context.Context, agenttools.ToolCall, agenttools.ExecutionContext, json.RawMessage) (any, []agenttools.ToolSource, error)) agenttools.ToolDefinition {
	return agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name: name, Version: "v1", DisplayName: display, Description: description,
			Level: agenttools.LevelRead, ReadOnly: true,
			Permissions:    []agenttools.Permission{{Resource: "reports", Action: "read"}},
			InputSchema:    schema,
			OutputSchema:   json.RawMessage(`{"type":"object"}`),
			SupportsDryRun: true, SupportsIdempotency: false, MaxRows: 2000, TimeoutSeconds: 30,
		},
		SkillIDs: skills,
		Handler: func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
			execution, err := agenttools.RequireExecutionContext(ctx)
			if err != nil {
				return rejected(call.CallID, agenttools.ErrorUnauthenticated, "authenticated tool context is required"), nil
			}
			if !containsAny(execution.SkillID, skills) {
				return rejected(call.CallID, agenttools.ErrorPermissionDenied, name+" restricted to "+strings.Join(skills, "/")+" skill"), nil
			}
			data, sources, readErr := read(ctx, call, execution, call.Arguments)
			if readErr != nil {
				return rejected(call.CallID, ecomErrorCode(readErr), ecomPublicError(readErr)), nil
			}
			return agenttools.ToolResult{CallID: call.CallID, Status: agenttools.StatusCompleted, Data: data, Sources: sources}, nil
		},
	}
}

func containsAny(haystack string, needles []string) bool {
	for _, n := range needles {
		if haystack == n {
			return true
		}
	}
	return false
}

func ecomErrorCode(err error) agenttools.ErrorCode {
	if strings.Contains(err.Error(), "scope") || strings.Contains(err.Error(), "permission") {
		return agenttools.ErrorScopeDenied
	}
	if errors.Is(err, repository.ErrEcomNotFound) {
		return agenttools.ErrorNotFound
	}
	return agenttools.ErrorInvalidArguments
}

func ecomPublicError(err error) string {
	return err.Error()
}
