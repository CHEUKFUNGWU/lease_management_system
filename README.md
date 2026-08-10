# Lease Management System

零售集团统一租赁管理平台。产品定位是：以 AI 录入为入口、以租赁全生命周期管理为日常、以 IFRS 16 合规计量为核心会计能力。系统覆盖合同台账、附件文档、关键日期提醒、条款义务、付款计划、范围判定、租赁计量、事件重算、月结分录、ERP 导出/回写、披露报表、审计留痕和 AI Agent 辅助录入。

## 当前状态

- MVP 与商业进阶路线图已落地。
- Docker Compose 默认运行 5 个基础服务：PostgreSQL、MinIO、Core Service、AI Service、Web；`worker` profile 可额外启动独立 `agent-runner`。
- 数据库初始化 schema 包含租赁业务表、AI Runtime 表、版本化 Agent Artifact/Evidence 表和草稿批次/幂等表，增量迁移位于 `db/migrations/`。
- IFRS 16 计量回归：22 个用例、148 条断言，自动生成对数报告。
- 注意：回归报告中的标准答案仍标记为 `pending_third_party_review`，正式审计背书需要第三方会计师复核。

## 核心能力

### Lease Administration

- 集中合同库：合同基础信息、门店/资产、出租方、承租方、标签、状态。
- 附件文档：主合同、补充协议、side letter 等文档元数据。
- 关键日期提醒：续租截止、break notice、租金 review、到期日、保险续保。
- 条款/义务管理：维修、CAM、保险、指数调整、恢复义务、押金、通知义务。
- 组合分析：按资产类型、区域/品牌、租赁范围查看合同组合与承诺租金。

### IFRS 16 Accounting

- 初始计量：租赁负债现值、使用权资产、初始直接成本、激励、恢复成本。
- 后续计量：利息摊销、折旧、付款冲减、负债滚动。
- 范围闸门：`in_scope` 资本化，`short_term_exempt` / `low_value_exempt` 直线法费用化，`not_a_lease` 跳过资本化。
- 会计区分：先付/后付租金、变量租金费用化、非租赁成分费用化。
- 事件驱动：modification、reassessment、impairment 等事件批准后触发重算。
- 月结闭环：生成计量结果和分录，复核、审批、过账、锁账/解锁。

### AI Intake And Agent

- AI 录入主入口：在 `/ai-chat` 上传合同或台账文件，自动解析并生成结构化草稿卡片。
- AI 文件解析：PDF/Excel/图片上传到 MinIO，PaddleOCR 优先，PyMuPDF fallback，LLM 抽取字段。
- Human-in-the-loop：AI 草稿必须人工确认后才能创建合同草稿，正式入库仍走审批。
- 折现率控制：AI 不得猜测折现率，缺失时标记 `discount_rate_missing`。
- 范围初判：AI 输出 `lease_scope`、豁免/排除原因和 scope confidence，低置信度人工复核。
- AI 问答：按权限检索合同、计量、分录、事件和报表上下文，返回引用来源。
- Agent Tool Runtime：Web、CLI 和 Pi-like Runner 共用 Tool Registry、Scope Guard、Review Gate、幂等和审计接缝。
- Skill Registry：合同台账、合同复核、租金表和审计包 Skill 均以版本化描述注册，并按角色过滤。
- `lease-agent` CLI：支持 Skill/Tool discovery、合同搜索/只读查询、Draft 命令、Capability 签发/撤销、Run Trace、worker lease 和机器可读退出码。
- `agent-runner`：独立 Pi-like Worker 进程，只通过 Agent Gateway 执行受控 Tool，并把 Run Event/Checkpoint/lease heartbeat 回写 Core；Worker 数据面按 `worker_id + lease_token` 绑定已领取 Run，SSE 控制流也沿用同一 lease 校验，不能读取或控制其他 Run；支持 `--worker-loop --plan <file>` 持续领取队列任务，旧 Gateway 无 SSE 时回退轮询。
- 生产 Worker：`docker compose --profile worker up -d --build agent-runner` 启动独立 Runner；使用 `--planner-url http://ai-service:8000` 调用 AI Service 生成受 Descriptor 约束的结构化 Tool plan，不挂载数据库或 MinIO 凭证。
- Tool Runtime 监控：`GET /api/v1/agent/metrics`（JSON）和 `GET /api/v1/agent/metrics/prometheus`（Prometheus，需 `agent_runtime:metrics` 或 `audit_logs:read`）提供低基数调用、失败、Review Gate 和延迟指标；`GET /api/v1/agent/usage` 与 Web `/agent-metrics` 提供按当前身份/法人范围的跨 Run Planner 用量汇总。Prometheus recording/alert rules 与最小权限 Token 采集模板见 [`ops/prometheus/README.md`](ops/prometheus/README.md)。AI Planner 返回的 `llm-usage.v1` 会以 `planner_usage` Run Event 写入统一 Trace；只有 token 完整且配置了 `LLM_PRICING_VERSION` 与价格时才计算成本，否则明确标记 unavailable。
- 认证刷新：登录返回 access/refresh token；Core 以哈希形式持久化 refresh session，轮换为一次性使用，并提供 `/api/v1/auth/refresh`、`/api/v1/auth/logout`、受保护的 `/api/v1/auth/logout-all` 和设备会话列表/撤销 API；CLI 提供 `lease-agent auth refresh`，Web API 在 401 时自动刷新并重试，过期 session 由 Core maintenance 按配置自动清理。
- 运维资料：Agent 健康检查、Prometheus recording/alert rules、最小权限 scrape 模板、LLM usage/cost 语义、发布 Smoke Run 和回滚边界见 [`docs/AI_Agent_运行运维手册.md`](docs/AI_Agent_运行运维手册.md) 与 [`ops/prometheus/README.md`](ops/prometheus/README.md)。
- 外部验收：生产监控、回滚演练、PaddleOCR、ERP、会计复核和 Worker 生产身份的责任人与证据要求见 [`docs/AI_Agent_外部验收清单.md`](docs/AI_Agent_外部验收清单.md)。

### Reporting And Integration

- Working / Official 双模式报表。
- 负债滚动表、合同汇总表、摊销表、现金流预测、标签汇总。
- ROI 测算页：估算传统 Excel 工时与系统处理工时差异。
- 敏感性分析：折现率冲击对负债和 ROU 的影响。
- 多准则对比：IFRS 16 / ASC 842 / 本地准则管理视角差异。
- ERP/总账：会计分录 CSV 导出 + ERP 凭证号回写。
- 审计日志：合同、事件、审批、月结、锁账等关键动作留痕。

## 技术栈

- **前端**: Next.js 14 + TypeScript + Ant Design
- **核心后端**: Go 1.23 + Gin
- **数据访问**: pgx（手写 SQL）
- **数据库**: PostgreSQL 16
- **AI / 文档解析服务**: Python 3.11 + FastAPI
- **OCR / 文档结构化**: PaddleOCR-VL-1.5（AI Studio 异步 API）/ PyMuPDF fallback
- **大模型能力**: DeepSeek API（默认）/ OpenAI API（备用）
- **对象存储**: MinIO
- **认证授权**: Go 自建 JWT + 基础 RBAC + 多租户行级过滤（`legal_entity_id`）；Agent Gateway 支持短时效、Run 绑定的 Capability Token
- **部署**: Docker Compose

## 项目结构

```text
lease_management_system/
├── db/
│   ├── init/                     # PostgreSQL 容器首次初始化 schema
│   └── migrations/               # 增量迁移 SQL
├── core-service/                 # Go + Gin 核心 API
│   ├── cmd/api/                  # API 服务入口
│   ├── cmd/ifrs16-regression/    # IFRS 16 回归报告生成命令
│   ├── cmd/lease-agent/           # 只调用 Agent Gateway 的 CLI Adapter
│   ├── cmd/agent-runner/           # Pi-like Worker 进程入口（无 DB/MinIO 凭证）
│   └── internal/
│       ├── agentartifact/         # Artifact/Evidence 协议
│       ├── agentskill/            # Skill Registry
│       ├── agentrunner/           # Pi-like Runner Adapter
│       ├── agenttools/            # Tool Registry/Runtime/Policy
│       ├── handlers/             # HTTP handlers
│       ├── middleware/            # JWT、tenant、CORS
│       ├── repository/            # pgx 数据访问层
│       └── services/              # audit、ifrs16 等业务服务
├── ai-service/                   # Python + FastAPI AI 服务
│   └── app/
│       ├── routers/              # files、parse、chat
│       └── services/             # OCR、LLM、storage、extractor
├── web/                          # Next.js 前端
│   └── app/
│       ├── ai-chat/              # AI 录入主入口
│       ├── contracts/            # 合同台账与详情
│       ├── monthly-closing/      # 月结跑批
│       ├── reports/              # 报表
│       ├── portfolio/            # 组合分析
│       ├── roi/                  # ROI 测算
│       ├── sensitivity/          # 敏感性分析
│       └── standards/            # 多准则对比
├── docs/                         # 需求、架构、进阶方案、回归报告、白皮书
├── scripts/                      # 辅助脚本
├── docker-compose.yml
├── Makefile
└── AGENTS.md
```

## 快速开始

### 1. 准备环境

需要安装：

- Docker / Docker Compose
- Make
- Go 1.23+
- Node.js 20+
- Python 3.11+

### 2. 配置环境变量

```bash
make setup
```

编辑 `.env`，重点配置：

- `DEEPSEEK_API_KEY`
- `OPENAI_API_KEY`（备用）
- `PADDLEOCR_ACCESS_TOKEN`（启用 PaddleOCR 时需要）

### 3. 启动服务

```bash
docker compose up -d --build
```

如需启动 Pi-like Worker，先将一个短时效、受限的 Gateway JWT 放入当前 shell 的 `AGENT_GATEWAY_TOKEN`，再执行：

```bash
export AGENT_CORE_URL="${AGENT_CORE_URL:-http://localhost:8080}"
export AGENT_GATEWAY_TOKEN="$(curl -fsS -X POST "$AGENT_CORE_URL/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d '{"username":"<worker-user>","password":"<password>"}' | jq -r '.token')"
docker compose --profile worker up -d --build agent-runner
```

本地如遇 8080/8081 已被占用，可在 `.env` 将 `CORE_PORT`/`AI_PORT` 改为可用宿主机端口；容器内部地址仍固定为 Core `8080`、AI `8000`。不要把 Worker JWT 写入 Git 或提交到 `.env.example`。

或：

```bash
make up
```

服务地址：

| 服务 | 地址 |
|------|------|
| Web | http://localhost:3000 |
| Core Service | http://localhost:8080 |
| AI Service | http://localhost:8081 |
| MinIO Console | http://localhost:9001 |
| PostgreSQL | localhost:5432 |

健康检查：

```bash
curl http://localhost:8080/health
curl http://localhost:8081/health
```

### 4. 测试账号

| 用户名 | 密码 | 角色 | 说明 |
|--------|------|------|------|
| `admin_user` | `password123` | admin | 管理员，跨租户 |
| `testuser` | `password123` | user | 普通测试用户 |
| `user_le001` | `password123` | user | LE001 法人 |
| `user_le002` | `password123` | user | LE002 法人 |

## 数据库说明

`db/init/01_init.sql` 只会在 PostgreSQL volume 首次为空时自动执行。已有旧 volume 时，新加字段不会自动补齐，需要执行增量迁移：

```bash
docker exec -i lease-postgres psql -U lease -d lease < db/migrations/005_lease_scope_gate.sql
docker exec -i lease-postgres psql -U lease -d lease < db/migrations/006_lease_admin_platform.sql
docker exec -i lease-postgres psql -U lease -d lease < db/migrations/007_obligations_portfolio.sql
docker exec -i lease-postgres psql -U lease -d lease < db/migrations/022_agent_draft_idempotency.sql
docker exec -i lease-postgres psql -U lease -d lease < db/migrations/023_agent_artifact_protocol.sql
docker exec -i lease-postgres psql -U lease -d lease < db/migrations/024_agent_draft_batches.sql
docker exec -i lease-postgres psql -U lease -d lease < db/migrations/025_agent_capability_revocations.sql
docker exec -i lease-postgres psql -U lease -d lease < db/migrations/026_event_draft_evidence.sql
docker exec -i lease-postgres psql -U lease -d lease < db/migrations/027_agent_run_checkpoints.sql
docker exec -i lease-postgres psql -U lease -d lease < db/migrations/028_agent_run_event_types.sql
docker exec -i lease-postgres psql -U lease -d lease < db/migrations/029_agent_run_worker_leases.sql
docker exec -i lease-postgres psql -U lease -d lease < db/migrations/030_auth_refresh_sessions.sql
docker exec -i lease-postgres psql -U lease -d lease < db/migrations/031_agent_run_audit_summaries.sql
docker exec -i lease-postgres psql -U lease -d lease < db/migrations/032_agent_run_checkpoint_audit_and_terminal_alerts.sql
docker exec -i lease-postgres psql -U lease -d lease < db/migrations/033_agent_run_audit_links.sql
```

如果要清空数据库并按最新 schema 重建：

```bash
make reset-db
make up
```

## 常用命令

```bash
make help                 # 查看命令
make up                   # 启动服务
make down                 # 停止服务
make restart              # 重启服务
make logs                 # 查看日志
make db                   # 进入 PostgreSQL
make web                  # 进入 Web 容器
make core                 # 进入 Core Service 容器
make ai                   # 进入 AI Service 容器
make ifrs16-regression    # 运行 IFRS 16 回归并生成报告
```

Agent Gateway / CLI：

```bash
cd core-service
go build -o /private/tmp/lease-agent ./cmd/lease-agent
/private/tmp/lease-agent skills --token "$LEASE_AGENT_TOKEN"
/private/tmp/lease-agent session create --token "$LEASE_AGENT_TOKEN" --title "CLI review"
/private/tmp/lease-agent tools --token "$LEASE_AGENT_TOKEN" --level read
/private/tmp/lease-agent contract search --token "$LEASE_AGENT_TOKEN" --search "上海" --status approved
/private/tmp/lease-agent contract get --token "$LEASE_AGENT_TOKEN" --id CONTRACT_ID
/private/tmp/lease-agent run create --token "$LEASE_AGENT_TOKEN" --session-id SESSION_ID --message "审阅合同"
/private/tmp/lease-agent run events --token "$LEASE_AGENT_TOKEN" --run-id RUN_ID
/private/tmp/lease-agent run trace --token "$LEASE_AGENT_TOKEN" --run-id RUN_ID
/private/tmp/lease-agent run claim --token "$LEASE_AGENT_TOKEN" --worker-id WORKER_ID
/private/tmp/lease-agent run heartbeat --token "$LEASE_AGENT_TOKEN" --worker-id WORKER_ID --run-id RUN_ID --lease-token LEASE_TOKEN
/private/tmp/lease-agent run release --token "$LEASE_AGENT_TOKEN" --worker-id WORKER_ID --run-id RUN_ID --lease-token LEASE_TOKEN
/private/tmp/lease-agent run steer --token "$LEASE_AGENT_TOKEN" --run-id RUN_ID --instruction "只关注租赁期限"
/private/tmp/lease-agent run branch --token "$LEASE_AGENT_TOKEN" --run-id RUN_ID --instruction "从 checkpoint 分支分析付款时点"
/private/tmp/lease-agent draft-batch get --token "$LEASE_AGENT_TOKEN" --id BATCH_ID
/private/tmp/lease-agent draft-batch retry --token "$LEASE_AGENT_TOKEN" --id BATCH_ID --artifact-id ARTIFACT_ID --input failed-items.json

# 受控 Pi-like Worker；plan.json 是 ToolCall 数组，生产环境可替换为受控 LLM Planner
go run ./cmd/agent-runner --token "$LEASE_AGENT_TOKEN" --message "审阅合同" --skill contract_review --plan plan.json

# 持续 Worker（plan 文件由受控 Planner/未来 Pi Planner 生成，Worker 不接触数据库）
go run ./cmd/agent-runner --token "$LEASE_AGENT_TOKEN" --worker-id lease-worker-01 --worker-loop --plan plan.json
```

核心 Gateway 路由：`GET /api/v1/agent/skills`、`GET /api/v1/agent/tools`、`POST /api/v1/agent/tools/execute`、`POST /api/v1/agent/capabilities`、`POST /api/v1/agent/capabilities/revoke`、`POST /api/v1/agent/sessions`、`POST /api/v1/agent/runs`、`GET/POST /api/v1/agent/runs/:id/events`、`GET/POST /api/v1/agent/runs/:id/checkpoint`、`GET /api/v1/agent/runs/:id/stream`、`POST /api/v1/agent/runs/:id/cancel`、`POST /api/v1/agent/runs/:id/steer`、`POST /api/v1/agent/runs/:id/follow-up` 和 `POST /api/v1/agent/runs/:id/branch`。终态告警为 `GET /api/v1/agent/alerts/terminal`、`POST /api/v1/agent/alerts/terminal/:id/ack`。草稿批次恢复路由为 `GET/POST /api/v1/ai/chat/draft-batches/:id`（POST 使用 `/retry`）。CLI/Pi-like Runner 不拥有数据库、MinIO 或任意 Shell 权限；Runner 到达终态后会请求撤销 Run Capability，并优先通过带 lease 的 Run Event SSE 消费取消/steer 控制事件，旧 Gateway 回退轮询，checkpoint 和 branch 由 Core Run 的 owner 校验保护。

## 验证命令

```bash
cd core-service
GOCACHE=$(pwd)/.gocache go test ./...

cd ../web
npm run type-check
npm run build

cd ..
make ifrs16-regression
PYTHONPYCACHEPREFIX=$(pwd)/.pycache python3 -m py_compile ai-service/app/routers/parse.py
```

最近一次完整验证通过：

- `go test ./...`
- `npm run type-check`
- `npm run build`
- `make ifrs16-regression`
- `python3 -m py_compile ai-service/app/routers/parse.py`

## 关键页面

| 页面 | 路径 | 说明 |
|------|------|------|
| 仪表板 | `/` | 合同统计、趋势、关键日期提醒 |
| AI 录入 | `/ai-chat` | 上传文件、自动解析、草稿卡片确认、AI 问答 |
| 合同台账 | `/contracts` | 合同列表、搜索、筛选、排序 |
| 新增合同 | `/contracts/new` | 手动创建合同草稿 |
| 传统上传 | `/upload` | 批量上传备用入口 |
| 月结跑批 | `/monthly-closing` | 生成、审批、过账、锁账、ERP 导出/回写 |
| 报表 | `/reports` | Working / Official 报表与 CSV 导出 |
| 组合分析 | `/portfolio` | 资产类型、范围、租金承诺与到期分布 |
| ROI 测算 | `/roi` | 人力工时与成本节省测算 |
| Agent 运营 | `/agent-metrics` | 跨 Run Planner 用量、Token 和成本状态（管理员/审计） |
| 敏感性分析 | `/sensitivity` | 折现率冲击分析 |
| 多准则对比 | `/standards` | IFRS 16 / ASC 842 / 本地准则对比 |
| 审计日志 | `/audit-logs` | 全链路操作留痕查询 |

## 开发约束

- 所有重大变更必须通过事件表处理，不得手工覆盖合同金额或日期。
- AI 识别结果不得直接写入正式台账，必须进入草稿层并人工确认。
- 正式入库、计量、月结过账仍由主系统规则、审批和锁账控制。
- AI 不得猜测折现率，缺失时必须触发 human-in-the-loop。
- 先付租金、后付租金、变量租金、非租赁成分必须严格区分。
- Working Report 可包含草稿/待审批数据；Official Report 仅包含已审批正式数据。

## 关键文档

- [AGENTS.md](AGENTS.md)
- [租赁平台进阶提升方案](docs/租赁平台进阶提升方案.md)
- [IFRS 16 计量回归对数报告](docs/IFRS16_计量回归对数报告.md)
- [IFRS 16 计量方法与准则映射白皮书](docs/IFRS16_计量方法与准则映射白皮书.md)
- [IFRS16 IT 需求文档](docs/IFRS16_IT_需求文档.md)
- [IFRS16 MVP 技术架构方案](docs/IFRS16_MVP_技术架构方案.md)

## License

Internal Use Only
