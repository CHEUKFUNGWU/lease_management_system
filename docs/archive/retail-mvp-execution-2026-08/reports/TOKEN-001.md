# TOKEN-001 交付报告（第一批）

> ⚠️ **ARCHIVED 2026-08-18 — 不是现行依据**
> 归档理由：零售 MVP（MAX-001~009）转型执行的过程记录，全部工单已 ACCEPTED；未完成的延后治理项见 docs/execution/精简MVP路线与延后治理清单.md
> 现行入口：`docs/AI_文档索引与现行决策.md`

提交人：ZCode（全栈）
提交时间：2026-08-16
分支：`fix/uiux-phase-four`（基线 main @ 33a901a，**未合 main**）
本批 commit：`c942387` TOKEN-001

---

## 1. 计数对照（任务书验收要求）

| 项 | 迁移前 | 迁移后 |
|---|---|---|
| globals.css 非 :root 区硬编码 hex | **2**（mask gradient 的 `#000`） | **0**（改 `var(--fg-primary)`） |
| tsx/ts 代码级硬编码 hex | **7**（BrandIcon 品牌色） | **0**（改 `colors.brand`） |
| theme.ts 注释里的 hex | 7 | 7（**文档注释**，引用 `colors.*`，非代码值） |
| tokens.ts hex（唯一真源） | 33 | **37**（补 chart×2 / brand×11 / surface×2 / overlay×4 相关） |
| :root 与 tokens.ts 镜像一致性 | 未校验 | **25/25 完全一致**（脚本断言 `root hex not in tokens: none`） |
| globals.css rgba 散落（非 shadow/brand） | 0 | 0 |
| 守卫 | — | 无新增违规（3 变更文件） |
| web 测试 | 294 | 294 全绿 |

**设计决策**（codebase-design）：
- **chart 单独成组**（`colors.chart`）：暗色下图表不是简单反转（DARK-001 硬前置）
- **brand 单独成组**（`colors.brand` + inverse 变体 + overlay）：品牌图形色不是 UI 语义槽位，主题切换时品牌标志保持不变，只有 inverse 变体翻转
- **surface.admin / surface.code** 独立语义槽：暗色主题可单独覆盖而不影响正文

## 2. 运行时实测表（成败判据：computed color 逐一相等）

headless Chrome，真实 AppLayout，每个值与迁移前硬编码字面量比对：

| 元素 | 迁移前字面量 | 实测 computed | 相等 |
|---|---|---|---|
| 登录 badge 背景 | `rgba(255,255,255,0.08)` | `rgba(255,255,255,0.08)` | ✓ |
| 登录 badge 边框 | `rgba(255,255,255,0.15)` | `rgba(255,255,255,0.15)` 1px ring | ✓ |
| 登录品牌区正文 | `rgba(255,255,255,0.72)` | `rgba(255,255,255,0.72)` | ✓ |
| BrandIcon 默认 frame | `#111827` | `rgb(17, 24, 39)` | ✓ |
| BrandIcon inverse frame | `#FFFFFF` | `rgb(255, 255, 255)` | ✓ |
| body 背景 | `#FFFFFF`（--bg-page） | `rgb(255, 255, 255)` | ✓ |
| body 正文 | `#000000`（--fg-primary） | `rgb(0, 0, 0)` | ✓ |
| ant-tag 文字 | `#000000`（colorPrimary） | `rgb(0, 0, 0)` | ✓ |
| admin surface（admin/login 引用） | `var(--admin-surface)` = `#001529` | 内联引用变量 ✓ | ✓ |
| code surface（ai-chat 引用） | `var(--code-surface)` = `#1E1E1E` | 内联引用变量 ✓ | ✓ |

（10 项全部相等；token 值就是旧字面量，纯重构零视觉变化是构造性保证。）

## 3. 结构变更

| 文件 | 变更 |
|---|---|
| `web/app/design-system/tokens.ts` | +chart / +brand(+inverse+overlay) / +surface 组，37 hex 唯一真源 |
| `web/app/components/BrandIcon.tsx` | 7 个颜色字面量 → `colors.brand` |
| `web/app/globals.css` | mask `#000` → `var(--fg-primary)`；:root +`--brand-overlay-*` 镜像；登录区 rgba → 变量 |

## 4. 我无法确认的部分

1. BrandIcon 的品牌色观感（值未变，但值得过目确认图形没因 token 化而意外变化）。
2. 登录品牌面板的 overlay 透明度观感（值未变）。
3. `--chart-blue/--chart-purple` 目前只在 :root，图表组件若直接硬编码了色值需 DARK-001 时核查（本票未发现图表侧硬编码）。

**第一批完成，待 Planner 复验。** 通过后开第二批（STY-010 + STATE-004 + FETCH-003）。
