package handlers

import (
	"testing"

	"github.com/lease-management-system/core-service/internal/aichat"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/sessionmanager"
)

// SI1 Part B 汇流断言（照 AR5-G1 的 ExecutorKind 模式）：生产 chat 的会话
// 创建与加载必须确实经过 sessionmanager，而不是停留在旧
// store.CreateSession / GetSessionByID 路径。G9 悬置数月的教训就是：没有
// 机器断言，「经过 AR2」只是读代码时的良好愿望。
//
// 断言分两层（与 ai_chat_kernel_convergence_test.go 同构）：
//  1. 带真实 repo 装配后，SessionOwnerKind() 报告 *aichat.sessionOwnerAdapter；
//  2. 反向对照：不接 seam 的 runtime 报告空串——若判别器把未接线也报成适配器，
//     断言恒真，什么也证明不了（风险红线 12：拿别名比自己不算检查）。
//
// 注意：production 构造函数传入 nil runtimeRepo 时 WithSessionOwner 无法接线
// （需要具体 repo 做内容双读），所以这里用真实 repo 构造——与 cmd/api/main.go
// 的唯一接线点同一条代码路径。
func TestProductionChatWiringRunsThroughSessionOwner(t *testing.T) {
	repo := &repository.AIChatRuntimeRepository{}
	manager := sessionmanager.New(nil, sessionmanager.Policy{}) // store nil: 仅在装配层测试，不触库
	owner := aichat.NewSessionOwner(manager, repo)
	runtime := aichat.NewRuntime[testAIResponse](
		nil, nil, nil, nil, aichat.Options{},
	).WithSessionOwner(owner)

	const adapterKind = "*aichat.sessionOwnerAdapter"
	if got := runtime.SessionOwnerKind(); got != adapterKind {
		t.Fatalf("session owner kind = %q, want %q — the AR2 wiring was reverted or bypassed", got, adapterKind)
	}
}

// 反向对照：未接 seam 的 runtime（legacy store 路径）绝不能报告适配器类型，
// 否则本断言恒真。
func TestSessionOwnerKindDiscriminatesLegacyWiring(t *testing.T) {
	runtime := aichat.NewRuntime[testAIResponse](
		nil, nil, nil, nil, aichat.Options{},
	)
	if got, adapter := runtime.SessionOwnerKind(), "*aichat.sessionOwnerAdapter"; got == adapter {
		t.Fatalf("legacy wiring reports %q — the SI1 discriminator cannot tell old from new", got)
	}
	if got, want := runtime.SessionOwnerKind(), ""; got != want {
		t.Fatalf("legacy session owner kind = %q, want %q", got, want)
	}
}

type testAIResponse struct{}
