# Spec：Agent 通用页面预填（agent-universal-pagefill-v1）

> 目标（产品负责人 2026-08-27 口述）：**员工扔一个 Excel 给 AI Agent，Agent 自己识别它是什么、填进对应功能页、在页面上生成 draft，人只需要确认提交。** 员工不再手工填数。
>
> 本 spec 是现行规格；上位约束见 AGENTS.md 五条底线与 Assist Mode 条款。

## 0. 现状基线（2026-08-27 审计，勿重复踩坑）

- `internal/pagefill` 协议已存在且形态正确：`Fill{TargetPage, TargetAPI, DeepLink, Payload(确认值), Suggestions(未确认值), ReviewRequired}`，核心不变量 **I5：prefill 永不 commit，提交必须人驱动；Exploratory 值结构上进不了 Payload**。
- 全站恰好 **1 个生产者工具**（`retail.store_days.import.preview`）↔ **1 个消费者页面**（`/retail-data-import?fill=…`），且生产者不在任何 skill 白名单（自然语言不可达）。管线两端造好、中间锁死。
- 生产聊天 function-calling 是单轮（`callLLMWithTools`）；多轮内核（agentcore loop）存在但未接聊天域逻辑。
- 技能路由存在权重遮蔽：含零售词的写请求被只读技能抢走（FP&A 反馈 §7.4.11 实测 4/4 失败）。

## 1. 设计立场：不是浏览器机器人

Agent 不操作 UI 控件，而是产出标准化的**页面预填 artifact**（pagefill.Fill），经深链落到功能页的表单里；数字权威仍在后端服务与工具治理内。理由：

1. 免去整套 UI 自动化的脆弱性与并发风险；
2. prefill 天然落在 I5 的「人工确认」闸前，与 Assist Mode 底线 8 同构；
3. 页面已有的行级校验、口径头、降级语义全部复用，AI 不另起一条写入路径。

## 2. 分期交付

### P0-A：把已有管线接活（不新增页面，1–2 天量级）

| 改动 | 内容 | 验收 |
|---|---|---|
| 技能绑定 | 新增 `ecommerce_*` skill 或并入既有 skill：绑定 §审计 B 表 18 工具中该用的部分，必含唯一 pagefill 生产者 `retail.store_days.import.preview` | Runtime.Describe 枚举 vs skill-contracts 双向差集为 0（或留有具名豁免清单） |
| 路由去遮蔽 | 写类意图命中时不受只读技能 Priority 压制（显式关键词→skill 绑定或匹配权重修正） | 反馈 §7.4.11 的四句实测问句重放：全部命中带写工具的 skill |
| 单轮→多轮 | 文件分诊后的 agent 流程接入 agentcore Loop（StreamFunc 包 callLLMWithTools，工具结果回灌下一轮），先只开「分诊→解析→建议目标页」三步 | 集成测试：一个 Excel 上传产生 ≥2 轮模型调用的事件轨迹 |

### P0-B：Excel→功能页 映射器扩容（每个映射 = 一个 tool + 一个页面消费点）

通用化 `retail_ingest_preview.go` 的模式：**一种业务对象 = 一个 triage 类别 + 一个 pagefill 生产者工具 + 一个页面的 fill 参数消费**。首批按员工真实投递频率排序：

| # | 投递物 | 目标页 | 生产者工具（新） | 复用后端 |
|---|---|---|---|---|
| 1 | 付款计划 Excel | `/contracts/drafts`（付款计划草稿） | `lease.payment_schedule.fill.draft` | 已有 parse_payment_schedule + 草稿层 |
| 2 | 合同 PDF/扫描件 | `/ai-chat` 草稿卡 → `/contracts` | 已有 `lease.contract.draft.create`（已绑定，作对照实现） | docparse + aiintake |
| 3 | 门店日事实 Excel | `/retail-data-import`（现有消费者保留） | 已有 import.preview（P0-A 接活） | retailingest |
| 4 | GL 试算平衡表 | `/retail-data-import` 数据导入区 | `fpna.trial_balance.fill.draft` | trialBalanceApi.import |
| 5 | 预算版本 Excel | `/retail-data-import` 计划版本区 | `fpna.plan_lines.fill.draft` | fpnaPlanImportApi |

规则（对每个新工具一致强制）：
- 命名空间照 AGENTS.md 三根（`lease.*` / `fpna.*` / `retail.*`），二级段 `.fill.`；Level=Draft + ReviewPolicy{Required} + Idempotency-Key 必填；
- 输出一律 `pagefill.New(...)`；数值列只在 Suggestions（未过行级校验不得进 Payload）；confidence 低字段必须留在 Suggestions 并给原因码；
- 识别失败返回具名 Gap（`doc_class_unresolved` 等），禁止「猜一个最像的页面」；
- 每个 Fill 落 artifact 存档（复核可回放），TTL 与 audit_logs 对齐。

### P0-C：前端通用消费协议（一次基建，所有页面受益）

现状 `/retail-data-import` 的消费是页面手写的（useEffect 读 `?fill=` 再逐字段 set）。抽成共享 hook：

```
web/app/lib/usePageFill.ts
  usePageFill(consumer: { page: string; apply: (payload, suggestions) => void })
    - 读 ?fill=<artifactId> → GET /ai/chat/artifacts/:id
    - 校验 data.target_page === consumer.page（防跨页误投）
    - payload → apply()；suggestions 高亮展示待确认字段
    - 成功提示「来自 AI 预填 · 待人工确认」（i18n 三语）
```

- `target_page` 校验是安全边界：artifact 只能被声明的目标页消费；
- Suggestions 一律可视化标注（黄底/角标），绝不静默混入表单默认值——延续「观察信号与事实描述分开」的产品语言；
- 每个接入页面补 GUARD-001 测试：删掉 apply 分支测试即红。

### P1：覆盖面扩展与体验闭环（第二批之后）

- 余下功能页逐个评估是否值得映射（判定式：这个表单员工多久手填一次 × 字段能否从文件可靠抽取；两者都低就不做，登记即可）;
- 会话内「填好了」卡片直接挂 deep_link（已有 artifacts 动作位），员工一键跳转核对;
- 转正式的确认动作沿用各页现有 review/approve 链，**Agent 不出现在批准路径上**。

#### P1 覆盖面判定登记（2026-08-31）

判定只看两项：员工手填频率，以及文件能否可靠抽取。页面有表单不等于值得做 pagefill。

| 写入场景 | 手填频率 | 可抽取性 | 决定 | 理由 / 复用路径 |
|---|---:|---:|---|---|
| 合同新建 `/contracts/new` | 高 | 高 | 不新增 pagefill | 合同 PDF 已走 `contract_draft` 草稿卡与人工确认，另造 pagefill 会产生第二条合同录入路径 |
| 既有合同付款计划 `/contracts/[id]` | 高 | 高 | 已做 | `lease.payment_schedule.fill.draft`；必须先绑定既有合同 |
| 新合同随附付款计划 | 高 | 高 | 后做 | 先确认合同草稿取得 `contract_id`，再生成付款计划 Fill；当前返回 `contract_unbound`，不得猜合同或临时直写 |
| 门店日事实 / 预算计划 / TB | 高 | 高 | 已做 | 统一落 `/retail-data-import`，不在脉搏、FP&A 工作台再建重复导入器 |
| 月粒度旧经营导入 `/performance` | 中 | 高 | 不新增 pagefill | 新增经营指标必须走 store-day；旧月粒度入口只保兼容，不扩能力 |
| 合同变更、续签、终止协议 | 中 | 高 | 后做 | 文件可抽取，但必须落事件草稿并沿既有审批链，不能预填后直接改合同 |
| 促销活动与成本 `/promotions` | 中 | 中 | 后做 | 活动 brief/费用表可抽取；先统一活动与费用的文件模板，再接两个草稿入口 |
| 新店 / 报价测算 `/pre-deal`、`/deal-compare` | 中 | 中 | 后做 | 报价单可抽取，但折现率缺失必须停在 human-in-the-loop，不给默认值 |
| 电商结算导入 `/settlement-workbench` | 高 | 高 | 后做 | 独立业务域且模板已列明，适合作为第二批映射 |
| 三表模型定义、公式与期初 `/financial-model` | 低 | 低 | 不做 | 定义受治理，公式只走后端 DSL 校验；期初优先来自 TB，不从任意文档猜值 |
| 月结、审批、锁账 `/monthly-closing` | 高 | 低 | 不做 | 高频但属于控制动作，输入来自系统状态；pagefill 不得进入批准、过账或锁账路径 |
| 参数、用户与权限 `/settings`、`/admin/users` | 低 | 低 | 不做 | 配置低频且涉及权限/政策，文件抽取收益低、风险高 |
| 情景、敏感性、准则比较 | 低 | 低 | 不做 | 输入少且需要人的判断，直接表单比文件抽取更清楚 |

## 3. 明确不做（Non-Goals）

1. 不做浏览器/UI 自动化（点击、截图驱动）；
2. 不让 Agent 直接 POST 各页面的写入 API——fill 工具只产 artifact，写库仍由人在页面上触发既有提交端点；
3. 不做跨法人记忆共享或 global 上下文（AR3 既定拒绝）;
4. 模拟数据的 Fill 不进入任何 Official 版本（底线 2 在 artifact 层同样生效：data_classification 必须随行）。

## 4. 验收总则（整个 epic 完成的定义）

一名新员工拿到一批混杂文件（合同 PDF / 租金表 Excel / TB 导出），全部拖进 /ai-chat：
1. Agent 对每个文件给出正确的 doc 分类与目标页卡片（≥95% 试点样本准确率，不确定的明确拒答）;
2. 点击卡片直达目标页，表单已按 Payload/Suggestions 分层预填;
3. 每一处 AI 来源字段可见 provenance；人工改掉一个 Suggestion 后提交，draft 正常入库且差异留痕;
4. 全程零次「AI 直接写库」。回放任一 draft 可还原当时的 Fill 快照。
