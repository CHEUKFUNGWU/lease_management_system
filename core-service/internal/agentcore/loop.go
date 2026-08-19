package agentcore

import (
	"context"
	"errors"
	"time"

	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/errcontract"
)

// StreamFunc is the loop's single LLM I/O exit. It produces one round of
// assistant output: an optional streaming segment and zero or more tool calls.
type StreamFunc func(ctx context.Context, s *State) (StreamResult, error)

// StreamResult is one round of model output.
type StreamResult struct {
	// Start opens a new streaming assistant message.
	Start bool
	// Updates carries zero or more streaming deltas, in order.
	Updates []Message
	// End finalizes the assistant message for this round.
	End *Message
	// ToolCalls the model asked to execute, in order.
	ToolCalls []agenttools.ToolCall
	// Terminate reports that the model has finished its turn. A run ends when
	// a round terminates without tool calls, or when a tool result asks to
	// stop early.
	Terminate bool
}

// ErrStreamStalled is returned when a stream round neither terminates nor
// requests tool calls — continuing would be a busy loop.
var ErrStreamStalled = errors.New("agentcore: stream neither terminated nor requested tool calls")

// Deps is everything the loop needs, all injected. The loop imports no
// database, no HTTP, no repository and no object store — this purity is a
// contract enforced by importguard_test.go (ACORE-1).
type Deps struct {
	Stream      StreamFunc
	Before      BeforeToolCall
	After       AfterToolCall
	ShouldStop  func(ctx context.Context, s *State) bool
	PrepareNext func(ctx context.Context, s *State) error
	Emit        func(Event)
	Principal   agenttools.Principal
}

// Loop runs the agent until the stream terminates, a hook or stream error
// fails the run, the context is cancelled, or ShouldStop reports true.
//
// Event ordering contract: AgentStart, then per round: TurnStart, (MessageStart,
// MessageUpdate*, MessageEnd), (ToolExecutionStart, ToolExecutionUpdate*,
// ToolExecutionEnd)*, TurnEnd; finally AgentEnd.
//
// Failure semantics: a stream error, a before-hook error or an after-hook
// error fails the run (after-hooks are the audit seam and cannot be skipped);
// a tool execution error is recorded as a failed ToolResult and the loop
// continues, leaving the decision to the caller's PrepareNext/ShouldStop.
func Loop(ctx context.Context, s *State, d Deps) error {
	if s == nil {
		return errors.New("agentcore: nil state")
	}
	if d.Stream == nil {
		return errors.New("agentcore: Stream is required")
	}
	emit := d.Emit
	if emit == nil {
		emit = func(Event) {}
	}

	emit(AgentStart{})
	defer func() { emit(AgentEnd{Messages: s.Messages()}) }()

	for {
		if err := ctx.Err(); err != nil {
			s.setLastError(err)
			return err
		}

		emit(TurnStart{})
		sr, err := d.Stream(ctx, s)
		if err != nil {
			s.setLastError(err)
			return err
		}

		if sr.Start {
			s.beginStreaming()
			emit(MessageStart{Message: s.streamingCopy()})
		}
		for i := range sr.Updates {
			u := sr.Updates[i]
			s.appendDelta(&u)
			emit(MessageUpdate{Message: s.streamingCopy(), Delta: AssistantDelta{Content: u.Content}})
		}
		if sr.End != nil {
			s.commitStreaming(sr.End)
			emit(MessageEnd{Message: *sr.End})
		}

		results := make([]agenttools.ToolResult, 0, len(sr.ToolCalls))
		earlyStop := false
		for i := range sr.ToolCalls {
			res, hookErr := runToolCall(ctx, s, d, sr.ToolCalls[i], emit)
			results = append(results, res)
			if hookErr != nil {
				s.setLastError(hookErr)
				return hookErr
			}
			if res.Terminate {
				earlyStop = true
			}
		}
		emit(TurnEnd{Message: sr.End, ToolResults: results})

		if earlyStop {
			return nil
		}
		if len(sr.ToolCalls) == 0 && sr.Terminate {
			return nil
		}
		if len(sr.ToolCalls) == 0 && !sr.Terminate {
			s.setLastError(ErrStreamStalled)
			return ErrStreamStalled
		}
		if d.ShouldStop != nil && d.ShouldStop(ctx, s) {
			return nil
		}
		if d.PrepareNext != nil {
			if err := d.PrepareNext(ctx, s); err != nil {
				s.setLastError(err)
				return err
			}
		}
	}
}

// LoopContinue runs a continuation round. In this stage it is Loop; the
// distinction exists so W3/W4 can give continuation a different transform
// without changing caller code.
func LoopContinue(ctx context.Context, s *State, d Deps) error {
	return Loop(ctx, s, d)
}

func runToolCall(ctx context.Context, s *State, d Deps, call agenttools.ToolCall, emit func(Event)) (agenttools.ToolResult, error) {
	tool, ok := s.toolByName(call.ToolName, call.ToolVersion)
	if !ok {
		res := agenttools.ToolResult{
			CallID: call.CallID,
			Status: agenttools.StatusFailed,
			Error:  errcontract.New(errcontract.CodeSystemFailure, "tool not registered: "+call.ToolName),
		}
		emit(ToolExecutionEnd{CallID: call.CallID, ToolName: call.ToolName, Result: res, IsError: true})
		return res, nil
	}
	desc := tool.Descriptor()

	emit(ToolExecutionStart{CallID: call.CallID, ToolName: call.ToolName, Args: call.Arguments})
	s.markPending(call.CallID)
	defer s.unmarkPending(call.CallID)

	if d.Before != nil {
		bres, err := d.Before(ctx, BeforeContext{Call: call, Descriptor: desc, State: s, Principal: d.Principal})
		if err != nil {
			res := agenttools.ToolResult{
				CallID: call.CallID,
				Status: agenttools.StatusFailed,
				Error:  errcontract.New(errcontract.CodeSystemFailure, err.Error()),
			}
			emit(ToolExecutionEnd{CallID: call.CallID, ToolName: call.ToolName, Result: res, IsError: true})
			return res, err
		}
		if bres.Block {
			reason := bres.Reason
			if reason == "" {
				reason = "blocked by policy"
			}
			res := agenttools.ToolResult{
				CallID: call.CallID,
				Status: agenttools.StatusRejected,
				Error:  errcontract.New(errcontract.CodeScopeDenied, reason),
			}
			emit(ToolExecutionEnd{CallID: call.CallID, ToolName: call.ToolName, Result: res, IsError: false})
			return res, nil
		}
		if bres.Short != nil {
			res := *bres.Short
			if res.CallID == "" {
				res.CallID = call.CallID
			}
			emit(ToolExecutionEnd{CallID: call.CallID, ToolName: call.ToolName, Result: res, IsError: res.Status == agenttools.StatusFailed})
			return res, nil
		}
		if len(bres.Args) > 0 {
			call.Arguments = bres.Args
		}
	}

	onUpdate := func(partial any) {
		emit(ToolExecutionUpdate{CallID: call.CallID, ToolName: call.ToolName, Partial: partial})
	}
	startedAt := time.Now()
	result, execErr := tool.Execute(ctx, call, onUpdate)
	completedAt := time.Now()
	if execErr != nil {
		res := agenttools.ToolResult{
			CallID: call.CallID,
			Status: agenttools.StatusFailed,
			Error:  errcontract.New(errcontract.CodeBusinessFailure, execErr.Error()),
		}
		emit(ToolExecutionEnd{CallID: call.CallID, ToolName: call.ToolName, Result: res, IsError: true})
		return res, nil
	}

	if d.After != nil {
		if _, err := d.After(ctx, AfterContext{
			Call: call, Descriptor: desc, Result: &result, Err: nil, State: s, Principal: d.Principal,
			StartedAt: startedAt, CompletedAt: completedAt,
		}); err != nil {
			result.Status = agenttools.StatusFailed
			if result.Error == nil {
				result.Error = errcontract.New(errcontract.CodeSystemFailure, err.Error())
			}
			emit(ToolExecutionEnd{CallID: call.CallID, ToolName: call.ToolName, Result: result, IsError: true})
			return result, err
		}
	}

	emit(ToolExecutionEnd{CallID: call.CallID, ToolName: call.ToolName, Result: result, IsError: result.Status == agenttools.StatusFailed})
	return result, nil
}
