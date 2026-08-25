package llm

// RT1-L3-B 信号语义测试。复验缺陷的根因是 LastCallSignal 用 time.Unix(0, nanos)
// 构造：未调用时 nanos=0 → 返回 1970-01-01，消费者的 IsZero 判据失效，
// defaultLLMState 永远落到 ok——「unknown」成为不可达状态。修在源头：
// nanos==0 必须转换为真正的零值 time.Time。变异自检：去掉 nanosToTime 的
// 零值转换，本文件与 handlers 的 TestDefaultLLMStateUnknownBeforeAnyRealCall
// 都必须变红。

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lease-management-system/core-service/internal/agentcore"
)

func resetSignal() { ResetCallSignal() }

func TestLastCallSignalTrueZeroBeforeAnyCall(t *testing.T) {
	resetSignal()
	s := LastCallSignal()
	if !s.LastSuccess.IsZero() || !s.LastFailure.IsZero() {
		t.Fatalf("before any real call both fields must be true zero time (IsZero-safe), got success=%v failure=%v — a 1970 timestamp here makes 'unknown' unreachable and fresh boots report ok", s.LastSuccess, s.LastFailure)
	}
}

func TestSignalRecordsOutcomes(t *testing.T) {
	resetSignal()
	recordCallFailure()
	s := LastCallSignal()
	if s.LastFailure.IsZero() || !s.LastSuccess.IsZero() {
		t.Fatalf("failure-only history must set failure and leave success zero, got %+v", s)
	}
	recordCallSuccess()
	s = LastCallSignal()
	if s.LastSuccess.IsZero() || s.LastFailure.IsZero() {
		t.Fatalf("both outcomes must be recorded once both happened, got %+v", s)
	}
}

// 覆盖声明钉住：StreamFunc（agentcore 交互循环的唯一入口）委托 Chat，信号必须
// 同样记录——否则注释宣称的全漏斗覆盖就是没有测试的断言。
func TestStreamFuncRecordsCall(t *testing.T) {
	resetSignal()
	var upstream http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"pong"}}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`))
	}
	srv := httptest.NewServer(upstream)
	defer srv.Close()

	client := &Client{cfg: Config{APIKey: "k", BaseURL: srv.URL, Model: "test-model"}, httpc: srv.Client()}
	state := agentcore.NewState()
	state.SetMessages([]agentcore.Message{{Role: "user", Content: "ping"}})

	streamFn := client.StreamFunc(StreamOptions{})
	if _, err := streamFn(context.Background(), state); err != nil {
		t.Fatalf("stream round failed: %v", err)
	}
	s := LastCallSignal()
	if s.LastSuccess.IsZero() {
		t.Fatal("a stream-func round must record into the health signal — the funnel claim (all provider I/O via Client.chat) is otherwise unproven")
	}
}
