package aiagent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lease-management-system/core-service/internal/agentcore"
	"github.com/lease-management-system/core-service/internal/agenttools"
	agenttooldefs "github.com/lease-management-system/core-service/internal/agenttools/tools"
	"github.com/lease-management-system/core-service/internal/llm"
)

// agentFileLoop is the multi-round file pipeline (agent-universal-pagefill-v1
// P0-A③). The previous single callLLMWithTools round made the model's first
// tool call final; with this loop the model sees the triage result, picks a
// parse tool, reads its outcome and can then answer — or run one more parse.
// Scope stays 分诊 → 解析 → 建议目标页: only the parse/triage definitions
// participate, every hop goes through Runtime.Execute (policy/scope/audit
// identical to the single-turn path), and PrepareNext caps rounds.
type AgentLLMLoop struct {
	client    *llm.Client
	runtime   *agenttools.Runtime
	defs      []agenttools.ToolDefinition
	maxRounds int
}

func NewAgentLLMLoop(client *llm.Client, runtime *agenttools.Runtime) *AgentLLMLoop {
	return &AgentLLMLoop{client: client, runtime: runtime, maxRounds: 4}
}

// loopTool adapts a governed definition into an agentcore Tool.
type loopTool struct {
	def      agenttools.ToolDefinition
	runtime  *agenttools.Runtime
	onRecord func(toolName string, status string)
}

func (t loopTool) Descriptor() agenttools.ToolDescriptor { return t.def.Descriptor }

func (t loopTool) Execute(ctx context.Context, call agenttools.ToolCall, _ agentcore.UpdateFunc) (agenttools.ToolResult, error) {
	// The runtime policy gate requires a bound execution context; keep the
	// caller's identity (production always binds one).
	execution, execErr := agenttools.RequireExecutionContext(ctx)
	if execErr != nil {
		return agenttools.ToolResult{}, execErr
	}
	_ = execution
	result, err := t.runtime.Execute(ctx, agenttools.ToolCall{
		CallID:      call.CallID,
		RunID:       execution.RunID,
		TraceID:     execution.TraceID,
		ToolName:    call.ToolName,
		ToolVersion: "v1",
		Arguments:   call.Arguments,
		// 写类解析工具需要幂等键；读类字段会被忽略。与单轮路径
		// fileParseIdempotencyKey 同源——tool 名 + 文件身份。
		IdempotencyKey: "file-loop:" + call.ToolName + ":" + string(call.Arguments),
	})
	status := string(result.Status)
	t.onRecord(call.ToolName, status)
	if err != nil {
		return result, err
	}
	if result.Error != nil {
		return result, fmt.Errorf("%s", result.Error.Message)
	}
	return result, nil
}

func (t loopTool) ExecutionMode() agentcore.ExecutionMode { return agentcore.Sequential }

const fileResultMarker = "[tool-result] "

var errFileLoopExhausted = fmt.Errorf("file pipeline exceeded its tool-round budget")

// Run drives 分诊→解析→建议目标页 as a model-driven loop and returns the
// final assistant answer plus the tools that actually ran (response cards).
// conversation 可为 nil（单条 userMessage）或预组装的完整历史（AR3 口径：
// 最后一条必须是本轮 user 消息），历史照原样进模型输入。
func (l *AgentLLMLoop) Run(ctx context.Context, systemPrompt string, conversation []ChatMessage, userMessage string) (string, []AgentToolCall, error) {
	if l == nil || l.client == nil || strings.TrimSpace(l.client.Config().APIKey) == "" {
		return "", nil, llm.ErrNotConfigured
	}
	if _, ctxErr := agenttools.RequireExecutionContext(ctx); ctxErr != nil {
		return "", nil, ctxErr
	}
	chainMu := make(chan []AgentToolCall, 1)
	chainMu <- nil
	record := func(toolName, status string) {
		current := <-chainMu
		current = append(current, AgentToolCall{Tool: toolName, Skill: "LLM Function Calling", Status: status})
		chainMu <- current
	}
	loopTools := make([]agentcore.Tool, 0, len(l.defs))
	for _, def := range l.defs {
		loopTools = append(loopTools, loopTool{def: def, runtime: l.runtime, onRecord: record})
	}

	initial := make([]agentcore.Message, 0, len(conversation)+1)
	for _, m := range conversation {
		initial = append(initial, agentcore.Message{Role: m.Role, Content: m.Content})
	}
	if len(initial) == 0 || initial[len(initial)-1].Content != userMessage {
		initial = append(initial, agentcore.Message{Role: "user", Content: userMessage})
	}
	state := agentcore.NewState()
	state.SetSystemPrompt(systemPrompt)
	state.SetTools(loopTools)
	state.SetMessages(initial)

	rounds := 0
	err := agentcore.Loop(ctx, state, agentcore.Deps{
		Stream: l.client.StreamFunc(llm.StreamOptions{Temperature: 0.1, MaxTokens: 2000}),
		// The loop does not fold tool results into State itself: Emit hands
		// each round's results over here, and we append them as tool-role rows
		// so the next StreamFunc round reads them via Messages().
		Emit: func(event agentcore.Event) {
			turnEnd, ok := event.(agentcore.TurnEnd)
			if !ok {
				return
			}
			for _, res := range turnEnd.ToolResults {
				raw, mErr := json.Marshal(map[string]any{"call_id": res.CallID, "status": res.Status, "data": res.Data, "error": res.Error})
				if mErr != nil {
					continue
				}
				messages := append(state.Messages(), agentcore.Message{
					Role: "tool", Content: fileResultMarker + string(raw),
				})
				state.SetMessages(messages)
			}
		},
		PrepareNext: func(_ context.Context, _ *agentcore.State) error {
			rounds++
			if rounds > l.maxRounds {
				return errFileLoopExhausted
			}
			return nil
		},
	})
	return lastAssistantContent(state), <-chainMu, err
}

// lastAssistantContent returns the newest non-empty assistant message; an
// empty answer means the model never produced one within budget.
func lastAssistantContent(state *agentcore.State) string {
	answer := ""
	for _, m := range state.Messages() {
		if m.Role == "assistant" && strings.TrimSpace(m.Content) != "" {
			answer = m.Content
		}
	}
	return answer
}

// fileLoopFor lazily builds the loop with this agent's resolved LLM client
// and its full parse/triage definition surface. Nil defs means the bundle has
// not registered yet — callers get ErrNotConfigured semantics from Run.
func (h *Agent) fileLoopFor() *AgentLLMLoop {
	if h.fileLoop != nil {
		return h.fileLoop
	}
	client, clientErr := h.llm()
	if clientErr != nil {
		return nil
	}
	loop := NewAgentLLMLoop(client, h.toolRuntime)
	loop.defs = append(h.fileParseDefinitions(), agenttooldefs.NewDocTriageDefinition(nil))
	h.fileLoop = loop
	return loop
}

// RunFilePipeline is the production entry for the multi-round file path. It
// answers with the model's final summary and the tools that actually ran;
// err carries llm.ErrNotConfigured when no provider is set, so callers keep
// their existing deterministic fallback.
func (h *Agent) RunFilePipeline(ctx context.Context, systemPrompt string, conversation []ChatMessage, userMessage string) (string, []AgentToolCall, error) {
	loop := h.fileLoopFor()
	if loop == nil {
		return "", nil, llm.ErrNotConfigured
	}
	return loop.Run(ctx, systemPrompt, conversation, userMessage)
}
