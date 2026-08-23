# Spec：架构蓝图缺口补齐（B1 批次 · SVG 可视化 / 草稿复核工作台 / IM 网关）

> 编制：2026-08-23 · 状态：`ready-for-agent`
> 来源：[docs/architecture/ARCHITECTURE_BLUEPRINT.md](../architecture/ARCHITECTURE_BLUEPRINT.md)，经代码实测复核后重写缺口清单
> 发布位置说明：本仓无 issue tracker，按既有惯例落 `docs/specs/`；`ready-for-agent` 标签以本行声明代替
> 配套模块设计：[CodebaseDesign_蓝图缺口补齐_模块深化.md](../CodebaseDesign_蓝图缺口补齐_模块深化.md)

---

## 先决：蓝图与代码的实测差异

蓝图的工程数字基本准确（28 个内部包 / 31 个页面 / 90 张表 / 迁移至 059，四项逐条核对通过），但有两处命名错误与三处「描述为已建成、实际零代码」，本 spec 以实测为准：

| 蓝图表述 | 实测 |
|---|---|
| `contract_drafts` 表 | 实为 `ai_contract_drafts`（`db/init/01_init.sql:268`），审批列为 `approval_status` |
| `services/contracts/drafts/` 草稿箱服务 | 不存在。草稿服务是 `services/draftapp`（`services/contracts/` 下只有 `discount_rate.go`） |
| `internal/gateway/` 多渠道 IM 网关 | **零代码**。全仓对 feishu / wecom / lark 的命中只在 `.gomodcache` 里 |
| `agenttools/svg_chart_tool.go` + `web/app/components/ai/SvgVisualizer.tsx` | **零代码**。两个文件都不存在，`web/app/components/ai/` 目录不存在 |
| 「Next.js ReactMarkdown 代码块拦截器」 | **依赖不存在**。`web/package.json` 里没有任何 markdown 渲染库，也没有 DOMPurify |
| `/contracts/drafts` 集中复核工作台 | 页面不存在（`web/app/contracts/` 只有 `[id]` / `new` / `page.tsx`）。**且后端也不完备**：草稿 REST 面只有 `GET /ai/chat/draft-batches/:id` 与 `POST /ai/chat/draft-batches/:id/retry`，没有待审列表、没有逐字段修改、没有批准/退回端点 |

蓝图 §三（per-user runtime）、§四闭环 B/C 已经建成，本 spec 不覆盖：会话隔离在 `handlers/agent_gateway_sessions.go`（已按 `legal_entity_id` 绑定），归因在 `services/varianceattribution`，三表与底稿在 `finmodel/` + `storepnl/` + `workingpaper/`。

---

## Problem Statement

站在大区财务 BP、门店经营分析师、合同录入 Editor 三类使用者的位置上，蓝图承诺但代码没兑现的部分造成三个具体障碍。

**第一，归因结论算得出来但读不进去。** `services/varianceattribution` 已经用连环替代法把利润总差异拆成客流、转化率、客单价、占用成本各自的贡献，逐格锁定、望远镜和恒等。但这个结果目前只能以表格呈现，而「一个总差异如何被逐个因子推高又推低、最后落到期末值」这种结构，表格读不出来——它本质上是一张瀑布图。BP 拿到表格后的第一件事是复制到 Excel 里画瀑布图，画完才看得懂。系统算对了，但没把结论送到人的眼睛里。

**第二，批量录入的草稿会丢。** AI 一次识别 10 份以上合同时，`draftapp` 已经正确地把它们批量写进了 `ai_contract_drafts`，幂等与续跑都有。但复核只能在 `/ai-chat` 的对话流里逐条点确认：会话一关，这批草稿就只剩一个 `batch_id` 能查，没有任何界面列得出「我这里还有哪些待审草稿」。Editor 的真实节奏是上午集中拍照上传、下午集中复核，而系统只支持「上传完立刻在聊天窗里一条条处理完」。赶时间的人于是绕开 AI 录入，回去手工填表——AI 识别这条链路在最需要它的批量场景下反而不可用。

**第三，人不在电脑前的时候系统够不着他。** 店长在店里巡场、大区 BP 在路上跑店，异常经营指标发生的当下他们都不在浏览器前。等回到电脑前打开 `/operating-pulse`，异常已经过去几天了。

## Solution

三块互相独立，可以分三批交付，任何一块单独上线都不依赖另外两块。

**SVG 可视化**：后端新增两个纯函数 Tool（瀑布图、流程图），把已经算好的归因结果渲染成确定性 SVG，作为强类型 artifact 走现有 artifact 通道送到前端；前端一个纯函数白名单消毒后渲染。不引入 markdown 渲染库——那等于让界面渲染任意 LLM 文本，攻击面比这个功能本身大一个量级。

**草稿复核工作台**：补齐草稿的 REST 面（列表 / 详情 / 字段级修改 / 批准 / 退回 / 批量批准），新增 `/contracts/drafts` 页面做左右双栏复核。审批流程完全复用既有的 `approval_status` 与六角色矩阵，不新建流程概念。这一块是把 `draftapp` 已有的能力接出来，不是新写业务逻辑。

**IM 网关**：**vendor `sipeed/picoclaw` 的飞书与企微渠道实现（MIT），自写租户层。** picoclaw 是 Go 1.25 单体 AI 助手，`pkg/channels/` 下有 21 个渠道实现，飞书与企微都是成熟的 **WebSocket 外拨**长连接（不是 HTTP webhook，因此不需要公网可达的回调地址，也没有入站签名要验）。协议拼装、重连、媒体处理这些与业务无关的部分直接复用，不重写。

自写的是 picoclaw 结构上没有、也不可能有的那一层：**渠道身份 → 内部用户 → `access.Scope`** 的映射。picoclaw 是个人助手，它的 `IsAllowed(senderID)` 是防打扰白名单，不是权限判定；本系统需要的是跨法人隔离。这条边界是 Ch3 的全部风险所在。

接线方式是**在 `main.go` 接线但默认关**（配置开关，无凭据则不启动），与既有 `worker` profile 同一范式。

## User Stories

### SVG 瀑布图与流程图

1. 作为大区财务 BP，我想在门店诊断结论里直接看到利润差异的瀑布图，以便一眼看出哪个因子吃掉了利润，而不用把表格复制进 Excel 重画
2. 作为大区财务 BP，我想让瀑布图的每一段都能悬浮看到精确数值与因子名称，以便在汇报时引用准确的数字
3. 作为经营分析师，我想把瀑布图导出为 `.svg` 文件，以便放进给管理层的汇报材料而不损失清晰度
4. 作为经营分析师，我想让瀑布图上标出的替代顺序（`DecompositionOrder`）可见，以便知道这组贡献值是在哪个顺序假设下算出来的
5. 作为经营分析师，我想在归因端口不可得时看到明确的「无法生成」说明，而不是一张空白或者被 0 填满的图
6. 作为集团 FP&A，我想看到三表模型版本间差异的瀑布图，以便理解 v1.1 相对 v1.0 的净利变化由哪些科目驱动
7. 作为集团 FP&A，我想看到当前审批流程的流程图，以便知道一份草稿卡在哪一环
8. 作为安全负责人，我想确保 AI 生成的 SVG 里任何脚本、事件处理器、外部引用都被剥离，以便这个功能不成为 XSS 入口
9. 作为安全负责人，我想知道每次消毒剥离了什么，以便在剥离量异常时发现被尝试的注入
10. 作为开发者，我想让同一份输入永远生成逐字节相同的 SVG，以便能用 golden 文件锁死渲染结果
11. 作为使用模拟数据的分析师，我想让基于 `simulated` 数据生成的图上带有模拟标识，以便它不被误当作真实经营结论（底线 2）

### 草稿复核工作台

12. 作为合同录入 Editor，我想看到一个列出我全部待审草稿的页面，以便上午上传、下午复核，而不必在会话关闭前处理完
13. 作为合同录入 Editor，我想按状态、置信度、缺失项数量、导入批次筛选待审草稿，以便优先处理最可能有问题的那些
14. 作为合同录入 Editor，我想在左栏看 PDF 原件、右栏改字段，以便逐项核对而不用来回切窗口
15. 作为合同录入 Editor，我想点击某个字段时原件上对应位置高亮，以便快速定位这个值是从哪里读出来的
16. 作为合同录入 Editor，我想看到每个字段的置信度，以便把注意力放在 AI 没把握的地方
17. 作为合同录入 Editor，我想让低置信度字段在我逐个确认之前无法提交批准，以便不会因为手快而放过一个错值
18. 作为合同录入 Editor，我想随时保存离开、之后接着改，以便复核可以被打断
19. 作为合同录入 Editor，我想看到缺失的必填项清单，以便知道还差什么才能提交
20. 作为 Finance Reviewer，我想批量选中一组已复核草稿一次性批准，以便处理 50 份同一批次的合同时不用点 50 次
21. 作为 Finance Reviewer，我想在批量批准部分失败时看到逐条结果并对失败项续跑，以便不必整批重来
22. 作为 Finance Reviewer，我想退回某份草稿并附上原因，以便 Editor 知道要改什么
23. 作为 Finance Approver，我想看到 AI 提取值与人工最终值的逐字段差异，以便判断这份草稿被改动了什么
24. 作为审计人员，我想看到每份草稿从生成到入库的完整动作日志（谁改了哪个字段、谁批准的、什么时候），以便复演这条入库路径
25. 作为 Finance Approver，我想确保草稿批准后才进入正式台账，且 AI 路径对 IFRS 16 正式表零写入，以便底线 5 不被破坏
26. 作为任一角色，我想只看到自己权限范围内法人的草稿，以便底线 1 成立
27. 作为 Editor，我想让同一份文档重复上传不产生第二条草稿，以便底线 4 成立
28. 作为 Editor，我想在页面上看到这批草稿的数据分类（production / simulated / mixed），以便模拟数据不被误批进正式链路

### IM 网关

29. 作为店长，我想在飞书里收到我这家店的异常经营指标预警，以便巡场时就知道出了什么问题，而不是几天后回到电脑前才发现
30. 作为大区 BP，我想在企微里直接问「上周华东哪家店毛利掉得最多」，以便在路上就能拿到答案
31. 作为店长，我想在飞书卡片上看到关键数字与一个直接打开对应门店详情页的带参链接，以便快速判断要不要深入看（**图表本身留在 Web 端**：飞书交互卡片与企微模板卡片都不渲染 SVG，栅格化成 PNG 会引入图形库依赖，与「前端新增依赖为零」同一类克制。2026-08-23 确认降级）
32. 作为使用者，我想让飞书/企微里的回答与 Web 端完全一致（同一套 Tool、同一套数据），以便不存在「两个渠道两个答案」
33. 作为安全负责人，我想让渠道身份映射后的权限范围由与 JWT 完全相同的解析器产出，以便不存在第二条权限判定路径（底线 1）
34. 作为安全负责人，我想让未绑定的渠道身份被直接拒绝，绝不落到任何默认租户，以便不存在越权读取的路径
35. 作为安全负责人，我想让 picoclaw 自带的 `IsAllowed` 白名单明确地**不**参与权限判定，以便不会有人误以为它是授权机制
36. 作为安全负责人，我想让渠道在未配置凭据时不启动，以便默认状态下不存在任何外部连接
37. 作为管理员，我想用一个配置开关控制渠道启停，以便无需改代码就能停掉一个出问题的渠道
38. 作为平台工程师，我想让 vendor 进来的 picoclaw 代码有明确的来源标注与版本记录，以便日后能对上游更新做对比
39. 作为平台工程师，我想让我们自写的租户层与 vendor 代码有清晰边界，以便上游更新时不会覆盖掉我们的安全逻辑
40. 作为审计人员，我想让经由 IM 渠道发起的每一次 Tool 调用与 Web 端一样进审计日志（含渠道来源），以便追溯路径不因入口不同而断裂
41. 作为店长，我想让 IM 渠道对我不可见的数据保持 `scope_denied` 的明确拒绝，以便拒绝原因不被软化成「无数据」

### 增量叠加纪律

42. 作为现有用户，我想让所有既有页面、路由、API 在这三块交付后行为不变，以便转型是叠加而不是替换
43. 作为开发者，我想让新增页面沿用既有 `AppLayout`、`useRetailQuery` 与 `StateBlock`，以便不产生第四套取数与状态呈现范式

## Implementation Decisions

### D-B1：SVG 走 artifact 通道，不引入 markdown 渲染层

蓝图 §五描述的是「SSE 吐出 ```svg 代码块 → ReactMarkdown 拦截器」。**否决**，两个理由：`web/` 目前没有任何 markdown 渲染库，引入它意味着前端开始渲染任意 LLM 文本，攻击面远超本功能；且代码块是弱类型的，SVG 是否合法、属于哪张图、基于哪个 run，全靠字符串约定。

改为：SVG 是 `agentartifact` 协议下的强类型 artifact，带 `kind` / `run_id` / `data_classification` / `decomposition_order` 等字段。前端按 artifact 类型分发渲染，不解析对话文本。

### D-B2：消毒是白名单，不是黑名单

`sanitizeSvg` 只放行显式列出的元素与属性，其余一律剥离并计数。黑名单（「删掉 `<script>` 和 `on*`」）总会漏——`<foreignObject>`、`xlink:href="javascript:"`、`<use href="http://...">`、CSS `url()` 都是已知绕过路径。

出参形状固定为 `{ svg, stripped }`，`stripped` 是被剥离项的清单。剥离清单为空是正常情况；非空要能被观察到（Story 9）。

### D-B3：SVG 生成必须是确定性纯函数

同一输入逐字节相同，否则 golden 测试写不了。这要求三件事：小数位显式固定（不依赖 `%v`）、元素 id 由输入派生而非随机或自增计数器、不嵌入生成时间戳。

### D-B4：瀑布图不重算任何数字

各段的值全部来自 `varianceattribution` 的输出，SVG 层只做坐标与路径计算。渲染层出现任何业务算术都是在造第二个真相源（风险红线 14 的同类错误）。

`DecompositionOrder` 必须回显在图上（Story 4）——这是既有约束，不是新要求。

### D-B5：端口不可得时诚实拒绝

归因数据不可得时 Tool 返回 unavailable，**不产出图**。不画空坐标系、不用 0 填段。这是 AGENTS.md「端口未接线时工具必须诚实拒绝」的直接落点。

### D-B6：草稿 REST 面挂在合同域，不挂在 `/ai/chat/*`

现有两个端点在 `/ai/chat/draft-batches/*` 下，语义是「这次会话产生的批次」。但草稿的生命周期长于会话——这正是问题二的核心。新端点挂在合同域（`/contracts/drafts`），按草稿自身的状态与归属组织，不按产生它的会话组织。

既有两个端点保留不动（增量叠加纪律）。

### D-B7：审批复用既有 `approval_status` 与六角色矩阵

不新建审批流程概念。流转仍是 `Agent 生成草稿 → Editor 修改确认 → Reviewer 复核 → Approver 审批 → 正式入库`，权限点沿用既有 `permission(...)` 声明方式。MVP 阶段 Editor 与 Reviewer 可为同一人这条也不变。

### D-B8：批量批准是逐条幂等，不是事务性全或无

50 份草稿里第 37 份数据有问题时，正确行为是前 36 份成功入库、第 37 份带原因失败、剩余继续，而不是整批回滚。`draftapp` 已有 `Resume*Batch` 与 `BatchResult` 逐条结果结构，直接复用（Story 21）。

这与「批准动作本身要幂等」不冲突：同一草稿被批准两次不产生第二条正式记录（底线 4）。

### D-B9：字段级修改必须留差异痕迹

AI 提取值与人工最终值分列存储，两者都不覆盖对方。审计要能回答「这个值是 AI 读出来的还是人改的」（Story 23、24）。这是 AGENTS.md「AI 识别与入库」数据规范里已列明的字段，不是新增概念。

### D-B10：低置信度字段未逐个确认则拒绝批准

服务端强制，不是前端禁用按钮了事。前端禁用是提示，服务端拒绝才是控制项——按「只有用户一个人类 + 一堆 AI」的现实，靠人盯的控制项会失效，控制必须做成机器可强制的。

### D-B11：新页面遵守既有前端契约，不新建范式

取数走 `useRetailQuery`（FETCH-001 的竞态门与 token 注入），状态呈现走 `classifyDataState` / `StateBlock`（STATE-001），新枚举登记进 `code-lists-contract.test.ts`（CONTRACT-001），容器遵守 DESIGN.md §8.1，其 §13 止血条款对新代码强制生效。

**注意 UI/UX 既有债**：`globals.css` 的 `!important` 级联已经导致过线上问题，改善计划在 `docs/UIUX改善方案.md`。本页是新增页面，不改既有样式，但如果实现过程中发现必须动全局样式才能做出双栏布局，**停下来先报告**，不要在这个 spec 的范围内顺手改级联。

### D-B12：vendor picoclaw 的飞书/企微渠道，不重写协议层

上游是 [`sipeed/picoclaw`](https://github.com/sipeed/picoclaw)，**MIT License, Copyright (c) 2026 PicoClaw contributors**，Go 1.25——与 `core-service` 同语言同版本。搬入范围严格限定：

| 搬入 | 不搬入 |
|---|---|
| `pkg/channels/feishu/`（含 `feishu_32.go` / `feishu_64.go` build tag 分架构） | `pkg/agent`、`pkg/bus`、`pkg/session`、`pkg/memory`、`pkg/routing` |
| `pkg/channels/wecom/` | `pkg/identity`、`pkg/auth`、`pkg/credential` |
| `channels` 包里被上述两者依赖的最小骨架（`base.go` / `interfaces.go` / `errors.go` / `media.go` 等按实际依赖裁剪） | 其余 19 个渠道 |

**不引入 picoclaw 的 agent 循环。** 本仓已有 `internal/agentcore`（ADR-0022：first-party Go，"Not tau, not pi, not any third-party runtime"），渠道层接到既有 agent 上，不是把 picoclaw 的大脑搬进来。`bus.OutboundMessage` / `bus.SenderInfo` 这两个跨接口的类型改写成本仓自己的类型，不为它们拖进整个 `pkg/bus`。

落位 `internal/gateway/vendor/picoclaw/`，每个文件头部保留上游路径、commit SHA 与 MIT 声明；仓库根部 `THIRD_PARTY_NOTICES` 登记。**vendor 目录内不写业务逻辑**——需要改上游行为时用包装而非就地改，否则日后对不上上游。

> 这是一次第三方代码引入，触及 ADR-0021 的开源许可姿态。**实现前需要一份新 ADR** 记录「为什么 vendor 而不是依赖上游 module、搬入边界在哪、许可义务怎么履行」。ADR-0024 拆掉 AGPL 依赖是先例，说明本仓把许可当架构决策处理。

### D-B13：身份映射走与 JWT 完全相同的权限解析器

渠道身份（飞书 `open_id` / 企微 `userid`）经绑定表映射到**既有内部用户**，然后由与 JWT 路径**同一个**解析器产出 `access.Scope` 与 `agenttools.Principal`。

**绝不允许**渠道适配器自己拼装 Scope 或直接携带 `legal_entity_id`。一旦有第二条权限判定路径，底线 1 就变成了「两条路径都别写错」而不是「结构上不可能写错」。

绑定关系落新表，含渠道类型、渠道用户 id、内部用户 id、绑定时间、绑定人。

**picoclaw 的 `IsAllowed(senderID)` / `IsAllowedSender(sender)` 不是授权机制。** 它在上游是个人助手的防打扰白名单。搬进来后它可以继续存在作为第一道粗过滤，但**权限判定必须在它之后、由本仓的 Scope 解析器独立完成**；`IsAllowed` 返回 true 不构成任何数据可见性结论。这一点要有测试锁定：构造一个 `IsAllowed` 放行但 Scope 不覆盖的请求，断言数据被拒。

### D-B14：未绑定身份直接拒绝，无 fallback 租户

映射不到内部用户时返回明确的未绑定错误，**不落到任何默认 / 匿名 / 兜底租户**。给兜底留口子就会有人走——这与「口径冲突只降级不换算」是同一类纪律。

### D-B15：接线但默认关，无凭据则不启动

上游飞书与企微都走 **WebSocket 外拨**长连接（企微是「WebSocket-based AI Bot channel with route persistence」，飞书是 WebSocket/SDK 模式），**不需要公网可达的回调地址，也没有入站 webhook 签名要验**。这消除了「开公网入口」这一风险面，因此不再保留「交付但不接线」的限制。

改为：在 `main.go` 接线，由配置开关控制，**默认 off**；凭据缺失时渠道不启动并记录明确原因，不 panic、不重试刷屏。与既有 `worker` profile（`agent-runner` 默认不起）同一范式。

配置开关本身要有测试：默认配置下断言无渠道启动。

### D-B16：连接/凭据类错误与身份未绑定是两类独立错误

不合并成一个「鉴权失败」。前者是运维问题（凭据过期、网络不通、上游限流），后者是运营问题（有同事没绑账号）。日志与告警要能区分，否则真实故障淹没在日常噪音里。

WebSocket 模式下没有「签名验证失败」这个类别——原 spec 基于 HTTP webhook 的假设已作废。

### 契约变更清单

| 类型 | 变更 |
|---|---|
| 新增 Tool | `render_waterfall_svg`、`render_flowchart_svg`（读类，不写业务表） |
| 新增 artifact kind | SVG 图形类，含 `data_classification` 与 `decomposition_order` |
| 新增 REST | 草稿列表 / 详情 / 字段修改 / 批准 / 退回 / 批量批准（合同域下） |
| 新增页面 | `/contracts/drafts`（第 32 个页面） |
| 新增包 | `internal/gateway`（第 29 个内部包），含 `internal/gateway/vendor/picoclaw/` |
| 新增第三方代码 | picoclaw 飞书 / 企微渠道（MIT，vendor 非 module 依赖）；需新 ADR + `THIRD_PARTY_NOTICES` |
| 新增表 | 渠道身份绑定表（第 91 张表）；需同时提供增量迁移 `060_*` 与 `01_init.sql` 空库版本 |
| 新增配置 | 渠道启停开关（默认 off）+ 飞书/企微凭据项 |
| 前端新增依赖 | 无。**不引入 react-markdown、不引入 DOMPurify**（消毒是自写白名单纯函数） |

数据库变更必须同时提供增量迁移和 `01_init.sql` 空库版本——已有 volume 不会重跑 init，缺一就环境漂移。

## Testing Decisions

### 什么算好测试

只测外部行为，不测实现细节。判定标准是那句自检：**把被测逻辑删掉或改错，这条测试会不会红？不会红就是没写对。**

反面教材在 AGENTS.md 里有先例：`expect(css).toMatch(/A[\s\S]*?B/)` 这类跨规则通配正则会断言恒真（FIX-021 教训）；恒真的勾稽（拿别名比自己、返回 `not_applicable` 的桩）是「假装在检查」（风险红线 12）。

### 各模块的测试与先例

| 模块 | 接缝 | 测试方式 | 先例 |
|---|---|---|---|
| SVG Tool | `agenttools.ToolRuntime`（`protocol.go:191` 自述「the highest external seam」） | 经 `Execute` 调用，golden 文件逐字节比对 SVG 输出 | `agenttools/runtime_test.go`、`agenttools/protocol_test.go` |
| `sanitizeSvg` | 纯函数 `(raw) => { svg, stripped }` | 表驱动 XSS 用例，不需要 DOM | `web/app/lib/` 下既有纯函数测试 |
| 草稿 REST | HTTP handler | 端点级测试 + 错误契约与信封形状 | `handlers/envelope_shape_consistency_test.go`、`handlers/error_contract_test.go`、`handlers/ai_chat_drafts_test.go` |
| 草稿页面 | `useRetailQuery` + `classifyDataState` | 组件测试 | `web/app/store-360/variance-attribution.test.tsx`、`web/app/retail/retail-fetch-seam.test.ts` |
| 渠道身份 → Scope | 本仓自写的映射层（**不测 vendor 代码**） | 绑定表命中 / 未命中 / Scope 不覆盖 三类，逐条 | `handlers/ai_chat_runtime_permissions_test.go` |
| 渠道消息编解码 | vendor 层出入口 | 录制报文夹具 | `agentseval/corr2_baseline_test.go` 的夹具模式 |
| vendor 边界守卫 | 架构测试 | 断言 `internal/gateway/vendor/**` 不 import 本仓业务包（`repository` / `services` / `agenttools`），方向只能是外层依赖 vendor | `finmodel/importguard_test.go`、`agentcore/importguard_test.go` |
| 默认关守卫 | 配置测试 | 默认配置下断言无渠道启动 | `config` 包既有测试 |

**不为 vendor 进来的 picoclaw 代码补写单元测试。** 上游有自己的测试，重写一遍既不增加信心也会在对上游更新时变成负担。测试写在**我们的包装层与租户层**——也就是 picoclaw 结构上给不了的那部分。

### 反向测试要求

以下每条都要先证红再证绿——写一个应当被拦住的输入，证明它确实被拦住：

- `sanitizeSvg`：`<script>`、`on*` 事件属性、`javascript:` 伪协议、`<foreignObject>`、外部 `<use href>`、CSS `url()` 各一条
- 低置信度未确认时批准被拒（D-B10）：服务端拒绝，不只是前端禁用
- 跨法人：法人 A 的账号读不到法人 B 的草稿，且拒绝原因保持 `scope_denied` 不被软化成「无数据」
- 幂等：同一草稿批准两次不产生第二条正式记录
- 批量部分失败：第 N 条失败时前 N-1 条已入库、后续继续，续跑不重复入库
- `gateway` 未绑定身份被拒绝，且不落任何默认租户
- `gateway` **`IsAllowed` 放行但 Scope 不覆盖时数据被拒**（D-B13 的核心反向测试）
- vendor 边界守卫：故意在 vendor 目录里 import 一个业务包，测试必须红
- 默认关守卫：默认配置下起服务，断言无渠道连接

### 集成测试

跨法人隔离、幂等与草稿落库的证据在集成测试里，需 `TEST_DATABASE_URL`。**只跑单元测试证明不了底线 1 和底线 4**——改这些路径时必须带库跑。

### 验证命令

```bash
cd core-service && GOCACHE=$(pwd)/.gocache go test ./... && go vet ./...
cd ../web && npm run type-check && npm run build && npm test
```

## Out of Scope

- **picoclaw 的 agent 循环 / bus / session / memory**：本仓已有 `internal/agentcore`（ADR-0022）。只搬渠道层，不搬大脑
- **picoclaw 另外 19 个渠道**（Slack / Telegram / Discord / QQ / 钉钉 等）：只搬飞书与企微。上游结构支持日后增量加，但本批次不做
- **真实飞书/企微应用凭据的申请与凭据管理方案**：交付含配置项与默认关开关，实际凭据由运维配置，不在代码库
- **picoclaw 的 WebUI Launcher / `picoclaw_fui`**：后者是 Flutter 桌面仪表盘（无 chat / artifact / 审批流），与本产品前端无关
- **Auto-Post Mode**：AI 仍然只在 Assist Mode 运行。草稿必须经人工审批入库，本 spec 不改这一点
- **markdown 渲染能力**：不引入 react-markdown。对话流仍按现有方式呈现
- **`/operating-pulse` 与经营驾驶舱的合并**：两者是同意图两套后端，2026-08-22 已决定推迟，本 spec 不碰
- **`globals.css` 级联治理**：UI/UX 债的系统性修复走 `docs/UIUX改善方案.md`，不在本批次
- **重命名**：容器、数据库、MinIO bucket、JWT、Go module 一律不动（AGENTS.md「不要顺手重命名」）
- **蓝图 §三 / 闭环 B / 闭环 C**：已建成，本 spec 不覆盖
- **PNG / 位图导出**：只做 SVG
- **角色模型重构**：零售角色（区域经理、门店经理）与数据范围解耦是已知缺口，不在本批次

## Further Notes

**交付顺序建议**：SVG → 草稿工作台 → gateway。三块之间无代码依赖，可并行；但 gateway 需要先出 ADR（D-B12），所以起步最慢。

**关于「直接抄 picoclaw」的边界。** 蓝图把 `sipeed/picoclaw` 列为「智能体架构核心参考与基线」，这个定位对 Ch3 成立、对 Ch1/Ch2 不成立，值得说清楚以免后续批次误判：

| | picoclaw 能给的 | 判断依据 |
|---|---|---|
| Ch1 SVG 瀑布图 | **无** | picoclaw 无 SVG / 图表渲染能力；瀑布图的输入是 `varianceattribution` 的连环替代结果，是本仓领域产物 |
| Ch2 草稿复核工作台 | **无** | `picoclaw_fui` 是 Flutter 桌面仪表盘，无 chat / artifact / 审批流；六角色审批矩阵与置信度必审是本仓合规模型 |
| Ch3 IM 网关 | **协议层大部分** | `pkg/channels/{feishu,wecom}` 是成熟的 Go 长连接实现，MIT |
| Agent 循环本身 | **架构已借鉴完毕** | ADR-0022 已采纳 pi/picoclaw 的纯 loop + `Deps` 注入 + import guard 三个结构选择，并明确「Not tau, not pi, not any third-party runtime」 |

一句话：**picoclaw 帮得上「怎么连上飞书」，帮不上「连上之后这个人能看哪家店的数」。** 后者是本产品的全部价值，也是全部风险。

**蓝图文档需要回写**：本 spec 首节列的六处差异应订正回 `docs/architecture/ARCHITECTURE_BLUEPRINT.md`，否则那份文档会继续把「计划」描述成「已建成」。特别是目录映射表里 `internal/gateway/`、`agenttools/svg_chart_tool.go`、`services/contracts/drafts/`、`web/app/components/ai/SvgVisualizer.tsx` 四项，它们在表里的写法与已建成的包完全一样，读者无从区分。建议在蓝图里给未建成项加显式标记。

**关于 unvalidated**：本批次同样没有真实 POS/ERP/GL 联调、没有客户验证，相关结论一律保持 `unvalidated`。SVG 图表让归因结论更易读，这不改变结论本身仍基于固定 seed / 构造数据这一事实——不得因为图好看就把它表述为经过验证的经营洞察（风险红线 11）。

**关于 gateway 的风险落点**：搬 picoclaw 省掉的是协议工作量，省不掉的是权限工作量，而后者才是这个包的全部风险。picoclaw 是个人助手——它的世界里只有一个用户，所以 `IsAllowed(senderID)` 这种白名单就够了。本系统的世界里有多个法人、多个区域、多个角色，一条越界的消息等于跨法人数据泄露。

因此 Ch3 的评审重点不是「飞书连上了没有」，而是 D-B13 那一条：**渠道身份产出 Scope 的路径，与 JWT 是不是同一条。** 如果实现里出现第二个拼装 `access.Scope` 的地方，无论功能跑不跑得通，都应当退回。
