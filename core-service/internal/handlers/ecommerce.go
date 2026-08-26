// 电商独立站模式 HTTP 面（EM11 的 API 半边）：
//
//	GET  /ecom/sites                        站点列表
//	POST /ecom/sites                        新建站点（master_data manage）
//	GET  /ecom/site-pulse                   站点脉搏（净收入/CM%/MER vs 前窗 + top 差异因子）
//	GET  /ecom/sites/:id/diagnostics        站点诊断（KPI + CAC + 保本）
//	GET  /ecom/sites/:id/pnl                单站利润表
//	GET  /ecom/sites/:id/reserve            滚动准备金头寸
//	GET/POST /ecom/settlement/runs          对账 run 列表 / 创建并匹配
//	GET  /ecom/settlement/runs/:id          run 详情
//	POST /ecom/settlement/runs/:id/transition 签认状态机推进
//	GET  /ecom/import/templates[:source]    标准模板清单 / CSV 下载
//	POST /ecom/import/preview|commit        受控导入
//	POST /ecom/scenarios/bfcm               大促保本情景（simulated）
//	POST /ecom/scenarios/price-sensitivity  定价敏感度（simulated）
//
// 前端零计算：所有行、合计、评分来自后端；口径冲突只降级不换算。
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/errcontract"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/controlledintake"
	"github.com/lease-management-system/core-service/internal/services/ecomfact"
	"github.com/lease-management-system/core-service/internal/services/ecomintake"
	"github.com/lease-management-system/core-service/internal/services/ecomkpi"
	"github.com/lease-management-system/core-service/internal/services/ecompulse"
	"github.com/lease-management-system/core-service/internal/services/ecomsim"
	"github.com/lease-management-system/core-service/internal/services/settlement"
	"github.com/lease-management-system/core-service/internal/services/sitepnl"
	"github.com/lease-management-system/core-service/internal/services/unitecon"
)

// EcommerceHandler 电商域 handler。governance 为 Data Quality Queue 写入口（可空时
// 差异只落 run 不进队列——构造期应保证非空）。
type EcommerceHandler struct {
	repo       *repository.EcommerceRepository
	governance *repository.FPnAGovernanceRepository
	now        func() time.Time
}

// NewEcommerceHandler 构造。
func NewEcommerceHandler(repo *repository.EcommerceRepository, governance *repository.FPnAGovernanceRepository) *EcommerceHandler {
	return &EcommerceHandler{repo: repo, governance: governance, now: time.Now}
}

func (h *EcommerceHandler) entity(c *gin.Context) access.EntityFilter {
	tenant := strings.TrimSpace(middleware.GetTenantID(c))
	if tenant == "" {
		return access.GlobalEntityFilter()
	}
	entity, err := access.EntityFilterFor(tenant)
	if err != nil {
		return access.GlobalEntityFilter()
	}
	return entity
}

func (h *EcommerceHandler) userID(c *gin.Context) *string {
	if id := strings.TrimSpace(userIDFromContext(c)); id != "" {
		return &id
	}
	return nil
}

// ecomEnvelope 读响应的来源信封（电商版最小形状）。
type ecomEnvelope struct {
	DataClassification string    `json:"data_classification"`
	SourceSystems      []string  `json:"source_systems"`
	FactVersionMin     int       `json:"fact_version_min"`
	FactVersionMax     int       `json:"fact_version_max"`
	HighestAsOf        *time.Time `json:"highest_as_of,omitempty"`
	SemanticVersion    string    `json:"semantic_version"`
	GeneratedAt        time.Time `json:"generated_at"`
}

func buildEcomEnvelope(facts []ecomfact.StorefrontDayFact) ecomEnvelope {
	env := ecomEnvelope{SemanticVersion: "ecom-kpi-v1", GeneratedAt: time.Now().UTC(), FactVersionMin: -1}
	srcSet := map[string]bool{}
	for _, f := range facts {
		if !srcSet[f.SourceEnvelope.SourceSystem] {
			srcSet[f.SourceEnvelope.SourceSystem] = true
			env.SourceSystems = append(env.SourceSystems, f.SourceEnvelope.SourceSystem)
		}
		if env.DataClassification == "" {
			env.DataClassification = f.SourceEnvelope.DataClassification
		} else if env.DataClassification != f.SourceEnvelope.DataClassification {
			env.DataClassification = "mixed"
		}
		if env.FactVersionMin < 0 || f.SourceEnvelope.FactVersion < env.FactVersionMin {
			env.FactVersionMin = f.SourceEnvelope.FactVersion
		}
		if f.SourceEnvelope.FactVersion > env.FactVersionMax {
			env.FactVersionMax = f.SourceEnvelope.FactVersion
		}
		if env.HighestAsOf == nil || f.SourceEnvelope.AsOfAt.After(*env.HighestAsOf) {
			t := f.SourceEnvelope.AsOfAt
			env.HighestAsOf = &t
		}
	}
	sort.Strings(env.SourceSystems)
	if env.FactVersionMin < 0 {
		env.FactVersionMin = 0
	}
	if env.DataClassification == "" {
		env.DataClassification = "unknown"
	}
	return env
}

// classificationParam 解析显式的 data_classification 参数（缺省 production；
// 非法值报 invalid_arguments——不许静默混读两类数据）。
func classificationParam(c *gin.Context) (string, bool) {
	classification := strings.TrimSpace(c.Query("data_classification"))
	if classification == "" {
		classification = "production"
	}
	if classification != "production" && classification != "simulated" {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments,
			"data_classification 仅允许 production|simulated", nil)
		return "", false
	}
	return classification, true
}

func datasetVersionParam(c *gin.Context, classification string) (string, bool) {
	version := strings.TrimSpace(c.Query("dataset_version"))
	if classification == "simulated" && version == "" {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments,
			"simulated 数据必须带 dataset_version", nil)
		return "", false
	}
	return version, true
}

// ListStorefronts GET /ecom/sites
func (h *EcommerceHandler) ListStorefronts(c *gin.Context) {
	sites, err := h.repo.ListStorefronts(c.Request.Context(), h.entity(c))
	if err != nil {
		writeSystemFailure(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": sites})
}

// CreateStorefront POST /ecom/sites
func (h *EcommerceHandler) CreateStorefront(c *gin.Context) {
	var req struct {
		LegalEntityID string `json:"legal_entity_id"`
		Code          string `json:"code"`
		Name          string `json:"name"`
		Market        string `json:"market"`
		Currency      string `json:"currency"`
		Platform      string `json:"platform"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "请求体不合法", nil)
		return
	}
	if req.LegalEntityID == "" {
		req.LegalEntityID = middleware.GetTenantID(c)
	}
	if req.Code == "" || req.Name == "" || len(req.Currency) != 3 {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "code/name/currency 必填（currency 三字母）", nil)
		return
	}
	site, err := h.repo.CreateStorefront(c.Request.Context(), &repository.Storefront{
		LegalEntityID: req.LegalEntityID, Code: req.Code, Name: req.Name,
		Market: req.Market, Currency: strings.ToUpper(req.Currency), Platform: req.Platform,
	})
	if err != nil {
		writeSystemFailure(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusCreated, site)
}

// parseWindow 解析 as_of + window_days 为 [From, To] 与对比窗口（等长前移）。
func (h *EcommerceHandler) parseWindow(c *gin.Context) (cur, prev ecomfact.Window, ok bool) {
	asOf := strings.TrimSpace(c.Query("as_of"))
	windowDays := 7
	if raw := strings.TrimSpace(c.Query("window_days")); raw != "" {
		if _, err := fmt.Sscanf(raw, "%d", &windowDays); err != nil || windowDays <= 0 || windowDays > 366 {
			writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "window_days 必须是 1..366", nil)
			return cur, prev, false
		}
	}
	end := h.now().UTC().Truncate(24 * time.Hour).AddDate(0, 0, -1) // 默认截至昨天
	if asOf != "" {
		parsed, err := time.Parse(time.DateOnly, asOf)
		if err != nil {
			writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "as_of 需要 YYYY-MM-DD", nil)
			return cur, prev, false
		}
		end = parsed.UTC()
	}
	start := end.AddDate(0, 0, -(windowDays - 1))
	cur = ecomfact.Window{From: start, To: end}
	prev = ecomfact.Window{From: start.AddDate(0, 0, -windowDays), To: start.AddDate(0, 0, -1)}
	return cur, prev, true
}

// SitePulse GET /ecom/site-pulse —— 周一 9:00 场景：三站一页，净收入 / CM% / MER vs 前窗，
// top 3 差异因子；全程币种分区，覆盖不足显式降级 DecisionReady=false。
// 组装逻辑在 services/ecompulse（HTTP 与 Agent 工具两个消费方共用）。
func (h *EcommerceHandler) SitePulse(c *gin.Context) {
	classification, ok := classificationParam(c)
	if !ok {
		return
	}
	datasetVersion, ok := datasetVersionParam(c, classification)
	if !ok {
		return
	}
	curWin, prevWin, ok := h.parseWindow(c)
	if !ok {
		return
	}
	pulse, err := ecompulse.Compute(c.Request.Context(), h.repo, h.entity(c), classification, datasetVersion, curWin, prevWin)
	if err != nil {
		writeSystemFailure(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, pulse)
}

// SiteDiagnostics GET /ecom/sites/:id/diagnostics —— KPI 全集 + CAC 双口径 + 保本 MER/ROAS。
func (h *EcommerceHandler) SiteDiagnostics(c *gin.Context) {
	classification, ok := classificationParam(c)
	if !ok {
		return
	}
	datasetVersion, ok := datasetVersionParam(c, classification)
	if !ok {
		return
	}
	curWin, _, ok := h.parseWindow(c)
	if !ok {
		return
	}
	entity := h.entity(c)
	site, err := h.repo.GetStorefront(c.Request.Context(), entity, c.Param("id"))
	if err == repository.ErrEcomNotFound {
		writeCodedError(c, http.StatusNotFound, errcontract.CodeNotFound, "site not found", nil)
		return
	}
	if err != nil {
		writeSystemFailure(c, http.StatusInternalServerError, err)
		return
	}
	filter := ecomfact.StorefrontFilter{Entity: entity, StorefrontIDs: []string{site.ID}}
	facts, err := h.repo.StorefrontDays(c.Request.Context(), filter, curWin)
	if err != nil {
		writeSystemFailure(c, http.StatusInternalServerError, err)
		return
	}
	facts = ecompulse.FilterStorefrontClassification(facts, classification, datasetVersion)
	adsPaid, err := h.repo.CampaignDays(c.Request.Context(), filter, curWin, ecomfact.AdBasisPaid)
	if err != nil {
		writeSystemFailure(c, http.StatusInternalServerError, err)
		return
	}
	adsPaid = ecompulse.FilterCampaignClassification(adsPaid, classification, datasetVersion)

	allCodes := []string{"gmv", "discounts", "refunds", "chargeback_losses", "net_revenue", "order_count",
		"new_customer_orders", "aov", "landed_cost", "fulfillment_cost", "payment_fee", "tax_collected",
		"cm1", "cm1_rate", "ad_spend_paid", "cm2", "cm2_rate", "mer", "roas", "refund_rate"}
	partitions, coverage := ecomkpi.EvaluateByCurrency(allCodes, facts, adsPaid, curWin)

	// CAC 与保本按主币种（站点申报币种优先，否则字典序首个分区）计算。
	currency := site.Currency
	found := false
	for _, p := range partitions {
		if p.Currency == currency {
			found = true
			break
		}
	}
	if !found && len(partitions) > 0 {
		currency = partitions[0].Currency
	}

	cacReport := uniteconCACView(factsInCurrency(facts, currency), adsPaid, currency)
	fixedCost, _ := h.repo.LatestFixedCost(c.Request.Context(), entity, site.ID, periodOfMonth(curWin.To))
	breakEven := uniteconBreakEven(factsInCurrency(facts, currency), fixedCost, currency)

	decisionReady := !ecomkpi.CoverageIncomplete(coverage) && len(facts) > 0
	c.JSON(http.StatusOK, gin.H{
		"envelope":       buildEcomEnvelope(facts),
		"storefront":     site,
		"window":         gin.H{"from": curWin.From.Format(time.DateOnly), "to": curWin.To.Format(time.DateOnly)},
		"currency":       currency,
		"kpis":           partitions,
		"coverage":       coverage,
		"decision_ready": decisionReady,
		"cac":            cacReport,
		"break_even":     breakEven,
	})
}

func periodOfMonth(t time.Time) string { return t.UTC().Format("2006-01") }

func factsInCurrency(facts []ecomfact.StorefrontDayFact, currency string) []ecomfact.StorefrontDayFact {
	out := make([]ecomfact.StorefrontDayFact, 0)
	for _, f := range facts {
		if f.Currency == currency {
			out = append(out, f)
		}
	}
	return out
}

// SitePnl GET /ecom/sites/:id/pnl?period=YYYY-MM | from&to&breakdown=
func (h *EcommerceHandler) SitePnl(c *gin.Context) {
	classification, ok := classificationParam(c)
	if !ok {
		return
	}
	if _, ok := datasetVersionParam(c, classification); !ok {
		return
	}
	entity := h.entity(c)
	site, err := h.repo.GetStorefront(c.Request.Context(), entity, c.Param("id"))
	if err == repository.ErrEcomNotFound {
		writeCodedError(c, http.StatusNotFound, errcontract.CodeNotFound, "site not found", nil)
		return
	}
	if err != nil {
		writeSystemFailure(c, http.StatusInternalServerError, err)
		return
	}

	req := sitepnl.SitePnlRequest{
		Storefront: ecomfact.StorefrontRef{LegalEntityID: site.LegalEntityID, StorefrontID: site.ID},
		Currency:   strings.TrimSpace(c.Query("currency")),
		Breakdown:  sitepnl.Breakdown(strings.TrimSpace(orDefault(c.Query("breakdown"), "none"))),
	}
	if month := strings.TrimSpace(c.Query("period")); month != "" {
		req.Period = sitepnl.Period{Kind: sitepnl.PeriodMonthly, Month: month}
	} else if fromRaw, toRaw := strings.TrimSpace(c.Query("from")), strings.TrimSpace(c.Query("to")); fromRaw != "" && toRaw != "" {
		from, err1 := time.Parse(time.DateOnly, fromRaw)
		to, err2 := time.Parse(time.DateOnly, toRaw)
		if err1 != nil || err2 != nil {
			writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "period 或 from/to 需要合法日期", nil)
			return
		}
		req.Period = sitepnl.Period{Kind: sitepnl.PeriodWeekly, From: from, To: to}
	} else {
		req.Period = sitepnl.Period{Kind: sitepnl.PeriodMonthly, Month: periodOfMonth(h.now())}
	}
	if raw := strings.TrimSpace(c.Query("target_profit")); raw != "" {
		var target float64
		if _, err := fmt.Sscanf(raw, "%g", &target); err == nil {
			req.TargetProfit = &target
		}
	}

	stmt, err := sitepnl.Project(c.Request.Context(), req, sitepnl.Readers{
		Facts: h.repo,
		GL:    &ecomGLReader{repo: h.repo, entity: entity},
		Fixed: &ecomFixedReader{repo: h.repo, entity: entity},
	})
	if err != nil {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, err.Error(), nil)
		return
	}
	c.JSON(http.StatusOK, stmt)
}

// ecomGLReader sitepnl.GLRevenueReader 生产适配：会计收入行唯一来源是 GL 导入（R-E3-5）。
type ecomGLReader struct {
	repo   *repository.EcommerceRepository
	entity access.EntityFilter
}

func (a *ecomGLReader) GLRevenue(ctx context.Context, ref ecomfact.StorefrontRef, period sitepnl.Period) (*sitepnl.GLRevenue, error) {
	if period.Kind != sitepnl.PeriodMonthly && period.Kind != "" {
		// 周度利润表没有周度 GL 口径——诚实返回未导入，由投影降级为 gl_unavailable。
		return nil, nil
	}
	row, err := a.repo.LatestGLRevenue(ctx, a.entity, ref.StorefrontID, period.Month)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, nil
	}
	return &sitepnl.GLRevenue{
		Amount: row.Amount, Currency: row.Currency, SourceSystem: row.SourceSystem,
		ImportBatchID: row.ImportBatchID, FactVersion: row.FactVersion, AsOfAt: row.AsOfAt,
	}, nil
}

// ecomFixedReader sitepnl.FixedCostReader 生产适配。
type ecomFixedReader struct {
	repo   *repository.EcommerceRepository
	entity access.EntityFilter
}

func (a *ecomFixedReader) FixedCost(ctx context.Context, ref ecomfact.StorefrontRef, period sitepnl.Period) (*sitepnl.FixedCost, error) {
	if period.Kind != sitepnl.PeriodMonthly && period.Kind != "" {
		return nil, nil
	}
	row, err := a.repo.LatestFixedCost(ctx, a.entity, ref.StorefrontID, period.Month)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, nil
	}
	return &sitepnl.FixedCost{Amount: row.Amount, Currency: row.Currency, SourceSystem: row.SourceSystem}, nil
}

// uniteconCACView 站点级 CAC 双口径（分子分母显式，R-E3-4）。
func uniteconCACView(facts []ecomfact.StorefrontDayFact, ads []ecomfact.CampaignDayFact, currency string) unitecon.CACReport {
	var spend float64
	seen := false
	for _, a := range ecomfact.HighestCampaignDays(ads) {
		if a.Currency == currency {
			spend += a.SpendAmount
			seen = true
		}
	}
	input := unitecon.CACInput{}
	if seen {
		s := math.Round(spend*100) / 100
		input.AdSpendPaid = &s
	}
	newCustomers := 0
	newOK := len(facts) > 0
	orders := 0
	orderOK := len(facts) > 0
	for _, f := range facts {
		if f.NewCustomerOrders == nil {
			newOK = false
		} else {
			newCustomers += *f.NewCustomerOrders
		}
		if f.OrderCount == nil {
			orderOK = false
		} else {
			orders += *f.OrderCount
		}
	}
	if newOK {
		input.PayingNewCustomers = &newCustomers
	}
	if orderOK {
		input.TotalOrders = &orders
	}
	return unitecon.CACView(input)
}

// uniteconBreakEven 站点级保本：CM1 率来自事实聚合，固定费来自分摊固定费表。
func uniteconBreakEven(facts []ecomfact.StorefrontDayFact, fixed *repository.FixedCostRow, currency string) unitecon.BreakEvenResult {
	if fixed == nil || fixed.Amount == nil || fixed.Currency != currency {
		return unitecon.BreakEvenResult{Status: unitecon.StatusUnachievable, Reason: "fixed_cost_missing"}
	}
	netRevenue, cm1 := 0.0, 0.0
	for _, f := range ecomfact.HighestStorefrontDays(facts) {
		if f.GMVAmount == nil || f.DiscountAmount == nil || f.RefundAmount == nil ||
			f.ChargebackLoss == nil || f.LandedCostAmount == nil || f.FulfillmentAmount == nil || f.PaymentFeeAmount == nil {
			return unitecon.BreakEvenResult{Status: unitecon.StatusUnachievable, Reason: "cm1_components_missing"}
		}
		nr := *f.GMVAmount - *f.DiscountAmount - *f.RefundAmount - *f.ChargebackLoss
		netRevenue += nr
		cm1 += nr - *f.LandedCostAmount - *f.FulfillmentAmount - *f.PaymentFeeAmount
	}
	if netRevenue == 0 {
		return unitecon.BreakEvenResult{Status: unitecon.StatusUnachievable, Reason: "net_revenue_is_zero"}
	}
	return unitecon.BreakEven(cm1/netRevenue, *fixed.Amount, 0)
}


// ReservePosition GET /ecom/sites/:id/reserve —— PayPal 滚动准备金占用与释放（R-E4-2）。
func (h *EcommerceHandler) ReservePosition(c *gin.Context) {
	entity := h.entity(c)
	site, err := h.repo.GetStorefront(c.Request.Context(), entity, c.Param("id"))
	if err == repository.ErrEcomNotFound {
		writeCodedError(c, http.StatusNotFound, errcontract.CodeNotFound, "site not found", nil)
		return
	}
	if err != nil {
		writeSystemFailure(c, http.StatusInternalServerError, err)
		return
	}
	events, err := h.repo.ListReserveEvents(c.Request.Context(), entity, site.ID)
	if err != nil {
		writeSystemFailure(c, http.StatusInternalServerError, err)
		return
	}
	reserveEvents := make([]settlement.ReserveEvent, 0, len(events))
	eventJSON := make([]repository.ReserveEventRow, 0, len(events))
	for _, e := range events {
		reserveEvents = append(reserveEvents, settlement.ReserveEvent{
			EventType: e.EventType, EventDate: e.EventDate, Currency: e.Currency,
			Amount: e.Amount, PayoutID: derefStr(e.PayoutID), HoldEventID: derefStr(e.HoldEventID),
		})
		eventJSON = append(eventJSON, e)
	}
	c.JSON(http.StatusOK, gin.H{
		"events":    eventJSON,
		"positions": settlement.ReservePositions(reserveEvents),
	})
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// ListSettlementRuns GET /ecom/settlement/runs?storefront_id=&period=
func (h *EcommerceHandler) ListSettlementRuns(c *gin.Context) {
	runs, err := h.repo.ListSettlementRuns(c.Request.Context(), h.entity(c),
		strings.TrimSpace(c.Query("storefront_id")), strings.TrimSpace(c.Query("period")))
	if err != nil {
		writeSystemFailure(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": runs})
}

// GetSettlementRun GET /ecom/settlement/runs/:id
func (h *EcommerceHandler) GetSettlementRun(c *gin.Context) {
	run, err := h.repo.GetSettlementRun(c.Request.Context(), h.entity(c), c.Param("id"))
	if err == repository.ErrEcomNotFound {
		writeCodedError(c, http.StatusNotFound, errcontract.CodeNotFound, "settlement run not found", nil)
		return
	}
	if err != nil {
		writeSystemFailure(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, run)
}

// CreateSettlementRun POST /ecom/settlement/runs —— 匹配 + 门禁裁决 + 差异入队（R-E4-1/3）。
func (h *EcommerceHandler) CreateSettlementRun(c *gin.Context) {
	idempotencyKey := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if idempotencyKey == "" {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "缺少 Idempotency-Key", nil)
		return
	}
	var req struct {
		StorefrontID string `json:"storefront_id"`
		Period       string `json:"period"` // YYYY-MM
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Period) != 7 || strings.TrimSpace(req.StorefrontID) == "" {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "storefront_id 与 period(YYYY-MM) 必填", nil)
		return
	}
	entity := h.entity(c)
	site, err := h.repo.GetStorefront(c.Request.Context(), entity, strings.TrimSpace(req.StorefrontID))
	if err == repository.ErrEcomNotFound {
		writeCodedError(c, http.StatusNotFound, errcontract.CodeNotFound, "site not found", nil)
		return
	}
	if err != nil {
		writeSystemFailure(c, http.StatusInternalServerError, err)
		return
	}

	from, _ := time.Parse("2006-01", req.Period)
	to := from.AddDate(0, 1, 0).AddDate(0, 0, -1)

	payoutRows, err := h.repo.ListPayoutLines(c.Request.Context(), entity, site.ID, from, to)
	if err != nil {
		writeSystemFailure(c, http.StatusInternalServerError, err)
		return
	}
	receivableRows, err := h.repo.ListReceivablesByPayout(c.Request.Context(), entity, site.ID, from, to)
	if err != nil {
		writeSystemFailure(c, http.StatusInternalServerError, err)
		return
	}
	bankRows, err := h.repo.ListBankLines(c.Request.Context(), entity, site.ID, from, to)
	if err != nil {
		writeSystemFailure(c, http.StatusInternalServerError, err)
		return
	}

	payouts := make([]settlement.PayoutLine, 0, len(payoutRows))
	for _, p := range payoutRows {
		payouts = append(payouts, settlement.PayoutLine{
			Provider: p.Provider, PayoutID: p.PayoutID, PayoutDate: p.PayoutDate, Currency: p.Currency,
			GrossAmount: p.GrossAmount, FeeAmount: p.FeeAmount, RefundAmount: p.RefundAmount,
			ChargebackAmount: p.ChargebackAmount, FXAmount: p.FXAmount, AdjustmentAmount: p.AdjustmentAmount,
			ReserveHoldAmount: p.ReserveHoldAmount, ReserveReleaseAmount: p.ReserveReleaseAmount,
			NetAmount: p.NetAmount,
		})
	}
	receivables := make([]settlement.ReceivableLine, 0, len(receivableRows))
	for _, rrow := range receivableRows {
		receivables = append(receivables, settlement.ReceivableLine{
			PayoutID: rrow.PayoutID, Currency: rrow.Currency, Amount: rrow.Amount,
		})
	}
	banks := make([]settlement.BankLine, 0, len(bankRows))
	for _, b := range bankRows {
		banks = append(banks, settlement.BankLine{BankRef: b.BankRef, ValueDate: b.ValueDate, Currency: b.Currency, Amount: b.Amount})
	}

	results := settlement.Match(payouts, receivables, banks, settlement.MatchPolicy{})
	verdict := settlement.ApprovalGate(req.Period, results)

	resultsJSON, _ := json.Marshal(results)
	diffJSON := marshalDifferences(results)
	run := &repository.SettlementRun{
		LegalEntityID: site.LegalEntityID, StorefrontID: site.ID, Period: req.Period,
		Currency: site.Currency, PolicyVersion: settlement.PolicyVersion,
		GateVerdict: strPtrVal(string(verdict.Verdict)),
		MatchedCount: countMatched(results), DifferenceCount: verdict.DifferenceCount,
		TotalDifferenceAmount: verdict.TotalDifference,
		Results: resultsJSON, Differences: diffJSON,
		CreatedBy: h.userID(c), IdempotencyKey: strPtrVal(idempotencyKey),
	}
	created, _, err := h.repo.CreateSettlementRun(c.Request.Context(), run)
	if err != nil {
		writeSystemFailure(c, http.StatusInternalServerError, err)
		return
	}
	h.enqueueSettlementDifferences(c, site, req.Period, verdict)
	c.JSON(http.StatusCreated, created)
}

// TransitionSettlementRun POST /ecom/settlement/runs/:id/transition —— Draft→Prepare→Pending→Approved/Rejected。
func (h *EcommerceHandler) TransitionSettlementRun(c *gin.Context) {
	var req struct {
		Action string `json:"action"` // prepare | submit | approve | reject
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Action == "" {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "action 必填", nil)
		return
	}
	actor := userIDFromContext(c)
	run, err := h.repo.TransitionSettlementRun(c.Request.Context(), h.entity(c), c.Param("id"), req.Action, actor, req.Reason)
	switch err {
	case nil:
	case repository.ErrEcomNotFound:
		writeCodedError(c, http.StatusNotFound, errcontract.CodeNotFound, "settlement run not found", nil)
		return
	default:
		if strings.Contains(err.Error(), "不允许") || strings.Contains(err.Error(), "门禁") {
			writeCodedError(c, http.StatusConflict, errcontract.CodeBusinessFailure, err.Error(), nil)
			return
		}
		writeSystemFailure(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, run)
}

// enqueueSettlementDifferences 差异自动进 Data Quality Queue（R-E4-3：
// 「进队列才有人处理」——单人团队里「有人会看到红色」不成立）。
func (h *EcommerceHandler) enqueueSettlementDifferences(c *gin.Context, site *repository.Storefront, period string, verdict settlement.GateVerdict) {
	if h.governance == nil || verdict.Verdict != "deny" {
		return
	}
	evidence, _ := json.Marshal(map[string]any{
		"difference_count": verdict.DifferenceCount,
		"by_category":      verdict.ByCategory,
		"reasons":          verdict.Reasons,
	})
	_, _ = h.governance.CreateDataQuality(c.Request.Context(), &repository.FPnADataQualityItem{
		ID:            "",
		LegalEntityID: strPtrVal(site.LegalEntityID),
		Period:        period,
		Dimension:     "storefront",
		Category:      "settlement_difference",
		Severity:      "high",
		SourceTable:   "settlement_runs",
		SourceRecordID: derefStr(nil),
		Description: fmt.Sprintf("站点 %s %s 收款对账未对平：%d 条差异，合计 %.2f %s",
			site.Code, period, verdict.DifferenceCount, verdict.TotalDifference, site.Currency),
		Status:     "open",
		Evidence:   evidence,
	})
}

// GenerateSimulation POST /ecom/simulations/generate —— 固定 seed 电商模拟数据集
// （D15：无设计伙伴时复演经营场景；simulated 标识贯穿，结论 unvalidated）。
func (h *EcommerceHandler) GenerateSimulation(c *gin.Context) {
	tenant := strings.TrimSpace(middleware.GetTenantID(c))
	if tenant == "" {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "需要法人上下文", nil)
		return
	}
	err := h.repo.SeedEcomSimulatedData(c.Request.Context(), tenant, h.userID(c))
	switch err {
	case nil:
		c.JSON(http.StatusCreated, gin.H{"data_classification": "simulated", "dataset_version": "ecom-sim-v1", "seeded": true})
	case repository.ErrEcomAlreadySeeded:
		c.JSON(http.StatusOK, gin.H{"data_classification": "simulated", "dataset_version": "ecom-sim-v1", "seeded": false, "note": "already seeded"})
	default:
		writeSystemFailure(c, http.StatusInternalServerError, err)
	}
}

// --- 导入 ---

// ImportTemplates GET /ecom/import/templates —— 全部标准模板清单。
func (h *EcommerceHandler) ImportTemplates(c *gin.Context) {
	list := make([]ecomintake.TemplateSummary, 0, len(ecomintake.Templates))
	sources := make([]ecomintake.Source, 0, len(ecomintake.Templates))
	for s := range ecomintake.Templates {
		sources = append(sources, s)
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i] < sources[j] })
	for _, s := range sources {
		t := ecomintake.Templates[s]
		names := make([]string, 0, len(t.Columns))
		for _, col := range t.Columns {
			names = append(names, col.Name)
		}
		list = append(list, ecomintake.TemplateSummary{Source: string(s), Version: t.Version, Grain: t.Grain, Columns: names})
	}
	c.JSON(http.StatusOK, gin.H{"data": list})
}

// DownloadImportTemplate GET /ecom/import/templates/:source —— CSV 模板下载。
func (h *EcommerceHandler) DownloadImportTemplate(c *gin.Context) {
	data, err := ecomintake.TemplateCSV(ecomintake.Source(c.Param("source")))
	if err != nil {
		writeCodedError(c, http.StatusNotFound, errcontract.CodeNotFound, "未知来源模板", nil)
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=ecom-%s-template-v%s.csv",
		c.Param("source"), ecomintake.TemplateVersions[ecomintake.Source(c.Param("source"))]))
	c.Data(http.StatusOK, "text/csv; charset=utf-8", data)
}

func (h *EcommerceHandler) parseImportForm(c *gin.Context) (ecomintake.ImportSpec, bool) {
	source := ecomintake.Source(strings.TrimSpace(c.PostForm("source")))
	if _, known := ecomintake.Templates[source]; !known {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "未知 source", nil)
		return ecomintake.ImportSpec{}, false
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "缺少 file", nil)
		return ecomintake.ImportSpec{}, false
	}
	f, err := fileHeader.Open()
	if err != nil {
		writeSystemFailure(c, http.StatusBadRequest, err)
		return ecomintake.ImportSpec{}, false
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 10<<20))
	if err != nil {
		writeSystemFailure(c, http.StatusBadRequest, err)
		return ecomintake.ImportSpec{}, false
	}
	spec := ecomintake.ImportSpec{
		LegalEntityID:   middleware.GetTenantID(c),
		StorefrontID:    strings.TrimSpace(c.PostForm("storefront_id")),
		UserID:          h.userID(c),
		Filename:        fileHeader.Filename,
		Data:            data,
		Format:          controlledFormat(fileHeader.Filename),
		Source:          source,
		TemplateVersion: strings.TrimSpace(c.PostForm("template_version")),
		IdempotencyKey:  strings.TrimSpace(c.GetHeader("Idempotency-Key")),
	}
	asOfRaw := strings.TrimSpace(c.PostForm("as_of_at"))
	asOf := h.now().UTC()
	if asOfRaw != "" {
		if t, err := time.Parse(time.RFC3339, asOfRaw); err == nil {
			asOf = t
		} else if t, err := time.Parse(time.DateOnly, asOfRaw); err == nil {
			asOf = t
		} else {
			writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "as_of_at 需要 RFC3339 或 YYYY-MM-DD", nil)
			return ecomintake.ImportSpec{}, false
		}
	}
	spec.Envelope = ecomintake.EnvelopeSpec{
		SourceSystem:             strings.TrimSpace(c.PostForm("source_system")),
		AsOfAt:                   asOf,
		DataClassification:       strings.TrimSpace(c.PostForm("data_classification")),
		SimulationDatasetVersion: strings.TrimSpace(c.PostForm("dataset_version")),
	}
	return spec, true
}

func controlledFormat(filename string) controlledintake.Format {
	if strings.HasSuffix(strings.ToLower(filename), ".xlsx") {
		return controlledintake.FormatXLSX
	}
	return controlledintake.FormatCSV
}

// PreviewImport POST /ecom/import/preview
func (h *EcommerceHandler) PreviewImport(c *gin.Context) {
	spec, ok := h.parseImportForm(c)
	if !ok {
		return
	}
	report, err := ecomintake.Preview(spec)
	if err != nil {
		writeImportError(c, err)
		return
	}
	c.JSON(http.StatusOK, report)
}

// CommitImport POST /ecom/import/commit —— 受控批次导入（信封 fail-closed + 双层幂等）。
func (h *EcommerceHandler) CommitImport(c *gin.Context) {
	spec, ok := h.parseImportForm(c)
	if !ok {
		return
	}
	result, err := ecomintake.IngestBatch(c.Request.Context(), spec, h.repo)
	if err != nil {
		writeImportError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func writeImportError(c *gin.Context, err error) {
	importErr, ok := err.(*ecomintake.ImportError)
	if !ok {
		writeSystemFailure(c, http.StatusInternalServerError, err)
		return
	}
	switch importErr.Kind {
	case ecomintake.FailureTemplate:
		writeCodedError(c, http.StatusUnprocessableEntity, errcontract.CodeInvalidArguments, importErr.Message,
			map[string]any{"current_template": currentTemplateOf()})
	case ecomintake.FailureEnvelope, ecomintake.FailureParse, ecomintake.FailureNoValidRows:
		writeCodedError(c, http.StatusUnprocessableEntity, errcontract.CodeInvalidArguments, importErr.Message, nil)
	case ecomintake.FailureConflict:
		writeCodedError(c, http.StatusConflict, errcontract.CodeConflict, importErr.Message, nil)
	default:
		writeSystemFailure(c, http.StatusInternalServerError, importErr)
	}
}

func currentTemplateOf() []map[string]any {
	out := []map[string]any{}
	sources := make([]ecomintake.Source, 0, len(ecomintake.Templates))
	for s := range ecomintake.Templates {
		sources = append(sources, s)
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i] < sources[j] })
	for _, s := range sources {
		out = append(out, map[string]any{"source": string(s), "version": ecomintake.TemplateVersions[s]})
	}
	return out
}

// --- 情景模拟 ---

// EvaluateBFCMScenario POST /ecom/scenarios/bfcm —— 输出顶层强制 simulated 标识（底线 2）。
func (h *EcommerceHandler) EvaluateBFCMScenario(c *gin.Context) {
	var in ecomsim.BFCMInput
	if err := c.ShouldBindJSON(&in); err != nil {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "请求体不合法", nil)
		return
	}
	out := ecomsim.EvaluateBFCM(c.Request.Context(), in, nil)
	c.JSON(http.StatusOK, out)
}

// EvaluatePriceScenario POST /ecom/scenarios/price-sensitivity
func (h *EcommerceHandler) EvaluatePriceScenario(c *gin.Context) {
	var req struct {
		Delta ecomsim.PriceDelta `json:"delta"`
		Base  ecomsim.CMBase     `json:"base"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "请求体不合法", nil)
		return
	}
	out := ecomsim.PriceSensitivity(req.Delta, req.Base)
	c.JSON(http.StatusOK, out)
}

// --- 小工具 ---

func countMatched(results []settlement.MatchResult) int {
	n := 0
	for _, r := range results {
		if r.Status == "matched" {
			n++
		}
	}
	return n
}

func marshalDifferences(results []settlement.MatchResult) json.RawMessage {
	diffs := []settlement.MatchResult{}
	for _, r := range results {
		if r.Status == "difference" {
			diffs = append(diffs, r)
		}
	}
	b, _ := json.Marshal(diffs)
	return b
}

func strPtrVal(s string) *string { return &s }
