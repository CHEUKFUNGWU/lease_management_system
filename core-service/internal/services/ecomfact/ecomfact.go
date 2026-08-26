// Package ecomfact 是电商独立站模式的唯一事实读口（模块深化 EM2）。
//
// 「读最新版本」这个决定如果由各消费方自己做，退款 120 天后到达的重述必然有人读了
// v1。所以 Highest Fact Version 解析、修订标记、币种分区、跨法人行级过滤全部藏在本包
// 与其生产实现后面；三个消费方（sitepnl / settlement / ecomkpi）都从 FactReader 取数，
// 谁也不私连事实表。
//
// 两条铁律：
//   - 分析端点 SQL 只触聚合事实表；触订单行表的代码路径必须经过 OrderLines 且携带证据引用。
//   - 本包禁 import ifrs16（P2 的 finmodel Port 反向只读接入；方向是 finmodel 读电商事实），
//     由 importguard_test.go 遍历子包钉住。
package ecomfact

import (
	"context"
	"sort"
	"time"

	"github.com/lease-management-system/core-service/internal/access"
)

// AdBasis 广告口径：booked = 平台后台账面 spend；paid = 代理发票实付。
// 两者是两个事实不是同一事实的两个视图（spec R-T3），不存在第三种取值。
type AdBasis string

const (
	AdBasisBooked AdBasis = "booked"
	AdBasisPaid   AdBasis = "paid"
)

// Valid 拒绝第三种口径——任何聚合若需合并口径必须出新的 Metric Definition Version。
func (b AdBasis) Valid() bool { return b == AdBasisBooked || b == AdBasisPaid }

// Window 事实窗口，[From, To] 双闭。
type Window struct {
	From time.Time
	To   time.Time
}

// Days 返回窗口覆盖的自然日数（含首尾）；From 在 To 之后返回 0。
func (w Window) Days() int {
	if w.To.Before(w.From) {
		return 0
	}
	return int(w.To.Sub(w.From).Hours()/24) + 1
}

// Contains 报告 day 是否落在窗口内（按 UTC 日界比较）。
func (w Window) Contains(day time.Time) bool {
	d := day.UTC().Truncate(24 * time.Hour)
	return !d.Before(w.From.UTC().Truncate(24*time.Hour)) && !d.After(w.To.UTC().Truncate(24*time.Hour))
}

// StorefrontFilter 站点筛选：法人行级过滤是必填语义（底线 1 从第一查询就生效），
// 零值 EntityFilter 视为配置错误，由 access.EntityFilter 自身的 fail-closed 约定兜住。
type StorefrontFilter struct {
	Entity        access.EntityFilter
	StorefrontIDs []string
}

// StorefrontRef 定位一个站点。
type StorefrontRef struct {
	LegalEntityID string
	StorefrontID  string
}

// Envelope 五信封字段的最小读侧投影（写侧完整形状在 intake/repository）。
type Envelope struct {
	SourceSystem            string     `json:"source_system"`
	ImportBatchID           string     `json:"import_batch_id"`
	FactVersion             int        `json:"fact_version"`
	AsOfAt                  time.Time  `json:"as_of_at"`
	DataClassification      string     `json:"data_classification"`
	SimulationDatasetVersion *string   `json:"simulation_dataset_version,omitempty"`
	Restated                bool       `json:"restated"`
}

// StorefrontDayFact 站点 × 日 × 渠道 × SKU 聚合事实（已解析到最高版本的一行）。
// 一切度量可空：缺失就是缺失，不许补 0、不许反推。
type StorefrontDayFact struct {
	StorefrontRef
	BusinessDate time.Time `json:"business_date"`
	Channel      string    `json:"channel"`
	SKU          string    `json:"sku"`
	Currency     string    `json:"currency"`

	GMVAmount          *float64 `json:"gmv_amount,omitempty"`
	DiscountAmount     *float64 `json:"discount_amount,omitempty"`
	RefundAmount       *float64 `json:"refund_amount,omitempty"`
	ChargebackLoss     *float64 `json:"chargeback_loss_amount,omitempty"`
	OrderCount         *int     `json:"order_count,omitempty"`
	NewCustomerOrders  *int     `json:"new_customer_orders,omitempty"`
	LandedCostAmount   *float64 `json:"landed_cost_amount,omitempty"`
	FulfillmentAmount  *float64 `json:"fulfillment_amount,omitempty"`
	PaymentFeeAmount   *float64 `json:"payment_fee_amount,omitempty"`
	TaxCollectedAmount *float64 `json:"tax_collected_amount,omitempty"`

	SourceEnvelope Envelope `json:"source_envelope"`
}

// CampaignDayFact 一个 campaign 一个日期的广告度量，账面/实付两行并存。
type CampaignDayFact struct {
	StorefrontRef
	CampaignID   string    `json:"campaign_id"`
	CampaignName *string   `json:"campaign_name,omitempty"`
	BusinessDate time.Time `json:"business_date"`
	Basis        AdBasis   `json:"basis"`
	MediaOwner   *string   `json:"media_owner,omitempty"`
	SpendAmount  float64   `json:"spend_amount"`
	Impressions  *int64    `json:"impressions,omitempty"`
	Clicks       *int64    `json:"clicks,omitempty"`
	Conversions  *float64  `json:"conversions,omitempty"`
	InvoiceNo    *string   `json:"invoice_no,omitempty"`
	Currency     string    `json:"currency"`

	SourceEnvelope Envelope `json:"source_envelope"`
}

// EvidenceRef 订单行证据引用：只有携带证据引用的路径才允许触订单行表。
type EvidenceRef struct {
	StorefrontID    string `json:"storefront_id"`
	PlatformOrderNo string `json:"platform_order_no"`
}

// OrderLine 订单行明细（裁剪列）：仅供下钻与对账证据，不进分析读路径。
type OrderLine struct {
	PlatformOrderNo string     `json:"platform_order_no"`
	LineNo          int        `json:"line_no"`
	BusinessDate    time.Time  `json:"business_date"`
	Channel         string     `json:"channel"`
	SKU             string     `json:"sku"`
	Quantity        *float64   `json:"quantity,omitempty"`
	GrossAmount     *float64   `json:"gross_amount,omitempty"`
	DiscountAmount  *float64   `json:"discount_amount,omitempty"`
	RefundAmount    *float64   `json:"refund_amount,omitempty"`
	TaxAmount       *float64   `json:"tax_amount,omitempty"`
	Currency        string     `json:"currency"`
	PayoutID        *string    `json:"payout_id,omitempty"`
	Evidence        map[string]any `json:"evidence,omitempty"`

	Envelope Envelope `json:"envelope"`
}

// FactReader 唯一事实缝：sitepnl / settlement / ecomkpi 全部经它取数。
// 生产实现负责 Highest Fact Version 解析与跨法人行级过滤。
type FactReader interface {
	StorefrontDays(ctx context.Context, f StorefrontFilter, w Window) ([]StorefrontDayFact, error)
	CampaignDays(ctx context.Context, f StorefrontFilter, w Window, basis AdBasis) ([]CampaignDayFact, error)
	OrderLines(ctx context.Context, ref EvidenceRef) ([]OrderLine, error)
}

// Versioned 可按业务键选最高版本的对象的最小视图。
type Versioned interface {
	VersionKey() string
	Version() int
}

// Highest 按 VersionKey 分组取 fact_version 最大的一行——版本选择只有这一份实现。
// 同键同版本的并列行按输入顺序保留第一行（写侧唯一键保证不会出现）。
func Highest(items []Versioned) []Versioned {
	if len(items) == 0 {
		return nil
	}
	best := make(map[string]Versioned, len(items))
	order := make([]string, 0, len(items))
	for _, it := range items {
		key := it.VersionKey()
		cur, ok := best[key]
		if !ok {
			best[key] = it
			order = append(order, key)
			continue
		}
		if it.Version() > cur.Version() {
			best[key] = it
		}
	}
	out := make([]Versioned, 0, len(order))
	for _, key := range order {
		out = append(out, best[key])
	}
	sort.Slice(out, func(i, j int) bool { return out[i].VersionKey() < out[j].VersionKey() })
	return out
}
