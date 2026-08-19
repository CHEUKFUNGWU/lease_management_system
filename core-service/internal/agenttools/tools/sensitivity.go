package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/services/reporting"
)

// SensitivityArguments is the strict input schema of the sensitivity tool.
type SensitivityArguments struct {
	ContractID string    `json:"contract_id"`
	BaseRate   *float64  `json:"base_rate,omitempty"`
	Shocks     []float64 `json:"shocks"`
}

// SensitivityReader is the seam over the deterministic reporting projection.
// The adapter builds the tenant-scoped snapshot exactly like the HTTP path
// (handlers.ReportHandler.SensitivityAnalysis); nothing here computes a
// number independently.
type SensitivityReader interface {
	Sensitivity(ctx context.Context, contractID string, baseRate *float64, shocks []float64) (reporting.ProjectionResult, error)
}

// NewReportingSensitivityReader adapts a snapshot builder into the tool seam.
func NewReportingSensitivityReader(builder *reporting.SnapshotBuilder) SensitivityReader {
	return reportingSensitivityReader{builder: builder}
}

type reportingSensitivityReader struct {
	builder *reporting.SnapshotBuilder
}

func (r reportingSensitivityReader) Sensitivity(ctx context.Context, contractID string, baseRate *float64, shocks []float64) (reporting.ProjectionResult, error) {
	legalEntityID := ""
	if scope, ok := access.ScopeFromContext(ctx); ok {
		legalEntityID = scope.LegalEntityID
	}
	snapshot, err := r.builder.Build(ctx, reporting.Request{Mode: reporting.Working, LegalEntityID: legalEntityID})
	if err != nil {
		return reporting.ProjectionResult{}, err
	}
	return reporting.Project(snapshot, reporting.ProjectionRequest{
		Kind:       reporting.KindSensitivity,
		ContractID: contractID,
		Rate:       baseRate,
		Shocks:     shocks,
	})
}

// NewSensitivityDefinition registers the sensitivity tool. It closes the gap
// between the /sensitivity page and the Agent: the same deterministic
// projection serves both (改造清单〔1〕).
func NewSensitivityDefinition(reader SensitivityReader) agenttools.ToolDefinition {
	return agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name:        "lease.report.sensitivity",
			Version:     "v1",
			DisplayName: "折现率敏感性分析",
			Description: "对指定合同的折现率冲击重算初始租赁负债与使用权资产，输出各冲击下金额与变动百分比。确定性引擎计算，Agent 不得自行重算。",
			Level:       agenttools.LevelRead,
			ReadOnly:    true,
			Permissions: []agenttools.Permission{{Resource: "reports", Action: "read"}},
			InputSchema: json.RawMessage(`{
				"type": "object",
				"required": ["contract_id", "shocks"],
				"properties": {
					"contract_id": {"type": "string"},
					"base_rate": {"type": "number", "exclusiveMinimum": 0},
					"shocks": {"type": "array", "minItems": 1, "items": {"type": "number"}}
				}
			}`),
			OutputSchema:        json.RawMessage(`{"type": "object", "required": ["basis", "sensitivity"]}`),
			SupportsDryRun:      true,
			MaxRows:             50,
			TimeoutSeconds:      30,
			SupportsIdempotency: true,
		},
		Handler: func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
			if reader == nil {
				return agenttools.ToolResult{}, errors.New("sensitivity reader is unavailable")
			}
			var args SensitivityArguments
			dec := json.NewDecoder(strings.NewReader(string(call.Arguments)))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&args); err != nil {
				return agenttools.ToolResult{}, errors.New("invalid sensitivity arguments")
			}
			if args.ContractID == "" {
				return agenttools.ToolResult{}, errors.New("contract_id is required")
			}
			if len(args.Shocks) == 0 {
				return agenttools.ToolResult{}, errors.New("shocks is required and must contain at least one rate change")
			}
			projection, err := reader.Sensitivity(ctx, args.ContractID, args.BaseRate, args.Shocks)
			if err != nil {
				return agenttools.ToolResult{}, fmt.Errorf("sensitivity projection failed: %w", err)
			}
			return agenttools.ToolResult{
				CallID: call.CallID,
				Status: agenttools.StatusCompleted,
				Data: map[string]any{
					"basis":        "Certified",
					"sensitivity":  projection.Payload,
					"side_effects": false,
				},
				Sources: []agenttools.ToolSource{{
					Type:    "engine",
					ID:      args.ContractID,
					Title:   "reporting sensitivity projection",
					Locator: "reporting:KindSensitivity",
				}},
			}, nil
		},
	}
}

// The deterministic projection is the only computation source: the tool
// re-uses the same snapshot + projection the /sensitivity page uses, so the
// page and the Agent can never disagree.
