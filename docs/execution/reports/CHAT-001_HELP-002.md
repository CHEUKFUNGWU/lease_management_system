# CHAT-001 / STY-006 / GUARD-001 / STATE-002 / FETCH-002 / HELP-002 交付报告

提交人：ZCode（全栈）
提交时间：2026-08-16
分支：`fix/workstation-shell-005-014`（叠加在 4c00455 任务书 commit 之上，**未合 main**）
6 个独立 commit（每票一个）：STATE-002 / GUARD-001 / STY-006 / CHAT-001 / FETCH-002 / HELP-002

---

## 1. STATE-002 判定顺序收口（P1）

**改动**：`web/app/lib/dataState.ts` 的 `classifyDataState` 判定顺序从
`scope_denied → actionFor → isRequestFailure → 404/422 empty → failed` 改为
`scope_denied → isRequestFailure → actionFor → 404/422 empty → failed`。

**理由**：`actionFor` 原排在 `isRequestFailure` 前，接缝「允许」未来调用方写一个
无条件 `actionFor`，把网络失败 / 500 呈现成「你去做点什么就好了」——正是
STATE-001 这一整批要消灭的形状。现在是接缝「拒绝」这种用法：`actionFor` 只能
升级后端回答过的错误（404/422/其余 4xx），请求没打通的错误先落 `failed`。

**测试**（`dataState.test.ts` 8 → 11 条）：无条件 `actionFor` + 网络失败 → `failed`；
无条件 `actionFor` + 500 → `failed`；无条件 `actionFor` + 404/422 → 仍可升级为
`actionable`（现有两个调用方——门店 360 的 404、设置页的 422——不受影响）。

```
web: npx vitest run lib/dataState.test.ts
✓ app/lib/dataState.test.ts  (11 tests) 2ms
```

---

## 2. GUARD-001 「替换类」改动验收规则（P0）

**改动**：`AGENTS.md` 验证一节新增「替换类改动的验收规则（GUARD-001，缺失即退回）」：

1. **运行时实测**：报告给出 `getComputedStyle` / `getBoundingClientRect` 的改动前后
   对照（FIX-019 / FIX-005 先例）；
2. **精确规则体断言**：用 `ruleBody()` 一类辅助函数取目标选择器规则体，禁止
   `expect(css).toMatch(/A[\s\S]*?B/)` 全文跨规则正则（FIX-021 教训）；
3. 样式重构类改动必须逐处核对「文件/行号/原内联值/现类名/规则值/是否同元素」，
   自检句：**把 B 的规则删掉或改错，这条测试会不会红？不会红就是没写对。**

未放宽任何守卫本体（`enforce-design`、`class-coverage` 原样）。

---

## 3. STY-006 STY-005 其余部分自查（P0）

**方法**：写 `web/scripts/sty006-audit.py`（保留在仓库供复跑），对照
`git show d5903cf:<file>` 与当前版本，提取每个文件的「原内联样式（行号+元素）」
与「现类名 + globals.css 规则体」，逐处核对 44 个替换点。

**核对结果：44 处中 41 处同元素同值 ✓，3 处值错位（FIX-034 回填时把类并进
共享规则，值被改掉了）**：

| 文件 | 元素 | 原内联值 | 现类名 | FIX-034 规则值 | 判定 |
|---|---|---|---|---|---|
| store-360/page.tsx L239 | peer benchmark Table | `marginTop: 8` | `store360-peer-table` | 16px（并进 block-gap 组） | **错位，已修** |
| scenario-workbench/page.tsx L196 | loading Flex | `minHeight: 160` | `scenario-loading-block` | 220px（并进 loading 组） | **错位，已修** |
| scenario-workbench/page.tsx L197 | baseline 卡内 tag 行 | `marginTop: 12` | `scenario-block-gap-sm` | 8px | **错位，已修** |

**修复**（`globals.css`）：三个类各自拆出独立规则，值回填为原内联值
（peer-table 8px、scenario-loading 160px、scenario-block-gap-sm 12px），并加注释
说明为什么不能并回共享组。

**实测断言**（`web/app/lib/replacement-values.test.ts`，4 条，kpi-card-height 的
精确规则体形状）：三个类的值各自锁定 + 断言三类未重新并入 16px 共享组。

**反向验证（必须红）**：把 peer-table 临时改回 16px →

```
FAIL  replacement-values.test.ts > keeps .store360-peer-table at its original 8px top gap
AssertionError: expected '\n  margin-top: 16px;\n' to match /margin-top:\s*8px/
```

已恢复，测试全绿。

**运行时实测值**（headless Chrome，登录后注入探测元素读 getComputedStyle）：

| 类 | 修复前（FIX-034） | 修复后（本批） |
|---|---|---|
| `.store360-peer-table` margin-top | 16px | **8px** |
| `.scenario-loading-block` min-height | 220px | **160px** |
| `.scenario-block-gap-sm` margin-top | 8px | **12px** |

---

## 4. CHAT-001 首页简报不再污染会话列表（P1）

### 4.1 设计（codebase-design 词汇）

会话（session）是一个 module，interface 是 `AIChatSession` + repository 操作，
「用户可见会话列表」（`ListSessions`）是它对外的一个查询。问题根源在
`Runtime.prepare`（runtime.go:151-164）：`session_id` 为空时自动 `CreateSession`，
所以每次刷新/开新标签/重登都新建一个「请读取当前经营脉搏并生成…」会话。

两个候选方案：
- **B（复用固定「每日简报」会话）是浅模块**：把「查找或创建固定会话、命名约定、
  消息无限增长后的清理」推给调用方，`GetSession` 拉 100 条消息的硬上限还会截断
  历史简报——复杂度在接口外散开。
- **A（`initiator` 维度 + 列表默认过滤）是深模块**：接口增量只有「一个枚举字段
  （user/system）+ 列表查询默认排除 system」。「用户可见」成为 `ListSessions` 的
  默认语义，规则收进模块内部（locality），前端 ai-chat 侧栏零改动，审计链路
  （session/run/messages/events）原样保留——每个简报仍是独立 session+run+trace。

**选 A**。

### 4.2 实现

- **DB**：`db/migrations/042_ai_chat_session_initiator.sql`（增量迁移）+
  `db/init/01_init.sql` 同步：`initiator VARCHAR(20) NOT NULL DEFAULT 'user'` +
  CHECK（user/system）。既有行默认 user，不编造数据。
- **repository**：`AIChatSession.Initiator`；`CreateSession`/`GetSessionByID`/
  `ListSessions` 读写该列；`AIChatSessionFilter.ExcludeInitiator`（空 = 不过滤）。
- **aichat**：`Input`/`SessionCommand` 携带 `Initiator`，`OpenSession` 与
  `prepare` 自动建会话时写入。
- **handler**：`aiagent.Request.Initiator`（`/ai/chat` 透传）；
  `GET /ai/chat/sessions` 默认 `ExcludeInitiator="system"`，`?include_system=true`
  放行（展示系统活动时可用，接口可扩展）。
- **前端**：`aiChatApi.chat` 接受 `initiator`；`runHomeBrief` 传 `"system"`。

### 4.3 验收（命令 + 实际输出）

```
core-service: GOCACHE=$(pwd)/.gocache go test ./...   → 38 packages ok
web: npx vitest run home/briefGate.test.ts             → 6 tests passed
```

**端到端运行时验证**（重建 core 镜像 + 迁移 042 应用到运行库 + 真实 API）：

| 断言 | 实测 |
|---|---|
| 触发 3 次 `initiator=system` 的 `/ai/chat` | 3 个 system 会话 + 3 个 run 创建（session 3fba1dd4 / f5fc6440 / 1bcb39fb） |
| 默认 `GET /ai/chat/sessions`（用户可见） | 49 条，**system-initiated 可见数 = 0** ✓ |
| `?include_system=true` | 50 条，system-initiated = 3 ✓（审计可达） |
| system 会话可查（`GET /sessions/{id}`） | initiator=system；runs=1、messages=2 ✓ |
| run trace | `GET /ai/chat/runs/{id}/events` → 5 个事件 ✓ |

镜像验证：`strings ./api | grep -c initiator` → 6（新二进制含 initiator）。

---

## 5. FETCH-002 取数层继续迁移（P2）

**实测现状与任务书前提的差异（如实报告）**：任务书说三页「已有测试」，但
`find web -name "*.test.*"` 全仓扫描 + grep `contractApi` / `workQueueApi` /
`reportsApi` 均无任何测试文件——三页实际**没有测试**。按任务书点名迁移（它们确有
真实流量），迁移行为等价性由冒烟验证覆盖；接缝本身的测试在 FETCH-001 已建。

| 页面 | 迁移内容 | 行为保持点 |
|---|---|---|
| 合同台账 `/contracts` | `loadContracts` 手写加载 → `useRetailQuery`（listParams 派生自 URL state，paramsKey=JSON） | 300ms 搜索防抖保留（URL 立即更新、请求经 debouncedSearch 延迟）；服务端页码归一化只在值不同时回写 URL，不会循环 |
| 我的待办 `/todo` | `load` 三个请求（workQueue+readiness+exceptions）→ 单个 seam 查询（fetcher 内 Promise.all） | 刷新按钮 → `retry()`；错误经 seam 出口 → `notifyError` |
| 报表查询 `/reports` | Tab1 ledger（liabilityRolling+contractSummary）与 tags 挂载查询 → seam；amortization 保留手动 fetch | amortization 是「用户点按钮 + 日期校验 + 成功 toast」的动作型查询，与 `contracts/[id]` 同理不强行套接缝（报告原文标注） |

**拦截遵守**：未把 `contracts/[id]`、`ai-chat` 迁入 `useRetailQuery`（它们有自己的
接缝，任务书明确不许强行套）。

**验证**：`tsc --noEmit` 干净；45 test files / 288 tests 全绿；build 通过；
enforce-design 无新增违规。冒烟：三页主内容均渲染
（`.contracts-desktop-table` / `.ant-card` / `.ant-tabs` 存在）。

---

## 6. HELP-002 教程面板铺开（P2）

三页接入既有机制（`PageHeader` help 槽位 + `HelpTrigger` + Drawer + SVG 流程图），
未发明新模式：

| 页面 | content 函数 | i18n keys | 说明 |
|---|---|---|---|
| 门店 360 `/store-360` | `store360HelpContent` | help.store360.*（24） | 四步流程 + 三节 |
| 经营驾驶舱 `/performance` | `performanceHelpContent` | help.performance.*（24） | 四步流程 + 三节 |
| 组合分析 `/portfolio` | `portfolioHelpContent` | help.portfolio.*（24） | 四步流程 + 三节 |

**红线遵守**：合规口径句全部留在页面常驻（store-360 `scope_note` meta、驾驶舱
「不替代 Official 关账」meta、组合分析模式 Alert），**未收进教程面板**。
performance / portfolio 两页原无 `useLanguage`，补上以便教程按当前语言渲染。

**验证**：i18n keys 测试（i18n-keys.test.ts 全仓扫描）通过；冒烟实测三页
`.page-header-help-trigger` 各 1 个（可见）。

---

## 7. 验证（命令级实际输出）

```
core-service: GOCACHE=$(pwd)/.gocache go test ./...       → 38 packages ok
core-service: GOCACHE=$(pwd)/.gocache go vet ./...        → VET_OK
web: npm run type-check                                    → 干净
web: npx vitest run                                        → 45 files / 288 tests passed
web: npx next build                                        → ✓ Compiled successfully, 28/28 静态页
web: node scripts/enforce-design.mjs                       → 98 个变更文件，无新增违规
```

运行时实测（headless Chrome / 真实容器 API）：
- STY-006 三处修复值：8px / 160px / 12px（见 §3 表）
- HELP-002 三页 help 触发器：1 / 1 / 1
- FETCH-002 三页主内容：rendered / rendered / rendered
- CHAT-001 列表过滤 + 审计链：见 §4.3 表

---

## 8. 结构变更清单

| 文件 | 变更 | 票 |
|---|---|---|
| `web/app/lib/dataState.ts` + `.test.ts` | 判定顺序收口 + 3 条矩阵测试 | STATE-002 |
| `AGENTS.md` | 验证节新增替换类改动验收规则 | GUARD-001 |
| `web/app/globals.css` | 3 处错位值回填（peer-table/loading/sm） | STY-006 |
| `web/app/lib/replacement-values.test.ts`（新） | 3 值 + 组归属断言（反向验证过） | STY-006 |
| `web/scripts/sty006-audit.py`（新） | 44 处替换点审计脚本（可复跑） | STY-006 |
| `db/migrations/042_ai_chat_session_initiator.sql`（新） | initiator 列 + CHECK | CHAT-001 |
| `db/init/01_init.sql` | ai_chat_sessions 同步 | CHAT-001 |
| `core-service/internal/repository/ai_chat_runtime.go` | Initiator 读写 + ExcludeInitiator | CHAT-001 |
| `core-service/internal/repository/ai_chat_runtime_postgres_integration_test.go` | initiator 过滤集成测试（skip 无 DB） | CHAT-001 |
| `core-service/internal/aichat/{types,runtime}.go` | Input/SessionCommand.Initiator | CHAT-001 |
| `core-service/internal/aiagent/agent.go` | Request.Initiator | CHAT-001 |
| `core-service/internal/handlers/ai_chat.go` + `ai_chat_runtime.go` | 透传 + 列表默认过滤 | CHAT-001 |
| `core-service/internal/handlers/ai_chat_runtime_test.go` | 过滤契约测试 ×2 | CHAT-001 |
| `web/app/lib/api.ts` + `home/briefGate.ts` + `.test.ts` | initiator 传参 + 测试 | CHAT-001 |
| `web/app/contracts/page.tsx` | 台账主查询 → 接缝 | FETCH-002 |
| `web/app/todo/page.tsx` | 三请求合并 → 接缝 | FETCH-002 |
| `web/app/reports/page.tsx` | ledger+tags → 接缝 | FETCH-002 |
| `web/app/components/help-content.ts` | 三份 content 函数 | HELP-002 |
| `web/app/lib/i18n.ts` | help.store360/performance/portfolio 三语 keys | HELP-002 |
| `web/app/{store-360,performance,portfolio}/page.tsx` | help 槽位接线；两页补 useLanguage | HELP-002 |

---

## 9. 我无法确认的部分（需要用户看）

1. **CHAT-001 侧栏观感**：迁移前历史残留的「请读取当前经营脉搏…」旧会话（迁移前
   创建的，initiator=user）仍会显示在侧栏——本票只拦新产生的，旧会话可手动删除
   或另开票清理。需要用户确认侧栏现状是否符合预期。
2. **STY-006 三处间距观感**：8px/160px/12px 是回填原内联值（零视觉变化），但
   peer table 与 loading 块与周围卡片的实际间距值得过目。
3. **FETCH-002 行为等价**：合同台账的搜索防抖与页码回写、待办刷新按钮、报表
   ledger 的 tab 切换体感——接缝竞态门应等价于原实现，但值得实机走查一次。
4. **HELP-002 教程内容**：三页教程的文案与流程图是否准确描述了页面用法
   （内容为代码可读事实推导，未实际点开 Drawer 目验）。

**后端已重建镜像 + 迁移 042 已应用到运行库**（`lease-core` 容器、`lease` 库）；
前端 dev server 3001 已重启（build 后覆盖过 `.next`，重启后冒烟通过）。

视觉影响由 Reviewer 据此推导交用户确认；**用户点头前不合并**。
