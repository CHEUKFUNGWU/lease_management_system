# AGENTS.md — IFRS 16 租赁管理系统

> 零售集团 IFRS 16 租赁管理系统。覆盖合同管理、租赁计量、事件变更、会计分录、披露报表、审计留痕及 AI Agent 智能录入。

## 当前状态

- 需求阶段已完成（见 `IFRS16_IT_需求文档.md`）
- MVP 技术架构方案已确定（见 `IFRS16_MVP_技术架构方案.md`）
- **项目已初始化**，5 个 Docker 容器正常运行
- 数据库 17 张表 + 种子数据已部署
- 阶段二、三、六部分功能已实现，**聚焦交互、计算逻辑、管理作用**

## 技术栈

- **前端**: Next.js 14 + TypeScript + Ant Design
- **核心后端**: Go 1.23 + Gin
- **数据访问**: pgx（手写 SQL，未用 sqlc/goose）
- **数据库**: PostgreSQL 16
- **AI / 文档解析服务**: Python 3.11 + FastAPI
- **OCR / 文档结构化**: PaddleOCR-VL-1.5（AI Studio 异步 API，multipart form data 方式提交）
- **大模型能力**: DeepSeek API（deepseek-v4-flash，默认）/ OpenAI API（备用）
- **对象存储**: MinIO
- **认证授权**: Go 自建 JWT + 基础 RBAC + 多租户行级过滤（legal_entity_id）
- **部署**: Docker Compose

## 已完成功能

### 基础设施
- Docker Compose 5 容器：PostgreSQL、MinIO、Core Service、AI Service、Web
- 数据库迁移：`db/init/01_init.sql`（合并不带 goose 标记，用于 PostgreSQL 自动初始化）
- 修复了 8 个部署 BUG（PostgreSQL 连接、ARM OCR 兼容、Next.js 版本、Docker 卷挂载覆盖等）

### 用户与权限
- JWT 认证 + 6 种角色（admin/editor/reviewer/approver/auditor/readonly）
- 多租户隔离：JWT 含 `legal_entity_id`，TenantMiddleware 自动注入，所有查询按法人过滤
- 公开注册已关闭，仅 admin 可通过 `/api/v1/admin/users` 创建用户
- Admin 管理后台：`/admin/login`（深色主题）+ `/admin/users`（用户 CRUD）

### 合同管理
- 合同 CRUD API（创建/列表/详情/更新）
- 合同详情页：4 个标签（信息/付款计划/变更事件/IFRS 16 计算摊销表）
- 审批工作流：submit/review/approve/reject（前端已接入，角色条件渲染）
- 合同列表搜索/筛选/排序（合同编号/名称/承租方/出租方/门店搜索 + 审批状态筛选 + 排序）
- 新增合同页面（手动填表，无 AI 草稿关联）

### 付款计划
- 创建付款计划 API（嵌套在合同下 `/contracts/:id/payment-schedules`）
- AI 租金表解析：上传 → MinIO → PaddleOCR/PyMuPDF → LLM 解析 → 草稿表格 → 逐条编辑/跳过/确认 → 批量导入
- 先付/后付正确区分

### 事件驱动
- 7 种事件类型 API（创建/列表，嵌套在合同下）
- 前端事件登记表单 + 审批流程（提交复核/复核通过/审批通过/驳回/重新提交）

### IFRS 16 计量引擎
- 初始/后续计量：租赁负债现值、使用权资产、利息摊销、折旧
- 36 个月完整摊销表（验证通过：初始负债 ¥3,255,676.79）
- 月结跑批：计量结果表 + 会计分录表 + 批次表 + 前端四 Tab（生成/分录预览/批次历史/锁账控制）
- 月结审批/过账工作流：分录草稿 → 审批 → 过账（支持单条和批量）
- 期间锁账控制：锁账/解锁 + 已锁账期间禁止重新生成
- 月结 2024-01 验证：3 笔分录（利息 ¥13,318、折旧 ¥92,170、付款 ¥50,000）

### 报表
- 报表双模式 API + 前端：Working/Official 切换 + CSV 导出
- 负债滚动表、合同汇总表

### 审计日志
- 全链路审计记录：合同/事件/审批/月结/锁账操作均写入 audit_logs
- 审计查询 API：支持表名/操作类型/记录ID/操作人/时间范围筛选
- 审计日志页面：JSON diff 展开、分页、重置筛选

### 仪表板
- 合同统计实时数据：总数/已审批/待处理/草稿（后端数据绑定）

### AI 文件解析（PaddleOCR + LLM）
- 端到端链路：上传 PDF/Excel → MinIO → PaddleOCR multipart 提交 → 轮询结果 → JSON 解析 → LLM 字段抽取 → 草稿生成
- 合同解析：自动下载 MinIO 文件 → PaddleOCR 提取文本 → DeepSeek LLM 抽取字段 + 置信度评分
- 付款计划解析：Excel/PDF 租金表 → 提取期间/金额/先付后付 → 批量导入
- 货币缺失检测 + 折现率缺失检测（AI 不得猜测，触发 human-in-the-loop）
- PyMuPDF fallback：PaddleOCR 不可用时自动切换

### AI Agent 聊天
- 聊天窗口 UI
- Core Service 关键词匹配 stub（**非真实 LLM 调用**）

## 待办事项

### P0 — 系统基本可用性

| # | 事项 | 状态 | 说明 |
|---|------|------|------|
| 1 | **审批工作流 UI** | ✅ 已完成 | 合同详情页接入 submit/review/approve/reject 按钮，角色条件渲染 |
| 2 | **AI 合同草稿入库链路** | ✅ 已完成 | 批量上传支持 AI 解析 → 草稿确认 → 批量创建合同 |
| 3 | **合同列表搜索/筛选/排序** | ✅ 已完成 | 支持合同编号/名称/承租方/出租方/门店搜索 + 审批状态筛选 + 排序 |

### P1 — 管理闭环

| # | 事项 | 状态 | 说明 |
|---|------|------|------|
| 4 | **月结审批/过账工作流** | ✅ 已完成 | 单条/批量审批 + 过账 + ERP 凭证号 + 期间锁账/解锁 |
| 5 | **付款计划草稿确认增强** | ✅ 已完成 | AI 草稿支持逐条编辑/跳过/确认/全选确认，仅导入已确认行 |
| 6 | **仪表板实时数据** | ✅ 已完成 | 硬编码数字 → 后端数据绑定 |

### P2 — 功能完善

| # | 事项 | 状态 | 说明 |
|---|------|------|------|
| 7 | **合同编辑/更新 API + UI** | ✅ 已完成 | 支持 draft/rejected 状态下编辑合同基本信息 |
| 8 | **事件审批流程** | ✅ 已完成 | 事件支持提交复核/复核通过/退回/审批通过/驳回/重新提交 |
| 9 | **IFRS 16 计算增强** | ⏳ 待办 | 先付租金首期处理、变量租金费用化 |

### P3 — 后续迭代

| # | 事项 | 状态 | 说明 |
|---|------|------|------|
| 10 | Modification/Reassessment 事件计算逻辑 | ✅ 已完成 | 事件自动分类（modification/reassessment/impairment）、批准后触发重算、生成 event_adjustments 记录、创建会计分录、前端支持预览影响和查看调整 |
| 11 | 审计日志查询 API + UI | ✅ 已完成 | 全链路审计记录（合同/事件/审批/月结操作）+ 查询页面（表名/操作类型/记录ID/时间范围筛选 + JSON diff 展开） |
| 12 | AI 聊天增强（真实 LLM 调用 + 权限过滤 + 引用来源） | ✅ 已完成 | Core Service 检索权限范围数据 → 构建系统提示 → AI Service 调用 DeepSeek/OpenAI → 返回带引用来源的回答 |

## 领域核心约束（必须遵守）

### 数据模型
- 以 **合同 + 门店/资产 + 付款计划 + 事件** 为核心数据模型
- 会计引擎与业务录入分离，业务变化必须通过**事件驱动重算**
- 所有计算结果必须可追溯到输入字段、参数版本和重算批次

### 关键会计区分（常见错误点）
- **先付租金 vs 后付租金**：先付租金（如 commencement date 当天支付）通常不形成未来融资成本，作为已支付租赁付款额影响使用权资产初始成本；后付租金纳入普通贴现及后续利息摊销
- **固定租金 vs 变量租金**：turnover rent / sales-based rent 必须**当期费用化**，不得资本化计入租赁负债
- **租赁成分 vs 非租赁成分**：CAM、管理费、服务费、清洁费、保安费、维修费、税费需按政策拆分或适用 practical expedient

### 事件驱动架构（禁止手工改表）
- 所有重大变更必须通过**事件表**处理，不得直接修改合同金额或日期
- 事件类型至少包括：新合同录入、新店开业、租赁开始、合同续签、提前终止、面积调整、固定租金变更、指数更新、续租/终止选择权判断变化、lease modification、reassessment、index/rate change、闭店、转租开始/结束、减值触发、恢复成本估计变化
- modification（范围/对价变化）与 reassessment（选择权判断变化导致租期变化）有独立的会计处理逻辑

### 月结与锁账
- 支持按法人、期间、区域、品牌批量跑批
- 分录预览 → 复核 → 审批 → 过账 → 总账回写过账状态
- 严格的锁账控制和重开期间控制，已关账期间不得覆盖

### AI Agent 约束
- AI 识别结果**不得直接写入正式台账**，必须通过草稿层（合同草稿、事件草稿、付款计划草稿）
- 必须包含字段级置信度评分、低置信度人工必审、原文定位高亮、差异留痕
- AI 模块仅为录入前台和辅助分析入口，正式入库和计量仍由主系统规则控制
- AI 问答必须基于权限范围检索，展示引用来源，区分"系统正式数据"与"AI 推断/建议"
- **AI 默认运行在 Assist Mode**：负责识别、建议、草稿生成，正式入库需人工审批
- Auto-Post Mode 需另行定义适用范围和阈值

### 报表双模式
- **Working Report**：可包含 Draft / Pending Approval 数据，用于内部试算和测试
- **Official Report**：仅包含 Approved 数据，用于正式财务和审计
- 报表需显示 approval_status、is_official_version、生成时间和版本
- 导出文件应标注 Draft / Pending Approval / Official

### Discount Rate 人机协同
- AI **不得猜测折现率**，缺失时必须触发 human-in-the-loop
- 处理顺序：检查合同文本 → 系统政策库匹配 → 无法唯一确定则人工确认
- 缺少 discount rate 的合同标记为 `discount_rate_missing = true`
- 需记录折现率来源、确认人、确认时间

## 角色与权限模型

### 系统角色（6个）

| 角色 | 代码 | 主要职责 |
|------|------|----------|
| System Admin | `admin` | 用户/角色管理、主数据配置、系统参数 |
| Finance Editor | `editor` | 上传合同、维护草稿、录入台账 |
| Finance Reviewer | `reviewer` | 复核合同/付款计划/事件草稿 |
| Finance Approver | `approver` | 审批正式入库、关键会计处理 |
| Auditor Readonly | `auditor` | 只读查看、导出审计资料 |
| Business Readonly | `readonly` | 查看授权范围内合同和报表 |

### 数据权限维度

- 法人 (legal_entity)
- 门店 (store)
- 区域 (region)
- 品牌 (brand)

### 审批流程

```
Agent 生成草稿 → Editor 修改确认 → Reviewer 复核 → Approver 审批 → 正式入库
```

- MVP 阶段 Editor 和 Reviewer 可为同一人
- 未批准数据可保留计算，但需区分 Working Report / Official Report

### 关键动作日志

新增、修改、删除、导入、导出、重算、审批、锁账/解锁

## 风险红线（设计时必须规避）

1. **会计政策未先统一**：续租/终止判断标准、CAM 拆分政策、turnover rent 口径、折现率政策、闭店减值触发口径必须在开发前固化
2. **合同数据质量不足**：关键日期缺失、付款计划不完整、先付/后付未明确、非租赁成分未拆分
3. **变更做成手工覆盖**：必须通过事件驱动，否则历史不可追溯、审计无法复演
4. **忽视先付与后付逻辑差异**：将直接导致初始租赁负债、使用权资产、利息摊销表、首期分录错误
5. **忽视变量租金和指数调整逻辑**：不应资本化的变量租金被计入租赁负债，需要重估的指数租金未正确更新
6. **接口和锁账控制不足**：台账与总账不一致、付款与负债滚动不一致、已关账期间被覆盖
7. **过度依赖手工调整**：优先保证标准场景自动化、高频场景批量化、例外场景可审批可追溯
8. **AI 识别结果直接入正式台账**：必须建立"AI 草稿层"，强制执行人工确认和审批控制
9. **OCR 和扫描件识别精度不足**：需低置信度标识、原文定位高亮、字段级人工修正、无法识别场景的人工转录兜底
10. **AI 问答越权或引用错误**：必须基于权限范围检索、展示引用来源、区分系统正式数据与 AI 推断

## 文件与数据规范

### 合同头关键字段
合同编号、名称、法人主体、门店编号/名称、出租方、资产/物业类别、币种、签约日期、commencement date、lease start date、lease end date、原始不可撤销期、续租/终止选择权描述及判断结果、折现率类型/版本、合同状态

### 付款计划关键字段
付款计划编号、合同编号、生效起止日、覆盖期间起止日、应付日期、实际支付日期、付款时点（先付/后付）、金额/币种/税额、金额类型（固定/指数调整/变量/服务费/押金/税金）、会计属性（租赁/非租赁/变量/税费）、是否进入负债现值计算

### 事件关键字段
事件编号、合同编号、事件类型、生效日期、申请/审批日期、原值/新值、变更原因、判断依据、事件状态、重算批次号

### 计量结果关键字段
合同编号、会计期间、期初/期末租赁负债、本期新增/利息/付款/重估调整、期初/期末使用权资产、本期新增/折旧/减值/终止处置

### 审计追溯字段
数据版本号、合同版本号、利率版本号、计算规则版本号、导入批次号、创建/更新人及时间、审批人、附件索引

### AI 识别与入库字段
AI 任务编号、文件编号/类型/哈希、上传人/时间、识别/OCR 状态、文档分类结果、识别出的合同/门店/事件、字段名称、AI 提取值/最终确认值、置信度评分、原文页码/位置坐标、差异原因、审核人/时间、入库结果状态

## MVP 架构分层

- **Web 前端** (Next.js + TypeScript + Ant Design): 合同台账、文件上传、AI Agent 聊天窗口、草稿确认、基础查询
- **Core Service** (Go + Gin): 用户登录与权限、合同主数据、付款计划、事件管理、IFRS 16 基础计量、草稿转正、审计日志
- **AI Service** (Python + FastAPI): 文件解析请求、PaddleOCR、PDF/Excel/scan copy 解析、LLM 字段抽取、草稿生成、置信度与原文定位
- **PostgreSQL**: 正式业务数据、AI 草稿数据、任务状态、审核记录、系统日志索引
- **MinIO**: 原始上传文件、合同附件、OCR 中间产物、解析后的文本和结构化结果文件

## 目录结构

```
ifrs16_management_system/
├── db/init/                     # 数据库初始化 SQL（01_init.sql + migrations）
├── core-service/                # Go + Gin 核心 API
│   ├── cmd/api/main.go          # 路由注册（含 middleware）
│   └── internal/
│       ├── handlers/            # HTTP handlers（auth, contract, approval, payment_schedule, event, calculation, monthly_closing, reports, discount_rate, ai_chat, user, role）
│       ├── middleware/           # auth.go（JWT）, tenant.go（多租户过滤）, cors.go
│       ├── repository/          # 数据访问层（contract, payment_schedule, event, calculation, monthly_closing, user, role, report, audit）
│       ├── models/              # 数据模型
│       └── services/            # 业务逻辑层
├── ai-service/                  # Python + FastAPI AI 服务
│   └── app/
│       ├── routers/             # files.py（上传）, parse.py（合同/付款计划解析）, chat.py（聊天）
│       ├── services/
│       │   ├── paddleocr.py     # PaddleOCR 异步 API 客户端（multipart form data 方式）
│       │   ├── document_extractor.py  # 文档文本提取（PaddleOCR 优先 + PyMuPDF fallback）
│       │   ├── llm.py           # DeepSeek/OpenAI LLM 客户端
│       │   └── storage.py       # MinIO 上传/下载/预签名 URL
│       └── config.py            # 环境变量配置
├── web/                         # Next.js 前端
│   └── app/
│       ├── page.tsx             # 仪表板
│       ├── login/               # 用户登录
│       ├── admin/               # Admin 管理后台（login + users）
│       ├── contracts/           # 合同列表 / 新增 / [id]详情（4 Tab）
│       ├── upload/              # 文件上传
│       ├── ai-chat/             # AI 助手聊天
│       ├── reports/             # 报表查询
│       ├── monthly-closing/     # 月结跑批
│       ├── settings/            # 系统设置
│       ├── components/          # AppLayout 等公共组件
│       ├── context/             # AuthContext（含 legal_entity_id）
│       └── lib/api.ts           # API 客户端（含 adminApi）
├── docker-compose.yml
├── .env                         # 环境变量（含 PADDLEOCR_ACCESS_TOKEN）
└── AGENTS.md
```

## 快速开始

```bash
# 启动所有服务
docker compose up -d --build

# 查看服务状态
docker compose ps

# 查看 AI Service 日志
docker logs ifrs16-ai --tail 20

# 查看 Core Service 日志
docker logs ifrs16-core --tail 20

# 重建单个服务
docker compose build ai-service && docker compose up -d ai-service
```

### 服务端口

| 服务 | 端口 | 说明 |
|------|------|------|
| Web | 3000 | Next.js 前端 |
| Core Service | 8080 | Go API |
| AI Service | 8081 | Python API |
| PostgreSQL | 5432 | 数据库 |
| MinIO | 9000/9001 | 对象存储 API / 控制台 |

### 测试账号

| 用户名 | 密码 | 角色 | 说明 |
|--------|------|------|------|
| `testuser` | `password123` | user | 普通用户 |
| `admin_user` | `password123` | admin | 管理员（跨租户） |
| `user_le001` | `password123` | user | LE001 法人 |
| `user_le002` | `password123` | user | LE002 法人 |

### 测试数据

- 测试合同 ID：`5adea424-3983-43e0-abf3-660c6f522bbe`（南京东路旗舰店，LEASE-2024-001）
- 法人种子数据：LE001（零售集团总公司）、LE002（零售集团上海公司）
- IFRS 16 计算：初始负债 ¥3,255,676.79，36 个月摊销
- 月结 2024-01：3 笔分录（利息 ¥13,318、折旧 ¥92,170、付款 ¥50,000）

## 关键设计决策

| 决策 | 原因 |
|------|------|
| `db/init/01_init.sql` 合并所有迁移（无 goose 标记） | PostgreSQL 容器自动初始化更简单 |
| Docker Compose 移除 `volumes: ./xxx:/app` 挂载 | 避免覆盖构建产物 |
| PaddleOCR 用 multipart form data 而非 base64 JSON | base64 JSON 方式返回 500，multipart 正常 |
| PaddleOCR 结果只解析 `jsonUrl`（无 `markdownUrl`） | API 当前只返回 `jsonUrl`，从 `result.layoutParsingResults[].markdown.text` 提取 |
| 多租户用 `legal_entity_id` 行级过滤 | MVP 更简单可控，admin 空 legal_entity_id 不加过滤 |
| 公开注册关闭 | 内部系统，管理员通过 `/api/v1/admin/users` 创建账号 |
| PaddleOCR 客户端全局单例 | 避免重复初始化 |

## 关键集成接口

- **ERP/总账**：输出 IFRS 16 会计分录，回传凭证号和过账状态，支持冲销/重过账
- **AP/付款系统**：获取实际付款信息，比对差异，识别预付/欠付/日期偏差
- **门店主数据**：获取门店新增、闭店、区域/品牌调整
- **销售系统**：获取门店销售额，支持 turnover rent 自动计算
- **固定资产/工程系统**：获取装修完工、恢复义务、减值线索
- **审批与身份系统**：单点登录、组织架构同步、审批流程集成

## 更新记录

- 2026-05-12: 基于 IFRS16_IT_需求文档.md 创建初始 AGENTS.md
- 2026-05-12: 基于 IFRS16_MVP_技术架构方案.md 更新技术栈、MVP 架构分层、核心业务链路、数据表建议
- 2026-05-13: 全面更新 — 反映实际开发进度、待办事项、目录结构、快速开始、设计决策、PaddleOCR 集成详情
- 2026-05-14: 完成 P0/P1 管理闭环 — 审批工作流 UI、合同列表搜索筛选排序、合同编辑更新、事件审批流程、付款计划草稿确认增强、月结审批过账锁账工作流
- 2026-05-14: 完成 P2/P3 核心计算与管理 — IFRS 16 计算增强（先付租金、变量租金）、Modification/Reassessment 事件重算逻辑、审计日志全链路记录与查询
- 2026-05-14: 完成 P3 AI 聊天增强 — Core Service 权限范围数据检索 + 系统提示构建 + AI Service DeepSeek/OpenAI LLM 调用 + 引用来源展示