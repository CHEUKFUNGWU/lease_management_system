package handlers

import (
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
	contractsvc "github.com/lease-management-system/core-service/internal/services/contracts"
	"github.com/lease-management-system/core-service/internal/services/reporting"
)

// Response aliases preserve the existing handler package contract while the
// projection implementation lives behind reporting.Project.
type AmortizationReportRow = reporting.AmortizationRow
type CashflowForecastRow = reporting.CashflowRow
type TagSummaryRow = reporting.TagSummaryRow
type PortfolioSummaryRow = reporting.PortfolioRow
type SensitivityScenarioRow = reporting.SensitivityRow
type StandardComparisonRow = reporting.StandardComparisonRow

type ReportHandler struct {
	snapshotBuilder *reporting.SnapshotBuilder
}

func NewReportHandler(
	contractRepo *repository.ContractRepository,
	psRepo *repository.PaymentScheduleRepository,
	mcRepo *repository.MonthlyClosingRepository,
	systemSettingRepo *repository.SystemSettingRepository,
) *ReportHandler {
	return &ReportHandler{
		snapshotBuilder: reporting.NewSnapshotBuilder(contractRepo, psRepo, systemSettingRepo, mcRepo),
	}
}

func (h *ReportHandler) LiabilityRolling(c *gin.Context) {
	h.projectJSON(c, reporting.NormalizeMode(c.Query("mode")), reporting.ProjectionRequest{
		Kind: reporting.KindLiabilityRolling,
	})
}

func (h *ReportHandler) ContractSummary(c *gin.Context) {
	h.projectJSON(c, reporting.NormalizeMode(c.Query("mode")), reporting.ProjectionRequest{
		Kind: reporting.KindContractSummary,
	})
}

func (h *ReportHandler) PortfolioSummary(c *gin.Context) {
	h.projectJSON(c, reporting.NormalizeMode(c.Query("mode")), reporting.ProjectionRequest{
		Kind: reporting.KindPortfolio,
	})
}

func (h *ReportHandler) SensitivityAnalysis(c *gin.Context) {
	contractID := c.Query("contract_id")
	if contractID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "contract_id is required"})
		return
	}
	var baseRate *float64
	if value, err := strconv.ParseFloat(c.Query("base_rate"), 64); err == nil && value > 0 {
		baseRate = &value
	}
	shocks := []float64(nil)
	if raw := c.Query("shocks"); raw != "" {
		shocks = make([]float64, 0)
		for _, part := range strings.Split(raw, ",") {
			if value, err := strconv.ParseFloat(strings.TrimSpace(part), 64); err == nil {
				shocks = append(shocks, value)
			}
		}
		if len(shocks) == 0 {
			shocks = []float64{0}
		}
	}
	h.projectJSON(c, reporting.Working, reporting.ProjectionRequest{
		Kind: reporting.KindSensitivity, ContractID: contractID, Rate: baseRate, Shocks: shocks,
	})
}

func (h *ReportHandler) StandardComparison(c *gin.Context) {
	contractID := c.Query("contract_id")
	if contractID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "contract_id is required"})
		return
	}
	var discountRate *float64
	if value, err := strconv.ParseFloat(c.Query("discount_rate"), 64); err == nil && value > 0 {
		discountRate = &value
	}
	h.projectJSON(c, reporting.Working, reporting.ProjectionRequest{
		Kind: reporting.KindStandardComparison, ContractID: contractID, Rate: discountRate,
	})
}

func (h *ReportHandler) ExportLiabilityRolling(c *gin.Context) {
	snapshot, ok := h.buildSnapshot(c, reporting.NormalizeMode(c.Query("mode")))
	if !ok {
		return
	}
	language := c.Query("language")
	if language == "" {
		language = "zh-CN"
	}
	result, err := reporting.Project(snapshot, reporting.ProjectionRequest{
		Kind: reporting.KindLiabilityRolling, Format: reporting.FormatCSV, Language: language,
	})
	if err != nil {
		writeProjectionError(c, err)
		return
	}
	if result.CSV == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "CSV projection is unavailable"})
		return
	}

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", result.CSV.Filename))
	c.Header("X-Report-Snapshot-ID", snapshot.ID)
	c.Header("X-Report-Policy-Version", snapshot.PolicyVersion)
	c.Header("X-Report-Mode", string(snapshot.Mode))
	_, _ = c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})
	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()
	_ = writer.WriteAll(result.CSV.Records)
}

func (h *ReportHandler) Amortization(c *gin.Context) {
	view := defaultValue(c.Query("view"), reporting.ViewSummary)
	granularity := defaultValue(c.Query("granularity"), reporting.GranularityMonth)
	if !oneOf(granularity, reporting.GranularityDay, reporting.GranularityMonth, reporting.GranularityQuarter, reporting.GranularityHalfYear, reporting.GranularityYear) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid granularity, must be day|month|quarter|half_year|year"})
		return
	}
	if !oneOf(view, reporting.ViewContract, reporting.ViewStore, reporting.ViewSummary, reporting.ViewTag) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid view, must be contract|store|summary|tag"})
		return
	}
	startDate, endDate, ok := parseDateRange(c)
	if !ok {
		return
	}
	rate, ok := parseOptionalRate(c, "discount_rate_override", true)
	if !ok {
		return
	}
	exchangeRate, ok := parseExchangeRate(c)
	if !ok {
		return
	}
	h.projectJSON(c, reporting.NormalizeMode(c.Query("mode")), reporting.ProjectionRequest{
		Kind: reporting.KindAmortization, View: view, Granularity: granularity,
		StartDate: startDate, EndDate: endDate, ContractID: c.Query("contract_id"),
		Store: c.Query("store"), Tags: reportTags(c), Rate: rate,
		ReportCurrency: c.Query("report_currency"), ExchangeRate: exchangeRate,
	})
}

func (h *ReportHandler) Tags(c *gin.Context) {
	h.projectJSON(c, reporting.Working, reporting.ProjectionRequest{Kind: reporting.KindTags})
}

func (h *ReportHandler) TagSummary(c *gin.Context) {
	h.projectJSON(c, reporting.Working, reporting.ProjectionRequest{Kind: reporting.KindTagSummary})
}

func (h *ReportHandler) CashflowForecast(c *gin.Context) {
	view := defaultValue(c.Query("view"), reporting.ViewSummary)
	granularity := defaultValue(c.Query("granularity"), reporting.GranularityMonth)
	startDate, endDate, ok := parseDateRange(c)
	if !ok {
		return
	}
	if !oneOf(granularity, reporting.GranularityMonth, reporting.GranularityQuarter, reporting.GranularityYear) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid granularity, must be month|quarter|year"})
		return
	}
	if !oneOf(view, reporting.ViewContract, reporting.ViewStore, reporting.ViewSummary) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid view, must be contract|store|summary"})
		return
	}
	h.projectJSON(c, reporting.NormalizeMode(c.Query("mode")), reporting.ProjectionRequest{
		Kind: reporting.KindCashflow, View: view, Granularity: granularity,
		StartDate: startDate, EndDate: endDate, ContractID: c.Query("contract_id"),
		Store: c.Query("store"), Tags: reportTags(c),
	})
}

// Disclosure returns the IFRS 16 disclosure note package for a reporting period.
// The period defaults to the current calendar year; period_end doubles as the
// as-of date for the maturity analysis.
func (h *ReportHandler) Disclosure(c *gin.Context) {
	now := time.Now().UTC()
	periodStart, ok := parseReportDate(c, "period_start", time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.UTC))
	if !ok {
		return
	}
	periodEnd, ok := parseReportDate(c, "period_end", time.Date(now.Year(), 12, 31, 0, 0, 0, 0, time.UTC))
	if !ok {
		return
	}
	if periodEnd.Before(periodStart) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "period_end must be >= period_start"})
		return
	}
	h.projectJSON(c, reporting.NormalizeMode(c.Query("mode")), reporting.ProjectionRequest{
		Kind: reporting.KindDisclosure, StartDate: periodStart, EndDate: periodEnd,
	})
}

func (h *ReportHandler) projectJSON(c *gin.Context, mode reporting.Mode, request reporting.ProjectionRequest) {
	snapshot, ok := h.buildSnapshot(c, mode)
	if !ok {
		return
	}
	result, err := reporting.Project(snapshot, request)
	if err != nil {
		writeProjectionError(c, err)
		return
	}
	c.JSON(http.StatusOK, result.Payload)
}

func (h *ReportHandler) buildSnapshot(c *gin.Context, mode reporting.Mode) (*reporting.Snapshot, bool) {
	snapshot, err := h.snapshotBuilder.Build(c.Request.Context(), reporting.Request{
		Mode: mode, LegalEntityID: middleware.GetTenantID(c),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return nil, false
	}
	return snapshot, true
}

func writeProjectionError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, reporting.ErrContractNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, reporting.ErrPaymentSchedulesRequired):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, contractsvc.ErrDiscountRateRequired):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error(), "discount_rate_missing": true})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func parseDateRange(c *gin.Context) (time.Time, time.Time, bool) {
	startRaw, endRaw := c.Query("start_date"), c.Query("end_date")
	if startRaw == "" || endRaw == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "start_date and end_date are required"})
		return time.Time{}, time.Time{}, false
	}
	start, err := time.Parse("2006-01-02", startRaw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid start_date: %v", err)})
		return time.Time{}, time.Time{}, false
	}
	end, err := time.Parse("2006-01-02", endRaw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid end_date: %v", err)})
		return time.Time{}, time.Time{}, false
	}
	if end.Before(start) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "end_date must be >= start_date"})
		return time.Time{}, time.Time{}, false
	}
	return start, end, true
}

func parseReportDate(c *gin.Context, key string, fallback time.Time) (time.Time, bool) {
	raw := c.Query(key)
	if raw == "" {
		return fallback, true
	}
	value, err := time.Parse("2006-01-02", raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid %s: %v", key, err)})
		return time.Time{}, false
	}
	return value, true
}

func parseOptionalRate(c *gin.Context, key string, normalizePercent bool) (*float64, bool) {
	raw := c.Query(key)
	if raw == "" {
		return nil, true
	}
	value, err := parseReportFloat(raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid %s: %v", key, err)})
		return nil, false
	}
	if normalizePercent && value > 1 {
		value /= 100
	}
	return &value, true
}

func parseExchangeRate(c *gin.Context) (float64, bool) {
	raw := c.Query("exchange_rate")
	if raw == "" {
		return 0, true
	}
	value, err := parseReportFloat(raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid exchange_rate: %v", err)})
		return 0, false
	}
	if value <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "exchange_rate must be > 0"})
		return 0, false
	}
	return value, true
}

func reportTags(c *gin.Context) []string {
	tags := append([]string(nil), c.QueryArray("tags")...)
	if raw := c.Query("tag"); raw != "" {
		for _, tag := range strings.Split(raw, ",") {
			if tag = strings.TrimSpace(tag); tag != "" {
				tags = append(tags, tag)
			}
		}
	}
	return tags
}

func parseReportFloat(value string) (float64, error) {
	return strconv.ParseFloat(strings.ReplaceAll(value, ",", "."), 64)
}

func defaultValue(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func oneOf(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if value == candidate {
			return true
		}
	}
	return false
}
