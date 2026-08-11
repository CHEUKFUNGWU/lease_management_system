package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agentcapability"
	"github.com/lease-management-system/core-service/internal/agentskill"
	"github.com/lease-management-system/core-service/internal/agenttools"
	agenttooldefs "github.com/lease-management-system/core-service/internal/agenttools/tools"
	"github.com/lease-management-system/core-service/internal/repository"
)

type gatewayContractReader struct {
	attributes access.ContractAttributes
	contract   *repository.Contract
	fullReads  int
}

func (f *gatewayContractReader) GetContractAttributes(context.Context, string) (access.ContractAttributes, bool, error) {
	return f.attributes, f.contract != nil, nil
}

func (f *gatewayContractReader) GetByID(context.Context, string, string) (*repository.Contract, error) {
	f.fullReads++
	return f.contract, nil
}

func newGatewayTestRouter(runtime agenttools.ToolRuntime, permissions []string, scope access.Scope, auditRecorders ...agenttools.AuditRecorder) *gin.Engine {
	return newGatewayTestRouterWithRole(runtime, permissions, scope, "editor", auditRecorders...)
}

func newGatewayTestRouterWithRole(runtime agenttools.ToolRuntime, permissions []string, scope access.Scope, role string, auditRecorders ...agenttools.AuditRecorder) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", "user-1")
		c.Set("role", role)
		c.Set("permissions", permissions)
		c.Set("access_scope", scope)
		c.Request = c.Request.WithContext(access.WithScope(c.Request.Context(), scope))
		c.Next()
	})
	handler := NewAgentGatewayHandler(runtime, auditRecorders...).WithSkillRegistry(agentskill.ProductionRegistry())
	router.GET("/agent/tools", handler.Describe)
	router.GET("/agent/skills", handler.Skills)
	router.GET("/agent/metrics", handler.Metrics)
	router.GET("/agent/metrics/prometheus", handler.MetricsPrometheus)
	router.POST("/agent/tools/execute", handler.Execute)
	return router
}

type gatewayAuditRecorder struct {
	calls  int
	record agenttools.ToolExecutionAudit
}

type gatewayTerminalAlertStore struct {
	listCalls      int
	acknowledgedID string
	acknowledgedBy string
	alerts         []*repository.AgentRunTerminalAlert
}

type gatewayUsageReader struct {
	query repository.AgentUsageQuery
}

func (r *gatewayUsageReader) SummarizePlannerUsage(_ context.Context, query repository.AgentUsageQuery) (*repository.AgentUsageSummary, error) {
	r.query = query
	return &repository.AgentUsageSummary{
		From: query.From, To: query.To, PlannerCalls: 1, TotalTokens: 15,
		CostMicros: 3, CostAccountingAvailable: true,
		Rollups: []repository.AgentUsageRollup{{Provider: "deepseek", Model: "deepseek-test", CostStatus: "calculated", PlannerCalls: 1, TotalTokens: 15, CostMicros: 3}},
	}, nil
}

func (s *gatewayTerminalAlertStore) ListTerminalAlerts(_ context.Context, userID, status string, limit int) ([]*repository.AgentRunTerminalAlert, error) {
	s.listCalls++
	if userID != "user-1" || status != "pending" || limit != 25 {
		return nil, context.Canceled
	}
	return s.alerts, nil
}

func (s *gatewayTerminalAlertStore) AcknowledgeTerminalAlert(_ context.Context, alertID, userID string) error {
	s.acknowledgedID, s.acknowledgedBy = alertID, userID
	return nil
}

func (r *gatewayAuditRecorder) RecordToolExecution(_ context.Context, record agenttools.ToolExecutionAudit) error {
	r.calls++
	r.record = record
	return nil
}

func newContractGatewayRuntime(reader *gatewayContractReader) agenttools.ToolRuntime {
	registry := agenttools.NewRegistry()
	if err := registry.Register(agenttooldefs.NewContractGetDefinition(reader)); err != nil {
		panic(err)
	}
	return agenttools.NewRuntime(registry, agenttools.RuntimeOptions{})
}

func TestAgentGatewayDescribeFiltersByAuthenticatedPermission(t *testing.T) {
	reader := &gatewayContractReader{
		attributes: access.ContractAttributes{LegalEntityID: "le-001"},
		contract:   &repository.Contract{ID: "contract-1", ContractName: "Lease"},
	}
	router := newGatewayTestRouter(newContractGatewayRuntime(reader), []string{"contracts:read"}, access.Scope{LegalEntityID: "le-001"})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/agent/tools?include_schema=true", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Tools []agenttools.ToolDescriptor `json:"tools"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Tools) != 1 || response.Tools[0].Name != "lease.contract.get" {
		t.Fatalf("described tools=%#v", response.Tools)
	}
	if len(response.Tools[0].InputSchema) == 0 {
		t.Fatal("expected requested schema")
	}
}

func TestAgentGatewaySkillsFilterByAuthoritativeRole(t *testing.T) {
	router := newGatewayTestRouter(nil, []string{"ai_chat:use"}, access.Scope{LegalEntityID: "le-001"})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/agent/skills", nil)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Skills []struct {
			ID string `json:"id"`
		} `json:"skills"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	for _, skill := range response.Skills {
		if skill.ID == "audit_pack" {
			t.Fatal("editor must not discover auditor-only audit_pack skill")
		}
	}
	if len(response.Skills) == 0 {
		t.Fatal("expected at least one editor skill")
	}
}

func TestAgentGatewayMetricsRequiresPermissionAndExposesPrometheus(t *testing.T) {
	reader := &gatewayContractReader{
		attributes: access.ContractAttributes{LegalEntityID: "le-001"},
		contract:   &repository.Contract{ID: "contract-1", ContractName: "Lease"},
	}
	runtime := newContractGatewayRuntime(reader)
	allowedRouter := newGatewayTestRouter(runtime, []string{"contracts:read", "agent_runtime:metrics"}, access.Scope{LegalEntityID: "le-001"})
	callRecorder := httptest.NewRecorder()
	callRequest := httptest.NewRequest(http.MethodPost, "/agent/tools/execute", strings.NewReader(`{"call_id":"call-1","run_id":"run-1","tool_name":"lease.contract.get","tool_version":"v1","arguments":{"contract_id":"contract-1"}}`))
	callRequest.Header.Set("Content-Type", "application/json")
	allowedRouter.ServeHTTP(callRecorder, callRequest)
	if callRecorder.Code != http.StatusOK {
		t.Fatalf("execute status=%d body=%s", callRecorder.Code, callRecorder.Body.String())
	}
	metricsRecorder := httptest.NewRecorder()
	allowedRouter.ServeHTTP(metricsRecorder, httptest.NewRequest(http.MethodGet, "/agent/metrics/prometheus", nil))
	if metricsRecorder.Code != http.StatusOK || !strings.Contains(metricsRecorder.Body.String(), "lease_agent_tool_executions_total") {
		t.Fatalf("metrics status=%d body=%s", metricsRecorder.Code, metricsRecorder.Body.String())
	}
	deniedRouter := newGatewayTestRouter(newContractGatewayRuntime(reader), []string{"contracts:read"}, access.Scope{LegalEntityID: "le-001"})
	deniedRecorder := httptest.NewRecorder()
	deniedRouter.ServeHTTP(deniedRecorder, httptest.NewRequest(http.MethodGet, "/agent/metrics", nil))
	if deniedRecorder.Code != http.StatusForbidden {
		t.Fatalf("metrics without permission status=%d body=%s", deniedRecorder.Code, deniedRecorder.Body.String())
	}
}

func TestAgentGatewayUsageIsPermissionedAndTenantDerived(t *testing.T) {
	usage := &gatewayUsageReader{}
	gin.SetMode(gin.TestMode)
	allowed := gin.New()
	allowed.Use(func(c *gin.Context) {
		c.Set("user_id", "user-1")
		c.Set("role", "editor")
		c.Set("permissions", []string{"agent_runtime:metrics"})
		scope := access.Scope{LegalEntityID: "le-001"}
		c.Set("access_scope", scope)
		c.Request = c.Request.WithContext(access.WithScope(c.Request.Context(), scope))
		c.Next()
	})
	allowed.GET("/agent/usage", NewAgentGatewayHandler(nil).WithUsageStore(usage).Usage)
	response := httptest.NewRecorder()
	allowed.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/agent/usage", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "deepseek-test") {
		t.Fatalf("usage status=%d body=%s", response.Code, response.Body.String())
	}
	if usage.query.UserID != "user-1" || usage.query.LegalEntityID != "le-001" || usage.query.Global {
		t.Fatalf("usage query=%+v", usage.query)
	}

	denied := gin.New()
	denied.Use(func(c *gin.Context) {
		c.Set("user_id", "user-1")
		c.Set("role", "editor")
		c.Set("permissions", []string{"contracts:read"})
		scope := access.Scope{LegalEntityID: "le-001"}
		c.Set("access_scope", scope)
		c.Request = c.Request.WithContext(access.WithScope(c.Request.Context(), scope))
		c.Next()
	})
	denied.GET("/agent/usage", NewAgentGatewayHandler(nil).WithUsageStore(usage).Usage)
	deniedResponse := httptest.NewRecorder()
	denied.ServeHTTP(deniedResponse, httptest.NewRequest(http.MethodGet, "/agent/usage", nil))
	if deniedResponse.Code != http.StatusForbidden {
		t.Fatalf("usage without permission status=%d body=%s", deniedResponse.Code, deniedResponse.Body.String())
	}
}

func TestAgentGatewayTerminalAlertsAreOwnerScopedAndAcknowledged(t *testing.T) {
	store := &gatewayTerminalAlertStore{alerts: []*repository.AgentRunTerminalAlert{{ID: "alert-1", RunID: "run-1", Status: "pending"}}}
	router := newGatewayTestRouter(nil, []string{"ai_chat:use"}, access.Scope{LegalEntityID: "le-001"})
	handler := NewAgentGatewayHandler(nil).WithTerminalAlertStore(store)
	router.GET("/agent/alerts/terminal", handler.ListTerminalAlerts)
	router.POST("/agent/alerts/terminal/:id/ack", handler.AcknowledgeTerminalAlert)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/agent/alerts/terminal?status=pending&limit=25", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "alert-1") {
		t.Fatalf("list status=%d body=%s", response.Code, response.Body.String())
	}
	ack := httptest.NewRecorder()
	router.ServeHTTP(ack, httptest.NewRequest(http.MethodPost, "/agent/alerts/terminal/alert-1/ack", nil))
	if ack.Code != http.StatusAccepted || store.acknowledgedID != "alert-1" || store.acknowledgedBy != "user-1" {
		t.Fatalf("ack status=%d body=%s store=%+v", ack.Code, ack.Body.String(), store)
	}
}

func TestAgentGatewayRejectsCrossStoreContractBeforeFullRead(t *testing.T) {
	reader := &gatewayContractReader{
		attributes: access.ContractAttributes{LegalEntityID: "le-001", StoreID: "store-foreign"},
		contract:   &repository.Contract{ID: "contract-foreign", ContractName: "Foreign Lease"},
	}
	router := newGatewayTestRouter(newContractGatewayRuntime(reader), []string{"contracts:read"}, access.Scope{
		LegalEntityID: "le-001", StoreIDs: []string{"store-allowed"},
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/agent/tools/execute", stringsReader(`{
  "call_id":"call-1",
  "run_id":"run-1",
  "tool_name":"lease.contract.get",
  "tool_version":"v1",
  "arguments":{"contract_id":"contract-foreign"}
}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	var result agenttools.ToolResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode result: %v; body=%s", err, recorder.Body.String())
	}
	if recorder.Code != http.StatusOK || result.Status != agenttools.StatusRejected || result.Error == nil || result.Error.Code != agenttools.ErrorNotFound {
		t.Fatalf("status=%d result=%#v", recorder.Code, result)
	}
	if reader.fullReads != 0 {
		t.Fatalf("full contract reads=%d, want 0", reader.fullReads)
	}
}

func TestAgentGatewayRejectsContractWhenLegalEntityScopeIsEmpty(t *testing.T) {
	reader := &gatewayContractReader{
		attributes: access.ContractAttributes{LegalEntityID: "le-001"},
		contract:   &repository.Contract{ID: "contract-1", ContractName: "Lease"},
	}
	router := newGatewayTestRouter(newContractGatewayRuntime(reader), []string{"contracts:read"}, access.Scope{})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/agent/tools/execute", stringsReader(`{
  "call_id":"call-empty-tenant",
  "run_id":"run-empty-tenant",
  "tool_name":"lease.contract.get",
  "tool_version":"v1",
  "arguments":{"contract_id":"contract-1"}
}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	var result agenttools.ToolResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode result: %v; body=%s", err, recorder.Body.String())
	}
	if recorder.Code != http.StatusOK || result.Status != agenttools.StatusRejected || result.Error == nil || result.Error.Code != agenttools.ErrorNotFound {
		t.Fatalf("status=%d result=%#v", recorder.Code, result)
	}
	if reader.fullReads != 0 {
		t.Fatalf("handler executed without legal entity scope: %d", reader.fullReads)
	}
}

func TestAgentGatewayFiltersSkillByAuthenticatedRole(t *testing.T) {
	router := newGatewayTestRouterWithRole(nil, []string{"contracts:read"}, access.Scope{LegalEntityID: "le-001"}, "readonly")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/agent/skills", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Skills []struct {
			ID string `json:"id"`
		} `json:"skills"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	for _, skill := range response.Skills {
		if skill.ID == "excel_ledger" || skill.ID == "payment_schedule" {
			t.Fatalf("readonly role must not discover draft skill %q", skill.ID)
		}
	}
}

func TestAgentGatewayUsesAuthenticatedPermissionNotForgedJSONFields(t *testing.T) {
	reader := &gatewayContractReader{
		attributes: access.ContractAttributes{LegalEntityID: "le-001"},
		contract:   &repository.Contract{ID: "contract-1"},
	}
	router := newGatewayTestRouter(newContractGatewayRuntime(reader), []string{"contracts:read"}, access.Scope{LegalEntityID: "le-001"})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/agent/tools/execute", stringsReader(`{
  "call_id":"call-1",
  "run_id":"run-1",
  "tool_name":"lease.contract.get",
  "tool_version":"v1",
  "arguments":{"contract_id":"contract-1"},
  "user_id":"attacker",
  "legal_entity_id":"le-999",
  "permissions":["*:*"]
}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if reader.fullReads != 0 {
		t.Fatalf("handler executed after forged identity fields: %d", reader.fullReads)
	}
}

func TestAgentGatewayReturnsPermissionRejectionWithoutRunningTool(t *testing.T) {
	reader := &gatewayContractReader{
		attributes: access.ContractAttributes{LegalEntityID: "le-001"},
		contract:   &repository.Contract{ID: "contract-1"},
	}
	router := newGatewayTestRouter(newContractGatewayRuntime(reader), []string{"events:read"}, access.Scope{LegalEntityID: "le-001"})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/agent/tools/execute", stringsReader(`{
  "call_id":"call-1",
  "run_id":"run-1",
  "tool_name":"lease.contract.get",
  "tool_version":"v1",
  "arguments":{"contract_id":"contract-1"}
}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	var result agenttools.ToolResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if recorder.Code != http.StatusOK || result.Status != agenttools.StatusRejected || result.Error == nil || result.Error.Code != agenttools.ErrorPermissionDenied {
		t.Fatalf("status=%d result=%#v", recorder.Code, result)
	}
	if reader.fullReads != 0 {
		t.Fatalf("handler executed after permission rejection: %d", reader.fullReads)
	}
}

func TestAgentGatewayAttachesPerRequestToolAudit(t *testing.T) {
	reader := &gatewayContractReader{
		attributes: access.ContractAttributes{LegalEntityID: "le-001"},
		contract:   &repository.Contract{ID: "contract-1", ContractName: "Lease"},
	}
	recorder := &gatewayAuditRecorder{}
	router := newGatewayTestRouter(newContractGatewayRuntime(reader), []string{"contracts:read"}, access.Scope{LegalEntityID: "le-001"}, recorder)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/agent/tools/execute", stringsReader(`{
  "call_id":"call-audit",
  "run_id":"run-audit",
  "trace_id":"trace-audit",
  "tool_name":"lease.contract.get",
  "tool_version":"v1",
  "arguments":{"contract_id":"contract-1"}
}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK || recorder.calls != 1 {
		t.Fatalf("status=%d audit_calls=%d body=%s", response.Code, recorder.calls, response.Body.String())
	}
	if recorder.record.ToolName != "lease.contract.get" || recorder.record.UserID != "user-1" || recorder.record.LegalEntityID != "le-001" {
		t.Fatalf("audit record=%#v", recorder.record)
	}
}

func TestAgentGatewayCapabilityRestrictsToolAndBindsRun(t *testing.T) {
	reader := &gatewayContractReader{
		attributes: access.ContractAttributes{LegalEntityID: "le-001"},
		contract:   &repository.Contract{ID: "contract-1", ContractName: "Lease"},
	}
	registry := agenttools.NewRegistry()
	if err := registry.Register(agenttooldefs.NewContractGetDefinition(reader)); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name: "lease.event.list", Version: "v1", Description: "event read", Level: agenttools.LevelRead,
			ReadOnly: true, Permissions: []agenttools.Permission{{Resource: "events", Action: "read"}},
		},
		Handler: func(context.Context, agenttools.ToolCall) (agenttools.ToolResult, error) {
			t.Fatal("capability-denied tool must not run")
			return agenttools.ToolResult{}, nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	runtime := agenttools.NewRuntime(registry, agenttools.RuntimeOptions{})
	issuer, err := agentcapability.NewIssuer("test-capability-secret", "lease-agent-gateway", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	capability, _, err := issuer.Issue(agentcapability.IssueRequest{
		UserID: "user-1", RunID: "run-1", Scope: access.Scope{LegalEntityID: "le-001"},
		Permissions: []string{"contracts:read", "events:read"}, AllowedTools: []string{"lease.contract.get"},
	})
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", "user-1")
		c.Set("role", "editor")
		c.Set("permissions", []string{"contracts:read", "events:read"})
		scope := access.Scope{LegalEntityID: "le-001"}
		c.Set("access_scope", scope)
		c.Request = c.Request.WithContext(access.WithScope(c.Request.Context(), scope))
		c.Next()
	})
	handler := NewAgentGatewayHandler(runtime).WithCapabilityIssuer(issuer)
	router.POST("/agent/tools/execute", handler.Execute)

	deniedResponse := httptest.NewRecorder()
	deniedRequest := httptest.NewRequest(http.MethodPost, "/agent/tools/execute", stringsReader(`{
  "call_id":"call-1", "run_id":"run-1", "tool_name":"lease.event.list", "tool_version":"v1", "arguments":{"contract_id":"contract-1"}
}`))
	deniedRequest.Header.Set("Content-Type", "application/json")
	deniedRequest.Header.Set("X-Agent-Capability", capability)
	router.ServeHTTP(deniedResponse, deniedRequest)
	var denied agenttools.ToolResult
	if err := json.Unmarshal(deniedResponse.Body.Bytes(), &denied); err != nil {
		t.Fatal(err)
	}
	if deniedResponse.Code != http.StatusOK || denied.Status != agenttools.StatusRejected || denied.Error == nil || denied.Error.Code != agenttools.ErrorCapabilityDenied {
		t.Fatalf("status=%d result=%#v", deniedResponse.Code, denied)
	}

	mismatchResponse := httptest.NewRecorder()
	mismatchRequest := httptest.NewRequest(http.MethodPost, "/agent/tools/execute", stringsReader(`{
  "call_id":"call-2", "run_id":"run-2", "tool_name":"lease.contract.get", "tool_version":"v1", "arguments":{"contract_id":"contract-1"}
}`))
	mismatchRequest.Header.Set("Content-Type", "application/json")
	mismatchRequest.Header.Set("X-Agent-Capability", capability)
	router.ServeHTTP(mismatchResponse, mismatchRequest)
	if mismatchResponse.Code != http.StatusForbidden {
		t.Fatalf("run mismatch status=%d body=%s", mismatchResponse.Code, mismatchResponse.Body.String())
	}
}

func TestAgentGatewayIssuesCapabilityOnlyForDiscoverableTools(t *testing.T) {
	reader := &gatewayContractReader{
		attributes: access.ContractAttributes{LegalEntityID: "le-001"},
		contract:   &repository.Contract{ID: "contract-1", ContractName: "Lease"},
	}
	runtime := newContractGatewayRuntime(reader)
	issuer, err := agentcapability.NewIssuer("test-capability-secret", "lease-agent-gateway", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", "user-1")
		c.Set("role", "editor")
		c.Set("permissions", []string{"contracts:read"})
		scope := access.Scope{LegalEntityID: "le-001"}
		c.Set("access_scope", scope)
		c.Request = c.Request.WithContext(access.WithScope(c.Request.Context(), scope))
		c.Next()
	})
	handler := NewAgentGatewayHandler(runtime).WithCapabilityIssuer(issuer)
	router.POST("/agent/capabilities", handler.IssueCapability)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/agent/capabilities", stringsReader(`{
  "run_id":"run-issue", "allowed_tools":["lease.contract.get"]
}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Token string `json:"capability_token"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	claims, err := issuer.Parse(body.Token)
	if err != nil || claims.RunID != "run-issue" || !claims.AllowsTool("lease.contract.get") {
		t.Fatalf("token claims=%+v err=%v", claims, err)
	}

	forbidden := httptest.NewRecorder()
	forbiddenRequest := httptest.NewRequest(http.MethodPost, "/agent/capabilities", stringsReader(`{
  "run_id":"run-issue", "allowed_tools":["lease.event.list"]
}`))
	forbiddenRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(forbidden, forbiddenRequest)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("unknown/unpermitted tool status=%d body=%s", forbidden.Code, forbidden.Body.String())
	}
}

func TestAgentGatewayRevokesRunBoundCapability(t *testing.T) {
	reader := &gatewayContractReader{
		attributes: access.ContractAttributes{LegalEntityID: "le-001"},
		contract:   &repository.Contract{ID: "contract-1"},
	}
	runtime := newContractGatewayRuntime(reader)
	issuer, err := agentcapability.NewIssuer("test-capability-secret", "lease-agent-gateway", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := issuer.Issue(agentcapability.IssueRequest{
		UserID: "user-1", RunID: "run-revoke", Scope: access.Scope{LegalEntityID: "le-001"},
		Permissions: []string{"contracts:read"}, AllowedTools: []string{"lease.contract.get"},
	})
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", "user-1")
		c.Set("role", "editor")
		c.Set("permissions", []string{"contracts:read"})
		scope := access.Scope{LegalEntityID: "le-001"}
		c.Set("access_scope", scope)
		c.Request = c.Request.WithContext(access.WithScope(c.Request.Context(), scope))
		c.Next()
	})
	handler := NewAgentGatewayHandler(runtime).WithCapabilityIssuer(issuer)
	router.POST("/agent/capabilities/revoke", handler.RevokeCapability)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/agent/capabilities/revoke", stringsReader(`{"run_id":"run-revoke"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if _, err := issuer.Parse(token); err == nil {
		t.Fatal("revoked capability should not parse")
	}
}

func stringsReader(value string) *strings.Reader {
	return strings.NewReader(value)
}
