# STY-010 / STATE-004 / FETCH-003 交付报告（第二批）

提交人：ZCode（全栈）
提交时间：2026-08-16
分支：`fix/uiux-phase-four`（**未合 main**）
commit 列表：`6429c16`（theme.ts 注释 hex 清理，评审反馈）· `b0d956e`（STY-010）· `3fa868c`（STATE-004）· `adbff3b`（FETCH-003）

---

## 0. 评审反馈处理：theme.ts 注释 hex 清理

评审发现 `theme.ts:45` 注释写 `colorBorder: colors.border.default // #E5E5E5` 而真实值是 #D9D9D9。已处理：**5 处行内 hex 注释改为指向 token 名**（`// colors.border.default`），2 处叙述性注释去掉 hex 字样保留推理。注释 hex 归零——不再有可漂移的字面量。

---

## 1. STY-010 ai-chat 内联样式（P1）

| 项 | 迁移前 | 迁移后 |
|---|---|---|
| ai-chat 内联样式 | **255** | **25**（全部真动态） |
| 全仓内联样式 | 572 | **342** |

- 230 静态 → 113 个生成类（`sty-<hash8>`，相同值共享）；25 真动态保留（`msg.role === "user" ? ...`、`activeSessionId === session.id ? ...`、`meta.color` 等条件/变量）
- **只改样式**：未动 ai-chat 的逻辑与取数接缝（任务书约束）
- 转换器踩坑两次，已修：① 元素已有 className 时产生重复属性（跨行 JSX 标签合并 + 2 处 `className={expr}` 手工合并）；② 一处静态样式被 offset 漂移漏掉（手工补转）
- 342 vs 任务书目标 ~320：差值是任务书按「255 全静态」估的，实际 25 处是真动态必须保留

**运行时实测**（headless Chrome，真实页面）：50 个 sty-* 类解析出真实值（flex / padding 16px 12px / bg #F7F7F7），零页面错误。

---

## 2. STATE-004 StateBlock 铺开（P1）

**迁移 6 个文件**（每处归态 + 理由）：

| 文件 | 迁移点 | 归入哪一态 | 理由 |
|---|---|---|---|
| store-360 | no-dataset / missing-version | **actionable** | 用户可去经营脉搏生成/选版本（带跳转按钮） |
| store-360 | options 加载失败 | **failed** | 查询失败，可重试 |
| store-360 | pick-filters | **empty** | 等用户选门店，非错 |
| ProfitFlowPanel | 加载失败 | **failed** | FIX-024 语义保持（原因保留、无重试通道） |
| ProfitFlowPanel | 无 flow / unavailable | **empty** | 后端说没有 |
| agent-metrics | 加载失败 | **failed** | 查询失败（原因显示） |
| scenario-workbench | pick-store | **empty** | 等用户选门店 |
| operating-pulse | no-facts | **empty** | 后端说没有事实（title+desc 保留） |
| settings | 标签列表空 | **empty** | 页面数据空态 |

**保留 12 个文件**（理由表）：

| 文件 | 保留理由 |
|---|---|
| pulse 趋势/信号图、portfolio locale | 图表/表格内嵌 fallback（组件局部呈现） |
| todo ×4 | 3 个区块级列表空态 + 1 个双动作空态 |
| performance ×4 | Tabs 内各页签空态 |
| contracts 移动端列表 | 双动作空态（新建+上传 / 清筛选） |
| pulse isEmptyInitial | admin 生成 / 联系管理员双分支 |
| ai-chat ×2 | 有自己接缝的页面；会话/追踪列表区块空态 |
| settings modal、monthly-closing ×3 | 弹窗/区块局部空态 |
| DashboardLists/Charts、NotificationBell、RetailAIDrawer | 复用组件契约 |

**判断边界**：StateBlock 管「单动作或纯提示的**页面级**数据状态」；多动作空态、图表/表格内嵌 fallback、组件契约、有自己接缝的页面保留。每处判断理由如上表。

---

## 3. FETCH-003 取数层铺完（P1）

| 项 | 迁移前 | 迁移后 |
|---|---|---|
| 直连 lib/api 的文件 | 38 | **29** |
| 其中只导入类型 | 7 | 7（**不算直连**，任务书口径） |

**迁移 8 个文件的主查询**：
- **audit-logs**：筛选+分页查询 → seam（paramsKey 含 filters + page；搜索/重置改 state 触发 refetch）
- **sensitivity / standards**：approved 合同下拉 → seam（挂载查询）
- **portfolio**：summary + unit-price 双查询 → seam（mode/grouping 驱动）；刷新按钮 → retry
- **performance**：四视图合并 → 单 seam 查询（`cockpit-${period}` key）；**FIX-026 语义保持**——periodApply.test 适配为断言 paramsKey 派生自 applied period 而非 draft，**反向验证**：把 key 改成 periodDraft 测试红 ✓
- **cashflow-forecast**：tags 下拉 → seam
- **admin/users**：用户列表 → seam；创建后刷新 → retry
- **RenewalCard**：参数驱动续租查询 → seam（参数不完整时 params=null 禁用查询，等价旧 early-return）

**保留 21 个文件**（报告理由）：
- **动作型页面**（deal-compare / pre-deal / roi / monthly-closing 动作 / sensitivity-standards 主计算 / contracts-new 创建）：按钮触发表单查询，与 FETCH-002 的 amortization 同理
- **表单副作用加载**（contracts/new：fetch 里 setFormValues，seam 的 fetcher 不能做副作用）
- **组件契约**（GlobalSearch 防抖搜索、NotificationBell、RetailAIDrawer、Dashboard lists/charts）
- **home 简报模块**（BriefColumn/briefGate/proposals——有自己的 brief 逻辑）
- **认证页**（login/admin-login）
- **有自己接缝**（ai-chat / contracts-[id]，任务书明确不强行套）
- **次要主数据**（admin/users 的 legal entities 下拉、settings 多区块配置查询）

**新增测试**：performance periodApply.test 适配 + 反向验证；每迁页的竞态/错误路径由 seam 自身测试覆盖（FETCH-001 已有），迁移页运行时冒烟零错误。

---

## 4. 验证（命令级实际输出）

```
web: npx vitest run        → 46 files / 294 tests passed
web: npx tsc --noEmit      → 干净
web: npx next build        → ✓ Compiled successfully
web: node scripts/enforce-design.mjs → 22 变更文件，无新增违规
```

运行时冒烟（headless Chrome）：audit-logs / portfolio / performance / sensitivity / standards 渲染正常（表格/卡片就位）、零 error Alert、零页面错误。

---

## 5. 结构变更清单

| 文件 | 变更 | 票 |
|---|---|---|
| `web/app/design-system/theme.ts` | 注释 hex → token 名 | 评审反馈 |
| `web/app/ai-chat/page.tsx`、`globals.css` | 230 静态内联 → 113 类 | STY-010 |
| `web/app/{store-360,agent-metrics,scenario-workbench,operating-pulse,settings}/page.tsx` + `ProfitFlowPanel.tsx` | Empty/error Alert → StateBlock | STATE-004 |
| `web/app/{audit-logs,sensitivity,standards,portfolio,performance,cashflow-forecast,admin/users}/page.tsx` + `RenewalCard.tsx` | 主查询 → useRetailQuery | FETCH-003 |
| `web/app/performance/periodApply.test.ts` | FIX-026 断言适配（paramsKey 语义，反向验证过） | FETCH-003 |
| `web/app/lib/i18n.ts` | +performance.load_failed / portfolio.* 三语 key | FETCH-003 |

---

## 6. 我无法确认的部分（需要用户看）

1. **STY-010 转换后的 ai-chat 布局**：230 处样式逐字搬运（哈希去重），理论零变化；但 ai-chat 是复杂布局页，值得实机走查一遍（尤其会话列表与消息气泡）。
2. **STATE-004 的 empty 呈现**：pulse no-facts 从 Card+Empty 变 StateBlock 安静行（密度更紧凑）；settings 标签空态同理——观感需确认。
3. **FETCH-003 迁移后的交互**：audit-logs 的搜索/分页节奏、portfolio 刷新、performance 期间切换、RenewalCard 参数调整的体感——接缝竞态门应等价，值得实机走查。
4. **保留的 21 个文件**：若 Planner 认为某些该迁（如 monthly-closing 的查询部分），可另开票。

**第二批完成，待 Planner 复验。** 通过后开第三批（DARK-001 + DENSITY-001）。
