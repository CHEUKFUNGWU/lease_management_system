package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/pagefill"
	"github.com/lease-management-system/core-service/internal/services/controlledintake"
	"github.com/lease-management-system/core-service/internal/workingpaper"
)

// TrialBalanceFillArguments is the strict input of the agent-side GL trial
// balance prefill. The file arrives by reference through the shared reader
// seam; envelope fields are human-provided, column mapping stays a suggestion.
type TrialBalanceFillArguments struct {
	FileID             string `json:"file_id"`
	ObjectName         string `json:"object_name"`
	ContentType        string `json:"content_type"`
	Name               string `json:"name"`
	SourceSystem       string `json:"source_system"`
	Period             string `json:"period"`
	FunctionalCurrency string `json:"functional_currency"`
}

// NewTrialBalanceFillDefinition registers the agent-side fill tool for the
// GL trial balance import block on /retail-data-import
// (agent-universal-pagefill-v1 P0-B①). LevelDraft, fpna_copilot-scoped; the
// page_fill payload carries only the human-provided envelope fields, the
// parsed row shape (headers + row count + account-code sample) rides the
// suggestions region as Exploratory until the human confirms the import —
// commit is always POSTed by the person from the page with the real file.
func NewTrialBalanceFillDefinition(reader IngestFileReader) agenttools.ToolDefinition {
	return agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name:        "fpna.trial_balance.fill.preview",
			Version:     "v1",
			DisplayName: "GL 试算平衡表预填",
			Description: "识别总账试算平衡表文件并预填导入表单：名称/来源系统/期间/功能货币由人提供，列结构与行数以建议形式呈现供核对。Agent 无 commit 权限——导入永远由人在页面上确认。",
			Level:       agenttools.LevelDraft,
			ReadOnly:    false,
			Permissions: []agenttools.Permission{{Resource: "master_data", Action: "manage"}},
			InputSchema: json.RawMessage(`{
				"type": "object",
				"required": ["file_id", "object_name", "content_type", "source_system", "period"],
				"properties": {
					"file_id": {"type": "string"},
					"object_name": {"type": "string"},
					"content_type": {"type": "string"},
					"name": {"type": "string"},
					"source_system": {"type": "string"},
					"period": {"type": "string", "pattern": "^[0-9]{4}-[0-9]{2}$"},
					"functional_currency": {"type": "string", "pattern": "^[A-Za-z]{3}$"}
				}
			}`),
			OutputSchema:        json.RawMessage(`{"type": "object", "required": ["page_fill", "side_effects"]}`),
			Review:              agenttools.ReviewPolicy{Required: true, Reasons: []string{"mapping_unconfirmed", "import_review"}, ConfirmAction: "confirm"},
			SupportsDryRun:      true,
			SupportsIdempotency: true,
			MaxRows:             5000,
			TimeoutSeconds:      60,
		},
		SkillIDs: []string{"fpna_copilot"},
		Handler: func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
			execution, err := agenttools.RequireExecutionContext(ctx)
			if err != nil {
				return agenttools.ToolResult{}, err
			}
			var args TrialBalanceFillArguments
			dec := json.NewDecoder(strings.NewReader(string(call.Arguments)))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&args); err != nil {
				return agenttools.ToolResult{}, errors.New("invalid trial balance fill arguments")
			}
			if strings.TrimSpace(args.SourceSystem) == "" || strings.TrimSpace(args.Period) == "" {
				return agenttools.ToolResult{}, errors.New("source_system and period are required")
			}
			if reader == nil {
				return agenttools.ToolResult{}, errors.New("ingest file reader is not wired; no trial balance preview can be produced")
			}

			raw, err := reader.ReadObject(ctx, args.ObjectName)
			if err != nil {
				return agenttools.ToolResult{}, fmt.Errorf("read uploaded file: %w", err)
			}
			headers, rows, err := controlledintake.Parse(controlledintake.Source{Filename: args.ObjectName, Data: raw})
			if err != nil {
				return agenttools.ToolResult{}, fmt.Errorf("parse trial balance file: %w", err)
			}
			lowered := make([]string, 0, len(headers))
			for _, h := range headers {
				lowered = append(lowered, strings.ToLower(strings.TrimSpace(h)))
			}
			for _, required := range []string{"account_code", "debit", "credit"} {
				found := false
				for _, h := range lowered {
					if h == required {
						found = true
					}
				}
				if !found {
					return agenttools.ToolResult{}, fmt.Errorf("template requires account_code, debit, credit columns; got %v", headers)
				}
			}

			fill := pagefill.New(
				"retail-data-import",
				"POST /gl/trial-balances/import",
				"/retail-data-import?tb_fill="+call.CallID+"&section=trial_balance",
			)
			now := time.Now().UTC().Format(time.RFC3339)
			putConfirmed := func(key string, value string) error {
				return fill.PutPayload(key, value, workingpaper.Provenance{
					Basis: workingpaper.BasisHumanInput, ConfirmedBy: execution.Principal.UserID, ConfirmedAt: now,
				})
			}
			for _, pair := range []struct{ key, value string }{
				{"source_system", args.SourceSystem},
				{"period", args.Period},
				{"name", args.Name},
				{"functional_currency", strings.ToUpper(args.FunctionalCurrency)},
			} {
				if strings.TrimSpace(pair.value) == "" {
					continue
				}
				if err := putConfirmed(pair.key, pair.value); err != nil {
					return agenttools.ToolResult{}, err
				}
			}
			// 结构概览是机器判断，未确认前只能落在 suggestions 区。
			fill.Suggest("column_structure", map[string]any{
				"headers":      headers,
				"row_count":    len(rows),
				"sample_codes": firstAccounts(rows, headers),
			}, workingpaper.Provenance{Basis: workingpaper.BasisExploratory, EngineVersion: "controlled-intake-parse-v1"})
			if err := fill.Validate(); err != nil {
				return agenttools.ToolResult{}, err
			}

			return agenttools.ToolResult{
				CallID: call.CallID,
				Status: agenttools.StatusCompleted,
				Data: map[string]any{
					"page_fill":    fill,
					"side_effects": false,
				},
				Review: agenttools.ReviewResult{Required: true, Reasons: []string{"mapping_unconfirmed"}},
			}, nil
		},
	}
}

// firstAccounts extracts up to five leading account codes for the reviewer's
// spot check.
func firstAccounts(rows [][]string, headers []string) []string {
	index := -1
	for i, h := range headers {
		if strings.ToLower(strings.TrimSpace(h)) == "account_code" {
			index = i
		}
	}
	out := []string{}
	if index < 0 {
		return out
	}
	for _, row := range rows {
		if index < len(row) && strings.TrimSpace(row[index]) != "" {
			out = append(out, strings.TrimSpace(row[index]))
		}
		if len(out) >= 5 {
			break
		}
	}
	return out
}
