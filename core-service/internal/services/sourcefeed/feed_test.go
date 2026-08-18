package sourcefeed

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestSourceFeed_FourAdaptersConsistency(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	env := FeedEnvelope{
		SourceSystem:       "test_pos",
		ImportBatchID:      "batch_001",
		AsOfAt:             now,
		Version:            1,
		DataClassification: "production",
	}

	// 1. Upload Feed (CSV)
	csvData := "store,business_date,currency,revenue,gross_profit\nS001,2026-06-01,CNY,10000.0,4000.0\n"
	uploadFeed := NewUploadFeed([]byte(csvData), env)
	b1, err := uploadFeed.Fetch(ctx, "")
	if err != nil {
		t.Fatalf("upload feed err: %v", err)
	}

	// 2. File Drop Feed (CSV Stream)
	fileDropFeed := NewFileDropFeed(strings.NewReader(csvData), env)
	b2, err := fileDropFeed.Fetch(ctx, "")
	if err != nil {
		t.Fatalf("file drop feed err: %v", err)
	}

	// 3. API Push Feed (JSON records)
	jsonRecords := []map[string]interface{}{
		{
			"store":         "S001",
			"business_date": "2026-06-01",
			"currency":      "CNY",
			"revenue":       "10000.0",
			"gross_profit":  "4000.0",
		},
	}
	pushFeed := NewAPIPushFeed(jsonRecords, env)
	b3, err := pushFeed.Fetch(ctx, "")
	if err != nil {
		t.Fatalf("push feed err: %v", err)
	}

	// 4. Self Service Feed (Nested JSON)
	nestedJSON := `{"data":[{"store":"S001","business_date":"2026-06-01","currency":"CNY","revenue":"10000.0","gross_profit":"4000.0"}]}`
	selfServiceFeed := NewSelfServiceFeed([]byte(nestedJSON), "data", env)
	b4, err := selfServiceFeed.Fetch(ctx, "")
	if err != nil {
		t.Fatalf("self service feed err: %v", err)
	}

	// Verify all four batches have identical rows count and identical source envelope
	batches := []Batch{b1, b2, b3, b4}
	for i, b := range batches {
		if len(b.Rows) != 1 {
			t.Fatalf("feed %d: expected 1 row, got %d", i, len(b.Rows))
		}
		if b.Envelope.SourceSystem != "test_pos" || b.Envelope.ImportBatchID != "batch_001" || b.Envelope.Version != 1 {
			t.Fatalf("feed %d: source envelope corrupted: %+v", i, b.Envelope)
		}
	}
}
