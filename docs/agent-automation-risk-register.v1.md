# Agent 有限自动化风险登记册 v1

本登记册把 Agent 从 Assist Mode 扩展到有限自动化前必须完成的业务决策记录下来。它不授权任何自动过账、审批或锁账行为；最终启用仍需业务负责人、会计政策确认、生产指标和回滚演练。

## 风险分级

| 等级 | 含义 | 默认策略 |
|---|---|---|
| L1 | 只读或不改变正式业务数据 | 可自动执行，结果必须带来源 |
| L2 | 产生可删除/可修改的 Draft 或解释 | 可自动生成，必须进入 Review Gate |
| L3 | 可能改变会计判断、正式状态或期间结果 | Assist Only，禁止自动提交/过账 |

## 候选场景

| 场景 | 风险 | 允许模式 | 必须的证据 | 回滚/控制 |
|---|---:|---|---|---|
| 只读审计包准备 | L1/L2 | Assist；可生成审计包 Artifact | 合同、计量、分录、审批和权限范围引用 | 删除/重建 Artifact；不写正式台账 |
| 数据质量扫描 | L1/L2 | Assist；可批量生成问题清单 | 字段缺失、付款覆盖、折现率和 scope 规则结果 | 关闭问题或重新扫描；不修改原始字段 |
| 月结异常解释 | L2 | Assist；可生成解释 Artifact | Working/Official basis、批次、分录和规则版本 | 丢弃解释；不得自动改分录 |
| 非正式报表摘要 | L1/L2 | Assist；只允许 Working Report | 报表版本、生成时间、筛选范围和来源 | 重新生成或撤销摘要 |
| 低风险合同/付款计划 Draft | L2 | Assist；逐项或批次 Review 后才能入库 | 字段级置信度、原文定位、幂等键和失败项 | Draft 删除/重试；不得直接 Approved |
| 合同审批 | L3 | Assist Only | 审批链、会计判断和人工决定 | 必须由 Reviewer/Approver 操作 |
| 月结分录过账 | L3 | Assist Only | 锁账、审批、期间和 ERP 对账状态 | 禁止 Agent 自动执行 |
| 期间锁账 | L3 | Assist Only | 全量覆盖、异常结论和授权人 | 禁止 Agent 自动执行 |
| ERP 凭证回写 | L3 | Assist Only | ERP 映射、凭证号、过账状态和对账结果 | 禁止 Agent 自动执行 |

## 上线前 Gate

- 每个候选场景必须指定业务负责人、会计政策版本和允许角色。
- L2 必须经过统一 Artifact、Evidence、Review Action、Draft Application Service 和幂等控制。
- L3 只能返回建议，Tool Policy 必须拒绝审批、过账、锁账和 ERP 回写的自动调用。
- 生产指标至少包括 Run 成功/失败/取消率、Review 通过率、低置信度率、重复写入率、拒绝率、延迟和人工返工率。
- 任何自动化扩展都必须先通过 Agent evaluation、IFRS 16 回归、越权测试、Prompt Injection 测试和客户 ERP 联调。

当前结论：L1 可优先试点；L2 维持 Assist + Human Review；L3 不进入默认 Auto-Post Mode。
