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
	var sheetRaw []byte
	for name, content := range files {
		if strings.HasPrefix(name, "xl/worksheets/") && strings.HasSuffix(name, ".xml") {
			sheetRaw = content
			break
		}
	}
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
