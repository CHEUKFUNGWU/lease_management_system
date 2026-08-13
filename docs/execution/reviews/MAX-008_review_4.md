# MAX-008 Review 4

结论：`ACCEPTED`  
Reviewer：Codex 主任务  
日期：2026-08-12

## 产品与边界结论

- 现有 `/ai-chat`、会话、上传、Artifact、AppLayout、旧 Starter/Skill/Tool、全部租赁/IFRS 16 页面和 API 均保留；三个零售页面只增加独立 AI 入口，没有替换原按钮或重做排版；
- `retail_operations@v1` 只允许 Pulse、Store diagnostics、Retail scenario 三个 Read tool；所有经营数字由已验收确定性服务生成，来源链接、classification、dataset、as-of、window 和版本可追溯；
- 单店 evidence gate 只评估目标店，Store diagnostics 仍保留授权同群；region/brand/store scope 过滤后 provenance 只来自允许事实；
- diagnostics/scenario 失败或 decision-ready=false 时保留 failed/needs-review、stable reason、0.40 confidence，并禁止 Scenario/金额 proposal；
- `retail_action_proposal` 仍是 Assist Mode Artifact，不写 action/Forecast/Official/IFRS 16/合同/事件/分录；真正保存只能回到 MAX-007 情景工作台人工确认。

## Reviewer 独立验证

真实 Docker PostgreSQL：

```text
go test -v ./internal/agenttools/tools \
  -run '^TestRetailOperationsPostgresIsolationNoWrites$' \
  -count=2
```

两轮 PASS，约 3.62s / 3.29s。覆盖 fixed-seed Golden、实际 generator anomaly code、production/simulated、Pulse → diagnostics → labor -10% Scenario、A/B 法人、region/brand/store scope、固定 7 次 QueryFacts 和七张业务表零写入。结束后 `AGENT-LE-*` 法人、门店、facts、datasets 残留均为 0。

其他证据：

- Retail Tool 与 Agent 定向 Go tests PASS；
- `go vet ./...`、`git diff --check` PASS；
- Web 10 files / 58 tests、type-check、Next production build PASS；build 保留 `/ai-chat`、三个零售页及全部旧页面；
- 授权同群 Store360 等价测试、scope provenance、失败态/prompt injection/LLM fallback、proposal 零业务写入均有自动化覆盖。

## 发布边界

1440×900、390×844、键盘、控制台、真实点击链路和截图是 MAX-009 的核心验收，不再作为“外部环境可跳过”项；MAX-009 不修改现有 UI 架构，发现产品缺陷时按发布 blocker 最小返工。

MAX-008 正式接受，MAX-009 放行。
