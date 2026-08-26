package aiagent

// C1（架构重构任务书 2026-08-26）步骤 3：上下文装配（AR3 接缝的使用侧）
// 从 agent.go 搬出。除经 h.ctxAssembler / h.llm() 的既有接缝外无 IO，
// 行为零改动。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agentcontext"
	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/contextassembler"
	"github.com/lease-management-system/core-service/internal/llm"
)

func (h *Agent) assembleTurnHistory(ctx context.Context, legalEntityID, userID string, req Request, toolDefs []contextassembler.ToolDef) (contextassembler.Prompt, error) {
	if h == nil || h.ctxAssembler == nil || strings.TrimSpace(req.SessionID) == "" {
		return contextassembler.Prompt{}, errAssemblerNotApplicable
	}
	client, err := h.llm()
	if err != nil {
		return contextassembler.Prompt{}, err
	}
	// AR1 D-C9b 消费方策略：global 上下文（无具体法人）没有可归属的记忆，
	// AR3 拒绝 global 键。这里在 seam 处显式回落 legacy 路径（与 AR3 的
	// Read 拒绝同源：记忆不跨法人搬运），而不是让 Assemble 硬报错。
	if scope, ok := access.ScopeFromContext(ctx); ok && scope.Global {
		return contextassembler.Prompt{}, errAssemblerNotApplicable
	}
	key, err := agentcontext.KeyFrom(agenttools.Principal{
		UserID: userID,
		Scope:  access.Scope{LegalEntityID: legalEntityID},
	}, req.SessionID, agentcontext.ClassificationProduction)
	if err != nil {
		// An identity that cannot form a key cannot be checked against stored
		// history; those requests keep the legacy path. Registered limitation:
		// the chat plane persists every session as 'production' today (062
		// column default), so the classification dimension is fixed until a
		// simulated-context chat exists.
		return contextassembler.Prompt{}, errAssemblerNotApplicable
	}
	turn := contextassembler.Turn{
		Model:    client.Config().Model,
		ToolDefs: toolDefs,
		// AF1-b contract: the current user message is NOT placed here. It is
		// persisted by aichat.prepare before execution and read back as the
		// final history row, so Assemble returns it as the prompt's LAST
		// message — exactly once. Turn.Messages stays a port for future
		// fresh-message consumers (AR6); the chat plane keeps it empty.
	}
	prompt, err := h.ctxAssembler.Assemble(ctx, key, turn)
	if err != nil {
		return contextassembler.Prompt{}, err
	}
	// RT1-A: feed the observability sink from the returned Prompt. Measured
	// and estimated travel as two separate series (AF1-a lesson: never merge
	// truth and guess into one number). WouldCompact is the pre-warning
	// signal that fires before any drop; Compacted is the post-hoc record.
	if h.contextMetrics != nil {
		h.contextMetrics.ObserveContext(turn.Model,
			prompt.Tokens-prompt.EstimatedTokens,
			prompt.EstimatedTokens,
			prompt.Budget,
			prompt.Compacted,
			prompt.WouldCompact || prompt.Compacted,
		)
	}
	// Defensive invariant: if a future path ever executes without persisting
	// the trigger first, the current message would silently vanish from the
	// wire. Refuse loudly instead.
	if n := len(prompt.Messages); n == 0 || prompt.Messages[n-1].Role != "user" {
		lastRole := ""
		if n > 0 {
			lastRole = prompt.Messages[n-1].Role
		}
		return contextassembler.Prompt{}, fmt.Errorf(
			"assembled prompt for session %s does not end with the current user message (%d messages, last role %q)",
			key.SessionID(), n, lastRole)
	}
	return prompt, nil
}

// fileParseToolDefs projects the static file-triage tool table onto the
// assembler's ToolDef shape. These schemas ride the triage request's `tools`
// param, so the provider counts them — they belong in the budget (AF4).
func fileParseToolDefs() []contextassembler.ToolDef {
	defs := make([]contextassembler.ToolDef, 0, len(fileParseTools))
	for _, t := range fileParseTools {
		fn, _ := t["function"].(map[string]interface{})
		name, _ := fn["name"].(string)
		desc, _ := fn["description"].(string)
		paramsJSON, _ := json.Marshal(fn["parameters"])
		defs = append(defs, contextassembler.ToolDef{Name: name, Description: desc, JSON: string(paramsJSON)})
	}
	return defs
}

// assembledConversation resolves the wire conversation for one LLM round:
// the assembled prompt under the flag, the legacy history+current-message
// fold otherwise. toolDefs carries ONLY the tool schemas that actually ride
// THIS request's tools param — plain chat sends none (adjudication: budget
// predicts provider-side overflow, and the provider counts what is sent;
// counting absent schemas would be a typed number with guessed semantics).
func (h *Agent) assembledConversation(ctx context.Context, legalEntityID, userIDStr string, req Request, toolDefs []contextassembler.ToolDef, emit func(context.Context, string, any) error) ([]ChatMessage, error) {
	prompt, asmErr := h.assembleTurnHistory(ctx, legalEntityID, userIDStr, req, toolDefs)
	switch {
	case asmErr == nil:
		// AF1-b invariant: the assembled prompt already ends with the current
		// user message — pass it verbatim, never append again.
		conversation := chatMessagesFromPrompt(prompt.Messages)
		if prompt.Compacted {
			// D-C16 evidence trail: what left the prompt is recorded as refs
			// resolvable back to ai_chat_messages rows — compaction deleted
			// nothing, and this event is the proof it happened.
			if err := emitAgentEvent(ctx, emit, "context_compacted", map[string]interface{}{
				"dropped":          prompt.Dropped,
				"preserved":        prompt.Preserved,
				"budget":           prompt.Budget,
				"tokens":           prompt.Tokens,
				"estimated_tokens": prompt.EstimatedTokens,
			}); err != nil {
				return nil, err
			}
		}
		return conversation, nil
	case errors.Is(asmErr, errAssemblerNotApplicable):
		return withCurrentMessage(req.History, req.Message), nil
	default:
		// Assemble failures are real failures (unconfigured budget geometry,
		// storage refusal). They stop the turn loudly instead of silently
		// degrading to an unbounded prompt.
		return nil, asmErr
	}
}

// chatMessagesFromPrompt projects assembled messages back onto the wire
// history shape. Kind never survives the projection because ai_chat_messages
// stores only text today — the audit-bearing taxonomy activates when tool
// messages enter history (registered future work, not silently dropped).
func chatMessagesFromPrompt(messages []contextassembler.Message) []ChatMessage {
	out := make([]ChatMessage, 0, len(messages))
	for _, m := range messages {
		out = append(out, ChatMessage{Role: m.Role, Content: m.Text})
	}
	return out
}

// measuredInputTokens extracts the provider-reported prompt tokens from a
// chat round's usage metadata. A missing block or count yields 0 — the
// "never measured" sentinel, never a fabricated number.

func measuredInputTokens(usage *llm.UsageMetadata) int {
	if usage == nil || usage.InputTokens == nil {
		return 0
	}
	return *usage.InputTokens
}
