package handlers

import (
	"testing"

	"github.com/lease-management-system/core-service/internal/aiagent"
	"github.com/lease-management-system/core-service/internal/aichat"
)

// AR5-G1 汇流断言（ADR-0028 §5）：生产 chat 入口必须实际经过新内核，
// 而不是 aichat.Runtime 的旧执行器。G1 悬置数月，正是因为从来没有这样一条
// 机器检查——agentcore.New 零调用这件事，靠读代码没人发现。
//
// 断言分两层：
//  1. 生产构造函数（与 cmd/api/main.go:137 唯一接线点同一条代码路径）装出的
//     runtime，其 executor 是内核适配器 *chatexec.Executor；
//  2. 反向对照：把旧执行器形状（executor=aiagent.Agent）喂给同一个判别器，
//     必须得到不同的答案——否则本断言恒真，什么也证明不了
//     （风险红线 12：拿别名比自己不算检查）。
//
// 行为层证据（生产装配下工具调用真实穿越 HookManager 派发、九控制逐项
// 红→绿→红）在 internal/agentkernel/chatexec 的 mutation_production_test.go。
func TestProductionChatWiringRunsThroughKernelExecutor(t *testing.T) {
	handler := NewAIChatHandlerWithOperationalReadersAndGovernanceAndRetail(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
	if handler == nil || handler.agentRuntime == nil {
		t.Fatal("production constructor produced no runtime")
	}
	const kernelExecutor = "*chatexec.Executor"
	if got := handler.agentRuntime.ExecutorKind(); got != kernelExecutor {
		t.Fatalf("production chat wiring executes through %q, want %q — the convergence was reverted or bypassed", got, kernelExecutor)
	}
}

func TestExecutorKindDiscriminatesLegacyWiring(t *testing.T) {
	// The legacy shape — executor = the agent itself — is exactly what the
	// production line looked like before AR5d. Feeding it to the same
	// discriminator must NOT satisfy the kernel assertion.
	agent := aiagent.NewWithOperationalReadersAndGovernanceAndRetail(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
	legacyRuntime := aichat.NewRuntime[aiagent.Response](nil, agent, agent, aiagent.ProjectResult, aichat.Options{})
	if got, kernel := legacyRuntime.ExecutorKind(), "*chatexec.Executor"; got == kernel {
		t.Fatalf("legacy wiring reports %q — the G1 discriminator cannot distinguish old from new", got)
	}
	if got, want := legacyRuntime.ExecutorKind(), "*aiagent.Agent"; got != want {
		t.Fatalf("legacy executor kind = %q, want %q", got, want)
	}
}
