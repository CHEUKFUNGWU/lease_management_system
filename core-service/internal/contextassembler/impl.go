package contextassembler

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/lease-management-system/core-service/internal/agentcontext"
)

// Token counting is dual-track (pi compaction.js shape): provider-reported
// MeasuredTokens win wherever present; unsent tail content falls back to the
// injected estimator. Budget geometry stays a per-model registry — window
// sizes are configuration, not guesses.
type budgetRegistry struct {
	mu      sync.RWMutex
	byModel map[string]BudgetSpec
}

func newBudgetRegistry() *budgetRegistry {
	return &budgetRegistry{byModel: map[string]BudgetSpec{}}
}

func (r *budgetRegistry) set(model string, spec BudgetSpec) error {
	if strings.TrimSpace(model) == "" {
		return fmt.Errorf("budget model name is required")
	}
	if spec.Window <= 0 {
		return fmt.Errorf("budget for %q must carry a positive context window", model)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byModel[model] = spec
	return nil
}

func (r *budgetRegistry) forModel(model string) (BudgetSpec, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	spec, ok := r.byModel[model]
	if !ok {
		return BudgetSpec{}, fmt.Errorf("%w: %q", ErrBudgetUnconfigured, model)
	}
	return spec, nil
}

// assembler is the only Assembler implementation. Its fields are shared
// configuration registries and ports — no per-run state lives here; per-turn
// state is the call's own locals.
type assembler struct {
	history    HistorySource
	budgets    *budgetRegistry
	estimator  TokenEstimator
	summarizer Summarizer
	mode       Mode // RT1-A: off/count/on（count 只计数不压缩）
}

// Option configures the assembler at construction.
type Option func(*assembler)

// WithSummarizer attaches the optional summarizer port. Absent ⇒ compaction
// drops without a recap and Story 3's limitation stays honestly visible
// (registered as an open item), never faked.
func WithSummarizer(s Summarizer) Option { return func(a *assembler) { a.summarizer = s } }

// WithEstimator overrides the tail estimator. Default is PiStyleEstimator.
func WithEstimator(e TokenEstimator) Option { return func(a *assembler) { a.estimator = e } }

// WithMode sets the RT1-A mode. Default is ModeOff (legacy path is gated
// upstream; an assembler constructed without WithMode behaves as before — no
// compaction). ModeCount runs the budget geometry and reports occupancy but
// never trims; ModeOn adds compaction.
func WithMode(m Mode) Option { return func(a *assembler) { a.mode = m } }

// NewAssembler constructs the module. All IO sits behind ports; every branch
// is testable without a database.
func NewAssembler(history HistorySource, opts ...Option) Assembler {
	a := &assembler{
		history:   history,
		budgets:   newBudgetRegistry(),
		estimator: PiStyleEstimator{},
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// RegisterBudget registers the context geometry for one model.
func RegisterBudget(a Assembler, model string, spec BudgetSpec) error {
	impl, ok := a.(*assembler)
	if !ok {
		return fmt.Errorf("RegisterBudget: unknown assembler implementation")
	}
	return impl.budgets.set(model, spec)
}

// Assemble implements the interface. Order inside is fixed: read → count →
// compare → compact → recount. The caller cannot reorder it and cannot skip
// a step.
//
// Over-budget handling: audit-bearing content (preserved) is a fixed floor —
// it is counted on every trim step but can never leave. Only the compactable
// half trims, oldest complete turn first, until the whole fits. When a
// summarizer is wired, what left is recapped into one system message; if that
// recap itself overflows the remaining room, the error says so instead of
// silently sending an oversized prompt.
func (a *assembler) Assemble(ctx context.Context, key agentcontext.ContextKey, turn Turn) (Prompt, error) {
	spec, err := a.budgets.forModel(turn.Model)
	if err != nil {
		return Prompt{}, fmt.Errorf("assemble %s: %w", key.SessionID(), err)
	}
	budget := spec.Budget()

	history, err := a.history.Read(ctx, key)
	if err != nil {
		return Prompt{}, fmt.Errorf("read history for session %s: %w", key.SessionID(), err)
	}

	msgs := append(append([]Message(nil), history...), turn.Messages...)
	countMsgs := func(list []Message) int { return countMessages(a.estimator, list) }

	preserved, compactable := classify(msgs)

	prompt := Prompt{
		Budget:    budget,
		Preserved: refsOf(preserved),
	}
	prompt.Tokens, prompt.EstimatedTokens = splitCount(a.estimator, msgs, turn.ToolDefs)
	if !overBudget(budget, prompt.Tokens) {
		prompt.Messages = msgs
		return prompt, nil
	}

	// RT1-A count mode: over budget but compression deferred. Report the
	// occupancy truth and the pre-warning WouldCompact signal, return the
	// UNTRIMMED prompt unchanged — prompt content is byte-for-byte what on
	// mode would have sent, and no content leaves. context_compacted remains
	// the post-hoc record only when compaction actually runs (mode on).
	if a.mode == ModeCount {
		prompt.Messages = msgs
		prompt.WouldCompact = true
		return prompt, nil
	}

	// Over budget from here on. Preserved content alone exceeding the budget
	// is unfixable by definition — refuse before touching anything.
	// AF4: the wire's tool schemas cost is computed ONCE here and shared by
	// every trim-step recount — the old code added it again inside countKept,
	// double-charging the fixed cost and cutting one turn too many.
	toolDefsCost := a.estimator.EstimateToolDefs(turn.ToolDefs)
	floor := countMsgs(preserved) + toolDefsCost
	if overBudget(budget, floor) {
		return Prompt{}, fmt.Errorf("%w: session %s (%d > %d)",
			ErrOverBudgetAfterCompaction, key.SessionID(), floor, budget)
	}

	countKept := func(kept []Message) int {
		return floor + countMsgs(kept)
	}
	kept, droppedMsgs := trimToBudget(compactable, countKept, budget)

	final := make([]Message, 0, len(preserved)+len(kept)+1)
	final = append(final, preserved...)

	if len(droppedMsgs) > 0 && a.summarizer != nil {
		summary, sumErr := a.summarizer.Summarize(ctx, droppedMsgs)
		if sumErr != nil {
			return Prompt{}, fmt.Errorf("summarize dropped content for session %s: %w", key.SessionID(), sumErr)
		}
		if summary != "" {
			recap := Message{
				Ref:  "summary:" + key.SessionID(),
				Role: "system",
				Kind: KindText,
				Text: summary,
			}
			prompt.Summary = summary
			final = append(final, recap)
		}
	}
	final = append(final, kept...)

	prompt.Messages = final
	prompt.Compacted = len(droppedMsgs) > 0
	prompt.Dropped = refsOf(droppedMsgs)
	prompt.Tokens, prompt.EstimatedTokens = splitCount(a.estimator, final, turn.ToolDefs)

	if overBudget(budget, prompt.Tokens) {
		return Prompt{}, fmt.Errorf(
			"post-compaction prompt still exceeds budget for session %s (%d > %d): the summary does not fit the remaining room",
			key.SessionID(), prompt.Tokens, budget)
	}
	return prompt, nil
}

// measuredBaselineIndex returns the position of the newest message carrying
// provider-measured round truth, or -1 when the list has none.
//
// Semantics (AF1-a, read side of D37 dual-track): MeasuredTokens stores the
// ROUND TOTAL of provider prompt_tokens as of the round where that message
// was the newest content — NOT a per-message token count. Summing those
// totals across rows double-counts every shared prefix (error grows
// quadratically with turns). The truthful reading: everything up to and
// including the newest measured row is already covered by its round total;
// only unsent rows AFTER it need estimation. The baseline slightly
// over-counts (that round's system prompt and tool defs rode in it), which
// errs toward earlier compaction — the safe direction.
func measuredBaselineIndex(msgs []Message) int {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].MeasuredTokens > 0 {
			return i
		}
	}
	return -1
}

// countMessages totals the list under the baseline semantics above.
func countMessages(est TokenEstimator, msgs []Message) int {
	total, _ := splitCount(est, msgs, nil)
	return total
}

// splitCount additionally reports how much of the total came from estimates —
// the observability hook for provider-usage calibration.
func splitCount(est TokenEstimator, msgs []Message, defs []ToolDef) (total, estimated int) {
	base := measuredBaselineIndex(msgs)
	for i, m := range msgs {
		switch {
		case i < base:
			// covered by the baseline round truth; adds nothing
		case i == base:
			total += m.MeasuredTokens
		default:
			e := est.EstimateMessage(m)
			total += e
			estimated += e
		}
	}
	defsEst := est.EstimateToolDefs(defs)
	total += defsEst
	estimated += defsEst
	return total, estimated
}
