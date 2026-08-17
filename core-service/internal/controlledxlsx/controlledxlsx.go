// Package controlledxlsx reads the controlled store-facts workbook contract:
// the first worksheet of an XLSX package with inline and shared-string cells
// as emitted by Excel. It is intentionally dependency-free (archive/zip +
// encoding/xml) and shared by the monthly importer and the store-day
// importer so both parse exactly the same workbook shape.
package controlledxlsx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

type cell struct {
	Ref    string `xml:"r,attr"`
	Type   string `xml:"t,attr"`
	Value  string `xml:"v"`
	Inline string `xml:"is>t"`
}
type row struct {
	Cells []cell `xml:"c"`
}
type sheet struct {
	Rows []row `xml:"sheetData>row"`
}
type sharedString struct {
	Text []string `xml:"t"`
}
type sharedStrings struct {
	Items []sharedString `xml:"si"`
}

// workbook.xml lists the sheets in tab order and points at each one by
// relationship id; workbook.xml.rels maps that id to the part path. Tab order
// is the only authority for "first worksheet" — file names need not match it,
// a workbook whose first tab lives in sheet3.xml is perfectly legal.
type workbookSheet struct {
	Name    string `xml:"name,attr"`
	SheetID string `xml:"sheetId,attr"`
	RID     string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
}
type workbook struct {
	Sheets []workbookSheet `xml:"sheets>sheet"`
}
type relationship struct {
	ID     string `xml:"Id,attr"`
	Target string `xml:"Target,attr"`
}
type relationships struct {
	Items []relationship `xml:"Relationship"`
}

// firstWorksheetPath resolves the tab-order-first worksheet part. It falls back
// to the lowest-numbered worksheet file when the workbook part or its
// relationships are absent, which keeps hand-built fixtures readable while
// never depending on map iteration order.
func firstWorksheetPath(files map[string][]byte) string {
	if rawBook, ok := files["xl/workbook.xml"]; ok {
		book := workbook{}
		if err := xml.Unmarshal(rawBook, &book); err == nil && len(book.Sheets) > 0 {
			targets := map[string]string{}
			if rawRels, relsOK := files["xl/_rels/workbook.xml.rels"]; relsOK {
				rels := relationships{}
				if relErr := xml.Unmarshal(rawRels, &rels); relErr == nil {
					for _, item := range rels.Items {
						targets[item.ID] = item.Target
					}
				}
			}
			if target, found := targets[book.Sheets[0].RID]; found {
				candidate := normaliseSheetTarget(target)
				if _, exists := files[candidate]; exists {
					return candidate
				}
			}
		}
	}
	names := make([]string, 0, len(files))
	for name := range files {
		if strings.HasPrefix(name, "xl/worksheets/") && strings.HasSuffix(name, ".xml") {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return ""
	}
	sort.Slice(names, func(i, j int) bool {
		left, leftOK := sheetOrdinal(names[i])
		right, rightOK := sheetOrdinal(names[j])
		if leftOK && rightOK && left != right {
			return left < right
		}
		return names[i] < names[j]
	})
	return names[0]
}

// normaliseSheetTarget turns a relationship target into a package path.
// Targets are relative to xl/ and may be written with a leading slash.
func normaliseSheetTarget(target string) string {
	target = strings.TrimPrefix(target, "/")
	if strings.HasPrefix(target, "xl/") {
		return target
	}
	return "xl/" + target
}

// sheetOrdinal reads the trailing number of "xl/worksheets/sheetN.xml" so
// sheet2 sorts before sheet10, which a plain string sort gets wrong.
func sheetOrdinal(name string) (int, bool) {
	base := strings.TrimSuffix(strings.TrimPrefix(name, "xl/worksheets/"), ".xml")
	digits := strings.TrimLeft(base, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	if digits == "" {
		return 0, false
	}
	value, err := strconv.Atoi(digits)
	if err != nil {
		return 0, false
	}
	return value, true
}

// Read returns the raw cell grid of the first worksheet, row by row, with
// column positions resolved from cell references (missing cells stay empty).
func Read(data []byte) ([][]string, error) {
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("invalid XLSX package: %w", err)
	}
	files := make(map[string][]byte)
	for _, file := range archive.File {
		reader, openErr := file.Open()
		if openErr != nil {
			return nil, openErr
		}
		content, readErr := io.ReadAll(reader)
		reader.Close()
		if readErr != nil {
			return nil, readErr
		}
		files[file.Name] = content
	}
	shared := sharedStrings{}
	if raw, ok := files["xl/sharedStrings.xml"]; ok {
		if err := xml.Unmarshal(raw, &shared); err != nil {
			return nil, fmt.Errorf("invalid shared strings: %w", err)
		}
	}
	sheetPath := firstWorksheetPath(files)
	if sheetPath == "" {
		return nil, fmt.Errorf("XLSX worksheet is missing")
	}
	sheetRaw := files[sheetPath]
	if len(sheetRaw) == 0 {
		return nil, fmt.Errorf("XLSX worksheet is missing")
	}
	parsed := sheet{}
	if err := xml.Unmarshal(sheetRaw, &parsed); err != nil {
		return nil, fmt.Errorf("invalid worksheet: %w", err)
	}
	rows := make([][]string, 0, len(parsed.Rows))
	for _, currentRow := range parsed.Rows {
		values := make([]string, 0)
		for _, currentCell := range currentRow.Cells {
			index := columnIndex(currentCell.Ref)
			for len(values) <= index {
				values = append(values, "")
			}
			value := currentCell.Inline
			if value == "" {
				value = currentCell.Value
				if currentCell.Type == "s" {
					if parsedIndex, parseErr := strconv.Atoi(value); parseErr == nil && parsedIndex >= 0 && parsedIndex < len(shared.Items) {
						value = strings.Join(shared.Items[parsedIndex].Text, "")
					}
				}
			}
			values[index] = value
		}
		rows = append(rows, values)
	}
	return rows, nil
}

func columnIndex(reference string) int {
	letters := strings.TrimRight(reference, "0123456789")
	index := 0
	for _, char := range strings.ToUpper(letters) {
		if char >= 'A' && char <= 'Z' {
			index = index*26 + int(char-'A'+1)
		}
	}
	if index == 0 {
		return 0
	}
	return index - 1
}
