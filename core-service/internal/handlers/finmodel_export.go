package handlers

// S2-9 三表模型导出：run 的逐行逐期结果渲染为带活公式的工作簿。小计行
// 是跨子行的 SUM/± 表达式（口径在模板 Children/Subtract 里，与 SM3 的导
// 出同一条纪律——文件里的算术可审计，不是冻结数字）。口径头载数据分类
// （模拟标识）、数据集版本与五条版本线；期间可按月/季/年折叠。

import (
	"fmt"

	"github.com/lease-management-system/core-service/internal/finmodel"
	"github.com/lease-management-system/core-service/internal/finmodel/template"
	"github.com/lease-management-system/core-service/internal/storepnl"
	"github.com/xuri/excelize/v2"
)

// ModelExportMeta is the 口径头 of one export.
type ModelExportMeta struct {
	ModelName               string
	DataClassification      string
	DatasetVersion          string
	AssumptionVersion       string
	ExchangeRateVersion     string
	MetricDefinitionVersion string
	EngineVersion           string
	FoldKind                finmodel.FoldKind
}

// modelExportRow is one template row plus its folded per-bucket values.
type modelExportRow struct {
	Key        string
	Label      string
	Kind       string
	Basis      string
	Children   []string
	Subtracted []string
	Values     map[string]*float64 // bucketLabel → value
}

// RenderModelRunXLSX writes the workbook. Source values are already
// folded by the caller (FoldMonthValues); this function only renders.
func RenderModelRunXLSX(tmpl *template.Template, rows []modelExportRow, buckets []finmodel.FoldBucket, meta ModelExportMeta) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	sheet := "model"
	if err := f.SetSheetName("Sheet1", sheet); err != nil {
		return nil, err
	}
	// 口径头：数据分类（模拟标识在其中）、数据集版本与四条版本线。
	header := fmt.Sprintf("data_classification: %s · dataset: %s · assumption: %s · exchange_rate: %s · metric_definition: %s · engine: %s",
		meta.DataClassification, orEmpty(meta.DatasetVersion), orEmpty(meta.AssumptionVersion),
		orEmpty(meta.ExchangeRateVersion), orEmpty(meta.MetricDefinitionVersion), meta.EngineVersion)
	_ = f.SetCellStr(sheet, "A1", "model: "+meta.ModelName+" · "+string(meta.FoldKind)+" fold · "+header)
	headerRow := 3
	if meta.DataClassification == "simulated" {
		_ = f.SetCellStr(sheet, "A2", "模拟数据 · SIMULATED（不进入正式过账链路）")
		headerRow = 4
	}
	headerCellA, _ := excelize.CoordinatesToCellName(1, headerRow)
	headerCellB, _ := excelize.CoordinatesToCellName(2, headerRow)
	_ = f.SetCellStr(sheet, headerCellA, "科目")
	_ = f.SetCellStr(sheet, headerCellB, "basis")
	for i, bucket := range buckets {
		cell, _ := excelize.CoordinatesToCellName(3+i, headerRow)
		_ = f.SetCellStr(sheet, cell, bucket.Label)
	}

	rowNumber := map[string]int{}
	for i, row := range rows {
		rowNumber[row.Key] = headerRow + 1 + i
	}
	for i, row := range rows {
		r := headerRow + 1 + i
		labelCell, _ := excelize.CoordinatesToCellName(1, r)
		basisCell, _ := excelize.CoordinatesToCellName(2, r)
		label := row.Label
		if row.Kind == "formula" || row.Kind == "check" {
			label += " (模型内自定义，未经指标治理)"
		}
		_ = f.SetCellStr(sheet, labelCell, label)
		_ = f.SetCellStr(sheet, basisCell, row.Basis)
			for j, bucket := range buckets {
				cell, _ := excelize.CoordinatesToCellName(3+j, r)
				if row.Kind == "subtotal" && len(row.Children) > 0 {
					_ = f.SetCellFormula(sheet, cell, storepnl.SubtotalFormula(row.Children, row.Subtracted, 3+j, rowNumber))
					continue
				}
				if value, ok := row.Values[bucket.Label]; ok && value != nil {
					_ = f.SetCellValue(sheet, cell, *value)
				} else {
					_ = f.SetCellStr(sheet, cell, "—")
				}
			}
		}
		buffer, err := f.WriteToBuffer()
		if err != nil {
			return nil, err
		}
		return buffer.Bytes(), nil
	}


// orEmpty renders a missing version line as an em-dash in the 口径头.
func orEmpty(value string) string {
	if value == "" {
		return "—"
	}
	return value
}
