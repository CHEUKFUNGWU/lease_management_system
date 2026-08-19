package agentcore

import "sync"

// QueueMode controls how a message queue drains.
type QueueMode int

const (
	// QueueAll drains every queued message in one call.
	QueueAll QueueMode = iota
	// QueueOneAtATime drains exactly one message per call.
	QueueOneAtATime
)

// messageQueue holds injected messages. Steering and follow-up share the same
// shape; the difference is when the caller drains them (steering is drained
// after each assistant turn, follow-up only when the agent is about to stop).
type messageQueue struct {
	mu    sync.Mutex
	mode  QueueMode
	items []Message
}

func newMessageQueue(mode QueueMode) *messageQueue {
	return &messageQueue{mode: mode}
}

func (q *messageQueue) Push(m Message) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = append(q.items, m)
}

// Drain removes and returns messages according to the queue's mode. It never
// returns a nil slice for a non-empty queue.
func (q *messageQueue) Drain() []Message {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		return nil
	}
	if q.mode == QueueOneAtATime {
		m := q.items[0]
		q.items = q.items[1:]
		return []Message{m}
	}
	out := q.items
	q.items = nil
	return out
}

func (q *messageQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

func (q *messageQueue) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = nil
}
