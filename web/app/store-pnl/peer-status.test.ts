/**
 * F0-3（任务指令：财务视角的 UI/UX 与术语整改）：store-pnl 同业对标状态
 * 不再泄漏英文枚举。GUARD-001：证明新值真的渲染出来——四个后端取值逐一
 * 断言中文输出，并把键集锁回 storepnl/project.go 的 PeerStatus 注释。
 */
import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import path from "node:path";
import { dict } from "../lib/i18n";
import {
  PEER_STATUS_LABEL,
  STORE_PNL_PEER_STATUSES,
  peerStatusLabel,
} from "./options";

const repoRoot = path.join(import.meta.dirname, "../../../");
const projectGo = readFileSync(path.join(repoRoot, "core-service/internal/storepnl/project.go"), "utf8");
const page = readFileSync(path.join(import.meta.dirname, "page.tsx"), "utf8");

describe("F0-3 同业对标状态映射", () => {
  it("四个后端取值各渲染一次中文（GUARD-001 对照表）", () => {
    expect(peerStatusLabel("complete", "zh-CN")).toBe("样本充足");
    expect(peerStatusLabel("insufficient_peers", "zh-CN")).toBe("可比门店不足");
    expect(peerStatusLabel("mixed_currency", "zh-CN")).toBe("币种不可比");
    expect(peerStatusLabel("unavailable", "zh-CN")).toBe("暂无对标");
  });

  it("繁体与英文同样齐全", () => {
    for (const status of STORE_PNL_PEER_STATUSES) {
      const key = PEER_STATUS_LABEL[status];
      expect(dict[key]["zh-HK"].length).toBeGreaterThan(0);
      expect(dict[key].en.length).toBeGreaterThan(0);
      // 界面文案不再是机器枚举本身
      expect(dict[key]["zh-CN"]).not.toBe(status);
    }
  });

  it("键集 = project.go PeerStatus 注释的封闭清单", () => {
    // 头部聚合状态（StorePnl.PeerStatus）的取值全集；行级字段（RowValue）允许空串，不在此列
    const match = /\/\/ (complete \| insufficient_peers \| mixed_currency \| unavailable)/.exec(projectGo);
    expect(match, "project.go PeerStatus value list found").not.toBeNull();
    const backendValues = match![1].split("|").map((v) => v.trim());
    for (const status of STORE_PNL_PEER_STATUSES) {
      expect(backendValues, `backend PeerStatus covers ${status}`).toContain(status);
    }
  });

  it("页面两处渲染点都经 peerStatusLabel，不再裸拼枚举", () => {
    expect(page).toContain("peerStatusLabel(row.peer_status, language)");
    expect(page).toContain("peerStatusLabel(pnl.peer_status, language)");
    expect(page).not.toContain("${pnl.peer_status}`}");
    expect(page).not.toContain("(row.peer_status || \"—\")");
  });
});
