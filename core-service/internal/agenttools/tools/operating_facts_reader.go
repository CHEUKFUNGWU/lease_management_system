package tools

// B-3 经营事实只读接入：门店主数据列表、store-day 原始事实、store-day 指标
// 聚合。粒度纪律：只有 store-day 粒度的读口，不提供月聚合入口；指标语义走
// retail-kpi-v1（严格 null/零分母、覆盖率门槛、来源冲突 409），数字全部由
// 确定性服务计算，前端与 Agent 都不得重算评分或排序。
//
// 来源信封（底线 3）：每行事实回显 data_classification / source_system /
// import_batch_id / as_of_at / version 五个字段；聚合视图由 sourceenvelope.Build
// （唯一信封生产者）产出汇总溯源。缺失是 nil，不填 0，不反推。
//
// 与既有三个 retail.* 工具同族：限定 retail_operations skill，权限按工具自身
// 性质取 reports:read。

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/retailkpi"
	"github.com/lease-management-system/core-service/internal/services/retailpulse"
	"github.com/lease-management-system/core-service/internal/services/sourceenvelope"
)

const (
	retailFactsSkill          = retailOperationsSkill
	maxRetailStoreDayPageSize = 5000
	maxRetailStoreDayRange    = 366
	maxRetailKPIDateRange     = 366
	defaultStoreDayPageSize   = 500
)

// OperatingFactsReader is the read-only seam over the operating-facts store
// list and the raw store-day ledger. *repository.OperatingFactsRepository
// implements it directly. Deliberately no write methods: fact writes belong
// to the import pipeline and its approval gates.
type OperatingFactsReader interface {
	ListStores(ctx context.Context, entity access.EntityFilter, period, storeID string) ([]*repository.StoreOperatingFact, error)
	ListRetailStoreDayFactsPage(ctx context.Context, entity access.EntityFilter, dateFrom, dateTo string, storeIDs []string, dataClassification string, pageSize, offset int) (*repository.RetailStoreDayFactsPage, error)
}

type factsStoresArguments struct {
	Period  string `json:"period,omitempty"`
	StoreID string `json:"store_id,omitempty"`
}

type factsStoreDaysArguments struct {
	DateFrom           string   `json:"date_from"`
	DateTo             string   `json:"date_to"`
	StoreIDs           []string `json:"store_ids,omitempty"`
	DataClassification string   `json:"data_classification,omitempty"`
	Page               int      `json:"page,omitempty"`
	PageSize           int      `json:"page_size,omitempty"`
}

type factsKpiStoreDaysArguments struct {
	DateFrom           string   `json:"date_from"`
	DateTo             string   `json:"date_to"`
	DataClassification string   `json:"data_classification"`
	DatasetVersion     string   `json:"dataset_version,omitempty"`
	GroupBy            string   `json:"group_by,omitempty"`
	StoreIDs           []string `json:"store_ids,omitempty"`
	SourceSystem       string   `json:"source_system,omitempty"`
}

// OperatingStoresToolData 门店经营事实列表（operating-facts/stores）。
type OperatingStoresToolData struct {
	Data        []*repository.StoreOperatingFact `json:"data"`
	Total       int                              `json:"total"`
	SideEffects bool                             `json:"side_effects"`
}

// OperatingStoreDaysToolData store-day 原始事实分页。每一行都携带完整来源
// 信封字段（data_classification/source_system/import_batch_id/as_of_at/version，
// 由结构体 JSON tag 逐字段输出），汇总信封由 sourceenvelope.Build 产出。
type OperatingStoreDaysToolData struct {
	Basis              string                           `json:"basis"`
	DataClassification string                           `json:"data_classification"`
	Envelope           sourceenvelope.Envelope          `json:"envelope"`
	CoverageFrom       string                           `json:"coverage_from"`
	CoverageTo         string                           `json:"coverage_to"`
	Total              int                              `json:"total"`
	ReturnedCount      int                              `json:"returned_count"`
	Page               int                              `json:"page"`
	PageSize           int                              `json:"page_size"`
	HasMore            bool                             `json:"has_more"`
	Data               []*repository.RetailStoreDayFact `json:"data"`
	SideEffects        bool                             `json:"side_effects"`
}

// KpiStoreDaysToolData retail-kpi-v1 的 store-day 聚合视图。decision_ready
// 为 false 时必须带原因；缺失 KPI 的 Value 是 nil，不是 0。
type KpiStoreDaysToolData struct {
	Basis                     string                  `json:"basis"`
	FormulaVersion            string                  `json:"formula_version"`
	DataClassification        string                  `json:"data_classification"`
	DatasetVersion            string                  `json:"dataset_version,omitempty"`
	SimulationDatasetVersions []string                `json:"simulation_dataset_versions,omitempty"`
	RequestedScope            map[string]any          `json:"requested_scope"`
	GroupBy                   string                  `json:"group_by"`
	Coverage                  retailkpi.Coverage      `json:"coverage"`
	DecisionReady             bool                    `json:"decision_ready"`
	DecisionReadyReason       string                  `json:"decision_ready_reason,omitempty"`
	MultiCurrency             bool                    `json:"multi_currency"`
	FactVersionMin            int                     `json:"fact_version_min"`
	FactVersionMax            int                     `json:"fact_version_max"`
	HighestAsOf               *time.Time              `json:"highest_as_of,omitempty"`
	SourceSystems             []string                `json:"source_systems"`
	TotalRows                 int                     `json:"total_rows"`
	Data                      []retailkpi.Aggregate   `json:"data"`
	Envelope                  sourceenvelope.Envelope `json:"envelope"`
	SideEffects               bool                    `json:"side_effects"`
}

// factsEntityFilter derives the tenant entity filter from the authenticated
// principal scope, failing closed on a scope that cannot produce a filter.
func factsEntityFilter(execution agenttools.ExecutionContext) (access.EntityFilter, error) {
	return access.FromScope(execution.Principal.Scope)
}

func NewOperatingStoresDefinition(reader OperatingFactsReader) agenttools.ToolDefinition {
	return agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name: "retail.operating_facts.stores.read", Version: "v1",
			DisplayName: "读取门店经营事实列表",
			Description: "读取权限范围内有经营事实的门店列表及其期间快照。可按 period（YYYY-MM）或 store_id 收窄。只读。",
			Level:       agenttools.LevelRead, ReadOnly: true,
			Permissions:    []agenttools.Permission{{Resource: "reports", Action: "read"}},
			InputSchema:    json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"period":{"type":"string","pattern":"^[0-9]{4}-[0-9]{2}$"},"store_id":{"type":"string","format":"uuid"}}}`),
			SupportsDryRun: true, MaxRows: 2000, TimeoutSeconds: 20,
		},
		SkillIDs: []string{retailFactsSkill},
		Handler: func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
			if reader == nil {
				return rejected(call.CallID, agenttools.ErrorDataUnavailable, "operating facts reader unavailable"), nil
			}
			execution, err := agenttools.RequireExecutionContext(ctx)
			if err != nil {
				return rejected(call.CallID, agenttools.ErrorUnauthenticated, "authenticated tool context is required"), nil
			}
			if execution.SkillID != retailFactsSkill {
				return rejected(call.CallID, agenttools.ErrorPermissionDenied, "retail tool is restricted to retail_operations skill"), nil
			}
			args, err := decodeStrict[factsStoresArguments](call.Arguments)
			if err != nil {
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, "arguments contain unsupported or invalid fields"), nil
			}
			entity, entityErr := factsEntityFilter(execution)
			if entityErr != nil {
				return rejected(call.CallID, agenttools.ErrorScopeDenied, "legal entity scope is required"), nil
			}
			rows, err := reader.ListStores(ctx, entity, strings.TrimSpace(args.Period), strings.TrimSpace(args.StoreID))
			if err != nil {
				return rejected(call.CallID, agenttools.ErrorBusinessFailure, "failed to load operating fact stores"), nil
			}
			return completed(call.CallID, OperatingStoresToolData{Data: rows, Total: len(rows), SideEffects: false}), nil
		},
	}
}

func NewOperatingStoreDaysDefinition(reader OperatingFactsReader) agenttools.ToolDefinition {
	return agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name: "retail.operating_facts.store_days.read", Version: "v1",
			DisplayName: "读取 store-day 原始事实",
			Description: "分页读取权限范围内 store-day 粒度的原始经营事实。每行逐字段携带来源信封：data_classification、source_system、import_batch_id、as_of_at、version——脱离信封的数字不是事实。缺失指标为 null，不用 0 填补。只读。",
			Level:       agenttools.LevelRead, ReadOnly: true,
			Permissions: []agenttools.Permission{{Resource: "reports", Action: "read"}},
			InputSchema: json.RawMessage(`{
  "type":"object",
  "additionalProperties":false,
  "required":["date_from","date_to"],
  "properties":{
    "date_from":{"type":"string","pattern":"^[0-9]{4}-[0-9]{2}-[0-9]{2}$"},
    "date_to":{"type":"string","pattern":"^[0-9]{4}-[0-9]{2}-[0-9]{2}$"},
    "store_ids":{"type":"array","items":{"type":"string","format":"uuid"}},
    "data_classification":{"type":"string","enum":["production","simulated"]},
    "page":{"type":"integer","minimum":1},
    "page_size":{"type":"integer","minimum":1,"maximum":5000}
  }
}`),
			SupportsDryRun: true, MaxRows: 5000, TimeoutSeconds: 30,
		},
		SkillIDs: []string{retailFactsSkill},
		Handler: func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
			if reader == nil {
				return rejected(call.CallID, agenttools.ErrorDataUnavailable, "operating facts reader unavailable"), nil
			}
			execution, err := agenttools.RequireExecutionContext(ctx)
			if err != nil {
				return rejected(call.CallID, agenttools.ErrorUnauthenticated, "authenticated tool context is required"), nil
			}
			if execution.SkillID != retailFactsSkill {
				return rejected(call.CallID, agenttools.ErrorPermissionDenied, "retail tool is restricted to retail_operations skill"), nil
			}
			args, err := decodeStrict[factsStoreDaysArguments](call.Arguments)
			if err != nil {
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, "arguments contain unsupported or invalid fields"), nil
			}
			fromText, toText := strings.TrimSpace(args.DateFrom), strings.TrimSpace(args.DateTo)
			from, fromErr := time.Parse("2006-01-02", fromText)
			to, toErr := time.Parse("2006-01-02", toText)
			if fromErr != nil || toErr != nil {
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, "date_from and date_to must be ISO dates (YYYY-MM-DD)"), nil
			}
			if to.Before(from) || int(to.Sub(from).Hours()/24)+1 > maxRetailStoreDayRange {
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, "date range exceeds maximum of 366 days"), nil
			}
			classification := strings.TrimSpace(args.DataClassification)
			if classification != "" && classification != "production" && classification != "simulated" {
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, "data_classification must be production or simulated"), nil
			}
			storeIDs, storeErr := parseRetailStoreIDList(args.StoreIDs)
			if storeErr != "" {
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, storeErr), nil
			}
			page, pageSize := args.Page, args.PageSize
			if page <= 0 {
				page = 1
			}
			if pageSize <= 0 {
				pageSize = defaultStoreDayPageSize
			}
			if pageSize > maxRetailStoreDayPageSize || page > 1_000_000 {
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, "pagination out of supported range"), nil
			}
			entity, entityErr := factsEntityFilter(execution)
			if entityErr != nil {
				return rejected(call.CallID, agenttools.ErrorScopeDenied, "legal entity scope is required"), nil
			}
			resultPage, err := reader.ListRetailStoreDayFactsPage(ctx, entity, fromText, toText, storeIDs, classification, pageSize, (page-1)*pageSize)
			if err != nil {
				return rejected(call.CallID, agenttools.ErrorBusinessFailure, "failed to load store-day facts"), nil
			}
			data := resultPage.Data
			factsForEnvelope := make([]retailkpi.DailyFact, 0, len(data))
			classifications := map[string]struct{}{}
			coverageFrom, coverageTo := fromText, toText
			storesSeen := map[string]struct{}{}
			for _, row := range data {
				classifications[row.DataClassification] = struct{}{}
				storesSeen[row.StoreID] = struct{}{}
				if coverageFrom == "" || row.BusinessDate < coverageFrom {
					coverageFrom = row.BusinessDate
				}
				if coverageTo == "" || row.BusinessDate > coverageTo {
					coverageTo = row.BusinessDate
				}
				datasetVersion := row.SimulationDatasetVersion
				if datasetVersion != nil && *datasetVersion == "" {
					datasetVersion = nil
				}
				businessDate, _ := time.Parse("2006-01-02", row.BusinessDate)
				factsForEnvelope = append(factsForEnvelope, retailkpi.DailyFact{
					StoreID: row.StoreID, BusinessDate: businessDate, AsOfAt: row.AsOfAt,
					SourceSystem: row.SourceSystem, DataClassification: row.DataClassification,
					SimulationDatasetVersion: datasetVersion, ImportBatchID: row.ImportBatchID, Version: row.Version,
				})
			}
			classificationSummary := "none"
			switch len(classifications) {
			case 1:
				for value := range classifications {
					classificationSummary = value
				}
			case 0:
			default:
				classificationSummary = "mixed"
			}
			envelope := sourceenvelope.Build(factsForEnvelope, sourceenvelope.Spec{
				Classification: classificationSummary,
				FormulaVersion: retailkpi.FormulaVersion,
				PulseVersion:   retailpulse.PulseVersion,
				Current:        sourceenvelope.PeriodSpec{From: mustParseFactsDate(coverageFrom), To: mustParseFactsDate(coverageTo)},
				DecisionReady:  false, DecisionReadyReason: "raw_facts_read",
			})
			hasMore := (page-1)*pageSize+len(data) < resultPage.Total
			return completed(call.CallID, OperatingStoreDaysToolData{
				Basis: "Working", DataClassification: classificationSummary,
				Envelope: envelope, CoverageFrom: coverageFrom, CoverageTo: coverageTo,
				Total: resultPage.Total, ReturnedCount: len(data),
				Page: page, PageSize: pageSize, HasMore: hasMore,
				Data: data, SideEffects: false,
			}), nil
		},
	}
}

func NewKpiStoreDaysDefinition(reader RetailOperationsReader) agenttools.ToolDefinition {
	return agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name: "retail.kpis.store_days.read", Version: "v1",
			DisplayName: "读取 store-day 指标聚合",
			Description: "按 retail-kpi-v1 从 store-day 事实聚合 KPI（group_by=total/store/region/brand）。严格 null 语义：缺失 KPI 的 value 是 nil 不是 0；覆盖不足时 decision_ready=false 并给原因；来源冲突返回 source_conflict。数据分类必填：production 不允许 dataset_version，simulated 必须给。",
			Level:       agenttools.LevelRead, ReadOnly: true,
			Permissions: []agenttools.Permission{{Resource: "reports", Action: "read"}},
			InputSchema: json.RawMessage(`{
  "type":"object",
  "additionalProperties":false,
  "required":["date_from","date_to","data_classification"],
  "properties":{
    "date_from":{"type":"string","pattern":"^[0-9]{4}-[0-9]{2}-[0-9]{2}$"},
    "date_to":{"type":"string","pattern":"^[0-9]{4}-[0-9]{2}-[0-9]{2}$"},
    "data_classification":{"type":"string","enum":["production","simulated"]},
    "dataset_version":{"type":"string"},
    "group_by":{"type":"string","enum":["total","region","brand","store"]},
    "store_ids":{"type":"array","items":{"type":"string","format":"uuid"}},
    "source_system":{"type":"string"}
  }
}`),
			SupportsDryRun: true, MaxRows: 5000, TimeoutSeconds: 30,
		},
		SkillIDs: []string{retailFactsSkill},
		Handler: func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
			if reader == nil {
				return rejected(call.CallID, agenttools.ErrorDataUnavailable, "retail kpi fact reader unavailable"), nil
			}
			execution, err := agenttools.RequireExecutionContext(ctx)
			if err != nil {
				return rejected(call.CallID, agenttools.ErrorUnauthenticated, "authenticated tool context is required"), nil
			}
			if execution.SkillID != retailFactsSkill {
				return rejected(call.CallID, agenttools.ErrorPermissionDenied, "retail tool is restricted to retail_operations skill"), nil
			}
			legalEntityID := strings.TrimSpace(execution.Principal.Scope.LegalEntityID)
			if legalEntityID == "" {
				return rejected(call.CallID, agenttools.ErrorScopeDenied, "legal entity scope is required"), nil
			}
			args, err := decodeStrict[factsKpiStoreDaysArguments](call.Arguments)
			if err != nil {
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, "arguments contain unsupported or invalid fields"), nil
			}
			fromText, toText := strings.TrimSpace(args.DateFrom), strings.TrimSpace(args.DateTo)
			from, fromErr := time.Parse("2006-01-02", fromText)
			to, toErr := time.Parse("2006-01-02", toText)
			if fromErr != nil || toErr != nil {
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, "date_from and date_to must be ISO dates (YYYY-MM-DD)"), nil
			}
			if to.Before(from) || int(to.Sub(from).Hours()/24)+1 > maxRetailKPIDateRange {
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, "date range must be 1-366 days"), nil
			}
			classification := strings.TrimSpace(args.DataClassification)
			if classification != "production" && classification != "simulated" {
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, "data_classification must be explicitly production or simulated"), nil
			}
			datasetVersion := strings.TrimSpace(args.DatasetVersion)
			if classification == "simulated" && datasetVersion == "" {
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, "dataset_version is required for simulated data"), nil
			}
			if classification == "production" && datasetVersion != "" {
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, "dataset_version is not allowed for production data"), nil
			}
			groupBy := strings.TrimSpace(args.GroupBy)
			if groupBy == "" {
				groupBy = "total"
			}
			if groupBy != "total" && groupBy != "region" && groupBy != "brand" && groupBy != "store" {
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, "group_by must be one of total, region, brand, store"), nil
			}
			storeIDs, storeErr := parseRetailStoreIDList(args.StoreIDs)
			if storeErr != "" {
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, storeErr), nil
			}
			sourceSystem := strings.TrimSpace(args.SourceSystem)
			set, err := reader.QueryFacts(ctx, legalEntityID, fromText, toText, classification, datasetVersion, sourceSystem, storeIDs)
			if err != nil {
				code := retailErrorCode(err)
				message := retailPublicError(err)
				if errors.Is(err, repository.ErrRetailKPISourceConflict) {
					return rejected(call.CallID, code, message+", specify source_system"), nil
				}
				return rejected(call.CallID, code, message), nil
			}
			expectedCount := set.ExpectedStoreCount
			aggregates, coverage, err := retailkpi.AggregateFacts(set.Facts, retailkpi.Request{
				DateFrom: from, DateTo: to, RequestedDateFrom: fromText, RequestedDateTo: toText,
				GroupBy: groupBy, ExpectedStoreCount: expectedCount,
			})
			if err != nil {
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, "store-day aggregation failed"), nil
			}
			readReady := len(aggregates) > 0
			for _, aggregate := range aggregates {
				readReady = readReady && aggregate.DecisionReady
			}
			readReadyReason := ""
			if !readReady {
				if coverage.ExpectedStoreDays > 0 && coverage.ObservedStoreDays < coverage.ExpectedStoreDays {
					readReadyReason = "incomplete_store_day_coverage"
				} else {
					readReadyReason = "not_decision_ready"
				}
			}
			var highestAsOf *time.Time
			if !set.HighestAsOf.IsZero() {
				value := set.HighestAsOf
				highestAsOf = &value
			}
			currencies := map[string]struct{}{}
			for _, fact := range set.Facts {
				currencies[fact.Currency] = struct{}{}
			}
			envelope := sourceenvelope.Build(set.Facts, sourceenvelope.Spec{
				Classification: classification,
				FormulaVersion: retailkpi.FormulaVersion,
				PulseVersion:   retailpulse.PulseVersion,
				Current: sourceenvelope.PeriodSpec{From: from, To: to,
					ExpectedStoreDays: coverage.ExpectedStoreDays},
				DecisionReady: readReady, DecisionReadyReason: readReadyReason,
			})
			data := KpiStoreDaysToolData{
				Basis: "Working", FormulaVersion: retailkpi.FormulaVersion,
				DataClassification: classification, DatasetVersion: datasetVersion,
				SimulationDatasetVersions: set.DatasetVersions,
				RequestedScope:            map[string]any{"legal_entity_id": legalEntityID, "store_ids": storeIDs},
				GroupBy:                   groupBy, Coverage: coverage,
				DecisionReady: readReady, DecisionReadyReason: readReadyReason,
				MultiCurrency:  len(currencies) > 1,
				FactVersionMin: set.MinFactVersion, FactVersionMax: set.MaxFactVersion,
				HighestAsOf: highestAsOf, SourceSystems: set.SourceSystems,
				TotalRows: len(aggregates), Data: aggregates,
				Envelope: envelope, SideEffects: false,
			}
			return completed(call.CallID, data), nil
		},
	}
}

func parseRetailStoreIDList(values []string) ([]string, string) {
	result := make([]string, 0, len(values))
	for _, raw := range values {
		item := strings.TrimSpace(raw)
		if item == "" {
			continue
		}
		if _, err := parseStoreID(item); err != nil {
			return nil, "store_ids must contain UUIDs"
		}
		result = append(result, item)
	}
	return result, ""
}

func mustParseFactsDate(value string) time.Time {
	parsed, _ := time.Parse("2006-01-02", value)
	return parsed
}
