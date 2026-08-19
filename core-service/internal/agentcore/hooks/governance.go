package hooks

import (
	"github.com/lease-management-system/core-service/internal/agentcore"
	"github.com/lease-management-system/core-service/internal/agenttools"
)

// Deps assembles the governance chain. Every dependency is injected; nothing
// here touches a database or network.
type Deps struct {
	Policy             agenttools.Policy
	MeasureResolver    MeasureResolver
	Budget             *Budget
	Replay             ReplayStore
	RequireDraftReview bool
	Audit              agenttools.AuditRecorder
	Sink               ArtifactSink
	Metrics            *agenttools.RuntimeMetrics
}

// Governance returns the ordered before and after chains. The order is the
// product (Agent Core design §6): TenantScope → CapabilityCheck →
// ProtectedMeasure → BudgetGuard → IdempotencyGuard → ReviewGate, then
// AuditRecorder → ArtifactCollector → MetricsRecorder. A new control has
// exactly one place to live.
func Governance(d Deps) (agentcore.BeforeToolCall, agentcore.AfterToolCall) {
	if d.Policy.AllowedLevels == nil {
		d.Policy = agenttools.DefaultPolicy()
	}
	before := agentcore.ChainBefore(
		TenantScope(),
		CapabilityCheck(d.Policy),
		ProtectedMeasure(d.MeasureResolver),
		BudgetGuard(d.Budget),
		IdempotencyGuard(d.Replay),
		ReviewGate(d.RequireDraftReview),
	)
	after := agentcore.ChainAfter(
		AuditRecorder(d.Audit),
		ArtifactCollector(d.Sink),
		MetricsRecorder(d.Metrics),
	)
	return before, after
}
