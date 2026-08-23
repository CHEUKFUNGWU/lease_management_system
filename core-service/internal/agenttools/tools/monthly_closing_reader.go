package tools

// B-2 月结只读接入：跑批状态、期间级分录预览、有分录期间列表、锁账状态。
//
// 边界（AGENTS.md）：月结的写口是审批与锁账控制的核心——生成、审批、过账、
// 红冲、ERP 回写、锁账、解锁一律不包装成 Tool，AI 零写入。四个工具全部
// LevelRead，权限按工具自身读写性质取 monthly_closing:read。
//
// 法人与维度隔离：legal entity 一律从执行上下文 principal scope 取（缺失即
// 拒绝，绝不从参数信任）；维度范围由仓储层 appendContractScopePredicate 按
// ctx 中的 scope 收紧。entries.preview 的 contract_id 即便传入异法人合同 ID，
// 实体谓词与合同谓词同时生效，只会得到空集，不泄漏存在性。

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/repository"
)

// MonthlyClosingReader is the read-only seam over the monthly-closing ledger.
// *repository.MonthlyClosingRepository implements it directly; nil keeps the
// tools honest (unavailable), and the wiring never registers the nil version
// over a real port (P0-8).
type MonthlyClosingReader interface {
	GetBatches(ctx context.Context, period, legalEntityID string) ([]*repository.MonthlyClosingBatch, error)
	ListJournalEntries(ctx context.Context, q repository.JournalEntryQuery) ([]*repository.JournalEntry, repository.JournalEntrySummary, error)
	ListEntryPeriods(ctx context.Context, legalEntityID string, limit int) ([]repository.JournalEntryPeriod, error)
	IsPeriodLocked(ctx context.Context, period, legalEntityID string) (bool, error)
}

var mcPeriodPattern = regexp.MustCompile(`^\d{4}-\d{2}$`)

func mcValidPeriod(period string) bool { return mcPeriodPattern.MatchString(period) }

type mcBatchesArguments struct {
	Period string `json:"period,omitempty"`
}

type mcEntriesPreviewArguments struct {
	Period     string `json:"period"`
	ContractID string `json:"contract_id,omitempty"`
	Status     string `json:"status,omitempty"`
	Page       int    `json:"page,omitempty"`
	PageSize   int    `json:"page_size,omitempty"`
}

type mcPeriodsArguments struct {
	Limit int `json:"limit,omitempty"`
}

type mcLockStatusArguments struct {
	Period string `json:"period"`
}

// MonthlyClosingBatchesToolData 跑批状态。period 过滤给出时回显该期间的
// 锁账状态，让人一眼看出批次是否落在已锁期间（已锁期间不得被覆盖）。
type MonthlyClosingBatchesToolData struct {
	Batches      []*repository.MonthlyClosingBatch `json:"data"`
	Total        int                               `json:"total"`
	Period       string                            `json:"period,omitempty"`
	PeriodLocked *bool                             `json:"period_locked,omitempty"`
	SideEffects  bool                              `json:"side_effects"`
}

// MonthlyClosingEntriesPreviewToolData 分录预览信封。Draft 口径声明是
// 硬约束：预览含 draft/approved 条目，is_official_version 恒 false，
// approval_status 汇总的是过滤全集（不只是当前页），每条 item 自带
// posting_status —— 不可能被误读成 Official 报表。
type MonthlyClosingEntriesPreviewToolData struct {
	ReportBasis       string                         `json:"report_basis"`
	IsOfficialVersion bool                           `json:"is_official_version"`
	ApprovalStatus    repository.JournalEntrySummary `json:"approval_status"`
	Period            string                         `json:"period"`
	PeriodLocked      bool                           `json:"period_locked"`
	Page              int                            `json:"page"`
	PageSize          int                            `json:"page_size"`
	Items             []*repository.JournalEntry     `json:"items"`
	SideEffects       bool                           `json:"side_effects"`
}

// MonthlyClosingPeriodsToolData 有分录的会计期间列表，每期带 is_locked。
type MonthlyClosingPeriodsToolData struct {
	Periods     []repository.JournalEntryPeriod `json:"data"`
	Total       int                             `json:"total"`
	SideEffects bool                            `json:"side_effects"`
}

// MonthlyClosingLockStatusToolData 单期间锁账状态。
type MonthlyClosingLockStatusToolData struct {
	Period      string `json:"period"`
	IsLocked    bool   `json:"is_locked"`
	SideEffects bool   `json:"side_effects"`
}

// mcLegalEntity extracts the caller's legal entity from the execution
// context. Missing legal entity is a scope denial, never an unfiltered read.
func mcLegalEntity(execution agenttools.ExecutionContext) (string, bool) {
	entity := strings.TrimSpace(execution.Principal.Scope.LegalEntityID)
	return entity, entity != ""
}

func NewMonthlyClosingBatchesDefinition(reader MonthlyClosingReader) agenttools.ToolDefinition {
	return agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name: "lease.monthly_closing.batches.read", Version: "v1",
			DisplayName: "读取月结跑批状态",
			Description: "读取权限范围内月结跑批批次及其状态（draft/running/completed/completed_with_errors/failed/cancelled）。指定 period 时回显该期间锁账状态；已锁期间不得再被生成或过账覆盖。只读：生成、审批、过账等动作保留在人工流程，Agent 无写口。",
			Level:       agenttools.LevelRead, ReadOnly: true,
			Permissions:    []agenttools.Permission{{Resource: "monthly_closing", Action: "read"}},
			InputSchema:    json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"period":{"type":"string","pattern":"^[0-9]{4}-[0-9]{2}$","description":"YYYY-MM；缺省返回全部期间"}}}`),
			SupportsDryRun: true, MaxRows: 500, TimeoutSeconds: 15,
		},
		SkillIDs: []string{"fpna_copilot"},
		Handler: func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
			if reader == nil {
				return rejected(call.CallID, agenttools.ErrorDataUnavailable, "monthly closing reader unavailable"), nil
			}
			execution, err := agenttools.RequireExecutionContext(ctx)
			if err != nil {
				return rejected(call.CallID, agenttools.ErrorUnauthenticated, "authenticated tool context is required"), nil
			}
			legalEntityID, ok := mcLegalEntity(execution)
			if !ok {
				return rejected(call.CallID, agenttools.ErrorScopeDenied, "legal entity scope is required"), nil
			}
			args, err := decodeStrict[mcBatchesArguments](call.Arguments)
			if err != nil {
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, "arguments contain unsupported or invalid fields"), nil
			}
			args.Period = strings.TrimSpace(args.Period)
			if args.Period != "" && !mcValidPeriod(args.Period) {
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, "period must be YYYY-MM"), nil
			}
			batches, err := reader.GetBatches(ctx, args.Period, legalEntityID)
			if err != nil {
				return rejected(call.CallID, agenttools.ErrorBusinessFailure, "failed to load monthly closing batches"), nil
			}
			data := MonthlyClosingBatchesToolData{Batches: batches, Total: len(batches), Period: args.Period, SideEffects: false}
			if args.Period != "" {
				locked, lockErr := reader.IsPeriodLocked(ctx, args.Period, legalEntityID)
				if lockErr != nil {
					return rejected(call.CallID, agenttools.ErrorBusinessFailure, "failed to check period lock"), nil
				}
				data.PeriodLocked = &locked
			}
			return completed(call.CallID, data), nil
		},
	}
}

func NewMonthlyClosingEntriesPreviewDefinition(reader MonthlyClosingReader) agenttools.ToolDefinition {
	return agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name: "lease.monthly_closing.entries.preview", Version: "v1",
			DisplayName: "预览月结会计分录",
			Description: "按期间预览租赁会计分录及审批/过账状态。Draft 口径（report_basis=draft）：最低包含到 Draft，即 Draft + Pending + Approved 全含，响应声明 is_official_version=false，审批状态汇总按过滤全集统计；Approved 口径仅含已批准分录。词表见 CONTEXT.md「Approval and Reporting Basis」。只读：不改变任何分录的审批或过账状态。",
			Level:       agenttools.LevelRead, ReadOnly: true,
			Permissions: []agenttools.Permission{{Resource: "monthly_closing", Action: "read"}},
			InputSchema: json.RawMessage(`{
  "type":"object",
  "additionalProperties":false,
  "required":["period"],
  "properties":{
    "period":{"type":"string","pattern":"^[0-9]{4}-[0-9]{2}$"},
    "contract_id":{"type":"string","minLength":1},
    "status":{"type":"string","enum":["draft","approved","posted","reversed"]},
    "page":{"type":"integer","minimum":1},
    "page_size":{"type":"integer","minimum":1,"maximum":500}
  }
}`),
			SupportsDryRun: true, MaxRows: 500, TimeoutSeconds: 15,
		},
		SkillIDs: []string{"fpna_copilot"},
		Handler: func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
			if reader == nil {
				return rejected(call.CallID, agenttools.ErrorDataUnavailable, "monthly closing reader unavailable"), nil
			}
			execution, err := agenttools.RequireExecutionContext(ctx)
			if err != nil {
				return rejected(call.CallID, agenttools.ErrorUnauthenticated, "authenticated tool context is required"), nil
			}
			legalEntityID, ok := mcLegalEntity(execution)
			if !ok {
				return rejected(call.CallID, agenttools.ErrorScopeDenied, "legal entity scope is required"), nil
			}
			args, err := decodeStrict[mcEntriesPreviewArguments](call.Arguments)
			if err != nil {
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, "arguments contain unsupported or invalid fields"), nil
			}
			args.Period = strings.TrimSpace(args.Period)
			if !mcValidPeriod(args.Period) {
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, "period is required and must be YYYY-MM"), nil
			}
			status, valid := repository.NormalizeEntryStatus(args.Status)
			if !valid {
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, "分录状态无效，可选：draft、approved、posted、reversed"), nil
			}
			page, pageSize := repository.NormalizeEntryPaging(args.Page, args.PageSize)

			locked, lockErr := reader.IsPeriodLocked(ctx, args.Period, legalEntityID)
			if lockErr != nil {
				return rejected(call.CallID, agenttools.ErrorBusinessFailure, "failed to check period lock"), nil
			}
			items, summary, err := reader.ListJournalEntries(ctx, repository.JournalEntryQuery{
				LegalEntityID: legalEntityID, Period: args.Period, ContractID: strings.TrimSpace(args.ContractID),
				Status: status, Page: page, PageSize: pageSize,
			})
			if err != nil {
				return rejected(call.CallID, agenttools.ErrorBusinessFailure, "failed to list journal entries"), nil
			}
			data := MonthlyClosingEntriesPreviewToolData{
				ReportBasis: "draft", IsOfficialVersion: false,
				ApprovalStatus: summary, Period: args.Period, PeriodLocked: locked,
				Page: page, PageSize: pageSize, Items: items, SideEffects: false,
			}
			return completed(call.CallID, data), nil
		},
	}
}

func NewMonthlyClosingPeriodsDefinition(reader MonthlyClosingReader) agenttools.ToolDefinition {
	return agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name: "lease.monthly_closing.periods.read", Version: "v1",
			DisplayName: "读取有分录的会计期间",
			Description: "列出权限范围内实际持有分录的会计期间及各期条数、金额、draft/posted 计数与锁账状态（is_locked），用于选择要预览或核对的期间。只读。",
			Level:       agenttools.LevelRead, ReadOnly: true,
			Permissions:    []agenttools.Permission{{Resource: "monthly_closing", Action: "read"}},
			InputSchema:    json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"limit":{"type":"integer","minimum":1,"maximum":100}}}`),
			SupportsDryRun: true, MaxRows: 100, TimeoutSeconds: 15,
		},
		SkillIDs: []string{"fpna_copilot"},
		Handler: func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
			if reader == nil {
				return rejected(call.CallID, agenttools.ErrorDataUnavailable, "monthly closing reader unavailable"), nil
			}
			execution, err := agenttools.RequireExecutionContext(ctx)
			if err != nil {
				return rejected(call.CallID, agenttools.ErrorUnauthenticated, "authenticated tool context is required"), nil
			}
			legalEntityID, ok := mcLegalEntity(execution)
			if !ok {
				return rejected(call.CallID, agenttools.ErrorScopeDenied, "legal entity scope is required"), nil
			}
			args, err := decodeStrict[mcPeriodsArguments](call.Arguments)
			if err != nil || args.Limit < 0 || args.Limit > 100 {
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, "arguments contain unsupported or invalid fields"), nil
			}
			periods, err := reader.ListEntryPeriods(ctx, legalEntityID, args.Limit)
			if err != nil {
				return rejected(call.CallID, agenttools.ErrorBusinessFailure, "failed to list entry periods"), nil
			}
			return completed(call.CallID, MonthlyClosingPeriodsToolData{Periods: periods, Total: len(periods), SideEffects: false}), nil
		},
	}
}

func NewMonthlyClosingLockStatusDefinition(reader MonthlyClosingReader) agenttools.ToolDefinition {
	return agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name: "lease.monthly_closing.lock_status.read", Version: "v1",
			DisplayName: "读取期间锁账状态",
			Description: "读取单个会计期间在权限范围内的锁账状态。已锁期间不可再被生成或过账覆盖；锁账与解锁动作本身保留在人工流程，Agent 无写口。",
			Level:       agenttools.LevelRead, ReadOnly: true,
			Permissions:    []agenttools.Permission{{Resource: "monthly_closing", Action: "read"}},
			InputSchema:    json.RawMessage(`{"type":"object","additionalProperties":false,"required":["period"],"properties":{"period":{"type":"string","pattern":"^[0-9]{4}-[0-9]{2}$"}}}`),
			SupportsDryRun: true, MaxRows: 1, TimeoutSeconds: 15,
		},
		SkillIDs: []string{"fpna_copilot"},
		Handler: func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
			if reader == nil {
				return rejected(call.CallID, agenttools.ErrorDataUnavailable, "monthly closing reader unavailable"), nil
			}
			execution, err := agenttools.RequireExecutionContext(ctx)
			if err != nil {
				return rejected(call.CallID, agenttools.ErrorUnauthenticated, "authenticated tool context is required"), nil
			}
			legalEntityID, ok := mcLegalEntity(execution)
			if !ok {
				return rejected(call.CallID, agenttools.ErrorScopeDenied, "legal entity scope is required"), nil
			}
			args, err := decodeStrict[mcLockStatusArguments](call.Arguments)
			if err != nil {
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, "arguments contain unsupported or invalid fields"), nil
			}
			args.Period = strings.TrimSpace(args.Period)
			if !mcValidPeriod(args.Period) {
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, "period is required and must be YYYY-MM"), nil
			}
			locked, err := reader.IsPeriodLocked(ctx, args.Period, legalEntityID)
			if err != nil {
				return rejected(call.CallID, agenttools.ErrorBusinessFailure, "failed to check period lock"), nil
			}
			return completed(call.CallID, MonthlyClosingLockStatusToolData{Period: args.Period, IsLocked: locked, SideEffects: false}), nil
		},
	}
}

// completed wraps a successful tool result.
func completed(callID string, data any) agenttools.ToolResult {
	return agenttools.ToolResult{CallID: callID, Status: agenttools.StatusCompleted, Data: data}
}
