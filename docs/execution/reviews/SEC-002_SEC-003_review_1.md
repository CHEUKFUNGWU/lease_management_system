# SEC-002 + SEC-003 / Review 1：`ACCEPTED`

评审人：Codex 主任务（Planner / Reviewer）
评审时间：2026-08-14
被评审对象：`fix/sec-002-003-tenant-hardening` @ `77d6af3`
基线：`main @ 19ae6a7`

**结论：`ACCEPTED`。无 P0/P1，可合并。** 两条 P2 与一条 P3 单列为后续票，不阻塞本票。

评审方式：不依据交付摘要。独立 worktree + 一次性 `postgres:15-alpine`（端口 55433，按 CI 方式加载 `db/init/01_init.sql`），跑完即销毁；同事的工作区与共享 `lease-postgres` 全程未触碰。

---

## 1. 最关键的一关：表征测试的断言有没有被改弱

任务票 §3.3 写明「若第二个 commit 改动了第一个 commit 里的断言，本票直接退回」。这是本次评审的首要检查项。

提交顺序正确：

```
528f2c8 feat(sec-002): bootstrap-admin
861e522 test(sec-003): characterization tests   ← 表征测试先行
ae2ee0f refactor(sec-003): value type           ← 替换在后
77d6af3 docs: delivery report
```

`ae2ee0f` 确实修改了 `861e522` 建立的全部 5 个表征测试文件。评审人把这些 diff 中的断言行全部抽出逐条比对：

| 形态 | 变更前 | 变更后 |
|---|---|---|
| 参数 | `pair.entityA` | `mustEntityFilter(t, pair.entityA)` |
| 期望 | `got == nil` / `got != nil` | **完全相同** |
| 期望 | `allowed` / `!allowed` | **完全相同** |
| 期望 | `frozen == nil` / `frozen != nil` | **完全相同** |
| 期望 | `err == nil` / `err != nil` | **完全相同** |

**每一处差异都只是签名适配，无一处期望值被修改。** 「仅参数机械适配」的说法核实属实，不构成退回条件。

---

## 2. 值类型的封闭性

`access.EntityFilter` 设计正确：

- 字段 `global` / `id` 均未导出 → 包外无法绕过构造器造值；
- **零值失败关闭**：`LegalEntityID()` 与 `SQLClause()` 对零值均返回 error，不会退化为「不过滤」；
- `SQLClause` 产出 `column::text = $N`，**无 `OR` 分支**；Global 产出空子句；
- 命名与注释显式对齐 CONTEXT.md 的 Legal Entity Access，未使用 `_Avoid_` 词。

**评审人追加检查（任务票未要求）**：零值失败关闭只有在调用方处理 error 时才有意义——若某处忽略 error，空子句就等于不过滤。逐一核查全部 `SQLClause(` 调用点，**全部为 `if clause, arg, err := …; err != nil` 形式，无一处 `_` 丢弃 error**。封闭性成立。

一处优于指令的实现：`bootstrap-admin` 按 `code = 'admin'` 解析角色，而非我在指令里建议的硬编码 role id。更稳健。

---

## 3. 独立复跑结果

| # | 检查 | 结果 |
|---|---|---|
| V1 | 全量 `go test ./...`（带 `TEST_DATABASE_URL`）+ `go vet` | ✅ FAIL 总数 0，vet 无输出 |
| V2 | `./internal/repository/ -count=2` 真实库 | ✅ 通过 |
| V3 | 法人逃生舱清零 | ✅ 见 §4 |
| V4 | 防回归检查确实会失败 | ✅ 见下 |
| V5 | 零值失败关闭 | ✅ `TestEntityFilterZeroValueFailsClosed` 等 6 个用例全 PASS |
| V6 | Global 管理员跨法人 | ✅ 值类型层面已由 `TestFromScope` 覆盖，运行态见实施者证据 |
| V7 | 两 commit 顺序 | ✅ 见 §1 |
| — | `web/` 零改动 | ✅ 0 个文件 |

**V4 评审人亲自复现**（不采信报告输出）：向 `internal/repository/budget.go` 注入 `($9='' OR legal_entity_id::text=$9)`，`TestNoLegalEntityEscapeHatchInRepositorySQL` 立即 FAIL 并给出 file:line；移除后恢复 ok。守卫真实有效。

**SEC-002 评审人实跑**：

| 场景 | 结果 |
|---|---|
| 缺 env | 拒绝，退出码 **2** |
| 空库首次引导 | 成功创建 |
| 重复执行 | 拒绝，退出码 **3** |
| 库内核对 | 恰好 1 个用户；`admin` 角色；`legal_entity_id` 为 NULL；`assigned_by` 为 NULL |
| 仓库内真实密码 | 无（`.env.example` 为空占位，README 用 `<你的强密码>`） |

---

## 4. 逃生舱清零的准确结论

`grep "= '' OR" internal/repository/` 仍有 8 处命中，分布在 `fpna_governance.go`(4)、`ai_chat_runtime.go`(2)、`approval.go`(1)、`exchange_rate.go`(1)。评审人逐条核对其过滤字段：

```
mapping_type · memo_type · metric_key · period · report_type · version_type
external_system · status · basis · grain · approval_status
from_currency · to_currency
```

**除下述 §5.1 一条外，全部是合法的可选业务过滤，与法人边界无关。**

> **本条是评审人自己的指令缺陷，须记录**：任务票 §3.6 的 V3 验收命令写成 `grep -rn "= '' OR" core-service/internal/repository/` 期望「无结果」，这个范围过宽，会误伤上述合法可选过滤。实施者按实质口径执行是对的。**下次写这类验收命令必须锚定字段名，不能只锚定形状。**

---

## 5. 后续票（不阻塞本票合并）

### 5.1 P2：第 39 处逃生舱

`core-service/internal/repository/ai_chat_runtime.go:236`

```sql
AND ($2 = '' OR COALESCE(legal_entity_id::text, '') = $2)
```

**这是一处真实的法人逃生舱**，被我原始 grep 漏掉——`COALESCE(` 卡在 `OR` 与字段名之间，所以任务票里写的「38 处」是低估，真实为 **39**。

实施者在 B4 中如实标注为「范围外」并未自行修改，**这完全符合任务票 §3.5 与 §6 的要求**，不是失误。

危害略高于普通逃生舱：`COALESCE(legal_entity_id::text, '')` 使 `legal_entity_id IS NULL` 的行在 `$2=''` 时同样命中，两个方向都漏。

### 5.2 P2：防回归守卫与我犯了同一个盲点

`legal_entity_escape_hatch_regression_test.go` 的模式为：

```go
regexp.MustCompile(`''\s*OR\s+legal_entity_id`)
```

评审人实测四种形态：

| 形态 | 守卫 |
|---|---|
| `($2='' OR legal_entity_id::text=$2)` | ✅ 捕获 |
| `($2='' OR v.legal_entity_id::text=$2)` | ❌ **漏掉** |
| `($2='' OR l.legal_entity_id::text=$2)` | ❌ **漏掉** |
| `($2 = '' OR COALESCE(legal_entity_id::text, '') = $2)` | ❌ **漏掉** |

别名形态并非假想——被替换掉的原代码里就存在 `v.` / `l.` / `s.` 前缀写法。守卫应放宽到形如 `''\s*OR\s+[^)]*legal_entity_id`，否则它只防得住最规整的一种写法。

### 5.3 P3：注释中的引号损坏

`internal/access/entity_filter.go` 顶部注释有两处字符损坏：`A string used ", for no filter"` 与 `($N=” OR`（弯引号）。不影响行为，顺手修即可。

### 5.4 实施者报告的 5 处预存缺陷

`UpdateDataQualityStatus` 42P08 恒失败、`CreatePlanLine` 的 `RETURNING created_at` 列不存在、`CreateBatch` 无幂等键恒失败、幂等查询信任 payload —— 均为本票范围外的既有缺陷，实施者未动是对的。建议单独开票，其中前三条看起来是「恒失败」，说明这些路径没有测试覆盖，值得一并评估。

---

## 6. 合并意见

同意合并 `fix/sec-002-003-tenant-hardening` 到 `main`，建议保留 merge commit。

合并后建议立即处理 §5.1 与 §5.2 —— 两者合起来才是「逃生舱不可重现」这个目标的完整闭环：现在的状态是主要形状已消除且有守卫，但守卫本身有已知盲点，且已知存在一处未清除的变体。
