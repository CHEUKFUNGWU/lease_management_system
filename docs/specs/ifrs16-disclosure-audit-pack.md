# IFRS 16 Disclosure and Audit Pack

> 规格状态：Ready for implementation
> 对应任务：P0.3 月结和审计产品化
> 依赖：现有 Working / Official Report Snapshot 与 disclosure projection

## Problem Statement

系统已有负债到期分析、负债滚动、ROU 调节、费用分解和现金流出计算，但它们仍更像查询页面。Finance Analyst、审计人员和月结负责人还需要手工拼接：

- 本次报告到底覆盖了多少合同、多少合同因数据不足被跳过。
- 每个汇总数字如何追溯到合同、付款计划、折现率、事件调整和计算批次。
- Official 报告使用了哪一个快照、规则版本、生成时间和审批状态。
- 合同层的期初/期末负债、ROU、利息、折旧、付款和折现调节是否能直接抽样。

这会导致审计取数依赖人工导出和二次 Excel 加工，也使 Finance BP 难以区分正式数据与 Working 试算。

## Solution

将现有 disclosure projection 产品化为“披露与审计底稿包”单一读取结果：

1. 保留五个 IFRS 16 披露主题：maturity、liability roll-forward、ROU reconciliation、expense breakdown、cash outflow。
2. 增加 `report_basis`，明确 snapshot、policy version、mode、is_official、approval population、computed/skipped count、period 和 as-of。
3. 增加合同级 `audit_workpaper`，每行包含源合同、付款计划/事件数量、折现率来源、初始计量、期间变动、期末余额和调节检查值。
4. Official 模式只读取 approved contract、approved payment schedule 和 approved event adjustment；Working 模式允许现有工作数据，但必须显式标识。
5. 前端在披露页展示报告基准和合同级底稿，并把底稿和五个披露主题一起导出为一个 XLSX 工作簿。

V1 只读，不创建会计分录，不改变 Official 数据，不把导出文件当作审批或审计签字。

## User Stories

1. As a Finance Analyst, I want one disclosure package for a selected period and mode, so that I do not reconcile multiple inconsistent exports.
2. As a Finance Analyst, I want report basis metadata visible, so that I know which snapshot and policy version produced the numbers.
3. As an Auditor, I want to drill from a disclosure total to contract-level workpaper rows, so that sampling and tie-out are practical.
4. As an Auditor, I want the discount rate value, source and version visible per contract, so that rate evidence is not hidden in a separate screen.
5. As an Auditor, I want approved payment and event counts visible, so that I can assess whether the calculation inputs were controlled.
6. As a Finance Manager, I want skipped contracts and the skip reason count visible, so that an apparently complete report cannot hide data incompleteness.
7. As an Approver, I want Official and Working labels in both UI and export, so that draft information is not used for formal reporting.
8. As a Finance BP, I want liability, ROU, P&L and cash metrics in the same package, so that I can explain the lease cost story.
9. As an Auditor, I want the workpaper to show whether the liability/ROU movement ties, so that testing starts from an explicit control check.
10. As a user, I want multi-currency caveats preserved, so that totals are not interpreted as a falsely aggregated functional-currency amount.

## Implementation Decisions

### Package contract

The existing `GET /api/v1/reports/disclosure` remains the primary endpoint. Its response gains:

- `report_basis`: `snapshot_id`, `policy_version`, `mode`, `is_official`, `approval_status`, `generated_at`, `period_start`, `period_end`, `as_of`, `population_count`, `computed_contract_count`, `skipped_contract_count`, `approval_status_policy`;
- `audit_workpaper.rows[]` with contract-level trace and tie-out fields;
- `audit_workpaper.totals` for count and amount checks.

`GET /api/v1/reports/close-pack?period=YYYY-MM&mode=working|official` packages the selected month's disclosure payload with the current close exceptions and monthly close batches for management/audit hand-off. It remains read-only and does not replace the source workflows.

The existing five disclosure sections remain backwards compatible.

### Workpaper row

Each row includes:

- contract ID/number/name, legal entity, store, asset type, currency and lease scope;
- approval status, report mode, commencement/end dates;
- discount rate, rate type, rate version, rate source and confirmation timestamp;
- approved payment schedule count and approved event adjustment count;
- initial liability and initial ROU;
- opening/closing liability, interest, payments, remeasurement and other adjustments;
- opening/closing ROU, additions, depreciation, impairment, remeasurement and other adjustments;
- `liability_tie_out` and `rou_tie_out` numeric residuals.

The row is computed from the same `disclosureFact` used by all five sections. The frontend must not recalculate accounting amounts.

### Official control

`report_basis.approval_status_policy` is `approved_only` for Official and `working_statuses` for Working. The package keeps `is_official` and `mode` in the JSON response and in every exported filename/sheet label. A multi-currency package shows a warning and does not silently convert totals.

### Export

The existing client-side workbook export adds a sixth sheet, `Audit Workpaper`, and a first `Report Basis` sheet. All exported rows include the mode marker and the snapshot ID. A future server-side immutable artifact can reuse the same payload; V1 does not persist the file.

## Testing Decisions

- disclosure package contains report basis metadata and correct Official/Working policy;
- workpaper row amount fields tie to the existing disclosure calculation for a known lease;
- skipped contracts are counted and do not appear as silently zero-valued rows;
- discount rate source/version and approved input counts are carried through;
- multi-currency warning and `is_official` survive the HTTP response;
- frontend export includes report basis and audit workpaper sheets;
- existing five disclosure reconciliation tests remain green.

## Out of Scope

- PDF rendering or digital audit sign-off;
- storing an immutable exported artifact in MinIO;
- third-party auditor workflow;
- changing IFRS 16 measurement formulas;
- automatic currency conversion without an explicit rate and policy;
- replacing the existing report snapshot model.

## Further Notes

This feature closes the gap between “the system can calculate a disclosure” and “Finance can hand an evidence-backed package to audit”. It deliberately reuses the existing projection and snapshot seams so that one accounting calculation feeds the UI, export and workpaper rather than creating a second reporting formula.
