// Package agentcore is the pure Agent runtime: state, events, tool execution,
// queues and the two hook points. It deliberately imports neither database/sql
// nor net/http nor any repository or object-store client — all I/O is injected
// through Deps. This purity is a contract, locked by importguard_test.go.
package agentcore

import "sync"

// Message is one conversation message. It is a plain value so callers can
// copy it freely; State keeps its own copies.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// State is the mutable agent state. It is safe for concurrent use: the loop
// goroutine reads and writes it while callers inspect it through the getters.
// Getters return copies of slice fields so external holders can never mutate
// the internal arrays.
type State struct {
	mu sync.RWMutex

	systemPrompt  string
	model         string
	thinkingLevel string

	tools    []Tool
	messages []Message

	streaming    bool
	streamingMsg *Message
	pending      map[string]struct{}
	lastError    error
}

// NewState returns an empty State with no tools and no messages.
func NewState() *State {
	return &State{pending: make(map[string]struct{})}
}

func (s *State) SystemPrompt() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.systemPrompt
}

func (s *State) SetSystemPrompt(v string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.systemPrompt = v
}

func (s *State) Model() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.model
}

func (s *State) SetModel(v string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.model = v
}

func (s *State) ThinkingLevel() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.thinkingLevel
}

func (s *State) SetThinkingLevel(v string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.thinkingLevel = v
}

// Tools returns a copy of the registered tools.
func (s *State) Tools() []Tool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Tool(nil), s.tools...)
}

// SetTools copies the input slice into the state.
func (s *State) SetTools(t []Tool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tools = append([]Tool(nil), t...)
}

// Messages returns a copy of the conversation so far.
func (s *State) Messages() []Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Message(nil), s.messages...)
}

// SetMessages copies the input slice into the state.
func (s *State) SetMessages(m []Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append([]Message(nil), m...)
}

// IsStreaming reports whether an assistant message is currently streaming.
func (s *State) IsStreaming() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.streaming
}

// PendingToolCalls returns the call IDs of tools currently executing.
func (s *State) PendingToolCalls() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, 0, len(s.pending))
	for id := range s.pending {
		out = append(out, id)
	}
	return out
}

// LastError returns the last error recorded by the loop, if any.
func (s *State) LastError() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastError
}

// beginStreaming opens a new assistant message for streaming.
func (s *State) beginStreaming() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.streaming = true
	s.streamingMsg = &Message{Role: "assistant"}
}

// appendDelta accumulates streaming content into the open message.
func (s *State) appendDelta(m *Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.streaming || s.streamingMsg == nil {
		s.streaming = true
		s.streamingMsg = &Message{Role: "assistant"}
	}
	if m != nil {
		s.streamingMsg.Content += m.Content
	}
}

// streamingCopy returns the current streaming message content as a value.
func (s *State) streamingCopy() Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.streamingMsg == nil {
		return Message{}
	}
	return *s.streamingMsg
}

// commitStreaming finalizes the open streaming message into the conversation.
func (s *State) commitStreaming(m *Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m != nil {
		s.messages = append(s.messages, *m)
	}
	s.streaming = false
	s.streamingMsg = nil
}

// toolByName resolves a registered tool by name and version. An empty version
// in the call matches any version.
func (s *State) toolByName(name, version string) (Tool, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, t := range s.tools {
		d := t.Descriptor()
		if d.Name != name {
			continue
		}
		if version == "" || d.Version == version {
			return t, true
		}
	}
	return nil, false
}

func (s *State) markPending(callID string) {
	if callID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pending[callID] = struct{}{}
}

func (s *State) unmarkPending(callID string) {
	if callID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pending, callID)
}

func (s *State) setLastError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastError = err
}

func (s *State) clearRunState() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = nil
	s.streaming = false
	s.streamingMsg = nil
	s.pending = make(map[string]struct{})
	s.lastError = nil
}
