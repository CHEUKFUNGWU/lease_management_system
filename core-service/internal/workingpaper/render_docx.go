package workingpaper

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"
	"time"
)

// RenderDOCX renders the paper as a minimal but valid OOXML .docx: cover
// page, one table per section and a provenance appendix. It is hand-rolled
// over archive/zip instead of pulling a third-party docx library — the
// repository keeps its direct dependencies minimal (design decision D-A).
// Output is deterministic for equal inputs and `now` (I7).
func RenderDOCX(p Paper, now time.Time) ([]byte, error) {
	var body strings.Builder
	body.WriteString(`<w:p><w:pPr><w:jc w:val="center"/></w:pPr><w:r><w:rPr><w:b/><w:sz w:val="36"/></w:rPr><w:t xml:space="preserve">` + xmlEscape(p.Title) + `</w:t></w:r></w:p>`)

	coverRows := [][2]string{
		{"期间", p.Period},
		{"法人范围", p.LegalEntityScope},
		{"生成时间", p.Cover.GeneratedAt},
		{"生成者", p.GeneratedBy},
		{"复核状态", string(p.Cover.ReviewState)},
		{"数据版本", p.DataVersion},
		{"口径/假设版本", p.AssumptionVersion},
		{"引擎版本", p.EngineVersion},
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
	for _, row := range coverRows {
		body.WriteString(`<w:p><w:r><w:rPr><w:b/></w:rPr><w:t xml:space="preserve">` + xmlEscape(row[0]) + `：</w:t></w:r><w:r><w:t xml:space="preserve">` + xmlEscape(row[1]) + `</w:t></w:r></w:p>`)
	}

	for _, sec := range p.Sections {
		body.WriteString(`<w:p><w:r><w:rPr><w:b/><w:sz w:val="28"/></w:rPr><w:t xml:space="preserve">` + xmlEscape(sec.Title) + `</w:t></w:r></w:p>`)
		if sec.Narrative != "" {
			body.WriteString(`<w:p><w:r><w:t xml:space="preserve">` + xmlEscape(sec.Narrative) + `</w:t></w:r></w:p>`)
		}
		if len(sec.Cells) > 0 {
			rows := [][]string{{"引用", "标签", "度量", "值", "单位", "币种", "来源依据", "出处"}}
			for _, c := range sec.Cells {
				rows = append(rows, []string{
					c.Ref, c.Label, c.MeasureID, fmt.Sprint(c.Value), c.Unit, c.Currency,
					string(c.Provenance.Basis), provenanceSource(c.Provenance),
				})
			}
			body.WriteString(docxTable(rows))
		}
	}

	body.WriteString(`<w:p><w:r><w:rPr><w:b/><w:sz w:val="28"/></w:rPr><w:t xml:space="preserve">来源附录</w:t></w:r></w:p>`)
	appendix := [][]string{{"引用", "章节", "标签", "度量", "basis", "tool_call_id", "engine_version", "source_table", "source_record_id", "confirmed_by"}}
	for _, sec := range p.Sections {
		for _, c := range sec.Cells {
			pr := c.Provenance
			appendix = append(appendix, []string{
				c.Ref, sec.ID, c.Label, c.MeasureID, string(pr.Basis),
				pr.ToolCallID, pr.EngineVersion, pr.SourceTable, pr.SourceRecordID, pr.ConfirmedBy,
			})
		}
	}
	body.WriteString(docxTable(appendix))
	body.WriteString(`<w:sectPr><w:pgSz w:w="11906" w:h="16838"/><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440"/></w:sectPr>`)

	document := docxDocumentPrefix + body.String() + docxDocumentSuffix
	return zipDocx(map[string][]byte{
		"[Content_Types].xml": []byte(docxContentTypes),
		"_rels/.rels":         []byte(docxRels),
		"word/document.xml":   []byte(document),
	})
}

func docxTable(rows [][]string) string {
	var b strings.Builder
	b.WriteString(`<w:tbl><w:tblPr><w:tblBorders>`)
	for _, side := range []string{"top", "left", "bottom", "right", "insideH", "insideV"} {
		b.WriteString(`<w:` + side + ` w:val="single" w:sz="4" w:color="BFBFBF"/>`)
	}
	b.WriteString(`</w:tblBorders></w:tblPr>`)
	for _, row := range rows {
		b.WriteString(`<w:tr>`)
		for _, cell := range row {
			b.WriteString(`<w:tc><w:tcPr><w:tcW w:w="0" w:type="auto"/></w:tcPr><w:p><w:r><w:t xml:space="preserve">` + xmlEscape(cell) + `</w:t></w:r></w:p></w:tc>`)
		}
		b.WriteString(`</w:tr>`)
	}
	b.WriteString(`</w:tbl>`)
	return b.String()
}

func xmlEscape(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

// zipDocx packs the parts into a zip archive in fixed order with zero
// timestamps so byte output is reproducible.
func zipDocx(parts map[string][]byte) ([]byte, error) {
	names := []string{"[Content_Types].xml", "_rels/.rels", "word/document.xml"}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range names {
		content, ok := parts[name]
		if !ok {
			return nil, fmt.Errorf("workingpaper: missing docx part %s", name)
		}
		w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate})
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(content); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

const docxContentTypes = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`

const docxRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`

const docxDocumentPrefix = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>`

const docxDocumentSuffix = `</w:body></w:document>`
