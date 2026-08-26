// Package mcp implements the RT1-L3-D v1 Model Context Protocol integration:
// a minimal first-party JSON-RPC client over stdio, plus the registration
// pipeline that turns a reviewed manifest into agenttools definitions.
//
// Trust boundary (L3-D ruling):
//   - Governance attributes (level, read-only, permissions, review) come from
//     the FIRST-PARTY manifest only. Server self-reported metadata is never
//     merged into descriptors — self-registration would be a path around the
//     audit trail and Review Gate.
//   - The outbound boundary is a WHITELIST: arguments are REBUILT from the
//     manifest input schema (undeclared fields are dropped, not scanned-and-
//     rejected). Key-name / principal-value scans remain as defence in depth
//     for "the whitelist was written too wide" — they are not the boundary.
//   - Subprocess environment is an explicit minimal allowlist. exec.Cmd with
//     Env == nil inherits os.Environ(), which would hand production secrets
//     (DB password, JWT key, MinIO root credentials, LLM API keys) to an
//     external process.
//
// v1 scope: read-only tools only; stdio transport only; no third-party MCP
// SDK (the protocol subset used here is initialize + tools/list + tools/call).
package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Manifest is the reviewed registration file. It lives in git so every change
// to what data may leave the process goes through PR review.
type Manifest struct {
	Servers []ServerEntry `json:"servers"`
}

// ServerEntry declares one external MCP server and the tools we register from it.
type ServerEntry struct {
	Name    string            `json:"name"`
	Command string            `json:"command"` // executed directly, never through a shell
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"` // allowlist: ONLY these reach the child
	Tools   []ToolEntry       `json:"tools"`
}

// ToolEntry declares one tool. Level and read-only are NOT fields: v1 registers
// read-only tools only, and BuildDefinition sets them unconditionally (see
// RegisterAll for the policy gate).
type ToolEntry struct {
	Name           string          `json:"name"`
	Description    string          `json:"description"`
	Level          string          `json:"level,omitempty"` // v1: "read" (default) only; RegisterAll refuses others
	Permissions    []Permission    `json:"permissions"`
	InputSchema    json.RawMessage `json:"input_schema"`
	TimeoutSeconds int             `json:"timeout_seconds,omitempty"`
}

type Permission struct {
	Resource string `json:"resource"`
	Action   string `json:"action"`
}

// Validate checks structural rules that must hold before any process spawns:
// non-empty names, direct executables (no shell interpretation), at least one
// declared permission per tool (otherwise any authenticated caller passes
// CapabilityCheck), positive timeout with a sane default, and input schemas
// that never declare an unshaped object (type:"object" without properties),
// because the egress whitelist would have nothing to project against there.
func (m Manifest) Validate() error {
	if len(m.Servers) == 0 {
		return fmt.Errorf("mcp manifest: no servers configured")
	}
	for _, server := range m.Servers {
		if strings.TrimSpace(server.Name) == "" {
			return fmt.Errorf("mcp manifest: server name is required")
		}
		if strings.TrimSpace(server.Command) == "" {
			return fmt.Errorf("mcp manifest: server %q command is required", server.Name)
		}
		if len(server.Tools) == 0 {
			return fmt.Errorf("mcp manifest: server %q declares no tools", server.Name)
		}
		for k := range server.Env {
			if !validEnvKey(k) {
				return fmt.Errorf("mcp manifest: server %q env key %q is not a valid variable name", server.Name, k)
			}
		}
		for _, tool := range server.Tools {
			if strings.TrimSpace(tool.Name) == "" {
				return fmt.Errorf("mcp manifest: tool name is required on server %q", server.Name)
			}
			if len(tool.Permissions) == 0 {
				return fmt.Errorf("mcp manifest: tool %s@%s must declare at least one permission — without it CapabilityCheck passes every authenticated caller", tool.Name, server.Name)
			}
			if tool.TimeoutSeconds < 0 {
				return fmt.Errorf("mcp manifest: tool %s@%s negative timeout", tool.Name, server.Name)
			}
			if err := validateSchemaShape(tool.InputSchema); err != nil {
				return fmt.Errorf("mcp manifest: tool %s@%s input schema: %w", tool.Name, server.Name, err)
			}
		}
	}
	return nil
}

// validateSchemaShape enforces DEFAULT-DENY on property shapes: every
// property must declare a type the egress whitelist knows how to project —
// a scalar, an array (registered v1 simplification: items pass through), or
// an object WITH sub-properties. Refused at registration:
//   - object-typed properties with no sub-properties (nothing to project);
//   - properties with NO type (pre-fix both Validate and projectValue only
//     acted on explicit type:"object", so omitting the field silently
//     bypassed the whitelist entirely);
//   - properties declaring sub-properties but no type (same bypass).
//
// Unknown future type values are refused at egress time by projectValue.
func validateSchemaShape(schema json.RawMessage) error {
	if len(schema) == 0 {
		return nil // RebuildArgs refuses empty schemas at call time
	}
	var node struct {
		Type       string                     `json:"type"`
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(schema, &node); err != nil {
		return fmt.Errorf("schema unreadable: %w", err)
	}
	for _, propSchema := range node.Properties {
		var prop struct {
			Type       string                     `json:"type"`
			Properties map[string]json.RawMessage `json:"properties"`
		}
		if err := json.Unmarshal(propSchema, &prop); err != nil {
			return fmt.Errorf("property schema unreadable: %w", err)
		}
		switch {
		case prop.Type == "":
			if len(prop.Properties) > 0 {
				return fmt.Errorf("property declares sub-properties but no type — an undeclared shape is a silent whitelist bypass; declare \"type\":\"object\"")
			}
			return fmt.Errorf("property declares no type — the whitelist refuses undeclared shapes; declare one of string/number/integer/boolean/array/object")
		case prop.Type == "object":
			if len(prop.Properties) == 0 {
				return fmt.Errorf("object property declares no properties — the whitelist cannot project it; declare the sub-shape explicitly")
			}
			if err := validateSchemaShape(propSchema); err != nil {
				return err
			}
		default:
			if !projectableTypes[prop.Type] {
				return fmt.Errorf("property declares unknown type %q — the whitelist projects string/number/integer/boolean/array/object only", prop.Type)
			}
		}
	}
	return nil
}

func validEnvKey(k string) bool {
	if k == "" {
		return false
	}
	for i, r := range k {
		switch {
		case r == '_':
		case r >= 'A' && r <= 'Z':
		case r >= 'a' && r <= 'z' && i > 0: // lower-case allowed except first char
		case r >= '0' && r <= '9' && i > 0: // digits legal after the first char (OAUTH2_TOKEN)
		default:
			return false
		}
	}
	return true
}

// ToolName renders the registered name: namespaced under mcp.<server>. so the
// audit trail answers "is this external?" without consulting the manifest.
func ToolName(serverName, toolName string) string {
	return fmt.Sprintf("mcp.%s.%s", serverName, toolName)
}

func defaultTimeoutSeconds() int { return 10 }
