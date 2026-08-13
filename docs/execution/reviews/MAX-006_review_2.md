# MAX-006 Review 2

结论：`CHANGES_REQUESTED`  
Reviewer：Codex 主任务  
日期：2026-08-12

## Review 1 已关闭项

- 非 rate KPI 已改为相对百分比，零 comparison 返回 null/reason；
- gross profit 已按两个排列做 2! Shapley，手工 Golden 30/30 通过；
- mixed-currency peer 已排除；目标 gap 时后端继续计算有效 peer，前端 peer 数据不再随 target 清空；
- observations 已消费 summary/benchmark/bridge，Evidence version 已收敛为目标门店范围；
- StoreOption 小写 DTO、client classification/dataset 互斥、合法空 options 状态、region/brand/store scope、partial/zero denominator/handler/IFRS16 zero-touch 测试均已补充；
- Reviewer 独立执行全量 Go tests/vet、Web 45 tests/type-check/build、`git diff --check`，全部通过；旧路由、旧页面和 UI 架构保持；
- Reviewer 在真实 PostgreSQL 运行 MAX-003/004/006 相关测试 `-count=2`，六个用例全部通过。Pulse 两轮分别约 15.59ms / 16.36ms、固定 2 Query + 1 QueryRow；Store 360 两轮各约 2.10s，内部 query count 断言通过；结束后相关法人、门店、事实和 dataset 残留均为 0。

## 剩余阻断问题

### P1：同群样本不足时 Daily Trend 仍画“中位数”线

位置：`core-service/internal/services/retailstore360/store360.go:489-516`。

Peer benchmark 已在少于 3 家时返回 `insufficient_peers`，但 `buildDailyTrend` 对任意 `len(vals)>0` 都返回 median。于是只有 1–2 家 peer 时，页面仍会画虚线，与任务票“同群样本不足时不画假线”冲突；现有 insufficient-peer 测试只检查 benchmark，没有检查 trend。

要求：每日某指标有效 peer 数少于 `MinimumPeerCount` 时保留真实 `peer_count`，但 `peer_median=null`；达到 3 家才返回 median。补两个运行时测试：固定 peer 总数少于 3；固定 peer 足够但某日有效 peer 少于 3。不得把不足样本补 0。

### P1：`decision_ready=false` 仍生成肯定性的期间变化观察

位置：`core-service/internal/services/retailstore360/store360.go:365-395,626-698`。

`makeSummary` 会为 partial KPI 计算 change，`buildObservations` 只检查 change 非空，因此 target/current 或 comparison 不完整时仍可能生成“本期较对比期变化 X%”的确定陈述。任务票明确要求 `decision_ready=false` 不生成肯定性文案；现有 partial 测试只检查 KPI/bridge，没有检查 observations。

要求：当整体 `decision_ready=false` 时只保留数据质量/核实类观察，不生成 summary、benchmark 或 bridge 的肯定性观察；或提供等价且可证明的严格 gate。补 partial、无事实、币种冲突三条断言，确保没有 `summary:*`、`benchmark:*` 或 bridge reference。decision-ready 完整路径继续保留三类观察与确定性排序。

## 最小 UI 收尾

### P2：Peer reason 未展示，gap 日 tooltip 误伤有效 peer

位置：`web/app/store-360/page.tsx:69-74,183`。

- peer 表只展示 raw status，没有任务票要求的 reason；`insufficient_peers` 用户看不到 `peer_count_below_minimum` 或可读说明；
- target gap 日虽然 peer line 已保留，但 tooltip 只看整行 `gap`，悬停 peer 也会显示“数据缺口”。

要求：peer 表展示 status + reason（可做稳定中文映射并保留原 code）；tooltip 只在 `name=target && row.gap` 时显示目标缺口，peer 有值时正常显示同群中位数。提取最小纯逻辑/formatter 并补测试；不重排页面、不引入新组件体系。

## 重新提交标准

- 只改上述 Store 360 service/test 与页面纯逻辑/测试、MAX-006 文档；不改 schema、旧页面、AppLayout、旧 route/API、IFRS 16、Official 或 MAX-007；
- 重新运行全量 Go/Web、`git diff --check`；真实 PostgreSQL本轮若未改 Repository/fixture 可引用 Reviewer Review 2 的已通过证据，无需 Executor 伪造本地通过；
- 报告写明 insufficient peer trend、decision-ready observation gate、peer reason 和 tooltip 的 expected/actual；
- 修复后任务票、报告、看板改回 `IN_REVIEW` 并停止等待 Review 3，不 commit、不 push、不进入 MAX-007。
