# Lease Management System

零售集团统一租赁管理平台。产品定位是：以 AI 录入为入口、以租赁全生命周期管理为日常、以 IFRS 16 合规计量为核心会计能力。系统覆盖合同台账、附件文档、关键日期提醒、条款义务、付款计划、范围判定、租赁计量、事件重算、月结分录、ERP 导出/回写、披露报表、审计留痕和 AI Agent 辅助录入。

## 当前状态

- MVP 与商业进阶路线图已落地。
- Docker Compose 运行 5 个服务：PostgreSQL、MinIO、Core Service、AI Service、Web。
- 数据库初始化 schema 包含 26 张表，增量迁移位于 `db/migrations/`。
- IFRS 16 计量回归：20 个用例、133 条断言，自动生成对数报告。
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
- **认证授权**: Go 自建 JWT + 基础 RBAC + 多租户行级过滤（`legal_entity_id`）
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
│   └── internal/
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
