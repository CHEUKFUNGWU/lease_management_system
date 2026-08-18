package sourcefeed

import (
	"context"
	"testing"
	"time"
)

func TestAPIPushFeed_MapsRecordsAndKeepsEnvelope(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	env := FeedEnvelope{
		SourceSystem:       "test_pos",
		ImportBatchID:      "batch_001",
		AsOfAt:             now,
		Version:            1,
		DataClassification: "production",
	}

	jsonRecords := []map[string]interface{}{
		{
			"store":         "S001",
			"business_date": "2026-06-01",
			"currency":      "CNY",
			"revenue":       "10000.0",
			"gross_profit":  "4000.0",
		},
	}
	feed := NewAPIPushFeed(jsonRecords, env)
	batch, err := feed.Fetch(ctx, "")
	if err != nil {
		t.Fatalf("push feed err: %v", err)
	}

	if len(batch.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(batch.Rows))
	}
	if batch.Envelope.SourceSystem != "test_pos" || batch.Envelope.ImportBatchID != "batch_001" || batch.Envelope.Version != 1 {
		t.Fatalf("source envelope corrupted: %+v", batch.Envelope)
	}
	// Headers are derived from the payload keys; rows line up with headers.
	if len(batch.Headers) != len(batch.Rows[0]) {
		t.Fatalf("headers %v do not line up with row %v", batch.Headers, batch.Rows[0])
	}
}

func TestAPIPushFeed_EmptyPayloadKeepsEnvelope(t *testing.T) {
	batch, err := NewAPIPushFeed(nil, FeedEnvelope{SourceSystem: "test_pos"}).Fetch(context.Background(), "")
	if err != nil {
		t.Fatalf("empty payload err: %v", err)
	}
	if len(batch.Rows) != 0 || batch.Envelope.SourceSystem != "test_pos" {
		t.Fatalf("empty payload must keep envelope, got %+v", batch)
	}
}
