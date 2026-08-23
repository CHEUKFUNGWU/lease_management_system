// Package agentartifact defines the versioned Artifact/Evidence contract
// shared by AI intake, the Core Agent Runtime, Web review actions and future
// Pi-like Agent clients.
package agentartifact

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const SchemaVersion = "agent-artifact.v1"

type ArtifactType string

const (
	ArtifactContractDraft        ArtifactType = "contract_draft"
	ArtifactPaymentScheduleDraft ArtifactType = "payment_schedule_draft"
	ArtifactEventDraft           ArtifactType = "event_draft"
	ArtifactAuditPack            ArtifactType = "audit_pack"
	ArtifactDataQualityIssues    ArtifactType = "data_quality_issue_list"
	ArtifactReportExplanation    ArtifactType = "report_explanation"
	ArtifactMonthlyCloseBlockers ArtifactType = "monthly_close_blockers"
	ArtifactRetailActionProposal ArtifactType = "retail_action_proposal"
	ArtifactWorkingPaper          ArtifactType = "working_paper"
	ArtifactPageFill              ArtifactType = "page_fill"
	// ArtifactChartSVG 是 Ch1 的确定性图表 artifact（D-B0/D-B1）：Data 携带
	// chart_svg / decomposition_order / data_classification。前端渲染前必须
	// 经 sanitizeSvg 白名单消毒。
	ArtifactChartSVG    ArtifactType = "chart_svg"
	ArtifactGeneric     ArtifactType = "generic"
)

type EvidenceLocator struct {
	Field       string    `json:"field"`
	Source      string    `json:"source"`
	Page        *int      `json:"page,omitempty"`
	Sheet       string    `json:"sheet,omitempty"`
	CellRange   string    `json:"cell_range,omitempty"`
	Coordinates []float64 `json:"coordinates,omitempty"`
	FileHash    string    `json:"file_hash,omitempty"`
	Quote       string    `json:"quote,omitempty"`
}

type EvidenceReference struct {
	ReferenceID   string            `json:"reference_id,omitempty"`
	SourceFileID  string            `json:"source_file_id,omitempty"`
	ObjectName    string            `json:"object_name,omitempty"`
	ContentType   string            `json:"content_type,omitempty"`
	FileHash      string            `json:"file_hash,omitempty"`
	Locators      []EvidenceLocator `json:"locators,omitempty"`
	Complete      bool              `json:"complete"`
	MissingReason string            `json:"missing_reason,omitempty"`
}

func (e EvidenceReference) Validate() error {
	if strings.TrimSpace(e.SourceFileID) == "" && strings.TrimSpace(e.ReferenceID) == "" {
		return errors.New("evidence reference requires source_file_id or reference_id")
	}
	if e.Complete && len(e.Locators) == 0 {
		return errors.New("complete evidence requires at least one locator")
	}
	if !e.Complete && strings.TrimSpace(e.MissingReason) == "" {
		return errors.New("incomplete evidence requires missing_reason")
	}
	for index, locator := range e.Locators {
		if strings.TrimSpace(locator.Field) == "" || strings.TrimSpace(locator.Source) == "" {
			return fmt.Errorf("evidence locator %d requires field and source", index)
		}
	}
	return nil
}

type Artifact struct {
	SchemaVersion    string              `json:"schema_version"`
	ArtifactType     ArtifactType        `json:"artifact_type"`
	Title            string              `json:"title"`
	Status           string              `json:"status"`
	Data             json.RawMessage     `json:"data"`
	Actions          json.RawMessage     `json:"actions,omitempty"`
	EvidenceRefs     []EvidenceReference `json:"evidence_refs"`
	EvidenceComplete bool                `json:"evidence_complete"`
	ReviewRequired   bool                `json:"review_required"`
	ReviewReasons    []string            `json:"review_reasons"`
	ModelVersion     string              `json:"model_version,omitempty"`
	RuleVersion      string              `json:"rule_version,omitempty"`
}

func (a Artifact) Validate() error {
	if strings.TrimSpace(a.SchemaVersion) == "" {
		return errors.New("artifact schema_version is required")
	}
	if strings.TrimSpace(string(a.ArtifactType)) == "" || strings.TrimSpace(a.Title) == "" {
		return errors.New("artifact type and title are required")
	}
	if !knownArtifactType(a.ArtifactType) {
		return fmt.Errorf("unknown artifact type %q", a.ArtifactType)
	}
	if strings.TrimSpace(a.Status) == "" {
		return errors.New("artifact status is required")
	}
	if len(a.Data) == 0 || !json.Valid(a.Data) {
		return errors.New("artifact data must be valid JSON")
	}
	if a.EvidenceComplete && len(a.EvidenceRefs) == 0 {
		return errors.New("evidence_complete requires evidence_refs")
	}
	for index, evidence := range a.EvidenceRefs {
		if err := evidence.Validate(); err != nil {
			return fmt.Errorf("evidence_refs[%d]: %w", index, err)
		}
	}
	return nil
}

func knownArtifactType(artifactType ArtifactType) bool {
	switch artifactType {
	case ArtifactContractDraft, ArtifactPaymentScheduleDraft, ArtifactEventDraft,
		ArtifactAuditPack, ArtifactDataQualityIssues, ArtifactReportExplanation,
		ArtifactMonthlyCloseBlockers, ArtifactRetailActionProposal, ArtifactWorkingPaper,
		ArtifactChartSVG, ArtifactPageFill, ArtifactGeneric:
		return true
	default:
		return false
	}
}

func Normalize(a Artifact) (Artifact, error) {
	if strings.TrimSpace(a.SchemaVersion) == "" {
		a.SchemaVersion = SchemaVersion
	}
	if strings.TrimSpace(a.Status) == "" {
		a.Status = "ready"
	}
	if a.EvidenceRefs == nil {
		a.EvidenceRefs = []EvidenceReference{}
	}
	if a.ReviewReasons == nil {
		a.ReviewReasons = []string{}
	}
	if err := a.Validate(); err != nil {
		return Artifact{}, err
	}
	return a, nil
}
