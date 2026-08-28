// Package agentskill contains the versioned, server-owned Skill Registry.
// Skills describe intent, required context, tool allowlists and review gates;
// they never contain repository or transport access.
package agentskill

import (
	"slices"

	"fmt"
	"sort"
	"strings"

	"github.com/lease-management-system/core-service/internal/agentartifact"
)

type ReviewPolicy struct {
	Required      bool     `json:"required"`
	Reasons       []string `json:"reasons,omitempty"`
	Blocking      []string `json:"blocking_reasons,omitempty"`
	Completion    []string `json:"completion_conditions,omitempty"`
	AllowedRoles  []string `json:"allowed_roles,omitempty"`
	ConfirmAction string   `json:"confirm_action,omitempty"`
}

type Definition struct {
	ID              string                       `json:"id"`
	Version         string                       `json:"version"`
	Name            string                       `json:"name"`
	Description     string                       `json:"description"`
	Aliases         []string                     `json:"aliases,omitempty"`
	IntentExamples  []string                     `json:"intent_examples,omitempty"`
	MatchTerms      []string                     `json:"match_terms,omitempty"`
	AllowedRoles    []string                     `json:"allowed_roles,omitempty"`
	RequiredInputs  []string                     `json:"required_inputs,omitempty"`
	RequiredContext []string                     `json:"required_context,omitempty"`
	AllowedTools    []string                     `json:"allowed_tools"`
	ArtifactTypes   []agentartifact.ArtifactType `json:"artifact_types,omitempty"`
	Review          ReviewPolicy                 `json:"review"`
	Priority        int                          `json:"priority"`
}

type Intent struct {
	Message          string
	Role             string
	HasFile          bool
	HasContract      bool
	HasPortfolio     bool
	RequestedSkillID string
	RequestedVersion string
}

type Registry struct {
	definitions map[string]map[string]Definition
}

func NewRegistry() *Registry {
	return &Registry{definitions: make(map[string]map[string]Definition)}
}

func ProductionRegistry() *Registry {
	registry := NewRegistry()
	for _, definition := range []Definition{
		{
			ID: "excel_ledger", Version: "v1", Name: "合同台账导入", Priority: 40,
			Description:    "从 Excel/CSV 合同台账生成可复核的合同草稿。",
			IntentExamples: []string{"导入合同台账", "批量创建合同草稿", "检查 Excel 台账"},
			MatchTerms:     []string{"excel 台账", "excel台账", "合同台账", "批量导入", "批量创建", "ledger import", "contract ledger"},
			AllowedRoles:   []string{"admin", "editor", "reviewer"}, RequiredInputs: []string{"file"},
			AllowedTools:  []string{"lease.file.triage", "lease.file.parse_contract_batch", "lease.contract.draft.create"},
			ArtifactTypes: []agentartifact.ArtifactType{agentartifact.ArtifactContractDraft},
			Review:        ReviewPolicy{Required: true, Reasons: []string{"assist_mode", "accounting_judgement"}, Blocking: []string{"missing_fields", "currency_missing", "discount_rate_missing", "low_confidence"}, Completion: []string{"evidence_reviewed", "draft_confirmed"}, AllowedRoles: []string{"reviewer", "approver"}, ConfirmAction: "create_draft"},
		},
		{
			ID: "contract_review", Version: "v1", Name: "合同复核", Priority: 30,
			Description:    "基于合同文件或当前合同上下文提取条款并生成 IFRS 16 复核事项。",
			IntentExamples: []string{"复核合同关键条款", "审阅续租选择权", "检查合同会计判断"},
			MatchTerms:     []string{"合同复核", "合同审阅", "合同审核", "复核", "审阅", "关键条款", "contract review", "review contract", "lease review"},
			AllowedRoles:   []string{"admin", "editor", "reviewer", "approver", "auditor", "readonly"}, RequiredInputs: []string{"file_or_contract"}, RequiredContext: []string{"contract_or_file"},
			AllowedTools:  []string{"lease.file.parse_contract", "lease.contract.get", "lease.measurement.list", "lease.event.list", "lease.journal.list"},
			ArtifactTypes: []agentartifact.ArtifactType{agentartifact.ArtifactReportExplanation, agentartifact.ArtifactDataQualityIssues},
			Review:        ReviewPolicy{Required: true, Reasons: []string{"accounting_judgement", "assist_mode"}, Blocking: []string{"evidence_incomplete", "discount_rate_missing", "scope_uncertain"}, Completion: []string{"evidence_reviewed", "review_notes_confirmed"}, AllowedRoles: []string{"reviewer", "approver"}, ConfirmAction: "confirm_review"},
		},
		{
			ID: "payment_schedule", Version: "v1", Name: "租金表导入", Priority: 35,
			Description:    "解析租金表并区分先付/后付、租赁/非租赁和变量租金。",
			Aliases:        []string{"payment_schedule_intake"},
			IntentExamples: []string{"导入付款计划", "解析租金表", "检查租金付款时点"},
			MatchTerms:     []string{"租金表", "付款计划", "付款表", "rent schedule", "payment schedule", "rental schedule"},
			AllowedRoles:   []string{"admin", "editor", "reviewer"}, RequiredInputs: []string{"file_or_contract"}, RequiredContext: []string{"contract_or_file"},
			AllowedTools:  []string{"lease.file.parse_payment_schedule", "lease.payment_schedule.draft.create"},
			ArtifactTypes: []agentartifact.ArtifactType{agentartifact.ArtifactPaymentScheduleDraft},
			Review:        ReviewPolicy{Required: true, Reasons: []string{"assist_mode", "payment_classification"}, Blocking: []string{"missing_period", "currency_missing", "variable_rent", "non_lease_component", "payment_timing_uncertain"}, Completion: []string{"evidence_reviewed", "draft_confirmed"}, AllowedRoles: []string{"reviewer", "approver"}, ConfirmAction: "import_draft"},
		},
		{
			ID: "event_change", Version: "v1", Name: "合同事件变更", Priority: 32,
			Description:    "将租赁范围、租期、租金或减值变化登记为可复核事件草稿，不直接触发重算或过账。",
			IntentExamples: []string{"登记租赁变更事件", "创建合同事件草稿", "录入 modification", "录入 reassessment"},
			MatchTerms:     []string{"事件草稿", "登记事件", "变更事件", "合同变更", "modification", "reassessment", "减值事件", "event draft"},
			AllowedRoles:   []string{"admin", "editor", "reviewer"}, RequiredInputs: []string{"message"}, RequiredContext: []string{"contract"},
			AllowedTools:  []string{"lease.file.parse_event", "lease.event.draft.create", "lease.contract.get", "lease.event.list"},
			ArtifactTypes: []agentartifact.ArtifactType{agentartifact.ArtifactEventDraft, agentartifact.ArtifactDataQualityIssues},
			Review:        ReviewPolicy{Required: true, Reasons: []string{"assist_mode", "accounting_judgement", "event_changes_measurement"}, Blocking: []string{"event_type_missing", "effective_date_missing", "evidence_incomplete", "accounting_treatment_missing"}, Completion: []string{"evidence_reviewed", "event_draft_confirmed", "approval_workflow_completed"}, AllowedRoles: []string{"reviewer", "approver"}, ConfirmAction: "review_ai_draft"},
		},
		{
			ID: "audit_pack", Version: "v1", Name: "审计包准备", Priority: 50,
			Description:    "在权限范围内整理合同、计量、分录、审批和证据索引，供人工审计复核。",
			IntentExamples: []string{"生成审计包", "准备披露核对清单", "整理审计底稿"},
			MatchTerms:     []string{"审计包", "审计底稿", "审计工作底稿", "披露核对", "抽样", "audit pack", "audit package", "disclosure checklist"},
			AllowedRoles:   []string{"admin", "reviewer", "approver", "auditor"}, RequiredInputs: []string{"message"}, RequiredContext: []string{"portfolio"},
			AllowedTools:  []string{"lease.contract.get", "lease.measurement.list", "lease.event.list", "lease.journal.list"},
			ArtifactTypes: []agentartifact.ArtifactType{agentartifact.ArtifactAuditPack, agentartifact.ArtifactDataQualityIssues},
			Review:        ReviewPolicy{Required: true, Reasons: []string{"scope_confirmation", "official_working_basis"}, Blocking: []string{"period_missing", "report_basis_missing"}, Completion: []string{"scope_confirmed", "pack_reviewed"}, AllowedRoles: []string{"reviewer", "auditor", "approver"}, ConfirmAction: "confirm_audit_scope"},
		},
		{
			ID: "fpna_copilot", Version: "v1", Name: "经营决策 Copilot", Priority: 45,
			Description:    "在权限范围内检索经营组合、门店四墙、设备表现和待处理行动，形成带来源的管理解释。",
			IntentExamples: []string{"为什么本月偏离 forecast", "生成经营日报", "哪些门店需要处理", "经营异常", "FP&A copilot", "finance business partner"},
			MatchTerms:     []string{"经营日报", "经营驾驶舱", "经营异常", "经营决策", "偏离 forecast", "偏离预算", "fp&a", "finance bp", "四墙", "租售比", "经营 copilot", "经营copilot"},
			AllowedRoles:   []string{"admin", "editor", "reviewer", "approver", "auditor", "readonly"}, RequiredInputs: []string{"message"}, RequiredContext: []string{"portfolio"},
			AllowedTools: []string{"lease.portfolio.summary", "fpna.management.pre_read", "retail.store.performance.read", "retail.rent_to_sales.read", "retail.equipment.performance.read", "fpna.actions.read", "lease.close.readiness", "fpna.budget.variance.read", "fpna.cashflow.scenario", "lease.renewal.decisions", "retail.store.scenario.simulate", "retail.equipment.scenario.simulate", "lease.deal.simulate", "lease.predeal.simulate", "lease.renewal.simulate", "fpna.decision.summary", "fpna.actions.draft.create", "fpna.explanations.draft.create", "fpna.meeting_actions.draft.create", "fpna.memos.decision.draft.create", "fpna.scenarios.draft.create", "lease.contract.search", "fpna.store_pnl.read", "lease.monthly_closing.batches.read", "lease.monthly_closing.entries.preview", "lease.monthly_closing.periods.read", "lease.monthly_closing.lock_status.read", "retail.operating_facts.stores.read", "retail.operating_facts.store_days.read", "retail.kpis.store_days.read", "lease.report.schedule.read", "lease.report.disclosure_package.read", "lease.report.contract_view.read", "lease.report.unit_price.read", "lease.report.tags.read", "fpna.coa.suggest_template",
				// agent-universal-pagefill-v1 P0-A①：此前不在任何 skill 白名单上
				// （审计 §B 的「自然语言不可达」清单）。写类全部只落草稿层。
				"fpna.assumptions.suggest", "fpna.assumptions.suggest_batch", "fpna.memos.model_diff.draft",
				"fpna.settlement.read", "fpna.settlement_recon_draft.create", "fpna.site_pnl.read",
				"fpna.statement_model.read", "fpna.statement_model.evaluate", "fpna.working_paper.finmodel.generate",
				"lease.report.sensitivity", "lease.working_paper.s1.generate",
				"fpna.trial_balance.fill.preview"},
			ArtifactTypes: []agentartifact.ArtifactType{agentartifact.ArtifactReportExplanation},
			Review:        ReviewPolicy{Required: true, Reasons: []string{"evidenced_management_explanation", "assist_mode"}, Blocking: []string{"insufficient_evidence", "out_of_scope"}, Completion: []string{"sources_reviewed", "human_confirmation"}, AllowedRoles: []string{"reviewer", "approver"}, ConfirmAction: "confirm_explanation"},
		},
		{
			ID: "retail_operations", Version: "v1", Name: "零售经营分析 Agent", Priority: 70,
			Description:    "基于 retail-pulse-v1、retail-store-diagnostics-v1 和 retail-store-scenario-v1 提供可追溯的零售经营事实与行动提议。",
			IntentExamples: []string{"经营脉搏", "门店诊断", "分析 Store006", "人工下降 10% 情景", "生成行动提议"},
			// FIX-029: 「门店贡献」改名为「门店经营利润」是展示层改动，匹配词只增不减——
			// 老说法仍要能路由到本 skill，行业说法「四墙利润」一并收进来。
			// M6.1: natural-phrasing coverage — compound terms keep the deterministic
			// match from hijacking lease questions ("为什么" alone is too greedy;
			// "毛利为什么下滑" is not).
			MatchTerms:     []string{"经营脉搏", "门店诊断", "门店分析", "经营情景", "零售经营", "客流", "转化", "客单", "人工", "占用现金成本", "门店经营利润", "门店贡献", "四墙利润", "同群", "门店异常", "行动草稿", "毛利", "毛利下滑", "为什么下滑", "为何下滑", "为咩", "下滑原因", "闭店", "门店续租", "续租测算", "续租决策", "租金谈判", "retail operations", "store diagnostics", "operating pulse"},
			AllowedRoles:   []string{"admin", "editor", "reviewer", "approver", "auditor", "readonly"},
			RequiredInputs: []string{"message"}, RequiredContext: []string{"retail_filters"},
			AllowedTools: []string{"retail.operating_pulse.read", "retail.store_diagnostics.read", "retail.store.scenario.evaluate",
				// agent-universal-pagefill-v1 P0-A①：电商读类三件套 + 零售写类
				//（底稿生成、导入预填——pagefill 唯一生产者）。写类只落草稿层，
				// 本 skill 的 ReviewPolicy 已 Required。
				"retail.site_pulse.read", "retail.site_diagnostics.read", "retail.site.scenario.evaluate",
				"fpna.ecom_assumption.suggest",
				"retail.working_paper.store.generate", "retail.store_days.import.preview"},
			ArtifactTypes: []agentartifact.ArtifactType{agentartifact.ArtifactRetailActionProposal},
			Review:        ReviewPolicy{Required: true, Reasons: []string{"assist_mode", "retail_data_confirmation"}, Blocking: []string{"missing_context", "insufficient_evidence", "source_conflict"}, Completion: []string{"sources_reviewed", "scenario_confirmed_in_workbench"}, AllowedRoles: []string{"reviewer", "approver"}, ConfirmAction: "open_scenario_workbench"},
		},
	} {
		_ = registry.Register(definition)
	}
	return registry
}

func (r *Registry) Register(definition Definition) error {
	if r == nil {
		return fmt.Errorf("skill registry is nil")
	}
	if err := definition.Validate(); err != nil {
		return err
	}
	if r.definitions[definition.ID] == nil {
		r.definitions[definition.ID] = make(map[string]Definition)
	}
	if _, exists := r.definitions[definition.ID][definition.Version]; exists {
		return fmt.Errorf("skill %s@%s is already registered", definition.ID, definition.Version)
	}
	r.definitions[definition.ID][definition.Version] = definition
	return nil
}

func (r *Registry) Resolve(id, version string) (Definition, bool) {
	if r == nil {
		return Definition{}, false
	}
	id = strings.TrimSpace(id)
	requestedVersion := strings.TrimSpace(version)
	var candidates []Definition
	for canonical, versions := range r.definitions {
		if canonical != id && !anyAlias(versions, id) {
			continue
		}
		for _, definition := range versions {
			if requestedVersion == "" || definition.Version == requestedVersion {
				candidates = append(candidates, definition)
			}
		}
	}
	if len(candidates) == 0 {
		return Definition{}, false
	}
	// A pinned version is an immutable replay address. For discovery without a
	// version, choose the newest deterministic version so map iteration order
	// can never change a newly created Run's contract.
	sort.SliceStable(candidates, func(i, j int) bool {
		return compareVersions(candidates[i].Version, candidates[j].Version) > 0
	})
	return candidates[0], true
}

func (r *Registry) Select(intent Intent) (Definition, bool) {
	if requested := strings.TrimSpace(intent.RequestedSkillID); requested != "" {
		definition, ok := r.Resolve(requested, intent.RequestedVersion)
		if ok && strings.TrimSpace(intent.Role) != "" && !roleAllowed(definition.AllowedRoles, intent.Role) {
			return Definition{}, false
		}
		return definition, ok
	}
	candidates := make([]Definition, 0)
	for _, versions := range r.definitions {
		for _, definition := range versions {
			if definition.Matches(intent) {
				candidates = append(candidates, definition)
			}
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		// agent-universal-pagefill-v1 P0-A②（路由去遮蔽）：当问句明确是
		// 草稿/建议类请求时，声明了写工具的技能优先于只读技能——否则
		// Priority 最高的只读零售技能会压住唯一带写工具的 FP&A 技能，
		// 「生成行动草稿/假设建议」等请求实际不可达（FP&A 反馈 2026-08-27
		// §7.4.11 实测 4/4 失败）。判定是纯文本规则、确定性可复演。
		writeA, writeB := hasDraftCapability(candidates[i]), hasDraftCapability(candidates[j])
		if intentRequestsDraft(intent.Message) && writeA != writeB {
			return writeA
		}
		if candidates[i].Priority != candidates[j].Priority {
			return candidates[i].Priority > candidates[j].Priority
		}
		if candidates[i].ID != candidates[j].ID {
			return candidates[i].ID < candidates[j].ID
		}
		return compareVersions(candidates[i].Version, candidates[j].Version) > 0
	})
	if len(candidates) == 0 {
		return Definition{}, false
	}
	return candidates[0], true
}

// List returns public, versioned descriptors that the supplied role may use.
// Match terms are intentionally omitted from public descriptors so clients
// cannot treat the server's intent vocabulary as an authorization mechanism.
func (r *Registry) List(roles []string) []Definition {
	if r == nil {
		return nil
	}
	var result []Definition
	for _, versions := range r.definitions {
		for _, definition := range versions {
			if len(roles) > 0 && !anyRoleAllowed(definition.AllowedRoles, roles) {
				continue
			}
			result = append(result, definition.Public())
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].ID == result[j].ID {
			return result[i].Version < result[j].Version
		}
		return result[i].ID < result[j].ID
	})
	return result
}

func (d Definition) Matches(intent Intent) bool {
	if !roleAllowed(d.AllowedRoles, intent.Role) && strings.TrimSpace(intent.Role) != "" {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(intent.Message))
	for _, term := range d.MatchTerms {
		if strings.Contains(message, strings.ToLower(strings.TrimSpace(term))) {
			return true
		}
	}
	return false
}

func (d Definition) Validate() error {
	if strings.TrimSpace(d.ID) == "" || strings.TrimSpace(d.Version) == "" || strings.TrimSpace(d.Name) == "" || strings.TrimSpace(d.Description) == "" {
		return fmt.Errorf("skill id, version, name and description are required")
	}
	if len(d.AllowedTools) == 0 {
		return fmt.Errorf("skill %s must declare allowed tools", d.ID)
	}
	if len(d.ArtifactTypes) == 0 && d.ID != "contract_review" && d.ID != "audit_pack" {
		return fmt.Errorf("skill %s must declare artifact types", d.ID)
	}
	if !d.Review.Required {
		return fmt.Errorf("skill %s must declare a review policy", d.ID)
	}
	return nil
}

func (d Definition) Public() Definition {
	d.AllowedTools = append([]string(nil), d.AllowedTools...)
	d.IntentExamples = append([]string(nil), d.IntentExamples...)
	d.MatchTerms = nil
	return d
}

func anyAlias(versions map[string]Definition, wanted string) bool {
	for _, definition := range versions {
		if slices.ContainsFunc(definition.Aliases, func(v string) bool { return strings.EqualFold(strings.TrimSpace(v), strings.TrimSpace(wanted)) }) {
			return true
		}
	}
	return false
}

// compareVersions handles the vN and dotted numeric versions used by the
// Skill contract. Unknown suffixes fall back to lexical comparison, which is
// deterministic and keeps a future semantic version from silently becoming
// incomparable.
func compareVersions(left, right string) int {
	leftParts := versionParts(left)
	rightParts := versionParts(right)
	for index := 0; index < len(leftParts) || index < len(rightParts); index++ {
		leftValue, rightValue := 0, 0
		if index < len(leftParts) {
			leftValue = leftParts[index]
		}
		if index < len(rightParts) {
			rightValue = rightParts[index]
		}
		if leftValue != rightValue {
			if leftValue > rightValue {
				return 1
			}
			return -1
		}
	}
	if strings.TrimSpace(left) == strings.TrimSpace(right) {
		return 0
	}
	if strings.TrimSpace(left) > strings.TrimSpace(right) {
		return 1
	}
	return -1
}

func versionParts(version string) []int {
	version = strings.TrimSpace(version)
	if strings.HasPrefix(strings.ToLower(version), "v") {
		version = version[1:]
	}
	parts := strings.Split(version, ".")
	values := make([]int, 0, len(parts))
	for _, part := range parts {
		value := 0
		for _, character := range part {
			if character < '0' || character > '9' {
				break
			}
			value = value*10 + int(character-'0')
		}
		values = append(values, value)
	}
	return values
}

func anyRoleAllowed(allowed, roles []string) bool {
	for _, role := range roles {
		if roleAllowed(allowed, role) {
			return true
		}
	}
	return false
}

func roleAllowed(allowed []string, role string) bool {
	role = strings.ToLower(strings.TrimSpace(role))
	if role == "user" {
		role = "editor"
	}
	return slices.ContainsFunc(allowed, func(v string) bool { return strings.EqualFold(strings.TrimSpace(v), strings.TrimSpace(role)) })
}

// hasDraftCapability reports whether the skill declares any draft-level
// (write-class) tool. Tool names ending in ".draft.*" / ".suggest" /
// ".suggest_batch" / ".preview" / ".generate" follow the naming convention
// in AGENTS.md's tool taxonomy; the list is closed — an unknown suffix does
// NOT count as a write capability (fail-closed).
func hasDraftCapability(definition Definition) bool {
	for _, tool := range definition.AllowedTools {
		if isDraftToolName(tool) {
			return true
		}
	}
	return false
}

func isDraftToolName(name string) bool {
	name = strings.TrimSpace(name)
	for _, suffix := range []string{
		".draft.create", ".draft.generate", ".suggest", ".suggest_batch",
		".import.preview", ".paper.s1.generate", ".coa.suggest_template",
	} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return strings.Contains(name, ".working_paper.") && strings.HasSuffix(name, ".generate")
}

// intentRequestsDraft matches the deterministic phrasings of "produce a draft
// for me" requests. It must stay narrow: broad write detection would flip
// ordinary read questions toward skills they did not mean to address.
func intentRequestsDraft(message string) bool {
	message = strings.ToLower(strings.TrimSpace(message))
	for _, phrase := range []string{
		"生成草稿", "创建草稿", "起草", "生成行动草稿", "行动草稿",
		"提交假设建议", "假设建议草稿", "生成建议草稿", "写个草稿",
		"给我一份…草稿", "帮我生成草稿", "生成决策备忘录", "备忘录草稿",
		"draft a", "create a draft", "generate a draft",
	} {
		if strings.Contains(message, phrase) {
			return true
		}
	}
	return false
}
