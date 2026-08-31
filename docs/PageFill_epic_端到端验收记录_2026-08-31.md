# PageFill epic 端到端验收记录（2026-08-31）

> 结论：**PageFill epic 仍未完成完整端到端验收，不得标为完成。** 本轮完成代码追链、带库集成测试和阻断修复；Docker 现已可用，并已用浏览器验证 Actual vs Budget 的经营脉搏、区域/品牌汇总和门店 360 展示。真实文件上传、PageFill 深链、人工改建议后提交与回放仍待补跑。全部结论只适用于本地模拟环境，不代表真实客户验证。

## 1. 本轮环境

- 工作树基线包含 2026-08-28 后的 pagefill 在途改动，本轮保留并续接，没有回滚。
- Docker API 已可用；Core、Web、PostgreSQL、MinIO 通过 Compose 启动。Core 的 `/live` 与业务 API 可用，`/ready` 的现有健康检查仍按数据库就绪语义返回，不在本轮改动。
- 使用 Compose 网络内的 PostgreSQL 运行带库集成测试；测试数据按既有 integration cleanup 清理。
- 浏览器已验证经营脉搏总览、区域/品牌汇总及门店 360 的 Actual vs Budget 表格；PageFill 文件上传链仍未取得浏览器证据。

## 2. 追链发现与修复

| 编号 | 发现 | 修复 | 回归证据 |
|---|---|---|---|
| PF-E2E-01 | 四个 fill 工具把 tool call ID 写进 `deep_link`，artifact 落库又生成另一 UUID。卡片即使有按钮也会打开不存在的 artifact | `aichat.Runtime.complete` 在持久化前生成 pagefill artifact UUID，并统一改写 `fill` / `schedule_fill` / `tb_fill` / `plan_fill` 参数；repository 保留调用方已给 ID | `TestBindPageFillArtifactID`；`internal/aichat` 通过 |
| PF-E2E-02 | `/ai-chat` 的 `page_fill` artifact 只有原始 JSON，没有目标页动作，也没有逐字段 provenance | 新增 pagefill 专用卡片，显示分类、目标页、Payload/Suggestions、basis/engine/confirmed_by，并只允许站内 `deep_link` | `pageFillCard.test.ts` 2 项通过 |
| PF-E2E-03 | 零售 mapping 静默进入提交状态；TB `column_structure` 完全未渲染；plan 只有摘要 | 零售映射只显示“AI 建议”，不写入提交 mapping；TB 显示行数、列名与抽样科目；plan 保留原摘要提示 | `usePageFillAdoption.test.ts` 7 项通过；前端 type-check 通过 |
| PF-E2E-04 | 预算和 TB 可命中 fill 工具，但分诊仍返回 `unknown`；未知文件没有 spec 指定的具名 Gap | 增加 `budget_plan` / `trial_balance` 分类；未知结果返回 `gap_code=doc_class_unresolved`；分诊结果随 pagefill artifact 展示 | `doc_triage_test.go`；`internal/agenttools/tools` 与 `internal/aiagent` 通过 |
| PF-E2E-05 | PageFill artifact 没有证据引用却声明 `EvidenceComplete`，持久化时会被 Artifact 协议拒绝 | 无证据引用时明确标为 incomplete 并加入 `evidence_incomplete` 复核理由 | `TestProjectResultPageFillWithoutEvidenceStaysIncomplete`；`internal/aiagent` 通过 |

## 3. 已执行验证

### 3.1 Go 定向回归

```text
go test ./internal/pagefill ./internal/agenttools/tools ./internal/aiagent ./internal/aichat ./internal/agentskill
```

结果：全部通过。

### 3.2 前端定向回归

```text
vitest: pageFillCard / usePageFillAdoption / planFill / scheduleFill
25 tests passed
tsc --noEmit
```

结果：全部通过。

### 3.3 一次性 PostgreSQL 集成回归

在本机一次性数据库加载 `db/init/01_init.sql` 后执行：

```text
go test -count=1 ./internal/repository ./internal/aichat ./internal/agenttools/tools ./internal/handlers
```

结果：四个包全部通过。该结果证明 repository、artifact runtime、工具与 HTTP handler 能在真实 PostgreSQL schema 上运行；它不证明 MinIO 文件读取和浏览器消费链。

## 4. Spec §4 状态

| §4 验收项 | 状态 | 已有证据 | 仍需补跑 |
|---|---|---|---|
| 1. 混杂文件正确分类，未知文件明确拒答 | 部分通过 | 新分类与 `doc_class_unresolved` 单测通过 | 用合同 PDF、付款计划、门店日事实、预算、TB、2 个四不像文件在 `/ai-chat` 真实上传；记录准确率 |
| 2. 卡片直达目标页，Payload/Suggestions 分层预填 | 部分通过 | artifact UUID 绑定、卡片动作、共享 hook 与各消费纯函数通过 | 浏览器逐个点击，确认 URL、表单值与黄标；付款计划需从已绑定合同会话进入 |
| 3. provenance 可见，改 Suggestion 后提交并留差异 | 部分通过 | 聊天卡片逐字段显示 provenance；建议标识可见 | 真实修改并提交，核对 draft、audit 与 artifact 快照可比较；当前未取得带库 UI 证据 |
| 4. 零 AI 直写，artifact 可回放 | 部分通过 | fill 工具 `side_effects=false`、I5 与 LevelDraft 单测；artifact 带库持久化路径通过 | 提交前后查询业务表计数、回放 artifact、重复投递验证幂等 |

## 5. 补跑步骤

Docker Desktop 恢复后：

1. `make up`，确认 migration 完成、`/health` readiness 通过。
2. 确认模拟数据集 `SIM-2853D953-007` 与一份可绑定合同存在。
3. 在同一 `/ai-chat` 会话依次上传五类文件；付款计划从合同详情进入 AI Chat。
4. 每个 artifact 记录 `artifact_id`、`document_class`、`deep_link`、目标页截图和业务表提交前后计数。
5. 对一个 Suggestion 人工改值后提交，保存 artifact JSON、draft/audit 记录与差异。
6. 同一文件和同一幂等键重放，确认不产生第二条业务记录。
7. 上传两个无关键词文件，确认 `gap_code=doc_class_unresolved`，且没有目标页猜测。
8. 跑 `go test ./...`、`go vet ./...`、前端 test/lint/type-check/build、`make test-integration`。

## 6. 已确认的语义边界

付款计划 Fill 只接受已存在的 `contract_id`。新合同随附租金表必须先完成合同草稿确认，再从合同上下文生成付款计划 Fill。当前的 `contract_unbound` 拒绝是正确降级，不应创建临时合同、猜合同或绕过草稿链。
