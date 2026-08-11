package agenttools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

// ToolExecutionAudit is the minimum durable trace for one Tool attempt. Raw
// arguments are intentionally excluded; only a one-way fingerprint is stored
// so operators can correlate retries without leaking contract amounts or
// document content into an audit index.
type ToolExecutionAudit struct {
	CallID          string     `json:"call_id"`
	RunID           string     `json:"run_id"`
	TraceID         string     `json:"trace_id,omitempty"`
	UserID          string     `json:"user_id,omitempty"`
	SubjectType     string     `json:"subject_type,omitempty"`
	LegalEntityID   string     `json:"legal_entity_id,omitempty"`
	SkillID         string     `json:"skill_id,omitempty"`
	SkillVersion    string     `json:"skill_version,omitempty"`
	ToolName        string     `json:"tool_name"`
	ToolVersion     string     `json:"tool_version"`
	Status          ToolStatus `json:"status"`
	ErrorCode       ErrorCode  `json:"error_code,omitempty"`
	ArgumentsSHA256 string     `json:"arguments_sha256"`
	ReviewRequired  bool       `json:"review_required"`
	StartedAt       time.Time  `json:"started_at"`
	CompletedAt     time.Time  `json:"completed_at"`
	DurationMillis  int64      `json:"duration_millis"`
}

type AuditRecorder interface {
	RecordToolExecution(context.Context, ToolExecutionAudit) error
}

type AuditRecorderFunc func(context.Context, ToolExecutionAudit) error

func (f AuditRecorderFunc) RecordToolExecution(ctx context.Context, audit ToolExecutionAudit) error {
	return f(ctx, audit)
}

func argumentsFingerprint(arguments []byte) string {
	arguments = []byte(strings.TrimSpace(string(arguments)))
	if len(arguments) == 0 {
		arguments = []byte("{}")
	}
	digest := sha256.Sum256(arguments)
	return hex.EncodeToString(digest[:])
}
