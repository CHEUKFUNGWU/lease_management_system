package agenttools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ToolLevel is the risk class of a registered business capability.
type ToolLevel string

const (
	LevelRead    ToolLevel = "read"
	LevelDraft   ToolLevel = "draft"
	LevelCommand ToolLevel = "command"
)

type ToolStatus string

const (
	StatusCompleted   ToolStatus = "completed"
	StatusNeedsReview ToolStatus = "needs_review"
	StatusFailed      ToolStatus = "failed"
	StatusRejected    ToolStatus = "rejected"
)

// Permission is the server-side RBAC permission required by a Tool.
// Identity and permissions are deliberately absent from ToolCall: clients do
// not get to assert who they are or what they may do.
type Permission struct {
	Resource string `json:"resource"`
	Action   string `json:"action"`
}

type RetryPolicy struct {
	Retryable    bool `json:"retryable"`
	MaxAttempts  int  `json:"max_attempts,omitempty"`
	BackoffMilli int  `json:"backoff_milli,omitempty"`
}

type ReviewPolicy struct {
	Required      bool     `json:"required"`
	Reasons       []string `json:"reasons,omitempty"`
	ConfirmAction string   `json:"confirm_action,omitempty"`
	AllowedRoles  []string `json:"allowed_roles,omitempty"`
}

// ToolDescriptor is the public contract for a server-registered Tool. The
// implementation, repository and database details remain behind the seam.
type ToolDescriptor struct {
	Name                string          `json:"name"`
	Version             string          `json:"version"`
	DisplayName         string          `json:"display_name"`
	Description         string          `json:"description"`
	Level               ToolLevel       `json:"level"`
	ReadOnly            bool            `json:"read_only"`
	Permissions         []Permission    `json:"permissions"`
	InputSchema         json.RawMessage `json:"input_schema,omitempty"`
	OutputSchema        json.RawMessage `json:"output_schema,omitempty"`
	Review              ReviewPolicy    `json:"review"`
	Retry               RetryPolicy     `json:"retry"`
	SupportsDryRun      bool            `json:"supports_dry_run"`
	SupportsIdempotency bool            `json:"supports_idempotency"`
	MaxRows             int             `json:"max_rows,omitempty"`
	TimeoutSeconds      int             `json:"timeout_seconds,omitempty"`
}

func (d ToolDescriptor) Validate() error {
	if !validToolName(d.Name) || strings.TrimSpace(d.Version) == "" {
		return fmt.Errorf("%w: name and version are required", ErrInvalidToolDescriptor)
	}
	if strings.TrimSpace(d.Description) == "" {
		return fmt.Errorf("%w: description is required", ErrInvalidToolDescriptor)
	}
	switch d.Level {
	case LevelRead:
		if !d.ReadOnly {
			return fmt.Errorf("%w: read tool must be read-only", ErrInvalidToolDescriptor)
		}
	case LevelDraft, LevelCommand:
		if d.ReadOnly {
			return fmt.Errorf("%w: write-capable tool cannot be read-only", ErrInvalidToolDescriptor)
		}
		if !d.SupportsIdempotency {
			return fmt.Errorf("%w: write-capable tool must support idempotency", ErrInvalidToolDescriptor)
		}
	default:
		return fmt.Errorf("%w: unknown level %q", ErrInvalidToolDescriptor, d.Level)
	}
	if len(d.Permissions) == 0 {
		return fmt.Errorf("%w: at least one permission is required", ErrInvalidToolDescriptor)
	}
	for _, permission := range d.Permissions {
		if strings.TrimSpace(permission.Resource) == "" || strings.TrimSpace(permission.Action) == "" {
			return fmt.Errorf("%w: permission resource and action are required", ErrInvalidToolDescriptor)
		}
	}
	if d.Level != LevelRead && !d.Review.Required {
		return fmt.Errorf("%w: write-capable tool must declare a review policy", ErrInvalidToolDescriptor)
	}
	if d.MaxRows < 0 || d.TimeoutSeconds < 0 {
		return fmt.Errorf("%w: limits cannot be negative", ErrInvalidToolDescriptor)
	}
	if err := validateJSONSchema(d.InputSchema, "input_schema"); err != nil {
		return err
	}
	if err := validateJSONSchema(d.OutputSchema, "output_schema"); err != nil {
		return err
	}
	return nil
}

type ToolFilter struct {
	Names         []string    `json:"names,omitempty"`
	Levels        []ToolLevel `json:"levels,omitempty"`
	SkillID       string      `json:"skill_id,omitempty"`
	IncludeSchema bool        `json:"include_schema,omitempty"`
}

// ToolCall is the only execution request exposed to external Agent clients.
// The server resolves user, role, scope, capability and permissions from the
// request context; none of those values are accepted as arguments.
type ToolCall struct {
	CallID  string `json:"call_id"`
	RunID   string `json:"run_id"`
	TraceID string `json:"trace_id,omitempty"`
	// Skill metadata is a routing/audit correlation field. It is never an
	// authority by itself; the Gateway compares it with the capability claims
	// (when a capability is present) and the server-owned registry.
	SkillID        string          `json:"skill_id,omitempty"`
	SkillVersion   string          `json:"skill_version,omitempty"`
	ToolName       string          `json:"tool_name"`
	ToolVersion    string          `json:"tool_version"`
	Arguments      json.RawMessage `json:"arguments"`
	IdempotencyKey string          `json:"idempotency_key,omitempty"`
	DryRun         bool            `json:"dry_run"`
}

func (c ToolCall) Validate() error {
	if strings.TrimSpace(c.CallID) == "" || strings.TrimSpace(c.RunID) == "" {
		return fmt.Errorf("%w: call_id and run_id are required", ErrInvalidToolCall)
	}
	if !validToolName(c.ToolName) || strings.TrimSpace(c.ToolVersion) == "" {
		return fmt.Errorf("%w: tool_name and tool_version are required", ErrInvalidToolCall)
	}
	arguments := bytes.TrimSpace(c.Arguments)
	if len(arguments) == 0 {
		arguments = []byte("{}")
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(arguments, &object); err != nil || object == nil {
		return fmt.Errorf("%w: arguments must be a JSON object", ErrInvalidToolCall)
	}
	return nil
}

type ToolSource struct {
	Type           string `json:"type"`
	ID             string `json:"id"`
	Title          string `json:"title,omitempty"`
	Locator        string `json:"locator,omitempty"`
	URL            string `json:"url,omitempty"`
	Classification string `json:"classification,omitempty"`
	DatasetVersion string `json:"dataset_version,omitempty"`
	AsOf           string `json:"as_of,omitempty"`
	FormulaVersion string `json:"formula_version,omitempty"`
}

type ReviewResult struct {
	Required bool     `json:"required"`
	Reasons  []string `json:"reasons,omitempty"`
	Actions  []string `json:"actions,omitempty"`
}

type ToolResult struct {
	CallID            string       `json:"call_id"`
	Status            ToolStatus   `json:"status"`
	Data              any          `json:"data,omitempty"`
	Sources           []ToolSource `json:"sources,omitempty"`
	Review            ReviewResult `json:"review"`
	Error             *ToolError   `json:"error,omitempty"`
	RetryAfterSeconds int          `json:"retry_after_seconds,omitempty"`
	Truncated         bool         `json:"truncated,omitempty"`
	// Terminate asks the agent loop to stop early after this call completes
	// (the pi "terminate: true" capability). Additive; omitted when false.
	Terminate bool `json:"terminate,omitempty"`
}

// ToolRuntime is the highest external seam. Registry, policy, scope, audit,
// timeout and result shaping live behind this two-method interface.
type ToolRuntime interface {
	Describe(ctx context.Context, filter ToolFilter) ([]ToolDescriptor, error)
	Execute(ctx context.Context, call ToolCall) (ToolResult, error)
}

func validToolName(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_' {
			continue
		}
		return false
	}
	return true
}

func validateJSONSchema(schema json.RawMessage, field string) error {
	if len(bytes.TrimSpace(schema)) == 0 {
		return nil
	}
	var value map[string]any
	if err := json.Unmarshal(schema, &value); err != nil || value == nil {
		return fmt.Errorf("%w: %s must be a JSON object", ErrInvalidToolDescriptor, field)
	}
	return nil
}
