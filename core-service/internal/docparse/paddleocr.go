package docparse

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// PaddleOCRConfig configures the PaddleOCR async job client. The endpoint
// contract mirrors the Python client being retired (ai-service/services/
// paddleocr.py): submit a multipart job, poll state, download resultUrl.
type PaddleOCRConfig struct {
	APIURL         string
	AccessToken    string
	Model          string
	PollInterval   time.Duration
	MaxPollSeconds time.Duration
	Client         *http.Client
}

// PaddleOCRClient implements DocumentParser over the PaddleOCR job API.
type PaddleOCRClient struct {
	cfg PaddleOCRConfig
}

// NewPaddleOCR applies defaults: model PaddleOCR-VL-1.5, 2s poll interval,
// 120s max wait, default HTTP client.
func NewPaddleOCR(cfg PaddleOCRConfig) *PaddleOCRClient {
	if cfg.APIURL == "" {
		cfg.APIURL = "https://paddleocr.aistudio-app.com/api/v2/ocr/jobs"
	}
	if cfg.Model == "" {
		cfg.Model = "PaddleOCR-VL-1.5"
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 2 * time.Second
	}
	if cfg.MaxPollSeconds <= 0 {
		cfg.MaxPollSeconds = 120
	}
	if cfg.Client == nil {
		cfg.Client = &http.Client{Timeout: 60 * time.Second}
	}
	return &PaddleOCRClient{cfg: cfg}
}

// Available reports whether OCR can run at all. Callers fall back to anydoc
// when it is not (ADR-0024 §4 honest degradation).
func (c *PaddleOCRClient) Available() bool {
	return c.cfg.AccessToken != "" && c.cfg.APIURL != ""
}

// Parse runs the async pipeline and returns markdown plus block-level
// locators with coordinate evidence.
func (c *PaddleOCRClient) Parse(ctx context.Context, src Source) (ParsedDocument, error) {
	if err := ctx.Err(); err != nil {
		return ParsedDocument{}, err
	}
	if err := CheckSize(src); err != nil {
		return ParsedDocument{}, err
	}
	if !c.Available() {
		return ParsedDocument{}, ErrParserUnavailable
	}

	jobID, err := c.submit(ctx, src)
	if err != nil {
		return ParsedDocument{}, err
	}
	data, err := c.waitForResult(ctx, jobID)
	if err != nil {
		return ParsedDocument{}, err
	}
	payload, err := c.download(ctx, data)
	if err != nil {
		return ParsedDocument{}, err
	}
	markdown, locators := parseLayoutPayload(payload)
	return ParsedDocument{
		Markdown:     markdown,
		Format:       "pdf",
		EvidenceMode: EvidenceCoordinate,
		Locators:     locators,
	}, nil
}

// submit posts the file as multipart form data. Base64 JSON is deliberately
// not used — that variant returns 500 on this API (see AGENTS.md decisions).
func (c *PaddleOCRClient) submit(ctx context.Context, src Source) (string, error) {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if err := mw.WriteField("model", c.cfg.Model); err != nil {
		return "", err
	}
	part, err := mw.CreateFormFile("file", src.Filename)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(src.Data); err != nil {
		return "", err
	}
	if err := mw.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.APIURL, &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "bearer "+c.cfg.AccessToken)

	resp, err := c.cfg.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: submit failed: %v", ErrParserUnavailable, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusTooManyRequests:
		return "", fmt.Errorf("%w: daily quota exhausted (3000 pages)", ErrParserUnavailable)
	case http.StatusForbidden:
		return "", fmt.Errorf("%w: token invalid", ErrParserUnavailable)
	default:
		return "", fmt.Errorf("%w: submit status %d: %s", ErrParserUnavailable, resp.StatusCode, truncate(string(raw), 200))
	}
	var envelope struct {
		Data struct {
			JobID string `json:"jobId"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || envelope.Data.JobID == "" {
		return "", fmt.Errorf("%w: no jobId in submit response", ErrParserUnavailable)
	}
	return envelope.Data.JobID, nil
}

// waitForResult polls the job until done or failed.
func (c *PaddleOCRClient) waitForResult(ctx context.Context, jobID string) (map[string]any, error) {
	deadline := time.Now().Add(time.Duration(c.cfg.MaxPollSeconds) * time.Second)
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("%w: OCR job %s timed out", ErrParserUnavailable, jobID)
		}
		data, err := c.getResult(ctx, jobID)
		if err != nil {
			return nil, err
		}
		switch state(data) {
		case "done":
			return data, nil
		case "failed":
			return nil, fmt.Errorf("%w: OCR job failed: %s", ErrParserUnavailable, stringField(data, "errorMsg"))
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(c.cfg.PollInterval):
		}
	}
}

func (c *PaddleOCRClient) getResult(ctx context.Context, jobID string) (map[string]any, error) {
	url := strings.TrimRight(c.cfg.APIURL, "/") + "/" + jobID
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "bearer "+c.cfg.AccessToken)
	resp, err := c.cfg.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: poll failed: %v", ErrParserUnavailable, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: poll status %d", ErrParserUnavailable, resp.StatusCode)
	}
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || envelope.Data == nil {
		return nil, fmt.Errorf("%w: bad poll response", ErrParserUnavailable)
	}
	return envelope.Data, nil
}

func (c *PaddleOCRClient) download(ctx context.Context, data map[string]any) (map[string]any, error) {
	resultURL, _ := data["resultUrl"].(map[string]any)
	if resultURL == nil {
		return nil, fmt.Errorf("%w: no resultUrl in job result", ErrParserUnavailable)
	}
	jsonURL, _ := resultURL["jsonUrl"].(string)
	if jsonURL == "" {
		return nil, fmt.Errorf("%w: no jsonUrl in job result", ErrParserUnavailable)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jsonURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.cfg.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: download failed: %v", ErrParserUnavailable, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, fmt.Errorf("%w: download read failed: %v", ErrParserUnavailable, err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("%w: bad layout payload", ErrParserUnavailable)
	}
	return payload, nil
}

func state(data map[string]any) string {
	return stringField(data, "state")
}

func stringField(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}

// parseLayoutPayload extracts markdown text and block-level locators from the
// layout parsing envelope, mirroring the Python client's
// _markdown_from_payload / _structured_locators.
func parseLayoutPayload(payload map[string]any) (string, []Locator) {
	var pages []any
	if r, ok := payload["result"].(map[string]any); ok {
		pages, _ = r["layoutParsingResults"].([]any)
	}
	if pages == nil {
		pages, _ = payload["layoutParsingResults"].([]any)
	}

	var b strings.Builder
	var locators []Locator
	for i, raw := range pages {
		page, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		pageNo := i + 1
		if md, ok := page["markdown"].(map[string]any); ok {
			if text, ok := md["text"].(string); ok {
				if b.Len() > 0 {
					b.WriteString("\n\n")
				}
				b.WriteString("--- Page ")
				b.WriteString(strconv.Itoa(pageNo))
				b.WriteString(" ---\n")
				b.WriteString(text)
			}
		}
		root := page
		if pruned, ok := page["prunedResult"].(map[string]any); ok {
			root = pruned
		}
		locators = append(locators, walkLocators(root, pageNo, "")...)
	}
	return b.String(), dedupeLocators(locators)
}

var textKeys = map[string]bool{"block_content": true, "content": true, "text": true, "rec_text": true, "label": true}
var boxKeys = map[string]bool{"block_bbox": true, "bbox": true, "box": true, "coordinate": true, "coordinates": true, "poly": true}
var skipKeys = map[string]bool{"markdown": true, "inputImage": true, "outputImages": true}

func walkLocators(node map[string]any, pageNo int, path string) []Locator {
	var out []Locator
	var text string
	var coords []int
	for k, v := range node {
		childPath := path + "." + k
		switch t := v.(type) {
		case string:
			if textKeys[k] && text == "" {
				text = t
			}
		case []any:
			if boxKeys[k] && coords == nil {
				coords = flattenNumbers(t)
			}
			for _, item := range t {
				if m, ok := item.(map[string]any); ok {
					out = append(out, walkLocators(m, pageNo, childPath+"[]")...)
				}
			}
		case map[string]any:
			if skipKeys[k] {
				continue
			}
			out = append(out, walkLocators(t, pageNo, childPath)...)
		}
	}
	if text != "" && len(coords) >= 4 {
		out = append(out, Locator{
			Page:        pageNo,
			Coordinates: normalizeBox(coords),
			Quote:       truncate(text, 1000),
			Source:      "paddleocr:page:" + strconv.Itoa(pageNo) + ":" + path,
		})
	}
	return out
}

func flattenNumbers(items []any) []int {
	var out []int
	for _, it := range items {
		switch n := it.(type) {
		case float64:
			out = append(out, int(n))
		case int:
			out = append(out, n)
		case json.Number:
			if v, err := n.Int64(); err == nil {
				out = append(out, int(v))
			}
		}
	}
	return out
}

// normalizeBox reduces any flat point list (4-point, N-point) to
// [minX, minY, maxX, maxY].
func normalizeBox(nums []int) []int {
	minX, minY := nums[0], nums[1]
	maxX, maxY := nums[0], nums[1]
	for i := 0; i+1 < len(nums); i += 2 {
		if nums[i] < minX {
			minX = nums[i]
		}
		if nums[i] > maxX {
			maxX = nums[i]
		}
		if nums[i+1] < minY {
			minY = nums[i+1]
		}
		if nums[i+1] > maxY {
			maxY = nums[i+1]
		}
	}
	return []int{minX, minY, maxX, maxY}
}

func dedupeLocators(in []Locator) []Locator {
	seen := map[string]bool{}
	var out []Locator
	for _, l := range in {
		key := fmt.Sprintf("%d|%v|%s", l.Page, l.Coordinates, l.Quote)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, l)
	}
	return out
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
