package contextassembler

import (
	"context"
	"errors"
	"fmt"
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
	// Round-total semantics (AF1-a): each assistant row carries the ROUND
	// TOTAL of that round's prompt. The newest value (40) already covers both
	// rows — summing would invent 70 out of a true prompt of ~40.
	hist := newFakeHistory(
		measuredMsg("m1", "user", strings.Repeat("a", 30), 0),
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
	if prompt.Tokens != 40 || prompt.Budget != 180 {
		t.Fatalf("tokens=%d budget=%d; want 40/180 (baseline round total, window 200 - reserve 20)", prompt.Tokens, prompt.Budget)
	}
	if len(prompt.Preserved) != 0 {
		t.Fatalf("plain prose must not be marked audit-bearing: %+v", prompt.Preserved)
	}
}

// ── AF1-a：以生产写入语义（轮总量）为输入的计数断言 ─────────────────────────

// The review probe: three rounds whose provider-measured prompt totals are
// 1000 / 1100 / 1200. Under the old sum-per-row reading this counted 3303
// while the true next-round prompt is ~1200 (error grows quadratically with
// turns). Baseline semantics must count the newest round truth plus the
// unsent tail estimate only. Mutation check: reverting to sum makes this red.
func TestMeasuredRoundTotalsAreBaselinesNotSummands(t *testing.T) {
	hist := newFakeHistory(
		textMsg("u1", "user", strings.Repeat("a", 40)),
		measuredMsg("a1", "assistant", strings.Repeat("b", 40), 1000),
		textMsg("u2", "user", strings.Repeat("c", 40)),
		measuredMsg("a2", "assistant", strings.Repeat("d", 40), 1100),
		textMsg("u3", "user", strings.Repeat("e", 40)),
		measuredMsg("a3", "assistant", strings.Repeat("f", 40), 1200),
		// Unsent tail: never part of any measured round.
		textMsg("u4", "user", strings.Repeat("g", 40)), // est ceil(40/4)=10
	)
	a := newTestAssembler(t, hist)
	// A budget large enough that the probe stays OUT of compaction — this
	// test isolates counting semantics, not trimming.
	if err := RegisterBudget(a, "test-model", BudgetSpec{Window: 2000, ReserveTokens: 20}); err != nil {
		t.Fatal(err)
	}

	prompt, err := a.Assemble(context.Background(), mustKeyAR3(t), Turn{Model: "test-model"})
	if err != nil {
		t.Fatal(err)
	}
	if prompt.Tokens != 1210 {
		t.Fatalf("tokens = %d; want 1210 = baseline 1200 + tail estimate 10 (summing would say 3310)", prompt.Tokens)
	}
	if prompt.EstimatedTokens != 10 {
		t.Fatalf("estimated portion = %d; want 10 (only the unsent tail)", prompt.EstimatedTokens)
	}
}

// Dropping the baseline carrier must degrade safely, not fabricate: with the
// newest measured row compacted away, counting falls back to the next-newest
// measured row still present (or full estimation) — never to a sum.
func TestCountingDegradesSafelyWhenBaselineRowIsDropped(t *testing.T) {
	hist := newFakeHistory(
		measuredMsg("a1", "assistant", strings.Repeat("b", 40), 500),
		measuredMsg("a2", "assistant", strings.Repeat("d", 40), 600),
		// Long unsent tail forces compaction of the oldest turns; a1 may drop.
		textMsg("u1", "user", strings.Repeat("x", 800)), // est 200
		textMsg("u2", "user", strings.Repeat("y", 800)), // est 200
	)
	a := newTestAssembler(t, hist)

	prompt, err := a.Assemble(context.Background(), mustKeyAR3(t), Turn{Model: "test-model"})
	if err != nil {
		t.Fatal(err)
	}
	if !prompt.Compacted {
		t.Fatal("expected compaction")
	}
	// Whatever was kept, the recount after compaction must be bounded by the
	// surviving measured truth plus estimates — strictly less than the naive
	// sum (500+600+400=1500).
	if prompt.Tokens >= 1500 {
		t.Fatalf("post-compaction tokens = %d; summed semantics resurfaced", prompt.Tokens)
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
	// The measured value is a ROUND TOTAL (production write semantics); the
	// unsent tail is estimated on top of it.
	hist := newFakeHistory(
		measuredMsg("m1", "user", strings.Repeat("中", 40), 0),
		measuredMsg("m2", "assistant", strings.Repeat("b", 8), 100), // round truth says 100
		textMsg("m3", "user", strings.Repeat("b", 8)),               // unsent → estimated ceil(8/4)=2
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
		textMsg("t1", "user", strings.Repeat("u", 60)),
		Message{Ref: "c1", Role: "assistant", Kind: KindToolCall, Text: "call lease.portfolio.summary{}"},
		Message{Ref: "r1", Role: "tool", Kind: KindToolResult, Text: strings.Repeat("r", 80)},
		Message{Ref: "a1", Role: "assistant", Kind: KindArtifactRef, Text: "artifact:wp-1"},
		textMsg("t2", "assistant", strings.Repeat("x", 60)),
		Message{Ref: "v1", Role: "assistant", Kind: KindApproval, Text: "approved wp-1"},
		Message{Ref: "d1", Role: "tool", Kind: KindScopeDenied, Text: "scope_denied: store-9"},
		// Newest measured round truth, followed by a long unsent tail whose
		// estimates push the prompt over budget.
		measuredMsg("m-last", "assistant", strings.Repeat("x", 40), 80),
		textMsg("u1", "user", strings.Repeat("y", 400)),
		textMsg("u2", "user", strings.Repeat("z", 400)),
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
		measuredMsg("h1", "user", strings.Repeat("1", 50), 0),
		measuredMsg("h2", "assistant", strings.Repeat("2", 50), 50),
		measuredMsg("h3", "user", strings.Repeat("3", 50), 0),
		measuredMsg("h4", "assistant", strings.Repeat("4", 50), 70),
		// Unsent tail pushes the baseline-plus-tail count over budget:
		// 70 + ceil(480/4)=120 → 190 > 180.
		textMsg("u5", "user", strings.Repeat("5", 480)),
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
		measuredMsg("h1", "user", strings.Repeat("1", 90), 0),
		measuredMsg("h2", "assistant", strings.Repeat("2", 90), 90),
		// Unsent tail drives the count over budget.
		textMsg("h3", "user", strings.Repeat("3", 400)),
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
		measuredMsg("h1", "user", strings.Repeat("1", 170), 0),
		measuredMsg("h2", "user", strings.Repeat("2", 30), 30),
		textMsg("h3", "user", strings.Repeat("9", 800)), // unsent tail forces compaction
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

// ── AF4：裁剪判据里工具定义只算一次 ────────────────────────────────────────

// Review probe reproduced: with the wire's tool-schema cost charged BOTH in
// the floor and again inside every trim-step recount, the trimmer over-cuts.
// Numbers below are tuned so that the true usage needs exactly ONE turn cut;
// under the double charge the old code cut TWO (dropped=2, want 1).
func TestTrimDoesNotDoubleChargeToolDefs(t *testing.T) {
	// Six single-message turns, each estimated at ceil(224/4)=56 tokens.
	rows := make([]Message, 0, 6)
	for i := 0; i < 6; i++ {
		rows = append(rows, textMsg(fmt.Sprintf("u%d", i), "user", strings.Repeat("a", 224)))
	}
	hist := newFakeHistory(rows...)
	a := newTestAssembler(t, hist)
	if err := RegisterBudget(a, "test-model", BudgetSpec{Window: 320, ReserveTokens: 20}); err != nil {
		t.Fatal(err)
	}

	defs := []ToolDef{{Name: "t", JSON: strings.Repeat("j", 41)}} // est ceil(42/4)=11
	prompt, err := a.Assemble(context.Background(), mustKeyAR3(t), Turn{Model: "test-model", ToolDefs: defs})
	if err != nil {
		t.Fatal(err)
	}
	if !prompt.Compacted {
		t.Fatal("expected compaction")
	}
	if len(prompt.Dropped) != 1 || len(prompt.Messages) != 5 {
		t.Fatalf("dropped=%d kept=%d; want exactly 1 turn cut (double charge would cut 2)",
			len(prompt.Dropped), len(prompt.Messages))
	}
}
