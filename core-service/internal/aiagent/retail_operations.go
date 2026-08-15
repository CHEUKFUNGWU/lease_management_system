package aiagent

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lease-management-system/core-service/internal/agenttools"
	agenttooldefs "github.com/lease-management-system/core-service/internal/agenttools/tools"
	"github.com/lease-management-system/core-service/internal/services/retailkpi"
	"github.com/lease-management-system/core-service/internal/services/retailpulse"
	"github.com/lease-management-system/core-service/internal/services/retailscenario"
	"github.com/lease-management-system/core-service/internal/services/retailstore360"
)

// RetailOperationsData is the stable, structured context consumed by the Web
// Agent. The response keeps the service payload intact; the Agent only adds
// intent, evidence state and provenance labels around it.
type RetailOperationsData struct {
	Intent             string                   `json:"intent"`
	DataClassification string                   `json:"data_classification,omitempty"`
	DatasetVersion     string                   `json:"dataset_version,omitempty"`
	SourceSystem       string                   `json:"source_system,omitempty"`
	AsOf               string                   `json:"as_of,omitempty"`
	WindowDays         int                      `json:"window_days,omitempty"`
	FormulaVersion     string                   `json:"formula_version,omitempty"`
	NumericAuthority   string                   `json:"numeric_authority"`
	SideEffects        bool                     `json:"side_effects"`
	EvidenceStatus     string                   `json:"evidence_status"`
	NeedsInput         bool                     `json:"needs_input,omitempty"`
	Reason             string                   `json:"reason,omitempty"`
	Pulse              *retailpulse.Response    `json:"pulse,omitempty"`
	Diagnostics        *retailstore360.Response `json:"diagnostics,omitempty"`
	Scenario           *retailscenario.Response `json:"scenario,omitempty"`
}

type RetailActionProposal struct {
	Type               string  `json:"type"`
	Status             string  `json:"status"`
	Title              string  `json:"title"`
	Store              any     `json:"store"`
	PlannedAction      string  `json:"planned_action"`
	OwnerName          *string `json:"owner_name,omitempty"`
	DueDate            *string `json:"due_date,omitempty"`
	VerificationPeriod string  `json:"verification_period,omitempty"`
	Scenario           any     `json:"scenario"`
	Evidence           any     `json:"evidence"`
	EvidenceComplete   bool    `json:"evidence_complete"`
	Envelope           any     `json:"envelope,omitempty"`
	DataClassification string  `json:"data_classification"`
	DatasetVersion     string  `json:"dataset_version,omitempty"`
	SourceSystem       string  `json:"source_system,omitempty"`
	FormulaVersion     string  `json:"formula_version"`
	FormalExecution    bool    `json:"formal_execution"`
	BusinessWrite      bool    `json:"business_write"`
	NextURL            string  `json:"next_url"`
}

const retailDeterministicModel = "deterministic-fallback"

func buildRetailOperationsRunbook(req Request) *AgentRunbook {
	return &AgentRunbook{
		SkillID: "retail_operations", SkillName: "零售经营分析 Agent",
		AnswerPrefix: "零售经营分析使用已验收的确定性服务；AI 不重算经营数字，不写入业务台账。",
		AgentPlan: []AgentPlanStep{
			{ID: "load_retail_context", Title: "读取经营事实与来源", Status: "pending"},
			{ID: "check_retail_quality", Title: "核对覆盖、币种和来源冲突", Status: "pending"},
			{ID: "prepare_review", Title: "输出可复核结论或情景提议", Status: "pending"},
		},
		ReviewPrompts: []AgentReviewPrompt{{ID: "retail_source_review", Title: "复核零售事实与来源", Description: "请在经营页面核对法人、日期、分类、数据集和公式版本；情景提议需回到情景工作台二次确认。", Severity: "warning", Action: "review_retail_sources"}},
	}
}

type retailAgentFilters struct {
	AsOf                string
	WindowDays          int
	WindowProvided      bool
	Classification      string
	DatasetVersion      string
	SourceSystem        string
	StoreID             string
	StoreIDs            []string
	HorizonMonths       int
	HorizonProvided     bool
	Assumptions         retailscenario.Assumptions
	ProvidedAssumptions map[string]bool
	InvalidAssumption   bool
}

var retailPercentPattern = regexp.MustCompile(`(?i)(?:人工|labor)[^0-9-]{0,8}下降[^0-9-]*(-?[0-9]+(?:\.[0-9]+)?)\s*%?`)

func retailFilters(req Request) retailAgentFilters {
	filters := retailAgentFilters{}
	if req.PageContext != nil {
		for key, value := range req.PageContext.Filters {
			switch strings.ToLower(strings.TrimSpace(key)) {
			case "as_of":
				filters.AsOf = strings.TrimSpace(value)
			case "window_days":
				if parsed, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
					filters.WindowDays = parsed
				}
				filters.WindowProvided = true
			case "classification", "data_classification":
				filters.Classification = strings.TrimSpace(value)
			case "dataset_version":
				filters.DatasetVersion = strings.TrimSpace(value)
			case "source_system":
				filters.SourceSystem = strings.TrimSpace(value)
			case "store_id":
				filters.StoreID = strings.TrimSpace(value)
			case "store_ids":
				filters.StoreIDs = splitRetailIDs(value)
			case "horizon_months":
				if parsed, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
					filters.HorizonMonths = parsed
				}
				filters.HorizonProvided = true
			case "revenue_change_pct":
				filters.Assumptions.RevenueChangePct = parseFloat(value)
				markRetailAssumption(&filters, "revenue_change_pct")
				filters.InvalidAssumption = filters.InvalidAssumption || !retailFloatValid(value)
			case "gross_margin_rate_change_pp":
				filters.Assumptions.GrossMarginRateChangePP = parseFloat(value)
				markRetailAssumption(&filters, "gross_margin_rate_change_pp")
				filters.InvalidAssumption = filters.InvalidAssumption || !retailFloatValid(value)
			case "labor_cost_change_pct":
				filters.Assumptions.LaborCostChangePct = parseFloat(value)
				markRetailAssumption(&filters, "labor_cost_change_pct")
				filters.InvalidAssumption = filters.InvalidAssumption || !retailFloatValid(value)
			case "fixed_rent_change_pct":
				filters.Assumptions.FixedRentChangePct = parseFloat(value)
				markRetailAssumption(&filters, "fixed_rent_change_pct")
				filters.InvalidAssumption = filters.InvalidAssumption || !retailFloatValid(value)
			case "variable_rent_rate_change_pp":
				filters.Assumptions.VariableRentRateChangePP = parseFloat(value)
				markRetailAssumption(&filters, "variable_rent_rate_change_pp")
				filters.InvalidAssumption = filters.InvalidAssumption || !retailFloatValid(value)
			case "non_lease_cost_change_pct":
				filters.Assumptions.NonLeaseCostChangePct = parseFloat(value)
				markRetailAssumption(&filters, "non_lease_cost_change_pct")
				filters.InvalidAssumption = filters.InvalidAssumption || !retailFloatValid(value)
			case "other_controllable_cost_change_pct":
				filters.Assumptions.OtherControllableCostChangePct = parseFloat(value)
				markRetailAssumption(&filters, "other_controllable_cost_change_pct")
				filters.InvalidAssumption = filters.InvalidAssumption || !retailFloatValid(value)
			}
		}
	}
	if filters.StoreID == "" && len(filters.StoreIDs) == 1 {
		filters.StoreID = filters.StoreIDs[0]
	}
	applyRetailMessageContext(&filters, req.Message)
	if match := retailPercentPattern.FindStringSubmatch(req.Message); match != nil && filters.Assumptions.LaborCostChangePct == 0 {
		filters.Assumptions.LaborCostChangePct = -parseFloat(match[1])
		markRetailAssumption(&filters, "labor_cost_change_pct")
	}
	return filters
}

func applyRetailMessageContext(filters *retailAgentFilters, message string) {
	if filters == nil {
		return
	}
	lower := strings.ToLower(message)
	if filters.Classification == "" {
		switch {
		case strings.Contains(lower, "production") || strings.Contains(message, "正式数据"):
			filters.Classification = "production"
		case strings.Contains(lower, "simulated") || strings.Contains(message, "模拟数据"):
			filters.Classification = "simulated"
		}
	}
	if filters.AsOf == "" {
		if match := regexp.MustCompile(`(?i)(?:as[_ -]?of|截至)\s*[:=]?\s*(\d{4}-\d{2}-\d{2})`).FindStringSubmatch(message); len(match) == 2 {
			filters.AsOf = match[1]
		}
	}
	if !filters.WindowProvided {
		if match := regexp.MustCompile(`(?i)(?:window[_ -]?days|窗口|最近)\s*[:=]?\s*(7|14|28)\s*(?:天|days?)?`).FindStringSubmatch(message); len(match) == 2 {
			filters.WindowDays, filters.WindowProvided = parseInt(match[1]), true
		}
	}
	if filters.DatasetVersion == "" {
		if match := regexp.MustCompile(`(?i)(?:dataset_version|dataset|数据集)\s*[:=]?\s*([A-Za-z0-9][A-Za-z0-9._-]*)`).FindStringSubmatch(message); len(match) == 2 {
			filters.DatasetVersion = match[1]
		}
	}
	if filters.SourceSystem == "" {
		if match := regexp.MustCompile(`(?i)source[_ -]?system\s*[:=]\s*([\w.-]+)`).FindStringSubmatch(message); len(match) == 2 {
			filters.SourceSystem = match[1]
		}
	}
	if filters.StoreID == "" {
		if match := regexp.MustCompile(`(?i)\b([0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12})\b`).FindStringSubmatch(message); len(match) == 2 {
			filters.StoreID = match[1]
		}
	}
	if !filters.HorizonProvided {
		if match := regexp.MustCompile(`(?i)(?:horizon[_ -]?months|horizon|情景期|未来)\s*[:=]?\s*(3|6|12)\s*(?:个月|months?)?`).FindStringSubmatch(message); len(match) == 2 {
			filters.HorizonMonths, filters.HorizonProvided = parseInt(match[1]), true
		}
	}
	assumptionPatterns := map[string]*float64{
		"revenue_change_pct":                 &filters.Assumptions.RevenueChangePct,
		"gross_margin_rate_change_pp":        &filters.Assumptions.GrossMarginRateChangePP,
		"labor_cost_change_pct":              &filters.Assumptions.LaborCostChangePct,
		"fixed_rent_change_pct":              &filters.Assumptions.FixedRentChangePct,
		"variable_rent_rate_change_pp":       &filters.Assumptions.VariableRentRateChangePP,
		"non_lease_cost_change_pct":          &filters.Assumptions.NonLeaseCostChangePct,
		"other_controllable_cost_change_pct": &filters.Assumptions.OtherControllableCostChangePct,
	}
	for key, target := range assumptionPatterns {
		if *target != 0 {
			continue
		}
		pattern := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(key) + `\s*[:=]\s*(-?[0-9]+(?:\.[0-9]+)?)`)
		if match := pattern.FindStringSubmatch(message); len(match) == 2 {
			*target = parseFloat(match[1])
			markRetailAssumption(filters, key)
		}
	}
}

func markRetailAssumption(filters *retailAgentFilters, key string) {
	if filters.ProvidedAssumptions == nil {
		filters.ProvidedAssumptions = make(map[string]bool)
	}
	filters.ProvidedAssumptions[key] = true
}

func allRetailAssumptionsProvided(filters retailAgentFilters) bool {
	for _, key := range []string{"revenue_change_pct", "gross_margin_rate_change_pp", "labor_cost_change_pct", "fixed_rent_change_pct", "variable_rent_rate_change_pp", "non_lease_cost_change_pct", "other_controllable_cost_change_pct"} {
		if !filters.ProvidedAssumptions[key] {
			return false
		}
	}
	return true
}

func parseInt(value string) int {
	parsed, _ := strconv.Atoi(strings.TrimSpace(value))
	return parsed
}

func splitRetailIDs(value string) []string {
	raw := strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ' ' || r == '\n' })
	result := make([]string, 0, len(raw))
	seen := map[string]bool{}
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item != "" && !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	sort.Strings(result)
	return result
}
func parseFloat(value string) float64 {
	parsed, _ := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return parsed
}

func retailFloatValid(value string) bool {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return err == nil && !math.IsNaN(parsed) && !math.IsInf(parsed, 0)
}

func retailIntent(message string, filters retailAgentFilters) string {
	lower := strings.ToLower(message)
	if strings.Contains(lower, "行动提议") || strings.Contains(lower, "行动草稿") || strings.Contains(lower, "action proposal") {
		return "action_draft"
	}
	if strings.Contains(lower, "情景") || strings.Contains(lower, "what-if") || strings.Contains(lower, "方案") || filters.Assumptions != (retailscenario.Assumptions{}) {
		return "scenario_evaluate"
	}
	if strings.Contains(lower, "客流") && (strings.Contains(lower, "下降") || strings.Contains(lower, "上升") || strings.Contains(lower, "怎样")) {
		return "scenario_evaluate"
	}
	if filters.StoreID != "" || strings.Contains(lower, "诊断") || strings.Contains(lower, "为什么") || strings.Contains(lower, "diagnostic") {
		return "store_diagnostics"
	}
	return "pulse_summary"
}

func (h *Agent) executeRetailOperations(ctx context.Context, req Request, emit func(context.Context, string, any) error, toolRuntime *agenttools.Runtime) (Response, error) {
	filters := retailFilters(req)
	intent := retailIntent(req.Message, filters)
	if conflict := retailContextConflict(req.Message, filters); conflict != "" {
		return retailNeedsInput(intent, filters, conflict), nil
	}
	if retailUnsupportedRequest(req.Message) {
		response := retailNeedsInput(intent, filters, "零售 Agent 只提供只读经营分析和待复核情景提议；删除、执行、改 Forecast/合同、创建事件、过账、自动通知等动作必须回到既有人工审批流程。")
		response.RetailOperations.Reason = "assist_mode_boundary"
		return response, nil
	}
	if filters.AsOf == "" || filters.Classification == "" || !filters.WindowProvided || (filters.Classification == "simulated" && filters.DatasetVersion == "") || (filters.Classification == "production" && filters.DatasetVersion != "") {
		return retailNeedsInput(intent, filters, "请提供 as_of、window_days、data_classification；window_days 只支持 7、14 或 28；simulated 还需要 dataset_version，production 不应携带 dataset_version。"), nil
	}
	if filters.WindowDays != 7 && filters.WindowDays != 14 && filters.WindowDays != 28 {
		return retailNeedsInput(intent, filters, "window_days 只支持 7、14 或 28。"), nil
	}
	if filters.InvalidAssumption {
		return retailNeedsInput(intent, filters, "情景假设必须是有限数字，不能使用 NaN 或 Inf。"), nil
	}
	if intent == "store_diagnostics" || intent == "scenario_evaluate" || intent == "action_draft" {
		if filters.StoreID == "" {
			return retailNeedsInput(intent, filters, "请从经营脉搏关注行选择一个 store_id 后再分析。"), nil
		}
	}
	if (intent == "scenario_evaluate" || intent == "action_draft") && !filters.HorizonProvided {
		return retailNeedsInput(intent, filters, "请提供 horizon_months（3、6 或 12）后再运行情景。"), nil
	}
	if strings.Contains(strings.ToLower(req.Message), "客流") && intent == "scenario_evaluate" && filters.Assumptions.RevenueChangePct == 0 {
		return retailNeedsInput(intent, filters, "客流是观测信号；情景工具需要明确 revenue_change_pct 等七项假设。"), nil
	}
	if (intent == "scenario_evaluate" || intent == "action_draft") && !allRetailAssumptionsProvided(filters) {
		return retailNeedsInput(intent, filters, "请提供七项情景假设；例如 labor_cost_change_pct=-10。"), nil
	}

	plan := buildRetailOperationsRunbook(req)
	plan.AgentPlan[0].Status = "running"
	response := Response{Answer: plan.AnswerPrefix, Confidence: 0.4, IsOfficial: false, Model: retailDeterministicModel, AgentMode: true, AgentPlan: plan.AgentPlan, ReviewPrompts: plan.ReviewPrompts}
	data := &RetailOperationsData{Intent: intent, DataClassification: filters.Classification, DatasetVersion: filters.DatasetVersion, SourceSystem: filters.SourceSystem, AsOf: filters.AsOf, WindowDays: filters.WindowDays, NumericAuthority: "deterministic_service", SideEffects: false, EvidenceStatus: "unavailable"}
	var sources []Source
	var calls []AgentToolCall
	if intent == "pulse_summary" || intent == "store_diagnostics" || intent == "scenario_evaluate" || intent == "action_draft" {
		pulseArgs := agenttooldefs.RetailOperatingPulseArguments{}
		pulseArgs.AsOf, pulseArgs.WindowDays, pulseArgs.DataClass, pulseArgs.DatasetVersion, pulseArgs.SourceSystem = filters.AsOf, filters.WindowDays, filters.Classification, filters.DatasetVersion, filters.SourceSystem
		pulseArgs.StoreIDs = retailPulseStoreIDs(intent, filters)
		pulseResult, pulseCall, err := h.executeRetailTool(ctx, toolRuntime, "retail.operating_pulse.read", pulseArgs, emit)
		calls = append(calls, pulseCall)
		if err != nil {
			data.Reason = retailToolReason(err, pulseResult)
			data.EvidenceStatus = "insufficient"
			data.NeedsInput = retailReasonNeedsInput(data.Reason)
			plan.AgentPlan[0].Status, plan.AgentPlan[1].Status, plan.AgentPlan[2].Status = "failed", "needs_review", "needs_review"
			response.ToolCalls = calls
			response.RetailOperations = data
			if data.Reason == "source_conflict" {
				response.Answer += "\n\n来源冲突：请明确 source_system 后重试；当前没有输出经营结论。"
			} else if data.Reason == "scope_denied" {
				response.Answer += "\n\n当前请求不在授权范围内；没有返回门店或法人信息。"
			} else {
				response.Answer += "\n\n当前经营脉搏不可用，请稍后重试或核对数据上下文。"
			}
			response.Answer += retailEvidenceMarker(data.Reason, data.EvidenceStatus, 0.40)
			return response, nil
		}
		pulseData, ok := pulseResult.Data.(agenttooldefs.RetailPulseToolData)
		if !ok || pulseData.Response == nil {
			return response, fmt.Errorf("retail pulse returned unexpected data")
		}
		data.Pulse = pulseData.Response
		data.FormulaVersion = pulseData.FormulaVersion
		data.EvidenceStatus = retailEvidence(pulseData.Response.DecisionReady, pulseData.Response.CurrentCoverage, pulseData.Response.ComparisonCoverage)
		sources = append(sources, toolSourcesToAgent(pulseResult.Sources)...)
		if (intent == "scenario_evaluate" || intent == "action_draft") && retailPulseInsufficient(pulseData.Response) {
			data.Reason = retailPulseInsufficientReason(pulseData.Response)
			data.NeedsInput = retailReasonNeedsInput(data.Reason)
			response.Confidence = 0.40
			response.Answer += retailPulseAnswer(pulseData.Response) + fmt.Sprintf("\n\n证据不足（%s）：当前覆盖或必需指标不完整，未执行情景，也未生成金额型行动提议。", data.Reason) + retailEvidenceMarker(data.Reason, data.EvidenceStatus, response.Confidence)
			plan.AgentPlan[0].Status, plan.AgentPlan[1].Status, plan.AgentPlan[2].Status = "completed", "needs_review", "needs_review"
			response.AgentPlan, response.ToolCalls, response.Sources, response.RetailOperations = plan.AgentPlan, calls, sources, data
			return response, nil
		}
		if intent == "pulse_summary" {
			response.Confidence = retailPulseConfidence(pulseData.Response)
			response.Answer += retailPulseAnswer(pulseData.Response)
			plan.AgentPlan[0].Status, plan.AgentPlan[1].Status, plan.AgentPlan[2].Status = "completed", "completed", "completed"
			response.AgentPlan = plan.AgentPlan
			response.ToolCalls, response.Sources, response.RetailOperations = calls, sources, data
			return response, nil
		}
	}
	if intent == "store_diagnostics" || intent == "scenario_evaluate" || intent == "action_draft" {
		diagnosticArgs := agenttooldefs.RetailStoreDiagnosticsArguments{}
		diagnosticArgs.AsOf, diagnosticArgs.WindowDays, diagnosticArgs.DataClass, diagnosticArgs.DatasetVersion, diagnosticArgs.SourceSystem, diagnosticArgs.StoreID = filters.AsOf, filters.WindowDays, filters.Classification, filters.DatasetVersion, filters.SourceSystem, filters.StoreID
		diagnosticResult, diagnosticCall, err := h.executeRetailTool(ctx, toolRuntime, "retail.store_diagnostics.read", diagnosticArgs, emit)
		calls = append(calls, diagnosticCall)
		if err == nil {
			if diagnosticData, ok := diagnosticResult.Data.(agenttooldefs.RetailDiagnosticsToolData); ok {
				data.Diagnostics = diagnosticData.Response
				sources = append(sources, toolSourcesToAgent(diagnosticResult.Sources)...)
				if diagnosticData.Response != nil {
					data.EvidenceStatus = retailEvidence(diagnosticData.Response.DecisionReady, diagnosticData.Response.TargetCoverage, diagnosticData.Response.ComparisonCoverage)
				}
			}
		} else if data.Reason == "" {
			data.Reason = retailToolReason(err, diagnosticResult)
		}
		// A no-facts pulse can still reach diagnostics with a scope_denied
		// response because the requested store is absent from the authorized
		// population. Preserve the business data-quality reason in the Agent
		// output while retaining the raw tool rejection in the trace.
		if data.Reason == "scope_denied" && data.Pulse != nil && retailPulseInsufficientReason(data.Pulse) == "no_facts" {
			data.Reason = "no_facts"
		}
		if err != nil || data.Diagnostics == nil {
			data.EvidenceStatus = "insufficient"
			data.NeedsInput = retailReasonNeedsInput(data.Reason)
		}
		if intent == "store_diagnostics" {
			response.Confidence = retailDiagnosticsConfidence(data.Diagnostics)
			response.Answer += retailDiagnosticsAnswer(data.Diagnostics)
			if err != nil || data.Diagnostics == nil {
				response.Confidence = 0.40
				response.Answer += retailEvidenceMarker(data.Reason, data.EvidenceStatus, response.Confidence)
				plan.AgentPlan[0].Status, plan.AgentPlan[1].Status, plan.AgentPlan[2].Status = "completed", "failed", "needs_review"
			} else if !data.Diagnostics.DecisionReady {
				data.Reason = "diagnostics_not_decision_ready"
				plan.AgentPlan[0].Status, plan.AgentPlan[1].Status, plan.AgentPlan[2].Status = "completed", "needs_review", "needs_review"
			} else {
				plan.AgentPlan[0].Status, plan.AgentPlan[1].Status, plan.AgentPlan[2].Status = "completed", "completed", "completed"
			}
			response.AgentPlan, response.ToolCalls, response.Sources, response.RetailOperations = plan.AgentPlan, calls, sources, data
			return response, nil
		}
		if err != nil || data.Diagnostics == nil {
			response.Confidence = 0.40
			response.Answer += "\n\n门店诊断证据不足，未执行情景，也未生成金额型行动提议。" + retailEvidenceMarker(data.Reason, data.EvidenceStatus, response.Confidence)
			plan.AgentPlan[0].Status, plan.AgentPlan[1].Status, plan.AgentPlan[2].Status = "completed", "failed", "needs_review"
			response.AgentPlan, response.ToolCalls, response.Sources, response.RetailOperations = plan.AgentPlan, calls, sources, data
			return response, nil
		}
		if !data.Diagnostics.DecisionReady {
			data.Reason = "diagnostics_not_decision_ready"
			data.EvidenceStatus = "insufficient"
			response.Confidence = 0.40
			response.Answer += "\n\n门店诊断尚未达到 decision_ready，未执行情景，也未生成金额型行动提议。" + retailEvidenceMarker(data.Reason, data.EvidenceStatus, response.Confidence)
			plan.AgentPlan[0].Status, plan.AgentPlan[1].Status, plan.AgentPlan[2].Status = "completed", "needs_review", "needs_review"
			response.AgentPlan, response.ToolCalls, response.Sources, response.RetailOperations = plan.AgentPlan, calls, sources, data
			return response, nil
		}
	}
	args := agenttooldefs.RetailScenarioEvaluateArguments{}
	args.AsOf, args.WindowDays, args.DataClass, args.DatasetVersion, args.SourceSystem, args.StoreID, args.HorizonMonths, args.Assumptions = filters.AsOf, filters.WindowDays, filters.Classification, filters.DatasetVersion, filters.SourceSystem, filters.StoreID, filters.HorizonMonths, filters.Assumptions
	scenarioResult, scenarioCall, err := h.executeRetailTool(ctx, toolRuntime, "retail.store.scenario.evaluate", args, emit)
	calls = append(calls, scenarioCall)
	if err == nil {
		if scenarioData, ok := scenarioResult.Data.(agenttooldefs.RetailScenarioToolData); ok {
			data.Scenario = scenarioData.Response
			data.FormulaVersion = scenarioData.FormulaVersion
			data.EvidenceStatus = retailScenarioEvidence(scenarioData.Response)
			sources = append(sources, toolSourcesToAgent(scenarioResult.Sources)...)
			if intent == "action_draft" && scenarioData.Response != nil {
				response.RetailActionProposal = makeRetailActionProposal(scenarioData.Response, filters)
			}
		}
		if data.Scenario == nil {
			err = errors.New("retail scenario returned unexpected data")
		}
	} else {
		data.Reason = retailToolReason(err, scenarioResult)
	}
	if err != nil {
		if data.Reason == "" {
			data.Reason = retailToolReason(err, scenarioResult)
		}
		data.EvidenceStatus = "insufficient"
		data.NeedsInput = retailReasonNeedsInput(data.Reason)
		response.Confidence = 0.40
		response.Answer += retailScenarioAnswer(nil, false) + retailEvidenceMarker(data.Reason, data.EvidenceStatus, response.Confidence)
		plan.AgentPlan[0].Status, plan.AgentPlan[1].Status, plan.AgentPlan[2].Status = "completed", "completed", "failed"
		response.AgentPlan, response.ToolCalls, response.Sources, response.RetailOperations = plan.AgentPlan, calls, sources, data
		return response, nil
	}
	response.Confidence = confidenceForRetail(data.EvidenceStatus)
	response.Answer += retailScenarioAnswer(data.Scenario, intent == "action_draft")
	plan.AgentPlan[0].Status, plan.AgentPlan[1].Status, plan.AgentPlan[2].Status = "completed", "completed", "completed"
	response.AgentPlan, response.ToolCalls, response.Sources, response.RetailOperations = plan.AgentPlan, calls, sources, data
	return response, nil
}

func (h *Agent) executeRetailTool(ctx context.Context, runtime *agenttools.Runtime, name string, arguments any, emit func(context.Context, string, any) error) (agenttools.ToolResult, AgentToolCall, error) {
	call := AgentToolCall{Tool: name, Skill: "零售经营分析 Agent", Status: "running", InputSummary: "读取服务端零售事实", OutputSummary: "确定性服务结果", RequiresReview: false}
	_ = emitAgentEvent(ctx, emit, "tool_start", map[string]any{"tool": name, "status": "running"})
	result, durationMs, err := h.executeToolCall(ctx, runtime, name, arguments, "")
	// The tool actually ran: its wall-clock duration is measured and carried
	// on the call, whether it completed or failed. Only calls that never ran
	// leave duration_ms absent.
	if durationMs != nil {
		call.DurationMs = durationMs
	}
	if err != nil || result.Status != agenttools.StatusCompleted || result.Error != nil {
		call.Status = "failed"
		call.OutputSummary = "数据不可用或查询被拒绝"
		_ = emitAgentEvent(ctx, emit, "tool_end", map[string]any{"tool": name, "status": "failed"})
		return result, call, errOrToolError(err, result)
	}
	call.Status = "completed"
	_ = emitAgentEvent(ctx, emit, "tool_end", map[string]any{"tool": name, "status": "completed"})
	return result, call, nil
}

func errOrToolError(err error, result agenttools.ToolResult) error {
	if err != nil {
		return err
	}
	if result.Error != nil {
		return fmt.Errorf("%s", result.Error.Message)
	}
	return fmt.Errorf("retail tool failed")
}

func retailToolReason(err error, result agenttools.ToolResult) string {
	if result.Error != nil {
		switch result.Error.Code {
		case agenttools.ErrorConflict:
			return "source_conflict"
		case agenttools.ErrorInvalidArguments:
			if strings.Contains(strings.ToLower(result.Error.Message), "resulting_rate_out_of_range") {
				return "resulting_rate_out_of_range"
			}
			return "invalid_context"
		case agenttools.ErrorPermissionDenied, agenttools.ErrorScopeDenied:
			return "scope_denied"
		case agenttools.ErrorDataUnavailable:
			return "data_unavailable"
		}
	}
	if err != nil {
		return "retail_service_unavailable"
	}
	return "retail_service_unavailable"
}

func retailReasonNeedsInput(reason string) bool {
	switch reason {
	case "invalid_context", "resulting_rate_out_of_range", "scope_denied", "source_conflict":
		return true
	default:
		return false
	}
}

func retailPulseStoreIDs(intent string, filters retailAgentFilters) []string {
	if (intent == "store_diagnostics" || intent == "scenario_evaluate" || intent == "action_draft") && filters.StoreID != "" {
		return []string{filters.StoreID}
	}
	return append([]string(nil), filters.StoreIDs...)
}

func toolSourcesToAgent(input []agenttools.ToolSource) []Source {
	result := make([]Source, 0, len(input))
	for _, source := range input {
		link := source.URL
		if link == "" {
			link = source.Locator
		}
		result = append(result, Source{Type: source.Type, ID: source.ID, Title: source.Title, Snippet: source.Locator, URL: link, Classification: source.Classification, DatasetVersion: source.DatasetVersion, AsOf: source.AsOf, FormulaVersion: source.FormulaVersion})
	}
	return result
}

func retailEvidence(decision bool, current, comparison any) string {
	if !decision {
		return "insufficient"
	}
	_ = current
	_ = comparison
	return "complete"
}
func retailScenarioEvidence(response *retailscenario.Response) string {
	if response == nil {
		return "unavailable"
	}
	if response.Evidence.CoverageRate == nil || *response.Evidence.CoverageRate < 100 {
		return "partial"
	}
	return "complete"
}
func confidenceForRetail(status string) float64 {
	switch status {
	case "complete":
		return 0.90
	case "partial":
		return 0.70
	default:
		return 0.40
	}
}

func retailPulseConfidence(response *retailpulse.Response) float64 {
	if response == nil || !response.DecisionReady {
		return 0.40
	}
	for _, metric := range response.Summary {
		if metric.Status != "complete" || metric.Current.Status != "complete" || metric.Comparison.Status != "complete" {
			return 0.70
		}
	}
	return 0.90
}

func retailDiagnosticsConfidence(response *retailstore360.Response) float64 {
	if response == nil || !response.DecisionReady {
		return 0.40
	}
	for _, metric := range response.Summary {
		if metric.Status != "complete" || metric.Current.Status != "complete" || metric.Comparison.Status != "complete" {
			return 0.40
		}
	}
	for _, peer := range response.PeerBenchmark {
		if peer.Status != "complete" {
			return 0.70
		}
	}
	for _, bridge := range response.Bridges {
		if bridge.Status != "complete" {
			return 0.70
		}
	}
	return 0.90
}

func retailNeedsInput(intent string, filters retailAgentFilters, reason string) Response {
	reasonCode := retailInputReasonCode(reason)
	data := &RetailOperationsData{Intent: intent, DataClassification: filters.Classification, DatasetVersion: filters.DatasetVersion, SourceSystem: filters.SourceSystem, AsOf: filters.AsOf, WindowDays: filters.WindowDays, NumericAuthority: "deterministic_service", SideEffects: false, EvidenceStatus: "insufficient", NeedsInput: true, Reason: reasonCode}
	return Response{Answer: "零售经营分析需要补充可验证的数据上下文。\n\n" + reason + retailEvidenceMarker(reasonCode, data.EvidenceStatus, 0.40), Confidence: 0.40, IsOfficial: false, Model: retailDeterministicModel, AgentMode: true, RetailOperations: data, ReviewPrompts: []AgentReviewPrompt{{ID: "retail_context_required", Title: "补充数据上下文", Description: reason, Severity: "warning", Action: "provide_retail_context"}}}
}

func retailInputReasonCode(reason string) string {
	switch {
	case strings.Contains(reason, "页面上下文"):
		return "context_conflict"
	case strings.Contains(reason, "只提供只读"):
		return "assist_mode_boundary"
	case strings.Contains(reason, "有限数字"):
		return "invalid_context"
	default:
		return "missing_context"
	}
}

func retailEvidenceMarker(reason, evidence string, confidence float64) string {
	if strings.TrimSpace(reason) == "" {
		reason = "data_unavailable"
	}
	if strings.TrimSpace(evidence) == "" || evidence == "unavailable" {
		evidence = "insufficient"
	}
	return fmt.Sprintf("\n\n证据状态：reason=%s；evidence=%s；confidence=%.2f。", reason, evidence, confidence)
}
func retailPulseAnswer(response *retailpulse.Response) string {
	if response == nil {
		return "\n\n当前没有可用经营脉搏事实；请核对覆盖期和数据集。"
	}
	status := retailEvidence(response.DecisionReady, response.CurrentCoverage, response.ComparisonCoverage)
	metrics := retailPulseMetricSummary(response.Summary)
	attention := retailPulseAttentionSummary(response.Attention)
	return fmt.Sprintf("\n\n数据上下文：%s · dataset=%s · source=%s · as_of=%s · window=%s · formula=%s · evidence=%s · confidence=%.2f。\n经营脉搏：%s 至 %s，关注项 %d；current 覆盖 %s，comparison 覆盖 %s，decision_ready=%t。摘要：%s。关注清单：%s。数字来自 retail-pulse-v1，缺失值保持为空，不补零。", response.DataClassification, displayRetailValue(response.DatasetVersion), retailSourceLabel(response.SourceSystems), response.Current.DateTo, formatWindow(response.Current.DateFrom, response.Current.DateTo), response.FormulaVersion, status, confidenceForRetail(status), response.Current.DateFrom, response.Current.DateTo, len(response.Attention), formatCoverage(response.CurrentCoverage), formatCoverage(response.ComparisonCoverage), response.DecisionReady, metrics, attention)
}
func retailDiagnosticsAnswer(response *retailstore360.Response) string {
	if response == nil {
		return "\n\n该门店诊断暂无足够事实，未生成肯定性观察。"
	}
	status := map[bool]string{true: "complete", false: "insufficient"}[response.DecisionReady]
	return fmt.Sprintf("\n\n数据上下文：%s · dataset=%s · source=%s · as_of=%s · window=%s · diagnostics=%s · formula=%s · evidence=%s · confidence=%.2f。\n门店 %s（%s · %s · %s）的诊断包含 %d 个同群样本、%d 个变化桥；target 覆盖 %s，comparison 覆盖 %s，decision_ready=%t。摘要：%s；同群：%s；变化桥：%s。这些是变化观察与待核实信号，不能单独确认原因。", response.DataClassification, displayRetailValue(response.DatasetVersion), retailSourceLabel(response.SourceSystems), response.Current.DateTo, formatWindow(response.Current.DateFrom, response.Current.DateTo), response.DiagnosticsVersion, response.FormulaVersion, status, retailDiagnosticsConfidence(response), response.Store.StoreCode, response.Store.StoreName, response.Store.Brand, response.Store.Region, len(response.PeerBenchmark), len(response.Bridges), formatCoverage(response.TargetCoverage), formatCoverage(response.ComparisonCoverage), response.DecisionReady, retailDiagnosticsMetricSummary(response.Summary), retailPeerSummary(response.PeerBenchmark), retailBridgeSummary(response.Bridges))
}
func retailScenarioAnswer(response *retailscenario.Response, proposal bool) string {
	if response == nil {
		return "\n\n情景评估不可用；未保存任何业务行动。"
	}
	suffix := ""
	if proposal {
		suffix = " 已生成待人工确认的行动提议；请前往情景工作台二次确认。"
	}
	confidence := 0.40
	if response.Evidence.CoverageRate != nil && *response.Evidence.CoverageRate >= 100 {
		confidence = 0.90
	}
	return fmt.Sprintf("\n\n数据上下文：%s · dataset=%s · source=%s · as_of=%s · window=%s · scenario=%s · formula=%s · evidence=%s · confidence=%.2f。\n%s 情景使用 %d 个月 run-rate 和服务端 Baseline/Plan 计算，覆盖 %s；Baseline 门店贡献额=%s，Plan 月度变化=%s，Plan 期内变化=%s，贡献桥=%s；经营占用现金成本是 Working 经营口径，不是 IFRS 16 会计费用；结果不触达 IFRS 16 或 Official。%s", response.DataClassification, displayRetailValue(response.DatasetVersion), displayRetailValue(response.SourceSystem), response.Current.DateTo, formatWindow(response.Current.DateFrom, response.Current.DateTo), response.ScenarioVersion, response.FormulaVersion, map[bool]string{true: "complete", false: "insufficient"}[response.Evidence.CoverageRate != nil && *response.Evidence.CoverageRate >= 100], confidence, response.Store.StoreCode, response.HorizonMonths, formatScenarioCoverage(response.Evidence.ObservedStoreDays, response.Evidence.ExpectedStoreDays, response.Evidence.CoverageRate), scenarioMetricResult(response.Baseline.Metrics, "store_contribution"), scenarioPlanChange(response, false), scenarioPlanChange(response, true), retailScenarioBridgeSummary(response), suffix)
}

func retailPulseMetricSummary(summary map[string]retailpulse.SummaryMetric) string {
	keys := []string{"revenue", "gross_margin_rate", "store_contribution", "occupancy_cash_cost"}
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		metric, ok := summary[key]
		if !ok {
			continue
		}
		parts = append(parts, key+"="+formatRetailMetric(metric.Current, metric.Comparison, metric.ChangeValue, metric.ChangeType))
	}
	if len(parts) == 0 {
		return "—"
	}
	return strings.Join(parts, "；")
}

func retailPulseAttentionSummary(attention []retailpulse.Attention) string {
	if len(attention) == 0 {
		return "无"
	}
	parts := make([]string, 0, len(attention))
	for _, item := range attention {
		parts = append(parts, fmt.Sprintf("#%d %s score=%s severity=%s", item.Rank, item.StoreCode, formatRetailNumber(&item.Score, "count"), item.Severity))
	}
	return strings.Join(parts, "；")
}

func retailDiagnosticsMetricSummary(summary map[string]retailstore360.SummaryMetric) string {
	keys := []string{"revenue", "gross_margin_rate", "store_contribution", "occupancy_cash_cost"}
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		metric, ok := summary[key]
		if !ok {
			continue
		}
		parts = append(parts, key+"="+formatRetailMetric(metric.Current, metric.Comparison, metric.ChangeValue, metric.ChangeType))
	}
	if len(parts) == 0 {
		return "—"
	}
	return strings.Join(parts, "；")
}

func retailPeerSummary(peers []retailstore360.PeerBenchmark) string {
	if len(peers) == 0 {
		return "无有效同群基准"
	}
	parts := make([]string, 0, len(peers))
	for _, peer := range peers {
		parts = append(parts, fmt.Sprintf("%s median=%s status=%s", peer.Code, formatRetailNumber(peer.Median, peer.Unit), displayRetailValue(peer.Status)))
	}
	return strings.Join(parts, "；")
}

func retailBridgeSummary(bridges []retailstore360.Bridge) string {
	if len(bridges) == 0 {
		return "无可用变化桥"
	}
	parts := make([]string, 0, len(bridges))
	for _, bridge := range bridges {
		parts = append(parts, bridge.Code+"="+formatRetailNumber(bridge.TotalChange, "currency")+"/"+displayRetailValue(bridge.Status))
	}
	return strings.Join(parts, "；")
}

func scenarioMetricResult(metrics map[string]retailscenario.Metric, code string) string {
	metric, ok := metrics[code]
	if !ok {
		return "—"
	}
	return formatRetailNumber(metric.Result, metric.Unit) + " (" + displayRetailValue(metric.Status) + ")"
}

func scenarioPlanChange(response *retailscenario.Response, horizon bool) string {
	if len(response.Scenarios) == 0 {
		return "—"
	}
	plan := response.Scenarios[0]
	if horizon {
		return formatRetailNumber(plan.HorizonContributionChange, "currency")
	}
	return formatRetailNumber(plan.MonthlyContributionChange, "currency")
}

func retailScenarioBridgeSummary(response *retailscenario.Response) string {
	if len(response.Scenarios) == 0 {
		return "无可用变化桥"
	}
	bridge := response.Scenarios[0].Bridge
	parts := make([]string, 0, len(bridge.Items))
	for _, item := range bridge.Items {
		parts = append(parts, item.Code+"="+formatRetailNumber(item.Contribution, item.Unit))
	}
	if len(parts) == 0 {
		return displayRetailValue(bridge.Status)
	}
	return strings.Join(parts, "；")
}

func formatRetailKPI(value retailkpi.KPIValue) string {
	formatted := formatRetailNumber(value.Value, value.Unit)
	if value.Status != retailkpi.StatusComplete {
		formatted += " (" + displayRetailValue(string(value.Status))
		if value.Reason != "" {
			formatted += "/" + value.Reason
		}
		formatted += ")"
	}
	return formatted
}

func formatRetailMetric(current, comparison retailkpi.KPIValue, change *float64, changeType string) string {
	result := "current=" + formatRetailKPI(current) + ", comparison=" + formatRetailKPI(comparison)
	if change != nil {
		result += ", change=" + formatRetailNumber(change, current.Unit)
		if changeType != "" {
			result += " (" + changeType + ")"
		}
	}
	return result
}

func formatRetailNumber(value *float64, unit string) string {
	if value == nil {
		return "—"
	}
	precision := 2
	if unit == "count" || unit == "sqm" {
		precision = 0
	}
	return strconv.FormatFloat(*value, 'f', precision, 64) + map[string]string{"percent": "%", "percentage_point": "pp"}[unit]
}

func retailUnsupportedRequest(message string) bool {
	lower := strings.ToLower(message)
	for _, phrase := range []string{"删除", "执行行动", "改 forecast", "改forecast", "改合同", "创建合同", "修改合同", "写合同", "创建事件", "建事件", "过账", "自动通知", "通知房东", "联系房东", "搬迁", "选址", "关店", "official", "ifrs16", "ifrs 16", "月结", "付款计划"} {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return false
}

func retailContextConflict(message string, filters retailAgentFilters) string {
	lower := strings.ToLower(message)
	if filters.Classification != "" {
		explicit := ""
		if strings.Contains(lower, "production") || strings.Contains(message, "正式数据") {
			explicit = "production"
		} else if strings.Contains(lower, "simulated") || strings.Contains(message, "模拟数据") {
			explicit = "simulated"
		}
		if explicit != "" && explicit != filters.Classification {
			return fmt.Sprintf("页面上下文 classification=%s，但消息明确要求 %s；请确认后再读取，系统不会静默选择。", filters.Classification, explicit)
		}
	}
	if filters.AsOf != "" {
		if match := regexp.MustCompile(`(?i)(?:as[_ -]?of|截至)\s*[:=]?\s*(\d{4}-\d{2}-\d{2})`).FindStringSubmatch(message); len(match) == 2 && match[1] != filters.AsOf {
			return fmt.Sprintf("页面上下文 as_of=%s，但消息明确要求 as_of=%s；请确认后再读取。", filters.AsOf, match[1])
		}
	}
	if filters.WindowDays != 0 {
		if match := regexp.MustCompile(`(?i)(?:window[_ -]?days|窗口|最近)\s*[:=]?\s*(7|14|28)`).FindStringSubmatch(message); len(match) == 2 {
			if parsed, err := strconv.Atoi(match[1]); err == nil && parsed != filters.WindowDays {
				return fmt.Sprintf("页面上下文 window_days=%d，但消息明确要求 %d；请确认后再读取。", filters.WindowDays, parsed)
			}
		}
	}
	if filters.SourceSystem != "" {
		if match := regexp.MustCompile(`(?i)source[_ -]?system\s*[:=]\s*([\w.-]+)`).FindStringSubmatch(message); len(match) == 2 && match[1] != filters.SourceSystem {
			return fmt.Sprintf("页面上下文 source_system=%s，但消息明确要求 %s；请确认后再读取。", filters.SourceSystem, match[1])
		}
	}
	if filters.DatasetVersion != "" {
		if match := regexp.MustCompile(`(?i)(?:dataset[_ -]?version|dataset|数据集)\s*[:=]\s*([A-Za-z0-9][A-Za-z0-9._-]*)`).FindStringSubmatch(message); len(match) == 2 && match[1] != filters.DatasetVersion {
			return fmt.Sprintf("页面上下文 dataset_version=%s，但消息明确要求 %s；请确认后再读取。", filters.DatasetVersion, match[1])
		}
	}
	if filters.HorizonProvided {
		if match := regexp.MustCompile(`(?i)(?:horizon[_ -]?months|horizon|情景期|未来)\s*[:=]?\s*(3|6|12)\s*(?:个月|months?)?`).FindStringSubmatch(message); len(match) == 2 {
			if parsed, err := strconv.Atoi(match[1]); err == nil && parsed != filters.HorizonMonths {
				return fmt.Sprintf("页面上下文 horizon_months=%d，但消息明确要求 %d；请确认后再运行情景。", filters.HorizonMonths, parsed)
			}
		}
	}
	return ""
}

func retailPulseInsufficient(response *retailpulse.Response) bool {
	if response == nil || !response.DecisionReady {
		return true
	}
	if len(response.Summary) == 0 {
		return true
	}
	for _, metric := range response.Summary {
		if metric.Status != "complete" || metric.Current.Status != "complete" || metric.Comparison.Status != "complete" {
			return true
		}
	}
	return false
}

func retailPulseInsufficientReason(response *retailpulse.Response) string {
	if response == nil {
		return "data_unavailable"
	}
	if response.CurrentCoverage.ExpectedStoreDays == 0 && response.ComparisonCoverage.ExpectedStoreDays == 0 {
		return "no_facts"
	}
	if retailkpi.CoverageIncomplete(response.CurrentCoverage) || retailkpi.CoverageIncomplete(response.ComparisonCoverage) {
		return "partial_coverage"
	}
	for _, metric := range response.Summary {
		if metric.Status != "complete" || metric.Current.Status != "complete" || metric.Comparison.Status != "complete" {
			return "partial_metrics"
		}
	}
	return "data_unavailable"
}

func formatCoverage(coverage retailkpi.Coverage) string {
	rate := "—"
	if coverage.CoverageRate != nil {
		rate = fmt.Sprintf("%.1f%%", *coverage.CoverageRate)
	}
	return fmt.Sprintf("%d/%d store-days (%s)", coverage.ObservedStoreDays, coverage.ExpectedStoreDays, rate)
}

func formatScenarioCoverage(observed, expected int, rate *float64) string {
	value := "—"
	if rate != nil {
		value = fmt.Sprintf("%.1f%%", *rate)
	}
	return fmt.Sprintf("%d/%d store-days (%s)", observed, expected, value)
}

func formatWindow(from, to string) string {
	return fmt.Sprintf("%d天", daysBetweenDates(from, to))
}

func daysBetweenDates(from, to string) int {
	a, errA := time.Parse("2006-01-02", from)
	b, errB := time.Parse("2006-01-02", to)
	if errA != nil || errB != nil || b.Before(a) {
		return 0
	}
	return int(b.Sub(a).Hours()/24) + 1
}

func displayRetailValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "—"
	}
	return value
}

func retailSourceLabel(values []string) string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	if len(result) == 0 {
		return "—"
	}
	return strings.Join(result, ",")
}

func makeRetailActionProposal(response *retailscenario.Response, filters retailAgentFilters) *RetailActionProposal {
	if response == nil {
		return nil
	}
	values := url.Values{}
	values.Set("store_id", response.Store.StoreID)
	values.Set("as_of", response.Current.DateTo)
	values.Set("window_days", strconv.Itoa(filters.WindowDays))
	values.Set("data_classification", response.DataClassification)
	if response.DatasetVersion != "" {
		values.Set("dataset_version", response.DatasetVersion)
	}
	if response.SourceSystem != "" {
		values.Set("source_system", response.SourceSystem)
	}
	values.Set("horizon_months", strconv.Itoa(response.HorizonMonths))
	values.Set("revenue_change_pct", strconv.FormatFloat(filters.Assumptions.RevenueChangePct, 'f', -1, 64))
	values.Set("gross_margin_rate_change_pp", strconv.FormatFloat(filters.Assumptions.GrossMarginRateChangePP, 'f', -1, 64))
	values.Set("labor_cost_change_pct", strconv.FormatFloat(filters.Assumptions.LaborCostChangePct, 'f', -1, 64))
	values.Set("fixed_rent_change_pct", strconv.FormatFloat(filters.Assumptions.FixedRentChangePct, 'f', -1, 64))
	values.Set("variable_rent_rate_change_pp", strconv.FormatFloat(filters.Assumptions.VariableRentRateChangePP, 'f', -1, 64))
	values.Set("non_lease_cost_change_pct", strconv.FormatFloat(filters.Assumptions.NonLeaseCostChangePct, 'f', -1, 64))
	values.Set("other_controllable_cost_change_pct", strconv.FormatFloat(filters.Assumptions.OtherControllableCostChangePct, 'f', -1, 64))
	return &RetailActionProposal{Type: "retail_action_proposal", Status: "proposal", Title: "门店经营情景行动提议", Store: response.Store, PlannedAction: "前往情景工作台复核 Baseline/Plan、证据和负责人后再保存", OwnerName: nil, DueDate: nil, VerificationPeriod: "", Scenario: response, Evidence: response.Evidence, EvidenceComplete: response.Evidence.CoverageRate != nil && *response.Evidence.CoverageRate >= 100, Envelope: response.Envelope, DataClassification: response.DataClassification, DatasetVersion: response.DatasetVersion, SourceSystem: response.SourceSystem, FormulaVersion: response.FormulaVersion, FormalExecution: false, BusinessWrite: false, NextURL: "/scenario-workbench?" + values.Encode()}
}

func (h *Agent) retailSourceScope(req Request) map[string]string {
	if req.PageContext == nil {
		return nil
	}
	result := map[string]string{}
	for key, value := range req.PageContext.Filters {
		switch key {
		case "as_of", "window_days", "classification", "data_classification", "dataset_version", "source_system", "store_id", "store_ids", "horizon_months", "revenue_change_pct", "gross_margin_rate_change_pp", "labor_cost_change_pct", "fixed_rent_change_pct", "variable_rent_rate_change_pp", "non_lease_cost_change_pct", "other_controllable_cost_change_pct":
			result[key] = value
		}
	}
	return result
}
