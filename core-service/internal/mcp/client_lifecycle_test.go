package mcp

// RT1-L3-D 返工回归（复验票 R1–R4，全部真子进程假 server，须在 -race 下绿）。
//
//  1. TestClientSurvivesTimeoutThenAnswersNextCall —— 一次超时不得毒化 client；
//     第二次调用必须拿到「自己的」响应（旧实现：超时后读取 goroutine 不退出，
//     占住管道并与下一次调用的 reader 竞争 → 该 server 全部工具永久报废）。
//  2. TestConcurrentCallsDispatchByID —— 同一 client 并发调用按 id 正确分发
//     （旧实现：nextID 无锁 + 每 call 起一个并发 reader = DATA RACE；runtime
//     不串行化 handler，HTTP 并发即可触发）。
//  3. TestStartRefusesSilentServer —— server 进程起得来但握手不应答时，
//     Start 必须在 handshakeTimeout 内报错返回（旧实现：context.Background()
//     无限挂死，「fail-fast」不成立）。
//  4. TestOversizedResponseIsBoundedAndFailsFast —— 无换行洪泛在 maxLineBytes
//     处被截断（内存上界在读入时生效，不是解析后的事后检查），且 client 之后
//     fail-fast（旧实现：64 KiB cap 是事后检查，128 MiB 恶意响应实测吃掉
//     640 MiB 堆）。

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func writeScript(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

const initOK = `{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-03-26","capabilities":{"tools":{}},"serverInfo":{"name":"fake"}}}`

// R1: 超时一次 → 第二次调用仍拿到正确响应。变异自检：roundTrip 回到
// 「ctx.Done 直接 return、reader 不回收」的旧写法时本测试红（第二次调用要么
// 拿到陈旧响应要么 -race 报数据竞争）。
func TestClientSurvivesTimeoutThenAnswersNextCall(t *testing.T) {
	scriptPath := writeScript(t, "laggy_server.sh", `#!/bin/sh
read init_line
printf '%s\n' '`+initOK+`'
read note_line
read call1
sleep 0.5
printf '%s\n' '{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"late","inputSchema":{}}]}}'
read call2
printf '%s\n' '{"jsonrpc":"2.0","id":3,"result":{"tools":[{"name":"ontime","inputSchema":{}}]}}'
sleep 30
`)
	client, err := Start(ServerEntry{Name: "laggy", Command: scriptPath})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	ctx1, cancel1 := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel1()
	if _, err := client.ListTools(ctx1); err == nil {
		t.Fatal("expected first call to time out")
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	startedAt := time.Now()
	catalogue, err := client.ListTools(ctx2)
	elapsed := time.Since(startedAt)
	if err != nil {
		t.Fatalf("client unusable after one timeout (%v after %v)", err, elapsed)
	}
	if len(catalogue) != 1 || catalogue[0].Name != "ontime" {
		t.Fatalf("stale/misrouted response after timeout: got %+v after %v", catalogue, elapsed)
	}
	if elapsed > 3*time.Second {
		t.Fatalf("second call should not wait out the first one's late response, took %v", elapsed)
	}
}

// R2: 并发调用同一 client（RegisterAll 的真实形状：一个 server 一个 client 被
// 该 server 全部工具共享）。假 server 按请求 id 原样回显，断言两个调用者各拿
// 到自己 id 的应答——证明按 id 分发正确，而不只是竞争没炸。-race 下旧实现的
// nextID 自增与并发 ReadString 必报 DATA RACE。
func TestConcurrentCallsDispatchByID(t *testing.T) {
	scriptPath := writeScript(t, "echo_server.sh", `#!/bin/sh
read init_line
printf '%s\n' '`+initOK+`'
read note_line
while read line; do
  id=$(printf '%s' "$line" | sed -n 's/.*"id":\([0-9][0-9]*\).*/\1/p')
  printf '{"jsonrpc":"2.0","id":%s,"result":{"content":[{"type":"text","text":"reply-%s"}]}}\n' "$id" "$id"
done
`)
	client, err := Start(ServerEntry{Name: "echo", Command: scriptPath})
	if err != nil {
		t.Fatal(err)
	}

	const calls = 8
	results := make([]string, calls)
	errs := make([]error, calls)
	var wg sync.WaitGroup
	for i := 0; i < calls; i++ {
		wg.Add(1)
		go func(slot int) {
			defer wg.Done()
			result, err := client.CallTool(context.Background(), "t", json.RawMessage(`{}`))
			results[slot], errs[slot] = result.Text, err
		}(i)
	}
	wg.Wait()

	want := make(map[string]bool)
	seen := make(map[string]bool)
	for i := 0; i < calls; i++ {
		if errs[i] != nil {
			t.Fatalf("concurrent call %d failed: %v", i, errs[i])
		}
		if !strings.HasPrefix(results[i], "reply-") {
			t.Fatalf("call %d got a response not addressed to any request: %q", i, results[i])
		}
		seen[results[i]] = true
		want["reply-"+strconv.Itoa(i+2)] = true // ids allocated sequentially from 2 (1 = initialize)
	}
	for reply := range want {
		if !seen[reply] {
			t.Fatalf("missing correctly-addressed reply %q in %v", reply, seen)
		}
	}
	if len(seen) != calls {
		t.Fatalf("responses must be in one-to-one correspondence with requests, got %v", seen)
	}
}

// R3: 握手无响应 → Start 在 handshakeTimeout 内拒绝启动，不是无限挂死。
// 变异自检：initialize 回到 context.Background() 时本测试红（go test 超时）。
func TestStartRefusesSilentServer(t *testing.T) {
	scriptPath := writeScript(t, "silent_server.sh", "#!/bin/sh\nsleep 600\n")
	done := make(chan error, 1)
	go func() {
		client, err := Start(ServerEntry{Name: "silent", Command: scriptPath})
		if client != nil {
			_ = client.Close()
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("a server that never answers the handshake must refuse startup")
		}
	case <-time.After(handshakeTimeout + 3*time.Second):
		t.Fatalf("Start() still blocked after %s — no handshake deadline; startup would hang forever", handshakeTimeout+3*time.Second)
	}
}

// R4: 内存上界在「读入时」生效。128 MiB 无换行洪泛只允许消耗有界的增量分配，
// 报错后 client fail-fast。变异自检：readLine 退回无上限累积时本测试红
// （分配随恶意载荷线性增长，远超阈值）。
func TestOversizedResponseIsBoundedAndFailsFast(t *testing.T) {
	scriptPath := writeScript(t, "flood_server.sh", `#!/bin/sh
read init_line
printf '%s\n' '`+initOK+`'
read note_line
read call_line
printf '{"jsonrpc":"2.0","id":2,"result":{"content":[{"type":"text","text":"'
dd if=/dev/zero bs=1048576 count=128 2>/dev/null | tr '\0' 'A'
printf '"}]}}\n'
sleep 30
`)
	client, err := Start(ServerEntry{Name: "flood", Command: scriptPath})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	_, callErr := client.CallTool(context.Background(), "t", json.RawMessage(`{}`))
	runtime.ReadMemStats(&after)

	if callErr == nil || !strings.Contains(callErr.Error(), "response line exceeds") {
		t.Fatalf("oversized frame must be cut at maxLineBytes with a named error, got %v", callErr)
	}
	grewMiB := (after.TotalAlloc - before.TotalAlloc) / (1 << 20)
	if grewMiB > 32 {
		t.Fatalf("one hostile response allocated %d MiB despite the %d-byte line cap — cap is post-hoc again", grewMiB, maxLineBytes)
	}
	// 截断即传输层不可信：后续调用 fail-fast，不再等待。
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := client.CallTool(ctx, "t", json.RawMessage(`{}`)); err == nil || !strings.Contains(err.Error(), "unusable") {
		t.Fatalf("client must fail fast after a framing violation, got %v", err)
	}
}

// N1（复验二）：server 活着但不读 stdin → 内联写曾握着 writeMu 且不看 ctx，
// 1s deadline 的调用永久阻塞。现在写槽获取与写入本身都受 ctx 约束；部分写
// 弃置后传输层被毒化。变异自检：writeToStdin 退回无 deadline 直写时本测试红。
func TestCallToolHonoursDeadlineWhenServerStopsReadingStdin(t *testing.T) {
	scriptPath := writeScript(t, "deaf_server.sh", `#!/bin/sh
read init_line
printf '%s\n' '`+initOK+`'
sleep 120
`)
	client, err := Start(ServerEntry{Name: "deaf", Command: scriptPath})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	big := strings.Repeat("A", 2<<20) // 2 MiB：一次写就撑爆管道缓冲
	firstDone := make(chan error, 1)
	firstStart := time.Now()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_, callErr := client.CallTool(ctx, "t", json.RawMessage(`{"q":"`+big+`"}`))
		firstDone <- callErr
	}()
	select {
	case callErr := <-firstDone:
		if callErr == nil {
			t.Fatal("a wedged server must not yield a successful result")
		}
		if elapsed := time.Since(firstStart); elapsed > 3*time.Second {
			t.Fatalf("1s deadline honoured late: returned after %v (inline write ignored ctx)", elapsed)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("PROBE HIT: CallTool blocked >10s on a 1s deadline — the stdin write ignores ctx")
	}

	// 合流点断言：超时调用弃置后，同一 client 的下一次调用不得被写槽卡住。
	secondStart := time.Now()
	ctx2, cancel2 := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel2()
	if _, secondErr := client.CallTool(ctx2, "t", json.RawMessage(`{"q":"x"}`)); secondErr == nil {
		t.Fatal("expected the follow-up call to fail against a wedged server")
	} else if elapsed := time.Since(secondStart); elapsed > 3*time.Second {
		t.Fatalf("follow-up call stuck behind the abandoned writer for %v — write slot was not released/ctx-aware", elapsed)
	}
}

// N3（复验二）：server 主动请求（method+id）不得消费我们的 pending id。
// 变异自检：readLoop 去掉 Method!=空 continue 时本测试红——真响应被丢弃，
// 调用方拿到 nil result 的解析错误。
func TestServerInitiatedRequestDoesNotConsumeOurPendingID(t *testing.T) {
	scriptPath := writeScript(t, "chatty_server.sh", `#!/bin/sh
read init_line
printf '%s\n' '`+initOK+`'
read note_line
read list_line
printf '%s\n' '{"jsonrpc":"2.0","id":2,"method":"sampling/createMessage","params":{"messages":[]}}'
printf '%s\n' '{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"real","inputSchema":{}}]}}'
sleep 30
`)
	client, err := Start(ServerEntry{Name: "chatty", Command: scriptPath})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	catalogue, listErr := client.ListTools(ctx)
	if listErr != nil || len(catalogue) != 1 || catalogue[0].Name != "real" {
		t.Fatalf("server-initiated request consumed our pending id (err=%v, got %+v)", listErr, catalogue)
	}
}
