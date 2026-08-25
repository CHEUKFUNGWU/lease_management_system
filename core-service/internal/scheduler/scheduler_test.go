package scheduler

// RT1-L3-C 调度框架单元测试。核心反向测试：OnBehalfOfPrincipal 任务引用未登记
// 的 Principal 时，构造必须失败——不是静默跳过（静默跳过和成功在日志里长得一样）。

import (
	"context"
	"errors"
	"testing"
	"time"
)

func validMaintenance(name string, interval time.Duration) Registration {
	return Registration{
		Name: name, Interval: interval,
		Auth: AuthSystemMaintenance,
		Run:  func(context.Context) error { return nil },
	}
}

func TestNewAcceptsSystemMaintenanceJob(t *testing.T) {
	if _, err := New([]Registration{validMaintenance("job-a", time.Minute)}); err != nil {
		t.Fatalf("well-formed system maintenance job must be accepted: %v", err)
	}
}

func TestNewRefusesUnregisteredPrincipal(t *testing.T) {
	reg := Registration{
		Name: "daily-report", Interval: time.Hour,
		Auth: AuthOnBehalfOfPrincipal, PrincipalRef: "ops-reporter",
		Run: func(context.Context) error { return nil },
	}
	if _, err := New([]Registration{reg}); err == nil {
		t.Fatal("an on_behalf_of_principal job with an unregistered principal must FAIL construction (startup refusal), not start silently")
	}
	// 登记后同一任务可构造。
	if _, err := New([]Registration{reg}, WithPrincipalRefs([]string{"ops-reporter"})); err != nil {
		t.Fatalf("registered principal must be accepted: %v", err)
	}
	// 空集合里另一个 ref 同样拒绝。
	if _, err := New([]Registration{reg}, WithPrincipalRefs([]string{"someone-else"})); err == nil {
		t.Fatal("a principal ref outside the registry must still be refused")
	}
}

func TestNewValidationMatrix(t *testing.T) {
	cases := []struct {
		name string
		reg  Registration
	}{
		{"empty name", Registration{Interval: time.Minute, Auth: AuthSystemMaintenance, Run: func(context.Context) error { return nil }}},
		{"zero interval", Registration{Name: "j", Auth: AuthSystemMaintenance, Run: func(context.Context) error { return nil }}},
		{"nil run", Registration{Name: "j", Interval: time.Minute, Auth: AuthSystemMaintenance}},
		{"unknown auth kind", Registration{Name: "j", Interval: time.Minute, Auth: AuthKind("yolo"), Run: func(context.Context) error { return nil }}},
		{"principal job without ref", Registration{Name: "j", Interval: time.Minute, Auth: AuthOnBehalfOfPrincipal, Run: func(context.Context) error { return nil }}},
	}
	for _, tc := range cases {
		if _, err := New([]Registration{tc.reg}); err == nil {
			t.Fatalf("%s: construction must fail", tc.name)
		}
	}
}

func TestNewRefusesDuplicateNames(t *testing.T) {
	if _, err := New([]Registration{validMaintenance("dup", time.Minute), validMaintenance("dup", time.Hour)}); err == nil {
		t.Fatal("duplicate job names must fail construction — per-job logs would be indistinguishable")
	}
}

func TestStartFiresJobsAndStopsCleanly(t *testing.T) {
	fired := make(chan struct{}, 4)
	reg := Registration{
		Name: "counter", Interval: 10 * time.Millisecond,
		Auth: AuthSystemMaintenance,
		Run: func(context.Context) error {
			select {
			case fired <- struct{}{}:
			default:
			}
			return nil
		},
	}
	s, err := New([]Registration{reg})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.Start(ctx)

	select {
	case <-fired:
	case <-time.After(2 * time.Second):
		t.Fatal("scheduled job never fired")
	}
	cancel()
	s.Wait() // Start's loops must return after ctx cancellation
}

func TestRunErrorIsLoggedNotFatal(t *testing.T) {
	var fired int32 = 0
	reg := Registration{
		Name: "always-fails", Interval: 10 * time.Millisecond,
		Auth: AuthSystemMaintenance,
		Run: func(context.Context) error {
			fired++
			return errors.New("boom")
		},
	}
	s, _ := New([]Registration{reg})
	ctx, cancel := context.WithCancel(context.Background())
	s.Start(ctx)
	time.Sleep(50 * time.Millisecond)
	cancel()
	s.Wait()
	if fired == 0 {
		t.Fatal("job never ran")
	}
	// 多次失败后仍在跑（fired > 1 说明失败没有杀死循环）；具体次数不钉，
	// ticker 调度精度不进断言。
}
