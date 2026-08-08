# Spec：租赁成本差异归因与行动治理

## 1. 目标

把“预算 vs 实际”的静态桥接升级为可行动的差异治理：既能回答差异来自哪里，也能回答谁负责、何时处理、是否已经解释。差异金额必须仍然与总差异勾稽，无法自动判断的部分进入 `other`/残差，不得被系统伪造归因。

## 2. 原因字典

自动归因按以下优先级输出，每个合同在一个期间只进入一个主原因：

1. `new_lease`：预算没有、实际出现的新合同。
2. `ended`：预算有、实际没有的合同，含闭店/终止导致的结束。
3. `renewal_or_termination`：续租判断变化、续租或提前终止事件。
4. `rent_change`：合同对价、面积或固定租金变化。
5. `index_adjustment`：指数或 CPI 调整。
6. `discount_rate`：折现率或重估利率变化。
7. `payment_timing`：付款日期、先付/后付或付款时点差异。
8. `data_correction`：付款计划/主数据被修正，且事件不足以解释变化。
9. `exchange_rate`：外币重估差异。
10. `other`：没有足够证据的残差。

系统可以显示建议原因，但不得把“原因猜测”当成事实；用户的人工说明作为独立字段留痕。

## 3. 用户故事

- 作为 Finance Analyst，我想按影响金额看到差异最大的合同，并知道自动原因和证据。
- 作为 Finance Reviewer，我想补充解释、分派责任人、设置截止日期和状态。
- 作为 FP&A，我想看到 Budget/Forecast/Actual 的管理层摘要，并且摘要能下钻到待跟进项。

## 4. 数据与接口

新增差异行动记录，唯一键为 `version_id + period + contract_id`：

- `explanation`：人工解释。
- `owner_name`：责任人/团队，允许先记录业务可读名称。
- `due_date`：截止日期。
- `status`：`open`、`in_progress`、`resolved`、`accepted`。
- `updated_by`、`updated_at`：审计字段。

接口：

- `GET /api/v1/budget-versions/:id/variance?period=YYYY-MM`：返回扩展后的原因字典、自动原因、行动状态、解释覆盖率和待跟进金额。
- `PUT /api/v1/budget-versions/:id/variance-actions`：批量 upsert 合同差异行动记录。

请求示例：

```json
{
  "period": "2026-06",
  "items": [
    {
      "contract_id": "...",
      "explanation": "门店提前闭店，实际付款已停止",
      "owner_name": "华东租赁团队",
      "due_date": "2026-07-10",
      "status": "in_progress"
    }
  ]
}
```

## 5. 验收标准

- 自动桥接至少区分新签、终止/闭店、续租、租金、指数、折现率、付款时点、汇率和残差。
- 桥接金额始终勾稽到 Actual - Plan，无法解释部分明确显示为残差。
- 可对任一合同保存人工解释、责任人、截止日期和跟进状态；刷新页面后仍保留。
- 返回 `explanation_coverage`（有解释合同数/有差异合同数）和 `open_action_amount`，支持管理层摘要。
- 前端默认按绝对影响金额排序，并突出逾期和未解释差异。

## 6. 非目标

- 本轮不建立通用任务中心，不替代月结异常工作台。
- 本轮不允许用户直接修改自动归因金额；用户只能补充解释或状态。
- 本轮不对跨币种金额做未经政策确认的合并换算。
