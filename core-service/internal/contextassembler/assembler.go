// Package contextassembler is the AR3 module: context engineering as ONE
// method (Spec C1 D-C2/D-C3, module design §5). Counting, budgeting and
// compaction happen inside Assemble — the caller neither chooses nor can skip
// them, so expressing the wrong order has no syntax.
//
// The interface is thin and the return value is thick: everything tests need
// to observe (was compaction triggered, what was dropped, what was preserved)
// travels in Prompt's fields instead of extra interface methods.
//
// Isolation rides on AR1: Assemble takes a ContextKey, so within-session
// history stays stable under the session id while scope fingerprint and data
// classification dimensions are carried for every cross-session consumer
// (memory in AR6, compression caches here).
package contextassembler

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/lease-management-system/core-service/internal/agentcontext"
)

// ErrBudgetUnconfigured is returned when the model in play has no context
// geometry registered. Window sizes are configuration, not something to guess.
//
// 计数决策修订（2026-08-24，用户裁决「学 pi」）：原 D-C15 方案是「精确分词器
// 注册制、缺失即拒绝」。实际采用 pi 的双轨形态——provider 每次响应回传的实测
// usage 是主真值（语言无关精确，中文分词差异不进入误差），仅对尚未发送过的
// 尾部消息用 chars/4 兜底且下一轮立即被真值覆盖。残余风险与依据登记在
// AI_文档索引 §2 与本包 PiStyleEstimator 注释。

// ErrOverBudgetAfterCompaction is returned when audit-bearing content alone
// exceeds the budget. Preserved content never participates in compaction
// (D-C14), so if it does not fit there is nothing honest to return — fail
// loudly rather than silently trimming evidence.

var ErrBudgetUnconfigured = errors.New("no context budget configured for model")
var ErrOverBudgetAfterCompaction = errors.New("audit-bearing messages alone exceed the context budget")

// MessageKind separates audit-bearing content from compactable prose. The
// taxonomy mirrors what lands in ai_chat_messages and run_events today.
type MessageKind string

const (
	KindText        MessageKind = "text"         // user prose / assistant answer — compactable
	KindToolCall    MessageKind = "tool_call"    // tool invocation incl. arguments
	KindToolResult  MessageKind = "tool_result"  // tool outcome
	KindArtifactRef MessageKind = "artifact_ref" // working-paper / artifact reference
	KindApproval    MessageKind = "approval"     // review / approval action
	KindScopeDenied MessageKind = "scope_denied" // permission conclusion
)

// AuditBearing reports whether the kind must NEVER leave the prompt.
// The set is deliberately defined HERE and consumed by classify — compaction
// code never sees these messages at all, so it cannot drop them.
func (k MessageKind) AuditBearing() bool {
	switch k {
	case KindToolCall, KindToolResult, KindArtifactRef, KindApproval, KindScopeDenied:
		return true
	default:
		return false
	}
}

// Message is one conversation entry as seen by the assembler. Ref resolves
// back to the stored row (ai_chat_messages id or run event reference) — it is
// what makes "compaction deleted nothing" an executable assertion (D-C16).
type Message struct {
	Ref  string // stored-row identity; empty only for synthetic messages
	Role string // user | assistant | system | tool
	Kind MessageKind
	Text string
	// MeasuredTokens stores the provider-reported ROUND TOTAL of prompt tokens
	// (prompt_tokens) as of the round where this message was the newest
	// content — NOT a per-message token count (AF1-a). Read side: the newest
	// measured row is the baseline for everything before it; only unsent rows
	// after it are estimated. Zero means "never measured" — a sentinel, never
	// a measured zero.
	MeasuredTokens int
}

// Turn carries this turn's additions: the model in play, the tool definitions
// that will ride on the request, and any fresh messages not yet in history.
type Turn struct {
	Model    string
	ToolDefs []ToolDef
	Messages []Message
}

// ToolDef is the minimal shape needed to count tool definitions.
type ToolDef struct {
	Name        string
	Description string
	JSON        string // serialized parameters schema as sent on the wire
}

// Prompt is the assembled output: what to send, plus the evidence of what
// happened inside.
type Prompt struct {
	Messages        []Message
	Tokens          int  // measured truth where available + tail estimates
	EstimatedTokens int  // the portion that came from the tail estimator (observability)
	Budget          int  // effective budget: window minus output reserve
	Compacted       bool // whether anything left the prompt
	// WouldCompact is the RT1-A pre-warning signal: true when the turn would
	// have exceeded its budget and therefore would have compacted — set in
	// count mode where compaction is deliberately deferred. It fires BEFORE
	// any content is dropped (count mode drops nothing), and the compaction
	// event (context_compacted) remains the post-hoc record when compaction
	// actually runs. The two coexist; one is not a rename of the other.
	WouldCompact bool
	Preserved    []MessageRef // audit-bearing content: never participated in compaction
	Dropped      []MessageRef // content that left the prompt; still resolvable in storage
	Summary      string       // summarizer output covering dropped content, when available
}

// MessageRef identifies a message that was preserved or dropped.
type MessageRef struct {
	Ref  string
	Kind MessageKind
}

// Assembler builds the per-turn prompt. One method (module design §5).
type Assembler interface {
	Assemble(ctx context.Context, key agentcontext.ContextKey, turn Turn) (Prompt, error)
}

// Mode is RT1-A's two-level switch: what the assembler does with the counting
// it already performs. off = legacy path (no assembly at all, gated upstream
// by the flag); count = run the full budget geometry, report occupancy, but
// do NOT compact (production traffic data before the first behavior change);
// on = count AND compact (the pre-RT1-A behavior of CONTEXT_ASSEMBLER_ENABLED).
type Mode string

const (
	ModeOff   Mode = "off"
	ModeCount Mode = "count"
	ModeOn    Mode = "on"
)

// ParseMode converts the env value to a Mode. Unknown and empty values are
// off so an unset env var keeps the legacy byte-for-byte path.
func ParseMode(value string) Mode {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "count":
		return ModeCount
	case "on", "true":
		return ModeOn
	default:
		return ModeOff
	}
}

// TokenEstimator estimates tokens for content that has no provider truth yet.
// The production default is PiStyleEstimator; the seam exists so the divisor
// or scheme can change without touching call sites.
type TokenEstimator interface {
	EstimateMessage(m Message) int
	EstimateToolDefs(defs []ToolDef) int
}

// PiStyleEstimator replicates pi's compaction.js estimateTokens: characters
// divided by four, rounded up. Zero dependencies, zero vocabulary files.
// Known limitation (registered): for pure-Chinese prose real density is
// roughly 0.6–1.5 tokens per character, so chars/4 under-counts there; the
// dual-track design caps the blast radius to unsent tail messages, and the
// divisor is a one-line change if traffic data justifies it.
type PiStyleEstimator struct{}

func (PiStyleEstimator) EstimateMessage(m Message) int {
	return ceilDiv(utf8.RuneCountInString(m.Text), 4)
}

func (PiStyleEstimator) EstimateToolDefs(defs []ToolDef) int {
	total := 0
	for _, d := range defs {
		total += utf8.RuneCountInString(d.Name) + utf8.RuneCountInString(d.JSON)
	}
	return ceilDiv(total, 4)
}

func ceilDiv(a, b int) int {
	if b <= 0 {
		return a
	}
	return (a + b - 1) / b
}

// Budgets answers the per-model budget configuration.
type Budgets interface {
	For(model string) (BudgetSpec, bool)
}

// BudgetSpec is one model's context geometry. ReserveTokens is the output
// reserve: budget = Window - Reserve, matching upstream's
// "compress when messages + tools + reserve > window".
type BudgetSpec struct {
	Window        int
	ReserveTokens int
}

// Budget returns the effective budget for a spec.
func (s BudgetSpec) Budget() int {
	b := s.Window - s.ReserveTokens
	if b < 0 {
		return s.Window
	}
	return b
}

// HistorySource reads this session's stored messages. Within-session reads
// locate by session id (stable across classification escalation, D-C20);
// ownership enforcement lives at the adapter's data boundary like AR2's store.
type HistorySource interface {
	Read(ctx context.Context, key agentcontext.ContextKey) ([]Message, error)
}

// Summarizer optionally condenses dropped content into a short recap that
// stays in the prompt. Absent ⇒ compaction drops without a summary and
// Story 3's limitation is honest (registered as an open item), never faked.
type Summarizer interface {
	Summarize(ctx context.Context, dropped []Message) (string, error)
}

// classify splits messages into audit-bearing (preserved) and compactable
// halves. Pure function; the compactor's signature accepts ONLY the
// compactable half, so it cannot discard audit-bearing content — protection
// by invisibility, not by remembered rules (D-C14).
func classify(msgs []Message) (preserved, compactable []Message) {
	for _, m := range msgs {
		if m.Kind.AuditBearing() {
			preserved = append(preserved, m)
		} else {
			compactable = append(compactable, m)
		}
	}
	return preserved, compactable
}

// refsOf projects messages to their refs.
func refsOf(msgs []Message) []MessageRef {
	out := make([]MessageRef, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, MessageRef{Ref: m.Ref, Kind: m.Kind})
	}
	return out
}

// fmtKey keeps error messages attributable without a String method on the key.
func fmtKey(key agentcontext.ContextKey) string {
	return fmt.Sprintf("session=%s", key.SessionID())
}
