# UIUX 交付复核 — 修复工单

> ⚠️ **ARCHIVED 2026-08-18 — 不是现行依据**
> 归档理由：同上，修复指令
> 现行入口：`docs/AI_文档索引与现行决策.md`

> 下发日期：2026-08-11
> 依据：[UIUX 自交付报告 — 独立复核意见](./UIUX_自交付报告_复核意见.md)
> 基线：[UIUX 设计与交互提升评估报告 v1.5](./UIUX_设计与交互提升评估报告.md)
>
> **先说明：上一轮交付质量是好的。** §7.1 八项静态检查全部独立复现通过，三项最难的 P0（状态标签配色、i18n 键名裸奔、多币种聚合）都修对了。本工单只针对复核中发现的 6 个遗留问题，不是推翻重做。

---

## 执行顺序

任务之间有依赖，**请严格按序**：

```
T1 (useUrlState)  ──► 独立，最先做
T2 (验收数据集)   ──► 必须早于 T3，否则 T3 改完无法验证
T3 (月租金列)     ──► 依赖 T2
T4 (页头统一)     ──► 独立，可与 T1 并行
T5 (三个 P2)      ──► 最后
```

每个任务完成后单独提交，commit message 注明工单号（如 `fix(T1): ...`）。

---

## T1 — 修复 `useUrlState` 多 setter 互相覆盖【P0】

### 现象

合同台账施加多个筛选后点「清除筛选」，**按钮看起来没反应**，筛选条件不变。`/cashflow-forecast` 点「重置」同样。

### 原因

`web/app/hooks/useUrlState.ts` 中 `paramsRef` 用的是 `useRef`，**每个 hook 实例私有，不跨实例共享**。而代码注释写的意图恰恰是"让同时更新的多个控件组合起来"——这个意图没有实现。

`web/app/contracts/page.tsx:141` 的 `clearFilters()` 在同一 tick 内连调 7 个 setter，7 个实例各持一份从**同一份未刷新快照**复制出来的 `URLSearchParams`，各自 `router.replace()` 一次。`router.replace` 是异步的，这一 tick 内快照不会更新，**最后一个调用覆盖前面全部**，最终只有 `page` 被删掉。而 `useUrlState` 的取值是 `searchParams.get(key) ?? defaultValue`，UI 直接读 URL，所以界面纹丝不动。

`web/app/cashflow-forecast/page.tsx:171` 的 `handleReset()` 连调 4 个，同一缺陷。

（`handleSearchChange` 两个 setter 之间隔了 300ms 防抖，快照已刷新，不受影响。）

### 改法

把缓冲区提到模块作用域，让页面上所有 hook 实例共享同一个对象。**整文件替换** `web/app/hooks/useUrlState.ts`：

```ts
"use client";

import { useCallback, useEffect } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";

// Shared across every hook instance on the page. Several setters commonly fire
// in the same tick — clearFilters() resets seven at once — and a per-instance
// snapshot left each of them starting from the same stale query string, so only
// the last router.replace() survived and the rest were silently dropped.
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
      // A route change invalidates the buffer; rebuild rather than carry
      // another page's query string across.
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

### 必须补的回归测试

新建 `web/app/hooks/useUrlState.test.ts`，覆盖：**同一 tick 内连续调用多个不同 key 的 setter，最终 URL 同时反映全部变更**。这类缺陷靠人工点击极易漏过，没有测试就等于没修。

### 验收

- [ ] 合同台账施加 `status` + `risk` + `lease_scope` 三个筛选 → 点「清除筛选」→ 地址栏只剩 `/contracts`，无任何残留查询参数
- [ ] `/cashflow-forecast` 施加视图 + 粒度 + 日期区间 → 点「重置」→ 四个参数全部清空
- [ ] 新增测试通过，`npm test` 全绿

---

## T2 — 补齐验收数据集【P1，T3 的前置】

### 为什么必须先做

`scripts/uiux_acceptance_dataset.sql:44–56` 给**每份合同只插入 1 条付款计划行**，且 `coverage_start_date` = 租期起、`coverage_end_date` = 租期止（跨数年）。

这直接导致 T3 的两个缺陷**在当前数据集上无法被发现**：只有一行，取哪一行都对；覆盖整个租期的金额被当成"月租金"显示，看起来还挺正常。

⚠️ 自交付报告 §3.4 那句「金额显示为带币种的 `¥31,250.00`」，**是缺陷的表现，不是缺陷不存在的证据**。

### 改法

在 `scripts/uiux_acceptance_dataset.sql` 中补充以下场景（合同编号继续用 `UIUX-ACCEPT-%` 前缀）：

| # | 场景 | 具体要求 | 用于暴露 |
|---|---|---|---|
| 1 | **多段付款计划** | 至少 1 份合同拆成 3 段（逐年 +5% 递增），`effective_start_date` / `effective_end_date` 逐段衔接，**其中恰好一段覆盖当前日期** | T3 缺陷 a |
| 2 | **季付频率** | 至少 1 份合同的付款行按季度切分（`coverage_start` → `coverage_end` 跨 3 个月） | T3 缺陷 b |
| 3 | **异币种付款行** | 至少 1 份合同的付款行 `currency` 与合同主币种不同 | T3 缺陷 c |
| 4 | **长名称合同** | 1 份 `contract_name` 长度 > 20 字符 | 评估报告 §7.0：窄屏列坍缩 |
| 5 | **已锁定会计期间** | 至少 1 个会计期间已生成分录并置为锁定 | 评估报告 §7.0：§3.2 结账流程轨道 |

### 验收

- [ ] 脚本可重复执行（幂等），不污染非 `UIUX-ACCEPT-%` 数据
- [ ] 执行后核对：存在含 3 段付款计划的合同、存在季付合同、存在异币种付款行、存在长名称合同、存在已锁定期间
- [ ] 脚本头部注释写明"仅用于开发/演示环境"

---

## T3 — 修复合同台账「月租金」列【P0，依赖 T2】

### 位置

`core-service/internal/repository/contract.go:443–449`；前端渲染 `web/app/contracts/page.tsx:278–282`；i18n 键 `contracts.col_monthly_rent`。

### 缺陷 a：取的是"最新一条"而非"当前生效的一条"

子查询没有任何当前日期约束：

```sql
ORDER BY ps.effective_start_date DESC, ps.due_date DESC LIMIT 1
```

`lease_payment_schedules` 用 `effective_start_date` / `effective_end_date` 对付款计划做版本化，含递增条款的合同会有多条记录。这样取到的是**生效日期最晚的一条，即未来的租金**。零售租约普遍按 CPI 或固定比例逐年调租，真实数据上几乎必然触发——台账会显示合同**最后一年**的租金。

**改法**：加当前日期约束。

```sql
   AND ps.effective_start_date <= CURRENT_DATE
   AND ps.effective_end_date   >= CURRENT_DATE
```

若当前日期无生效行（合同未起租或已到期），**返回 `NULL`**，前端按 `fmtMoney` 现有逻辑显示 `—`。不要回退到"最后一条"——宁可不显示，也不要显示一个错的。

### 缺陷 b：`amount` 不一定是"月"金额

`lease_payment_schedules` **没有频率字段**（见 `db/init/01_init.sql:190–212`）。一行记录的 `amount` 对应 `coverage_start_date` → `coverage_end_date` 这段区间。季付合同的 `amount` 是季度金额，标成「月租金」**高估 3 倍**；年付高估 12 倍。

**口径已定：采用方案 A —— 不再假造"月"这个口径。** 理由：这是会计产品，不能用按日均摊造出一个账上不存在的"月租金"数字；显示真实付款额 + 覆盖区间既正确又更有信息量。

- 列改名为「**当期租金**」
- 一并返回 `coverage_start_date` / `coverage_end_date`，在金额下方以小字标注覆盖区间

### 缺陷 c：币种可能不匹配

子查询取 `ps.amount`，但付款计划有自己的 `currency` 列，可能与合同主币种不同；前端却用 `record.currency` 渲染。一并返回付款行自己的币种。

### 统一改法（三个缺陷一次改完）

因为现在需要从**同一行**取出 4 个值（金额、币种、覆盖起、覆盖止），继续用 4 个相关子查询既浪费又有风险——排序出现并列时它们可能各自取到**不同的行**。改用 `LATERAL` 一次取一行：

**① `core-service/internal/repository/contract.go` — 删掉原 `monthly_rent` 子查询（第 443–449 行），在 `FROM lease_contracts` 之后加：**

```sql
LEFT JOIN LATERAL (
    SELECT ps.amount, ps.currency, ps.coverage_start_date, ps.coverage_end_date
    FROM lease_payment_schedules ps
    WHERE ps.contract_id = lease_contracts.id
      AND ps.is_fixed = true
      AND ps.is_variable = false
      AND ps.is_lease_component = true
      AND ps.is_non_lease_component = false
      -- Only the instalment in force today. Without this bound the query
      -- returned the latest-dated row instead, so any lease with a scheduled
      -- escalation showed its final year's rent in the ledger list.
      AND ps.effective_start_date <= CURRENT_DATE
      AND ps.effective_end_date   >= CURRENT_DATE
    ORDER BY ps.effective_start_date DESC, ps.due_date DESC
    LIMIT 1
) cur ON true
```

SELECT 列表中相应改为：

```sql
cur.amount               AS current_rent,
cur.currency             AS current_rent_currency,
cur.coverage_start_date  AS current_rent_coverage_start,
cur.coverage_end_date    AS current_rent_coverage_end
```

当前日期无生效行时（未起租 / 已到期），`LEFT JOIN LATERAL` 自然产出 `NULL`，前端按 `fmtMoney` 现有逻辑显示 `—`。**不要**回退到"最后一条"——宁可不显示，也不要显示一个错的。

**② 同文件配套改 3 处：**

| 行 | 原内容 | 改为 |
|---|---|---|
| `:76` | `MonthlyRent *float64 \`json:"monthly_rent,omitempty"\`` | `CurrentRent *float64` + `CurrentRentCurrency *string` + `CurrentRentCoverageStart *time.Time` + `CurrentRentCoverageEnd *time.Time`，json 标签依次 `current_rent` / `current_rent_currency` / `current_rent_coverage_start` / `current_rent_coverage_end`，均 `omitempty` |
| `:397` | `allowedSortColumns` 中的 `"monthly_rent": "monthly_rent"` | `"current_rent": "current_rent"` |
| `:573` | `&c.MonthlyRent` | `&c.CurrentRent, &c.CurrentRentCurrency, &c.CurrentRentCoverageStart, &c.CurrentRentCoverageEnd`（顺序须与 SELECT 列表一致） |

> ⚠️ **不要做全局重命名。** `MonthlyRent` 在 core-service 中有 60+ 处命中，但只有上面这 3 处属于合同台账。其余分布在 `services/predeal`、`services/renewaldecision`、`services/dealcompare`、`services/operating`、`services/reporting/unit_price_projection` —— 那些是**用户手工输入的月租金假设**，与付款计划无关，语义正确，动了会破坏签约前决策、续租卡、条款比价三个模块。

**③ `web/app/contracts/page.tsx` — 3 处：**

- `:60` 接口字段：`monthly_rent?: number` → 换成上面四个字段
- `:278–282` 表格列：

```tsx
{
  title: t("contracts.col_current_rent", language),
  key: "current_rent",
  width: 190,
  align: "right" as const,
  sorter: true,
  render: (_: unknown, record: Contract) => (
    <div>
      <div className="money-cell">
        {fmtMoney(record.current_rent, record.current_rent_currency ?? record.currency)}
      </div>
      {record.current_rent_coverage_start && record.current_rent_coverage_end && (
        <div style={{ fontSize: 12, color: "var(--fg-muted)", whiteSpace: "nowrap" }}>
          {dayjs(record.current_rent_coverage_start).format("YYYY-MM-DD")}
          {" ~ "}
          {dayjs(record.current_rent_coverage_end).format("YYYY-MM-DD")}
        </div>
      )}
    </div>
  ),
}
```

- **`:702` 移动端卡片视图**（最容易漏的一处）：同样改用 `current_rent` + `current_rent_currency`，覆盖区间以小字放在金额下方

**④ `web/app/lib/i18n.ts:6774` — 键改名并新增一条：**

```ts
"contracts.col_current_rent": { "zh-CN": "当期租金", "zh-HK": "當期租金", en: "Current rent" },
```

删除 `contracts.col_monthly_rent`（确认无其他引用后）。若列头需要说明，另加一条 tooltip 文案："按付款计划中当前生效的一期显示，下方为该期覆盖区间"。

### 验收（必须在 T2 完成后的新数据集上执行）

- [ ] 含 3 段递增付款计划的合同，台账显示的是**当前生效段**的金额，不是最后一段
- [ ] 季付合同显示季度实付金额，下方覆盖区间跨 3 个月（而非被折算成月均值）
- [ ] 异币种付款行的合同，金额前的币种符号与**付款行**币种一致
- [ ] 未起租 / 已到期合同显示 `—`，不显示错误金额
- [ ] 移动端卡片视图与桌面表格显示同一口径（`:702` 未漏改）
- [ ] 按「当期租金」列排序可用（`allowedSortColumns` 已同步改名）
- [ ] 签约前决策、续租卡、条款比价三个页面功能不受影响（确认未误改 `services/predeal`、`renewaldecision`、`dealcompare` 中的 `MonthlyRent`）
- [ ] `go test ./...` 与 `npm run type-check` 通过

---

## T4 — 统一 18 个页面的页头【P1】

### 现象

页面标题在 **22 / 24 / 28 / 30px 四档之间跳**，字重在 **600 / 700** 之间跳。逐页切换时，30px/600 的七个页面明显"大一号且更细"，24px/700 的五个页面偏小偏重。这是最容易被感知为"做得不整齐"的一类问题。

浏览器实测（注入真实 antd 哈希类后测量计算样式）：

| 写法 | 实测 | 页面 |
|---|---|---|
| `<h1>`（走 `globals.css`） | **24px / 700** | `todo` `contracts` `portfolio` `standards` `sensitivity` |
| `DashboardHeader` | **28px / 700** | 首页、`monthly-closing` |
| 内联 `fontSize: 28` | **28px / 700** | `reports` |
| `<Title level={2}>` | **30px / 600** | `pre-deal` `roi` `deal-compare` `cashflow-forecast` `settings` `agent-metrics` `performance` |
| `<Title level={3}>` | **22px / 600** | `admin/users` |
| 无标题 | — | `audit-logs` `contracts/new` |

评估报告 §9.1 曾指出页头是"复制粘贴的一致性而非组件化的一致性"，建议抽 `<PageHeader>`。上一轮只在首页和结账中心引入了共享组件，其余 16 页仍各写各的——**差异反而比改造前更大**（改造前至少 `<h1>` 一派占多数）。

### 改法

1. **抽取 `web/app/components/PageHeader.tsx`**，签名与现有 `DashboardHeader` 对齐（`title` / `subtitle` / `primaryAction?` / `secondaryAction?`）。统一规格：

   - 标题：**28px / 700 / `letter-spacing: -0.04em` / `margin-bottom: 4px`**
   - 副标题：**14px / `var(--fg-muted)` / `margin: 0`**
   - 容器：`display:flex; justify-content:space-between; align-items:flex-start; margin-bottom:32px`

   让 `DashboardHeader` 内部改为直接复用它，避免出现第二套。

2. **18 个业务页面全部改用 `<PageHeader>`**，删除各自的 `<Title>` / `<h1>` / 内联 `fontSize: 28`。

3. **给 `audit-logs` 和 `contracts/new` 补上标题与副标题。** 副标题按评估报告 §9.3 判据**必须含动态值**，例如：
   - `audit-logs`：「共 {n} 条记录 · 最近 30 天」
   - `contracts/new`：「手工录入 · 或改用 AI 上传解析」

4. **删除 `web/app/performance/page.tsx:122` 的内层 `padding: "24px 28px"`**。`AppLayout` 的 Content 已有 `padding: "32px 48px"`（`AppLayout.tsx:477`），两层叠加后经营驾驶舱内容比其他页面多缩进一圈（上 56px / 左 76px），肉眼可见地不对齐。

5. **加检查防回流**，纳入 CI：

   ```bash
   # 业务页面内不得再出现自定义页面标题
   grep -rnE '<Title level=|<Typography\.Title level=|fontSize: 28' web/app --include='page.tsx'
   ```

   通过标准：输出为空。

### 验收

- [ ] 18 个页面逐页截图，页面标题的 `fontSize` / `fontWeight` / `marginBottom` 三项完全一致
- [ ] `.ant-layout-content` 的直接子元素计算样式 `padding` 为 `0px`
- [ ] `audit-logs` 与 `contracts/new` 有标题，且副标题含动态值
- [ ] 上述 grep 输出为空

---

## T5 — 三个 P2

### T5-1：`lib/api.ts` 硬编码中文

`web/app/lib/api.ts:24`：

```ts
if (status === 401 || code === "invalid_token" || code === "unauthorized")
  return "登录已过期，请重新登录。";
```

错误文案层做得对（不再直出 `invalid token`），但文案本身没走 `t()`，zh-HK / en 用户会看到简体中文。与本轮"i18n 缺键归零"的成果不一致。

**改法**：错误映射表改为返回 i18n key，由调用方 `t()` 渲染；或给 `api.ts` 注入当前语言。同时把评估报告 §7.1 ③ 的裸中文检查纳入 CI 防回流。

### T5-2：`StatusTag` 的 `style` 可覆盖配色

`web/app/components/StatusTag.tsx` 中 `...style` 展开在 `background` / `color` / `border` **之后**，调用方一个 `style={{ color: "#999" }}` 就能破坏已验证的对比度。

**改法**：把 `...style` 移到颜色三项**之前**，使配色不可被覆盖；或把 `style` 收窄为不含颜色的白名单属性（如仅 `margin` / `maxWidth`）。

### T5-3：`#434343` 仍是死令牌

`web/app/globals.css:13` 的 `--mono-30: #434343` 全库消费数为 1（即它自己的定义行），不满足评估报告 §9.5 第 2 项「每级 ≥ 2」。

同批的 `#0A0A0A` / `#141414` 已正确删除，`#737373` 已按建议提拔使用，`#8C8C8C` / `#BFBFBF` 已归零——仅此一个遗漏。

**改法**：删除该行；若打算保留作图标默认色，则找一处真实使用。

### 附带（文档）

自交付报告 §3.1 写「i18n 条目三语完整性 1,321 条」，实测为 **1,431 条**（0 缺失）。结论无误，数字更正即可。

---

## 全部完成后的收尾

1. 重跑评估报告 **§7.1 全套八项静态检查** + **§7.2 ⑦ 多币种运行时校验**
2. 针对 T3，在 T2 的新数据集（含多段付款计划、季付、异币种）上**重新出具验收证据**——上一轮的证据不作数
3. `npm run type-check` / `npm test` / `npm run build` / `go test ./...` 全绿
4. 更新自交付报告的 §3 验收证据章节，并注明本工单各项的处置结果

**口径决策已在下发前拍板**（T3 缺陷 b 采用方案 A：显示当期实付租金 + 覆盖区间，不做按日折算），按工单执行即可，无需再确认。

若实施中发现工单里的判断与代码实际不符，**先反馈再动手**，不要自行扩大改动范围——尤其 T3 的 `MonthlyRent` 同名字段遍布五个业务模块，全局重命名会连带破坏签约前决策、续租卡与条款比价。
