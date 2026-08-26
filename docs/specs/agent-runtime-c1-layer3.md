# 工单：C1 第三层（L3，四部分 · 范围已裁剪）

> 编制：2026-08-25 · 状态：**四部分全部交付并复验（2026-08-26），仅余 A-3 观察窗口出数**
> 相关：[母 spec C1](agent-runtime-overhaul-c1.md) · [模块深化 AR1–AR6](../CodebaseDesign_AgentRuntime升级_模块深化.md) · [ADR-0028 + Correction](../adr/0028-extend-picoclaw-adoption-to-the-whole-runtime.md) · [AI 文档索引 §5 未决 #9 / #12 / #14](../AI_文档索引与现行决策.md)

前两层 2026-08-25 验收通过，D-C10 的分层纪律条件已满足，第三层可开工。

## 交付状态（2026-08-26）

| 部分 | 提交 | 状态 |
|---|---|---|
| **L3-A** 三个尾巴 | `032328a` | A-1 未决 #12 重登记 ✅ · A-2 Story 18 延期登记 ✅（索引 §5 #9 + G10）· **A-3 观察窗口采集中**（口径 `tmp/L3-A-observation-plan.md`，≥7 天，2026-08-26 起） |
| **L3-B** 健康检查 | `1a73bc1` | ✅ 交付并复验 |
| **L3-C** 定时触发 | `5cc1953` | ✅ 交付并复验；复验补 `TestLeaseRecoveryJobDoesNotResurrectCompletedRuns`，副产 G11 事实登记 |
| **L3-D** MCP 接入 | `7b2f1c2` + 返工 `8607fa2` | ✅ 交付，**经两轮复验返工后通过**：客户端层七个缺陷（超时毒化 / 并发数据竞争 / 握手无超时 / 无界行缓冲 / 写阻塞无视 deadline / 白名单对省略 `type` 全绕过 / server 主动请求吃掉 pending id）各配回归测试并逐个变异实证 |

**本工单唯一未完项是 A-3 的数据小结。** 数据到手后：切不切 `CONTEXT_ASSEMBLER_MODE=on` 另开决策票；模型路由的重开条件（D44）随之满足；本工单完结并按索引 §1.5 惯例删除。

收尾登记四条已于 2026-08-26 完成，见本文末「收尾登记」节。

## 范围裁决（2026-08-25）

第三层原定六项能力，本票只做三项：

| 能力 | 本票 | 理由 |
|---|---|---|
| **MCP 接入** | ✅ 做 | 唯一一项能减少后续工作量的——接外部工具不必每次改 `aiagent` |
| **定时触发** | ✅ 做 | 有一个现存缺口靠它才能关（见 L3-C：`RecoverRunLeases` 两侧都实现了，**全仓零调用方**） |
| **健康检查** | ✅ 做 | 现有 `/health` 是「假装在检查」的活体样本（见 L3-B），修它不是新增能力而是修正既有谎报 |
| 子任务委派（AR4） | ⏸ 推迟 | 新增能力同时新增攻击面（D-C8），且是六项里唯一需要 picoclaw 完整回合循环做地基的（未决 #12）。当前没有「华东十家店逐店归因」这类实际负载在等 |
| 模型路由 | ⏸ 推迟 | 成本/质量优化，在没有生产流量数据前无判据。等 L3-A 的观察窗口出数再定 |
| 跨会话记忆（AR6） | ⏸ 推迟 | 依赖 AR3 真正在生产上跑过（L3-A 的前置尚未完成），且它是新的跨租户泄漏面 |

推迟三项**不是取消**：`ContextKey` 的五维隔离键与 `narrow` 求交的设计留在模块深化文档里，未来重开时接口不用重画。本裁决要写进 README 与文档索引（见「收尾登记」）。

## 顺序与依赖

| 部分 | 内容 | 依赖 | 位置理由 |
|---|---|---|---|
| **L3-A** | 三个尾巴 | 无 | A-1 是纯裁决（半天）；A-3 要跑观察窗口，越早开始越好，它与 B/C/D 并行 |
| **L3-B** | 健康检查 | 无 | 最小且修的是现存谎报；它给 A-3 的观察期提供最基本的「服务是否真活着」判据 |
| **L3-C** | 定时触发 | **A-1** 裁决完成 | 定时任务的 Principal 从哪来，与未决 #14（worker→法人绑定）同源，不能绕过 |
| **L3-D** | MCP 接入 | **L3-B** 交付 | MCP 引入进程外依赖，没有能反映依赖真实状态的健康面就没法运维 |

**四部分各自独立提交**，理由同 RT1：把 MCP 的问题和健康检查的问题绑在一个提交里，其中一个要回滚，另一个陪葬。

---

## L3-A：三个尾巴

开工前必须清掉的三项，两项是裁决，一项是运营动作。

### A-1：未决 #12 的阻塞范围重新登记（纯文档）

索引 §5 第 12 项写着 picoclaw 完整回合循环启用「阻塞 AR3/AR4 的地基选择」。

**实测：这条现在不成立于 AR3，且本票范围内不成立于任何一项。**

- `internal/agentkernel/third_party/picoclaw/agent/` 十个文件里**七个挂 `//go:build picoclaw_agent_core`**（pipeline 六件套 + `event_payloads.go`），默认不编译；真正参与编译的只有 `hooks.go` / `events.go` / `turn_context.go`
- 治理链消费的是 `picoclawagent.ToolInterceptor` / `HookDecision` 这组 **hook 符号**，不是回合循环——这正是 ADR-0028 Correction 说清楚的那件事（"first-party code references only picoclaw's hook symbols"）
- AR3 已交付并接线；MCP / 定时 / 健康检查都不碰回合循环

要做的：把第 12 项的「阻塞什么」改成如实描述——**只阻塞 AR4 子任务委派**（`turn_coord` / `subturn` 在 tagged 文件里），且 AR4 已按本票范围裁决推迟，因此**当前不阻塞任何在途工作**。同时把「约 6,300 行闭包」的接手清单指向 `tmp/blocked-AR5b.md` 保持有效。

不要顺手去掉 build tag。那是独立票。

### A-2：Story 18 会话状态迁移可追溯（未决 #9）

索引 #9 给了两条去向：进六处调用方接线票范围，或登记延后。**接线票已经完成（G9 已关闭），第一条路走不通了**，所以只剩实现或正式延期，不能继续悬着。

**实测缺口（2026-08-26 订正：本段原写「迁移发生在三处」是错的，实际只有创建一处是状态写入）**：`internal/sessionmanager/session.go:53` 的 `Status string // active | archived | closed` 声明三个状态，但全仓唯一写路径是创建时的 `active`：

- `manager.go:235` `newSessionFromKey` → `active` —— **唯一的状态写入**
- `manager.go:249` `Close` —— 缓存结算（把锚 Save 回 Store），**不改 Status**，不是迁移
- `manager.go:297` `evictExpired` —— 纯缓存淘汰（代码注释明说不触 Store），既不改 Status 也不是会话生命周期事件

即：archived/closed 是零产生者的死值，**不存在可追溯的迁移**。Story 18 要的「复演一次对话的完整经过」今天由 runs/messages/events 时间线承载，不缺 status 这一环。

**裁决（2026-08-26）：延期登记，理由是前提缺失而非成本**——现在实现 tracing = 给不存在的迁移造永远为空的日志，与正常工作的无法区分。登记落在 AI 文档索引 §5 #9；三值承诺未兑现的事实单独登记为索引 §3 G10。延期不影响上述两条硬约束的效力，它们随重开条件一并生效：

1. **不得把 `legal_entity_id` / `user_id` 撒进日志**——`agentcontext.ContextKey` 特意不实现 `String()` 就是为了这个（`key.go` D-C11 注释），RT1-A 的指标标签纪律是同一条纪律的另一个出口
2. **`sessionmanager` 不做 IO**（D-C4，存储经端口注入）。留痕要么经已有的 Store 端口，要么经一个新的窄端口，不能让 manager 直接拿到库句柄

**重开条件（带牙齿）**：任何引入 archived/closed 写路径的改动必须同票携带 (a) 迁移留痕——经 Store 端口扩展或新窄端口；(b) 留痕不含法人/用户标识；(c) 反向测试。届时 G10 一并关闭。

### A-3：AR3 在生产流量上打开到 `count`

装配器已接线、评审整改已验、指标已交付，**至今没在生产流量上跑过一次**。RT1-A 特意把开关做成两级就是为了这一步：`CONTEXT_ASSEMBLER_MODE=count` 只计数、只上报、不压缩（`cmd/api/main.go:159-180`）。

要做的：

1. 生产环境置 `CONTEXT_ASSEMBLER_MODE=count`，**先确认 `ValidateModelCoverage` 对当前 `llm.ConfigFromEnv().Model` 通过**——它是 `log.Fatalf`，配错了服务起不来（AF1-c 的 fail-fast 是故意的，别为了让它起来去放宽校验）
2. 观察窗口**不少于 7 天**，收集占用率分布（实测 vs 估算分开看，RT1-A 已把两者分开暴露）
3. 窗口结束后产出一份数据小结：占用率的 p50/p95/max、压缩若开启会在多少比例的回合触发、撞墙前信号的实际触发频次
4. **本票不负责切到 `on`**。切不切、阈值定多少，拿这份数据另开决策

验收：只计数模式下 prompt 内容**逐字节不变**（RT1-A 已有这条测试，本部分只需实证它在生产配置下也成立）；观察小结落到 `tmp/` 或文档，不要只留在对话里。

---

## L3-B：健康检查

### 先订正一处 spec 事实

C1 spec 的能力清单写着「健康检查 | **零**（全仓无 `/health` 类端点）」。**这句是错的**：`cmd/api/main.go:249` 有一个 `/health`。

但它比没有更糟：

```go
r.GET("/health", func(c *gin.Context) {
    dbStatus := "ok"
    if err := database.HealthCheck(ctx); err != nil {
        dbStatus = "error: " + err.Error()
    }
    c.JSON(200, gin.H{"status": "ok", "service": "core-service", ...})
})
```

**数据库探测失败时，它仍然返回 200 且 `"status": "ok"`**，只把错误塞进一个附属字段。任何按状态码或按 `status` 字段判活的探针——k8s、负载均衡器、`docker-compose` 的 `healthcheck`——都会认为服务健康。

这是风险红线 12「假装在检查」的又一处形态，与 2026-08-23 那次 anydoc SRI 同类：控制项存在、从不为假。索引 §5.1 已经为构建期守卫写过这个教训，运行期守卫是同一类。

第二个问题：`"error: " + err.Error()` 把内部错误原文吐给任何未认证调用方。健康端点必须公开（探针没有凭据），因此不能带内部细节。

### 要做的

1. **状态随依赖真实变化。** DB 探测失败时状态必须为不健康——状态码与 `status` 字段两处都要变，不能只改一处（探针可能只看其中之一）。
2. **区分 liveness 与 readiness。** 进程活着但 DB 断了：liveness 应该通过（重启救不了 DB），readiness 应该失败（别往这台上转流量）。做成两个端点还是一个端点带参数，由你定，说明理由。
3. **依赖清单**：至少 postgres、MinIO。**LLM provider 不主动探活**——那是花钱的调用，改用最近一次真实调用的成功/失败缓存作为信号。这条是硬约束，不要为了「全面」去打 provider。
4. **不泄露内部细节。** 公开面只给健康/不健康与依赖名，错误原文进日志不进响应体。
5. **`agent-runner` 目前没有任何 HTTP 面**（`cmd/agent-runner/main.go` 只有出站 `http.Client`），因此结构上不可探活，只能靠进程存活判断。Story 30 要的是「runtime 与其依赖的实际状态」。给它一个最小健康端口，还是登记为「按进程判活 + 用 worker 心跳作为间接信号」，**动手前抛方案**——后者可能是对的，`HeartbeatRunLease` 确实是一个真实的活性信号，但要说清楚谁在读它。
6. **`docker-compose.yml` 补 `healthcheck`**：postgres（:16）和 MinIO（:34）都有，`core-service`（:40）和 `agent-runner`（:89）都没有。

### L3-B 验收

- **核心反向测试**：断开数据库（或注入失败的探测端口），断言健康端点**不再报健康**。把判断改回恒 200 必须变红——本项的全部价值就在于它不恒真，这条测试不存在则本项等于没做
- 断言响应体不含内部错误原文（把 `err.Error()` 拼回去必须变红）
- liveness 与 readiness 在「进程活 + DB 断」这一情形下给出不同结论，有测试
- 全量 `go test ./...` + `go vet` 绿；`make test-integration` 实跑非 SKIP

---

## L3-C：定时触发

### 有一个现存缺口靠它才能关

`RecoverExpiredRunLeases`（`internal/repository/agent_run_queue.go:141`）把过期租约重新入队。它经 `handlers/agent_gateway_queue.go:134` 暴露成 `POST /api/v1/agent/runs/recover-leases`（`cmd/api/main.go:589`），runner 侧也实现了客户端 `HTTPGateway.RecoverRunLeases`（`internal/agentrunner/http_gateway.go:204`）。

**全仓没有任何东西调用它。** 两侧都实现了，`compose`、`ops/`、`scripts/`、Makefile 里都没有外部触发器。也就是说**过期租约目前永远不会被重新入队**，除非有人手工 POST。

这是本项的第一个用户，也是它的验收标的——比 Story 29 的经营日报更适合打头阵：无 LLM、无租户身份问题、缺口已实证。

### 要做的

**1. 调度产物是一条 run，不是一条旁路。** 定时任务入既有队列（`ai_chat_runs` + 租约），走同一条治理链、同一套审计。任何「定时任务直接调工具」的形状都是绕过审批的通道，不接受。

**2. 幂等靠既有闸，不新造。** 多副本部署下同一任务会被触发多次。`ToolDescriptor.SupportsIdempotency` 与 `IdempotencyGuard`（`governance.go:552`）已经在链上，调度产生的 run 用 `job_id + 触发时刻` 作幂等键，重复触发被既有控制拦住。**不要引入 leader election** 来解决一个已有控制能解决的问题——除非你能证明幂等键路线不成立，那就抛方案。

**3. Principal 从哪来，是本部分最难的一条。** 定时任务没有请求上下文，但每次工具调用都要过 `TenantScope`（链上第一控制）。约束：

- 身份来自**登记表**，不是运行时构造
- 登记的 scope **不得宽于登记者当时的 scope**——照 D-C17 `narrow` 求交的思路，不要允许字面量构造
- 与未决 #14（worker→法人绑定）同源：worker 池从共享队列取队、不按法人分区。定时任务把「谁的身份在跑」这个问题从用户轴挪到了服务轴，**这一条动手前抛方案**，且方案要说清它与 #14 的关系（是同一个决策的两面，还是可以分开定）

**4. v1 范围建议：只做调度框架 + lease recovery 一个 job。** 经营日报（Story 29）作为第二个提交——它需要 Principal 决策落地，而 lease recovery 是纯运维动作、不需要法人身份（它跨所有法人重排队列，这本身要在方案里说清是否可接受）。

**5. 表结构。** 需不需要新表（job 登记 + 上次触发时刻），还是配置文件够用，由你定。有新表则迁移与 `01_init.sql` 空库版本必须同时提供——RT1 那批已经踩过这条（迁移 062 的登记）。

### L3-C 验收

- **反向测试**：同一 job 在同一触发时刻被触发两次，断言只产生一次执行；把幂等键去掉必须变红
- **反向测试**：未登记 Principal 的 job 拒绝启动（不是静默跳过——静默跳过和成功在日志里长得一样）
- lease recovery job 有集成测试：造一条过期租约，跑一轮调度，断言它回到队列
- ~~调度产生的 run 在审计里与用户发起的 run 可区分~~ **转移（2026-08-26）：v1 无 run 可谈**——lease recovery 被裁决为 Type A 系统维护型（见下），其可区分痕迹是 requeue 自带的 error_message 与 queue_update（reason=lease_expired）trace；run 级区分随第一个 Type B 任务落地时兑现
- 全量 `go test ./...` + `go vet` 绿；`make test-integration` 实跑非 SKIP

**L3-C 裁决落地（2026-08-26，复验人批准）**：

1. **lease recovery 不进治理链**。九控制全是围绕 ToolCall 的闸门；给不调工具的运维动作造假 ToolCall 过链本身是新的说谎面。本段「走同一条治理链」的禁令针对的是「定时任务直接调工具绕过审批」——lease recovery 不是那个形状。结构性守卫：internal/scheduler 仅依赖标准库（import guard 双向测试含种违规 fixture），Registration.Run 签名无任何 runtime/registry 参数。
2. **任务两类分治**：Type A 系统维护型（v1 三件：lease recovery、capability cleanup、auth-refresh cleanup——后两者是 main.go 既有手写 ticker 的吸收）与 Type B 业务运行型（Story 29 等）。跨法人豁免、边界、与 #14 的关系登记为决策 D39（AI 文档索引 §2）。
3. **幂等谓词三重冗余**（status='running' / leased_until<NOW / SET 置空 leased_until）：任一存活即保住幂等，单点变异打不穿是设计强度而非测试弱点；变异自检用双点失效验证（去掉 status 谓词且不清 leased_until → once-only 断言与活租约保护各红一条）。

---

## L3-D：MCP 接入

### 硬约束（D-C5 + ADR-0028 Non-goals，不可协商）

- MCP 引入的工具走**同一条**九控制链：TenantScope → CapabilityCheck → ProtectedMeasure → BudgetGuard → IdempotencyGuard → ReviewGate → AuditRecorder → ArtifactCollector → MetricsRecorder（`governance.go:546`）
- **默认不可用，需显式登记。** 一个未经登记就能被调用的工具，就是一条绕过权限、审计与 Review Gate 的路径

### 本部分的核心设计判断：信任边界画在哪

MCP server 的 `tools/list` 会自报工具元数据。**这份自报不得用来决定治理属性。**

看一眼 `ToolDescriptor.Validate()`（`internal/agenttools/protocol.go:70`）就知道为什么：`Level` 决定 `CapabilityCheck` 放不放行，`ReadOnly` 决定它算不算写操作，`Permissions` 决定要哪些权限，`Review` 决定过不过 Review Gate。如果这四个字段来自外部 server，那么一个 server 只要自称 `level: read, read_only: true` 就能取得读权限并跳过 Review Gate——**登记就退化成了自我登记**。

因此：

- **`ToolDescriptor` 由本仓登记清单提供**（谁能调、什么级别、要不要复核），是 first-party 事实
- MCP server 提供的 schema **只用于参数校验与调用编码**，不参与任何治理判断
- 登记清单里的名字与 server 上的名字对不上 → 拒绝注册，不做模糊匹配

### v1 范围建议

- **只允许 `LevelRead` 且 `ReadOnly` 的 MCP 工具。** 写能力经外部进程是另一个量级的风险面，留到有实际需求时按证据重开。这条写进登记校验，不是靠人守
- 传输方式选一种（stdio 或 HTTP），不要两种都做
- **出站数据是新的出口，必须单独回答**：MCP server 是进程外的第三方。哪些数据允许出去？法人标识、受保护度量、门店明细？默认应当是「工具入参里不含租户标识，只含已解析结果」，但这需要你按实际接入场景论证。**这一条动手前抛方案**，它是底线 1 和底线 2 在一个新出口上的落点
- 超时与失败：MCP server 挂掉不得让整个回合挂住。`ToolDescriptor.TimeoutSeconds` 已有，用它

### L3-D 验收

- **反向测试**：未登记的 MCP 工具被拒，拒绝码与一等工具的未知工具一致（不软化）
- **反向测试**：登记为 `command` 的工具，其 MCP server 自报 `read_only: true`——断言以**登记**为准仍要求 Review。把优先级改成以 server 自报为准必须变红。这是本部分最重要的一条测试
- MCP 工具的一次调用在审计里留下与一等工具**同样的九控制痕迹**（不是「大部分」）
- MCP server 超时/崩溃时回合正常收尾并留痕，不挂住
- 全量 `go test ./...` + `go vet` 绿；`make test-integration` 实跑非 SKIP

---

## 收尾登记

四部分全部验收后（**2026-08-26 已全部完成**）：

- ✅ README 状态行：C1 从「前两层完成，第三层未开始」改为如实描述——三项已做、三项按 2026-08-25 裁决推迟，**不写成「C1 完成」**
- ✅ 文档索引 §5：第 9 项按 A-2 转为带日期的延期（并新登记 G10）；第 12 项按 A-1 重写阻塞范围；第 14 项按 L3-C 的 Principal 方案更新（并入 Type B 登记表来源，与 worker→法人绑定同场裁决）
- ✅ 母 spec C1 的能力清单：健康检查那一格的「零（全仓无 `/health` 类端点）」按 L3-B 订正
- ✅ 推迟的三项（AR4 / 模型路由 / AR6）在索引留可检索登记 **D44**，含推迟日期与重开条件；模块深化 AR4/AR6 行同步标注

**同轮附带**（超出本票原定范围，因删除已完结工单需先满足「结论已吸收」前提）：索引补 **D41**（RC1 拒绝码保真）、**D42**（SI1+SI2 直读路径法人边界）、**D43**（实测与估算不得合并），随后删除五份已完结的 C1 工单，见索引 §1.5「2026-08-26 的收缩」。

## 贯穿全票的纪律

- **四部分各自独立提交**，一张票不等于一个提交
- **先证红**：测试断言正确行为，因此今天写出来就是红的，修完变绿。不要断言当前行为再回头改测试
- **反向变异自检**：把修好的逻辑改回错的，新测试必须变红。本票有三条测试的全部价值就在于它们不恒真（L3-B 的健康判断、L3-C 的幂等、L3-D 的登记优先级），这三条尤其不能省
- **`make test-integration` 实跑非 SKIP**——skip 与 pass 在 `go test` 输出里都不是 FAIL
- **标注了「先抛方案」的地方不要跳过**：A-2 的 evict 留痕归属、L3-B 第 5 条 runner 探活、L3-C 第 3 条 Principal 来源、L3-D 的出站数据边界
- **结论句不得比证据强一档**。交付登记里写「已验证」的每一句，都要能指到一条实跑过的测试或一段实测输出
