package aiagent

import (
	"slices"

	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agentartifact"
	agentcorehooks "github.com/lease-management-system/core-service/internal/agentcore/hooks"
	"github.com/lease-management-system/core-service/internal/agentskill"
	"github.com/lease-management-system/core-service/internal/agenttools"
	agenttooldefs "github.com/lease-management-system/core-service/internal/agenttools/tools"
	"github.com/lease-management-system/core-service/internal/aichat"
	"github.com/lease-management-system/core-service/internal/aiintake"
	"github.com/lease-management-system/core-service/internal/pagefill"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/draftapp"
	"github.com/lease-management-system/core-service/internal/workingpaper"
)

type Agent struct {
	contractRepo  *repository.ContractRepository
	mcRepo        *repository.MonthlyClosingRepository
	eventRepo     *repository.EventRepository
	toolRuntime   *agenttools.Runtime
	skillRegistry *agentskill.Registry
}

// ToolRuntime exposes only the stable execution seam to an HTTP/CLI adapter.
// The Agent's repository wiring and model orchestration remain private.
func (h *Agent) ToolRuntime() *agenttools.Runtime {
	if h == nil {
		return nil
	}
	return h.toolRuntime
}

// SkillRegistry exposes the read-only skill descriptor seam to adapters.
// Skill selection and execution remain server-owned.
func (h *Agent) SkillRegistry() *agentskill.Registry {
	if h == nil {
		return nil
	}
	return h.skillRegistry
}

// NewWithOperationalReadersAndGovernanceAndRetail is the production
// constructor: it wires the operating-facts / close-readiness / control /
// governance / retail seams into the same governed Agent Tool Runtime.
func NewWithOperationalReadersAndGovernanceAndRetail(contractRepo *repository.ContractRepository, mcRepo *repository.MonthlyClosingRepository, eventRepo *repository.EventRepository, performance agenttooldefs.PerformanceReader, closeReadiness agenttooldefs.CloseReadinessReader, controls *agenttooldefs.ControlReaders, governance agenttooldefs.DecisionMemoDraftWriter, retail agenttooldefs.RetailOperationsReader, sensitivity agenttooldefs.SensitivityReader, fillReader agenttooldefs.IngestFileReader, draftServices ...*draftapp.Service) *Agent {
	return newAgent(contractRepo, mcRepo, eventRepo, performance, closeReadiness, controls, governance, retail, sensitivity, fillReader, draftServices...)
}

func newAgent(contractRepo *repository.ContractRepository, mcRepo *repository.MonthlyClosingRepository, eventRepo *repository.EventRepository, performance agenttooldefs.PerformanceReader, closeReadiness agenttooldefs.CloseReadinessReader, controls *agenttooldefs.ControlReaders, governance agenttooldefs.DecisionMemoDraftWriter, retail agenttooldefs.RetailOperationsReader, sensitivity agenttooldefs.SensitivityReader, fillReader agenttooldefs.IngestFileReader, draftServices ...*draftapp.Service) *Agent {
	agent := &Agent{
		contractRepo: contractRepo, mcRepo: mcRepo, eventRepo: eventRepo,
		skillRegistry: agentskill.ProductionRegistry(),
	}
	registry := agenttools.NewRegistry()
	registered := false
	if contractRepo != nil {
		_ = registry.Register(agenttooldefs.NewContractSearchDefinition(contractRepo))
		if err := registry.Register(agenttooldefs.NewContractGetDefinition(contractRepo)); err == nil {
			registered = true
			if mcRepo != nil {
				_ = registry.Register(agenttooldefs.NewMeasurementListDefinition(contractRepo, mcRepo))
				_ = registry.Register(agenttooldefs.NewJournalListDefinition(contractRepo, mcRepo))
			}
			if eventRepo != nil {
				_ = registry.Register(agenttooldefs.NewEventListDefinition(contractRepo, eventRepo))
			}
		}
	}
	for _, definition := range agent.fileParseDefinitions() {
		if err := registry.Register(definition); err == nil {
			registered = true
		}
	}
	if err := registry.Register(agenttooldefs.NewDocTriageDefinition(nil)); err == nil {
		registered = true
	}
	if err := registry.Register(agenttooldefs.NewS1GenerateDefinition()); err == nil {
		registered = true
	}
	// The fill seam registers without a file reader for now (D-D2): the tool
	// refuses honestly until W5 wires minio-go into core-service.
	if err := registry.Register(agenttooldefs.NewRetailIngestPreviewDefinition(fillReader)); err == nil {
		registered = true
	}
	// SM7：三表模型工具注册。端口工厂当前为空——工具诚实拒绝
	// （unavailable），生产接线随 /financial-model 工作台落地。
	if err := registry.Register(agenttooldefs.NewStatementModelReadDefinition(nil)); err == nil {
		registered = true
	}
	if err := registry.Register(agenttooldefs.NewStatementModelEvaluateDefinition(nil)); err == nil {
		registered = true
	}
	if err := registry.Register(agenttooldefs.NewFinModelPaperDefinition(nil)); err == nil {
		registered = true
	}
	if err := registry.Register(agenttooldefs.NewAssumptionSuggestionDefinition(nil)); err == nil {
		registered = true
	}
	// S4-3 / S4-4：批量假设初稿（按区块）与模型差异四层备忘录。与 SM7 同一
	// 策略：写入端口当前为空，工具诚实拒绝；生产接线随工作台阶段落地。
	if err := registry.Register(agenttooldefs.NewAssumptionSuggestionBatchDefinition(nil)); err == nil {
		registered = true
	}
	if err := registry.Register(agenttooldefs.NewModelDiffMemoDefinition(nil)); err == nil {
		registered = true
	}
	if len(draftServices) > 0 && draftServices[0] != nil {
		if err := registry.Register(agenttooldefs.NewContractDraftDefinition(draftServices[0])); err == nil {
			registered = true
		}
		if err := registry.Register(agenttooldefs.NewPaymentScheduleDraftDefinition(draftServices[0])); err == nil {
			registered = true
		}
		if eventRepo != nil {
			if err := registry.Register(agenttooldefs.NewEventDraftDefinition(draftServices[0])); err == nil {
				registered = true
			}
		}
	}
	if performance != nil {
		for _, definition := range []agenttools.ToolDefinition{
			agenttooldefs.NewPortfolioSummaryDefinition(performance),
			agenttooldefs.NewManagementPreReadDefinition(performance),
			agenttooldefs.NewStorePerformanceDefinition(performance),
			agenttooldefs.NewRentToSalesDefinition(performance),
			agenttooldefs.NewEquipmentPerformanceDefinition(performance),
			agenttooldefs.NewActionListDefinition(performance),
		} {
			if err := registry.Register(definition); err == nil {
				registered = true
			}
		}
		if writer, ok := performance.(agenttooldefs.ActionDraftWriter); ok {
			if err := registry.Register(agenttooldefs.NewActionDraftDefinition(writer)); err == nil {
				registered = true
			}
			if err := registry.Register(agenttooldefs.NewExplanationDraftDefinition(writer)); err == nil {
				registered = true
			}
			if err := registry.Register(agenttooldefs.NewMeetingActionDraftDefinition(writer)); err == nil {
				registered = true
			}
		}
		if writer, ok := performance.(agenttooldefs.ScenarioDraftWriter); ok {
			if err := registry.Register(agenttooldefs.NewScenarioDraftDefinition(writer)); err == nil {
				registered = true
			}
		}
	}
	if governance != nil {
		if err := registry.Register(agenttooldefs.NewDecisionMemoDraftDefinition(governance)); err == nil {
			registered = true
		}
	}
	for _, definition := range []agenttools.ToolDefinition{
		agenttooldefs.NewStoreScenarioDefinition(),
		agenttooldefs.NewEquipmentScenarioDefinition(),
		agenttooldefs.NewDealSimulationDefinition(),
		agenttooldefs.NewPreDealSimulationDefinition(),
		agenttooldefs.NewRenewalSimulationDefinition(),
		agenttooldefs.NewDecisionSummaryDefinition(),
	} {
		if err := registry.Register(definition); err == nil {
			registered = true
		}
	}
	if closeReadiness != nil {
		if err := registry.Register(agenttooldefs.NewCloseReadinessDefinition(closeReadiness)); err == nil {
			registered = true
		}
	}
	if controls != nil {
		for _, definition := range []agenttools.ToolDefinition{
			agenttooldefs.NewBudgetVarianceDefinition(controls.Budget),
			agenttooldefs.NewCashflowScenarioDefinition(controls.Cashflow),
			agenttooldefs.NewRenewalDecisionDefinition(controls.Renewal),
		} {
			if err := registry.Register(definition); err == nil {
				registered = true
			}
		}
	}
	if retail != nil {
		for _, definition := range []agenttools.ToolDefinition{
			agenttooldefs.NewRetailOperatingPulseDefinition(retail),
			agenttooldefs.NewRetailStoreDiagnosticsDefinition(retail),
			agenttooldefs.NewRetailScenarioEvaluateDefinition(retail),
			agenttooldefs.NewRetailPaperDefinition(retail),
		} {
			if err := registry.Register(definition); err == nil {
				registered = true
			}
		}
	}
	if sensitivity != nil {
		if err := registry.Register(agenttooldefs.NewSensitivityDefinition(sensitivity)); err == nil {
			registered = true
		}
	}
	if registered {
		// W2: every tool call in the chat plane crosses the ordered governance
		// chain (TenantScope → CapabilityCheck → ProtectedMeasure →
		// BudgetGuard → IdempotencyGuard → ReviewGate) instead of scattered
		// policy checks. Behaviour is equivalent; the chain is the mount
		// point for future controls.
		before, after := agentcorehooks.Governance(agentcorehooks.Deps{
			Policy:             agenttools.DefaultPolicy(),
			RequireDraftReview: true,
		})
		agent.toolRuntime = agenttools.NewRuntime(registry, agenttools.RuntimeOptions{
			Guard: agentcorehooks.NewExecutionGuard(before, after),
		})
	}
	return agent
}

func (h *Agent) Plan(input aichat.Input, sourceRun *repository.AIChatRun) aichat.Plan {
	req := requestFromRuntimeInput(input)
	effectiveContractID := effectiveContractIDFromRequest(req)
	var runbook *AgentRunbook
	if h != nil {
		registry := h.skillRegistry
		if registry == nil {
			registry = agentskill.ProductionRegistry()
		}
		if definition, ok := registry.Select(agentskill.Intent{
			Message: input.Message, Role: input.Role,
			HasFile:          input.FileID != "" && input.ObjectName != "",
			HasContract:      effectiveContractID != "",
			RequestedSkillID: input.SkillID, RequestedVersion: input.SkillVersion,
		}); ok {
			runbook = runbookForSkillID(definition.ID, req, effectiveContractID)
			if runbook != nil {
				runbook.SkillVersion = definition.Version
			}
		}
	}
	if runbook == nil && strings.TrimSpace(input.SkillID) == "" {
		runbook = h.buildAgentRunbook(req, effectiveContractID)
	}
	if runbook == nil && strings.TrimSpace(input.SkillID) == "" && sourceRun != nil && sourceRun.SkillID != nil {
		runbook = runbookForSkillID(*sourceRun.SkillID, req, effectiveContractID)
		if runbook != nil && h != nil {
			registry := h.skillRegistry
			if registry == nil {
				registry = agentskill.ProductionRegistry()
			}
			version := ""
			if sourceRun.SkillVersion != nil {
				version = *sourceRun.SkillVersion
			}
			if definition, ok := registry.Resolve(*sourceRun.SkillID, version); ok {
				runbook.SkillVersion = definition.Version
			}
		}
	}
	if runbook == nil {
		return aichat.Plan{}
	}
	// G1 bridge trigger: a user request that asks for a runner-executed
	// scenario (structured assumptions → planner → engine) queues the run
	// for the worker instead of answering with display-only cards. The
	// system-initiated morning brief and the retail deterministic plane are
	// never dispatched.
	queueForWorker := false
	if input.Initiator != "system" && runbook.SkillID != "retail_operations" {
		if tool, ok := runnerIntentTool(input.Message); ok && runbookHasTool(runbook, tool) {
			queueForWorker = true
		}
	}
	return aichat.Plan{
		AgentMode: true, SkillID: runbook.SkillID, SkillVersion: runbook.SkillVersion,
		ReviewRequired: len(runbook.ReviewPrompts) > 0,
		AgentPlan:      runbook.AgentPlan, ToolCalls: runbook.ToolCalls,
		ReviewPrompts: runbook.ReviewPrompts, Payload: runbook,
		QueueForWorker: queueForWorker,
	}
}

// runnerIntentFamilies maps user phrases to the runner-executed simulation
// tools. A phrase family that hits a card the chat plane never executes
// deterministically is the G1 dispatch signal; misses keep today's behaviour.
var runnerIntentFamilies = []struct {
	tool  string
	terms []string
}{
	{"lease.renewal.simulate", []string{"续租方案", "退租", "关店测算", "搬迁", "议价空间", "renewal scenario"}},
	{"lease.deal.simulate", []string{"对比报价", "报价对比", "compare offers", "deal simulate"}},
	{"lease.predeal.simulate", []string{"签约前测算", "签约前方案", "pre-deal"}},
	{"lease.cashflow.scenario", []string{"现金流情景", "cashflow scenario", "现金流测算"}},
	{"lease.store.scenario.simulate", []string{"门店情景测算", "门店搬迁测算", "门店关店测算"}},
	{"lease.equipment.scenario.simulate", []string{"设备 buy/lease", "设备情景"}},
	{"lease.decision.summary", []string{"决策摘要", "决策备忘", "decision summary"}},
	{"lease.fpna.action.draft.create", []string{"行动草稿", "action draft"}},
	{"lease.decision.memo.draft.create", []string{"决策备忘录草稿"}},
	{"lease.meeting.action.draft.create", []string{"会议行动草稿"}},
}

// runnerIntentTool returns the runner tool a message asks for, if any.
func runnerIntentTool(message string) (string, bool) {
	lower := strings.ToLower(message)
	for _, family := range runnerIntentFamilies {
		for _, term := range family.terms {
			if strings.Contains(lower, term) {
				return family.tool, true
			}
		}
	}
	return "", false
}

// runbookHasTool reports whether the runbook declares the given tool card.
func runbookHasTool(runbook *AgentRunbook, tool string) bool {
	if runbook == nil {
		return false
	}
	for _, call := range runbook.ToolCalls {
		if strings.EqualFold(strings.TrimSpace(call.Tool), tool) {
			return true
		}
	}
	return false
}

func runbookForSkillID(skillID string, req Request, effectiveContractID string) *AgentRunbook {
	hasFile := req.FileID != "" && req.ObjectName != ""
	hasContractContext := effectiveContractID != ""
	switch skillID {
	case "excel_ledger", "contract_batch_intake":
		return buildContractLedgerRunbook(hasFile)
	case "contract_review":
		return buildContractReviewRunbook(hasFile, hasContractContext)
	case "payment_schedule", "payment_schedule_intake":
		return buildPaymentScheduleRunbook(hasFile, hasContractContext)
	case "event_change":
		return buildEventChangeRunbook(hasContractContext)
	case "audit_pack":
		return buildAuditPackRunbook()
	case "fpna_copilot", "retail_performance", "manufacturing_performance":
		return buildPerformanceRunbook(skillID, req)
	case "retail_operations":
		return buildRetailOperationsRunbook(req)
	default:
		return nil
	}
}

func buildPerformanceRunbook(skillID string, req Request) *AgentRunbook {
	period := "当前期间"
	if req.PageContext != nil && req.PageContext.Period != "" {
		period = req.PageContext.Period
	}
	return &AgentRunbook{
		SkillID: skillID, SkillName: "FP&A / Finance BP Copilot", NeedsPortfolioContext: true,
		AnswerPrefix: fmt.Sprintf("Agent 已切换到经营决策 Copilot。我会基于权限范围内的系统事实、期间 %s 和可追溯来源，先解释偏差与数据缺口，再提出需要人工确认的行动。AI 不会直接修改正式台账或 Official 版本。", period),
		AgentPlan: []AgentPlanStep{
			{ID: "load_operating_facts", Title: "读取经营组合与事实版本", Status: "completed"},
			{ID: "check_data_quality", Title: "识别覆盖、版本和数据质量缺口", Status: "needs_review"},
			{ID: "explain_drivers", Title: "使用确定性指标和残差解释驱动", Status: "needs_review"},
			{ID: "prepare_actions", Title: "形成待人工确认的行动建议", Status: "pending"},
		},
		ToolCalls: []AgentToolCall{
			{Tool: "lease.portfolio.summary", Skill: "FP&A Copilot", Status: "completed", InputSummary: "读取经营组合摘要", OutputSummary: "返回事实覆盖、数据准备度和行动影响", RequiresReview: false},
			{Tool: "lease.management.pre_read", Skill: "Management Reporting", Status: "pending", InputSummary: "等待期间和权限范围", OutputSummary: "生成会前摘要、优先行动和问题清单", RequiresReview: true},
			{Tool: "lease.store.performance", Skill: "Retail Finance BP", Status: "completed", InputSummary: "读取门店四墙表现", OutputSummary: "返回四墙 EBITDA、租售比、坪效和数据缺口", RequiresReview: false},
			{Tool: "lease.rent_to_sales", Skill: "Retail Finance BP", Status: "completed", InputSummary: "读取门店租售比", OutputSummary: "返回固定/变量租金与销售额的租售比及证据缺口", RequiresReview: false},
			{Tool: "lease.equipment.performance", Skill: "Manufacturing Finance BP", Status: "completed", InputSummary: "读取设备经营事实", OutputSummary: "返回成本桥和未解释残差", RequiresReview: false},
			{Tool: "lease.fpna.actions", Skill: "FP&A Action Center", Status: "needs_review", InputSummary: "读取异常和行动兑现状态", OutputSummary: "仅生成带来源的行动草稿，正式动作需人工确认", RequiresReview: true},
			{Tool: "lease.close.readiness", Skill: "FP&A Close Readiness", Status: "needs_review", InputSummary: "读取期间关账准备度和证据缺口", OutputSummary: "返回阻塞项与证据缺口，不会自动关账或过账", RequiresReview: true},
			{Tool: "lease.budget.variance", Skill: "FP&A Driver Bridge", Status: "pending", InputSummary: "等待预算/Forecast 版本和期间", OutputSummary: "返回确定性差异桥与 residual", RequiresReview: false},
			{Tool: "lease.cashflow.scenario", Skill: "FP&A Cash Planning", Status: "pending", InputSummary: "等待现金流情景结构化假设", OutputSummary: "返回无副作用租赁现金流 Scenario", RequiresReview: true},
			{Tool: "lease.renewal.decisions", Skill: "Retail Finance BP", Status: "pending", InputSummary: "等待合同范围内续租决策快照", OutputSummary: "读取既有 Scenario 证据，不会创建续租事件", RequiresReview: false},
			{Tool: "lease.fpna.action.draft.create", Skill: "FP&A Action Center", Status: "pending", InputSummary: "等待用户确认解释、负责人和预期收益", OutputSummary: "创建 Assist Mode 行动草稿，需 Review Gate", RequiresReview: true},
			{Tool: "lease.store.scenario.simulate", Skill: "Retail Finance BP", Status: "pending", InputSummary: "等待门店续租/议价/搬迁/关店结构化假设", OutputSummary: "返回无副作用 Scenario 结果，不写入合同", RequiresReview: true},
			{Tool: "lease.equipment.scenario.simulate", Skill: "Manufacturing Finance BP", Status: "pending", InputSummary: "等待设备 Buy/Lease/Replace/Outsource 假设", OutputSummary: "返回经营 NPV 与 IFRS 16 分开展示", RequiresReview: true},
			{Tool: "lease.deal.simulate", Skill: "Deal / Pre-deal Finance", Status: "pending", InputSummary: "等待候选报价和已确认折现率", OutputSummary: "返回有效租金、现值和现金比较；无副作用", RequiresReview: true},
			{Tool: "lease.predeal.simulate", Skill: "Deal / Pre-deal Finance", Status: "pending", InputSummary: "等待签约前结构化条款", OutputSummary: "返回费用曲线、EBITDA 桥和退出曲线", RequiresReview: true},
			{Tool: "lease.renewal.simulate", Skill: "Retail Finance BP", Status: "pending", InputSummary: "等待续租/退出情景及已确认折现率", OutputSummary: "返回现金、损益和 IFRS 16 影响；无副作用", RequiresReview: true},
			{Tool: "lease.decision.summary", Skill: "Decision Memo", Status: "pending", InputSummary: "等待事实、计算、假设和反方论点", OutputSummary: "生成一页决策摘要，保留数据缺口和待问问题", RequiresReview: true},
			{Tool: "lease.fpna.scenario.draft.create", Skill: "FP&A Scenario Governance", Status: "pending", InputSummary: "等待用户确认假设和确定性计算结果", OutputSummary: "保存 Scenario 草稿，需 Review Gate，不覆盖 Budget/Forecast", RequiresReview: true},
			{Tool: "lease.decision.memo.draft.create", Skill: "Decision Memo", Status: "pending", InputSummary: "等待系统事实、确定性计算和人类输入确认", OutputSummary: "保存分层决策备忘录草稿，需 Review Gate", RequiresReview: true},
			{Tool: "lease.meeting.action.draft.create", Skill: "Meeting Follow-up", Status: "pending", InputSummary: "等待会议纪要、负责人和截止日期确认", OutputSummary: "保存会议行动草稿，后续用 Actual 验证兑现", RequiresReview: true},
		},
		ReviewPrompts: []AgentReviewPrompt{{ID: "fpna_explanation_review", Title: "确认经营解释和行动", Description: "请确认系统事实、数据覆盖、驱动解释与行动负责人；AI 建议不会自动成为正式结论。", Severity: "warning", Action: "复核来源和残差后确认解释或创建行动草稿。"}},
	}
}

func (h *Agent) Execute(ctx context.Context, execution aichat.Execution) (Response, error) {
	runbook, _ := execution.Plan.Payload.(*AgentRunbook)
	scope, scoped := access.ScopeFromContext(ctx)
	if !scoped {
		scope = access.Scope{
			Global:        hasGlobalPermission(execution.Input.Permissions),
			LegalEntityID: execution.Input.LegalEntityID,
		}
	}
	runID := ""
	if execution.Run != nil {
		runID = execution.Run.ID
	}
	toolRuntime := h.toolRuntime
	if toolRuntime != nil {
		toolRuntime = toolRuntime.WithAudit(agenttools.AuditRecorderFunc(func(auditCtx context.Context, audit agenttools.ToolExecutionAudit) error {
			if execution.Emit == nil {
				return nil
			}
			return execution.Emit(auditCtx, "tool_execution", audit)
		}))
	}
	ctx = withAIServiceAuth(ctx, execution.Input.AuthHeader)
	ctx = agenttools.WithExecutionContext(ctx, agenttools.ExecutionContext{
		Principal: agenttools.Principal{
			UserID:      execution.Input.UserID,
			SubjectType: "web_ai_agent",
			Role:        execution.Input.Role,
			Permissions: append([]string(nil), execution.Input.Permissions...),
			Scope:       scope,
			AgentMode:   "assist",
		},
		RunID: runID, SkillID: execution.Plan.SkillID, SkillVersion: execution.Plan.SkillVersion,
	})
	return h.executeChatRequest(ctx, execution.Input.AuthHeader, requestFromRuntimeInput(execution.Input), execution.Input.LegalEntityID, execution.Input.UserID, execution.Input.Role, execution.Emit, runbook, toolRuntime)
}

func hasGlobalPermission(permissions []string) bool {
	for _, permission := range permissions {
		if strings.EqualFold(strings.TrimSpace(permission), "*:*") {
			return true
		}
	}
	return false
}

// confidencePointer carries the answer confidence forward for persistence;
// a zero or unset confidence is treated as absent rather than fabricated.
func confidencePointer(confidence float64) *float64 {
	if confidence <= 0 {
		return nil
	}
	return &confidence
}

// confidenceReasonFor derives the degradation reason from signals the agent
// already produced. It explains why the confidence is what it is; the
// confidence calculation itself is untouched.
func confidenceReasonFor(response Response) *string {
	if strings.EqualFold(response.Model, "fallback") {
		reason := "AI 服务暂不可用，以下为系统数据摘要"
		return &reason
	}
	if len(response.ReviewPrompts) > 0 {
		reason := "部分内容需人工复核"
		return &reason
	}
	return nil
}

func ProjectResult(response Response) aichat.Result {
	result := aichat.Result{
		Answer: response.Answer, Model: response.Model, Sources: response.Sources,
		ToolCalls: response.ToolCalls, ReviewPrompts: response.ReviewPrompts,
		ReviewRequired:   len(response.ReviewPrompts) > 0,
		Confidence:       confidencePointer(response.Confidence),
		ConfidenceReason: confidenceReasonFor(response),
	}
	if len(response.DraftContracts) > 0 {
		result.Artifacts = append(result.Artifacts, aichat.ArtifactDraft{
			Type: string(agentartifact.ArtifactContractDraft), Title: "合同草稿", ReviewRequired: true,
			SchemaVersion: agentartifact.SchemaVersion, EvidenceRefs: response.EvidenceRefs,
			EvidenceComplete: response.BatchSummary != nil && response.BatchSummary.EvidenceComplete,
			ReviewReasons:    batchReviewReasons(response.BatchSummary), ModelVersion: response.Model, RuleVersion: "lease-agent-rule.v1",
			Data: map[string]any{"contracts": response.DraftContracts, "summary": response.BatchSummary},
		})
	}
	if len(response.DraftPaymentSchedules) > 0 {
		result.Artifacts = append(result.Artifacts, aichat.ArtifactDraft{
			Type: string(agentartifact.ArtifactPaymentScheduleDraft), Title: "付款计划草稿", ReviewRequired: true,
			SchemaVersion: agentartifact.SchemaVersion, EvidenceRefs: response.EvidenceRefs,
			EvidenceComplete: response.PaymentScheduleSummary != nil && response.PaymentScheduleSummary.EvidenceComplete,
			ReviewReasons:    paymentReviewReasons(response.PaymentScheduleSummary), ModelVersion: response.Model, RuleVersion: "lease-agent-rule.v1",
			Data: map[string]any{"schedules": response.DraftPaymentSchedules, "summary": response.PaymentScheduleSummary},
		})
	}
	if response.AuditPack != nil {
		reviewReasons := []string{"audit_scope_confirmation", "report_basis_confirmation"}
		evidenceComplete := len(response.EvidenceRefs) > 0
		if !evidenceComplete {
			reviewReasons = append(reviewReasons, "evidence_incomplete")
		}
		result.Artifacts = append(result.Artifacts, aichat.ArtifactDraft{
			Type: string(agentartifact.ArtifactAuditPack), Title: "审计包准备摘要", ReviewRequired: true,
			SchemaVersion: agentartifact.SchemaVersion, EvidenceRefs: response.EvidenceRefs,
			EvidenceComplete: evidenceComplete, ReviewReasons: reviewReasons,
			ModelVersion: response.Model, RuleVersion: "audit-pack-rule.v1",
			Data: response.AuditPack,
		})
	}
	if response.ReportExplanation != nil {
		evidenceRefs := response.EvidenceRefs
		if len(evidenceRefs) == 0 {
			evidenceRefs = evidenceReferencesFromSources(response.Sources)
		}
		evidenceComplete := len(evidenceRefs) > 0
		result.Artifacts = append(result.Artifacts, aichat.ArtifactDraft{
			Type: string(agentartifact.ArtifactReportExplanation), Title: "报表解释摘要", ReviewRequired: true,
			SchemaVersion: agentartifact.SchemaVersion, EvidenceRefs: evidenceRefs,
			EvidenceComplete: evidenceComplete, ReviewReasons: []string{"report_basis_confirmation", "ai_explanation_review"},
			ModelVersion: response.Model, RuleVersion: "report-explanation-rule.v1", Data: response.ReportExplanation,
		})
	}
	if response.EventDraft != nil {
		evidenceRefs := response.EvidenceRefs
		if len(evidenceRefs) == 0 {
			evidenceRefs = evidenceReferencesFromSources(response.Sources)
		}
		evidenceComplete := len(evidenceRefs) > 0
		result.Artifacts = append(result.Artifacts, aichat.ArtifactDraft{
			Type: string(agentartifact.ArtifactEventDraft), Title: "合同事件草稿", ReviewRequired: true,
			SchemaVersion: agentartifact.SchemaVersion, EvidenceRefs: evidenceRefs,
			EvidenceComplete: evidenceComplete, ReviewReasons: []string{"event_draft_review", "accounting_treatment_missing"},
			ModelVersion: response.Model, RuleVersion: "event-draft-rule.v1",
			Data: map[string]any{"event": response.EventDraft},
		})
	}
	if response.PageFill != nil {
		result.Artifacts = append(result.Artifacts, aichat.ArtifactDraft{
			Type: string(agentartifact.ArtifactPageFill), Title: "零售导入预填",
			ReviewRequired:   true,
			SchemaVersion:    agentartifact.SchemaVersion,
			EvidenceComplete: true,
			ReviewReasons:    []string{"import_mapping_review"},
			ModelVersion:     response.Model,
			RuleVersion:      "page-fill-rule.v1",
			Data:             response.PageFill,
		})
	}
	if response.WorkingPaper != nil {
		paper := *response.WorkingPaper
		paper = workingpaper.Build(paper, time.Now())
		evidenceRefs := response.EvidenceRefs
		result.Artifacts = append(result.Artifacts, aichat.ArtifactDraft{
			Type: string(agentartifact.ArtifactWorkingPaper), Title: paper.Title,
			ReviewRequired:   true,
			SchemaVersion:    agentartifact.SchemaVersion,
			EvidenceRefs:     evidenceRefs,
			EvidenceComplete: false,
			ReviewReasons:    workingPaperReviewReasons(paper),
			ModelVersion:     response.Model,
			RuleVersion:      "s1-paper-rule.v1",
			Data:             paper,
		})
	}
	if response.RetailActionProposal != nil {
		proposal := response.RetailActionProposal
		evidenceRefs := response.EvidenceRefs
		if len(evidenceRefs) == 0 {
			evidenceRefs = evidenceReferencesFromSources(response.Sources)
		}
		result.Artifacts = append(result.Artifacts, aichat.ArtifactDraft{
			Type: "retail_action_proposal", Title: proposal.Title, ReviewRequired: true,
			SchemaVersion: agentartifact.SchemaVersion, EvidenceRefs: evidenceRefs,
			EvidenceComplete: proposal.EvidenceComplete && len(evidenceRefs) > 0,
			ReviewReasons:    []string{"retail_action_review", "scenario_workbench_confirmation"},
			ModelVersion:     response.Model, RuleVersion: "retail-operations-rule.v1", Data: proposal,
		})
	}
	appendDataQualityArtifacts(&result, response)
	return result
}

func appendDataQualityArtifacts(result *aichat.Result, response Response) {
	if result == nil {
		return
	}
	appendOne := func(title, source string, missingFields, warnings, reasons []string, intakeID string, evidenceComplete bool) {
		if len(missingFields) == 0 && len(warnings) == 0 {
			return
		}
		reviewReasons := append([]string{"data_quality_review"}, reasons...)
		evidenceRefs := append([]agentartifact.EvidenceReference(nil), response.EvidenceRefs...)
		if len(evidenceRefs) == 0 && strings.TrimSpace(intakeID) != "" {
			evidenceRefs = []agentartifact.EvidenceReference{{
				ReferenceID: intakeID, Complete: false, MissingReason: "解析任务未提供可定位的原文证据",
			}}
		}
		result.Artifacts = append(result.Artifacts, aichat.ArtifactDraft{
			Type: string(agentartifact.ArtifactDataQualityIssues), Title: title, ReviewRequired: true,
			SchemaVersion: agentartifact.SchemaVersion, EvidenceRefs: evidenceRefs,
			EvidenceComplete: evidenceComplete && len(evidenceRefs) > 0, ReviewReasons: reviewReasons,
			ModelVersion: response.Model, RuleVersion: "agent-data-quality-rule.v1",
			Data: map[string]any{
				"source": source, "missing_fields": missingFields, "warnings": warnings,
				"review_reasons": reviewReasons, "intake_id": intakeID,
			},
		})
	}
	if response.BatchSummary != nil {
		appendOne("合同数据质量问题", "contract_batch", response.BatchSummary.MissingFields,
			response.BatchSummary.Warnings, response.BatchSummary.ReviewReasons, response.BatchSummary.IntakeID,
			response.BatchSummary.EvidenceComplete)
	}
	if response.PaymentScheduleSummary != nil {
		appendOne("付款计划数据质量问题", "payment_schedule", response.PaymentScheduleSummary.MissingFields,
			response.PaymentScheduleSummary.Warnings, response.PaymentScheduleSummary.ReviewReasons,
			response.PaymentScheduleSummary.IntakeID, response.PaymentScheduleSummary.EvidenceComplete)
	}
}

func batchReviewReasons(summary *BatchParseSummary) []string {
	if summary == nil {
		return []string{"assist_mode"}
	}
	return append([]string(nil), summary.ReviewReasons...)
}

func paymentReviewReasons(summary *PaymentScheduleParseSummary) []string {
	if summary == nil {
		return []string{"assist_mode"}
	}
	return append([]string(nil), summary.ReviewReasons...)
}

func requestFromRuntimeInput(input aichat.Input) Request {
	return Request{
		SessionID: input.SessionID, Message: input.Message,
		ContractID: input.ContractID, History: input.History,
		FileID: input.FileID, ObjectName: input.ObjectName, ContentType: input.ContentType,
		PageContext: input.PageContext, Language: input.Language, SkillID: input.SkillID, SkillVersion: input.SkillVersion,
	}
}

type PageContext = aichat.PageContext

type Request struct {
	SessionID    string        `json:"session_id,omitempty"`
	RunID        string        `json:"run_id,omitempty"`
	Message      string        `json:"message" binding:"required"`
	ContractID   string        `json:"contract_id,omitempty"`
	History      []ChatMessage `json:"history,omitempty"`
	FileID       string        `json:"file_id,omitempty"`
	ObjectName   string        `json:"object_name,omitempty"`
	ContentType  string        `json:"content_type,omitempty"`
	PageContext  *PageContext  `json:"page_context,omitempty"`
	Language     string        `json:"language,omitempty"`
	SkillID      string        `json:"skill_id,omitempty"`
	SkillVersion string        `json:"skill_version,omitempty"`
	// Initiator marks a run that creates a system-initiated session when no
	// session_id is given (CHAT-001: the home brief). "user" or empty keeps
	// the session user-visible; "system" hides it from the session list while
	// the run and audit trail persist as usual.
	Initiator string `json:"initiator,omitempty"`
}

type ChatMessage = aichat.Message

type Response struct {
	SessionID              string                            `json:"session_id,omitempty"`
	RunID                  string                            `json:"run_id,omitempty"`
	Answer                 string                            `json:"answer"`
	Sources                []Source                          `json:"sources"`
	Confidence             float64                           `json:"confidence"`
	IsOfficial             bool                              `json:"is_official"`
	Model                  string                            `json:"model,omitempty"`
	AgentMode              bool                              `json:"agent_mode,omitempty"`
	AgentPlan              []AgentPlanStep                   `json:"agent_plan,omitempty"`
	ToolCalls              []AgentToolCall                   `json:"tool_calls,omitempty"`
	ReviewPrompts          []AgentReviewPrompt               `json:"review_prompts,omitempty"`
	DraftContracts         []ContractDraftItem               `json:"draft_contracts,omitempty"`
	BatchSummary           *BatchParseSummary                `json:"batch_summary,omitempty"`
	DraftPaymentSchedules  []PaymentScheduleDraftItem        `json:"draft_payment_schedules,omitempty"`
	PaymentScheduleSummary *PaymentScheduleParseSummary      `json:"payment_schedule_summary,omitempty"`
	EvidenceRefs           []agentartifact.EvidenceReference `json:"evidence_refs,omitempty"`
	AuditPack              *AuditPackData                    `json:"audit_pack,omitempty"`
	ReportExplanation      *ReportExplanationData            `json:"report_explanation,omitempty"`
	EventDraft             *EventDraftData                   `json:"event_draft,omitempty"`
	RetailOperations       *RetailOperationsData             `json:"retail_operations,omitempty"`
	RetailActionProposal   *RetailActionProposal             `json:"retail_action_proposal,omitempty"`
	FileTriage             *agenttooldefs.TriageResult       `json:"file_triage,omitempty"`
	WorkingPaper           *workingpaper.Paper               `json:"working_paper,omitempty"`
	PageFill               *pagefill.Fill                    `json:"page_fill,omitempty"`
}

type AuditPackData struct {
	Basis       string   `json:"basis"`
	Scope       string   `json:"scope"`
	Answer      string   `json:"answer"`
	SourceCount int      `json:"source_count"`
	SourceIDs   []string `json:"source_ids,omitempty"`
}

type ReportExplanationData struct {
	Page      string            `json:"page"`
	Period    string            `json:"period,omitempty"`
	Basis     string            `json:"basis"`
	Filters   map[string]string `json:"filters,omitempty"`
	Answer    string            `json:"answer"`
	SourceIDs []string          `json:"source_ids,omitempty"`
}

type EventDraftData struct {
	ContractID         string          `json:"contract_id"`
	EventType          string          `json:"event_type"`
	EffectiveDate      string          `json:"effective_date"`
	OriginalValue      *string         `json:"original_value,omitempty"`
	NewValue           *string         `json:"new_value,omitempty"`
	ChangeReason       string          `json:"change_reason"`
	JudgmentBasis      string          `json:"judgment_basis,omitempty"`
	RevisionParameters json.RawMessage `json:"revision_parameters,omitempty"`
}

type AgentPlanStep struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"` // pending | running | completed | needs_review
}

type AgentToolCall struct {
	Tool           string `json:"tool"`
	Skill          string `json:"skill"`
	Status         string `json:"status"` // completed | failed | needs_review
	InputSummary   string `json:"input_summary"`
	OutputSummary  string `json:"output_summary"`
	RequiresReview bool   `json:"requires_review"`
	// DurationMs is the wall-clock time a tool execution took, set only for
	// calls that actually ran. Planned or LLM-suggested calls leave it absent —
	// a missing duration is never fabricated as zero (AGENTS.md: 不用 0 填补缺失).
	DurationMs *int64 `json:"duration_ms,omitempty"`
}

// LLMToolCall represents a tool call returned by the LLM (OpenAI/DeepSeek format).
type LLMToolCall struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Function map[string]interface{} `json:"function"`
}

// LLMChatResponse is the raw response from AI Service when using function calling.
type LLMChatResponse struct {
	Answer     string        `json:"answer"`
	Sources    []Source      `json:"sources"`
	Confidence float64       `json:"confidence"`
	Model      string        `json:"model"`
	ToolCalls  []LLMToolCall `json:"tool_calls,omitempty"`
}

type AgentReviewPrompt struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Description     string   `json:"description"`
	Severity        string   `json:"severity"` // info | warning | critical
	Action          string   `json:"action"`
	ContractNumbers []string `json:"contract_numbers,omitempty"`
}

type AgentRunbook struct {
	SkillID               string
	SkillVersion          string
	SkillName             string
	RequiresEvidenceInput bool
	NeedsPortfolioContext bool
	AnswerPrefix          string
	AgentPlan             []AgentPlanStep
	ToolCalls             []AgentToolCall
	ReviewPrompts         []AgentReviewPrompt
}

type ContractDraftItem = aiintake.ContractDraftData

type BatchParseSummary struct {
	TotalCount           int      `json:"total_count"`
	OverallConfidence    float64  `json:"overall_confidence"`
	RequiresHumanConfirm bool     `json:"requires_human_confirmation"`
	MissingFields        []string `json:"missing_fields"`
	Warnings             []string `json:"warnings"`
	SchemaVersion        string   `json:"schema_version"`
	IntakeID             string   `json:"intake_id"`
	EvidenceComplete     bool     `json:"evidence_complete"`
	ReviewReasons        []string `json:"review_reasons"`
}

type PaymentScheduleDraftItem struct {
	PeriodStart      string  `json:"period_start"`
	PeriodEnd        string  `json:"period_end"`
	DueDate          string  `json:"due_date"`
	Amount           float64 `json:"amount"`
	PaymentTiming    string  `json:"payment_timing"`
	IsFixed          bool    `json:"is_fixed"`
	IsLeaseComponent bool    `json:"is_lease_component"`
	AmountType       string  `json:"amount_type"`
	Currency         string  `json:"currency"`
	Confidence       float64 `json:"confidence"`
}

type PaymentScheduleParseSummary struct {
	TotalCount           int      `json:"total_count"`
	OverallConfidence    float64  `json:"overall_confidence"`
	RequiresHumanConfirm bool     `json:"requires_human_confirmation"`
	MissingFields        []string `json:"missing_fields"`
	Warnings             []string `json:"warnings"`
	CanImport            bool     `json:"can_import"`
	ContractID           string   `json:"contract_id,omitempty"`
	SchemaVersion        string   `json:"schema_version"`
	IntakeID             string   `json:"intake_id"`
	EvidenceComplete     bool     `json:"evidence_complete"`
	ReviewReasons        []string `json:"review_reasons"`
}

type Source struct {
	Type           string `json:"type"`
	ID             string `json:"id"`
	Title          string `json:"title"`
	Snippet        string `json:"snippet"`
	URL            string `json:"url,omitempty"`
	Classification string `json:"classification,omitempty"`
	DatasetVersion string `json:"dataset_version,omitempty"`
	AsOf           string `json:"as_of,omitempty"`
	FormulaVersion string `json:"formula_version,omitempty"`
}

func effectiveContractIDFromRequest(req Request) string {
	effectiveContractID := req.ContractID
	if effectiveContractID == "" && req.PageContext != nil && req.PageContext.ContractID != "" {
		effectiveContractID = req.PageContext.ContractID
	}
	return effectiveContractID
}

func (h *Agent) executeChatRequest(ctx context.Context, authHeader string, req Request, legalEntityID, userIDStr, roleStr string, emit func(context.Context, string, any) error, agentRunbook *AgentRunbook, toolRuntime *agenttools.Runtime) (Response, error) {
	if agentRunbook != nil && agentRunbook.SkillID == "retail_operations" {
		return h.executeRetailOperations(ctx, req, emit, toolRuntime)
	}
	// S1 pre-deal working paper: a message carrying a structured, human-
	// confirmed assumption block is routed deterministically to the paper
	// tool — no LLM guessing, no discount-rate invention.
	if s1Block, ok := extractS1Input(req.Message); ok {
		return h.executeS1Paper(ctx, req, s1Block, emit, toolRuntime)
	}
	// Retail operating working paper (the product main line): a 底稿 request
	// with valid retail filters routes to the engine-backed paper tool.
	if retailPaperRequested(req) {
		return h.executeRetailPaper(ctx, req, emit, toolRuntime)
	}
	var sources []Source
	var contextData strings.Builder
	var executedCalls []AgentToolCall
	effectiveContractID := effectiveContractIDFromRequest(req)

	// 1. Handle file upload if present — use Function Calling to let LLM decide which tool to invoke
	if req.FileID != "" && req.ObjectName != "" {
		fileSystemPrompt := fmt.Sprintf(`用户上传了一个文件，文件名: %s，MIME类型: %s。
请根据用户的意图和文件类型，决定调用哪个解析工具：
- parse_contract_batch: 当文件是合同台账（Excel/PDF包含多份合同）时使用
- parse_payment_schedule: 当文件是租金表/付款计划时使用
- parse_contract: 当文件是单份合同（PDF/Word/图片）时使用

用户消息: %s`, req.ObjectName, req.ContentType, req.Message)

		_, modelName, toolCalls, err := h.callLLMWithTools(ctx, authHeader, fileSystemPrompt, req.Message, req.History, req.Language, fileParseTools)
		triage := agenttooldefs.DeterministicTriage(agenttooldefs.TriageRequest{
			FileID:      req.FileID,
			ObjectName:  req.ObjectName,
			ContentType: req.ContentType,
			UserMessage: req.Message,
		})
		fallbackTool := triageToParseTool(triage)
		selectedTool := fallbackTool
		toolExecutionChain := []AgentToolCall{}
		contractID := effectiveContractID

		if err == nil && len(toolCalls) > 0 && toolCalls[0].Tool != "" {
			selectedTool = toolCalls[0].Tool
			toolExecutionChain = toolCalls
		} else if fallbackTool != "" {
			modelName = "deterministic-router"
		}

		if selectedTool == "" {
			// R3: no silent fallback. Unknown or unsupported files stop the
			// pipeline and ask the user — with one upgrade: operating data
			// files route to the import-page prefill seam (appendix A).
			if triage.DocClass == agenttooldefs.DocOperatingData {
				return h.executeRetailIngestFill(ctx, req, triage, emit, toolRuntime)
			}
			return fileTriageRefusal(req, triage), nil
		}

		if selectedTool != "" {
			if err := emitAgentEvent(ctx, emit, "tool_start", map[string]interface{}{
				"tool":        selectedTool,
				"status":      "running",
				"file_id":     req.FileID,
				"file_name":   req.ObjectName,
				"contract_id": contractID,
			}); err != nil {
				return Response{Answer: "AI agent event persistence failed", Model: "runtime"}, err
			}
		}

		switch selectedTool {
		case "parse_contract_batch":
			toolResult, durationMs, err := h.executeFileParseTool(ctx, toolRuntime, "lease.file.parse_contract_batch", fileParseArguments{
				FileID: req.FileID, ObjectName: req.ObjectName, ContentType: req.ContentType,
			})
			if err != nil {
				resp := Response{
					Answer:     fmt.Sprintf("文件解析失败: %s", err.Error()),
					Sources:    sources,
					Confidence: 0.5,
					IsOfficial: false,
					Model:      "fallback",
				}
				return resp, err
			}
			batchResult, ok := toolResult.Data.(*BatchParseResult)
			if !ok || batchResult == nil {
				return Response{Answer: "文件解析失败: 解析结果格式无效", Sources: sources, Confidence: 0.5, IsOfficial: false, Model: "fallback"}, fmt.Errorf("contract batch parse returned unexpected result")
			}
			// The LLM-selected call in the chain is the one that actually ran;
			// its duration belongs on it. Calls that never ran stay without one.
			if len(toolExecutionChain) > 0 {
				toolExecutionChain[0].DurationMs = durationMs
			}
			resp := Response{
				Answer:         batchResult.SummaryText,
				Sources:        batchResult.Sources,
				Confidence:     batchResult.Confidence,
				IsOfficial:     false,
				Model:          modelName,
				AgentMode:      true,
				AgentPlan:      batchResult.AgentPlan,
				ToolCalls:      append(toolExecutionChain, batchResult.ToolCalls...),
				ReviewPrompts:  batchResult.ReviewPrompts,
				DraftContracts: batchResult.Contracts,
				BatchSummary:   batchResult.Summary,
				EvidenceRefs:   batchResult.EvidenceRefs,
			}
			return resp, nil

		case "parse_payment_schedule":
			toolResult, durationMs, err := h.executeFileParseTool(ctx, toolRuntime, "lease.file.parse_payment_schedule", fileParseArguments{
				FileID: req.FileID, ObjectName: req.ObjectName, ContentType: req.ContentType, ContractID: contractID,
			})
			if err != nil {
				resp := Response{
					Answer:     fmt.Sprintf("租金表解析失败: %s", err.Error()),
					Sources:    sources,
					Confidence: 0.5,
					IsOfficial: false,
					Model:      "fallback",
				}
				return resp, err
			}
			scheduleResult, ok := toolResult.Data.(*PaymentScheduleParseResult)
			if !ok || scheduleResult == nil {
				return Response{Answer: "租金表解析失败: 解析结果格式无效", Sources: sources, Confidence: 0.5, IsOfficial: false, Model: "fallback"}, fmt.Errorf("payment schedule parse returned unexpected result")
			}
			if len(toolExecutionChain) > 0 {
				toolExecutionChain[0].DurationMs = durationMs
			}
			resp := Response{
				Answer:                 scheduleResult.SummaryText,
				Sources:                scheduleResult.Sources,
				Confidence:             scheduleResult.Confidence,
				IsOfficial:             false,
				Model:                  modelName,
				AgentMode:              true,
				AgentPlan:              scheduleResult.AgentPlan,
				ToolCalls:              append(toolExecutionChain, scheduleResult.ToolCalls...),
				ReviewPrompts:          scheduleResult.ReviewPrompts,
				DraftPaymentSchedules:  scheduleResult.Schedules,
				PaymentScheduleSummary: scheduleResult.Summary,
				EvidenceRefs:           scheduleResult.EvidenceRefs,
			}
			return resp, nil

		default:
			toolResult, _, err := h.executeFileParseTool(ctx, toolRuntime, "lease.file.parse_contract", fileParseArguments{
				FileID: req.FileID, ObjectName: req.ObjectName, ContentType: req.ContentType,
			})
			if err != nil {
				contextData.WriteString(fmt.Sprintf("\n## 文件解析失败\n错误: %s\n", err.Error()))
			} else {
				parsed, ok := toolResult.Data.(*aiintake.ContractDraft)
				if !ok || parsed == nil {
					contextData.WriteString("\n## 文件解析失败\n错误: 解析结果格式无效\n")
				} else {
					contextData.WriteString("\n## 上传文件解析结果\n")
					contractJSON, marshalErr := json.MarshalIndent(parsed.ExtractedData, "", "  ")
					if marshalErr == nil {
						contextData.Write(contractJSON)
						contextData.WriteString("\n")
					}
					sources = append(sources, Source{
						Type:    "file",
						ID:      parsed.Evidence.SourceFileID,
						Title:   "上传文件",
						Snippet: parsed.Evidence.ObjectName,
					})
				}
			}
		}

		// G1 card backfill source: what actually ran in the file branch
		// becomes the execution record the final response folds into the
		// runbook's display cards.
		executedCalls = toolExecutionChain
	}

	if agentRunbook != nil && agentRunbook.NeedsPortfolioContext {
		h.appendAgentPortfolioContext(ctx, toolRuntime, legalEntityID, &contextData, &sources)
	}

	// 2. Page-context-aware data retrieval
	// 2a. Contract detail context: always retrieve full contract data regardless of message keywords
	if effectiveContractID != "" {
		h.retrieveContractDetail(ctx, effectiveContractID, &contextData, &sources, toolRuntime)
	}

	// 2b. Reports page context
	if req.PageContext != nil && req.PageContext.Page == "reports" {
		h.appendReportsContext(req.PageContext, &contextData)
	}

	// 2c. Monthly closing page context
	if req.PageContext != nil && req.PageContext.Page == "monthly-closing" {
		h.appendMonthlyClosingContext(ctx, toolRuntime, req.PageContext, &contextData, &sources)
	}

	// 3. Keyword-based retrieval (backward compatible, works alongside page context)
	msgLower := strings.ToLower(req.Message)
	if h.shouldLoadPerformanceContext(msgLower, agentRunbook, req.PageContext) {
		h.appendPerformanceContext(ctx, toolRuntime, req.PageContext, &contextData, &sources)
	}

	// Contract list queries (only if no specific contract ID is resolved)
	if effectiveContractID == "" && containsAny(msgLower, []string{"合同", "租赁", "门店", "承租", "出租", "lease", "contract"}) {
		// P3-34: the contract list flows through the tool runtime — audited,
		// dimension-scoped — instead of a direct repository read.
		if result, ok := h.executeReadTool(ctx, toolRuntime, "lease.contract.search", agenttooldefs.ContractSearchArguments{}); ok {
			if data, dataOK := result.Data.(agenttooldefs.ContractSearchData); dataOK && len(data.Items) > 0 {
				contextData.WriteString(fmt.Sprintf("\n## 合同数据（共 %d 份）\n", len(data.Items)))
				for _, contract := range data.Items {
					contextData.WriteString(fmt.Sprintf("- ID: %s, 编号: %s, 名称: %s, 状态: %s, 承租方: %s, 出租方: %s\n",
						contract.ID, contract.ContractNumber, contract.ContractName,
						contract.ApprovalStatus, contract.LesseeName, contract.LessorName))
					sources = append(sources, Source{
						Type:    "contract",
						ID:      contract.ID,
						Title:   contract.ContractName,
						Snippet: fmt.Sprintf("编号: %s, 状态: %s", contract.ContractNumber, contract.ApprovalStatus),
					})
				}
			}
		}
	}

	// Measurement / liability / depreciation queries (only if contract detail context hasn't already loaded them)
	if effectiveContractID == "" && containsAny(msgLower, []string{"负债", "折旧", "利息", "摊销", "rou", "计量", "measurement", "depreciation", "liability"}) {
		if req.ContractID != "" {
			if result, ok := h.executeReadTool(ctx, toolRuntime, "lease.measurement.list", agenttooldefs.MeasurementListArguments{ContractID: req.ContractID}); ok {
				data, dataOK := result.Data.(agenttooldefs.MeasurementListData)
				if dataOK && len(data.Items) > 0 {
					contextData.WriteString(fmt.Sprintf("\n## 计量结果（合同 %s，共 %d 期）\n", req.ContractID, len(data.Items)))
					for _, r := range data.Items {
						contextData.WriteString(fmt.Sprintf("- 期间: %s, 期初负债: %.2f, 期末负债: %.2f, 利息: %.2f, 本金偿还: %.2f, 折旧: %.2f, 期末ROU: %.2f\n",
							r.AccountingPeriod, r.OpeningLiability, r.ClosingLiability, r.InterestExpense,
							r.PrincipalRepayment, r.Depreciation, r.ClosingROUAsset))
					}
					sources = append(sources, Source{
						Type:    "measurement",
						ID:      req.ContractID,
						Title:   "计量结果",
						Snippet: fmt.Sprintf("共 %d 期", len(data.Items)),
					})
				}
			}
		}
	}

	// Journal entry queries (only if contract detail context hasn't already loaded them)
	if effectiveContractID == "" && containsAny(msgLower, []string{"分录", "journal", "会计", "过账", "post", "voucher", "entry", "凭证"}) {
		if req.ContractID != "" {
			if result, ok := h.executeReadTool(ctx, toolRuntime, "lease.journal.list", agenttooldefs.JournalListArguments{ContractID: req.ContractID}); ok {
				data, dataOK := result.Data.(agenttooldefs.JournalListData)
				if dataOK && len(data.Items) > 0 {
					contextData.WriteString(fmt.Sprintf("\n## 会计分录（合同 %s，共 %d 条）\n", req.ContractID, len(data.Items)))
					for _, e := range data.Items {
						contextData.WriteString(fmt.Sprintf("- 期间: %s, 类型: %s, 借方: %s, 贷方: %s, 金额: %.2f %s, 状态: %s, 描述: %s\n",
							e.AccountingPeriod, e.EntryType, e.DebitAccount, e.CreditAccount,
							e.Amount, e.Currency, e.PostingStatus, e.Description))
					}
					sources = append(sources, Source{
						Type:    "journal",
						ID:      req.ContractID,
						Title:   "会计分录",
						Snippet: fmt.Sprintf("共 %d 条", len(data.Items)),
					})
				}
			}
		}
	}

	// Event queries (only if contract detail context hasn't already loaded them)
	if effectiveContractID == "" && containsAny(msgLower, []string{"事件", "变更", "modification", "reassessment", "impairment", "event", "change"}) {
		if req.ContractID != "" {
			if result, ok := h.executeReadTool(ctx, toolRuntime, "lease.event.list", agenttooldefs.EventListArguments{ContractID: req.ContractID}); ok {
				data, dataOK := result.Data.(agenttooldefs.EventListData)
				if dataOK && len(data.Items) > 0 {
					contextData.WriteString(fmt.Sprintf("\n## 事件数据（合同 %s，共 %d 条）\n", req.ContractID, len(data.Items)))
					for _, ev := range data.Items {
						contextData.WriteString(fmt.Sprintf("- 类型: %s, 生效日: %s, 状态: %s, 审批状态: %s, 原因: %s\n",
							ev.EventType, ev.EffectiveDate.Format("2006-01-02"), ev.Status, ev.ApprovalStatus, ev.ChangeReason))
					}
					sources = append(sources, Source{
						Type:    "event",
						ID:      req.ContractID,
						Title:   "变更事件",
						Snippet: fmt.Sprintf("共 %d 条", len(data.Items)),
					})
				}
			}
		}
	}

	// 4. If no context was built and no file was parsed, provide helpful guidance
	if contextData.Len() == 0 {
		contextData.WriteString(`
## 系统信息
当前系统暂无匹配的数据。建议用户：
- 指定具体的合同 ID 以查询计量结果和分录
- 询问合同列表（如"有哪些合同"）
- 上传合同文件或租金表进行 AI 解析
`)
	}

	if agentRunbook != nil && agentRunbook.RequiresEvidenceInput && req.FileID == "" && effectiveContractID == "" {
		resp := Response{
			Answer:        agentRunbook.AnswerPrefix,
			Sources:       sources,
			Confidence:    0.85,
			IsOfficial:    false,
			Model:         "lease-agent",
			AgentMode:     true,
			AgentPlan:     agentRunbook.AgentPlan,
			ToolCalls:     agentRunbook.ToolCalls,
			ReviewPrompts: agentRunbook.ReviewPrompts,
		}
		return resp, nil
	}

	// 5. Build system prompt
	isWorkingData := false
	if req.PageContext != nil && (req.PageContext.ReportView == "working" || req.PageContext.Page == "monthly-closing") {
		isWorkingData = true
	}
	systemPrompt := h.buildSystemPrompt(userIDStr, roleStr, legalEntityID, contextData.String(), isWorkingData, req.Language)

	// 6. Call AI Service
	if err := emitAgentEvent(ctx, emit, "tool_start", map[string]interface{}{
		"tool":   "llm.chat_completion",
		"status": "running",
		"model":  "auto",
	}); err != nil {
		return Response{Answer: "AI agent event persistence failed", Model: "runtime", Sources: sources}, err
	}
	answer, modelName, err := h.callLLM(ctx, authHeader, systemPrompt, req.Message, req.History, req.Language)
	if err != nil {
		// Fallback: return context data without LLM if AI Service is unavailable
		fallbackAnswer := fmt.Sprintf("（AI 服务暂不可用，以下为系统数据摘要）\n\n%s", contextData.String())
		if agentRunbook != nil {
			fallbackAnswer = agentRunbook.AnswerPrefix + "\n\n" + fallbackAnswer
		}
		plan, calls := foldExecutedIntoRunbook(agentRunbook, executedCalls)
		resp := Response{
			Answer:        fallbackAnswer,
			Sources:       sources,
			Confidence:    0.5,
			IsOfficial:    false,
			Model:         "fallback",
			AgentMode:     agentRunbook != nil,
			AgentPlan:     plan,
			ToolCalls:     calls,
			ReviewPrompts: reviewPromptsFromRunbook(agentRunbook),
		}
		if agentRunbook != nil && agentRunbook.SkillID == "event_change" {
			resp.EventDraft, resp.EvidenceRefs = extractEventDraft(req.Message, effectiveContractID, sources)
		}
		return resp, err
	}
	if err := emitAgentEvent(ctx, emit, "tool_end", []map[string]interface{}{
		{
			"tool":   "llm.chat_completion",
			"status": "completed",
			"model":  modelName,
		},
	}); err != nil {
		return Response{Answer: answer, Model: modelName, Sources: sources}, err
	}

	// 7. Extract sources from answer and merge with context sources.
	// If no source citations found in the answer, fall back to all known sources.
	extractedSources := extractSourcesFromAnswer(answer, sources)

	if agentRunbook != nil && agentRunbook.AnswerPrefix != "" {
		answer = agentRunbook.AnswerPrefix + "\n\n" + answer
	}

	plan, calls := foldExecutedIntoRunbook(agentRunbook, executedCalls)
	resp := Response{
		Answer:        answer,
		Sources:       extractedSources,
		Confidence:    0.9,
		IsOfficial:    false,
		Model:         modelName,
		AgentMode:     agentRunbook != nil,
		AgentPlan:     plan,
		ToolCalls:     calls,
		ReviewPrompts: reviewPromptsFromRunbook(agentRunbook),
	}
	if req.PageContext != nil && strings.EqualFold(strings.TrimSpace(req.PageContext.Page), "reports") {
		basis := strings.ToLower(strings.TrimSpace(req.PageContext.ReportView))
		if basis == "" {
			basis = "working"
		}
		sourceIDs := make([]string, 0, len(extractedSources))
		for _, source := range extractedSources {
			if strings.TrimSpace(source.ID) != "" {
				sourceIDs = append(sourceIDs, source.ID)
			}
		}
		resp.EvidenceRefs = evidenceReferencesFromSources(extractedSources)
		resp.ReportExplanation = &ReportExplanationData{
			Page: req.PageContext.Page, Period: req.PageContext.Period, Basis: basis,
			Filters: req.PageContext.Filters, Answer: answer, SourceIDs: sourceIDs,
		}
	}
	if agentRunbook != nil && agentRunbook.SkillID == "audit_pack" {
		basis := "working"
		if req.PageContext != nil && strings.EqualFold(strings.TrimSpace(req.PageContext.ReportView), "official") {
			basis = "official"
		}
		resp.EvidenceRefs = evidenceReferencesFromSources(extractedSources)
		sourceIDs := make([]string, 0, len(extractedSources))
		for _, source := range extractedSources {
			if strings.TrimSpace(source.ID) != "" {
				sourceIDs = append(sourceIDs, source.ID)
			}
		}
		resp.AuditPack = &AuditPackData{
			Basis: basis, Scope: legalEntityID, Answer: answer,
			SourceCount: len(extractedSources), SourceIDs: sourceIDs,
		}
	}
	if agentRunbook != nil && agentRunbook.SkillID == "event_change" {
		resp.EventDraft, resp.EvidenceRefs = extractEventDraft(req.Message, effectiveContractID, extractedSources)
	}
	return resp, nil
}

func (h *Agent) shouldLoadPerformanceContext(message string, runbook *AgentRunbook, page *PageContext) bool {
	if runbook != nil && (runbook.SkillID == "fpna_copilot" || runbook.SkillID == "retail_performance" || runbook.SkillID == "manufacturing_performance") {
		return true
	}
	if page != nil && (page.Page == "performance" || page.Page == "portfolio") {
		return true
	}
	return containsAny(message, []string{"经营", "预算", "forecast", "行动", "异常", "四墙", "租售比", "坪效", "门店表现", "工厂", "产线", "设备", "oee", "fp&a", "finance bp"})
}

func (h *Agent) appendPerformanceContext(ctx context.Context, toolRuntime *agenttools.Runtime, page *PageContext, contextData *strings.Builder, sources *[]Source) {
	period := time.Now().UTC().Format("2006-01")
	if page != nil && strings.TrimSpace(page.Period) != "" {
		period = strings.TrimSpace(page.Period)
	}
	if result, ok := h.executeReadTool(ctx, toolRuntime, "lease.portfolio.summary", agenttooldefs.PerformanceArguments{Period: period}); ok {
		encoded, _ := json.Marshal(result.Data)
		contextData.WriteString("\n## 经营组合摘要（系统正式事实，Working）\n")
		contextData.Write(encoded)
		contextData.WriteString("\n")
		*sources = append(*sources, Source{Type: "performance_overview", ID: period, Title: "经营组合摘要", Snippet: "期间=" + period})
	}
	if result, ok := h.executeReadTool(ctx, toolRuntime, "lease.store.performance", agenttooldefs.PerformanceArguments{Period: period}); ok {
		encoded, _ := json.Marshal(result.Data)
		contextData.WriteString("\n## 门店四墙表现（系统正式事实，Working）\n")
		contextData.Write(encoded)
		contextData.WriteString("\n")
		for _, source := range result.Sources {
			*sources = append(*sources, Source{Type: source.Type, ID: source.ID, Title: source.Title, Snippet: source.Locator})
		}
	}
	if result, ok := h.executeReadTool(ctx, toolRuntime, "lease.equipment.performance", agenttooldefs.PerformanceArguments{Period: period}); ok {
		encoded, _ := json.Marshal(result.Data)
		contextData.WriteString("\n## 制造设备表现（系统正式事实，Working）\n")
		contextData.Write(encoded)
		contextData.WriteString("\n")
		for _, source := range result.Sources {
			*sources = append(*sources, Source{Type: source.Type, ID: source.ID, Title: source.Title, Snippet: source.Locator})
		}
	}
	if result, ok := h.executeReadTool(ctx, toolRuntime, "lease.fpna.actions", agenttooldefs.PerformanceArguments{Period: period}); ok {
		encoded, _ := json.Marshal(result.Data)
		contextData.WriteString("\n## 经营行动与异常（系统正式事实，待人工处理）\n")
		contextData.Write(encoded)
		contextData.WriteString("\n")
		for _, source := range result.Sources {
			*sources = append(*sources, Source{Type: source.Type, ID: source.ID, Title: source.Title, Snippet: source.Locator})
		}
	}
	if result, ok := h.executeReadTool(ctx, toolRuntime, "lease.close.readiness", agenttooldefs.PerformanceArguments{Period: period}); ok {
		encoded, _ := json.Marshal(result.Data)
		contextData.WriteString("\n## 关账准备度与证据缺口（系统正式事实，Working）\n")
		contextData.Write(encoded)
		contextData.WriteString("\n")
		for _, source := range result.Sources {
			*sources = append(*sources, Source{Type: source.Type, ID: source.ID, Title: source.Title, Snippet: source.Locator})
		}
	}
	if page != nil && page.Filters != nil {
		versionID := strings.TrimSpace(page.Filters["version_id"])
		if versionID == "" {
			versionID = strings.TrimSpace(page.Filters["budget_version_id"])
		}
		if versionID != "" {
			if result, ok := h.executeReadTool(ctx, toolRuntime, "lease.budget.variance", agenttooldefs.BudgetVarianceArguments{VersionID: versionID, Period: period}); ok {
				encoded, _ := json.Marshal(result.Data)
				contextData.WriteString("\n## 预算差异桥（系统确定性服务，Working）\n")
				contextData.Write(encoded)
				contextData.WriteString("\n")
				for _, source := range result.Sources {
					*sources = append(*sources, Source{Type: source.Type, ID: source.ID, Title: source.Title, Snippet: source.Locator})
				}
			}
		}
	}
}

func evidenceReferencesFromSources(sources []Source) []agentartifact.EvidenceReference {
	refs := make([]agentartifact.EvidenceReference, 0, len(sources))
	for _, source := range sources {
		id := strings.TrimSpace(source.ID)
		if id == "" {
			continue
		}
		refs = append(refs, agentartifact.EvidenceReference{
			ReferenceID: id, Complete: true,
			Locators: []agentartifact.EvidenceLocator{{
				Field: "record", Source: "system:" + strings.TrimSpace(source.Type), Quote: strings.TrimSpace(source.Snippet),
			}},
		})
	}
	return refs
}

// workingPaperReviewReasons lists the review topics for a generated paper,
// naming the honesty fields the reviewer must check.
func workingPaperReviewReasons(paper workingpaper.Paper) []string {
	reasons := []string{"s1_paper_review", "assumptions_human_confirmed"}
	if len(paper.DataGaps) > 0 {
		reasons = append(reasons, "data_gaps_acknowledged")
	}
	return reasons
}

func emitAgentEvent(ctx context.Context, emit func(context.Context, string, any) error, eventType string, payload any) error {
	if emit == nil {
		return nil
	}
	return emit(ctx, eventType, payload)
}

func (h *Agent) buildAgentRunbook(req Request, effectiveContractID string) *AgentRunbook {
	hasFile := req.FileID != "" && req.ObjectName != ""
	hasContractContext := effectiveContractID != ""
	if h == nil {
		return nil
	}
	registry := h.skillRegistry
	if registry == nil {
		registry = agentskill.ProductionRegistry()
	}
	definition, ok := registry.Select(agentskill.Intent{
		Message: req.Message, HasFile: hasFile, HasContract: hasContractContext,
	})
	if !ok {
		return nil
	}
	runbook := runbookForSkillID(definition.ID, req, effectiveContractID)
	if runbook != nil {
		runbook.SkillVersion = definition.Version
	}
	return runbook
}

func buildContractLedgerRunbook(hasFile bool) *AgentRunbook {
	evidenceStatus := "needs_review"
	parserStatus := "pending"
	prefix := "Agent 已准备执行 Excel 台账导入技能。请上传合同台账文件后，我会先用 Office skill 展开工作簿，再由 AI Agent 进行语义理解并生成合同草稿；不会直接写入正式台账。"
	reviewPrompts := []AgentReviewPrompt{
		{ID: "upload_ledger", Title: "上传合同台账", Description: "需要 Excel、CSV 或可解析的台账文件作为证据输入。", Severity: "info", Action: "上传文件后再次发送导入指令。"},
	}
	if hasFile {
		evidenceStatus = "completed"
		parserStatus = "needs_review"
		prefix = "Agent 正在按 Excel 台账导入技能处理上传文件。"
		reviewPrompts = []AgentReviewPrompt{
			{ID: "ledger_review", Title: "复核台账草稿", Description: "上传文件已作为证据输入。下一步需要核对草稿字段、折现率和租赁范围判断。", Severity: "warning", Action: "确认字段无误后再创建 draft 合同。"},
		}
	}
	return &AgentRunbook{
		SkillID:               "excel_ledger",
		SkillName:             "Office Excel Ledger Skill",
		RequiresEvidenceInput: !hasFile,
		AnswerPrefix:          prefix,
		AgentPlan: []AgentPlanStep{
			{ID: "read_workbook", Title: "读取并展开 Office 工作簿", Status: evidenceStatus},
			{ID: "understand_columns", Title: "理解非标准字段、表头和合并单元格", Status: parserStatus},
			{ID: "human_review", Title: "人工确认缺失字段、折现率和租赁范围", Status: "needs_review"},
			{ID: "create_drafts", Title: "确认后创建 draft 合同", Status: "pending"},
		},
		ToolCalls: []AgentToolCall{
			{Tool: "office.excel_reader", Skill: "Office Excel Skill", Status: evidenceStatus, InputSummary: "等待或读取上传的 Excel 台账", OutputSummary: "将工作簿展开为带 sheet、行列坐标和单元格值的证据文本", RequiresReview: !hasFile},
			{Tool: "lease.contract_batch_parser", Skill: "Lease Intake Skill", Status: parserStatus, InputSummary: "从非标准台账中抽取合同、门店、出租方、日期、租金和范围判断", OutputSummary: "生成合同草稿和字段级复核清单", RequiresReview: true},
			{Tool: "lease.draft_contract_creator", Skill: "Core Service Draft Skill", Status: "pending", InputSummary: "等待用户确认草稿", OutputSummary: "只创建 draft 合同，正式入库仍走审批", RequiresReview: true},
		},
		ReviewPrompts: reviewPrompts,
	}
}

func buildContractReviewRunbook(hasFile, hasContractContext bool) *AgentRunbook {
	hasEvidence := hasFile || hasContractContext
	evidenceStatus := statusFromReview(!hasEvidence)
	reviewPrompts := []AgentReviewPrompt{
		{ID: "contract_evidence", Title: "提供合同证据", Description: "合同复核需要上传合同文件，或在合同详情页指定当前合同。", Severity: "info", Action: "上传 PDF/Word/Excel 合同文件，或进入某份合同详情后提问。"},
	}
	if hasEvidence {
		reviewPrompts = []AgentReviewPrompt{
			{ID: "contract_judgement", Title: "复核关键会计判断", Description: "已具备合同证据。请重点确认续租/终止选择权、非租赁成分、折现率来源和租赁范围。", Severity: "warning", Action: "根据 Agent 提取结果逐项确认判断依据。"},
		}
	}
	return &AgentRunbook{
		SkillID:               "contract_review",
		SkillName:             "Office Contract Review Skill",
		RequiresEvidenceInput: !hasEvidence,
		AnswerPrefix:          "Agent 已切换到合同复核技能。我会提取关键条款、识别 IFRS 16 风险点，并把需要人工判断的事项列为复核提示。",
		AgentPlan: []AgentPlanStep{
			{ID: "collect_evidence", Title: "读取合同文件或当前合同上下文", Status: evidenceStatus},
			{ID: "extract_terms", Title: "抽取租赁期限、付款、选择权和非租赁成分", Status: statusFromReview(!hasEvidence)},
			{ID: "accounting_review", Title: "生成 IFRS 16 复核事项", Status: "needs_review"},
			{ID: "prepare_notes", Title: "形成可追溯复核结论", Status: "pending"},
		},
		ToolCalls: []AgentToolCall{
			{Tool: "office.document_reader", Skill: "Office/PDF Document Skill", Status: evidenceStatus, InputSummary: "读取合同文件或当前合同详情", OutputSummary: "提取条款证据文本和可引用来源", RequiresReview: !hasEvidence},
			{Tool: "lease.term_reviewer", Skill: "Lease Review Skill", Status: statusFromReview(!hasEvidence), InputSummary: "识别期限、付款、续租/终止选择权、CAM/服务费、折现率线索", OutputSummary: "输出字段级风险和人工复核点", RequiresReview: true},
		},
		ReviewPrompts: reviewPrompts,
	}
}

func buildPaymentScheduleRunbook(hasFile, hasContractContext bool) *AgentRunbook {
	hasEvidence := hasFile || hasContractContext
	evidenceStatus := statusFromReview(!hasEvidence)
	reviewPrompts := []AgentReviewPrompt{
		{ID: "schedule_evidence", Title: "提供租金表证据", Description: "付款计划导入需要租金表、合同付款页或当前合同上下文。", Severity: "info", Action: "上传 Excel/PDF 租金表，或进入合同详情后提问。"},
	}
	if hasEvidence {
		reviewPrompts = []AgentReviewPrompt{
			{ID: "schedule_review", Title: "复核付款计划判断", Description: "已具备付款证据。请重点确认先付/后付、变量租金、非租赁成分和缺失期间。", Severity: "warning", Action: "确认后再导入付款计划草稿。"},
		}
	}
	return &AgentRunbook{
		SkillID:               "payment_schedule",
		SkillName:             "Office Rent Schedule Skill",
		RequiresEvidenceInput: !hasEvidence,
		AnswerPrefix:          "Agent 已切换到租金表技能。我会先理解付款表结构，再区分先付/后付、固定租金、变量租金和非租赁成分。",
		AgentPlan: []AgentPlanStep{
			{ID: "read_schedule", Title: "读取租金表或合同付款上下文", Status: evidenceStatus},
			{ID: "classify_payments", Title: "分类付款时点和会计属性", Status: statusFromReview(!hasEvidence)},
			{ID: "human_review", Title: "人工确认异常金额、缺失期间和变量租金", Status: "needs_review"},
			{ID: "import_schedule", Title: "确认后导入付款计划草稿", Status: "pending"},
		},
		ToolCalls: []AgentToolCall{
			{Tool: "office.sheet_reader", Skill: "Office Excel Skill", Status: evidenceStatus, InputSummary: "读取上传的租金表或合同付款上下文", OutputSummary: "展开期间、金额、币种、税额和备注字段", RequiresReview: !hasEvidence},
			{Tool: "lease.payment_schedule_parser", Skill: "Payment Schedule Skill", Status: statusFromReview(!hasEvidence), InputSummary: "抽取付款计划并判断资本化属性", OutputSummary: "生成付款计划草稿和异常清单", RequiresReview: true},
		},
		ReviewPrompts: reviewPrompts,
	}
}

func buildEventChangeRunbook(hasContractContext bool) *AgentRunbook {
	evidenceStatus := "needs_review"
	prompt := AgentReviewPrompt{ID: "event_contract_context", Title: "指定事件所属合同", Description: "事件草稿必须绑定到当前权限范围内的合同。", Severity: "info", Action: "从合同详情页发起，或提供合同上下文后再登记事件。"}
	if hasContractContext {
		evidenceStatus = "completed"
		prompt = AgentReviewPrompt{ID: "event_review", Title: "复核事件和会计处理", Description: "事件会影响后续重算。请确认事件类型、生效日期、变更原因、判断依据及原文证据。", Severity: "critical", Action: "确认事件草稿后再提交复核，不会自动触发重算或过账。"}
	}
	return &AgentRunbook{
		SkillID: "event_change", SkillName: "Lease Event Change Skill",
		RequiresEvidenceInput: !hasContractContext,
		AnswerPrefix:          "Agent 已切换到合同事件变更技能。我会先整理事件证据和影响范围，再生成待复核事件草稿；批准前不会触发重算或过账。",
		AgentPlan: []AgentPlanStep{
			{ID: "bind_contract", Title: "绑定合同和数据范围", Status: evidenceStatus},
			{ID: "classify_event", Title: "区分 modification、reassessment、减值或其他事件", Status: "needs_review"},
			{ID: "collect_evidence", Title: "记录生效日期、变更原因和判断依据", Status: "needs_review"},
			{ID: "create_event_draft", Title: "确认后创建事件草稿", Status: "pending"},
		},
		ToolCalls:     []AgentToolCall{{Tool: "lease.event.draft.create", Skill: "event_change", Status: "pending", InputSummary: "等待结构化事件字段和原文证据确认", OutputSummary: "仅创建 draft 事件，后续走复核/审批/重算流程", RequiresReview: true}},
		ReviewPrompts: []AgentReviewPrompt{prompt},
	}
}

func buildAuditPackRunbook() *AgentRunbook {
	return &AgentRunbook{
		SkillID:               "audit_pack",
		SkillName:             "Lease Audit Pack Skill",
		NeedsPortfolioContext: true,
		AnswerPrefix:          "Agent 已切换到审计包技能。我会基于权限范围内的系统数据生成审计资料清单、抽样建议和数据质量提示；正式审计结论仍需人工复核。",
		AgentPlan: []AgentPlanStep{
			{ID: "define_scope", Title: "确认审计期间、法人和报表口径", Status: "needs_review"},
			{ID: "collect_system_data", Title: "检索合同、计量、分录和审批状态", Status: "completed"},
			{ID: "quality_checks", Title: "识别缺失字段、未审批和口径差异", Status: "needs_review"},
			{ID: "prepare_pack", Title: "生成审计包目录和导出建议", Status: "pending"},
		},
		ToolCalls: []AgentToolCall{
			{Tool: "core.contract_repository", Skill: "Lease Data Retrieval Skill", Status: "completed", InputSummary: "读取当前用户权限范围内的合同组合", OutputSummary: "形成审计包覆盖范围和抽样基础", RequiresReview: false},
			{Tool: "lease.audit_pack_builder", Skill: "Audit Pack Skill", Status: "needs_review", InputSummary: "整理合同清单、计量结果、分录、审批和附件需求", OutputSummary: "等待用户确认审计期间和 Official/Working 口径", RequiresReview: true},
		},
		ReviewPrompts: []AgentReviewPrompt{
			{ID: "audit_scope", Title: "确认审计范围", Description: "生成审计包前需要确认法人、期间、Official/Working 口径和抽样规则。", Severity: "warning", Action: "指定期间和口径，例如：生成 2024-01 至 2024-12 Official 审计包。"},
		},
	}
}

// P3-34: the portfolio read flows through the audited, scope-narrowing tool
// runtime instead of a direct repository query.
func (h *Agent) appendAgentPortfolioContext(ctx context.Context, toolRuntime *agenttools.Runtime, legalEntityID string, contextData *strings.Builder, sources *[]Source) {
	result, ok := h.executeReadTool(ctx, toolRuntime, "lease.contract.search", agenttooldefs.ContractSearchArguments{})
	if !ok {
		// The refusal keeps its nature: a denied read is never softened into
		// "no data" (AGENTS.md red line).
		contextData.WriteString("\n## Agent 组合数据\n组合数据读取失败或当前身份无权访问，无法提供组合数据。\n")
		return
	}
	data, dataOK := result.Data.(agenttooldefs.ContractSearchData)
	if !dataOK || len(data.Items) == 0 {
		contextData.WriteString("\n## Agent 组合数据\n当前权限范围内未检索到合同组合数据。\n")
		return
	}
	contracts := data.Items

	statusCounts := make(map[string]int)
	showCount := len(contracts)
	if showCount > 20 {
		showCount = 20
	}
	for _, contract := range contracts {
		statusCounts[contract.ApprovalStatus]++
	}

	contextData.WriteString(fmt.Sprintf("\n## Agent 组合数据（共 %d 份合同，显示前 %d 份）\n", len(contracts), showCount))
	contextData.WriteString("- 审批状态分布:")
	for status, count := range statusCounts {
		contextData.WriteString(fmt.Sprintf(" %s=%d", status, count))
	}
	contextData.WriteString("\n")
	for i, contract := range contracts {
		if i >= showCount {
			break
		}
		contextData.WriteString(fmt.Sprintf("- ID: %s, 编号: %s, 名称: %s, 审批状态: %s, 承租方: %s, 出租方: %s, 门店: %s\n",
			contract.ID, contract.ContractNumber, contract.ContractName, contract.ApprovalStatus, contract.LesseeName, contract.LessorName, contract.StoreName))
		*sources = append(*sources, Source{
			Type:    "contract",
			ID:      contract.ID,
			Title:   contract.ContractName,
			Snippet: fmt.Sprintf("编号: %s, 状态: %s", contract.ContractNumber, contract.ApprovalStatus),
		})
	}
}

func agentPlanFromRunbook(runbook *AgentRunbook) []AgentPlanStep {
	if runbook == nil {
		return nil
	}
	return runbook.AgentPlan
}

func toolCallsFromRunbook(runbook *AgentRunbook) []AgentToolCall {
	if runbook == nil {
		return nil
	}
	return runbook.ToolCalls
}

// foldExecutedIntoRunbook is G1's card backfill: the runbook's display cards
// become a record of what actually ran. Pending cards whose tool executed
// are replaced with the real call (status + duration); every plan step that
// had work executed turns completed, or failed when a fold-in call failed.
// Cards without execution stay pending — they genuinely did not run, and the
// UI must keep showing that instead of pretending.
func foldExecutedIntoRunbook(runbook *AgentRunbook, executed []AgentToolCall) ([]AgentPlanStep, []AgentToolCall) {
	if runbook == nil {
		return nil, executed
	}
	if len(executed) == 0 {
		return agentPlanFromRunbook(runbook), toolCallsFromRunbook(runbook)
	}
	used := make([]bool, len(executed))
	calls := append([]AgentToolCall(nil), runbook.ToolCalls...)
	anyFailed := false
	anyDone := false
	for i := range calls {
		for j := range executed {
			if used[j] || strings.TrimSpace(executed[j].Tool) != strings.TrimSpace(calls[i].Tool) {
				continue
			}
			calls[i] = executed[j]
			used[j] = true
			anyDone = true
			if executed[j].Status == "failed" {
				anyFailed = true
			}
			break
		}
	}
	plan := append([]AgentPlanStep(nil), runbook.AgentPlan...)
	for i := range plan {
		if !anyDone {
			continue
		}
		plan[i].Status = "completed"
		if anyFailed {
			plan[i].Status = "failed"
		}
	}
	return plan, calls
}

func reviewPromptsFromRunbook(runbook *AgentRunbook) []AgentReviewPrompt {
	if runbook == nil {
		return nil
	}
	return runbook.ReviewPrompts
}

// retrieveContractDetail fetches and appends contract basic info, latest measurements,
// events, and latest journal entries for a given contract ID.
func (h *Agent) retrieveContractDetail(ctx context.Context, contractID string, contextData *strings.Builder, sources *[]Source, toolRuntime *agenttools.Runtime) {
	result, ok := h.executeReadTool(ctx, toolRuntime, "lease.contract.get", agenttooldefs.ContractGetArguments{ContractID: contractID})
	if !ok {
		return
	}
	contract, ok := result.Data.(agenttooldefs.ContractView)
	if ok {
		contextData.WriteString(fmt.Sprintf("\n## 合同详情\n- ID: %s\n- 编号: %s\n- 名称: %s\n- 状态: %s\n- 审批状态: %s\n- 承租方: %s\n- 出租方: %s\n- 门店: %s\n- 币种: %s\n",
			contract.ID, contract.ContractNumber, contract.ContractName,
			contract.ApprovalStatus, contract.Status, contract.LesseeName,
			contract.LessorName, contract.StoreName, contract.Currency))
		if contract.CommencementDate.Year() > 1 {
			contextData.WriteString(fmt.Sprintf("- 租赁开始日: %s\n", contract.CommencementDate.Format("2006-01-02")))
		}
		if contract.LeaseEndDate.Year() > 1 {
			contextData.WriteString(fmt.Sprintf("- 租赁结束日: %s\n", contract.LeaseEndDate.Format("2006-01-02")))
		}
		if contract.DiscountRateValue != nil {
			contextData.WriteString(fmt.Sprintf("- 折现率: %.4f%%\n", *contract.DiscountRateValue*100))
		}
		if contract.DiscountRateMissing {
			contextData.WriteString("- ⚠️ 折现率缺失，需要人工补充\n")
		}
		*sources = append(*sources, Source{
			Type:    "contract",
			ID:      contract.ID,
			Title:   contract.ContractName,
			Snippet: fmt.Sprintf("编号: %s, 状态: %s", contract.ContractNumber, contract.ApprovalStatus),
		})
	}

	// Latest measurement results
	if result, ok := h.executeReadTool(ctx, toolRuntime, "lease.measurement.list", agenttooldefs.MeasurementListArguments{ContractID: contractID}); ok {
		data, dataOK := result.Data.(agenttooldefs.MeasurementListData)
		if dataOK && len(data.Items) > 0 {
			contextData.WriteString(fmt.Sprintf("\n## 计量结果（合同 %s，共 %d 期）\n", contractID, len(data.Items)))
			for _, r := range data.Items {
				contextData.WriteString(fmt.Sprintf("- 期间: %s, 期初负债: %.2f, 期末负债: %.2f, 利息: %.2f, 本金偿还: %.2f, 折旧: %.2f, 期末ROU: %.2f\n",
					r.AccountingPeriod, r.OpeningLiability, r.ClosingLiability, r.InterestExpense,
					r.PrincipalRepayment, r.Depreciation, r.ClosingROUAsset))
			}
			*sources = append(*sources, Source{
				Type:    "measurement",
				ID:      contractID,
				Title:   "计量结果",
				Snippet: fmt.Sprintf("合同 %s 共 %d 期", contractID, len(data.Items)),
			})
		}
	}

	// Events for this contract
	if result, ok := h.executeReadTool(ctx, toolRuntime, "lease.event.list", agenttooldefs.EventListArguments{ContractID: contractID}); ok {
		data, dataOK := result.Data.(agenttooldefs.EventListData)
		if dataOK && len(data.Items) > 0 {
			contextData.WriteString(fmt.Sprintf("\n## 变更事件（合同 %s，共 %d 条）\n", contractID, len(data.Items)))
			for _, ev := range data.Items {
				contextData.WriteString(fmt.Sprintf("- 类型: %s, 生效日: %s, 状态: %s, 审批状态: %s, 原因: %s\n",
					ev.EventType, ev.EffectiveDate.Format("2006-01-02"), ev.Status, ev.ApprovalStatus, ev.ChangeReason))
			}
			*sources = append(*sources, Source{
				Type:    "event",
				ID:      contractID,
				Title:   "变更事件",
				Snippet: fmt.Sprintf("合同 %s 共 %d 条", contractID, len(data.Items)),
			})
		}
	}

	// Latest journal entries for this contract
	if result, ok := h.executeReadTool(ctx, toolRuntime, "lease.journal.list", agenttooldefs.JournalListArguments{ContractID: contractID}); ok {
		data, dataOK := result.Data.(agenttooldefs.JournalListData)
		if dataOK && len(data.Items) > 0 {
			contextData.WriteString(fmt.Sprintf("\n## 会计分录（合同 %s，共 %d 条）\n", contractID, len(data.Items)))
			for _, e := range data.Items {
				contextData.WriteString(fmt.Sprintf("- 期间: %s, 类型: %s, 借方: %s, 贷方: %s, 金额: %.2f %s, 状态: %s, 描述: %s\n",
					e.AccountingPeriod, e.EntryType, e.DebitAccount, e.CreditAccount,
					e.Amount, e.Currency, e.PostingStatus, e.Description))
			}
			*sources = append(*sources, Source{
				Type:    "journal",
				ID:      contractID,
				Title:   "会计分录",
				Snippet: fmt.Sprintf("合同 %s 共 %d 条", contractID, len(data.Items)),
			})
		}
	}
}

func (h *Agent) executeReadTool(ctx context.Context, toolRuntime *agenttools.Runtime, toolName string, arguments any) (agenttools.ToolResult, bool) {
	result, _, err := h.executeToolCall(ctx, toolRuntime, toolName, arguments, "")
	if err != nil || result.Status != agenttools.StatusCompleted || result.Error != nil {
		return agenttools.ToolResult{}, false
	}
	return result, true
}

func (h *Agent) executeToolCall(ctx context.Context, toolRuntime *agenttools.Runtime, toolName string, arguments any, idempotencyKey string) (agenttools.ToolResult, *int64, error) {
	if toolRuntime == nil {
		return agenttools.ToolResult{}, nil, fmt.Errorf("tool runtime is unavailable")
	}
	execution, err := agenttools.RequireExecutionContext(ctx)
	if err != nil {
		return agenttools.ToolResult{}, nil, err
	}
	encoded, err := json.Marshal(arguments)
	if err != nil {
		return agenttools.ToolResult{}, nil, err
	}
	startedAt := time.Now()
	result, err := toolRuntime.Execute(ctx, agenttools.ToolCall{
		CallID:         toolName + "-" + execution.RunID,
		RunID:          execution.RunID,
		TraceID:        execution.TraceID,
		ToolName:       toolName,
		ToolVersion:    "v1",
		Arguments:      encoded,
		IdempotencyKey: idempotencyKey,
	})
	durationMs := time.Since(startedAt).Milliseconds()
	return result, &durationMs, err
}

// appendReportsContext appends report page context info from the frontend payload.
// It does not query the reports endpoint internally; it uses the provided context only.
func (h *Agent) appendReportsContext(pc *PageContext, contextData *strings.Builder) {
	contextData.WriteString("\n## 当前报表上下文\n")
	if pc.Title != "" {
		contextData.WriteString(fmt.Sprintf("- 报表标题: %s\n", pc.Title))
	}
	if pc.ReportView != "" {
		contextData.WriteString(fmt.Sprintf("- 报表口径: %s\n", pc.ReportView))
	}
	if pc.Period != "" {
		contextData.WriteString(fmt.Sprintf("- 覆盖期间: %s\n", pc.Period))
	}
	if len(pc.Filters) > 0 {
		contextData.WriteString("- 筛选条件:\n")
		for k, v := range pc.Filters {
			contextData.WriteString(fmt.Sprintf("  - %s: %s\n", k, v))
		}
	}
	if pc.Summary != "" {
		contextData.WriteString(fmt.Sprintf("- 摘要: %s\n", pc.Summary))
	}
}

// appendMonthlyClosingContext appends monthly closing page context and queries
// journal entries for the selected period if provided.
func (h *Agent) appendMonthlyClosingContext(ctx context.Context, toolRuntime *agenttools.Runtime, pc *PageContext, contextData *strings.Builder, sources *[]Source) {
	contextData.WriteString("\n## 当前结账中心上下文\n")
	if pc.Period != "" {
		contextData.WriteString(fmt.Sprintf("- 选定期间: %s\n", pc.Period))
	}
	if pc.Summary != "" {
		contextData.WriteString(fmt.Sprintf("- 摘要: %s\n", pc.Summary))
	}

	if pc.Period != "" {
		// P3-34: journal entries flow through the audited tool runtime.
		if result, ok := h.executeReadTool(ctx, toolRuntime, "lease.journal.list", agenttooldefs.JournalListArguments{Period: pc.Period}); ok {
			if data, dataOK := result.Data.(agenttooldefs.JournalListData); dataOK && len(data.Items) > 0 {
				entries := data.Items
				showCount := len(entries)
				if showCount > 20 {
					showCount = 20
				}
				postedCount := 0
				for _, e := range entries {
					if e.PostingStatus == "posted" {
						postedCount++
					}
				}
				contextData.WriteString(fmt.Sprintf("\n### 期间 %s 的会计分录（共 %d 条，显示前 %d 条）\n", pc.Period, len(entries), showCount))
				for i, e := range entries {
					if i >= showCount {
						break
					}
					contextData.WriteString(fmt.Sprintf("- 类型: %s, 借方: %s, 贷方: %s, 金额: %.2f %s, 状态: %s, 描述: %s\n",
						e.EntryType, e.DebitAccount, e.CreditAccount,
						e.Amount, e.Currency, e.PostingStatus, e.Description))
				}
				contextData.WriteString(fmt.Sprintf("\n汇总: 共 %d 条分录，其中 %d 条已过账\n", len(entries), postedCount))
				*sources = append(*sources, Source{
					Type:    "journal",
					ID:      pc.Period,
					Title:   fmt.Sprintf("期间 %s 会计分录", pc.Period),
					Snippet: fmt.Sprintf("共 %d 条", len(entries)),
				})
			}
		}
	}
}

// buildSystemPrompt constructs the structured system prompt for the LLM.
func (h *Agent) buildSystemPrompt(userID, role, legalEntityID, contextData string, isWorkingData bool, language string) string {
	workingWarning := ""
	if isWorkingData {
		workingWarning = "\n⚠️ 注意：当前页面展示的是 Working/试算数据，这不是 Official 口径，请在回答中提醒用户。\n"
	}

	// Language instruction
	var langInstruction string
	switch language {
	case "en":
		langInstruction = "\n\nPlease answer in English."
	case "zh-TW":
		langInstruction = "\n\n請用繁體中文回答。"
	default: // zh-CN
		langInstruction = "\n\n请用简体中文回答。"
	}

	return fmt.Sprintf(`你是零售经营分析工作站的 AI Agent，协助经营分析人员完成「发现问题 — 解释原因 — 模拟方案 — 形成行动」的闭环。IFRS 16 租赁会计是工作站中的一个高价值合规模块，不是全部。准确性、可追溯性和专业审慎比速度更重要。

当前用户: %s
角色: %s
可访问法人: %s
%s
%s
%s

## 核心原则

- 你不是普通聊天机器人。你应把用户目标拆解为可执行步骤，并说明需要调用或已经调用的系统工具/Office skill
- Office skill 只负责读取、展开或生成文件；字段语义、会计判断和风险提示由 AI Agent 完成；正式入库和计量由 Core Service 规则控制
- 只能基于以上系统数据回答问题，不得编造任何数字、日期、会计分录、准则引用或审计结论
- 严格区分：事实（系统数据）、假设（用户提供的参数）、计算结果（系统输出的计量数值）、会计估计（如折现率、租赁期限判断）、专业判断（如分类建议）
- 保留原始数值，不得擅自四舍五入、归并、重分类或净额列示（例如 ¥3,255,676.79 不得写成约 ¥326万）
- 当不确定性影响财务报告或合规判断时，优先保守解释
- 在做出重大假设前，先向用户确认

## 证据优先

- 每条结论必须可追溯到系统中的具体合同、计量结果、会计分录或付款计划
- 引用数据时标注：来源类型、编号/期间、版本状态（Approved/Draft/Pending）
- 汇总财务数据时，注明涵盖的合同范围、期间、币种和数据口径
- 发现以下情况时必须明确标注：
  - 数据缺失或不完整
  - 数值不一致或无法勾稽
  - 日期逻辑异常（如开始日晚于结束日）
  - 审批状态异常（如已过账但分录仍为草稿）
- 不确定时使用审慎措辞："根据当前系统数据..."、"未看到支持...的证据"、"这表明..."、"在得出结论前需要进一步核实..."

## IFRS 16 会计处理

- 在生成或解释会计分录前，先确认以下要素：
  - 交易/事件日期
  - 涉及的合同和法人主体
  - 金额和币种
  - 相关科目（租赁负债、使用权资产、利息费用、折旧费用等）
  - 适用的会计准则（IFRS 16 / CAS 21）
  - 支撑证据（合同条款、付款计划、事件记录）
- 会计分录需展示借贷方、金额、合计，并附简要说明
- 若科目分类不确定，列出现有选项并解释各选项的影响
- 不得代替用户审批、过账或确认分录

## 计量与计算

- 涉及重要计算时（租赁负债现值、利息摊销、折旧、重估调整），展示公式和中间步骤
- 尽可能将合计数与源头数据勾稽核对
- 发现差异时高亮显示，而非隐藏
- 若涉及会计估计（折现率、租赁期限、残值等），明确指出：输入值、方法、假设前提及敏感性

## 回答结构

严格遵循以下结构：

【结论】
用 1-3 句话给出直接回答。

【依据】
逐条列出支撑结论的事实，每条标注来源，格式为：
- ...（来源：合同 LEASE-XXXX-XXXX，状态：Approved）
- ...（来源：计量结果 2024-01）
- ...（来源：会计分录 2024-01，状态：Posted）
- ...（来源：AI 推断，非系统正式数据）← 如涉及推断必须标注

【数据质量提示】
如发现以下问题，在此列出：
- 缺失数据（缺少哪类数据）
- 不一致或异常（具体描述）
- Working/试算数据提醒（非 Official 口径）
如无问题，标注"数据完整，未发现异常"。

【建议动作】
根据数据和结论，给出具体可执行的建议操作，例如：
- 去补录折现率（合同 XXX 缺少折现率）
- 去执行 IFRS 16 重算（因事件变更尚未重算）
- 去查看摊销报表（核对利息和折旧）
- 去审批待处理事件（X 条事件待审批）
- 无需操作，数据正常

## 边界

- 不得签署、认证、审批或替代持牌专业人员的工作
- 不得代替管理层做决策
- 不得篡改原始数据
- 不得隐瞒错误、绕过控制或协助粉饰财务信息
- 如被要求执行误导性、不合规或无依据的操作，必须拒绝并建议合规替代方案`, userID, role, legalEntityID, workingWarning, langInstruction, contextData)
}

// fileParseTools defines the available tools for LLM function calling when a file is uploaded.
var fileParseTools = []map[string]interface{}{
	{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "parse_contract_batch",
			"description": "解析合同台账文件（Excel/PDF），批量提取多份合同草稿。当用户上传包含多份合同的台账文件，或提到'台账'、'批量'、'导入多份合同'、'ledger'时使用。",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"file_id":      map[string]interface{}{"type": "string", "description": "上传文件的ID"},
					"object_name":  map[string]interface{}{"type": "string", "description": "文件在MinIO中的对象名"},
					"content_type": map[string]interface{}{"type": "string", "description": "文件MIME类型"},
				},
				"required": []string{"file_id", "object_name", "content_type"},
			},
		},
	},
	{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "parse_payment_schedule",
			"description": "解析租金表/付款计划文件。当用户上传租金表、付款计划、rent schedule，或提到'租金'、'付款'、'payment schedule'时使用。",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"file_id":      map[string]interface{}{"type": "string", "description": "上传文件的ID"},
					"object_name":  map[string]interface{}{"type": "string", "description": "文件在MinIO中的对象名"},
					"content_type": map[string]interface{}{"type": "string", "description": "文件MIME类型"},
					"contract_id":  map[string]interface{}{"type": "string", "description": "关联的合同ID（可选）"},
				},
				"required": []string{"file_id", "object_name", "content_type"},
			},
		},
	},
	{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "parse_contract",
			"description": "解析单个合同文件（PDF/Word/图片），提取合同字段生成草稿。当用户上传单份合同文件，且未明确提到'台账'或'批量'时使用。",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"file_id":      map[string]interface{}{"type": "string", "description": "上传文件的ID"},
					"object_name":  map[string]interface{}{"type": "string", "description": "文件在MinIO中的对象名"},
					"content_type": map[string]interface{}{"type": "string", "description": "文件MIME类型"},
				},
				"required": []string{"file_id", "object_name", "content_type"},
			},
		},
	},
}

// callLLM sends the prompt and message history to the AI Service chat endpoint.
func (h *Agent) callLLM(ctx context.Context, authHeader, systemPrompt, userMessage string, history []ChatMessage, language string) (string, string, error) {
	aiServiceURL := os.Getenv("AI_SERVICE_URL")
	if aiServiceURL == "" {
		aiServiceURL = "http://ai-service:8000"
	}

	// Build messages from history + current message
	messages := []map[string]string{}
	for _, h := range history {
		messages = append(messages, map[string]string{"role": h.Role, "content": h.Content})
	}
	messages = append(messages, map[string]string{"role": "user", "content": userMessage})

	reqBody, err := json.Marshal(map[string]interface{}{
		"messages":      messages,
		"system_prompt": systemPrompt,
		"temperature":   0.3,
		"max_tokens":    2000,
		"language":      language,
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", aiServiceURL+"/api/v1/chat", bytes.NewReader(reqBody))
	if err != nil {
		return "", "", fmt.Errorf("failed to create request: %w", err)
	}
	httpReq = httpReq.WithContext(ctx)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", authHeader)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", "", fmt.Errorf("AI service unreachable: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("AI service returned %d: %s", resp.StatusCode, string(body))
	}

	var result Response
	if err := json.Unmarshal(body, &result); err != nil {
		return "", "", fmt.Errorf("failed to parse response: %w", err)
	}

	return result.Answer, result.Model, nil
}

// callLLMWithTools sends the prompt and message history to the AI Service chat endpoint with function calling support.
// It returns the LLM answer, model name, and any tool_calls if the LLM decided to invoke tools.
func (h *Agent) callLLMWithTools(ctx context.Context, authHeader, systemPrompt, userMessage string, history []ChatMessage, language string, tools []map[string]interface{}) (string, string, []AgentToolCall, error) {
	aiServiceURL := os.Getenv("AI_SERVICE_URL")
	if aiServiceURL == "" {
		aiServiceURL = "http://ai-service:8000"
	}

	// Build messages from history + current message
	messages := []map[string]string{}
	for _, h := range history {
		messages = append(messages, map[string]string{"role": h.Role, "content": h.Content})
	}
	messages = append(messages, map[string]string{"role": "user", "content": userMessage})

	reqBody, err := json.Marshal(map[string]interface{}{
		"messages":      messages,
		"system_prompt": systemPrompt,
		"temperature":   0.1,
		"max_tokens":    2000,
		"language":      language,
		"tools":         tools,
		"tool_choice":   "auto",
	})
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", aiServiceURL+"/api/v1/chat", bytes.NewReader(reqBody))
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq = httpReq.WithContext(ctx)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", authHeader)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", "", nil, fmt.Errorf("AI service unreachable: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", "", nil, fmt.Errorf("AI service returned %d: %s", resp.StatusCode, string(body))
	}

	var result LLMChatResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", "", nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Convert tool_calls to AgentToolCall format
	var agentToolCalls []AgentToolCall
	if result.ToolCalls != nil {
		for _, tc := range result.ToolCalls {
			funcName := ""
			funcArgs := ""
			if tc.Function != nil {
				if name, ok := tc.Function["name"].(string); ok {
					funcName = name
				}
				if args, ok := tc.Function["arguments"].(string); ok {
					funcArgs = args
				}
			}
			agentToolCalls = append(agentToolCalls, AgentToolCall{
				Tool:           funcName,
				Skill:          "LLM Function Calling",
				Status:         "completed",
				InputSummary:   funcArgs,
				OutputSummary:  "等待执行",
				RequiresReview: false,
			})
		}
	}

	return result.Answer, result.Model, agentToolCalls, nil
}

// extractSourcesFromAnswer parses the LLM answer for source citations and matches them
// against the known sources from context retrieval. It returns a deduplicated slice.
func extractSourcesFromAnswer(answer string, knownSources []Source) []Source {
	if len(knownSources) == 0 {
		return knownSources
	}

	// Find source citations in the answer, pattern: （来源：XXX）
	re := regexp.MustCompile(`（来源[：:]\s*([^）)]+)）`)
	matches := re.FindAllStringSubmatch(answer, -1)

	// M6.2 citation fidelity: sources are ONLY the citations the model
	// explicitly wrote that intersect the known sources. An answer with no
	// citations carries an empty list — the UI labels it 「未附来源」 — and
	// citations that match nothing are dropped, never widened into "all
	// known sources" the model never claimed.
	if len(matches) == 0 {
		return nil
	}

	// Build a set of cited source tokens
	citedTokens := make(map[string]bool)
	for _, m := range matches {
		if len(m) > 1 {
			token := strings.TrimSpace(strings.ToLower(m[1]))
			citedTokens[token] = true
		}
	}

	// Find which known sources were cited
	seen := make(map[string]bool)
	var cited []Source
	for _, s := range knownSources {
		// Check if any part of the source (id, title, snippet) matches a citation
		isCited := false
		sourceTokens := strings.ToLower(s.ID + " " + s.Title + " " + s.Snippet)
		for token := range citedTokens {
			if strings.Contains(sourceTokens, token) {
				isCited = true
				break
			}
		}
		if isCited && !seen[s.ID] {
			cited = append(cited, s)
			seen[s.ID] = true
		}
	}

	return cited
}

func (h *Agent) parseFile(ctx context.Context, authHeader, fileID, objectName, contentType string) (*aiintake.ContractDraft, error) {
	aiServiceURL := os.Getenv("AI_SERVICE_URL")
	if aiServiceURL == "" {
		aiServiceURL = "http://ai-service:8000"
	}

	reqBody, err := json.Marshal(map[string]interface{}{
		"file_id":      fileID,
		"object_name":  objectName,
		"content_type": contentType,
		"mode":         "assist",
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", aiServiceURL+"/api/v1/parse/contract", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("AI service returned %d: %s", resp.StatusCode, string(body))
	}

	draft, err := aiintake.DecodeContract(resp.Body)
	if err != nil {
		return nil, err
	}
	if err := validateAIIntakeSource(draft.IntakeMetadata, fileID, objectName, contentType); err != nil {
		return nil, err
	}
	return draft, nil
}

func validateAIIntakeSource(metadata aiintake.IntakeMetadata, fileID, objectName, contentType string) error {
	if metadata.FileID != fileID || metadata.Evidence.ObjectName != objectName || metadata.Evidence.ContentType != contentType {
		return fmt.Errorf("AI intake response does not match the requested source")
	}
	return nil
}

type PaymentScheduleParseResult struct {
	SummaryText   string
	Sources       []Source
	Confidence    float64
	AgentPlan     []AgentPlanStep
	ToolCalls     []AgentToolCall
	ReviewPrompts []AgentReviewPrompt
	Schedules     []PaymentScheduleDraftItem
	Summary       *PaymentScheduleParseSummary
	EvidenceRefs  []agentartifact.EvidenceReference
}

func (h *Agent) parsePaymentSchedule(ctx context.Context, authHeader, fileID, objectName, contentType, contractID string) (*PaymentScheduleParseResult, error) {
	aiServiceURL := os.Getenv("AI_SERVICE_URL")
	if aiServiceURL == "" {
		aiServiceURL = "http://ai-service:8000"
	}

	reqBody, err := json.Marshal(map[string]interface{}{
		"file_id":      fileID,
		"object_name":  objectName,
		"content_type": contentType,
		"mode":         "assist",
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", aiServiceURL+"/api/v1/parse/payment-schedule", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader)

	client := &http.Client{Timeout: 180 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("AI service returned %d: %s", resp.StatusCode, string(body))
	}

	intake, err := aiintake.DecodePaymentSchedule(resp.Body)
	if err != nil {
		return nil, err
	}
	if err := validateAIIntakeSource(intake.IntakeMetadata, fileID, objectName, contentType); err != nil {
		return nil, err
	}

	schedules := make([]PaymentScheduleDraftItem, 0, len(intake.Schedules))
	for _, schedule := range intake.Schedules {
		item := PaymentScheduleDraftItem{
			PeriodStart:      schedule.PeriodStart,
			PeriodEnd:        schedule.PeriodEnd,
			DueDate:          schedule.DueDate,
			Amount:           schedule.Amount,
			PaymentTiming:    schedule.PaymentTiming,
			IsFixed:          schedule.IsFixed,
			IsLeaseComponent: schedule.IsLeaseComponent,
			AmountType:       schedule.AmountType,
			Currency:         schedule.Currency,
			Confidence:       schedule.Confidence,
		}
		schedules = append(schedules, item)
	}

	overallConf := intake.ConfidenceScores["overall"]
	requiresHuman := intake.ReviewGate.Required || intake.RequiresHumanConfirmation
	missingFields := intake.MissingFields
	warnings := intake.Warnings
	canImport := contractID != "" && len(schedules) > 0

	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("Agent 已完成租金表理解和付款计划草稿生成：识别出 **%d** 笔付款计划。\n\n", len(schedules)))
	if contractID == "" {
		summary.WriteString("⚠️ **尚未绑定合同**：付款计划必须挂到具体合同后才能入库。请在合同详情页进入 AI Chat，或在 URL 中带上 `contract_id` 后再确认导入。\n\n")
	}
	if requiresHuman || len(warnings) > 0 || len(missingFields) > 0 {
		summary.WriteString("⚠️ **需要人工确认**：请重点核对覆盖期间、应付日、金额、先付/后付、变量租金和非租赁成分。\n\n")
	} else {
		summary.WriteString("识别结果总体良好，仍需人工快速核对后才能导入。\n\n")
	}
	summary.WriteString("下方付款计划草稿只处于 Assist Mode。确认导入后也只写入当前合同付款计划，不会绕过合同审批、计量重算和月结控制。")

	planStatus := statusFromReview(requiresHuman || len(warnings) > 0 || len(missingFields) > 0)
	importStatus := "pending"
	if !canImport {
		importStatus = "needs_review"
	}
	toolCalls := []AgentToolCall{
		{
			Tool:           "office.sheet_reader",
			Skill:          "Office Excel/PDF Skill",
			Status:         "completed",
			InputSummary:   fmt.Sprintf("读取上传租金表 %s", objectName),
			OutputSummary:  "已将租金表展开为可供 LLM 理解的付款证据",
			RequiresReview: false,
		},
		{
			Tool:           "lease.payment_schedule_parser",
			Skill:          "Payment Schedule Skill",
			Status:         planStatus,
			InputSummary:   "识别覆盖期间、应付日、金额、先付/后付和会计属性",
			OutputSummary:  fmt.Sprintf("生成 %d 笔付款计划草稿，缺失字段 %d 类，警告 %d 条", len(schedules), len(missingFields), len(warnings)),
			RequiresReview: planStatus == "needs_review",
		},
		{
			Tool:           "lease.payment_schedule_importer",
			Skill:          "Core Service Payment Schedule Skill",
			Status:         importStatus,
			InputSummary:   "等待用户确认付款计划草稿",
			OutputSummary:  "确认后写入指定合同的付款计划",
			RequiresReview: true,
		},
	}
	agentPlan := []AgentPlanStep{
		{ID: "read_schedule", Title: "读取租金表并保留证据", Status: "completed"},
		{ID: "classify_payments", Title: "理解付款期间、金额和会计属性", Status: planStatus},
		{ID: "human_review", Title: "人工确认异常金额、缺失期间和变量租金", Status: "needs_review"},
		{ID: "import_schedule", Title: "确认后导入付款计划", Status: importStatus},
	}

	return &PaymentScheduleParseResult{
		SummaryText:   summary.String(),
		Sources:       []Source{{Type: "file", ID: intake.Evidence.SourceFileID, Title: "租金表文件", Snippet: intake.Evidence.ObjectName}},
		Confidence:    overallConf,
		AgentPlan:     agentPlan,
		ToolCalls:     toolCalls,
		ReviewPrompts: buildPaymentScheduleReviewPrompts(schedules, missingFields, warnings, contractID),
		Schedules:     schedules,
		EvidenceRefs:  []agentartifact.EvidenceReference{evidenceReferenceFromIntake(intake.Evidence)},
		Summary: &PaymentScheduleParseSummary{
			TotalCount:           len(schedules),
			OverallConfidence:    overallConf,
			RequiresHumanConfirm: requiresHuman,
			MissingFields:        missingFields,
			Warnings:             warnings,
			CanImport:            canImport,
			ContractID:           contractID,
			SchemaVersion:        intake.SchemaVersion,
			IntakeID:             intake.IntakeID,
			EvidenceComplete:     intake.Evidence.Complete,
			ReviewReasons:        intake.ReviewGate.Reasons,
		},
	}, nil
}

type BatchParseResult struct {
	SummaryText   string
	Sources       []Source
	Confidence    float64
	AgentPlan     []AgentPlanStep
	ToolCalls     []AgentToolCall
	ReviewPrompts []AgentReviewPrompt
	Contracts     []ContractDraftItem
	Summary       *BatchParseSummary
	EvidenceRefs  []agentartifact.EvidenceReference
}

func (h *Agent) parseContractBatch(ctx context.Context, authHeader, fileID, objectName, contentType string) (*BatchParseResult, error) {
	aiServiceURL := os.Getenv("AI_SERVICE_URL")
	if aiServiceURL == "" {
		aiServiceURL = "http://ai-service:8000"
	}

	reqBody, err := json.Marshal(map[string]interface{}{
		"file_id":      fileID,
		"object_name":  objectName,
		"content_type": contentType,
		"mode":         "assist",
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", aiServiceURL+"/api/v1/parse/contract-batch", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader)

	client := &http.Client{Timeout: 180 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("AI service returned %d: %s", resp.StatusCode, string(body))
	}

	intake, err := aiintake.DecodeContractBatch(resp.Body)
	if err != nil {
		return nil, err
	}
	if err := validateAIIntakeSource(intake.IntakeMetadata, fileID, objectName, contentType); err != nil {
		return nil, err
	}

	contracts := intake.Contracts
	totalCount := intake.TotalCount
	overallConf := intake.ConfidenceScores["overall"]
	requiresHuman := intake.ReviewGate.Required || intake.RequiresHumanConfirmation
	warnings := intake.Warnings
	missingFields := intake.MissingFields

	// Build summary text
	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("Agent 已完成文件理解和合同草稿生成：识别出 **%d** 份合同。\n\n", totalCount))
	if requiresHuman {
		summary.WriteString("⚠️ **需要人工确认**：部分合同存在缺失字段、低置信度识别结果或会计判断事项，请逐条核对后再确认入库。\n\n")
	} else {
		summary.WriteString("识别结果总体良好，建议快速核对后确认入库。\n\n")
	}
	if len(missingFields) > 0 {
		summary.WriteString(fmt.Sprintf("需要优先确认的字段：%s。\n\n", strings.Join(missingFields, ", ")))
	}
	summary.WriteString("请在下方草稿表格中逐条编辑、跳过或确认每份合同。确认后 Agent 只会创建 draft 合同，正式入库仍进入正常审批流程。")

	sources := []Source{{
		Type:    "file",
		ID:      intake.Evidence.SourceFileID,
		Title:   "合同台账文件",
		Snippet: intake.Evidence.ObjectName,
	}}

	officeSkill := "Office Excel Skill"
	if !isExcelContentType(contentType) {
		officeSkill = "Office Document Skill"
	}
	toolCalls := []AgentToolCall{
		{
			Tool:           "office.file_reader",
			Skill:          officeSkill,
			Status:         "completed",
			InputSummary:   fmt.Sprintf("读取上传文件 %s", objectName),
			OutputSummary:  "已将文件展开为可供 LLM 理解的文本/表格证据",
			RequiresReview: false,
		},
		{
			Tool:           "lease.contract_batch_parser",
			Skill:          "Lease Intake Skill",
			Status:         statusFromReview(requiresHuman),
			InputSummary:   "基于文件证据语义识别合同台账字段、范围判定和缺失项",
			OutputSummary:  fmt.Sprintf("生成 %d 份合同草稿，缺失字段 %d 类，警告 %d 条", len(contracts), len(missingFields), len(warnings)),
			RequiresReview: requiresHuman,
		},
		{
			Tool:           "lease.draft_contract_creator",
			Skill:          "Core Service Draft Skill",
			Status:         "needs_review",
			InputSummary:   "等待用户确认草稿卡片",
			OutputSummary:  "确认后批量创建 draft 合同，不绕过审批",
			RequiresReview: true,
		},
	}
	agentPlan := []AgentPlanStep{
		{ID: "read_file", Title: "读取 Office 文件并保留证据", Status: "completed"},
		{ID: "understand_ledger", Title: "理解非标准台账并生成合同草稿", Status: statusFromReview(requiresHuman)},
		{ID: "human_review", Title: "人工确认缺失字段和会计判断", Status: "needs_review"},
		{ID: "create_draft", Title: "确认后创建 draft 合同并进入审批", Status: "pending"},
	}
	reviewPrompts := buildReviewPrompts(contracts, missingFields, warnings)

	return &BatchParseResult{
		SummaryText:   summary.String(),
		Sources:       sources,
		Confidence:    overallConf,
		AgentPlan:     agentPlan,
		ToolCalls:     toolCalls,
		ReviewPrompts: reviewPrompts,
		Contracts:     contracts,
		EvidenceRefs:  []agentartifact.EvidenceReference{evidenceReferenceFromIntake(intake.Evidence)},
		Summary: &BatchParseSummary{
			TotalCount:           totalCount,
			OverallConfidence:    overallConf,
			RequiresHumanConfirm: requiresHuman,
			MissingFields:        missingFields,
			Warnings:             warnings,
			SchemaVersion:        intake.SchemaVersion,
			IntakeID:             intake.IntakeID,
			EvidenceComplete:     intake.Evidence.Complete,
			ReviewReasons:        intake.ReviewGate.Reasons,
		},
	}, nil
}

func isExcelContentType(contentType string) bool {
	return contentType == "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" ||
		contentType == "application/vnd.ms-excel"
}

func evidenceReferenceFromIntake(evidence aiintake.Evidence) agentartifact.EvidenceReference {
	locators := make([]agentartifact.EvidenceLocator, 0, len(evidence.Locators))
	for _, locator := range evidence.Locators {
		locators = append(locators, agentartifact.EvidenceLocator{
			Field: locator.Field, Source: locator.Source, Page: locator.Page,
			Sheet: locator.Sheet, CellRange: locator.CellRange,
			Coordinates: append([]float64(nil), locator.Coordinates...), FileHash: locator.FileHash, Quote: locator.Quote,
		})
	}
	return agentartifact.EvidenceReference{
		SourceFileID: evidence.SourceFileID, ObjectName: evidence.ObjectName,
		ContentType: evidence.ContentType, FileHash: evidence.FileHash, Locators: locators,
		Complete: evidence.Complete, MissingReason: evidence.MissingReason,
	}
}

func buildReviewPrompts(contracts []ContractDraftItem, missingFields []string, warnings []string) []AgentReviewPrompt {
	var prompts []AgentReviewPrompt

	if len(missingFields) > 0 {
		prompts = append(prompts, AgentReviewPrompt{
			ID:          "missing_fields",
			Title:       "补齐全局缺失字段",
			Description: fmt.Sprintf("本次解析发现 %d 类缺失字段：%s。缺失字段会影响正式台账完整性。", len(missingFields), strings.Join(missingFields, ", ")),
			Severity:    "warning",
			Action:      "在下方草稿表格中补齐字段，或明确跳过不适用字段后再确认入库。",
		})
	}

	discountRateContracts := make([]string, 0)
	lowConfidenceContracts := make([]string, 0)
	scopeReviewContracts := make([]string, 0)
	for _, contract := range contracts {
		if contract.DiscountRate == 0 || slices.Contains(contract.MissingFields, "discount_rate") {
			discountRateContracts = append(discountRateContracts, contract.ContractNumber)
		}
		if contract.Confidence < 0.8 || contract.ScopeConfidence < 0.8 {
			lowConfidenceContracts = append(lowConfidenceContracts, contract.ContractNumber)
		}
		if contract.LeaseScope == "" || contract.LeaseScope != "in_scope" || contract.ScopeConfidence < 0.8 {
			scopeReviewContracts = append(scopeReviewContracts, contract.ContractNumber)
		}
	}

	if len(discountRateContracts) > 0 {
		prompts = append(prompts, AgentReviewPrompt{
			ID:              "discount_rate",
			Title:           "确认折现率",
			Description:     fmt.Sprintf("%d 份合同缺少折现率或折现率为 0。AI 不得猜测折现率。", len(discountRateContracts)),
			Severity:        "critical",
			Action:          "检查合同文本、系统利率政策或请人工选择适用 IBR 后再确认。",
			ContractNumbers: firstStrings(discountRateContracts, 8),
		})
	}

	if len(scopeReviewContracts) > 0 {
		prompts = append(prompts, AgentReviewPrompt{
			ID:              "lease_scope",
			Title:           "复核租赁范围判断",
			Description:     fmt.Sprintf("%d 份合同需要确认是否资本化、短期/低价值豁免或非租赁。", len(scopeReviewContracts)),
			Severity:        "warning",
			Action:          "复核 lease_scope、scope confidence 和豁免/排除原因，确认后才进入 draft 合同。",
			ContractNumbers: firstStrings(scopeReviewContracts, 8),
		})
	}

	if len(lowConfidenceContracts) > 0 {
		prompts = append(prompts, AgentReviewPrompt{
			ID:              "low_confidence",
			Title:           "核对低置信度草稿",
			Description:     fmt.Sprintf("%d 份合同存在整体或范围判断低置信度。", len(lowConfidenceContracts)),
			Severity:        "warning",
			Action:          "优先核对这些合同的日期、主体、租金、付款时点和范围判断。",
			ContractNumbers: firstStrings(lowConfidenceContracts, 8),
		})
	}

	if len(prompts) == 0 && len(warnings) == 0 && len(contracts) > 0 {
		prompts = append(prompts, AgentReviewPrompt{
			ID:          "quick_review",
			Title:       "快速核对后确认",
			Description: "未发现全局缺失字段或低置信度草稿，但 AI 结果仍需人工确认。",
			Severity:    "info",
			Action:      "抽查关键字段和原始文件证据，确认后创建 draft 合同。",
		})
	}

	return prompts
}

func buildPaymentScheduleReviewPrompts(schedules []PaymentScheduleDraftItem, missingFields []string, warnings []string, contractID string) []AgentReviewPrompt {
	var prompts []AgentReviewPrompt
	if contractID == "" {
		prompts = append(prompts, AgentReviewPrompt{
			ID:          "bind_contract",
			Title:       "绑定目标合同",
			Description: "付款计划必须挂到具体合同后才能入库，当前 AI Chat 请求没有合同上下文。",
			Severity:    "critical",
			Action:      "进入合同详情页后上传租金表，或在 AI Chat URL 中带上 contract_id。",
		})
	}
	if len(missingFields) > 0 {
		prompts = append(prompts, AgentReviewPrompt{
			ID:          "schedule_missing_fields",
			Title:       "补齐付款计划缺失字段",
			Description: fmt.Sprintf("本次解析发现 %d 类缺失字段：%s。", len(missingFields), strings.Join(missingFields, ", ")),
			Severity:    "warning",
			Action:      "核对租金表原文，补齐覆盖期间、应付日、金额或币种后再导入。",
		})
	}

	lowConfidence := 0
	variableOrNonLease := 0
	emptyCurrency := 0
	for _, schedule := range schedules {
		if schedule.Confidence < 0.8 {
			lowConfidence++
		}
		if !schedule.IsFixed || !schedule.IsLeaseComponent || schedule.AmountType == "turnover_rent" || schedule.AmountType == "cam" || schedule.AmountType == "service_fee" {
			variableOrNonLease++
		}
		if schedule.Currency == "" {
			emptyCurrency++
		}
	}
	if lowConfidence > 0 {
		prompts = append(prompts, AgentReviewPrompt{
			ID:          "schedule_low_confidence",
			Title:       "核对低置信度付款行",
			Description: fmt.Sprintf("%d 笔付款计划置信度低于 0.8。", lowConfidence),
			Severity:    "warning",
			Action:      "优先核对这些行的期间、金额、付款时点和会计属性。",
		})
	}
	if variableOrNonLease > 0 {
		prompts = append(prompts, AgentReviewPrompt{
			ID:          "schedule_accounting_attribute",
			Title:       "确认变量租金和非租赁成分",
			Description: fmt.Sprintf("%d 笔付款可能涉及变量租金或非租赁成分。", variableOrNonLease),
			Severity:    "warning",
			Action:      "确认 turnover rent、CAM、服务费等不得错误资本化计入租赁负债。",
		})
	}
	if emptyCurrency > 0 {
		prompts = append(prompts, AgentReviewPrompt{
			ID:          "schedule_currency",
			Title:       "确认币种",
			Description: fmt.Sprintf("%d 笔付款缺少币种。AI 不应猜测文件中未出现的币种。", emptyCurrency),
			Severity:    "warning",
			Action:      "根据合同或租金表原文确认币种后再导入。",
		})
	}
	if len(prompts) == 0 && len(warnings) == 0 && len(schedules) > 0 {
		prompts = append(prompts, AgentReviewPrompt{
			ID:          "schedule_quick_review",
			Title:       "快速核对后导入",
			Description: "未发现明显缺失字段或低置信度付款行，但付款计划仍需人工确认。",
			Severity:    "info",
			Action:      "抽查期间、金额、付款时点和会计属性，确认后导入付款计划。",
		})
	}
	return prompts
}

func statusFromReview(requiresHuman bool) string {
	if requiresHuman {
		return "needs_review"
	}
	return "completed"
}

func firstStrings(values []string, limit int) []string {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}

func containsAny(msg string, targets []string) bool {
	for _, t := range targets {
		if strings.Contains(strings.ToLower(msg), strings.ToLower(t)) {
			return true
		}
	}
	return false
}

// extractS1Input detects a structured S1 assumption block in the user
// message. It requires the human-confirmed essentials the engine cannot live
// without: commencement date, monthly rent and — explicitly — a discount
// rate, which AI is never allowed to guess.
func extractS1Input(message string) (json.RawMessage, bool) {
	start := strings.Index(message, "{")
	end := strings.LastIndex(message, "}")
	if start < 0 || end <= start {
		return nil, false
	}
	candidate := message[start : end+1]
	var probe map[string]json.RawMessage
	if json.Unmarshal([]byte(candidate), &probe) != nil {
		return nil, false
	}
	draftRaw, ok := probe["draft"]
	if !ok {
		return nil, false
	}
	var draft map[string]json.RawMessage
	if json.Unmarshal(draftRaw, &draft) != nil {
		return nil, false
	}
	for _, key := range []string{"commencement_date", "monthly_rent", "discount_rate", "term_months"} {
		if _, ok := draft[key]; !ok {
			return nil, false
		}
	}
	return json.RawMessage(candidate), true
}

// executeS1Paper runs the S1 paper tool for a confirmed assumption block and
// shapes the review-gated response. The tool is LevelDraft with review
// required: the runtime marks the call needs_review and the paper becomes a
// working_paper artifact awaiting human confirmation.
func (h *Agent) executeS1Paper(ctx context.Context, req Request, s1Block json.RawMessage, emit func(context.Context, string, any) error, toolRuntime *agenttools.Runtime) (Response, error) {
	execution, err := agenttools.RequireExecutionContext(ctx)
	if err != nil {
		return Response{Answer: "agent execution context missing", Model: "runtime"}, err
	}
	arguments := json.RawMessage(fmt.Sprintf(`{"input":%s}`, s1Block))

	if err := emitAgentEvent(ctx, emit, "tool_start", map[string]interface{}{
		"tool":   "lease.working_paper.s1.generate",
		"status": "running",
	}); err != nil {
		return Response{Answer: "AI agent event persistence failed", Model: "runtime"}, err
	}
	result, durationMs, err := h.executeToolCall(ctx, toolRuntime, "lease.working_paper.s1.generate", arguments, "tool:s1:"+execution.RunID)
	if err != nil {
		if emitErr := emitAgentEvent(ctx, emit, "tool_end", []map[string]interface{}{{
			"tool": "lease.working_paper.s1.generate", "skill": "", "status": "failed",
			"input_summary": "S1 假设块", "output_summary": err.Error(), "requires_review": false,
		}}); emitErr != nil {
			return Response{Answer: "AI agent event persistence failed", Model: "runtime"}, emitErr
		}
		return Response{
			Answer:     fmt.Sprintf("S1 底稿生成失败：%s。请检查假设数值（折现率必须由你确认，AI 不猜测）。", err.Error()),
			Model:      "deterministic-router",
			Confidence: 0.4,
		}, nil
	}

	status := string(result.Status)
	summary := ""
	if result.Error != nil {
		summary = result.Error.Message
	}
	if err := emitAgentEvent(ctx, emit, "tool_end", []map[string]interface{}{{
		"tool": "lease.working_paper.s1.generate", "skill": "", "status": status,
		"input_summary": "S1 假设块", "output_summary": summary,
		"requires_review": result.Review.Required, "duration_ms": durationMs,
	}}); err != nil {
		return Response{Answer: "AI agent event persistence failed", Model: "runtime"}, err
	}

	data, ok := result.Data.(map[string]any)
	if !ok {
		return Response{Answer: "S1 底稿生成失败：工具结果格式无效", Model: "deterministic-router"}, nil
	}
	paper, ok := data["paper"].(workingpaper.Paper)
	if !ok {
		return Response{Answer: "S1 底稿生成失败：底稿格式无效", Model: "deterministic-router"}, nil
	}

	return Response{
		Answer:       "S1 签约前决策底稿已生成。所有数字来自确定性引擎（predeal/dealcompare），并标注了出处；请复核关键假设（尤其折现率）后确认。",
		Model:        "deterministic-router",
		Confidence:   0.9,
		WorkingPaper: &paper,
		ReviewPrompts: []AgentReviewPrompt{{
			ID: "s1_paper_review", Title: "复核 S1 底稿",
			Description: "底稿内的 IFRS 16 影响、EBITDA 桥与退出曲线均为引擎计算值；复核折现率与关键假设后确认。",
			Severity:    "critical", Action: "review_confirm",
		}},
	}, nil
}

// executeRetailIngestFill handles an operating-data upload by asking the
// import-preview tool to prefill the retail import page. When the file
// reader is not wired (D-D2) or the call fails, the response stays honest:
// guidance to the import page, no fabricated preview.
func (h *Agent) executeRetailIngestFill(ctx context.Context, req Request, triage agenttooldefs.TriageResult, emit func(context.Context, string, any) error, toolRuntime *agenttools.Runtime) (Response, error) {
	sourceSystem := extractSourceSystem(req.Message)
	// The tool demands a human-provided source system; without one the fill
	// cannot be built and we guide instead.
	if sourceSystem == "" {
		return fileTriageRefusalWithHint(req, triage, "生成预填需要你提供来源系统（例如「来源系统 pos-a」）。也可以直接前往「零售数据导入」页上传。"), nil
	}

	if err := emitAgentEvent(ctx, emit, "tool_start", map[string]interface{}{
		"tool": "retail.store_days.import.preview", "status": "running",
	}); err != nil {
		return Response{Answer: "AI agent event persistence failed", Model: "runtime"}, err
	}
	args := map[string]any{
		"file_id": req.FileID, "object_name": req.ObjectName,
		"content_type": req.ContentType, "source_system": sourceSystem,
	}
	result, durationMs, err := h.executeToolCall(ctx, toolRuntime, "retail.store_days.import.preview", args, "tool:ingest-fill:"+req.FileID)
	if err != nil || result.Data == nil {
		_ = emitAgentEvent(ctx, emit, "tool_end", []map[string]interface{}{{
			"tool": "retail.store_days.import.preview", "status": "failed",
			"output_summary": errorSummary(err, result), "requires_review": false,
		}})
		return fileTriageRefusalWithHint(req, triage, "文件预填暂不可用（文件读取通道尚未接通）。请直接前往「零售数据导入」页上传并确认映射。"), nil
	}
	summary := ""
	if result.Error != nil {
		summary = result.Error.Message
	}
	if err := emitAgentEvent(ctx, emit, "tool_end", []map[string]interface{}{{
		"tool": "retail.store_days.import.preview", "status": string(result.Status),
		"input_summary": "零售导入预填", "output_summary": summary,
		"requires_review": result.Review.Required, "duration_ms": durationMs,
	}}); err != nil {
		return Response{Answer: "AI agent event persistence failed", Model: "runtime"}, err
	}

	data, ok := result.Data.(map[string]any)
	if !ok {
		return fileTriageRefusalWithHint(req, triage, "预填结果格式无效，请直接前往「零售数据导入」页上传。"), nil
	}
	fill, ok := data["page_fill"].(*pagefill.Fill)
	if !ok {
		return fileTriageRefusalWithHint(req, triage, "预填结果格式无效，请直接前往「零售数据导入」页上传。"), nil
	}
	return Response{
		Answer:     "已在「零售数据导入」页为你预填（来源系统、as-of）。列映射以建议形式呈现，请确认后入库——导入动作由你在页面上完成。",
		Model:      "deterministic-router",
		Confidence: 0.8,
		PageFill:   fill,
		FileTriage: &triage,
		ReviewPrompts: []AgentReviewPrompt{{
			ID: "import_mapping_review", Title: "确认导入映射",
			Description: "预填页的列映射尚未确认；请核对后提交。Agent 无权 commit，入库由你完成。",
			Severity:    "warning", Action: "review_confirm",
		}},
	}, nil
}

func fileTriageRefusalWithHint(req Request, triage agenttooldefs.TriageResult, hint string) Response {
	resp := fileTriageRefusal(req, triage)
	resp.Answer = hint
	return resp
}

func errorSummary(err error, result agenttools.ToolResult) string {
	if err != nil {
		return err.Error()
	}
	if result.Error != nil {
		return result.Error.Message
	}
	return ""
}

// extractSourceSystem picks a human-stated source system out of the message.
func extractSourceSystem(message string) string {
	lower := strings.ToLower(message)
	for _, marker := range []string{"来源系统 ", "来源系统：", "source system ", "source_system "} {
		if idx := strings.Index(lower, marker); idx >= 0 {
			// Byte-length arithmetic is safe here: CJK markers match
			// verbatim and ASCII case folding is 1:1 in bytes.
			rest := strings.TrimSpace(message[idx+len(marker):])
			field := strings.Fields(rest)
			if len(field) > 0 {
				return cutAtPunctuation(field[0])
			}
		}
	}
	return ""
}

// cutAtPunctuation truncates a token at the first CJK/ASCII separator so a
// trailing 「，谢谢」 never rides along with the value.
func cutAtPunctuation(token string) string {
	for i, r := range token {
		if strings.ContainsRune("，,。.；;、 ", r) {
			if i == 0 {
				return ""
			}
			return token[:i]
		}
	}
	return token
}

// triageToParseTool maps a triage result to the file-parse tool selector. No
// match — including unknown and classes without an automated parse path —
// maps to "" so the pipeline stops and asks the user instead of guessing.
func triageToParseTool(triage agenttooldefs.TriageResult) string {
	if triage.Confidence < agenttooldefs.TriageThreshold {
		return ""
	}
	switch triage.DocClass {
	case agenttooldefs.DocRentSchedule:
		return "parse_payment_schedule"
	case agenttooldefs.DocContractLedger:
		return "parse_contract_batch"
	case agenttooldefs.DocLeaseContract, agenttooldefs.DocAmendment:
		return "parse_contract"
	default:
		return ""
	}
}

// fileTriageRefusal stops the pipeline when the file class is unknown or has
// no automated parse path. It is the honest replacement for the former
// "default to contract" fallback.
func fileTriageRefusal(req Request, triage agenttooldefs.TriageResult) Response {
	var answer string
	switch triage.DocClass {
	case agenttooldefs.DocUnknown:
		answer = "我不确定这份文件是什么类型。请告诉我它是哪一类：租赁合同、租金表/付款计划、合同台账、经营数据、财务报表、发票或其他。"
	case agenttooldefs.DocOperatingData:
		answer = "这是经营数据文件。请前往「零售数据导入」页面上传，系统会引导列映射与入库确认。"
	default:
		answer = fmt.Sprintf("该文件类型（%s）暂不支持自动解析，请人工处理或换一个文件。", triage.DocClass)
	}
	return Response{
		Answer:     answer,
		Confidence: 0.4,
		Model:      "deterministic-router",
		FileTriage: &triage,
	}
}
