# CodebaseDesign：AI 阶段 4（page_fill 填表缝）模块深化

> 文档状态：Draft for Review
> 编制日期：2026-08-19
> 上游依据：Agent Core 设计附录 A（page_fill 协议、确认点纪律、provenance 同构）、AI 文档索引（主链路 ① 的「填功能区 → 人确认入库」段）、AGENTS.md（AI 识结果不进正式台账、导入白名单纪律）。
> 目标：Agent 把经营数据文件预填进零售数据导入功能区，**commit 永远是人**。填表 = 预填 + 人确认。

---

## 1. 范围与边界

### 1.1 本阶段做什么

| # | 产出 |
|---|---|
| M1 | `internal/pagefill`：`Fill` 协议（payload 仅装非 Exploratory 值 + 建议区 + 确认动作）+ I5/ACORE-12 断言 |
| M2 | `agentartifact` 增加 `page_fill` 类型 |
| M3 | `retail.store_days.import.preview` 工具（Agent 白名单，LevelDraft）：`FileReader` 缝 + retailingest 预览 → page_fill 载荷 |
| M4 | aiagent：triage=operating_data 时改调 preview 工具 → `page_fill` artifact（读不到文件时保留诚实拒绝）；`retail_ingest_fill` 技能注册 |
| M5 | `GET /ai/chat/artifacts/:id`（page_fill 消费端）+ 导入页消费 `?fill=artifact_id` 预填（人 commit 不动） |
| M6 | ACORE-10/12 测试：commit 不进 Agent 白名单（既有）；Exploratory 值不得进入 payload/commit |

### 1.2 本阶段刻意不做

- **MinIO 文件读取接线**：core-service 无 MinIO 凭据与客户端（W5 的 minio-go 落地项）。`FileReader` 缝本阶段以接口 + 测试假实现落地，生产适配器随 W5——**Agent 端到端预填在文件读取接通前不生效**，triage 升级后读不到文件时保持诚实拒绝，不回退欺骗。
- 不改 commit 端点本体；commit 的白名单纪律沿用 CLI/HTTP 人路径。

---

## 2. 模块设计

### M1 · `internal/pagefill`

**Interface**

```go
type FillValue struct {
    Value      any                       `json:"value"`
    Provenance workingpaper.Provenance   `json:"provenance"`   // 与底稿单元格同构（A.3）
}
type Fill struct {
    SchemaVersion  string                 `json:"schema_version"`   // page-fill.v1
    TargetPage     string                 `json:"target_page"`      // "retail-data-import"
    TargetAPI      string                 `json:"target_api"`       // 既有 preview/commit 端点
    DeepLink       string                 `json:"deep_link"`
    Payload        map[string]FillValue   `json:"payload"`          // 仅 basis != Exploratory
    Suggestions    map[string]FillValue   `json:"suggestions"`      // 待人工确认（多是 Exploratory）
    Confidence     float64                `json:"confidence"`
    ReviewRequired bool                   `json:"review_required"`
    ReviewReasons  []string               `json:"review_reasons,omitempty"`
}
func (f *Fill) Confirm(field string, value any, confirmedBy, confirmedAt string) error // 建议 → payload，basis=HumanInput
func (f *Fill) ExploratoryRefs() []string                                             // I5 断言源
func (f *Fill) AssertNoExploratoryInPayload() error                                   // ACORE-12 断言
func (f Fill) Validate() error                                                        // 目标页/API 非空，payload 值必须带 provenance
```

**Construction invariant**: `Payload` 只在三处被写：构造时（调用方只给 HumanInput/SystemFact/Certified）、`Confirm`（显式写入 HumanInput）。**Exploratory 值唯一合法的去处是 Suggestions**——因此「Exploratory 进 commit」在类型流向上就不可能，ACORE-12 断言在 `Confirm` 与 `Validate` 双点锁死。

**Depth**：调用方（preview 工具、导入页）只学 Confirm/Suggestions/Payload；I5 的执行面（同底稿 I5 一条规则两条路径）藏在构造与确认动作里。

**删除测试**：删掉它，Exploratory 字段的出入库纪律要靠每个调用点自觉——正是底稿方案 A.3 说的「两套并存断链」与 I5 失效模式。

### M3 · 预览工具（Agent 侧）

输入 `{file_id, object_name, content_type, source_system, as_of}`；经 `IngestFileReader` 缝取字节 → `retailingest.ParseTemplate` → 规则映射建议 → `Fill{Payload: {source_system, as_of（HumanInput）}, Suggestions: {mapping 各列（规则建议，basis 见下）}}`。技能白名单 `retail_ingest_fill`。映射建议 provenance：规则回退 = SystemFact 不合适（它们不是库中事实）；采用 **Exploratory + 规则来源注明**（`EngineVersion="rule-mapping-v1"` 放注记）最诚实——任何未确认的映射都进 Suggestions。

### M4 · aiagent

triage `operating_data` 分支：优先 preview 工具 → `Response.PageFill` → ProjectResult 产出 `page_fill` artifact（ReviewRequired=true）；工具不可用（文件读取未接通）→ 保留既有的导入页指引回答，**不软化不伪造**。

### M5 · Web

`GET /ai/chat/artifacts/:id` 返回 artifact（含 data）；导入页挂载时读 `?fill=` 参数 → 取 artifact → 预填 source_system/as_of；mapping 建议以可确认 UI 呈现（**本阶段只预填已确认字段 + 展示建议**，确认交互复用现有导入页 mapping 确认流）。

---

## 3. 关键决策记录

| # | 决策 | 理由 | 日期 |
|---|---|---|---|
| D-D1 | 规则映射建议也标 Exploratory（不进 payload，只进 Suggestions） | 未人工确认的映射=不可直接入库，无论来自 AI 还是规则；诚实性统一 | 2026-08-19 |
| D-D2 | FileReader 缝本阶段无生产适配器（core 无 MinIO 凭据，W5 落地 minio-go）；读不到文件时 Agent 返回诚实拒绝 | 不临时伪造文件访问；断链显性可测 | 2026-08-19 |
| D-D3 | commit 端点不动；确认点纪律靠「payload 不含 Exploratory」构造不变量 + 页面只提交 payload | 附录 A：confirm 永远是人驱动的写入口 | 2026-08-19 |

## 4. 任务-验收映射

| 序 | 任务 | 验收锚点 |
|---|---|---|
| 1 | pagefill 包 + 测试 | I5 断言、Confirm 语义、Validate |
| 2 | agentartifact page_fill + 工具 + 测试 | 白名单 skill、Review Gate、Exploratory 不入 payload（ACORE-12 证据） |
| 3 | aiagent triage 升级 + ProjectResult + 测试 | operating_data → page_fill artifact；读不到文件 → 诚实拒绝 |
| 4 | GET artifact 端点 + 导入页 ?fill= 预填 + 测试 | 页面预填生效；commit 路径不变 |
| 5 | 全量验证 | core-service test/vet、web type-check/build/test、评测绿 |

## 5. 落地顺序

**1 → 2 → 3 → 4 → 5**
