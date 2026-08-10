package agentrunner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lease-management-system/core-service/internal/agenttools"
)

var ErrCheckpointNotFound = errors.New("agent runner checkpoint not found")

const CheckpointSchemaVersion = "agent-checkpoint.v1"

type Checkpoint struct {
	SchemaVersion string                  `json:"schema_version"`
	RunID         string                  `json:"run_id"`
	SessionID     string                  `json:"session_id,omitempty"`
	SkillID       string                  `json:"skill_id,omitempty"`
	SkillVersion  string                  `json:"skill_version,omitempty"`
	Message       string                  `json:"message,omitempty"`
	Plan          []agenttools.ToolCall   `json:"plan"`
	NextIndex     int                     `json:"next_index"`
	ToolResults   []agenttools.ToolResult `json:"tool_results"`
	Status        string                  `json:"status"`
	UpdatedAt     time.Time               `json:"updated_at"`
}

type CheckpointStore interface {
	Save(context.Context, Checkpoint) error
	Load(context.Context, string) (Checkpoint, error)
}

type MemoryCheckpointStore struct {
	mu     sync.RWMutex
	values map[string]Checkpoint
}

func NewMemoryCheckpointStore() *MemoryCheckpointStore {
	return &MemoryCheckpointStore{values: make(map[string]Checkpoint)}
}

func (s *MemoryCheckpointStore) Save(_ context.Context, checkpoint Checkpoint) error {
	if s == nil {
		return errors.New("checkpoint store is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	checkpoint.Plan = append([]agenttools.ToolCall(nil), checkpoint.Plan...)
	checkpoint.ToolResults = append([]agenttools.ToolResult(nil), checkpoint.ToolResults...)
	s.values[checkpoint.RunID] = checkpoint
	return nil
}

func (s *MemoryCheckpointStore) Load(_ context.Context, runID string) (Checkpoint, error) {
	if s == nil {
		return Checkpoint{}, ErrCheckpointNotFound
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	checkpoint, ok := s.values[runID]
	if !ok {
		return Checkpoint{}, ErrCheckpointNotFound
	}
	checkpoint.Plan = append([]agenttools.ToolCall(nil), checkpoint.Plan...)
	checkpoint.ToolResults = append([]agenttools.ToolResult(nil), checkpoint.ToolResults...)
	return checkpoint, nil
}

// JSONFileCheckpointStore is an explicit, caller-owned durable checkpoint
// adapter. It is useful for a single Runner worker or a local CLI process; a
// multi-instance deployment should replace it with a transactional store.
// Each run is written to a hashed filename, preventing run IDs from becoming
// path traversal input.
type JSONFileCheckpointStore struct {
	Directory string
	mu        sync.Mutex
}

func NewJSONFileCheckpointStore(directory string) (*JSONFileCheckpointStore, error) {
	directory = strings.TrimSpace(directory)
	if directory == "" {
		return nil, errors.New("checkpoint directory is required")
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, fmt.Errorf("create checkpoint directory: %w", err)
	}
	return &JSONFileCheckpointStore{Directory: directory}, nil
}

func (s *JSONFileCheckpointStore) Save(ctx context.Context, checkpoint Checkpoint) error {
	if s == nil || strings.TrimSpace(s.Directory) == "" {
		return errors.New("checkpoint store is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(checkpoint.RunID) == "" {
		return errors.New("checkpoint run ID is required")
	}
	checkpoint.Plan = append([]agenttools.ToolCall(nil), checkpoint.Plan...)
	checkpoint.ToolResults = append([]agenttools.ToolResult(nil), checkpoint.ToolResults...)
	raw, err := json.Marshal(checkpoint)
	if err != nil {
		return fmt.Errorf("encode checkpoint: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	temporary, err := os.CreateTemp(s.Directory, ".agent-checkpoint-*")
	if err != nil {
		return fmt.Errorf("create checkpoint temp file: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("protect checkpoint temp file: %w", err)
	}
	if _, err := temporary.Write(raw); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write checkpoint: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close checkpoint temp file: %w", err)
	}
	if err := os.Rename(temporaryName, s.filename(checkpoint.RunID)); err != nil {
		return fmt.Errorf("commit checkpoint: %w", err)
	}
	return nil
}

func (s *JSONFileCheckpointStore) Load(ctx context.Context, runID string) (Checkpoint, error) {
	if s == nil || strings.TrimSpace(s.Directory) == "" {
		return Checkpoint{}, ErrCheckpointNotFound
	}
	if err := ctx.Err(); err != nil {
		return Checkpoint{}, err
	}
	raw, err := os.ReadFile(s.filename(runID))
	if errors.Is(err, os.ErrNotExist) {
		return Checkpoint{}, ErrCheckpointNotFound
	}
	if err != nil {
		return Checkpoint{}, fmt.Errorf("read checkpoint: %w", err)
	}
	var checkpoint Checkpoint
	if err := json.Unmarshal(raw, &checkpoint); err != nil {
		return Checkpoint{}, fmt.Errorf("decode checkpoint: %w", err)
	}
	if strings.TrimSpace(checkpoint.RunID) == "" || checkpoint.RunID != runID {
		return Checkpoint{}, errors.New("checkpoint run ID mismatch")
	}
	return checkpoint, nil
}

func (s *JSONFileCheckpointStore) filename(runID string) string {
	digest := sha256.Sum256([]byte(strings.TrimSpace(runID)))
	return filepath.Join(s.Directory, hex.EncodeToString(digest[:])+".json")
}

// CompactContext preserves only explicitly verified Tool results. Unverified
// model prose is never silently promoted to a system fact.
type ContextItem struct {
	Text         string
	Verified     bool
	EvidenceRefs []string
}

type CompactedContext struct {
	VerifiedFacts []ContextItem `json:"verified_facts"`
	Inferences    []ContextItem `json:"inferences"`
}

func CompactContext(items []ContextItem) CompactedContext {
	result := CompactedContext{VerifiedFacts: []ContextItem{}, Inferences: []ContextItem{}}
	for _, item := range items {
		if item.Verified && len(item.EvidenceRefs) > 0 {
			result.VerifiedFacts = append(result.VerifiedFacts, item)
			continue
		}
		item.Verified = false
		result.Inferences = append(result.Inferences, item)
	}
	return result
}
