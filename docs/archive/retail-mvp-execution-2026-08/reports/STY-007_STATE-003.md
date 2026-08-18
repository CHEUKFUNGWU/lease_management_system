# STY-007 / STY-008 / STY-009 / STATE-003 / DATA-001 交付报告

> ⚠️ **ARCHIVED 2026-08-18 — 不是现行依据**
> 归档理由：零售 MVP（MAX-001~009）转型执行的过程记录，全部工单已 ACCEPTED；未完成的延后治理项见 docs/execution/精简MVP路线与延后治理清单.md
> 现行入口：`docs/AI_文档索引与现行决策.md`

提交人：ZCode（全栈）
提交时间：2026-08-16
分支：`fix/uiux-phase-one`（基线 main @ 3af1572，**未合 main**）
commit 列表（每票独立，STY-007 按组件族拆 6 个）：
`38df14b` STY-007(table) · `446725c` STY-007(button) · `bf8b960` STY-007(card) ·
`1a6e236` STY-007(modal) · `a4a23ad` STY-007(menu) · `44b93c9` STY-007(tag) ·
`0d56c2f` STY-007(rest) · `98df26a` STY-008 · `1d2f7d1` STATE-003 ·
`cdf51e8` DATA-001 · `e23e8cc` + `8a66176` STY-009

---

## 1. 计数总览（本批唯一能证明进度的东西）

| 项 | 目标 | 基线 | 现在 |
|---|---|---|---|
| `!important` | ≤ 20 | **140** | **4**（全部在 prefers-reduced-motion 块，a11y 必需） |
| `1px solid` | 0 | **25** | **1**（FIX-009 已记录例外，见 §3） |
| 全仓内联样式 | — | **795** | **572**（目标 ~570 ✓） |
| 守卫违规 | 0 | 0 | 0（16 变更文件） |
| web 测试 | 全绿 | 288 | **294**（+6 StateBlock） |
| build | 通过 | 通过 | 通过 |

---

## 2. STY-007 AntD 覆盖改走 token（P0，本批主体）

**方法**：逐个组件族迁移，一族一个 commit。流程：找出该族 `!important` 规则 → 判断它覆盖的 token → 值搬进 `theme.ts` → 删 CSS；token 表达不了的才保留 CSS，去 `!important` 用 `body` 前缀提高特异性（antd 运行时规则后插入且 `:where()` 不贡献特异性，同特异性下 antd 赢——FIX-010 教训）。

**114 处 ant-\* 全部清零**（140 → 30 → 4）。关键决策：

| 族 | 处数 | 进 token | 保留 CSS（无 token） |
|---|---|---|---|
| Table | 14 | headerBg/headerColor/headerSplitColor/cellPaddingBlock/Inline（**补 cellPaddingBlockSM/InlineSM=12/16**：size=small 表格原被 !important 强制 12/16，antd small 默认 4px 会改值）；rowHover/SelectedBg、borderColor、cellFontSize 已有 | th 字重/字号/tracking、td 颜色（body 前缀） |
| Button | 8 | **fontWeight 500**（token 原是 600——CSS 覆盖的现状值）；**colorPrimaryHover=fg.secondary**（primary hover 填充）；**defaultHoverBorderColor=fg.primary**（原 token 是 border.strong）；primary 填充由 colorPrimary 派生 | 按压动画（body 前缀） |
| Card | 8 | border/radius/shadow 已有；**bodyPadding=20 + bodyPaddingSM=20**（原 CSS 强制 20，antd 默认 24/12） | hover 交互（token 无 card-hover）；head 的 16px 20px（antd headerPadding 是单值）；border-bottom 分割线 |
| Modal | 8 | radius/paddings/shadow/背景 全部已有且值一致（CSS 是冗余覆盖） | header/footer 分割线、overflow 裁剪（body .ant-modal 前缀——**先写了 body .ant-modal-header 被 antd 的 .ant-modal .ant-modal-header 压过，实测发现后补 wrapper 类**） |
| Menu | 9 | itemBorderRadius 6 已有；**subMenuItemBorderRadius=6**（补） | margin 2px 8px、selected 字重、border-right 移除、transition |
| Tag | 7 | defaultBg/defaultColor 已有 | radius 4/字重 500/字号 12/padding/line-height 18/border-width（antd 从全局 fontSize 派生 tag 尺寸，无 token） |
| 其余 20 族 | 56 | Statistic.contentFontSize **28**（token 原是 24）；**colorTextDescription=fg.tertiary**（antd Statistic title 用它）；Badge.textFontSize 10/textFontWeight 600/indicatorHeight 16；Switch/Radio/Checkbox/Slider/Progress/Empty/Descriptions/Popover/Tooltip/Notification radius 或填充 已有 | dropdown/select radius+shadow+padding、tooltip 尺寸、ink-bar 高度、statistic/result 字重与 tracking、indeterminate 填充、slider handle 环、segmented/collapse 字重 |

**踩坑记录（FIX-034 教训的活例）**：
1. **Statistic.contentFontWeight 是死 token**——查 antd 5.29 源码确认它从不被消费，weight 600 只能留 CSS。最初按「token 已有」删了 CSS，实测 weight 掉到 400，恢复。
2. **Modal header 分割线**：`body .ant-modal-header`（0,1,1）输给 antd 的 `.ant-modal .ant-modal-header`（0,2,0），实测 border 消失，补成 `body .ant-modal .ant-modal-header` 才赢。
3. **Table size=small**：删除 padding !important 后 small 表格回落到 antd 4px inline——补 SM token 钉回 12/16 才不改变渲染。
4. **FIX-009 的 .home-readiness-card**：hairline ring 在纯白背景不可见（该 commit 的注释就是决策记录），转 ring 会真消失——保留真实 border（已是 token 色），是 shadow-as-border 的已记录例外。

**实测**（headless Chrome，真实 AppLayout，GUARD-001 规则）：
- Table th/td padding 12px 16px、header 600/12px/#595959/#F0F0F0 ✓
- Button default 500/#fff/#D9D9D9；primary hover bg rgb(38,38,38)=#262626（真实指针 hover）✓
- Card border #D9D9D9 1px、radius 10px、body 20px ✓
- Modal radius 12px、header 20px 24px、分割线 1px #F0F0F0 ✓
- Menu radius 6px、margin 2px 8px、selected #F0F0F0/600 ✓
- Tag 4px/500/12px/2px 8px/18px/1px ✓
- badge-count 10px/600/16px（从 antd 生成规则文本确认）✓
- Statistic 28px/600、title 12px/500/#595959 ✓

**反向验证**：把 Statistic 改回 24 或删掉 CSS weight——实测 weight 变 400 或 size 变 24，测试/实测即红（本批靠运行时实测而非静态测试锁值；`replacement-values.test.ts` 已覆盖 STY-006 的三处）。

---

## 3. STY-008 剩余 !important 与 1px solid（P1）

**30 → 4**：
- focus 环（btn/select `:focus-visible`）：`body` 前缀，flag 去掉
- 响应式 layout 尺寸（56px header、12/16/20/24px paddings）：无 token 等价物（breakpoint 特例），`body` 前缀
- ai-chat 移动端块（shell/sidebar/input/header/model）：**全是自身类**，无竞争——flag 纯防御性，去掉
- session-more-btn hover 链：hover/focus 规则按类数天然压过基础规则
- pulse/store-360 移动端 card-body 16px：`body` 前缀
- **保留 4 处**：`prefers-reduced-motion` 块的 animation/transition-duration——它必须压过 antd/framer-motion 的内联动画样式，!important 是唯一手段（a11y 正确用法）

**1px solid 25 → 1**：23 处 border 转 ring 阴影（`box-shadow: 0 0 0 1px`），单边线转单边 ring（`0 1px 0 0`）；`contract-mobile-card` 的 elevation shadow 与 ring 分层叠加；`ai-confidence-badge` 的状态色边框（`.is-low` 变体）转 box-shadow 色变体。唯一幸存者 `.home-readiness-card`（FIX-009 例外，见 §2 踩坑 4）。

**实测**：header 60px（token）、data-trust-bar/card/badge ring、sider 右边线 ring 全部按预期 token 渲染。

---

## 4. STATE-003 StateBlock（P1）

**新建 `web/app/components/StateBlock.tsx`**（+ 6 条 SSR 测试）：入参直接吃 `DataState` 的 kind，四态固定呈现：
- `empty`：安静一行 + 为什么空（FIX-033 密度）
- `actionable`：说明 + 下一步动作按钮（onAction）
- `failed`：错误 + 原因 + 重试（onRetry，可选）
- `scope_denied`：权限拒绝，**与 failed 可区分**（无重试按钮、原因保留），永不软化
- `ready`：渲染空——页面拥有数据视图

**落点**（本票范围）：
| 页面 | 迁移 |
|---|---|
| store-360 | 三个手写 Alert（actionable/failed/scope_denied）→ 单个 StateBlock 吃 diagState |
| operating-pulse | 合并的 error 字符串拆回 DataState：参数校验错（缺 dataset version/无效 window）→ actionable；请求失败 → failed；拒绝 → scope_denied |
| scenario-workbench | options 加载失败 → StateBlock（failed + retry） |
| 首页右栏 BriefView | error/scope_denied/no_data/needs_input 映射到 failed/scope_denied/empty/actionable；not_decision_ready 保留 DataTrustBar 前缀 |
| 合同详情 | **无可迁呈现**：无 `<Empty>` / `<Alert type="error">`；其 actionable 路径已在 STATE-001 落地（notify + payments 页签跳转）——报告如实说明 |

**i18n**：新增 `state.empty/actionable/failed/scope_denied_label` 四 key（三语）；`scope_denied_title` 先加后用不到即删。

**验收**：`npx vitest run components/StateBlock.test.tsx` 6 条过；已迁页面不再直接使用 `<Empty>`/`<Alert type="error">`（store-360/pulse/scenario/BriefView 的对应渲染已替换）；运行时 store-360 empty 态在真实 AppLayout 渲染 ✓。

---

## 5. DATA-001 历史简报会话重新打标（P1，改动生产数据）

**dry-run**（先 SELECT，判据从 i18n 生成不硬抄）：
```
bucket    | n
----------+----
zh-CN     | 47
en        |  1
（合计 48；用户提问「哪些门店需要关注？」不在其中）
```

**判据修正**（重要发现）：任务书说「标题等于 `home.brief_prompt` 三语之一」——**不准确**。`summarizeTitle` 只取前 20 个 rune + `"..."`（runtime.go:414），所以实际标题是 prompt 的**前缀**（「请读取当前经营脉搏并生成今日经营简报：总…」47 条、「Read the current ope...」1 条）。判据按前缀匹配，前缀从 i18n 字典生成。用户提问 9 个字不命中任何前缀 ✓。

**执行**：`db/migrations/043_retag_home_brief_sessions.sql`（幂等：备份表 `data_001_retag_backup` ON CONFLICT DO NOTHING + UPDATE）。已应用到运行库：48 条 UPDATE、48 条备份。

**复验**：
| 项 | 值 |
|---|---|
| 默认会话列表（用户可见） | **1 条**（「哪些门店需要关注？」user）✓ |
| system 会话总数 | 95（48 打标 + CHAT-001 后 7 + 更早测试运行），全部 page=home |
| ai_chat_runs | 96（不变）✓ |
| ai_chat_messages | 192（不变）✓ |
| 回滚 | `data_001_retag_backup` 48 条，脚本注释含恢复 SQL |

---

## 6. STY-009 内联样式存量（P2）

**五文件 226 → 3**（224 静态转 95 个生成类 `sty-<hash8>`，3 真动态保留）：
| 文件 | 基线 | 现在 |
|---|---|---|
| reports/page.tsx | 62 | 0 |
| monthly-closing/page.tsx | 51 | 0 |
| ContractWorkspaceDialogs.tsx | 41 | 2（三元条件，真动态） |
| contracts/page.tsx | 38 | 0 |
| BudgetVariancePanel.tsx | 34 | 1（含变量，真动态） |

**工具**：`web/scripts/sty009_convert.py`（保留供后续批次）——提取平衡 style 块、判定静态（无变量/三元/模板引用）、按规范化值哈希成类（相同值共享一个类）、规则追加 globals.css、元素已有 className 时合并。

**拦坑遵守**：动态值不转类（3 处保留）；值不同分开写（哈希天然保证）。**转换器踩坑**：JSX 字符串值（`"100%"`、`"var(--x)"`）原样进 CSS 会生成无效规则（`border: "1px solid..."` 带引号）——修复为去引号并转 ring（8a66176 修正 commit）。

**计数**：795 → **572**（目标 ~570 ✓）。实测四页 sty-* 类生效（reports 10 / monthly 6 / contracts 40 / portfolio 0）且解析出真实 computed 值，零页面错误。

---

## 7. 验证（命令级实际输出）

```
web: npx vitest run                 → 46 files / 294 tests passed（基线 288 + 6 StateBlock）
web: npx tsc --noEmit               → 干净
web: npx next build                 → ✓ Compiled successfully，28/28 静态页
web: node scripts/enforce-design.mjs → 16 个变更文件，无新增违规
core-service: go test ./internal/repository/ ./internal/handlers/ → ok（DATA-001 无 Go 改动）
```

运行时实测（headless Chrome / 真实容器 API）：见 §2 各表、§5 复验表、§6 计数。

---

## 8. 结构变更清单

| 文件 | 变更 | 票 |
|---|---|---|
| `web/app/design-system/theme.ts` | Button/Card/Menu/Statistic/Badge token 值对齐 + colorPrimaryHover/colorTextDescription/bodyPadding(SM)/SM tokens | STY-007 |
| `web/app/globals.css` | 110 处 !important 清零（token 化 + body 前缀）；23 处 border→ring；StateBlock 类 | STY-007/008/STATE-003 |
| `web/app/components/StateBlock.tsx` + `.test.tsx` | 四态统一呈现 + 6 条测试 | STATE-003 |
| `web/app/lib/i18n.ts` | state.* 四 key（三语） | STATE-003 |
| `web/app/{store-360,operating-pulse,scenario-workbench}/page.tsx` | 状态渲染 → StateBlock | STATE-003 |
| `web/app/home/BriefView.tsx` | 降级态映射到 DataState + StateBlock | STATE-003 |
| `db/migrations/043_retag_home_brief_sessions.sql` | 幂等打标 + 回滚备份表 | DATA-001 |
| 五文件（reports/monthly-closing/Dialogs/contracts/BudgetVariance） | 224 静态内联 → 95 生成类 | STY-009 |
| `web/scripts/sty009_convert.py` | 转换工具（保留复跑） | STY-009 |

---

## 9. 我无法确认的部分（需要用户看）

1. **STY-007 视觉等价**：所有 token 值对齐「CSS 覆盖的现状值」（非原 token 意图），理论上零视觉变化；但 Table small padding、Button hover 边框、Statistic 28px 等是「从 CSS 现状搬进 token」——值得过目。primary 按钮边框从强制 #000 变 antd 透明（背景覆盖，视觉等同）。
2. **STY-008 ring 边框观感**：border → box-shadow ring 在圆角元素上视觉一致，但单边 ring 在无 radius 元素上应与原 border 相同；1px hairline 在纯白背景的可见性（FIX-009 例外以外的 23 处）值得抽查。
3. **STY-009 生成的 95 个类**：值是原内联值的逐字搬运（哈希去重），理论零变化；但自动转换的类名（sty-xxxx）不可读，后续可考虑语义化命名（另开票）。
4. **STATE-003 观感**：actionable 从 warning Alert 变 StateBlock 的统一呈现；pulse 的参数校验错误从 error 红变 warning 黄（语义更正确但颜色变了）。
5. **DATA-001**：侧栏现在只剩 1 条真实会话——历史 95 条简报全部隐藏（可经 include_system=true 查）。用户应确认侧栏观感符合预期。

**运行库已变更**（043 已应用、48 条打标）；前端 dev server 3001 已重启（build 后）。视觉影响由 Reviewer 推导交用户确认；**用户点头前不合并**。
