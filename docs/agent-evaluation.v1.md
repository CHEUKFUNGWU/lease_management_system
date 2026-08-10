# Agent Evaluation Dataset v1

这是 Skill/Tool 运行时的最小确定性评估集，不替代真实 LLM 评估。它固定服务端不可回归的控制行为，并作为模型、Prompt、Skill 或 Tool 白名单变更前的阻断门：

- 意图路由：合同台账、合同复核、租金表、审计包。
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
