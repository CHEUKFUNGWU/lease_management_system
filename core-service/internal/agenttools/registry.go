package agenttools

import (
	"slices"

	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// ToolHandler is an application-level Adapter. It receives a validated call
// and a context carrying the authenticated principal; it does not receive a
// database handle or raw HTTP/Shell capability.
type ToolHandler func(context.Context, ToolCall) (ToolResult, error)

type ToolDefinition struct {
	Descriptor ToolDescriptor
	SkillIDs   []string
	Handler    ToolHandler
}

type Registry struct {
	mu    sync.RWMutex
	tools map[string]map[string]ToolDefinition
}

func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]map[string]ToolDefinition)}
}

func (r *Registry) Register(definition ToolDefinition) error {
	if r == nil {
		return fmt.Errorf("%w: registry is nil", ErrInvalidToolDescriptor)
	}
	if err := definition.Descriptor.Validate(); err != nil {
		return err
	}
	if definition.Handler == nil {
		return fmt.Errorf("%w: handler is required", ErrInvalidToolDescriptor)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	versions := r.tools[definition.Descriptor.Name]
	if versions == nil {
		versions = make(map[string]ToolDefinition)
		r.tools[definition.Descriptor.Name] = versions
	}
	if _, exists := versions[definition.Descriptor.Version]; exists {
		return fmt.Errorf("%w: tool %s@%s is already registered", ErrInvalidToolDescriptor, definition.Descriptor.Name, definition.Descriptor.Version)
	}
	versions[definition.Descriptor.Version] = definition
	return nil
}

func (r *Registry) Resolve(name, version string) (ToolDefinition, bool) {
	if r == nil {
		return ToolDefinition{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	versions := r.tools[strings.TrimSpace(name)]
	if versions == nil {
		return ToolDefinition{}, false
	}
	definition, ok := versions[strings.TrimSpace(version)]
	return definition, ok
}

func (r *Registry) definitions(filter ToolFilter) []ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	nameSet := make(map[string]struct{}, len(filter.Names))
	for _, name := range filter.Names {
		nameSet[strings.TrimSpace(name)] = struct{}{}
	}
	levelSet := make(map[ToolLevel]struct{}, len(filter.Levels))
	for _, level := range filter.Levels {
		levelSet[level] = struct{}{}
	}
	var definitions []ToolDefinition
	for _, versions := range r.tools {
		for _, definition := range versions {
			if len(nameSet) > 0 {
				if _, ok := nameSet[definition.Descriptor.Name]; !ok {
					continue
				}
			}
			if len(levelSet) > 0 {
				if _, ok := levelSet[definition.Descriptor.Level]; !ok {
					continue
				}
			}
			if filter.SkillID != "" && !slices.Contains(definition.SkillIDs, filter.SkillID) {
				continue
			}
			definitions = append(definitions, definition)
		}
	}
	sort.Slice(definitions, func(i, j int) bool {
		left, right := definitions[i].Descriptor, definitions[j].Descriptor
		if left.Name == right.Name {
			return left.Version < right.Version
		}
		return left.Name < right.Name
	})
	return definitions
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(wanted)) {
			return true
		}
	}
	return false
}
