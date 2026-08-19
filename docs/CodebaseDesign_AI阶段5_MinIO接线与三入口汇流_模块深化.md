# CodebaseDesign：AI 阶段 5（MinIO 接线 + 三入口统一 + G1 两平面汇流）模块深化

> 文档状态：Draft for Review
> 编制日期：2026-08-19
> 上游依据：底稿方案 §10（两平面汇流）、重点蓝图 ③（主页聊天框 / AI Chat 页 / RetailAIDrawer 统一运行时、单会话体验）、阶段 4 决策 D-D2（FileReader 缝 → 本阶段接线）、OPS-5 / ACORE-4 验收。

---

## 1. 范围与边界

| # | 交付物 | 说明 |
|---|---|---|
| M1 | **MinIO 读取接线** | `internal/miniostore` 适配器 + core config（MINIO_ENDPOINT/KEY/SECRET/BUCKET）+ main.go 注入 → `retail.store_days.import.preview` 获得生产文件读取 → **page_fill 全链路点亮**（阶段 4 D-D2 收口） |
| M2 | **三入口统一（前端）** | BriefColumn / RetailAIDrawer 迁到 `useAIChatRuntime`（SSE + 会话 + localStorage），消息类型归一为 runtime `Message`；主页保留系统早报入口语义（initiator=system） |
| M3 | **G1 两平面汇流（后端）** | 多步意图（底稿/迁移类）从静态 runbook 卡片改为真实 Run：planner 计划 → 工具执行 → 事件流回填卡片状态；单会话时间线合并渲染（§10.2） |

### 1.2 本阶段刻意不做

- W4 internal/llm（LLM 分类器与 Go planner 仍待 W4；本阶段汇流沿用既有 `agent_plan.py` planner 通道——契约不变）
- W3 订阅者持久化、W6 agentrunner 收敛（G1 汇流先走既有 Gateway API，不换内核）
- 上下文压缩（compact）、沙箱

---

## 2. 关键设计决策

| # | 决策 | 理由 | 日期 |
|---|---|---|---|
| D-E1 | MinIO 客户端 = `minio-go/v7`，仅 `GetObject` 读用途进场（写入仍归 ai-service 直至 W5 退役） | core 只消费上传文件做导入预览；写路径保持单一来源，未提前搬 upload | 2026-08-19 |
| D-E2 | core config 默认值 = compose 默认（minio:9000 / minioadmin / lease-uploads）；endpoint 空 = 禁用 | 本地开箱即用；无凭据环境优雅降级为诚实拒绝（沿用 D-D2 语义） | 2026-08-19 |
| D-E3 | 三入口汇聚顺序：先 Drawer（已有 session_id 续会话能力，改动最小），再 BriefColumn（早报逻辑保留、聊天帧替换），AI-Chat 页不动 | 由小到大，每个入口独立上线可回退；OPS-5 逐入口验证 | 2026-08-19 |
| D-E4 | G1 汇流 = 在既有 Gateway Run API 上加「chat 会话 → run」桥，事件转写进 chat 会话时间线；静态卡片渲染改为执行态回填 | 不重做 Runtime；ACORE-4 平价门（20 会话新旧对比）后置到 W6 全量比对 | 2026-08-19 |

---

## 3. 模块设计

### M1 · `internal/miniostore`

```go
type Config struct {
    Endpoint  string  // "minio:9000"
    AccessKey string
    SecretKey string
    Bucket    string  // "lease-uploads"
    Secure    bool
}
type Client struct{ mc *minio.Client; bucket string }
func New(cfg Config) (*Client, error)          // endpoint 空 → nil, nil（禁用）
func (c *Client) GetObject(ctx context.Context, objectName string) ([]byte, error)
```

main.go：config 有 endpoint → 构造 `&IngestFileReader`（小包装让 `IngestFileReader` 接口落在此）→ 注入 AIChatHandler 构造链 → aiagent 注册预览工具。禁用时工具继续诚实拒绝。

### M2 · 三入口统一

- `RetailAIDrawer`：把内部 `DrawerMessage` + 一次性 JSON 替换为 `useAIChatRuntime`（保留 sd page 的 filters → `page_context` 透传），session 键由 `retail-ai-drawer:${page}` sessionStorage 迁 runtime 会话列表。
- `BriefColumn`：早报逻辑保留（系统 initiator 会话 + runBrief），后续对话帧改 SSE 运行时；`home-chat-*` 气泡样式迁移为共享组件（ToolChip/ConfidenceBadge 已在复用）。
- 验收：OPS-5 三入口逐条（一条会话、steer/follow-up 消费统一）。

### M3 · G1 汇流

`buildAgentRunbook` 的多步 plan（含原 23 张 pending 卡片）→ 触发 Gateway Run（skill 映射既定）→ run 事件（tool_started/tool_completed/run_finished）转写为 chat 事件流并**回填卡片状态**；前端 timeline 已在消费 tool_start/tool_end（阶段 0），新增 run_* → 卡片状态映射即可。

---

## 4. 任务-验收映射

| 序 | 任务 | 验收锚点 |
|---|---|---|
| 1 | miniostore + config + 注入 + 测试 | page_fill 工具经真实 client 读取；禁用路径保持诚实拒绝 |
| 2 | Drawer 迁 runtime | 零售三页 Drawer 一条会话；既有 ai-002 契约测试不破 |
| 3 | BriefColumn 迁 runtime | 早报 + 对话同会话；chatLayout.test 不破 |
| 4 | G1 汇流桥 + 卡片回填 | 多步请求产生真实 run；卡片状态=执行记录；OPS-5 |
| 5 | 全量验证 | core-service test/vet、web type-check/build/test、评测绿 |

## 5. 落地顺序

**1 → 2 → 3 → 4 → 5**（MinIO 先行：id 立即点亮 page_fill）
