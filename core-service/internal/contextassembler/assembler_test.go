package contextassembler

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agentcontext"
	"github.com/lease-management-system/core-service/internal/agenttools"
)

// ── fakes ────────────────────────────────────────────────────────────────────

type fakeHistory struct {
	mu   sync.Mutex
	rows map[string]Message // keyed by Ref
	list []Message          // read order
}

func newFakeHistory(rows ...Message) *fakeHistory {
	f := &fakeHistory{rows: map[string]Message{}}
	for _, r := range rows {
		f.rows[r.Ref] = r
		f.list = append(f.list, r)
	}
	return f
}

func (f *fakeHistory) Read(_ context.Context, _ agentcontext.ContextKey) ([]Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]Message, len(f.list))
	copy(out, f.list)
	return out, nil
}

// resolveAll asserts every ref resolves back to the stored rows — the
// executable form of "compaction deleted nothing" (D-C16).
func (f *fakeHistory) resolveAll(refs []MessageRef) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, ref := range refs {
		if _, ok := f.rows[ref.Ref]; !ok {
			return errors.New("unresolvable ref: " + ref.Ref)
		}
	}
	return nil
}

type countingSummarizer struct {
	got    []Message
	called int
	text   string
}

func (s *countingSummarizer) Summarize(_ context.Context, dropped []Message) (string, error) {
	s.called++
	s.got = dropped
	return s.text, nil
}

func mustKeyAR3(t *testing.T) agentcontext.ContextKey {
	t.Helper()
	key, err := agentcontext.KeyFrom(agenttools.Principal{
		UserID: "user-1",
		Scope:  accessScopeForAR3("entity-a"),
	}, "session-1", "production")
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func accessScopeForAR3(entity string) access.Scope { return access.Scope{LegalEntityID: entity} }

func newTestAssembler(t *testing.T, hist HistorySource, opts ...Option) Assembler {
	t.Helper()
	a := NewAssembler(hist, opts...)
	if err := RegisterBudget(a, "test-model", BudgetSpec{Window: 200, ReserveTokens: 20}); err != nil {
		t.Fatal(err)
	}
	return a
}

func textMsg(ref, role, text string) Message {
	return Message{Ref: ref, Role: role, Kind: KindText, Text: text}
}

// measuredMsg is a row the provider has already counted: truth rides on the
// message and the estimator never sees it.
func measuredMsg(ref, role, text string, tokens int) Message {
	return Message{Ref: ref, Role: role, Kind: KindText, Text: text, MeasuredTokens: tokens}
}

// ── 正常路径：不压缩 ────────────────────────────────────────────────────────

func TestAssembleFitsBudgetWithoutCompaction(t *testing.T) {
	hist := newFakeHistory(
		measuredMsg("m1", "user", strings.Repeat("a", 30), 30),
		measuredMsg("m2", "assistant", strings.Repeat("b", 40), 40),
	)
	a := newTestAssembler(t, hist)

	prompt, err := a.Assemble(context.Background(), mustKeyAR3(t), Turn{Model: "test-model"})
	if err != nil {
		t.Fatal(err)
	}
	if prompt.Compacted || len(prompt.Dropped) != 0 {
		t.Fatalf("unexpected compaction: %+v", prompt)
	}
	if prompt.Tokens != 70 || prompt.Budget != 180 {
		t.Fatalf("tokens=%d budget=%d; want 70/180 (window 200 - reserve 20)", prompt.Tokens, prompt.Budget)
	}
	if len(prompt.Preserved) != 0 {
		t.Fatalf("plain prose must not be marked audit-bearing: %+v", prompt.Preserved)
	}
}

// ── 验收 3（修订）：预算缺失即拒绝；计数双轨 ────────────────────────────────

func TestUnconfiguredBudgetRefuses(t *testing.T) {
	hist := newFakeHistory(textMsg("m1", "user", "hello"))
	a := NewAssembler(hist) // no budget registered

	_, err := a.Assemble(context.Background(), mustKeyAR3(t), Turn{Model: "deepseek-v4-flash"})
	if !errors.Is(err, ErrBudgetUnconfigured) {
		t.Fatalf("error = %v; want ErrBudgetUnconfigured — window geometry is configuration, not a guess", err)
	}
}

// Dual-track core (pi shape): provider-measured truth always wins; only
// never-sent messages fall back to the chars/4 tail estimate.
func TestMeasuredTokensTakePrecedenceOverEstimates(t *testing.T) {
	hist := newFakeHistory(
		measuredMsg("m1", "user", strings.Repeat("中", 40), 100), // est would say 10, truth says 100
		textMsg("m2", "assistant", strings.Repeat("b", 8)),      // unsent → estimated ceil(8/4)=2
	)
	a := newTestAssembler(t, hist)

	prompt, err := a.Assemble(context.Background(), mustKeyAR3(t), Turn{Model: "test-model"})
	if err != nil {
		t.Fatal(err)
	}
	if prompt.Tokens != 102 {
		t.Fatalf("tokens = %d; want 100 truth + 2 tail estimate", prompt.Tokens)
	}
	if prompt.EstimatedTokens != 2 {
		t.Fatalf("estimated portion = %d; want 2", prompt.EstimatedTokens)
	}
}

// ── AR3-G1：审计承载内容经类型不可达性保护 ─────────────────────────────────

// Over-budget session whose history carries tool calls, results, artifact
// refs, approvals and a scope_denied conclusion. After compaction every one
// of them must still be in the prompt — protection by invisibility.
func TestCompactionNeverDropsAuditBearingContent(t *testing.T) {
	hist := newFakeHistory(
		measuredMsg("t1", "user", strings.Repeat("u", 60), 60),
		Message{Ref: "c1", Role: "assistant", Kind: KindToolCall, Text: "call lease.portfolio.summary{}", MeasuredTokens: 15},
		Message{Ref: "r1", Role: "tool", Kind: KindToolResult, Text: strings.Repeat("r", 80), MeasuredTokens: 80},
		Message{Ref: "a1", Role: "assistant", Kind: KindArtifactRef, Text: "artifact:wp-1", MeasuredTokens: 6},
		measuredMsg("t2", "assistant", strings.Repeat("x", 60), 60),
		Message{Ref: "v1", Role: "assistant", Kind: KindApproval, Text: "approved wp-1", MeasuredTokens: 5},
		Message{Ref: "d1", Role: "tool", Kind: KindScopeDenied, Text: "scope_denied: store-9", MeasuredTokens: 7},
		measuredMsg("t3", "user", strings.Repeat("y", 60), 60),
		measuredMsg("t4", "assistant", strings.Repeat("z", 60), 60),
	)
	a := newTestAssembler(t, hist)

	prompt, err := a.Assemble(context.Background(), mustKeyAR3(t), Turn{Model: "test-model"})
	if err != nil {
		t.Fatal(err)
	}
	if !prompt.Compacted {
		t.Fatal("expected compaction to trigger")
	}
	inPrompt := map[string]bool{}
	for _, m := range prompt.Messages {
		inPrompt[m.Ref] = true
	}
	for _, ref := range []string{"c1", "r1", "a1", "v1", "d1"} {
		if !inPrompt[ref] {
			t.Fatalf("audit-bearing message %q left the prompt — D-C14 violated", ref)
		}
	}
	for _, dropped := range prompt.Dropped {
		if dropped.Kind.AuditBearing() {
			t.Fatalf("audit-bearing kind %q appears in Dropped", dropped.Kind)
		}
	}
	// Preserved lists exactly the audit-bearing set.
	if len(prompt.Preserved) != 5 {
		t.Fatalf("preserved = %d; want the five audit-bearing refs", len(prompt.Preserved))
	}
}

// Reverse fixture for AR3-G1: corrupting classify's classification must turn
// the guard red. If this test stops failing under a mutated AuditBearing(),
// the invisibility guarantee is untested.
func TestClassifyMutationWouldExposeAuditBearingContent(t *testing.T) {
	msgs := []Message{
		{Ref: "c1", Kind: KindToolCall},
		{Ref: "p1", Kind: KindText},
	}
	preserved, compactable := classify(msgs)
	if len(preserved) != 1 || preserved[0].Ref != "c1" {
		t.Fatalf("classify lost the audit-bearing message: %+v", preserved)
	}
	if len(compactable) != 1 || compactable[0].Ref != "p1" {
		t.Fatalf("classify swallowed compactable content: %+v", compactable)
	}
	// The mutation check lives in the assertion above: if someone edits
	// AuditBearing() to return false for KindToolCall, preserved comes back
	// empty here and this test goes red. The compactor cannot be fed what
	// classify never hands it.
}

// ── 验收 2：Dropped 全部解析回存储原行 ─────────────────────────────────────

func TestDroppedRefsResolveBackToStorage(t *testing.T) {
	rows := []Message{
		measuredMsg("h1", "user", strings.Repeat("1", 50), 50),
		measuredMsg("h2", "assistant", strings.Repeat("2", 50), 50),
		measuredMsg("h3", "user", strings.Repeat("3", 50), 50),
		measuredMsg("h4", "assistant", strings.Repeat("4", 50), 50),
	}
	hist := newFakeHistory(rows...)
	a := newTestAssembler(t, hist)

	prompt, err := a.Assemble(context.Background(), mustKeyAR3(t), Turn{Model: "test-model"})
	if err != nil {
		t.Fatal(err)
	}
	if !prompt.Compacted || len(prompt.Dropped) == 0 {
		t.Fatalf("expected drops, got compacted=%v dropped=%d", prompt.Compacted, len(prompt.Dropped))
	}
	if err := hist.resolveAll(prompt.Dropped); err != nil {
		t.Fatalf("compaction deleted records: %v", err)
	}
}

// ── Turn 边界切割：tool-call 序列不被撕裂 ───────────────────────────────────

func TestTrimCutsAtTurnBoundariesOnly(t *testing.T) {
	msgs := []Message{
		textMsg("u1", "user", "first question"),
		textMsg("a1", "assistant", "answer"),
		textMsg("u2", "user", "second question"),
		textMsg("a2", "assistant", "answer two"),
		textMsg("u3", "user", "third"),
	}
	if cut := nextTrimStart(msgs); cut != 2 {
		t.Fatalf("nextTrimStart = %d; want 2 (second turn boundary)", cut)
	}
	kept, dropped := trimToBudget(msgs, func(list []Message) int { return len(list) }, 3)
	if kept == nil || len(dropped) == 0 {
		t.Fatalf("trim did not drop: kept=%d dropped=%d", len(kept), len(dropped))
	}
	if dropped[0].Role != "user" || kept[0].Role != "user" {
		t.Fatalf("cut landed mid-turn: first dropped=%q first kept=%q", dropped[0].Role, kept[0].Role)
	}
}

// ── Summarizer：注入生效、缺省诚实 ─────────────────────────────────────────

func TestSummarizerRecapStaysInPrompt(t *testing.T) {
	hist := newFakeHistory(
		measuredMsg("h1", "user", strings.Repeat("1", 90), 90),
		measuredMsg("h2", "assistant", strings.Repeat("2", 90), 90),
		measuredMsg("h3", "user", strings.Repeat("3", 50), 50),
	)
	sum := &countingSummarizer{text: "earlier: user confirmed discount rate 4.5%"}
	a := newTestAssembler(t, hist, WithSummarizer(sum))

	key := mustKeyAR3(t)
	prompt, err := a.Assemble(context.Background(), key, Turn{Model: "test-model"})
	if err != nil {
		t.Fatal(err)
	}
	if sum.called != 1 || len(sum.got) == 0 {
		t.Fatalf("summarizer not exercised: called=%d got=%d", sum.called, len(sum.got))
	}
	found := false
	for _, m := range prompt.Messages {
		if strings.Contains(m.Text, "discount rate 4.5%") {
			found = true
		}
	}
	if !found || prompt.Summary == "" {
		t.Fatalf("recap missing from prompt: found=%v summary=%q", found, prompt.Summary)
	}
}

func TestOversizedSummaryIsAnErrorNotASilentSend(t *testing.T) {
	hist := newFakeHistory(
		measuredMsg("h1", "user", strings.Repeat("1", 170), 170),
		measuredMsg("h2", "user", strings.Repeat("2", 30), 30),
	)
	sum := &countingSummarizer{text: strings.Repeat("s", 2000)}
	a := newTestAssembler(t, hist, WithSummarizer(sum))

	_, err := a.Assemble(context.Background(), mustKeyAR3(t), Turn{Model: "test-model"})
	if err == nil || !strings.Contains(err.Error(), "does not fit") {
		t.Fatalf("error = %v; want the oversized-summary refusal", err)
	}
}

// ── 审计底价本身超预算：fail loud ──────────────────────────────────────────

func TestAuditFloorOverBudgetFailsLoudly(t *testing.T) {
	hist := newFakeHistory(
		Message{Ref: "big", Role: "tool", Kind: KindToolResult, Text: strings.Repeat("r", 500), MeasuredTokens: 500},
	)
	a := newTestAssembler(t, hist)

	_, err := a.Assemble(context.Background(), mustKeyAR3(t), Turn{Model: "test-model"})
	if !errors.Is(err, ErrOverBudgetAfterCompaction) {
		t.Fatalf("error = %v; want ErrOverBudgetAfterCompaction — evidence does not shrink to fit", err)
	}
}

// ── 工具定义计入预算三元式 ─────────────────────────────────────────────────

func TestToolDefsCountTowardBudget(t *testing.T) {
	hist := newFakeHistory(measuredMsg("m1", "user", strings.Repeat("a", 150), 150))
	a := newTestAssembler(t, hist)

	prompt, err := a.Assemble(context.Background(), mustKeyAR3(t), Turn{
		Model:    "test-model",
		ToolDefs: []ToolDef{{Name: "t", JSON: strings.Repeat("j", 200)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !prompt.Compacted {
		t.Fatalf("150 msg chars + 50 def chars > 180 budget must trigger compaction, tokens=%d", prompt.Tokens)
	}
}
