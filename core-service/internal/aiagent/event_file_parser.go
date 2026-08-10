package aiagent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/lease-management-system/core-service/internal/agentartifact"
	"github.com/lease-management-system/core-service/internal/aiintake"
)

// EventFileParseResult is deliberately a draft-shaped response. It contains
// no approved event status and no calculated accounting adjustment.
type EventFileParseResult struct {
	SummaryText   string                            `json:"summary_text"`
	Sources       []Source                          `json:"sources"`
	Confidence    float64                           `json:"confidence"`
	MissingFields []string                          `json:"missing_fields"`
	Warnings      []string                          `json:"warnings"`
	ReviewPrompts []AgentReviewPrompt               `json:"review_prompts"`
	Event         aiintake.EventDraftData           `json:"event"`
	EvidenceRefs  []agentartifact.EvidenceReference `json:"evidence_refs"`
	IntakeID      string                            `json:"intake_id"`
	SchemaVersion string                            `json:"schema_version"`
}

func (h *Agent) parseEvent(ctx context.Context, authHeader, fileID, objectName, contentType, contractID string) (*EventFileParseResult, error) {
	aiServiceURL := os.Getenv("AI_SERVICE_URL")
	if aiServiceURL == "" {
		aiServiceURL = "http://ai-service:8000"
	}
	reqBody, err := json.Marshal(map[string]string{
		"file_id": fileID, "object_name": objectName, "content_type": contentType,
		"contract_id": contractID, "mode": "assist",
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", aiServiceURL+"/api/v1/parse/event", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader)
	resp, err := (&http.Client{Timeout: 180 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return nil, readErr
		}
		return nil, fmt.Errorf("AI service returned %d: %s", resp.StatusCode, string(body))
	}
	intake, err := aiintake.DecodeEvent(resp.Body)
	if err != nil {
		return nil, err
	}
	if err := validateAIIntakeSource(intake.IntakeMetadata, fileID, objectName, contentType); err != nil {
		return nil, err
	}
	warnings := append([]string(nil), intake.Warnings...)
	warnings = append(warnings, "事件草稿必须人工核对原文、事件分类、有效日期和变更参数")
	summary := fmt.Sprintf("已从 %s 生成事件草稿；缺失字段 %d 项，结果只处于 Assist Mode。", objectName, len(intake.MissingFields))
	return &EventFileParseResult{
		SummaryText: summary, Confidence: intake.ConfidenceScores["overall"],
		MissingFields: intake.MissingFields, Warnings: warnings,
		Sources: []Source{{Type: "file", ID: intake.Evidence.SourceFileID, Title: "合同事件文件", Snippet: intake.Evidence.ObjectName}},
		Event:   intake.Event, IntakeID: intake.IntakeID, SchemaVersion: intake.SchemaVersion,
		EvidenceRefs: []agentartifact.EvidenceReference{evidenceReferenceFromIntake(intake.Evidence)},
		ReviewPrompts: []AgentReviewPrompt{{
			ID: "event_document_review", Title: "复核合同事件文档",
			Description: "请核对原文页码/坐标、事件类型、有效日期、原值/新值和判断依据；确认后仍需走事件审批。",
			Severity:    "critical", Action: "review_ai_draft",
		}},
	}, nil
}
