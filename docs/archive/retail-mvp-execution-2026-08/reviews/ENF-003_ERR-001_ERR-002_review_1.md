# ENF-003 + ERR-001 + ERR-002 / Review 1：`ACCEPTED`

> ⚠️ **ARCHIVED 2026-08-18 — 不是现行依据**
> 归档理由：零售 MVP（MAX-001~009）转型执行的过程记录，全部工单已 ACCEPTED；未完成的延后治理项见 docs/execution/精简MVP路线与延后治理清单.md
> 现行入口：`docs/AI_文档索引与现行决策.md`

评审人：Codex 主任务（Planner / Reviewer）
评审时间：2026-08-14
被评审对象：`feat/err-001-002-error-contract` @ `6564aa3`
基线：`main @ 831579f`

**结论：`ACCEPTED`。无 P0/P1，可合并。** 一条 P3。

---

## 1. 常设规则生效了

上一批评审把「守卫语义变更必须单独成 commit」升级为常设要求。本批三个功能 commit 中，**只有 ENF-003 动了 `enforce-design.mjs`**，另外两个未触碰。规则被遵守，流程问题闭环。

---

## 2. ENF-003：五个场景全部自行注入验证

判定实现正是建议的计数法：

```js
isNewViolation = violationCount(pattern, line) > violationCount(pattern, oldText)
```

评审人不采信报告输出，逐个注入：

| # | 注入 | 结果 |
|---|---|---|
| N1 | 往已有 `!important` 的行再加一个 | ✅ **拦截**（上批放行） |
| N2 | 既有内联样式 2 属性 → 5 属性 | ✅ **拦截**（上批放行） |
| N3 | **只改该行的值、不增加违规数** | ✅ **放行，无误报** |
| N4 | 全新一行加违规 | ✅ 拦截 |
| N5 | 干净树 | ✅ 通过 |

**N3 是本票的承重点**——若改成计数后触碰存量行开始误报，就把上一批解决的问题又带回来了。实测未误报，语义漂移彻底修复。

---

## 3. ERR-001：最容易静默破坏的地方是安全的

`agenttools` 改为重新导出，评审人重点核查了**是否保持 JSON 兼容**：

```go
type ErrorCode = errcontract.Code    // 类型别名，非新类型
type ToolError = errcontract.Error   // 类型别名
```

`errcontract.Error` 的 tag 与 `main` 上旧 `ToolError` **逐字节一致**：

```go
Code      Code           `json:"code"`
Message   string         `json:"message"`
Retryable bool           `json:"retryable"`
Details   map[string]any `json:"details,omitempty"`
```

用 `=` 别名而非新类型，意味着既有调用方**无需改动即可编译，且序列化结果完全相同**。Agent 接缝零行为变化。做法正确。

`errcontract` 确认是**叶子包**（0 处内部依赖），不存在 HTTP 反向依赖 `agenttools` 的循环。

### 3.1 复核结果

| # | 检查 | 结果 |
|---|---|---|
| R1 | 全量 `go test ./...`（带 `TEST_DATABASE_URL`）+ vet | ✅ FAIL 数 0，vet 干净 |
| R2 | 六个零售 handler 的 `gin.H{"error"` | ✅ **0** |
| R3 | 全仓剩余 | ✅ **735**（= 824 − 89，精确） |
| R4 | `scope_denied` 端到端 | ✅ `TestRetailStoreDayFactsUpsertScopeDeniedEndToEnd`、`TestErrorContractPassesScopeDeniedThroughUpsert` PASS |
| R6 | 脱敏 | ✅ `TestErrorContractRedactsInternalFailure` PASS |
| — | Web 全量 | ✅ 16 套件 / 102 用例，type-check 通过 |
| C1 | 页面本地 `errorCopy` | ✅ **0**（三处已删并合一） |

### 3.2 顺带修的 nil 解引用

`retail_scenarios.go` 的 `writeError` nil 解引用（架构评审曾标记为「今天不可达」）在转换过程中被修掉。**属于必要后果**——转换该文件时必然经过这段，留着反而是明知有缺陷不修。已在报告注明，不算范围蔓延。

---

## 4. ERR-002：`scope_denied` 终于有了自己的说法

```
zh-CN: 该对象不在你的数据范围内，无法访问。请确认所属法人或门店范围。
zh-HK: 該對象不在你的數據範圍內，無法存取。請確認所屬法人或門店範圍。
en:    This object is outside your data scope and cannot be accessed. …
```

三语齐全，与通用权限文案明确区分，**且不含「暂无数据」字样**——AGENTS.md「`scope_denied` 不得被软化」这条规则，第一次有了实际的界面保证。

---

## 5. P3：一处可避免的文案退化

实施者主动报告：`store-360` 的 404 文案从

> 「该门店不在当前法人或数据范围内。」

退回通用的

> 「请求的数据不存在或已被移除。」

理由是「读路径无法区分门店不存在与在其他法人，区分需存在性二次查询，会泄漏跨法人存在性」。

**主动报告是对的，但这条理由不成立。** 旧文案「不在当前法人**或**数据范围内」本身就是**歧义构造**——它既不确认也不否认该门店存在，泄漏为零。所以为避免泄漏而放弃它，并没有换来任何安全收益，只损失了信息量。

不阻塞合并（三处页面本地映射合一是任务票明确要求的，方向正确）。**系统性修法**：后端现在会发 code 了，让 scoped 资源的 404 携带 `details: { resource: "store" }`，共享映射器据此选上下文文案。这比恢复页面本地映射更好——既保住合一，又拿回具体性。归入后续票。

---

## 6. 另记两项（实施者已如实报告，无需动作）

- **422 + `system_failure` 的组合**出现在模拟数据生成的非幂等失败。code 与状态码是两个维度，本票明令不动状态码，处理正确。状态码语义收敛若要做，单独开票。
- R4 排障记录（`created_by` 外键、`defer pool.Close()` 早于 `t.Cleanup` 导致清理静默失败，改用 `t.Cleanup(pool.Close)` 走 LIFO）已写进测试注释。**这类踩坑记录留在代码里比留在报告里有用**，做法可取。

---

## 7. 合并意见

同意合并，保留 merge commit。§5 的 404 上下文文案归入后续票。

## 8. 后续待办

| 项 | 归属 |
|---|---|
| scoped 资源 404 携带 `details.resource`，映射器选上下文文案 | 下一批或阶段二 |
| 其余 735 处错误站点转换 | 后续分批 |
| 状态码语义收敛（如 422 + `system_failure`） | 独立票 |
| 存量 `!important` 139 处、内联样式 876 处 | 后续分批 |
