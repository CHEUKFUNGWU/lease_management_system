# DESIGN.md — 设计系统与前端约束

> 适用范围：`web/` 全部前端代码
> 参考基准：[Vercel Design System](https://github.com/educlopez/design-bites/blob/main/design-mds/vercel.com/DESIGN.md)（克制）、[Beautiful UI](https://www.beautifului.dev/)（AI 界面的透明与可控）
> 现状偏离与整改计划：[docs/UIUX改善方案.md](docs/UIUX改善方案.md)

## 0. 这份文档的地位

**这是规则，不是计划。**

- 本文写的是**新代码必须遵守什么**。存量代码有大量违反项，那是已知债务，整改排期在 UIUX 改善方案里，不在本文重复。
- 判断标准很简单：**你写的这一行，一年后别人照着抄会不会把系统带偏？** 会，就按本文改。
- 本文与代码不一致时，**以本文为准并修代码**；如果确实是本文错了，改本文，不要在代码里开特例。

三条最常被问到的判定：

| 想做的事 | 答案 |
|---|---|
| 写死一个颜色值（`#1677FF`、`red`） | ❌ 一律用 token |
| 加一条 `!important` | ❌ 除非在 globals.css 顶部标注的 AntD 覆盖区内，且写明原因 |
| 加一个内联 `style={{}}` | ❌ 用类名 + token；动态值除外（见 §5.3） |

---

## 1. 单一真相源（当前是坏的，必须先知道）

设计令牌现在有**两套并存的定义，且已经漂移**：

| 定义位置 | 消费者 |
|---|---|
| `web/app/design-system/tokens.ts` | → `theme.ts` → `ThemeProvider` → **所有 Ant Design 组件**；`StatusTag.tsx` 也直接引用 |
| `web/app/globals.css` 的 `:root` | → **所有自定义 CSS 与 `var(--*)` 引用** |

已确认的漂移：

| 项 | `tokens.ts`（→ AntD 组件） | `globals.css`（→ 自定义 CSS） | 后果 |
|---|---|---|---|
| `border.default` | `#E5E5E5` | `--mono-90` = `#D9D9D9` | 同一屏里 AntD 卡片和自定义卡片边框颜色不同 |
| `border.strong` | `#D9D9D9` | `--mono-70` = `#A6A6A6` | hover 态边框深浅不一致 |
| 页面主标题 | `display` 32px / 700 | `.page-header-title` 28px / 700 | 两种页面标题尺寸 |
| 静态深度 | `depth.static` = `1px solid #D9D9D9` | `--shadow-static` = `0 0 0 1px rgba(0,0,0,.04)` | **一个用 border 一个用 shadow ring**，叠加处出现 1px 错位 |
| 等宽字体 | `JetBrains Mono` | `ui-monospace, SFMono-Regular, Menlo` | `JetBrains Mono` 从未被加载，是死配置 |

**规则：`tokens.ts` 是唯一真相源。** `globals.css` 的 `:root` 必须与之逐项对齐（AntD 的 `ConfigProvider` 只能吃 JS 值，所以源必须在 TS 侧）。

**改令牌时：改 `tokens.ts` → 同步 `:root` → 跑对齐测试。** 只改一边就是引入下一次漂移。上表五项的对齐属于改善方案阶段一，未完成前**不要基于任何一侧的值新增硬编码**。

---

## 2. 设计原则

1. **克制即工程。** 每一处装饰都要能说出为什么存在。删掉一个元素比加一个元素更需要理由。
2. **无彩底色。** 界面基本只有黑白灰。彩色是**信息**，不是装饰。
3. **颜色不是唯一信号。** 任何用颜色表达的状态，必须同时有图标或文字。色盲用户和灰度打印都要读得懂。
4. **层级靠尺寸和间距，不靠字重。** 字重只有三档。
5. **留白分区，少用分割线。** 优先用间距和微弱面色变化划分区域。
6. **交互只改颜色。** 不做位移、缩放、透明度动画——这是数据密集界面，位移会干扰读数。
7. **数据可信度是一等公民。** 覆盖率、来源、版本、decision-ready 必须可见，这是产品差异化所在（见 §10）。
8. **AI 过程必须可理解、可干预。** 见 §9。

---

## 3. 颜色

### 3.1 灰度（唯一的底色系统）

```
--mono-0   #000000   --fg-primary     标题、主要操作、关键数据
--mono-20  #262626   --fg-secondary   正文、重要标签
--mono-40  #595959   --fg-tertiary    描述、元数据
--mono-60  #737373   --fg-muted       占位符、禁用、提示（白底 AA 达标）
--mono-70  #A6A6A6   --border-strong  hover 边框
--mono-90  #D9D9D9   --border-default 标准边框
--mono-95  #F0F0F0   --bg-inset       表头、次级面板
--mono-98  #F7F7F7   --bg-surface     卡片、面板
--mono-100 #FFFFFF   --bg-page        页面画布
```

### 3.2 语义色

只有四种状态色，且**只有成对的 bg / text / border 三件套**可用：

| 状态 | text | bg | border |
|---|---|---|---|
| success | `#216E39` | `#ECF5EE` | `#CFE5D6` |
| info / processing | `#1F4E9C` | `#EDF2FA` | `#CFDDF2` |
| warning | `#8A5300` | `#FDF3E3` | `#F0DCB8` |
| error | `#A8071A` | `#FDEDED` | `#F5C9C9` |

全部 text 值在对应 bg 上通过 WCAG AA。

### 3.3 硬规则

- ❌ **不使用 Ant Design 的预设色名**：`<Tag color="red">`、`color="gold"` 一律禁止。它们不在我们的色板里，且是大面积填充。
- ❌ **状态不做大面积彩色填充。** 严重度、优先级这类信息用 **8px 状态点 + 常规字色**，不用彩色 Tag。一屏出现几十个红色标签会让真正的异常失去意义。
- ✅ 需要状态标签时用 `<StatusTag kind="success|processing|warning|error|neutral">`。
- **图表**：单序列用 `--chart-blue`；多序列优先用灰度深浅 + 形状区分，确需多色时不超过 3 色。数据缺口必须可见（不要 `connectNulls`），不能用 0 冒充缺失。
- **强调色**：蓝色只表示「可交互」。不要用蓝色做装饰或表示「好」。

---

## 4. 排版

### 4.1 字阶

| 用途 | 尺寸 / 行高 | 字重 | 字距 |
|---|---|---|---|
| 页面标题 display | 28 / 36 | 600 | -0.04em |
| 区块标题 h1 | 24 / 32 | 600 | -0.03em |
| 卡片标题 h2 | 18 / 28 | 600 | -0.02em |
| 小节 h3 | 15 / 24 | 600 | -0.01em |
| 正文 body | 14 / 22 | 400 | 0 |
| 次要正文 | 13 / 20 | 400 | 0 |
| 标签 caption | 12 / 16 | 500 | 0.01em |
| 元数据 micro | 11 / 14 | 500 | 0.02em |

### 4.2 字重只有三档

```
400 normal    正文、按钮、标签、表单、链接
500 medium    小标题、数字、代码、徽章
600 semibold  标题 —— 上限
```

❌ **不使用 700 和 800。** 存量有 `.page-header-title` 用 700、Logo 用 800，属于待清理项，**新代码不得新增**。层级用尺寸和字距拉开，不靠加粗。

### 4.3 数字

- 所有会变化的数值（KPI、金额、百分比、计数）必须 `font-variant-numeric: tabular-nums`，否则跳动时列宽会抖。
- 金额一律走统一 formatter，不在组件里各自 `toLocaleString`。
- 缺失值显示 `—`，**不显示 0**。0 和「没有数据」是两件事。

---

## 5. 间距与布局

### 5.1 4px 基数

```
4  8  12  16  24  32  48  64
xs sm md  lg  xl  2xl 3xl 4xl
```

图标与文字间距 4；行内元素 8；表单项与列表项 12；卡片内边距 16；卡片之间 24；页面区块之间 32。

### 5.2 布局常量

| 项 | 值 |
|---|---|
| 侧栏宽 / 收起宽 | 240 / 64 |
| 顶栏高 | 60 |
| 内容区最大宽 | 1440 |
| 断点 | 640 / 768 / 1024 / 1280 / 1440 / 1920 |

移动端断点是 768：`< 768` 走 Drawer 导航，桌面保留侧栏。

### 5.3 内联样式

❌ 默认禁止 `style={{}}`。它无法被 token 化、无法响应主题、无法被媒体查询覆盖——`AppLayout.tsx` 现有 30 处内联样式正是暗色模式做不了的原因。

✅ 唯一例外：**运行时才知道的动态值**（如 `width: ${percent}%`、图表容器高度）。即便如此也只写动态那一个属性，其余走类名。

---

## 6. 深度与边框

**统一用 shadow-as-border，不用 `border`。**

```css
--shadow-static:   0 0 0 1px rgba(0,0,0,.04)
--shadow-hover:    0 1px 2px rgba(0,0,0,.06), 0 0 0 1px rgba(0,0,0,.04)
--shadow-card:     0 0 0 1px rgba(0,0,0,.04), 0 2px 8px rgba(0,0,0,.04)
--shadow-dropdown: 0 0 0 1px rgba(0,0,0,.04), 0 4px 12px rgba(0,0,0,.06)
--shadow-modal:    0 0 0 1px rgba(0,0,0,.06), 0 8px 24px rgba(0,0,0,.08)
```

理由：环形阴影不占布局空间，不会在 hover 加边框时产生 1px 位移，且可与投影叠加。

圆角：控件 6 / 卡片 8 / 弹层 10 / 胶囊 9999。

---

## 7. 动效

```
100ms  hover、颜色变化
150ms  标准过渡
250ms  布局变化、页面切换
350ms  弹层开合

--ease-micro  cubic-bezier(0.4, 0, 0.2, 1)   标准
--ease-enter  cubic-bezier(0, 0, 0.2, 1)     出现
--ease-exit   cubic-bezier(0.4, 0, 1, 1)     消失
```

- ❌ 交互态不做 `transform` / `scale` / 位移。
- ❌ **不用 JS 写 hover**。`onMouseEnter` 改 `style.background` 有三个问题：键盘 focus 拿不到同样反馈、每次悬停触发 DOM 写入、与 CSS `:hover` 打架。一律用 `:hover, :focus-visible`。
- ✅ 尊重 `prefers-reduced-motion`（globals.css 已有全局处理，不要绕过）。

---

## 8. 复用组件

**新页面不要自己拼装这些，直接用：**

| 组件 | 用途 |
|---|---|
| `<AppLayout>` | 页面骨架。所有页面必须包在里面 |
| `<ProtectedRoute>` | 鉴权包装 |
| `<PageHeader title subtitle primaryAction secondaryAction>` | 页面标题区 |
| `<StatusTag kind>` | 状态标签，唯一合法的彩色标签 |
| `<GlobalSearch>` / `<NotificationBell>` | 顶栏功能 |

**待建（改善方案阶段二、三，建成后同样强制复用）：** `<DataTrustBar>`、`<StateBlock>`、`<SeverityDot>`、`<ToolChip>`、`<ThinkingTrace>`、`<SourceCitation>`、`<ApprovalCard>`、`<ContextCard>`、`<ConfidenceBadge>`。

在这些组件建成前，**不要在页面里现搓一个同名功能**——现在 `/operating-pulse` 用 `Empty` + `Spin` + `Alert` 拼出三套空/载/错语言，就是这么来的。

---

## 9. AI 界面规范

后端已经具备完整的可解释能力（run events、trace、evidence、confidence、proposal artifact）。**界面必须把它们呈现出来，不能只显示最终答案。**

| 必须可见 | 数据来源 |
|---|---|
| AI 调用了哪些工具、耗时、数据量 | run events → `<ToolChip>` |
| 推理轨迹（默认收起，可展开） | run trace → `<ThinkingTrace>` |
| 每条结论的引用来源，可点击回到证据 | `evidence.source_systems` → `<SourceCitation>` |
| 置信度，以及降级原因 | `confidence` + `reason` → `<ConfidenceBadge>` |
| 当前会话看的是哪批数据 | dataset / window / scope → `<ContextCard>` |
| 行动建议的采纳 / 修改 / 拒绝 | `retail_action_proposal` → `<ApprovalCard>` |

硬规则：

- **AI 建议与系统正式数据必须在视觉上可区分**，不得混排成同一种卡片。
- **置信度 0.40 和 0.90 必须看得出区别**，且必须能看到为什么降级。
- **权限拒绝要如实显示。** `scope_denied` 不能被 UI 软化成「暂无数据」——那会掩盖权限问题。
- **AI 出现在工作发生的地方。** 优先在当前页拉起 Drawer，而不是跳转到 `/ai-chat` 丢掉上下文。

---

## 10. 数据可信度展示

这是本产品相对通用 BI 的核心差异化，**必须有专属组件，不能塞进一条文本 Alert**。

任何展示经营数据的页面都要能回答：

1. 这是**模拟数据还是正式数据**（模拟必须显著标识）
2. **覆盖率**是多少（observed / expected store-days）
3. 数据**截至什么时候**，对比区间是哪一段
4. **来源系统**是什么
5. **口径版本**（formula / pulse / retail-kpi-v1）与**事实版本**区间
6. 是否 **decision-ready**

展示规则：

- 默认一行摘要，可展开看全量字段。晨检只需要知道「能不能用」，追查口径时才需要全部。
- **`decision_ready = false` 时必须显著降级**：整条变色，且所有 KPI 上加统一角标。这个判定不能藏在第五个 span 里。
- 覆盖不足不要用 0 补齐，显示 `—` 并说明原因。

---

## 11. 可访问性

- **焦点环用双环**：`0 0 0 2px var(--bg-page), 0 0 0 4px var(--accent-interactive)`。现有单环 `2px solid var(--fg-primary)`（纯黑）在 `--admin-surface #001529` 和 `--code-surface #1E1E1E` 上**不可见**，属待修项。
- 所有可交互元素必须键盘可达，且 `:focus-visible` 有可见反馈。
- 用原生语义元素（`<button>`、`<a>`），不要给 `<div>` 挂 `onClick`。
- 图标按钮必须有 `aria-label`。
- 正文对比度 ≥ 4.5:1，大字号 ≥ 3:1。
- 表格与列表需要可读的空态，不是一片空白。

---

## 12. 国际化

- ❌ **禁止硬编码 CJK 字面量。** 所有面向用户的文案走 `t("key", language)`。
- `web/app/lib/i18n.ts` 里每个 key 必须同时提供 `zh-CN` / `zh-HK` / `en` 三种译文。
- ⚠️ 现状：`/operating-pulse`、`/store-360`、`/scenario-workbench` 三页**完全没有引入 `t()`**，是已知违规项（改善方案阶段四）。**新页面不得重复这个错误。**

---

## 13. 止血条款

存量债务不在此文修，但**新代码不得再增加**以下任何一项：

| # | 禁止 | 替代做法 |
|---|---|---|
| 1 | 新增 `!important` | 提高选择器特异性，或改 AntD `ConfigProvider` token |
| 2 | 新增内联 `style={{}}`（动态值除外） | 类名 + CSS 变量 |
| 3 | JS `onMouseEnter` / `onMouseLeave` 改样式 | CSS `:hover, :focus-visible` |
| 4 | 硬编码颜色值 | token |
| 5 | `<Tag color="red">` 等 AntD 预设色 | `<StatusTag>` 或 `<SeverityDot>` |
| 6 | `fontWeight` > 600 | 用尺寸和字距做层级 |
| 7 | 硬编码 CJK 文案 | `t()` |
| 8 | `border: 1px solid` | `--shadow-*` 环形边框 |
| 9 | 用 0 填补缺失数据 | 显示 `—` 并说明原因 |

### PR 自检清单

- [ ] 没有新增 §13 里的任何一项
- [ ] 颜色、间距、圆角、字号全部来自 token
- [ ] 数值用了 `tabular-nums`，金额走统一 formatter
- [ ] 空态 / 加载态 / 错误态都有，且用的是共享组件
- [ ] 键盘可完整操作，`:focus-visible` 可见
- [ ] 1440×900 与 390×844 两档视口都验证过
- [ ] 展示经营数据的页面带了 §10 的可信度信息
- [ ] 文案走 `t()`，三种语言齐全

---

## 14. 已知偏离

以下是本文规则与当前代码**已确认的不一致**。它们是债务，不是先例，**不要拿来当「现在就是这么写的」的依据**：

| 偏离 | 规模 | 整改 |
|---|---|---|
| 内联 `style={{}}` | **906 处**（全 `web/app`，其中 `AppLayout.tsx` 30 处） | 改善方案阶段一 |
| `globals.css` 的 `!important` | 142 处 | 阶段一 |
| `.ant-` 覆盖 | 91 处 | 阶段一 |
| 字重 700 / 800 | 13 处 | 阶段一 |
| `border: 1px solid` | 11 处 | 阶段一 |
| JS hover handler | 6 处 | 阶段一 |
| `tokens.ts` 与 `:root` 漂移 | 见 §1 表 | 阶段一 |
| 字体走 Google Fonts `@import`，未用 `next/font` | 全仓 0 处 `next/font` | 阶段一 |
| 单环 focus，深色面上不可见 | 全站 | 阶段一 |
| 三个零售页无 `t()` | 3 页 | 阶段四 |
| 无暗色模式 | 全站 | 阶段四 |

整改排期与验收标准见 [docs/UIUX改善方案.md](docs/UIUX改善方案.md)。

## 15. 这份文档目前没有牙齿

必须写在最后，否则前面 13 节会被误读为「已经在执行的规则」：

| 执行机制 | 状态 |
|---|---|
| ESLint 配置文件 | ❌ **不存在**。`web/package.json` 有 `"lint": "next lint"`，但没有配置文件，CI 也不跑 |
| §1 要求的 `tokens.ts` ↔ `:root` 对齐测试 | ❌ **从未写过** |
| 组件测试 | ❌ 未安装 `@testing-library`，61 个用例全是纯逻辑 |
| §13 止血条款的自动拦截 | ❌ 无 |

**906 处内联样式说明「自觉遵守」从来没有奏效过。** 在补上执行机制之前，本文的效力仅限于人工 code review。

最小执行机制（当天可落，列为阶段一的第一项）：

```
1. ESLint 配置 + 内联样式规则（新文件 error，存量 warn）
2. CI 检查：新增 !important / fontWeight>600 / 硬编码 CJK 即失败
3. tokens.ts ↔ globals.css 对齐测试
```

来源：[docs/架构改善方案.md](docs/架构改善方案.md) §3。
