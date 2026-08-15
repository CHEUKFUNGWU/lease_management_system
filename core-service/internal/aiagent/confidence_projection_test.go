package aiagent

import (
	"testing"
)

// FIX-001 F3: ProjectResult carries the answer confidence and a derived
// degradation reason into the persisted message projection.
func TestProjectResultCarriesConfidenceAndReason(t *testing.T) {
	// A fallback answer explains its own low confidence.
	fallback := ProjectResult(Response{Answer: "摘要", Model: "fallback", Confidence: 0.5})
	if fallback.Confidence == nil || *fallback.Confidence != 0.5 {
		t.Fatalf("fallback confidence = %v, want 0.5", fallback.Confidence)
	}
	if fallback.ConfidenceReason == nil || *fallback.ConfidenceReason != "AI 服务暂不可用，以下为系统数据摘要" {
		t.Fatalf("fallback reason = %v", fallback.ConfidenceReason)
	}

	// Review prompts are a degradation signal: the answer needs human review.
	review := ProjectResult(Response{
		Answer: "草稿已生成", Model: "gpt-4o-mini", Confidence: 0.9,
		ReviewPrompts: []AgentReviewPrompt{{ID: "low_confidence", Title: "低置信度", Severity: "warning", Action: "复核"}},
	})
	if review.Confidence == nil || *review.Confidence != 0.9 {
		t.Fatalf("review confidence = %v", review.Confidence)
	}
	if review.ConfidenceReason == nil || *review.ConfidenceReason != "部分内容需人工复核" {
		t.Fatalf("review reason = %v", review.ConfidenceReason)
	}

	// A normal answer carries its confidence without inventing a reason.
	normal := ProjectResult(Response{Answer: "正常回答", Model: "gpt-4o-mini", Confidence: 0.9})
	if normal.Confidence == nil || *normal.Confidence != 0.9 {
		t.Fatalf("normal confidence = %v", normal.Confidence)
	}
	if normal.ConfidenceReason != nil {
		t.Fatalf("normal answer must not invent a degradation reason, got %v", normal.ConfidenceReason)
	}

	// Zero or absent confidence stays absent — never fabricated.
	none := ProjectResult(Response{Answer: "无置信度", Model: "gpt-4o-mini"})
	if none.Confidence != nil {
		t.Fatalf("absent confidence must stay absent, got %v", none.Confidence)
	}
}
