# UIUX 审查报告（2026-08-26）

> 审计方法：`ui-ux-pro-max` skill 检索（design-system / ux / chart 规则库）+ [DESIGN.md](../DESIGN.md) 现行约束逐条核对 + 前后端接口面机械对齐（前端 137 个端点字面量 × 后端 275 条路由）+ 项目自有守卫实测运行（非文档声称）。
>
> 基线对照：skill 返回的推荐方向是「Data-Dense Dashboard / 克制 / 无彩底 / tabular 数字」——与 DESIGN.md 现行规范一致，因此本报告以 **DESIGN.md 为判定基准**（项目规则严于通用建议）。

---

## A. 前后端接口完整性 ✅ 总体健康

| 检查项 | 结果 |
|---|---|
| 前端端点 → 后端路由覆盖 | **137/137 全部命中**，无断链 |
| 疑似缺失 5 项（`ai/chat/artifacts`、`financial-model/runs` 等） | 逐一验证为模板字符串拼接前缀，**全部假阳性** |
| 错误契约对齐 | 后端 `errcontract` 15 个 code，前端 `ApiError.userMessage` 覆盖 14 个；**仅 `rate_limited` 未映射**（会落到 generic 文案） |
| `/api/ai/files/upload` 经 Next rewrite → core `/api/v1/ai/*` | 链路正确（W5-5 已接线） |

### 接口层发现（3 项）

1. **6 处裸 `fetch()` 绕过 `apiRequest`**：
   - `web/app/ai-chat/page.tsx:254`（artifact 导出）
   - `web/app/ai-chat/page.tsx:1666`（文件上传）
   - `web/app/retail-data-import/page.tsx:60`
   - `web/app/reports/page.tsx:439`
   - `web/app/lib/retail-export.ts:39`
   - `web/app/ai-chat/runtime/transport.ts:60`（SSE 可豁免）

   这些调用**没有 401 自动刷新 token 的逻辑**——token 过期时用户会看到一次莫名失败，重试才恢复。建议统一走 `apiRequest` 或抽一个 `downloadBlob` 共享缝。

2. **`rate_limited` 未在前端错误映射表中**：触发限流时用户看到的是「请求失败」而不是「请求过于频繁」，违反 ERR-002 的「分支于共享错误契约词表」原则。

3. **后端 102 条路由无前端消费者**：抽查确认绝大多数是 agent-runner 内部 API（runs/lease/checkpoint/capabilities），属预期；但 `budget-versions/compare`、`budget-versions/management-brief`、`contracts-by-status` 值得确认是遗留还是漏接。

---

## B. UX 规则审计（按 ui-ux-pro-max 优先级 1→10）

### 通过项 ✅（实测验证，非文档声称）

| 类别 | 证据 |
|---|---|
| 设计令牌单一真相源 | `tokens-alignment.test.ts` 9 测试通过；暗色令牌测试在位 |
| 容器原语（PRIM-001） | `container-primitives.test.ts` 通过；4 个图表组件全部有 `Tooltip` 且包 `ResponsiveContainer`；`connectNulls={false}` 是正确用法 |
| 字重纪律 | 全仓 **0 处** fontWeight 700/800（DESIGN.md §14 记载的 2 处已清完） |
| `!important` | globals.css 仅剩 **5 处**（从 142 → 34 → 5，持续收敛） |
| 语义化交互元素 | `<div onClick>` 非 semantic 元素 **0 处** |
| 键盘可访问 | `focus-visible` 23 条规则；`prefers-reduced-motion` 全局处理在位 |
| 图片可访问性 | 无 alt 的 `<img>` 仅存在于测试 fixture |
| 响应式 | AppLayout 有 768px Drawer 断点，符合 §5.2 |
| diff 级守卫 | `enforce-design.mjs` 当前通过（1 变更文件无违规）；债务基线测试通过 |
| i18n | 2985 keys，三语字典完整 |

### 发现项 ⚠️（按优先级排序）

#### P1 — 数据可信度展示覆盖不全（DESIGN.md §10，产品差异化核心）

`DataTrustBar` 只出现在 5 处（operating-pulse、store-360、scenario-workbench、ai-chat、home）。但以下页面**展示经营数据且无任何 classification/trust 标识**：

- `/promotions`、`/portfolio`、`/performance` —— grep `simulated|classification` 均 **0 命中**
- `/retail-data-import`

`/store-pnl` 用内联 provenance 行满足了 §10 的实质（classification、decision_ready、source_days 都渲染了），不算违规但不是标准组件。底线 2 要求模拟数据全程带标识——**promotions/portfolio/performance 三页是最接近违反底线 2 展示面的地方**。

#### P2 — §13-5 `<Tag color="…">` AntD 预设色 31 处 / 11 文件

集中在 `fpna-workbench/components/*`（13 处）、`store-360/*Panel`（10 处）、settings、cashflow-forecast。这正是 enforce-design.mjs **未实现的规则缺口**（DESIGN.md §15 已承认），所以它一直在静默增长。大面积彩色 Tag 违反「状态不做大面积彩色填充」。建议下一轮把 §13-5 并入 enforce-design.mjs。

#### P3 — 存量债务仍在水位上（受基线管理，但应持续下调）

| 项 | 当前 | 备注 |
|---|---|---|
| 内联 `style={{}}` | 916 | 从 1032 回落 ✅，但仍高；§13-2 多行块检测已补 |
| `border: 1px solid`（tsx） | 18 | §13-8，应为 shadow-as-border |
| JS hover handler | 4 | §13-3 窄规则放行了部分形态 |
| i18n dead keys | 105 | 不影响用户，属维护噪音 |

#### P4 — StateBlock 采用率 16 文件 vs 32 页面

13 个页面自拼 loading/error：audit-logs、contracts、sensitivity、admin/users、todo、ai-chat、promotions、cashflow-forecast、portfolio、monthly-closing、performance、reports、standards。抽查 portfolio/reports 均有 Alert/Empty/Spin 三态，**实质达标但形式不统一**；风险在于 `scope_denied` 是否与「无数据」可区分只有部分页面经 `classifyDataState` 保证（这触及 AGENTS.md「权限拒绝必须保持原因」红线，值得逐页人工复核）。

#### P5 — 待复测的已知项（DESIGN.md §11 自认）

单环 focus 在深色面（`--admin-surface #001529`）上的可见性自记载以来「未复测」。

---

## C. 结论与建议动作

1. **接口层可以放心**：无断链，唯一实质缺口是 401 刷新不覆盖 6 处裸 fetch + `rate_limited` 映射缺失——都是小改动。
2. **最优先修 P1**：给 `/promotions`、`/portfolio`、`/performance`、`/retail-data-import` 接入 `DataTrustBar`（或至少 classification 标识）。这不是样式问题，是底线 2 的展示面合规问题。
3. **把 §13-5 并入 enforce-design.mjs**，止住 Tag color 的静默增长。
4. 存量内联样式/border 按既有债务基线节奏消化即可，无需专项。

---

## 附：审计命令复现

```bash
# 前端端点清单
cd web && grep -rhoE '`/api/v1[^`?]*`|"/api/v1/[^"?]*"' app --include="*.tsx" --include="*.ts"

# 后端路由清单
grep -oE '(api|r)\.(GET|POST|PUT|DELETE|PATCH)\("[^"]+"' core-service/cmd/api/main.go
grep -oE 'Handle\(http\.Method[A-Za-z]+, "[^"]+"' core-service/cmd/api/main.go

# 守卫实测
cd web && npx vitest run scripts/container-primitives.test.ts scripts/design-debt-baseline.test.ts app/design-system/tokens-alignment.test.ts
node scripts/enforce-design.mjs && node scripts/audit-i18n.mjs
```

> 本报告为时点快照。存量数字（内联样式数、Tag color 数等）会随整改漂移，引用前请按附录命令重新核出。
