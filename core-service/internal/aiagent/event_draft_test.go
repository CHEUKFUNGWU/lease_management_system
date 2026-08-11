package aiagent

import (
	"testing"

	"github.com/lease-management-system/core-service/internal/agentartifact"
)

func TestExtractEventDraftRequiresExplicitCompleteInputs(t *testing.T) {
	draft, evidence := extractEventDraft("请创建合同事件草稿：2026年8月10日租金变更，需复核", "contract-1", []Source{{
		Type: "contract", ID: "contract-1", Snippet: "合同编号 LEASE-001",
	}})
	if draft == nil || draft.EventType != "rent_change" || draft.EffectiveDate != "2026-08-10" {
		t.Fatalf("draft=%+v", draft)
	}
	if len(evidence) != 1 || !evidence[0].Complete {
		t.Fatalf("evidence=%+v", evidence)
	}

	if draft, _ := extractEventDraft("请创建合同事件草稿：租金变更", "contract-1", nil); draft != nil {
		t.Fatalf("incomplete date should not create draft: %+v", draft)
	}
	if draft, _ := extractEventDraft("请查询事件：2026-08-10租金变更", "contract-1", nil); draft != nil {
		t.Fatalf("read-only query should not create draft: %+v", draft)
	}
}

func TestProjectResultCreatesEventDraftArtifactWithReviewGate(t *testing.T) {
	result := ProjectResult(Response{
		Answer: "事件草稿", Model: "deterministic-event-parser",
		Sources: []Source{{Type: "contract", ID: "contract-1", Snippet: "合同记录"}},
		EventDraft: &EventDraftData{
			ContractID: "contract-1", EventType: "rent_change", EffectiveDate: "2026-08-10",
			ChangeReason: "租金变更", JudgmentBasis: "用户指令提取，需人工复核",
		},
	})
	if len(result.Artifacts) != 1 || result.Artifacts[0].Type != string(agentartifact.ArtifactEventDraft) {
		t.Fatalf("artifacts=%+v", result.Artifacts)
	}
	artifact := result.Artifacts[0]
	if !artifact.ReviewRequired || !artifact.EvidenceComplete || len(artifact.EvidenceRefs) != 1 {
		t.Fatalf("event artifact=%+v", artifact)
	}
}
