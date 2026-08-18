# MAX-005 Review 1

> ⚠️ **ARCHIVED 2026-08-18 — 不是现行依据**
> 归档理由：零售 MVP（MAX-001~009）转型执行的过程记录，全部工单已 ACCEPTED；未完成的延后治理项见 docs/execution/精简MVP路线与延后治理清单.md
> 现行入口：`docs/AI_文档索引与现行决策.md`

结论：`CHANGES_REQUESTED`  
Reviewer：Codex 主任务  
日期：2026-08-12

## 已通过

- 新增 `/operating-pulse`，旧 `/`、`/performance` 均无 diff；生产构建保留原 25 个路由并新增该页面；
- 导航只将“分析与决策”组前置并新增“经营脉搏”首项，原项目名称、route、权限判断和相对顺序保持不变；
- 页面沿用 `AppLayout`、`PageHeader`、Ant Design、Recharts 和现有 design tokens，没有改 Logo、字体、全局主题或旧页面布局；
- Latest dataset GET、Pulse 读取、手动生成固定模拟数据、URL repeated `store_id`、请求竞态保护、gap/null、单店身份、suppressed attention、来源冲突入口和 IFRS 16 口径隔离的主体实现成立；
- Reviewer 独立运行 `go test ./...`、`go vet ./...`、`npm test`（7 files / 36 tests）、`npm run type-check`、`npm run build` 和 `git diff --check`，全部通过；
- Reviewer 在真实 PostgreSQL 连续运行两次模拟生成与 Pulse 集成测试，均通过。60 店 Pulse 固定为 2 Query + 1 QueryRow，约 13.11ms / 29.78ms；A/B 法人隔离、模拟/正式边界、幂等和 IFRS 16/Official 零触达仍成立。

## 阻断问题

以下问题直接影响经营晨检可用性或任务票明确验收，不涉及高级审计、UI 重构或原功能改动。

### P1：正式数据切回模拟数据会进入不可恢复的缺 dataset 状态

位置：`web/app/operating-pulse/page.tsx:259`。

production URL 不携带 `dataset_version`；当前切回 simulated 时只读取当前空的 `datasetVersion` 或仅在“当前正好已是 matching simulated dataset”时存在的 `latestMetadata`。虽然 discovery state 中已有 `latest`，切换后仍会生成没有 dataset version 的 simulated URL，随后显示“模拟数据缺少 dataset_version”。discovery guard 已标记 loaded，不会再次自动补齐。

要求：切回 simulated 必须直接使用已发现的 `latest.dataset_version`、`latestAnomalyDate(latest)` 和 `retail_simulator`；没有 latest 时进入现有无数据/生成引导，不得构造无 dataset 的 simulated 请求。simulated → production 继续移除 dataset version。把模式切换规范化提取为可测试的纯逻辑，至少覆盖 production → simulated、simulated → production、无 latest 三条路径。

### P1：辅助 KPI 未满足 current/comparison/status/reason 合同

位置：`web/app/operating-pulse/page.tsx:253`。

五个辅助指标当前只显示 current 和 change，没有 comparison、status 或 reason。用户无法判断“人工成本率上升”来自完整数据还是 partial/unavailable，也不符合“每项显示 current、comparison 变化、正确单位和 status”的冻结标准。

要求：辅助指标显示 current、comparison、change、status；partial/unavailable/null 显示明确 reason 和 `—`，沿用核心 KPI 的方向颜色与 `%`/`pp` 语义。可抽取复用现有状态/格式逻辑，但不要重排页面或引入新设计系统。补 formatter/view-model 单测。

### P1：Attention 的单位展示会把 percent 直接显示成英文类型名

位置：`web/app/operating-pulse/page.tsx:109-110`。

`percentage_point` 被格式化为 `pp`，但 percent 当前显示字面量 `percent`；threshold 又没有同步单位。Tooltip 的 current/comparison 也缺少格式化单位。经营用户会看到例如 `-10.00 percent · 阈值 -8`，容易误读阈值口径。

要求：observed change、threshold、current、comparison 使用同一 unit formatter；percent 显示 `%`，percentage point 显示 `pp`，金额/数量按 API 单位合理展示，不得输出内部类型名。补 `%` 与 `pp` 的运行时断言。

## 必补的最小数据测试

### P2：Latest dataset 的 Repository 验收证据未覆盖完整选择边界

位置：`core-service/internal/repository/retail_simulation_postgres_integration_test.go`。

当前真实库测试证明了 A/B 和“较新的 completed”可选中，但没有插入时间更晚的 `generating` / `failed` 证明会忽略，也没有证明 `completed_at` 相同时按 `created_at, id` 稳定决胜，Repository 无数据也未直接断言。SQL 实现看起来正确，但任务票明确要求这些回归保护。

要求：在现有真实 PostgreSQL 测试中补最小 fixture，断言：忽略较新的 generating/failed；completed_at 相同按 created_at、id 确定排序；一个无 completed dataset 的法人返回 nil 且不串读。保持测试可连续运行并清理干净。

## 非阻断记录

- `RetailKPIUnit`、severity/status 等联合类型尾部带 `| string` 会退化为宽字符串；本轮不阻塞，登记后续类型 hardening 即可；
- `source_system` 输入每个字符都更新 URL/请求，体验可后续防抖或改为提交式；不进入本次最小返工；
- production 与 simulated 的零事实文案可分别说明导入事实或生成/选择 dataset；建议本次顺手修正文案，但不单独阻塞；
- 1440×900、390×844 和四张截图因 in-app browser 的本地 URL 权限被拒绝，Reviewer 也无法复现。该项保持“外部环境待验”，禁止伪造截图；本次返工无需反复尝试浏览器。视觉验收在环境允许时补做，并在最终 MVP 发布验收前关闭。

## 最小返工范围与重新提交标准

- 只修改 `/operating-pulse` 页面/纯逻辑测试、Latest dataset 现有 Repository 测试及 MAX-005 文档；
- 不修改旧 `/`、`/performance`、其他旧页面、AppLayout 结构、IFRS 16、Official、月结、Agent、高级审计或数据库 schema；
- 重新运行全量 Go/Web 验证，并用真实 PostgreSQL 对相关测试 `-count=2`；
- 报告逐项写明上述三个用户路径的修复和 expected/actual；截图继续如实标为环境待验；
- 修复后把任务票、报告和看板统一改回 `IN_REVIEW`，停止等待 Review 2；不进入 MAX-006，不 commit、不 push。
