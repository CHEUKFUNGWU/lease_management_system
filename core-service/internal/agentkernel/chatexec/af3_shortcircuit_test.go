package chatexec

import (
	"context"
	"testing"

	"github.com/lease-management-system/core-service/internal/agentkernel/governance"
	"github.com/lease-management-system/core-service/internal/agenttools"
)

// ── AF3-b：deriveShortCircuit 与链上挂载顺序同源 ────────────────────────────
//
// The chain mounts IdempotencyGuard BEFORE ReviewGate. When both predicates
// apply to one call — a stored replay result AND a tool that requires review
// — the derivation must answer what the chain answers: the REPLAY wins.
// The old hand-written order checked review first, so the day a replay store
// got wired, replay hits on review-bearing tools would have silently become
// needs_review instead.

func TestDeriveShortCircuitReplayBeatsReviewWhenBothApply(t *testing.T) {
	stored := &agenttools.ToolResult{Status: agenttools.StatusCompleted, Data: "replayed"}
	w := newMutationWorld(t, func(deps *Deps) {
		deps.Replay = staticLookup{stored: stored}
	})

	commandDescriptor := agenttools.ToolDescriptor{
		Name: "lease.month.close.lock", Version: "v1", Description: "lock", Level: agenttools.LevelCommand,
		Permissions:         []agenttools.Permission{{Resource: "monthly_closing", Action: "lock"}},
		SupportsIdempotency: true,
		Review:              agenttools.ReviewPolicy{Required: true, Reasons: []string{"lock_period_review"}, ConfirmAction: "confirm_lock"},
	}
	call := agenttools.ToolCall{
		CallID: "mc-both", RunID: "run-mut", ToolName: "lease.month.close.lock",
		ToolVersion: "v1", IdempotencyKey: "replayed-key",
	}

	short := w.kernel.deriveShortCircuit(context.Background(), commandDescriptor, call)
	if short == nil {
		t.Fatal("both short-circuits apply; derivation returned nil")
	}
	if short.Status != agenttools.StatusCompleted || short.Data != "replayed" {
		t.Fatalf("short-circuit = %+v; want the replay payload (chain order: IdempotencyGuard before ReviewGate)", short)
	}
}

// Structural mutation check: if ShortCircuitOrder ever flips to match the old
// wrong derivation order, this test goes red. The Assembly guard additionally
// panics at construction when the mount list stops matching the order.
func TestShortCircuitOrderMatchesMountedChain(t *testing.T) {
	w := newMutationWorld(t, func(deps *Deps) {
		deps.Replay = staticLookup{stored: &agenttools.ToolResult{Status: agenttools.StatusCompleted, Data: "x"}}
	})
	var respondMounted []string
	for _, control := range w.kernel.controls {
		for _, sc := range governance.ShortCircuitOrder {
			if control.Name == sc {
				respondMounted = append(respondMounted, sc)
			}
		}
	}
	if len(respondMounted) != len(governance.ShortCircuitOrder) {
		t.Fatalf("mounted Respond-capable controls %v do not cover ShortCircuitOrder %v",
			respondMounted, governance.ShortCircuitOrder)
	}
	for i, name := range governance.ShortCircuitOrder {
		if respondMounted[i] != name {
			t.Fatalf("mount order %v diverges from ShortCircuitOrder %v", respondMounted, governance.ShortCircuitOrder)
		}
	}
}
