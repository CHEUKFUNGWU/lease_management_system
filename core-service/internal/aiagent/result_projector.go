package aiagent

// C1（架构重构任务书 2026-08-26）步骤 3：结果投影是无 IO 的纯函数群——
// Response → aichat.Result 的全部映射、数据质量 Artifact 与复核理由的
// 推导都在这里，从 agent.go 单体中搬出，便于独立单测。逻辑零改动。

import (
	"strings"
	"time"

	"github.com/lease-management-system/core-service/internal/agentartifact"
	"github.com/lease-management-system/core-service/internal/aichat"
	"github.com/lease-management-system/core-service/internal/workingpaper"
)

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
		MeasuredTokens:   response.MeasuredTokens,
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
	if response.ProfitWaterfall != nil {
		result.Artifacts = append(result.Artifacts, aichat.ArtifactDraft{
			Type:          string(agentartifact.ArtifactChartSVG),
			Title:         "利润差异瀑布图",
			SchemaVersion: agentartifact.SchemaVersion,
			Data: map[string]any{
				"chart_svg":           response.ProfitWaterfall.SVG,
				"decomposition_order": response.ProfitWaterfall.DecompositionOrder,
				"data_classification": response.ProfitWaterfall.DataClassification,
				"status":              response.ProfitWaterfall.Status,
				"currency":            response.ProfitWaterfall.Currency,
			},
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
		// 卡片标题跟目标页走：合同工作台的付款计划预填不能冒充零售导入。
		title := "零售导入预填"
		if response.PageFill.TargetPage == "contract-workspace" {
			title = "付款计划预填"
		}
		result.Artifacts = append(result.Artifacts, aichat.ArtifactDraft{
			Type: string(agentartifact.ArtifactPageFill), Title: title,
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
