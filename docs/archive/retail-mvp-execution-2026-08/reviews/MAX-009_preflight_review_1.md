# MAX-009 Preflight Review 1

> ⚠️ **ARCHIVED 2026-08-18 — 不是现行依据**
> 归档理由：零售 MVP（MAX-001~009）转型执行的过程记录，全部工单已 ACCEPTED；未完成的延后治理项见 docs/execution/精简MVP路线与延后治理清单.md
> 现行入口：`docs/AI_文档索引与现行决策.md`

日期：2026-08-13  
Reviewer：Codex 主任务  
任务状态：`IN_PROGRESS`（非正式验收结论）

## 结论

MAX-009 尚未达到 `IN_REVIEW` 条件。现有证据确认一项 P1 移动端发布阻断；其余核心页面截图属于旧运行态、错误态或无效黑屏，不能支持 MVP `GO`。浏览器权限恢复前保持 `IN_PROGRESS`，不得用 mock、raw CDP、命令行截图或伪造结果替代真实浏览器验收。

## 已确认发现

### P1 — AI Chat 在 390×844 下发生页面级横向溢出

证据：`docs/execution/evidence/MAX-009/blocked-ai-chat-mobile-390x844.png`。

实际表现：会话侧栏占据大部分视口，聊天正文被压缩成近似竖排，关键上下文与操作不可正常阅读，违反 MAX-009 第 6、9 节的移动端验收标准。

代码定位：

- `web/app/ai-chat/page.tsx:771`：会话侧栏固定 `width: 260`；
- `web/app/ai-chat/page.tsx:1816-1822`：页面壳固定使用桌面端 `margin: "-32px -48px"`；
- `web/app/ai-chat/page.tsx:1897-1903`：消息区固定 `padding: "20px 20%"`；
- `web/app/ai-chat/page.tsx:2480-2486`：输入区固定 `padding: "16px 20%"`；
- `web/app/globals.css:886-917`：移动端 AppLayout 内容 padding 已改为 16px，因而桌面负边距在移动端不再匹配。

最小修复边界：

1. 仅为 AI Chat 页面增加局部响应式 class/状态，不修改 AppLayout、全局导航、桌面栅格或设计 token；
2. `<=767px` 时将会话历史放入默认关闭的 Drawer，保留选择、删除和新建能力，并提供可键盘操作、具备可访问名称的打开按钮；
3. 移动端页面壳与 AppLayout 的 16px padding 对齐，消息区和输入区使用可读的移动端左右边距；
4. 桌面端 260px 会话侧栏和原排版保持不变；
5. 补自动化状态/可访问性测试，并在真实浏览器记录 `scrollWidth <= clientWidth`、键盘开关 Drawer 和 390×844 修复后截图。

### P1 — 当前发布证据不足，真实 E2E 主链未成立

现有 8 张截图逐项复核：

| 页面 | Desktop | Mobile | 结论 |
|---|---|---|---|
| Operating Pulse | 显示“数据不存在或已被移除” | 同一错误态 | 旧运行态，不是 Golden 晨检证据 |
| Store 360 | 黑屏 | 黑屏 | 无效证据 |
| Scenario Workbench | 黑屏 | 黑屏 | 无效证据 |
| AI Chat | 页面可见，但为旧实例且未显示零售 Agent Starter | 页面溢出 | 不满足 MAX-008/009 主链 |

Reviewer 已以非破坏方式重建当前共享 `core-service` 与 `web` 容器。Executor 获得浏览器权限后必须刷新当前实例，从 fixed seed 重新运行完整主链并替换所有 `blocked-*` / stale 截图；最终证据还必须覆盖 Console、Network、键盘、action confirm、AI proposal/source 和旧页面兼容性。

## 当前外部阻塞

Reviewer 的 in-app Browser 新建本地页面操作被用户权限明确拒绝。根据工具授权约束，本轮没有尝试 alternate browser、Playwright CLI、raw CDP 或其他绕过方式。此项属于真实浏览器验收的外部前置条件，不改变上述已确认产品缺陷，也不能把任务提前标为 `IN_REVIEW`、`GO` 或 `ACCEPTED`。

## Executor 下一步

1. 在不需要浏览器的阶段完成上述 AI Chat 局部响应式最小修复及自动化测试；
2. 浏览器权限恢复后，在最新共享实例重跑 fixed-seed 全链并补齐证据；
3. 通过全部 Go/vet/Web/build/真实 PostgreSQL 与五条底线回归；
4. 创建报告、演示脚本、发布清单和证据索引，给出真实 `GO`/`NO-GO`；
5. 仅在所有 MAX-009 验收项完成后改为 `IN_REVIEW`，等待正式 Review。
