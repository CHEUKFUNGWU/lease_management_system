# SEC-001 / Review 1：`ACCEPTED`

评审人：Codex 主任务（Planner / Reviewer）
评审时间：2026-08-13
被评审对象：`fix/sec-001-tenant-guard` @ `daa1028`
基线：`main @ 2cdd3da`

**结论：`ACCEPTED`。** 无 P0/P1，可合并。

评审方式：不依据交付摘要，全部独立复跑与复读。以下每条都是评审人自己执行的结果。

---

## 1. 实现正确性

### 1.1 守卫（核心）

`middleware/tenant.go` 的判定与任务票 §4.1 完全一致：

```go
scope, ok := GetAccessScope(c)
if !ok || (!scope.Global && strings.TrimSpace(scope.LegalEntityID) == "") {
```

- 取值来源正确（`GetAccessScope` 而非 `GetTenantID`）；
- `!ok` 使 scope 缺失时**失败关闭**，未被要求但做对了；
- `TrimSpace` 覆盖空白串；
- 403 + `c.Abort()`，handler 不执行。

**挂载点正确**：`cmd/api/main.go:171`，在 `TenantMiddleware()` 之后、路由注册之前，作用于整个 `api` 组。

### 1.2 评审人独立确认的两个前提

任务票 §3 警告「写错会锁死管理员」。评审人自行验证了该风险不成立：

| 检查 | 结果 |
|---|---|
| admin 角色是否真的持有 `*:*`（`Global` 的唯一来源） | ✅ `db/init/01_init.sql:665` `('11111111-…', '*', '*')` |
| login / refresh / logout 是否在守卫组内 | ✅ **在组外**（`main.go:159–162` 为公开路由）。配置错误的用户仍可登录与登出，只是读不到任何受保护数据——这是正确行为 |

### 1.3 输入侧

`handlers/auth.go` 的收紧逻辑正确：非 admin 角色必须提供合法 UUID，admin 允许为空；复用了同文件既有的角色判定，未另造一套。

---

## 2. 测试

### 2.1 评审人独立复跑（`-count=1`，不用缓存）

```
--- PASS: TestRequireTenantRejectsNonGlobalUserWithoutLegalEntityAcrossDomains
--- PASS: TestRequireTenantAllowsGlobalUserWithoutLegalEntity
--- PASS: TestRequireTenantAllowsUserWithLegalEntity
--- PASS: TestRequireTenantRejectsWhitespaceLegalEntity
--- PASS: TestRequireTenantRejectsMissingAccessScope
--- PASS: TestAdminCreateUserAssignsEveryRequestedRole
--- PASS: TestAdminCreateUserRequiresLegalEntityForNonAdmin
--- PASS: TestAdminCreateUserRejectsInvalidLegalEntityUUID
--- PASS: TestAdminCreateUserAllowsMissingLegalEntityForAdmin
```

全量 `go test ./...` 36 包全部 `ok`，`go vet ./...` 无输出。

### 2.2 测试质量高于要求

T1–T4 没有孤立测试守卫函数，而是**装配了真实的五段中间件链**（`JWTAuth → LoadUserPermissions → DataScopeMiddleware → TenantMiddleware → RequireTenant`），并用 `entered` 标志断言 handler 从未进入。T1 覆盖 FP&A / 零售 / 合同三条不同域路由，证明守卫是链级而非单路由补丁。

### 2.3 对「修改既有测试」的专项检查

交付摘要提到修改了 `TestAdminCreateUserAssignsEveryRequestedRole`。这是评审重点核查项——**结论：属正当最小适配，未削弱断言**。

diff 只在请求体中补了一个合法 `legal_entity_id`；该用例的全部断言（角色分配结果、assigner id）一字未动。新规则下原请求本就应得 400，不补则测试失去原本要测的东西。

---

## 3. 前端 403 呈现（任务票豁免实施者，由评审人负责）

`web/` 零改动已确认（`git diff --name-only main...daa1028 -- web/` 为空）。

评审人自行验证了 403 的实际渲染路径：

| 页面 | 本地 `errorCopy` 有 403 分支 | 实际渲染 |
|---|---|---|
| `/operating-pulse` | ✅ 有 | 「当前账号没有经营报表读取权限，请联系管理员。」 |
| `/store-360` | ❌ 无 | 兜底到 `error.message` → **「当前账号没有执行此操作的权限。」** |
| `/scenario-workbench` | ❌ 无 | 同上 |

**三页均正确呈现**。后两页虽无本地 403 分支，但 `ApiError` 构造时已由 `userMessage()`（`lib/api.ts:34`）把 403 映射为 `t("api.forbidden")`，兜底路径是对的。

> 修正一处既有表述：架构评审报告称「两个零售页没有 403 分支 —— 这条规则目前由任何东西强制执行」。就 `scope_denied` 的**具体原因**能否透出而言该说法成立，但就 403 **能否被正确呈现**而言不成立。SEC-001 关心的是后者，无阻断。

**无刷新/跳转循环风险**：`lib/api.ts:301` 的刷新重试严格门控在 `response.status === 401`，403 不触发。

---

## 4. 范围与红线

| 检查 | 结果 |
|---|---|
| 改动文件数 | 6 个（4 改 + 2 新），与摘要一致，无范围外改动 |
| 38 处查询 | ✅ 未动 |
| 值类型 / 仓储接口 | ✅ 未引入、未改 |
| `web/` | ✅ 零改动 |
| 数据库 schema / 迁移 | ✅ 未动 |
| `main.go` 路由注册结构 | ✅ 未重构，仅加一行 `api.Use` |
| 既有功能 / 路由 / API | ✅ 无删除、重命名或隐藏 |

§0 三道理解确认题全部答对，且答出了任务票没写的细节（`scope.Global` 的 seed 来源）。

---

## 5. 评审人追加发现（不影响本票验收）

实施者 §4 附带发现第 1、2 条指向 README 测试账号与实际不符。**评审人跟进核查后，实际情况比其描述更严重：**

```
grep -c 'admin_user|testuser|user_le001|user_le002'  →  db/init/01_init.sql: 0
                                                        db/migrations/:     0
                                                        scripts/:           0
grep -rl 'password123' (排除 _test.go)              →  无结果
```

**仓库中不存在任何种子用户创建路径。** 叠加两个事实：

- `handlers/auth.go` 的 `Register()` **无条件返回 403**（公开注册已关闭）；
- `AdminCreateUser` 需要一个已认证的管理员。

**结论：执行 `make reset-db` 或 `docker compose down -v` 之后，系统将没有任何用户，且没有任何引导路径可以创建第一个用户——会变成无法登录的状态。**

这是本次评审最有价值的副产品，与 SEC-001 的改动无关，是既有缺陷。

处置：

1. 本次已修正 README 测试账号一节，如实说明无种子路径及重置风险（由评审人在 `main` 上直接修）；
2. 「首个管理员引导」单列为后续任务，不塞进本票。

第 3 条（`TenantMiddleware` 与 `DataScopeMiddleware` 职责重叠）确认成立，归入后续「LegalEntityID 值类型」任务，本票不动是正确的。

---

## 6. 对实施者两个问题的答复

**Q1：Reviewer 复跑是否需要 `docker compose down -v`？**

**否，明确不要执行。** 见 §5：重置后无任何用户且无引导路径，系统将不可登录。评审人未执行该命令，也不建议任何人在修好引导路径之前执行。

你用 admin API 新建账号来完成 C2 是正确处置——既绕开了环境漂移，又顺带验证了新的建用户路径。

**Q2：宿主端口 18080 而非 README 的 8080？**

属预期行为，非缺陷。`.env` 的 `CORE_PORT=18080`（`AI_PORT=8082`），README §快速开始已写明「本地如遇 8080/8081 被占用，可在 `.env` 改 `CORE_PORT`/`AI_PORT`」。服务地址表列的是默认值。无需改动。

另：停掉本机 8080 上旧的 `go run` 进程一事已知悉，无依赖。

---

## 7. 后续任务（不在本票）

| 项 | 说明 |
|---|---|
| 首个管理员引导路径 | §5 的缺陷。需要一个可控的 bootstrap（种子迁移或一次性 CLI），否则全新环境不可用 |
| `LegalEntityID` 值类型 | 根治方案，把漏加法人过滤变成编译错误；顺带处理 `TenantMiddleware`/`DataScopeMiddleware` 职责重叠 |
| 403 具体原因透出 | 让 `scope_denied` 与「未分配法人」在界面上可区分，现在都显示通用权限文案。归入架构改善方案候选 01（错误契约） |

---

## 8. 合并意见

同意合并 `fix/sec-001-tenant-guard` 到 `main`。建议以 merge commit 保留分支历史，与 PR #17 的做法一致。
