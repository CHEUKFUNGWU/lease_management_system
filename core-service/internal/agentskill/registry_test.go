package agentskill

import (
	_ "embed"
	"encoding/json"
	"sort"
	"strings"
	"testing"
)

//go:embed testdata/skill-contracts.v1.json
var skillContractFixture []byte

func TestProductionSkillsMatchVersionedContractFixture(t *testing.T) {
	var contracts []struct {
		ID             string   `json:"id"`
		Version        string   `json:"version"`
		AllowedTools   []string `json:"allowed_tools"`
		ArtifactTypes  []string `json:"artifact_types"`
		ReviewRequired bool     `json:"review_required"`
		ConfirmAction  string   `json:"confirm_action"`
	}
	if err := json.Unmarshal(skillContractFixture, &contracts); err != nil {
		t.Fatal(err)
	}
	registry := ProductionRegistry()
	if len(registry.List(nil)) != len(contracts) {
		t.Fatalf("production skill count=%d fixture count=%d", len(registry.List(nil)), len(contracts))
	}
	for _, contract := range contracts {
		t.Run(contract.ID+"@"+contract.Version, func(t *testing.T) {
			definition, ok := registry.Resolve(contract.ID, contract.Version)
			if !ok {
				t.Fatalf("skill is not registered")
			}
			tools := append([]string(nil), definition.AllowedTools...)
			sort.Strings(tools)
			expectedTools := append([]string(nil), contract.AllowedTools...)
			sort.Strings(expectedTools)
			if stringSliceKey(tools) != stringSliceKey(expectedTools) {
				t.Fatalf("allowed tools=%v, want=%v", tools, expectedTools)
			}
			artifacts := make([]string, 0, len(definition.ArtifactTypes))
			for _, artifact := range definition.ArtifactTypes {
				artifacts = append(artifacts, string(artifact))
			}
			sort.Strings(artifacts)
			expectedArtifacts := append([]string(nil), contract.ArtifactTypes...)
			sort.Strings(expectedArtifacts)
			if stringSliceKey(artifacts) != stringSliceKey(expectedArtifacts) {
				t.Fatalf("artifact types=%v, want=%v", artifacts, expectedArtifacts)
			}
			if definition.Review.Required != contract.ReviewRequired || definition.Review.ConfirmAction != contract.ConfirmAction {
				t.Fatalf("review policy=%+v", definition.Review)
			}
		})
	}
}

func stringSliceKey(values []string) string {
	return strings.Join(values, "\x00")
}

func TestProductionRegistrySelectsSkillsByIntentAndRole(t *testing.T) {
	registry := ProductionRegistry()

	definition, ok := registry.Select(Intent{Message: "请生成审计包，包含披露核对清单", Role: "auditor"})
	if !ok || definition.ID != "audit_pack" || definition.Version != "v1" {
		t.Fatalf("audit skill = %+v, ok=%v", definition, ok)
	}

	definition, ok = registry.Select(Intent{Message: "导入租金表", Role: "editor"})
	if !ok || definition.ID != "payment_schedule" {
		t.Fatalf("payment skill = %+v, ok=%v", definition, ok)
	}

	if _, ok := registry.Select(Intent{Message: "生成审计包", Role: "editor"}); ok {
		t.Fatal("editor must not select auditor-only audit pack skill")
	}
}

func TestPinnedSkillContractReplaySurvivesNewerVersion(t *testing.T) {
	fixture, err := ProductionSkillContractFixture()
	if err != nil {
		t.Fatal(err)
	}
	registry := ProductionRegistry()
	base, ok := registry.Resolve("contract_review", "v1")
	if !ok {
		t.Fatal("contract_review v1 is not registered")
	}
	newer := base
	newer.Version = "v2"
	newer.AllowedTools = append(append([]string(nil), newer.AllowedTools...), "lease.report.preview")
	if err := registry.Register(newer); err != nil {
		t.Fatal(err)
	}
	if err := ValidateSkillContractFixture(registry, fixture); err != nil {
		t.Fatal(err)
	}

	pinned, ok := registry.Resolve("contract_review", "v1")
	if !ok || pinned.Version != "v1" || len(pinned.AllowedTools) != len(base.AllowedTools) {
		t.Fatalf("pinned replay resolved=%+v", pinned)
	}
	latest, ok := registry.Resolve("contract_review", "")
	if !ok || latest.Version != "v2" {
		t.Fatalf("unversioned discovery=%+v", latest)
	}
}

func TestProductionSkillContractFixtureIsValid(t *testing.T) {
	fixture, err := ProductionSkillContractFixture()
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateSkillContractFixture(ProductionRegistry(), fixture); err != nil {
		t.Fatal(err)
	}
}

func TestRegistryResolvesSkillAliasAndHidesMatchTerms(t *testing.T) {
	registry := ProductionRegistry()
	definition, ok := registry.Resolve("payment_schedule_intake", "v1")
	if !ok || definition.ID != "payment_schedule" {
		t.Fatalf("alias resolution = %+v, ok=%v", definition, ok)
	}
	public := definition.Public()
	if len(public.MatchTerms) != 0 || len(public.AllowedTools) == 0 {
		t.Fatalf("public skill descriptor leaked or lost tools: %+v", public)
	}
}

func TestExplicitSkillSelectionCannotBypassRoleRestriction(t *testing.T) {
	if _, ok := ProductionRegistry().Select(Intent{RequestedSkillID: "audit_pack", RequestedVersion: "v1", Role: "editor"}); ok {
		t.Fatal("explicit skill selection must still enforce allowed roles")
	}
}

// M6.1: natural phrasing routes to the retail skill without keyword
// memorization, while a bare "为什么" does NOT hijack lease questions.
func TestRetailSkillNaturalPhrasingRouting(t *testing.T) {
	registry := ProductionRegistry()
	for _, message := range []string{
		"A 门店毛利为什么下滑",
		"Store006 毛利下滑的原因",
		"帮我看看这个月哪些门店同群掉队",
		"北区闭店影响有多大",
		"给我做一下续租测算",
	} {
		definition, ok := registry.Select(Intent{Message: message, Role: "editor"})
		if !ok || definition.ID != "retail_operations" {
			t.Fatalf("%q routed to %+v ok=%v", message, definition, ok)
		}
	}
	// A bare causal question without retail markers must not be captured by
	// the retail skill.
	if definition, ok := registry.Select(Intent{Message: "为什么租赁负债上升了", Role: "editor"}); ok && definition.ID == "retail_operations" {
		t.Fatalf("lease why-question captured by retail skill: %+v", definition)
	}
}

// agent-universal-pagefill-v1 P0-A②：写意图去遮蔽。FP&A 反馈 2026-08-27
// §7.4.11 实测：含零售词（门店/毛利/人工成本率）的写请求被 Priority 70 的
// 只读 retail_operations 压住，唯一带写工具的 fpna_copilot（Priority 45）
// 永远选不中。修法是确定性的 tie-break：问句明确要草稿时，声明了写工具的
// 技能优先。
func TestSelectPrefersDraftCapableSkillOnDraftIntent(t *testing.T) {
	registry := ProductionRegistry()
	cases := []string{
		"请为经营利润下滑的门店生成行动草稿",
		"基于当前毛利表现帮我生成草稿",
		"针对人工成本率偏高的门店起草改进方案",
		"生成决策备忘录：门店经营利润连续下滑",
	}
	for _, message := range cases {
		definition, ok := registry.Select(Intent{Message: message, Role: "editor"})
		if !ok {
			t.Fatalf("%q matched no skill", message)
		}
		if !hasDraftCapability(definition) {
			t.Fatalf("%q routed to %s which has no draft tools", message, definition.ID)
		}
	}
	// 反向守卫：普通只读问句不受偏置影响——"哪些门店需要关注"仍走
	// retail_operations（确定性事实管道），不能被拉进写技能。
	readDefinition, ok := registry.Select(Intent{Message: "当前经营脉搏中哪些门店需要关注？", Role: "readonly"})
	if !ok || readDefinition.ID != "retail_operations" {
		t.Fatalf("plain read question changed routing: %v %v", readDefinition.ID, ok)
	}
}
