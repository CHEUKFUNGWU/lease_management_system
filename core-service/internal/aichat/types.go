package aichat

import (
	"context"
	"encoding/json"
	"time"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agentartifact"
	"github.com/lease-management-system/core-service/internal/repository"
)

type PageContext struct {
	Page       string            `json:"page,omitempty"`
	Title      string            `json:"title,omitempty"`
	ContractID string            `json:"contract_id,omitempty"`
	Period     string            `json:"period,omitempty"`
	ReportView string            `json:"report_view,omitempty"`
	Filters    map[string]string `json:"filters,omitempty"`
	Summary    string            `json:"summary,omitempty"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Input struct {
	SessionID     string
	ParentRunID   string
	Message       string
	ContractID    string
	History       []Message
	FileID        string
	ObjectName    string
	ContentType   string
	Language      string
	PageContext   *PageContext
	UserID        string
	LegalEntityID string
	Role          string
	Permissions   []string
	AuthHeader    string
	AgentMode     *bool
	SkillID       string
	SkillVersion  string
	// Initiator marks who started the session when this run creates one:
	// "user" (default) or "system" for automatic runs (CHAT-001 home brief).
	// It is written onto the session so the user-facing list can filter it.
	Initiator string
}

type SessionCommand struct {
	UserID          string
	LegalEntityID   string
	Title           string
	BoundContractID string
	ContextSnapshot any
	Initiator       string
}

type Plan struct {
	AgentMode      bool
	SkillID        string
	SkillVersion   string
	ReviewRequired bool
	AgentPlan      any
	ToolCalls      any
	ReviewPrompts  any
	Payload        any
	// QueueForWorker marks the plan for the Gateway plane (G1 bridge): the
	// chat run stays queued, the agent-runner worker claims it and executes
	// the planner-driven tool sequence, and its events stream back into the
	// same chat timeline. The chat plane must not execute such runs.
	QueueForWorker bool
}

type ArtifactDraft struct {
	Type             string
	Title            string
	Data             any
	ReviewRequired   bool
	SchemaVersion    string
	EvidenceRefs     []agentartifact.EvidenceReference
	EvidenceComplete bool
	ReviewReasons    []string
	ModelVersion     string
	RuleVersion      string
}

type Result struct {
	Answer         string
	Model          string
	Sources        any
	ToolCalls      any
	ReviewPrompts  any
	ReviewRequired bool
	Artifacts      []ArtifactDraft
	// Confidence and ConfidenceReason are the assistant answer's measured
	// confidence and its degradation reason; they are persisted with the
	// message so a reloaded session can still render the ConfidenceBadge.
	// Nil means no confidence is available.
	Confidence       *float64
	ConfidenceReason *string
	// MeasuredTokens carries the provider-reported ROUND TOTAL of prompt
	// tokens for the round (AF1-a: not a per-message count). Persisted onto
	// the assistant message row; 0 means not measured — never a measured zero.
	MeasuredTokens int
}

type Execution struct {
	Input   Input
	Session *repository.AIChatSession
	Run     *repository.AIChatRun
	Plan    Plan
	Emit    func(context.Context, string, any) error
}

type Executor[T any] interface {
	Execute(context.Context, Execution) (T, error)
}

type ExecutorFunc[T any] func(context.Context, Execution) (T, error)

func (f ExecutorFunc[T]) Execute(ctx context.Context, execution Execution) (T, error) {
	return f(ctx, execution)
}

type Planner interface {
	Plan(Input, *repository.AIChatRun) Plan
}

type PlannerFunc func(Input, *repository.AIChatRun) Plan

func (f PlannerFunc) Plan(input Input, sourceRun *repository.AIChatRun) Plan {
	return f(input, sourceRun)
}

type Projector[T any] func(T) Result

type Started struct {
	Run            *repository.AIChatRun
	TriggerMessage *repository.AIChatMessage
	Plan           Plan
	StreamPath     string
	Continuation   *Continuation
}

type Completed[T any] struct {
	Started        *Started
	Response       T
	ExecutionError error
}

type Target struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type ContinueCommand struct {
	Target        Target
	Instruction   string
	ContractID    string
	Language      string
	PageContext   *PageContext
	UserID        string
	LegalEntityID string
	Role          string
	Permissions   []string
	AuthHeader    string
}

type Continuation struct {
	Target              Target `json:"target"`
	ParentRunID         string `json:"parent_run_id"`
	ResolvedInstruction string `json:"resolved_instruction"`
	EffectiveContractID string `json:"effective_contract_id"`
	ResolvedSkillID     string `json:"resolved_skill_id"`
	HistoryMessageCount int    `json:"history_message_count"`
	CompressionStrategy string `json:"compression_strategy"`
}

type ReviewCommand struct {
	ArtifactID    string
	ActionType    string
	ActionPayload map[string]any
	Comment       string
	UserID        string
	FollowUp      *ContinueCommand
}

type ReviewResult struct {
	Action         *repository.AIChatReviewAction
	ArtifactID     string
	ArtifactStatus string
	CommitResult   any `json:"-"`
	FollowUp       *Started
}

type Inspection struct {
	Run    *repository.AIChatRun
	Events []*repository.AIChatRunEvent
}

// store is an internal seam. Runtime callers use command methods and do not
// learn the persistence operation set; production and in-memory adapters are
// selected only while constructing the module.
type store interface {
	CreateSession(context.Context, *repository.AIChatSession) error
	GetSessionByID(context.Context, string, string, access.EntityFilter) (*repository.AIChatSession, error)
	CreateRun(context.Context, *repository.AIChatRun) error
	LinkRunTriggerMessage(context.Context, string, string) error
	UpdateRunStatus(context.Context, string, string, bool, *string, *string, *time.Time, *time.Time) error
	GetRunByID(context.Context, string, string) (*repository.AIChatRun, error)
	CreateMessage(context.Context, *repository.AIChatMessage) error
	GetNextMessageSequence(context.Context, string) (int, error)
	ListMessagesBySession(context.Context, string, int) ([]*repository.AIChatMessage, error)
	GetMessageByID(context.Context, string, string) (*repository.AIChatMessage, error)
	AppendRunEvent(context.Context, *repository.AIChatRunEvent) error
	ListRunEvents(context.Context, string, int, int) ([]*repository.AIChatRunEvent, error)
	GetNextRunEventSequence(context.Context, string) (int, error)
	CreateArtifact(context.Context, *repository.AIChatArtifact) error
	GetArtifactByID(context.Context, string, string) (*repository.AIChatArtifact, error)
	UpdateArtifactStatus(context.Context, string, string) error
	RecordReviewAction(context.Context, *repository.AIChatReviewAction) error
	GetReviewActionByID(context.Context, string, string) (*repository.AIChatReviewAction, error)
	CreateAttachment(context.Context, *repository.AIChatAttachment) error
}

// atomicReviewCommitter is an optional production seam. PostgreSQL adapters
// use it to commit the review action and Artifact status transition together;
// lightweight test stores may keep the two primitive methods.
type atomicReviewCommitter interface {
	CommitReviewAction(context.Context, *repository.AIChatReviewAction, string) error
}

// ReviewCommitFunc is an optional application-level transaction seam. The
// HTTP handler can use it to include business Draft writes in the same
// transaction as the runtime Artifact, Review Action and Run Event records.
type ReviewCommitFunc func(context.Context, *repository.AIChatArtifact, *repository.AIChatReviewAction, string, ReviewCommand) (any, error)

type Options struct {
	Dispatch     func(func())
	Timeout      time.Duration
	Now          func() time.Time
	ReviewCommit ReviewCommitFunc
}

func marshalJSON(value any) json.RawMessage {
	if value == nil {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return data
}
