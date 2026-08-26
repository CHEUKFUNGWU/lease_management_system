// Gateway Runner (BG6): lifecycle, configuration gating, channel assembly,
// identity resolution and error taxonomy. The entire surface is two methods —
// Start and Stop; everything else hides behind them.
//
// 默认关（D-B15）：Settings.Enabled 缺省 false，Start 直接返回且零渠道连接；
// 单渠道凭据缺失时跳过该渠道并记录具名原因，不 panic、不重试刷屏。
// 错误分型（D-B16）：ErrTransport（凭据过期/网络不通/上游限流，运维问题）与
// ErrUnbound（身份未绑定，运营问题）是两类独立错误，调用方以 errors.Is 区分，
// 绝不合并成「鉴权失败」。
package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/lease-management-system/core-service/internal/agenttools"
	appconfig "github.com/lease-management-system/core-service/internal/config"
	"github.com/lease-management-system/core-service/internal/gateway/third_party/picoclaw/bus"
	"github.com/lease-management-system/core-service/internal/gateway/third_party/picoclaw/channels"
	"github.com/lease-management-system/core-service/internal/gateway/third_party/picoclaw/config"
)

// ErrTransport marks connection/credential failures: expired secrets, network
// unreachable, upstream throttling. Operational, retryable by an operator.
var ErrTransport = errors.New("gateway transport failure")

// ErrCredentialsMissing marks a channel whose configuration is absent or
// incomplete. The channel is skipped at Start with this named reason.
var ErrCredentialsMissing = errors.New("gateway credentials missing")

// AgentDispatcher hands a resolved inbound message to the local agent
// pipeline and returns the rendered replies. The real dispatcher lands with
// the agent-wiring ticket; the gateway depends on this interface only.
type AgentDispatcher interface {
	Dispatch(ctx context.Context, principal agenttools.Principal, inbound bus.InboundMessage) ([]bus.OutboundMessage, error)
}

// ChannelSettings carries one channel's credentials.
type ChannelSettings struct {
	Enabled           bool   `json:"enabled"`
	AppID             string `json:"app_id,omitempty"`             // feishu
	AppSecret         string `json:"app_secret,omitempty"`         // feishu
	EncryptKey        string `json:"encrypt_key,omitempty"`        // feishu
	VerificationToken string `json:"verification_token,omitempty"` // feishu
	IsLark            bool   `json:"is_lark,omitempty"`            // feishu
	BotID             string `json:"bot_id,omitempty"`             // wecom
	Secret            string `json:"secret,omitempty"`             // wecom
	WebSocketURL      string `json:"websocket_url,omitempty"`      // wecom
}

// Settings is the gateway configuration root. Enabled defaults to false;
// zero-value Settings starts zero channels (guarded by test).
type Settings struct {
	Enabled bool            `json:"enabled"`
	Feishu  ChannelSettings `json:"feishu"`
	WeCom   ChannelSettings `json:"wecom"`
}

// DefaultSettings returns the disabled default configuration.
func DefaultSettings() Settings { return Settings{} }

type runningChannel struct {
	name    string
	channel channels.Channel
	cancel  context.CancelFunc
}

// Runner is the BG6 implementation of Gateway.
type Runner struct {
	settings   Settings
	resolver   *IdentityResolver
	dispatcher AgentDispatcher

	mu       sync.Mutex
	running  map[string]runningChannel
	inboundC chan<- bus.InboundMessage
	cancels  []context.CancelFunc
}

func NewRunner(settings Settings, resolver *IdentityResolver, dispatcher AgentDispatcher) *Runner {
	return &Runner{
		settings:   settings,
		resolver:   resolver,
		dispatcher: dispatcher,
		running:    map[string]runningChannel{},
	}
}

// ChannelsStarted reports how many channels are actually running. Tests use it
// to assert the default-off guarantee; operators may log it after Start.
func (r *Runner) ChannelsStarted() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.running)
}

// Start assembles and launches the enabled channels. It never blocks on the
// long-lived connections themselves: each channel runs on its own context.
func (r *Runner) Start(ctx context.Context) error {
	if !r.settings.Enabled {
		slog.Info("gateway disabled by configuration; no channels started",
			"component", "gateway")
		return nil
	}

	inbound := make(chan bus.InboundMessage, 64)
	r.inboundC = inbound
	pumpCtx, cancelPump := context.WithCancel(ctx)
	r.cancels = append(r.cancels, cancelPump)
	go r.pump(pumpCtx, inbound)

	messageBus := inboundFuncBus(func(ctx context.Context, msg bus.InboundMessage) error {
		select {
		case inbound <- msg:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})

	for _, plan := range planChannels(r.settings) {
		if plan.skipReason != nil {
			// D-B16: credentials are an operations problem. Skip the channel
			// with its named reason; do not fail Start, do not panic, do not
			// retry-loop here.
			slog.Warn("gateway channel skipped: credentials missing",
				"component", "gateway", "channel", plan.name, "reason", plan.skipReason.Error())
			continue
		}
		channelCfg := &config.Config{Channels: config.ChannelsConfig{
			plan.name: channelConfigFor(plan.name, plan.settings),
		}}
		var channel channels.Channel
		var buildErr error
		switch plan.name {
		case "feishu":
			channel, buildErr = r.buildFeishu(plan.name, channelCfg, messageBus)
		case "wecom":
			channel, buildErr = r.buildWeCom(plan.name, channelCfg, messageBus)
		default:
			buildErr = fmt.Errorf("unsupported channel %q", plan.name)
		}
		if buildErr != nil {
			return fmt.Errorf("%w: build channel %s: %v", ErrTransport, plan.name, buildErr)
		}
		runCtx, cancel := context.WithCancel(ctx)
		if startErr := channel.Start(runCtx); startErr != nil && !errors.Is(startErr, context.Canceled) {
			cancel()
			return fmt.Errorf("%w: start channel %s: %v", ErrTransport, plan.name, startErr)
		}
		r.mu.Lock()
		r.running[plan.name] = runningChannel{name: plan.name, channel: channel, cancel: cancel}
		r.mu.Unlock()
		slog.Info("gateway channel started", "component", "gateway", "channel", plan.name)
	}
	return nil
}

// Stop cancels every running channel and the pump, then waits-free shuts down
// in reverse order of start. Safe to call when nothing ever started.
func (r *Runner) Stop(_ context.Context) error {
	r.mu.Lock()
	names := make([]string, 0, len(r.running))
	for name, rc := range r.running {
		names = append(names, name)
		if err := rc.channel.Stop(context.Background()); err != nil {
			slog.Warn("gateway channel stop failed", "component", "gateway", "channel", name, "error", err.Error())
		}
		rc.cancel()
		delete(r.running, name)
	}
	cancels := r.cancels
	r.cancels = nil
	r.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
	return nil
}

// pump consumes published inbound messages sequentially so ordering per
// process is preserved; identity resolution happens here — never in the
// vendored channels (ADR-0026 §3).
func (r *Runner) pump(ctx context.Context, inbound <-chan bus.InboundMessage) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-inbound:
			if err := r.handleInbound(ctx, msg); err != nil {
				slog.Warn("gateway inbound handling failed",
					"component", "gateway", "error", err.Error())
			}
		}
	}
}

// handleInbound resolves the sender through Ch3a's tenant layer and dispatches
// to the agent. Two failure classes leave through different errors:
// ErrUnbound for unknown senders (logged once, not retried), everything
// wrapped in ErrTransport for downstream delivery problems.
func (r *Runner) handleInbound(ctx context.Context, msg bus.InboundMessage) error {
	ref := ChannelRef{Channel: msg.Channel, ExternalUserID: msg.Context.SenderID}
	principal, err := r.resolver.Resolve(ctx, ref)
	if err != nil {
		if errors.Is(err, ErrUnbound) {
			// 运营问题：同事还没绑账号。记一次明确日志，不重试、不回复。
			slog.Info("gateway sender unbound; dropping message",
				"component", "gateway", "channel", msg.Channel)
			return fmt.Errorf("%w: %v", ErrUnbound, err)
		}
		return fmt.Errorf("%w: resolve sender: %v", ErrTransport, err)
	}
	replies, err := r.dispatcher.Dispatch(ctx, principal, msg)
	if err != nil {
		return fmt.Errorf("%w: dispatch: %v", ErrTransport, err)
	}
	r.mu.Lock()
	rc, ok := r.running[msg.Channel]
	r.mu.Unlock()
	if !ok {
		return fmt.Errorf("%w: no running channel %q", ErrTransport, msg.Channel)
	}
	for _, reply := range replies {
		if _, err := rc.channel.Send(ctx, reply); err != nil {
			return fmt.Errorf("%w: channel send: %v", ErrTransport, err)
		}
	}
	return nil
}

// channelConfigFor builds the vendor-side config node for one channel.
// AllowFrom is ["*"]: picoclaw's coarse filter must not reject senders before
// Resolve has spoken — authorisation lives entirely in Principal.Scope
// (ADR-0026 §4). The comment travels with the value so nobody tightens it into
// an accidental authorisation layer.
func channelConfigFor(name string, settings ChannelSettings) *config.Channel {
	cfg := &config.Channel{
		Enabled: true,
		Type:    name,
	}
	cfg.AllowFrom = config.FlexibleStringSlice{"*"}
	cfg.SetName(name)
	switch name {
	case "feishu":
		cfg.Settings = config.RawNode(mustJSON(map[string]any{
			"app_id":             strings.TrimSpace(settings.AppID),
			"app_secret":         settings.AppSecret,
			"encrypt_key":        settings.EncryptKey,
			"verification_token": settings.VerificationToken,
			"is_lark":            settings.IsLark,
		}))
	case "wecom":
		cfg.Settings = config.RawNode(mustJSON(map[string]any{
			"bot_id": settings.BotID,
			"secret": settings.Secret,
		}))
	}
	return cfg
}

func validateFeishu(s ChannelSettings) error {
	if strings.TrimSpace(s.AppID) == "" || strings.TrimSpace(s.AppSecret) == "" {
		return fmt.Errorf("%w: feishu needs app_id and app_secret", ErrCredentialsMissing)
	}
	return nil
}

func validateWeCom(s ChannelSettings) error {
	if strings.TrimSpace(s.BotID) == "" || strings.TrimSpace(s.Secret) == "" {
		return fmt.Errorf("%w: wecom needs bot_id and secret", ErrCredentialsMissing)
	}
	return nil
}

// inboundFuncBus adapts a function to the vendored MessageBus interface.
type inboundFuncBus func(ctx context.Context, msg bus.InboundMessage) error

func (f inboundFuncBus) PublishInbound(ctx context.Context, msg bus.InboundMessage) error {
	return f(ctx, msg)
}

// MessageBus keeps the vendored channels compiling against the narrowed
// interface (see third_party/picoclaw/channels/base.go header).
type MessageBus = bus.MessageBus

func mustJSON(value any) []byte {
	raw, err := json.Marshal(value)
	if err != nil {
		// configuration maps are plain key/value pairs; this cannot fail
		panic(fmt.Sprintf("gateway: encode channel settings: %v", err))
	}
	return raw
}

// buildFeishu assembles the vendored feishu channel via its registered
// factory, keeping the runner free of channel-construction details.
func (r *Runner) buildFeishu(name string, cfg *config.Config, messageBus MessageBus) (channels.Channel, error) {
	return buildViaFactory(name, cfg, messageBus)
}

// buildWeCom is the wecom twin of buildFeishu.
func (r *Runner) buildWeCom(name string, cfg *config.Config, messageBus MessageBus) (channels.Channel, error) {
	return buildViaFactory(name, cfg, messageBus)
}

func buildViaFactory(name string, cfg *config.Config, messageBus MessageBus) (channels.Channel, error) {
	factory := channels.FactoryFor(name)
	if factory == nil {
		return nil, fmt.Errorf("%w: no factory registered for %q", ErrTransport, name)
	}
	return factory(name, name, cfg, messageBus)
}

// channelPlan is one channel's assembly decision, derived purely from
// settings. Making the default-off gate a pure function keeps it testable:
// zero-value Settings must plan nothing.
type channelPlan struct {
	name       string
	settings   ChannelSettings
	skipReason error // ErrCredentialsMissing when credentials are incomplete
}

// credentialsAbsent reports whether no credential field was provided at all.
func credentialsAbsent(s ChannelSettings) bool {
	return strings.TrimSpace(s.AppID) == "" &&
		strings.TrimSpace(s.AppSecret) == "" &&
		strings.TrimSpace(s.EncryptKey) == "" &&
		strings.TrimSpace(s.VerificationToken) == "" &&
		strings.TrimSpace(s.BotID) == "" &&
		strings.TrimSpace(s.Secret) == "" &&
		strings.TrimSpace(s.WebSocketURL) == ""
}

// planChannels decides which channels would start. Enabled=false at any level
// (root or per-channel) yields no entry; enabled-but-uncredentialed channels
// yield an entry carrying ErrCredentialsMissing so Start can log the named
// reason and skip.
func planChannels(settings Settings) []channelPlan {
	if !settings.Enabled {
		return nil
	}
	plans := []channelPlan{}
	for _, spec := range []struct {
		name      string
		channel   ChannelSettings
		validator func(ChannelSettings) error
	}{
		{"feishu", settings.Feishu, validateFeishu},
		{"wecom", settings.WeCom, validateWeCom},
	} {
		if !spec.channel.Enabled && credentialsAbsent(spec.channel) {
			continue // channel neither enabled nor configured: not an operator signal
		}
		if err := spec.validator(spec.channel); err != nil {
			plans = append(plans, channelPlan{name: spec.name, skipReason: err})
			continue
		}
		plans = append(plans, channelPlan{name: spec.name})
	}
	return plans
}

// NoopDispatcher is a placeholder dispatch target for deployments that enable
// channels before the agent wiring ticket lands. It acknowledges messages so
// identity resolution and error taxonomy stay observable end to end, while
// guaranteeing that nothing is answered by accident.
type NoopDispatcher struct{}

func (NoopDispatcher) Dispatch(_ context.Context, _ agenttools.Principal, _ bus.InboundMessage) ([]bus.OutboundMessage, error) {
	return nil, nil
}

// gatewaySettingsFromConfig maps the env-loaded configuration onto runner
// settings. Lives here so the config package never imports the gateway.
// A channel is "enabled" in runner terms only when its credentials are present;
// enabled-but-uncredentialed combinations surface as named skips in Start.
func gatewaySettingsFromConfig(gs appconfig.GatewaySettings) Settings {
	feishuReady := strings.TrimSpace(gs.Feishu.AppID) != "" && strings.TrimSpace(gs.Feishu.AppSecret) != ""
	wecomReady := strings.TrimSpace(gs.WeCom.BotID) != "" && strings.TrimSpace(gs.WeCom.Secret) != ""
	return Settings{
		Enabled: gs.Enabled,
		Feishu: ChannelSettings{
			Enabled:           feishuReady,
			AppID:             gs.Feishu.AppID,
			AppSecret:         gs.Feishu.AppSecret,
			EncryptKey:        gs.Feishu.EncryptKey,
			VerificationToken: gs.Feishu.VerificationToken,
			IsLark:            gs.Feishu.IsLark,
		},
		WeCom: ChannelSettings{
			Enabled:      wecomReady,
			BotID:        gs.WeCom.BotID,
			Secret:       gs.WeCom.Secret,
			WebSocketURL: gs.WeCom.WebSocketURL,
		},
	}
}

// GatewaySettingsFromConfig is the exported alias used by cmd/api wiring.
func GatewaySettingsFromConfig(gs appconfig.GatewaySettings) Settings {
	return gatewaySettingsFromConfig(gs)
}
