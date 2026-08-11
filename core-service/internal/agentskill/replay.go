package agentskill

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
)

// SkillContractFixture is the single versioned contract used by CI and the
// replay command. It describes the externally observable Skill surface rather
// than internal implementation details.
type SkillContractFixture struct {
	SchemaVersion string            `json:"schema_version"`
	Skills        []SkillContract   `json:"skills"`
	ReplayCases   []SkillReplayCase `json:"replay_cases"`
}

type SkillContract struct {
	ID             string   `json:"id"`
	Version        string   `json:"version"`
	AllowedTools   []string `json:"allowed_tools"`
	ArtifactTypes  []string `json:"artifact_types"`
	ReviewRequired bool     `json:"review_required"`
	ConfirmAction  string   `json:"confirm_action"`
}

type SkillReplayCase struct {
	ID             string   `json:"id"`
	SkillID        string   `json:"skill_id"`
	SkillVersion   string   `json:"skill_version"`
	Role           string   `json:"role"`
	ExpectedTools  []string `json:"expected_tools"`
	ExpectedAssets []string `json:"expected_artifact_types"`
}

//go:embed testdata/skill-contract-replay.v1.json
var skillContractReplayFixture []byte

func ProductionSkillContractFixture() (SkillContractFixture, error) {
	var fixture SkillContractFixture
	if err := json.Unmarshal(skillContractReplayFixture, &fixture); err != nil {
		return SkillContractFixture{}, fmt.Errorf("decode skill contract replay fixture: %w", err)
	}
	if fixture.SchemaVersion == "" || len(fixture.Skills) == 0 || len(fixture.ReplayCases) == 0 {
		return SkillContractFixture{}, fmt.Errorf("skill contract replay fixture is incomplete")
	}
	return fixture, nil
}

// ValidateSkillContractFixture checks both the public contract and the
// version-pinned replay address. A historical Run must pass its stored
// version explicitly; a newer version may coexist without changing it.
func ValidateSkillContractFixture(registry *Registry, fixture SkillContractFixture) error {
	if registry == nil {
		return fmt.Errorf("skill registry is nil")
	}
	for _, expected := range fixture.Skills {
		definition, ok := registry.Resolve(expected.ID, expected.Version)
		if !ok {
			return fmt.Errorf("skill %s@%s is not registered", expected.ID, expected.Version)
		}
		if !sameStrings(definition.AllowedTools, expected.AllowedTools) {
			return fmt.Errorf("skill %s@%s allowed tools drifted", expected.ID, expected.Version)
		}
		actualArtifacts := make([]string, 0, len(definition.ArtifactTypes))
		for _, artifact := range definition.ArtifactTypes {
			actualArtifacts = append(actualArtifacts, string(artifact))
		}
		if !sameStrings(actualArtifacts, expected.ArtifactTypes) ||
			definition.Review.Required != expected.ReviewRequired ||
			definition.Review.ConfirmAction != expected.ConfirmAction {
			return fmt.Errorf("skill %s@%s review/artifact contract drifted", expected.ID, expected.Version)
		}
	}
	for _, replay := range fixture.ReplayCases {
		definition, ok := registry.Resolve(replay.SkillID, replay.SkillVersion)
		if !ok {
			return fmt.Errorf("replay case %s cannot resolve %s@%s", replay.ID, replay.SkillID, replay.SkillVersion)
		}
		if !sameStrings(definition.AllowedTools, replay.ExpectedTools) {
			return fmt.Errorf("replay case %s changed allowed tools", replay.ID)
		}
		actualArtifacts := make([]string, 0, len(definition.ArtifactTypes))
		for _, artifact := range definition.ArtifactTypes {
			actualArtifacts = append(actualArtifacts, string(artifact))
		}
		if !sameStrings(actualArtifacts, replay.ExpectedAssets) {
			return fmt.Errorf("replay case %s changed artifact types", replay.ID)
		}
		if replay.Role != "" && !roleAllowed(definition.AllowedRoles, replay.Role) {
			return fmt.Errorf("replay case %s is no longer available to role %s", replay.ID, replay.Role)
		}
	}
	return nil
}

func sameStrings(left, right []string) bool {
	left = append([]string(nil), left...)
	right = append([]string(nil), right...)
	sort.Strings(left)
	sort.Strings(right)
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
