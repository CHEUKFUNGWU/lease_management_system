# HOME-004 交付报告 —— 首页 chat 优先重构

提交人：ZCode（全栈）
提交时间：2026-08-15
分支：`feat/home-004-chat-first`（已推送，**未合 main**）
基线：`main @ f348c42`
commit：`46a6451`（简报带）→ `4dd69b8`（chat 主体与版面）

> **本票必须先经用户视觉确认才能合并**（任务书 §6）。命令级验证全绿是必要条件，不是充分条件。

---

## 0. 先说明两个执行中的裁决

1. **复用路径（用户已裁决）**：`/ai-chat` 的 `MessageContent` / `TypewriterMessage` 是页面私有函数，无法 import；抽取共享 = 改动超出首页 + 需同步去内联化（`enforce-design` 拦截）。用户选择**首页内升级、不抽代码、`/ai-chat` 零改动**。首页气泡用 home 局部 CSS 类复刻 `/ai-chat` 的视觉语言（user 右 / assistant 左、头像、16px 圆角带一个直角），渲染走共享组件（ToolChip / ConfidenceBadge / SourceCitation / ThinkingTrace / DataTrustBar）。**取舍**：首页无打字机效果、无代码块解析（经营问答场景基本不含代码块；需要时去 `/ai-chat`）。后续可另开票真正共享化。
2. **执行环境**：共享工作树与另一会话（landing 页）发生分支竞争，本批改在独立 worktree `/Users/cheukfungwu/ifrs16_home004` 完成，主工作树未受影响。

---

## 1. 结构变更清单

### 1.1 组件落位（重构后）

| 位置 | 组件 | 职责 |
|---|---|---|
| 中栏顶部 | `<BriefBand>`（**新增**，`home/BriefBand.tsx`） | 紧凑可展开简报带。收起 = 标题 + 关注门店数 + DataTrustBar 摘要行；展开 = KPI 三卡 + 结构化关注门店卡 + 工具行/置信度 + ThinkingTrace + 引用 |
| 中栏主体 | `<BriefColumn>`（重写渲染层，数据层不动） | 真实消息流：气泡、空会话建议提问、自动滚动、常驻底部输入框、pending 气泡 |
| 降级态 | `<BriefView>`（**收缩**为降级态专用） | 只渲染 no_data / not_decision_ready / needs_input / scope_denied / error；ready 与 loading 由 BriefBand 承担，原 ready 分支（§2.2 禁止的 answer 倒出）已删除 |
| 右栏 | `<RightColumn>` 不变（L7 一处改动，见下） | 待确认建议 → 需要你处理 → 总负债/本月费用 → 月结就绪度 → 关键日期，一项不少 |
| 角色分支 | `page.tsx` 零改动 | `canViewHomeBrief` 分支保留：editor/reviewer/approver 仍渲染 `WorkQueueFocus` |

### 1.2 `answer` 的去向（§2.2）

- 简报带与每条 assistant 消息**都不再渲染 `answer` 原文**；它作为文本进 `<ThinkingTrace>` 折叠区（简报带里与 plan 合并；消息气泡里单独成段）
- 后端 `answer` 字段、`/ai-chat` 对它的渲染：**未动**

### 1.3 新增 / 删除的 CSS 类

**新增**（全部 token 化、无内联）：

| 类 | 用途 |
|---|---|
| `home-brief-band` / `-header` / `-title` / `-count` / `-toggle` / `-arrow` / `-skeleton` / `-body` | 简报带容器与收起行 |
| `home-brief-band-kpis` / `home-band-kpi` / `-label` / `-value` / `-change` | KPI 三卡（20px/600/tabular-nums，值行 ellipsis） |
| `home-band-attention` / `-title` / `-card` / `-store` / `-rank` / `-storecode` / `-storename` / `-citation` / `-severity` / `-signals` / `-signal` | 关注门店卡三槽位（grid-template-areas） |
| `home-band-meta` | 展开态尾部元信息组 |
| `home-chat-column` / `-body` / `-messages` / `-starters` / `-starters-label` / `-starter-chips` / `-starter-chip` / `-composer` | chat 列结构（68ch 限宽在 `-messages` 与 `-starters`） |
| `home-msg` / `.is-user` / `.is-assistant` / `-avatar` / `-bubble` / `-error` / `-pending` | 消息气泡（复刻 /ai-chat 视觉语言） |
| `home-proposals-empty-line` | L7 收缩空态 |

**删除**（孤儿清理，随 Commit 2）：`home-brief-column`、`home-brief-body`、`home-brief-spin-block`、`home-brief`、`home-brief-answer`、`home-brief-attention*` 全族、`home-brief-title`、`home-brief-composer`、`home-followups`、`home-followup*` 全族、`home-proposals-empty`。

**保留**：`home-brief-state`（降级态 padding）、`home-brief-sources/-source/-source-index`（BriefBand 展开态引用仍用）。

**Token/断点变更**：`.home-grid` 从 `minmax(0,1fr) 340px` → `minmax(640px,1fr) 300px`；新增 `@media (max-width:1439px)` 过渡档 `minmax(0,1fr) 300px`；`<768` 单栏 + Drawer 不变。

### 1.4 DataTrustBar 变更（additive）

新增可选受控 props `expanded` / `onToggle`；不传则维持原内部状态。既有调用方（pulse / store-360 / scenario-workbench / RetailAIDrawer / BriefView）全部未传，行为不变。首页用受控模式让「带的展开」与「信任条明细」由同一个 toggle 驱动，永不出现两者状态错位。

### 1.5 断点行为

| 宽度 | 行为 |
|---|---|
| ≥1440 | `nav(240) | 中栏 ≥640 | 右栏 300`（内容区 max 1440 内） |
| 1024–1439 | 中栏 `minmax(0,1fr)`（可缩），右栏仍 300 可用 |
| 768–1023 | 同上（内容 padding 24） |
| <768 | 单栏 + 待办 Drawer（原行为，未动） |

### 1.6 每种角色的分支

| 角色 | 中栏 | 右栏 |
|---|---|---|
| admin / readonly / auditor | BriefBand + chat 消息流 | RightColumn（全量） |
| editor / reviewer / approver | `WorkQueueFocus`（不变） | RightColumn（全量） |

三态（no_data / not_decision_ready）+ scope_denied（不软化）+ needs_input + error：全部保留，测试覆盖见 §3。

---

## 2. §4 版面规矩逐条自查表

| # | 规矩 | 落点 | 机械验证 |
|---|---|---|---|
| L1 | 正文行长 68ch | `.home-chat-messages { max-width: 68ch }`、`.home-chat-starters { max-width: 68ch }` | `chatLayout.test.ts` L1 |
| L2 | 4px 基数、同层级一致 | 列节奏 16px（column/band↔body↔messages）、组内 12px、带内 8px——同层级同值 | L2 断言四组 gap |
| L3 | 四级层级、字重 ≤600 | 带标题 13/600/-0.01em；KPI 值 20/600；信号 12/400；元数据 11–12。全 CSS 无 700+ | L3 断言 + 全 css 无 `font-weight:7/8/900` |
| L4 | 置信度与 chip 同排 | BriefBand 与 ChatMessage 的 `ai-tool-row` 内 ToolChip 与 ConfidenceBadge 同容器 | L4 源码断言（两处渲染器） |
| L5 | 信任条分组 | DataTrustBar 本体（classification/reason/detail 分组 span，非纯文本串）——本次未改其渲染 | L5 组件源码断言 |
| L6 | tabular-nums | `home-band-kpi-value/-change`、`home-band-attention-signals`、`home-brief-band-count` | L6 断言 |
| L7 | 右栏空态收缩一行 | `home-proposals-empty-line`（12px/muted 一行文案），删掉 Empty 插画块 | L7 + RightColumn.test（`not.toContain("ant-empty")`） |
| L8 | 1440 中栏 ≥640、1280 右栏可用 | `.home-grid { grid-template-columns: minmax(640px,1fr) 300px }` + 1439 断点降为 `minmax(0,1fr) 300px` | L8 断言 |

---

## 3. 验证（命令级）

```
web: npm run type-check       通过
web: npm test                 Test Files 33 passed (33)，Tests 213 passed (213)
web: npm run build            ✓ Compiled successfully（28 静态页）
web: node enforce-design.mjs  11 个变更文件，无新增违规
core-service                  本票未涉及后端（answer 字段与 /ai-chat 均不动，无需跑）
```

新增测试：`BriefBand.test.tsx`（7 用例：收起行构成、answer 不再作正文、降级态与 scope_denied、组件复用、单 toggle、L4、§2.3 三槽位）+ `chatLayout.test.ts`（10 用例：L1–L8 + §3 输入框/建议提问/滚动 + §5 角色接线）。更新：`BriefView.test.tsx`（ready 覆盖移交 BriefBand）、`RightColumn.test.tsx`（L7 新空态）。既有 61 → 全套 213 全绿，无跳过。

**角色矩阵与三态**由 `home-flow.test.ts`（角色分支）+ `BriefView.test.tsx`/`BriefBand.test.tsx`（五态）+ `logic.test.ts`（classifyHomeBrief 纯逻辑）继续覆盖，全部通过。

---

## 4. 我无法确认的部分（需要用户看）

命令级与源码契约都过了，但**这张票存在的理由是观感**，以下只能由用户在 1440×900 与 390×844 两档视口实际打开确认：

1. **整体气质**：简报带收起时是否真有「一条上下文带」的轻量感，还是仍显重（收起行 = 标题 + 关注数 + 信任条摘要，信任条本身有边框底色）
2. **简报带展开态**：KPI 三卡在 640px 中栏里的密度；关注门店卡 severity 列右对齐后的平衡感
3. **消息流**：气泡与 `/ai-chat` 的视觉一致性（两侧都有头像、同圆角语言，但首页底色/留白不同）；68ch 限宽在 640px 栏内实际几乎满栏，观感上是否还需要更窄
4. **L7 收缩空态**：一行 muted 文案是否过于寡淡（原来是大块 Empty 插画）
5. **1024–1439 过渡档**：中栏可缩后与右栏 300px 的比例在 1280 实机上的舒适度
6. **繁中/英文**下的收起行长度（英文 "Attention stores: 3" 较长，信任条同行 wrap 后的换行形态）

用户点头后由 Reviewer 整理检查表走合并流程；在此之前不合 main。

---

## 5. 边界遵守

- `/ai-chat` 零改动（git diff 确认未触碰 `web/app/ai-chat/**`）
- 后端零改动；`answer` 字段语义未变
- 右栏六项内容、ApprovalCard 零写入链路、角色分支、`<768` 退化，全部未动
- 无新增内联样式、`!important`、>600 字重、硬编码中文（enforce-design 通过）；新增文案 6 条全部三语
