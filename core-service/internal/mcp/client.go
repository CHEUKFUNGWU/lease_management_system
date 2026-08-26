package mcp

// Minimal MCP client: newline-delimited JSON-RPC 2.0 over the stdio pipes of
// a child process. Protocol subset used: initialize, notifications/initialized,
// tools/list, tools/call. No third-party SDK — keeping the audit surface small
// is deliberate (L3-D ruling).

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
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

// handshakeTimeout bounds the initialize round-trip inside Start. A server
// that spawns but never answers must REFUSE startup within this budget, not
// hang it: RegisterMCPTools runs before HTTP serving, so an unbounded
// handshake is a silent, logless startup deadlock.
const handshakeTimeout = 5 * time.Second

// readBufferSize is the bufio window for the child's stdout.
const readBufferSize = 64 << 10

// maxLineBytes caps ONE wire line (JSON-RPC frames are newline-delimited)
// before any parsing. It is the memory bound; the 64 KiB result cap applied
// after parsing is a semantic cap, not a memory one. Worst transient
// allocation per line is ~readBufferSize + maxLineBytes regardless of what
// the (untrusted) server emits; exceeding the cap poisons the client — once
// framing is lost mid-line there is no honest way to resynchronize.
const maxLineBytes = 1 << 20

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Method  string          `json:"method,omitempty"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Client talks to ONE spawned MCP server over its stdio pipes. One client is
// created per SERVER at registration and captured by EVERY tool registered
// from that server (register.go), so it must be safe for CONCURRENT use:
// request ids are allocated under a mutex, writes are serialized, and a
// single long-lived reader goroutine owns the stdout pipe and dispatches
// responses to waiting callers by request id. Calls are multiplexed at the
// dispatch level; a call that hits its deadline abandons only itself — its
// late response is discarded by id and later calls on the same client work.
type Client struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	reader *bufio.Reader

	writeSlot chan struct{} // cap 1: ctx-aware mutual exclusion for stdin writes

	mu      sync.Mutex              // guards nextID, pending, dead, closed
	nextID  int64                   // first request carries id 1 (JSON-RPC 1-based), matching server expectations
	pending map[int64]chan envelope // in-flight requests awaiting their response
	dead    error                   // non-nil once the transport is unrecoverable
	closed  bool                    // Close has been called
}

type envelope struct {
	raw json.RawMessage
	err *rpcError
}

// Start spawns the server process. cmd.Env is ALWAYS an explicit allowlist —
// never nil. exec.Cmd with nil Env inherits os.Environ(), which in this
// process includes the database password, JWT secret, MinIO root credentials
// and LLM API keys; handing those to an external process is exactly what this
// package exists to prevent. Pinned by TestBuildEnvListAllowlist (pure half)
// and TestSpawnedServerReceivesOnlyAllowlistedEnv (real child echoing what it
// actually received).
func Start(server ServerEntry) (*Client, error) {
	if strings.TrimSpace(server.Command) == "" {
		return nil, fmt.Errorf("mcp: server command is required")
	}
	client := &Client{nextID: 1, pending: make(map[int64]chan envelope), writeSlot: make(chan struct{}, 1)}

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
	client.reader = bufio.NewReaderSize(stdout, readBufferSize)
	go client.readLoop()

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

// readLine reads one newline-terminated frame with a HARD byte cap. Unlike
// bufio.Scanner-style buffering, the cap here is enforced WHILE accumulating,
// so a hostile server streaming gigabytes without a newline costs the client
// at most ~maxLineBytes + readBufferSize of memory, not the full payload.
func (c *Client) readLine() ([]byte, error) {
	var buf []byte
	for {
		chunk, err := c.reader.ReadSlice('\n')
		buf = append(buf, chunk...)
		switch {
		case err == nil:
			return buf, nil
		case errors.Is(err, bufio.ErrBufferFull):
			if len(buf) > maxLineBytes {
				return nil, fmt.Errorf("response line exceeds %d bytes", maxLineBytes)
			}
			// Window exhausted without a newline; keep accumulating up to the cap.
		default:
			return nil, err
		}
	}
}

// readLoop is the ONLY reader of the child's stdout. It owns c.reader for its
// entire life — no other goroutine ever touches it, which is what makes
// concurrent calls race-free (the pre-fix design spawned one reader per call;
// after a timeout the abandoned reader kept reading and raced with the next).
func (c *Client) readLoop() {
	for {
		line, err := c.readLine()
		if err != nil {
			c.failPending(fmt.Errorf("transport failed: %v", err))
			// Contain the failure: a dead or frame-breaking server must not
			// linger holding pipes. Idempotent; a no-op if Close already ran.
			_ = c.Close()
			return
		}
		var candidate rpcResponse
		if uerr := json.Unmarshal(line, &candidate); uerr != nil {
			continue // not a JSON-RPC response; outside our protocol subset
		}
		if candidate.Method != "" {
			// A server-initiated REQUEST (sampling/createMessage, roots/list, …)
			// carries method + id and shares our id space; consuming it as a
			// response would hand the caller a nil result. We advertise
			// capabilities:{} so a compliant server sends none — this client's
			// premise is that the server is NOT trusted to be compliant.
			continue
		}
		c.mu.Lock()
		ch, known := c.pending[candidate.ID]
		if known {
			delete(c.pending, candidate.ID)
		}
		c.mu.Unlock()
		if !known {
			continue // notification, or the late reply to an already-abandoned call
		}
		if candidate.Error != nil {
			ch <- envelope{err: candidate.Error}
		} else {
			ch <- envelope{raw: candidate.Result}
		}
	}
}

// failPending marks the transport dead and wakes every in-flight caller with
// the cause. Safe against double-fire: entries are only removed under mu, and
// removal always happens before the sole permitted send.
func (c *Client) failPending(cause error) {
	c.mu.Lock()
	if c.dead == nil {
		c.dead = cause
	}
	abandoned := c.pending
	c.pending = make(map[int64]chan envelope)
	c.mu.Unlock()
	for _, ch := range abandoned {
		ch <- envelope{err: &rpcError{Message: cause.Error()}}
	}
}

func (c *Client) removePending(id int64) {
	c.mu.Lock()
	delete(c.pending, id)
	c.mu.Unlock()
}

func (c *Client) initialize() error {
	params, _ := json.Marshal(map[string]any{
		"protocolVersion": protocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]string{"name": "lease-core-service", "version": "0.1.0"},
	})
	ctx, cancel := context.WithTimeout(context.Background(), handshakeTimeout)
	defer cancel()
	if _, err := c.roundTrip(ctx, "initialize", params); err != nil {
		return fmt.Errorf("handshake did not complete within %s: %w", handshakeTimeout, err)
	}
	// notifications/initialized carries no id and expects no response.
	notification, _ := json.Marshal(rpcRequest{JSONRPC: "2.0", Method: "notifications/initialized"})
	if _, err := c.writeToStdin(ctx, append(notification, '\n')); err != nil {
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
		if total > c.maxResultBytes() {
			return ToolCallResult{Duration: time.Since(startedAt)}, fmt.Errorf("mcp: result exceeds %d bytes", c.maxResultBytes())
		}
	}
	return ToolCallResult{
		Text:     strings.Join(texts, "\n"),
		IsError:  parsed.IsError,
		Duration: time.Since(startedAt),
	}, nil
}

// maxResultBytes is the SEMANTIC cap on tool output handed to the model. The
// MEMORY cap is maxLineBytes, enforced during readLine before any parsing.
func (c *Client) maxResultBytes() int { return 64 << 10 }

// writeToStdin writes one frame under the write slot, interruptible by ctx.
// The slot replaces the plain mutex: acquiring it selects on ctx.Done(), so a
// wedged writer holding the slot cannot stall later callers past THEIR
// deadlines. While the slot is held the deadline is applied to the stdin fd,
// so a server that stopped reading stdin cannot block us past the deadline
// either. Partial writes (deadline fired mid-frame) corrupt wire framing:
// callers must poison the transport in that case.
func (c *Client) writeToStdin(ctx context.Context, payload []byte) (int, error) {
	select {
	case c.writeSlot <- struct{}{}:
	case <-ctx.Done():
		return 0, ctx.Err()
	}
	defer func() { <-c.writeSlot }()

	type deadliner interface{ SetWriteDeadline(time.Time) error }
	if f, ok := c.stdin.(deadliner); ok {
		if dl, hasDL := ctx.Deadline(); hasDL {
			_ = f.SetWriteDeadline(dl)
		} else {
			_ = f.SetWriteDeadline(time.Time{}) // no deadline: unblock the fd
		}
		defer func() { _ = f.SetWriteDeadline(time.Time{}) }()
	}
	return c.stdin.Write(payload)
}

// roundTrip performs one request/response exchange. Concurrency contract:
//
//   - id allocation + pending registration happen atomically under c.mu;
//   - the write itself is serialized AND ctx-interruptible (writeToStdin):
//     one writer at a time, but never past the caller's deadline;
//   - the response is delivered by the single readLoop goroutine, keyed by id.
//
// A caller whose ctx expires removes its pending entry and returns; if the
// response arrives afterwards the readLoop finds no entry and discards it.
// Nothing is left behind: no zombie readers, no leaked channels.
func (c *Client) roundTrip(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
	c.mu.Lock()
	if c.dead != nil {
		err := fmt.Errorf("mcp %s: connection unusable: %v", method, c.dead)
		c.mu.Unlock()
		return nil, err
	}
	id := c.nextID
	c.nextID++
	ch := make(chan envelope, 1)
	c.pending[id] = ch
	c.mu.Unlock()

	request, err := json.Marshal(rpcRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params})
	if err != nil {
		c.removePending(id)
		return nil, err
	}

	n, writeErr := c.writeToStdin(ctx, append(request, '\n'))
	if writeErr != nil {
		c.removePending(id)
		if n > 0 {
			// Partial frame on the wire: whatever follows is desynchronized.
			// Same ruling as an oversized line — framing loss poisons the
			// transport; fail every waiter and contain the process.
			cause := fmt.Errorf("request write aborted after %d/%d bytes: %v — wire framing compromised", n, len(request)+1, writeErr)
			c.failPending(cause)
			go func() { _ = c.Close() }()
			return nil, fmt.Errorf("mcp %s: %v", method, cause)
		}
		return nil, fmt.Errorf("mcp %s: write request: %w", method, writeErr)
	}

	select {
	case got := <-ch:
		if got.err != nil {
			return nil, fmt.Errorf("mcp %s: %s", method, got.err.Message)
		}
		return got.raw, nil
	case <-ctx.Done():
		c.removePending(id)
		return nil, ctx.Err()
	}
}

// Close terminates the child process (whole process group) and reaps it.
// Idempotent. After Close — or after any transport failure — subsequent calls
// fail fast with a named error instead of hanging.
func (c *Client) Close() error {
	c.mu.Lock()
	if c.closed || c.cmd == nil || c.cmd.Process == nil {
		c.closed = true
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()

	_ = c.stdin.Close()
	// Kill the whole process group, then reap: leaking a zombie or an
	// orphaned pipe-holder would stall shutdown and trip go test's WaitDelay.
	if c.cmd.Process.Pid > 0 {
		_ = unix.Kill(-c.cmd.Process.Pid, unix.SIGKILL)
	}
	_ = c.cmd.Wait()
	// Re-sweep the group AFTER reaping the shell. A shell that was fork+exec
	// ing a grandchild when the first sweep landed dies mid-fork; the child
	// can materialize into the group afterwards and miss that signal forever
	// (observed: a `sleep` surviving with ppid=1 in our pgid). Group
	// membership survives reparenting, so this second sweep still reaches it.
	if c.cmd.Process.Pid > 0 {
		_ = unix.Kill(-c.cmd.Process.Pid, unix.SIGKILL)
	}
	return nil
}
