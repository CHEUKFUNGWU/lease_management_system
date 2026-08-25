package mcp

// Minimal MCP client: newline-delimited JSON-RPC 2.0 over the stdio pipes of
// a child process. Protocol subset used: initialize, notifications/initialized,
// tools/list, tools/call. No third-party SDK — keeping the audit surface small
// is deliberate (L3-D ruling).

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

const protocolVersion = "2025-03-26"

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Client talks to one spawned MCP server. Safe for serialized use; v1 does
// not multiplex concurrent calls on one server (each registration is a
// distinct client, and scheduled/interactive traffic is far below the need).
type Client struct {
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	reader   *bufio.Reader
	writeMu  sync.Mutex
	nextID   int64
	maxBytes int
}

// Start spawns the server process. cmd.Env is ALWAYS an explicit allowlist —
// never nil. exec.Cmd with nil Env inherits os.Environ(), which in this
// process includes the database password, JWT secret, MinIO root credentials
// and LLM API keys; handing those to an external process is exactly what this
// package exists to prevent. TestExecEnvAllowlist pins this.
func Start(server ServerEntry) (*Client, error) {
	if strings.TrimSpace(server.Command) == "" {
		return nil, fmt.Errorf("mcp: server command is required")
	}
	client := &Client{nextID: 1, maxBytes: defaultMaxBytes()}

	cmd := exec.Command(server.Command, server.Args...) // direct exec; no shell interpretation
	cmd.Env = buildEnvList(server.Env)
	cmd.Stderr = os.Stderr // server diagnostics land in our logs, not a black hole
	// Own process group so Close can kill the WHOLE tree (a shell's children
	// inherit the pipes; killing only the shell would leave them holding the
	// stdout write end — go test then waits out WaitDelay and FAILs the run).
	cmd.SysProcAttr = &unix.SysProcAttr{Setpgid: true}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("mcp: stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("mcp: stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("mcp: start %q: %w", server.Command, err)
	}
	client.cmd = cmd
	client.stdin = stdin
	client.reader = bufio.NewReaderSize(stdout, maxScannerBuffer)

	if err := client.initialize(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("mcp: initialize %q: %w", server.Name, err)
	}
	return client, nil
}

func buildEnvList(extra map[string]string) []string {
	// Minimal allowlist: PATH so bare command names resolve, plus ONLY the
	// variables the reviewed manifest declares for this server.
	env := []string{"PATH=" + os.Getenv("PATH")}
	for key, value := range extra {
		env = append(env, key+"="+value)
	}
	return env
}

const maxScannerBuffer = 1 << 20

func defaultMaxBytes() int { return 64 << 10 }

func (c *Client) initialize() error {
	params, _ := json.Marshal(map[string]any{
		"protocolVersion": protocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]string{"name": "lease-core-service", "version": "0.1.0"},
	})
	if _, err := c.roundTrip(context.Background(), "initialize", params); err != nil {
		return err
	}
	// notifications/initialized carries no id and expects no response.
	notification, _ := json.Marshal(rpcRequest{JSONRPC: "2.0", Method: "notifications/initialized"})
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if _, err := c.stdin.Write(append(notification, '\n')); err != nil {
		return fmt.Errorf("write initialized notification: %w", err)
	}
	return nil
}

// ListTools returns the server's tool catalogue. Used at registration to
// verify declared tools exist and to record their schemas for reference —
// NEVER for governance attributes (those come from the manifest alone).
func (c *Client) ListTools(ctx context.Context) ([]struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}, error) {
	result, err := c.roundTrip(ctx, "tools/list", json.RawMessage("{}"))
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Tools []struct {
			Name        string          `json:"name"`
			Description string          `json:"description"`
			InputSchema json.RawMessage `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(result, &parsed); err != nil {
		return nil, fmt.Errorf("parse tools/list: %w", err)
	}
	return parsed.Tools, nil
}

type ToolCallResult struct {
	Text     string
	IsError  bool
	Duration time.Duration
}

// CallTool invokes one tool with REBUILT arguments (see RebuildArgs — never
// pass model JSON straight through). The context carries the timeout from the
// manifest entry.
func (c *Client) CallTool(ctx context.Context, name string, arguments json.RawMessage) (ToolCallResult, error) {
	startedAt := time.Now()
	params, _ := json.Marshal(map[string]any{"name": name, "arguments": json.RawMessage(arguments)})
	result, err := c.roundTrip(ctx, "tools/call", params)
	if err != nil {
		return ToolCallResult{Duration: time.Since(startedAt)}, err
	}
	var parsed struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(result, &parsed); err != nil {
		return ToolCallResult{Duration: time.Since(startedAt)}, fmt.Errorf("parse tools/call result: %w", err)
	}
	texts := make([]string, 0, len(parsed.Content))
	total := 0
	for _, item := range parsed.Content {
		if item.Type == "text" {
			texts = append(texts, item.Text)
			total += len(item.Text)
		}
		if total > c.maxBytes {
			return ToolCallResult{Duration: time.Since(startedAt)}, fmt.Errorf("mcp: result exceeds %d bytes", c.maxBytes)
		}
	}
	return ToolCallResult{
		Text:     strings.Join(texts, "\n"),
		IsError:  parsed.IsError,
		Duration: time.Since(startedAt),
	}, nil
}

func (c *Client) roundTrip(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
	id := c.nextID
	c.nextID++ // first request carries id 1 (JSON-RPC 1-based), matching server expectations
	request, err := json.Marshal(rpcRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params})
	if err != nil {
		return nil, err
	}

	type envelope struct {
		raw json.RawMessage
		err *rpcError
	}
	responses := make(chan envelope, 1)
	go func() {
		c.writeMu.Lock()
		if _, writeErr := c.stdin.Write(append(request, '\n')); writeErr != nil {
			c.writeMu.Unlock()
			responses <- envelope{err: &rpcError{Message: writeErr.Error()}}
			return
		}
		c.writeMu.Unlock()
		for {
			line, readErr := c.reader.ReadString('\n')
			if readErr != nil {
				responses <- envelope{err: &rpcError{Message: fmt.Sprintf("read response: %v", readErr)}}
				return
			}
			var candidate rpcResponse
			if unmarshalErr := json.Unmarshal([]byte(line), &candidate); unmarshalErr != nil || candidate.ID != id {
				continue // notification or unrelated response
			}
			if candidate.Error != nil {
				responses <- envelope{err: candidate.Error}
				return
			}
			responses <- envelope{raw: candidate.Result}
			return
		}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case got := <-responses:
		if got.err != nil {
			return nil, fmt.Errorf("mcp %s: %s", method, got.err.Message)
		}
		return got.raw, nil
	}
}

// Close terminates the child process. A crashed child is detected by the next
// call failing (broken pipe / EOF), which surfaces as a failed tool result —
// the turn wraps up instead of hanging.
func (c *Client) Close() error {
	if c == nil || c.cmd == nil || c.cmd.Process == nil {
		return nil
	}
	_ = c.stdin.Close()
	// Kill the whole process group, then reap: leaking a zombie or an
	// orphaned pipe-holder would stall shutdown and trip go test's WaitDelay.
	if c.cmd.Process.Pid > 0 {
		_ = unix.Kill(-c.cmd.Process.Pid, unix.SIGKILL)
	}
	_ = c.cmd.Wait()
	return nil
}
