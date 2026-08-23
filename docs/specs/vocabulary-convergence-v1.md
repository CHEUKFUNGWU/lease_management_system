# Spec：词表收敛（V1 批次 · 报表口径与审批状态）

> 编制：2026-08-24 · 状态：`ready-for-agent`
> 来源：CONTEXT.md「Approval and Reporting Basis」词条定案后遗留的代码分叉，登记在 AGENTS.md「已知词表分叉」
> 发布位置说明：本仓无 issue tracker，按既有惯例落 `docs/specs/`

---

## 先决：盘点订正，`?official=true` 不在迁移范围

2026-08-23 的初次盘点把两个不同的 `official` 混为一谈，登记在 AGENTS.md 的清单因此把「公开 API 查询参数」列为最难改的一项。**那是错的**，本 spec 予以订正。

`handlers/fpna_governance.go:167` 的 `?official=true` 用在 `FreezePlanVersion` 上，语义是「冻结为 Official 计划版本」（需 approver 角色）。按 CONTEXT.md 的词条，**Official Version 是保留概念**——「在同一事物的多个 Approved 版本中当前承载权威的那一个」。这个参数命名正确，不改。

订正后的分类：

| 位置 | 语义 | 处置 |
|---|---|---|
| `?official=true`（FreezePlanVersion） | 版本地位 | **保留** |
| `is_official_version` / `is_official` 列 | 版本地位 | **保留** |
| `fpna_plan_versions.status` 的 `'official'` | **混淆**：审批状态列里塞了版本地位 | **拆** |
| `fpna_plan_versions.status` 的 `'review'` | 审批状态，旧名 | 改为 `pending` |
| `finmodel/view` 的 `basis_mode` | 报表口径 | 改名 + JSONB 迁移 |
| `services/reporting` 的 `Mode` | 报表口径 | 改名 |
| `services/retailexport` 的 `mark` | 报表口径 | 改名 |
| `draftapp` 的 `ReportMode` | 报表口径 | 改名 |
| `aiagent` 的 `PageContext.ReportView` | 报表口径 | 改名 + 前端同步 |

## Problem Statement

**混淆的确切形态在这段代码里：**

```go
// repository/fpna_governance.go:397
status := "approved"
if official {
    status = "official"
}
UPDATE fpna_plan_versions SET status=$2, is_official=$3, ...
```

`status` 与 `is_official` 由**同一个布尔值**派生。于是 `status` 一列同时承载了审批状态（approved）与版本地位（official），而 `is_official` 又把地位重复编码一遍。CONTEXT.md 把这两者定义为**独立的轴**：一份记录可以 Approved 但已被取代。这里它们被压成了一个。

**伤害尚未发生。** 全仓找不到 `status IN ('approved','official')` 这类被迫的写法。所以这张单是**预防性的，不是修复性的**——具体风险是下一个写「找出全部已批准的计划版本」的人会写 `status='approved'`，然后**安静地漏掉那些 official 的**，因为它们的 status 不是 approved。

**报表口径那边是另一件事**：`working` / `official` 作为口径名已被 CONTEXT.md 列为 `_Avoid_`，正名是 `draft` / `pending` / `approved`（累积语义，「最低包含到哪一级」）。B-2 与 B-4 的新代码已按新词表写，既有五处仍是旧名。留着的代价是每写一个新报表工具就多一处要对齐的地方。

`working` 这个词还有个额外问题：它与 CONTEXT.md 既有的 **Working Paper**（审计底稿）撞词根却毫不相关。

## Solution

分两步，第二步依赖第一步。

**第一步：报表口径改名。** 五处内部枚举与一处前端传入字段，从 `working`/`official` 改为 `draft`/`pending`/`approved`。`basis_mode` 存在 saved view 的 JSONB 配置里，需要一次数据迁移。

**第二步：拆 `fpna_plan_versions.status`。** 把版本地位从审批状态列里取出来，`status` 收敛为 `draft`/`pending`/`approved`，地位由既有的 `is_official` 布尔承担，`retired` 单独处理。

## User Stories

1. 作为开发者，我想让全系统的审批状态只有一套词，以便读代码时不必判断这个 `status` 是哪套词表
2. 作为开发者，我想让「找出全部已批准的计划版本」写成 `status='approved'` 就是对的，以便不会安静地漏掉 official 的那些
3. 作为开发者，我想让报表口径的取值与审批状态同名，以便「最低包含到哪一级」这条语义在代码里显而易见
4. 作为使用者，我想让既有的 saved view 在迁移后仍然可用，以便我保存的视图不会失效
5. 作为使用者，我想让前端与后端对口径的理解一致，以便不会出现前端传 `working` 后端认 `draft` 的错配
6. 作为开发者，我想让 `review` 这个旧名从代码里消失，以便不会有人以为它和 `pending` 是两件事
7. 作为审计人员，我想让版本地位与审批状态在数据层可独立查询，以便「已批准但已被取代」这种状态可以被表达
8. 作为开发者，我想让 `?official=true` 保持不变，以便不破坏前端已在调用的接口

## Implementation Decisions

### D-V1：`?official=true` 与 `is_official*` 列不动

它们表达版本地位，是 CONTEXT.md 的保留概念。改它们既没有收益，又要破坏 `web/app/lib/api.ts:1409` 已在调用的接口。

**这条写在最前面，是因为初次盘点搞错过一次。** 判定方法：问这个 `official` 说的是「这份报表包含什么」还是「这一版是不是当前权威版」。前者是口径，要改；后者是地位，保留。

### D-V2：报表口径改名，取值用累积语义

`working` → `draft`，`official` → `approved`，`pending` 保持（B-4 已加）。

语义是**「最低包含到哪一级」**：`approved` 仅含 Approved；`pending` 含 Pending + Approved；`draft` 含全部三级。这不是三选一的互斥标签，是一个下界。

涉及五处：`finmodel/view` 的 `basis_mode`、`services/reporting` 的 `Mode`、`services/retailexport` 的 `mark`、`draftapp` 的 `ReportMode`、`aiagent` 的 `PageContext.ReportView`。

### D-V3：`basis_mode` 的 JSONB 迁移读写双向兼容，不做一次性切换

`basis_mode` 存在 saved view 的 JSONB 配置里（`repository/saved_views_*` 的 config）。一次性 UPDATE 全表看起来干净，但它有一个窗口期：迁移跑完之前，新代码读到旧值。

改为：**读取时双向接受**（`working` 与 `draft` 都认作 draft 口径），**写入时只写新值**，然后跑一次数据迁移把存量改掉。迁移完成后保留读取兼容至少一个发布周期再删。

这条比一次性切换多写十几行，换来的是迁移期间不会有用户的 saved view 打不开。

### D-V4：`PageContext.ReportView` 前后端同步改，后端先兼容

前端传入字段，改它需要前后端同时发布。同 D-V3：后端先接受两种值，前端改完之后再删旧值的接受。

**顺序是后端先行。** 反过来（前端先改）会在两次发布之间产生一个后端不认识新值的窗口。

### D-V5：`fpna_plan_versions.status` 拆成两轴

现状 `CHECK (status IN ('draft','review','approved','official','retired'))`，五值混着两个概念。

拆后：

| 轴 | 载体 | 取值 |
|---|---|---|
| 审批状态 | `status` | `draft` / `pending` / `approved` |
| 版本地位 | `is_official`（已存在） | 布尔 |
| 生命周期 | `retired_at`（新增，可空时间戳） | NULL 表示未退役 |

**为什么 `retired` 用时间戳而不是留在 status**：退役是一个发生在某时刻的事件，不是一个与 draft/pending/approved 并列的审批阶段。一个已退役的版本仍然是「曾经 approved 的」，把它写进 status 就丢掉了这个事实。

**迁移映射**：`review` → `pending`；`official` → `approved` 且 `is_official=true`（该值本就同步写入，见 Problem Statement 的代码）；`retired` → `approved` 且 `retired_at=NOW()`（退役前必然已批准）。

`FreezePlanVersion` 相应改为只设 `is_official`，不再改写 `status`。

### D-V6：改完之后 AGENTS.md 的分叉清单要删掉

那份清单记的是「已知未收敛」，收敛完不删就变成误导。同时把 D-V1 的订正写回去——初次盘点把 `?official=` 错列为迁移项，这个错误值得留痕，因为它示范了「同一个词在两个语义里」的判定方法。

## Testing Decisions

| 对象 | 测试方式 |
|---|---|
| 口径改名 | 全仓搜不到作为口径值的 `"working"`；`"official"` 仅剩版本地位用法 |
| JSONB 双向兼容 | 构造一条存量 `basis_mode: "working"` 的 saved view，断言读取后等价于 `draft` |
| `PageContext` 兼容 | 后端同时接受 `working` 与 `draft`，断言两者走同一分支 |
| status 拆分 | 迁移后 `status='approved' AND is_official=true` 能查到原 official 的行 |
| 退役语义 | `retired_at` 非空的行，其 `status` 仍是 `approved` |
| 迁移一致性 | migration 061 与 `01_init.sql` 空库版本内容一致 |

### 反向测试

- 把 JSONB 读取的兼容分支删掉，存量 saved view 的测试必须红
- 把 `status` 的 CHECK 约束改回五值，拆分后的断言必须红
- 构造一条 `status='approved' AND is_official=true` 的行，断言「找出全部已批准版本」的查询能命中它（这条正是本 spec 要防的那个安静漏掉）

## Out of Scope

- **`?official=true` 与 `is_official*` 列**：D-V1，保留
- **`approval_status` 列本身**：各业务表的 `draft`/`pending`/`approved` 已经是正名，不动
- **报表口径的语义变更**：只改名与拆列，不改「哪个口径包含什么」的判定逻辑
- **前端 UI 文案**：i18n 层的展示文案另说，本批次只动代码取值

## Further Notes

**这张单是预防性的。** 全仓找不到被重载 `status` 逼出来的写法，说明伤害尚未发生。做它的理由是成本随时间上升而不是当前有故障：每多一个报表工具就多一处要对齐，而 `status` 的重载会在某个人写下 `status='approved'` 的那天安静地咬人。

**初次盘点的错误值得记住。** 我把 `?official=true` 当成报表口径列进了迁移清单，实际它是版本地位。判定方法很简单——问这个词说的是「包含什么」还是「哪一版权威」——但在盘点时容易跳过。同一个词出现在两个语义里，是词表收敛这类工作最常见的陷阱。
