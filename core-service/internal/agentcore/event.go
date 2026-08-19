package agentcore

import (
	"encoding/json"

	"github.com/lease-management-system/core-service/internal/agenttools"
)

// Event is the union of everything the loop can emit. Subscribers receive
// events in a fixed, deterministic order; see Loop for the ordering contract.
type Event interface{ isAgentEvent() }

// AgentStart marks the beginning of a run.
type AgentStart struct{}

// AgentEnd is emitted last, carrying the final message list. A run is not
// considered settled until every subscriber has returned from processing
// AgentEnd — see Agent.WaitForIdle.
type AgentEnd struct{ Messages []Message }

// TurnStart marks the beginning of one stream round.
type TurnStart struct{}

// TurnEnd closes a stream round with the assistant message (if any) and the
// tool results produced in that round.
type TurnEnd struct {
	Message     *Message
	ToolResults []agenttools.ToolResult
}

// MessageStart is emitted when an assistant message begins streaming.
type MessageStart struct{ Message Message }

// MessageUpdate carries one streaming delta.
type MessageUpdate struct {
	Message Message
	Delta   AssistantDelta
}

// AssistantDelta is the incremental content of a streaming update.
type AssistantDelta struct {
	Content string `json:"content"`
}

// MessageEnd is emitted when an assistant message is final.
type MessageEnd struct{ Message Message }

// ToolExecutionStart is emitted before a tool call executes, after parameter
// validation, before the before-hook chain runs.
type ToolExecutionStart struct {
	CallID   string
	ToolName string
	Args     json.RawMessage
}

// ToolExecutionUpdate carries an optional progress payload from a tool.
type ToolExecutionUpdate struct {
	CallID   string
	ToolName string
	Partial  any
}

// ToolExecutionEnd is emitted once per tool call, however it ended: executed,
// blocked by a before-hook, short-circuited, or failed.
type ToolExecutionEnd struct {
	CallID   string
	ToolName string
	Result   agenttools.ToolResult
	IsError  bool
}

func (AgentStart) isAgentEvent()          {}
func (AgentEnd) isAgentEvent()            {}
func (TurnStart) isAgentEvent()           {}
func (TurnEnd) isAgentEvent()             {}
func (MessageStart) isAgentEvent()        {}
func (MessageUpdate) isAgentEvent()       {}
func (MessageEnd) isAgentEvent()          {}
func (ToolExecutionStart) isAgentEvent()  {}
func (ToolExecutionUpdate) isAgentEvent() {}
func (ToolExecutionEnd) isAgentEvent()    {}
