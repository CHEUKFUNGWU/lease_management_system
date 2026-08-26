package mcp

// Outbound boundary, RT1-L3-D.
//
// The boundary is a WHITELIST: arguments sent to the external server are
// REBUILT from the manifest input schema — fields the schema does not declare
// are dropped before serialization, never transmitted. Passing the model's
// JSON through and scanning it would be a blacklist; blacklists protect
// against the threats we happened to name, whitelists against everything
// else.
//
// Two scan rules remain BELOW the whitelist as defence in depth: they exist
// for the day someone writes the whitelist too wide (e.g. declares a
// legal_entity_id property). They are not the boundary.

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lease-management-system/core-service/internal/agenttools"
)

// ErrEgressBlocked is returned ONLY when the defence-in-depth scan trips —
// it is the machine-checkable half of the rejection (the client-facing half
// is the stable authored reason in register.go). Malformed calls (missing
// required argument, unreadable schema) are ordinary invalid_arguments, NOT
// egress violations: ops must be able to tell "our boundary blocked this"
// from "the call was malformed" from "the external tool failed".
var ErrEgressBlocked = fmt.Errorf("mcp egress policy")

// bannedArgKeys are tenant-identifier key names. Matching is case-insensitive;
// values are matched exactly against the acting principal's identifiers.
var bannedArgKeys = map[string]bool{
	"legal_entity_id": true,
	"legalentityid":   true,
	"entity_id":       true,
	"user_id":         true,
	"userid":          true,
}

// RebuildArgs projects `provided` onto the manifest schema and returns only
// the declared fields, recursively for object-typed properties. Missing
// required properties are an error (invalid call); undeclared properties are
// silently DROPPED — that drop IS the boundary.
//
// Shape coverage, stated precisely (rework2 N2 — default deny):
//   - scalars (string/number/integer/boolean): passed through when declared;
//   - nested objects: projected against their declared sub-properties at every
//     depth; an object-typed property WITHOUT declared sub-properties projects
//     to {} — nothing undeclared survives;
//   - arrays: the declared array passes through WHOLESALE, item contents are
//     NOT projected (v1 registered simplification — DefenceScan still scans
//     array elements' string values);
//   - properties with NO type or an unknown type value: REFUSED with an
//     error, never passed through. manifest.Validate refuses the same shapes
//     at registration; this is the boundary guarantee if one slips past.
func RebuildArgs(schema json.RawMessage, provided json.RawMessage) (json.RawMessage, error) {
	if len(schema) == 0 {
		return nil, fmt.Errorf("mcp egress: tool declared no input schema")
	}
	var schemaObj struct {
		Type       string                     `json:"type"`
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}
	if err := json.Unmarshal(schema, &schemaObj); err != nil {
		return nil, fmt.Errorf("mcp egress: manifest input schema unreadable: %w", err)
	}
	if len(schemaObj.Properties) == 0 {
		return nil, fmt.Errorf("mcp egress: manifest input schema declares no properties — refusing to forward untyped arguments")
	}

	var providedObj map[string]json.RawMessage
	if len(provided) > 0 {
		if err := json.Unmarshal(provided, &providedObj); err != nil {
			return nil, fmt.Errorf("mcp egress: arguments are not a JSON object")
		}
	}

	required := map[string]bool{}
	for _, key := range schemaObj.Required {
		required[key] = true
	}

	out := make(map[string]json.RawMessage)
	for key, propSchema := range schemaObj.Properties {
		value, present := providedObj[key]
		if !present {
			if required[key] {
				return nil, fmt.Errorf("mcp egress: missing required argument %q", key)
			}
			continue
		}
		projected, err := projectValue(propSchema, value)
		if err != nil {
			return nil, err
		}
		out[key] = projected
	}

	encoded, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("mcp egress: encode rebuilt arguments: %w", err)
	}
	return encoded, nil
}

// projectableTypes is the allowlist of JSON-Schema types the whitelist knows
// how to handle. Anything else — including a MISSING type — is refused:
// "not mentioned" must not be a silent passthrough path.
var projectableTypes = map[string]bool{
	"string": true, "number": true, "integer": true, "boolean": true,
	"array":  true, // whole-value passthrough; item projection is a registered v1 simplification
	"object": true,
}

func projectValue(propSchema json.RawMessage, value json.RawMessage) (json.RawMessage, error) {
	var prop struct {
		Type       string                     `json:"type"`
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(propSchema, &prop); err != nil {
		return nil, fmt.Errorf("mcp egress: property schema unreadable: %w", err)
	}
	// DEFAULT DENY: an undeclared or unknown type is not projected and NOT
	// passed through — it is an error. Pre-fix both this function and manifest
	// validation only acted on explicit type:"object", so omitting the field
	// silently bypassed the entire whitelist (rework2 N2). manifest.Validate
	// refuses these shapes at registration; this refusal is the boundary
	// guarantee if a schema slips past.

	if !projectableTypes[prop.Type] {
		return nil, fmt.Errorf("mcp egress: property declares no known type (got %q) — refusing to forward undeclared shape", prop.Type)
	}
	if prop.Type == "object" {
		var inner map[string]json.RawMessage
		if len(value) > 0 {
			if err := json.Unmarshal(value, &inner); err != nil {
				return nil, fmt.Errorf("mcp egress: object argument is not a JSON object")
			}
		}
		out := make(map[string]json.RawMessage)
		for key, innerSchema := range prop.Properties {
			innerValue, present := inner[key]
			if !present {
				continue
			}
			projected, err := projectValue(innerSchema, innerValue)
			if err != nil {
				return nil, err
			}
			out[key] = projected
		}
		// An object property with NO declared sub-properties projects to {}:
		// the whitelist has nothing to project against, so nothing may pass.
		// (Pre-fix this branch returned the value untouched, letting an entire
		// undeclared subtree out. manifest.Validate refuses the shape anyway;
		// this is the boundary guarantee if a schema slips past.)
		return json.Marshal(out)
	}
	// Scalars and arrays pass through as-is when declared. Array item
	// projection is a known v1 simplification, registered in the delivery.
	return value, nil
}

// DefenceScan checks the REBUILT payload for tenant-identifying keys/values.
// It is defence in depth below the whitelist: if the manifest declares a
// tenant-identifying property, this scan is what still blocks the egress.
func DefenceScan(rebuilt json.RawMessage, principal agenttools.Principal) error {
	var decoded any
	if err := json.Unmarshal(rebuilt, &decoded); err != nil {
		return fmt.Errorf("%w: rebuilt payload unreadable", ErrEgressBlocked)
	}
	var hit string
	walkJSON(decoded, func(key string, value any) {
		if hit != "" {
			return
		}
		if key != "" && bannedArgKeys[strings.ToLower(strings.TrimSpace(key))] {
			hit = fmt.Sprintf("tenant-identifying key %q", key)
			return
		}
		if text, ok := value.(string); ok {
			// Substring match: production identifiers are UUIDs, so
			// false positives are negligible while embedded occurrences
			// ("prepared for <uuid>") are still caught. A rejection is
			// cheap and reviewable; a leak is not.
			if principal.Scope.LegalEntityID != "" && strings.Contains(text, principal.Scope.LegalEntityID) {
				hit = "acting legal entity id"
			} else if principal.UserID != "" && strings.Contains(text, principal.UserID) {
				hit = "acting user id"
			}
		}
	})
	if hit != "" {
		// Detail goes to the server log via the caller; the client-facing
		// message stays generic so probing the boundary teaches nothing.
		return fmt.Errorf("%w: arguments carried %s", ErrEgressBlocked, hit)
	}
	return nil
}

func walkJSON(node any, visit func(key string, value any)) {
	switch typed := node.(type) {
	case map[string]any:
		for key, value := range typed {
			visit(key, value)
			walkJSON(value, visit)
		}
	case []any:
		for _, item := range typed {
			visit("", item) // bare strings inside arrays must be scanned too
			walkJSON(item, visit)
		}
	}
}
