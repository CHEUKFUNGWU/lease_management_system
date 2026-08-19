package agentcore

import (
	"context"
	"fmt"

	"github.com/lease-management-system/core-service/internal/agenttools"
)

// ExecutionMode declares how a tool may be executed within a turn.
type ExecutionMode int

const (
	// Sequential is the default and the only mode allowed for any tool whose
	// level is not read: parallel execution would scramble idempotency and
	// audit ordering.
	Sequential ExecutionMode = iota
	// Parallel allows the loop to run this tool concurrently with others.
	Parallel
)

// UpdateFunc is the optional progress channel of a tool execution.
type UpdateFunc func(partial any)

// Tool is agenttools.ToolDefinition plus the two capabilities the existing
// registry lacks: an execution mode and a streaming progress callback.
type Tool interface {
	Descriptor() agenttools.ToolDescriptor
	Execute(ctx context.Context, call agenttools.ToolCall, onUpdate UpdateFunc) (agenttools.ToolResult, error)
	ExecutionMode() ExecutionMode
}

type definitionTool struct {
	def  agenttools.ToolDefinition
	mode ExecutionMode
}

func (d definitionTool) Descriptor() agenttools.ToolDescriptor { return d.def.Descriptor }

func (d definitionTool) Execute(ctx context.Context, call agenttools.ToolCall, _ UpdateFunc) (agenttools.ToolResult, error) {
	return d.def.Handler(ctx, call)
}

func (d definitionTool) ExecutionMode() ExecutionMode { return d.mode }

// FromDefinition adapts an existing tool definition into a Tool with the
// given execution mode. It fails at registration time (not at run time) when a
// non-read tool is declared Parallel.
func FromDefinition(d agenttools.ToolDefinition, mode ExecutionMode) (Tool, error) {
	if err := validateMode(d.Descriptor, mode); err != nil {
		return nil, err
	}
	return definitionTool{def: d, mode: mode}, nil
}

// MustTool is FromDefinition for call sites that assemble a fixed, tested
// tool set at process startup.
func MustTool(d agenttools.ToolDefinition, mode ExecutionMode) Tool {
	t, err := FromDefinition(d, mode)
	if err != nil {
		panic(err)
	}
	return t
}

// ValidateTools checks the whole set: no non-read tool may be Parallel and no
// name+version pair may repeat. It is the registration-time gate (ACORE-5).
func ValidateTools(tools []Tool) error {
	seen := make(map[string]struct{}, len(tools))
	for _, t := range tools {
		d := t.Descriptor()
		if err := validateMode(d, t.ExecutionMode()); err != nil {
			return err
		}
		key := d.Name + "@" + d.Version
		if _, dup := seen[key]; dup {
			return fmt.Errorf("agentcore: duplicate tool %s", key)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func validateMode(d agenttools.ToolDescriptor, mode ExecutionMode) error {
	if d.Level != agenttools.LevelRead && mode == Parallel {
		return fmt.Errorf("agentcore: tool %s is %s-level and must be Sequential", d.Name, d.Level)
	}
	return nil
}
