package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/errcontract"
)

type eventNames []string

type recorder struct {
	events []Event
	names  []string
}

func (r *recorder) record(ev Event) {
	r.events = append(r.events, ev)
	switch ev.(type) {
	case AgentStart:
		r.names = append(r.names, "AgentStart")
	case AgentEnd:
		r.names = append(r.names, "AgentEnd")
	case TurnStart:
		r.names = append(r.names, "TurnStart")
	case TurnEnd:
		r.names = append(r.names, "TurnEnd")
	case MessageStart:
		r.names = append(r.names, "MessageStart")
	case MessageUpdate:
		r.names = append(r.names, "MessageUpdate")
	case MessageEnd:
		r.names = append(r.names, "MessageEnd")
	case ToolExecutionStart:
		r.names = append(r.names, "ToolExecutionStart")
	case ToolExecutionUpdate:
		r.names = append(r.names, "ToolExecutionUpdate")
	case ToolExecutionEnd:
		r.names = append(r.names, "ToolExecutionEnd")
	}
}

func collectTool(t *testing.T, name string, execute func(ctx context.Context, call agenttools.ToolCall, onUpdate UpdateFunc) agenttools.ToolResult) Tool {
	def := agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{Name: name, Version: "v1", Level: agenttools.LevelRead},
		Handler: func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
			return execute(ctx, call, nil), nil
		},
	}
	tool, err := FromDefinition(def, Sequential)
	if err != nil {
		t.Fatal(err)
	}
	return tool
}

// round is a canned stream round.
type round struct {
	start     bool
	updates   []string
	end       string
	toolCalls []agenttools.ToolCall
	terminate bool
}

func (r round) stream(ctx context.Context, s *State) (StreamResult, error) {
	sr := StreamResult{Start: r.start, Terminate: r.terminate, ToolCalls: r.toolCalls}
	for _, u := range r.updates {
		sr.Updates = append(sr.Updates, Message{Role: "assistant", Content: u})
	}
	if r.end != "" {
		sr.End = &Message{Role: "assistant", Content: r.end}
	}
	return sr, nil
}

func TestLoopEventOrderSingleRound(t *testing.T) {
	s := NewState()
	rec := &recorder{}
	rounds := []round{
		{start: true, updates: []string{"hel", "lo"}, end: "hello", terminate: true},
	}
	idx := 0
	err := Loop(context.Background(), s, Deps{
		Stream: func(ctx context.Context, s *State) (StreamResult, error) {
			r := rounds[idx]
			idx++
			return r.stream(ctx, s)
		},
		Emit: rec.record,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"AgentStart", "TurnStart", "MessageStart", "MessageUpdate", "MessageUpdate", "MessageEnd", "TurnEnd", "AgentEnd"}
	if !reflect.DeepEqual(rec.names, want) {
		t.Fatalf("event order mismatch:\n got %v\nwant %v", rec.names, want)
	}
	if msgs := s.Messages(); len(msgs) != 1 || msgs[0].Content != "hello" {
		t.Fatalf("committed message mismatch: %+v", msgs)
	}
}

func TestLoopExecutesToolCallsAndContinues(t *testing.T) {
	s := NewState()
	tool := collectTool(t, "lease.contract.get", func(ctx context.Context, call agenttools.ToolCall, onUpdate UpdateFunc) agenttools.ToolResult {
		return agenttools.ToolResult{CallID: call.CallID, Status: agenttools.StatusCompleted, Data: map[string]any{"id": "c1"}}
	})
	s.SetTools([]Tool{tool})
	rec := &recorder{}
	rounds := []round{
		{toolCalls: []agenttools.ToolCall{{CallID: "call-1", ToolName: "lease.contract.get", ToolVersion: "v1", Arguments: json.RawMessage(`{}`)}}, terminate: true},
		{end: "done", terminate: true},
	}
	idx := 0
	err := Loop(context.Background(), s, Deps{
		Stream: func(ctx context.Context, s *State) (StreamResult, error) {
			r := rounds[idx]
			idx++
			return r.stream(ctx, s)
		},
		Emit: rec.record,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"AgentStart", "TurnStart", "ToolExecutionStart", "ToolExecutionEnd", "TurnEnd", "TurnStart", "MessageEnd", "TurnEnd", "AgentEnd"}
	if !reflect.DeepEqual(rec.names, want) {
		t.Fatalf("event order mismatch:\n got %v\nwant %v", rec.names, want)
	}
}

func TestLoopBeforeBlockShortCircuits(t *testing.T) {
	s := NewState()
	executed := false
	tool := collectTool(t, "lease.contract.get", func(ctx context.Context, call agenttools.ToolCall, onUpdate UpdateFunc) agenttools.ToolResult {
		executed = true
		return agenttools.ToolResult{Status: agenttools.StatusCompleted}
	})
	s.SetTools([]Tool{tool})
	rec := &recorder{}
	rounds := []round{
		{toolCalls: []agenttools.ToolCall{{CallID: "call-1", ToolName: "lease.contract.get", ToolVersion: "v1"}}, terminate: true},
		{end: "blocked", terminate: true},
	}
	idx := 0
	err := Loop(context.Background(), s, Deps{
		Stream: func(ctx context.Context, s *State) (StreamResult, error) {
			r := rounds[idx]
			idx++
			return r.stream(ctx, s)
		},
		Before: ChainBefore(func(ctx context.Context, bc BeforeContext) (BeforeResult, error) {
			return BeforeResult{Block: true, Reason: "scope denied"}, nil
		}),
		Emit: rec.record,
	})
	if err != nil {
		t.Fatal(err)
	}
	if executed {
		t.Fatal("blocked tool must not execute")
	}
	var found bool
	for _, ev := range rec.events {
		if end, ok := ev.(ToolExecutionEnd); ok {
			found = true
			if end.Result.Status != agenttools.StatusRejected {
				t.Fatalf("blocked call must be rejected, got %s", end.Result.Status)
			}
			if end.Result.Error == nil || end.Result.Error.Code != errcontract.CodeScopeDenied {
				t.Fatalf("blocked call must carry scope_denied error, got %+v", end.Result.Error)
			}
		}
	}
	if !found {
		t.Fatal("ToolExecutionEnd missing")
	}
}

func TestLoopBeforeShortResult(t *testing.T) {
	s := NewState()
	executed := false
	tool := collectTool(t, "lease.contract.draft.create", func(ctx context.Context, call agenttools.ToolCall, onUpdate UpdateFunc) agenttools.ToolResult {
		executed = true
		return agenttools.ToolResult{Status: agenttools.StatusCompleted}
	})
	s.SetTools([]Tool{tool})
	rec := &recorder{}
	rounds := []round{
		{toolCalls: []agenttools.ToolCall{{CallID: "call-1", ToolName: "lease.contract.draft.create", ToolVersion: "v1"}}, terminate: true},
		{end: "needs your review", terminate: true},
	}
	idx := 0
	err := Loop(context.Background(), s, Deps{
		Stream: func(ctx context.Context, s *State) (StreamResult, error) {
			r := rounds[idx]
			idx++
			return r.stream(ctx, s)
		},
		Before: ChainBefore(func(ctx context.Context, bc BeforeContext) (BeforeResult, error) {
			return BeforeResult{Short: &agenttools.ToolResult{Status: agenttools.StatusNeedsReview, Review: agenttools.ReviewResult{Required: true, Reasons: []string{"assist_mode"}}}}, nil
		}),
		Emit: rec.record,
	})
	if err != nil {
		t.Fatal(err)
	}
	if executed {
		t.Fatal("short-circuited tool must not execute")
	}
	for _, ev := range rec.events {
		if end, ok := ev.(ToolExecutionEnd); ok && end.Result.Status != agenttools.StatusNeedsReview {
			t.Fatalf("short-circuit must surface needs_review, got %s", end.Result.Status)
		}
	}
}

func TestLoopAfterHookErrorFailsRun(t *testing.T) {
	s := NewState()
	tool := collectTool(t, "lease.contract.get", func(ctx context.Context, call agenttools.ToolCall, onUpdate UpdateFunc) agenttools.ToolResult {
		return agenttools.ToolResult{Status: agenttools.StatusCompleted}
	})
	s.SetTools([]Tool{tool})
	rec := &recorder{}
	rounds := []round{{toolCalls: []agenttools.ToolCall{{CallID: "call-1", ToolName: "lease.contract.get", ToolVersion: "v1"}}, terminate: true}}
	idx := 0
	err := Loop(context.Background(), s, Deps{
		Stream: func(ctx context.Context, s *State) (StreamResult, error) {
			r := rounds[idx]
			idx++
			return r.stream(ctx, s)
		},
		After: ChainAfter(func(ctx context.Context, ac AfterContext) (AfterResult, error) {
			return AfterResult{}, errors.New("audit write failed")
		}),
		Emit: rec.record,
	})
	if err == nil {
		t.Fatal("after-hook error must fail the run")
	}
}

func TestLoopToolExecutionErrorContinuesRun(t *testing.T) {
	s := NewState()
	tool := collectTool(t, "lease.contract.get", func(ctx context.Context, call agenttools.ToolCall, onUpdate UpdateFunc) agenttools.ToolResult {
		return agenttools.ToolResult{Status: agenttools.StatusFailed, Error: errcontract.New(errcontract.CodeBusinessFailure, "boom")}
	})
	s.SetTools([]Tool{tool})
	rec := &recorder{}
	rounds := []round{
		{toolCalls: []agenttools.ToolCall{{CallID: "call-1", ToolName: "lease.contract.get", ToolVersion: "v1"}}, terminate: true},
		{end: "recovered", terminate: true},
	}
	idx := 0
	err := Loop(context.Background(), s, Deps{
		Stream: func(ctx context.Context, s *State) (StreamResult, error) {
			r := rounds[idx]
			idx++
			return r.stream(ctx, s)
		},
		Emit: rec.record,
	})
	if err != nil {
		t.Fatalf("tool execution failure is data, not a run failure: %v", err)
	}
	if got := s.Messages(); len(got) != 1 || got[0].Content != "recovered" {
		t.Fatalf("run must continue after failed tool, messages=%+v", got)
	}
}

func TestLoopStalledStreamFails(t *testing.T) {
	s := NewState()
	err := Loop(context.Background(), s, Deps{
		Stream: func(ctx context.Context, s *State) (StreamResult, error) {
			return StreamResult{}, nil
		},
	})
	if !errors.Is(err, ErrStreamStalled) {
		t.Fatalf("expected ErrStreamStalled, got %v", err)
	}
}

func TestLoopToolResultTerminateStopsEarly(t *testing.T) {
	s := NewState()
	tool := collectTool(t, "lease.contract.get", func(ctx context.Context, call agenttools.ToolCall, onUpdate UpdateFunc) agenttools.ToolResult {
		return agenttools.ToolResult{Status: agenttools.StatusCompleted, Terminate: true}
	})
	s.SetTools([]Tool{tool})
	rounds := []round{
		{toolCalls: []agenttools.ToolCall{{CallID: "call-1", ToolName: "lease.contract.get", ToolVersion: "v1"}}, terminate: false},
	}
	idx := 0
	err := Loop(context.Background(), s, Deps{
		Stream: func(ctx context.Context, s *State) (StreamResult, error) {
			r := rounds[idx]
			idx++
			return r.stream(ctx, s)
		},
	})
	if err != nil {
		t.Fatalf("early stop is not an error: %v", err)
	}
	if idx != 1 {
		t.Fatalf("loop must stop after terminate, stream called %d times", idx)
	}
}

func TestLoopContextCancellation(t *testing.T) {
	s := NewState()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := Loop(ctx, s, Deps{
		Stream: func(ctx context.Context, s *State) (StreamResult, error) {
			return StreamResult{Terminate: true}, nil
		},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestLoopPrepareNextRunsBetweenTurns(t *testing.T) {
	s := NewState()
	rounds := []round{
		{toolCalls: []agenttools.ToolCall{{CallID: "call-1", ToolName: "missing.tool", ToolVersion: "v1"}}, terminate: true},
		{end: "after", terminate: true},
	}
	idx := 0
	var prepared int
	err := Loop(context.Background(), s, Deps{
		Stream: func(ctx context.Context, s *State) (StreamResult, error) {
			r := rounds[idx]
			idx++
			return r.stream(ctx, s)
		},
		PrepareNext: func(ctx context.Context, s *State) error {
			prepared++
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if prepared != 1 {
		t.Fatalf("PrepareNext must run once between two turns, got %d", prepared)
	}
}

func TestLoopPrepareNextErrorFailsRun(t *testing.T) {
	s := NewState()
	rounds := []round{{toolCalls: []agenttools.ToolCall{{CallID: "c", ToolName: "missing.tool"}}, terminate: true}}
	idx := 0
	want := errors.New("transform failed")
	err := Loop(context.Background(), s, Deps{
		Stream: func(ctx context.Context, s *State) (StreamResult, error) {
			r := rounds[idx]
			idx++
			return r.stream(ctx, s)
		},
		PrepareNext: func(ctx context.Context, s *State) error { return want },
	})
	if !errors.Is(err, want) {
		t.Fatalf("expected PrepareNext error to fail run, got %v", err)
	}
}

func TestLoopStreamErrorFailsRun(t *testing.T) {
	s := NewState()
	want := errors.New("llm down")
	err := Loop(context.Background(), s, Deps{
		Stream: func(ctx context.Context, s *State) (StreamResult, error) {
			return StreamResult{}, want
		},
	})
	if !errors.Is(err, want) {
		t.Fatalf("expected stream error to fail run, got %v", err)
	}
}
