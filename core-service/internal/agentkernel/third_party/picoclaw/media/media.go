// First-party replacement for picoclaw pkg/media: only the store contract the
// vendored agent slice compiles against (types from upstream media/store.go,
// commit bbf6893ca7afad27f1d00a0f5a45982a549c6ed6). Storage implementations stay out of this tree.
package media

import (
	"context"
	"os"
	"path/filepath"
)

type CleanupPolicy string

const (
	CleanupPolicyDeleteOnCleanup CleanupPolicy = "delete_on_cleanup"
	CleanupPolicyForgetOnly      CleanupPolicy = "forget_only"
)

type MediaMeta struct {
	Filename      string
	ContentType   string
	Source        string
	CleanupPolicy CleanupPolicy
}

type MediaStore interface {
	Store(localPath string, meta MediaMeta, scope string) (string, error)
	Resolve(ref string) (string, error)
	ResolveWithMeta(ref string) (string, MediaMeta, error)
	ReleaseAll(ctx context.Context, scope string) error
}

func TempDir() string {
	dir := os.Getenv("PICOCLAW_MEDIA_DIR")
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "picoclaw-media")
	}
	return dir
}
