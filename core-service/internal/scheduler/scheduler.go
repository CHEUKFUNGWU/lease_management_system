// Package scheduler is the single home for in-process scheduled jobs (RT1-L3-C).
//
// Before this package the repository had three same-shaped hand-written ticker
// loops scattered across cmd/api/main.go — the exact "有存储无管理者、散在多处"
// shape AR2's session manager was built to end. New jobs go here; do not write
// another bare goroutine+ticker.
//
// Two job classes exist, and the distinction is a security decision (L3-C
// Principal ruling, 2026-08-26):
//
//   - AuthSystemMaintenance jobs perform queue/infrastructure upkeep. They
//     produce no runs and call no tools, so they never touch the governance
//     chain — there is nothing for it to gate. Their authorization shape is
//     deployment-level machine trust (same trust model as the worker axis,
//     open decision #14). They operate cross-entity BY DEFINITION; see
//     decision D39 in docs/AI_文档索引与现行决策.md §2 for why that is
//     acceptable and where the exemption ends.
//
//   - AuthOnBehalfOfPrincipal jobs produce real runs that execute tools. They
//     MUST carry a registered principal reference resolvable at construction;
//     an unresolvable ref fails construction loudly (startup refuses, it does
//     not silently skip — silent skip and success look identical in logs).
//     v1 ships no such job; the principal registry (table vs config) is ruled
//     together with open decision #14 when the first Type B job lands.
//
// Structural guard: this package imports ONLY the standard library. It cannot
// reach agenttools/agentkernel/aiagent even by accident — enforced by
// importguard_test.go (with a planted-violation reverse fixture, because a
// guard that has never seen red is not a guard).
package scheduler

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

type AuthKind string

const (
	AuthSystemMaintenance   AuthKind = "system_maintenance"
	AuthOnBehalfOfPrincipal AuthKind = "on_behalf_of_principal"
)

// Registration declares one scheduled job. Run receives only a context — the
// signature carries no runtime, registry or tool seam, and the package cannot
// import one (see importguard_test.go). Narrow ports for concrete maintenance
// operations are declared next to their constructors (e.g. LeaseRecoveryQueue).
type Registration struct {
	Name         string
	Interval     time.Duration
	Auth         AuthKind
	PrincipalRef string // required iff Auth == AuthOnBehalfOfPrincipal
	Run          func(ctx context.Context) error
}

type Scheduler struct {
	registrations []Registration
	principalRefs map[string]bool
	wg            sync.WaitGroup
}

// Option customises construction.
type Option func(*Scheduler)

// WithPrincipalRefs seeds the resolvable principal-reference set for
// OnBehalfOfPrincipal jobs. v1 passes none; the first Type B job rules the
// registry source (table vs config) together with open decision #14.
func WithPrincipalRefs(refs []string) Option {
	return func(s *Scheduler) {
		for _, ref := range refs {
			s.principalRefs[ref] = true
		}
	}
}

// New validates every registration and fails loudly on the first problem —
// including an OnBehalfOfPrincipal job whose reference is not registered.
// Startup refusal is the contract: a silently skipped job and a successful one
// are indistinguishable in logs.
func New(registrations []Registration, opts ...Option) (*Scheduler, error) {
	s := &Scheduler{principalRefs: map[string]bool{}}
	for _, opt := range opts {
		opt(s)
	}
	seen := map[string]bool{}
	for _, reg := range registrations {
		if err := s.validate(reg); err != nil {
			return nil, fmt.Errorf("scheduler: job %q refused: %w", reg.Name, err)
		}
		if seen[reg.Name] {
			return nil, fmt.Errorf("scheduler: duplicate job name %q", reg.Name)
		}
		seen[reg.Name] = true
	}
	s.registrations = registrations
	return s, nil
}

func (s *Scheduler) validate(reg Registration) error {
	switch {
	case reg.Name == "":
		return fmt.Errorf("name is required")
	case reg.Interval <= 0:
		return fmt.Errorf("interval must be positive")
	case reg.Run == nil:
		return fmt.Errorf("run is required")
	}
	switch reg.Auth {
	case AuthSystemMaintenance:
		return nil
	case AuthOnBehalfOfPrincipal:
		if reg.PrincipalRef == "" {
			return fmt.Errorf("on_behalf_of_principal requires a registered principal reference")
		}
		if !s.principalRefs[reg.PrincipalRef] {
			return fmt.Errorf("principal reference %q is not registered — register it or use system_maintenance; unregistered delegation is a path around the audit trail", reg.PrincipalRef)
		}
		return nil
	default:
		return fmt.Errorf("unknown auth kind %q", reg.Auth)
	}
}

// Start launches one loop per registration. Each execution runs with panic
// recovery: one panicking job logs and keeps its slot instead of killing the
// unrelated maintenance work sharing the process. Blocks until ctx is done.
func (s *Scheduler) Start(ctx context.Context) {
	for _, reg := range s.registrations {
		reg := reg
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			ticker := time.NewTicker(reg.Interval)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					s.execute(ctx, reg)
				case <-ctx.Done():
					return
				}
			}
		}()
	}
}

// Wait blocks until all job loops have returned (after their ctx was cancelled).
func (s *Scheduler) Wait() { s.wg.Wait() }

func (s *Scheduler) execute(ctx context.Context, reg Registration) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("scheduled job %q panicked: %v", reg.Name, r)
		}
	}()
	if err := reg.Run(ctx); err != nil {
		log.Printf("scheduled job %q failed: %v", reg.Name, err)
	}
}
