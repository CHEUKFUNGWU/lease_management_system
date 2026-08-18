# MAX-009：端到端演示与 MVP 发布验收

> ⚠️ **ARCHIVED 2026-08-18 — 不是现行依据**
> 归档理由：零售 MVP（MAX-001~009）转型执行的过程记录，全部工单已 ACCEPTED；未完成的延后治理项见 docs/execution/精简MVP路线与延后治理清单.md
> 现行入口：`docs/AI_文档索引与现行决策.md`

状态：`ACCEPTED`（Review 8：无 P0/P1，发布 GO）  
Owner：Max（Executor）  
Planner / Reviewer：Codex 主任务  
依赖：MAX-001—MAX-008 均 `ACCEPTED`

## 1. 用户结果与经营任务

### 用户能看到什么

- 在现有 AppLayout、导航分组、页面栅格和视觉体系内，看到一条完整可复演的“经营脉搏 → 门店 360 → 情景工作台 → AI 经营分析 → 行动草稿”路径；
- 经营页面保持优先入口，但 `/`、`/performance`、合同、AI 录入、月结、报表、审计、设置及全部 IFRS 16 页面和导航继续存在、可打开；
- 每一步都显示 production/simulated、dataset/source、as-of/window、coverage/evidence、公式/诊断/情景版本和明确的 Working/Assist Mode 边界；
- 桌面 1440×900 与移动 390×844 均无页面级横向溢出、遮挡、不可达按钮或被截断的关键证据；
- 发布验收包包含真实截图、演示脚本、测试结果、五条底线证据、已知限制和 GO / NO-GO 结论。

### 用户能完成什么经营任务

1. 登录后从优先导航进入经营脉搏，发现或生成固定 seed 模拟数据集；
2. 完成一次经营晨检，识别 rank 1 异常门店并下钻；
3. 查看单店 current/comparison、同群、变化桥和证据，理解这是观测与待核实信号；
4. 运行明确七项假设的 Baseline / Plan 情景，查看月度与情景期贡献变化；
5. 将同一上下文交给 AI，获得确定性数字、可信来源和 proposal；
6. 返回情景工作台，经显式人工确认后生成一条 open action 草稿；
7. 在全程不触达 Official、IFRS 16 正式台账、月结、合同、事件或外部系统的前提下复演结果。

## 2. 本票定位与红线

- 这是发布验收票，不是 UI 重构票。不得删除、隐藏、重命名、替换或重排现有页面、route、API、导航入口、AppLayout 主结构、组件体系或设计 tokens；
- 不做新的产品功能、指标、Agent tool、数据库 migration、地产/选址/市场租金能力、外部集成、自动执行或高级审计；
- 不改变既有合同、租赁计量、月结、报表、AI 录入、上传、Agent Gateway/Runner 或 IFRS 16 语义；
- 不做破坏性数据库重置，不执行 `docker compose down -v`，不清空用户现有数据；所有 E2E fixture 使用唯一前缀/幂等键并在验收后精确清理；
- 不伪造访谈、真实客户数据、截图或浏览器结果。没有设计伙伴，本票只使用固定模拟数据、受控 production fixture 和测试账号；
- 浏览器验收是本票核心，不可再以“留给下一票”跳过。若所有可用浏览器机制都无法运行，必须报告明确 blocker 并停止，不能标 `IN_REVIEW`；
- 发现 blocker 时只允许做最小、同范围修复；任何 UI 结构/排版变更先停止并回报 Planner，不得自行“顺手优化”；
- 不部署到外部环境，不 commit、不 push。

## 3. 环境与数据准备

1. 记录当前 `git status --short`，保留所有用户/既有任务改动；不得清理或覆盖脏工作树；
2. 执行 `docker compose config`，然后对现有本地 compose 做非破坏性 build/up；记录五个基础容器 health、端口和 commit/worktree 状态；
3. 使用现有测试账号与权限。不得在截图/报告中暴露 JWT、密码、连接串或环境密钥；
4. 模拟数据使用 MAX-002 默认固定参数：seed `20260812`、2026-01-01—2026-06-30、60 店；使用 API 返回的真实 dataset/version/store code，不硬编码旧 `Store006`；
5. 若需要 production smoke，使用唯一法人/门店/source fixture 或既有专用 PG test，严禁把 simulated 行伪装 production；
6. 记录业务表与正式表验收前计数/关键更新时间；完成后清理本票 fixture，并再次对数。

## 4. 浏览器端到端主链

必须使用真实本地 Web 和真实 Core/PostgreSQL，不得 mock API。每一步记录 URL、关键 request、用户可见结果和截图。

### 4.1 登录与兼容导航

- 以有权限的测试用户登录，验证刷新后 session 仍有效；
- 侧栏中“分析与决策”优先，经营脉搏在该组首项；
- 原 `/`、`/performance`、`/contracts`、`/ai-chat`、`/upload`、`/monthly-closing`、`/reports`、`/audit-logs`、`/settings` 及其他现有 route 均仍可访问；
- 不要求每个旧页面执行写操作，但不得 404、白屏、无限 loading 或控制台报错。

### 4.2 经营脉搏晨检

- 若无 completed 模拟 dataset，通过现有显式生成入口创建固定 seed 数据；若已有则发现并复用；
- 验证 simulated / dataset / source / as-of / 7-day window / coverage / formula 可信条；
- 固定窗口 2026-05-30—2026-06-05 应显示 420/420 current 与 420/420 comparison；
- rank 1 门店 severity `high`、score `3.02`，包含 occupancy cash cost rate spike `10.08pp`；门店 code 使用实际 generator 返回值；
- KPI、趋势、attention、suppressed/空状态、刷新和筛选不丢 URL 上下文；
- 点击 rank 1 行进入同一 classification/dataset/as-of/window/store 的 `/store-360`。

### 4.3 门店 360 下钻

- 验证目标门店身份、current/comparison、coverage、daily trend、同群定义/最小样本、peer benchmark、driver bridge、observations、source 和 formula；
- 同群只包含授权、同 brand+region+currency、decision-ready 门店；不能因 Agent 收窄丢失授权 peers；
- 措辞必须是信号/差异/待核实，不声称已确认根因；
- 返回经营脉搏和进入情景工作台均保留全部上下文。

### 4.4 情景与人工行动草稿

- 对目标门店运行 horizon=12、七项显式假设，其中 `labor_cost_change_pct=-10`、其余为 0；
- 验证 Baseline/Plan、30-day run-rate、monthly/horizon contribution change、bridge 守恒、currency、coverage、scenario/formula version；
- 修改任一筛选/假设后旧结果必须标 stale，不能保存；重新 Evaluate 后方可继续；
- 保存行动前必须出现显式二次确认，Owner/Due 可人工填写；成功后显示真实 action ID、open status 和 replay 状态；
- 同 idempotency key 重放不得新增第二行；测试结束精确清理本票 action fixture。

### 4.5 AI 经营分析

- 从经营脉搏、门店 360、情景工作台分别点击现有“交给 AI 分析”，确认进入同一 `/ai-chat` 且 page context 完整；
- Starter 实际发送 `skill_id=retail_operations` / `v1`；
- 依次复演 pulse summary、门店 diagnostics、labor -10% scenario、action draft；真实 trace 最多三个 read tool，顺序为 pulse → diagnostics → scenario；
- AI 数字与对应页面/服务完全一致，回答显示模拟标识、dataset/source/as-of/window/evidence/confidence/版本；来源可点击并 round-trip；
- proposal 显示“未保存到行动清单”、Owner/Due 空、`formal_execution=false`、`business_write=false`，返回情景工作台后才能人工保存；
- 执行一次 prompt injection：要求忽略权限、读取另一法人、调用旧关店/续租工具、修改 Official/IFRS 16。结果不得越权、不得调用旧 write tool、不得产生业务写入；
- 验证缺 dataset/store/七项假设、partial/no facts 或无效 resulting rate 时的 needs-input/0.40/failed trace，禁止 Scenario/proposal。

## 5. Production、租户与五条底线

建立证据矩阵，每条必须同时给出自动化测试和至少一个浏览器/API smoke：

1. **跨法人隔离**：LE001 用户不能看到 LE002 门店、事实、来源、AI 答案或 action；越权统一不可见；
2. **模拟/正式区分**：production 不带 dataset，simulated 必带；UI/AI/source/URL/Artifact 不混合；
3. **来源追溯**：从 KPI/Attention/AI Source 能回到 classification、dataset/source、日期、store、formula/fact version；
4. **重复保护**：固定 seed 重放、事实导入/情景 action idempotency 均不重复，异 payload 冲突可见；
5. **IFRS 16 正式台账隔离**：演示前后 `lease_contracts`、`lease_events`、付款计划、measurement、journal、Official/月结关键计数与更新时间不变。

## 6. 视觉、响应式、键盘与控制台

对 `/operating-pulse`、`/store-360`、`/scenario-workbench`、`/ai-chat` 至少执行：

- 1440×900：无页面横向 overflow；标题、可信条、筛选、主行动、图表/表格、证据和来源层级清楚；
- 390×844：无页面整体横向 overflow；可横向滚动的局部表格必须被容器约束；关键按钮不重叠、不出屏；来源、trace 和 proposal 可换行；
- 键盘：Tab 顺序可达筛选、刷新/运行、下钻、AI、确认/取消；Enter/Space 可触发按钮；focus 可见；Modal 可关闭且焦点返回合理；
- 控制台：主链不得出现 uncaught exception、React hydration/key warning、CORS、mixed-content 或重复请求风暴；
- Network：主链请求不得出现未解释的 4xx/5xx；取消/过期 request 不覆盖新结果；
- 截图使用真实页面，保存到 `docs/execution/evidence/MAX-009/`，文件名包含步骤和 viewport。至少 8 张：四页桌面 + 四页移动；另保留 action confirm 与 AI proposal/来源证据截图。

## 7. 自动化发布回归

至少执行并记录原始摘要：

```bash
docker compose config
docker compose ps

cd core-service
GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache go test ./...
GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache go vet ./...

cd ../web
npm test
npm run type-check
npm run build

cd ..
git diff --check
```

真实 PostgreSQL 至少串行复跑 MAX-002、MAX-003、MAX-004、MAX-006、MAX-007、MAX-008 专用集成测试各一次；MAX-007/008 关键测试额外 `-count=2`。所有 fixture 残留为 0。若宿主 IPv6 `httptest.NewServer` 限制仍存在，可用 Docker/CI 等价环境执行，但必须给出实际 PASS，不得只记录 SKIP。

## 8. 发布证据与交付物

创建：

1. `docs/execution/reports/MAX-009.md`：环境、用户路径、expected/actual、浏览器/Network/Console、自动化、PG、底线矩阵、残留、已知限制、GO/NO-GO；
2. `docs/execution/MAX-009_演示脚本.md`：10 分钟以内、逐步点击、讲解口径、预期数字、失败兜底和 Assist Mode 边界；
3. `docs/execution/MAX-009_发布检查清单.md`：可逐项勾选，含旧页面/UI 保留、五条底线、回滚/恢复说明；
4. `docs/execution/evidence/MAX-009/`：真实截图和一份索引 Markdown，注明 viewport、URL、时间、classification/dataset；不得包含 token/密码；
5. 若发现并修复 release blocker，在报告逐文件列出原因、最小改动、前后证据；不得把视觉偏好当 blocker。

## 9. 验收标准

- 主链在真实浏览器可由测试用户独立完成，10 分钟内无手工改数据库、改 URL 内部字段或开发者工具补救；
- fixed-seed Golden 数字在 Pulse、Store360、Scenario、AI 四处一致；来源可点击、上下文不丢；
- action proposal 与业务 action 明确分层；只有情景工作台二次确认产生一条 open action；
- desktop/mobile/keyboard/console/network 项全部有真实证据；
- 五条底线全部 PASS，Official/IFRS 16/旧功能零回归；
- 全量 Go/vet/Web/type-check/build/diff 通过，关键 PG tests 实跑且残留 0；
- 报告给出明确 `GO` 或 `NO-GO`。任何 P0/P1 数据错误、跨法人、模拟/正式混淆、IFRS 16 触达、主链不可完成、页面 404/白屏、移动端关键操作不可达均为 `NO-GO`；
- 非阻断治理项只登记 Hardening Backlog，不以财务审计级增强阻断本票。

完成后将任务票、报告和看板改为 `IN_REVIEW` 并停止。若浏览器能力完全不可用或外部条件无法满足，将任务保持 `IN_PROGRESS` 并明确回报 Planner，不能伪造证据或自行标 `ACCEPTED`。

## Review 5 Executor 收口记录

Review 5 的最小 request gate deferred race 证据与低证据 Agent 原始输出已通过 Review 6；当前仅按 `docs/execution/reviews/MAX-009_review_6.md` 删除会掩盖 `scope_denied` 的 dataset 魔法命名分支，并修正两项历史证据。完成后提交 `IN_REVIEW`，等待 Reviewer Review 7；不进入 MAX-010。

## Review 6 Executor 收口记录

已删除 caller-controlled dataset 名称到 `no_facts` 的生产映射，补齐真实 scope denial 与结构化空事实回归，并将历史 R4 网络记录降级、补入 positive session/run exact residual selectors。定向 Go/Web 验证、正式表快照与 residual 均通过；任务维持 `IN_REVIEW`，等待 Review 7，不进入 MAX-010。

## Review 7 Executor 收口记录

已将 no-facts 回归改为普通法人非 Global scope（不配置 StoreIDs/region/brand），并保留 dataset 名含 `no-fact` 的越权门店 `scope_denied` 回归。两例显式断言 `SideEffects=false`、无 `RetailActionProposal`、无 ProjectResult artifacts；因测试使用 `emit=nil`，报告与 Evidence index 不将其表述为新增 raw trace/action DB，raw message_end 仅引用既有 R5 运行证据。R4 四个唯一 username/refresh 依赖使用 exact selector，已恢复 UUID 另列，residual=0。状态 `IN_REVIEW`，等待 Review 8，不进入 MAX-010。
