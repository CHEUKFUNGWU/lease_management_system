// First-party replacement for picoclaw pkg/tools: only the symbols the
// vendored agent kernel slice compiles against (ToolResult shape and context
// helpers mirror upstream tools/shared/result.go and base.go, commit bbf6893ca7afad27f1d00a0f5a45982a549c6ed6).
// The tool registry itself is first-party in this repo (agenttools.ToolRuntime).
package tools

import (
	"context"
	"strings"

	"github.com/lease-management-system/core-service/internal/agentkernel/third_party/picoclaw/providers"
)

// ToolResult mirrors upstream tools/shared/result.go.
type ToolResult struct {
	ForLLM          string   `json:"for_llm"`
	ForUser         string   `json:"for_user,omitempty"`
	Silent          bool     `json:"silent"`
	IsError         bool     `json:"is_error"`
	Async           bool     `json:"async"`
	Err             error    `json:"-"`
	Media           []string `json:"media,omitempty"`
	Messages        []providers.Message `json:"-"`
	ArtifactTags    []string `json:"artifact_tags,omitempty"`
	ResponseHandled bool     `json:"response_handled,omitempty"`
}

// ContentForLLM returns the normalized textual content appended to the
// conversation after a tool call. Errors fall back to Err when ForLLM is empty.
func (tr *ToolResult) ContentForLLM() string {
	if tr == nil {
		return ""
	}
	content := tr.ForLLM
	if content == "" && tr.Err != nil {
		content = tr.Err.Error()
	}
	return content
}

// WithError records err on the result and marks it as an error.
func (tr *ToolResult) WithError(err error) *ToolResult {
	if tr == nil {
		tr = &ToolResult{}
	}
	tr.Err = err
	tr.IsError = true
	if content := err.Error(); strings.TrimSpace(tr.ForLLM) == "" {
		tr.ForLLM = content
	}
	return tr
}

// ErrorResult builds an error result carrying message for both LLM and user.
func ErrorResult(message string) *ToolResult {
	return &ToolResult{ForLLM: message, ForUser: message, IsError: true}
}

// UserResult builds a user-only result.
func UserResult(content string) *ToolResult {
	return &ToolResult{ForUser: content}
}

type inboundContextKey struct{}

// WithToolInboundContext returns a child context carrying channel/chat and
// inbound IDs (upstream tools/shared/base.go semantics).
func WithToolInboundContext(
	ctx context.Context,
	channel, chatID, messageID, replyToMessageID string,
) context.Context {
	return context.WithValue(ctx, inboundContextKey{},
		inboundFacts{channel: channel, chatID: chatID, messageID: messageID, replyToMessageID: replyToMessageID})
}

type inboundFacts struct {
	channel, chatID, messageID, replyToMessageID string
}

type sessionContextKey struct{}

// WithToolSessionContext returns a child context carrying turn-scoped session
// metadata. Upstream carries *session.SessionScope; that package is not
// vendored, so the scope rides as an opaque value.
func WithToolSessionContext(
	ctx context.Context,
	agentID, sessionKey string,
	scope any,
) context.Context {
	ctx = context.WithValue(ctx, sessionContextKey{}, sessionFacts{agentID: agentID, sessionKey: sessionKey})
	if scope != nil {
		ctx = context.WithValue(ctx, sessionScopeKey{}, scope)
	}
	return ctx
}

type sessionScopeKey struct{}
type sessionFacts struct {
	agentID, sessionKey string
}
