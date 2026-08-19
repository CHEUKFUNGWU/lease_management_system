package agentcore

import "testing"

func TestQueueDrainAll(t *testing.T) {
	q := newMessageQueue(QueueAll)
	q.Push(Message{Role: "user", Content: "a"})
	q.Push(Message{Role: "user", Content: "b"})
	got := q.Drain()
	if len(got) != 2 || got[0].Content != "a" || got[1].Content != "b" {
		t.Fatalf("QueueAll must drain everything in order, got %+v", got)
	}
	if q.Len() != 0 {
		t.Fatal("queue must be empty after drain")
	}
	if got := q.Drain(); got != nil {
		t.Fatalf("empty queue must drain nil, got %+v", got)
	}
}

func TestQueueDrainOneAtATime(t *testing.T) {
	q := newMessageQueue(QueueOneAtATime)
	q.Push(Message{Role: "user", Content: "a"})
	q.Push(Message{Role: "user", Content: "b"})
	first := q.Drain()
	if len(first) != 1 || first[0].Content != "a" {
		t.Fatalf("first drain must return one message, got %+v", first)
	}
	if q.Len() != 1 {
		t.Fatalf("one message must remain, got %d", q.Len())
	}
	second := q.Drain()
	if len(second) != 1 || second[0].Content != "b" {
		t.Fatalf("second drain must return the next message, got %+v", second)
	}
}

func TestQueueClear(t *testing.T) {
	q := newMessageQueue(QueueAll)
	q.Push(Message{Role: "user", Content: "a"})
	q.Clear()
	if q.Len() != 0 {
		t.Fatal("queue must be empty after Clear")
	}
}
