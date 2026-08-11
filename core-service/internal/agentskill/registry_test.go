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
