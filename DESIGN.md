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

## 1. 单一真相源

**`web/app/design-system/tokens.ts` 是唯一真相源。**

| 定义位置 | 消费者 |
|---|---|
| `tokens.ts` | → `theme.ts` → `ThemeProvider` → **所有 Ant Design 组件**；`StatusTag` 等组件也直接引用 |
| `globals.css` 的 `:root` | → **所有自定义 CSS 与 `var(--*)` 引用**，必须逐项镜像 `tokens.ts` |

源必须在 TS 侧，因为 AntD 的 `ConfigProvider` 只吃 JS 值。

**改令牌的顺序：改 `tokens.ts` → 同步 `:root` → 跑对齐测试。** 只改一边就是引入下一次漂移——两套定义曾经漂移过一轮（STY-001），修的时候才发现边框色、标题字重、静态深度、等宽字体四处各不相同。

对齐由 `web/app/design-system/tokens-alignment.test.ts` 守护，任何一边改动未同步即失败。暗色令牌另有 `theme-dark.test.ts`。

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

### 3.1 中性阶（唯一的底色系统）

2026-08 换色为 **Coastal Navy（Slate 冷调中性阶）**，取代此前的纯灰阶。它仍然是「无彩底色」——只是中性色带极轻的蓝调，比纯灰更有层次且不刺眼。**这些是 `tokens.ts` 的实际值，不要凭印象写 `#595959` 一类的旧灰。**

| 语义 | 值 | 用途 |
|---|---|---|
| `foreground.primary` | `#0F172A` | 标题、主要操作、关键数据 |
| `foreground.secondary` | `#1E293B` | 正文、重要标签 |
| `foreground.tertiary` | `#334155` | 描述、元数据 |
| `foreground.muted` | `#64748B` | 占位符、禁用、提示、对比基线 |
| `foreground.inverse` | `#FFFFFF` | 深色底上的文字 |
| `border.strong` | `#64748B` | hover / 激活边框 |
| `border.default` | `#E2E8F0` | 标准边框、卡片描边 |
| `border.subtle` | `#F1F5F9` | 内部分隔线、表格行 |
| `background.inset` | `#F1F5F9` | 表头、次级面板 |
| `background.surface` | `#FFFFFF` | 卡片、面板 |
| `background.page` | `#F8FAFC` | 页面画布 |

**页面画布不是纯白。** `#F8FAFC` 与 `#FFFFFF` 的卡片形成层次，这是不靠边框分区的前提（§2 原则 5）。

**身份面（identity surface）例外**：`background.brandSlab` = `#000000` / `onBrandSlab` = `#FFFFFF`。登录页那块品牌黑板是**图形**不是前景色，两个主题下都保持黑底白字（DARK-003）。不要拿 `--fg-primary` 去画它，暗色模式会把它翻成白色。

### 3.2 语义色

**五种**状态色，且**只有成对的 bg / text / border 三件套**可用：

| 状态 | text | bg | border |
|---|---|---|---|
| success | `#065F46` | `#ECFDF5` | `#A7F3D0` |
| processing / info | `#1E40AF` | `#EFF6FF` | `#BFDBFE` |
| warning | `#92400E` | `#FFFBEB` | `#FDE68A` |
| error | `#9F1239` | `#FFF1F2` | `#FECDD3` |
| neutral | `#475569` | `#F1F5F9` | `#E2E8F0` |

全部 text 值在对应 bg 上通过 WCAG AA。

另有**独立的强调色** `state.*`，用于图标与细描边，不用于大面积填充：success `#059669`、warning `#D97706`、error `#E11D48`、info `#2563EB`。

> `colors.morandi.*` 是上一轮配色的遗留映射，标着 `legacy … backwards compatibility`。**新代码不要引用**，它随时会被删。

### 3.3 硬规则

- ❌ **不使用 Ant Design 的预设色名**：`<Tag color="red">`、`color="gold"` 一律禁止。它们不在我们的色板里，且是大面积填充。
- ❌ **状态不做大面积彩色填充。** 严重度、优先级这类信息用 **8px 状态点 + 常规字色**，不用彩色 Tag。一屏出现几十个红色标签会让真正的异常失去意义。
- ✅ 需要状态标签时用 `<StatusTag kind="success|processing|warning|error|neutral">`。
- **强调色**：蓝色只表示「可交互」。不要用蓝色做装饰或表示「好」。

### 3.4 图表色板

图表有自己的一套令牌（`colors.chart.*`），是**深炭石板系**，不是通用强调色。**槽位是有语义的，按语义取，不要按「第几条线」取。**

| 令牌 | 值 | 语义 |
|---|---|---|
| `chart.primary` | `#0F172A` | 主序列 |
| `chart.blue` | `#1E293B` | 次主序列（名字是历史遗留，它不是蓝色）|
| `chart.purple` | `#334155` | 第三序列（同上，不是紫色）|
| `chart.secondary` | `#64748B` | 对比基线、上年同期 |
| `chart.accent` | `#2D4B46` | 需要区分的正向序列 |
| `chart.negative` | `#7F473E` | 负向 / 恶化序列 |
| `chart.fill` | `#E2E8F0` | 面积填充、参考带 |

规则不变，且因为色板变深更要守住：

- 多序列优先用**深浅 + 形状**区分，确需多色时不超过 3 色。七个槽位是调色板，不是让你一次用满。
- 数据缺口必须可见——**不要 `connectNulls`**，不能用 0 冒充缺失。
- `chart.blue` / `chart.purple` 的名字与实际颜色已经对不上，**按语义列选，不要按名字选**。

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
| `<GlobalSearch>` / `<NotificationBell>` / `<CommandPalette>` | 顶栏功能 |
| `<ThemeToggle>` / `<LanguageToggle>` | 主题与语言切换 |

**曾列为「待建」的共享组件已全部建成（`ContextCard` 除外），现在是强制复用项：**

| 组件 | 用途 |
|---|---|
| `<DataTrustBar>` | 覆盖率 / 来源 / 版本 / decision-ready（§10 的唯一渲染方式） |
| `<StateBlock>` | 空 / 载入 / 错误 / `scope_denied` 四态。**`scope_denied` 与 `failed` 必须可区分**，不得软化成「无数据」 |
| `<SeverityDot>` | 严重度点，替代彩色 Tag |
| `<StatusTag>` | 状态标签，唯一合法的彩色标签 |
| `<ToolChip>` / `<ThinkingTrace>` / `<SourceCitation>` / `<ConfidenceBadge>` / `<ApprovalCard>` | AI 界面五件套，见 §9 |

另有成套目录组件：`components/charts/`、`components/bento/`、`components/dashboard/`、`components/enterprise-table/`。**新页面从这里取，不要自己拼。**

`<ContextCard>` 仍未建。需要它时先建组件再用，不要在页面里现搓——`/operating-pulse` 早期用 `Empty` + `Spin` + `Alert` 拼出三套空/载/错语言，`<StateBlock>` 就是为收拾那个局面才建的。

### 8.1 容器原语（PRIM-001，2026-08-15）

两个容器坑各已发生一次，规则如下，**守卫测试强制**（`web/scripts/container-primitives.test.ts`）：

1. **表格横向滚动必须走 `tableScrollX(rowCount, width)`**（`web/app/lib/tableScroll.ts`）。直接写 `scroll={{ x: 数字 }}` 会在空表上渲染一个幽灵横向滚动条（FIX-004 的原始缺陷，六周后在付款计划页签再次被抓到）。例外：表格本身在「有数据才渲染」的条件分支里（`rows.length ? <Table/> : <Empty/>`），此时空表不存在，可保留字面量——守卫按行检查 `?` 条件放行。
2. **recharts 图表（LineChart / BarChart / Sankey / AreaChart 等）必须包在 `<ResponsiveContainer>` 里**。裸图表拿不到宽度，画出一个空盒子（FIX-028：桑基图「状态: complete」下面是个高盒子）。图表容器高度是唯一允许的静态内联（动态值条款）。
3. **第三类容器检查结论**：虚拟列表全仓不存在；Drawer 内表格同样受规则 1 约束（Drawer 宽度固定，横向滚动仍按数据行数走）。

---

### 8.2 页面组装范式（2026-08 已批准的现行做法）

前面几节写的是**令牌与禁令**，这一节写的是**一个分析页长什么样**。以 `/store-360`、`/operating-pulse`、`/cashflow-forecast` 为准——**这三页的做法是标准，新页面照抄，不要另创版式。**

**页面骨架，自上而下固定这个顺序：**

```
PageHeader        标题 + 一句话副标题（口径声明，如「仅供 Working 经营分析，不作解释性判断」）
                  右侧动作区：帮助 · 刷新 · 导出 · 交给 AI 分析 · 情景分析 · 返回上级
筛选条            数据分类切换 · 主体选择 · 日期 · 窗口类型 · 窗口档位 · 数据源
DataTrustBar      分类 · 口径 · 覆盖率 · decision-ready · 展开全部口径
主体卡片          KPI 网格 → 图表区（左右两栏）
辅助指标          低密度多列，次要指标放这里，不与主 KPI 争夺注意力
口径脚注          右下角小字，说明本页数字的边界
```

**KPI 卡片**（`.store-360-kpi-card`）：

| 层 | 规格 |
|---|---|
| 标签 | 13 / 400，`--fg-tertiary` |
| 数值 | **22 / 600**，`tabular-nums`，字距 `-0.02em`，`--fg-primary` |
| 变化 | 12，改善 `--state-success-text` `#059669` / 恶化 `--state-error-text` `#E11D48` |
| 对比基线 | 12，`--fg-muted`，格式固定为「对比: <值>」 |
| 卡片内边距 | `16px 20px` |

**百分比指标的变化用 `pp`（百分点）不用 `%`**——毛利率从 30.92% 到 31.37% 是 `+0.45pp`，不是 `+1.45%`。这两者含义不同，混用会让人读错。

**变化色只上在「变化」和「结论性数值」上，不上在事实数值上。** 销售额 61,912.41 是黑的，它的 `+8.39%` 是绿的。例外是现金流这类本身有方向的数值（毛利额、净现金流），值本身可以带色。

**Segmented 分段控件**（`.precision-segmented`）：未选中 `--bg-inset` 底 + `--fg-secondary` 字；**选中态是 `#0F172A` 黑底 + `#FFFFFF` 白字**。这是全站唯一的高对比选中态，不要改成蓝色或描边。

**图表语义配色**——三色制，对应中性 / 正向 / 负向：

| 角色 | 令牌 | 用在 |
|---|---|---|
| 中性 / 主序列 | `chart.primary` `#0F172A` | 期初期末、总量、基准线 |
| 正向 / 流入 | `chart.accent` `#2D4B46` | 毛利流入、改善项 |
| 负向 / 流出 | `chart.negative` `#7F473E` | 租金支出、恶化项 |
| 参考带 / 填充 | `chart.fill` `#E2E8F0` | 同群置信带、面积 |

单序列趋势图保持**纯黑线 + 灰色同群带 + 虚线中位数**，不上色——颜色留给「有正负语义」的图（贡献分解、现金流入出）。

**守恒类图表必须带守恒说明行**：桑基图与瀑布图下方固定一行「平衡状态 · 未归因金额 · 指标公式版本」。不平就显示不平，不许调平。

**口径分离的脚注是强制项**，不是装饰：经营口径的页面必须写明「未混入 IFRS 16 计量或 Official 过账链路」。这是 §2 原则 7 在版面上的落点。

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
- ✅ 现状（2026-08-18 复测）：三页均已接入 `t()`（调用数 104 / 118 / 71）；store-360 的三个新面板（`InventoryTurnoverPanel` / `CompetitorBenchmarkPanel` / `CategoryCompositionPanel`）此前残留的硬编码 CJK 已全部迁移到 `inventory.*` / `competitor.*` / `category.*` 键组（含 `{count}` / `{qty}` / `{days}` / `{rate}` 插值）。**新页面、新面板不得再引入 CJK 字面量。**

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

**实测于 2026-08-18**（此前记的是 08-13 的数，已全部过期）：

| 偏离 | 上次（08-13） | 现在 | 走向 |
|---|---|---|---|
| 内联 `style={{}}` | 906 | **946** | ⬆️ 变差 |
| `border: 1px solid`（tsx） | 11 | **29** | ⬆️ 变差 |
| JS hover handler | 6 | **8** | ⬆️ 变差 |
| `globals.css` 的 `!important` | 142 | **34** | ⬇️ 大幅改善 |
| 字重 700 / 800 | 13 | **2** | ⬇️ 改善 |
| `.ant-` 覆盖 | 91 | **91** | ➡️ 未动 |
| `next/font` | 0 处 | **已接入** | ✅ 已解决 |
| `tokens.ts` 与 `:root` 漂移 | 已消除 | 对齐测试守护 | ✅ 已解决 |
| 三个零售页无 `t()` | 3 页 | **3 页均已接入** | ✅ 已解决 |
| 无暗色模式 | 全站无 | **已有**（`ThemeToggle` + `darkColors` + `theme-dark.test.ts`，DARK-003 服务端决定主题） | ✅ 已解决 |
| 单环 focus，深色面上不可见 | 全站 | 未复测 | ❓ |

**前三行要认真对待。** 内联样式、字面量边框、JS hover 在这一轮 UI 升级中**不降反增**——§13 的守卫是 diff 级的（只拦新增行、放行存量），所以这些增量本应被拦住。要么是绕过了 `npm run lint`，要么是守卫的匹配规则有漏网。下次动前端前值得先查一次。

**2026-08-19 分支级记账（T5/T6b）**：`docs/retail-bp-workstation-prd` 分支相对 `main` 的累计新增违规已全部落入 `web/scripts/design-debt-baseline.json`（按「文件 × 规则」记允许数量，超出即失败，由 `web/scripts/design-debt-baseline.test.ts` 守护）。记账当日的分支级数量：

| 规则 | 分支新增条数 | 处置 |
|---|---|---|
| §13-2 内联 `style={{}}` | 653 | 记入基线，按文件记账 |
| §13-4 硬编码颜色值 | 90 | 记入基线（T5 新增规则，首扫即发现 90 条） |
| §13-7 硬编码 CJK | 137 | 记入基线，按文件记账 |
| §13-8 `border: 1px solid` | 28 | 记入基线（T5 新增规则，首扫即发现 28 条） |

这是**分支级**债务：合并前每笔都必须消掉或显式续期；清完一笔就把基线里对应文件的数量下调，`design-debt-baseline.test.ts` 会盯住两者不一致。全树级存量（上表 08-18 那批）仍归 UIUX 改善方案，不在此账本里。

整改排期见 [docs/UIUX改善方案.md](docs/UIUX改善方案.md)。

## 15. 执行机制：本文现在有牙齿了

上一版这一节写的是「本文目前没有牙齿，效力仅限于人工 code review」。**那已经不成立**，写在这里是因为它会直接影响你要不要认真对待前面 14 节。

| 执行机制 | 状态 | 位置 |
|---|---|---|
| §13 止血条款自动拦截 | ✅ **CI 强制** | `web/scripts/enforce-design.mjs`（ENF-001），经 `npm run lint` 在 CI 跑。**已实现 9 条中的 6 条**：§13-1 `!important`、§13-2 内联样式、§13-4 硬编码颜色（T5）、§13-6 字重、§13-7 硬编码 CJK、§13-8 `border: 1px solid`（T5）。**未覆盖**：§13-3 JS hover（依赖事件处理器的语义分析，纯正则误报率太高，暂缓）、§13-5 `<Tag color="red">`（AntD 预设色名，与 §13-4 的 token 规则重叠，可在下一轮并入）、§13-9 用 0 填补缺失数据（需要数据流语义，不在文本扫描范畴） |
| §13 存量债务基线 | ✅ **测试守护** | `web/scripts/design-debt-baseline.json`（按「文件 × 规则」记允许数量，带日期）+ `web/scripts/design-debt-baseline.test.ts`；超出基线即 CI 失败（T6b） |
| §1 `tokens.ts` ↔ `:root` 对齐 | ✅ 测试守护 | `app/design-system/tokens-alignment.test.ts` |
| 暗色令牌完整性 | ✅ 测试守护 | `app/design-system/theme-dark.test.ts` |
| §8.1 容器原语 | ✅ 测试守护 | `web/scripts/container-primitives.test.ts` |
| 类名覆盖 / app shell CSS | ✅ 测试守护 | `app/lib/class-coverage.test.ts`、`app/lib/app-shell-css.test.ts` |
| i18n 硬编码文案 | ✅ 审计脚本 | `web/scripts/audit-i18n.mjs` |
| 组件测试 | ✅ 13 个 `.test.tsx` | 用 `renderToStaticMarkup` SSR 断言，不依赖 `@testing-library` |
| ESLint 配置文件 | ⚠️ **存在但只产 warning** | `web/.eslintrc.json` 已存在（`extends: next/core-web-vitals`），`next lint` 确实读取它；缺的是把规则提到 error——当前 21 条 `react-hooks/exhaustive-deps` 等只 warning 不 fail，`next lint` 因此永远绿 |

**`enforce-design.mjs` 是 diff 级拦截器**：基线 CI 用 `origin/main`、本地用 `main`，只检查**新增行**。存量违规（§14 那批）一律放行——全树扫描会立刻全红，变成没人能合的噪音。分支级存量按 §14 的基线文件显式记账，超出允许数量仍然失败。

这个设计的代价写在 §14：**它只在你跑了 `npm run lint` 时生效。** 内联样式从 906 涨到 946 说明这条路径被绕过过，或者匹配规则漏了某些写法。ESLint 配置不是缺失而是太松——`next/core-web-vitals` 的规则只 warning 不 fail，要补的一环是把关键规则提到 error（或给 `next lint` 加 `--max-warnings=0`）。
