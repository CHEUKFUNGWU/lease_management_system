package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agentkernel/chatexec"
	"github.com/lease-management-system/core-service/internal/agentskill"
	"github.com/lease-management-system/core-service/internal/agenttools"
	agenttooldefs "github.com/lease-management-system/core-service/internal/agenttools/tools"
	"github.com/lease-management-system/core-service/internal/aiagent"
	"github.com/lease-management-system/core-service/internal/aichat"
	"github.com/lease-management-system/core-service/internal/contextassembler"
	"github.com/lease-management-system/core-service/internal/docparse"
	"github.com/lease-management-system/core-service/internal/errcontract"
	finadapter "github.com/lease-management-system/core-service/internal/finmodel/adapter"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/agentguard"
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
	agent         *aiagent.Agent
	runtimeRepo   aiChatRuntimeStore
	contractRepo  *repository.ContractRepository
	draftService  *draftapp.Service
	auditRepo     AgentAuditReader
	workerRuns    AgentWorkerRunStore
	agentRuntime  *aichat.Runtime[aiagent.Response]
	toolRuntime   *agenttools.Runtime
	skillRegistry *agentskill.Registry
	guard         *agentguard.Guard
}

// SetDocumentParser injects the document parser seam (W5-1) into the inner Agent used by the chat/parse paths.
func (h *AIChatHandler) SetDocumentParser(p docparse.DocumentParser) {
	if h == nil || h.agent == nil {
		return
	}
	h.agent.SetDocumentParser(p)
}

// SetFileBytesReader injects the MinIO read seam (W5-3) into the inner Agent's
// intake parse endpoints.
func (h *AIChatHandler) SetFileBytesReader(f aiagent.FileBytesReader) {
	if h == nil || h.agent == nil {
		return
	}
	h.agent.SetFileBytesReader(f)
}

// NewAIChatHandlerWithOperationalReadersAndGovernanceAndRetail is the
// production constructor used by cmd/api wiring.
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
	sensitivity agenttooldefs.SensitivityReader,
	fillReader agenttooldefs.IngestFileReader,
	storePnl agenttooldefs.StorePnlReader,
	finModelRepo *repository.FinModelRepository,
	facts finadapter.FactsSource,
	plans *repository.FPnAGovernanceRepository,
	factsReader agenttooldefs.OperatingFactsReader,
	reports agenttooldefs.ReportReader,
	draftServices ...*draftapp.Service,
) *AIChatHandler {
	agent := aiagent.NewWithOperationalReadersAndGovernanceAndRetail(contractRepo, mcRepo, eventRepo, performance, closeReadiness, controls, governance, retail, sensitivity, fillReader, storePnl, finModelRepo, facts, plans, factsReader, reports, draftServices...)
	handler := &AIChatHandler{agent: agent, runtimeRepo: runtimeRepo, contractRepo: contractRepo, draftService: firstDraftService(draftServices), toolRuntime: agent.ToolRuntime(), skillRegistry: agent.SkillRegistry()}
	// AR5d convergence (ADR-0028 §5): the chat plane's executor is the kernel
	// adapter. Persistence, planning and projection keep their previous
	// injections; only the executor changes. The guarded convergence assertion
	// lives in ai_chat_kernel_convergence_test.go.
	executor := chatexec.New(chatexec.Deps{
		Domain: agent, Tools: handler.toolRuntime, MaxToolCalls: chatexec.DefaultChatToolBudget,
	})
	handler.agentRuntime = aichat.NewRuntime(runtimeRepo, agent, executor, aiagent.ProjectResult, aichat.Options{ReviewCommit: handler.commitReviewTransaction})
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
// WithGuard attaches the M6.3 budget guard (rate + cost + history bounds);
// a nil guard keeps the previous behaviour.
func (h *AIChatHandler) WithGuard(guard *agentguard.Guard) *AIChatHandler {
	h.guard = guard
	return h
}

// SetContextAssembler injects the AR3 context assembler into the inner chat
// agent. Wiring-only seam (feature flag lives in cmd/api); tests may also use
// it to pin a stub assembler. Nil keeps the legacy history path.
func (h *AIChatHandler) SetContextAssembler(a contextassembler.Assembler) {
	if h == nil || h.agent == nil {
		return
	}
	h.agent.SetContextAssembler(a)
}

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

// GetArtifact returns one artifact with its data — the consumption seam for
// page_fill deep links (the import page reads the fill and renders it).
func (h *AIChatHandler) GetArtifact(c *gin.Context) {
	userID, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user context"})
		return
	}
	userIDStr, _ := userID.(string)
	artifact, err := h.runtimeRepo.GetArtifactByID(c.Request.Context(), c.Param("id"), userIDStr)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "artifact not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"artifact": artifact})
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
	// M6.3: the budget guard refuses before any LLM call; the reason is
	// preserved verbatim in the 429 body (never softened).
	if h.guard != nil {
		// M6.3: the budget is consumed atomically (check + book in one
		// step) before any LLM call; the reason is preserved verbatim in
		// the 429 body (never softened). Token usage is not observable on
		// this path yet, so the rate window counts and cost accrues once
		// usage is plumbed.
		if guardErr := h.guard.Consume(c.Request.Context(), userIDFromContext(c), "chat", 0); guardErr != nil {
			writeCodedError(c, http.StatusTooManyRequests, errcontract.CodeRateLimited, guardErr.Error(), nil)
			return
		}
		req.History = agentguard.BoundHistory(h.guard, req.History, func(message ChatMessage) string {
			return message.Role + "\n" + message.Content
		})
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
		UserID:    userIDString, LegalEntityID: middleware.GetTenantID(c),
		Role: roleString, Permissions: append([]string(nil), permissionStrings...),
		AuthHeader: c.GetHeader("Authorization"),
	}
}
