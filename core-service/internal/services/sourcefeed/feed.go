package sourcefeed

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
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

type SourceFeed interface {
	Fetch(ctx context.Context, cursor Cursor) (Batch, error)
}

// ----------------------------------------------------------------------
// 1. UploadFeed: Takes raw CSV data stream
// ----------------------------------------------------------------------
type UploadFeed struct {
	Data     []byte
	Envelope FeedEnvelope
}

func NewUploadFeed(data []byte, envelope FeedEnvelope) *UploadFeed {
	return &UploadFeed{Data: data, Envelope: envelope}
}

func (f *UploadFeed) Fetch(ctx context.Context, cursor Cursor) (Batch, error) {
	r := csv.NewReader(bytes.NewReader(f.Data))
	records, err := r.ReadAll()
	if err != nil {
		return Batch{}, fmt.Errorf("parse upload csv: %w", err)
	}
	if len(records) == 0 {
		return Batch{Envelope: f.Envelope}, nil
	}
	return Batch{
		Headers:  records[0],
		Rows:     records[1:],
		Envelope: f.Envelope,
	}, nil
}

// ----------------------------------------------------------------------
// 2. APIPushFeed: Receives pushed JSON records payload
// ----------------------------------------------------------------------
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

// ----------------------------------------------------------------------
// 3. FileDropFeed: Reads file dropped in storage
// ----------------------------------------------------------------------
type FileDropFeed struct {
	Content  io.Reader
	Envelope FeedEnvelope
}

func NewFileDropFeed(r io.Reader, envelope FeedEnvelope) *FileDropFeed {
	return &FileDropFeed{Content: r, Envelope: envelope}
}

func (f *FileDropFeed) Fetch(ctx context.Context, cursor Cursor) (Batch, error) {
	r := csv.NewReader(f.Content)
	records, err := r.ReadAll()
	if err != nil {
		return Batch{}, fmt.Errorf("read file drop csv: %w", err)
	}
	if len(records) == 0 {
		return Batch{Envelope: f.Envelope}, nil
	}
	return Batch{
		Headers:  records[0],
		Rows:     records[1:],
		Envelope: f.Envelope,
	}, nil
}

// ----------------------------------------------------------------------
// 4. SelfServiceFeed: Extracts tabular batch from configured JSON response
// ----------------------------------------------------------------------
type SelfServiceFeed struct {
	JSONBody []byte
	DataPath string
	Envelope FeedEnvelope
}

func NewSelfServiceFeed(body []byte, dataPath string, envelope FeedEnvelope) *SelfServiceFeed {
	return &SelfServiceFeed{JSONBody: body, DataPath: dataPath, Envelope: envelope}
}

func (f *SelfServiceFeed) Fetch(ctx context.Context, cursor Cursor) (Batch, error) {
	var raw interface{}
	if err := json.Unmarshal(f.JSONBody, &raw); err != nil {
		return Batch{}, fmt.Errorf("unmarshal self service json: %w", err)
	}

	var list []map[string]interface{}
	switch v := raw.(type) {
	case []interface{}:
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				list = append(list, m)
			}
		}
	case map[string]interface{}:
		target := v
		if f.DataPath != "" {
			if nested, ok := v[f.DataPath].([]interface{}); ok {
				for _, item := range nested {
					if m, ok := item.(map[string]interface{}); ok {
						list = append(list, m)
					}
				}
			}
		} else {
			list = append(list, target)
		}
	}

	pushFeed := NewAPIPushFeed(list, f.Envelope)
	return pushFeed.Fetch(ctx, cursor)
}
