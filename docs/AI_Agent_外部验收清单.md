# AI Agent / CLI 外部验收清单

> 目的：把实施计划中无法仅靠本地代码证明的项目，转换为可以在客户或生产环境逐项签字的验收任务。
>
> 适用范围：`AI_Agent_与_CLI_架构演进实施计划.md` 的 AG-035、生产监控、PaddleOCR、ERP 和会计复核收口。
>
> 原则：测试账号、访问 Token、合同原件、ERP 凭证和会计签字不得提交 Git；证据只保存脱敏摘要、版本号、时间和结果链接。

## 1. 验收总表

| 编号 | 验收项 | 责任人 | 需要的外部输入 | 通过证据 | 当前状态 |
|---|---|---|---|---|---|
| EXT-01 | 生产 Tool/Planner 监控基线 | Platform Ops | 至少一个完整结账周期的流量和 Tool 分布 | Prometheus recording rules、阈值批准记录、告警通知截图/事件号 | 待生产环境 |
| EXT-02 | 应用与数据库回滚演练 | Release + DBA | 上一验收镜像、数据库备份、变更窗口 | 演练记录、健康检查、只读 Smoke Run、回滚后回归结果 | 待批准窗口 |
| EXT-03 | PaddleOCR 真实样本验收 | AI/Data Owner | 脱敏 PDF、扫描件、复杂表格样本和 PaddleOCR Token | job/result ID、页数、字段定位覆盖率、人工复核结果 | 待真实样本 |
| EXT-04 | 客户 ERP 联调 | ERP Owner + Finance | 目标 ERP sandbox、字段映射、凭证回写接口 | 导出文件、sandbox 凭证号、重试/失败记录、对账结果 | 待客户环境 |
| EXT-05 | IFRS 16 会计复核 | Controller + 第三方会计师 | 回归报告、会计政策和客户样例 | 签字版报告、标准答案版本、例外处理记录 | 待第三方 |
| EXT-06 | Worker 生产身份 | Security + Platform Ops | 专用服务账号、权限审批、Token 轮换方案 | 权限清单、短期 Token 配置、轮换演练和审计记录 | 本地仅临时账号 |

## 2. EXT-01：生产监控基线

1. 部署 [`ops/prometheus/prometheus.yml.example`](../ops/prometheus/prometheus.yml.example)，将 `bearer_token_file` 指向 Secret 管理系统，不把 Token 写入 Compose、Git 或日志。
2. 加载 [`ops/prometheus/lease-agent.recording.yml`](../ops/prometheus/lease-agent.recording.yml) 与 [`ops/prometheus/lease-agent.rules.yml`](../ops/prometheus/lease-agent.rules.yml)。
3. 以一个完整结账周期记录以下基线：Tool 调用量、失败/拒绝率、Review Gate 比率、平均延迟、Planner 调用量、Token、成本可用率。
4. 由 Platform Ops 批准阈值。仓库默认值（失败率 20%、平均延迟 5 秒）只是技术初始值，不自动视为客户 SLA。
5. 使用一次受控测试触发每类告警，并保留通知系统事件号；确认恢复通知也能到达。

通过标准：Prometheus `check config`/`check rules` 通过；指标标签不含 user、Run、合同或法人 ID；告警能到达指定接收人；成本 unavailable 时不会被换算成猜测金额。

## 3. EXT-02：应用与数据库回滚演练

演练前必须创建数据库备份并获得变更窗口批准。禁止使用 `docker compose down -v`。

1. 记录当前 Core、AI、Web、Runner 镜像 digest、迁移编号和 IFRS16 回归报告版本。
2. 停止新 Worker 领取任务，保留已领取 Run 的 lease recovery 路径。
3. 将应用镜像回退到上一验收版本；数据库只使用向前兼容路径，若必须反向迁移，由 DBA 执行经过审批的 SQL。
4. 检查 `/health`、Tool discovery、只读 Contract Tool 和 Agent Trace；确认没有新增合同、付款计划、事件或分录。
5. 执行 lease recovery，再跑 Go 全量测试、Web 检查和 IFRS16 回归。
6. 记录演练开始/结束时间、负责人、异常、恢复耗时和是否触发告警。

通过标准：业务数据无非预期变化；Run Event、Artifact、审计和 refresh session 可读取；恢复后的只读 Smoke Run 完成；财务回归仍为 PASS。

## 4. EXT-03：PaddleOCR 真实样本

样本至少包含：可复制文本 PDF、扫描合同、带表格的扫描租金表、倾斜/低清图片和多页复杂版面。每个样本需脱敏并记录文件 SHA-256。

对每个样本记录：

- PaddleOCR model、API 版本、job/result ID、页数和耗时；
- Markdown 文本是否完整；
- provider-owned page/box locator 数量和字段覆盖率；
- 缺少坐标或无法匹配时是否正确进入 `evidence_complete=false` 和人工 Review Gate；
- 人工确认后的字段差异和原因。

通过标准：任何模型生成的坐标都不能绕过 PaddleOCR provider locator 校验；无法证明来源的字段不得进入正式台账；失败任务可以回退或人工转录。

## 5. EXT-04：ERP 联调

先在 sandbox 运行，不接生产总账。固定一组包含利息、折旧、付款、汇兑和事件调整的测试分录，验证：

- CSV/接口字段映射、币种和借贷方向；
- 幂等键、重复提交和超时重试；
- ERP reference / voucher number 回写；
- 失败凭证不会被标记为已过账；
- Working/Official 数据边界不会被跨模式导出。

通过标准：系统分录与 ERP sandbox 凭证逐行对账，失败和重试均有审计记录，Finance Owner 签署映射版本。

## 6. EXT-05：会计复核

第三方会计师需复核 [`docs/IFRS16_计量回归对数报告.md`](IFRS16_计量回归对数报告.md) 中的标准答案、政策假设和客户样例，至少覆盖：

- 先付/后付；
- 变量租金和非租赁成分；
- 短期/低价值/非租赁范围闸门；
- lease modification、reassessment、指数调租；
- 多币种 IAS 21 处理；
- 期末 rounding 和终止处置。

通过标准：签字版复核报告注明规则版本、样例版本、例外项和是否允许作为审计背书；未签字前只能标记 `pending_third_party_review`。

## 7. EXT-06：Worker 生产身份

生产 Worker 不得复用管理员 JWT。专用身份至少只拥有：

- `agent_runtime:worker`；
- 领取、心跳、事件、checkpoint、lease release/recovery 所需的最小 Run 数据面；
- 不拥有合同写入、审批、过账、锁账、ERP writeback 或数据库/MinIO 凭证。

验收时验证 Token 过期、轮换、撤销、跨 Run lease 拒绝和容器内无 DB/MinIO Secret。当前 Docker 本地验证使用的临时 Token 只能作为开发 smoke test，不得复制到生产。

## 8. 归档要求

每个外部任务完成后，在本表更新状态并附：

1. 执行环境和镜像/规则/价格版本；
2. 脱敏输入摘要和 SHA-256；
3. 命令输出或外部系统事件号；
4. 责任人、复核人、时间和结论；
5. 失败项、补救措施和重新验收日期。
