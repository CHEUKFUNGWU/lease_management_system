# UI/UX 改善方案 / UI-UX Improvement Plan

> 版本：v1.0
> 日期：2026-08-13
> 适用产品：线下零售经营分析工作站 / Retail Performance Workstation
> 参考基准：[Beautiful UI（AI-native primitives）](https://www.beautifului.dev/)、[Vercel Design System](https://github.com/educlopez/design-bites/blob/main/design-mds/vercel.com/DESIGN.md)
> 前序文档：[UIUX 设计与交互提升评估报告](./UIUX_设计与交互提升评估报告.md)、[产品转型可行性报告与规划](./线下零售经营分析工作站_产品转型可行性报告与规划.md)

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
| **一** | Token 层、三档字重、状态点、清理 `!important` / 内联样式 / JS hover、字体自托管 | 1 周 | `!important` ≤ 20；AppLayout 内联样式 = 0；JS hover handler = 0；`1px solid` = 0；无 `fontWeight > 600`；Lighthouse 无渲染阻塞字体请求 |
| **二** | `<DataTrustBar />`、`<StateBlock />`，四个页面接入 | 1.5 周 | 可信度信息一行可读、可展开全量；`decision_ready=false` 在 KPI 卡上可见；空/载/错三态全站同一组件 |
| **三** | 六个 AI-native 组件；AI 从跳页改为同页 Drawer | 2 周 | 每条 AI 结论可点开工具链与来源；proposal 走统一 `<ApprovalCard />`；`/store-360` 提问不离开页面 |
| **四** | 双语回归、暗色模式、密度模式 | 1.5 周 | 英文可切换且三个零售页无残留中文硬编码；CI 拦截新增硬编码；暗色下对比度全部 ≥ 4.5:1 |

总计约 **6 周**，与转型「先内部验证、不扩编」的节奏一致。

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

## 6. 附：为什么这四个阶段的顺序不能换

- 阶段一不先做 → 阶段二三的新组件会被 142 处 `!important` 随机覆盖，重演 MAX-009 Review 2 的缺陷；
- 阶段二不先做 → 阶段三的 AI 组件无法复用统一的可信度与来源展示，会各写一套；
- 阶段四的暗色模式依赖阶段一的 token 化，依赖阶段二三不再产生新的内联样式。

**一句话：先让样式系统可预测，再谈让它好看。**
