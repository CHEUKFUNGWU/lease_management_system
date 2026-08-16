# 任务指令：FIX-019 ~ FIX-023 + SANKEY-001 + HELP-001

状态：`READY`
优先级：P0（FIX-019、FIX-021）/ P1（FIX-020、FIX-022、SANKEY-001）/ P2（FIX-023、HELP-001）
Planner / Reviewer：Claude
基线：`fix/workstation-shell-005-014 @ 5615efa`（FIX-015~018 交付 commit）
预计工作量：约 3–4 天

本批三个来源：

- **A 组（Reviewer 复验 FIX-015~018 发现）**：FIX-019 ~ FIX-021。命令级全绿（tsc 干净 / 242 tests / enforce-design 无新增违规）、四票实现方向正确、视觉已由用户确认；但测试层有两处问题，其中一处让 P0 回归护栏形同虚设。
- **B 组（用户实机走查）**：FIX-019 由用户在 ⌘K 命令面板上发现，Reviewer 追到根因后发现影响面远大于命令面板。
- **C 组（上一轮已确认方向、尚未开工）**：SANKEY-001、HELP-001、FIX-023，全部来自 `docs/execution/reports/FIX-015_018.md` §8。

---

## 0. 必读文档

**必须读**：

- `docs/execution/reports/FIX-015_018.md` —— 本批的直接前身，尤其 **§8（已确认方向）**、**§9（本地环境现状与两个坑）**、**§4.1（i18n 债）**
- `web/app/design-system/theme.ts` —— FIX-019 的现场
- `AGENTS.md` 样式约束 + `DESIGN.md` §13-2（禁止新增静态内联样式）

**不需要读**：

- `docs/PRD_零售经营分析工作站_BP日常支撑完善.md` —— 与本批无交集
- HOME-004 / FIX-005~014 的任务书 —— 结论已被后续批次覆盖，读了会误导

---

## 1. FIX-019 全站 Modal 标题栏高 809px（**P0，一行代码**）

用户原话「command k 出来的搜索框很丑」。Reviewer 用 `getComputedStyle` 追到根因，**影响的不是命令面板一个模态框，而是全站 6 个文件里的 17 个 `<Modal>`**。

### 1.1 实测数据（1440×900，打开 ⌘K 命令面板）

| 元素 | 实测高度 |
|---|---|
| `.ant-modal-content` | **1371.5px**（视口 900，顶部溢出到 `top: -389`） |
| `.ant-modal-header` | **809px** |
| `.ant-modal-title` | **768px**（内容只有「搜索与快捷操作」七个字） |
| `.ant-modal-body` | 531px（正常） |

`.ant-modal-title` 的 computed `line-height` 是 **`768px`**，AntD 生成的规则原文是 `line-height: 32` —— **无单位**。无单位 line-height 是倍数：32 × 字号 24px = 768px。

### 1.2 根因与修法

`web/app/design-system/theme.ts:166`：

```ts
Modal: {
  titleFontSize:   typography.sizes.h1.size,        // 24
  titleLineHeight: typography.sizes.h1.lineHeight,  // 32  ← 传了像素值
```

AntD 的 `titleLineHeight` token 要的是**比值**，不是像素。同文件第 48 行的全局 token 就写对了：

```ts
lineHeight: typography.sizes.body.lineHeight / typography.sizes.body.size,
```

166 行漏了这个除法。改为 `typography.sizes.h1.lineHeight / typography.sizes.h1.size`（32/24 ≈ 1.333），标题回到 32px，header 回到 72px（32 + 上下各 20 padding）。

### 1.3 顺带排查（本票范围内）

`theme.ts` 里**所有**传给 AntD 的 `*LineHeight` / `lineHeight*` token 逐个核对是否为比值。报告里给一张表：token 名 / 当前值 / 是比值还是像素 / 是否需要改。`typography.sizes` 里存的全是像素，凡是直接引用而没做除法的都是同一个 bug。

### 1.4 ⚠️ 验收方式特别说明

**这个缺陷源码里看不出来** —— 出问题的 CSS 是 AntD 在运行时按 token 生成的，既不在 `globals.css` 里，`enforce-design` 查不到，`chatLayout.test.ts` 那套读文件正则的办法也够不着。

**验收必须是运行时实测**：

```
web: npm test -- theme
```

断言 `theme.ts` 导出的 Modal token 中 `titleLineHeight < 3`（比值不可能大于 3），并对所有 `*LineHeight` token 加同类断言。

另外在报告里贴出**运行时实测值**：打开任意一个 Modal，给出
`getComputedStyle(document.querySelector('.ant-modal-title')).lineHeight` 的修复前后对照。**不接受「标题看起来正常了」这类描述。**

---

## 2. FIX-020 `chatLayout.test.ts` 大段重复（P1）

现状：`describe` 块被复制粘贴多份 —— FIX-005 **3 份**（行 96 / 157 / 167，逐字相同），FIX-007 / 008 / 009 / 013 各 **2 份**。全文 21 个 `it()` 里约 11 个是复制品，227 行能瘦掉一半。

所以「242 passed，比基线 +8」这个数字里有一部分是复制出来的，不是新增覆盖。

**真正的风险不是浪费**：以后有人改其中一份而漏掉另一份，两份契约会静默分叉，谁也不知道哪份是权威。

**要求**：去重，每个契约只保留一份。**去重后测试数会下降，这是预期结果，不是回归** —— 在报告里写明去重前后的数量差，并逐条说明删掉的都是逐字重复（不是删了独立用例）。

---

## 3. FIX-021 FIX-005 的回归护栏是空的（**P0**）

`chatLayout.test.ts` 里这条断言：

```js
// The money KPI value line must never reflow: the flex value row has no
// wrap permission anywhere in the home right column.
expect(css).toMatch(/\.home-right-kpis[\s\S]*?flex-direction:\s*column/);
```

**它锁不住任何东西。** `.home-right-kpis` 的实际定义（`globals.css:1593`）是：

```css
.home-right-kpis { display: grid; grid-template-columns: repeat(2, minmax(0,1fr)); gap: 16px; }
```

**根本没有 `flex-direction`**。正则里的 `[\s\S]*?` 会一路懒惰扫到 69 行之后的第 1662 行，命中一条完全无关的规则 —— 所以这条断言恒真。

FIX-005 是这批的 P0、是对已交付成果（金额卡固定高度 + 值行截断）的回归修复，结果它的回归护栏是空的。

**要求**：

1. 用 `ruleBody()` 精确取规则体，不要对 `css` 全文做跨规则正则 —— 文件里已有这个辅助函数，这一条绕过了它。
2. 断言真正决定「值行不折行」的那条规则自身含 `white-space: nowrap` + `text-overflow: ellipsis`（先确认是 `MoneyKPICard` 的哪个类，再写死它）。
3. 同一断言里 `330px` 那半截是**有效的**（确实锁住了 `.home-grid` 列宽），保留。

**⚠️ 拦坑**：全文件扫描式的 `expect(css).toMatch(/A[\s\S]*?B/)` 这个写法在本文件里可能不止一处。**顺手全部排查一遍**，凡是「选择器 + 懒惰通配 + 属性」的组合都要改成 `ruleBody()`。报告里列出你改了哪几条、各自原本会误命中哪一行。

---

## 4. FIX-022 `AuthContext` 的 `JSON.parse` 没有兜底（P1）

上一轮交付报告 §9.2 自己标出来的，Reviewer 已核实属实。`web/app/context/AuthContext.tsx:35`：

```ts
if (storedToken && storedUser) {
  setToken(storedToken);
  setUser(JSON.parse(storedUser));   // 无 try/catch
}
```

localStorage 里的 `user` 一旦损坏，异常在 `setIsLoading(false)` 之前抛出 —— 页面**永久卡在「加载中…」且不跳登录页**，用户没有任何自救路径（除非自己会清 localStorage）。

**要求**：`try/catch` 包住，解析失败即按 logout 处理（清 token + user，跳登录页），并在 `catch` 里留一行说明为什么不静默吞掉。

**⚠️ 为什么这票值得单独做**：它的现象与上一轮 §9.1 那个「`next dev` 与 `next build` 共用 `.next` 目录导致 chunk 404」**长得一模一样**（都是永久「加载中…」），但成因完全不同。留着它，以后每次排查都要先排除一遍。

---

## 5. FIX-023 三个页面接入 i18n（P2，是恢复状态显示的前置）

上一轮 FIX-017 把 ROI 测算 / 签约前决策 / 条款比价三页的状态句**整条删除**了（「当前假设 · N 份合同」「最近测算 CNY · N 个年度期间」「已比较 N 个方案」），而不是像其他页那样移到标题旁做计数。

原因是这三页**零 i18n 接线**（无 `useLanguage`，各有 22 / 49 / 39 处硬编码中文），加计数等于新增硬编码中文，被 `enforce-design` 实际拦下过一次。**上一轮守住了守卫、没去改它，这个处理是对的** —— 但代价是三页少了用户能看见的信息。

**要求**：

1. 三页接入 i18n（`useLanguage` + 全部硬编码中文转 key，三语齐备）。
2. 恢复那三条状态显示，按 FIX-017 定的形态放在标题旁 `.page-header-count`。
3. **不要顺手改这三页的其他任何东西** —— 这是一张纯搬运票，混入逻辑改动会让 review 无法分辨。

**验收**：
```
web: node scripts/enforce-design.mjs
web: npm test
```
外加 `grep -c` 给出三页改动前后的硬编码中文数量（应为 22/49/39 → 0）。

---

## 6. SANKEY-001 利润流向桑基图（一期）（P1）

方向已在上一轮确认，**一期今天就能做**，不需要新采集任何数据。

### 6.1 为什么一期没有阻塞

`retail_store_day_facts` 表里 `revenue`、`gross_profit`、`labor_cost`、`fixed_rent`、`variable_rent`、`non_lease_cost`、`other_controllable_cost` **全部是 `DECIMAL(18,2)` 金额字段**，只是 API 对外投影成了 `labor_cost_rate` 这类比率。所以：

- 不需要新事实表、不需要新导入链路
- **不存在**前端拿比率乘回金额导致轧不平的问题（金额是一手数据）
- recharts 3.8 自带 `Sankey`，无新依赖

### 6.2 一期范围

| 侧 | 粒度 |
|---|---|
| 营收侧 | **不分品类**（单一「营业额」节点） |
| 费用侧 | 按 labor / rent / non_lease / other 分流 |

能回答：钱花在哪、门店贡献是怎么来的。

### 6.3 建议契约（可调整，但字段语义不许少）

```
GET /api/v1/retail/stores/{id}/pl-flow
{ nodes: [{ key, label }],
  links: [{ from, to, value }],
  currency, basis, residual, status, formula_version }
```

`residual` 与 `status` 是硬要求 —— 桑基图左右不平读者一眼就能看出来，必须显式表达而不是悄悄抹平。口径继续遵守既有底线：Working / 模拟数据标识、来源追溯、`retail-kpi-v1` 语义、经营占用口径 ≠ IFRS 16 口径。

### 6.4 ⚠️ 二期、三期不在本票范围，但现在就要写进注释

- **二期（营收按大类分流）是数据模型问题，不是图表问题**。`retail_store_day_facts` 的唯一键是 `(store_id, business_date, version, source_system)`，**整表没有任何品类或 SKU 列**。要做必须新增 `store × date × category` 事实表加一整条导入链路；给现表加维度会破坏唯一键并打乱所有现有 KPI 口径。前置依赖是**目标客户的 POS 能否按品类出数** —— 这是商务问题，不是工程问题。
- 大类映射将来要复用现表已有的 `mapping_status IN ('mapped','unmapped','ambiguous')` 三态语义，**不要另发明一套**；未匹配的销售额必须在图上显示成**独立的一条流**。
- **三期（品类利润）的警告**：营销、活动、人工全是店级记录，没有品类归属。要算「男装利润」就必须分摊，而按销售额 / 按陈列面积 / 按导购工时三种算法会得出三个完全不同的答案。**分摊是判断**，与产品页面上「仅供 Working 经营分析，不作解释性判断」的自我声明直接冲突。真要做，分摊规则必须可配置、图上标注、且能切回不分摊视图。

**本票只做一期。** 把 §6.4 这三段作为注释写进接口定义处，避免下一个人想当然地加维度。

---

## 7. HELP-001 页面使用教程面板（P2）

方向已确认：页面右侧放感叹号，点开是整页使用教程含流程图。落地要点（上一轮已定，照做）：

- **入口**：`PageHeader` 单开 `help` 槽位，**不要混进 `primaryAction` / `secondaryAction`** —— 它不是操作按钮
- **展现**：用 Drawer 不用 Modal（`RetailAIDrawer` 已是此模式，可以边看教程边操作页面）
- **流程图**：recharts 画不了流程图。倾向 mermaid（文本即图、可维护），备选静态 SVG 资源或手写 SVG 组件 —— **选型请先说明理由再动手**
- **内容**：独立结构化内容，与页面代码分开；三语齐全是主要工作量
- **红线**：**合规口径不得收进教程面板**，必须常驻页面（FIX-017 已把门店 360 与租金谈判测算的口径句移到 `meta` 槽位，不许再动）

**本票建议只做 1–2 个页面的样板**（推荐经营脉搏 + 租金谈判测算），把机制跑通，其余页面另开票批量填内容。

---

## 8. 提前拦坑

1. **FIX-019 改的是全局 token**，17 个 Modal 全受影响。改完要逐个打开确认没有反向压扁（尤其 `ai-chat` 的上传弹窗与 `admin/users` 的表单弹窗）—— 用运行时实测值，不要靠眼睛。
2. **FIX-020 去重后测试数会掉**，这是预期。别为了保住 242 这个数字而留着重复块。
3. **FIX-021 不要把断言改成永远能过的形式** —— 这张票存在的理由就是上一次这么干了。改完请自问：如果我把 `330px` 改回 `300px`，这条测试会不会红？不会红就是没改对。
4. **SANKEY-001 不要用比率乘回金额** —— 金额字段是现成的，乘回去会引入舍入误差并让 `residual` 失去意义。
5. **`next dev` 与 `next build` 共用 `.next` 目录**：跑完 `npm run build` 会覆写正在运行的 dev server 的 chunk，页面卡「加载中…」且 `main-app.js` 404。**这不是代码 bug**，重启 dev server 即可。上一轮在这里误判过一次。
6. 全批次：**不得删除或隐藏任何既有页面、路由、API 与 IFRS 16 能力**（转型执行看板规则 11/12）。

---

## 9. 交付要求

- 每张票独立 commit，commit message 带票号；继续在 `fix/workstation-shell-005-014` 上叠加（该分支已有 15 个 commit 未推送），**不合 main**，推送后通知 Planner。
- 合并交付报告写到 `docs/execution/reports/FIX-019_023.md`，分节对应票号：**结构变更清单** + **逐条自查表** + **你无法确认的部分**。
- 验收标准全部写成**可执行命令 + 实际输出**。**不接受「已验证」「看起来正常」这类结论式描述。**
- 你没有视觉能力，所以不要写「外观变化清单」。改为列**可从代码直接读出的结构事实**与**运行时实测数值**（`getComputedStyle()` / `getBoundingClientRect()`）—— 这些是机械可取的。FIX-019 尤其如此，它的整个缺陷就只存在于运行时。
- 若时间不够，**先交 FIX-019 与 FIX-021**（一个是全站可见缺陷，一个是 P0 护栏失效），其余顺延并在报告里说明。

---

## 10. 理解确认题

1. FIX-019 里，`titleLineHeight: 32` 为什么会变成 768px？同一个文件里哪一行是写对的范例？
2. 为什么 FIX-019 的验收不能只跑 `npm test` 和 `enforce-design`？
3. FIX-021 那条断言为什么恒真？`[\s\S]*?` 实际命中了哪里？
4. FIX-020 去重之后测试总数会变多还是变少？这算不算回归？
5. SANKEY-001 一期为什么不需要新表？二期为什么不能给现表直接加品类列？
6. HELP-001 里，哪一类文案**绝对不许**收进教程面板？
