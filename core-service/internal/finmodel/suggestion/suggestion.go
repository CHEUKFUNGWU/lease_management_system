// Package suggestion is SM5: the AI assumption-suggestion write path. AI
// produces drafts, never approvals; a draft without evidence is rejected
// structurally (an empty Basis cannot pass SaveDrafts) — the "无依据不建议"
// rule is a type-level check, not a prompt.
package suggestion

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
)

// EvidenceRef cites one supporting read: the tool call, data scope and
// period that justify the suggested value.
type EvidenceRef struct {
	ToolCallID string `json:"tool_call_id"`
	Scope      string `json:"scope"`
	Period     string `json:"period,omitempty"`
}

// SuggestionDraft is one AI-drafted assumption value.
type SuggestionDraft struct {
	AssumptionKey string        `json:"assumption_key"`
	Category      string        `json:"category"`
	Value         json.RawMessage `json:"value"`
	Unit          string        `json:"unit,omitempty"`
	Basis         []EvidenceRef `json:"basis"`
	Confidence    float64       `json:"confidence"`
	SourceTag     string        `json:"source_tag"` // always "ai_suggestion"
}

// ErrMissingBasis is the structural refusal: no evidence, no draft.
var ErrMissingBasis = errors.New("suggestion: drafts without basis are refused (无依据不建议)")

// Validate enforces the draft contract before any write.
func (d SuggestionDraft) Validate() error {
	if strings.TrimSpace(d.AssumptionKey) == "" {
		return errors.New("suggestion: assumption_key is required")
	}
	if d.SourceTag != "" && d.SourceTag != "ai_suggestion" {
		return errors.New("suggestion: source must be ai_suggestion")
	}
	if len(d.Basis) == 0 {
		return ErrMissingBasis
	}
	if d.Confidence < 0 || d.Confidence > 1 {
		return errors.New("suggestion: confidence must be within [0,1]")
	}
	for _, ref := range d.Basis {
		if strings.TrimSpace(ref.ToolCallID) == "" {
			return errors.New("suggestion: every basis ref needs a tool call id")
		}
	}
	return nil
}

// Store is the persistence seam; two adapters exist (Postgres in
// repository, memory here). Only drafts are ever written through it — the
// approved path is a human-only flow elsewhere.
type Store interface {
	SaveDrafts(ctx context.Context, legalEntityID string, drafts []SuggestionDraft, idempotencyKey string) ([]string, error)
}

// MemoryStore is the in-memory adapter for tests.
type MemoryStore struct {
	LegalEntityID string
	Drafts        []storedDraft
}

type storedDraft struct {
	SuggestionDraft
	ID             string
	LegalEntityID  string
	IdempotencyKey string
	Status         string
}

// NewMemoryStore builds an empty in-memory store.
func NewMemoryStore(legalEntityID string) *MemoryStore {
	return &MemoryStore{LegalEntityID: legalEntityID}
}

// SaveDrafts validates each draft (basis rule included) and stores them as
// draft-status records keyed by the idempotency key.
func (m *MemoryStore) SaveDrafts(ctx context.Context, legalEntityID string, drafts []SuggestionDraft, idempotencyKey string) ([]string, error) {
	_ = ctx
	ids := make([]string, 0, len(drafts))
	for i, draft := range drafts {
		if err := draft.Validate(); err != nil {
			return nil, err
		}
		id := "draft-" + idempotencyKey + "-" + string(rune('a'+i))
		m.Drafts = append(m.Drafts, storedDraft{
			SuggestionDraft: draft, ID: id, LegalEntityID: legalEntityID,
			IdempotencyKey: idempotencyKey, Status: "draft",
		})
		d := draft
		_ = d
		ids = append(ids, id)
	}
	return ids, nil
}

// List returns stored drafts for assertions.
func (m *MemoryStore) List() []storedDraft { return m.Drafts }
