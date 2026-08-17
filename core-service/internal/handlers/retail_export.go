package handlers

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/services/retailkpi"
	"github.com/lease-management-system/core-service/internal/services/retailexport"
	"github.com/lease-management-system/core-service/internal/services/retailpulse"
	"github.com/lease-management-system/core-service/internal/services/retailstore360"
)

// RetailExportHandler serves the export descriptors (the single column/
// formula source consumed by the web ExcelJS/pptxgenjs exporters) and the
// row extractors behind format=csv on the retail reads (design M3).
type RetailExportHandler struct{}

func NewRetailExportHandler() *RetailExportHandler { return &RetailExportHandler{} }

// Descriptors returns every registered export descriptor (CONTRACT-001
// shape: backend whitelist, frontend consumes).
func (h *RetailExportHandler) Descriptors(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"basis": "Working", "data": retailexport.Descriptors()})
}

// PulseExportRows flattens a pulse response's attention ranking (all
// currency partitions, group rows included) into descriptor rows.
func PulseExportRows(response *retailpulse.Response) []retailexport.Row {
	rows := make([]retailexport.Row, 0)
	appendPartition := func(attention []retailpulse.Attention) {
		for _, item := range attention {
			identity := item.StoreCode
			brandRegion := fmt.Sprintf("%s · %s", item.Brand, item.Region)
			if item.GroupBy == "region" || item.GroupBy == "brand" {
				identity = item.GroupLabel
				if item.GroupBy == "region" {
					brandRegion = "按区域"
				} else {
					brandRegion = "按品牌"
				}
			}
			signals := make([]string, 0, len(item.ObservedSignals))
			for _, signal := range item.ObservedSignals {
				signals = append(signals, signal.SignalCode)
			}
			rows = append(rows, retailexport.Row{
				"rank":            strconv.Itoa(item.Rank),
				"identity":        identity,
				"brand_region":    brandRegion,
				"signals":         joinNonEmpty(signals, "、"),
				"score":           strconv.FormatFloat(item.Score, 'f', 2, 64),
				"severity":        item.Severity,
				"revenue":         kpiText(item.CurrentKPIs["revenue"]),
				"revenue_change":  changeText(item.CurrentKPIs["revenue"], item.ComparisonKPIs["revenue"]),
				"store_contribution": kpiText(item.CurrentKPIs["store_contribution"]),
				"contribution_change": changeText(item.CurrentKPIs["store_contribution"], item.ComparisonKPIs["store_contribution"]),
				"source_systems":  joinNonEmpty(item.Evidence.SourceSystems, ","),
				"currency":        item.Currency,
			})
		}
	}
	if len(response.Partitions) == 0 {
		appendPartition(response.Attention)
	}
	for _, partition := range response.Partitions {
		appendPartition(partition.Attention)
	}
	return rows
}

// DiagnosticsExportRows flattens the store-360 summary into descriptor rows
// in a deterministic metric order.
func DiagnosticsExportRows(response *retailstore360.Response) []retailexport.Row {
	codes := make([]string, 0, len(response.Summary))
	for code := range response.Summary {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	rows := make([]retailexport.Row, 0, len(codes))
	for _, code := range codes {
		metric := response.Summary[code]
		change := ""
		if metric.ChangeValue != nil {
			change = strconv.FormatFloat(*metric.ChangeValue, 'f', 2, 64)
		}
		rows = append(rows, retailexport.Row{
			"metric": code, "unit": metric.Current.Unit,
			"current": kpiText(metric.Current), "comparison": kpiText(metric.Comparison),
			"change": change, "status": metric.Status,
		})
	}
	return rows
}

func kpiText(value retailkpi.KPIValue) string {
	if value.Value == nil {
		return ""
	}
	return strconv.FormatFloat(*value.Value, 'f', 2, 64)
}

func changeText(current, comparison retailkpi.KPIValue) string {
	if current.Value == nil || comparison.Value == nil {
		return ""
	}
	rate, reason := retailkpi.ChangeRate(current.Value, comparison.Value, retailkpi.ChangeRateType("revenue"))
	if rate == nil {
		return reason
	}
	return strconv.FormatFloat(*rate, 'f', 2, 64)
}

func joinNonEmpty(values []string, separator string) string {
	result := ""
	for _, value := range values {
		if value == "" {
			continue
		}
		if result != "" {
			result += separator
		}
		result += value
	}
	return result
}

// writeExportCSV streams a rendered export as a download with UTF-8 BOM.
func writeExportCSV(c *gin.Context, filename string, content []byte) {
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	bom := []byte{0xEF, 0xBB, 0xBF}
	if len(content) < 3 || content[0] != bom[0] || content[1] != bom[1] || content[2] != bom[2] {
		content = append(bom, content...)
	}
	c.Data(http.StatusOK, "text/csv; charset=utf-8", content)
}
