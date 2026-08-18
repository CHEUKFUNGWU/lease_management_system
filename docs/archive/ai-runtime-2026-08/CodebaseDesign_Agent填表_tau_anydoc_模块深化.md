# CodebaseDesign：Agent 填表升级（tau + anydoc）模块深化

> ⚠️ **ARCHIVED 2026-08-18 — 不是现行依据**
> 归档理由：同配套实施计划：tau 模块边界作废，M1 与 M4 已迁入《Agent Core（Go）设计》§8.2 与附录 A
> 现行入口：`docs/AI_文档索引与现行决策.md`

> ⚠️ **文档状态：Partially Superseded（2026-08-18）**
>
> 与配套实施计划同步作废/保留：**tau 相关的模块边界作废**（见 [ADR-0022](adr/0022-first-party-go-agent-core-modelled-on-pi.md)），**anydoc 解析适配器与页面 fill 缝的模块设计保留**（见 [ADR-0024](adr/0024-remove-the-agpl-pdf-dependency.md)）。
>
> Agent 内核的模块边界以 [Agent Core（Go）设计 —— 对齐 pi 架构](Agent_Core_Go设计_对齐pi架构.md) §4–§7 为准。
>
> 原文档状态：Draft（待评审）
>
> 配套文档：[AI Agent 填表升级（tau + anydoc）实施计划](AI_Agent_填表升级_tau_anydoc_实施计划.md)
>
> 上游设计：[CodebaseDesign_零售经营分析工作站_模块深化.md](CodebaseDesign_零售经营分析工作站_模块深化.md)（M6/M8）、ADR-0019（Agent Tool Runtime 政策与威胁模型）、ADR-0021（许可证姿态）
>
> 词汇：module（模块）、interface（接口）、depth（深度：小接口藏大实现）、seam（接缝）、adapter（适配器）、locality（局部性）、leverage（复用杠杆）、deletion test（删除测试）

## 0. 总体形状

```text
用户（Web /ai-chat、/retail-data-import）
        │
        ▼
tau-agent 容器（Python 3.12）        ← 换脑（M2）
   └ tau_agent 循环（工具 = M3 装配的 schema）
        │ 子进程
        ▼
lease-agent CLI（Go）               ← 契约与执行器（M3-A）
        │ HTTP
        ▼
Core Agent Gateway ── Tool Runtime ── 业务服务（draftapp / retailingest / plan / TB / scenario）
        ▲
ai-service（单发端点：parse → M1 anydoc、chat、suggest-mapping）
```

四条设计原则（与实施计划 D1~D5 对应）：

1. **大脑可换，接缝不动。** tau 只依赖 `GET /api/v1/agent/tools` + `POST /api/v1/agent/tools/execute` + run/capability 路由。这些契约已由 AG-024/025 固化并测试；换脑不触碰 Core。
2. **CLI 是契约，不是装饰。** tau 与人都消费同一 CLI 语义；CLI 不复制业务逻辑（AG-026 原则延续）。
3. **填表 = 预填 + 人确认。** Agent 的写权限停在 preview / artifact；commit 是人或人驱动的 CLI（D3）。
4. **新模块只做 adapter。** 业务规则、指标语义（`retail-kpi-v1`）、会计引擎一概不动。

## 1. M1 — anydoc 解析适配器（深模块）

**位置**：`ai-service/app/services/anydoc_adapter.py`（约 120 行）。

**接口**（整个模块对外的唯一入口）：

```python
class ParsedDocument(BaseModel):
    markdown: str                      # GFM，供 LLM 与证据锚点使用
    format: str                        # docx / xls / pptx / odt / rtf / epub ...
    evidence_mode: Literal["quote", "coordinate"]  # office=quote；PDF 维持 coordinate
    warnings: list[str] = []           # 降级说明，进入 review 上下文

def parse(source: Source) -> ParsedDocument   # Source{data, filename, size_limit}
```

**深度——藏起来的东西**：格式探测（扩展名 + 内容）、10MB 上限、加密/损坏文件的错误分类（转成 `parse_unsupported` / `file_encrypted` / `file_too_large`，绝不静默成功）、CSV 的确定性路径（标准库，不交给 anydoc/LLM）、evidence 降级（office 无坐标体系 → quote 锚点，`evidence_complete=false` 的既有语义不变）。

**调用方收益**：`parse.py` 的分派逻辑从「4 种解析器 + 各自错误转换」收敛为对 M1 的一次调用；新增一种 office 格式时只动 M1。

**seam 判定**：M1 没有第二个实现（anydoc 是真实依赖，测试用固定 fixture 文件）。这使它是一般接口而非接口+实现接缝——可接受，因为接口小到足以被整体替换，且删除测试兜底：**删掉 adapter，office 解析测试立刻红**。

**GUARD-001 证据设计**：`.xls`（openpyxl 读不了的旧格式）fixture 经 M1 解析出正确单元格内容；同一 docx 断言 GFM 输出含预期文本；删除测试红。

## 2. M2 — tau harness（深模块）

**位置**：`tau-service/app/tau/harness.py`（新容器，Python 3.12）。

**接口**：

```python
async def run(request: RunRequest) -> AsyncIterator[RunEvent]
# RunRequest{message, file_refs, skill_id, session_ref, limits}
# RunEvent{kind, payload}  # plan / tool_started / tool_completed / artifact_ready / review_required / finished / failed
```

**深度——藏起来的东西**：`tau_agent` 的会话与循环原语、provider 配置（经 `tau_ai` 接 DeepSeek/OpenAI）、工具 schema 装配（调 M3）、子进程生命周期管理（超时杀掉、退出码映射）、事件翻译（tau 事件 → Core run events）、checkpoint 序列化（tau 会话状态 → Core checkpoint API）、预算（最大工具数 / deadline / token 上限）、取消信号。

**locality 边界**：tau 的会话状态是**临时记忆**；权威持久化全部在 Core（run / checkpoint / artifact / 审计）。tau 容器崩溃 → 重跑即可，无业务状态丢失。

**关键约束（写进代码而不是文档）**：工具注册表只接受 M3 产出的 schema；`tau_coding` 自带的 read/write/edit/bash **不导入**（AG-004：禁止任意 Shell/文件工具）。这条用删除测试表达：**删掉 schema_bridge，harness 没有任何可注册工具 → run 无法开始（测试红）**。

**为什么不是内嵌 ai-service**：ai-service 是 3.11、单发、无循环；tau 需要 3.12 和长生命周期循环。独立容器延续 `agent-runner` 的进程边界（无数据库/MinIO 凭证，只有 Gateway HTTP 权限）。

## 3. M3 — ToolSchemaBridge（两个适配器的真接缝）

**位置**：`tau-service/app/tau/schema_bridge.py` + `cli_executor.py`。

**接口**：

```python
class Executor(Protocol):
    async def execute(self, call: ToolCall) -> ToolResult   # ToolCall{tool, version, arguments, run_id, idempotency_key}
    def to_tau_schema(self, descriptor: dict) -> dict       # Core ToolDescriptor → tau tool schema
```

**两个适配器 = 真接缝**：

- **A（本轮落地）CLI 子进程**：`lease-agent execute --tool X --version v1 --arguments - --run-id … --capability-token …`，stdin 写参数 JSON、stdout 读 ToolResult JSON、stderr 丢弃到日志、退出码映射为 tau 工具错误（0=成功 / 2=参数 / 3=认证 / 4=权限或需复核 / 5=业务或系统失败——沿用 AG-026 已固化的退出码契约）。
- **B（预留）HTTP 直连**：同一 ToolDescriptor 经 httpx 调 `POST /api/v1/agent/tools/execute`。当单 Run 工具调用量超过子进程开销阈值时切换，接口不变。

**深度**：调用方（M2）只看到「schema + executor」两件事；CLI 的 token 注入（capability token 从环境读取、绝不进日志）、stdout 污染防护、超时与重试全部藏在 M3 里。

**契约测试**：用 Core 导出的真实 ToolDescriptor fixture（含 input/output schema、level、review policy）断言转换结果可被 tau 注册、参数 JSON 可被 CLI 解析；CONTRACT-001 风格——后端是 schema 唯一源，前端（此处是 tau 侧）只消费。

**删除测试**：删掉 schema_bridge → M2 无工具可用（上节）；删掉 cli_executor → 工具注册成功但执行路径不存在（测试红）。

## 4. M4 — PageFill seam（每页填表契约）

**位置**：Core 侧 `internal/agentartifact/`（协议扩展）+ Web 侧页面消费组件。

**协议**（新 artifact_type `page_fill`，迁移 `046_page_fill_artifact.sql` 双交付 `db/init/01_init.sql`）：

```json
{
  "artifact_id": "…",
  "artifact_type": "page_fill",
  "schema_version": "v1",
  "target_page": "retail-data-import",          // 枚举：retail-data-import | scenario-workbench | …
  "target_api": "POST /retail/operating-facts/store-days/import/preview",
  "payload": { },                               // 页面表单模型，由 target_page 的 schema 定义
  "sources": [], "model_version": "…", "rule_version": "…",
  "confidence": 0.0,
  "review_required": true, "review_reasons": [],
  "deep_link": "/retail-data-import?fill=artifact-…"
}
```

**深度**：页面只认「填表载荷」，不认 Agent 内部结构。payload 的 schema 与页面表单一一对应（同一份字段定义驱动表单与校验——与 M8/retailingest 的「backend 白名单、frontend 消费」同一纪律）。Agent 换脑（tau ↔ 其他）不改变页面消费端。

**确认点纪律**：每个 page_fill 只有一个 `confirm` 落点（对应既有业务写入 API）；confirm 动作保留人工触发 + 幂等键。页面预填只是「把人要点的按钮准备好」，不替代人。

## 5. 每页 fill 契约表

| 功能页 | 现有写入口 | 新增 Agent 工具（白名单） | CLI 命令（人） | 预填 payload | 确认点（人） |
|---|---|---|---|---|---|
| 合同台账 | `lease.*.draft.create`（已有） | 已有，不动 | 已有 | 草稿字段 | 审批流 |
| 零售数据导入 | preview / commit | `retail.store_days.import.preview` | `retail import preview\|commit` | mapping、source_system、as_of | 导入页 commit 按钮 |
| 租金谈判测算 | evaluate + action-draft save | `retail.store.scenario.evaluate`（已有）+ `retail.scenario.action.save` | `scenario evaluate\|save` | assumptions、store_id、window_days | 保存/采纳按钮 |
| FP&A 计划 | `/fpna/plan-versions/import` | `fpna.plan.import.preview` | `plan import preview\|commit` | 映射 + 校验行 | commit 按钮 |
| Trial Balance | `/gl/trial-balances/import` | `gl.trial_balance.import.preview` | `tb import preview\|commit` | 映射 + 校验行 | commit 按钮 |
| 月结 | generate（已有） | 无新增（只读解释） | 无 | — | 审批/锁账不变 |

**原则**：preview 工具对 Agent 开放（LevelDraft，产出 page_fill）；commit 工具只对 CLI（人）开放，不进 Agent 白名单（D3）。经营脉搏 / 门店 360 保持只读——它们的「填表」就是导入页，不另开写口。

## 6. 五底线映射（设计级承担）

| 底线 | 承担模块 | 机制 |
|---|---|---|
| 1 跨法人隔离 | Core Gateway（不动） | Capability Token + 服务端身份/范围交集；CLI 与 tau 均无绕过路径 |
| 2 模拟 / 正式区分 | retailingest（不动） | 信封三件套 + `data_classification`；page_fill payload 携带该字段 |
| 3 来源追溯 | M4 协议 + 既有审计 | payload 回写 run_id / artifact_id / model_version / rule_version |
| 4 重复导入保护 | 既有幂等（不动） | M3 透传 `--idempotency-key`；CLI commit 重放不产生第二条 |
| 5 IFRS 16 正式台账隔离 | 白名单纪律（M2/M3） | fill 工具全部落在 draft / preview / 事实导入层；计量正式表零写入 |

## 7. GUARD-001 验收设计（替换类改动）

自检句：**把 B 删掉或改错，测试会不会红？不会红就是没写对。**

| 波次 | B 是什么 | 证据 |
|---|---|---|
| T1 | anydoc adapter | `.xls` fixture 解析断言 + 删除测试红（旧解析路径无 .xls 能力） |
| T2 | tau harness | run trace 出现 tau 事件与 CLI 子进程调用记录；同任务与 `agent-runner` 输出对照；删除 schema_bridge 测试红 |
| T3 | 页面预填 | 组件受控值断言（渲染 page_fill → 表单字段 = payload），非「旧入口消失」 |
| T4 | 端到端填表 | 固定 seed 的 POS 风格 Excel：预填 → 确认 → pulse 显示 production 数据 + 诚实覆盖率 |

## 8. 边界（不做什么）

- 不给 tau 任何 bash / 文件系统 / 网络自由工具（AG-004）。
- 不自动 commit、不自动审批、不自动过账（D3；AG 底线不变）。
- 不重写 Web 零售确定性读链（M6.1「不引入 LLM function calling 主链路」对既有零售问答路径继续有效；tau 是新增的填表路径）。
- 不动 `retail-kpi-v1` 指标语义、不动 IFRS 16 计量引擎（ADR-0021 的提取约束保持）。
- 不在本轮做 Auto-Post Mode 政策。
