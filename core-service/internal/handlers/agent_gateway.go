package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agentcapability"
	"github.com/lease-management-system/core-service/internal/agentskill"
	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/aichat"
	"github.com/lease-management-system/core-service/internal/middleware"
	auditservice "github.com/lease-management-system/core-service/internal/services/audit"
)

// AgentGatewayHandler is the narrow HTTP adapter for external Agent/CLI
// callers. It has no repository or database dependency; all business access
// remains behind ToolRuntime's policy and scope seam.
type AgentGatewayHandler struct {
	runtime     agenttools.ToolRuntime
	audit       agenttools.AuditRecorder
	capability  *agentcapability.Issuer
	skills      *agentskill.Registry
	sessions    AgentSessionStore
	contracts   AgentContractScopeReader
	runs        AgentRunStore
	checkpoints AgentRunCheckpointStore
	queue       AgentRunQueueStore
	workerRuns  AgentWorkerRunStore
	alerts      AgentRunTerminalAlertStore
	usage       AgentUsageReader
	contextMetrics *agenttools.ContextMetrics
	sessionOwner   aichat.SessionOwner
}

// WithContextMetrics attaches the RT1-A context-budget sink so
// /agent/metrics/prometheus appends its payload to the tool metrics.
func (h *AgentGatewayHandler) WithContextMetrics(m *agenttools.ContextMetrics) *AgentGatewayHandler {
	if h == nil {
		return h
	}
	h.contextMetrics = m
	return h
}

// WithSessionOwner shares the AR2 lifecycle seam with the gateway plane
// (RT1-B): gateway session creation flows through sessionmanager exactly like
// the chat plane, so the runner (whose only session path is this HTTP entry)
// is covered too. Nil keeps the legacy store path.
func (h *AgentGatewayHandler) WithSessionOwner(owner aichat.SessionOwner) *AgentGatewayHandler {
	if h == nil {
		return h
	}
	h.sessionOwner = owner
	return h
}

// SessionOwnerKind reports the concrete wired session owner type. Machine
// assertion seam for RT1-B (AR5-G1 / SI1 SessionOwnerKind pattern): empty
// when legacy; the reverse control in the test proves the discriminator can
// tell wired from unwired.
func (h *AgentGatewayHandler) SessionOwnerKind() string {
	if h == nil || h.sessionOwner == nil {
		return ""
	}
	return fmt.Sprintf("%T", h.sessionOwner)
}

func NewAgentGatewayHandler(runtime agenttools.ToolRuntime, auditRecorders ...agenttools.AuditRecorder) *AgentGatewayHandler {
	var recorder agenttools.AuditRecorder
	if len(auditRecorders) > 0 {
		recorder = auditRecorders[0]
	}
	return &AgentGatewayHandler{runtime: runtime, audit: recorder}
}

func (h *AgentGatewayHandler) WithCapabilityIssuer(issuer *agentcapability.Issuer) *AgentGatewayHandler {
	if h == nil {
		return h
	}
	clone := *h
	clone.capability = issuer
	return &clone
}

func (h *AgentGatewayHandler) WithSkillRegistry(registry *agentskill.Registry) *AgentGatewayHandler {
	if h == nil {
		return h
	}
	clone := *h
	clone.skills = registry
	return &clone
}

func (h *AgentGatewayHandler) WithRunStore(store AgentRunStore) *AgentGatewayHandler {
	if h == nil {
		return h
	}
	clone := *h
	clone.runs = store
	return &clone
}

func (h *AgentGatewayHandler) WithCheckpointStore(store AgentRunCheckpointStore) *AgentGatewayHandler {
	if h == nil {
		return h
	}
	clone := *h
	clone.checkpoints = store
	return &clone
}

func (h *AgentGatewayHandler) WithQueueStore(store AgentRunQueueStore) *AgentGatewayHandler {
	if h == nil {
		return h
	}
	clone := *h
	clone.queue = store
	return &clone
}

func (h *AgentGatewayHandler) WithWorkerRunStore(store AgentWorkerRunStore) *AgentGatewayHandler {
	if h == nil {
		return h
	}
	clone := *h
	clone.workerRuns = store
	return &clone
}

func (h *AgentGatewayHandler) WithTerminalAlertStore(store AgentRunTerminalAlertStore) *AgentGatewayHandler {
	if h == nil {
		return h
	}
	clone := *h
	clone.alerts = store
	return &clone
}

func (h *AgentGatewayHandler) WithUsageStore(store AgentUsageReader) *AgentGatewayHandler {
	if h == nil {
		return h
	}
	clone := *h
	clone.usage = store
	return &clone
}

func (h *AgentGatewayHandler) WithSessionStore(store AgentSessionStore) *AgentGatewayHandler {
	if h == nil {
		return h
	}
	clone := *h
	clone.sessions = store
	return &clone
}

func (h *AgentGatewayHandler) WithContractScopeReader(reader AgentContractScopeReader) *AgentGatewayHandler {
	if h == nil {
		return h
	}
	clone := *h
	clone.contracts = reader
	return &clone
}

// Skills returns server-owned Skill descriptors filtered by authoritative
// roles. Skill discovery is descriptive only; execution still goes through
// Tool Runtime policy and scope checks.
func (h *AgentGatewayHandler) Skills(c *gin.Context) {
	if h == nil || h.skills == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent skill registry unavailable"})
		return
	}
	if _, status, err := h.gatewayContext(c, "skill-discovery", ""); err != nil {
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	roles := []string{}
	if value, ok := c.Get("roles"); ok {
		roles, _ = value.([]string)
	}
	if len(roles) == 0 {
		if value, ok := c.Get("role"); ok {
			if role, ok := value.(string); ok && strings.TrimSpace(role) != "" {
				roles = []string{role}
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"skills": h.skills.List(roles)})
}

// MetricsPrometheus is the scrape-friendly form of Metrics. The same
// permission check is intentionally applied before exposing the payload.
func (h *AgentGatewayHandler) MetricsPrometheus(c *gin.Context) {
	ctx, status, err := h.gatewayContext(c, "agent-metrics-prometheus", "")
	if err != nil {
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	runtime, status, ok := h.metricsRuntime(ctx)
	if !ok {
		message := "agent metrics permission is required"
		if status == http.StatusServiceUnavailable {
			message = "agent runtime metrics unavailable"
		}
		c.JSON(status, gin.H{"error": message})
		return
	}
	payload := runtime.Prometheus(time.Now().UTC())
	if h != nil && h.contextMetrics != nil {
		payload += h.contextMetrics.Prometheus(time.Now().UTC())
	}
	c.Data(http.StatusOK, "text/plain; version=0.0.4; charset=utf-8", []byte(payload))
}

func (h *AgentGatewayHandler) metricsRuntime(ctx context.Context) (*agenttools.RuntimeMetrics, int, bool) {
	execution, ok := agenttools.ExecutionContextFromContext(ctx)
	if !ok || (!middleware.HasPermission(execution.Principal.Permissions, "agent_runtime", "metrics") &&
		!middleware.HasPermission(execution.Principal.Permissions, "audit_logs", "read") &&
		!middleware.HasPermission(execution.Principal.Permissions, "*", "*")) {
		return nil, http.StatusForbidden, false
	}
	runtime, ok := h.runtime.(*agenttools.Runtime)
	if !ok || runtime.Metrics() == nil {
		return nil, http.StatusServiceUnavailable, false
	}
	return runtime.Metrics(), http.StatusOK, true
}

// Describe returns only Tools discoverable by the authenticated principal.
// A caller can request schemas explicitly, but cannot ask the server to
// describe a Tool it is not permitted to execute.
func (h *AgentGatewayHandler) Describe(c *gin.Context) {
	if h == nil || h.runtime == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent tool gateway unavailable"})
		return
	}
	runID := "gateway-describe"
	if requestedRunID := strings.TrimSpace(c.Query("run_id")); requestedRunID != "" {
		runID = requestedRunID
	}
	ctx, status, err := h.gatewayContext(c, runID, "")
	if err != nil {
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	filter, err := describeFilter(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	descriptors, err := h.runtime.Describe(ctx, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to describe agent tools"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"tools": descriptors})
}

type issueCapabilityRequest struct {
	SessionID    string   `json:"session_id,omitempty"`
	RunID        string   `json:"run_id"`
	SkillID      string   `json:"skill_id,omitempty"`
	SkillVersion string   `json:"skill_version,omitempty"`
	AllowedTools []string `json:"allowed_tools"`
	TTLSeconds   int      `json:"ttl_seconds,omitempty"`
}

type revokeCapabilityRequest struct {
	RunID   string `json:"run_id"`
	TokenID string `json:"token_id,omitempty"`
}

// IssueCapability mints a short-lived, run-bound capability from the already
// authenticated normal JWT context. The requested tools are checked against
// the same runtime that executes them, so a caller cannot mint a grant for a
// tool outside its current permissions.
func (h *AgentGatewayHandler) IssueCapability(c *gin.Context) {
	if h == nil || h.runtime == nil || h.capability == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent capability issuer unavailable"})
		return
	}
	var request issueCapabilityRequest
	if err := decodeStrictJSON(c, &request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid capability request"})
		return
	}
	request.RunID = strings.TrimSpace(request.RunID)
	request.SkillID = strings.TrimSpace(request.SkillID)
	request.SkillVersion = strings.TrimSpace(request.SkillVersion)
	if request.RunID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "run_id is required"})
		return
	}
	request.AllowedTools = normalizeNames(request.AllowedTools)
	if len(request.AllowedTools) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "allowed_tools must contain at least one tool"})
		return
	}
	if request.TTLSeconds < 0 || (request.TTLSeconds > 0 && time.Duration(request.TTLSeconds)*time.Second > h.capability.TTL()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ttl_seconds exceeds the configured capability limit"})
		return
	}
	ctx, status, err := gatewayContext(c, request.RunID, "capability-issue")
	if err != nil {
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	descriptors, err := h.runtime.Describe(ctx, agenttools.ToolFilter{Names: request.AllowedTools, SkillID: request.SkillID})
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "requested tools are not available to this principal"})
		return
	}
	described := make(map[string]struct{}, len(descriptors))
	for _, descriptor := range descriptors {
		described[strings.ToLower(descriptor.Name)] = struct{}{}
	}
	for _, name := range request.AllowedTools {
		if _, ok := described[strings.ToLower(name)]; !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "requested tools are not available to this principal"})
			return
		}
	}
	execution, ok := agenttools.ExecutionContextFromContext(ctx)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "authenticated tool context is required"})
		return
	}
	if request.SkillID != "" {
		if h.skills == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent skill registry unavailable"})
			return
		}
		definition, found := h.skills.Resolve(request.SkillID, request.SkillVersion)
		if !found || !skillRoleAllowed(definition.AllowedRoles, execution.Principal.Role) {
			c.JSON(http.StatusForbidden, gin.H{"error": "requested Skill is not available to this principal"})
			return
		}
		if request.SkillVersion == "" {
			request.SkillVersion = definition.Version
		}
	}
	ttl := h.capability.TTL()
	if request.TTLSeconds > 0 {
		ttl = time.Duration(request.TTLSeconds) * time.Second
	}
	token, claims, err := h.capability.Issue(agentcapability.IssueRequest{
		UserID: execution.Principal.UserID, SessionID: strings.TrimSpace(request.SessionID), RunID: request.RunID,
		SkillID: request.SkillID, SkillVersion: request.SkillVersion,
		Scope: execution.Principal.Scope, Permissions: execution.Principal.Permissions,
		AllowedTools: request.AllowedTools, TTL: ttl,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "could not issue capability"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"capability_token": token,
		"token_type":       agentcapability.TokenType,
		"run_id":           claims.RunID,
		"skill_id":         claims.SkillID,
		"skill_version":    claims.SkillVersion,
		"allowed_tools":    claims.AllowedTools,
		"expires_at":       claims.ExpiresAt.Time.UTC().Format(time.RFC3339),
	})
}

// RevokeCapability invalidates a run-bound capability without storing the raw
// JWT. The normal JWT remains mandatory, so revocation cannot be used to
// manufacture a new identity or broaden scope.
func (h *AgentGatewayHandler) RevokeCapability(c *gin.Context) {
	if h == nil || h.capability == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent capability issuer unavailable"})
		return
	}
	var request revokeCapabilityRequest
	if err := decodeStrictJSON(c, &request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid capability revocation request"})
		return
	}
	request.RunID = strings.TrimSpace(request.RunID)
	request.TokenID = strings.TrimSpace(request.TokenID)
	if request.RunID == "" && request.TokenID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "run_id or token_id is required"})
		return
	}
	ctx, status, err := h.gatewayContext(c, request.RunID, "capability-revoke")
	if err != nil {
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	execution, ok := agenttools.ExecutionContextFromContext(ctx)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "authenticated tool context is required"})
		return
	}
	if request.RunID != "" {
		if err := h.capability.RevokeRunForUser(request.RunID, execution.Principal.UserID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "could not revoke capability run"})
			return
		}
	}
	if request.TokenID != "" {
		if err := h.capability.RevokeTokenForUser(request.TokenID, execution.Principal.UserID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "could not revoke capability token"})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"revoked": true, "run_id": request.RunID, "token_id": request.TokenID})
}

// Execute accepts only the versioned ToolCall protocol. Identity, permissions,
// tenant and scope fields are rejected as unknown JSON fields and are always
// resolved from the authenticated HTTP context.
func (h *AgentGatewayHandler) Execute(c *gin.Context) {
	if h == nil || h.runtime == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent tool gateway unavailable"})
		return
	}
	var call agenttools.ToolCall
	if err := decodeStrictJSON(c, &call); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tool call"})
		return
	}
	if strings.TrimSpace(call.CallID) == "" {
		call.CallID = uuid.NewString()
	}
	if strings.TrimSpace(call.RunID) == "" {
		call.RunID = uuid.NewString()
	}
	if strings.TrimSpace(call.TraceID) == "" {
		call.TraceID = uuid.NewString()
	}
	ctx, status, err := h.gatewayContext(c, call.RunID, call.TraceID)
	if err != nil {
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	if claims, ok := capabilityClaimsFromContext(c); ok {
		if call.SkillID != "" && !strings.EqualFold(call.SkillID, claims.SkillID) {
			c.JSON(http.StatusForbidden, gin.H{"error": "tool call skill does not match the capability"})
			return
		}
		if call.SkillVersion != "" && !strings.EqualFold(call.SkillVersion, claims.SkillVersion) {
			c.JSON(http.StatusForbidden, gin.H{"error": "tool call skill version does not match the capability"})
			return
		}
	}
	if execution, ok := agenttools.ExecutionContextFromContext(ctx); ok && call.SkillID != "" {
		execution.SkillID = call.SkillID
		execution.SkillVersion = call.SkillVersion
		ctx = agenttools.WithExecutionContext(ctx, execution)
	}

	result, executionErr := h.runtimeForRequest().Execute(ctx, call)
	if executionErr != nil && result.Error == nil {
		// Do not expose handler/database details. A structured ToolResult remains
		// the protocol response whenever Runtime has one.
		c.JSON(http.StatusInternalServerError, gin.H{"error": "agent tool execution failed"})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *AgentGatewayHandler) gatewayContext(c *gin.Context, runID, traceID string) (context.Context, int, error) {
	ctx, status, err := gatewayContext(c, runID, traceID)
	if err != nil {
		return ctx, status, err
	}
	if h == nil || h.capability == nil {
		return ctx, http.StatusOK, nil
	}
	rawCapability := strings.TrimSpace(c.GetHeader("X-Agent-Capability"))
	if rawCapability == "" {
		return ctx, http.StatusOK, nil
	}
	claims, err := h.capability.Parse(strings.TrimPrefix(rawCapability, "Bearer "))
	if err != nil {
		return nil, http.StatusUnauthorized, errors.New("invalid agent capability")
	}
	execution, ok := agenttools.ExecutionContextFromContext(ctx)
	if !ok || claims.UserID != execution.Principal.UserID {
		return nil, http.StatusForbidden, errors.New("agent capability subject mismatch")
	}
	if claims.RunID != runID {
		return nil, http.StatusForbidden, errors.New("agent capability is bound to a different run")
	}
	c.Set(stringCapabilityClaimsKey, claims)
	narrowedScope, ok := access.IntersectScopes(execution.Principal.Scope, claims.Scope())
	if !ok {
		return nil, http.StatusForbidden, errors.New("agent capability scope is outside the authenticated scope")
	}
	permissions := intersectPermissions(execution.Principal.Permissions, claims.Permissions)
	if len(permissions) == 0 {
		return nil, http.StatusForbidden, errors.New("agent capability has no remaining permissions")
	}
	execution.Principal.Scope = narrowedScope
	execution.Principal.Permissions = permissions
	execution.Principal.CapabilityGrants = append([]string(nil), claims.AllowedTools...)
	execution.Principal.CapabilityActive = true
	execution.SkillID = claims.SkillID
	execution.SkillVersion = claims.SkillVersion
	ctx = access.WithScope(ctx, narrowedScope)
	ctx = agenttools.WithExecutionContext(ctx, execution)
	return ctx, http.StatusOK, nil
}

// capabilityClaimsFromContext is backed by request-scoped server state rather
// than a client-provided header. It is only used to compare optional ToolCall
// skill metadata with verified capability claims.
func capabilityClaimsFromContext(c *gin.Context) (agentcapability.Claims, bool) {
	value, exists := c.Get(stringCapabilityClaimsKey)
	if !exists {
		return agentcapability.Claims{}, false
	}
	claims, ok := value.(agentcapability.Claims)
	return claims, ok
}

const stringCapabilityClaimsKey = "agent_capability_claims"

func skillRoleAllowed(allowed []string, role string) bool {
	role = strings.ToLower(strings.TrimSpace(role))
	if role == "user" {
		role = "editor"
	}
	for _, candidate := range allowed {
		if strings.EqualFold(strings.TrimSpace(candidate), role) {
			return true
		}
	}
	return false
}

func (h *AgentGatewayHandler) runtimeForRequest() agenttools.ToolRuntime {
	if h == nil || h.runtime == nil || h.audit == nil {
		return h.runtime
	}
	if runtime, ok := h.runtime.(*agenttools.Runtime); ok {
		return runtime.WithAudit(h.audit)
	}
	return h.runtime
}

// NewAgentToolAuditRecorder adapts the generic Tool audit record to the
// existing audit_logs table. Run IDs are retained in NewValues; record_id is
// required by the legacy table to be UUID-shaped, so non-UUID external run IDs
// receive a generated correlation row ID rather than being interpolated into
// SQL.
func NewAgentToolAuditRecorder(logger *auditservice.Logger) agenttools.AuditRecorder {
	if logger == nil {
		return nil
	}
	return agenttools.AuditRecorderFunc(func(ctx context.Context, record agenttools.ToolExecutionAudit) error {
		recordID := record.RunID
		if _, err := uuid.Parse(recordID); err != nil {
			recordID = uuid.NewString()
		}
		changedBy := record.UserID
		if _, err := uuid.Parse(changedBy); err != nil {
			changedBy = ""
		}
		return logger.LogEvent(ctx, "agent_tool_executions", recordID, "tool_execute", nil, record, auditservice.Metadata{
			ChangedBy: changedBy, LegalEntityID: record.LegalEntityID,
		})
	})
}

func gatewayContext(c *gin.Context, runID, traceID string) (context.Context, int, error) {
	userID, _ := c.Get("user_id")
	userIDString, _ := userID.(string)
	if strings.TrimSpace(userIDString) == "" {
		return nil, http.StatusUnauthorized, errors.New("missing authenticated user")
	}
	permissionsValue, _ := c.Get("permissions")
	permissions, ok := permissionsValue.([]string)
	if !ok {
		return nil, http.StatusForbidden, errors.New("invalid permission context")
	}
	role, _ := c.Get("role")
	roleString, _ := role.(string)
	scope, scoped := middleware.GetAccessScope(c)
	if !scoped {
		scope, scoped = access.ScopeFromContext(c.Request.Context())
	}
	if !scoped {
		return nil, http.StatusForbidden, errors.New("access scope required")
	}

	ctx := agenttools.WithExecutionContext(c.Request.Context(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{
			UserID: userIDString, SubjectType: "agent_gateway", Role: roleString,
			Permissions: append([]string(nil), permissions...), Scope: scope, AgentMode: "assist",
		},
		RunID: runID, TraceID: traceID,
	})
	return agenttools.WithDelegationCredential(ctx, c.GetHeader("Authorization")), http.StatusOK, nil
}

func normalizeNames(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, value)
	}
	return result
}

func intersectPermissions(base, restriction []string) []string {
	result := make([]string, 0)
	for _, permission := range base {
		if permissionCoveredByAny(permission, restriction) {
			result = appendUniqueString(result, permission)
		}
	}
	for _, permission := range restriction {
		if permissionCoveredByAny(permission, base) {
			result = appendUniqueString(result, permission)
		}
	}
	return result
}

func permissionCoveredByAny(required string, grants []string) bool {
	for _, grant := range grants {
		if permissionCovers(grant, required) {
			return true
		}
	}
	return false
}

func permissionCovers(grant, required string) bool {
	grantParts := strings.SplitN(strings.ToLower(strings.TrimSpace(grant)), ":", 2)
	requiredParts := strings.SplitN(strings.ToLower(strings.TrimSpace(required)), ":", 2)
	if len(grantParts) != 2 || len(requiredParts) != 2 {
		return false
	}
	return (grantParts[0] == "*" || grantParts[0] == requiredParts[0]) &&
		(grantParts[1] == "*" || grantParts[1] == requiredParts[1])
}

func appendUniqueString(values []string, value string) []string {
	for _, existing := range values {
		if strings.EqualFold(existing, value) {
			return values
		}
	}
	return append(values, value)
}

func describeFilter(c *gin.Context) (agenttools.ToolFilter, error) {
	names := splitQueryValues(c.QueryArray("name"))
	levelsRaw := splitQueryValues(c.QueryArray("level"))
	levels := make([]agenttools.ToolLevel, 0, len(levelsRaw))
	for _, raw := range levelsRaw {
		level := agenttools.ToolLevel(raw)
		switch level {
		case agenttools.LevelRead, agenttools.LevelDraft, agenttools.LevelCommand:
			levels = append(levels, level)
		default:
			return agenttools.ToolFilter{}, errors.New("invalid tool level")
		}
	}
	includeSchema := false
	if raw := strings.TrimSpace(c.Query("include_schema")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			return agenttools.ToolFilter{}, errors.New("include_schema must be boolean")
		}
		includeSchema = parsed
	}
	return agenttools.ToolFilter{
		Names: names, Levels: levels, SkillID: strings.TrimSpace(c.Query("skill_id")), IncludeSchema: includeSchema,
	}, nil
}

func splitQueryValues(values []string) []string {
	var result []string
	for _, value := range values {
		for _, item := range strings.Split(value, ",") {
			if item = strings.TrimSpace(item); item != "" {
				result = append(result, item)
			}
		}
	}
	return result
}

func decodeStrictJSON(c *gin.Context, target any) error {
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("multiple JSON values")
	}
	return nil
}
