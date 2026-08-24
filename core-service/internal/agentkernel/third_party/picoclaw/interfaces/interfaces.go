// Vendored from github.com/sipeed/picoclaw at commit bbf6893ca7afad27f1d00a0f5a45982a549c6ed6.
// Upstream path: agent/interfaces/interfaces.go
// SPDX-License-Identifier: MIT — Copyright (c) 2026 PicoClaw contributors.
// See THIRD_PARTY_NOTICES at the repository root. Do not add business
// logic here; adapt upstream behaviour in the wrapping layer instead.

// PicoClaw - Ultra-lightweight personal AI agent

package interfaces

import (
	"context"

	"github.com/lease-management-system/core-service/internal/agentkernel/third_party/picoclaw/bus"
)

// Channel is the minimal channel surface the kernel's outbound path needs;
// mirrors the method set used by AgentLoop send paths.
type Channel interface {
	Name() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Send(ctx context.Context, msg bus.OutboundMessage) ([]string, error)
	IsRunning() bool
}

// MessageBus publishes inbound and outbound messages.
// It is the primary communication channel for the agent loop.
type MessageBus interface {
	// PublishInbound sends an inbound message to be processed.
	PublishInbound(ctx context.Context, msg bus.InboundMessage) error

	// PublishOutbound sends an outbound message to the appropriate channel.
	PublishOutbound(ctx context.Context, msg bus.OutboundMessage) error

	// PublishOutboundMedia sends an outbound media message.
	PublishOutboundMedia(ctx context.Context, msg bus.OutboundMediaMessage) error

	// GetStreamer returns a channel streamer when the active channel supports streaming.
	GetStreamer(ctx context.Context, channel, chatID, sessionKey string) (bus.Streamer, bool)

	// InboundChan returns the channel for receiving inbound messages.
	InboundChan() <-chan bus.InboundMessage
}

// ChannelManager manages channel lifecycle and provides channel access.
type ChannelManager interface {
	// GetChannel returns the channel with the given name.
	// Channel is the minimal shape the kernel needs from a registered
	// channel; the full channels.Channel lives in the gateway tree
	// (ADR-0026 §1). Defined locally to avoid importing that tree.
	GetChannel(name string) (Channel, bool)

	// GetEnabledChannels returns the list of enabled channel names.
	GetEnabledChannels() []string

	// InvokeTypingStop signals that typing has stopped.
	InvokeTypingStop(channel, chatID string)

	// SendMessage sends a text message to the specified channel and chat.
	SendMessage(ctx context.Context, msg bus.OutboundMessage) error

	// SendMedia sends a media message to the specified channel and chat.
	SendMedia(ctx context.Context, msg bus.OutboundMediaMessage) error

	// SendPlaceholder sends a placeholder message (e.g., for audio transcription).
	SendPlaceholder(ctx context.Context, channel, chatID string) bool

	// DismissToolFeedback clears any tracked tool feedback animation for the
	// given channel/chat. Call this when a turn ends without a final response
	// (e.g., ResponseHandled tools) to avoid orphaned animation goroutines.
	// outboundCtx carries topic/thread info needed for channels that use
	// scoped tracker keys (e.g., Telegram forum topics); may be nil.
	DismissToolFeedback(ctx context.Context, channel, chatID string, outboundCtx *bus.InboundContext)
}
