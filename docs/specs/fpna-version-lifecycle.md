# Spec：FP&A 版本生命周期与可追溯比较

## 1. 目标

把当前只有 Budget 快照的能力升级为可被 Finance Business Partner、FP&A 和 Finance Analyst 共同使用的版本体系：

- `Budget`：批准或固化的计划基线。
- `Forecast`：Latest Estimate，基于最新已知信息形成的滚动预测。
- `Scenario`：用于假设分析的版本，不得冒充正式计划或实际。
- `Actual`：来自已生成计量/关账结果的只读事实层，不创建可编辑的预算版本。

用户必须能知道每个数字“在什么时候、由谁、基于什么来源、覆盖什么范围”生成，并能比较两个计划版本或任一计划版本与 Actual。

## 2. 用户故事

- 作为 FP&A，我想把 FY 预算和最新预测并列比较，识别计划漂移，而不是覆盖原预算。
- 作为 Finance Analyst，我想看到版本创建人、创建时间、来源、覆盖范围和 Official 状态，避免把 Working Scenario 当成正式数字。
- 作为 Finance Business Partner，我想把版本差异下钻到合同和期间，并确认总额可复核。

## 3. 领域决策

1. `version_type` 只允许 `budget`、`forecast`、`scenario`；Actual 不落在版本表中。
2. `is_official=true` 只表达治理状态，不改变计算结果；当前创建入口只允许用户主动声明，后续可接审批流。
3. 版本一旦创建即冻结，不能原地编辑；修正必须创建新版本并保留来源说明。
4. 版本比较必须使用同一期间、同一币种/法人范围；系统只比较现有覆盖范围，不隐式补齐缺失合同。
5. 任何比较响应都返回 `left_basis`、`right_basis`、`period`、`ties_out` 和明细，便于导出和审计复演。

## 4. 接口

### 创建版本

`POST /api/v1/budget-versions`

```json
{
  "name": "FY2026 Latest Estimate 01",
  "version_type": "forecast",
  "source": "close_2026_06",
  "is_official": false,
  "coverage_scope": "LE001 / approved contracts",
  "from_period": "2026-01",
  "to_period": "2026-12"
}
```

### 列出版本

`GET /api/v1/budget-versions`

除现有字段外，返回 `version_type`、`source`、`coverage_scope`、`is_official`、创建人和创建时间。

### 比较版本

`GET /api/v1/budget-versions/compare?left_id=<uuid>&right_id=<uuid|actual>&period=YYYY-MM`

`right_id=actual` 表示读取 `measurement_results` 的 Actual。结果包含两侧总额、差异、合同明细、两侧版本元数据和 `ties_out`。

## 5. 验收标准

- 能创建 Budget、Forecast、Scenario 三种版本，并在列表中显示元数据。
- 现有 Budget vs Actual 不回归；Actual 仍然只来自实际计量结果。
- 至少支持 Forecast vs Budget 和 Forecast vs Actual 两种比较。
- 对比结果能下钻到合同，差异总额等于明细差异之和（允许 0.05 舍入误差）。
- 缺少期间或版本不存在时返回明确的 4xx 错误，不返回空成功结果。
- 前端能看到版本类型、Official 标识和比较双方口径。

## 6. 非目标

- 本轮不实现版本审批工作流和自动生成 Forecast。
- 本轮不允许直接编辑已冻结版本的行。
- 本轮不把预算行扩展成完整损益表；仍聚焦租赁成本、付款和负债。
