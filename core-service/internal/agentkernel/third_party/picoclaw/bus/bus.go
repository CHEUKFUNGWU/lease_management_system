// First-party replacement for picoclaw pkg/bus (ADR-0026 §1 pattern, reused by
// the agent kernel slice). NOT upstream code.
//
// The vendored pipeline talks to the bus through this narrow interface; the
// runner supplies the implementation that bridges to aichat persistence.
package bus

import "context"

// MessageBus is the message boundary the vendored agent core publishes and
// consumes through. Upstream holds a concrete *bus.MessageBus; here it is an
// interface so the runner can bridge each direction to first-party storage.
type MessageBus interface {
	PublishInbound(ctx context.Context, msg InboundMessage) error
	PublishOutbound(ctx context.Context, msg OutboundMessage) error
	PublishOutboundMedia(ctx context.Context, msg OutboundMediaMessage) error
	GetStreamer(ctx context.Context, channel, chatID, sessionKey string) (Streamer, bool)
}

// Streamer pushes incremental content to a streaming-capable channel.
type Streamer interface {
	Update(ctx context.Context, content string) error
	Finalize(ctx context.Context, content string) error
	Cancel(ctx context.Context)
}

// ContextUsageStreamer can attach final context-window usage metadata when a
// streaming channel's final message replaces the normal outbound response.
type ContextUsageStreamer interface {
	Streamer
	FinalizeWithContext(ctx context.Context, content string, usage *ContextUsage) error
}

// ReasoningStreamer can show incremental model reasoning/thought content
// separately from the final user-visible answer stream.
type ReasoningStreamer interface {
	UpdateReasoning(ctx context.Context, content string) error
	FinalizeReasoning(ctx context.Context, content string) error
}
