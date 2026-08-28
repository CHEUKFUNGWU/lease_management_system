package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/pagefill"
	"github.com/lease-management-system/core-service/internal/services/controlledintake"
	"github.com/lease-management-system/core-service/internal/workingpaper"
)

// PlanLinesFillArguments is the strict input of the agent-side budget/plan
// prefill. The envelope fields (version identity, coverage, currency) are
// human-provided and land in the payload; the file's rows stay suggestions.
type PlanLinesFillArguments struct {
	FileID      string `json:"file_id"`
	ObjectName  string `json:"object_name"`
	ContentType string `json:"content_type"`
	Name        string `json:"name"`
	VersionType string `json:"version_type"`
	Source      string `json:"source"`
	AsOfPeriod  string `json:"as_of_period"`
	FromPeriod  string `json:"from_period"`
	ToPeriod    string `json:"to_period"`
	Currency    string `json:"currency"`
	IsOfficial  bool   `json:"is_official"`
}

// planLineColumns are the numeric columns the import handler accepts; all
// optional, all non-negative.
var planLineColumns = []string{
	"revenue", "gross_profit", "labor_cost",
	"fixed_rent", "variable_rent", "non_lease_cost", "four_wall_ebitda",
}

// NewPlanLinesFillDefinition registers the agent-side fill tool for the
// budget / plan-version import block (agent-universal-pagefill-v1 P0-B①).
// LevelDraft, fpna_copilot-scoped; the page_fill payload carries only the
// human-provided envelope, the parsed plan lines ride the suggestions region
// as Exploratory until the human confirms the import — the commit is always
// the existing POST /fpna/plan-versions/import fired by the person from the
// page with the real file.
func NewPlanLinesFillDefinition(reader IngestFileReader) agenttools.ToolDefinition {
	return agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name:        "fpna.plan_lines.fill.draft",
			Version:     "v1",
			DisplayName: "预算计划行预填",
			Description: "识别预算/计划版本文件并预填导入表单：版本名/类型/覆盖期间由人提供，行级数值（分店、期间、收入与成本行）只以建议形式呈现供核对。Agent 无 commit 权限——导入永远由人在页面上确认。",
			Level:       agenttools.LevelDraft,
			ReadOnly:    false,
			Permissions: []agenttools.Permission{{Resource: "master_data", Action: "manage"}},
			InputSchema: json.RawMessage(`{
				"type": "object",
				"required": ["file_id", "object_name", "content_type", "name", "version_type", "as_of_period", "from_period", "to_period"],
				"properties": {
					"file_id": {"type": "string"},
					"object_name": {"type": "string"},
					"content_type": {"type": "string"},
					"name": {"type": "string"},
					"version_type": {"type": "string", "enum": ["budget", "forecast", "scenario"]},
					"source": {"type": "string"},
					"as_of_period": {"type": "string", "pattern": "^[0-9]{4}-(0[1-9]|1[0-2])$"},
					"from_period": {"type": "string", "pattern": "^[0-9]{4}-(0[1-9]|1[0-2])$"},
					"to_period": {"type": "string", "pattern": "^[0-9]{4}-(0[1-9]|1[0-2])$"},
					"currency": {"type": "string", "pattern": "^[A-Za-z]{3}$"},
					"is_official": {"type": "boolean"}
				}
			}`),
			OutputSchema:        json.RawMessage(`{"type": "object", "required": ["page_fill", "side_effects"]}`),
			Review:              agenttools.ReviewPolicy{Required: true, Reasons: []string{"mapping_unconfirmed", "plan_import_review"}, ConfirmAction: "confirm"},
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
			var args PlanLinesFillArguments
			dec := json.NewDecoder(strings.NewReader(string(call.Arguments)))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&args); err != nil {
				return agenttools.ToolResult{}, errors.New("invalid plan lines fill arguments")
			}
			if args.VersionType != "budget" && args.VersionType != "forecast" && args.VersionType != "scenario" {
				return agenttools.ToolResult{}, errors.New("version_type must be one of budget|forecast|scenario")
			}
			if args.ToPeriod < args.FromPeriod {
				return agenttools.ToolResult{}, errors.New("period_range_invalid: from_period must be <= to_period")
			}
			if reader == nil {
				return agenttools.ToolResult{}, errors.New("ingest file reader is not wired; no plan lines preview can be produced")
			}

			raw, err := reader.ReadObject(ctx, args.ObjectName)
			if err != nil {
				return agenttools.ToolResult{}, fmt.Errorf("read uploaded file: %w", err)
			}
			headers, rows, err := controlledintake.Parse(controlledintake.Source{Filename: args.ObjectName, Data: raw})
			if err != nil {
				return agenttools.ToolResult{}, fmt.Errorf("parse plan lines file: %w", err)
			}
			index := planColumnIndex(headers)
			if index["store_code"] < 0 || index["period"] < 0 {
				return agenttools.ToolResult{}, fmt.Errorf("template requires store_code, period columns; got %v", headers)
			}

			parsed, skipped := parsePlanRows(rows, index, args.FromPeriod, args.ToPeriod)
			if len(parsed) == 0 {
				return agenttools.ToolResult{}, errors.New("no valid plan lines found (all rows skipped); refusing to fabricate a prefill")
			}

			fill := pagefill.New(
				"retail-data-import",
				"POST /api/v1/fpna/plan-versions/import",
				"/retail-data-import?plan_fill="+call.CallID+"&section=plan",
			)
			now := time.Now().UTC().Format(time.RFC3339)
			putConfirmed := func(key string, value string) error {
				return fill.PutPayload(key, value, workingpaper.Provenance{
					Basis: workingpaper.BasisHumanInput, ConfirmedBy: execution.Principal.UserID, ConfirmedAt: now,
				})
			}
			for _, pair := range []struct{ key, value string }{
				{"name", args.Name},
				{"version_type", args.VersionType},
				{"source", args.Source},
				{"as_of_period", args.AsOfPeriod},
				{"from_period", args.FromPeriod},
				{"to_period", args.ToPeriod},
				{"currency", strings.ToUpper(args.Currency)},
			} {
				if strings.TrimSpace(pair.value) == "" {
					continue
				}
				if err := putConfirmed(pair.key, pair.value); err != nil {
					return agenttools.ToolResult{}, err
				}
			}
			// is_official 是显式人输入：false 是表单默认而不是确认结果，
			// 只有人明确要求正式时才进 payload。
			if args.IsOfficial {
				if err := putConfirmed("is_official", "true"); err != nil {
					return agenttools.ToolResult{}, err
				}
			}
			// 行级数值与结构概览都是机器判断，未确认前只能落在 suggestions 区。
			fill.Suggest("first_row", parsed[0], workingpaper.Provenance{Basis: workingpaper.BasisExploratory, EngineVersion: "controlled-intake-parse-v1"})
			fill.Suggest("plan_summary", planSummary(parsed, skipped), workingpaper.Provenance{Basis: workingpaper.BasisExploratory, EngineVersion: "controlled-intake-parse-v1"})
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

// planColumnIndex maps the canonical column names to positions; absent
// optional columns stay at -1.
func planColumnIndex(headers []string) map[string]int {
	index := map[string]int{"store_code": -1, "period": -1, "currency": -1,
		"revenue": -1, "gross_profit": -1, "labor_cost": -1, "fixed_rent": -1,
		"variable_rent": -1, "non_lease_cost": -1, "four_wall_ebitda": -1}
	for i, h := range headers {
		name := strings.ToLower(strings.TrimSpace(h))
		if _, known := index[name]; known && index[name] < 0 {
			index[name] = i
		}
	}
	return index
}

// parsePlanRows applies the same row rules as the import handler's
// planImportLine: YYYY-MM periods inside the version range, non-negative
// numbers, optional row currency. Rows failing shape checks are skipped with
// a reason; store-master matching stays with the page's real import (the
// tool has no store-master port and must not guess).
func parsePlanRows(rows [][]string, index map[string]int, fromPeriod, toPeriod string) ([]map[string]any, []string) {
	parsed := make([]map[string]any, 0, len(rows))
	skipped := make([]string, 0)
	for i, row := range rows {
		lineNo := i + 1
		storeCode := cellAt(row, index["store_code"])
		period := cellAt(row, index["period"])
		if storeCode == "" || period == "" {
			skipped = append(skipped, fmt.Sprintf("第 %d 行缺少必要字段 (store_code/period)，已跳过", lineNo))
			continue
		}
		if !planPeriodShape(period) {
			skipped = append(skipped, fmt.Sprintf("第 %d 行 period 必须是 YYYY-MM: %s", lineNo, period))
			continue
		}
		if period < fromPeriod || period > toPeriod {
			skipped = append(skipped, fmt.Sprintf("第 %d 行期间 %s 不在版本范围内 (%s ~ %s)", lineNo, period, fromPeriod, toPeriod))
			continue
		}
		if currency := cellAt(row, index["currency"]); currency != "" && !planCurrencyShape(currency) {
			skipped = append(skipped, fmt.Sprintf("第 %d 行币种必须是三位字母: %s", lineNo, currency))
			continue
		}
		line := map[string]any{"store_code": strings.ToLower(storeCode), "period": period}
		badNumber := false
		for _, column := range planLineColumns {
			raw := cellAt(row, index[column])
			if raw == "" {
				continue
			}
			value, err := strconv.ParseFloat(strings.ReplaceAll(raw, ",", ""), 64)
			if err != nil || value < 0 {
				skipped = append(skipped, fmt.Sprintf("第 %d 行 %s 必须是非负数字: %s", lineNo, column, raw))
				badNumber = true
				break
			}
			line[column] = value
		}
		if badNumber {
			continue
		}
		parsed = append(parsed, line)
	}
	return parsed, skipped
}

func planPeriodShape(value string) bool {
	if len(value) != 7 || value[4] != '-' {
		return false
	}
	_, err := time.Parse("2006-01", value)
	return err == nil
}

func planCurrencyShape(value string) bool {
	if len(value) != 3 {
		return false
	}
	for _, r := range value {
		if !(r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z') {
			return false
		}
	}
	return true
}

// planSummary aggregates the parsed lines for the reviewer's spot check;
// skip reasons are capped so a hostile file cannot blow up the artifact.
func planSummary(parsed []map[string]any, skipped []string) map[string]any {
	minPeriod, maxPeriod := parsed[0]["period"].(string), parsed[0]["period"].(string)
	totalRevenue := 0.0
	stores := map[string]bool{}
	periods := map[string]bool{}
	for _, line := range parsed {
		period := line["period"].(string)
		if period < minPeriod {
			minPeriod = period
		}
		if period > maxPeriod {
			maxPeriod = period
		}
		periods[period] = true
		stores[line["store_code"].(string)] = true
		if revenue, ok := line["revenue"].(float64); ok {
			totalRevenue += revenue
		}
	}
	reasons := skipped
	if len(reasons) > 5 {
		reasons = reasons[:5]
	}
	return map[string]any{
		"valid_rows":   len(parsed),
		"skipped_rows": len(skipped),
		"skip_reasons": reasons,
		"min_period":   minPeriod,
		"max_period":   maxPeriod,
		"period_count": len(periods),
		"store_count":  len(stores),
		"total_revenue": totalRevenue,
	}
}
