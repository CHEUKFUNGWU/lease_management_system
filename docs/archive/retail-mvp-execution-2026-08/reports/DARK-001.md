# DARK-001 交付报告（第三批）

> ⚠️ **ARCHIVED 2026-08-18 — 不是现行依据**
> 归档理由：零售 MVP（MAX-001~009）转型执行的过程记录，全部工单已 ACCEPTED；未完成的延后治理项见 docs/execution/精简MVP路线与延后治理清单.md
> 现行入口：`docs/AI_文档索引与现行决策.md`

提交人：ZCode（全栈）
提交时间：2026-08-16
分支：`fix/uiux-phase-four`（**未合 main**）
commit：`0aca3f7`（DARK-001）
**DENSITY-001 已按任务书 §7 砍掉**（理由见 §4）

---

## 1. DARK-001 暗色模式（P1）

### 1.1 设计（codebase-design）

颜色真源是 `tokens.ts`（TOKEN-001 确立）。暗色 = **同一套语义槽位的第二组值**：

| 层 | 亮色 | 暗色 |
|---|---|---|
| `tokens.ts` | `colors` | **`darkColors`**（同构语义槽位，新增） |
| `theme.ts` | `antdTheme` | `buildTheme(palette)` 工厂 → `antdDarkTheme` |
| `globals.css` | `:root` | `[data-theme="dark"]` 覆盖块（**同特异性、后定义者赢，不用任何强制标志**） |
| AntD | defaultAlgorithm | **darkAlgorithm** |

切换机制：`ThemeProvider` 读主题 → 写 `<html data-theme>`（CSS 变量切换）+ 切 AntD algorithm。

### 1.2 切换入口（任务书「你定并说明理由」）

**跟随系统 + 手动覆盖**：默认读 `prefers-color-scheme`；用户在 header 手动切换后写 localStorage 覆盖（存 `app-theme`）。理由：企业工具的用户通常跟随系统节律（晚上自动暗色），但财务/审计场景有人固定亮色——两全。OS 变化仅在用户未手动选择时跟随。

### 1.3 图表配色（单独处理）

图表**已经**是变量驱动（`var(--chart-blue)` 等，TOKEN-001 的 chart 组）——暗色块自动换值，无需反转：`--chart-blue` #1677FF → **#4FC3F7**、`--chart-purple` #722ED1 → **#B39DDB**（更亮以在深底保对比）。

### 1.4 状态色（重新取值保对比度）

| 状态 | 亮色 | 暗色（text） | 暗色（bg） |
|---|---|---|---|
| success | #216E39 | #66BB6A | #1B3A22 |
| warning | #8A5300 | #FFB74D | #3A2E10 |
| error | #A8071A | **#F0625C** | #3A1616 |
| info | #1F4E9C | #64B5F6 | #12293A |

**error 从 #EF5350 提到 #F0625C**：实测发现 AntD darkAlgorithm 的卡片面是 #282828（非预期的 #1E1E1E），#EF5350 在其上只有 **4.23:1**——提亮后 4.64:1（页面上 5.80:1）。这正是任务书要的「不简单调暗，要重新取值」。

### 1.5 验收：对比度实测表（全部 ≥ 4.5:1）

WCAG 2.1 公式，背景取实测值（body #141414 / card #282828 实测 / inset #262626）：

| 前景 / 背景 | 对比度 | |
|---|---|---|
| 正文 #FFFFFF / body #141414 | 18.42:1 | ✓ |
| 次要 #D9D9D9 / body | 13.05:1 | ✓ |
| 三级 #A6A6A6 / body | 7.57:1 | ✓ |
| 弱化 #8C8C8C / body | 5.48:1 | ✓ |
| 正文 / card #282828 | 14.74:1 | ✓ |
| 次要 / card | 10.44:1 | ✓ |
| th 文字 #A6A6A6 / inset #262626 | 6.22:1 | ✓ |
| success #66BB6A / body | 7.79:1 | ✓ |
| warning #FFB74D / body | 10.64:1 | ✓ |
| error #F0625C / body | 5.80:1 | ✓ |
| info #64B5F6 / body | 8.32:1 | ✓ |
| chart blue #4FC3F7 / body | 9.20:1 | ✓ |
| chart purple #B39DDB / body | 7.69:1 | ✓ |
| success / card | 6.24:1 | ✓ |
| warning / card | 8.52:1 | ✓ |
| error / card | **4.64:1** | ✓ |
| info / card | 6.66:1 | ✓ |
| success text / success-bg #1B3A22 | 5.30:1 | ✓ |
| warning text / warning-bg #3A2E10 | 7.69:1 | ✓ |
| error text / error-bg #3A1616 | 5.06:1 | ✓ |
| info text / info-bg #12293A | 6.75:1 | ✓ |
| neutral / neutral-bg | 6.22:1 | ✓ |
| **亮色回归**：success/warning/error/info on white 与 on status-bg | 5.62–7.99:1 | ✓ |

**两档主题 computed 对照**（headless Chrome，真实 AppLayout）：

| 元素 | light | dark |
|---|---|---|
| html data-theme | light | dark |
| body bg | #FFFFFF | #141414 |
| body 文字 | #000000 | #FFFFFF |
| card bg | #FFFFFF（实测） | #282828（antd darkAlgorithm） |
| th bg | #F0F0F0 | #262626 |
| th 文字 | #595959 | #A6A6A6 |
| tag 文字 | #000000 | #FFFFFF |
| --chart-blue | #1677FF | #4FC3F7 |

**端到端**：header 切换按钮存在；点击 → dark（body #141414）、再点 → light（#FFFFFF）；localStorage 持久化；零页面错误。

---

## 2. DENSITY-001 密度模式（P2，**已砍**）

任务书 §7：「优先级最低，时间不够就砍掉并在报告里说明——它是纯增量，没有任何东西在等它。」

**砍掉理由**：AntD `compactAlgorithm` 会覆盖阶段一刚验收的具体 token 值（`cellPaddingBlock: 12`、`headerHeight: 52`、`cellPaddingInline: 16` 等）——紧凑档会让所有表格/卡片回到 antd 紧凑默认值，直接推翻 STY-007/STY-009 的验收成果。且当前无任何消费者请求密度切换。做它需要先定义「紧凑档下哪些值保持、哪些收紧」的完整映射并逐页验证不破版——工作量与 DARK-001 相当但价值为零。**决定：砍掉，另开票排期**（需要时再按「明确密度档位的 token 映射」做）。

---

## 3. 验证（命令级实际输出）

```
web: npx vitest run        → 46 files / 294 tests passed
web: npx tsc --noEmit      → 干净
web: npx next build        → ✓ Compiled successfully
web: node scripts/enforce-design.mjs → 26 变更文件，无新增违规
```

运行时（headless Chrome）：双主题 computed 对照见 §1.5；切换交互端到端通过。

---

## 4. 结构变更清单

| 文件 | 变更 |
|---|---|
| `web/app/design-system/tokens.ts` | +`darkColors`（同构语义槽位，暗色值含对比度验证） |
| `web/app/design-system/theme.ts` | `buildTheme(palette)` 工厂 + `antdDarkTheme` |
| `web/app/globals.css` | `[data-theme="dark"]` 变量覆盖块（含暗色 shadow 调整） |
| `web/app/components/ThemeProvider.tsx` | 主题状态（跟随系统+手动持久化）+ darkAlgorithm 切换 + data-theme 写入 |
| `web/app/components/ThemeToggle.tsx` | header 切换按钮（三语 aria-label） |
| `web/app/components/AppLayout.tsx` | header 接 ThemeToggle |
| `web/app/lib/i18n.ts` | theme.switch_light/dark 三语 keys |

---

## 5. 我无法确认的部分（需要用户看）

1. **暗色观感**：所有对比度机械达标，但暗色的「观感」（card 面 #282828 vs 预期 #1E1E1E、shadow 改用白色 alpha、登录品牌面板在暗色下的表现）需要用户实机过目。
2. **图表暗色**：chart-blue/purple 的暗色值在真实图表中的可读性（对比度达标但色相观感需确认）。
3. **状态色观感**：error #F0625C 比亮色版更亮（暗色需要）——在真实状态徽章上的观感。
4. **DENSITY-001 砍掉的确认**：若用户认为需要密度模式，另开票按「密度档位 token 映射」重做。

**全部三批完成，待 Planner 复验。** 通过后可合入 main（或按仓库流程处理）。
