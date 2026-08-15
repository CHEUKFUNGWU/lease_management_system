package handlers

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/errcontract"
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
	snapshotBuilder    *reporting.SnapshotBuilder
	closeControlRepo   *repository.CloseControlRepository
	monthlyClosingRepo *repository.MonthlyClosingRepository
}

func NewReportHandler(
	contractRepo *repository.ContractRepository,
	psRepo *repository.PaymentScheduleRepository,
	mcRepo *repository.MonthlyClosingRepository,
	systemSettingRepo *repository.SystemSettingRepository,
	masterDataRepo *repository.MasterDataRepository,
	closeControlRepos ...*repository.CloseControlRepository,
) *ReportHandler {
	var closeControlRepo *repository.CloseControlRepository
	if len(closeControlRepos) > 0 {
		closeControlRepo = closeControlRepos[0]
	}
	return &ReportHandler{
		snapshotBuilder: reporting.NewSnapshotBuilder(contractRepo, psRepo, systemSettingRepo, mcRepo).
			WithStores(masterDataRepo),
		closeControlRepo:   closeControlRepo,
		monthlyClosingRepo: mcRepo,
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
	}
	if len(shocks) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "shocks is required and must contain at least one numeric rate change"})
		return
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

// UnitPrice compares rent per square metre across stores, brands or regions.
func (h *ReportHandler) UnitPrice(c *gin.Context) {
	groupBy := defaultValue(c.Query("group_by"), reporting.GroupByStore)
	if !oneOf(groupBy, reporting.GroupByStore, reporting.GroupByBrand, reporting.GroupByRegion) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group_by, must be store|brand|region"})
		return
	}
	h.projectJSON(c, reporting.NormalizeMode(c.Query("mode")), reporting.ProjectionRequest{
		Kind: reporting.KindUnitPrice, View: groupBy,
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

// ClosePack aggregates the disclosure/audit package with the close evidence
// for a single accounting period. It is still read-only: it does not resolve
// exceptions, approve a batch or change an Official snapshot.
func (h *ReportHandler) ClosePack(c *gin.Context) {
	period := strings.TrimSpace(c.Query("period"))
	periodStart, err := time.Parse("2006-01", period)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "period 格式应为 YYYY-MM"})
		return
	}
	periodEnd := periodStart.AddDate(0, 1, -1)
	mode := reporting.NormalizeMode(c.Query("mode"))
	snapshot, ok := h.buildSnapshot(c, mode)
	if !ok {
		return
	}
	disclosure, err := reporting.Project(snapshot, reporting.ProjectionRequest{
		Kind: reporting.KindDisclosure, StartDate: periodStart, EndDate: periodEnd,
	})
	if err != nil {
		writeProjectionError(c, err)
		return
	}
	var exceptions []closeControlExceptionResponse
	if h.closeControlRepo != nil {
		items, listErr := h.closeControlRepo.ListExceptions(c.Request.Context(), period, middleware.GetTenantID(c))
		if listErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": listErr.Error()})
			return
		}
		exceptions = make([]closeControlExceptionResponse, 0, len(items))
		for _, item := range items {
			exceptions = append(exceptions, closeControlExceptionResponse{
				ID: item.ID, RuleCode: item.RuleCode, Severity: item.Severity,
				ExceptionState: item.ExceptionState, ClosingDisposition: item.ClosingDisposition,
				ContractNumber: item.ContractNumber, BatchNumber: item.BatchNumber,
			})
		}
	}
	var batches []*repository.MonthlyClosingBatch
	if h.monthlyClosingRepo != nil {
		batches, err = h.monthlyClosingRepo.GetBatches(c.Request.Context(), period, middleware.GetTenantID(c))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"period":                period,
		"mode":                  mode,
		"is_official":           mode == reporting.Official,
		"report_basis":          disclosure.Payload["report_basis"],
		"disclosure":            disclosure.Payload,
		"close_exceptions":      exceptions,
		"monthly_close_batches": batches,
		"exception_count":       len(exceptions),
	})
}

// ExportClosePack writes the same server-side disclosure projection and close
// evidence returned by ClosePack into an immutable, self-describing ZIP. The
// browser may still offer a convenient XLSX export, but the audit hand-off
// must have one Core-generated snapshot, manifest and hashable evidence set.
// This endpoint is read-only: it never changes approval, posting or lock state.
func (h *ReportHandler) ExportClosePack(c *gin.Context) {
	period := strings.TrimSpace(c.Query("period"))
	periodStart, err := time.Parse("2006-01", period)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "period 格式应为 YYYY-MM"})
		return
	}
	periodEnd := periodStart.AddDate(0, 1, -1)
	mode := reporting.NormalizeMode(c.Query("mode"))
	snapshot, ok := h.buildSnapshot(c, mode)
	if !ok {
		return
	}
	disclosure, err := reporting.Project(snapshot, reporting.ProjectionRequest{
		Kind: reporting.KindDisclosure, StartDate: periodStart, EndDate: periodEnd,
	})
	if err != nil {
		writeProjectionError(c, err)
		return
	}

	exceptions := []closeControlExceptionResponse{}
	if h.closeControlRepo != nil {
		items, listErr := h.closeControlRepo.ListExceptions(c.Request.Context(), period, middleware.GetTenantID(c))
		if listErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": listErr.Error()})
			return
		}
		for _, item := range items {
			exceptions = append(exceptions, closeControlExceptionResponse{
				ID: item.ID, RuleCode: item.RuleCode, Severity: item.Severity,
				ExceptionState: item.ExceptionState, ClosingDisposition: item.ClosingDisposition,
				ContractNumber: item.ContractNumber, BatchNumber: item.BatchNumber,
			})
		}
	}
	batches := []*repository.MonthlyClosingBatch{}
	if h.monthlyClosingRepo != nil {
		batches, err = h.monthlyClosingRepo.GetBatches(c.Request.Context(), period, middleware.GetTenantID(c))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	closePack := map[string]any{
		"period": period, "mode": mode, "is_official": mode == reporting.Official,
		"report_basis": disclosure.Payload["report_basis"], "disclosure": disclosure.Payload,
		"close_exceptions": exceptions, "monthly_close_batches": batches,
		"exception_count": len(exceptions),
	}
	files, err := closePackFiles(closePack, disclosure.Payload)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build close pack: " + err.Error()})
		return
	}
	manifest := map[string]any{
		"schema_version": "lease.close-pack.v1", "period": period, "mode": mode,
		"is_official": mode == reporting.Official, "snapshot_id": snapshot.ID,
		"policy_version": snapshot.PolicyVersion, "generated_at": time.Now().UTC(),
		"legal_entity_id": middleware.GetTenantID(c), "files": closePackManifest(files),
		"review_note": "该导出包是系统生成的审计资料包，不替代第三方会计师复核或正式审计结论。",
	}
	manifestRaw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to encode close pack manifest"})
		return
	}
	files["manifest.json"] = manifestRaw

	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	for name, content := range files {
		writer, createErr := zw.Create(name)
		if createErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create close pack archive"})
			return
		}
		if _, writeErr := writer.Write(content); writeErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to write close pack archive"})
			return
		}
	}
	if err := zw.Close(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to finalize close pack archive"})
		return
	}

	filename := fmt.Sprintf("IFRS16_Close_Pack_%s_%s_%s.zip", period, mode, safeFilename(snapshot.ID))
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("X-Report-Snapshot-ID", snapshot.ID)
	c.Header("X-Report-Policy-Version", snapshot.PolicyVersion)
	c.Header("X-Report-Mode", string(mode))
	c.Data(http.StatusOK, "application/zip", archive.Bytes())
}

func closePackFiles(closePack map[string]any, disclosure map[string]any) (map[string][]byte, error) {
	closePackRaw, err := json.MarshalIndent(closePack, "", "  ")
	if err != nil {
		return nil, err
	}
	disclosureRaw, err := json.MarshalIndent(disclosure, "", "  ")
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{
		"close_pack.json": closePackRaw, "disclosure.json": disclosureRaw,
	}
	workpaper, ok := disclosure["audit_workpaper"]
	if !ok {
		return files, nil
	}
	workpaperRaw, err := json.Marshal(workpaper)
	if err != nil {
		return nil, err
	}
	var decoded struct {
		Rows []reporting.AuditWorkpaperRow `json:"rows"`
	}
	if err := json.Unmarshal(workpaperRaw, &decoded); err != nil {
		return nil, err
	}
	var csvBuffer bytes.Buffer
	csvWriter := csv.NewWriter(&csvBuffer)
	if err := csvWriter.Write([]string{
		"contract_id", "contract_number", "contract_name", "legal_entity_id", "store_name",
		"asset_type", "currency", "lease_scope", "approval_status", "report_mode",
		"discount_rate", "discount_rate_source", "payment_schedule_count", "event_adjustment_count",
		"opening_liability", "additions", "interest", "payments", "liability_remeasurement",
		"closing_liability", "opening_rou", "rou_additions", "depreciation", "impairment",
		"closing_rou", "liability_tie_out", "rou_tie_out",
	}); err != nil {
		return nil, err
	}
	for _, row := range decoded.Rows {
		if err := csvWriter.Write([]string{
			row.ContractID, row.ContractNumber, row.ContractName, row.LegalEntityID, row.StoreName,
			row.AssetType, row.Currency, row.LeaseScope, row.ApprovalStatus, row.ReportMode,
			fmt.Sprintf("%.10f", row.DiscountRate), row.DiscountRateSource,
			strconv.Itoa(row.PaymentScheduleCount), strconv.Itoa(row.EventAdjustmentCount),
			fmt.Sprintf("%.2f", row.OpeningLiability.Float64()), fmt.Sprintf("%.2f", row.Additions.Float64()),
			fmt.Sprintf("%.2f", row.Interest.Float64()), fmt.Sprintf("%.2f", row.Payments.Float64()),
			fmt.Sprintf("%.2f", row.LiabilityRemeasurement.Float64()), fmt.Sprintf("%.2f", row.ClosingLiability.Float64()),
			fmt.Sprintf("%.2f", row.OpeningROU.Float64()), fmt.Sprintf("%.2f", row.ROUAdditions.Float64()),
			fmt.Sprintf("%.2f", row.Depreciation.Float64()), fmt.Sprintf("%.2f", row.Impairment.Float64()),
			fmt.Sprintf("%.2f", row.ClosingROU.Float64()), fmt.Sprintf("%.2f", row.LiabilityTieOut.Float64()),
			fmt.Sprintf("%.2f", row.ROUTieOut.Float64()),
		}); err != nil {
			return nil, err
		}
	}
	csvWriter.Flush()
	if err := csvWriter.Error(); err != nil {
		return nil, err
	}
	files["audit_workpaper.csv"] = csvBuffer.Bytes()
	return files, nil
}

func closePackManifest(files map[string][]byte) []map[string]any {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	manifest := make([]map[string]any, 0, len(names))
	for _, name := range names {
		manifest = append(manifest, map[string]any{
			"name": name, "sha256": fmt.Sprintf("%x", sha256.Sum256(files[name])), "bytes": len(files[name]),
		})
	}
	return manifest
}

func safeFilename(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "snapshot-missing"
	}
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "snapshot"
	}
	return b.String()
}

type closeControlExceptionResponse struct {
	ID                 string `json:"id"`
	RuleCode           string `json:"rule_code"`
	Severity           string `json:"severity"`
	ExceptionState     string `json:"exception_state"`
	ClosingDisposition string `json:"closing_disposition"`
	ContractNumber     string `json:"contract_number,omitempty"`
	BatchNumber        string `json:"batch_number,omitempty"`
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
		// A missing discount rate is a data condition the caller can fix
		// (confirm the rate on the contract or set a global policy), not a
		// server fault — it must not surface as a retryable 500.
		var missing *reporting.DiscountRateMissingError
		if errors.As(err, &missing) {
			writeDiscountRateMissing(c, missing)
			return nil, false
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return nil, false
	}
	return snapshot, true
}

// writeDiscountRateMissing is the single HTTP adapter for
// ErrDiscountRateRequired: 422 + data_unavailable + the affected contract
// numbers, so the client can say what is missing and where to fix it instead
// of telling the user to retry a request that will never succeed.
func writeDiscountRateMissing(c *gin.Context, missing *reporting.DiscountRateMissingError) {
	writeCodedError(c, http.StatusUnprocessableEntity, errcontract.CodeDataUnavailable, missing.Error(), gin.H{
		"discount_rate_missing": true,
		"contracts":             missing.ContractNumbers,
	})
}

func writeProjectionError(c *gin.Context, err error) {
	// DiscountRateMissingError wraps ErrDiscountRateRequired, so the As check
	// must win over the Is check below it to keep the contract numbers.
	var missing *reporting.DiscountRateMissingError
	switch {
	case errors.Is(err, reporting.ErrContractNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, reporting.ErrPaymentSchedulesRequired):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, reporting.ErrSensitivityShocksRequired):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.As(err, &missing):
		writeDiscountRateMissing(c, missing)
	case errors.Is(err, contractsvc.ErrDiscountRateRequired):
		writeDiscountRateMissing(c, &reporting.DiscountRateMissingError{})
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
