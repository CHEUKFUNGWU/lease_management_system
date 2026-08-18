# MAX-004 Review 2

> ⚠️ **ARCHIVED 2026-08-18 — 不是现行依据**
> 归档理由：零售 MVP（MAX-001~009）转型执行的过程记录，全部工单已 ACCEPTED；未完成的延后治理项见 docs/execution/精简MVP路线与延后治理清单.md
> 现行入口：`docs/AI_文档索引与现行决策.md`

结论：`ACCEPTED`  
Reviewer：Codex 主任务  
日期：2026-08-12

## 最终独立验收

- `retail-pulse-v1` 复用 `retail-kpi-v1`，current/comparison、每日趋势、固定异常、coverage suppression、多币种和 zero-comparison 语义均有运行时测试；
- 下钻模板和实际 URL 保留 classification、dataset、source、全部显式 `store_id` 及两段日期；单店 attention 只进入授权门店；
- 空法人参数返回 400，来源冲突返回 409，其余 Repository 错误返回 500；零授权、零事实返回结构稳定且不伪造币种；
- 每店 provenance、suppressed attention 排序、score/severity 边界和 `contribution_turns_negative` 固定 `+1` 例外均已公开并由测试保护；
- 真实 PostgreSQL Golden 测试连续运行两次通过。60 店 Pulse 分别为约 48.87ms 和 35.95ms，均为 2 次 Query + 1 次 QueryRow，无门店级 N+1；
- 真实库路径证明 queried window 内最高事实版本为 2、420/420 覆盖且不重复聚合；production facts 可计算且不携带 simulated dataset version；A/B 法人、store/region/brand scope 均隔离；
- 两次真实 PostgreSQL 测试结束后测试法人、门店、数据集、批次残留均为 0；
- 全量 `go test ./...`、`go vet ./...`、Web `npm run type-check`、`npm run build` 和 `git diff --check` 全部通过；原 25 个页面路由完整生成；
- 本票未修改 `web/`，未删除、重命名或隐藏既有页面、导航、API、合同、IFRS 16、月结、报表或 Agent 能力，也未写入 Official/IFRS 16 台账。

## 放行决定

MAX-004 已满足经营脉搏首页的稳定消费合同、数据正确性和产品兼容红线，状态改为 `ACCEPTED`。MAX-005“经营脉搏首页”解除依赖。

非阻断的接口可维护性改进（查询参数对象化、scope/drilldown DTO 化、指标规格集中注册）登记到 Hardening Backlog，不进入当前用户可见功能关键路径。
