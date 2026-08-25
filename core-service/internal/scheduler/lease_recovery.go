package scheduler

// Lease recovery is the canonical Type A (system-maintenance) job: it requeues
// runs whose worker lease expired. It produces no runs and calls no tools —
// the governance chain has nothing to gate here, and inventing a fake ToolCall
// to push it through would itself be a new lying surface (L3-C ruling,
// 2026-08-26). Cross-entity operation is the job's definition: it only resets
// queue/lease columns and never reads tenant business data (decision D39).
//
// The dependency is a narrow port declared HERE, satisfied by the repository
// elsewhere: this package cannot import business code even if it wanted to
// (importguard_test.go), so a job body inside the scheduler is structurally
// incapable of reaching the tool runtime.

import (
	"context"
	"fmt"
	"log"
	"time"
)

// LeaseRecoveryQueue is the narrow port the recovery job needs — nothing more.
type LeaseRecoveryQueue interface {
	RecoverExpiredRunLeases(ctx context.Context) (int, error)
}

// LeaseRecovery returns the registration for the expired-lease recovery job.
//
// The row-state machine in RecoverExpiredRunLeases carries two distinct
// duties, and its three predicates are NOT interchangeable (RT1-L3-C review,
// 2026-08-26):
//
//   - Idempotency: a second execution at the same moment finds nothing to
//     reset. Any ONE of the three predicates surviving keeps this property —
//     they are redundant against each other here.
//   - Correctness: `status = 'running'` alone prevents resurrecting finished
//     runs. A completed run can legitimately carry an expired leased_until
//     (UpdateClaimedRunStatus writes status without clearing the lease;
//     only ReleaseRunLease clears it). No other predicate covers that duty.
//
// Both duties are pinned by lease_recovery_integration_test.go; removing the
// status predicate turns TestLeaseRecoveryJobDoesNotResurrectCompletedRuns
// red.
func LeaseRecovery(queue LeaseRecoveryQueue, interval time.Duration) Registration {
	return Registration{
		Name:     "agent-run-lease-recovery",
		Interval: interval,
		Auth:     AuthSystemMaintenance,
		Run: func(ctx context.Context) error {
			if queue == nil {
				return fmt.Errorf("lease recovery queue is not wired")
			}
			recovered, err := queue.RecoverExpiredRunLeases(ctx)
			if err != nil {
				return err
			}
			if recovered > 0 {
				log.Printf("agent run lease recovery: requeued %d run(s)", recovered)
			}
			return nil
		},
	}
}
