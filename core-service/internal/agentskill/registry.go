// Package agentskill contains the versioned, server-owned Skill Registry.
// Skills describe intent, required context, tool allowlists and review gates;
// they never contain repository or transport access.
package agentskill

import (
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
			AllowedTools:  []string{"lease.file.parse_contract_batch", "lease.contract.draft.create"},
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
			AllowedTools:  []string{"lease.portfolio.summary", "lease.management.pre_read", "lease.store.performance", "lease.rent_to_sales", "lease.equipment.performance", "lease.fpna.actions", "lease.close.readiness", "lease.budget.variance", "lease.cashflow.scenario", "lease.renewal.decisions", "lease.store.scenario.simulate", "lease.equipment.scenario.simulate", "lease.deal.simulate", "lease.predeal.simulate", "lease.renewal.simulate", "lease.decision.summary", "lease.fpna.action.draft.create", "lease.explanation.draft.create", "lease.meeting.action.draft.create", "lease.decision.memo.draft.create", "lease.fpna.scenario.draft.create", "lease.contract.search"},
			ArtifactTypes: []agentartifact.ArtifactType{agentartifact.ArtifactReportExplanation},
			Review:        ReviewPolicy{Required: true, Reasons: []string{"evidenced_management_explanation", "assist_mode"}, Blocking: []string{"insufficient_evidence", "out_of_scope"}, Completion: []string{"sources_reviewed", "human_confirmation"}, AllowedRoles: []string{"reviewer", "approver"}, ConfirmAction: "confirm_explanation"},
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

func containsFold(values []string, wanted string) bool {
	wanted = strings.ToLower(strings.TrimSpace(wanted))
	if wanted == "" {
		return false
	}
	for _, value := range values {
		if strings.ToLower(strings.TrimSpace(value)) == wanted {
			return true
		}
	}
	return false
}

func anyAlias(versions map[string]Definition, wanted string) bool {
	for _, definition := range versions {
		if containsFold(definition.Aliases, wanted) {
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
	return containsFold(allowed, role)
}
