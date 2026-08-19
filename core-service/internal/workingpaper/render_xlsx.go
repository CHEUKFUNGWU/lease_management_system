package workingpaper

import (
	"fmt"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

const (
	coverSheetName  = "封面"
	sourceSheetName = "_来源"
)

// exploratoryFill is the uniform底色 for every exploratory cell, per the
// working-paper design §7.2.
const exploratoryFill = "FCE4D6"

var xlsxHeaders = []string{"引用", "标签", "度量", "值", "单位", "币种", "来源依据", "出处"}

// RenderXLSX renders the paper as an xlsx workbook: one sheet per section,
// a fixed cover sheet first and a fixed `_来源` sheet last listing every
// cell's provenance. Certified cells carry a comment of the form
// "engine / call_id". The output is deterministic for equal inputs and `now`
// (I7): no wall-clock values other than the injected cover timestamp.
func RenderXLSX(p Paper, now time.Time) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	if err := f.SetSheetName("Sheet1", coverSheetName); err != nil {
		return nil, err
	}
	// Pin document properties so the archive bytes do not embed wall-clock
	// values (excelize writes created/modified from these fields).
	if err := f.SetDocProps(&excelize.DocProperties{
		Creator:        "workingpaper",
		Title:          p.Title,
		Subject:        "IFRS/retail working paper",
		Created:        now.UTC().Format(time.RFC3339),
		Modified:       now.UTC().Format(time.RFC3339),
		LastModifiedBy: "workingpaper",
	}); err != nil {
		return nil, err
	}

	if err := writeCoverSheet(f, p); err != nil {
		return nil, err
	}
	used := map[string]struct{}{coverSheetName: {}}
	for _, sec := range p.Sections {
		if err := writeSectionSheet(f, sec, used); err != nil {
			return nil, err
		}
	}
	if err := writeSourceSheet(f, p); err != nil {
		return nil, err
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeCoverSheet(f *excelize.File, p Paper) error {
	rows := [][]string{
		{"标题", p.Title},
		{"期间", p.Period},
		{"法人范围", p.LegalEntityScope},
		{"生成时间", p.Cover.GeneratedAt},
		{"生成者", p.GeneratedBy},
		{"复核状态", string(p.Cover.ReviewState)},
		{"数据版本", p.DataVersion},
		{"口径/假设版本", p.AssumptionVersion},
		{"引擎版本", p.EngineVersion},
		{"指标定义版本", p.MetricDefinitionVersion},
		{"模板版本", p.TemplateVersion},
		{"汇率版本", p.ExchangeRateVersion},
		{"沙箱镜像 digest", p.SandboxImageDigest},
		{"Certified 数字数", fmt.Sprintf("%d", p.Cover.CertifiedCount)},
		{"Exploratory 数字数", fmt.Sprintf("%d", p.Cover.ExploratoryCount)},
		{"系统事实数", fmt.Sprintf("%d", p.Cover.SystemFactCount)},
		{"人工确认数", fmt.Sprintf("%d", p.Cover.HumanInputCount)},
		{"数据缺口数", fmt.Sprintf("%d", p.Cover.DataGapCount)},
		{"未解释残差", p.UnexplainedResidual},
		{"数据缺口清单", strings.Join(p.DataGaps, "；")},
		{"待决问题", strings.Join(p.OpenQuestions, "；")},
	}
	for i, row := range rows {
		for j, v := range row {
			cell, _ := excelize.CoordinatesToCellName(j+1, i+1)
			if err := f.SetCellStr(coverSheetName, cell, v); err != nil {
				return err
			}
		}
	}
	style, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return err
	}
	last, _ := excelize.CoordinatesToCellName(1, len(rows))
	return f.SetCellStyle(coverSheetName, "A1", last, style)
}

func writeSectionSheet(f *excelize.File, sec Section, used map[string]struct{}) error {
	name := sheetName(sec, used)
	if _, err := f.NewSheet(name); err != nil {
		return err
	}
	if err := f.SetCellStr(name, "A1", sec.Title); err != nil {
		return err
	}
	if sec.Narrative != "" {
		if err := f.SetCellStr(name, "A2", sec.Narrative); err != nil {
			return err
		}
	}

	row := 4
	for j, h := range xlsxHeaders {
		cell, _ := excelize.CoordinatesToCellName(j+1, row)
		if err := f.SetCellStr(name, cell, h); err != nil {
			return err
		}
	}
	row++
	for _, c := range sec.Cells {
		values := []string{c.Ref, c.Label, c.MeasureID, "", c.Unit, c.Currency, string(c.Provenance.Basis), provenanceSource(c.Provenance)}
		for j, v := range values {
			cell, _ := excelize.CoordinatesToCellName(j+1, row)
			if err := f.SetCellStr(name, cell, v); err != nil {
				return err
			}
		}
		valueCell, _ := excelize.CoordinatesToCellName(4, row)
		if err := f.SetCellValue(name, valueCell, c.Value); err != nil {
			return err
		}
		if c.Provenance.Basis == BasisCertified {
			comment := excelize.Comment{
				Author:   "workingpaper",
				AuthorID: 0,
				Cell:     valueCell,
				Text:     fmt.Sprintf("certified: engine=%s call_id=%s", c.Provenance.EngineVersion, c.Provenance.ToolCallID),
			}
			if err := f.AddComment(name, comment); err != nil {
				return err
			}
		}
		if c.Provenance.Basis == BasisExploratory {
			style, err := f.NewStyle(&excelize.Style{
				Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{exploratoryFill}},
			})
			if err != nil {
				return err
			}
			if err := f.SetCellStyle(name, valueCell, valueCell, style); err != nil {
				return err
			}
		}
		row++
	}
	return nil
}

func writeSourceSheet(f *excelize.File, p Paper) error {
	if _, err := f.NewSheet(sourceSheetName); err != nil {
		return err
	}
	headers := []string{"引用", "章节", "标签", "度量", "basis", "tool_call_id", "engine_version", "input_hash", "sandbox_run_id", "code_hash", "image_digest", "source_table", "source_record_id", "data_version", "confirmed_by", "confirmed_at"}
	for j, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(j+1, 1)
		if err := f.SetCellStr(sourceSheetName, cell, h); err != nil {
			return err
		}
	}
	row := 2
	for _, sec := range p.Sections {
		for _, c := range sec.Cells {
			pr := c.Provenance
			values := []string{
				c.Ref, sec.ID, c.Label, c.MeasureID,
				string(pr.Basis), pr.ToolCallID, pr.EngineVersion, pr.InputHash,
				pr.SandboxRunID, pr.CodeHash, pr.ImageDigest,
				pr.SourceTable, pr.SourceRecordID, pr.DataVersion,
				pr.ConfirmedBy, pr.ConfirmedAt,
			}
			for j, v := range values {
				cell, _ := excelize.CoordinatesToCellName(j+1, row)
				if err := f.SetCellStr(sourceSheetName, cell, v); err != nil {
					return err
				}
			}
			row++
		}
	}
	return nil
}

// provenanceSource renders the human-readable origin of a cell.
func provenanceSource(pr Provenance) string {
	switch pr.Basis {
	case BasisCertified:
		return fmt.Sprintf("%s / %s", pr.EngineVersion, pr.ToolCallID)
	case BasisSystemFact:
		if pr.SourceTable != "" {
			return fmt.Sprintf("%s.%s", pr.SourceTable, pr.SourceRecordID)
		}
		return "system fact"
	case BasisExploratory:
		return fmt.Sprintf("sandbox run %s / code %s / image %s", pr.SandboxRunID, pr.CodeHash, pr.ImageDigest)
	case BasisHumanInput:
		return fmt.Sprintf("confirmed by %s at %s", pr.ConfirmedBy, pr.ConfirmedAt)
	}
	return ""
}

// sheetName sanitizes a section ID into a valid, unique sheet name (31 runes
// max, no []:*?/\ characters). Uniqueness is tracked in used.
func sheetName(sec Section, used map[string]struct{}) string {
	base := sec.ID
	if base == "" {
		base = "section"
	}
	var b strings.Builder
	for _, r := range base {
		switch r {
		case '[', ']', ':', '*', '?', '/', '\\':
			b.WriteRune('_')
		default:
			b.WriteRune(r)
		}
	}
	name := strings.TrimSpace(b.String())
	if name == "" {
		name = "section"
	}
	runes := []rune(name)
	if len(runes) > 31 {
		name = string(runes[:31])
	}
	candidate, i := name, 1
	for {
		if _, taken := used[candidate]; !taken {
			used[candidate] = struct{}{}
			return candidate
		}
		candidate = fmt.Sprintf("%s_%d", name, i)
		i++
	}
}
