# MAX-005 Review 2

结论：`ACCEPTED`  
Reviewer：Codex 主任务  
日期：2026-08-12

## 最终独立验收

- `/operating-pulse` 作为增量经营晨检首页成立；原 `/`、`/performance` 没有 diff，原页面、路由、导航项目、合同、IFRS 16、月结、报表和 Agent 能力均继续保留；
- production → simulated 使用已发现的 latest dataset version、manifest 最晚日期和 `retail_simulator`，simulated → production 移除 dataset version；无 latest 时回到明确空状态，不再构造不可恢复请求；
- 六项核心 KPI 和五项辅助 KPI 均展示 current、comparison、change、status/reason；null、partial、unavailable 不转成 0，也不使用“正常”视觉；
- `%`、`pp`、currency、count、currency/㎡ 的 formatter 贯穿 KPI 和 Attention observed/current/comparison/threshold，页面不再泄露内部 `percent` 类型名；
- latest discovery 只选择当前法人 completed dataset，确定排序为 completed_at、created_at、id；更晚的 generating/failed 被忽略，无 completed 是 `(nil,nil)` 并由 Handler 返回 200 `data:null`；
- 请求竞态、URL 状态、repeated `store_id`、gap 断线、multi-currency 分区、attention 原 rank、suppressed 分离、单店身份和来源可信条均有代码/测试证据；
- Reviewer 独立运行全量 `go test ./...`、`go vet ./...`、Web `npm test`（7 files / 38 tests）、`npm run type-check`、`npm run build` 和 `git diff --check`，全部通过；Next build 共 26 个应用路由，旧 25 个完整保留；
- Reviewer 通过真实 Docker PostgreSQL 连续运行两次模拟生成与 Pulse 集成测试，全部通过。Pulse 固定 2 Query + 1 QueryRow，约 18.21ms / 14.57ms；A/B 法人隔离、幂等、模拟/正式边界及 IFRS 16/Official 零触达继续成立；
- 真实测试完成后 `DAY-LE-sim-*` 法人、门店、数据集及固定排序 fixture ID 残留均为 0。

## 视觉证据例外

任务票要求的 1440×900、390×844 和四张截图仍未完成：Executor 与 Reviewer 的 in-app browser 均被本地 URL 权限策略拒绝。没有伪造或替代截图。该限制不是代码或产品逻辑缺陷，本次不再阻塞下一张用户可见功能票；桌面/移动端视觉、键盘与控制台验收必须保留为 MAX-009 发布验收的前置关闭项。

## 放行决定

MAX-005 的经营晨检主路径、数据正确性、租户隔离、模拟标识、来源追溯和旧产品兼容红线均已满足，状态改为 `ACCEPTED`。MAX-006“门店 360 与异常下钻”解除依赖。

非阻断维护项（TypeScript 宽字符串类型、source_system 输入防抖）留在 Hardening/体验 backlog，不进入当前 70% 用户可见功能关键路径。
