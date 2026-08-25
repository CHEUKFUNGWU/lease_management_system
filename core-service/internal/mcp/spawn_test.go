package mcp

// RT1-L3-D spawn 路径测试：真实子进程 + 真实 stdio JSON-RPC。
//
// 最重要的一条：TestSpawnedServerReceivesOnlyAllowlistedEnv——把本进程环境
// 故意污染上密钥，spawn 一个把收到的全部环境变量回显成工具结果的假 server，
// 断言密钥名与值都不在回显里。变异自检：ExecClient 把 cmd.Env 改回 nil
// （继承 os.Environ() 的默认写法）时它必须变红。
//
// 假 server 是一段 sh：按位置应答三轮固定 id 的 JSON-RPC（1=initialize、
// 2=tools/list、3=tools/call），env 用 base64 编码塞进 content 规避转义。

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSpawnedServerReceivesOnlyAllowlistedEnv(t *testing.T) {
	// 污染本进程环境：这些是 main.go 里真实存在的密钥名形状。
	const secretDB = "super-secret-db-password"
	const secretJWT = "super-secret-jwt-key"
	t.Setenv("DB_PASSWORD", secretDB)
	t.Setenv("JWT_SECRET", secretJWT)
	t.Setenv("MINIO_SECRET_KEY", "minio-root-secret")

	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "fake_server.sh")
	script := `#!/bin/sh
read init_line
printf '%s\n' '{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-03-26","capabilities":{"tools":{}},"serverInfo":{"name":"fake"}}}'
read note_line
printf '%s\n' '{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"echo_env","inputSchema":{"type":"object","properties":{"q":{"type":"string"}}},"required":["q"]}]}}'
read call_line
b64=$(printf 'DB=%s|JWT=%s|MINIO=%s|ALLOW=%s' "$DB_PASSWORD" "$JWT_SECRET" "$MINIO_SECRET_KEY" "$MCP_FAKE_MODE" | base64)
printf '{"jsonrpc":"2.0","id":3,"result":{"content":[{"type":"text","text":"%s"}]}}\n' "$b64"
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	entry := ServerEntry{
		Name:    "fake",
		Command: scriptPath,
		// 白名单里故意不含任何密钥；server 想看也拿不到。
		Env: map[string]string{"MCP_FAKE_MODE": "echo"},
		Tools: []ToolEntry{{
			Name:        "echo_env",
			Description: "echoes what it received",
			Permissions: []Permission{{Resource: "mcp", Action: "echo"}},
			InputSchema: json.RawMessage(`{"type":"object","properties":{"q":{"type":"string"}},"required":["q"]}`),
		}},
	}

	client, err := Start(entry)
	if err != nil {
		t.Fatalf("start fake server: %v", err)
	}
	defer client.Close()

	catalogue, err := client.ListTools(context.Background())
	if err != nil || len(catalogue) != 1 || catalogue[0].Name != "echo_env" {
		t.Fatalf("tools/list round failed: %v %+v", err, catalogue)
	}

	result, err := client.CallTool(context.Background(), "echo_env", json.RawMessage(`{"q":"hello"}`))
	if err != nil {
		t.Fatalf("call failed: %v", err)
	}
	decoded, decodeErr := base64.StdEncoding.DecodeString(strings.TrimSpace(result.Text))
	if decodeErr != nil {
		t.Fatalf("fake server payload not base64 (script contract broken): %v raw=%q", decodeErr, result.Text)
	}
	echoed := string(decoded)

	// 三槽密钥必须一个都到不了子进程。
	for _, forbidden := range []string{secretDB, secretJWT, "minio-root-secret"} {
		if strings.Contains(echoed, forbidden) {
			t.Fatalf("secret %q reached the external process environment — cmd.Env must be an explicit allowlist, child saw: %s", forbidden, echoed)
		}
	}
	// 密钥名本身也不得出现（值可能碰巧不在，但名出现说明环境泄露路径存在）。
	for _, name := range []string{"DB_PASSWORD", "JWT_SECRET", "MINIO_SECRET_KEY"} {
		if strings.Contains(echoed, name) && !strings.Contains(echoed, name+"=|") && strings.Contains(echoed, name+"=") {
			t.Fatalf("secret variable name %q visible in child env: %s", name, echoed)
		}
	}
	// 白名单内的变量确实到达：回显里应出现 ALLOW=echo。
	if !strings.Contains(echoed, "ALLOW=echo") {
		t.Fatalf("allowlisted variable must be forwarded to the child: %s", echoed)
	}
}

// 超时语义：server 不应答 → 调用按时失败，回合不挂住。
func TestCallToolHonoursDeadline(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "slow_server.sh")
	script := `#!/bin/sh
read init_line
printf '%s\n' '{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-03-26","capabilities":{"tools":{}},"serverInfo":{"name":"slow"}}}'
sleep 30
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	entry := ServerEntry{Name: "slow", Command: scriptPath}
	client, err := Start(entry)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	startedAt := time.Now()
	if _, err := client.ListTools(ctx); err == nil {
		t.Fatal("unresponsive server must surface as error")
	}
	if elapsed := time.Since(startedAt); elapsed > 5*time.Second {
		t.Fatalf("call hung past the deadline: %v", elapsed)
	}
}
