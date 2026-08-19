package miniostore

import (
	"context"
	"strings"
	"testing"
)

func TestEmptyEndpointDisablesClient(t *testing.T) {
	client, err := New(Config{Endpoint: "", Bucket: "b"})
	if err != nil {
		t.Fatal(err)
	}
	if client != nil {
		t.Fatal("empty endpoint must yield a disabled (nil) client")
	}
	reader := NewIngestReader(client)
	if _, err := reader.ReadObject(context.Background(), "x.csv"); err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("disabled client must refuse honestly, got %v", err)
	}
}

func TestMissingBucketFails(t *testing.T) {
	if _, err := New(Config{Endpoint: "minio:9000"}); err == nil {
		t.Fatal("missing bucket must fail construction")
	}
}

func TestUnreachableEndpointFailsGet(t *testing.T) {
	client, err := New(Config{Endpoint: "127.0.0.1:1", AccessKey: "k", SecretKey: "s", Bucket: "lease-uploads"})
	if err != nil {
		t.Fatal(err)
	}
	if client == nil {
		t.Fatal("valid endpoint must build a client")
	}
	if _, err := client.GetObject(context.Background(), "nope.csv"); err == nil {
		t.Fatal("unreachable endpoint must surface an error, not a fabricated object")
	}
}
