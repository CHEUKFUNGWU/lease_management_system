# SEC-004 + ENF-001 + UI-001 + UI-002 / Review 1：`ACCEPTED`

评审人：Codex 主任务（Planner / Reviewer）
评审时间：2026-08-14
被评审对象：`feat/sec-004-enf-001-ui-001-002` @ `78d7c1b`
基线：`main @ fbb5a6b`

**结论：`ACCEPTED`。无 P0/P1，可合并。** 一条 P2、两条 P3 单列为后续项。

评审方式：独立 worktree + 一次性 `postgres:15-alpine`（端口 55434，按 CI 方式加载 schema），跑完销毁；未触碰共享环境。

---

## 1. 本批最重要的事：关掉了一个比本票目标更严重的漏洞

SEC-004 在把 `ListSessions` 转成 `access.EntityFilter` 时，移除了 handler 里的这段：

```go
legalEntityID := c.Query("legal_entity_id")   // 用户可控
if legalEntityID == "" {
    legalEntityID = middleware.GetTenantID(c)
}
```

**这是一个用户可控的跨租户读取漏洞**：任何已登录用户对 `/api/v1/ai/chat/sessions` 传 `?legal_entity_id=<他人法人>`，即可读到该法人的 AI 会话列表。它比本票原定要修的逃生舱更严重——逃生舱需要用户被配错，这个任何人直接可用。

**评审判断：这不是范围蔓延，是必要后果。** 仓储层改用值类型后，handler 被迫从 scope 构造过滤器，query 覆盖随之变得不可表达。修复从设计里自然掉出来。

评审人独立确认兼容性：`git grep legal_entity_id main -- web/app/**` 的全部命中都在管理员建用户与展示字段，**前端从未向该端点传过此参数**，移除无兼容风险。

### 1.1 顺带排查的同款模式（结论：不是同一个洞）

`handlers/master_data.go:33` 也把 `c.Query("legal_entity_id")` 传进仓储，且 UI-001 的门店搜索正走这条路。评审人查了仓储实现：

```go
filterEntityID := legalEntityID
if scope, scoped := access.ScopeFromContext(ctx); scoped && !scope.Global {
    filterEntityID = scope.LegalEntityID   // ← 非 Global 用户的输入被丢弃
```

非 Global 用户传入的值会被真实 scope **覆盖**，Global（管理员）保留查询任意法人的能力是合理的。**不是漏洞，UI-001 使用它是安全的。** 但这种"看起来像洞、靠下游兜住"的写法值得记入后续清理。

---

## 2. 独立复跑结果

| # | 检查 | 结果 |
|---|---|---|
| S1 | 全量 `go test ./...`（带 `TEST_DATABASE_URL`）+ `vet` | ✅ FAIL 数 0，vet 干净 |
| S2 | 守卫四形态断言 | ✅ `TestEscapeHatchPatternCapturesEveryKnownForm` PASS |
| S3 | **评审人亲自注入四形态** | ✅ 无别名 / `v.` / `l.` / `COALESCE` **四种全部拦截**，移除后恢复 |
| E1 | 基线上跑 `enforce-design` | ✅ 17 个变更文件，无新增违规 |
| E2 | **评审人亲自造四条违规** | ✅ 内联样式 / fontWeight>600 / `!important` / 硬编码中文 **四条全部拦截**并报出 file:line |
| U1–U4 | Web 测试 | ✅ 13 套件 / 83 用例全过；type-check 通过 |
| B1 | `lease_` 命名空间前后比对 | ✅ **661 = 661，完全未动** |
| B3 | 源码中「租赁管理系统」 | ✅ 0 |
| B4 | 源码中 `fontWeight: 800` | ✅ 0 |

`ListStores` 门店搜索的选择与理由已在报告中说明（`storeOptions` 的 `data_classification` 语义对导航面板不自然），判断合理，且**未新增后端接口**，符合票的约束。

---

## 3. 前端核验（票已豁免实施者，由评审人负责）

### 3.1 角色可见性：与导航栏逐条一致

UI-001 的 `palette.ts` `canViewGroup` 与 `AppLayout.tsx` `useMenuItems` 对照：

| 分组 | palette | useMenuItems | |
|---|---|---|---|
| daily | 无条件 | 无条件 | ✅ |
| analysis | `admin \|\| readonly \|\| auditor` | 同 | ✅ |
| accounting | `admin \|\| auditor \|\| editor \|\| reviewer \|\| approver` | 同 | ✅ |
| system | `admin \|\| auditor` | 同 | ✅ |
| `/settings`、`/admin/users` | 专属 `admin` 覆盖 | 同 | ✅ |

**不存在 readonly 用户搜出管理员页面的情况。** 22 条路由已登记，U3 未登记检测机制存在。

### 3.2 未能完成的部分

**真实浏览器交互未验证。** 登录需要输入密码，这项我不能代做。以下留待你本地确认（都是低风险的观感项，不影响本票验收）：

- ⌘K 面板里新增的 21 条路由与门店搜索的实际观感与键盘导航；
- Logo 徽标「营」在 28×28 深色方块内的视觉效果（字号 11px 是否偏小）；
- 新开场白在 AI Chat 首屏的换行表现。

---

## 4. P2：徽标豁免屏蔽了三条检查，不止一条

`enforce-design.mjs` 的徽标豁免用 `continue` 跳出整个行循环：

```js
if (BRAND_BADGE_LINE.test(line)) {
  continue;   // 同时跳过 !important / 内联样式 / fontWeight 三条
}
```

评审人实测：把徽标行的 `fontWeight: 600` 改回 `800`，**守卫放行**（退出码 0，报「无新增违规」）。

而这一行恰恰是本票要求从 800 降到 600 的那一行——现在它对三条检查永久失去防护。豁免的注释自称"只允许改字重与字形"，但实现比声明宽。

**建议**：把 `continue` 改成只豁免内联样式一条（例如在内联样式判定处单独跳过），`fontWeight` 与 `!important` 仍然生效。改动很小。

---

## 5. P3

### 5.1 行级语义是在 UI-002 提交里改的

`addedLines()`（从"整个变更文件"收窄到"仅新增行"）由 `dccabf2`（UI-002）引入，而非 `23cc507`（ENF-001）。也就是说 ENF-001 交付时扫的是整文件，UI-002 为让自身通过而放宽了它。

**结论上评审人接受这个语义**——DESIGN.md §13 写的是"新代码不得再增加"，行级新增才是忠于原文的读法，整文件反而比文档更严。但**放宽守卫的动作应该发生在守卫自己的提交里并说明理由**，而不是在被它拦住的功能提交里顺手做掉。下次遇到同类情况请先停下来问（这正是指令 §8 的适用场景）。

### 5.2 `layout.tsx` 是整文件豁免 CJK

`METADATA_EXEMPT_FILES` 豁免了整个 `web/app/layout.tsx`，而非其中的 metadata 行。该文件目前很小，影响有限，但语义上比"元数据豁免"宽。

---

## 6. 合并意见

同意合并 `feat/sec-004-enf-001-ui-001-002` 到 `main`，保留 merge commit。

合并后建议立即处理 §4（改动只有几行），其余归入 UIUX 改善方案阶段一。

---

## 7. 后续待办汇总

| 项 | 来源 | 归属 |
|---|---|---|
| 徽标豁免收窄为仅内联样式 | 本次 §4 | 立即 |
| `master_data.go` 的 `c.Query("legal_entity_id")` 写法清理 | 本次 §1.1 | 阶段一 |
| `layout.tsx` 走 `generateMetadata` 做 i18n | 实施者附带发现 | 阶段四 |
| 三个零售页的硬编码中文与豁免名单 | DESIGN.md §14 | 阶段四 |
| `tokens.ts` ↔ `:root` 令牌对齐 + 对齐测试 | DESIGN.md §1 | 阶段一（含视觉变化，需评审人验证） |
| SEC-003 报告 B4 的 5 处预存缺陷 | 上一批 | 待开票 |
