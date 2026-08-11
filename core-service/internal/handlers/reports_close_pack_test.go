package handlers

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/lease-management-system/core-service/internal/services/reporting"
)

func TestClosePackFilesIncludesHashableAuditWorkpaper(t *testing.T) {
	workpaper := map[string]any{
		"rows": []reporting.AuditWorkpaperRow{{
			ContractID: "contract-1", ContractNumber: "LC-001", ContractName: "旗舰店",
			LegalEntityID: "le-001", Currency: "CNY", LeaseScope: "in_scope",
			ApprovalStatus: "approved", ReportMode: "official", DiscountRate: 0.05,
			PaymentScheduleCount: 12, ClosingLiability: 1234.56, LiabilityTieOut: 0,
		}},
	}
	files, err := closePackFiles(
		map[string]any{"period": "2025-01", "disclosure": "see disclosure.json"},
		map[string]any{"report_basis": map[string]any{"snapshot_id": "snap-1"}, "audit_workpaper": workpaper},
	)
	if err != nil {
		t.Fatalf("closePackFiles() error = %v", err)
	}
	for _, name := range []string{"close_pack.json", "disclosure.json", "audit_workpaper.csv"} {
		if len(files[name]) == 0 {
			t.Fatalf("missing non-empty file %q", name)
		}
	}
	if !strings.Contains(string(files["audit_workpaper.csv"]), "LC-001") {
		t.Fatalf("audit workpaper CSV = %s", files["audit_workpaper.csv"])
	}
	manifest := closePackManifest(files)
	if len(manifest) != 3 || manifest[0]["name"] != "audit_workpaper.csv" {
		t.Fatalf("manifest = %#v", manifest)
	}
	if manifest[0]["sha256"] == "" || manifest[0]["bytes"] == 0 {
		t.Fatalf("manifest entry = %#v", manifest[0])
	}
}

func TestClosePackZipRoundTrip(t *testing.T) {
	files := map[string][]byte{"manifest.json": []byte(`{"schema_version":"lease.close-pack.v1"}`), "audit_workpaper.csv": []byte("contract_number\nLC-001\n")}
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create zip entry: %v", err)
		}
		if _, err := entry.Write(content); err != nil {
			t.Fatalf("write zip entry: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	reader, err := zip.NewReader(bytes.NewReader(archive.Bytes()), int64(archive.Len()))
	if err != nil {
		t.Fatalf("read zip: %v", err)
	}
	seen := map[string]bool{}
	for _, entry := range reader.File {
		readCloser, err := entry.Open()
		if err != nil {
			t.Fatalf("open %s: %v", entry.Name, err)
		}
		body, err := io.ReadAll(readCloser)
		_ = readCloser.Close()
		if err != nil {
			t.Fatalf("read %s: %v", entry.Name, err)
		}
		seen[entry.Name] = string(body) == string(files[entry.Name])
	}
	if len(seen) != len(files) || !seen["manifest.json"] || !seen["audit_workpaper.csv"] {
		t.Fatalf("zip entries = %#v", seen)
	}
	var manifest map[string]any
	if err := json.Unmarshal(files["manifest.json"], &manifest); err != nil {
		t.Fatalf("manifest should remain JSON: %v", err)
	}
}
