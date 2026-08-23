/**
 * RH2 Display Basis Guard 单元测试。
 *
 * 双向断言（工单 R0-1 验收第四条）：equipment→retail_store 必须不可用、
 * retail_store→retail_store 必须可用。只测第一个方向，一个恒返回 false 的
 * 实现也能过；两个方向都断言，恒 true / 恒 false 的实现都活不下来。
 */
import { describe, expect, it } from "vitest";
import { resolveBasis, type Basis } from "./displayBasis";
import { dict } from "./i18n";

describe("resolveBasis", () => {
  it("设备口径进零售语境：不可用，并给出 i18n 原因键", () => {
    const result = resolveBasis("oee_pct", "equipment", "retail_store");
    expect(result).toEqual({ usable: false, reasonKey: "lib.display_basis.mismatch" });
  });

  it("零售口径进零售语境：可用", () => {
    expect(resolveBasis("sales_per_sqm", "retail_store", "retail_store")).toEqual({ usable: true });
  });

  it("同口径自洽：设备口径进设备语境也可用", () => {
    expect(resolveBasis("oee_pct", "equipment", "equipment")).toEqual({ usable: true });
  });

  it("来源口径未知时不可用——不知道是什么口径的数不能贴任何语境标题", () => {
    for (const ctx: Basis[] = ["retail_store", "equipment"]; ctx.length > 0; ctx.shift()) {
      expect(resolveBasis("x", "unknown", ctx[0]!).usable).toBe(false);
      expect(resolveBasis("x", ctx[0]!, "unknown").usable).toBe(false);
    }
  });

  it("reasonKey 指向的文案三语齐全", () => {
    const key = resolveBasis("oee_pct", "equipment", "retail_store");
    if (!key.usable) {
      expect(dict[key.reasonKey]).toBeTruthy();
      expect(dict[key.reasonKey]["zh-CN"]).toBeTruthy();
      expect(dict[key.reasonKey]["zh-HK"]).toBeTruthy();
      expect(dict[key.reasonKey].en).toBeTruthy();
    } else {
      throw new Error("expected unusable");
    }
  });
});
