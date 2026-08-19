package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/finmodel"
	"github.com/lease-management-system/core-service/internal/workingpaper"
	finpaper "github.com/lease-management-system/core-service/internal/workingpaper/finmodel"
)

// StatementModelReader reads persisted model state (definitions, runs, lines,
// tie-outs) for the read tool. Production adapter = finmodel repository
// binding; nil keeps the tool honest (unavailable, not empty).
type StatementModelReader interface {
	ReadRun(ctx context.Context, runID string) (json.RawMessage, error)
	ReadDefinition(ctx context.Context, definitionID string) (json.RawMessage, error)
}

// FinModelPorts assembles the engine inputs for a run/evaluate call; the
// production adapter binds the five readers to their real services. The
// evaluate tool and the formal run path share finmodel.Run — this factory is
// the only seam between them.
type FinModelPorts interface {
	Build(ctx context.Context, principal agenttools.Principal, request json.RawMessage) (finmodel.ModelDef, finmodel.ModelInputs, error)
}

// NewStatementModelReadDefinition registers fpna.statement_model.read.
func NewStatementModelReadDefinition(reader StatementModelReader) agenttools.ToolDefinition {
	return agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name: "fpna.statement_model.read", Version: "v1", DisplayName: "读取三表模型",
			Description: "读取三表模型的定义、版本、run 结果与勾稽状态。权限与法人范围过滤由适配器执行；scope_denied 原因原样透传。",
			Level:       agenttools.LevelRead, ReadOnly: true,
			Permissions:    []agenttools.Permission{{Resource: "reports", Action: "read"}},
			InputSchema:    json.RawMessage(`{"type":"object","required":["run_id"],"properties":{"run_id":{"type":"string"},"definition_id":{"type":"string"}}}`),
			SupportsDryRun: true, MaxRows: 2000, TimeoutSeconds: 20,
		},
		Handler: func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
			if reader == nil {
				return agenttools.ToolResult{}, errors.New("statement model reader unavailable")
			}
			var args struct {
				RunID        string `json:"run_id"`
				DefinitionID string `json:"definition_id"`
			}
			dec := json.NewDecoder(strings.NewReader(string(call.Arguments)))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&args); err != nil || args.RunID == "" {
				return agenttools.ToolResult{}, errors.New("run_id is required")
			}
			run, err := reader.ReadRun(ctx, args.RunID)
			if err != nil {
				return agenttools.ToolResult{}, err
			}
			data := map[string]any{"run": json.RawMessage(run)}
			if args.DefinitionID != "" {
				if def, defErr := reader.ReadDefinition(ctx, args.DefinitionID); defErr == nil {
					data["definition"] = json.RawMessage(def)
				}
			}
			return agenttools.ToolResult{CallID: call.CallID, Status: agenttools.StatusCompleted, Data: data}, nil
		},
	}
}

// NewStatementModelEvaluateDefinition registers the side-effect-free
// simulation: it runs finmodel.Run on the given assumption drafts and
// returns values — nothing is persisted, nothing is overwritten (the Retail
// Scenario "只评估不落库" semantics).
func NewStatementModelEvaluateDefinition(ports FinModelPorts) agenttools.ToolDefinition {
	return agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name: "fpna.statement_model.evaluate", Version: "v1", DisplayName: "三表模型试算",
			Description: "以当前 approved 假设叠加对话中的假设草稿做无副作用试算：调用与正式运行相同的确定性引擎，不落库、不覆盖任何版本。",
			Level:       agenttools.LevelRead, ReadOnly: true,
			Permissions:    []agenttools.Permission{{Resource: "reports", Action: "read"}},
			InputSchema:    json.RawMessage(`{"type":"object","required":["model"],"properties":{"model":{"type":"object"},"assumption_overrides":{"type":"object"}}}`),
			SupportsDryRun: true, MaxRows: 5000, TimeoutSeconds: 60,
		},
		Handler: func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
			if ports == nil {
				return agenttools.ToolResult{}, errors.New("finmodel port factory unavailable")
			}
			execution, err := agenttools.RequireExecutionContext(ctx)
			if err != nil {
				return agenttools.ToolResult{}, err
			}
			def, inputs, err := ports.Build(ctx, execution.Principal, call.Arguments)
			if err != nil {
				return agenttools.ToolResult{}, err
			}
			result, err := finmodel.Run(ctx, def, inputs)
			if err != nil {
				return agenttools.ToolResult{}, err
			}
			return agenttools.ToolResult{CallID: call.CallID, Status: agenttools.StatusCompleted, Data: map[string]any{
				"result": result, "side_effects": false,
			}}, nil
		},
	}
}

// NewFinModelPaperDefinition registers the review-gated paper generator:
// evaluate → SM6 builder → lint fail-closed → working_paper artifact data.
func NewFinModelPaperDefinition(ports FinModelPorts) agenttools.ToolDefinition {
	return agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name: "fpna.working_paper.finmodel.generate", Version: "v1", DisplayName: "三表模型底稿生成",
			Description: "运行三表模型并把结果生成全 Certified 底稿（勾稽校验作为 check 区块；失败的 run 在封面与区块双重标红且不产出 artifact）。",
			Level:       agenttools.LevelDraft, ReadOnly: false,
			Permissions:    []agenttools.Permission{{Resource: "reports", Action: "read"}},
			InputSchema:    json.RawMessage(`{"type":"object","required":["model","title"],"properties":{"model":{"type":"object"},"title":{"type":"string"}}}`),
			OutputSchema:   json.RawMessage(`{"type":"object","required":["paper","side_effects"]}`),
			Review:         agenttools.ReviewPolicy{Required: true, Reasons: []string{"finmodel_paper_review", "assumptions_human_confirmed"}, ConfirmAction: "confirm"},
			SupportsDryRun: true, SupportsIdempotency: true, MaxRows: 5000, TimeoutSeconds: 120,
		},
		Handler: func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
			if ports == nil {
				return agenttools.ToolResult{}, errors.New("finmodel port factory unavailable")
			}
			execution, err := agenttools.RequireExecutionContext(ctx)
			if err != nil {
				return agenttools.ToolResult{}, err
			}
			var args struct {
				Model json.RawMessage `json:"model"`
				Title string          `json:"title"`
			}
			dec := json.NewDecoder(strings.NewReader(string(call.Arguments)))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&args); err != nil || args.Title == "" {
				return agenttools.ToolResult{}, errors.New("title is required")
			}
			def, inputs, err := ports.Build(ctx, execution.Principal, call.Arguments)
			if err != nil {
				return agenttools.ToolResult{}, err
			}
			result, err := finmodel.Run(ctx, def, inputs)
			if err != nil {
				return agenttools.ToolResult{}, err
			}
			paper, err := finpaper.Build(finpaper.Input{
				Title: args.Title, LegalEntityID: def.LegalEntityID, Currency: def.Currency,
				DataClassification:     inputs.DataClassification,
				ModelDefinitionVersion: inputs.Versions.ModelDefinition,
				TemplateVersion:        "v1", DataVersion: inputs.Versions.Data,
				AssumptionVersion: inputs.Versions.Assumption, ExchangeRateVersion: inputs.Versions.ExchangeRate,
				MetricDefinitionVersion: inputs.Versions.MetricDefinition,
				Periods:                 result.Periods,
				Lines:                   mapEngineLines(result),
				TieOuts:                 mapEngineTieOuts(result),
				GapDetails:              gapStrings(result),
				ToolCallID:              call.CallID,
				GeneratedBy:             execution.Principal.UserID,
			})
			if err != nil {
				return agenttools.ToolResult{}, err
			}
			built := workingpaper.Build(paper, time.Now())
			report := workingpaper.Lint(built, ownCallAudit{callID: call.CallID, status: agenttools.StatusCompleted})
			if !report.OK {
				details, _ := json.Marshal(report.Violations)
				return agenttools.ToolResult{}, errors.New("finmodel paper failed lint: " + string(details))
			}
			return agenttools.ToolResult{
				CallID: call.CallID, Status: agenttools.StatusCompleted,
				Data:   map[string]any{"paper": built, "side_effects": false},
				Review: agenttools.ReviewResult{Required: true, Reasons: []string{"finmodel_paper_review"}},
			}, nil
		},
	}
}

func mapEngineLines(result *finmodel.RunResult) []finpaper.LineValue {
	out := make([]finpaper.LineValue, 0, len(result.Lines))
	for _, line := range result.Lines {
		out = append(out, finpaper.LineValue{
			RowKey: line.RowKey, Label: line.RowKey, Period: line.Period,
			Value: line.Value, SourceType: line.Provenance.SourceType,
			Classification: line.Provenance.DataClassification,
		})
	}
	return out
}

func mapEngineTieOuts(result *finmodel.RunResult) []finpaper.TieOutValue {
	out := make([]finpaper.TieOutValue, 0, len(result.TieOuts))
	for _, t := range result.TieOuts {
		out = append(out, finpaper.TieOutValue{
			CheckCode: t.CheckCode, Period: t.Period,
			Expected: t.Expected, Actual: t.Actual, Diff: t.Diff, Status: t.Status,
		})
	}
	return out
}

func gapStrings(result *finmodel.RunResult) []string {
	out := make([]string, 0, len(result.Gaps))
	for _, g := range result.Gaps {
		out = append(out, g.Detail)
	}
	return out
}
