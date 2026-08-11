package agentartifact

import (
	"encoding/json"
	"testing"
)

func TestNormalizeRequiresCompleteEvidenceLocators(t *testing.T) {
	_, err := Normalize(Artifact{
		ArtifactType: ArtifactContractDraft, Title: "合同草稿", Data: json.RawMessage(`{}`),
		EvidenceComplete: true,
		EvidenceRefs:     []EvidenceReference{{SourceFileID: "file-1", Complete: true}},
	})
	if err == nil {
		t.Fatal("expected complete evidence without locators to be rejected")
	}
	artifact, err := Normalize(Artifact{
		ArtifactType: ArtifactContractDraft, Title: "合同草稿", Data: json.RawMessage(`{}`),
		EvidenceComplete: true,
		EvidenceRefs:     []EvidenceReference{{SourceFileID: "file-1", Complete: true, Locators: []EvidenceLocator{{Field: "contracts[0]", Source: "Sheet1!A2"}}}},
	})
	if err != nil || artifact.SchemaVersion != SchemaVersion || artifact.Status != "ready" {
		t.Fatalf("artifact=%+v err=%v", artifact, err)
	}
}

func TestIncompleteEvidenceRequiresReason(t *testing.T) {
	_, err := Normalize(Artifact{
		ArtifactType: ArtifactPaymentScheduleDraft, Title: "付款计划", Data: json.RawMessage(`{}`),
		EvidenceRefs: []EvidenceReference{{SourceFileID: "file-1", Complete: false}},
	})
	if err == nil {
		t.Fatal("expected incomplete evidence reason validation")
	}
}

func TestNormalizeRejectsUnknownArtifactType(t *testing.T) {
	_, err := Normalize(Artifact{ArtifactType: "unsafe_type", Title: "bad", Data: json.RawMessage(`{}`)})
	if err == nil {
		t.Fatal("expected unknown artifact type to be rejected")
	}
}
