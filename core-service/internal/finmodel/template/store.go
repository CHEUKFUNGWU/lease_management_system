package template

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// Store is the SM1 seam. Two adapters exist: in-memory (this file, for unit
// and engine tests) and Postgres (repository.SaveStatementTemplate /
// LoadStatementTemplate). Both enforce the same two rules: templates enter
// only through Parse, and a used version is frozen — saving the same
// (name, version) pair again is an error, editing means a new version.
type Store interface {
	Save(ctx context.Context, def TemplateDef) (*Template, error)
	Get(ctx context.Context, name string, version int) (*Template, error)
	List(ctx context.Context) ([]Version, error)
}

// ErrVersionFrozen reports an attempt to overwrite an existing version.
var ErrVersionFrozen = errors.New("template version already exists: edit by creating a new version")

// MemoryStore is the in-memory adapter.
type MemoryStore struct {
	mu     sync.RWMutex
	byName map[string][]*Template // name -> versions (append-only)
	byKey  map[string]*Template   // name@version
}

// NewMemoryStore builds an empty store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{byName: map[string][]*Template{}, byKey: map[string]*Template{}}
}

func (s *MemoryStore) Save(_ context.Context, def TemplateDef) (*Template, error) {
	tmpl, err := Parse(def)
	if err != nil {
		return nil, err
	}
	key := fmt.Sprintf("%s@%d", tmpl.Name, tmpl.Major)
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.byKey[key]; exists {
		return nil, fmt.Errorf("%w: %s", ErrVersionFrozen, key)
	}
	s.byKey[key] = tmpl
	s.byName[tmpl.Name] = append(s.byName[tmpl.Name], tmpl)
	return tmpl, nil
}

func (s *MemoryStore) Get(_ context.Context, name string, version int) (*Template, error) {
	key := fmt.Sprintf("%s@%d", name, version)
	s.mu.RLock()
	defer s.mu.RUnlock()
	tmpl, ok := s.byKey[key]
	if !ok {
		return nil, errors.New("template not found")
	}
	return tmpl, nil
}

func (s *MemoryStore) List(_ context.Context) ([]Version, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Version
	for _, versions := range s.byName {
		for _, tmpl := range versions {
			out = append(out, Version{Name: tmpl.Name, Version: tmpl.Major})
		}
	}
	return out, nil
}
