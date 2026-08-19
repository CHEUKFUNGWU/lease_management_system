package docparse

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakePaddleOCR spins up the whole async contract: multipart submit → poll
// state → download jsonUrl.
func fakePaddleOCR(t *testing.T) *httptest.Server {
	t.Helper()
	var mu sync.Mutex
	polls := 0
	layout := map[string]any{
		"layoutParsingResults": []any{
			map[string]any{
				"markdown": map[string]any{"text": "租金每月 50,000 元"},
				"prunedResult": map[string]any{
					"block_content": "租金每月 50,000 元",
					"block_bbox":    []any{float64(10), float64(20), float64(100), float64(20), float64(100), float64(40), float64(10), float64(40)},
				},
			},
			map[string]any{
				"markdown": map[string]any{"text": "租期 36 个月"},
				"prunedResult": map[string]any{
					"content": "租期 36 个月",
					"bbox":    []any{float64(5), float64(5), float64(50), float64(5), float64(50), float64(15), float64(5), float64(15)},
				},
			},
		},
	}
	layoutJSON, _ := json.Marshal(layout)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/ocr/jobs/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			if got := r.Header.Get("Authorization"); got != "bearer test-token" {
				http.Error(w, "bad auth", http.StatusForbidden)
				return
			}
			if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
				http.Error(w, "must be multipart", http.StatusInternalServerError)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"jobId": "job-1"}})
			return
		}
		// GET /api/v2/ocr/jobs/job-1
		mu.Lock()
		polls++
		p := polls
		mu.Unlock()
		state := "running"
		if p >= 2 {
			state = "done"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
			"state":     state,
			"resultUrl": map[string]any{"jsonUrl": "http://" + r.Host + "/layout.json"},
		}})
	})
	mux.HandleFunc("/layout.json", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(layoutJSON)
	})
	return httptest.NewServer(mux)
}

func TestPaddleOCREndToEnd(t *testing.T) {
	srv := fakePaddleOCR(t)
	defer srv.Close()

	c := NewPaddleOCR(PaddleOCRConfig{
		APIURL:         srv.URL + "/api/v2/ocr/jobs",
		AccessToken:    "test-token",
		PollInterval:   5 * time.Millisecond,
		MaxPollSeconds: 5,
	})
	doc, err := c.Parse(context.Background(), Source{
		Filename: "scan.pdf",
		Data:     []byte("%PDF-1.4"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if doc.EvidenceMode != EvidenceCoordinate {
		t.Fatalf("OCR output must carry Coordinate evidence, got %s", doc.EvidenceMode)
	}
	if !strings.Contains(doc.Markdown, "--- Page 1 ---") || !strings.Contains(doc.Markdown, "租金每月") {
		t.Fatalf("markdown page markers missing: %q", doc.Markdown)
	}
	if len(doc.Locators) != 2 {
		t.Fatalf("expected 2 locators, got %d: %+v", len(doc.Locators), doc.Locators)
	}
	first := doc.Locators[0]
	if first.Page != 1 || len(first.Coordinates) != 4 {
		t.Fatalf("locator shape wrong: %+v", first)
	}
	// bbox [10,20,100,20,100,40,10,40] must normalize to [10,20,100,40].
	if first.Coordinates[0] != 10 || first.Coordinates[1] != 20 || first.Coordinates[2] != 100 || first.Coordinates[3] != 40 {
		t.Fatalf("box normalization wrong: %v", first.Coordinates)
	}
	if len(first.Quote) == 0 || !strings.HasPrefix(first.Source, "paddleocr:page:1:") {
		t.Fatalf("locator quote/source wrong: %+v", first)
	}
}

func TestPaddleOCRUnavailable(t *testing.T) {
	c := NewPaddleOCR(PaddleOCRConfig{APIURL: "http://x"})
	if c.Available() {
		t.Fatal("client without token must be unavailable")
	}
	_, err := c.Parse(context.Background(), Source{Filename: "a.pdf", Data: []byte("x")})
	if !errors.Is(err, ErrParserUnavailable) {
		t.Fatalf("unavailable OCR must yield ErrParserUnavailable, got %v", err)
	}
}

func TestPaddleOCRFailedJob(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/ocr/jobs/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"jobId": "job-9"}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
			"state":    "failed",
			"errorMsg": "document too damaged",
		}})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := NewPaddleOCR(PaddleOCRConfig{
		APIURL:         srv.URL + "/api/v2/ocr/jobs",
		AccessToken:    "t",
		PollInterval:   5 * time.Millisecond,
		MaxPollSeconds: 5,
	})
	_, err := c.Parse(context.Background(), Source{Filename: "a.pdf", Data: []byte("x")})
	if !errors.Is(err, ErrParserUnavailable) || !strings.Contains(err.Error(), "document too damaged") {
		t.Fatalf("failed job must surface its message as parser_unavailable, got %v", err)
	}
}

func TestParseLayoutPayloadNestedEnvelope(t *testing.T) {
	payload := map[string]any{
		"result": map[string]any{
			"layoutParsingResults": []any{
				map[string]any{
					"markdown": map[string]any{"text": "第一页"},
					"prunedResult": map[string]any{
						"text": "第一页",
						"poly": []any{float64(0), float64(0), float64(10), float64(0), float64(10), float64(10), float64(0), float64(10)},
					},
				},
			},
		},
	}
	md, locs := parseLayoutPayload(payload)
	if !strings.Contains(md, "--- Page 1 ---\n第一页") {
		t.Fatalf("nested envelope markdown wrong: %q", md)
	}
	if len(locs) != 1 || locs[0].Coordinates[2] != 10 {
		t.Fatalf("nested envelope locator wrong: %+v", locs)
	}
}
