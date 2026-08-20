// Package miniostore is the core-service's read-only MinIO seam: it fetches
// uploaded files for agent-side preview (page-fill). Writes stay with
// ai-service until W5 retires it — this client deliberately offers no
// upload path, keeping the write discipline in one place.
package miniostore

import (
	"bytes"
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

// PutObject writes an uploaded file (W5-5 write seam). It ensures the bucket
// exists and returns a stable object URL-shaped path "/{bucket}/{object}".
func (c *Client) PutObject(ctx context.Context, objectName string, data []byte, contentType string) (string, error) {
	if c == nil || c.mc == nil {
		return "", fmt.Errorf("miniostore: client is disabled")
	}
	exists, err := c.mc.BucketExists(ctx, c.bucket)
	if err != nil {
		return "", fmt.Errorf("miniostore bucket check: %w", err)
	}
	if !exists {
		if err := c.mc.MakeBucket(ctx, c.bucket, minio.MakeBucketOptions{}); err != nil {
			return "", fmt.Errorf("miniostore make bucket: %w", err)
		}
	}
	_, err = c.mc.PutObject(ctx, c.bucket, objectName, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return "", fmt.Errorf("miniostore put %s: %w", objectName, err)
	}
	return "/" + c.bucket + "/" + objectName, nil
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
