package mcp

// Registration pipeline: reviewed manifest → agenttools definitions.
//
// Authority rule (L3-D): governance attributes come from the manifest entry
// ALONE. The server's self-reported tool metadata is consulted for exactly
// one thing — verifying the declared tool exists on the server. It is never
// merged into descriptors: a server self-reporting "read_only" cannot
// downgrade a command-level registration, and TestBuildDefinitionAuthority
// pins that. v1 policy gate: RegisterAll accepts read-only entries only, so
// a command-level manifest entry is refused before any process spawns — the
// authority path stays tested at BuildDefinition level for the day the policy
// widens.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lease-management-system/core-service/internal/agenttools"
)

// registrationTimeout bounds the tools/list round inside RegisterAll. The
// caller (main.go → RegisterMCPTools) passes context.Background() and runs
// BEFORE HTTP serving — an unbounded catalogue fetch would be a silent
// startup hang. A server that answers the handshake but stalls here refuses
// startup instead.
const registrationTimeout = 10 * time.Second

// ServerTool is what tools/list reported for one tool. Deliberately opaque:
// only Name is read (existence check). Nothing here feeds descriptors.
type ServerTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// InvokeFunc performs one tools/call round against the server. Splitting this
// from process management lets tests exercise registration and the chain
// without spawning anything.
type InvokeFunc func(ctx context.Context, wireToolName string, arguments json.RawMessage) (ToolCallResult, error)

// BuildDefinition builds the registered definition from the manifest entry.
// Governance attributes are set by THIS package from the entry alone — never
// from server metadata.
func BuildDefinition(serverName string, entry ToolEntry, invoke InvokeFunc) (agenttools.ToolDefinition, error) {
	if strings.TrimSpace(entry.Name) == "" {
		return agenttools.ToolDefinition{}, fmt.Errorf("mcp: tool name required")
	}
	if len(entry.InputSchema) == 0 {
		return agenttools.ToolDefinition{}, fmt.Errorf("mcp: tool %s declared no input schema", entry.Name)
	}
	timeout := entry.TimeoutSeconds
	if timeout <= 0 {
		timeout = defaultTimeoutSeconds()
	}
	permissions := make([]agenttools.Permission, 0, len(entry.Permissions))
	for _, permission := range entry.Permissions {
		permissions = append(permissions, agenttools.Permission{
			Resource: permission.Resource, Action: permission.Action,
		})
	}

	level := agenttools.LevelRead
	readOnly := true
	var review agenttools.ReviewPolicy
	switch strings.ToLower(entry.Level) {
	case "", "read":
		// v1 default and v1 policy ceiling.
	case "command":
		// Reachable only through BuildDefinition directly (RegisterAll
		// refuses command entries). Authority semantics: OUR declaration
		// stands — external tools at this level demand review like any
		// first-class command tool.
		level = agenttools.LevelCommand
		readOnly = false
		review = agenttools.ReviewPolicy{
			Required:      true,
			Reasons:       []string{"external mcp tool requires human review"},
			ConfirmAction: "confirm_mcp_call",
		}
	default:
		return agenttools.ToolDefinition{}, fmt.Errorf("mcp: unsupported level %q", entry.Level)
	}

	wireName := entry.Name // name on the MCP protocol level
	definition := agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name:           ToolName(serverName, entry.Name),
			Version:        "v1",
			Description:    fmt.Sprintf("[mcp:%s] %s", serverName, entry.Description),
			Level:          level,
			ReadOnly:       readOnly,
			Permissions:    permissions,
			TimeoutSeconds: timeout,
			Review:         review,
		},
		Handler: func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
			execution, _ := agenttools.ExecutionContextFromContext(ctx)
			rebuilt, rebuildErr := RebuildArgs(entry.InputSchema, call.Arguments)
			if rebuildErr != nil {
				// RebuildArgs failures are malformed calls (missing required
				// argument, unreadable schema) — never egress violations; it
				// does not wrap ErrEgressBlocked. Egress blocking happens in
				// the DefenceScan below and only there.
				return agenttools.ToolResult{
					CallID: call.CallID,
					Status: agenttools.StatusRejected,
					Error: &agenttools.ToolError{
						Code:    agenttools.ErrorInvalidArguments,
						Message: "arguments do not match the registered input schema",
					},
				}, nil
			}
			if scanErr := DefenceScan(rebuilt, execution.Principal); scanErr != nil {
				logRejection(scanErr)
				return outboundBlockedResult(), nil
			}
			callCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
			defer cancel()
			result, invokeErr := invoke(callCtx, wireName, rebuilt)
			if invokeErr != nil {
				if errors.Is(callCtx.Err(), context.DeadlineExceeded) {
					return agenttools.ToolResult{
						CallID: call.CallID,
						Status: agenttools.StatusFailed,
						Error: &agenttools.ToolError{
							Code:    agenttools.ErrorTimeout,
							Message: "external mcp server did not answer in time",
						},
					}, nil
				}
				return agenttools.ToolResult{
					CallID: call.CallID,
					Status: agenttools.StatusFailed,
					Error: &agenttools.ToolError{
						Code:    agenttools.ErrorBusinessFailure,
						Message: "external mcp tool invocation failed",
					},
				}, nil
			}
			if result.IsError {
				return agenttools.ToolResult{
					CallID: call.CallID,
					Status: agenttools.StatusFailed,
					Error: &agenttools.ToolError{
						Code:    agenttools.ErrorBusinessFailure,
						Message: "external mcp tool reported an error",
					},
				}, nil
			}
			return agenttools.ToolResult{
				CallID: call.CallID,
				Status: agenttools.StatusCompleted,
				Data:   map[string]any{"text": result.Text},
			}, nil
		},
	}
	return definition, nil
}

func logRejection(err error) {
	fmt.Printf("mcp egress blocked: %v\n", err)
}

func outboundBlockedResult() agenttools.ToolResult {
	// RC1 discipline: the code travels explicitly. business_failure +
	// stable authored prefix distinguishes "our boundary blocked this" from
	// "the external tool itself failed" (which answers with timeout /
	// business failure messages above).
	return agenttools.ToolResult{
		Status: agenttools.StatusRejected,
		Error: &agenttools.ToolError{
			Code:    agenttools.ErrorBusinessFailure,
			Message: OutboundBlockedReason,
		},
	}
}

// OutboundBlockedReason is the stable client-facing reason for boundary
// rejections. Ops branches on this prefix to route to the manifest, not the
// server. Full cause goes to server logs only.
const OutboundBlockedReason = "mcp egress policy: arguments carried data the tool is not registered to receive"

// RegisterAll spawns every server, verifies declared tools exist on them, and
// registers definitions into the registry. Policy gate lives HERE: v1 accepts
// read-only entries only, refused before any process spawns.
// buildFromCatalogue builds ONE tool from a manifest entry against the
// server's offered-name set. The entry's schema is the ONLY schema that
// reaches the definition; the catalogue contributes existence only. Splitting
// this out makes the schema-source wiring unit-testable: a server that
// reports a wider schema must still be bound by the manifest one
// (TestRegisterUsesManifestSchemaNotServerSchema); the description makes the
// merge-risk visible at the exact seam where someone could introduce it.
func buildFromCatalogue(serverName string, entry ToolEntry, offered map[string]bool, invoke InvokeFunc) (agenttools.ToolDefinition, error) {
	if !offered[entry.Name] {
		return agenttools.ToolDefinition{}, fmt.Errorf("mcp: tool %s@%s is not offered by the server — refusing registration (no fuzzy matching)", entry.Name, serverName)
	}
	return BuildDefinition(serverName, entry, invoke)
}

func RegisterAll(ctx context.Context, registry interface {
	Register(definition agenttools.ToolDefinition) error
}, manifest Manifest) error {
	if err := manifest.Validate(); err != nil {
		return err
	}
	for _, server := range manifest.Servers {
		for _, entry := range server.Tools {
			if strings.ToLower(entry.Level) != "read" && entry.Level != "" {
				return fmt.Errorf("mcp: tool %s@%s declares level %q — v1 registers read-only external tools only", entry.Name, server.Name, entry.Level)
			}
		}
	}
	for _, server := range manifest.Servers {
		client, err := Start(server)
		if err != nil {
			return fmt.Errorf("mcp: server %q unavailable: %w", server.Name, err)
		}
		listCtx, cancel := context.WithTimeout(ctx, registrationTimeout)
		catalogue, listErr := client.ListTools(listCtx)
		cancel()
		if listErr != nil {
			_ = client.Close()
			return fmt.Errorf("mcp: server %q tools/list failed: %w", server.Name, listErr)
		}
		offered := map[string]bool{}
		for _, tool := range catalogue {
			offered[tool.Name] = true
		}
		for _, entry := range server.Tools {
			// The existence check runs inside buildFromCatalogue against this
			// real offered set — production exercises the same path as the
			// unit tests; no duplicated inline check.
			definition, buildErr := buildFromCatalogue(server.Name, entry, offered, func(callCtx context.Context, wireName string, arguments json.RawMessage) (ToolCallResult, error) {
				return client.CallTool(callCtx, wireName, arguments)
			})
			if buildErr != nil {
				_ = client.Close()
				return fmt.Errorf("mcp: build %s@%s: %w", server.Name, entry.Name, buildErr)
			}
			if registerErr := registry.Register(definition); registerErr != nil {
				_ = client.Close()
				return fmt.Errorf("mcp: register %s: %w", definition.Descriptor.Name, registerErr)
			}
		}
	}
	return nil
}
