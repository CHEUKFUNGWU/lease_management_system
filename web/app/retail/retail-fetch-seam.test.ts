/**
 * FETCH-001: 取数接缝的机械契约（源码层）。
 *
 * D23 的三个零售页此前对竞态给出三种答案（requestGate / let active /
 * useRef）；接缝必须统一竞态门与 token 注入，错误出口是 STATE-001 的
 * DataState。这里钉住接缝本身的形状；行为由 useRetailQuery 的使用方
 * 测试覆盖。
 */
import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import path from "node:path";

const hook = readFileSync(path.join(import.meta.dirname, "useRetailQuery.ts"), "utf8");
const pulse = readFileSync(path.join(import.meta.dirname, "../operating-pulse/page.tsx"), "utf8");
const store360 = readFileSync(path.join(import.meta.dirname, "../store-360/page.tsx"), "utf8");
const scenario = readFileSync(path.join(import.meta.dirname, "../scenario-workbench/page.tsx"), "utf8");

describe("FETCH-001 retail fetch seam", () => {
  it("hook 统一竞态门（序号 + active 双保险）", () => {
    expect(hook).toContain("seq.current");
    expect(hook).toContain("id !== seq.current");
    expect(hook).toContain("active = false");
  });

  it("hook 注入 token 并产出 STATE-001 DataState", () => {
    expect(hook).toContain("fetcher(paramsRef.current, token)");
    expect(hook).toContain("classifyDataState");
    expect(hook).toContain("DataState<T>");
  });

  it("三个零售页都经接缝取数（不再各自手搓竞态）", () => {
    for (const [name, source] of [["operating-pulse", pulse], ["store-360", store360], ["scenario-workbench", scenario]] as const) {
      expect(source, `${name} uses useRetailQuery`).toContain("useRetailQuery");
    }
    // D23 的三种旧竞态写法不得新增
    for (const [name, source] of [["operating-pulse", pulse], ["store-360", store360]] as const) {
      expect(source, `${name} no longer hand-rolls requestGate`).not.toContain("createLatestRequestGate");
    }
  });
});
