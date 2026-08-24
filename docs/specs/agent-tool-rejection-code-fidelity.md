# 工单：治理链拒绝的错误码保真（RC1）

> 编制：2026-08-24 · 状态：`ready-for-agent`
> 来源：C1 批次评审（`bcdb8a4..80222b5`）的附带发现，当时明确**不塞进 AF1–AF4，单独开票**
> 相关：[C1 批次评审整改（AF1–AF4）](agent-runtime-c1-review-fixes.md) · [母 spec](agent-runtime-overhaul-c1.md)

**这不是本批次引入的缺陷。** `bbdec17` 之前就是这个样子。开票的理由是它的代价变了：治理链现在真的在保护生产 chat 流量（AR5d），所以它的拒绝语义从「保护一个没在跑的内核」变成了「线上每一次工具拒绝的对外表述」。

---

## 现状

`agenttools/runtime.go` 的 `execute` 有两条路径，错误码保真度不同。

**未装 guard 时**（`Evaluate` 路径），策略错误经 `policyErrorCode` 分型：

| 拒绝原因 | 错误码 |
|---|---|
| `ErrExecutionContextRequired` | `unauthenticated` |
| `ErrToolCapabilityRequired` | `capability_denied` |
| `ErrToolNotPermitted` | `permission_denied` |
| 其余（含 dry_run 不支持、缺 idempotency_key、descriptor 版本不匹配） | `invalid_arguments` |

对外文案另经 `publicPolicyError` 做一次收敛，内部 sentinel 文本不直接外泄。

**装了 guard 时**（生产 chat 现在走的路径），无论治理链因为什么拒绝：

```go
if out.Block {
    return rejectedResult(call.CallID, ErrorScopeDenied, out.Reason, false), nil
}
```

**九个控制的所有拒绝——能力不足、权限不足、dry_run 不支持、缺幂等键、预算耗尽、受保护度量不通过——一律变成 `scope_denied`**，且 `out.Reason` 是治理链拼的原始文案，绕过了 `publicPolicyError`。

## 为什么这条要修

1. **`scope_denied` 在本仓有专门含义。** 它是底线 1（跨法人隔离）的证据码，AGENTS.md 明确要求「权限拒绝必须保持 `scope_denied` 原因，不得软化」。现在反过来了：不是软化，是**把六种别的拒绝伪装成跨法人拒绝**。审计里读到一条 `scope_denied`，无法判断到底是租户越界还是忘了传幂等键。
2. **错误码词表是共享契约。** `ErrorScopeDenied` 是 `errcontract.CodeScopeDenied` 的再导出，HTTP 接缝用同一套码。`errors.go` 的注释写着「callers branch on the code, never on the human-readable text」——调用方按码分支，而码现在是错的。
3. **`publicPolicyError` 的收敛被绕过。** 那一层的存在本身说明「对外文案要收敛」是既有决策，guard 路径把它跳过了。

## 要做的

**目标：guard 路径的拒绝，错误码与未装 guard 时一致。**

`GuardResult` 目前只有 `Block bool` / `Reason string` / `Short *ToolResult`——没有承载码的位置，这是根因。建议给它加一个错误码字段，由治理链的各个控制在 deny 时明确带上，而不是在 runtime 侧靠字符串匹配 `out.Reason` 反推 sentinel（字符串匹配会在文案微调时静默失配，是比现状更糟的形状）。

对应关系照 `policyErrorCode` 的既有分型，逐个控制点名：

| 控制 | 拒绝场景 | 码 |
|---|---|---|
| TenantScope | 身份不完整 / facts 解析失败 | `unauthenticated` |
| CapabilityCheck | level 被禁用、capability 未授予 | `capability_denied` |
| CapabilityCheck | permission 未授予 | `permission_denied` |
| CapabilityCheck | descriptor 版本不匹配、dry_run 不支持 | `invalid_arguments` |
| ProtectedMeasure | 受保护度量落在非认证工具上 | 由实现方裁定后写进本表 |
| BudgetGuard | 预算耗尽 | 由实现方裁定后写进本表 |
| IdempotencyGuard | 写类工具缺 idempotency_key | `invalid_arguments` |

最后两行我没有既有对应物可照抄——`Evaluate` 里预算闸不存在，缺幂等键归在 `invalid_arguments`。**动手前把这两条的提案抛回来**，别自己定了就写。

同时把对外文案重新走一次收敛（等价于 `publicPolicyError` 的作用），不要把治理链内部 sentinel 文本原样外抛。

## 验收

- 六类拒绝各一条测试，断言**码**而非文案；把码映射改回一律 `scope_denied`，六条全红
- 装 guard 与不装 guard 两条路径对同一个拒绝场景返回**同一个码**（这是本单的核心断言，也是当初两条路径分叉没被发现的原因）
- `scope_denied` 只在真正的租户/范围越界时出现——反向测试：构造一次能力不足的调用，断言码**不是** `scope_denied`
- 全量 `go test` / `go vet` / `make test-integration` 绿

## 不要顺手改的

仓库里现有 8 处测试断言 `ErrorScopeDenied`（`agenttools/tools/*_reader_test.go`、`handlers/store_pnl_agent_integration_test.go`）。**这些是对的，不要动**——它们断言的是工具 handler 自己做的跨法人拒绝，本来就该是 `scope_denied`，与本单要修的 guard 路径映射是两回事。改红了它们说明改错了方向。
