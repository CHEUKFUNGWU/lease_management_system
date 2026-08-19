package agentcore

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/agenttools"
)

// TestAgentEndToEnd drives a complete run through Agent: Prompt, a tool
// round, subscriber collection and settlement.
func TestAgentEndToEnd(t *testing.T) {
	s := NewState()
	// A custom Tool so the progress callback is actually exercised — the
	// FromDefinition adapter cannot emit updates because legacy handlers have
	// no progress parameter.
	progressTool := toolWithUpdate(func(ctx context.Context, call agenttools.ToolCall, onUpdate UpdateFunc) agenttools.ToolResult {
		if onUpdate != nil {
			onUpdate("progress-1")
		}
		return agenttools.ToolResult{CallID: call.CallID, Status: agenttools.StatusCompleted}
	})
	s.SetTools([]Tool{progressTool})

	rounds := []round{
		{toolCalls: []agenttools.ToolCall{{CallID: "call-1", ToolName: "lease.contract.get", ToolVersion: "v1"}}, terminate: true},
		{end: "final answer", terminate: true},
	}
	idx := 0
	var mu sync.Mutex
	var names []string
	agent := New(Options{
		State: s,
		Deps: Deps{
			Stream: func(ctx context.Context, s *State) (StreamResult, error) {
				mu.Lock()
				r := rounds[idx]
				idx++
				mu.Unlock()
				return r.stream(ctx, s)
			},
		},
	})
	agent.Subscribe(func(ctx context.Context, ev Event) error {
		mu.Lock()
		defer mu.Unlock()
		switch ev.(type) {
		case ToolExecutionUpdate:
			names = append(names, "ToolExecutionUpdate")
		case ToolExecutionEnd:
			names = append(names, "ToolExecutionEnd")
		case AgentEnd:
			names = append(names, "AgentEnd")
		}
		return nil
	})

	if err := agent.Prompt(context.Background(), Message{Role: "user", Content: "get it"}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := agent.WaitForIdle(ctx); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if got := s.Messages(); len(got) != 2 || got[1].Content != "final answer" {
		t.Fatalf("expected final answer committed, got %+v", got)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(names) != 3 || names[0] != "ToolExecutionUpdate" || names[1] != "ToolExecutionEnd" || names[2] != "AgentEnd" {
		t.Fatalf("subscriber sequence mismatch: %v", names)
	}
}

// TestAgentCriticalSubscriberFailsRun is ACORE-3's audit half: a failing
// critical subscriber must fail the whole run, not be swallowed.
func TestAgentCriticalSubscriberFailsRun(t *testing.T) {
	s := NewState()
	rounds := []round{{end: "ok", terminate: true}}
	idx := 0
	agent := New(Options{
		State: s,
		Deps: Deps{
			Stream: func(ctx context.Context, s *State) (StreamResult, error) {
				r := rounds[idx]
				idx++
				return r.stream(ctx, s)
			},
		},
	})
	auditErr := errors.New("audit store down")
	agent.SubscribeCritical(func(ctx context.Context, ev Event) error {
		if _, ok := ev.(AgentEnd); ok {
			return auditErr
		}
		return nil
	})
	if err := agent.Prompt(context.Background(), Message{Role: "user", Content: "hi"}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := agent.WaitForIdle(ctx); !errors.Is(err, auditErr) {
		t.Fatalf("critical subscriber failure must fail the run, got %v", err)
	}
}

// TestAgentNonCriticalSubscriberErrorIsIgnored verifies the push-class
// subscriber does not fail the run.
func TestAgentNonCriticalSubscriberErrorIsIgnored(t *testing.T) {
	s := NewState()
	rounds := []round{{end: "ok", terminate: true}}
	idx := 0
	agent := New(Options{
		State: s,
		Deps: Deps{
			Stream: func(ctx context.Context, s *State) (StreamResult, error) {
				r := rounds[idx]
				idx++
				return r.stream(ctx, s)
			},
		},
	})
	agent.Subscribe(func(ctx context.Context, ev Event) error {
		return errors.New("sse push failed")
	})
	if err := agent.Prompt(context.Background(), Message{Role: "user", Content: "hi"}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := agent.WaitForIdle(ctx); err != nil {
		t.Fatalf("non-critical subscriber failure must not fail the run: %v", err)
	}
}

// TestAgentAbort is ACORE-6: abort stops the loop with context cancellation
// and WaitForIdle returns normally.
func TestAgentAbort(t *testing.T) {
	s := NewState()
	agent := New(Options{
		State: s,
		Deps: Deps{
			Stream: func(ctx context.Context, s *State) (StreamResult, error) {
				<-ctx.Done()
				return StreamResult{}, ctx.Err()
			},
		},
	})
	if err := agent.Prompt(context.Background(), Message{Role: "user", Content: "hi"}); err != nil {
		t.Fatal(err)
	}
	// Give the loop a moment to reach the stream wait, then abort.
	time.Sleep(20 * time.Millisecond)
	agent.Abort()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := agent.WaitForIdle(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("aborted run must settle with context.Canceled, got %v", err)
	}
}

// toolWithUpdate builds a Tool whose Execute forwards the progress callback.
type toolWithUpdate func(ctx context.Context, call agenttools.ToolCall, onUpdate UpdateFunc) agenttools.ToolResult

func (f toolWithUpdate) Descriptor() agenttools.ToolDescriptor {
	return agenttools.ToolDescriptor{Name: "lease.contract.get", Version: "v1", Level: agenttools.LevelRead}
}

func (f toolWithUpdate) Execute(ctx context.Context, call agenttools.ToolCall, onUpdate UpdateFunc) (agenttools.ToolResult, error) {
	return f(ctx, call, onUpdate), nil
}

func (f toolWithUpdate) ExecutionMode() ExecutionMode { return Sequential }

// TestAgentPromptWhileBusy rejects a second run before the first settles.
func TestAgentPromptWhileBusy(t *testing.T) {
	s := NewState()
	release := make(chan struct{})
	agent := New(Options{
		State: s,
		Deps: Deps{
			Stream: func(ctx context.Context, s *State) (StreamResult, error) {
				<-release
				return StreamResult{Terminate: true}, nil
			},
		},
	})
	if err := agent.Prompt(context.Background(), Message{Role: "user", Content: "hi"}); err != nil {
		t.Fatal(err)
	}
	if err := agent.Prompt(context.Background(), Message{Role: "user", Content: "again"}); !errors.Is(err, ErrAgentBusy) {
		t.Fatalf("second Prompt must fail with ErrAgentBusy, got %v", err)
	}
	close(release)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := agent.WaitForIdle(ctx); err != nil {
		t.Fatal(err)
	}
}

// TestAgentResetClearsRunState verifies Reset drops the conversation while
// keeping tools registered.
func TestAgentResetClearsRunState(t *testing.T) {
	s := NewState()
	s.SetMessages([]Message{{Role: "user", Content: "old"}})
	agent := New(Options{State: s})
	agent.Steer(Message{Role: "user", Content: "steer"})
	agent.FollowUp(Message{Role: "user", Content: "follow"})
	agent.Reset()
	if got := s.Messages(); len(got) != 0 {
		t.Fatalf("Reset must clear messages, got %+v", got)
	}
	if agent.HasQueued() {
		t.Fatal("Reset must clear queues")
	}
}

// TestAgentSteerFollowUpDrain verifies ACORE-8's queue semantics: steering
// drains before follow-up, and OneAtATime yields one message per drain.
func TestAgentSteerFollowUpDrain(t *testing.T) {
	agent := New(Options{State: NewState(), SteerMode: QueueOneAtATime, FollowUpMode: QueueAll})
	agent.Steer(Message{Role: "user", Content: "s1"})
	agent.Steer(Message{Role: "user", Content: "s2"})
	agent.FollowUp(Message{Role: "user", Content: "f1"})

	steered := agent.DrainSteering()
	if len(steered) != 1 || steered[0].Content != "s1" {
		t.Fatalf("OneAtATime steering must drain one message, got %+v", steered)
	}
	steered = agent.DrainSteering()
	if len(steered) != 1 || steered[0].Content != "s2" {
		t.Fatalf("second steering drain must return the next message, got %+v", steered)
	}
	followed := agent.DrainFollowUp()
	if len(followed) != 1 || followed[0].Content != "f1" {
		t.Fatalf("follow-up must drain all in QueueAll mode, got %+v", followed)
	}
	if agent.HasQueued() {
		t.Fatal("queues must be empty after drains")
	}
}
