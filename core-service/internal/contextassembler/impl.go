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
}

// Option configures the assembler at construction.
type Option func(*assembler)

// WithSummarizer attaches the optional summarizer port. Absent ⇒ compaction
// drops without a recap and Story 3's limitation stays honestly visible
// (registered as an open item), never faked.
func WithSummarizer(s Summarizer) Option { return func(a *assembler) { a.summarizer = s } }

// WithEstimator overrides the tail estimator. Default is PiStyleEstimator.
func WithEstimator(e TokenEstimator) Option { return func(a *assembler) { a.estimator = e } }

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

	// Over budget from here on. Preserved content alone exceeding the budget
	// is unfixable by definition — refuse before touching anything.
	floor := countMsgs(preserved) + a.estimator.EstimateToolDefs(turn.ToolDefs)
	if overBudget(budget, floor) {
		return Prompt{}, fmt.Errorf("%w: session %s (%d > %d)",
			ErrOverBudgetAfterCompaction, key.SessionID(), floor, budget)
	}

	countKept := func(kept []Message) int {
		return floor + countMsgs(kept) + a.estimator.EstimateToolDefs(turn.ToolDefs)
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

// countMessages sums measured truth where present; estimation only for
// messages that have never been sent.
func countMessages(est TokenEstimator, msgs []Message) int {
	total := 0
	for _, m := range msgs {
		if m.MeasuredTokens > 0 {
			total += m.MeasuredTokens
		} else {
			total += est.EstimateMessage(m)
		}
	}
	return total
}

// splitCount additionally reports how much of the total came from estimates —
// the observability hook for provider-usage calibration.
func splitCount(est TokenEstimator, msgs []Message, defs []ToolDef) (total, estimated int) {
	for _, m := range msgs {
		if m.MeasuredTokens > 0 {
			total += m.MeasuredTokens
		} else {
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
