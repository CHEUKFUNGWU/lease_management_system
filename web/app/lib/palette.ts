/**
 * 命令面板的路由注册表（UI-001）。
 *
 * 单一真相源：GlobalSearch 从这里渲染可搜页面，U3 测试从这里核对
 * 「app/ 下每个业务页面都已登记」。新增业务页面 = 在这里登记一行，
 * 否则测试失败。
 *
 * 角色可见性规则与 AppLayout.tsx 的 useMenuItems（L71-79）保持同一套；
 * 那里是导航的权威实现，这里是面板的镜像。改导航可见性时必须同步
 * 改 canViewGroup，U2 测试会把两组的语义都锁住。
 */
import { hasRole, type User } from "../context/AuthContext";

export type PaletteGroup = "daily" | "analysis" | "accounting" | "system";

export interface PalettePageDef {
  path: string;
  /** i18n nav.* key，三语言齐全 */
  labelKey: string;
  group: PaletteGroup;
  /** 缺省按 group 规则；settings / admin/users 用 admin 专属覆盖 */
  visible: (user: User | null | undefined) => boolean;
}

export function canViewGroup(
  user: User | null | undefined,
  group: PaletteGroup,
): boolean {
  const isAdmin = hasRole(user, "admin");
  const isAuditor = hasRole(user, "auditor");
  const isReadonly = hasRole(user, "readonly");
  switch (group) {
    case "daily":
      return true;
    case "analysis":
      return isAdmin || isReadonly || isAuditor;
    case "accounting":
      return (
        isAdmin ||
        isAuditor ||
        hasRole(user, "editor") ||
        hasRole(user, "reviewer") ||
        hasRole(user, "approver")
      );
    case "system":
      return isAdmin || isAuditor;
  }
}

const byGroup =
  (group: PaletteGroup) => (user: User | null | undefined) =>
    canViewGroup(user, group);

export const PALETTE_PAGES: PalettePageDef[] = [
  // 日常作业（所有登录用户）
  { path: "/todo", labelKey: "nav.todo", group: "daily", visible: byGroup("daily") },
  { path: "/contracts", labelKey: "nav.contracts", group: "daily", visible: byGroup("daily") },
  { path: "/ai-chat", labelKey: "nav.ai_chat", group: "daily", visible: byGroup("daily") },
  { path: "/upload", labelKey: "nav.upload", group: "daily", visible: byGroup("daily") },

  // 零售经营主线（admin / readonly / auditor）
  { path: "/operating-pulse", labelKey: "nav.operating_pulse", group: "analysis", visible: byGroup("analysis") },
  { path: "/store-360", labelKey: "nav.store_360", group: "analysis", visible: byGroup("analysis") },
  { path: "/scenario-workbench", labelKey: "nav.scenario_workbench", group: "analysis", visible: byGroup("analysis") },
  { path: "/performance", labelKey: "nav.performance", group: "analysis", visible: byGroup("analysis") },
  { path: "/portfolio", labelKey: "nav.portfolio", group: "analysis", visible: byGroup("analysis") },
  { path: "/pre-deal", labelKey: "nav.pre_deal", group: "analysis", visible: byGroup("analysis") },
  { path: "/deal-compare", labelKey: "nav.deal_compare", group: "analysis", visible: byGroup("analysis") },
  { path: "/sensitivity", labelKey: "nav.sensitivity", group: "analysis", visible: byGroup("analysis") },
  { path: "/cashflow-forecast", labelKey: "nav.cashflow", group: "analysis", visible: byGroup("analysis") },
  { path: "/roi", labelKey: "nav.roi", group: "analysis", visible: byGroup("analysis") },

  // 会计与合规（admin / auditor / editor / reviewer / approver）
  { path: "/reports", labelKey: "nav.reports", group: "accounting", visible: byGroup("accounting") },
  { path: "/monthly-closing", labelKey: "nav.monthly_closing", group: "accounting", visible: byGroup("accounting") },
  { path: "/standards", labelKey: "nav.standards", group: "accounting", visible: byGroup("accounting") },
  { path: "/audit-logs", labelKey: "nav.audit_logs", group: "accounting", visible: byGroup("accounting") },

  // 系统（agent-metrics: admin/auditor；settings 与 admin/users: admin）
  { path: "/agent-metrics", labelKey: "nav.agent_metrics", group: "system", visible: byGroup("system") },
  { path: "/settings", labelKey: "nav.settings", group: "system", visible: (user) => hasRole(user, "admin") },
  { path: "/admin/users", labelKey: "nav.users", group: "system", visible: (user) => hasRole(user, "admin") },
];

/** U3：app/ 下被排除在面板之外的页面（auth / 首页 / 详情 / 新建动作页）。 */
export const PALETTE_EXCLUDED_ROUTES = new Set([
  "/",
  "/landing",
  "/login",
  "/admin/login",
  "/contracts/[id]",
  "/contracts/new",
]);
