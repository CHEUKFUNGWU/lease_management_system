# Agent Evaluation Dataset v1

这是 Skill/Tool 运行时的最小确定性评估集，不替代真实 LLM 评估。它固定服务端不可回归的控制行为，并作为模型、Prompt、Skill 或 Tool 白名单变更前的阻断门：

- 意图路由：**零售经营分析（主线）**、合同台账、合同复核、租金表、审计包。
- 角色限制：Editor 不能发现或显式选择 Auditor-only 审计包 Skill。
- Prompt Injection：文档/用户文本不能被当作 SQL、Shell 或任意工具权限。
- Review Gate：写入型 Skill 必须声明人工确认原因、阻塞原因和完成条件。
- Artifact：Skill 必须声明允许的 Artifact 类型。
- 数据质量闸门：缺失折现率、币种、付款时点或租赁范围不确定时必须进入人工复核。
- 高风险拒绝：审批、过账、锁账、解锁和 ERP 回写不能被默认 Skill/自然语言意图选中。

当前版本包含 14 个确定性案例，数据位于
`core-service/internal/agentskill/testdata/agent-evaluation.v1.json`，涵盖路由、角色、Prompt Injection、缺失字段、租赁范围和高风险动作拒绝。

运行：

```bash
cd core-service
GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache go run ./cmd/agent-evaluation \
  -out /private/tmp/agent-evaluation-report.v1.md \
  -json /private/tmp/agent-evaluation.v1.json
```

命令在任一案例失败时以退出码 `1` 结束；参数、注册表或报告写入错误以退出码 `2` 结束。Markdown 报告按案例 ID 排序，JSON 报告用于 CI 或发布归档。它只验证服务端确定性控制，不宣称已经验证真实 LLM 的抽取质量；真实模型评估还需记录模型、Prompt、Skill、Tool Schema 和规则版本，并单独抽样复核证据定位、变量/非租赁成分和 Working/Official 报表口径。

---

## 在三层评测体系中的位置（2026-08-18）

本评估集是 **L1**。完整体系见
[AI 底稿与 Paperwork Agent 设计方案](AI_底稿与Paperwork_Agent设计方案.md) §12.7：

| 层 | 测什么 | 数据集 | 频率 |
|---|---|---|---|
| **L1（本文件）** | 技能路由、角色限制、禁用工具、Review Gate、注入拒绝、**provenance 不变量 I1–I8** | `testdata/agent-evaluation.v1.json` | CI 每次提交 |
| L2 | 条款抽取、triage 分类、列映射准确率 | 人工标注金标准语料 | Nightly + 阶段门 |
| L3 | 端到端底稿质量 | 场景任务集 + 评分 rubric | 阶段门人工双评分 |

L1 保持"确定性、不测 prose 质量"的原有定位不变，只扩容 category：

- **`provenance`**：I1 完整性、I2 Certified 可追溯、I5 Exploratory 不入正式、I6 封面页一致性
- **`protected_measure`**：请求沙箱计算租赁负债 / ROU / 折现率 / 分录金额必须被拒（ADR-0025）
- **`middleware_chain`**：ACORE-2 变异测试——从治理链移除任一中间件，本层必须变红
- **`triage_refusal`**：域外文件误分类为 `lease_contract` 的数量必须为 0

最后一条对应 CORR-6，是当前 `return "contract"` 静默兜底最危险失效模式的守门测试。
