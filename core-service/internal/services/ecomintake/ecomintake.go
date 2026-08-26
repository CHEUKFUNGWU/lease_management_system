// Package ecomintake 是电商五类来源的受控导入编排（模块深化 EM1）。
//
// 五类来源各自一套列结构，但没有各自一套导入纪律：幂等、信封、模板版本必须只有一份
// 实现——形状照 retailingest 的 IngestBatch 接缝（C3 收拢），共享的是 controlledintake
// 与 sourceenvelope 两层，不是编排本身。事实族不同（storefront-day / campaign-day）、
// 业务键不同（平台订单号 / payout ID / 发票号），合并进 retailingest 会让一个 Service
// 拥有两套业务键规则。
//
// 编排顺序：解析 → 模板版本比对 → 信封完整性校验（缺字段整批拒绝）→ 行级校验与业务键
// 去重 → 建批次（请求级幂等键）→ 重放短路 → 分块落库 → 终态回写。
package ecomintake

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lease-management-system/core-service/internal/services/controlledintake"
)

// Source 来源标识。前六个对应 PRD E1 的五类来源（shopify 一类含订单/退款），
// gl_revenue 与 overhead 是 R-E3-5（会计收入只读来自 GL 导入）与利润表固定费行的落地来源。
type Source string

const (
	SourceShopify     Source = "shopify"     // 订单/退款 → storefront_day_facts（销售度量族）
	SourceAdsBooked   Source = "ads_booked"  // 平台后台 campaign 日耗 → campaign_day_facts(basis=booked)
	SourceAdInvoice   Source = "ad_invoice"  // 代理发票 → campaign_day_facts(basis=paid) + ad_invoices 登记
	SourceSettlement  Source = "settlement"  // PayPal/Stripe 结算文件 → payout_lines（+准备金事件派生）
	SourceBank        Source = "bank"        // 银行到账流水 → bank_lines
	Source3PL         Source = "3pl"         // 履约账单 → storefront_day_facts(fulfillment)
	SourceProcurement Source = "procurement" // 采购/头程发票 → storefront_day_facts(landed_cost)
	SourceOrderLines  Source = "order_lines" // 订单行明细 → order_line_evidence（下钻与对账证据）
	SourceGLRevenue   Source = "gl_revenue"  // 总账收入导出 → storefront_gl_revenues
	SourceOverhead    Source = "overhead"    // 分摊固定费 → storefront_fixed_costs
)

// AdBasis 取值（campaign 表落库用）。
const (
	BasisBooked = "booked"
	BasisPaid   = "paid"
)

// 目标表名常量（ParsedRow.Table 白名单）。
const (
	TableStorefrontDayFacts = "storefront_day_facts"
	TableCampaignDayFacts   = "campaign_day_facts"
	TablePayoutLines        = "payout_lines"
	TableBankLines          = "bank_lines"
	TableAdInvoices         = "ad_invoices"
	TableReserveEvents      = "rolling_reserve_events"
	TableOrderLineEvidence  = "order_line_evidence"
	TableGLRevenues         = "storefront_gl_revenues"
	TableFixedCosts         = "storefront_fixed_costs"
)

// ColumnKind 模板列类型。
type ColumnKind string

const (
	KindString  ColumnKind = "string"
	KindDate    ColumnKind = "date"
	KindMoney   ColumnKind = "money"
	KindInt     ColumnKind = "int"
	KindDecimal ColumnKind = "decimal"
	KindEnum    ColumnKind = "enum"
)

// TemplateColumn 标准模板列。Required 列缺失表头或行值为空 ⇒ 行错误；未知列 ⇒ 整批拒绝。
type TemplateColumn struct {
	Name     string     `json:"name"`
	Required bool       `json:"required"`
	Kind     ColumnKind `json:"kind"`
	Default  string     `json:"default,omitempty"`
	Enum     []string   `json:"enum,omitempty"`
}

// TemplateDef 一份标准模板：版本随导入留痕（R-E1-4），旧版本导入被拒并提示当前版本。
type TemplateDef struct {
	Source       Source           `json:"source"`
	Version      string           `json:"version"` // 当前版本
	Grain        string           `json:"grain"`
	TargetTables []string         `json:"target_tables"`
	Columns      []TemplateColumn `json:"columns"`
}

// TemplateVersions 全部模板当前版本（改列结构必须递增）。
var TemplateVersions = map[Source]string{
	SourceShopify: "1", SourceAdsBooked: "1", SourceAdInvoice: "1",
	SourceSettlement: "1", SourceBank: "1", Source3PL: "1",
	SourceProcurement: "1", SourceOrderLines: "1", SourceGLRevenue: "1", SourceOverhead: "1",
}

func cols(defs ...TemplateColumn) []TemplateColumn { return defs }

func str(name string, required bool, def ...string) TemplateColumn {
	c := TemplateColumn{Name: name, Required: required, Kind: KindString}
	if len(def) > 0 {
		c.Default = def[0]
	}
	return c
}

func money(name string, required bool) TemplateColumn {
	return TemplateColumn{Name: name, Required: required, Kind: KindMoney}
}

func intCol(name string, required bool) TemplateColumn {
	return TemplateColumn{Name: name, Required: required, Kind: KindInt}
}

func dateCol(name string, required bool) TemplateColumn {
	return TemplateColumn{Name: name, Required: required, Kind: KindDate}
}

func enumCol(name string, required bool, def string, values ...string) TemplateColumn {
	return TemplateColumn{Name: name, Required: required, Kind: KindEnum, Enum: values, Default: def}
}

var currencyCol = str("currency", true)

// dayKeyCols 是站点日事实族共用的维度列。
func dayCols(extra ...TemplateColumn) []TemplateColumn {
	out := []TemplateColumn{
		dateCol("business_date", true),
		str("channel", false, "direct"),
		str("sku", false, ""),
		currencyCol,
	}
	return append(out, extra...)
}

// Templates 全部标准模板（列序即模板下载的表头序）。
var Templates = map[Source]TemplateDef{
	SourceShopify: {
		Source: SourceShopify, Version: "1",
		Grain:        "storefront × business_date × channel × sku（聚合；订单行明细走对账证据文件）",
		TargetTables: []string{TableStorefrontDayFacts},
		Columns: cols(dayCols(
			money("gmv_amount", true),
			money("discount_amount", false),
			money("refund_amount", false),
			money("chargeback_loss_amount", false),
			intCol("order_count", false),
			intCol("new_customer_orders", false),
			money("payment_fee_amount", false),
			money("tax_collected_amount", false),
		)...),
	},
	SourceAdsBooked: {
		Source: SourceAdsBooked, Version: "1",
		Grain:        "campaign × business_date（账面口径 booked）",
		TargetTables: []string{TableCampaignDayFacts},
		Columns: cols(
			dateCol("business_date", true),
			str("campaign_id", false, "all"),
			str("campaign_name", false),
			str("media_owner", false),
			currencyCol,
			money("spend_amount", true),
			intCol("impressions", false),
			intCol("clicks", false),
			money("conversions", false),
		),
	},
	SourceAdInvoice: {
		Source: SourceAdInvoice, Version: "1",
		Grain:        "invoice × campaign × business_date（实付口径 paid；发票号必填）",
		TargetTables: []string{TableCampaignDayFacts, TableAdInvoices},
		Columns: cols(
			str("invoice_no", true),
			dateCol("invoice_date", false),
			str("agent_name", false),
			str("media_owner", false),
			dateCol("period_start", false),
			dateCol("period_end", false),
			money("gross_amount", false),
			money("rebate_amount", false),
			money("payable_amount", false),
			dateCol("business_date", true),
			str("campaign_id", false, "all"),
			currencyCol,
			money("spend_amount", true),
			intCol("impressions", false),
			intCol("clicks", false),
			money("conversions", false),
		),
	},
	SourceSettlement: {
		Source: SourceSettlement, Version: "1",
		Grain:        "provider × payout_id（payout 明细；准备金列非零自动派生占用/释放事件）",
		TargetTables: []string{TablePayoutLines, TableReserveEvents},
		Columns: cols(
			str("provider", true),
			str("payout_id", true),
			dateCol("payout_date", true),
			currencyCol,
			money("gross_amount", false),
			money("fee_amount", false),
			money("refund_amount", false),
			money("chargeback_amount", false),
			money("fx_amount", false),
			money("adjustment_amount", false),
			money("reserve_hold_amount", false),
			money("reserve_release_amount", false),
			money("net_amount", true),
		),
	},
	SourceBank: {
		Source: SourceBank, Version: "1",
		Grain:        "bank_ref（银行到账流水）",
		TargetTables: []string{TableBankLines},
		Columns: cols(
			str("bank_ref", true),
			dateCol("value_date", true),
			currencyCol,
			money("amount", true),
			enumCol("direction", false, "in", "in", "out"),
			str("counterparty", false),
		),
	},
	Source3PL: {
		Source: Source3PL, Version: "1",
		Grain:        "storefront × business_date × channel × sku（履约成本度量族）",
		TargetTables: []string{TableStorefrontDayFacts},
		Columns: cols(dayCols(
			money("fulfillment_amount", true),
		)...),
	},
	SourceProcurement: {
		Source: SourceProcurement, Version: "1",
		Grain:        "storefront × business_date × channel × sku（落地成本度量族）",
		TargetTables: []string{TableStorefrontDayFacts},
		Columns: cols(dayCols(
			money("landed_cost_amount", true),
		)...),
	},
	SourceOrderLines: {
		Source: SourceOrderLines, Version: "1",
		Grain:        "platform_order_no × line_no（订单行证据：仅供下钻与对账，不进分析读路径）",
		TargetTables: []string{TableOrderLineEvidence},
		Columns: cols(
			str("platform_order_no", true),
			intCol("line_no", false),
			dateCol("business_date", true),
			str("channel", false, "direct"),
			str("sku", false, ""),
			currencyCol,
			money("gross_amount", false),
			money("discount_amount", false),
			money("refund_amount", false),
			money("tax_amount", false),
			money("quantity", false),
			str("payout_id", false),
		),
	},
	SourceGLRevenue: {
		Source: SourceGLRevenue, Version: "1",
		Grain:        "storefront × period × currency（会计口径收入唯一入口；sitepnl 只读消费）",
		TargetTables: []string{TableGLRevenues},
		Columns: cols(
			str("period", true), // YYYY-MM
			currencyCol,
			money("revenue_amount", true),
			str("gl_account", false),
		),
	},
	SourceOverhead: {
		Source: SourceOverhead, Version: "1",
		Grain:        "storefront × period × currency（分摊固定费）",
		TargetTables: []string{TableFixedCosts},
		Columns: cols(
			str("period", true),
			currencyCol,
			money("fixed_cost_amount", true),
			str("memo", false),
		),
	},
}

// HasColumn 报告模板是否声明该列。
func (t TemplateDef) HasColumn(name string) bool {
	for _, c := range t.Columns {
		if c.Name == name {
			return true
		}
	}
	return false
}

// EnvelopeSpec 五信封字段（R-E1-1）：请求级声明，缺任一整批拒绝。
type EnvelopeSpec struct {
	SourceSystem             string    `json:"source_system"`
	AsOfAt                   time.Time `json:"as_of_at"`
	DataClassification       string    `json:"data_classification"`
	SimulationDatasetVersion string    `json:"simulation_dataset_version,omitempty"`
}

// Validate fail-closed 校验。
func (e EnvelopeSpec) Validate() error {
	if strings.TrimSpace(e.SourceSystem) == "" {
		return fmt.Errorf("%w: source_system 必填", ErrEnvelopeIncomplete)
	}
	if e.AsOfAt.IsZero() {
		return fmt.Errorf("%w: as_of_at 必填", ErrEnvelopeIncomplete)
	}
	switch e.DataClassification {
	case "":
		return fmt.Errorf("%w: data_classification 必填", ErrEnvelopeIncomplete)
	case "simulated":
		if strings.TrimSpace(e.SimulationDatasetVersion) == "" {
			return fmt.Errorf("%w: simulated 数据必须带 simulation_dataset_version", ErrEnvelopeIncomplete)
		}
	case "production":
		if strings.TrimSpace(e.SimulationDatasetVersion) != "" {
			return fmt.Errorf("%w: production 数据不得带 simulation_dataset_version", ErrEnvelopeIncomplete)
		}
	default:
		return fmt.Errorf("%w: data_classification 仅允许 production|simulated", ErrEnvelopeIncomplete)
	}
	return nil
}

// ImportSpec 一次受控导入的全部输入。
type ImportSpec struct {
	LegalEntityID   string
	StorefrontID    string
	UserID          *string
	Filename        string
	Data            []byte
	Format          controlledintake.Format
	Source          Source
	TemplateVersion string // 客户端声明的模板版本；旧版本拒绝并报当前版
	IdempotencyKey  string // 请求级幂等键（批次级）
	Envelope        EnvelopeSpec
}

// ImportErrorKind 失败分类（照 retailingest 的 failure taxonomy）。
type ImportErrorKind string

const (
	FailureParse       ImportErrorKind = "parse"
	FailureEnvelope    ImportErrorKind = "envelope"
	FailureTemplate    ImportErrorKind = "template_version"
	FailureNoValidRows ImportErrorKind = "no_valid_rows"
	FailureSystem      ImportErrorKind = "system"
	FailureConflict    ImportErrorKind = "idempotency_conflict"
)

// ImportError 让传输层把类别映射到状态码与 errcontract code。
type ImportError struct {
	Kind    ImportErrorKind
	Message string
	Err     error
}

func (e *ImportError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *ImportError) Unwrap() error { return e.Err }

func failure(kind ImportErrorKind, format string, args ...any) *ImportError {
	return &ImportError{Kind: kind, Message: fmt.Sprintf(format, args...)}
}

// ErrEnvelopeIncomplete 信封不完整的哨兵错误（fail-closed 整批拒绝）。
var ErrEnvelopeIncomplete = fmt.Errorf("envelope incomplete")

// ErrTemplateVersionRejected 旧模板版本的哨兵错误。
var ErrTemplateVersionRejected = fmt.Errorf("template version rejected")

// ParsedRow 归一化后的行：目标表白名单 + 类型化值 + 业务键。
type ParsedRow struct {
	Table  string
	Key    string
	Values map[string]any
}

// RowError 行级错误契约（照 controlledintake.RowError）。
type RowError struct {
	Row     int    `json:"row"`
	Column  string `json:"column,omitempty"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// BatchInfo 批次信息（生产实现映射到 operating_fact_batches 行）。
type BatchInfo struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	AcceptedRows int    `json:"accepted_rows"`
	RejectedRows int    `json:"rejected_rows"`
}

// CommitResult 分块落库结果。
type CommitResult struct {
	SavedIDs []string `json:"saved_ids"`
	Replayed bool     `json:"replayed"`
}

// Sink 生产实现的落库端口（repository 实现）。
type Sink interface {
	BeginBatch(ctx context.Context, spec ImportSpec, totalRows int) (*BatchInfo, bool, error)
	FinalizeBatch(ctx context.Context, batchID string, accepted, rejected int, status, errorsJSON string) error
	CommitChunk(ctx context.Context, spec ImportSpec, table string, rows []ParsedRow, chunkKey, payloadSHA string) (*CommitResult, error)
	RegisterInvoiceHeaders(ctx context.Context, spec ImportSpec, invoices []InvoiceHeader) error
}

// InvoiceHeader ad_invoice 来源附带的发票登记（ON CONFLICT DO NOTHING——发票只登记一次）。
type InvoiceHeader struct {
	InvoiceNo     string
	AgentName     *string
	MediaOwner    *string
	PeriodStart   *string
	PeriodEnd     *string
	InvoiceDate   *string
	Currency      string
	GrossAmount   float64
	RebateAmount  float64
	PayableAmount float64
}

// MaxChunkRows 单块行数上限（照 retailingest）。
const MaxChunkRows = 500

// TemplateSummary 模板摘要（拒绝旧版本时告知当前版本，R-E1-4）。
type TemplateSummary struct {
	Source  string   `json:"source"`
	Version string   `json:"version"`
	Grain   string   `json:"grain"`
	Columns []string `json:"columns"`
}

// Result IngestBatch 的返回。
type Result struct {
	Batch            BatchInfo       `json:"batch"`
	Report           ImportReport    `json:"report"`
	IdempotentReplay bool            `json:"idempotent_replay"`
	TemplateVersion  string          `json:"template_version"`
	CurrentTemplate  TemplateSummary `json:"current_template"`
}

// ImportReport 导入报告。
type ImportReport struct {
	Source          string     `json:"source"`
	TemplateVersion string     `json:"template_version"`
	TotalRows       int        `json:"total_rows"`
	AcceptedRows    int        `json:"accepted_rows"`
	FailedRows      int        `json:"failed_rows"`
	Errors          []RowError `json:"errors,omitempty"`
}

// Preview 干跑：解析 + 全部校验，不落一行。供导入前的确认界面使用。
func Preview(spec ImportSpec) (*ImportReport, error) {
	_, prep, err := prepare(spec)
	if err != nil {
		return nil, err
	}
	prep.report.AcceptedRows = len(prep.rows)
	return &prep.report, nil
}

// IngestBatch 受控导入主编排。任何信封/模板失败在写第一行之前返回（fail-closed）。
func IngestBatch(ctx context.Context, spec ImportSpec, sink Sink) (*Result, error) {
	if sink == nil {
		return nil, failure(FailureSystem, "sink 未接线")
	}
	if strings.TrimSpace(spec.IdempotencyKey) == "" {
		return nil, failure(FailureEnvelope, "缺少请求级幂等键（Idempotency-Key）")
	}
	tmpl, prep, err := prepare(spec)
	if err != nil {
		return nil, err
	}
	report := &prep.report

	batch, replay, err := sink.BeginBatch(ctx, spec, report.TotalRows)
	if err != nil {
		return nil, wrapSystem("建批次", err)
	}
	if replay {
		return &Result{
			Batch: *batch,
			Report: ImportReport{Source: string(spec.Source), TemplateVersion: tmpl.Version,
				TotalRows: batch.AcceptedRows + batch.RejectedRows,
				AcceptedRows: batch.AcceptedRows, FailedRows: batch.RejectedRows},
			IdempotentReplay: true,
			TemplateVersion:  tmpl.Version, CurrentTemplate: summaryOf(tmpl),
		}, nil
	}

	savedTotal := 0
	failedTotal := len(report.Errors)
	for start := 0; start < len(prep.rows); start += MaxChunkRows {
		end := start + MaxChunkRows
		if end > len(prep.rows) {
			end = len(prep.rows)
		}
		chunk := prep.rows[start:end]
		chunkKey := fmt.Sprintf("%s#chunk:%d", spec.IdempotencyKey, start/MaxChunkRows)
		res, cerr := sink.CommitChunk(ctx, spec, tmpl.TargetTables[0], chunk, chunkKey, payloadSHA(chunk))
		if cerr != nil {
			status := "failed"
			_ = sink.FinalizeBatch(ctx, batch.ID, savedTotal, failedTotal+len(chunk), status, marshalErrors(report.Errors))
			return nil, wrapSystem("分块落库", cerr)
		}
		savedTotal += len(res.SavedIDs)
	}
	if len(prep.invoices) > 0 && spec.Source == SourceAdInvoice {
		if ierr := sink.RegisterInvoiceHeaders(ctx, spec, prep.invoices); ierr != nil {
			_ = sink.FinalizeBatch(ctx, batch.ID, savedTotal, failedTotal, "failed", marshalErrors(report.Errors))
			return nil, wrapSystem("登记代理发票", ierr)
		}
	}

	status := "completed"
	if failedTotal > 0 && savedTotal == 0 && report.TotalRows > 0 {
		status = "failed"
	}
	if ferr := sink.FinalizeBatch(ctx, batch.ID, savedTotal, failedTotal, status, marshalErrors(report.Errors)); ferr != nil {
		return nil, wrapSystem("终态回写", ferr)
	}
	report.AcceptedRows = savedTotal
	report.FailedRows = failedTotal
	return &Result{
		Batch:           BatchInfo{ID: batch.ID, Status: status, AcceptedRows: savedTotal, RejectedRows: failedTotal},
		Report:          *report,
		TemplateVersion: tmpl.Version, CurrentTemplate: summaryOf(tmpl),
	}, nil
}

// BusinessKey 每个来源的业务键构造（D4）：同一业务键重导以新 fact_version 追加；
// 同文件内重复业务键被 duplicate_in_file 拒绝。业务键定义只有这一份。
func BusinessKey(source Source, v map[string]any) string {
	get := func(k string) string {
		if s, ok := v[k].(string); ok {
			return s
		}
		return ""
	}
	switch source {
	case SourceShopify, Source3PL, SourceProcurement:
		return strings.Join([]string{get("business_date"), get("channel"), get("sku")}, "|")
	case SourceAdsBooked:
		return strings.Join([]string{get("business_date"), get("campaign_id")}, "|")
	case SourceAdInvoice:
		return strings.Join([]string{get("invoice_no"), get("business_date"), get("campaign_id")}, "|")
	case SourceSettlement:
		return strings.Join([]string{get("provider"), get("payout_id")}, "|")
	case SourceBank:
		return get("bank_ref")
	case SourceOrderLines:
		lineNo := "1"
		if n, ok := v["line_no"].(int); ok && n > 0 {
			lineNo = fmt.Sprintf("%d", n)
		}
		return strings.Join([]string{get("platform_order_no"), lineNo}, "|")
	case SourceGLRevenue, SourceOverhead:
		return get("period")
	}
	return ""
}

// prepare 共享校验管线：模板版本 → 信封 → 解析 → 行校验/去重。
func prepare(spec ImportSpec) (TemplateDef, *importPrepare, error) {
	tmpl, ok := Templates[spec.Source]
	if !ok {
		return TemplateDef{}, nil, failure(FailureTemplate, "未知来源 %q", spec.Source)
	}
	current := TemplateVersions[spec.Source]
	if strings.TrimSpace(spec.TemplateVersion) != current {
		return TemplateDef{}, nil, &ImportError{
			Kind:    FailureTemplate,
			Message: fmt.Sprintf("模板版本 %q 已过时，当前版本 %q（来源 %s）", spec.TemplateVersion, current, spec.Source),
			Err:     ErrTemplateVersionRejected,
		}
	}
	if err := spec.Envelope.Validate(); err != nil {
		return TemplateDef{}, nil, &ImportError{Kind: FailureEnvelope, Message: err.Error(), Err: ErrEnvelopeIncomplete}
	}
	if strings.TrimSpace(spec.LegalEntityID) == "" || strings.TrimSpace(spec.StorefrontID) == "" {
		return TemplateDef{}, nil, failure(FailureEnvelope, "legal_entity_id 与 storefront_id 必填")
	}

	headers, records, err := controlledintake.Parse(controlledintake.Source{Filename: spec.Filename, Data: spec.Data})
	if err != nil {
		return TemplateDef{}, nil, failure(FailureParse, "解析失败: %v", err)
	}
	headerIdx := map[string]int{}
	for i, h := range headers {
		headerIdx[strings.ToLower(strings.TrimSpace(h))] = i
	}
	var missingHeaders []string
	for _, c := range tmpl.Columns {
		if c.Required {
			if _, ok := headerIdx[c.Name]; !ok {
				missingHeaders = append(missingHeaders, c.Name)
			}
		}
	}
	// 表头必须完全落在模板白名单内（未知列拒绝——防止静默丢列）
	for h := range headerIdx {
		if !tmpl.HasColumn(h) {
			return TemplateDef{}, nil, failure(FailureParse, "未知列 %q：模板 %s@%s 不包含该列", h, spec.Source, current)
		}
	}
	if len(missingHeaders) > 0 {
		sort.Strings(missingHeaders)
		return TemplateDef{}, nil, failure(FailureParse, "缺少必需列: %s", strings.Join(missingHeaders, ", "))
	}

	prep := &importPrepare{}
	prep.report.Source = string(spec.Source)
	prep.report.TemplateVersion = current
	prep.report.TotalRows = len(records)

	type seenRow struct {
		rowNum int
		values map[string]any
	}
	seenKeys := map[string]seenRow{}
	invoiceSeen := map[string]bool{}

	for i, rec := range records {
		rowNum := i + 2 // 表头占第 1 行
		values := map[string]string{}
		for name, idx := range headerIdx {
			v := ""
			if idx < len(rec) {
				v = strings.TrimSpace(rec[idx])
			}
			values[name] = v
		}
		parsed := map[string]any{}
		rowFailed := false
		failRow := func(code, col, msg string) {
			prep.report.Errors = append(prep.report.Errors, RowError{Row: rowNum, Column: col, Code: code, Message: msg})
			rowFailed = true
		}
		for _, c := range tmpl.Columns {
			raw := values[c.Name]
			if raw == "" {
				switch {
				case c.Required:
					failRow("missing_required_value", c.Name, "必填列为空")
				case c.Default != "":
					parsed[c.Name] = defaultString(c)
				default:
					parsed[c.Name] = nil
				}
				continue
			}
			v, err := convert(c, raw)
			if err != nil {
				failRow("invalid_value", c.Name, err.Error())
				continue
			}
			parsed[c.Name] = v
		}
		if rowFailed {
			continue
		}
		key := BusinessKey(spec.Source, parsed)
		if prev, dup := seenKeys[key]; dup {
			prep.report.Errors = append(prep.report.Errors, RowError{Row: rowNum, Code: "duplicate_in_file",
				Message: fmt.Sprintf("业务键与第 %d 行重复：%s", prev.rowNum, key)})
			continue
		}
		seenKeys[key] = seenRow{rowNum: rowNum, values: parsed}
		prep.rows = append(prep.rows, ParsedRow{Table: tmpl.TargetTables[0], Key: key, Values: parsed})

		if hdr := invoiceHeaderFrom(spec.Source, parsed); hdr != nil && !invoiceSeen[hdr.InvoiceNo] {
			invoiceSeen[hdr.InvoiceNo] = true
			prep.invoices = append(prep.invoices, *hdr)
		}
	}
	return tmpl, prep, nil
}

type importPrepare struct {
	rows     []ParsedRow
	invoices []InvoiceHeader
	report   ImportReport
}

// invoiceHeaderFrom 从 paid 口径行抽取发票头（每个发票号只登记首行）。
func invoiceHeaderFrom(source Source, parsed map[string]any) *InvoiceHeader {
	if source != SourceAdInvoice {
		return nil
	}
	h := &InvoiceHeader{Currency: asString(parsed["currency"])}
	h.InvoiceNo = asString(parsed["invoice_no"])
	if h.InvoiceNo == "" {
		return nil
	}
	h.AgentName = asPtr(parsed["agent_name"])
	h.MediaOwner = asPtr(parsed["media_owner"])
	h.PeriodStart = asPtr(parsed["period_start"])
	h.PeriodEnd = asPtr(parsed["period_end"])
	h.InvoiceDate = asPtr(parsed["invoice_date"])
	h.GrossAmount = asFloat(parsed["gross_amount"])
	h.RebateAmount = asFloat(parsed["rebate_amount"])
	h.PayableAmount = asFloat(parsed["payable_amount"])
	return h
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func asPtr(v any) *string {
	if s, ok := v.(string); ok && s != "" {
		return &s
	}
	return nil
}

func asFloat(v any) float64 {
	if f, ok := v.(float64); ok {
		return f
	}
	return 0
}

func defaultString(c TemplateColumn) string { return c.Default }

func convert(c TemplateColumn, raw string) (any, error) {
	switch c.Kind {
	case KindString:
		return raw, nil
	case KindEnum:
		for _, v := range c.Enum {
			if v == raw {
				return raw, nil
			}
		}
		return nil, fmt.Errorf("取值 %q 不在枚举 %v 内", raw, c.Enum)
	case KindMoney, KindDecimal:
		f, err := parseDecimal(raw)
		if err != nil {
			return nil, fmt.Errorf("非法数值 %q", raw)
		}
		return f, nil
	case KindInt:
		var n int
		if _, err := fmt.Sscanf(raw, "%d", &n); err != nil {
			return nil, fmt.Errorf("非法整数 %q", raw)
		}
		return n, nil
	case KindDate:
		for _, layout := range []string{"2006-01-02", "2006/01/02"} {
			if t, err := time.Parse(layout, raw); err == nil {
				return t.Format(time.DateOnly), nil
			}
		}
		return nil, fmt.Errorf("非法日期 %q（期望 YYYY-MM-DD）", raw)
	}
	return raw, nil
}

func parseDecimal(raw string) (float64, error) {
	var f float64
	raw = strings.ReplaceAll(raw, ",", "")
	if _, err := fmt.Sscanf(raw, "%g", &f); err != nil {
		return 0, err
	}
	return f, nil
}

func payloadSHA(rows []ParsedRow) string {
	hasher := sha256.New()
	for _, r := range rows {
		keys := make([]string, 0, len(r.Values))
		for k := range r.Values {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		fmt.Fprintf(hasher, "%s|%s", r.Table, r.Key)
		for _, k := range keys {
			fmt.Fprintf(hasher, "|%v", r.Values[k])
		}
		fmt.Fprint(hasher, "\n")
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

func marshalErrors(errs []RowError) string {
	if len(errs) == 0 {
		return "[]"
	}
	var b strings.Builder
	b.WriteString("[")
	for i, e := range errs {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `{"row":%d,"column":%q,"code":%q,"message":%q}`, e.Row, e.Column, e.Code, e.Message)
	}
	b.WriteString("]")
	return b.String()
}

func summaryOf(t TemplateDef) TemplateSummary {
	names := make([]string, 0, len(t.Columns))
	for _, c := range t.Columns {
		names = append(names, c.Name)
	}
	return TemplateSummary{Source: string(t.Source), Version: t.Version, Grain: t.Grain, Columns: names}
}

func wrapSystem(stage string, err error) *ImportError {
	return &ImportError{Kind: FailureSystem, Message: stage + "失败", Err: err}
}

// TemplateCSV 生成标准模板下载内容（表头一行；R-E1-4「信封字段一个不少」由请求级
// 信封参数承担，CSV 只含数据列）。CRLF 结尾便于 Excel 直接打开。
func TemplateCSV(source Source) ([]byte, error) {
	tmpl, ok := Templates[source]
	if !ok {
		return nil, failure(FailureTemplate, "未知来源 %q", source)
	}
	names := make([]string, 0, len(tmpl.Columns))
	for _, c := range tmpl.Columns {
		names = append(names, c.Name)
	}
	return []byte(strings.Join(names, ",") + "\r\n"), nil
}
