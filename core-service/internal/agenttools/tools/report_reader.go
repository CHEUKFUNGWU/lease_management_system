package tools

// B-4 报表只读接入。五个工具覆盖 /reports 的未包装只读路由，全部经
// reporting.SnapshotBuilder + reporting.Project 的同一条确定性投影路径——
// 与 HTTP 面共享同一份实现，不在 Agent 侧重算任何数字。
//
// 口径词表（CONTEXT.md「Report Basis」，本批的命名权威）：
//
//	approved  仅 Approved                正式财务与审计
//	pending   Pending + Approved         待批件复核
//	draft     Draft + Pending + Approved 内部试算，可叠加用户自己的测算
//
// 累积不互斥；working/official 不再是口径名（official 的版本地位由
// is_official_version 承担，与审批状态是两条独立的轴）。线格式 mode 值
// （working/pending/official）作为 legacy 字段回显；公开 ?mode= 参数的收敛
// 是另一个带迁移的专项（AGENTS.md 词表分叉清单），本工具不动那些位置。
//
// draft 口径的叠加层：scenario_drafts / scenario_type='custom' 计划版本 /
// draft 假设版本单独列出并标注为用户自建内容，绝不混入确定性投影数字。
// approved / pending 口径恒不带叠加层（approved-only 读取永不回采 draft，
// 有测试锁定）。

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/reporting"
)

const reportsSkill = "fpna_copilot"

// ReportReader is the read-only report seam: the deterministic projection
// path plus the close-pack composition and the three user-authored surfaces
// the draft basis layers on top. No write methods — report generation state
// is not agent-mutable.
type ReportReader interface {
	Project(ctx context.Context, mode reporting.Mode, request reporting.ProjectionRequest) (reporting.ProjectionResult, error)
	// ClosePack composes the disclosure projection with close exceptions and
	// monthly batches for one period — the same composition the HTTP
	// /reports/close-pack route serves.
	ClosePack(ctx context.Context, mode reporting.Mode, period string) (map[string]any, error)
	ListPlanVersions(ctx context.Context, entity access.EntityFilter, versionType, status, asOfPeriod string) ([]*repository.FPnAPlanVersion, error)
	ListAssumptions(ctx context.Context, entity access.EntityFilter, key string) ([]*repository.FPnAAssumptionVersion, error)
	ListScenarioDrafts(ctx context.Context, entity access.EntityFilter, limit int) ([]*repository.FPnAScenarioDraft, error)
}

type reportBasisArguments struct {
	ReportBasis string `json:"report_basis,omitempty"`
}

type reportScheduleArguments struct {
	reportBasisArguments
	Kind                 string   `json:"kind"`
	StartDate            string   `json:"start_date"`
	EndDate              string   `json:"end_date"`
	View                 string   `json:"view"`
	Granularity          string   `json:"granularity"`
	ContractID           string   `json:"contract_id"`
	Store                string   `json:"store"`
	Tags                 []string `json:"tags"`
	ReportCurrency       string   `json:"report_currency"`
	DiscountRateOverride *float64 `json:"discount_rate_override"`
	ExchangeRate         float64  `json:"exchange_rate"`
}

type reportDisclosurePackageArguments struct {
	reportBasisArguments
	Kind        string `json:"kind"`
	PeriodStart string `json:"period_start"`
	PeriodEnd   string `json:"period_end"`
	Period      string `json:"period"`
}

type reportContractViewArguments struct {
	reportBasisArguments
	Kind         string   `json:"kind"`
	ContractID   string   `json:"contract_id"`
	DiscountRate *float64 `json:"discount_rate"`
}

type reportUnitPriceArguments struct {
	reportBasisArguments
	GroupBy string `json:"group_by"`
}

type reportTagsArguments struct {
	reportBasisArguments
	Kind string `json:"kind"`
}

// ReportProjectionToolData 报表工具统一信封。UserOverlays 仅在 draft 口径
// 非 nil；approved/pending 恒 nil。
type ReportProjectionToolData struct {
	Kind               string         `json:"kind"`
	ReportBasis        string         `json:"report_basis"`
	Mode               string         `json:"mode"`
	IsOfficialVersion  bool           `json:"is_official_version"`
	DataClassification string         `json:"data_classification"`
	UserOverlays       map[string]any `json:"user_overlays,omitempty"`
	Payload            map[string]any `json:"data"`
	SideEffects        bool           `json:"side_effects"`
}

var reportBasisToMode = map[string]reporting.Mode{
	"approved": reporting.Official,
	"pending":  reporting.Pending,
	"draft":    reporting.Working,
}

// resolveReportBasis maps the CONTEXT.md vocabulary onto the projection
// modes. Empty defaults to approved — an agent asking for "the report"
// without qualifiers gets the formal-grade one; seeing drafts is explicit.
func resolveReportBasis(raw string) (reporting.Mode, string, bool) {
	basis := strings.TrimSpace(raw)
	if basis == "" {
		basis = "approved"
	}
	mode, ok := reportBasisToMode[basis]
	return mode, basis, ok
}

// buildUserOverlays reads the three user-authored surfaces. Each section
// keeps its own approval status so a reader can tell what is still Draft.
func buildUserOverlays(ctx context.Context, reader ReportReader, entity access.EntityFilter) (map[string]any, error) {
	plans, err := reader.ListPlanVersions(ctx, entity, "", "", "")
	if err != nil {
		return nil, err
	}
	custom := make([]*repository.FPnAPlanVersion, 0)
	for _, plan := range plans {
		if plan.ScenarioType == "custom" {
			custom = append(custom, plan)
		}
	}
	assumptions, err := reader.ListAssumptions(ctx, entity, "")
	if err != nil {
		return nil, err
	}
	draftAssumptions := make([]*repository.FPnAAssumptionVersion, 0)
	for _, assumption := range assumptions {
		if assumption.Status == "draft" {
			draftAssumptions = append(draftAssumptions, assumption)
		}
	}
	drafts, err := reader.ListScenarioDrafts(ctx, entity, 20)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"note":                 "用户自建测算内容，独立于 data 里的确定性投影数字；approved/pending 口径不返回本节",
		"scenario_drafts":      drafts,
		"custom_plan_versions": custom,
		"draft_assumptions":    draftAssumptions,
	}, nil
}

func reportProjectionResponse(callID, kind, basis string, mode reporting.Mode, payload map[string]any, overlays map[string]any) agenttools.ToolResult {
	data := ReportProjectionToolData{
		Kind: kind, ReportBasis: basis, Mode: string(mode),
		IsOfficialVersion: mode == reporting.Official,
		// 快照输入只有正式租赁域表（合同/付款计划/事件调整），不含模拟事实；
		// data_classification 如实声明 production。
		DataClassification: "production",
		UserOverlays:       overlays, Payload: payload, SideEffects: false,
	}
	return completed(callID, data)
}

func parseFactsDateRange(argsDateFrom, argsDateTo string) (time.Time, time.Time, string) {
	fromText, toText := strings.TrimSpace(argsDateFrom), strings.TrimSpace(argsDateTo)
	if fromText == "" || toText == "" {
		return time.Time{}, time.Time{}, "start_date and end_date are required"
	}
	from, fromErr := time.Parse("2006-01-02", fromText)
	to, toErr := time.Parse("2006-01-02", toText)
	if fromErr != nil || toErr != nil {
		return time.Time{}, time.Time{}, "start_date and end_date must be ISO dates (YYYY-MM-DD)"
	}
	if to.Before(from) {
		return time.Time{}, time.Time{}, "end_date must be >= start_date"
	}
	return from, to, ""
}

var reportScheduleKinds = []string{"amortization", "liability_rolling", "cashflow_forecast"}

func NewReportScheduleDefinition(reader ReportReader) agenttools.ToolDefinition {
	const name = "lease.report.schedule.read"
	schema := `{
  "type":"object",
  "additionalProperties":false,
  "required":["kind"],
  "properties":{
    "kind":{"type":"string","enum":["amortization","liability_rolling","cashflow_forecast"]},
    "report_basis":{"type":"string","enum":["approved","pending","draft"]},
    "start_date":{"type":"string","pattern":"^[0-9]{4}-[0-9]{2}-[0-9]{2}$"},
    "end_date":{"type":"string","pattern":"^[0-9]{4}-[0-9]{2}-[0-9]{2}$"},
    "view":{"type":"string","enum":["contract","store","summary","tag"]},
    "granularity":{"type":"string","enum":["day","month","quarter","half_year","year"]},
    "contract_id":{"type":"string"},
    "store":{"type":"string"},
    "tags":{"type":"array","items":{"type":"string"}},
    "report_currency":{"type":"string"},
    "discount_rate_override":{"type":"number"},
    "exchange_rate":{"type":"number","exclusiveMinimum":0}
  }
}`
	return agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name: name, Version: "v1",
			DisplayName: "读取租赁摊销与滚动报表",
			Description: "读取摊销表（amortization）、租赁负债滚动（liability_rolling）或现金流预测（cashflow_forecast）。口径词表：approved 仅含 Approved；pending 含 Pending+Approved；draft 含全部并可叠加用户自建测算（见 user_overlays）。数字由确定性投影服务计算，Agent 不重算。",
			Level:       agenttools.LevelRead, ReadOnly: true,
			Permissions:    []agenttools.Permission{{Resource: "reports", Action: "read"}},
			InputSchema:    json.RawMessage(schema),
			SupportsDryRun: true, MaxRows: 5000, TimeoutSeconds: 30,
		},
		SkillIDs: []string{reportsSkill},
		Handler: func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
			if reader == nil {
				return rejected(call.CallID, agenttools.ErrorDataUnavailable, "report reader unavailable"), nil
			}
			execution, err := agenttools.RequireExecutionContext(ctx)
			if err != nil {
				return rejected(call.CallID, agenttools.ErrorUnauthenticated, "authenticated tool context is required"), nil
			}
			entity, entityErr := access.FromScope(execution.Principal.Scope)
			if entityErr != nil {
				return rejected(call.CallID, agenttools.ErrorScopeDenied, "legal entity scope is required"), nil
			}
			args, err := decodeStrict[reportScheduleArguments](call.Arguments)
			if err != nil {
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, "arguments contain unsupported or invalid fields"), nil
			}
			mode, basis, ok := resolveReportBasis(args.ReportBasis)
			if !ok {
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, "report_basis must be approved, pending or draft"), nil
			}
			kind := strings.TrimSpace(args.Kind)
			request := reporting.ProjectionRequest{}
			switch kind {
			case "amortization":
				view := defaultString(args.View, reporting.ViewSummary)
				granularity := defaultString(args.Granularity, reporting.GranularityMonth)
				if !oneOfString(granularity, reporting.GranularityDay, reporting.GranularityMonth, reporting.GranularityQuarter, reporting.GranularityHalfYear, reporting.GranularityYear) {
					return rejected(call.CallID, agenttools.ErrorInvalidArguments, "invalid granularity, must be day|month|quarter|half_year|year"), nil
				}
				if !oneOfString(view, reporting.ViewContract, reporting.ViewStore, reporting.ViewSummary, reporting.ViewTag) {
					return rejected(call.CallID, agenttools.ErrorInvalidArguments, "invalid view, must be contract|store|summary|tag"), nil
				}
				start, end, dateErr := parseFactsDateRange(args.StartDate, args.EndDate)
				if dateErr != "" {
					return rejected(call.CallID, agenttools.ErrorInvalidArguments, dateErr), nil
				}
				if args.ExchangeRate < 0 {
					return rejected(call.CallID, agenttools.ErrorInvalidArguments, "exchange_rate must be > 0 when provided"), nil
				}
				request = reporting.ProjectionRequest{Kind: reporting.KindAmortization, View: view, Granularity: granularity,
					StartDate: start, EndDate: end, ContractID: strings.TrimSpace(args.ContractID), Store: strings.TrimSpace(args.Store),
					Tags: args.Tags, Rate: args.DiscountRateOverride, ReportCurrency: strings.TrimSpace(args.ReportCurrency), ExchangeRate: args.ExchangeRate}
			case "liability_rolling":
				request = reporting.ProjectionRequest{Kind: reporting.KindLiabilityRolling}
			case "cashflow_forecast":
				view := defaultString(args.View, reporting.ViewSummary)
				granularity := defaultString(args.Granularity, reporting.GranularityMonth)
				if !oneOfString(granularity, reporting.GranularityMonth, reporting.GranularityQuarter, reporting.GranularityYear) {
					return rejected(call.CallID, agenttools.ErrorInvalidArguments, "invalid granularity, must be month|quarter|year"), nil
				}
				if !oneOfString(view, reporting.ViewContract, reporting.ViewStore, reporting.ViewSummary) {
					return rejected(call.CallID, agenttools.ErrorInvalidArguments, "invalid view, must be contract|store|summary"), nil
				}
				start, end, dateErr := parseFactsDateRange(args.StartDate, args.EndDate)
				if dateErr != "" {
					return rejected(call.CallID, agenttools.ErrorInvalidArguments, dateErr), nil
				}
				request = reporting.ProjectionRequest{Kind: reporting.KindCashflow, View: view, Granularity: granularity,
					StartDate: start, EndDate: end, ContractID: strings.TrimSpace(args.ContractID), Store: strings.TrimSpace(args.Store), Tags: args.Tags}
			default:
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, "kind must be amortization, liability_rolling or cashflow_forecast"), nil
			}
			result, err := reader.Project(ctx, mode, request)
			if err != nil {
				return rejected(call.CallID, reportErrorCode(err), reportPublicError(err)), nil
			}
			var overlays map[string]any
			if basis == "draft" {
				if overlays, err = buildUserOverlays(ctx, reader, entity); err != nil {
					return rejected(call.CallID, agenttools.ErrorBusinessFailure, "failed to load user overlays"), nil
				}
			}
			return reportProjectionResponse(call.CallID, kind, basis, mode, result.Payload, overlays), nil
		},
	}
}

func NewReportDisclosurePackageDefinition(reader ReportReader) agenttools.ToolDefinition {
	const name = "lease.report.disclosure_package.read"
	schema := `{
  "type":"object",
  "additionalProperties":false,
  "required":["kind"],
  "properties":{
    "kind":{"type":"string","enum":["disclosure","close_pack"]},
    "report_basis":{"type":"string","enum":["approved","pending","draft"]},
    "period_start":{"type":"string","pattern":"^[0-9]{4}-[0-9]{2}-[0-9]{2}$"},
    "period_end":{"type":"string","pattern":"^[0-9]{4}-[0-9]{2}-[0-9]{2}$"},
    "period":{"type":"string","pattern":"^[0-9]{4}-[0-9]{2}$"}
  }
}`
	return agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name: name, Version: "v1",
			DisplayName: "读取披露报告与关账包",
			Description: "读取 IFRS 16 披露报告（disclosure，默认当年日历年）或单期间关账包（close_pack，需 period=YYYY-MM）。关账包只读：不解决例外、不审批批次、不改快照。口径词表同 lease.report.schedule.read。",
			Level:       agenttools.LevelRead, ReadOnly: true,
			Permissions:    []agenttools.Permission{{Resource: "reports", Action: "read"}},
			InputSchema:    json.RawMessage(schema),
			SupportsDryRun: true, MaxRows: 5000, TimeoutSeconds: 30,
		},
		SkillIDs: []string{reportsSkill},
		Handler: func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
			if reader == nil {
				return rejected(call.CallID, agenttools.ErrorDataUnavailable, "report reader unavailable"), nil
			}
			execution, err := agenttools.RequireExecutionContext(ctx)
			if err != nil {
				return rejected(call.CallID, agenttools.ErrorUnauthenticated, "authenticated tool context is required"), nil
			}
			entity, entityErr := access.FromScope(execution.Principal.Scope)
			if entityErr != nil {
				return rejected(call.CallID, agenttools.ErrorScopeDenied, "legal entity scope is required"), nil
			}
			args, err := decodeStrict[reportDisclosurePackageArguments](call.Arguments)
			if err != nil {
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, "arguments contain unsupported or invalid fields"), nil
			}
			mode, basis, ok := resolveReportBasis(args.ReportBasis)
			if !ok {
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, "report_basis must be approved, pending or draft"), nil
			}
			kind := strings.TrimSpace(args.Kind)
			var request reporting.ProjectionRequest
			var result reporting.ProjectionResult
			switch kind {
			case "disclosure":
				now := time.Now().UTC()
				start, startErr := optionalFactsDate(args.PeriodStart, time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.UTC))
				end, endErr := optionalFactsDate(args.PeriodEnd, time.Date(now.Year(), 12, 31, 0, 0, 0, 0, time.UTC))
				if startErr != "" || endErr != "" {
					return rejected(call.CallID, agenttools.ErrorInvalidArguments, firstNonEmpty(startErr, endErr)), nil
				}
				if end.Before(start) {
					return rejected(call.CallID, agenttools.ErrorInvalidArguments, "period_end must be >= period_start"), nil
				}
				request = reporting.ProjectionRequest{Kind: reporting.KindDisclosure, StartDate: start, EndDate: end}
			case "close_pack":
				period := strings.TrimSpace(args.Period)
				if _, parseErr := time.Parse("2006-01", period); parseErr != nil {
					return rejected(call.CallID, agenttools.ErrorInvalidArguments, "period 格式应为 YYYY-MM"), nil
				}
				payload, packErr := reader.ClosePack(ctx, mode, period)
				if packErr != nil {
					return rejected(call.CallID, reportErrorCode(packErr), reportPublicError(packErr)), nil
				}
				result = reporting.ProjectionResult{Payload: payload}
			default:
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, "kind must be disclosure or close_pack"), nil
			}
			if kind == "disclosure" {
				var projectErr error
				result, projectErr = reader.Project(ctx, mode, request)
				if projectErr != nil {
					return rejected(call.CallID, reportErrorCode(projectErr), reportPublicError(projectErr)), nil
				}
			}
			var overlays map[string]any
			if basis == "draft" {
				if overlays, err = buildUserOverlays(ctx, reader, entity); err != nil {
					return rejected(call.CallID, agenttools.ErrorBusinessFailure, "failed to load user overlays"), nil
				}
			}
			return reportProjectionResponse(call.CallID, kind, basis, mode, result.Payload, overlays), nil
		},
	}
}

func NewReportContractViewDefinition(reader ReportReader) agenttools.ToolDefinition {
	const name = "lease.report.contract_view.read"
	schema := `{
  "type":"object",
  "additionalProperties":false,
  "required":["kind"],
  "properties":{
    "kind":{"type":"string","enum":["contract_summary","standard_comparison"]},
    "report_basis":{"type":"string","enum":["approved","pending","draft"]},
    "contract_id":{"type":"string"},
    "discount_rate":{"type":"number","exclusiveMinimum":0}
  }
}`
	return agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name: name, Version: "v1",
			DisplayName: "读取合同汇总与准则对比",
			Description: "读取合同汇总（contract_summary）或新旧准则对比（standard_comparison，需 contract_id，可给 discount_rate 覆盖）。对比视图展示同一合同在两种计量下的差异，用于转换影响评估。",
			Level:       agenttools.LevelRead, ReadOnly: true,
			Permissions:    []agenttools.Permission{{Resource: "reports", Action: "read"}},
			InputSchema:    json.RawMessage(schema),
			SupportsDryRun: true, MaxRows: 5000, TimeoutSeconds: 30,
		},
		SkillIDs: []string{reportsSkill},
		Handler: func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
			if reader == nil {
				return rejected(call.CallID, agenttools.ErrorDataUnavailable, "report reader unavailable"), nil
			}
			execution, err := agenttools.RequireExecutionContext(ctx)
			if err != nil {
				return rejected(call.CallID, agenttools.ErrorUnauthenticated, "authenticated tool context is required"), nil
			}
			entity, entityErr := access.FromScope(execution.Principal.Scope)
			if entityErr != nil {
				return rejected(call.CallID, agenttools.ErrorScopeDenied, "legal entity scope is required"), nil
			}
			args, err := decodeStrict[reportContractViewArguments](call.Arguments)
			if err != nil {
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, "arguments contain unsupported or invalid fields"), nil
			}
			mode, basis, ok := resolveReportBasis(args.ReportBasis)
			if !ok {
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, "report_basis must be approved, pending or draft"), nil
			}
			kind := strings.TrimSpace(args.Kind)
			var request reporting.ProjectionRequest
			switch kind {
			case "contract_summary":
				request = reporting.ProjectionRequest{Kind: reporting.KindContractSummary}
			case "standard_comparison":
				contractID := strings.TrimSpace(args.ContractID)
				if contractID == "" {
					return rejected(call.CallID, agenttools.ErrorInvalidArguments, "contract_id is required for standard_comparison"), nil
				}
				request = reporting.ProjectionRequest{Kind: reporting.KindStandardComparison, ContractID: contractID, Rate: args.DiscountRate}
			default:
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, "kind must be contract_summary or standard_comparison"), nil
			}
			result, err := reader.Project(ctx, mode, request)
			if err != nil {
				return rejected(call.CallID, reportErrorCode(err), reportPublicError(err)), nil
			}
			var overlays map[string]any
			if basis == "draft" {
				if overlays, err = buildUserOverlays(ctx, reader, entity); err != nil {
					return rejected(call.CallID, agenttools.ErrorBusinessFailure, "failed to load user overlays"), nil
				}
			}
			return reportProjectionResponse(call.CallID, kind, basis, mode, result.Payload, overlays), nil
		},
	}
}

func NewReportUnitPriceDefinition(reader ReportReader) agenttools.ToolDefinition {
	const name = "lease.report.unit_price.read"
	schema := `{
  "type":"object",
  "additionalProperties":false,
  "properties":{
    "report_basis":{"type":"string","enum":["approved","pending","draft"]},
    "group_by":{"type":"string","enum":["store","brand","region"]}
  }
}`
	return agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name: name, Version: "v1",
			DisplayName: "读取租金单价对比",
			Description: "按门店、品牌或区域对比每平方米租金单价，用于议价与选址基准。数字由确定性投影服务计算。",
			Level:       agenttools.LevelRead, ReadOnly: true,
			Permissions:    []agenttools.Permission{{Resource: "reports", Action: "read"}},
			InputSchema:    json.RawMessage(schema),
			SupportsDryRun: true, MaxRows: 2000, TimeoutSeconds: 30,
		},
		SkillIDs: []string{reportsSkill},
		Handler: func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
			if reader == nil {
				return rejected(call.CallID, agenttools.ErrorDataUnavailable, "report reader unavailable"), nil
			}
			execution, err := agenttools.RequireExecutionContext(ctx)
			if err != nil {
				return rejected(call.CallID, agenttools.ErrorUnauthenticated, "authenticated tool context is required"), nil
			}
			entity, entityErr := access.FromScope(execution.Principal.Scope)
			if entityErr != nil {
				return rejected(call.CallID, agenttools.ErrorScopeDenied, "legal entity scope is required"), nil
			}
			args, err := decodeStrict[reportUnitPriceArguments](call.Arguments)
			if err != nil {
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, "arguments contain unsupported or invalid fields"), nil
			}
			mode, basis, ok := resolveReportBasis(args.ReportBasis)
			if !ok {
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, "report_basis must be approved, pending or draft"), nil
			}
			groupBy := defaultString(args.GroupBy, reporting.GroupByStore)
			if !oneOfString(groupBy, reporting.GroupByStore, reporting.GroupByBrand, reporting.GroupByRegion) {
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, "group_by must be store|brand|region"), nil
			}
			result, err := reader.Project(ctx, mode, reporting.ProjectionRequest{Kind: reporting.KindUnitPrice, View: groupBy})
			if err != nil {
				return rejected(call.CallID, reportErrorCode(err), reportPublicError(err)), nil
			}
			var overlays map[string]any
			if basis == "draft" {
				if overlays, err = buildUserOverlays(ctx, reader, entity); err != nil {
					return rejected(call.CallID, agenttools.ErrorBusinessFailure, "failed to load user overlays"), nil
				}
			}
			return reportProjectionResponse(call.CallID, "unit_price", basis, mode, result.Payload, overlays), nil
		},
	}
}

func NewReportTagsDefinition(reader ReportReader) agenttools.ToolDefinition {
	const name = "lease.report.tags.read"
	schema := `{
  "type":"object",
  "additionalProperties":false,
  "required":["kind"],
  "properties":{
    "kind":{"type":"string","enum":["tags","tag_summary"]},
    "report_basis":{"type":"string","enum":["approved","pending","draft"]}
  }
}`
	return agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name: name, Version: "v1",
			DisplayName: "读取合同标签及标签汇总",
			Description: "读取权限范围内可用的合同标签清单（tags）或按标签聚合的组合摘要（tag_summary），用于组合切片分析的维度发现。",
			Level:       agenttools.LevelRead, ReadOnly: true,
			Permissions:    []agenttools.Permission{{Resource: "reports", Action: "read"}},
			InputSchema:    json.RawMessage(schema),
			SupportsDryRun: true, MaxRows: 2000, TimeoutSeconds: 20,
		},
		SkillIDs: []string{reportsSkill},
		Handler: func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
			if reader == nil {
				return rejected(call.CallID, agenttools.ErrorDataUnavailable, "report reader unavailable"), nil
			}
			execution, err := agenttools.RequireExecutionContext(ctx)
			if err != nil {
				return rejected(call.CallID, agenttools.ErrorUnauthenticated, "authenticated tool context is required"), nil
			}
			entity, entityErr := access.FromScope(execution.Principal.Scope)
			if entityErr != nil {
				return rejected(call.CallID, agenttools.ErrorScopeDenied, "legal entity scope is required"), nil
			}
			args, err := decodeStrict[reportTagsArguments](call.Arguments)
			if err != nil {
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, "arguments contain unsupported or invalid fields"), nil
			}
			mode, basis, ok := resolveReportBasis(args.ReportBasis)
			if !ok {
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, "report_basis must be approved, pending or draft"), nil
			}
			kind := strings.TrimSpace(args.Kind)
			var request reporting.ProjectionRequest
			switch kind {
			case "tags":
				request = reporting.ProjectionRequest{Kind: reporting.KindTags}
			case "tag_summary":
				request = reporting.ProjectionRequest{Kind: reporting.KindTagSummary}
			default:
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, "kind must be tags or tag_summary"), nil
			}
			result, err := reader.Project(ctx, mode, request)
			if err != nil {
				return rejected(call.CallID, reportErrorCode(err), reportPublicError(err)), nil
			}
			var overlays map[string]any
			if basis == "draft" {
				if overlays, err = buildUserOverlays(ctx, reader, entity); err != nil {
					return rejected(call.CallID, agenttools.ErrorBusinessFailure, "failed to load user overlays"), nil
				}
			}
			return reportProjectionResponse(call.CallID, kind, basis, mode, result.Payload, overlays), nil
		},
	}
}

func oneOfString(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if value == candidate {
			return true
		}
	}
	return false
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func optionalFactsDate(raw string, fallback time.Time) (time.Time, string) {
	if strings.TrimSpace(raw) == "" {
		return fallback, ""
	}
	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, fmt.Sprintf("invalid date %q: must be YYYY-MM-DD", raw)
	}
	return parsed, ""
}

// reportErrorCode keeps discount-rate failures explicit: the snapshot refuses
// to measure with an invented rate, so the agent sees a named gap, not a
// system failure.
func reportErrorCode(err error) agenttools.ErrorCode {
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "discount rate") || strings.Contains(message, "discount_rate") {
		return agenttools.ErrorDataUnavailable
	}
	if strings.Contains(message, "required") || strings.Contains(message, "must") || strings.Contains(message, "invalid") || strings.Contains(message, "unknown kind") || strings.Contains(message, "格式") {
		return agenttools.ErrorInvalidArguments
	}
	return agenttools.ErrorBusinessFailure
}

func reportPublicError(err error) string {
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "discount rate") || strings.Contains(message, "discount_rate") {
		return "discount_rate_missing: 合同缺少可用折现率，快照拒绝以假设利率计量；请先经人工确认折现率"
	}
	return strings.TrimSpace(err.Error())
}

// snapshotReportReader is the production ReportReader: the same
// SnapshotBuilder the /reports routes serve, plus the close-pack composition
// and the draft overlay reads. Read-only.
type snapshotReportReader struct {
	builder      *reporting.SnapshotBuilder
	plans        *repository.FPnAGovernanceRepository
	ops          *repository.OperatingFactsRepository
	closeControl *repository.CloseControlRepository
	monthly      *repository.MonthlyClosingRepository
}

// NewReportSnapshotReader builds the production seam. A nil builder keeps
// every tool honest (unavailable at call time) — the wiring never registers
// a nil version over a real port (P0-8).
func NewReportSnapshotReader(builder *reporting.SnapshotBuilder, plans *repository.FPnAGovernanceRepository, ops *repository.OperatingFactsRepository, closeControl *repository.CloseControlRepository, monthly *repository.MonthlyClosingRepository) ReportReader {
	return snapshotReportReader{builder: builder, plans: plans, ops: ops, closeControl: closeControl, monthly: monthly}
}

func (r snapshotReportReader) Project(ctx context.Context, mode reporting.Mode, request reporting.ProjectionRequest) (reporting.ProjectionResult, error) {
	if r.builder == nil {
		return reporting.ProjectionResult{}, fmt.Errorf("report snapshot builder unavailable")
	}
	scope, scoped := access.ScopeFromContext(ctx)
	if !scoped || strings.TrimSpace(scope.LegalEntityID) == "" {
		return reporting.ProjectionResult{}, fmt.Errorf("legal entity scope is required")
	}
	snapshot, err := r.builder.Build(ctx, reporting.Request{Mode: mode, LegalEntityID: scope.LegalEntityID})
	if err != nil {
		return reporting.ProjectionResult{}, err
	}
	return reporting.Project(snapshot, request)
}

// reportCloseExceptionRow mirrors the HTTP close-pack exception shape so the
// Agent and the route present one composition.
type reportCloseExceptionRow struct {
	ID                 string `json:"id"`
	RuleCode           string `json:"rule_code"`
	Severity           string `json:"severity"`
	ExceptionState     string `json:"exception_state"`
	ClosingDisposition string `json:"closing_disposition"`
	ContractNumber     string `json:"contract_number,omitempty"`
	BatchNumber        string `json:"batch_number,omitempty"`
}

func (r snapshotReportReader) ClosePack(ctx context.Context, mode reporting.Mode, period string) (map[string]any, error) {
	if r.builder == nil {
		return nil, fmt.Errorf("report snapshot builder unavailable")
	}
	scope, scoped := access.ScopeFromContext(ctx)
	if !scoped || strings.TrimSpace(scope.LegalEntityID) == "" {
		return nil, fmt.Errorf("legal entity scope is required")
	}
	periodStart, err := time.Parse("2006-01", period)
	if err != nil {
		return nil, fmt.Errorf("period 格式应为 YYYY-MM")
	}
	snapshot, err := r.builder.Build(ctx, reporting.Request{Mode: mode, LegalEntityID: scope.LegalEntityID})
	if err != nil {
		return nil, err
	}
	disclosure, err := reporting.Project(snapshot, reporting.ProjectionRequest{
		Kind: reporting.KindDisclosure, StartDate: periodStart, EndDate: periodStart.AddDate(0, 1, -1),
	})
	if err != nil {
		return nil, err
	}
	exceptions := make([]reportCloseExceptionRow, 0)
	if r.closeControl != nil {
		items, listErr := r.closeControl.ListExceptions(ctx, period, scope.LegalEntityID)
		if listErr != nil {
			return nil, listErr
		}
		for _, item := range items {
			exceptions = append(exceptions, reportCloseExceptionRow{
				ID: item.ID, RuleCode: item.RuleCode, Severity: item.Severity,
				ExceptionState: item.ExceptionState, ClosingDisposition: item.ClosingDisposition,
				ContractNumber: item.ContractNumber, BatchNumber: item.BatchNumber,
			})
		}
	}
	var batches []*repository.MonthlyClosingBatch
	if r.monthly != nil {
		batches, err = r.monthly.GetBatches(ctx, period, scope.LegalEntityID)
		if err != nil {
			return nil, err
		}
	}
	return map[string]any{
		"period":                period,
		"mode":                  string(mode),
		"is_official":           mode == reporting.Official,
		"report_basis":          disclosure.Payload["report_basis"],
		"disclosure":            disclosure.Payload,
		"close_exceptions":      exceptions,
		"monthly_close_batches": batches,
		"exception_count":       len(exceptions),
	}, nil
}

func (r snapshotReportReader) ListPlanVersions(ctx context.Context, entity access.EntityFilter, versionType, status, asOfPeriod string) ([]*repository.FPnAPlanVersion, error) {
	if r.plans == nil {
		return nil, fmt.Errorf("fpna plan reader unavailable")
	}
	return r.plans.ListPlanVersions(ctx, entity, versionType, status, asOfPeriod)
}

func (r snapshotReportReader) ListAssumptions(ctx context.Context, entity access.EntityFilter, key string) ([]*repository.FPnAAssumptionVersion, error) {
	if r.ops == nil {
		return nil, fmt.Errorf("assumption reader unavailable")
	}
	return r.ops.ListAssumptions(ctx, entity, key)
}

func (r snapshotReportReader) ListScenarioDrafts(ctx context.Context, entity access.EntityFilter, limit int) ([]*repository.FPnAScenarioDraft, error) {
	if r.ops == nil {
		return nil, fmt.Errorf("scenario draft reader unavailable")
	}
	return r.ops.ListScenarioDrafts(ctx, entity, limit)
}
