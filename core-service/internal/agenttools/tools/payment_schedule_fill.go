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

// PaymentScheduleFillArguments is the strict input of the agent-side payment
// schedule prefill. The file arrives by reference through the shared reader
// seam; the contract binding, currency and timing are human-provided.
type PaymentScheduleFillArguments struct {
	FileID        string `json:"file_id"`
	ObjectName    string `json:"object_name"`
	ContentType   string `json:"content_type"`
	ContractID    string `json:"contract_id"`
	Currency      string `json:"currency"`
	PaymentTiming string `json:"payment_timing"`
}

// scheduleFillRow is one parsed schedule row. Rows are machine-extracted, so
// they only ever travel in the Fill's suggestions region; the human confirms
// them through the workspace form before any commit.
type scheduleFillRow struct {
	DueDate       string  `json:"due_date"`
	Amount        float64 `json:"amount"`
	PeriodStart   string  `json:"period_start,omitempty"`
	PeriodEnd     string  `json:"period_end,omitempty"`
	PaymentTiming string  `json:"payment_timing,omitempty"`
}

// NewPaymentScheduleFillDefinition registers the agent-side fill tool for the
// payment schedule intake (agent-universal-pagefill-v1 P0-B①). LevelDraft,
// payment_schedule-scoped; the page_fill payload carries only the
// human-provided envelope (contract binding / currency / timing), the parsed
// rows ride the suggestions region as Exploratory until the human confirms
// them row by row in the contract workspace — the commit is always the
// existing POST /contracts/:id/payment-schedules fired by the person.
func NewPaymentScheduleFillDefinition(reader IngestFileReader) agenttools.ToolDefinition {
	return agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name:        "lease.payment_schedule.fill.draft",
			Version:     "v1",
			DisplayName: "付款计划预填",
			Description: "识别租金表文件并预填合同工作台的付款计划表单：合同绑定/币种/收付时点由人提供，行级数值只以建议形式呈现供逐行核对。Agent 无 commit 权限——新增付款计划永远由人在页面上提交。",
			Level:       agenttools.LevelDraft,
			ReadOnly:    false,
			Permissions: []agenttools.Permission{{Resource: "payment_schedules", Action: "create"}},
			InputSchema: json.RawMessage(`{
				"type": "object",
				"required": ["file_id", "object_name", "content_type", "contract_id"],
				"properties": {
					"file_id": {"type": "string"},
					"object_name": {"type": "string"},
					"content_type": {"type": "string"},
					"contract_id": {"type": "string"},
					"currency": {"type": "string", "pattern": "^[A-Za-z]{3}$"},
					"payment_timing": {"type": "string", "enum": ["prepaid", "postpaid"]}
				}
			}`),
			OutputSchema:        json.RawMessage(`{"type": "object", "required": ["page_fill", "side_effects"]}`),
			Review:              agenttools.ReviewPolicy{Required: true, Reasons: []string{"mapping_unconfirmed", "schedule_review"}, ConfirmAction: "confirm"},
			SupportsDryRun:      true,
			SupportsIdempotency: true,
			MaxRows:             5000,
			TimeoutSeconds:      60,
		},
		SkillIDs: []string{"payment_schedule", "payment_schedule_intake"},
		Handler: func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
			execution, err := agenttools.RequireExecutionContext(ctx)
			if err != nil {
				return agenttools.ToolResult{}, err
			}
			var args PaymentScheduleFillArguments
			dec := json.NewDecoder(strings.NewReader(string(call.Arguments)))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&args); err != nil {
				return agenttools.ToolResult{}, errors.New("invalid payment schedule fill arguments")
			}
			if strings.TrimSpace(args.ContractID) == "" {
				return agenttools.ToolResult{}, errors.New("contract_unbound: a payment schedule prefill must target one contract; bind a contract_id first")
			}
			if reader == nil {
				return agenttools.ToolResult{}, errors.New("ingest file reader is not wired; no payment schedule preview can be produced")
			}

			raw, err := reader.ReadObject(ctx, args.ObjectName)
			if err != nil {
				return agenttools.ToolResult{}, fmt.Errorf("read uploaded file: %w", err)
			}
			headers, rows, err := controlledintake.Parse(controlledintake.Source{Filename: args.ObjectName, Data: raw})
			if err != nil {
				return agenttools.ToolResult{}, fmt.Errorf("parse payment schedule file: %w", err)
			}
			index := scheduleColumnIndex(headers)
			if index["due_date"] < 0 || index["amount"] < 0 {
				return agenttools.ToolResult{}, fmt.Errorf("template requires due_date, amount columns; got %v", headers)
			}

			parsed, skipped := parseScheduleRows(rows, index)
			if len(parsed) == 0 {
				return agenttools.ToolResult{}, errors.New("no valid payment schedule rows found (all rows skipped); refusing to fabricate a prefill")
			}

			fill := pagefill.New(
				"contract-workspace",
				"POST /api/v1/contracts/:id/payment-schedules",
				"/contracts/"+args.ContractID+"?schedule_fill="+call.CallID,
			)
			now := time.Now().UTC().Format(time.RFC3339)
			putConfirmed := func(key string, value string) error {
				return fill.PutPayload(key, value, workingpaper.Provenance{
					Basis: workingpaper.BasisHumanInput, ConfirmedBy: execution.Principal.UserID, ConfirmedAt: now,
				})
			}
			for _, pair := range []struct{ key, value string }{
				{"contract_id", args.ContractID},
				{"currency", strings.ToUpper(args.Currency)},
				{"payment_timing", args.PaymentTiming},
			} {
				if strings.TrimSpace(pair.value) == "" {
					continue
				}
				if err := putConfirmed(pair.key, pair.value); err != nil {
					return agenttools.ToolResult{}, err
				}
			}
			// 行级数值与结构概览都是机器判断，未确认前只能落在 suggestions 区。
			fill.Suggest("first_row", parsed[0], workingpaper.Provenance{Basis: workingpaper.BasisExploratory, EngineVersion: "controlled-intake-parse-v1"})
			fill.Suggest("schedule_summary", scheduleSummary(parsed, skipped), workingpaper.Provenance{Basis: workingpaper.BasisExploratory, EngineVersion: "controlled-intake-parse-v1"})
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

// scheduleColumnIndex maps the canonical column names to their positions;
// absent optional columns stay at -1.
func scheduleColumnIndex(headers []string) map[string]int {
	index := map[string]int{"due_date": -1, "amount": -1, "period_start": -1, "period_end": -1, "payment_timing": -1}
	for i, h := range headers {
		name := strings.ToLower(strings.TrimSpace(h))
		if _, known := index[name]; known && index[name] < 0 {
			index[name] = i
		}
	}
	return index
}

// parseScheduleRows applies the same row rules as the aiintake validation:
// positive numeric amounts, YYYY-MM-DD dates, prepaid/postpaid timing —
// invalid rows are skipped with a reason instead of being coerced.
func parseScheduleRows(rows [][]string, index map[string]int) ([]scheduleFillRow, []string) {
	parsed := make([]scheduleFillRow, 0, len(rows))
	skipped := make([]string, 0)
	for i, row := range rows {
		lineNo := i + 1
		dueDate := cellAt(row, index["due_date"])
		amountRaw := cellAt(row, index["amount"])
		if dueDate == "" || amountRaw == "" {
			skipped = append(skipped, fmt.Sprintf("第 %d 行缺少必要字段 (due_date/amount)，已跳过", lineNo))
			continue
		}
		amount, err := strconv.ParseFloat(strings.ReplaceAll(amountRaw, ",", ""), 64)
		if err != nil {
			skipped = append(skipped, fmt.Sprintf("第 %d 行金额无法解析为数字: %s", lineNo, amountRaw))
			continue
		}
		if amount <= 0 {
			skipped = append(skipped, fmt.Sprintf("第 %d 行金额 <= 0，已跳过", lineNo))
			continue
		}
		dates := map[string]string{"due_date": dueDate}
		if index["period_start"] >= 0 {
			dates["period_start"] = cellAt(row, index["period_start"])
		}
		if index["period_end"] >= 0 {
			dates["period_end"] = cellAt(row, index["period_end"])
		}
		rowValid := true
		for field, value := range dates {
			if value == "" {
				continue
			}
			if _, err := time.Parse("2006-01-02", value); err != nil {
				skipped = append(skipped, fmt.Sprintf("第 %d 行 %s 日期格式不正确: %s", lineNo, field, value))
				rowValid = false
			}
		}
		if !rowValid {
			continue
		}
		timing := strings.ToLower(strings.TrimSpace(cellAt(row, index["payment_timing"])))
		if timing != "" && timing != "prepaid" && timing != "postpaid" {
			skipped = append(skipped, fmt.Sprintf("第 %d 行收付时点无法识别: %s", lineNo, timing))
			continue
		}
		parsed = append(parsed, scheduleFillRow{
			DueDate: dueDate, Amount: amount,
			PeriodStart: dates["period_start"], PeriodEnd: dates["period_end"],
			PaymentTiming: timing,
		})
	}
	return parsed, skipped
}

func cellAt(row []string, index int) string {
	if index < 0 || index >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[index])
}

// scheduleSummary aggregates the parsed rows for the reviewer's spot check;
// skip reasons are capped so a hostile file cannot blow up the artifact.
func scheduleSummary(parsed []scheduleFillRow, skipped []string) map[string]any {
	minDue, maxDue, total := parsed[0].DueDate, parsed[0].DueDate, 0.0
	for _, row := range parsed {
		if row.DueDate < minDue {
			minDue = row.DueDate
		}
		if row.DueDate > maxDue {
			maxDue = row.DueDate
		}
		total += row.Amount
	}
	reasons := skipped
	if len(reasons) > 5 {
		reasons = reasons[:5]
	}
	sample := parsed
	if len(sample) > 5 {
		sample = sample[:5]
	}
	return map[string]any{
		"valid_rows":   len(parsed),
		"skipped_rows": len(skipped),
		"skip_reasons": reasons,
		"min_due_date": minDue,
		"max_due_date": maxDue,
		"total_amount": total,
		"sample":       sample,
	}
}
