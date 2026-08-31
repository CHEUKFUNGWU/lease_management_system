/**
 * agent-universal-pagefill-v1 P0-C：/retail-data-import 的 ?fill= 消费从
 * 页面手写 fetch 迁移到共享 usePageFill hook；tb_fill / plan_fill 两条
 * 通道随后跟进，三条通道现在共用同一条消费缝。
 *
 * GUARD-001：本文件钉「hook 真的生效」——
 *  - 页面调用 usePageFill 且声明 page: "retail-data-import"（hook 的
 *    target_page 校验是安全边界）；
 *  - 手写 fetch 已从页面完全消失（?fill= / ?tb_fill= / ?plan_fill= 三条
 *    通道的 artifact 取数都只在 hook 内）；
 *  - 跨页误投对用户可见（mismatch 提示键）；
 *  - 提示键三语齐全。
 * 把页面的 usePageFill 调用删掉或改回手写 fetch，下面的断言即红。
 */
import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import path from "node:path";

const page = readFileSync(path.join(import.meta.dirname, "page.tsx"), "utf8");

describe("?fill= 消费已迁移到 usePageFill（P0-C）", () => {
  it("页面通过共享 hook 消费，并声明目标页", () => {
    expect(page).toContain("usePageFill({");
    expect(page).toContain('page: "retail-data-import"');
  });

  it("三条通道的手写 artifact 取数已全部消失（ID 提取保留在页面）", () => {
    // ID 提取仍在页面；artifact 取数统一在 usePageFill hook 内。
    expect(page).toContain('.get("fill")');
    expect(page).toContain('.get("tb_fill")');
    expect(page).toContain('.get("plan_fill")');
    const fetchSites = page.match(/ai\/chat\/artifacts\//g) ?? [];
    expect(fetchSites.length).toBe(0);
  });

  it("tb_fill / plan_fill 两条通道也经 usePageFill 消费", () => {
    expect(page).toContain("const tbFill = usePageFill({");
    expect(page).toContain("const planFill = usePageFill({");
  });

  it("零售映射与 TB 列结构建议都有可见的 AI 标识", () => {
    expect(page).toContain("aiSuggestedMapping");
    expect(page).toContain('tbFill.suggestions.column_structure');
    expect(page).toContain('message={t("tb_fill.title", language)}');
    expect(page).not.toContain("setMapping(suggested)");
  });

  it("apply 只读 payload 确认值；建议映射从 suggestions 区取", () => {
    expect(page).toContain("apply: (payload) =>");
    expect(page).toContain("retailFill.suggestions.mapping");
  });

  it("跨页误投对用户可见，不静默丢弃", () => {
    expect(page).toContain('"mismatch"');
    expect(page).toContain("retail_import.fill_mismatch");
  });

  it("mismatch 提示三语齐全", async () => {
    const { dict, t } = await import("../lib/i18n");
    for (const language of ["zh-CN", "zh-HK", "en"] as const) {
      expect(dict["retail_import.fill_mismatch"]?.[language]?.length ?? 0, language).toBeGreaterThan(0);
      expect(t("retail_import.fill_mismatch", language), `renders ${language}`).toBeTruthy();
    }
  });
});
