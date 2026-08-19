package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/services/retailpulse"
	"github.com/lease-management-system/core-service/internal/services/retailscenario"
	"github.com/lease-management-system/core-service/internal/services/retailstore360"
	"github.com/lease-management-system/core-service/internal/workingpaper"
	retailpaper "github.com/lease-management-system/core-service/internal/workingpaper/retail"
)

// RetailPaperArguments is the strict input of the retail working-paper tool.
// The scenario assumptions arrive human-confirmed; the tool stamps the
// authenticated caller when the payload omits confirmation.
type RetailPaperArguments struct {
	retailContextArguments
	StoreID       string                     `json:"store_id,omitempty"`
	HorizonMonths int                        `json:"horizon_months,omitempty"`
	Assumptions   retailscenario.Assumptions `json:"assumptions,omitempty"`
	ConfirmedBy   string                     `json:"confirmed_by,omitempty"`
	ConfirmedAt   string                     `json:"confirmed_at,omitempty"`
}

// NewRetailPaperDefinition registers the retail operating working-paper
// generator. The paper is engine output only: pulse / store360 / scenario
// numbers pass through verbatim, gaps are named, and nothing here computes a
// KPI (retail-kpi-v1 owns the semantics).
func NewRetailPaperDefinition(reader RetailOperationsReader) agenttools.ToolDefinition {
	return agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name:        "retail.working_paper.store.generate",
			Version:     "v1",
			DisplayName: "零售门店经营底稿生成",
			Description: "生成零售门店经营底稿：数据边界与质量、经营脉搏（retail-kpi-v1 语义）、同店销售、关注门店、门店 360 诊断与情景测算。全部数字来自确定性引擎，缺失与降级如实列出。",
			Level:       agenttools.LevelDraft,
			ReadOnly:    false,
			Permissions: []agenttools.Permission{{Resource: "reports", Action: "read"}},
			InputSchema: json.RawMessage(`{
				"type": "object",
				"required": ["as_of", "window_days", "data_classification"],
				"properties": {
					"as_of": {"type": "string", "pattern": "^[0-9]{4}-[0-9]{2}-[0-9]{2}$"},
					"window_days": {"type": "integer", "enum": [7, 14, 28]},
					"data_classification": {"type": "string", "enum": ["production", "simulated"]},
					"dataset_version": {"type": "string"},
					"source_system": {"type": "string"},
					"store_ids": {"type": "array", "items": {"type": "string"}},
					"store_id": {"type": "string"},
					"attention_limit": {"type": "integer"},
					"horizon_months": {"type": "integer", "enum": [3, 6, 12]},
					"assumptions": {"type": "object"},
					"confirmed_by": {"type": "string"},
					"confirmed_at": {"type": "string"}
				}
			}`),
			OutputSchema:        json.RawMessage(`{"type": "object", "required": ["paper", "side_effects"]}`),
			Review:              agenttools.ReviewPolicy{Required: true, Reasons: []string{"retail_paper_review", "assumptions_human_confirmed"}, ConfirmAction: "confirm"},
			SupportsDryRun:      true,
			SupportsIdempotency: true,
			MaxRows:             2000,
			TimeoutSeconds:      60,
		},
		Handler: func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
			if reader == nil {
				return agenttools.ToolResult{}, errors.New("retail operations reader is unavailable")
			}
			execution, err := agenttools.RequireExecutionContext(ctx)
			if err != nil {
				return agenttools.ToolResult{}, err
			}
			var args RetailPaperArguments
			dec := json.NewDecoder(strings.NewReader(string(call.Arguments)))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&args); err != nil {
				return agenttools.ToolResult{}, errors.New("invalid retail paper arguments")
			}
			q, err := retailQuery(args.retailContextArguments, execution.Principal.Scope.LegalEntityID)
			if err != nil {
				return agenttools.ToolResult{}, err
			}

			pulseStoreIDs := q.storeIDs
			if strings.TrimSpace(args.StoreID) != "" {
				id, parseErr := parseStoreID(args.StoreID)
				if parseErr != nil {
					return agenttools.ToolResult{}, parseErr
				}
				pulseStoreIDs = []string{id}
			}

			// Pulse — the base every paper carries.
			pulseService := retailpulse.NewService(scopedRetailReader{base: reader, scope: execution.Principal.Scope, requestedStoreIDs: pulseStoreIDs})
			pulse, err := pulseService.Build(ctx, retailpulse.Query{
				LegalEntityID: q.legalEntityID, AsOf: q.asOf, WindowDays: q.windowDays,
				Classification: q.classification, DatasetVersion: q.datasetVersion,
				SourceSystem: q.sourceSystem, StoreIDs: pulseStoreIDs,
				AttentionLimit: attentionCap(args.AttentionLimit),
			})
			if err != nil {
				return agenttools.ToolResult{}, fmt.Errorf("经营脉搏构建失败：%w", err)
			}

			var diag *retailstore360.Response
			if strings.TrimSpace(args.StoreID) != "" {
				diagService := retailstore360.NewService(scopedRetailReader{base: reader, scope: execution.Principal.Scope, requestedStoreIDs: []string{args.StoreID}})
				diag, err = diagService.Build(ctx, retailstore360.Query{
					LegalEntityID: q.legalEntityID, StoreID: args.StoreID, AsOf: q.asOf, WindowDays: q.windowDays,
					Classification: q.classification, DatasetVersion: q.datasetVersion, SourceSystem: q.sourceSystem,
				})
				if err != nil {
					return agenttools.ToolResult{}, fmt.Errorf("门店诊断构建失败：%w", err)
				}
			}

			// Scenario mirrors the chat plane's blocking semantics (decision
			// D-C3): runs only when the pulse is sufficient and diagnostics
			// are decision-ready. Skipping is a recorded gap, not a failure.
			var scenario *retailscenario.Response
			if pulseSufficient(pulse) && diag != nil && diag.DecisionReady && allPaperAssumptionsProvided(call.Arguments) {
				scenarioService := retailscenario.NewService(scopedRetailReader{base: reader, scope: execution.Principal.Scope, requestedStoreIDs: []string{args.StoreID}})
				scenario, err = scenarioService.Evaluate(ctx, retailscenario.Query{
					LegalEntityID: q.legalEntityID, StoreID: args.StoreID, AsOf: q.asOf, WindowDays: q.windowDays,
					Classification: q.classification, DatasetVersion: q.datasetVersion, SourceSystem: q.sourceSystem,
				}, retailscenario.EvaluateRequest{
					HorizonMonths: args.HorizonMonths,
					Scenarios: []retailscenario.ScenarioInput{
						{Key: "baseline", Name: "Baseline", Assumptions: retailscenario.Assumptions{}},
						{Key: "plan", Name: "Plan", Assumptions: args.Assumptions},
					},
				})
				if err != nil {
					return agenttools.ToolResult{}, fmt.Errorf("情景测算失败：%w", err)
				}
			}

			confirmedBy := args.ConfirmedBy
			confirmedAt := args.ConfirmedAt
			if confirmedBy == "" {
				confirmedBy = execution.Principal.UserID
			}
			if confirmedAt == "" {
				confirmedAt = time.Now().UTC().Format(time.RFC3339)
			}

			in := retailpaper.Input{
				Pulse: pulse, Diagnostics: diag, Scenario: scenario,
				Assumptions: args.Assumptions,
				ConfirmedBy: confirmedBy, ConfirmedAt: confirmedAt,
				ToolCallID:     call.CallID,
				AttentionLimit: args.AttentionLimit,
			}
			paper, err := retailpaper.Build(in)
			if err != nil {
				return agenttools.ToolResult{}, err
			}
			built := workingpaper.Build(paper, time.Now())
			report := workingpaper.Lint(built, ownCallAudit{callID: call.CallID, status: agenttools.StatusCompleted})
			if !report.OK {
				details, _ := json.Marshal(report.Violations)
				return agenttools.ToolResult{}, fmt.Errorf("retail paper failed lint: %s", details)
			}
			return agenttools.ToolResult{
				CallID: call.CallID,
				Status: agenttools.StatusCompleted,
				Data: map[string]any{
					"paper":        built,
					"side_effects": false,
				},
				Sources: []agenttools.ToolSource{{
					Type: "engine", ID: call.CallID,
					Title:   "retailpulse / retailstore360 / retailscenario 确定性引擎",
					Locator: "retail:" + built.Period,
				}},
				Review: agenttools.ReviewResult{Required: true, Reasons: []string{"retail_paper_review", "assumptions_human_confirmed"}},
			}, nil
		},
	}
}

// pulseSufficient mirrors the chat plane's gate: the pulse must be
// decision-ready with a populated summary and complete core KPIs.
func pulseSufficient(p *retailpulse.Response) bool {
	if p == nil || !p.DecisionReady || len(p.Summary) == 0 {
		return false
	}
	for _, code := range []string{"revenue", "gross_profit", "footfall"} {
		if m, ok := p.Summary[code]; !ok || m.Current.Status != "complete" || m.Current.Value == nil {
			return false
		}
	}
	return true
}

// allPaperAssumptionsProvided requires every one of the seven scenario
// assumptions to appear explicitly in the raw arguments — presence is
// tracked on the wire, not inferred from zero values.
func allPaperAssumptionsProvided(raw json.RawMessage) bool {
	var probe struct {
		Assumptions map[string]json.RawMessage `json:"assumptions"`
	}
	if json.Unmarshal(raw, &probe) != nil || probe.Assumptions == nil {
		return false
	}
	for _, key := range []string{
		"revenue_change_pct", "gross_margin_rate_change_pp", "labor_cost_change_pct",
		"fixed_rent_change_pct", "variable_rent_rate_change_pp", "non_lease_cost_change_pct",
		"other_controllable_cost_change_pct",
	} {
		if _, ok := probe.Assumptions[key]; !ok {
			return false
		}
	}
	return true
}

func attentionCap(limit int) int {
	if limit <= 0 {
		return 20
	}
	if limit > 50 {
		return 50
	}
	return limit
}
