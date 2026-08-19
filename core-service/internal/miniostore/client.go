// Package miniostore is the core-service's read-only MinIO seam: it fetches
// uploaded files for agent-side preview (page-fill). Writes stay with
// ai-service until W5 retires it — this client deliberately offers no
// upload path, keeping the write discipline in one place.
package miniostore

import (
	"context"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Config mirrors the compose defaults so local setups work out of the box.
type Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	Secure    bool
}

// Client fetches objects from one fixed bucket.
type Client struct {
	mc     *minio.Client
	bucket string
}

// New builds the client. An empty endpoint means "disabled" and returns a nil
// client without error — callers keep the honest-refusal path (D-D2).
func New(cfg Config) (*Client, error) {
	if cfg.Endpoint == "" {
		return nil, nil
	}
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("miniostore: bucket is required")
	}
	mc, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.Secure,
	})
	if err != nil {
		return nil, fmt.Errorf("miniostore: %w", err)
	}
	return &Client{mc: mc, bucket: cfg.Bucket}, nil
}

// GetObject reads a whole object by name into memory. Callers cap size at
// their own limits (the ingest template path is ≤10MB by policy).
func (c *Client) GetObject(ctx context.Context, objectName string) ([]byte, error) {
	if c == nil || c.mc == nil {
		return nil, fmt.Errorf("miniostore: client is disabled")
	}
	obj, err := c.mc.GetObject(ctx, c.bucket, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("miniostore get %s: %w", objectName, err)
	}
	defer obj.Close()
	data, err := io.ReadAll(io.LimitReader(obj, 10<<20))
	if err != nil {
		return nil, fmt.Errorf("miniostore read %s: %w", objectName, err)
	}
	if len(data) >= 10<<20 {
		return nil, fmt.Errorf("miniostore read %s: object exceeds the 10MB preview limit", objectName)
	}
	return data, nil
}

// IngestReader adapts the client to the agenttools IgestFileReader seam.
type IngestReader struct {
	client *Client
}

// NewIngestReader wraps a client (nil client = disabled, honest refusal).
func NewIngestReader(client *Client) *IngestReader {
	return &IngestReader{client: client}
}

// ReadObject satisfies agenttools.IngestFileReader.
func (r *IngestReader) ReadObject(ctx context.Context, objectName string) ([]byte, error) {
	return r.client.GetObject(ctx, objectName)
}
