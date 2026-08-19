package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMultipartFileBody(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "store-days.xlsx")
	if err := os.WriteFile(path, []byte("fake-xlsx"), 0o600); err != nil {
		t.Fatal(err)
	}
	body, contentType, err := multipartFileBody(path, map[string]string{
		"source_system": "pos-a",
		"as_of_at":      "", // empty fields must be skipped
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(contentType, "multipart/form-data; boundary=") {
		t.Fatalf("wrong content type: %s", contentType)
	}
	if !bytes.Contains(body, []byte("pos-a")) || !bytes.Contains(body, []byte("fake-xlsx")) {
		t.Fatalf("multipart body must carry the field and the file content: %q", body)
	}
	if bytes.Contains(body, []byte("as_of_at")) {
		t.Fatal("empty form fields must be skipped")
	}
}
