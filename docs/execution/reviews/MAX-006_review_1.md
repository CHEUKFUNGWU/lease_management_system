# MAX-006 Review 1

结论：`CHANGES_REQUESTED`  
Reviewer：Codex 主任务  
日期：2026-08-12

## 已通过

- 新增 `/store-360`、授权门店选择、Working 数据可信条、同群表、趋势、变化桥和证据折叠区；`/operating-pulse` 原“查看门店脉搏”保留并新增独立“门店 360”入口；
- 旧 `/`、`/performance` 无 diff，原页面、route、AppLayout、导航结构、视觉 tokens、IFRS 16、月结、Official 和 Agent 能力没有被删除、替换或重构；
- diagnostics 主读取只调用一次 `QueryFacts`；Reviewer 的真实 PostgreSQL 运行记录显示固定 2 Query + 1 QueryRow，未出现 N+1；
- 法人 A 不能读取法人 B 的目标门店；模拟数据仍携带 dataset/source/version，production 路径不携带 dataset；
- Reviewer 独立运行 `go test ./...`、`go vet ./...`、`npm test`（8 files / 44 tests）、`npm run type-check`、`npm run build` 和 `git diff --check`；除下述真实 PostgreSQL MAX-006 验收用例外均通过。生产构建包含 `/store-360` 且保留全部旧路由；
- 真实 PostgreSQL 测试失败后 cleanup 生效，`PULSE-ST-store360-*` 门店、事实和法人残留均为 0；
- 浏览器仍受本地 URL 权限阻断，未伪造截图。本项继续留到 MAX-009 发布验收，不单独阻断本次代码返工。

## 规格轴阻断问题

以下问题会直接让经营用户看到错误变化率、错误分解或错误同群，因此不能以“测试通过”替代修复。

### P1：非比率 KPI 把绝对差额标成百分比

位置：`core-service/internal/services/retailstore360/store360.go:359-381`。

`makeSummary` 对 revenue、gross profit、footfall、ATV、contribution、sales/sqm 等设置 `change_type=percent`，但 `change_value` 实际是 `current-comparison`。前端复用 percent formatter 后会给绝对金额或数量差额加 `%`。例如 current=120、comparison=80，当前返回 `40%`，正确值应为 `50%`。

要求：非 rate 指标使用 `(current-comparison)/abs(comparison)*100`；comparison 为 0 时返回 null 和稳定 reason，不制造无穷值。rate 指标继续返回百分点差。复用或抽取 MAX-004/MAX-005 已验证语义，补一个“绝对差额不等于百分比”的运行时测试和零 comparison 测试。

### P1：毛利桥声称 2! Shapley，实际只计算一个顺序

位置：`core-service/internal/services/retailstore360/store360.go:558-570`。

`shapleyBridge2` 只走“先 revenue、后 margin”一个顺序，守恒成立但不等于 Shapley。例：旧 revenue=100/margin=20%，新 revenue=200/margin=40%，正确 2! 平均后两个贡献均为 30；当前实现返回 revenue=20、margin=40。

要求：平均两个排列的边际贡献，或使用可接受任意维数的通用 permutation 实现；新增上述手工 Golden，分别断言 item contribution，而不只断言总和守恒。

### P1：混合币种门店可错误进入“同币种”同群

位置：`core-service/internal/services/retailstore360/store360.go:391-405`。

代码先按目标币种过滤事实，再调用 `singleCurrency`，因此同时存在 CNY 与 USD 的 peer 只要有 CNY 行就会被接受。这样会把币种冲突门店纳入中位数和分位。

要求：先检查 peer 当前期全部事实只存在一个币种且等于目标币种，再按该币种聚合；新增 mixed-currency peer 排除测试，并保留目标门店、brand、region、decision-ready、最少 3 家边界。

### P1：目标门店缺某日时错误抹掉仍有效的 peer 趋势

位置：`core-service/internal/services/retailstore360/store360.go:475-510`、`web/app/store-360/page.tsx:69-74`。

目标 gap 只应断开目标线；固定 peer 集在该日仍有有效事实时，同群中位数应继续显示。当前后端把所有 peer median/count 清空，前端也以 `row.gap` 再次强制 peer=null，现有测试反而把错误行为固化。

要求：目标 target 保持 null/gap，但独立计算当天有效 peer median/count；前端 peer 不依赖 target gap。改写测试为“目标 gap + peer 有效”，同时覆盖 peer 当日也不足时的 null。

### P1：观察信号未消费 summary 与 peer benchmark

位置：`core-service/internal/services/retailstore360/store360.go:609-650`。

`buildObservations` 明确丢弃 `s` 和 `b`，只输出数据质量与 bridge item。任务票要求观察信号来自 current/comparison、peer benchmark 和 bridge，并按 bridge 绝对贡献或 target-minus-median 排序；当前报告“证据完整”表述过度。

要求：至少生成可追溯的 period-change 与 peer-difference 观察项，引用稳定 evidence/benchmark/bridge ID，按冻结规则确定性排序；只描述事实差异，不使用“根因、因果、导致、证明”等词。补 peer 差异、排序、insufficient peer、不完整数据和全部禁词运行时测试。

### P1：真实 PostgreSQL production 验收连续两次失败

位置：`core-service/internal/repository/retail_store_diagnostics_postgres_integration_test.go:77-89`。

fixture 注释与断言声称 current/comparison 均完整，但只插入 `2026-05-23..2026-06-05` 14 天；请求的 comparison 是 `2026-05-09..2026-05-22`，所以 `DecisionReady=false`。Reviewer 用既有 Docker PostgreSQL 执行相关测试 `-count=2`，两轮 MAX-006 用例均在该断言失败；其余 simulation/Pulse 测试通过。

要求：production fixture 插入完整 28 天，并分别断言 current/comparison coverage，而不是打印整个巨大 envelope。真实 PostgreSQL 连续两次必须通过，结束后相关门店、事实、dataset、batch、法人残留为 0。

## 工程标准与证据缺口

### P2：Evidence 的事实版本范围不是目标门店范围

位置：`core-service/internal/services/retailstore360/store360.go:261-267`。

响应顶层已用 `factVersionRange(storeFacts)`，但 `Evidence` 又使用整个授权 FactSet 的 min/max，可能把 peer 或其他门店版本写成目标门店证据。要求 Evidence 与顶层统一使用目标门店 current+comparison 的版本范围，并补“其他门店有更高版本、目标门店没有”的测试。

### P2：前端数据合同与空门店状态不稳

位置：`web/app/lib/api.ts:1366-1377,1529-1539`、`web/app/store-360/page.tsx:110-119,161`。

- `RetailStore360Option` 同时允许 Go 大写和 JSON 小写字段，页面再 `as any`；Handler 实际已返回稳定小写 DTO。应只保留小写合同并移除 `any`；
- `storeOptions` / `storeDiagnostics` 客户端没有像 Pulse 一样强制 simulated 必须有 dataset、production 禁止 dataset；补两 API 的互斥单测；
- Select 以 `options.length===0` 判断 loading，合法空列表会永久转圈。增加独立 `optionsLoading`，加载完成后的空列表显示可操作空状态。

### P2：任务票要求的 scope/降级测试尚未形成证据

当前 service 测试主要覆盖单次查询、守恒、固定方向、窗口和错误的 gap 行为；真实库测试只证明 A/B 法人和 production，未证明报告所称 region/brand/store scope。要求补最小集合：

- region、brand、store scope 下 options 与 diagnostics 都不越权；
- peer brand/region 排除、mixed currency 排除、少于 3 家 `insufficient_peers`；
- partial/unavailable、零分母、bridge unavailable、不补 0、rounding residual；
- Handler 的空法人、production+dataset、simulated 无 dataset、source conflict、repository 500，以及 options 小写 JSON 合同；
- API client 的 classification/dataset 互斥；
- MAX-006 测试前后 IFRS 16/Official 关键表计数不变，或明确复用同一测试进程中已有的边界断言并给出结果。

## 最小返工边界与重新提交标准

- 只修改 MAX-006 新增的 store-360 service/handler/repository test、`/store-360` 页面/逻辑/API client 测试和 MAX-006 文档；允许抽取小型共享 percent-change helper，但不得改旧页面视觉与行为；
- 不修改数据库 schema，不改 `/`、`/performance`、`/operating-pulse` 既有能力、AppLayout 架构、Logo/字体/主题，不删除或重命名任何旧 route/API/功能；
- 不进入 MAX-007，不扩展到 AI、情景写入、行动执行、高级审计或 Official；
- 全量运行 Go tests/vet、Web tests/type-check/build、`git diff --check`；真实 PostgreSQL运行 MAX-003/004/006 相关测试 `-count=2` 并记录 query count、耗时和零残留；
- 报告逐项给出本 Review 的 expected/actual，纠正 Shapley、gap、scope 和观察信号的过度声明；
- 修复后把任务票、报告和看板统一改回 `IN_REVIEW`，停止等待 Review 2；不得 commit、push 或自行接受。
