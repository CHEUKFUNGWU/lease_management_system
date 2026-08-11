package aiintake

import (
	"strings"
	"testing"
)

func TestDecodeEventAcceptsIncompleteAssistDraft(t *testing.T) {
	input := `{
  "schema_version":"ai-intake.v1", "intake_id":"intake-1", "task_id":"task-1", "file_id":"file-1",
  "mode":"assist", "draft_type":"event_draft", "status":"draft_generated",
  "confidence_scores":{"overall":0.42}, "missing_fields":["effective_date"],
  "warnings":["需要人工核对扫描件"], "requires_human_confirmation":true,
  "evidence":{"source_file_id":"file-1","object_name":"event/notice.pdf","content_type":"application/pdf","locators":[],"complete":false,"missing_reason":"no coordinates"},
  "review_gate":{"required":true,"reasons":["assist_mode","evidence_incomplete"],"confidence_threshold":0.8},
  "event":{"contract_id":"contract-1","event_type":"modification","effective_date":"","change_reason":"租金调整","judgment_basis":"补充协议","revision_parameters":{},"field_confidence":{"event_type":0.8}}
}`
	draft, err := DecodeEvent(strings.NewReader(input))
	if err != nil {
		t.Fatalf("DecodeEvent() error = %v", err)
	}
	if draft.Event.ContractID != "contract-1" || draft.Event.EffectiveDate != "" {
		t.Fatalf("event = %+v", draft.Event)
	}
}

func TestDecodeEventRejectsInvalidDateAndConfidence(t *testing.T) {
	input := `{
  "schema_version":"ai-intake.v1", "intake_id":"intake-1", "task_id":"task-1", "file_id":"file-1",
  "mode":"assist", "draft_type":"event_draft", "status":"draft_generated",
  "confidence_scores":{"overall":0.9}, "warnings":["review"], "requires_human_confirmation":true,
  "evidence":{"source_file_id":"file-1","object_name":"event/notice.pdf","content_type":"application/pdf","locators":[],"complete":false,"missing_reason":"no coordinates"},
  "review_gate":{"required":true,"reasons":["assist_mode"],"confidence_threshold":0.8},
  "event":{"effective_date":"2025-99-01","field_confidence":{"event_type":1.2}}
}`
	if _, err := DecodeEvent(strings.NewReader(input)); err == nil {
		t.Fatal("DecodeEvent() accepted invalid event")
	}
}
