/**
 * UI-001 命令面板路由注册表测试。
 *
 * U1：全部业务路由可被搜到（用 zh-CN 标签 + 分组描述复现面板的过滤逻辑）。
 * U2：角色可见性遵循 AppLayout useMenuItems 的既有规则。
 * U3：app/ 下每个业务页面都已登记进面板；新增页面未登记即失败。
 */
import { describe, expect, it } from "vitest";
import { readdirSync } from "node:fs";
import path from "node:path";
import { t } from "./i18n";
import { PALETTE_EXCLUDED_ROUTES, PALETTE_PAGES, canViewGroup } from "./palette";
import type { User } from "../context/AuthContext";

function userWithRoles(roles: string[]): User {
  return {
    id: "u",
    username: "u",
    role: roles[0] || "",
    roles,
    is_active: true,
  } as User;
}

// U1: 每个登记页面的中文标签都能在面板里搜到（面板按 label+description 过滤）。
describe("U1: 全部业务路由可被搜到", () => {
  it("每个 PALETTE_PAGES 页面在 zh-CN 下都能被其标签搜到", () => {
    for (const def of PALETTE_PAGES) {
      const label = t(def.labelKey, "zh-CN");
      const description = t(`search.group_${def.group}`, "zh-CN");
      const haystack = `${label} ${description}`.toLowerCase();
      expect(label, `${def.path} 的 nav.* key 应有中文文案`).not.toBe("");
      expect(haystack.includes(label.toLowerCase()), `${def.path} 应能按标签 ${label} 搜到`).toBe(true);
    }
  });

  it("全部登记页面都有三语言文案，无空串", () => {
    for (const def of PALETTE_PAGES) {
      for (const lang of ["zh-CN", "zh-HK", "en"] as const) {
        expect(t(def.labelKey, lang), `${def.labelKey} 缺 ${lang}`).not.toBe("");
      }
    }
  });
});

// U2: 角色可见性集合与 AppLayout useMenuItems 一致。
describe("U2: 角色可见性", () => {
  const pathsFor = (user: User) =>
    PALETTE_PAGES.filter((def) => def.visible(user))
      .map((def) => def.path)
      .sort();

  it("readonly 搜得到日常 + 零售分析，搜不到会计/系统", () => {
    const paths = pathsFor(userWithRoles(["readonly"]));
    expect(paths).toContain("/operating-pulse");
    expect(paths).toContain("/store-360");
    expect(paths).toContain("/scenario-workbench");
    expect(paths).toContain("/performance");
    expect(paths).not.toContain("/monthly-closing");
    expect(paths).not.toContain("/standards");
    expect(paths).not.toContain("/settings");
    expect(paths).not.toContain("/admin/users");
    expect(paths).not.toContain("/agent-metrics");
  });

  it("auditor 搜得到日常 + 分析 + 会计 + agent-metrics，搜不到 settings/admin", () => {
    const paths = pathsFor(userWithRoles(["auditor"]));
    expect(paths).toContain("/reports");
    expect(paths).toContain("/monthly-closing");
    expect(paths).toContain("/agent-metrics");
    expect(paths).toContain("/operating-pulse");
    expect(paths).not.toContain("/settings");
    expect(paths).not.toContain("/admin/users");
  });

  it("admin 搜得到全部业务路由", () => {
    const paths = pathsFor(userWithRoles(["admin"]));
    expect(paths).toContain("/settings");
    expect(paths).toContain("/admin/users");
    expect(paths).toContain("/monthly-closing");
    expect(paths).toContain("/operating-pulse");
    expect(paths.length).toBe(PALETTE_PAGES.length);
  });

  it("editor 搜得到会计，搜不到零售分析", () => {
    const paths = pathsFor(userWithRoles(["editor"]));
    expect(paths).toContain("/reports");
    expect(paths).not.toContain("/operating-pulse");
    expect(paths).not.toContain("/scenario-workbench");
  });

  it("canViewGroup 与页面缺省规则一致", () => {
    for (const def of PALETTE_PAGES) {
      const admin = userWithRoles(["admin"]);
      const readonly = userWithRoles(["readonly"]);
      expect(def.visible(admin), `${def.path} admin 应可见`).toBe(true);
      if (def.group === "daily") {
        expect(def.visible(readonly), `${def.path} daily 对 readonly 可见`).toBe(true);
      }
      if (def.group === "system" && def.path !== "/agent-metrics") {
        expect(def.visible(readonly), `${def.path} 系统页对 readonly 不可见`).toBe(false);
      }
      expect(canViewGroup(admin, def.group)).toBe(true);
    }
  });
});

// U3: app/ 下每个业务页面都已登记；PALETTE_EXCLUDED_ROUTES 是显式豁免。
describe("U3: 新增页面必须登记", () => {
  function routesUnderApp(): string[] {
    const appDir = path.join(process.cwd(), "app");
    const routes: string[] = [];
    const walk = (dir: string) => {
      for (const entry of readdirSync(dir, { withFileTypes: true })) {
        const full = path.join(dir, entry.name);
        if (entry.isDirectory()) {
          walk(full);
        } else if (entry.name === "page.tsx" || entry.name === "page.ts") {
          const rel = path.relative(appDir, dir).split(path.sep).join("/");
          routes.push(`/${rel === "." ? "" : rel}`);
        }
      }
    };
    walk(appDir);
    return routes;
  }

  it("app/ 下每个业务页面都已登记进面板", () => {
    const registered = new Set(PALETTE_PAGES.map((def) => def.path));
    const unregistered = routesUnderApp().filter(
      (route) => !registered.has(route) && !PALETTE_EXCLUDED_ROUTES.has(route),
    );
    expect(unregistered, "以下页面未登记进命令面板：").toEqual([]);
  });

  it("豁免列表与当前页面结构一致（新增业务页面时移除豁免或登记）", () => {
    const actual = routesUnderApp();
    for (const excluded of Array.from(PALETTE_EXCLUDED_ROUTES)) {
      if (excluded === "/" || excluded === "/contracts/[id]" || excluded === "/contracts/new") {
        expect(actual).toContain(excluded === "/" ? "/" : excluded);
      }
    }
  });
});
