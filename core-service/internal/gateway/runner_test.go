package gateway

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/gateway/third_party/picoclaw/bus"
)

// ── D-B15: default-off gate ─────────────────────────────────────────────────

func TestDefaultSettingsPlanZeroChannels(t *testing.T) {
	settings := DefaultSettings()
	if settings.Enabled {
		t.Fatal("DefaultSettings must be disabled — the gateway ships wired but off")
	}
	if DefaultSettings() != (Settings{}) {
		t.Fatal("zero-value Settings must equal DefaultSettings: no hidden defaults")
	}
	if !credentialsAbsent(DefaultSettings().Feishu) || !credentialsAbsent(DefaultSettings().WeCom) {
		t.Fatal("default channel settings must carry no credentials")
	}
	if plans := planChannels(settings); len(plans) != 0 {
		t.Fatalf("default configuration planned %d channels; want zero", len(plans))
	}
	runner := NewRunner(DefaultSettings(), nil, nil)
	if err := runner.Start(context.Background()); err != nil {
		t.Fatalf("Start with default settings errored: %v", err)
	}
	if runner.ChannelsStarted() != 0 {
		t.Fatalf("default Start left %d channels running; want zero connections", runner.ChannelsStarted())
	}
}

// Enabled root with no credentials still starts nothing, but the plan carries
// the named skip reason so operators see WHY (D-B16 taxonomy at the edge).
// 渠道级显式开启但凭据未配：条目必须带 ErrCredentialsMissing 出现在计划里，
// 让运维在日志里看到「为什么这个渠道没起」而不是无声消失。
func TestEnabledWithoutCredentialsPlansSkips(t *testing.T) {
	settings := Settings{
		Enabled: true,
		Feishu:  ChannelSettings{Enabled: true},
		WeCom:   ChannelSettings{Enabled: true},
	}
	plans := planChannels(settings)
	if len(plans) != 2 {
		t.Fatalf("plans = %+v; want both channels attempted", plans)
	}
	for _, plan := range plans {
		if !errors.Is(plan.skipReason, ErrCredentialsMissing) {
			t.Fatalf("channel %s skip reason = %v; want ErrCredentialsMissing", plan.name, plan.skipReason)
		}
	}
}

func TestEnabledWithCredentialsPlansBoth(t *testing.T) {
	settings := Settings{
		Enabled: true,
		Feishu:  ChannelSettings{Enabled: true, AppID: "cli-x", AppSecret: "s"},
		WeCom:   ChannelSettings{Enabled: true, BotID: "b", Secret: "s"},
	}
	plans := planChannels(settings)
	if len(plans) != 2 || plans[0].skipReason != nil || plans[1].skipReason != nil {
		t.Fatalf("fully credentialed settings should plan two clean channels: %+v", plans)
	}
}

// ── D-B16: transport vs unbound are distinct error classes ─────────────────

type fakeResolver struct {
	err error
}

func (f fakeResolver) Resolve(_ context.Context, _ ChannelRef) (agenttools.Principal, error) {
	return agenttools.Principal{}, f.err
}

type fakeDispatcher struct{}

func (fakeDispatcher) Dispatch(_ context.Context, _ agenttools.Principal, _ bus.InboundMessage) ([]bus.OutboundMessage, error) {
	return nil, errors.New("dispatcher down")
}

func newPumpedRunner(resolver *IdentityResolver) *Runner {
	r := NewRunner(Settings{}, resolver, fakeDispatcher{})
	return r
}

func TestHandleInboundUnboundStaysUnbound(t *testing.T) {
	resolver := newTestResolver(ErrUnbound)
	r := newPumpedRunner(resolver)
	msg := bus.InboundMessage{Channel: "feishu"}
	msg.Context.SenderID = "ou-123"
	err := r.handleInbound(context.Background(), msg)
	if !errors.Is(err, ErrUnbound) {
		t.Fatalf("err=%v; want ErrUnbound in the chain", err)
	}
	if errors.Is(err, ErrTransport) {
		t.Fatalf("unbound must not be rebranded as transport failure: %v", err)
	}
}

func TestHandleInboundDispatchFailureIsTransport(t *testing.T) {
	resolver := newTestResolver(nil)
	r := newPumpedRunner(resolver)
	err := r.handleInbound(context.Background(), bus.InboundMessage{})
	if err == nil || !errors.Is(err, ErrTransport) {
		t.Fatalf("dispatch failure should be ErrTransport, got %v", err)
	}
	if errors.Is(err, ErrUnbound) {
		t.Fatalf("transport failure must not carry the unbound marker: %v", err)
	}
}

// ── vendor import direction guard (ADR-0026 §5) ────────────────────────────

func TestVendorDoesNotImportBusinessPackages(t *testing.T) {
	violations := scanThirdPartyImports(thirdPartyRoot(t))
	if len(violations) > 0 {
		t.Fatalf("vendored code must not import business packages (ADR-0026 §5):\n  %s",
			strings.Join(violations, "\n  "))
	}
}

// Reverse fixture lives beside the real tree so CI proves the scanner walks it.
func TestVendorImportGuardDetectsViolations(t *testing.T) {
	path := thirdPartyRoot(t) + "/picoclaw/channels/zz_fixture_business_import.go"
	fixture := "package channels\n\nimport _ \"github.com/lease-management-system/core-service/internal/repository\"\n"
	if err := writeFile(path, fixture); err != nil {
		t.Fatal(err)
	}
	defer removeFile(path)

	violations := scanThirdPartyImports(thirdPartyRoot(t))
	if len(violations) == 0 {
		t.Fatal("vendor import guard failed to detect a planted business import")
	}
}

// newTestResolver wires a resolver whose dependency fakes resolve without a
// database: the binding store yields the fixed user, the user reader an active
// editor, and the role repo one permission.
func newTestResolver(err error) *IdentityResolver {
	bindings := &fakeBindings{userID: "u-1", findErr: err}
	return NewIdentityResolver(bindings, &fakeUsers{user: activeUser()}, fakeRoles{})
}

func thirdPartyRoot(t *testing.T) string {
	t.Helper()
	dir, err := filepathAbs("third_party")
	if err != nil {
		t.Fatal(err)
	}
	return dir
}
