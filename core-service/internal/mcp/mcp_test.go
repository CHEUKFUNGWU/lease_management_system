package mcp

// RT1-L3-D 单元测试。三条不恒真的核心：
//  1. 权威性：登记为 command 的条目，server 自报 read_only 也不降级
//     （TestBuildDefinitionAuthority）——把优先级改成以 server 自报为准必须红；
//  2. 出站白名单：schema 未声明的字段被丢弃而非透传（TestRebuildArgsDropsUndeclared），
//     把重建改回透传必须红；
//  3. 子进程环境：cmd.Env 是显式白名单，绝不 nil（TestBuildEnvListAllowlist +
//     TestSpawnedServerReceivesOnlyAllowlistedEnv——后者的变异是 ExecClient 改回
//     Env:nil）。

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agenttools"
)

const weatherSchema = `{"type":"object","properties":{"city":{"type":"string"},"date":{"type":"string"}},"required":["city"]}`

func mustRebuild(t *testing.T, provided string) json.RawMessage {
	t.Helper()
	out, err := RebuildArgs(json.RawMessage(weatherSchema), json.RawMessage(provided))
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	return out
}

// ── 出站白名单 ────────────────────────────────────────────────────────────────

func TestRebuildArgsDropsUndeclaredFields(t *testing.T) {
	out := mustRebuild(t, `{"city":"上海","date":"2026-05","tenant_hint":"entity-9","secret_notes":"revenue=42"}`)
	var decoded map[string]any
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatal(err)
	}
	if _, ok := decoded["tenant_hint"]; ok {
		t.Fatalf("undeclared field must be DROPPED by the whitelist boundary, got %s", out)
	}
	if _, ok := decoded["secret_notes"]; ok {
		t.Fatalf("undeclared field must be dropped even when it smells like business data: %s", out)
	}
	if decoded["city"] != "上海" || decoded["date"] != "2026-05" {
		t.Fatalf("declared fields must survive projection, got %s", out)
	}
}

func TestRebuildArgsMissingRequiredRejected(t *testing.T) {
	_, err := RebuildArgs(json.RawMessage(weatherSchema), json.RawMessage(`{"date":"2026-05"}`))
	if err == nil || !strings.Contains(err.Error(), "required") {
		t.Fatalf("missing required argument must be rejected, got %v", err)
	}
}

// 纵深防御：白名单被写宽（schema 声明了租户键）时，扫描仍挡下。
func TestDefenceScanCatchesTenantKeysWhenWhitelistTooWide(t *testing.T) {
	wideSchema := `{"type":"object","properties":{"legal_entity_id":{"type":"string"}}}`
	rebuilt := mustRebuildWith(t, wideSchema, `{"legal_entity_id":"entity-9"}`)
	principal := agenttools.Principal{UserID: "u1", Scope: access.Scope{LegalEntityID: "entity-9"}}
	err := DefenceScan(rebuilt, principal)
	if err == nil || !errors.Is(err, ErrEgressBlocked) {
		t.Fatalf("defence scan must block tenant-identifying keys below a too-wide whitelist, got %v", err)
	}
}

func TestDefenceScanCatchesPrincipalValues(t *testing.T) {
	schema := `{"type":"object","properties":{"note":{"type":"string"}}}`
	rebuilt := mustRebuildWith(t, schema, `{"note":"report for entity-9 prepared by u1"}`)
	principal := agenttools.Principal{UserID: "u1", Scope: access.Scope{LegalEntityID: "entity-9"}}
	if err := DefenceScan(rebuilt, principal); err == nil || !errors.Is(err, ErrEgressBlocked) {
		t.Fatalf("principal identifier values inside declared strings must still be caught (defence in depth), got %v", err)
	}
}

func TestDefenceScanPassesCleanPayloads(t *testing.T) {
	rebuilt := mustRebuild(t, `{"city":"上海","date":"2026-05"}`)
	principal := agenttools.Principal{UserID: "u1", Scope: access.Scope{LegalEntityID: "entity-9"}}
	if err := DefenceScan(rebuilt, principal); err != nil {
		t.Fatalf("clean payload must pass the depth scan: %v", err)
	}
}

func mustRebuildWith(t *testing.T, schema, provided string) json.RawMessage {
	t.Helper()
	out, err := RebuildArgs(json.RawMessage(schema), json.RawMessage(provided))
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	return out
}

// ── 权威性：登记优先于 server 自报 ─────────────────────────────────────────────

func TestBuildDefinitionAuthority(t *testing.T) {
	entry := ToolEntry{
		Name:        "monthly_lock",
		Description: "lock period",
		Level:       "command",
		Permissions: []Permission{{Resource: "monthly_closing", Action: "lock"}},
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
	}
	definition, err := BuildDefinition("external", entry, nil)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	descriptor := definition.Descriptor
	if descriptor.Level != agenttools.LevelCommand {
		t.Fatalf("the MANIFEST level must stand regardless of what the server reports about itself, got level=%v", descriptor.Level)
	}
	if descriptor.ReadOnly {
		t.Fatalf("a command-level registration must not be downgraded to read-only by server self-report")
	}
	if !descriptor.Review.Required {
		t.Fatalf("command-level external tool must demand review exactly like a first-class one: %+v", descriptor.Review)
	}

	// 变异锚：若有人把 BuildDefinition 改成采纳 server 自报的注解/只读标志来
	// 覆盖登记值（level→read / readOnly→true / review 移除），本测试即红。
}

func TestRegisterAllRefusesCommandEntries(t *testing.T) {
	manifest := Manifest{Servers: []ServerEntry{{
		Name: "ext", Command: "/bin/true",
		Tools: []ToolEntry{{
			Name: "t", Level: "command",
			Permissions: []Permission{{Resource: "x", Action: "y"}},
			InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
		}},
	}}}
	registry := agenttools.NewRegistry()
	err := RegisterAll(context.Background(), registry, manifest)
	if err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("v1 policy gate must refuse non-read entries before spawning, got %v", err)
	}
}

// ── 清单校验 ──────────────────────────────────────────────────────────────────

func TestManifestValidationMatrix(t *testing.T) {
	validTool := ToolEntry{
		Name: "t", Permissions: []Permission{{Resource: "mcp", Action: "read"}},
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
	}
	cases := []struct {
		name     string
		manifest Manifest
		wantErr  string
	}{
		{"no servers", Manifest{}, "no servers"},
		{"no command", Manifest{Servers: []ServerEntry{{Name: "s"}}}, "command is required"},
		{"no tools", Manifest{Servers: []ServerEntry{{Name: "s", Command: "/bin/true"}}}, "declares no tools"},
		{"tool without permissions", Manifest{Servers: []ServerEntry{{Name: "s", Command: "/bin/true", Tools: []ToolEntry{{
			Name: "t", InputSchema: json.RawMessage(`{}`),
		}}}}}, "at least one permission"},
		{"bad env key", Manifest{Servers: []ServerEntry{{Name: "s", Command: "/bin/true", Env: map[string]string{"bad-key": "v"}, Tools: []ToolEntry{validTool}}}}, "not a valid variable name"},
	}
	for _, tc := range cases {
		if err := tc.manifest.Validate(); err == nil || !strings.Contains(err.Error(), tc.wantErr) {
			t.Fatalf("%s: expected error containing %q, got %v", tc.name, tc.wantErr, err)
		}
	}
	ok := Manifest{Servers: []ServerEntry{{Name: "s", Command: "/bin/true", Tools: []ToolEntry{validTool}}}}
	if err := ok.Validate(); err != nil {
		t.Fatalf("valid manifest rejected: %v", err)
	}
}

// ── 子进程环境白名单（纯函数半边；spawn 半边在 spawn_test.go）────────────────

func TestBuildEnvListAllowlist(t *testing.T) {
	t.Setenv("DB_PASSWORD", "supersecret")
	t.Setenv("JWT_SECRET", "jwt-secret-value")
	t.Setenv("PATH", "/usr/bin:/bin")

	env := buildEnvList(map[string]string{"MCP_DATA_DIR": "/data/public"})
	joined := strings.Join(env, "\n")

	if !strings.Contains(joined, "PATH=/usr/bin:/bin") {
		t.Fatalf("PATH must be forwarded so bare command names resolve: %s", joined)
	}
	if !strings.Contains(joined, "MCP_DATA_DIR=/data/public") {
		t.Fatalf("manifest-declared variables must be forwarded: %s", joined)
	}
	if strings.Contains(joined, "DB_PASSWORD") || strings.Contains(joined, "JWT_SECRET") {
		t.Fatalf("process secrets must never reach the child environment: %s", joined)
	}
	if len(env) != 2 {
		t.Fatalf("allowlist must be minimal (PATH + declared), got %d entries: %v", len(env), env)
	}
}

// ── 登记级 schema 来源：server 自报放宽 schema 时仍以清单为准 ───────────────────
// 信任边界的行为化表达：catalogue 只负责「工具存在」，不负责 schema。若有人把
// 接线改成采纳 server 的 inputSchema（server 试图让租户标识字段合法化），本测试
// 必须红。
func TestRegisterUsesManifestSchemaNotServerSchema(t *testing.T) {
	entry := ToolEntry{
		Name:        "lookup",
		Description: "lookup",
		Permissions: []Permission{{Resource: "mcp", Action: "lookup"}},
		InputSchema: json.RawMessage(`{"type":"object","properties":{"q":{"type":"string"}},"required":["q"]}`),
	}

	var captured json.RawMessage
	definition, err := buildFromCatalogue("ext", entry, map[string]bool{"lookup": true}, func(_ context.Context, _ string, arguments json.RawMessage) (ToolCallResult, error) {
		captured = arguments
		return ToolCallResult{Text: "ok"}, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx := agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{
			UserID: "u1", Scope: access.Scope{LegalEntityID: "entity-a"},
			Permissions: []string{"mcp:lookup"},
		},
	})
	// 模型把 server 想要的宽字段塞进参数：合法 q + 试图外泄的 tenant 字段。
	_, invokeErr := definition.Handler(ctx, agenttools.ToolCall{
		CallID: "mcp-schema", ToolName: "mcp.ext.lookup", ToolVersion: "v1",
		Arguments: json.RawMessage(`{"q":"上海","legal_entity_id":"entity-a"}`),
	})
	if invokeErr != nil {
		t.Fatal(invokeErr)
	}
	var sent map[string]any
	if err := json.Unmarshal(captured, &sent); err != nil {
		t.Fatal(err)
	}
	if _, ok := sent["legal_entity_id"]; ok {
		t.Fatalf("a server-reported wider schema must NOT win over the manifest — arguments were rebuilt with the manifest schema, got %s", captured)
	}
	if sent["q"] != "上海" {
		t.Fatalf("declared field must survive rebuild: %s", captured)
	}
}
