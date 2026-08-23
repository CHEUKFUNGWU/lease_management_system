// First-party replacement for picoclaw pkg/media: only the store contract the
// vendored channels compile against (upstream media/store.go types, same
// commit). Storage implementations stay out — this repo wires MinIO instead.
package media

import (
	"context"
	"os"
	"path/filepath"
)

// CleanupPolicy controls how the MediaStore treats the underlying file when a
// processing scope ends.
type CleanupPolicy string

const (
	// CleanupPolicyDeleteOnCleanup means the file is store-managed and may be
	// deleted on cleanup.
	CleanupPolicyDeleteOnCleanup CleanupPolicy = "delete_on_cleanup"
	// CleanupPolicyForgetOnly means the store should only drop ref mappings
	// and leave the underlying file alone.
	CleanupPolicyForgetOnly CleanupPolicy = "forget_only"
)

// MediaMeta describes one stored media item.
type MediaMeta struct {
	Filename      string
	ContentType   string
	Source        string        // "telegram", "discord", "tool:image-gen", etc.
	CleanupPolicy CleanupPolicy // defaults to CleanupPolicyDeleteOnCleanup
}

// MediaStore manages the lifecycle of media files associated with processing
// scopes. Store registers an existing local file; Resolve returns its path;
// ResolveWithMeta also returns the metadata; ReleaseAll deletes all files
// registered under the scope, ignoring file-not-exist errors.
type MediaStore interface {
	Store(localPath string, meta MediaMeta, scope string) (string, error)
	Resolve(ref string) (string, error)
	ResolveWithMeta(ref string) (string, MediaMeta, error)
	ReleaseAll(ctx context.Context, scope string) error
}

// TempDir returns the base directory for media scratch files, mirroring
// upstream media/tempdir.go (os.TempDir + "picoclaw-media").
func TempDir() string {
	dir := os.Getenv("PICOCLAW_MEDIA_DIR")
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "picoclaw-media")
	}
	return dir
}
