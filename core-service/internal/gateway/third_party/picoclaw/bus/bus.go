// First-party replacement for picoclaw pkg/bus (ADR-0026 §1: two structs were
// not worth a module dependency; the same logic extends to the narrow bus
// surface the vendored channels need). NOT upstream code.
//
// MessageBus is an interface here — upstream BaseChannel holds a concrete
// *bus.MessageBus. The gateway runner supplies the one-method implementation
// that hands InboundMessages to the local agent pipeline.
package bus

import "context"

// MessageBus receives inbound messages from channels. The runner-side
// implementation routes them into the agent and publishes replies outbound.
type MessageBus interface {
	PublishInbound(ctx context.Context, msg InboundMessage) error
}

// StreamDelegate returns a Streamer for a channel+chat pair when streaming is
// supported. The gateway runner implements it against the agent's stream
// callbacks.
type StreamDelegate interface {
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
