package sourcefeed

import (
	"context"
	"fmt"
	"time"
)

type Cursor string

type FeedEnvelope struct {
	SourceSystem       string    `json:"source_system"`
	ImportBatchID      string    `json:"import_batch_id"`
	AsOfAt             time.Time `json:"as_of_at"`
	Version            int       `json:"version"`
	DataClassification string    `json:"data_classification"`
}

type Batch struct {
	Headers    []string     `json:"headers"`
	Rows       [][]string   `json:"rows"`
	Envelope   FeedEnvelope `json:"envelope"`
	NextCursor Cursor       `json:"next_cursor,omitempty"`
}

// APIPushFeed maps pushed JSON records to a tabular batch. It is the only
// production feed adapter — machine credentials push store-day facts through
// POST /api/v1/retail/push/facts — so the speculative CSV / file-drop / nested
// JSON adapters that only ever served their own tests are not kept here.
type APIPushFeed struct {
	Payload  []map[string]interface{}
	Envelope FeedEnvelope
}

func NewAPIPushFeed(payload []map[string]interface{}, envelope FeedEnvelope) *APIPushFeed {
	return &APIPushFeed{Payload: payload, Envelope: envelope}
}

func (f *APIPushFeed) Fetch(ctx context.Context, cursor Cursor) (Batch, error) {
	if len(f.Payload) == 0 {
		return Batch{Envelope: f.Envelope}, nil
	}

	headerSet := make(map[string]struct{})
	for _, row := range f.Payload {
		for k := range row {
			headerSet[k] = struct{}{}
		}
	}
	var headers []string
	for k := range headerSet {
		headers = append(headers, k)
	}

	rows := make([][]string, len(f.Payload))
	for i, record := range f.Payload {
		row := make([]string, len(headers))
		for j, h := range headers {
			if v, ok := record[h]; ok && v != nil {
				row[j] = fmt.Sprintf("%v", v)
			} else {
				row[j] = ""
			}
		}
		rows[i] = row
	}

	return Batch{
		Headers:  headers,
		Rows:     rows,
		Envelope: f.Envelope,
	}, nil
}
