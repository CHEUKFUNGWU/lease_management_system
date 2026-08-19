package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/workingpaper"
	s1paper "github.com/lease-management-system/core-service/internal/workingpaper/s1"
)

// S1GenerateArguments is the strict input of the S1 working-paper tool.
// Assumptions arrive human-confirmed; the handler stamps the caller as the
// confirmer when the payload omits it.
type S1GenerateArguments struct {
	Input s1paper.Input `json:"input"`
}

// NewS1GenerateDefinition registers the S1 pre-deal working-paper generator
// (阶段 1). It is LevelDraft with review required: the paper is a
// reviewable artifact, never a direct write.
func NewS1GenerateDefinition() agenttools.ToolDefinition {
	return agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name:        "lease.working_paper.s1.generate",
			Version:     "v1",
			DisplayName: "S1 签约前决策底稿生成",
			Description: "根据人工确认的报价假设生成 S1 签约前决策底稿：IFRS 16 影响、EBITDA 桥、退出曲线与折现率敏感性全部由确定性引擎计算，每个数字带 Certified provenance。",
			Level:       agenttools.LevelDraft,
			ReadOnly:    false,
			Permissions: []agenttools.Permission{{Resource: "reports", Action: "read"}},
			InputSchema: json.RawMessage(`{
				"type": "object",
				"required": ["input"],
				"properties": {"input": {"type": "object"}}
			}`),
			OutputSchema: json.RawMessage(`{"type": "object", "required": ["paper", "side_effects"]}`),
			Review: agenttools.ReviewPolicy{
				Required:      true,
				Reasons:       []string{"s1_paper_review", "assumptions_human_confirmed", "discount_rate_not_guessed"},
				ConfirmAction: "confirm",
			},
			SupportsDryRun:      true,
			SupportsIdempotency: true,
			MaxRows:             200,
			TimeoutSeconds:      60,
		},
		Handler: func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
			var args S1GenerateArguments
			dec := json.NewDecoder(strings.NewReader(string(call.Arguments)))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&args); err != nil {
				return agenttools.ToolResult{}, errors.New("invalid s1 generate arguments")
			}
			if args.Input.ConfirmedBy == "" || args.Input.ConfirmedAt == "" {
				execution, ok := agenttools.ExecutionContextFromContext(ctx)
				if !ok || execution.Principal.UserID == "" {
					return agenttools.ToolResult{}, errors.New("assumptions must be confirmed by a human (confirmed_by/confirmed_at are empty and no authenticated user context)")
				}
				if args.Input.ConfirmedBy == "" {
					args.Input.ConfirmedBy = execution.Principal.UserID
				}
				if args.Input.ConfirmedAt == "" {
					args.Input.ConfirmedAt = time.Now().UTC().Format(time.RFC3339)
				}
			}
			// I2 anchor: the paper's certified cells point at THIS audited
			// call — provenance must trace to a real, completed tool audit
			// record, never to a fabricated sub-call.
			args.Input.ToolCallID = call.CallID

			paper, err := s1paper.Build(args.Input)
			if err != nil {
				return agenttools.ToolResult{}, err
			}
			// Fail-closed before anything leaves the tool: the paper the tool
			// produces must pass the lint with its own call as audit evidence.
			built := workingpaper.Build(paper, time.Now())
			report := workingpaper.Lint(built, ownCallAudit{callID: call.CallID, status: agenttools.StatusCompleted})
			if !report.OK {
				details, _ := json.Marshal(report.Violations)
				return agenttools.ToolResult{}, fmt.Errorf("s1 paper failed lint: %s", details)
			}
			return agenttools.ToolResult{
				CallID: call.CallID,
				Status: agenttools.StatusCompleted,
				Data: map[string]any{
					"paper":        built,
					"side_effects": false,
				},
				Sources: []agenttools.ToolSource{{
					Type:    "engine",
					ID:      call.CallID,
					Title:   "predeal/dealcompare 确定性引擎",
					Locator: "s1:" + built.Title,
				}},
				Review: agenttools.ReviewResult{Required: true, Reasons: []string{"s1_paper_review", "assumptions_human_confirmed"}},
			}, nil
		},
	}
}

type ownCallAudit struct {
	callID string
	status agenttools.ToolStatus
}

func (a ownCallAudit) CompletedToolCall(callID string) bool {
	return callID == a.callID && a.status == agenttools.StatusCompleted
}
