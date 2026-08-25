# 工单：run / artifact 直读路径的法人边界（SI2）

> 编制：2026-08-25 · 状态：`ready-for-agent`
> 来源：SI1 复验时实测。SI1 把这一面登记为「经会话闸门保护的直连端点」——**实测该说法不成立**，本单纠正并关掉它
> 前置：[SI1](session-cross-entity-isolation-and-ar2-wiring.md) 已交付（`acee42c` / `8960755` / `4bd451a`）

**这不是 SI1 引入的**，是与续接缺口同源的既有问题。SI1 关掉了续接那扇门，这一单关剩下的。**留着它，SI1 的价值会被绕过**——同一份内容，续接路径拒绝，读取路径照给。

---

## 实测事实

以下仓储方法**不接受任何法人边界参数**（不是传错了，是没有这个参数），只按 `user_id` 过滤或完全不过滤：

| 方法 | 过滤 |
|---|---|
| `GetRunByID(ctx, runID, userID)` | 仅 user |
| `GetMessageByID(ctx, messageID, userID)` | 仅 user（JOIN 会话但只比 `s.user_id`）|
| `GetArtifactByID(ctx, artifactID, userID)` | 仅 user |
| `GetReviewActionByID(ctx, actionID, userID)` | 仅 user |
| `GetRunCheckpoint(ctx, runID, userID)` | 仅 user |
| `ListRunEvents(ctx, runID, after, limit)` | **无任何归属过滤** |
| `ListArtifactsBySession` / `ListMessagesBySession` / `ListRunsBySession` / `ListReviewActionsBySession` | 仅 sessionID |

调用它们的 handler **不加载会话**，因而不经过 SI1 建立的任何闸门。两处代表：

- `handlers/ai_chat_runtime.go:329` — `GetRunByID(runID, userID)` → `ListRunEvents(runID, …)` → 直接返回 `{run, events}`
- `handlers/ai_chat_trace.go:42` — 同款，另加 `ListArtifactsBySession(run.SessionID, …)`

### 实测证据（复验探针，今天就是红的）

种一个横跨法人 A / B 的用户，会话与 run 属于法人 A，run 上挂一条含内容的 event，然后照 handler 的调法取：

```
GetRunByID 返回 run 6d87c04c… ; ListRunEvents 返回 1 条 event
```

**取回成功。** 全程没有一个参数能表达调用方的法人。

### 受影响的暴露面

`cmd/api/main.go` 已注册：

- `GET /ai/chat/runs/:id/events`
- `GET /ai/chat/runs/:id/trace`、`GET /agent/runs/:id/trace`
- `GET /ai/chat/runs/:id/stream`
- `GET /ai/chat/artifacts/:id`、`GET /ai/chat/artifacts/:id/export`
- **`POST /ai/chat/artifacts/:id/actions`（写路径）**

**最后一条最重**：`CreateReviewAction` 经 `ai_chat_runtime.go:519` 的 `GetArtifactByID(…, userIDStr)` 取 artifact，仅按 user 校验。这意味着跨法人的不只是读到内容，而是**对另一法人会话的 artifact 执行审批动作**。这一条要单独证、单独修，不要淹没在读路径里。

## 要做的

1. **先证红**，判据同 SI1：测试断言**正确行为**（跨法人取 run/artifact 必须拒绝、原因 `scope_denied`），因此今天写出来就是红的，修完变绿。不要断言当前的泄漏行为。上面那条探针可以作为起点，但**永久测试要断言拒绝**。
2. **照 Part A 已验证过的 `access.EntityFilter` 模式改**，不另发明：边界取自 ctx 里已解析的 scope（`entityBoundary` 已经存在）、零值 fail-closed、global 由解析器验证而非从空字符串推断、拒绝保持 `scope_denied` 不软化。
3. **写路径优先**：`POST /ai/chat/artifacts/:id/actions` 先修先证。
4. **`List*BySession` 系列**：它们只收 sessionID。是给它们加边界参数，还是要求调用方先经过已带边界的会话加载再进入——**两种都行得通，动手前把选择和理由抛回来**。倾向后者的话，要说明怎么保证「不经会话就拿不到」在结构上成立，而不是靠调用方自觉。

## 先回答再动手

- **gateway 平面（`/agent/*`）不在本单默认范围。** 它走机器身份（`execution.Principal`）而非 JWT scope，`entityBoundary` 的 ctx-scope 前提在那里未必成立。先说清楚那条路径的法人从哪来，再决定是本单一并处理还是留给 G9。**不要想当然地把 chat 平面的做法套过去。**
- **streaming 端点**（`/runs/:id/stream`）的边界检查发生在建流前还是每帧，要明确。建流前一次即可的话，说明为什么长连接期间权限变更不构成问题——或者承认它构成，登记下来。

## 验收

- 六个暴露端点各一条测试，断言跨法人拒绝且 `scope_denied` 不软化；写路径单独证
- 删掉边界比对，上述测试必须变红
- 同法人正向路径不回归
- `make test-integration` 实跑非 SKIP（skip 不算证据）
- 全量 `go test ./...` + `go vet` 绿

## 不要顺手改的

- **SI1 已建立的续接闸门不要重做**：`aichat/continuation.go` 与 `runtime.go` 的 `GetSessionByID(…, boundary)` 已经验证过，本单只补它没覆盖的直读面。
- **`sessionmanager` / `contextassembler` 的归属语义不要动**，它们是本单要照抄的参考实现。
