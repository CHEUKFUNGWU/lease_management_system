# MAX-009 Review 1

日期：2026-08-13  
Reviewer：Codex 主任务  
结论：`CHANGES_REQUESTED`  
任务状态：`IN_PROGRESS`

## 总结

Executor 对先前 AI Chat 390px 固定侧栏问题做了正确方向的局部修复，交付文档也如实保持 `NO-GO`，没有把无效截图冒充发布证据。Reviewer 已解除其报告中的 Docker/PostgreSQL 环境阻塞，并完成独立自动化与真实 PG 回归。但当前移动端实现仍有两个 P1 可用性缺陷，真实浏览器主链、五条底线 smoke 和发布截图仍未完成，因此 MAX-009 不能进入 `IN_REVIEW`。

## Standards 轴

### P1 — 移动端顶部栏仍可能溢出或相互遮挡

`web/app/ai-chat/page.tsx:1879-1923` 在原桌面顶部栏加入 Drawer trigger，但左侧 flex/title 没有 `min-width: 0`、ellipsis；右侧仍完整显示模型名称与新建按钮。会话标题允许较长、连续字符时，trigger + robot + title + 完整模型控件在 390px 的可用宽度内无法可靠共存，可能重现本票要关闭的页面级 overflow/关键按钮不可达。

返工：只为 AI Chat 添加 scoped header class。移动端让左区及 title 可收缩、单行省略，模型入口使用紧凑但仍可操作的表现；桌面端结构和显示不变。不得修改 AppLayout 或全局 header。

### P2 — 自动化仅验证 breakpoint 数字，不能验证交互合同

`web/app/ai-chat/responsive.test.ts:5-17` 只断言 `width <= 767`。即使页面同时渲染两个 sidebar、删除 trigger、停止 selection 后关闭 Drawer 或重新引入 CSS overflow，测试仍会通过。

返工：不新增测试依赖；抽出并让页面实际消费最小 view-state/布局合同，覆盖 390/767/768/1440、desktop sidebar vs mobile trigger/Drawer、selection/new 后 close 和 focus-return intention。浏览器的 DOM、键盘、overflow 证据仍是最终权威，纯逻辑测试不能替代。

### P3 — desktop/mobile SessionSidebar wiring 重复

`web/app/ai-chat/page.tsx:1845-1862` 与 `:2662-2684` 重复 delete-confirm closure。至少抽取一个 `confirmDeleteSession` callback；不要为消除重复引入页面架构重构。

## Spec 轴

### P1 — 会话项不可键盘操作，关闭后焦点回归未实现

预检要求移动 Drawer 可键盘使用并验证焦点。`web/app/ai-chat/page.tsx:812-921` 的会话选择仍是仅有 `onClick` 的 `<div>`，无法自然 Tab 聚焦，也没有 Enter/Space 语义；`page.tsx:1880-1887` 的 trigger 没有 ref，`page.tsx:2653-2684` 的关闭路径只改 state，没有显式将焦点返回 trigger。

返工：会话项改为语义 button 或等价可访问控件；保留会话删除菜单。为 trigger 建立 ref，在 selection/new/Escape/普通 close 后合理恢复焦点。真实浏览器必须记录 Tab、Enter/Space、Escape 与焦点结果。

### P1 — 真实发布主链及证据仍缺失

任务票 4、6、9 节要求真实 Web/Core/PostgreSQL 上完成 fixed seed → Pulse → Store360 → Scenario → AI proposal → 人工 action，并覆盖四页双 viewport、键盘、Console、Network。当前 `docs/execution/evidence/MAX-009/index.md` 中 8 张截图全部标为 `INVALID`，报告也明确未完成以上步骤。任务继续 `IN_PROGRESS / NO-GO` 是正确的，但不能作为最终交付。

### P1 — 报告中的 Docker blocker 已过期

`docs/execution/reports/MAX-009.md:12-13,37,61-63` 写 Docker socket permission denied。Reviewer 当前环境已可访问 Docker，并完成如下独立验证：

- `docker compose ps`：Core、Web、AI、PostgreSQL、MinIO 均运行，PostgreSQL/MinIO healthy；
- Web：63 tests、type-check、production build 全部 PASS；
- Go：`go test ./...`、`go vet ./...` PASS；
- 真实 PG 串行 MAX-002/003/004/006/007/008 全部 PASS；
- MAX-007 `-count=2`：约 3.37s / 3.35s；MAX-008 `-count=2`：约 3.40s / 3.43s；
- 残留查询：测试法人 0、`AGENT-PROD-*` store 0、`scenario-pg-*` action 0。

返工：以当前容器刷新浏览器并继续主链，更新报告，不得继续把 Docker permission 作为停止条件。

### P2 — 缺少可执行回滚/恢复说明

`docs/execution/MAX-009_发布检查清单.md:25-30` 只有“不 down -v/精确清理”原则，没有任务票第 8 节要求的可执行恢复步骤。补充：如何重建 Core/Web、不删除 volume；如何按本票唯一 token 清 action/data fixture；如何验证旧页面/正式表前后计数；失败时如何回到明确的 `NO-GO` 状态。不得编造无法执行的命令。

## Executor 重新提交要求

1. 完成上述 AI Chat 最小返工及自动化；
2. 使用已建立的原 in-app Browser 会话，在当前 `localhost:3000` 重跑完整主链，不复用 invalid 截图；
3. 记录 390px `scrollWidth <= clientWidth`、长标题、Drawer 键盘/焦点，四页 desktop/mobile、Console/Network、action confirm 与 AI proposal/source；
4. 完成 production/A-B/五条底线 API/browser smoke 与旧 route smoke；
5. 更新报告、证据索引、演示脚本、发布清单及看板；
6. 全部通过才提交 `IN_REVIEW`；若仍有产品主链 blocker，保持 `IN_PROGRESS / NO-GO` 并给出精确复现。不得自行 `ACCEPTED`、commit 或 push。
