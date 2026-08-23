// First-party replacement for picoclaw pkg/config. NOT upstream code.
//
// Only the shapes the vendored channels touch survive: the two channel
// settings structs keep upstream field names and json tags (the protocol code
// reads them), everything else — yaml loading, env resolution, credential
// schemes, 21-channel registries — is dropped. SecureString collapses to a
// plain string holder; secret management is an operations concern per
// ADR-0026 Non-goals.
package config

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ChannelFeishu and ChannelWeCom are the factory names the vendored init()
// functions register under.
const (
	ChannelFeishu = "feishu"
	ChannelWeCom  = "wecom"
)

// Config is the trimmed root config. The factory signature in
// channels/registry.go requires it; only Channels survives here.
type Config struct {
	Channels ChannelsConfig `json:"channel_list"`
}

// ChannelsConfig indexes channel instances by their config map key.
type ChannelsConfig map[string]*Channel

// GroupTriggerConfig controls when group messages trigger the bot.
type GroupTriggerConfig struct {
	MentionOnly bool     `json:"mention_only,omitempty"`
	Prefixes    []string `json:"prefixes,omitempty"`
}

// StreamingConfig controls streamed replies (WeCom).
type StreamingConfig struct {
	Enabled         bool `json:"enabled,omitempty"`
	ThrottleSeconds int  `json:"throttle_seconds,omitempty"`
	MinGrowthChars  int  `json:"min_growth_chars,omitempty"`
}

// TypingConfig controls typing indicator behavior.
type TypingConfig struct {
	Enabled bool `json:"enabled,omitempty"`
}

// PlaceholderConfig controls placeholder message behavior.
type PlaceholderConfig struct {
	Enabled bool                `json:"enabled"`
	Text    FlexibleStringSlice `json:"text,omitempty"`
}

// GetRandomText returns a random placeholder text, or the default.
func (p *PlaceholderConfig) GetRandomText() string {
	if len(p.Text) == 0 {
		return "Thinking..."
	}
	return p.Text[0]
}

// RawNode holds raw channel-specific settings JSON until a factory decodes it.
type RawNode json.RawMessage

// Decode unmarshals the raw settings into a target struct.
func (r *RawNode) Decode(target any) error {
	if len(*r) == 0 {
		return nil
	}
	return json.Unmarshal(*r, target)
}

// IsEmpty reports whether no settings were provided.
func (r *RawNode) IsEmpty() bool {
	return len(*r) == 0
}

// Channel is one configured channel instance.
type Channel struct {
	name               string
	Enabled            bool                `json:"enabled"`
	Type               string              `json:"type"`
	AllowFrom          FlexibleStringSlice `json:"allow_from,omitempty"`
	ReasoningChannelID string              `json:"reasoning_channel_id"`
	GroupTrigger       GroupTriggerConfig  `json:"group_trigger,omitempty"`
	Typing             TypingConfig        `json:"typing,omitempty"`
	Placeholder        PlaceholderConfig   `json:"placeholder,omitempty"`
	Settings           RawNode             `json:"settings,omitempty"`
	extend             any
}

// Name returns the config map key of this channel instance.
func (b *Channel) Name() string { return b.name }

// SetName assigns the config map key; called by the loader, not by channels.
func (b *Channel) SetName(name string) { b.name = name }

// GetDecoded returns the decoded channel-specific settings. The type follows
// b.Type; unknown types yield an error so callers can reject early.
func (b *Channel) GetDecoded() (any, error) {
	if b.extend != nil {
		return b.extend, nil
	}
	var target any
	switch b.Type {
	case ChannelFeishu:
		target = &FeishuSettings{}
	case ChannelWeCom:
		target = &WeComSettings{}
	default:
		return nil, fmt.Errorf("channel %q: unsupported type %q", b.name, b.Type)
	}
	if err := b.Settings.Decode(target); err != nil {
		return nil, fmt.Errorf("channel %q failed to decode settings: %w", b.name, err)
	}
	b.extend = target
	return target, nil
}

// FeishuSettings mirrors upstream field names and tags.
type FeishuSettings struct {
	AppID               string              `json:"app_id"`
	AppSecret           SecureString        `json:"app_secret,omitzero"`
	EncryptKey          SecureString        `json:"encrypt_key,omitzero"`
	VerificationToken   SecureString        `json:"verification_token,omitzero"`
	RandomReactionEmoji FlexibleStringSlice `json:"random_reaction_emoji"`
	IsLark              bool                `json:"is_lark"`
}

// WeComSettings mirrors upstream field names and tags.
type WeComSettings struct {
	BotID               string          `json:"bot_id"`
	Secret              SecureString    `json:"secret,omitzero"`
	WebSocketURL        string          `json:"websocket_url,omitempty"`
	SendThinkingMessage bool            `json:"send_thinking_message"`
	Streaming           StreamingConfig `json:"streaming,omitzero"`
}

// SetSecret assigns the WeCom shared secret programmatically.
func (c *WeComSettings) SetSecret(secret string) { c.Secret = SecureString(secret) }

// FlexibleStringSlice accepts one string or a list, mirroring upstream JSON
// tolerance for single-value configs.
type FlexibleStringSlice []string

// UnmarshalJSON accepts a single JSON string or an array.
func (f *FlexibleStringSlice) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "null" {
		*f = nil
		return nil
	}
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		if single != "" {
			*f = FlexibleStringSlice{single}
		}
		return nil
	}
	var many []string
	if err := json.Unmarshal(data, &many); err == nil {
		*f = many
		return nil
	}
	return fmt.Errorf("flexible string slice: cannot unmarshal %s", trimmed)
}

// SecureString is a plain holder for secret values. Upstream adds encrypted
// reference resolution; that scheme is out of scope here — values arrive as
// plain strings from the runner configuration.
type SecureString string

// String returns the secret value.
func (s SecureString) String() string { return string(s) }

// IsZero reports whether the secret is unset.
func (s SecureString) IsZero() bool { return s == "" }
