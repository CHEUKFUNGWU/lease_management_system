# Spec：Agent Runtime 完整升级（C1 批次 · 参考 picoclaw）

> 编制：2026-08-23 · 状态：`ready-for-agent`
> 来源：ADR-0027（采用 picoclaw agent 内核）的范围扩展。用户 2026-08-23 明确：**不止迁移 AgentCore，是把一个 agent runtime 应有的能力完整囊括进来，包含 session manager 与 context engineering**
> 发布位置说明：本仓无 issue tracker，按既有惯例落 `docs/specs/`
> 配套模块设计：[CodebaseDesign_AgentRuntime升级_模块深化.md](../CodebaseDesign_AgentRuntime升级_模块深化.md)
> **ADR-0027 需随本 spec 扩写**：其 Non-goals 现排除 `providers`/`identity`/`auth`/`credential`，未提 `bus`/`session`/`memory`/`routing`，与本 spec 的范围不一致

---

## 先决：两边的实测规模

**本仓 agent 侧 ≈ 18,200 行**（不含测试）：`aiagent` 6603 · `agentrunner` 2455 · `agentcore` 1819 · `agenttools` 1806 · `aichat` 1784 · `llm` 1086 · `agentskill` 857 · `agentcapability` 765 · `agentseval` 658 · `agentartifact` 184 · `agentreaders` 174

**picoclaw `pkg/` 36 个包**（Go 1.25, MIT, 谱系 picoclaw ← openclaw ← pi）。

**「完整升级」不等于替换那 18,200 行。** 最大的两块恰恰不该换：`aiagent` 6603 行是领域接线，picoclaw 没有本产品的业务因而给不出对应物；`agentrunner` 2455 行是 checkpoint 与租约恢复，ADR-0022 原文即写「pi does not have and this product does need」。**升级的是能力面，不是行数。**

## Problem Statement

站在 BP、店长与平台工程师三类使用者的位置上，当前 runtime 有三个层次的问题。

**第一，长会话是靠运气。** 全仓**没有任何上下文管理**——无裁剪、无窗口、无预算，连 token 计数都没有（`tiktoken` / `CountTokens` / `NumTokens` 全仓零命中）。`agentcore.State.Messages()` 原样返回整个会话。一次底稿排查对话跑到几十轮，消息列表只增不减，直到某次 LLM 调用因超限失败——失败点不可预测，且失败时用户已经投入了半小时的上下文。ADR-0022 的 Non-goals 把压缩推迟到「观察到真实溢出」，但**没有任何机制会观察到它**：没有 token 计数就没有可观测量，条件永远不会被满足。

**第二，会话有存储但没有管理者。** 库里有七张表（`ai_chat_sessions` / `runs` / `messages` / `run_events` / `artifacts` / `review_actions` / `attachments`），生命周期逻辑却散在六处：`handlers/ai_chat.go`、`handlers/ai_chat_runtime.go`、`handlers/agent_gateway_sessions.go`、`aichat/runtime.go`、`aichat/continuation.go`、`agentrunner/runner.go`。「一个会话现在是什么状态、能不能续、该不该淘汰、并发进来两条消息怎么办」这些问题没有单一的回答处，只能靠读六个文件拼出来。ADR-0022 §2 已经要求「持久化是订阅者不是循环里的一步」，但那只解决了写的时机，没有解决**谁拥有会话**。

**第三，runtime 缺少一批本该有的基础能力。** MCP（全仓零）、模型路由、定时触发、健康探针、跨会话记忆——这些不是锦上添花，是「agent runtime」这个词的通常含义的一部分。缺了它们，每一个新需求都要在 `aiagent` 那 6603 行里再接一根线，那个文件已经是全仓最大的单包。

## Solution

分三层推进，每层可独立交付、独立验收。

**第一层：内核置换（ADR-0027 §1）。** `agentcore` 的 1819 行换成 picoclaw `pkg/agent` 的适配版，治理链移植到其 `ToolInterceptor` / `LLMInterceptor` / `ToolApprover` 挂点。这是后两层的地基——`context_manager` 与 `session` 都挂在这个内核上。

**第二层：Session Manager 与 Context Engineering。** 前者把散在六处的生命周期收拢成一个模块；后者从零建起 token 计数 → 预算 → 压缩三级，且压缩必须对审计与溯源无损。

**第三层：能力补齐。** 子任务委派、MCP、模型路由、定时、健康探针、记忆，按价值排序逐个接入，每个都是可独立回退的增量。

**不做的**：`providers`（ADR-0022 §4，`internal/llm` 的中国区 endpoint 是一等公民）、`identity`/`auth`/`credential`（ADR-0022 §3 + 底线 1，picoclaw 是个人助手无租户模型）、以及边缘硬件场景的 `audio`/`media`/`devices`/`netbind`/`pid`/`seahorse`。

## User Stories

### Context Engineering

1. 作为 BP，我想让一次长时间的底稿排查对话不会因为上下文超限而中断，以便半小时的工作不会一次性丢失
2. 作为 BP，我想在上下文接近上限时收到提示，以便主动收束话题而不是被动撞墙
3. 作为 BP，我想让压缩发生后对话仍然记得前面确认过的关键结论，以便不用把已经说过的假设再说一遍
4. 作为审计人员，我想让压缩**永不丢弃承载审计意义的内容**（工具调用、其参数与结果、Artifact 引用、审批动作），以便压缩后的会话仍可复演
5. 作为审计人员，我想让被压缩掉的内容仍可从 `ai_chat_messages` 原表取回，以便压缩是呈现层的裁剪而不是记录的删除
6. 作为平台工程师，我想有一个可观测的 token 用量指标，以便「观察到真实溢出」这个条件真的可被观察
7. 作为平台工程师，我想让 token 计数与所用模型的分词器一致，以便预算不是靠字符数估算
8. 作为平台工程师，我想让上下文预算按模型可配置，以便换模型时不用改代码
9. 作为 BP，我想让压缩后的 run 仍能从 checkpoint 续跑，以便压缩不破坏既有的租约恢复能力
10. 作为使用者，我想让压缩策略在与底稿溯源冲突时**让位于溯源**，以便证据链优先于对话流畅

### Session Manager

11. 作为平台工程师，我想有单一模块回答「这个会话现在什么状态」，以便不必读六个文件拼答案
12. 作为平台工程师，我想让会话的创建、续接、淘汰、并发控制在一处实现，以便修一次就到处都修好
13. 作为 BP，我想在同一会话里连续提问不会因为并发而串话，以便两条消息不会交叉执行
14. 作为 BP，我想关掉浏览器再回来时会话还在，以便工作可以被打断
15. 作为店长（IM 渠道），我想让飞书里的会话与 Web 会话遵循同一套生命周期，以便行为可预期
16. 作为安全负责人，我想让会话严格绑定 `legal_entity_id`，跨法人不可读取，以便底线 1 在会话层也成立
17. 作为平台工程师，我想让空闲会话按策略淘汰而不是无限占用内存，以便长期运行不泄漏
18. 作为审计人员，我想让会话的每次状态迁移可追溯，以便复演一次对话的完整经过
19. 作为平台工程师，我想让 session manager 不做 IO，存储经端口注入，以便可以不起库测试全部生命周期分支

### 子任务委派

20. 作为 BP，我想让一个复杂问题被拆成子任务并行推进，以便「华东十家店逐店归因」不必串行等半天
21. 作为安全负责人，我想让**每个子回合独立经过完整治理链**，以便子任务不成为绕过审批的通道
22. 作为安全负责人，我想让子回合的权限范围**只能等于或小于**父回合，以便委派不能提权
23. 作为平台工程师，我想让委派深度与并发有可配置上限，以便无界递归不会在无人看管时跑飞
24. 作为审计人员，我想让子回合的工具调用可追溯到父回合，以便委派不产生审计盲区

### MCP

25. 作为平台工程师，我想让 runtime 支持 MCP，以便接入外部工具时不必每次改 `aiagent`
26. 作为安全负责人，我想让 MCP 引入的工具**同样经过治理链**（六前三后），以便外部工具不绕过权限、审计与 Review Gate
27. 作为安全负责人，我想让 MCP 工具默认不可用、需显式登记，以便不存在「装上就能用」的开放生态（ADR-0022 Non-goals）

### 其余能力

28. 作为平台工程师，我想按规则把不同任务路由到不同模型，以便成本与质量可以分别优化
29. 作为 BP，我想让经营日报在每天早上自动生成，以便不用手动触发
30. 作为运维，我想有健康探针反映 runtime 与其依赖的实际状态，以便故障可被发现而不是被用户报告
31. 作为 BP，我想让 agent 记得我上次说过的偏好（如常看的门店、习惯的口径），以便不必每次重复
32. 作为安全负责人，我想让跨会话记忆同样按 `legal_entity_id` 隔离，以便记忆不成为跨租户泄漏的新通道

### 迁移纪律

33. 作为安全负责人，我想让 ACORE-2 的九项变异在新挂点上逐项重新证红证绿，以便治理链不在迁移中被悄悄削弱
34. 作为平台工程师，我想让每一层都能独立回退，以便一层出问题不必整体回滚
35. 作为使用者，我想让所有既有 Agent 能力在升级后行为不变，以便升级是叠加而不是替换

## Implementation Decisions

### D-C0：判据是「runtime 该有的能力都必须有」，同时「吸收有用的、避开冗余的」

这两句是同一条判据的两面，都不能单独成立。

只讲「都必须有」会导致把 36 个包整体搬进来——其中七个是边缘硬件场景（`audio`/`media`/`devices`/`netbind`/`pid`/`seahorse`）、两个是自更新（`updater`/`evolution`，与本产品的发布与审计模型不相容），搬进来不增加任何能力，只增加维护面与上游漂移风险。

只讲「避开冗余」则会导致以「我们已经有类似的」为由跳过真实缺口——`agentcore` 有 `State.Messages()` 所以「有上下文管理」，这个推理正是当前长会话靠运气的由来。

**执行方式**：先按能力清单逐项点名（下表），每项标注现状是「已有」「有存储无管理者」还是「零」，再对每个非「已有」项决定自建还是取自 picoclaw。**判断依据是能力有无，不是包有无。**

| 能力 | 现状 | 处置 |
|---|---|---|
| 循环 / 回合控制 | 已有（`agentcore`） | 换内核（D-C1） |
| 工具执行与注册表 | 已有且更强（治理链、权限、审计、Review Gate） | 留 |
| 流式输出 | 已有（11 种事件） | 随内核 |
| 打断 / 纠偏 / 中止 | 已有 | 随内核 |
| 治理中间件 | 已有且 picoclaw 无 | **移植保留**（D-C6） |
| checkpoint / 租约恢复 | 已有且 picoclaw 无 | 留 |
| 技能注册与合约 | 已有 | 留 |
| 预算闸 / 限流 | 已有（`agentguard`） | 留 |
| 追踪 / 审计 | 已有（`ai_chat_trace`、`run_events`） | 留 |
| **会话管理** | **有存储无管理者**（散在六处） | **重建**（D-C4） |
| **上下文工程** | **零**（无计数、无预算、无压缩） | **新建**（D-C2/C3） |
| **子任务委派** | **零** | **取自 picoclaw**（D-C8） |
| **MCP** | **零** | **取自 picoclaw**（D-C5） |
| **模型路由** | **零** | 取自 picoclaw |
| **定时触发** | **零** | 取自 picoclaw |
| **健康探针 / 心跳** | **零** | 取自 picoclaw |
| **跨会话记忆** | **零** | 取自 picoclaw |
| 身份 / 权限 / 租户 | 已有且 picoclaw 结构上不可能有 | 留（ADR-0022 §3） |
| Provider 抽象 | 已有（中国区一等公民） | 留（ADR-0022 §4） |
| 领域接线 | 已有（`aiagent`，picoclaw 无对应物） | 留 |

清单里没有「零」而未被处置的项。这是「都必须有」的兑现方式。

### D-C1：A 类包逐个裁决，不做整体替换

| picoclaw | 本仓 | 裁决 | 依据 |
|---|---|---|---|
| `agent` | `agentcore`(1819) | **换** | ADR-0027 §1 |
| `tools` | `agenttools`(1806) | **留** | 已含治理、权限、审计、Review Gate；picoclaw 只有 `tool_allowlist` |
| `skills` | `agentskill`(857) | **留** | 已有受控注册与合约版本化 |
| `providers` | `llm`(1086) | **留** | ADR-0022 §4，中国区 endpoint 一等公民 |
| `auth`/`identity`/`credential` | `access` + `middleware` | **留** | ADR-0022 §3 + 底线 1；picoclaw 无租户模型 |
| `session` | 散在六处 | **重建**（见 D-C4） | 有存储无管理者 |
| `events`/`state` | `agentcore` 内 | **随内核换** | 与 loop 同生命周期 |
| `config` | `config` | **留** | 已接既有配置体系 |
| — | `aiagent`(6603) | **留** | 领域接线，picoclaw 无对应物 |
| — | `agentrunner`(2455) | **留** | checkpoint 与租约恢复，ADR-0022 明言 pi 缺此而本产品需要 |

### D-C2：Context Engineering 分三级，先计数再预算最后压缩

**顺序不可颠倒。** 没有 token 计数就没有预算，没有预算就无法判断何时该压缩——直接上压缩等于凭感觉裁剪。

1. **计数**：token 计数必须与所用模型的分词器一致，不得用字符数或词数估算。估算的误差会在预算边界处系统性失效，而那正是唯一重要的地方。
2. **预算**：按模型可配置的上下文预算，暴露为可观测指标。这条同时兑现 ADR-0022 Non-goals 里那个从未可被满足的重入条件——「观察到真实溢出」需要先有观察手段。
3. **压缩**：仅在预算触发时执行。

### D-C3：压缩是呈现层裁剪，不是记录删除

被压缩的内容**不从 `ai_chat_messages` 删除**。压缩只影响送给 LLM 的消息序列，原始记录完整保留，可随时取回。

**审计承载内容永不参与压缩**：工具调用及其参数与结果、Artifact 引用、审批与 Review 动作、`scope_denied` 等权限结论。这类内容是证据链的一部分；把它压掉，一个通过了 Review Gate 的 run 就不再可复演。

**与底稿溯源冲突时溯源优先**（ADR-0027 §4 已记为 D31）。压缩带来的是对话流畅，溯源带来的是结论可信——后者不可交易。

### D-C4：Session Manager 是一个模块，接口只回答生命周期问题

散在六处的现状要收拢成单一所有者。接口只暴露生命周期语义，不暴露存储细节：获取或创建、续接、结算、淘汰。存储经端口注入，模块本身不做 IO——因而全部生命周期分支可以不起库测试。

**会话严格绑定 `legal_entity_id`**，且该值由与 JWT 同一个解析器产出，不接受调用方传入（与 ADR-0027 §3 对渠道身份的要求同构）。

**并发控制在模块内**：同一会话的两条消息不得交叉执行。当前这个语义分散在 `aichat` 与 handlers 之间，正是「修一处漏一处」的典型形状。

### D-C5：MCP 工具必须穿过治理链，且默认不可用

MCP 是能力面的补齐，不是治理的例外。经 MCP 引入的工具与一等工具走**同一条**六前三后链：租户 scope、能力检查、受保护度量、预算闸、幂等闸、Review Gate。

**默认不可用、需显式登记。** ADR-0022 Non-goals 写明「No open extension ecosystem. Tool registration stays controlled」——这条不因 MCP 而松动。装上即可用的生态与本产品的审批模型不相容。

### D-C6：治理链移植后 ACORE-2 九项逐项重新证红证绿

ADR-0027 §3 已定，此处重述因为它是整个 C 批次的**中止条件**：九项变异一项不少地在新挂点上先证红再证绿，否则迁移停止而不是带着削弱的链继续。

「测试改成匹配新实现所以绿了」不算通过——那正是让 `fpna.assumptions.suggest` 静默消失几个月的同一种失败：绿色覆盖在一条没有东西真正经过的路径上。

### D-C8：子任务委派的每个子回合独立经过治理链与租户绑定

picoclaw 的 `subturn` / `turn_coord` 让一个回合可以派生子任务。这是本 spec 唯一一处「新增能力同时新增攻击面」的地方，因此约束比其余各项都紧：

- **子回合不继承父回合的放行结论。** 每个子回合的每次工具调用独立走完整治理链——父回合通过了 Review Gate 不代表子回合的调用免检。放宽这一条，子任务就成了绕过审批的通道。
- **子回合的 `Principal` 与 `Scope` 由父回合原样传递，不可扩大。** 不允许子任务以任何方式取得父任务没有的权限；缩小可以，扩大不行。
- **深度与并发有上限且可配置**，超限即拒绝。无界递归在单人团队里没有人会看着它跑。
- **子回合的工具调用同样进审计**，且能追溯到父回合。断了这条链，一次委派就成了审计盲区。

反向测试：构造一个子回合尝试调用父回合 scope 外的数据，断言被拒且原因是 `scope_denied`；构造超过深度上限的委派，断言被拒。

### D-C7：分层交付，每层可独立回退

三层之间是地基关系而非原子事务。第一层（内核）不通过则后两层不启动；第二层内部，Context 与 Session 可并行；第三层每个能力独立增量。

任何一层都不得在上一层的验收未通过时开工。

### 契约变更清单

| 类型 | 变更 |
|---|---|
| 替换包 | `agentcore` → picoclaw `pkg/agent` 适配版 |
| 新增包 | session manager、context engineering（计数/预算/压缩）、MCP 接入层 |
| 新增第三方代码 | picoclaw `pkg/agent` 及其最小依赖骨架（MIT，vendor 非 module 依赖） |
| 新增可观测指标 | token 用量与上下文预算占用 |
| 新增配置 | 按模型的上下文预算、压缩阈值、会话淘汰策略、MCP 登记表 |
| 表结构 | **本批次不改表**。七张 `ai_chat_*` 表沿用；压缩不删记录因而无需新表 |
| ADR | ADR-0027 扩写（或续 ADR-0028）以覆盖本 spec 范围 |

## Testing Decisions

### 什么算好测试

只测外部行为。自检句照旧：**把被测逻辑删掉或改错，这条测试会不会红？不会红就是没写对。**

### 各模块的测试与先例

| 模块 | 接缝 | 测试方式 | 先例 |
|---|---|---|---|
| 治理链移植 | `ToolInterceptor`/`LLMInterceptor`/`ToolApprover` | **ACORE-2 九项变异逐项重跑** | `agentcore/hooks/mutation_test.go` |
| 内核纯度 | import guard | 断言不 import `database/sql`/`net/http`/`repository`/MinIO | `agentcore/importguard_test.go` |
| token 计数 | 纯函数 | 已知文本 × 已知模型的 golden 值 | `agenttools/protocol_test.go` 的表驱动 |
| 上下文压缩 | 纯函数（消息序列进、消息序列出） | 审计承载内容在压缩前后逐条比对**必须完全一致** | — |
| Session Manager | 生命周期接口（存储经端口注入） | 全部分支不起库；跨法人隔离带库 | `handlers/ai_chat_runtime_permissions_test.go` |
| MCP 治理 | 同一条治理链 | MCP 工具与一等工具走同样的拒绝路径 | `agenttools/runtime_test.go` |
| 工具注册完整性 | 运行时枚举 | 迁移前后工具数与名单一致 | `aiagent/registration_completeness_test.go` |

### 反向测试要求

- 压缩：构造一个含工具调用与 Artifact 引用的超预算会话，断言压缩后这些内容一条不少
- 压缩：断言被压缩内容仍可从 `ai_chat_messages` 取回
- 预算：断言超限时触发压缩而非直接调用 LLM
- Session：同一会话并发两条消息，断言不交叉执行
- Session：法人 A 的账号取不到法人 B 的会话，且拒绝保持 `scope_denied` 不被软化
- MCP：一个 MCP 工具在权限不足时被拒，拒绝路径与一等工具完全相同
- MCP：未登记的 MCP 工具不可被调用
- 迁移：工具数与名单在内核置换前后一致（当前基线 52 个，33 读 / 19 写）

### 集成测试

跨法人隔离与幂等的证据在集成测试里。**用 `make test-integration`**（可选 `ARGS=`），它起一次性库、跑完销毁、不碰既有 `lease-postgres` 卷。

**skip 掉的集成测试不构成任何证据**——skip 与 pass 在 `go test` 输出里都不是 FAIL。

## Out of Scope

- **`providers` / `llm` 替换**：ADR-0022 §4，`internal/llm` 保留
- **`identity` / `auth` / `credential`**：ADR-0022 §3 + 底线 1，权限与租户模型全部自有
- **`aiagent` 与 `agentrunner` 重写**：前者是领域接线，后者是 checkpoint 与租约恢复，两者 picoclaw 都给不出对应物
- **边缘硬件包**：`audio` / `media` / `devices` / `netbind` / `pid` / `seahorse`
- **`isolation` 沙箱**：D11 已把沙箱后置到阶段 4，进入条件是「有 ≥1 真实客户且合规要求明确」，条件未满足
- **`updater` / `evolution` 自更新**：与本产品的发布与审计模型不相容
- **开放扩展生态**：MCP 接入不等于开放生态，工具注册仍受控（ADR-0022 Non-goals）
- **表结构变更**：本批次不改表
- **Auto-Post Mode**：AI 仍只在 Assist Mode 运行

## Further Notes

**为什么 Context Engineering 排在 Session Manager 之前不是随意的。** 两者都挂在新内核上，但压缩的正确性依赖于「哪些内容不可丢」这个判断，而那个判断的边界（审计承载内容）已经由既有的 Review Gate 与 Artifact 协议定义清楚了；session 的并发与淘汰策略则还需要从散落的六处逆向出现行语义。前者的规格更确定，先做风险更低。

**ADR-0022 Non-goals 那条推迟条款要正式了结。** 它写的是「deferred until a real context overflow is observed」。本 spec 建立观察手段的同时也就满足了重入条件——但要在 ADR 里明确写下这一点，否则日后会有人拿那条 Non-goal 来质疑压缩为何提前做了。

**关于 `unvalidated`。** 本批次同样没有真实 POS/ERP/GL 联调、没有客户验证。runtime 能力增强不改变结论仍基于固定 seed / 构造数据这一事实——不得因为 agent 更强就把它表述为经过验证的经营洞察（风险红线 11）。

**关于「完整」这个词。** 本 spec 的范围是「一个 agent runtime 应有的能力」，判据是能力面而非包数。三十六个包里有七个属于边缘硬件场景、四个已由 ADR 判定不换、两个（`aiagent`/`agentrunner`）在 picoclaw 侧根本不存在。把它们都搬进来不会让 runtime 更完整，只会让它更像别人的产品。
