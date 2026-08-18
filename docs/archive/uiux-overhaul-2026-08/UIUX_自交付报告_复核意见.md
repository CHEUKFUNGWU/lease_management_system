# UIUX 自交付报告 — 独立复核意见

> ⚠️ **ARCHIVED 2026-08-18 — 不是现行依据**
> 归档理由：同上，复核意见
> 现行入口：`docs/AI_文档索引与现行决策.md`

> ## ✅ 复核结论：全部关闭（2026-08-11 二次复验）
>
> 本文 §2–§6 提出的 6 项问题（P0×2 / P1×2 / P2×3，含文档订正）已全部修复并独立复验通过，详见文末「二次复验记录」。本文自此转为归档文档，无未决项。

> 复核日期：2026-08-11
> 复核对象：[UIUX 自交付报告](./UIUX_自交付报告.md)
> 复核基线：[UIUX 设计与交互提升评估报告 v1.5](./UIUX_设计与交互提升评估报告.md) §7 验收清单
> 复核方法：对工作区代码独立执行 §7.1 静态检查全套脚本 + 阅读新增/变更源码 + `npm run type-check` / `npm test`

---

## 0. 总体结论

**这轮交付质量高，主体工作扎实。** §7.1 的八项静态检查全部独立复现通过，评估报告中最难的三项 P0（§2.6 状态标签配色、§2.7 i18n 键名裸奔、§2.8 多币种聚合）都修得正确——尤其 §2.8 的 `latestPerCurrency` 实现是对的，没有走"用假汇率合并成一个数"的捷径。

**但有 2 个必须修的缺陷，且它们没被发现的原因是同一个：验收数据集过于简化，恰好绕开了这两条路径。** 其中一条缺陷的"验收证据"实际上正是缺陷本身的表现。

| 级别 | 数量 | 摘要 |
|---|---|---|
| P0 | 2 | 合同台账「月租金」列取错行且口径可能错 3 倍；`useUrlState` 多 setter 互相覆盖导致「清除筛选」失效 |
| P1 | 2 | 验收数据集有两处缺口，掩盖了上面的 P0-1；**页面标题四种尺寸并存，16 个页面页头未统一** |
| P2 | 3 | `api.ts` 硬编码中文；`StatusTag` 配色可被 style 覆盖；`#434343` 仍是死令牌 |
| 文档 | 1 | i18n 条目数笔误 |

**关于"有没有改到变丑"**：视觉体系本身（配色、状态标签、金额排版、图表）改得干净；但**页头统一只做了首页和结账中心两页**，其余 16 页各写各的，标题在 22/24/28/30px 四档之间跳、字重在 600/700 之间跳。逐页切换时最容易被感知为"不整齐"。详见 §6。

---

## 1. 独立复现通过的部分（无需再动）

以下均在本机重新执行，结果与自交付报告一致：

| 检查项 | 声称 | 复核实测 | |
|---|---|---|---|
| `.tsx` 字面量 hex | 0 | **0**（剩余 33 处仅在 `tokens.ts` / `theme.ts`，即令牌源，合理） | ✅ |
| `Tag color=` | 0 | **0**（基线 85） | ✅ |
| `bodyStyle` | 0 | **0** | ✅ |
| 说教型文案 | 0 | **0** | ✅ |
| `QuickActionsCard` / `quickActions` | 0 | **0** | ✅ |
| i18n 缺键 | 0 | **0**（基线 20） | ✅ |
| i18n 三语完整性 | 0 缺失 | **1431 条 / 0 缺失** | ✅ |
| 动画导出全部被引用 | 3 个 | **3 个，引用数 2/3/5** | ✅ |
| `tokens.ts` 外部消费者 | StatusTag | **`components/StatusTag.tsx`** | ✅ |
| `npm run type-check` | 通过 | **零错误** | ✅ |
| `npm test` | 27 passed | **4 files / 27 tests passed** | ✅ |

源码层面另行确认：

- **§2.6 状态标签**：`tokens.ts` 的 `colors.status` 五组值与批准的方案 B **完全一致**；`globals.css` 的 `color:#fff !important` 已删除，`.ant-tag` 的 `border:none` 已按建议改为 `border-width:1px; border-style:solid`；`StatusTag` 强制渲染图标且**没有** `showIcon` 开关。补丁 1/2/3 同批落地，未触发"单独删 `!important` 导致 success/error 退回 2.03:1"的风险。
- **§2.2 动画冻结**：`ThemeProvider` 已包 `<MotionConfig reducedMotion="user">`，`globals.css:815` 有 `prefers-reduced-motion` 块；**全部 6 个 recharts 消费文件、15/15 个图表元素**均设 `isAnimationActive={false}`，无遗漏。
- **§2.8 多币种**：`page.tsx` 的 `latestPerCurrency()` 按 `currency|period` 聚合后各币种取最新期，`summedMoney()` 按币种分别求和，**全程无跨币种相加**；首页 KPI 从 4 个减为 2 个并带多币种提示。
- **§2.4 键盘可达**：`AppLayout.tsx` 中 `<div onClick>` **归零**，改为 3 个 `<button>` + 4 处 `aria-label`。
- **§3.1 导航**：侧栏 4 个 `type:"group"` 分组；窄屏走 `Drawer`。
- **§3.7 全局搜索**：`GlobalSearch` 已改为服务端搜索（`page_size: 8`），不再全量拉取合同表。
- **§5.3 金额**：`fmtMoney` 币种为必填参数、空值 `—`、负数会计括号、无法识别的币种码回退为 `XYZ 1,000.00`；`.money-cell` 已启用 `font-variant-numeric: tabular-nums`。
- **§2.5 错误文案**：`ApiError` 已建立，401 统一处理。

---

## 2. P0-1：合同台账「月租金」列取错行，且口径可能高估 3 倍

**位置**：`core-service/internal/repository/contract.go:443–449`

```sql
(SELECT ps.amount
 FROM lease_payment_schedules ps
 WHERE ps.contract_id = lease_contracts.id
   AND ps.is_fixed = true AND ps.is_variable = false
   AND ps.is_lease_component = true AND ps.is_non_lease_component = false
 ORDER BY ps.effective_start_date DESC, ps.due_date DESC
 LIMIT 1) AS monthly_rent
```

前端以「月租金」渲染：`web/app/contracts/page.tsx:278–282`，i18n `contracts.col_monthly_rent` = 「月租金」。

### 缺陷 a — 取的是"最新一条"，不是"当前生效的一条"

子查询**没有任何当前日期约束**。`lease_payment_schedules` 用 `effective_start_date` / `effective_end_date` 对付款计划做版本化，一份含租金递增条款的合同会有多条记录。`ORDER BY effective_start_date DESC LIMIT 1` 取到的是**生效日期最晚的一条，即未来的租金**。

零售租约最常见的变更就是按 CPI 或固定比例逐年调租，因此这条路径在真实数据上几乎必然触发：台账会显示合同**最后一年**的租金。

### 缺陷 b — `amount` 不一定是"月"金额

`lease_payment_schedules` **没有频率字段**（见 `db/init/01_init.sql:190–212`）。一行记录的 `amount` 对应的是 `coverage_start_date` → `coverage_end_date` 这段区间。季付合同的 `amount` 是季度金额，标成「月租金」会**高估 3 倍**；年付则高估 12 倍。

### 缺陷 c — 币种可能不匹配

子查询取 `ps.amount`，但 `lease_payment_schedules` 有自己的 `currency` 列，可能与合同主币种不同。前端却用 `record.currency` 渲染（`contracts/page.tsx:282`），会给出一个错误的币种符号。

### 修复指令

**① 加当前日期约束（必改，无歧义）**

```sql
   AND ps.effective_start_date <= CURRENT_DATE
   AND ps.effective_end_date   >= CURRENT_DATE
```

若当前日期无生效行（合同未起租或已到期），应回退到"离今天最近的一条"而非最后一条，建议改为按 `ABS(ps.effective_start_date - CURRENT_DATE)` 排序，或直接返回 `NULL` 让前端显示 `—`。

**② 解决频率歧义（二选一，属口径决策，请与财务确认后再动手）**

- **方案 A（推荐，语义正确）**：不再假造"月"这个口径。列改名为「当期租金」，同时返回 `coverage_start_date` / `coverage_end_date`，在单元格下方以小字标注覆盖区间（如「2026-01-01 ~ 2026-03-31」）。i18n 键 `contracts.col_monthly_rent` 一并更名。
- **方案 B（快速，近似值）**：保留「月租金」标签，但按覆盖天数折算，并在列头 tooltip 注明"按付款覆盖期折算的月均值"：

  ```sql
  (SELECT ps.amount * 30.44 / GREATEST(ps.coverage_end_date - ps.coverage_start_date + 1, 1) ...)
  ```

  ⚠️ 折算值不可用于任何入账或对外披露用途，只作台账浏览参考。

**③ 币种**：子查询一并返回 `ps.currency AS monthly_rent_currency`，前端改用该字段渲染；若与合同币种不一致，额外加一个提示标记。

---

## 3. P0-2：`useUrlState` 多个 setter 互相覆盖，「清除筛选」实际失效

**位置**：`web/app/hooks/useUrlState.ts`

```ts
const paramsRef = useRef<URLSearchParams | null>(null);   // ← 每个 hook 实例各一份

useEffect(() => {
  paramsRef.current = new URLSearchParams(searchParams.toString());
}, [searchParams]);
```

注释写的意图是「让同时更新的多个控件能够组合，而不是各自从同一份过期查询串出发」——**但 `useRef` 是每个 hook 实例私有的，并不跨实例共享**，所以这个意图没有实现。

### 触发路径

`web/app/contracts/page.tsx:141` `clearFilters()` 在同一 tick 内连续调用 **7 个** setter：

```ts
setSearch(""); setStatusFilter(""); setRiskFilter("");
setScopeFilter(""); setAssetFilter(""); setExpiryFilter(""); setPageParam("1");
```

7 个 hook 实例各持有一份**独立的** `URLSearchParams` 副本，都从同一份尚未更新的 `searchParams` 快照出发，各自 `router.replace()` 一次。`router.replace` 是异步的，快照在这一 tick 内不会刷新，**最后一个调用覆盖前面全部**。

最终 URL = 原查询串仅删掉 `page`。而 `useUrlState` 的取值是 `searchParams.get(key) ?? defaultValue`，UI 直接读 URL —— 所以 **用户点「清除筛选」，六个筛选条件纹丝不动，按钮看起来没反应**。

`web/app/cashflow-forecast/page.tsx:171` `handleReset()` 同样连调 4 个 URL setter，同一缺陷。

（`handleSearchChange` 因为两个 setter 之间隔了 300ms 防抖，快照已刷新，不受影响。）

### 修复指令

把缓冲区提到模块作用域，让所有 hook 实例共享同一个对象：

```ts
"use client";

import { useCallback, useEffect } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";

// Shared across every hook instance on the page. Several setters commonly fire
// in the same tick (clearFilters resets seven at once); a per-instance snapshot
// would leave each one starting from the same stale query string, so only the
// last router.replace() survived and the rest were silently dropped.
let buffer: { path: string; params: URLSearchParams } | null = null;

export function useUrlState(key: string, defaultValue: string) {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const value = searchParams.get(key) ?? defaultValue;

  useEffect(() => {
    buffer = { path: pathname, params: new URLSearchParams(searchParams.toString()) };
  }, [pathname, searchParams]);

  const setValue = useCallback(
    (next: string) => {
      // A route change invalidates the buffer; rebuild rather than carry another
      // page's query string across.
      if (!buffer || buffer.path !== pathname) {
        buffer = { path: pathname, params: new URLSearchParams(searchParams.toString()) };
      }
      const params = buffer.params;
      if (!next || next === defaultValue) params.delete(key);
      else params.set(key, next);
      const query = params.toString();
      router.replace(query ? `${pathname}?${query}` : pathname, { scroll: false });
    },
    [defaultValue, key, pathname, router, searchParams]
  );

  return [value, setValue] as const;
}
```

**必须补一条回归测试**（这类缺陷靠人工点击很容易漏）：模拟同一 tick 内连续调用多个 setter，断言最终 URL 同时反映全部变更。

**验收**：合同台账施加 status + risk + lease_scope 三个筛选 → 点「清除筛选」→ 地址栏应只剩路径，无残留查询参数；`/cashflow-forecast` 点「重置」同样核验。

---

## 4. P1-3：验收数据集两处缺口，且正好掩盖了 P0-1

**位置**：`scripts/uiux_acceptance_dataset.sql:44–56`

```sql
SELECT c.id, c.lease_start_date, c.lease_end_date, c.lease_start_date,
       c.lease_end_date, c.lease_start_date, 'postpaid',
       25000 + (...)::integer * 1250, c.currency, 'fixed_rent', ...
```

**每份合同只插入 1 条付款计划行，且 `coverage_start_date` = 租期起、`coverage_end_date` = 租期止。**

后果：

1. **P0-1 缺陷 a 测不出来** —— 只有一行，`ORDER BY ... DESC LIMIT 1` 永远取对。
2. **P0-1 缺陷 b 测不出来，而且被固化了** —— 这一行的覆盖区间是**整个租期**（数年），其 `amount` 却被当作「月租金」展示。自交付报告 §3.4 那句「金额显示为带币种的 `¥31,250.00`」**正是缺陷的表现，而非缺陷不存在的证据**。

另外对照评估报告 §7.0 的数据集要求，还缺两项：

| §7.0 要求 | 数据集现状 |
|---|---|
| 含 1 个长名称合同（> 20 字符），用于验证窄屏列坍缩 | ❌ 未覆盖 |
| 至少 1 个已生成分录并锁定的会计期间，用于验证 §3.2 结账流程轨道 | ❌ 未覆盖（无 `monthly_closing` / 期间锁种子数据） |

### 修复指令

在 `uiux_acceptance_dataset.sql` 中补充：

1. **多段付款计划**：至少 1 份合同拆成 3 段（如逐年 +5% 递增），`effective_start_date` / `effective_end_date` 逐段衔接，其中一段覆盖当前日期。
2. **非月度频率**：至少 1 份合同的付款行按**季度**切分（`coverage` 跨 3 个月），用于暴露 P0-1 缺陷 b。
3. **长名称合同**：1 份 `contract_name` 长度 > 20 字符。
4. **已锁定期间**：为至少 1 个会计期间生成分录并置为锁定状态。
5. **付款计划币种**：至少 1 份合同的付款行 `currency` 与合同主币种不同，用于暴露 P0-1 缺陷 c。

补完后需**重跑** P0-1 相关验收——当前的"通过"不作数。

---

## 5. P2 级问题

### P2-4：`lib/api.ts` 硬编码中文，绕过 i18n

`web/app/lib/api.ts:24`：

```ts
if (status === 401 || code === "invalid_token" || code === "unauthorized")
  return "登录已过期，请重新登录。";
```

错误文案层做得对（不再直出 `invalid token`），但文案本身没走 `t()`。与本轮"i18n 缺键归零"的成果不一致，且 zh-HK / en 用户会看到简体中文。

**修复**：错误映射表改为返回 i18n key，由调用方 `t()` 渲染；或给 `api.ts` 注入当前语言。同时建议把评估报告 §7.1 ③ 的裸中文检查纳入 CI，防止再次回流。

### P2-5：`StatusTag` 的 `style` 可覆盖配色，绕过对比度保证

`web/app/components/StatusTag.tsx`：`...style` 展开在 `background` / `color` / `border` **之后**，调用方一个 `style={{ color: "#999" }}` 就能破坏已验证的对比度。

**修复**：把 `style` 限制为不含颜色的白名单属性（如仅允许 `margin` / `maxWidth`），或将 `...style` 移到颜色三项**之前**，使配色不可被覆盖。

### P2-6：`#434343` 仍是死令牌

`web/app/globals.css:13` 的 `--mono-30: #434343` 全库消费数为 **1**（即它自己的定义行）。评估报告 §9.5 第 2 项要求每级 ≥ 2。

同批的 `#0A0A0A` / `#141414` 已正确删除，`#737373` 已按建议提拔使用，`#8C8C8C` / `#BFBFBF` 已归零——仅此一个遗漏。

**修复**：删除该行；若打算保留作图标默认色，则找一处真实使用。

### 文档 nit

自交付报告 §3.1 写「i18n 条目三语完整性 1,321 条」，实测为 **1,431 条**（0 缺失）。结论无误，数字笔误。

---

## 6. P1-7：页面标题三种尺寸并存，六个页面比其他页面明显"大一号"

本轮改造集中在首页、合同台账、结账中心和公共组件，**其余页面的页头没有跟着统一**。浏览器实测（注入真实 antd 哈希类后测量计算样式）：

| 写法 | 实测渲染 | 页面数 | 页面 |
|---|---|---|---|
| `<h1>`（走 `globals.css`） | **24px / 700** | 5 | `todo` `contracts` `portfolio` `standards` `sensitivity` |
| `DashboardHeader` 内联 | **28px / 700** | 2 | 首页 `monthly-closing` |
| 内联 `fontSize: 28` | **28px / 700** | 1 | `reports` |
| `<Title level={2}>` | **30px / 600** | **7** | `pre-deal` `roi` `deal-compare` `cashflow-forecast` `settings` `agent-metrics` `performance` |
| `<Title level={3}>` | **22px / 600** | 1 | `admin/users` |
| 无页面标题 | — | 2 | `audit-logs` `contracts/new` |

也就是说页面标题在 **22 / 24 / 28 / 30px 四档之间跳**，字重在 **600 / 700** 之间跳。在同一个产品里逐页切换时，30px/600 的那七个页面会明显"大一号且更细"，24px/700 的五个页面则偏小偏重——这是最容易被感知为"做得不整齐"的一类问题。

评估报告 §9.1 曾指出页头是"复制粘贴的一致性而非组件化的一致性"，建议抽 `<PageHeader>`。本轮只在首页和结账中心引入了共享组件（`DashboardHeader`），**其余 16 个页面仍各写各的**，差异反而比改造前更大了（改造前至少 `<h1>` 一派占多数）。

另有两个附带问题：

- **`audit-logs` 与 `contracts/new` 完全没有页面标题**，用户进入后只能靠面包屑判断位置。
- **`performance/page.tsx:122` 自带 `padding: "24px 28px"`**，而 `AppLayout` 的 Content 已有 `padding: "32px 48px"`（`AppLayout.tsx:477`）。两层叠加后经营驾驶舱的内容比其他页面多缩进一圈（上 56px / 左 76px），视觉上明显不对齐。

### 修复指令

1. **抽取 `web/app/components/PageHeader.tsx`**，签名与现有 `DashboardHeader` 对齐（`title` / `subtitle` / `primaryAction` / `secondaryAction`），统一为 **28px / 700 / `letter-spacing: -0.04em` / `margin-bottom: 4px`**，副标题 14px `var(--fg-muted)`。
2. **全部 18 个业务页面改用它**，删除各自的 `<Title>` / `<h1>` / 内联 `fontSize`。
3. **给 `audit-logs` 和 `contracts/new` 补上标题与副标题**（副标题按评估报告 §9.3 的判据，须含动态值，如"共 N 条记录 · 最近 30 天"）。
4. **删除 `performance/page.tsx:122` 的内层 `padding`**，由 `AppLayout` 统一负责。
5. 加一条 lint 或 review 检查：业务页面内不允许再出现 `<Title level=` 与 `fontSize: 28`。

**验收**：逐页截图，页面标题的 `fontSize` / `fontWeight` / `marginBottom` 三项完全一致；`.ant-layout-content` 的直接子元素无额外 padding。

---

## 7. 建议的修复顺序

1. **P0-2 `useUrlState`** —— 改动最小、影响最直接（用户点按钮没反应），且不依赖任何数据。先修。
2. **P1-3 补验收数据集** —— 必须在 P0-1 之前完成，否则修完仍然验证不了。
3. **P0-1 月租金列** —— 缺陷 a、c 直接改；缺陷 b 的口径方案需先与财务确认「当期租金 + 覆盖区间」还是「折算月均值」。
4. **P2-4 / P2-5 / P2-6** —— 可并入下一批。

修完后请重跑评估报告 §7.1 全套脚本 + §7.2 ⑦（多币种运行时校验），并针对 P0-1 在**含多段付款计划、季付、异币种**的新数据集上重新出具证据。

---

## 附：二次复验记录（2026-08-11）

对修复后的工作区独立执行全套检查，**不采信自交付结论**。

### 逐项关闭情况

| 工单 | 问题 | 复验方法 | 结论 |
|---|---|---|---|
| T1 | `useUrlState` 多 setter 覆盖 | 读源码 + 读测试 + `npm test` | ✅ 缓冲区已提至模块作用域；**额外抽出纯函数 `updateUrlStateBuffer` 便于测试**，2 条用例正好覆盖原来的两种失败模式（跨 key 组合、clearFilters 全清） |
| T2 | 验收数据集缺口 | 逐场景 grep 脚本 | ✅ 年度递增、季度付款、异币种（`:136–152`，注释说明为刻意构造）、长名称、锁账期间五项齐备；含 9 处幂等构造 |
| T3 | 当期租金列 | 读 SQL + 结构体 + 前端 + 反向确认未误伤 | ✅ `LEFT JOIN LATERAL` + 当前生效日期约束；4 字段贯通 struct/Scan/排序白名单；**移动端卡片视图（`:709–715`）未漏改**；`services/{predeal,renewaldecision,dealcompare,operating}` 的 `MonthlyRent` 计数保持 6/14/27/14 未变，未发生误重命名 |
| T4 | 18 页页头统一 | grep 残留 + 读 CSS + 读 DashboardHeader | ✅ 业务页无任何裸 `<h1>` / `<Title level=` / `fontSize: 28`；`.page-header-title` 单一实现 28px/700；`DashboardHeader` 已委托给 `PageHeader`，无第二套；`performance` 内层 padding 已删 |
| T5-1 | `api.ts` 硬编码中文 | grep 中文字面量 | ✅ 0 处 |
| T5-2 | `StatusTag` 配色可被覆盖 | 读源码 | ✅ `...style` 已移至调色板**之前**，颜色不可覆盖 |
| T5-3 | `#434343` 死令牌 | 灰阶消费统计 | ✅ 已删除（当前 0 命中） |
| 文档 | i18n 条目数 | 括号配对脚本 | ✅ 实测 **1,440 条 / 0 缺译**，与订正后数字一致 |

### 自动化验证

| 项 | 结果 |
|---|---|
| §7.1 全套静态检查 | i18n 缺键 0 · `.tsx` 字面量 hex 0 · `Tag color=` 0 · `globals.css` ant-tag 覆盖 0 · 说教型文案 0 · `bodyStyle` 0 · 动画导出引用均 ≥1 |
| `npm run type-check` | 零错误 |
| `npm test` | **5 files / 29 tests passed** |
| `go test ./...` | 全部 `ok`，无 FAIL |
| `npm run build` | 成功 |
| `git diff --check` | 通过 |

### 一条非阻塞观察（可择期处理）

`current_rent` / `latest_liability` / `latest_rou_asset` 三列在 Postgres 中默认 `NULLS FIRST`（降序时）。按「当期租金」降序排列时，未起租或已到期（值为 `NULL`）的合同会排在最前面。不影响正确性，但按金额排序时首屏会是一片 `—`。

建议在 `ORDER BY` 后统一追加 `NULLS LAST`：

```go
query += fmt.Sprintf(" ORDER BY %s %s NULLS LAST", sortCol, sortOrder)
```

此项为体验优化，非缺陷，不阻塞本轮关闭。
