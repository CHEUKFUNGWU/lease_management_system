# Codebase Design：架构蓝图缺口补齐 — 模块深化设计

> 编制：2026-08-23 · 状态：Current
> 前序：[Spec：架构蓝图缺口补齐（B1 批次）](specs/blueprint-delta-omnichannel-svg-drafts-b1.md)（D-B1~D-B16 在那里，本文不重复，只在需要时引用）
> 同级：[CodebaseDesign_经营工作站诚实性与能力补齐_模块深化.md](CodebaseDesign_经营工作站诚实性与能力补齐_模块深化.md)（RH1–RH8）、[CodebaseDesign_三表模型与单店利润表_模块深化.md](CodebaseDesign_三表模型与单店利润表_模块深化.md)（SM1–SM8）
> 模块编号用 **BG**（Blueprint Gap），以免与 SM / RH / N / MAX 撞号；本文新增决策留痕从 **D-B17** 起接续 Spec。

---

## 0. 设计原则（判定标准）

1. **深度看接口的杠杆，不看实现行数。** 问「删掉它，复杂度是消失还是散到 N 个调用方去」。
2. **一个适配器 = 假想接缝，两个才是真接缝。** 本轮六个模块里只有一个建新接缝（`Channel`，两个适配器），其余全部复用既有接缝。
3. **接口就是测试面。** 想绕过接口测内部，说明模块形状不对。
4. **把错的东西做成表达不出来，而不是靠评审拦。** 只有用户一个人类 + 一堆 AI，靠人盯的控制项会失效。本文多处的接口形状是按这条选的，不是按美观选的。
5. **本轮零删除。** 表、列、路由、API、导航项一个不动。

---

## 1. 接缝总览

| 模块 | 接缝位置 | 新增? | 适配器数 |
|---|---|---|---|
| BG1 Waterfall Renderer | `agenttools.ToolRuntime`（既有，`protocol.go:191`） | 复用 | — |
| BG2 SVG Sanitizer | 前端纯函数 `web/app/lib/` | **新增（无端口）** | — |
| BG3 Draft Review | `draftapp.DraftStore`（既有） | 复用 | — |
| BG4 Channel Identity | `access.Scope` 解析器（既有） | 复用 | — |
| BG5 Channel | `gateway.Channel`（vendor 自带） | **新增** | 2（feishu / wecom） |
| BG6 Gateway Runner | `main.go` 生命周期 | 复用 | — |

新增接缝只有一个真的（BG5，两个适配器）。BG2 是纯函数没有端口。其余四个全部挂在既有接缝上。

---

## 2. BG1 Waterfall Renderer — 瀑布图渲染

### 问题

`varianceattribution` 已产出逐因子贡献，`finmodel/memo` 已产出版本间科目差异。两者的呈现需求是同一个形状：**一个起始值，一串有序的正负贡献，一个终值**。如果按调用方各写各的，坐标计算、刻度选择、标签避让、负值着色会出现两套。

### 接口

```go
// Waterfall 是渲染层与业务层之间唯一的共享词汇。
// 业务层负责翻译成它；渲染层不认识 varianceattribution 也不认识 finmodel。
type Waterfall struct {
    StartLabel string
    StartValue money.Amount
    Steps      []Step        // 有序，顺序即语义
    EndLabel   string
    EndValue   money.Amount
    Currency   string
    Classification string     // production / simulated / mixed
    OrderNote  string         // 连环替代的 DecompositionOrder，原样回显
}

type Step struct {
    Label string
    Delta money.Amount
}

// Render 是本模块的全部对外接口。
func Render(w Waterfall) (svg string, err error)
```

**一个类型 + 一个函数。** 没有 `Options`、没有宽高参数、没有配色参数。

### 深度理由

删掉它，两个调用方各自要做：值域到像素的缩放、刻度取整、连接线路径、负值配色、标签宽度估算与避让、模拟标识水印、`OrderNote` 排版、XML 转义。这些复杂度不会消失，会翻倍。

**为什么不给 `Options`。** 一旦有 `Options{Width, Height, Palette}`，调用方就要懂这些，接口从「一个业务概念」退化成「一个绘图库」——那正是浅模块的定义。尺寸由 SVG `viewBox` 交给前端 CSS 控制，配色是产品决定不是调用方决定。

**为什么 `Classification` 在 `Waterfall` 里而不是 `Options` 里。** 它是这份数据的属性，不是渲染偏好。放进数据结构意味着调用方**无法**渲染一张不带模拟标识的模拟数据图（底线 2）——把错的东西做成表达不出来。

### 内部接缝

`scale(values) → (min, max, ticks)` 是私有函数，本模块自己的测试直接测它（刻度取整边界用例多，经 `Render` 测太绕）。它不出现在接口里。

### 确定性（D-B3 的落点）

- 小数位显式格式化，禁 `%v` / `%f` 默认精度
- 元素 id 由 `Label` 派生的稳定哈希产生，**禁自增计数器**（自增在并发或顺序微调时会漂）
- 不嵌入生成时间

### 测试

golden 文件逐字节比对，经 `agenttools.ToolRuntime.Execute` 走完整路径。反向测试：改一个 `Delta` 的符号，golden 必须红。

### 决策留痕 D-B17

> **`Waterfall` 是共享词汇类型，不是端口。** 曾考虑让渲染模块定义 `WaterfallSource` 接口由业务包实现。否决：那是「一个适配器 = 假想接缝」的典型误用，且会让 `varianceattribution` 反向依赖渲染层。业务包做一次纯数据翻译更便宜也更清楚。

---

## 3. BG2 SVG Sanitizer — 前端消毒

### 接口

```typescript
export function sanitizeSvg(raw: string): { svg: string; stripped: string[] };
```

**一个函数，一个入参，无配置。**

### 深度理由

白名单表、属性解析、URL scheme 判定、嵌套规则全在实现里。调用方需要知道的只有「进去脏的，出来干净的，`stripped` 告诉你剥了什么」。

### 决策留痕 D-B18

> **禁止把白名单做成参数。** `sanitizeSvg(raw, { allowedTags })` 看起来更灵活，实际是把安全决策外推给每个调用方——第二个调用方一定会抄第一个的参数并加一项。白名单是模块的不变量，必须在实现里，不在接口里。
>
> 同理**不导出白名单常量**。导出即被引用，被引用即被绕过。

### 决策留痕 D-B19

> **不引入 DOMPurify。** 它的能力面是「消毒任意 HTML」，我们只需要「消毒受控 SVG 子集」。自写白名单的攻击面小一个量级，且不新增前端依赖（Spec 契约变更清单里前端依赖为零）。代价是白名单要自己维护——用表驱动测试锁死已知绕过路径来偿付。

### 测试

表驱动，每条一个已知绕过向量，先证红：`<script>`、`on*` 属性、`javascript:` 伪协议、`<foreignObject>`、外部 `<use href>`、CSS `url()`、`<style>` 里的 `@import`、大小写混淆（`<ScRiPt>`）、实体编码绕过。

自检句：把白名单表清空，这些测试会不会红？会红才算写对。

---

## 4. BG3 Draft Review — 草稿复核

### 问题

`draftapp.Service` 已有 30+ 个导出方法（`CreateContractDraft` / `...InStore` / `...Batch` / `Resume...Batch` × 三种草稿类型）。这个接口已经偏宽。复核需要的六个 REST 动作如果继续加到它上面，会加剧浅化。

### 接口

新模块 `internal/services/draftreview`：

```go
type Service interface {
    List(ctx context.Context, f Filter) (Page, error)
    Get(ctx context.Context, id string) (Detail, error)
    Revise(ctx context.Context, id string, edits []FieldEdit) (Detail, error)
    Decide(ctx context.Context, decisions []Decision) (Outcome, error)
}

type Decision struct {
    DraftID string
    Verdict Verdict   // approve | reject
    Reason  string    // reject 必填
}
```

**四个方法。** 关键的一步在 `Decide`。

### 深度理由：为什么批准/退回/批量批准是同一个方法

它们的差异只在 `Verdict` 和列表长度上。做成三个方法（`Approve` / `Reject` / `BatchApprove`）会让三处各自实现：置信度闸、权限检查、逐条幂等、差异留痕、部分失败装配、审计写入。做成一个 `Decide([]Decision)` 之后单条批准就是长度为 1 的列表，上述逻辑只有一份。

删掉本模块，这些复杂度会散到 handler 层，且必然分叉。

### 置信度闸的位置（D-B10 的落点）

低置信度未逐个确认则拒绝批准，**判定在 `Decide` 内部**，不在 handler、不在前端。

`Revise` 是唯一能把字段标记为「已人工确认」的入口，且它同时写差异痕迹（AI 提取值与最终值分列，互不覆盖）。**没有第二条路径能把 `confirmed` 置真** —— 这是 D-B10 从「规定」变成「结构上不可绕过」的地方。

### 部分失败（D-B8 的落点）

`Outcome` 逐条带结果，沿用 `draftapp.BatchResult` 的既有形状。第 N 条失败不回滚前 N-1 条。续跑复用既有 `Resume*Batch`。

### 为什么不建新端口

写路径经既有的 `draftapp.DraftStore` 接缝（已存在，已有 fake），读路径经既有 repository。两者都只有一个适配器——按「一个适配器 = 假想接缝」，不新建端口。

### 测试

- 单元：`DraftStore` 用既有 fake，测置信度闸、部分失败装配、差异留痕
- 集成（需 `TEST_DATABASE_URL`）：跨法人隔离、幂等。**只跑单元证明不了底线 1 和底线 4**
- 反向：低置信未确认时 `Decide` 返回拒绝；跨法人读取返回 `scope_denied` 且原文不被软化

### 决策留痕 D-B20

> **`Decide` 收列表而不是单个。** 单条 API 更「自然」，但会诱导 handler 层写 `for` 循环，从而把部分失败语义、幂等边界、审计批次的判断散到调用方。收列表让「批量是常态、单条是特例」成为接口事实。

---

## 5. BG4 Channel Identity — 渠道身份到租户（Ch3 的全部风险所在）

### 问题

这是本批次唯一可能导致跨法人数据泄露的模块。picoclaw 给不了任何东西：它是个人助手，`IsAllowed(senderID)` 是防打扰白名单。

### 接口

```go
// Resolve 是渠道进入本系统的唯一入口。
// 返回完整 Principal 或错误，没有第三种可能。
func Resolve(ctx context.Context, ref ChannelRef) (agenttools.Principal, error)

type ChannelRef struct {
    Channel        string  // "feishu" | "wecom"
    ExternalUserID string  // open_id / userid
}
```

**一个函数。** 入参两个字符串，出参一个既有类型。

### 深度理由：这个形状是为了让错误无法表达

接口**只返回完整的 `agenttools.Principal`**，不返回 `Scope`、不返回 `legal_entity_id`、不返回「用户存在与否」的中间结果。因此调用方**在类型上没有材料**去自己拼装权限——D-B13 从一条纪律变成了一个编译期事实。

实现内部：绑定表查询 → 内部用户查询 → **委托给与 JWT 完全相同的 Scope 解析器**。第三步是重点，它不是复制一份逻辑，是调用同一个函数。

删掉本模块，每个渠道适配器都要自己做这三步，且第三步一定会被写成「从 user 表读 legal_entity_id 拼一个 Scope」——那就是第二条权限判定路径，底线 1 当场失守。

### `IsAllowed` 的定位（D-B13 的落点）

vendor 进来的 `IsAllowed(senderID)` / `IsAllowedSender(sender)` 保留作为**第一道粗过滤（防打扰）**，但：

- 它返回 true **不构成任何数据可见性结论**
- 权限判定一律在 `Resolve` 之后由 `Principal.Scope` 完成
- 反向测试锁定：构造 `IsAllowed` 放行但 Scope 不覆盖的请求，断言数据被拒

### 机器可强制的守卫

架构测试断言 `internal/gateway/**`（含 vendor）**不出现 `access.Scope` 的构造**，也不出现 `legal_entity_id` 字面量。渠道层拿不到拼装材料，只能走 `Resolve`。

这条比代码评审可靠——按「只有用户一个人类 + 一堆 AI」的现实，评审拦不住。

### 未绑定即拒绝（D-B14 的落点）

`Resolve` 查不到绑定时返回具名错误。**没有默认租户参数、没有 fallback 入参**——同样是让错误表达不出来。

### 决策留痕 D-B21

> **`Resolve` 返回 `Principal` 而不是 `(userID, scope)`。** 后者读起来更直白，但把两个可分离的值交给调用方，就等于允许调用方只用其中一个、或者混用来自不同来源的两个。返回一个不可拆的聚合是这里唯一安全的形状。

---

## 6. BG5 Channel — vendor 渠道接缝

### 接口

沿用 vendor 自带的 `Channel` 接口（`Name` / `Start` / `Stop` / `Send` / `IsRunning` / `IsAllowed` / `IsAllowedSender` / `ReasoningChannelID`）。

**这个接口按本文标准偏宽（8 个方法），但不改它。** 理由：它是上游的形状，改了就对不上上游更新，而对上游更新的能力正是 vendor 而非 clean-room 的全部收益。两个适配器（feishu / wecom）使它是真接缝。

`bus.OutboundMessage` / `bus.SenderInfo` 改写成本仓类型，**不为两个类型拖进整个 `pkg/bus`**。

### vendor 隔离守卫

架构测试断言 `internal/gateway/vendor/picoclaw/**` 不 import `internal/repository`、`internal/services`、`internal/agenttools`、`internal/access`。依赖方向只能是外层依赖 vendor。

**vendor 目录内不写业务逻辑。** 需要改上游行为时用包装，不就地改。

### 决策留痕 D-B22

> **接受一个偏宽的 vendor 接口，换取可对上游的能力。** 这是本文唯一一次为了外部约束放弃深度。代价被 vendor 隔离守卫限制住：这个宽接口只在 `internal/gateway` 内部可见，不外泄给业务层——业务层看到的是 BG6 的两个方法。

---

## 7. BG6 Gateway Runner — 生命周期

### 接口

```go
type Gateway interface {
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
}
```

**两个方法。** `main.go` 只知道这两个。

### 深度理由

实现里有：配置读取与开关判定、凭据校验、渠道注册、长连接建立与重连、消息泵、`Resolve` 身份解析、agent 分发、回复渲染、审计写入、优雅关闭。全部藏在两个方法后面。

这是本轮最深的模块——大量实现，最小接口。

### 默认关（D-B15 的落点）

- 配置开关默认 off
- 凭据缺失时该渠道不启动，记录明确原因，**不 panic、不重试刷屏**
- 测试：默认配置下 `Start` 后断言零渠道连接

### 审计（Story 40）

经 IM 发起的 Tool 调用走**与 Web 完全相同**的审计路径，额外带渠道来源字段。不新建审计通道。

---

## 8. 一个需要回写 Spec 的发现

**Spec 的 Story 31（「在飞书卡片上直接看到瀑布图」）在本轮做不到，需要降级。**

飞书交互卡片与企微模板卡片都**不渲染 SVG**。要在卡片里出图，只有三条路：卡片内嵌图片（需先把 SVG 栅格化成 PNG 并上传换 `image_key`）、外链到 Web、或纯文字化。

Spec 的 Out of Scope 明确写了「不做 PNG / 位图导出」，与 Story 31 直接冲突。

**建议解法**：IM 卡片承载**关键数字 + 降级说明 + 回 Web 的带参深度链接**（如 `/store-360?store_id=SH-001`），图留在 Web 端看。理由：栅格化会引入图形库依赖（与「前端新增依赖为零」同一类克制），而店长在手机上真正需要的是「哪家店出了什么问题」和「点进去看详情」，不是在 3 寸屏上读瀑布图。

Story 31 应改写为：

> 31. 作为店长，我想在飞书卡片上看到关键数字与一个能直接打开对应门店详情页的链接，以便快速判断要不要深入看

这条要你确认后再改 Spec。

---

## 9. 交付顺序与依赖

| 顺序 | 模块 | 阻塞项 |
|---|---|---|
| 1 | BG1 + BG2 | 无。两者可并行，合起来即 Ch1 |
| 2 | BG3 | 无。Ch2 后端；前端页面随后 |
| 3 | BG4 + BG5 + BG6 | **先出 ADR**（第三方代码引入，D-B12）。BG4 可先于 vendor 动作独立开发与测试 |

BG4 不依赖 vendor 落地——它的入参只是两个字符串。**建议先写 BG4 并把守卫测试落地，再搬 vendor 代码**，这样 vendor 进来的那一刻越权路径已经被守卫堵死。
