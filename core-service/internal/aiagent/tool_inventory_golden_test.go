package aiagent

// C1/C2 开工基线（架构重构任务书 2026-08-26）：
//
// 工具数与工具面只能运行时枚举（AGENTS.md），本测试用 productionWire()
// 构造生产配置 registry，把 Runtime.Describe 的全量清单落盘为
// testdata/tool-inventory.json。三个重构候选的共同验收底线是
// 「工具枚举数量变化 = 停」：重构前后这份快照必须逐字节一致。
//
// 再生方式：UPDATE_TOOL_INVENTORY=1 go test ./internal/aiagent/ -run TestToolInventoryGolden
// （只在有意变更工具面的专项里再生；重构类改动出现 diff = 立即回报）。

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/lease-management-system/core-service/internal/agenttools"
)

type toolInventoryEntry struct {
	Name    string                  `json:"name"`
	Version string                  `json:"version"`
	Level   string                  `json:"level"`
	Perms   []agenttools.Permission `json:"permissions"`
}

func TestToolInventoryGolden(t *testing.T) {
	agent := productionWire()
	ctx := agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{UserID: "x", Role: "admin", Permissions: []string{"*:*"}},
		RunID:     "x",
	})
	descriptors, err := agent.ToolRuntime().Describe(ctx, agenttools.ToolFilter{})
	if err != nil {
		t.Fatalf("Runtime.Describe: %v", err)
	}
	if len(descriptors) == 0 {
		t.Fatal("empty tool inventory — the wiring must register tools")
	}

	entries := make([]toolInventoryEntry, 0, len(descriptors))
	for _, d := range descriptors {
		perms := append([]agenttools.Permission(nil), d.Permissions...)
		sort.Slice(perms, func(i, j int) bool {
			if perms[i].Resource != perms[j].Resource {
				return perms[i].Resource < perms[j].Resource
			}
			return perms[i].Action < perms[j].Action
		})
		entries = append(entries, toolInventoryEntry{
			Name: d.Name, Version: d.Version, Level: string(d.Level), Perms: perms,
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })

	got, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		t.Fatalf("marshal inventory: %v", err)
	}
	got = append(got, '\n')

	goldenPath := filepath.Join("testdata", "tool-inventory.json")
	if os.Getenv("UPDATE_TOOL_INVENTORY") != "" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("mkdir testdata: %v", err)
		}
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("tool inventory regenerated: %d tools", len(entries))
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden (run once with UPDATE_TOOL_INVENTORY=1 to seed it): %v", err)
	}
	if string(want) != string(got) {
		t.Fatalf("tool inventory drifted from the refactor baseline (%d tools):\nwant:\n%s\ngot:\n%s",
			len(entries), want, got)
	}
	t.Logf("tool inventory matches baseline: %d tools", len(entries))
}
