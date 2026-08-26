package handlers

// C3 golden 锚（架构重构任务书 2026-08-26）：
//
// 把 retail_ingest.go 的编排收拢进 retailingest.IngestBatch 接缝之前，先把
// preview / commit 的 HTTP 响应逐字节钉死。挂钟时间字段（batch 的
// as_of_at / fact_version 等）由 nowUTC 注入，不属于契约，scrub 后比较；
// 其余任何字节——键序、错误文案、envelope 三元组、幂等标记——漂了就是漂了。
//
// 再生：UPDATE_INGEST_GOLDEN=1 go test ./internal/handlers/ -run TestRetailIngestGolden

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/retailkpi"
)

// ingestGoldenFacts is the fixed fixture: two clean rows for S001 plus one
// row whose revenue is negative (deterministic row error), exercising the
// report error path alongside the happy path.
const ingestGoldenCSV = "门店编号,日期,币种,营业额\nS001,2026-07-01,CNY,100\nS001,2026-07-02,CNY,101\nS001,2026-07-03,CNY,-5\n"

// scrubVolatile drops wall-clock fields so two runs of the same request are
// byte-comparable. These keys are data, not contract: they change per run.
func scrubVolatile(t *testing.T, body []byte) []byte {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if batch, ok := doc["batch"].(map[string]any); ok {
		delete(batch, "as_of_at")
		delete(batch, "fact_version")
		delete(batch, "created_at")
		delete(batch, "updated_at")
	}
	normalized, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("re-marshal: %v", err)
	}
	return normalized
}

// compareWithGolden compares scrubbed bytes against testdata/golden/<name>.
func compareWithGolden(t *testing.T, name string, body []byte) {
	t.Helper()
	goldenPath := filepath.Join("testdata", "golden", name)
	if os.Getenv("UPDATE_INGEST_GOLDEN") != "" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(goldenPath, body, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden %s (seed once with UPDATE_INGEST_GOLDEN=1): %v", goldenPath, err)
	}
	if string(want) != string(body) {
		t.Fatalf("ingest golden drift in %s (HTTP contract changed):\nwant:\n%s\ngot:\n%s", name, want, body)
	}
}

func goldenPopulation() *fakeIngestPopulation {
	return &fakeIngestPopulation{stores: []retailkpi.StorePopulation{
		{StoreID: "11111111-1111-1111-1111-111111111111", StoreCode: "S001", StoreName: "一号店"},
	}}
}

// replayingStore fakes the second push of the same Idempotency-Key: the batch
// row already finalized as completed with accepted rows.
type replayingStore struct {
	fakeIngestStore
}

func (f *replayingStore) CreateBatch(_ context.Context, batch *repository.OperatingFactBatch) (*repository.OperatingFactBatch, error) {
	batch.ID = "batch-replay-1"
	batch.Status = "completed"
	batch.AcceptedRows = 2
	batch.RejectedRows = 0
	return batch, nil
}

func TestRetailIngestPreviewGolden(t *testing.T) {
	handler := newIngestHandler()
	body, contentType := newImportMultipart(t, ingestGoldenCSV, map[string]string{"source_system": "pos"})
	c, recorder := newIngestTestContext(t, body, contentType)
	handler.Preview(c)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	compareWithGolden(t, "ingest_preview.json", scrubVolatile(t, recorder.Body.Bytes()))
}

func TestRetailIngestCommitGolden(t *testing.T) {
	store := &fakeIngestStore{}
	handler := NewRetailIngestHandler(goldenPopulation(), store, nil)
	body, contentType := newImportMultipart(t, ingestGoldenCSV, map[string]string{"source_system": "pos", "as_of_at": "2026-08-16"})
	c, recorder := newIngestTestContext(t, body, contentType)
	c.Request.Header.Set("Idempotency-Key", "golden-key-1")
	handler.Commit(c)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	compareWithGolden(t, "ingest_commit.json", scrubVolatile(t, recorder.Body.Bytes()))
}

func TestRetailIngestCommitReplayGolden(t *testing.T) {
	handler := NewRetailIngestHandler(goldenPopulation(), &replayingStore{}, nil)
	body, contentType := newImportMultipart(t, ingestGoldenCSV, map[string]string{"source_system": "pos", "as_of_at": "2026-08-16"})
	c, recorder := newIngestTestContext(t, body, contentType)
	c.Request.Header.Set("Idempotency-Key", "golden-key-1")
	handler.Commit(c)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	compareWithGolden(t, "ingest_commit_replay.json", scrubVolatile(t, recorder.Body.Bytes()))
}
