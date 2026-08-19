package hooks

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/lease-management-system/core-service/internal/agentcore"
	"github.com/lease-management-system/core-service/internal/agenttools"
)

// CertifiedCall is one completed certified tool invocation, collected for
// working-paper provenance (invariant I2's production source, design §6
// ArtifactCollector).
type CertifiedCall struct {
	CallID        string
	ToolName      string
	ToolVersion   string
	ArgumentsHash string
}

// ArtifactSink receives completed certified calls.
type ArtifactSink interface {
	RecordCertified(ctx context.Context, call CertifiedCall) error
}

// buildAudit reconstructs the tool execution audit from the after-context,
// mirroring agenttools.Runtime.recordAudit so the two paths stay compatible.
func buildAudit(ac agentcore.AfterContext) agenttools.ToolExecutionAudit {
	audit := agenttools.ToolExecutionAudit{
		CallID:          ac.Call.CallID,
		RunID:           ac.Call.RunID,
		TraceID:         ac.Call.TraceID,
		UserID:          ac.Principal.UserID,
		SubjectType:     ac.Principal.SubjectType,
		LegalEntityID:   ac.Principal.Scope.LegalEntityID,
		ToolName:        ac.Call.ToolName,
		ToolVersion:     ac.Call.ToolVersion,
		ArgumentsSHA256: argumentsHash(ac.Call.Arguments),
		StartedAt:       ac.StartedAt,
		CompletedAt:     ac.CompletedAt,
	}
	if !ac.StartedAt.IsZero() && !ac.CompletedAt.IsZero() {
		audit.DurationMillis = ac.CompletedAt.Sub(ac.StartedAt).Milliseconds()
	}
	if ac.Result != nil {
		audit.Status = ac.Result.Status
		audit.ReviewRequired = ac.Result.Review.Required
		if ac.Result.Error != nil {
			audit.ErrorCode = ac.Result.Error.Code
		}
	} else {
		audit.Status = agenttools.StatusFailed
	}
	return audit
}

// AuditRecorder is the after-hook version of the runtime's audit write. It
// serves the future agentcore run path; the runtime keeps its own recorder
// (which also covers early-rejected calls) until W3 — the two must not be
// wired together on one path (decision D-B2).
func AuditRecorder(rec agenttools.AuditRecorder) agentcore.AfterToolCall {
	return func(ctx context.Context, ac agentcore.AfterContext) (agentcore.AfterResult, error) {
		if rec == nil {
			return agentcore.AfterResult{}, nil
		}
		return agentcore.AfterResult{}, rec.RecordToolExecution(context.WithoutCancel(ctx), buildAudit(ac))
	}
}

// ArtifactCollector feeds completed certified calls into the provenance sink.
func ArtifactCollector(sink ArtifactSink) agentcore.AfterToolCall {
	return func(ctx context.Context, ac agentcore.AfterContext) (agentcore.AfterResult, error) {
		if sink == nil || ac.Result == nil || ac.Result.Status != agenttools.StatusCompleted {
			return agentcore.AfterResult{}, nil
		}
		return agentcore.AfterResult{}, sink.RecordCertified(ctx, CertifiedCall{
			CallID:        ac.Call.CallID,
			ToolName:      ac.Call.ToolName,
			ToolVersion:   ac.Call.ToolVersion,
			ArgumentsHash: argumentsHash(ac.Call.Arguments),
		})
	}
}

// MetricsRecorder observes the process metrics sink.
func MetricsRecorder(m *agenttools.RuntimeMetrics) agentcore.AfterToolCall {
	return func(ctx context.Context, ac agentcore.AfterContext) (agentcore.AfterResult, error) {
		if m == nil {
			return agentcore.AfterResult{}, nil
		}
		m.Observe(buildAudit(ac))
		return agentcore.AfterResult{}, nil
	}
}

func argumentsHash(args []byte) string {
	sum := sha256.Sum256(args)
	return hex.EncodeToString(sum[:])
}
