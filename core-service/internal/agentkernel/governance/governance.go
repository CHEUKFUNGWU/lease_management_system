// Package governance ports the W2 governance middleware chain (ACORE-2) onto
// picoclaw's hook mount points (ADR-0028 §2).
//
// Transport rules learned the hard way:
//
//   - picoclaw's dispatcher treats a hook ERROR as "skip this hook"
//     (`!ok → continue`, hooks.go runInterceptorHook). Every rejection
//     therefore travels as HookDecision{DenyTool} — never as an error.
//   - short-circuits travel as HookDecision{Respond}. Both sites bypass
//     ApproveTool by upstream design; written verdicts live at each site.
//   - audit failure follows adjudication (b): record + continue here, fail
//     the run at turn end via DrainAuditFailures.
//
// Each control is its OWN ToolInterceptor implementation. That is what makes
// ACORE-2's removal mutations meaningful: unmounting one control removes
// exactly that control's logic.
package governance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	picoclawagent "github.com/lease-management-system/core-service/internal/agentkernel/third_party/picoclaw/agent"
	"github.com/lease-management-system/core-service/internal/agentkernel/third_party/picoclaw/tools"
	"github.com/lease-management-system/core-service/internal/agenttools"
)

// Classification vocabulary (底线 2), mirrored from store-day envelopes.
const (
	ClassificationProduction = "production"
	ClassificationSimulated  = "simulated"
	ClassificationMixed      = "mixed"
)

// CallFacts carries what the vendored hook request lacks: the resolved
// principal and the tool descriptor. The wrapper injects them per call —
// upstream structures are NOT extended in place (vendor rule).
type CallFacts struct {
	Descriptor agenttools.ToolDescriptor
	Call       agenttools.ToolCall
	Principal  agenttools.Principal
	Result     *agenttools.ToolResult // after-phase only
}

// FactsResolver produces CallFacts for one hook request.
type FactsResolver interface {
	FactsFor(ctx context.Context, req *picoclawagent.ToolCallHookRequest) (CallFacts, error)
}

// MeasureResolver mirrors agentcore hooks.
type MeasureResolver interface {
	MeasuresFor(toolName string) []string
	IsCertified(toolName string) bool
}

// ReplayStore mirrors agentcore hooks.
type ReplayStore interface {
	Lookup(ctx context.Context, key string) (*agenttools.ToolResult, bool)
}

// ArtifactSink mirrors agentcore hooks.
type ArtifactSink interface {
	RecordCertified(ctx context.Context, call CertifiedCall) error
}

// CertifiedCall is one completed certified invocation (working-paper source).
type CertifiedCall struct {
	CallID        string
	ToolName      string
	ToolVersion   string
	ArgumentsHash string
}

// Deps assembles the chain. Everything injected; no DB, no network.
type Deps struct {
	Policy             agenttools.Policy
	Facts              FactsResolver
	MeasureResolver    MeasureResolver
	Budget             *Budget
	Replay             ReplayStore
	RequireDraftReview bool
	Audit              agenttools.AuditRecorder
	Sink               ArtifactSink
	Metrics            *agenttools.RuntimeMetrics
}

// Budget counts tool calls; nil disables the guard.
type Budget struct {
	MaxToolCalls int
	count        int
}

func (b *Budget) exhausted() bool { return b != nil && b.MaxToolCalls > 0 && b.count >= b.MaxToolCalls }
func (b *Budget) take() {
	if b != nil && b.MaxToolCalls > 0 && b.count < b.MaxToolCalls {
		b.count++
	}
}

func deny(reason string) picoclawagent.HookDecision {
	return picoclawagent.HookDecision{Action: picoclawagent.HookActionDenyTool, Reason: reason}
}

// resolveFacts is the wrapper's infrastructure step: it only reports whether
// the FactsResolver itself failed. Identity completeness is NOT judged here —
// that is TenantScope's exclusive check (ACORE-2 mutation #1 requires exactly
// one owner).
func resolveFacts(ctx context.Context, resolver FactsResolver, req *picoclawagent.ToolCallHookRequest) (CallFacts, bool) {
	facts, err := resolver.FactsFor(ctx, req)
	if err != nil {
		return CallFacts{}, false
	}
	return facts, true
}

// identityComplete is the TenantScope-owned judgement.
func identityComplete(facts CallFacts) bool {
	return strings.TrimSpace(facts.Principal.UserID) != "" &&
		strings.TrimSpace(facts.Principal.Scope.LegalEntityID) != ""
}

// ── before controls ────────────────────────────────────────────────────────

// TenantScope is the first gate: the wrapper must have resolved a usable
// identity into facts. Removing it lets unauthenticated calls reach the chain.
type TenantScope struct{ Facts FactsResolver }

func (TenantScope) Name() string { return "TenantScope" }

func (g TenantScope) BeforeTool(
	ctx context.Context,
	request *picoclawagent.ToolCallHookRequest,
) (*picoclawagent.ToolCallHookRequest, picoclawagent.HookDecision, error) {
	facts, ok := resolveFacts(ctx, g.Facts, request)
	if !ok {
		// wrapper infrastructure failure — fail closed
		return request, deny("execution context could not be resolved"), nil
	}
	// THE check this hook exclusively owns (ACORE-2 mutation #1): identity
	// completeness. No other control re-judges it; removing this hook must
	// let an incomplete-identity call reach the executor.
	if !identityComplete(facts) {
		return request, deny("missing execution context"), nil
	}
	return request, picoclawagent.HookDecision{Action: picoclawagent.HookActionContinue}, nil
}

func (TenantScope) AfterTool(
	ctx context.Context,
	result *picoclawagent.ToolResultHookResponse,
) (*picoclawagent.ToolResultHookResponse, picoclawagent.HookDecision, error) {
	return result, picoclawagent.HookDecision{Action: picoclawagent.HookActionContinue}, nil
}

// CapabilityCheck replicates policy.Evaluate's level/capability/permission/
// dry-run decisions with the same sentinel reasons as agentcore hooks.
type CapabilityCheck struct {
	Policy          agenttools.Policy
	Facts           FactsResolver
	DescriptorFor   func(toolName string) (agenttools.ToolDescriptor, bool)
}

func (c CapabilityCheck) Name() string { return "CapabilityCheck" }

func (c CapabilityCheck) BeforeTool(
	ctx context.Context,
	request *picoclawagent.ToolCallHookRequest,
) (*picoclawagent.ToolCallHookRequest, picoclawagent.HookDecision, error) {
	facts, ok := resolveFacts(ctx, c.Facts, request)
	if !ok {
		// wrapper infrastructure failure — fail closed, but this is not the
		// TenantScope check (that one owns identity completeness)
		return request, deny("call facts could not be resolved"), nil
	}
	desc := facts.Descriptor
	if desc.Name == "" {
		if loaded, found := c.DescriptorFor(request.Tool); found {
			desc = loaded
		}
	}
	call := facts.Call
	if call.ToolName == "" {
		call.ToolName = request.Tool
	}
	policy := c.Policy
	if policy.AllowedLevels == nil {
		policy = agenttools.DefaultPolicy()
	}
	if desc.Name != "" && desc.Name != call.ToolName {
		return request, deny(fmt.Sprintf("%s: tool descriptor version mismatch", agenttools.ErrInvalidToolCall)), nil
	}
	if !policy.AllowedLevels[desc.Level] {
		return request, deny(fmt.Sprintf("%s: level %s is disabled", agenttools.ErrToolCapabilityRequired, desc.Level)), nil
	}
	if facts.Principal.CapabilityActive && !facts.Principal.HasCapability(desc.Name) {
		return request, deny(fmt.Sprintf("%s: %s", agenttools.ErrToolCapabilityRequired, desc.Name)), nil
	}
	if desc.Level == agenttools.LevelCommand && (!policy.AllowCommand || !facts.Principal.HasCapability(desc.Name)) {
		return request, deny(fmt.Sprintf("%s: %s", agenttools.ErrToolCapabilityRequired, desc.Name)), nil
	}
	for _, permission := range desc.Permissions {
		if !facts.Principal.HasPermission(permission) {
			return request, deny(fmt.Sprintf("%s: %s:%s",
				agenttools.ErrToolNotPermitted, permission.Resource, permission.Action)), nil
		}
	}
	if call.DryRun && !desc.SupportsDryRun {
		return request, deny(fmt.Sprintf("%s: tool does not support dry_run", agenttools.ErrInvalidToolCall)), nil
	}
	return request, picoclawagent.HookDecision{Action: picoclawagent.HookActionContinue}, nil
}

// ProtectedMeasure blocks protected measures on non-certified tools.
type ProtectedMeasure struct {
	Resolver MeasureResolver
	Facts    FactsResolver
}

func (p ProtectedMeasure) Name() string { return "ProtectedMeasure" }

func (p ProtectedMeasure) BeforeTool(
	ctx context.Context,
	request *picoclawagent.ToolCallHookRequest,
) (*picoclawagent.ToolCallHookRequest, picoclawagent.HookDecision, error) {
	if p.Resolver == nil {
		return request, picoclawagent.HookDecision{Action: picoclawagent.HookActionContinue}, nil
	}
	measures := p.Resolver.MeasuresFor(request.Tool)
	if len(measures) == 0 {
		return request, picoclawagent.HookDecision{Action: picoclawagent.HookActionContinue}, nil
	}
	if decision := agenttools.RouteMeasures(measures, p.Resolver.IsCertified(request.Tool)); decision.Tier == "Reject" {
		return request, deny(decision.RejectReason), nil
	}
	return request, picoclawagent.HookDecision{Action: picoclawagent.HookActionContinue}, nil
}

// BudgetGuard blocks once the per-turn budget is spent.
type BudgetGuard struct{ Budget *Budget }

func (b BudgetGuard) Name() string { return "BudgetGuard" }

func (b BudgetGuard) BeforeTool(
	ctx context.Context,
	request *picoclawagent.ToolCallHookRequest,
) (*picoclawagent.ToolCallHookRequest, picoclawagent.HookDecision, error) {
	if b.Budget.exhausted() {
		return request, deny(fmt.Sprintf("tool call budget exhausted (%d)", b.Budget.MaxToolCalls)), nil
	}
	b.Budget.take()
	return request, picoclawagent.HookDecision{Action: picoclawagent.HookActionContinue}, nil
}

// IdempotencyGuard enforces write-key requirement and replays stored results.
type IdempotencyGuard struct {
	Replay ReplayStore
	Facts  FactsResolver
}

func (g IdempotencyGuard) Name() string { return "IdempotencyGuard" }

func (g IdempotencyGuard) BeforeTool(
	ctx context.Context,
	request *picoclawagent.ToolCallHookRequest,
) (*picoclawagent.ToolCallHookRequest, picoclawagent.HookDecision, error) {
	facts, ok := resolveFacts(ctx, g.Facts, request)
	if !ok {
		return request, deny("call facts could not be resolved"), nil
	}
	if facts.Descriptor.Level != agenttools.LevelRead && strings.TrimSpace(facts.Call.IdempotencyKey) == "" {
		return request, deny(fmt.Sprintf("%s: write-capable tool requires idempotency_key", agenttools.ErrInvalidToolCall)), nil
	}
	if g.Replay != nil && strings.TrimSpace(facts.Call.IdempotencyKey) != "" {
		if stored, hit := g.Replay.Lookup(ctx, facts.Call.IdempotencyKey); hit && stored != nil {
			request.HookResult = toVendorResult(stored)
			return request, picoclawagent.HookDecision{
				Action: picoclawagent.HookActionRespond,
				Reason: "idempotency replay of a previously approved result",
			}, nil
		}
	}
	return request, picoclawagent.HookDecision{Action: picoclawagent.HookActionContinue}, nil
}

// ReviewGate short-circuits command-level calls requiring review.
type ReviewGate struct {
	Policy             agenttools.Policy
	RequireDraftReview bool
	Facts              FactsResolver
}

func (g ReviewGate) Name() string { return "ReviewGate" }

func (g ReviewGate) BeforeTool(
	ctx context.Context,
	request *picoclawagent.ToolCallHookRequest,
) (*picoclawagent.ToolCallHookRequest, picoclawagent.HookDecision, error) {
	facts, ok := resolveFacts(ctx, g.Facts, request)
	if !ok {
		return request, deny("call facts could not be resolved"), nil
	}
	desc := facts.Descriptor
	if !agenttools.RequiresReviewDecision(desc, agenttools.Policy{RequireDraftReview: g.RequireDraftReview}) ||
		desc.Level != agenttools.LevelCommand {
		return request, picoclawagent.HookDecision{Action: picoclawagent.HookActionContinue}, nil
	}
	reasons := append([]string(nil), desc.Review.Reasons...)
	if len(reasons) == 0 {
		reasons = []string{"tool policy requires human review"}
	}
	short := &agenttools.ToolResult{
		CallID: facts.Call.CallID,
		Status: agenttools.StatusNeedsReview,
		Review: agenttools.ReviewResult{
			Required: true,
			Reasons:  reasons,
			Actions:  append([]string(nil), desc.Review.ConfirmAction),
		},
	}
	request.HookResult = toVendorResult(short)
	return request, picoclawagent.HookDecision{Action: picoclawagent.HookActionRespond}, nil
}

// ── after controls ─────────────────────────────────────────────────────────

// auditFailures collects adjudication-(b) markers: audit failures recorded at
// hook time and drained by the runner at turn end to fail the whole run.
type auditFailures struct {
	mu    sync.Mutex
	errs  []error
}

func (a *auditFailures) record(err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.errs = append(a.errs, err)
}

func (a *auditFailures) drain() []error {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := a.errs
	a.errs = nil
	return out
}

// AuditRecorder writes the tool execution audit. Failure lands in the shared
// marker (never silent) but does not abort the turn mid-flight.
type AuditRecorder struct {
	Recorder agenttools.AuditRecorder
	Facts    FactsResolver
	failures *auditFailures
}

func (a *AuditRecorder) Name() string { return "AuditRecorder" }

func (a *AuditRecorder) AfterTool(
	ctx context.Context,
	result *picoclawagent.ToolResultHookResponse,
) (*picoclawagent.ToolResultHookResponse, picoclawagent.HookDecision, error) {
	if a.Recorder == nil {
		return result, picoclawagent.HookDecision{Action: picoclawagent.HookActionContinue}, nil
	}
	facts, err := a.Facts.FactsFor(ctx, &picoclawagent.ToolCallHookRequest{Meta: result.Meta, Tool: result.Tool})
	var audit agenttools.ToolExecutionAudit
	if err != nil {
		audit = agenttools.ToolExecutionAudit{Status: agenttools.StatusFailed, ErrorCode: "facts_unavailable"}
	} else {
		audit = buildAudit(facts.Call, facts.Descriptor, facts.Principal, facts.Result)
	}
	if recordErr := a.Recorder.RecordToolExecution(context.WithoutCancel(ctx), audit); recordErr != nil {
		a.failures.record(recordErr)
	}
	return result, picoclawagent.HookDecision{Action: picoclawagent.HookActionContinue}, nil
}

// DrainAuditFailures returns and clears every recorded audit failure since the
// last drain. Non-empty at turn end means the run must fail.
func (a *AuditRecorder) DrainAuditFailures() []error { return a.failures.drain() }

// ArtifactCollector feeds completed certified calls into the provenance sink.
type ArtifactCollector struct {
	Sink  ArtifactSink
	Facts FactsResolver
}

func (a ArtifactCollector) Name() string { return "ArtifactCollector" }

func (a ArtifactCollector) AfterTool(
	ctx context.Context,
	result *picoclawagent.ToolResultHookResponse,
) (*picoclawagent.ToolResultHookResponse, picoclawagent.HookDecision, error) {
	if a.Sink == nil {
		return result, picoclawagent.HookDecision{Action: picoclawagent.HookActionContinue}, nil
	}
	facts, err := a.Facts.FactsFor(ctx, &picoclawagent.ToolCallHookRequest{Meta: result.Meta, Tool: result.Tool})
	if err != nil {
		return result, picoclawagent.HookDecision{Action: picoclawagent.HookActionContinue}, nil
	}
	if facts.Result != nil && facts.Result.Status == agenttools.StatusCompleted {
		_ = a.Sink.RecordCertified(ctx, CertifiedCall{
			CallID:        facts.Call.CallID,
			ToolName:      facts.Call.ToolName,
			ToolVersion:   facts.Call.ToolVersion,
			ArgumentsHash: argumentsHash(extractArguments(result.Arguments)),
		})
	}
	return result, picoclawagent.HookDecision{Action: picoclawagent.HookActionContinue}, nil
}

// MetricsRecorder observes the process metrics sink.
type MetricsRecorder struct {
	Metrics *agenttools.RuntimeMetrics
	Facts   FactsResolver
}

func (m MetricsRecorder) Name() string { return "MetricsRecorder" }

func (m MetricsRecorder) AfterTool(
	ctx context.Context,
	result *picoclawagent.ToolResultHookResponse,
) (*picoclawagent.ToolResultHookResponse, picoclawagent.HookDecision, error) {
	if m.Metrics == nil {
		return result, picoclawagent.HookDecision{Action: picoclawagent.HookActionContinue}, nil
	}
	facts, err := m.Facts.FactsFor(ctx, &picoclawagent.ToolCallHookRequest{Meta: result.Meta, Tool: result.Tool})
	if err != nil {
		return result, picoclawagent.HookDecision{Action: picoclawagent.HookActionContinue}, nil
	}
	m.Metrics.Observe(buildAudit(facts.Call, facts.Descriptor, facts.Principal, facts.Result))
	return result, picoclawagent.HookDecision{Action: picoclawagent.HookActionContinue}, nil
}

// ── assembly ───────────────────────────────────────────────────────────────

// NamedControl pairs one control with its mount name. The names ARE the
// assembly contract: TestAssemblyMountsAllNineControls pins them.
type NamedControl struct {
	Name string
	Hook picoclawagent.ToolInterceptor
}

// Assembly returns the nine controls in product order:
// TenantScope → CapabilityCheck → ProtectedMeasure → BudgetGuard →
// IdempotencyGuard → ReviewGate, then AuditRecorder → ArtifactCollector →
// MetricsRecorder. A new control has exactly one place to live. The returned
// recorder exposes adjudication-(b) audit failure markers to the runner.
func Assembly(d Deps) ([]NamedControl, *AuditRecorder) {
	if d.Policy.AllowedLevels == nil {
		d.Policy = agenttools.DefaultPolicy()
	}
	failures := &auditFailures{}
	recorder := &AuditRecorder{Recorder: d.Audit, Facts: d.Facts, failures: failures}
	mount := func(name string, hook picoclawagent.ToolInterceptor) NamedControl {
		return NamedControl{Name: name, Hook: hook}
	}
	return []NamedControl{
		mount("TenantScope", TenantScope{Facts: d.Facts}),
		mount("CapabilityCheck", CapabilityCheck{Policy: d.Policy, Facts: d.Facts}),
		mount("ProtectedMeasure", ProtectedMeasure{Resolver: d.MeasureResolver, Facts: d.Facts}),
		mount("BudgetGuard", BudgetGuard{Budget: d.Budget}),
		mount("IdempotencyGuard", IdempotencyGuard{Replay: d.Replay, Facts: d.Facts}),
		mount("ReviewGate", ReviewGate{Policy: d.Policy, RequireDraftReview: d.RequireDraftReview, Facts: d.Facts}),
		mount("AuditRecorder", recorder),
		mount("ArtifactCollector", ArtifactCollector{Sink: d.Sink, Facts: d.Facts}),
		mount("MetricsRecorder", MetricsRecorder{Metrics: d.Metrics, Facts: d.Facts}),
	}, recorder
}


// ── inert counterparts ─────────────────────────────────────────────────────
// picoclaw's ToolInterceptor requires both phases; controls that own only one
// phase carry an inert method for the other. Inert = continue, no logic.

func (c CapabilityCheck) AfterTool(
	ctx context.Context,
	result *picoclawagent.ToolResultHookResponse,
) (*picoclawagent.ToolResultHookResponse, picoclawagent.HookDecision, error) {
	return result, picoclawagent.HookDecision{Action: picoclawagent.HookActionContinue}, nil
}

func (p ProtectedMeasure) AfterTool(
	ctx context.Context,
	result *picoclawagent.ToolResultHookResponse,
) (*picoclawagent.ToolResultHookResponse, picoclawagent.HookDecision, error) {
	return result, picoclawagent.HookDecision{Action: picoclawagent.HookActionContinue}, nil
}

func (b BudgetGuard) AfterTool(
	ctx context.Context,
	result *picoclawagent.ToolResultHookResponse,
) (*picoclawagent.ToolResultHookResponse, picoclawagent.HookDecision, error) {
	return result, picoclawagent.HookDecision{Action: picoclawagent.HookActionContinue}, nil
}

func (g IdempotencyGuard) AfterTool(
	ctx context.Context,
	result *picoclawagent.ToolResultHookResponse,
) (*picoclawagent.ToolResultHookResponse, picoclawagent.HookDecision, error) {
	return result, picoclawagent.HookDecision{Action: picoclawagent.HookActionContinue}, nil
}

func (g ReviewGate) AfterTool(
	ctx context.Context,
	result *picoclawagent.ToolResultHookResponse,
) (*picoclawagent.ToolResultHookResponse, picoclawagent.HookDecision, error) {
	return result, picoclawagent.HookDecision{Action: picoclawagent.HookActionContinue}, nil
}

func (a *AuditRecorder) BeforeTool(
	ctx context.Context,
	request *picoclawagent.ToolCallHookRequest,
) (*picoclawagent.ToolCallHookRequest, picoclawagent.HookDecision, error) {
	return request, picoclawagent.HookDecision{Action: picoclawagent.HookActionContinue}, nil
}

func (a ArtifactCollector) BeforeTool(
	ctx context.Context,
	request *picoclawagent.ToolCallHookRequest,
) (*picoclawagent.ToolCallHookRequest, picoclawagent.HookDecision, error) {
	return request, picoclawagent.HookDecision{Action: picoclawagent.HookActionContinue}, nil
}

func (m MetricsRecorder) BeforeTool(
	ctx context.Context,
	request *picoclawagent.ToolCallHookRequest,
) (*picoclawagent.ToolCallHookRequest, picoclawagent.HookDecision, error) {
	return request, picoclawagent.HookDecision{Action: picoclawagent.HookActionContinue}, nil
}

// ── helpers ────────────────────────────────────────────────────────────────

func argumentsHash(args []byte) string {
	sum := sha256.Sum256(args)
	return hex.EncodeToString(sum[:])
}

func extractArguments(args map[string]any) []byte {
	raw, err := json.Marshal(args)
	if err != nil {
		return []byte("{}")
	}
	return raw
}

func buildAudit(call agenttools.ToolCall, _ agenttools.ToolDescriptor, principal agenttools.Principal, result *agenttools.ToolResult) agenttools.ToolExecutionAudit {
	startedAt := time.Now()
	completedAt := time.Now()
	audit := agenttools.ToolExecutionAudit{
		CallID:        call.CallID,
		RunID:         call.RunID,
		TraceID:       call.TraceID,
		UserID:        principal.UserID,
		SubjectType:   principal.SubjectType,
		LegalEntityID: principal.Scope.LegalEntityID,
		ToolName:      call.ToolName,
		ToolVersion:   call.ToolVersion,
		StartedAt:     startedAt,
		CompletedAt:   completedAt,
	}
	if result != nil {
		audit.Status = result.Status
		audit.ReviewRequired = result.Review.Required
		if result.Error != nil {
			audit.ErrorCode = result.Error.Code
		}
	} else {
		audit.Status = agenttools.StatusFailed
	}
	return audit
}

// toVendorResult projects the first-party result into the vendored shape.
// Status/Review ride in ForLLM framing because the vendored ToolResult has no
// such fields; the runtime reads its own ToolResult for canonical status.
func toVendorResult(r *agenttools.ToolResult) *tools.ToolResult {
	if r == nil {
		return nil
	}
	out := &tools.ToolResult{
		Silent:          r.Terminate,
		ResponseHandled: r.Terminate,
	}
	if r.Error != nil {
		out.Err = fmt.Errorf("%s", r.Error.Code)
		out.IsError = true
	}
	if r.Review.Required {
		text := fmt.Sprintf("needs_review: %s", strings.Join(r.Review.Reasons, "; "))
		out.ForLLM = text
		out.ForUser = text
	}
	return out
}
