# UI/UX 改善方案 / UI-UX Improvement Plan

> 版本：v1.2（v1.0 设计系统诊断；v1.1 追加主线错配清单与 Agent 首页方案；v1.2 并入架构评审的前端结论）
> 日期：2026-08-13
> 适用产品：线下零售经营分析工作站 / Retail Performance Workstation
> 参考基准：[Beautiful UI（AI-native primitives）](https://www.beautifului.dev/)、[Vercel Design System](https://github.com/educlopez/design-bites/blob/main/design-mds/vercel.com/DESIGN.md)
> 前序文档（评审报告与转型战略输入）已归档，结论已并入本文；索引见 `docs/AI_文档索引与现行决策.md`。

## 0.1 前端合并排期（2026-08-18 从架构改善方案内联）

原架构评审的合并排期曾是本方案的外部依赖，该文档已归档，故把仍然有效的部分内联于此。

**架构候选的执行状态**（截至 2026-08-15）：

| 候选 | 状态 | 交付批次 |
|---|---|---|
| 01 错误契约（errcontract） | ✅ 已落地 | ENF/ERR 批次 |
| 02 Source Envelope | ✅ 已落地 | ENV 批次 |
| 03 单一 KPI 引擎 | ✅ 已落地 | KPI 批次（`retailkpi` + `retail-kpi-v1`） |
| 04 法人隔离值类型 | ✅ 已落地 | SEC-002/003 |
| 05 fetch 模块 | ⬜ 进行中 | FETCH-001 |

**因此前端只剩三个阶段**：

| 阶段 | 内容 | 工期 | 关口 |
|---|---|---|---|
| 4 | 样式系统收敛（本方案阶段一） | 1 周 | 依赖执行机制（DESIGN.md §15）先就位 |
| 5 | 候选 05 fetch 模块 + 本方案阶段二、三（组件层） | 3 周 | |
| 6 | 三栏 Agent 首页 + 双语回归 + 暗色模式 | 3 周 | |

原排期里「候选 04 必须先于 02/03」那条硬约束已随四者全部落地而失效，不再适用。

---

## 0. 摘要 / Executive Summary

转型已经把**能力**建起来了：`/operating-pulse`、`/store-360`、`/scenario-workbench` 和 `retail_operations@v1` Agent 全部通过验收。但界面还停留在「Ant Design 默认外观 + 局部覆盖」的阶段，没有形成属于这个产品的视觉语言。

本方案的判断是：**当前 UI 的问题不是不好看，而是三件事没有被系统化表达** ——

1. **数据可信度**。这是本产品最大的差异化（覆盖率、来源、事实版本、decision-ready），现在被压缩进一条 Ant `Alert` 里的十几个纯文本 span。
2. **AI 的可解释与可控**。我们已经有 trace、evidence、confidence、proposal artifact，但界面上没有对应的一等公民组件。
3. **视觉一致性**。`globals.css` 里 **142 处 `!important`、91 处 `.ant-` 覆盖**，AppLayout 里 30 处内联样式、6 个 JS hover handler。这不是风格问题，已经造成过真实缺陷。

第三点有实证：MAX-009 Review 2 记录的发布阻断缺陷就是「移动端 More/delete 被文件后部同 specificity `!important` 规则重新隐藏」。**样式系统已经在制造 bug，不只是在制造不一致。**

两个参考基准恰好分别解决其中两类问题：Vercel 的设计系统解决「克制与一致」，Beautiful UI 解决「AI 界面的透明与可控」。本方案把两者拆成可执行的四个阶段。

---

## 1. 参考基准与取舍 / What We Take From Each Reference

### 1.1 Vercel Design System —— 取「克制」

| 原则 / Principle | 具体做法 | 对本产品的意义 |
|---|---|---|
| **无彩底色 + 单一强调色**<br>Achromatic base, one accent | 界面基本只有黑白灰，蓝色专属于「可交互」 | 我们是密集数据界面，彩色必须留给**数据**，不能给装饰 |
| **状态色只做小圆点**<br>Status color as ≤10px dots | 12 种状态色只以约 10px 的圆点出现，从不做大面积填充 | 直接解决现在 Attention 表格里满屏红/黄 Tag 的问题 |
| **Shadow-as-border** | `box-shadow: 0 0 0 1px rgba(0,0,0,.08)` 取代 `border`，不产生 layout shift，可叠加 | 我们已有 `--shadow-static` token，但仍混用 11 处 `1px solid` |
| **三档字重**<br>Three weights only | 400 / 500 / 600，**不用 700** | 我们现在有 700（页面标题）和 800（Logo） |
| **负字距**<br>Negative letter-spacing | 大号标题 -1.28px ~ -2.28px，制造密度与精确感 | 我们 `page-header-title` 已用 -0.04em，方向正确，可推广 |
| **双环 focus**<br>Double-ring focus | `0 0 0 2px white, 0 0 0 4px #0072F5`，任何底色上都可见 | 我们是单环 `2px solid var(--fg-primary)`，在深色面上不可见 |
| **间距分区，不用分割线**<br>Spacing over dividers | 用留白和微弱面色变化分区 | 减少视觉噪音，让数据本身成为焦点 |
| **交互只改颜色**<br>Color-only interaction | 不做 transform / opacity / transition 位移 | 数据密集界面里位移动效会造成阅读干扰 |

**不照搬的部分：** Vercel 的 12px 正文和 48px h1 是营销站尺度；我们是长时间使用的工作台，正文按 13–14px、标题按 20–24px 收敛更合适。Vercel 也没有暗色模式文档，这部分我们自己定。

### 1.2 Beautiful UI —— 取「AI 的透明与可控」

Beautiful UI 的核心主张是：**让 AI 的过程可理解、可干预**。它提供的 19 类 primitive 里，与本产品直接对应的有六类：

| Primitive | 我们已有的后端能力 | 界面现状 |
|---|---|---|
| **Thinking / 推理过程展示** | Run events、`planner_usage`、trace | ❌ 未产品化 |
| **Expandable trace / 可展开轨迹** | 完整 Run Trace API | ❌ 未产品化 |
| **Tool chips / 工具调用标记** | `retail_operations@v1` 三个只读 Tool | ❌ 未产品化 |
| **Approval cards / 人工审批卡** | `retail_action_proposal` Artifact + confirm Modal | ⚠️ 有 Modal，但不是可复用组件 |
| **Context cards / 上下文卡** | 可信数据上下文（dataset、window、scope） | ⚠️ 拼在文本里 |
| **Source attribution / 来源引用** | `evidence.source_systems`、可点击来源 | ⚠️ 有链接，无统一样式 |

**结论：后端的 AI 可解释能力远超前端的表达能力。** 这是投入产出比最高的一块 —— 不需要新建后端，只需要把已有数据用对的组件呈现出来。

---

## 2. 现状诊断 / Current-State Diagnosis

以下每一条都可在代码中定位，不是主观评价。

### 2.1 P0 — 样式系统正在制造缺陷

| # | 问题 | 证据 | 后果 |
|---|---|---|---|
| D1 | `globals.css` 累积 **142 处 `!important`**、**91 处 `.ant-` 覆盖**，共 1068 行 | [`web/app/globals.css`](../web/app/globals.css) | 后写的规则以相同 specificity 覆盖前面的规则，且无法预测。**MAX-009 Review 2 的发布阻断缺陷正是此因** |
| D2 | AppLayout 有 **30 处内联 `style={{}}`** | [`AppLayout.tsx`](../web/app/components/AppLayout.tsx) | 内联样式无法被 token 化、无法响应暗色模式、无法被媒体查询覆盖 |
| D3 | **6 个 JS hover handler**（`onMouseEnter` / `onMouseLeave` 直接改 `style.background`） | [`AppLayout.tsx:310`](../web/app/components/AppLayout.tsx#L310)、`:365`、`:395` | 键盘 focus 得不到同样的反馈；每次悬停触发 DOM 写入；与 CSS `:hover` 状态冲突 |
| D4 | 边框实现混用：11 处 `1px solid var(--border-*)`，同时又定义了 `--shadow-static` 环形边框 | `globals.css`、`AppLayout.tsx:245/421` | 同一界面两套边框语言，圆角与阴影叠加处出现 1px 错位 |
| D5 | 字重超出三档：`page-header-title` 用 700，Logo 用 800 | `globals.css`、[`AppLayout.tsx:279`](../web/app/components/AppLayout.tsx#L279) | 层级靠字重堆砌而非尺寸/间距，重字面积一大就显得廉价 |

### 2.2 P0 — 差异化能力没有被界面表达

| # | 问题 | 证据 |
|---|---|---|
| D6 | **数据可信条是一条塞了 12 个 span 的 Ant `Alert`**：classification、basis、日期区间、对比区间、覆盖率、source、formula、dataset、generator、pulse 版本、fact version 区间、decision-ready、最高事实时间 | [`operating-pulse/page.tsx:285`](../web/app/operating-pulse/page.tsx#L285) —— 单行 JSX 超过 1200 字符 |
| D7 | **严重度用大面积彩色 Tag**（`color="red"` / `"gold"`），一屏可出现数十个 | [`operating-pulse/page.tsx:110`](../web/app/operating-pulse/page.tsx#L110) |
| D8 | 「交给 AI 分析」是**跳转到另一个页面**，用户离开了当前分析上下文 | [`operating-pulse/page.tsx:270`](../web/app/operating-pulse/page.tsx#L270) |
| D9 | 空态 / 加载态 / 错误态用了三套视觉语言：`Empty` + `Spin` + `Alert`，且 `Empty` 内部又嵌 `Space` + `Typography` + `Button` 手工拼装 | `operating-pulse/page.tsx:281–286` |

### 2.3 P1 — 双语能力已在代码中断裂

| # | 问题 | 证据 |
|---|---|---|
| D10 | 三个零售新页面**完全没有引入 `t()`**，界面文案 100% 硬编码简体中文 | `operating-pulse/page.tsx`、`store-360/page.tsx`、`scenario-workbench/page.tsx` 均无 `import ... from "../lib/i18n"` |
| D11 | `SUPPORTED_LANGUAGES = ["zh-CN"]`，语言切换器因此在 Header 被隐藏 | [`LanguageContext.tsx:20`](../web/app/context/LanguageContext.tsx#L20)、`AppLayout.tsx:338` |
| D12 | 但 `i18n.ts` 里**每个 key 都已备齐 zh-CN / zh-HK / en 三种译文**（6879 行） | [`web/app/lib/i18n.ts`](../web/app/lib/i18n.ts) |

**这一条对本次仓库更新尤其关键**：README 已经改为中英双语，但产品本身当前无法切换到英文。翻译资产是现成的，缺的只是新页面接线和把开关打开。

### 2.4 P2 — 可访问性与主题

| # | 问题 | 证据 |
|---|---|---|
| D13 | Focus ring 为单环 `2px solid var(--fg-primary)`（即纯黑），在 `--admin-surface: #001529` 和 `--code-surface: #1E1E1E` 上**不可见** | `globals.css:638`、`:863` |
| D14 | **无暗色模式**。`ThemeProvider` 只是 AntD `ConfigProvider` + `MotionConfig`，没有主题切换 | [`ThemeProvider.tsx`](../web/app/components/ThemeProvider.tsx) |
| D15 | 字体通过 CSS `@import url('https://fonts.googleapis.com/...')` 加载，**未使用 `next/font`**（全仓 0 处引用） | [`globals.css:5`](../web/app/globals.css#L5) |

D15 有三重代价：CSS 内的 `@import` 是渲染阻塞的；产生对 Google 的外部运行时依赖（内网/离线部署会退化为系统字体）；且没有 `size-adjust`，首屏会有可见的字体跳动。

---

## 3. 改善方案 / The Plan

### 3.1 阶段一（P0）：收敛设计语言 —— 1 周

**目标：让样式系统停止制造缺陷。**

**A. 建立 token 第二层，并冻结字重与彩色规则**

在 `globals.css` 的 `:root` 中补齐语义层（现有 mono/state token 保留不动，向下兼容）：

```css
:root {
  /* 字重：三档，禁用 700/800 */
  --weight-normal: 400;   /* 正文、按钮、标签、表单 */
  --weight-medium: 500;   /* 小标题、数字、代码 */
  --weight-semibold: 600; /* 页面标题 —— 上限 */

  /* 排版：工作台尺度，非营销尺度 */
  --text-display: 24px;  --tracking-display: -0.03em;
  --text-title:   18px;  --tracking-title:   -0.02em;
  --text-body:    14px;
  --text-label:   13px;
  --text-caption: 12px;
  --numeric: "Geist Mono", ui-monospace, "SF Mono", monospace;

  /* 间距：4px 基数 */
  --space-1: 4px;  --space-2: 8px;  --space-3: 12px; --space-4: 16px;
  --space-5: 24px; --space-6: 32px; --space-7: 48px;

  /* 圆角 */
  --radius-control: 6px;  /* 按钮、输入框 */
  --radius-card: 10px;    /* 卡片、面板 */
  --radius-pill: 9999px;

  /* 边框统一走 shadow-as-border，删除所有 1px solid */
  --ring-border: inset 0 0 0 1px rgba(0, 0, 0, 0.08);

  /* 双环 focus —— 任何底色上都可见 */
  --focus-ring: 0 0 0 2px var(--bg-page), 0 0 0 4px var(--accent-interactive);
  --accent-interactive: #1677FF; /* 唯一强调色，只表示"可交互" */

  /* 状态点：只做 8px 圆点，不做面积填充 */
  --dot-size: 8px;
}
```

**B. 状态色降级为「点 + 文字」**

新建 `<SeverityDot severity="critical|high|medium|low" />`，替换 Attention 表格与情景工作台里的彩色 `Tag`：

```tsx
// 之前：<Tag color="red">占用成本率飙升</Tag>   ← 大面积红色填充
// 之后：<SeverityDot severity="critical" /> 占用成本率飙升  ← 8px 圆点 + 常规字色
```

彩色从此只出现在两个地方：**8px 状态点** 和 **图表数据本身**。

**C. 清理 `!important` 与内联样式**

- 把 AppLayout 的 30 处内联样式迁到 `.app-header`、`.app-sider`、`.app-content` 等类；
- 把 6 个 JS hover handler 换成 CSS `:hover, :focus-visible`（顺带修复键盘用户拿不到反馈的问题）；
- 用一次性脚本审计 142 处 `!important`，凡是能靠提高选择器特异性或 AntD `ConfigProvider` token 解决的一律删除。**目标：`!important` 降到 20 以内，且集中在文件顶部一个明确标注的「AntD 覆盖区」。**

> 这一步的验收标准不是「更好看」，而是：**任意一条样式规则的最终生效值，可以只看一个地方就确定。**

**D. 字体自托管**

删除 `globals.css:5` 的 `@import`，改用 `next/font/local` 自托管 Inter（或换 Geist），并配置 `display: "swap"` 与中文回退栈。收益：消除渲染阻塞、消除外部依赖、消除首屏字体跳动。

---

### 3.2 阶段二（P0）：把「可信度」做成一等公民 —— 1.5 周

这是本产品对通用 BI 的核心差异化，必须有专属组件，而不是一条文本 Alert。

**新建 `<DataTrustBar />`** —— 替换 `operating-pulse/page.tsx:285` 的巨型 Alert：

```
┌──────────────────────────────────────────────────────────────────────┐
│ ● 模拟数据   Working   2026-08-01 – 08-07   覆盖 420/420 (100%)   ⓘ  │
└──────────────────────────────────────────────────────────────────────┘
                                                                    ↓ 点击展开
┌──────────────────────────────────────────────────────────────────────┐
│  对比区间   2026-07-25 – 07-31 · 覆盖 420/420 (100%)                 │
│  来源系统   retail_simulator                                          │
│  口径版本   formula v1 · pulse v1 · retail-kpi-v1                     │
│  事实版本   1 – 2 · 最高事实截至 2026-08-07 23:00                     │
│  数据集     SIM-a3f9c2 · generator v1 · seed 20260801                 │
│  判定       ✅ decision-ready                                         │
└──────────────────────────────────────────────────────────────────────┘
```

设计要点：

1. **默认一行，可展开**。晨检时用户只需要知道「数据能不能用」；追查口径时才需要全部 12 个字段。
2. **decision-ready = false 时整条变色并置顶**，且在所有 KPI 卡上加统一的「仅供查看」角标 —— 现在这个判定藏在 description 的第五个 span 里。
3. **模拟 / 正式用状态点区分**，不用大面积黄色底。
4. 全站复用：`/store-360`、`/scenario-workbench`、`/ai-chat` 共用同一组件，口径展示语言统一。

**配套新建 `<StateBlock />`** 统一空态 / 加载态 / 错误态，替换现在 `Empty` + `Spin` + `Alert` 三套拼装：

```tsx
<StateBlock kind="empty|loading|error|no-permission"
            title="当前正式数据窗口没有事实"
            description="请先导入并完成门店日事实映射。系统不会用 0 填补缺失。"
            action={<Button>去导入</Button>} />
```

---

### 3.3 阶段三（P1）：AI-native 组件层 —— 2 周

对齐 Beautiful UI 的六类 primitive。**后端数据全部已存在，这一阶段基本是纯前端。**

| 新组件 | 消费的现有数据 | 解决的问题 |
|---|---|---|
| `<ToolChip />` | Run events 里的 tool 调用 | 让用户看见 AI 到底查了什么：`⚙ retail.pulse · 420 store-days · 1.2s` |
| `<ThinkingTrace />` | Run trace / checkpoint | 可折叠的推理轨迹，默认收起，出问题时可展开自证 |
| `<SourceCitation />` | `evidence.source_systems` + 可点击来源 | 统一的引用角标 `[1]`，点击跳回 `/store-360` 对应证据 |
| `<ApprovalCard />` | `retail_action_proposal` Artifact | 把「AI 建议 → 人工确认」做成标准卡片：建议内容 / 影响金额 / 证据 / 置信度 / 采纳·修改·拒绝 |
| `<ContextCard />` | dataset、window、scope、classification | 会话开始时明示「AI 现在看的是哪批数据」 |
| `<ConfidenceBadge />` | `confidence` + `reason` | 0.40 与 0.90 必须在视觉上有区别，且必须能看到降级原因 |

**同时修复 D8（AI 入口跳走）**：把「交给 AI 分析」从 `router.push('/ai-chat?...')` 改为**在当前页拉起右侧 AI Drawer**。用户在 `/store-360` 看着某家门店的曲线提问，答案和证据就应该出现在同一屏 —— 这正是 Beautiful UI「让 AI 出现在工作发生的地方」的主张。`/ai-chat` 独立页继续保留，不删除任何入口。

---

### 3.4 阶段四（P1/P2）：双语回归、暗色模式与密度 —— 1.5 周

**A. 双语回归（P1，配合本次 README 双语化）**

1. 抽取三个零售页面的硬编码中文到 `i18n.ts`（英文译文按现有风格补齐）；
2. `SUPPORTED_LANGUAGES` 恢复为 `["zh-CN", "zh-HK", "en"]`，Header 语言切换器自动重新出现（`AppLayout.tsx:338` 的条件已经写好）；
3. 加一条 CI 检查：**新页面出现未走 `t()` 的 CJK 字面量则失败**，防止再次断裂。

**B. 暗色模式（P2）**

现有 token 已经是语义化的（`--bg-page` / `--fg-primary` 而非 `--white` / `--black`），所以只需加一层覆盖：

```css
:root[data-theme="dark"] {
  --bg-page: #0A0A0A;  --bg-surface: #141414;  --bg-inset: #1F1F1F;
  --fg-primary: #EDEDED; --fg-secondary: #A1A1A1; --fg-muted: #707070;
  --ring-border: inset 0 0 0 1px rgba(255, 255, 255, 0.10);
}
```

前置条件是阶段一必须完成 —— **只要还有 30 处内联样式和 142 处 `!important`，暗色模式就不可能做对。** 同时把 `--admin-surface: #001529` 和 `--code-surface: #1E1E1E` 这两个硬编码深色值并入主题层。

**C. 密度模式（P2）**

经营分析师会在这个界面上待一整天。当前 `Content` 内边距 32/48px、页面标题 28px，一屏能看到的门店行数偏少。提供 `comfortable` / `compact` 两档密度（AntD `ConfigProvider` 的 `componentSize` + 一组 CSS 变量），默认 `comfortable`，用户偏好持久化。

---

## 4. 路线图与验收 / Roadmap & Acceptance

| 阶段 | 内容 | 工期 | 验收标准 |
|---|---|---|---|
| **零** | 主线错配速修：命令面板补齐零售页面与门店搜索、品牌文案、AI Chat 开场白与快捷提问；定位 D21 弹窗缺陷 | 2–3 天 | 见 §7 各条验收；D21 成因确认并记录 |
| **一** | Token 层、三档字重、状态点、清理 `!important` / 内联样式 / JS hover、字体自托管 | **重估：按存量分摊，约 3–4 周**（见下） | `!important` ≤ 20；AppLayout 内联样式 = 0；JS hover handler = 0；`1px solid` = 0；无 `fontWeight > 600`；Lighthouse 无渲染阻塞字体请求 |
| **二** | `<DataTrustBar />`、`<StateBlock />`，四个页面接入 | 1.5 周 | 可信度信息一行可读、可展开全量；`decision_ready=false` 在 KPI 卡上可见；空/载/错三态全站同一组件 |
| **三** | 六个 AI-native 组件；AI 从跳页改为同页 Drawer | 2 周 | 每条 AI 结论可点开工具链与来源；proposal 走统一 `<ApprovalCard />`；`/store-360` 提问不离开页面 |
| **四** | 双语回归、暗色模式、密度模式 | 1.5 周 | 英文可切换且三个零售页无残留中文硬编码；CI 拦截新增硬编码；暗色下对比度全部 ≥ 4.5:1 |

总计约 **6 周**，与转型「先内部验证、不扩编」的节奏一致。

**阶段一工期重估（DOC-001 更新，2026-08-15）**：原「1 周」按存量实测不成立——STY-001~004 六周只清了 `AppLayout` 一个文件，全仓内联样式从 906 降到 839、`!important` 140 处、`1px solid` 25 处。阶段一按「守卫止血 + 按页面渐进」推进（STY-005），不再按单文件目标排期；STY-005 报告给出零售三页 + 合同详情的收敛样本与重估口径。

**阶段一是其余三个阶段的硬前置**，不能并行插队 —— 否则新组件会继续被 `!important` 覆盖。

---

## 5. 不做什么 / Out of Scope

明确划走，避免范围蔓延：

1. **不更换 UI 框架。** 继续用 Ant Design。目标是「用 token 驯服 AntD」，不是替换成 shadcn/Radix —— 那会让 23 个既有页面全部返工，违反「增量叠加、不破坏既有功能」的转型底线。
2. **不做物理重命名。** 路由、组件名、容器名保持 `lease_*`，等内部验证门槛通过后再定。
3. **不删除、不隐藏任何既有页面或导航入口。** 本方案只改视觉与交互层，功能面零删减。
4. **不为了像 Vercel 而照搬 Vercel 的尺度。** 12px 正文、48px 标题是营销站参数；我们取的是它的**原则**（克制、单一强调色、状态色不做面积、三档字重），不是它的数值表。
5. **不引入位移类动效。** 数据密集界面里的 transform/scale 会干扰读数，交互反馈只用颜色 —— 这一条与 Vercel 一致。

---

## 5.5 架构评审补充的前端问题（2026-08-13）

来自外部架构评审（该评审文档已归档，结论摘录于此）。这三条改变了本方案的规模判断：

| ID | 问题 | 与本方案的关系 |
|---|---|---|
| **D22** | **没有 fetch 模块。** `lib/api.ts` 175 个传输方法（62 个已死）被 30 个文件直接调用；29 个页面手搓 loading/error，58 个 loading 标志，3 种不同的竞态策略，169 处 token 当参数传。`notify.ts:15` 那个 3 秒去重是因为并行 loader 叠出重复 toast —— 在错误的层打了补丁 | **提前到阶段一之后（FETCH-001）**。`contracts/[id]` 和 `ai-chat` 是仅有的两个已有此接缝的页面，也是仅有的两个有像样测试的页面；2026-08-15 实测：`apiRequest` 调用 169 处、直接 import `lib/api` 38 个文件、手搓 loading 标志 50 个 |
| **D23** | **三个零售页对同样五个问题给出三种答案**：竞态（requestGate / `let active` / `useRef`）、403（一个处理两个漏）、`formula_version`（一个从响应取两个硬编码）、Decision Ready（三种渲染）、`t()`（三个都没有）。而 `operating-pulse/logic.ts` 已经事实上成了共享模块，另两页都从页面目录里 import 它 | 强化了阶段二、三的必要性；`changeTone` 在浏览器里硬编码「哪个方向算坏」，而 `retailpulse:465` 的 `direction` 从未被透出 |
| **D24** | **本方案与 DESIGN.md 都没有执行机制。** 无 ESLint 配置、无组件测试、token 对齐测试从未写过 | **阶段一的第一项**，见 DESIGN.md §15 |

**并且修正一处低估**：内联样式不是 30 处，是**全仓 906 处**（30 处是 `AppLayout.tsx` 的数字）。阶段一的工作量需要按这个量级重估。

## 6. 附：为什么这些阶段的顺序不能换

- 阶段零是纯文案与配置，不碰布局，所以可以插在最前面先止血；
- 阶段一不先做 → 阶段二三五的新组件会被 142 处 `!important` 随机覆盖，重演 MAX-009 Review 2 的缺陷；
- 阶段二不先做 → 阶段三的 AI 组件无法复用统一的可信度与来源展示，会各写一套；
- 阶段四的暗色模式依赖阶段一的 token 化，依赖阶段二三不再产生新的内联样式；
- 阶段五（§8 三栏首页）依赖阶段二的 `<DataTrustBar />` 和阶段三的 `<ApprovalCard />`，提前做等于把这两个组件在首页再写一遍。

**一句话：先让样式系统可预测，再谈让它好看。**

---

## 6.5 数据状态的判定与表达（DOC-001 新增，2026-08-15）

一次实机走查里四个问题都是同一个病：「没有数据」被呈现成「出错了」，或「出错了」被藏成「什么都没有」。判定层由此建立（STATE-001）：

| 状态 | 含义 | 呈现 |
|---|---|---|
| `empty` | 请求成功，事实为空 | 空态 + 说明为什么空 |
| `actionable` | 请求成功或失败，但用户能自己解决 | 空态 + 明确的下一步动作（带入口） |
| `failed` | 真异常，用户无能为力 | 错误态 + 原因 + 重试 |

- 判定是纯函数（`lib/dataState.ts` 的 `classifyDataState`），不在组件里。
- `scope_denied` 是独立第四态，永不并入三分法（权限拒绝必须保持原因）。
- 必须区分「后端说没有」（404/422 → empty，可升级 actionable）与「请求没打通」（网络/5xx → failed）。
- **与阶段二 `<StateBlock />` 的关系**：StateBlock 是呈现层（空/载/错的统一组件）；`classifyDataState` 是判定层（这一次到底是哪一种）。先有判定，再谈呈现。

---

## 7. 转型主线错配问题清单（2026-08-13 追加）

> 来源：转型合并后第一次真实使用巡检（登录首屏、命令面板、AI Chat）。
> 与 §2 的区别：§2 是**设计系统**问题（怎么长的），本节是**产品主线**问题（说的还是旧产品）。
> 状态口径：`OPEN` = 已确认未修；`UNDIAGNOSED` = 现象确认但成因未定位。

### 7.1 问题总表

| ID | 严重度 | 问题 | 证据 | 状态 |
|---|---|---|---|---|
| **D16** | P0 | **登录第一屏与转型主线完全不符。** 主页卡片为待复核合同、待过账分录、临近关键日期、总租赁负债、本月租赁费用、月结就绪度 —— 零个零售经营指标，没有 pulse、没有 attention 门店、没有任何进入 `/operating-pulse` 的入口 | [`web/app/page.tsx`](../web/app/page.tsx) L149–193 | OPEN |
| **D17** | P0 | **命令面板搜不到零售功能。** 可搜页面仅 todo / contracts / reports / monthly-closing / cashflow-forecast / sensitivity / settings 七项；`/operating-pulse`、`/store-360`、`/scenario-workbench`、`/performance`、`/portfolio`、`/pre-deal`、`/deal-compare`、`/standards`、`/roi`、`/audit-logs`、`/agent-metrics` **全部缺失**。实体搜索也只查合同，**不能按门店编码或门店名搜索** | [`GlobalSearch.tsx:41`](../web/app/components/GlobalSearch.tsx#L41)（pageItems）、`:51`（actionItems）、`:81`（只调 `contractApi.list`） | OPEN |
| **D18** | P0 | **AI Chat 自我介绍还是租赁助手。** 开场白称"我是租赁管理 AI Agent"，四条能力全部是 Excel 台账 / PDF 合同 / 计量结果 / 审批状态；三条快捷提问全部是租赁（折现率缺失合同、待审批事项、即将到期合同）；五个 skill chip 中"零售经营分析"排在最后一位 | [`i18n.ts:256`](../web/app/lib/i18n.ts#L256)、`:401`、`:406`、`:411`、`:5315` | OPEN |
| **D19** | P1 | **品牌文案仍为旧产品。** `app.title` = "租赁管理系统"，Header 与 Sider footer 都用它；Logo 徽标硬编码字符串 `L16`（Lease + IFRS 16） | [`i18n.ts:224`](../web/app/lib/i18n.ts#L224)、[`AppLayout.tsx:279`](../web/app/components/AppLayout.tsx#L279) | OPEN |
| **D20** | P1 | **首页没有任何 AI 入口。** 主页两个按钮是"新增合同"和"在 AI Chat 上传文件"（跳转到录入场景）。结合 §2 的 D8（三个零售页的"交给 AI 分析"也是跳走），**产品里不存在"在当前上下文里用 AI"的路径** | [`page.tsx:151`](../web/app/page.tsx#L151)–L152 | OPEN |
| **D21** | P1 | **命令面板弹窗布局错乱。** JSX 顺序为 输入框 → 结果列表 → 键盘提示，但实际渲染是：标题浮在弹窗垂直中部、中间大片空白、输入框被挤到弹窗最底部且超出视口 | 用户截图（localhost:3000，⌘K 打开）；JSX 见 [`GlobalSearch.tsx:169`](../web/app/components/GlobalSearch.tsx#L169)–L206 | **UNDIAGNOSED** |

### 7.2 D21 的调查记录

已排除的可能：

- `.command-palette-results`（`display:grid; max-height:420px; margin-top:12px`）、`.command-palette-item`、`.command-palette-hint` 无异常；
- `.ant-modal-content` / `-header` / `-body` / `-footer` 四条覆盖只改圆角、内边距和阴影，**未设置任何 height / min-height / flex-direction**；
- 全文件 `height:` / `min-height:` 逐条查过，无一命中弹窗；
- 断点 media query（含 `.ant-layout-header .global-search { display:none }` 一段）不影响弹窗内部布局。

**结论：静态读码无法定位，不做臆测归因。** 下一步需要真实复现后读计算样式（`.ant-modal`、`.ant-modal-content`、`.ant-modal-body` 的 computed `display` / `height` / `flex-direction`，以及 `.command-palette-results` 的实际高度）。复现需要登录，密码须由用户本人输入。

**倾向性怀疑（待证实，不作为结论）：** 与 §2 的 D1 同源 —— 142 处 `!important` 中存在后位规则以相同 specificity 覆盖了弹窗布局。若证实，D21 应并入阶段一一起修，而不是单独打补丁。

### 7.3 各条验收标准

| ID | 验收标准 |
|---|---|
| D16 | 登录首屏包含当日经营简报与 attention 门店，且可一键进入 `/operating-pulse`；原有租赁 / 会计四张卡一张不少（增量叠加底线） |
| D17 | 命令面板覆盖全部 18 个业务路由；支持按门店编码与门店名搜索；新增页面未登记时 CI 失败 |
| D18 | 开场白、能力列表、快捷提问以零售经营为主，租赁 / IFRS 16 作为其中一类保留；skill chip 顺序反映主线 |
| D19 | 显示名与徽标更新；**代码命名空间 `lease_*` 不变**（见下方红线） |
| D20 | 首页与三个零售页均可在当前上下文唤起 AI，不跳走（与 §3.3 的 Drawer 改造合并验收） |
| D21 | 成因写入本节；修复后 ⌘K 弹窗在 1440×900 与 390×844 下输入框均在标题正下方 |

### 7.4 一条必须守住的区分：显示名 ≠ 命名空间

D19 容易被扩大成一次全仓重命名，必须提前划清：

| 可以改（文案层） | 不能改（代码层） |
|---|---|
| `app.title`、Logo 徽标、页面标题、开场白、README | repo 名、容器名、数据库名、JWT secret、`lease_*` 包名与路由、`lease-agent` CLI |

依据：可行性报告明确「底层大规模物理重命名应等内部技术门槛通过后再决定」，且 2026-05 已经改过一次名。**把改文案的需求执行成改命名空间，是本条最大的风险。**

---

## 8. 追加方案：Agent 优先的三栏首页（阶段五）

### 8.1 需求来源与判断

用户提出：参照 Codex / Claude Code Desktop 的布局，让 AI Agent 成为主页面，最左仍是导航栏，Agent 右侧放待办卡片。

**方向采纳，但形态需要调整：中间栏首屏不应是空的输入框。** 两条理由：

1. **任务性质不同。** Claude Code 首页是空 prompt，因为编码任务每次都不一样。经营晨检相反 —— 每天问的是同一个问题（"昨天哪儿出问题了"）。让分析师每天早上重复打同一句话，比现在的仪表盘更低效。
2. **会承诺后端给不了的能力。** `retail_operations@v1` 是**三个只读 tool**，action proposal 明确不落业务表（见五条底线第 5 条）。一个空 chat 首页会诱导用户下达执行类指令（"帮我把 A 店排班改了"），而系统只能回建议。这个预期落差比"首页不够新"伤害更大。

**采纳形态：三栏骨架照办，中间栏首屏是 Agent 自动生成的「今日经营简报」。** 页面加载即由 Agent 调 `retail.pulse` 跑一次晨检，输出带引用编号的卡片流；底部常驻 composer 支持追问。既拿到 Agent 优先的心智，又不越过只读边界，还把可信度与引用摆在最显眼的位置。

### 8.2 布局

```
┌──────────┬────────────────────────────────────┬─────────────────┐
│ 导航      │  Agent 工作区                       │ 行动与待办       │
│          │                                     │                 │
│ 经营分析  │  ◈ 今日经营简报  ● 模拟 覆盖100% ⓘ  │ 待确认建议 (2)   │
│  脉搏     │  ─────────────────────────────      │ ┌─────────────┐ │
│  门店360  │  昨日销售 -8.2%，主要来自 3 家门店   │ │A店人工 -10% │ │
│  情景     │  ⚙ retail.pulse · 420 store-days    │ │+2.1万/月 [1]│ │
│          │                                     │ │ 采纳 改 拒  │ │
│ 日常作业  │  ① SIM-006 占用成本率 +10.08pp [1]  │ └─────────────┘ │
│  待办     │  ② SIM-023 客流连续下降 [2]         │                 │
│  合同     │  ③ SIM-041 转化跌破同群 P25 [3]     │ 待我审批 (8)     │
│          │                                     │ 待过账分录 (2)   │
│ 会计合规  │  ▸ 展开推理轨迹                      │ 临近关键日期 (4) │
│  报表     │                                     │ 月结阻塞项 (2)   │
│  结账     │  ┌───────────────────────────────┐  │                 │
│  审计     │  │ 追问，或让 Agent 起草行动…  ↑ │  │                 │
│          │  └───────────────────────────────┘  │                 │
└──────────┴────────────────────────────────────┴─────────────────┘
```

### 8.3 三条设计主张

1. **首屏自动跑一次简报，不等提问。** 输出带引用编号的卡片流，可信度信息（模拟 / 正式、覆盖率、decision-ready）挂在简报头部，复用阶段二的 `<DataTrustBar />`。
2. **中间产出 → 右栏沉淀。** Agent 提出的 action proposal 直接落到右栏"待确认建议"，用阶段三的 `<ApprovalCard />`。这把只读边界转成产品优势：Agent 只提议，人来确认，而"确认"有明确落点。
3. **右栏是收敛区，不是第二个仪表盘。** 现有主页的待复核合同 / 待过账分录 / 临近关键日期 / 月结就绪度四张卡原样迁入，租赁与会计入口一个不少。

### 8.4 依赖与风险

- **硬依赖阶段一。** 三栏布局会大量新增布局 CSS，在 142 处 `!important` 未清理前落地，新组件会被随机覆盖 —— 这正是 MAX-009 Review 2 缺陷的复现条件。
- **硬依赖阶段二、三。** `<DataTrustBar />` 与 `<ApprovalCard />` 必须先存在，否则会在首页重写一遍。
- **不删旧首页组件。** `DashboardHeader`、`MoneyKPICard`、`UpcomingDatesCard`、`WorkQueueSummaryCard` 全部保留并在右栏复用。
- **移动端需要单独设计。** 三栏在 390px 下不成立，需退化为"简报单栏 + 待办 Drawer"。

### 8.5 建议执行顺序

**阶段零（速修 D17/D18/D19 + 定位 D21）→ 阶段一（样式收敛）→ 阶段二、三（组件层）→ 阶段五（三栏首页，D16/D20 在此闭环）。**

阶段零可以立即开始：全部是文案、路由登记和配置，不碰布局，与后续阶段无冲突，且能顺带验证样式系统的实际腐化程度。

## 9. F 批次：财务视角的 UI/UX 与术语整改（2026-08-22 执行）

按 `docs/execution/财务视角UIUX与术语整改_任务指令.md`（transient 工单，复核后可删）完成的整改。审查方法：ui-ux-pro-max（119 条 UX 准则）+ unslop，站在财务会计 / FP&A 分析师 / Finance BP / 经营分析师四个岗位视角逐页核查。**未改任何计算逻辑与后端契约。**

### F0 输入正确性与信息泄漏

| 票 | 内容 | 机械证据 |
|---|---|---|
| F0-1 | 期初合约行三个输入补可视 label（第一行 `<label htmlFor>`、后续行 aria-label），placeholder 降级为填写示例 | `contract-rows.test.tsx`（4 用例：label↔id 配对、placeholder≠语义文案、第二行无重复 label、两张子表 id 互斥） |
| F0-2 | `/financial-model` 五处机器枚举改经 i18n 映射表（run 状态 / 勾稽总状态 / gap.kind / 导出粒度；勾稽明细行状态列一并修）；`enums.ts` 新增联合类型键的 Record 映射，漏值即编不过 | `page-enums.test.ts` + `gap-kinds.test.ts`（GAP_KIND_LABEL 每键 ↔ engine.go 字面量跨语言断言） |
| F0-3 | `store-pnl` 同业对标列不再渲染 `insufficient_peers` 等英文枚举（列内 + 头部 StatusTag 两处） | `peer-status.test.ts`（四取值逐一断言中文输出 + 键集锁回 project.go 注释） |
| F0-4 | 用户文案清除内部路线图编号（「B 阶段」等），保留诚实不可用态 | `i18n-no-roadmap.test.ts` 全字典三语扫描（阶段编号 / Wx / Px-y 形态） |
| F0-5 | 「枚举不许裸渲染」CI 守卫 `enum-leak.test.ts`（四规则扫全树 tsx） | 已验证修复前红（命中 financial-model:311/372/391-393 + store-pnl:260）、修复后绿；存量两处（monthly-closing `{result.status}`、store-360 `({response.currency_status})`）逐条记账并写明理由 |

### F1 术语与文案（unslop）

译名统一：run→测算、Gap→缺口清单说明、30-day run-rate→30 天日均折算值、observed store-days→实际观测店天数、「白名单 DSL」→「仅支持受限语法」、「不绿/人话」语域收敛。F1-4 拆轴：数据标识（正式数据/模拟数据）与版本态（工作底稿口径 Working/正式过账口径 Official）分离——`trust.classification_*` 去掉复合后缀，financial-model 分类下拉旁挂独立版本态标签+tooltip，ai-chat 上下文标签改走 t()。守卫：`i18n-finance-copy.test.ts`。

### F2 帮助覆盖

`financial-model` / `store-pnl` / `monthly-closing` 三页补 HelpTrigger（help-content.ts 三函数 + i18n 三语）。内容回答真实疑问：三道闸各拦什么、勾稽不过下一步做什么、发布的计划版本谁能看到；经营口径 vs IFRS 16 口径为何不同、Decision Ready 不满足还能不能用；月结动作与总账的关系、哪些动作不可逆。financial-model 的 flow 直接复用页面自身 finmodel.step_* 五步键。测试：`help-content-f0.test.tsx`。

### F3 已拍板项

- **F3-1 假设输入区重做为键值表单**：每行中文名 + 单位后缀数字输入 + 口径说明；裸 JSON 降级为折叠的「高级：粘贴 JSON」，两入口汇合到 `workbench.applyAssumptionFormValues → parseAssumptions` 同一接缝。单位换算只在展示层（界面 2% ↔ payload 0.02），单位登记表 `hints.ts ASSUMPTION_UNITS` 逐键从后端公式推得（percent×11 / days×4 / multiple=ramp_factor（引擎 (1+v)，显示 payload+1）/ amount=allocation、marketing）。未知键原样保留并诚实标注。守卫：copy-guard 更新且全绿、hints.test 后端键集断言仍绿、新增 assumption-form.test.tsx（单位互逆 + SSR 渲染 + 回调→接缝→payload 全链路）。
- **F3-2 导航拆组**：「分析与决策」13 项按 CONTEXT.md 领域边界拆为「经营分析」（零售经营域 8 项）与「租赁决策」（租赁/交易域 5 项）；导航标签对齐页面标题（租赁组合分析 / FP&A 经营工作台 / 未来现金流与财务预测 / 促销活动与 ROI 闭环）。测试：`nav-grouping.test.ts`。「经营脉搏 / 经营驾驶舱」重叠按工单 §4.1 未动。

### 验证

```
npm run type-check ✅   npm test ✅（77 文件 / 472 用例）
npm run build ✅        npm run lint ✅（--max-warnings 0 + enforce-design 无违规）
```

遗留（下一批次）：monthly-closing 批次状态与 store-360 currency_status 的枚举翻译（后者需后端先固化取值全集）；「经营脉搏/经营驾驶舱」功能重叠待产品票。
