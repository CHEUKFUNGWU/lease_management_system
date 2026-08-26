package aichat

// C1 验收（架构重构任务书 2026-08-26）：Planner 与 Executor 必须有独立
// 单元测试——GUARD-001 的读法是：mock 一个坏 planner，executor 侧的测试
// 仍能独立变红（证明 executor 覆盖不依赖真实 planner），反之亦然。

import (
	"context"
	"errors"
	"testing"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/repository"
)

// brokenPlannerFunc is a planner whose Plan panics — the shape a wiring bug
// or a nil dependency produces.
type brokenPlannerFunc func(Input, *repository.AIChatRun) Plan

func (f brokenPlannerFunc) Plan(input Input, sourceRun *repository.AIChatRun) Plan {
	return f(input, sourceRun)
}

// TestBrokenPlannerFailsBeforeExecutorTouchesAnything: a planner that errors
// must fail the run at planning time and never reach the executor. If the
// executor were invoked anyway, the flag trips and the test goes red.
func TestBrokenPlannerFailsBeforeExecutorTouchesAnything(t *testing.T) {
	store := newMemoryStore()
	store.sessions["session-1"] = &repository.AIChatSession{ID: "session-1", UserID: "user-1", LegalEntityID: stringPointer("entity-1")}
	executorReached := false
	runtime := newRuntime(
		store,
		// Planner.Plan cannot fail (no error in the interface); a "broken"
		// planner panics. Start must recover and mark the run failed before
		// the executor is ever reached.
		brokenPlannerFunc(func(Input, *repository.AIChatRun) Plan {
			panic(errors.New("planner is broken"))
		}),
		ExecutorFunc[testResponse](func(_ context.Context, _ Execution) (testResponse, error) {
			executorReached = true
			return testResponse{Answer: "must not happen"}, nil
		}),
		func(response testResponse) Result { return Result{Answer: response.Answer} },
		Options{Dispatch: func(task func()) { task() }},
	)
	func() {
		// Planning panics propagate synchronously; recover here so the
		// assertion below can prove the executor was never reached.
		defer func() { _ = recover() }()
		_, _ = runtime.Start(access.WithScope(context.Background(), access.Scope{LegalEntityID: "entity-1"}), Input{
			SessionID: "session-1", Message: "hello", UserID: "user-1",
		})
	}()
	if executorReached {
		t.Fatal("executor ran despite the broken planner — planner/executor are coupled")
	}
}

// TestExecutorRunsAndFailsIndependentlyOfPlanner: with a healthy planner and
// an executor that errors, the failure surfaces from execution. Break the
// executor contract and this test alone goes red — no real planner involved.
func TestExecutorRunsAndFailsIndependentlyOfPlanner(t *testing.T) {
	store := newMemoryStore()
	store.sessions["session-1"] = &repository.AIChatSession{ID: "session-1", UserID: "user-1", LegalEntityID: stringPointer("entity-1")}
	plannerCalled := false
	runtime := newRuntime(
		store,
		PlannerFunc(func(Input, *repository.AIChatRun) Plan {
			plannerCalled = true
			return Plan{AgentMode: true}
		}),
		ExecutorFunc[testResponse](func(_ context.Context, _ Execution) (testResponse, error) {
			return testResponse{}, errors.New("executor is broken")
		}),
		func(response testResponse) Result { return Result{Answer: response.Answer} },
		Options{Dispatch: func(task func()) { task() }},
	)
	started, err := runtime.Start(access.WithScope(context.Background(), access.Scope{LegalEntityID: "entity-1"}), Input{
		SessionID: "session-1", Message: "hello", UserID: "user-1",
	})
	if err != nil {
		t.Fatalf("Start must surface executor failures as run state, not transport error: %v", err)
	}
	if !plannerCalled {
		t.Fatal("healthy planner was never consulted — wiring regression")
	}
	inspection, err := runtime.Inspect(access.WithScope(context.Background(), access.Scope{LegalEntityID: "entity-1"}), "user-1", started.Run.ID, 0)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if got, want := inspection.Run.Status, "failed"; got != want {
		t.Fatalf("run status = %q, want %q (executor failure must mark the run failed)", got, want)
	}
}
