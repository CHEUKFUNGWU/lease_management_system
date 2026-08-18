# STATE-001 / CONTRACT-001 / PRIM-001 / FETCH-001 / STY-005 / DOC-001 交付报告

> ⚠️ **ARCHIVED 2026-08-18 — 不是现行依据**
> 归档理由：零售 MVP（MAX-001~009）转型执行的过程记录，全部工单已 ACCEPTED；未完成的延后治理项见 docs/execution/精简MVP路线与延后治理清单.md
> 现行入口：`docs/AI_文档索引与现行决策.md`

提交人：ZCode（全栈）
提交时间：2026-08-16
分支：`fix/workstation-shell-005-014`（叠加在 FIX-033 之上，**未合 main**）
基线：`d5903cf`（本批指令 commit）
6 个独立 commit（STATE-001 / CONTRACT-001 / DOC-001 / FETCH-001 / STY-005 / PRIM-001 各一）

---

## 1. STATE-001 数据状态的判定与表达（P0）

### 1.1 判定层

`web/app/lib/dataState.ts` 的 `classifyDataState` 纯函数（三分法 + 独立第四态）：

| 状态 | 判定 | 呈现 |
|---|---|---|
| `empty` | 200 且空；404（后端明确说没有）；422 data_unavailable（默认，可经 actionFor 升级） | 空态 + 原因 |
| `actionable` | 请求成功或失败但用户能自己解决（actionFor 升级：折现率缺失、选错分类、无付款计划） | 空态 + 下一步动作（带入口） |
| `failed` | 网络失败 / 5xx / timeout / 其余 4xx | 错误态 + 原因 + 重试 |
| `scope_denied` | 独立第四态，永不并入三分法 | 权限拒绝 + 原因（AGENTS.md 红线） |

「后端说没有」（404/422）与「请求没打通」（网络/5xx）**显式分开**（`isRequestFailure`），正是今晚被混为一谈一小时的那一对。

**测试**：`dataState.test.ts` 8 条矩阵（成功空 / 404 / 422 / 422+actionFor / 500 / 网络失败 / 其余 4xx / scope_denied）；`state-landings.test.ts` 5 条落点断言。

### 1.2 四处落点

| 位置 | 现在 | 应该（已落地） |
|---|---|---|
| 门店 360 · 正式数据无事实（404） | 红色「请求的数据不存在或已被移除」 | `actionable`：「正式数据下没有该门店的经营事实。切换到模拟数据，或先导入正式数据。」+ **切换按钮**（writeQuery 切 simulated） |
| 合同详情 · 无付款计划算 IFRS 16（422 payment schedules required） | 「请求未成功（/calculate）」 | `actionable`：「该合同还没有付款计划，无法计量。已为你打开付款计划页签，去添加付款计划。」+ **自动切到 payments 页签**（`isActionableCalculateError` 纯函数判定） |
| 设置页 · 标签统计（422 data_unavailable 折现率缺失） | 红色 toast「加载标签数据失败」 | `actionable` inline Alert：「标签统计需要先补全折现率：CT-LE001、CT-LE002。请到合同工作台补录，或在本页配置全局折现率。」（合同编号从 error details 提取，不硬编码） |
| store-360 · pl-flow 失败 | FIX-024 已修（参考实现，error 到达面板） | 保持；FETCH-001 迁移后由接缝 failed 出口呈现 |

**设置页复现取证表**（浏览器实测，本批）：

| 请求 | HTTP | code | body | 修复前呈现 | 修复后呈现 |
|---|---|---|---|---|---|
| `GET /api/v1/reports/tags/summary` | 422 | `data_unavailable` | `{contracts:[CT-LE001,CT-LE002], discount_rate_missing:true}` | toast「加载标签数据失败」 | inline warning「标签统计需要先补全折现率：CT-LE001、CT-LE002…」 |

（合同详情场景的后端 422 路径与前端呈现链已按代码确认：`calculation.go:66` 的 422 → workspace `calculate` catch → 本批改为 actionable。）

**拦坑遵守**：failed 仍显眼（error Alert 保留）；scope_denied 独立；没有 catch 掉异常显示空白。

---

## 2. CONTRACT-001 后端能力清单 ↔ 前端选项清单契约（P0）

**做法**：前端 vitest 跨语言契约测试（`code-lists-contract.test.ts` 直接读取 core-service 的 Go 源码，正则提取白名单/定义清单）。

| 场景 | 前端清单 | 后端来源 | 断言 |
|---|---|---|---|
| 经营脉搏趋势 | `PULSE_KPI_CODES`（6） | `selectTrendKPIs` 白名单（9） | 前端选项 ⊆ 白名单 |
| 门店 360 卡片 | `STORE360_CODES`（6） | `retailkpi.Definitions` 的 Code 清单（19） | 每项 ⊆ 定义 |
| 桑基节点 | 前端不持有 key 清单（单一来源在后端） | `pl_flow.go` nodes | 前端无节点 key 字面量 + 后端 link ref ⊆ node keys |

**反向验证（必须红）**：临时从 `selectTrendKPIs` 白名单删除 `gross_profit` →

```
AssertionError: backend whitelist covers gross_profit: expected [ Array(8) ] to include 'gross_profit'
  → FAIL  app/lib/code-lists-contract.test.ts
```

已恢复（`git diff` 确认零残留）。4 条断言全部通过。

---

## 3. FETCH-001 取数层（P1，本批最大）

### 3.1 接缝

`web/app/retail/useRetailQuery.ts`：统一 loading / 竞态门（seq 序号 + active 双保险）/ token 注入 / STATE-001 出口（`DataState`）。paramsKey（字符串）作依赖解决对象引用漂移。参考了 `contracts/[id]`（命令总线 + transport）与 `ai-chat`（runtime）的既有接缝形态——**没有发明第三种模式**：它们是「命令/事件总线」，本接缝是查询场景的对应物，两者并存。

### 3.2 三页迁移

| 页面 | 迁移内容 | 旧竞态写法 |
|---|---|---|
| store-360 | storeDiagnostics + plFlow 两个查询 → useRetailQuery（actionFor 传 404→actionable） | `let active`（两处） |
| operating-pulse | loadPulse → useRetailQuery（pulseKey 作 paramsKey），selectedCurrency 派生 effect | requestGate |
| scenario-workbench | storeOptions 加载 → useRetailQuery | `let active` |

错误出口全部改由接缝产生：`plFlowError` 来自 `state.failed.message`（FIX-024 语义保留，`plFlowFailure.test.tsx` 断言适配为「接缝 failed 出口 + error={plFlowError}」，原「setPlFlowError 调用」断言随迁移更新）。

**实测（浏览器冒烟，迁移后）**：operating-pulse 6 张 KPI 卡、趋势图渲染、无错误；store-360 无错误；settings actionable 文案呈现 ✓。

其余 35 个直接 import `lib/api` 的文件不在本票范围（渐进迁移，另开票）。

---

## 4. STY-005 阶段一存量收敛重新立项（P2）

### 4.1 四文件静态内联收敛（按页面渐进）

| 文件 | 改动前 | 改动后 |
|---|---|---|
| `operating-pulse/page.tsx` | 12 处 | **0** |
| `store-360/page.tsx` | 15 处 | **0** |
| `scenario-workbench/page.tsx` | 11 处 | **0** |
| `contracts/[id]/page.tsx` | 6 处 | **0** |
| 合计 | **44** | **0** |

全部为静态常量（margin/width/height 等）转类（`pulse-*` / `store360-*` / `scenario-*` / `contract-*` 前缀类，token 化）；动态值（图表高度等）保留内联（DESIGN.md §13-2 允许）。`contract-block-gap` 初版混用 margin-top/bottom 语义，已拆 `-bottom` 变体修正。

### 4.2 止血条款

守卫对**新增（未跟踪）文件整文件扫描**既有生效（`enforce-design.mjs` 的 untracked 分支）——新文件引入内联样式 / !important 即拦。已由 `container-primitives.test.ts` 第 3 条断言固化。

### 4.3 工期重估

UIUX §4 已改为「按存量分摊，约 3–4 周」（DOC-001 落地）：原 1 周按 STY-001~004 六周只清一个文件的实测不成立。

---

## 5. PRIM-001 容器原语（P2）

**DESIGN.md §8.1 新增三条规则**：

1. **表格横向滚动必须走 `tableScrollX(rowCount, width)`**；裸 `scroll={{ x:` 只允许在「有数据才渲染」的条件分支（`rows.length ? <Table/> : <Empty/>`）——守卫按行检查 `?` 放行。
2. **recharts 图表必须包在 `<ResponsiveContainer>` 里**（FIX-028 教训：裸 Sankey 画空盒子）。
3. **第三类容器检查结论**：虚拟列表全仓不存在；Drawer 内表格受规则 1 约束。

**守卫测试**（`container-primitives.test.ts`，3 条）：全仓裸 `scroll={{ x:` 扫描（当前仅 3 处，全在条件分支内——operating-pulse attention 表与 performance 两张表，均有 `length ?` 守卫，不产生幽灵滚动条）；全仓图表组件必须有 ResponsiveContainer（修掉了把 `BarChartOutlined` 图标误判为图表的初版正则）；守卫新文件扫描规则固化。

---

## 6. DOC-001 更新两份方案文档（P1）

**《架构改善方案》**：
- §3.1 新增执行状态：D24 已建（enforce-design + 守卫测试，多批实际拦下违规）；第 3 条以 tokens-alignment.test.ts 落地，第 1 条（ESLint）仍待做
- §4 排期表后新增候选执行状态表：候选 01/02/03/04 ✅ 打勾并注明交付批次（ENF/ERR、ENV、KPI、SEC-002/003），候选 05（fetch）⬜ 进行中（本批 FETCH-001）
- §5.1 两个待决策 → 已决：i18n 三语全开（SUPPORTED_LANGUAGES + 零售三页 161 处 t()）；金额 ADR-0020 已落

**《UIUX改善方案》**：
- §4 阶段一工期重估（1 周 → 按存量分摊 3–4 周，附 STY-005 实测依据）
- D22 位置从阶段三**提前到阶段一之后**（FETCH-001），更新实测数字（apiRequest 169 处 / 38 文件 / 50 loading 标志）
- 新增 §6.5「数据状态的判定与表达」：STATE-001 三分法写成规则，并写明与阶段二 `<StateBlock />` 的关系（判定层 vs 呈现层）

**只更新事实与排期**，未改写两份文档的立场与红线（不换框架、不物理重命名、不删页面、经营口径 ≠ IFRS 16 口径）。

---

## 7. 验证（命令级实际输出）

```
web: npm run type-check          通过
web: npm test                    Test Files 43 passed (43)，Tests 279 passed (279)
web: npm run build               ✓ Compiled successfully
web: node scripts/enforce-design.mjs  81 个变更文件，无新增违规
core-service: go test ./...      38 packages ok
core-service: go vet ./...       VET_OK
```

浏览器冒烟（headless，登录后）：settings actionable 文案呈现（无 toast）；operating-pulse 6 卡 + 趋势图 + 无错误；store-360 无错误。

---

## 8. 结构变更清单

| 文件 | 变更 | 票 |
|---|---|---|
| `lib/dataState.ts` + `.test.ts` | classifyDataState 三分法纯函数 + 8 条矩阵 | STATE |
| `lib/state-landings.test.ts` | 四处落点断言（5 条） | STATE |
| `contracts/[id]/workspace/workspace.ts` | calculate catch → actionable（isActionableCalculateError + 切 payments 页签） | STATE |
| `settings/page.tsx` | 标签统计 422 → inline actionable（合同编号从 details 提取） | STATE |
| `store-360/page.tsx` | 404 → actionable（切模拟按钮）；诊断/pl-flow 迁 useRetailQuery | STATE/FETCH |
| `lib/code-lists-contract.test.ts` | 三对清单跨语言契约 + 反向验证 | CONTRACT |
| `retail/useRetailQuery.ts` + seam 测试 | 取数接缝（竞态门/token/三分法出口） | FETCH |
| `operating-pulse/page.tsx` | loadPulse → 接缝；requestGate 移除；内联清零 | FETCH/STY |
| `scenario-workbench/page.tsx` | storeOptions → 接缝；内联清零 | FETCH/STY |
| `contracts/[id]/page.tsx` | 内联清零 | STY |
| `globals.css` | 类定义（STATE actionable + STY 收敛类） | STATE/STY |
| `DESIGN.md` | §8.1 容器原语 | PRIM |
| `scripts/container-primitives.test.ts` | 全仓守卫契约（3 条） | PRIM |
| 两份方案文档 | 已决/排期/候选状态/数据状态节 | DOC |

**断点/角色分支**：无变更。**后端**：本批零改动（纯前端 + 文档）。

---

## 9. 我无法确认的部分（需要用户看）

1. **STATE-001 落点的观感**：settings actionable Alert 在标签卡上方的位置；store-360 切模拟按钮的文案长度（英文）；contracts 计算失败后自动切 payments 页签的跳转体感。
2. **STY-005 收敛后的排版一致性**：44 处内联转类后四页的间距是否与其它页面一致（类值即原内联值，理论上零视觉变化，但值得过目）。
3. **FETCH-001 迁移后的竞态体感**：快速切换分类/门店时 loading 与结果出现的节奏（接缝的竞态门应等价于原实现）。

视觉影响由 Reviewer 据此推导交用户确认；**用户点头前不合并**。
