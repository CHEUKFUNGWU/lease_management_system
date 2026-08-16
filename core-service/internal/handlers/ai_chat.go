package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agentskill"
	"github.com/lease-management-system/core-service/internal/agenttools"
	agenttooldefs "github.com/lease-management-system/core-service/internal/agenttools/tools"
	"github.com/lease-management-system/core-service/internal/aiagent"
	"github.com/lease-management-system/core-service/internal/aichat"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/draftapp"
)

// aiChatRuntimeStore is the read-side seam used by the HTTP adapters. Run
// transitions and writes live behind aichat.Runtime's interface.
type aiChatRuntimeStore interface {
	GetSessionByID(context.Context, string, string) (*repository.AIChatSession, error)
	ListSessions(context.Context, repository.AIChatSessionFilter) ([]*repository.AIChatSession, error)
	GetRunByID(context.Context, string, string) (*repository.AIChatRun, error)
	ListRunsBySession(context.Context, string, int, int) ([]*repository.AIChatRun, error)
	ListMessagesBySession(context.Context, string, int) ([]*repository.AIChatMessage, error)
	ListRunEvents(context.Context, string, int, int) ([]*repository.AIChatRunEvent, error)
	ListArtifactsBySession(context.Context, string, int) ([]*repository.AIChatArtifact, error)
	GetArtifactByID(context.Context, string, string) (*repository.AIChatArtifact, error)
	UpdateArtifactStatus(context.Context, string, string) error
	ListReviewActionsBySession(context.Context, string, int) ([]*repository.AIChatReviewAction, error)
}

// AgentRunStore is the narrow persistence seam used by the external Agent
// Gateway. It allows a Pi-like Runner to create/inspect/control a Core Run
// without gaining access to repositories or SQL.
type AgentRunStore interface {
	GetSessionByID(context.Context, string, string) (*repository.AIChatSession, error)
	CreateRun(context.Context, *repository.AIChatRun) error
	GetRunByID(context.Context, string, string) (*repository.AIChatRun, error)
	UpdateRunStatus(context.Context, string, string, bool, *string, *string, *time.Time, *time.Time) error
	AppendRunEvent(context.Context, *repository.AIChatRunEvent) error
	GetNextRunEventSequence(context.Context, string) (int, error)
	ListRunEvents(context.Context, string, int, int) ([]*repository.AIChatRunEvent, error)
}

// AgentRunCheckpointStore is kept separate from AgentRunStore so existing
// lightweight Gateway adapters remain compatible while production Core can
// persist Runner checkpoints on the owned AI Run row.
type AgentRunCheckpointStore interface {
	SaveRunCheckpoint(context.Context, string, string, json.RawMessage) error
	GetRunCheckpoint(context.Context, string, string) (json.RawMessage, error)
}

// AgentRunQueueStore is the worker-only lease seam. It deliberately does not
// expose SQL or arbitrary Run updates to the Agent Gateway.
type AgentRunQueueStore interface {
	ClaimQueuedRun(context.Context, string, time.Duration) (*repository.AIChatRun, string, error)
	HeartbeatRunLease(context.Context, string, string, string, time.Duration) error
	ReleaseRunLease(context.Context, string, string, string, bool) error
	RecoverExpiredRunLeases(context.Context) (int, error)
}

// AgentWorkerRunStore is the narrow data plane for a claimed Run. Unlike
// AgentRunStore, every operation is bound to the current worker lease, so a
// worker cannot read or mutate an arbitrary user's Run after the claim has
// expired or been released.
type AgentWorkerRunStore interface {
	GetClaimedRun(context.Context, string, string, string) (*repository.AIChatRun, error)
	ListClaimedRunEvents(context.Context, string, string, string, int, int) ([]*repository.AIChatRunEvent, error)
	AppendClaimedRunEvent(context.Context, string, string, string, *repository.AIChatRunEvent) error
	UpdateClaimedRunStatus(context.Context, string, string, string, string, bool, *string, *string, *time.Time, *time.Time) error
	SaveClaimedRunCheckpoint(context.Context, string, string, string, json.RawMessage) error
	GetClaimedRunCheckpoint(context.Context, string, string, string) (json.RawMessage, error)
}

// AgentSessionStore is the narrow persistence seam used to create an Agent
// session. The Gateway exposes only this operation, never the repository.
type AgentSessionStore interface {
	CreateSession(context.Context, *repository.AIChatSession) error
}

type AgentContractScopeReader interface {
	GetContractAttributes(context.Context, string) (access.ContractAttributes, bool, error)
}

// AgentAuditReader is the read-side seam for the unified Run Trace. The
// repository applies the current tenant and data-scope predicates before
// returning audit rows.
type AgentAuditReader interface {
	List(context.Context, repository.AuditLogFilter) ([]*repository.AuditLog, int, error)
}

type AgentRunAuditSummaryReader interface {
	GetRunAuditSummary(context.Context, string) (*repository.AgentRunAuditSummary, error)
}

type AgentRunTerminalAlertStore interface {
	ListTerminalAlerts(context.Context, string, string, int) ([]*repository.AgentRunTerminalAlert, error)
	AcknowledgeTerminalAlert(context.Context, string, string) error
}

type AgentRunAuditLinkReader interface {
	ListRunAuditLinks(context.Context, string, string) ([]*repository.AgentRunAuditLink, error)
	ListRunCheckpointAudits(context.Context, string, string) ([]*repository.AgentRunCheckpointAudit, error)
}

type AIChatHandler struct {
	runtimeRepo   aiChatRuntimeStore
	contractRepo  *repository.ContractRepository
	draftService  *draftapp.Service
	auditRepo     AgentAuditReader
	workerRuns    AgentWorkerRunStore
	agentRuntime  *aichat.Runtime[aiagent.Response]
	toolRuntime   *agenttools.Runtime
	skillRegistry *agentskill.Registry
}

func NewAIChatHandler(
	contractRepo *repository.ContractRepository,
	mcRepo *repository.MonthlyClosingRepository,
	eventRepo *repository.EventRepository,
	runtimeRepo *repository.AIChatRuntimeRepository,
	draftServices ...*draftapp.Service,
) *AIChatHandler {
	return newAIChatHandler(contractRepo, mcRepo, eventRepo, runtimeRepo, nil, draftServices...)
}

// NewAIChatHandlerWithPerformance wires the optional operating-facts read seam
// into the same server-owned Agent Tool Runtime as the lease tools.
func NewAIChatHandlerWithPerformance(
	contractRepo *repository.ContractRepository,
	mcRepo *repository.MonthlyClosingRepository,
	eventRepo *repository.EventRepository,
	runtimeRepo *repository.AIChatRuntimeRepository,
	performance agenttooldefs.PerformanceReader,
	draftServices ...*draftapp.Service,
) *AIChatHandler {
	return newAIChatHandler(contractRepo, mcRepo, eventRepo, runtimeRepo, performance, draftServices...)
}

func NewAIChatHandlerWithReaders(
	contractRepo *repository.ContractRepository,
	mcRepo *repository.MonthlyClosingRepository,
	eventRepo *repository.EventRepository,
	runtimeRepo *repository.AIChatRuntimeRepository,
	performance agenttooldefs.PerformanceReader,
	closeReadiness agenttooldefs.CloseReadinessReader,
	draftServices ...*draftapp.Service,
) *AIChatHandler {
	return newAIChatHandlerWithReaders(contractRepo, mcRepo, eventRepo, runtimeRepo, performance, closeReadiness, draftServices...)
}

func NewAIChatHandlerWithOperationalReaders(
	contractRepo *repository.ContractRepository,
	mcRepo *repository.MonthlyClosingRepository,
	eventRepo *repository.EventRepository,
	runtimeRepo *repository.AIChatRuntimeRepository,
	performance agenttooldefs.PerformanceReader,
	closeReadiness agenttooldefs.CloseReadinessReader,
	controls *agenttooldefs.ControlReaders,
	draftServices ...*draftapp.Service,
) *AIChatHandler {
	return newAIChatHandlerWithOperationalReaders(contractRepo, mcRepo, eventRepo, runtimeRepo, performance, closeReadiness, controls, draftServices...)
}

func NewAIChatHandlerWithOperationalReadersAndGovernance(
	contractRepo *repository.ContractRepository,
	mcRepo *repository.MonthlyClosingRepository,
	eventRepo *repository.EventRepository,
	runtimeRepo *repository.AIChatRuntimeRepository,
	performance agenttooldefs.PerformanceReader,
	closeReadiness agenttooldefs.CloseReadinessReader,
	controls *agenttooldefs.ControlReaders,
	governance agenttooldefs.DecisionMemoDraftWriter,
	draftServices ...*draftapp.Service,
) *AIChatHandler {
	return newAIChatHandlerWithOperationalReadersAndGovernance(contractRepo, mcRepo, eventRepo, runtimeRepo, performance, closeReadiness, controls, governance, draftServices...)
}

// NewAIChatHandlerWithOperationalReadersAndGovernanceAndRetail is the
// additive constructor used by production wiring for retail_operations@v1.
// Existing constructors deliberately keep their old registry contents.
func NewAIChatHandlerWithOperationalReadersAndGovernanceAndRetail(
	contractRepo *repository.ContractRepository,
	mcRepo *repository.MonthlyClosingRepository,
	eventRepo *repository.EventRepository,
	runtimeRepo *repository.AIChatRuntimeRepository,
	performance agenttooldefs.PerformanceReader,
	closeReadiness agenttooldefs.CloseReadinessReader,
	controls *agenttooldefs.ControlReaders,
	governance agenttooldefs.DecisionMemoDraftWriter,
	retail agenttooldefs.RetailOperationsReader,
	draftServices ...*draftapp.Service,
) *AIChatHandler {
	return newAIChatHandlerWithOperationalReadersAndGovernanceAndRetail(contractRepo, mcRepo, eventRepo, runtimeRepo, performance, closeReadiness, controls, governance, retail, draftServices...)
}

func newAIChatHandler(
	contractRepo *repository.ContractRepository,
	mcRepo *repository.MonthlyClosingRepository,
	eventRepo *repository.EventRepository,
	runtimeRepo *repository.AIChatRuntimeRepository,
	performance agenttooldefs.PerformanceReader,
	draftServices ...*draftapp.Service,
) *AIChatHandler {
	return newAIChatHandlerWithReaders(contractRepo, mcRepo, eventRepo, runtimeRepo, performance, nil, draftServices...)
}

func newAIChatHandlerWithReaders(
	contractRepo *repository.ContractRepository,
	mcRepo *repository.MonthlyClosingRepository,
	eventRepo *repository.EventRepository,
	runtimeRepo *repository.AIChatRuntimeRepository,
	performance agenttooldefs.PerformanceReader,
	closeReadiness agenttooldefs.CloseReadinessReader,
	draftServices ...*draftapp.Service,
) *AIChatHandler {
	return newAIChatHandlerWithOperationalReaders(contractRepo, mcRepo, eventRepo, runtimeRepo, performance, closeReadiness, nil, draftServices...)
}

func newAIChatHandlerWithOperationalReaders(
	contractRepo *repository.ContractRepository,
	mcRepo *repository.MonthlyClosingRepository,
	eventRepo *repository.EventRepository,
	runtimeRepo *repository.AIChatRuntimeRepository,
	performance agenttooldefs.PerformanceReader,
	closeReadiness agenttooldefs.CloseReadinessReader,
	controls *agenttooldefs.ControlReaders,
	draftServices ...*draftapp.Service,
) *AIChatHandler {
	agent := aiagent.NewWithOperationalReaders(contractRepo, mcRepo, eventRepo, performance, closeReadiness, controls, draftServices...)
	handler := &AIChatHandler{
		runtimeRepo: runtimeRepo, contractRepo: contractRepo,
		draftService: firstDraftService(draftServices), toolRuntime: agent.ToolRuntime(), skillRegistry: agent.SkillRegistry(),
	}
	handler.agentRuntime = aichat.NewRuntime(
		runtimeRepo, agent, agent, aiagent.ProjectResult,
		aichat.Options{ReviewCommit: handler.commitReviewTransaction},
	)
	return handler
}

func newAIChatHandlerWithOperationalReadersAndGovernance(
	contractRepo *repository.ContractRepository,
	mcRepo *repository.MonthlyClosingRepository,
	eventRepo *repository.EventRepository,
	runtimeRepo *repository.AIChatRuntimeRepository,
	performance agenttooldefs.PerformanceReader,
	closeReadiness agenttooldefs.CloseReadinessReader,
	controls *agenttooldefs.ControlReaders,
	governance agenttooldefs.DecisionMemoDraftWriter,
	draftServices ...*draftapp.Service,
) *AIChatHandler {
	return newAIChatHandlerWithOperationalReadersAndGovernanceAndRetail(contractRepo, mcRepo, eventRepo, runtimeRepo, performance, closeReadiness, controls, governance, nil, draftServices...)
}

func newAIChatHandlerWithOperationalReadersAndGovernanceAndRetail(
	contractRepo *repository.ContractRepository,
	mcRepo *repository.MonthlyClosingRepository,
	eventRepo *repository.EventRepository,
	runtimeRepo *repository.AIChatRuntimeRepository,
	performance agenttooldefs.PerformanceReader,
	closeReadiness agenttooldefs.CloseReadinessReader,
	controls *agenttooldefs.ControlReaders,
	governance agenttooldefs.DecisionMemoDraftWriter,
	retail agenttooldefs.RetailOperationsReader,
	draftServices ...*draftapp.Service,
) *AIChatHandler {
	agent := aiagent.NewWithOperationalReadersAndGovernanceAndRetail(contractRepo, mcRepo, eventRepo, performance, closeReadiness, controls, governance, retail, draftServices...)
	handler := &AIChatHandler{runtimeRepo: runtimeRepo, contractRepo: contractRepo, draftService: firstDraftService(draftServices), toolRuntime: agent.ToolRuntime(), skillRegistry: agent.SkillRegistry()}
	handler.agentRuntime = aichat.NewRuntime(runtimeRepo, agent, agent, aiagent.ProjectResult, aichat.Options{ReviewCommit: handler.commitReviewTransaction})
	return handler
}

func (h *AIChatHandler) WithAuditRepository(reader AgentAuditReader) *AIChatHandler {
	if h == nil {
		return h
	}
	clone := *h
	clone.auditRepo = reader
	return &clone
}

// WithWorkerRunStore enables the lease-protected event stream used by an
// external Runner. It is deliberately optional so the legacy AI Chat handler
// and lightweight test adapters remain owner-scoped and source-compatible.
func (h *AIChatHandler) WithWorkerRunStore(store AgentWorkerRunStore) *AIChatHandler {
	if h == nil {
		return h
	}
	clone := *h
	clone.workerRuns = store
	return &clone
}

func firstDraftService(services []*draftapp.Service) *draftapp.Service {
	if len(services) == 0 {
		return nil
	}
	return services[0]
}

// AgentToolRuntime is the adapter seam used by the standalone Agent Gateway.
// It intentionally returns the Tool Runtime rather than the Agent so Gateway
// callers cannot reach planner, model or repository internals.
func (h *AIChatHandler) AgentToolRuntime() agenttools.ToolRuntime {
	if h == nil {
		return nil
	}
	return h.toolRuntime
}

// AgentSkillRegistry is the read-only descriptor seam used by the Agent
// Gateway. It does not expose planner or repository internals.
func (h *AIChatHandler) AgentSkillRegistry() *agentskill.Registry {
	if h == nil {
		return nil
	}
	return h.skillRegistry
}

func (h *AIChatHandler) AgentRunStore() AgentRunStore {
	if h == nil || h.runtimeRepo == nil {
		return nil
	}
	store, _ := h.runtimeRepo.(AgentRunStore)
	return store
}

func (h *AIChatHandler) AgentRunCheckpointStore() AgentRunCheckpointStore {
	if h == nil || h.runtimeRepo == nil {
		return nil
	}
	store, _ := h.runtimeRepo.(AgentRunCheckpointStore)
	return store
}

func (h *AIChatHandler) AgentSessionStore() AgentSessionStore {
	if h == nil || h.runtimeRepo == nil {
		return nil
	}
	store, _ := h.runtimeRepo.(AgentSessionStore)
	return store
}

// GetDraftBatch exposes only the persisted progress envelope. The application
// service performs the owner check; this handler never reads the batch table.
func (h *AIChatHandler) GetDraftBatch(c *gin.Context) {
	if h == nil || h.draftService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "draft application service unavailable"})
		return
	}
	userID, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user context"})
		return
	}
	actorID, _ := userID.(string)
	batch, err := h.draftService.GetDraftBatch(c.Request.Context(), c.Param("id"), actorID)
	if err != nil {
		if errors.Is(err, draftapp.ErrDraftBatchNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "draft batch not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load draft batch"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"batch": batch})
}

type PageContext = aichat.PageContext
type ChatMessage = aichat.Message
type AIChatRequest = aiagent.Request

func (h *AIChatHandler) Chat(c *gin.Context) {
	var req AIChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Language == "" {
		req.Language = "zh-CN"
	}
	completed, err := h.agentRuntime.Run(c.Request.Context(), runtimeInput(c, req))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to run AI agent: " + err.Error()})
		return
	}
	response := completed.Response
	response.SessionID = completed.Started.Run.SessionID
	response.RunID = completed.Started.Run.ID
	c.JSON(http.StatusOK, response)
}

func runtimeInput(c *gin.Context, req AIChatRequest) aichat.Input {
	userID, _ := c.Get("user_id")
	userIDString, _ := userID.(string)
	role, _ := c.Get("role")
	roleString, _ := role.(string)
	permissions, _ := c.Get("permissions")
	permissionStrings, _ := permissions.([]string)
	return aichat.Input{
		SessionID: req.SessionID, Message: req.Message,
		ContractID: req.ContractID, History: req.History,
		FileID: req.FileID, ObjectName: req.ObjectName, ContentType: req.ContentType,
		Language: req.Language, PageContext: req.PageContext,
		SkillID: req.SkillID, SkillVersion: req.SkillVersion,
		Initiator: req.Initiator,
		UserID: userIDString, LegalEntityID: middleware.GetTenantID(c),
		Role: roleString, Permissions: append([]string(nil), permissionStrings...),
		AuthHeader: c.GetHeader("Authorization"),
	}
}
