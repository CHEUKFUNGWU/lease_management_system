package aichat

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/sessionmanager"
)

// SI1 Part B 验收：同一会话并发两条消息，断言不交叉执行。
//
// Acquire 的独占租约让第二个 run 在 prepare（会话加载）阶段被阻塞，直到
// 第一个 run 完成执行并释放。这是 D-C4 / Story 13 的既定意图，但是真实
// 行为变更（旧路径两条消息可交叉）。
//
// 确定性判别（不走调度窗口）：租约的实质是「第二个 run 的事件流不可能在
// 第一个 run 终结之前开始」。断言：并发两 run 完成后，后终结者（late）的
// 首个事件 CreatedAt 不早于先终结者（early）的终结事件 CreatedAt。
// 无租约时两个 prepare 几乎同时完成，后者的首个事件必然早于前者的终结
// （SPIN_HOT 制造时间窗），断言红。
//
// 跑 make test-integration 实跑；skip 不算证据。
func TestSameSessionConcurrentRunsSerialize(t *testing.T) {
	pool := postgresPool(t)
	baseCtx := context.Background()

	entityA, _, userID, sessionID, _ := seedContinuationTenant(t, baseCtx, pool)

	repo := repository.NewAIChatRuntimeRepository(pool)
	mgr := sessionmanager.New(sessionmanager.NewPostgresStore(pool), sessionmanager.Policy{})
	defer mgr.Stop()
	owner := NewSessionOwner(mgr, repo)

	runtime := newRuntime(
		repo,
		PlannerFunc(func(Input, *repository.AIChatRun) Plan { return Plan{AgentMode: true} }),
		ExecutorFunc[testResponse](func(_ context.Context, _ Execution) (testResponse, error) {
			spinHot() // 让第一个 run 真正占用一段时间；第二个若没被租约挡住会同时执行
			return testResponse{Answer: "done", Model: "test"}, nil
		}),
		func(response testResponse) Result {
			return Result{Answer: response.Answer, Model: response.Model}
		},
		Options{Dispatch: func(task func()) { task() }},
	).WithSessionOwner(owner)

	ctx := access.WithScope(baseCtx, access.Scope{LegalEntityID: entityA})

	completed := make([]*Completed[testResponse], 2)
	var wg sync.WaitGroup
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func(idx int) {
			defer wg.Done()
			c, _ := runtime.Run(ctx, Input{
				SessionID: sessionID, Message: fmt.Sprintf("第%d条消息", idx+1), UserID: userID,
			})
			completed[idx] = c
		}(i)
	}
	wg.Wait()

	if completed[0] == nil || completed[1] == nil {
		t.Fatalf("runs did not complete: %+v / %+v", completed[0], completed[1])
	}

	// 每个 run 的终结事件时间。
	ends := [2]time.Time{}
	for i := 0; i < 2; i++ {
		events, err := repo.ListRunEvents(context.Background(), completed[i].Started.Run.ID, 0, 200)
		if err != nil || len(events) == 0 {
			t.Fatalf("run%d events: %v (n=%d)", i+1, err, len(events))
		}
		for _, ev := range events {
			if ev.IsTerminal && ev.CreatedAt.After(ends[i]) {
				ends[i] = ev.CreatedAt
			}
		}
		if ends[i].IsZero() {
			t.Fatalf("run%d has no terminal event", i+1)
		}
	}

	// early / late：按终结时间排序。
	earlyIdx, lateIdx := 0, 1
	if ends[1].Before(ends[0]) {
		earlyIdx, lateIdx = 1, 0
	}

	// 后终结者的首个事件时间。
	lateEvents, err := repo.ListRunEvents(context.Background(), completed[lateIdx].Started.Run.ID, 0, 1)
	if err != nil || len(lateEvents) == 0 {
		t.Fatalf("late run first event: %v (n=%d)", err, len(lateEvents))
	}
	lateFirst := lateEvents[0].CreatedAt

	// 无租约时：late run 的 prepare 与 early run 并行完成，其首事件必然
	// 早于 early run 的终结（spinHot 时间窗）。有租约时：strictly after。
	if lateFirst.Before(ends[earlyIdx]) {
		t.Fatalf("两 run 交叉：后终结者首事件 %v 早于先终结者终结 %v —— 独占租约未生效\n"+
			"run早=%s run晚=%s end早=%v end晚=%v",
			lateFirst, ends[earlyIdx], completed[earlyIdx].Started.Run.ID, completed[lateIdx].Started.Run.ID,
			ends[earlyIdx], ends[lateIdx])
	}
}

// spinHot keeps the goroutine on-core so the first run occupies a real time
// window (scheduling-independent: the lease blocks at prepare, not here).
func spinHot() {
	var sink int32
	for i := 0; i < 2000000; i++ {
		sink++
	}
}
