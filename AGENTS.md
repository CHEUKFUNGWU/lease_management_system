# AGENTS.md — 租赁管理系统

> 零售集团租赁管理系统。覆盖合同管理、租赁计量、事件变更、会计分录、披露报表、审计留痕及 AI Agent 智能录入。

## 当前状态

- 需求阶段已完成（见 `IFRS16_IT_需求文档.md`）
- MVP 技术架构方案已确定（见 `IFRS16_MVP_技术架构方案.md`）
- 商业进阶路线图已落地（见 `docs/租赁平台进阶提升方案.md`）
- **项目已初始化**，5 个 Docker 容器正常运行
- 数据库 26 张表 + 种子数据已部署，`db/init/01_init.sql` 作为容器自动初始化主 schema
- MVP + 进阶能力已实现：正确性证明、范围闸门、租赁全生命周期管理、AI 对话式录入、ROI、ERP 导出/回写、组合分析、多准则对比、敏感性分析
- 计量回归报告：20 个用例 / 133 条断言全部通过（见 `docs/IFRS16_计量回归对数报告.md`）
- 注意：回归用例当前标记为 `pending_third_party_review`，仍需第三方会计师线下复核后才能作为正式审计背书

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
- 合同详情页：信息、付款计划、变更事件、IFRS 16 计算摊销表、关键日期、附件文档、条款/义务等管理视图
- 审批工作流：submit/review/approve/reject（前端已接入，角色条件渲染）
- 合同列表搜索/筛选/排序（合同编号/名称/承租方/出租方/门店搜索 + 审批状态筛选 + 排序）
- 新增合同页面支持手动录入合同基本信息、资产类型、租赁范围、豁免原因、折现率来源等字段
- 租赁全生命周期管理：合同附件、关键日期提醒、条款/义务、资产类型维度

### 付款计划
- 创建付款计划 API（嵌套在合同下 `/contracts/:id/payment-schedules`）
- AI 租金表解析：上传 → MinIO → PaddleOCR/PyMuPDF → LLM 解析 → 草稿表格 → 逐条编辑/跳过/确认 → 批量导入
- 先付/后付正确区分
- 固定租金、变量租金、非租赁成分按会计属性区分，变量租金与非租赁成分不资本化

### 事件驱动
- 7 种事件类型 API（创建/列表，嵌套在合同下）
- 前端事件登记表单 + 审批流程（提交复核/复核通过/审批通过/驳回/重新提交）
- Modification / reassessment / impairment 自动分类，批准后触发重算、生成 event_adjustments、创建影响分录并支持前端预览

### IFRS 16 计量引擎
- 初始/后续计量：租赁负债现值、使用权资产、利息摊销、折旧
- 36 个月完整摊销表（验证通过：初始负债 ¥3,255,676.79）
- 先付租金首期处理：首期付款不形成未来融资成本，进入 ROU 初始成本
- 变量租金费用化、非租赁成分费用化，避免错误资本化
- 租赁范围闸门：`in_scope` 资本化；`short_term_exempt` / `low_value_exempt` 直线法费用化；`not_a_lease` 跳过资本化
- 自动化回归测试：20 个典型场景、133 条断言、一键生成对数报告
- 准则映射白皮书：计量方法与 IFRS 16 条款对应关系
- 月结跑批：计量结果表 + 会计分录表 + 批次表 + 前端四 Tab（生成/分录预览/批次历史/锁账控制）
- 月结审批/过账工作流：分录草稿 → 审批 → 过账（支持单条和批量）
- 期间锁账控制：锁账/解锁 + 已锁账期间禁止重新生成
- 月结 2024-01 验证：3 笔分录（利息 ¥13,318、折旧 ¥92,170、付款 ¥50,000）

### 报表
- 报表双模式 API + 前端：Working/Official 切换 + CSV 导出
- 负债滚动表、合同汇总表
- 组合分析：按资产类型、区域/品牌、租赁范围展示合同组合、承诺租金与到期分布
- 敏感性分析：折现率冲击对负债和 ROU 的影响
- 多准则对比：IFRS 16 / ASC 842 / 本地准则管理视角差异展示
- 现金流预测、标签汇总、摊销表 API

### 审计日志
- 全链路审计记录：合同/事件/审批/月结/锁账操作均写入 audit_logs
- 审计查询 API：支持表名/操作类型/记录ID/操作人/时间范围筛选
- 审计日志页面：JSON diff 展开、分页、重置筛选

### 仪表板
- 合同统计实时数据：总数/已审批/待处理/草稿（后端数据绑定）
- 关键日期提醒、趋势图和合同状态图基于真实数据或空状态展示，移除硬编码演示数字

### ERP / 总账集成
- 会计分录 CSV 导出：`GET /api/v1/monthly-closing/entries/export`
- ERP 凭证号回写：`POST /api/v1/monthly-closing/erp-writeback`
- 月结页面支持按期间导出、凭证回写、展示 ERP reference / voucher 信息

### AI 文件解析（PaddleOCR + LLM）
- 端到端链路：上传 PDF/Excel → MinIO → PaddleOCR multipart 提交 → 轮询结果 → JSON 解析 → LLM 字段抽取 → 草稿生成
- 合同解析：自动下载 MinIO 文件 → PaddleOCR 提取文本 → DeepSeek LLM 抽取字段 + 置信度评分
- 付款计划解析：Excel/PDF 租金表 → 提取期间/金额/先付后付 → 批量导入
- 货币缺失检测 + 折现率缺失检测（AI 不得猜测，触发 human-in-the-loop）
- 租赁范围 AI 初判：输出是否租赁、建议 `lease_scope`、豁免/排除原因、scope confidence，低置信度人工复核
- PyMuPDF fallback：PaddleOCR 不可用时自动切换

### AI Agent 聊天
- 聊天窗口 UI
- Core Service 检索权限范围内合同、计量、分录、事件、报表上下文
- AI Service 调用 DeepSeek / OpenAI 生成回答，返回引用来源并区分正式数据与 AI 建议
- 对话式录入主入口：在 `/ai-chat` 上传文件后自动解析，生成结构化合同草稿卡片，人工确认后批量创建草稿合同
- 保留 `/upload` 传统批量上传路径作为高级/备用入口

### ROI 与商业化展示
- ROI 测算页 `/roi`：按合同数量、人力成本、传统处理工时、本系统处理工时估算节省
- 用于售前说明 "Excel 工时 vs 平台工时"、审计返工风险下降、月结效率提升

## 进阶路线图落地状态

### MVP 管理闭环

| # | 事项 | 状态 | 说明 |
|---|------|------|------|
| 1 | 审批工作流 UI | ✅ 已完成 | 合同详情页接入 submit/review/approve/reject 按钮，角色条件渲染 |
| 2 | AI 合同草稿入库链路 | ✅ 已完成 | AI 解析 → 草稿确认 → 批量创建合同，正式入库仍走审批 |
| 3 | 合同列表搜索/筛选/排序 | ✅ 已完成 | 合同编号/名称/承租方/出租方/门店搜索 + 状态筛选 + 排序 |
| 4 | 月结审批/过账工作流 | ✅ 已完成 | 单条/批量审批 + 过账 + ERP 凭证号 + 期间锁账/解锁 |
| 5 | 付款计划草稿确认增强 | ✅ 已完成 | AI 草稿支持逐条编辑/跳过/确认/全选确认，仅导入已确认行 |
| 6 | 仪表板实时数据 | ✅ 已完成 | 硬编码数字 → 后端数据或空状态 |
| 7 | 合同编辑/更新 API + UI | ✅ 已完成 | 支持 draft/rejected 状态编辑合同基本信息 |
| 8 | 事件审批流程 | ✅ 已完成 | 提交复核/复核通过/退回/审批通过/驳回/重新提交 |
| 9 | IFRS 16 计算增强 | ✅ 已完成 | 先付租金首期处理、变量租金费用化、非租赁成分费用化 |

### 商业进阶路线图

| 优先级 | 主题 | 状态 | 关键产出 |
|--------|------|------|----------|
| P0 | 正确性证明 | ✅ 已完成 | 20 个回归测试用例、133 条断言、自动化对数报告、准则映射白皮书 |
| P0 | 范围判定与分流 | ✅ 已完成 | `lease_scope` 字段、AI 初判、计量引擎分流、豁免费用化、披露分列 |
| P1 | 平台化第一步 | ✅ 已完成 | 集中合同库、附件文档、关键日期提醒、`asset_type`、AI 录入 |
| P1 | AI 对话式入口第一步 | ✅ 已完成 | `/ai-chat` 上传文件 → 自动解析 → 结构化草稿卡片 → 人工确认入库 |
| P1 | ROI 量化 | ✅ 已完成 | `/roi` ROI 测算页 |
| P2 | 平台化第二步 | ✅ 已完成 | 条款/义务管理、组合分析、设备租赁维度 |
| P2 | ERP/总账集成 | ✅ 已完成 | 分录 CSV 导出 + ERP 凭证号回写 |
| P2 | 对话入口默认化 | ✅ 已完成 | 侧边栏前置 `AI 录入`，传统 `/upload` 降为备用路径 |
| P3 | 多准则对比 | ✅ 已完成 | `/standards` IFRS 16 / ASC 842 / 本地准则管理视角对比 |
| P3 | 敏感性与预测 | ✅ 已完成 | `/sensitivity` 折现率冲击分析、现金流预测接口 |

### 尚需外部完成

| 事项 | 状态 | 说明 |
|------|------|------|
| 第三方会计复核 | ⏳ 外部待办 | 回归报告中的标准答案已可自动对数，但需第三方会计师复核并签字后才能作为正式审计背书 |
| 真实客户 ERP 联调 | ⏳ 外部待办 | 系统已有导出/回写链路；真实客户环境仍需按用友/金蝶/SAP/Oracle 等目标系统做字段映射和联调 |

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
合同编号、名称、法人主体、门店编号/名称、出租方、资产/物业类别、`asset_type`、币种、签约日期、commencement date、lease start date、lease end date、原始不可撤销期、续租/终止选择权描述及判断结果、折现率类型/版本、`lease_scope`、豁免/排除原因、scope 来源与置信度、合同状态

### 付款计划关键字段
付款计划编号、合同编号、生效起止日、覆盖期间起止日、应付日期、实际支付日期、付款时点（先付/后付）、金额/币种/税额、金额类型（固定/指数调整/变量/服务费/押金/税金）、会计属性（租赁/非租赁/变量/税费）、是否进入负债现值计算

### 事件关键字段
事件编号、合同编号、事件类型、生效日期、申请/审批日期、原值/新值、变更原因、判断依据、事件状态、重算批次号

### 租赁管理关键字段
合同附件/文档类型、版本号、文件哈希、MinIO object name、关键日期类型、目标日期、提醒天数、负责人、提醒状态、义务类型、责任方、义务状态、原文引用、结构化条款值

### 计量结果关键字段
合同编号、会计期间、期初/期末租赁负债、本期新增/利息/付款/重估调整、期初/期末使用权资产、本期新增/折旧/减值/终止处置

### 审计追溯字段
数据版本号、合同版本号、利率版本号、计算规则版本号、导入批次号、创建/更新人及时间、审批人、附件索引

### AI 识别与入库字段
AI 任务编号、文件编号/类型/哈希、上传人/时间、识别/OCR 状态、文档分类结果、识别出的合同/门店/事件、字段名称、AI 提取值/最终确认值、置信度评分、scope 初判、原文页码/位置坐标、差异原因、审核人/时间、入库结果状态

## MVP 架构分层

- **Web 前端** (Next.js + TypeScript + Ant Design): 合同台账、AI 录入、文件上传、草稿确认、月结、报表、组合分析、ROI、敏感性、多准则对比
- **Core Service** (Go + Gin): 用户登录与权限、合同主数据、付款计划、事件管理、租赁管理、IFRS 16 计量、月结、ERP 导出/回写、审计日志
- **AI Service** (Python + FastAPI): 文件解析请求、PaddleOCR、PDF/Excel/scan copy 解析、LLM 字段抽取、草稿生成、置信度与原文定位
- **PostgreSQL**: 正式业务数据、AI 草稿数据、任务状态、审核记录、系统日志索引
- **MinIO**: 原始上传文件、合同附件、OCR 中间产物、解析后的文本和结构化结果文件

## 目录结构

```
lease_management_system/
├── db/init/                     # 数据库初始化 SQL（01_init.sql + migrations）
├── db/migrations/               # 增量迁移 SQL（scope gate、lease admin、obligations/portfolio）
├── core-service/                # Go + Gin 核心 API
│   ├── cmd/api/main.go          # 路由注册（含 middleware）
│   ├── cmd/ifrs16-regression/   # IFRS 16 回归对数报告生成命令
│   └── internal/
│       ├── handlers/            # HTTP handlers（auth, contract, approval, payment_schedule, event, calculation, monthly_closing, reports, lease_admin, discount_rate, ai_chat, user, role）
│       ├── middleware/           # auth.go（JWT）, tenant.go（多租户过滤）, cors.go
│       ├── repository/          # 数据访问层（contract, payment_schedule, event, calculation, monthly_closing, lease_admin, user, role, report, audit）
│       ├── models/              # 数据模型
│       └── services/            # 业务逻辑层（audit, ifrs16）
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
│       ├── contracts/           # 合同列表 / 新增 / [id]详情（合同、付款、事件、计量、日期、文档、义务）
│       ├── ai-chat/             # AI 录入主入口（上传 → 解析 → 草稿卡片 → 人工确认）
│       ├── upload/              # 传统批量上传备用入口
│       ├── reports/             # 报表查询
│       ├── portfolio/           # 组合分析
│       ├── roi/                 # ROI 测算
│       ├── sensitivity/         # 敏感性分析
│       ├── standards/           # 多准则对比
│       ├── cashflow-forecast/   # 现金流预测
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
docker logs lease-ai --tail 20

# 查看 Core Service 日志
docker logs lease-core --tail 20

# 重建单个服务
docker compose build ai-service && docker compose up -d ai-service

# 后端测试
cd core-service && GOCACHE=$(pwd)/.gocache go test ./...

# 前端类型检查与生产构建
cd web && npm run type-check && npm run build

# 生成 IFRS 16 计量回归对数报告
make ifrs16-regression
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

- 法人种子数据：LE001（零售集团总公司）、LE002（零售集团上海公司）
- 测试账号：testuser / admin_user / user_le001 / user_le002（密码均为 password123）
- ⚠️ 数据库已重置（2026-05-30），历史测试合同数据需重新创建
- IFRS 16 计算参考值：初始负债 ¥3,255,676.79，36 个月摊销
- 月结 2024-01 参考分录：利息 ¥13,318、折旧 ¥92,170、付款 ¥50,000
- 回归测试参考：20 个用例 / 133 条断言，通过后生成 `docs/IFRS16_计量回归对数报告.md`

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
| 租赁范围闸门前置到计量引擎 | 避免短期租赁、低价值资产、非租赁合同被错误资本化 |
| AI 运行在 Assist Mode | AI 只生成建议和草稿，正式入库、计量、过账仍由审批和锁账控制 |
| `/ai-chat` 作为默认录入入口，`/upload` 保留 | 降低录入摩擦，同时保留批量上传和逐步复核备用路径 |
| ERP 集成先做 CSV 导出 + 凭证回写 | 先跑通一条最小可演示链路，再按客户 ERP 做字段映射 |
| IFRS 回归报告与白皮书纳入交付物 | 将计量正确性变成可复验的售前和审计材料 |

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
- 2026-05-30: 项目重命名 — `ifrs16-management-system` → `lease-management-system`（容器名、数据库名、MinIO bucket、JWT secret、前端 branding、CSV 前缀、localStorage keys）
- 2026-05-30: 修复数据库 schema — 补充 `lease_contracts` 缺失列（`lessee_name`, `lessor_name`, `store_name`, `store_address`, `tags`, `approved_at`）
- 2026-05-30: 修复 AI Chat Session 删除按钮 hover 显示
- 2026-05-30: 仪表板清理硬编码 — 租赁负债趋势图和合同状态饼图改为基于真实数据或空状态
- 2026-05-31: 完成商业进阶路线图 P0 — IFRS 16 回归测试框架、20 个测试用例、自动化对数报告、计量方法与准则映射白皮书
- 2026-05-31: 完成范围闸门 — `lease_scope` 数据模型、AI 初判、计量引擎分流、短期/低价值豁免费用化、非租赁合同跳过资本化
- 2026-05-31: 完成租赁管理平台第一/二步 — lease_documents、critical_dates、lease_obligations、asset_type、合同详情页管理视图、仪表板关键日期提醒、组合分析
- 2026-05-31: 完成 AI 对话式录入升级 — `/ai-chat` 上传文件自动解析，生成结构化合同草稿卡片，透传 scope 字段，人工确认后批量创建草稿合同
- 2026-05-31: 完成商业化与分析能力 — ROI 测算页、ERP 分录导出与凭证回写、敏感性分析、多准则对比、现金流预测接口
- 2026-05-31: 验证通过 — `go test ./...`、`npm run type-check`、`npm run build`、`make ifrs16-regression`、`python3 -m py_compile ai-service/app/routers/parse.py`
