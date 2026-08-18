# MAX-007 Review 4

> ⚠️ **ARCHIVED 2026-08-18 — 不是现行依据**
> 归档理由：零售 MVP（MAX-001~009）转型执行的过程记录，全部工单已 ACCEPTED；未完成的延后治理项见 docs/execution/精简MVP路线与延后治理清单.md
> 现行入口：`docs/AI_文档索引与现行决策.md`

结论：`ACCEPTED`  
Reviewer：Codex 主任务  
日期：2026-08-12

## 产品与规格结论

- `/scenario-workbench`、Store 360 进入/返回路径、导航优先级均以增量方式交付；旧页面、AppLayout、route/API、IFRS 16、Official、合同/事件和 Agent 主链路未删除、改名或替换；
- 30-day run-rate、七类显式 delta、Baseline/Plan、贡献桥、Golden、rounding、zero denominator、production/simulated provenance 和 current decision-ready 门槛符合冻结规格；
- Evaluate 保持一次 `QueryFacts`、零写入；422 返回稳定 reason 与 evidence；
- Action save 服务端重算，只写既有 `fpna_action_items`：同 key 重放、异 payload 409、首次并发收敛到同一真实 ID、新 key 新行、跨法人 key 隔离成立；未新增 migration；
- 页面 freshness/race gate、过期结果标识、证据与公式、options 错误、行动二次确认、真实 ID/status/Owner/Due/replay 和 `/performance` 跳转完成；不存在执行、过账、通知房东或租赁事件按钮。

## Reviewer 独立验证

真实 Docker PostgreSQL：

```text
go test -v ./internal/repository \
  -run '^TestRetailScenarioPostgresGoldenIsolationAndZeroTouch$' \
  -count=2
```

结果：两轮 PASS，分别约 3.31s / 3.24s；每轮日志 `scenario simulated+production SQL queries=18 query_rows=0`。覆盖首次并发、新 key 新行、真实 ID、同 key 冲突、A/B 法人、region/brand/store scope、production/simulated、provenance 与 IFRS16/Official/production zero-touch。结束后 `SCENARIO-*` 法人、`scenario-pg-*` action、scenario request 残留均为 0。

其他验证：

- `go test ./internal/services/retailscenario ./internal/handlers ./internal/repository`：PASS；
- `go vet ./...`：PASS；
- Web Vitest：9 files / 55 tests PASS；
- Web type-check、Next production build：PASS；build 同时保留 `/scenario-workbench`、`/store-360`、`/performance` 和全部旧页面；
- `git diff --check`：PASS；
- 全量 Go 的既有 `internal/aiagent` IPv6 httptest 沙箱限制不是 MAX-007 代码回归，已在交付报告如实登记；本票相关包与真实数据库均独立通过。

## 发布边界

- 浏览器 1440×900 / 390×844 的真实交互截图仍按既定外部权限阻塞登记到 MAX-009，不伪造；
- MAX-007 可以作为 AI 经营分析 Agent 的确定性情景工具底座；AI 仍只能 Assist Mode 调用/解释，不得绕过 action 草稿、权限、Official 或 IFRS 16 隔离。

MAX-007 正式接受，MAX-008 放行。
