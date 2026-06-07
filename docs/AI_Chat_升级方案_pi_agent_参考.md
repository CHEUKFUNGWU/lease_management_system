# AI Chat 升级方案（参考 Pi Coding Agent）

## 1. 结论

Pi 的价值不在于“长得像一个聊天框”，而在于它把 Agent 运行时拆成了几层可以组合的能力：

- 会话运行时：有流式事件、消息队列、分支、压缩、切换
- 扩展层：可以注册工具、命令、UI 组件、事件钩子
- 嵌入层：既能走 SDK，也能走 RPC，方便接到任意前端

对本项目来说，**最值得借鉴的不是 terminal 交互本身，而是 Agent Runtime 的设计方式**。  
当前我们的 AI Chat 已有“文件上传 + 草稿生成 + 人工确认”的业务闭环，但在运行时层面仍然偏“同步问答接口”。如果要把 AI Chat 升级成真正的“租赁管理主入口”，建议优先补齐：

1. 流式事件总线
2. 服务端会话与审计留痕
3. 可中途干预的消息队列
4. 可扩展技能/工具注册层
5. 面向业务对象的 Artifact 渲染层

## 2. Pi 里值得借鉴的能力

以下内容来自 Pi Coding Agent 的 README、SDK、RPC 和 Extensions 文档：

- Pi README 明确把核心定位成“minimal coding harness”，并把扩展点放在 Extensions、Skills、Prompt Templates、Themes、Pi Packages，而不是把所有能力写死在核心里。
  - 参考：[Pi README](https://github.com/earendil-works/pi/tree/main/packages/coding-agent)
- Pi SDK 的 `AgentSession` 提供了 `prompt()`、`steer()`、`followUp()`、`subscribe()`、`navigateTree()`、`compact()`、`abort()` 等运行时能力。
  - 参考：[Pi SDK](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/sdk.md)
- Pi SDK 的事件流不是只有“最终回答”，而是细分为 `message_update`、`tool_execution_start`、`tool_execution_update`、`tool_execution_end`、`turn_start`、`turn_end`、`queue_update`、`compaction_*` 等。
  - 参考：[Pi SDK](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/sdk.md)
- Pi RPC 支持在流式执行期间插入 `steer` 和 `follow_up`，允许用户在 Agent 尚未结束时中途修正方向，而不是只能等当前请求结束。
  - 参考：[Pi RPC](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/rpc.md)
- Pi Extensions 支持注册工具、命令、UI、自定义组件，并能在消息开始/更新/结束、工具调用等节点注入行为。
  - 参考：[Pi Extensions](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/extensions.md)
- Pi 还支持会话树、分支、切换、标签和压缩，不把“会话”视作单条线性的 message list。
  - 参考：[Pi SDK](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/sdk.md)

## 3. 当前本项目 AI Chat 的现状

### 3.1 前端现状

当前 [web/app/ai-chat/page.tsx](../web/app/ai-chat/page.tsx) 已经具备：

- 独立 AI Chat 页面
- 本地多会话
- 文件上传
- Page Context 透传
- Tool Call、Review Prompt、草稿卡片展示
- 合同草稿与付款计划草稿确认动作

但仍存在明显约束：

- 会话只存在浏览器 `localStorage`，不是服务端会话
- 请求模式是“一次提交 -> 等全部完成 -> 一次返回”
- 没有真正的 token streaming / tool streaming
- 用户不能在执行中插入“改方向/追加要求”
- 工具流是结果回填，不是事件流
- 草稿卡片是硬编码到页面里的，不是统一的 artifact/render 协议

### 3.2 Core Service 现状

当前 [core-service/internal/handlers/ai_chat.go](../core-service/internal/handlers/ai_chat.go) 已经具备：

- 文件上传后的函数调用路由
- `parse_contract_batch` / `parse_payment_schedule` / `parse_contract`
- runbook 风格的计划、工具、复核提示
- 页面上下文检索
- 系统提示词构建

但核心还是同步接口：

- `Chat()` 是单次 HTTP 请求
- `callLLM()` / `callLLMWithTools()` 都是阻塞式等待结果
- 没有“Agent 运行态”的持久化对象
- 没有事件订阅接口
- 没有服务端队列和中途 steering
- 技能选择主要靠 `buildAgentRunbook()` 里的关键字分支，扩展成本较高

### 3.3 AI Service 现状

当前 [ai-service/app/routers/chat.py](../ai-service/app/routers/chat.py) 与 [ai-service/app/services/llm.py](../ai-service/app/services/llm.py) 的职责比较干净：

- AI Service 只负责调模型
- Core Service 负责上下文和工具编排

这是对的，但现状也意味着：

- 还没有 streaming chat API
- 还没有 tool call 的多轮执行循环
- 还没有面向 UI 的事件模型

## 4. 我们不应该照搬 Pi 的部分

Pi 是 coding agent，不是 IFRS 16 审计/台账 Agent。以下部分不建议直接复制：

- 不建议把所有能力都做成开放式扩展市场
  - 这个系统是财务合规系统，工具注册必须受控
- 不建议把 session tree 先做到太重
  - 业务优先级应先放在可审计、可恢复、可中断
- 不建议先上“任意命令式工具生态”
  - 先把合同复核、租金表导入、审计包、月结解释这几个技能标准化

正确做法是：**借鉴 Pi 的运行时抽象，不照搬其开放生态策略。**

## 5. 推荐升级方向

## 5.1 P0：把 AI Chat 从“同步问答接口”升级为“事件驱动会话”

### 目标

让前端不再只拿最终结果，而是拿到完整运行过程。

### 建议能力

- 新增服务端会话 ID 和 run ID
- 新增 SSE 或 WebSocket 事件流接口
- 事件类型至少包括：
  - `message_start`
  - `message_delta`
  - `message_end`
  - `tool_start`
  - `tool_update`
  - `tool_end`
  - `review_prompt`
  - `artifact_ready`
  - `queue_update`
  - `run_end`
  - `run_error`

### 为什么这一步最重要

这是所有后续能力的基础：

- 没有事件流，就做不好“中途干预”
- 没有服务端 run，就做不好“断点恢复”
- 没有统一事件模型，就做不好“多技能/多 artifact 扩展”

### 落地建议

- 在 Core Service 引入 `AgentSession` / `AgentRun` 概念
- `POST /api/v1/ai/chat` 改为：
  - 创建 run
  - 立即返回 `session_id`、`run_id`
- 新增：
  - `GET /api/v1/ai/chat/runs/:run_id/events`
  - 或 `GET /api/v1/ai/chat/stream?run_id=...`

## 5.2 P0：把本地会话改成服务端会话

### 当前问题

现在浏览器清缓存、换设备、换浏览器后，会话全部丢失。

### 升级后

新增数据库表，例如：

- `ai_chat_sessions`
- `ai_chat_messages`
- `ai_chat_attachments`
- `ai_chat_runs`
- `ai_chat_run_events`
- `ai_chat_review_actions`

### 好处

- 符合审计系统的留痕要求
- 可跨端恢复
- 可记录谁上传了什么文件、谁确认了什么草稿
- 后续可把 AI Chat 纳入审计日志和权限体系

## 5.3 P0：引入“中途纠偏”能力

Pi 的 `steer()` / `followUp()` 很值得借鉴。

### 在本系统中的真实场景

- OCR 很慢时，用户想补一句：“这个其实是租金表，不是合同”
- 合同批量解析中，用户想补充：“只导入 LE001 的合同”
- 审计包生成中，用户想补充：“只看 Official，不要 Working”

### 建议做法

前端增加：

- `Steer current run`
- `Add follow-up after run`

后端增加：

- `POST /api/v1/ai/chat/runs/:run_id/steer`
- `POST /api/v1/ai/chat/runs/:run_id/follow-up`

语义上区分：

- `steer`: 当前轮工具执行完、下次 LLM 调用前插入
- `follow-up`: 当前 run 完成后再执行

## 5.4 P1：把 runbook 关键字路由升级为“技能注册中心”

当前 `buildAgentRunbook()` 虽然已经有 skill 雏形，但本质还是硬编码分支。

### 建议目标

做一个受控的 `Skill Registry`，每个技能有统一定义：

- `id`
- `display_name`
- `intent_examples`
- `required_inputs`
- `required_context`
- `tool_chain`
- `artifact_types`
- `review_policy`
- `allowed_roles`

### 第一批技能

- `contract_batch_intake`
- `contract_review`
- `payment_schedule_intake`
- `audit_pack`
- `monthly_close_explainer`
- `report_explainer`

### 好处

- 降低 `ai_chat.go` 的分支复杂度
- 技能可以配置化测试
- 后续更容易做技能面板、权限控制、A/B 版本切换

## 5.5 P1：统一 Artifact 协议，替代硬编码草稿卡片

当前前端已经有：

- 合同草稿卡片
- 付款计划草稿卡片

但它们仍然是特例。

### 建议统一成

`artifacts: []`

每个 artifact 至少有：

- `artifact_id`
- `type`
- `title`
- `status`
- `data`
- `actions`
- `evidence_refs`
- `review_required`

### 适用对象

- 合同草稿
- 付款计划草稿
- 审计包目录建议
- 数据质量异常清单
- 报表解释卡
- 月结阻塞项清单

这样 AI Chat 才会从“只会聊天”变成“能驱动业务对象”的前台。

## 5.6 P1：证据工作区（Evidence Workspace）

Pi 强调工具输入和事件可见性。  
本系统更需要“证据可见性”。

### 建议在 AI Chat 右侧增加证据面板

- 已上传文件
- 当前绑定合同
- 当前报表上下文
- 当前期间/法人/品牌过滤器
- OCR 状态
- 解析状态
- 证据引用位置

### 业务收益

- 用户明确知道 AI 现在“看到了什么”
- 降低越权误解和错误解释
- 更符合审计复核场景

## 5.7 P2：会话分支与 checkpoint

Pi 的 session tree / fork 很适合我们的“复核分支”场景。

### 在本系统中的适用场景

- 同一份合同，两种租赁范围判断
- 同一份付款计划，两种变量租金处理
- 同一期间，两种 Official/Working 分析口径

### 建议不是照搬通用 tree UI

先做业务友好的 checkpoint：

- 保存当前分析为 checkpoint
- 从某个 checkpoint 重开一条分析分支
- 给 checkpoint 打标签，例如：
  - `scope-review`
  - `official-pack`
  - `closing-2024-01`

## 5.8 P2：上下文压缩（Compaction）

Pi 的 compaction 很适合长会话。  
我们的 AI Chat 如果要成为真正主入口，长会话一定会出现：

- 一次合同批量导入过程可能持续几十轮
- 一次审计包生成可能跨多个导出动作

### 建议

不是做通用摘要，而是做**会计安全压缩**：

- 保留已确认结论
- 保留未解决风险点
- 保留证据引用
- 保留用户显式确认过的判断
- 不压缩关键金额、期间、合同编号

## 5.9 P2：命令式技能入口

Pi 有 prompt template / command 体系。  
我们可以做受控命令，而不是开放 shell 风格命令：

- `/合同复核`
- `/租金表导入`
- `/审计包`
- `/解释月结`
- `/解释报表`

这样能减少意图识别误差，也让重度用户效率更高。

## 6. 推荐目标架构

```text
Web AI Chat
  -> Chat Session API
  -> Run API
  -> Event Stream API (SSE/WebSocket)
  -> Review Action API

Core Service
  -> Session Manager
  -> Run Orchestrator
  -> Skill Registry
  -> Tool Executor
  -> Artifact Builder
  -> Audit Logger

AI Service
  -> LLM Chat / Tool Call / Streaming Adapter
  -> OCR / Parse services
  -> Structured extraction

PostgreSQL
  -> ai_chat_sessions
  -> ai_chat_messages
  -> ai_chat_runs
  -> ai_chat_run_events
  -> ai_chat_artifacts
  -> ai_chat_review_actions

MinIO
  -> uploaded evidence
  -> OCR intermediate outputs
  -> artifact exports
```

## 7. 最推荐的三阶段落地顺序

## 阶段一：先做运行时底座

范围：

- 服务端 session/run
- SSE 事件流
- 流式消息/工具状态
- 前端改成 event-driven UI

这一阶段完成后，AI Chat 的“质感”会有明显升级。

## 阶段二：再做技能与 artifact 抽象

范围：

- Skill Registry
- Artifact 协议
- Evidence Workspace
- review action 统一接口

这一阶段完成后，AI Chat 才会真正成为“主工作台”。

## 阶段三：最后做高级能力

范围：

- steer/follow-up
- checkpoint / branch
- compaction
- 命令式技能入口

这一阶段是把好用变成非常好用。

## 8. 对当前代码的最直接改造建议

### 前端

优先改 [web/app/ai-chat/page.tsx](../web/app/ai-chat/page.tsx)：

- 把“等待完整响应”改成事件订阅渲染
- 把 toolCalls/reviewPrompts/draftContracts/draftPaymentSchedules 改成统一 artifact/event model
- 新增右侧 `Evidence Workspace`
- 新增 run 状态条和 queue 状态条

### Core Service

优先改 [core-service/internal/handlers/ai_chat.go](../core-service/internal/handlers/ai_chat.go)：

- 拆分 `Chat()` 为：
  - session 管理
  - run 创建
  - 事件发布
  - skill 执行
- 把 `buildAgentRunbook()` 抽成 `Skill Registry`
- 把 `parseContractBatch` / `parsePaymentSchedule` 的结果包装成标准 artifact

### AI Service

优先改 [ai-service/app/routers/chat.py](../ai-service/app/routers/chat.py) 与 [ai-service/app/services/llm.py](../ai-service/app/services/llm.py)：

- 增加 streaming chat
- 为 tool call 循环返回更细粒度中间事件
- 保持“AI Service 只管模型调用，Core Service 负责编排”的职责边界

## 9. 一句话优先级建议

如果这周只能做一件事，就先做：

**“服务端会话 + SSE 事件流 + 前端流式工具轨迹”**

因为它最接近 Pi 的核心优势，也最能直接提升你们 AI Chat 作为主入口的可用性和可信度。
