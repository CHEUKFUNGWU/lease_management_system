# AI Agent 运行与发布运维手册

> 适用系统：线下零售经营分析工作站 / Retail Performance Workstation（IFRS 16 计量是其中一个合规模块）
>
> 适用组件：Core Agent Tool Runtime、AI Planner、`lease-agent` CLI、`agent-runner` Worker
>
> 文档状态：技术版；客户生产上线前仍需完成环境参数、ERP、PaddleOCR 和会计复核签署

## 1. 运行边界

Agent Worker 只能通过 Core Agent Gateway 调用注册 Tool。Worker 不挂载 PostgreSQL、MinIO 管理凭证，也不能访问任意 URL、SQL 或 Shell。

AI 默认运行在 Assist Mode。合同、付款计划和事件的 AI 结果只能进入 Draft/Artifact 层；审批、过账、锁账和 ERP 回写继续由既有权限与审批分离控制。

## 2. 健康检查与启动

```bash
docker compose up -d --build
curl -fsS "${CORE_URL:-http://localhost:8080}/health"
curl -fsS "${AI_URL:-http://localhost:8081}/health"
curl -fsS "${WEB_URL:-http://localhost:3000}/login" >/dev/null
```

启动 Worker 前，必须在当前 shell 注入短时效、最小权限的 `AGENT_GATEWAY_TOKEN`。不要把 JWT 写入 `.env`、Compose 文件或 CI 日志。

```bash
export AGENT_GATEWAY_TOKEN="..."
docker compose --profile worker up -d --build agent-runner
docker compose ps
```

本地宿主机端口可以通过 `.env` 的 `CORE_PORT`/`AI_PORT` 覆盖；容器内部 Core 与 AI 地址分别固定为 `http://core-service:8080` 和 `http://ai-service:8000`。

## 3. 监控接口

需要 `agent_runtime:metrics` 或 `audit_logs:read` 权限：

```bash
curl -fsS "$CORE_URL/api/v1/agent/metrics" \
  -H "Authorization: Bearer $ACCESS_TOKEN"

curl -fsS "$CORE_URL/api/v1/agent/metrics/prometheus" \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

当前指标为 Core 进程内低基数聚合，标签只允许注册 Tool 名和结果状态，不得加入用户、Run、合同或法人 ID。Prometheus recording rules、告警规则和带 secret Token 的采集模板见 [`ops/prometheus/README.md`](../ops/prometheus/README.md)。

关键指标：

| 指标 | 含义 | 建议处理 |
|---|---|---|
| `lease_agent_tool_executions_total` | Tool 尝试次数 | 观察流量与 Tool 分布 |
| `lease_agent_tool_failures_total` | 失败或拒绝次数 | 检查权限、参数、依赖和上游 |
| `lease_agent_tool_review_required_total` | 进入人工复核次数 | 不应被当作系统错误 |
| `lease_agent_tool_execution_duration_milliseconds_*` | Tool 延迟 | 分辨数据库慢、外部服务慢和模型慢 |
| `lease_agent_tool_cost_accounting_available` | 成本价目是否可用 | 为 `0` 时成本必须显示 unavailable |

## 4. LLM 使用量与成本

AI Planner 通过 `llm-usage.v1` 返回输入 token、输出 token、总 token、供应商、模型、价格版本和成本状态，并以 `planner_usage` Run Event 写入 Trace。

成本只有在以下条件同时满足时才可计算：

1. Provider 返回完整 token usage；
2. `LLM_PRICING_VERSION` 不是 `unconfigured`；
3. 输入和输出价格均为非负、已审批配置。

任一条件缺失都必须保留 `cost_status=unavailable`，不能用模型名称、历史价格或 token 数量猜测金额。AI Service 启动配置会拒绝空版本、负价格和半配置价格簿；正式环境应把价格版本纳入发布记录和审计资料。

## 5. 发布前验收

每次 Core、AI Service 或 Runner 发布至少执行：

```bash
cd core-service
GOCACHE="$(pwd)/.gocache" go test ./...
GOCACHE="$(pwd)/.gocache" go build -o /private/tmp/lease-agent ./cmd/lease-agent
GOCACHE="$(pwd)/.gocache" go build -o /private/tmp/agent-runner ./cmd/agent-runner

cd ../web
npm run type-check
npm test -- --run

cd ..
docker compose config -q
docker compose --profile worker config -q
git diff --check
```

发布后执行一次只读 Smoke Run：

1. 创建 Agent Session/Run；
2. 使用 `contract_review@v1` 只读取合同列表；
3. 确认 Trace 中存在 `planner_usage`、`tool_started`、`tool_completed`、`run_finished`；
4. 确认 Run 为 `completed`，Worker lease 已清空；
5. 确认没有写入合同、付款计划、事件或会计分录。

## 6. 回滚策略

### 应用回滚

1. 停止新 Worker，避免新版本继续领取队列：

   ```bash
   docker compose stop agent-runner
   ```

2. 将 Core、AI、Web 镜像回退到上一已验收版本；不要通过删除 PostgreSQL/MinIO volume 回滚。
3. 重启 Core/AI/Web，并确认 `/health`、Tool discovery 和只读 Smoke Run。
4. 执行过期 lease recovery；已领取但未完成的 Run 只能通过 lease recovery 重新排队，禁止人工直接改业务台账。

### 数据库迁移回滚

数据库迁移默认采用向前兼容策略。已写入的 Run Event、Artifact、审计和 refresh session 不做破坏性删除。若迁移需要回滚，必须先停止写入流量、备份数据库、由 DBA 执行对应的反向 SQL，并重新跑审计/计量回归；禁止使用 `docker compose down -v` 作为回滚手段。

## 7. 上线前外部确认

- 真实 PaddleOCR 样本与扫描件坐标证据覆盖率；
- 客户 ERP 字段映射、凭证回写和失败重试；
- 生产 LLM 价格表、成本基线和 Prometheus 告警接收人；
- Worker 专用账号、最小权限和 Token 轮换策略；
- 第三方会计师对 IFRS 16 回归标准答案和正式审计背书的复核；
- 一次经过批准的发布/回滚演练记录。
