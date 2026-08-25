package handlers

// RT1-L3-D: MCP registration seam on AIChatHandler. The manifest is a
// reviewed git file; anything that changes what data may leave the process
// goes through PR review. Registration happens once at startup, BEFORE the
// server accepts traffic (main.go calls it before r.Run).
//
// Tools register into the SAME registry the chat and gateway planes resolve,
// so every MCP call crosses the full nine-control governance chain — this is
// the point of D-C5. Failure is fail-fast: a malformed manifest or an
// unreachable server refuses startup rather than booting with a silently
// missing tool (the honest-absence rule, same shape as the aiagent
// registration completeness guard; MCP paths simply live OUTSIDE that
// collector's accounting — their refusal mode is construction-time, never a
// silent skip).

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/lease-management-system/core-service/internal/mcp"
)

// RegisterMCPTools loads the manifest, spawns the declared servers, verifies
// the declared tools exist on them, and registers the definitions into the
// shared runtime registry. It blocks until every server has completed the
// handshake — the caller is expected to log.Fatal on error before serving.
func (h *AIChatHandler) RegisterMCPTools(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("mcp manifest %s: %w", path, err)
	}
	var manifest mcp.Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return fmt.Errorf("mcp manifest %s: %w", path, err)
	}
	if h.toolRuntime == nil {
		return fmt.Errorf("mcp: tool runtime is nil — cannot register")
	}
	if err := mcp.RegisterAll(context.Background(), h.toolRuntime.Registry(), manifest); err != nil {
		return err
	}
	return nil
}
