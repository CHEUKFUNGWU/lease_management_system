# IFRS 16 租赁管理系统

零售集团 IFRS 16 租赁管理系统，覆盖合同管理、租赁计量、事件变更、会计分录、披露报表、审计留痕及 AI Agent 智能录入。

## 技术栈

- **前端**: Next.js + TypeScript + Ant Design
- **核心后端**: Golang + Gin
- **数据访问**: pgx + sqlc + goose
- **数据库**: PostgreSQL
- **AI / 文档解析服务**: Python + FastAPI
- **OCR / 文档结构化**: PaddleOCR
- **大模型能力**: OpenAI API
- **对象存储**: MinIO
- **认证授权**: Go 自建 JWT + 基础 RBAC
- **监控排障**: slog + pprof + 基础 metrics
- **部署**: Docker

## 项目结构

```
.
├── docker-compose.yml          # Docker 编排配置
├── .env.example                # 环境变量模板
├── Makefile                    # 常用命令
├── AGENTS.md                   # 项目指令与规范
├── README.md                   # 本文件
├── docs/                       # 需求与架构文档
│   ├── IFRS16_IT_需求文档.md
│   └── IFRS16_MVP_技术架构方案.md
├── web/                        # Next.js 前端
│   ├── app/                    # App Router
│   ├── Dockerfile
│   └── package.json
├── core-service/               # Go + Gin 核心服务
│   ├── cmd/api/                # 服务入口
│   ├── internal/               # 内部模块
│   ├── pkg/                    # 公共工具
│   ├── Dockerfile
│   └── go.mod
├── ai-service/                 # Python + FastAPI AI 服务
│   ├── app/                    # 应用模块
│   ├── main.py                 # 服务入口
│   ├── Dockerfile
│   └── requirements.txt
├── db/                         # 数据库迁移
│   └── migrations/             # goose 迁移文件
└── scripts/                    # 辅助脚本
```

## 快速开始

### 1. 环境准备

确保已安装：
- Docker & Docker Compose
- Make
- Go 1.22+
- Node.js 20+
- Python 3.11+

### 2. 初始化环境

```bash
make setup
```

编辑 `.env` 文件配置环境变量（特别是 `OPENAI_API_KEY`）。

### 3. 启动服务

```bash
make up
```

服务启动后：
- Web 前端: http://localhost:3000
- Core Service: http://localhost:8080
- AI Service: http://localhost:8081
- MinIO Console: http://localhost:9001
- PostgreSQL: localhost:5432

### 4. 常用命令

```bash
make help       # 查看所有可用命令
make up         # 启动所有服务
make down       # 停止所有服务
make restart    # 重启所有服务
make logs       # 查看日志
make db         # 进入 PostgreSQL 命令行
make migrate    # 执行数据库迁移
make web        # 进入前端容器
make core       # 进入核心服务容器
make ai         # 进入 AI 服务容器
```

## 核心业务链路

1. **新合同上传**: 用户上传 PDF/Excel → AI 识别 → 生成草稿 → 人工确认 → 正式入库 → 触发计量
2. **租金表上传**: 上传租金表 → AI 提取付款计划 → 批量确认 → 写入付款计划 → 触发重算
3. **AI 问答**: 聊天窗口提问 → 权限校验 → AI 基于台账回答 → 返回引用来源

## 开发规范

- 所有重大变更必须通过**事件表**处理，禁止直接修改合同金额或日期
- AI 识别结果**不得直接写入正式台账**，必须通过草稿层
- 所有计算结果必须可追溯到输入字段、参数版本和重算批次
- 先付租金与后付租金逻辑必须严格区分
- turnover rent / sales-based rent 必须当期费用化，不得资本化

## 文档

- [需求文档](docs/IFRS16_IT_需求文档.md)
- [MVP 技术架构方案](docs/IFRS16_MVP_技术架构方案.md)
- [开发规范](AGENTS.md)

## License

Internal Use Only
