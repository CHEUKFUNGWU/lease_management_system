/**
 * F0-2（任务指令：财务视角的 UI/UX 与术语整改）：GAP_KIND_LABEL ↔ 后端
 * DataGap.Kind 单一来源。CONTRACT-001 惯例（仿 hints.test.ts）：后端 Kind
 * 是开放字符串，没有封闭联合可锁，但本表登记的每个「已知键」都必须真的
 * 在后端源码里出现——前端不得给一个后端不会产出的缺口类型起中文名，
 * 那是假翻译。从后端删掉一个 Kind 而不同步本表即红。
 */
import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import path from "node:path";
import {
  FIN_MODEL_PERIOD_GRAINS,
  GAP_KIND_LABEL,
  PERIOD_GRAIN_LABEL,
  RUN_STATUS_LABEL,
  TIE_OUT_LABEL,
} from "./enums";
import { dict } from "../lib/i18n";

const repoRoot = path.join(import.meta.dirname, "../../../");
const engineGo = readFileSync(path.join(repoRoot, "core-service/internal/finmodel/engine.go"), "utf8");

describe("F0-2 枚举映射表契约", () => {
  it("GAP_KIND_LABEL 的每个已知键都在 finmodel/engine.go 里有字面量", () => {
    for (const key of Object.keys(GAP_KIND_LABEL)) {
      expect(engineGo.includes(`Kind: "${key}"`), `backend emits gap kind "${key}"`).toBe(true);
    }
  });

  it("每张映射表指向的 i18n 键都存在于字典（三语完整性由 i18n-keys 测试另锁）", () => {
    for (const value of Object.values(RUN_STATUS_LABEL)) expect(dict[value], `dict has ${value}`).toBeTruthy();
    for (const value of Object.values(TIE_OUT_LABEL)) expect(dict[value], `dict has ${value}`).toBeTruthy();
    for (const grain of FIN_MODEL_PERIOD_GRAINS) expect(dict[PERIOD_GRAIN_LABEL[grain]], `dict has ${PERIOD_GRAIN_LABEL[grain]}`).toBeTruthy();
    for (const value of Object.values(GAP_KIND_LABEL)) expect(dict[value], `dict has ${value}`).toBeTruthy();
  });

  it("导出粒度选项 = finmodel/fold.go 的 FoldKind 封闭集合", () => {
    const foldGo = readFileSync(path.join(repoRoot, "core-service/internal/finmodel/fold.go"), "utf8");
    const folds = Array.from(foldGo.matchAll(/Fold\w+\s+FoldKind\s+=\s+"([^"]+)"/g), (m) => m[1]);
    expect(folds.length).toBeGreaterThanOrEqual(3);
    for (const grain of FIN_MODEL_PERIOD_GRAINS) {
      expect(folds, `backend FoldKind covers ${grain}`).toContain(grain);
    }
    // 前端不引入第四档（如 week）——那会被 ValidFoldKind 拒绝
    for (const grain of Object.keys(PERIOD_GRAIN_LABEL)) {
      expect(FIN_MODEL_PERIOD_GRAINS as readonly string[], `${grain} is a registered grain`).toContain(grain);
    }
  });
});
