package agentcore

import (
	"context"
	"errors"
	"sync"
)

// ErrAgentBusy is returned when Prompt/Continue is called while a run is
// already active.
var ErrAgentBusy = errors.New("agentcore: agent is already running")

// Options assembles an Agent.
type Options struct {
	State        *State
	Deps         Deps
	SteerMode    QueueMode
	FollowUpMode QueueMode
}

// SubscriberFn consumes events. A subscriber that returns an error from a
// critical subscription fails the run (audit is not optional); non-critical
// subscriber errors only poison that subscriber.
type SubscriberFn func(ctx context.Context, ev Event) error

type subscriberEntry struct {
	fn       SubscriberFn
	critical bool
	active   bool
}

// Agent is the stateful wrapper around Loop: it owns the steering and
// follow-up queues, the subscriber set, the abort signal and the settlement
// semantics (a run is not idle until every subscriber has returned).
type Agent struct {
	mu sync.Mutex

	state     *State
	deps      Deps
	steering  *messageQueue
	followUp  *messageQueue
	subs      []*subscriberEntry
	subsMu    sync.Mutex
	cancel    context.CancelFunc
	done      chan struct{}
	runErr    error
	critErr   error
	critErrMu sync.Mutex
	running   bool
}

// New builds an Agent. The state must not be nil; a zero-value Options gets
// sensible defaults (both queues QueueAll).
func New(opts Options) *Agent {
	if opts.State == nil {
		opts.State = NewState()
	}
	a := &Agent{
		state:    opts.State,
		deps:     opts.Deps,
		steering: newMessageQueue(opts.SteerMode),
		followUp: newMessageQueue(opts.FollowUpMode),
		done:     make(chan struct{}),
	}
	return a
}

// Prompt appends the messages to the state and starts a run in the
// background. It fails with ErrAgentBusy if a run is already active.
func (a *Agent) Prompt(ctx context.Context, msg ...Message) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.running {
		return ErrAgentBusy
	}
	if len(msg) > 0 {
		a.state.SetMessages(append(a.state.Messages(), msg...))
	}
	return a.start(ctx)
}

// Continue starts a background run without appending messages.
func (a *Agent) Continue(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.running {
		return ErrAgentBusy
	}
	return a.start(ctx)
}

func (a *Agent) start(ctx context.Context) error {
	runCtx, cancel := context.WithCancel(ctx)
	a.cancel = cancel
	a.done = make(chan struct{})
	a.runErr = nil
	a.critErr = nil
	a.running = true

	emit := func(ev Event) {
		a.dispatch(runCtx, ev)
	}
	deps := a.deps
	deps.Emit = emit

	go func() {
		defer close(a.done)
		err := Loop(runCtx, a.state, deps)
		a.mu.Lock()
		a.runErr = err
		a.running = false
		a.mu.Unlock()
	}()
	return nil
}

// Steer queues a message to inject after the assistant turn ends.
func (a *Agent) Steer(msg Message) { a.steering.Push(msg) }

// FollowUp queues a message to inject only when the agent is about to stop.
func (a *Agent) FollowUp(msg Message) { a.followUp.Push(msg) }

// ClearQueues drops both queues.
func (a *Agent) ClearQueues() {
	a.steering.Clear()
	a.followUp.Clear()
}

// HasQueued reports whether either queue holds a message.
func (a *Agent) HasQueued() bool {
	return a.steering.Len() > 0 || a.followUp.Len() > 0
}

// DrainSteering removes queued steering messages according to SteerMode.
// Callers wire this into Deps.PrepareNext.
func (a *Agent) DrainSteering() []Message { return a.steering.Drain() }

// DrainFollowUp removes queued follow-up messages according to FollowUpMode.
// Callers wire this into Deps.PrepareNext, and should only call it when the
// agent is about to stop.
func (a *Agent) DrainFollowUp() []Message { return a.followUp.Drain() }

// Subscribe registers a non-critical subscriber. Its errors do not fail the
// run. The returned function removes the subscription.
func (a *Agent) Subscribe(fn SubscriberFn) func() {
	return a.subscribe(fn, false)
}

// SubscribeCritical registers a critical subscriber (the audit seam). An error
// from a critical subscriber fails the whole run.
func (a *Agent) SubscribeCritical(fn SubscriberFn) func() {
	return a.subscribe(fn, true)
}

func (a *Agent) subscribe(fn SubscriberFn, critical bool) func() {
	a.subsMu.Lock()
	defer a.subsMu.Unlock()
	entry := &subscriberEntry{fn: fn, critical: critical, active: true}
	a.subs = append(a.subs, entry)
	return func() {
		a.subsMu.Lock()
		defer a.subsMu.Unlock()
		entry.active = false
	}
}

func (a *Agent) dispatch(ctx context.Context, ev Event) {
	a.subsMu.Lock()
	subs := make([]*subscriberEntry, 0, len(a.subs))
	for _, e := range a.subs {
		if e.active {
			subs = append(subs, e)
		}
	}
	a.subsMu.Unlock()

	for _, e := range subs {
		if err := e.fn(ctx, ev); err != nil && e.critical {
			a.critErrMu.Lock()
			if a.critErr == nil {
				a.critErr = err
			}
			a.critErrMu.Unlock()
		}
	}
}

// Abort cancels the running loop. Already-started tool calls receive the
// cancelled context; no new tool calls are initiated after the next check.
func (a *Agent) Abort() {
	a.mu.Lock()
	cancel := a.cancel
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// WaitForIdle blocks until the run settles: the loop finished and every
// subscriber has returned (subscriber dispatch is synchronous inside the
// loop, so loop completion implies subscriber completion). It returns the run
// error, which may be context.Canceled after Abort, a loop/hook error, or a
// critical subscriber error.
func (a *Agent) WaitForIdle(ctx context.Context) error {
	a.mu.Lock()
	done := a.done
	a.mu.Unlock()
	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}
	a.mu.Lock()
	err := a.runErr
	a.mu.Unlock()
	if err != nil {
		return err
	}
	a.critErrMu.Lock()
	crit := a.critErr
	a.critErrMu.Unlock()
	return crit
}

// Reset clears run state: conversation, streaming, pending calls, last error
// and both queues. Tools, system prompt, model and thinking level survive.
func (a *Agent) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ClearQueues()
	a.state.clearRunState()
	a.runErr = nil
	a.critErr = nil
}

// State exposes the underlying state for callers to prepare before a run.
func (a *Agent) State() *State { return a.state }
