# PRD：租赁经营决策与 AI Copilot 平台

> 状态：Ready for Agent  
> 日期：2026-08-10  
> 来源：`FP&A与Finance_BP经营决策及AI辅助需求清单.md`  
> 产品边界：租赁、门店和租赁设备相关的经营分析与决策，不替代 ERP、POS、MES 或集团 EPM

## Problem Statement

集团 FP&A、零售 Finance BP、制造 Finance BP 和 Finance Analyst 已经能够在当前系统中完成租赁合同录入、IFRS 16 计量、事件重算、月结、报表、预算差异、租售比、现金流预测和部分续租/签约前分析，但这些能力仍分散在不同功能页面和技术接口中。

用户每天真正面对的不是单一合同或单张报表，而是一连串经营问题：本月为什么偏离 Forecast、哪些门店或设备需要优先处理、续租还是退出、房东报价是否合理、行动能挽回多少利润或现金、上期承诺是否已经兑现。当前系统尚未把“发现偏差—核实证据—解释驱动—模拟方案—形成行动—验证兑现”组织成统一工作流。

同时，AI Agent 当前主要服务于文件解析、合同草稿和合同级读取。它尚未完整连接现有预算差异、租售比、组合、现金流、Close Readiness、续租和签约前确定性服务，因此无法可靠地回答跨合同、跨门店、跨期间的经营问题。若直接让 LLM 生成金额、原因或正式动作，又会破坏系统现有的事件驱动、Working/Official、Review Gate、多租户范围、职责分离和审计控制。

制造业务还缺少工厂、产线、设备利用率、标准成本和产能事实；零售业务虽然已有营收及租售比雏形，但尚未形成四墙损益、店铺经营驱动和完整的续租/关店/搬店经济性。因此，用户仍需要在系统外导出数据、拼接 Excel、写解释、制作管理层材料并手工追踪行动。

## Solution

将现有租赁管理系统升级为“租赁经营决策与 AI Copilot 平台”，继续以可信租赁子账和 IFRS 16 控制为底座，在其上增加统一经营事实、异常与行动、驱动式计划、零售门店经济性、制造设备经济性以及受控 Agent 工具。

用户将获得一个以经营问题而非功能模块为中心的工作入口：

1. 经营驾驶舱按影响金额、风险和时效展示最值得关注的偏差、数据问题、关键日期和行动。
2. 管理层差异桥使用确定性服务解释 Actual、Budget、Forecast 和 Scenario 之间的变化，并保留未解释残差。
3. 零售工作台结合租赁事实、营收、毛利、人工、面积和其他门店费用，提供四墙损益及续租、关店、搬店决策。
4. 制造工作台将租赁合同与工厂、产线和设备关联，结合产量、OEE、成本和利用率提供成本桥及 Buy/Lease/Replace/Outsource 决策。
5. AI Copilot 通过 Core Service 的受控 Tool Runtime 查询事实、调用无副作用的确定性模拟、生成解释或行动草稿，并始终显示来源、范围和事实/推断边界。
6. 所有正式合同变化仍通过事件驱动；所有正式计划、解释、行动和决策仍经过人工复核或审批；AI 不拥有会计控制权。

交付按四个阶段推进：先连接现有分析能力和行动闭环，再完成零售 Finance BP 场景，然后完成制造 Finance BP 场景，最后在数据质量稳定后增加预测优化和主动监控。

## User Stories

1. As a Group FP&A user, I want to compare Actual, Budget, Latest Forecast, Prior Year and Scenario in one view, so that I can understand current performance without reconciling multiple exports.
2. As a Group FP&A user, I want to filter results by legal entity, business segment, brand, region, store, plant, asset type and currency, so that I can review the group at the right management grain.
3. As a Group FP&A user, I want every headline number to show its source, as-of time, version, coverage and reporting mode, so that I know whether it is suitable for management or official use.
4. As a Group FP&A user, I want consolidated totals to tie to drill-down details, so that I can defend the numbers in management review.
5. As a Group FP&A user, I want cross-currency totals to identify the exchange-rate version, so that currency translation is reproducible.
6. As a Group FP&A user, I want incomplete coverage to be shown as incomplete rather than zero or healthy, so that missing data cannot create a false conclusion.
7. As a Group FP&A user, I want a driver bridge between Actual and the selected comparison basis, so that I can explain performance using business causes.
8. As a Group FP&A user, I want unexplained differences to remain visible as residuals, so that the system and AI cannot fabricate attribution.
9. As a Group FP&A user, I want assumptions and materiality rules to be versioned, so that prior reports remain reproducible after policy changes.
10. As a Group FP&A user, I want to create monthly or quarterly rolling Forecast versions without overwriting previous versions, so that forecast evolution is auditable.
11. As a Group FP&A user, I want Actual periods to replace elapsed Forecast periods while future periods remain forecasted, so that the latest estimate remains current.
12. As a Group FP&A user, I want Forecast Accuracy and Bias by driver and organizational unit, so that I can improve future planning quality.
13. As a Group FP&A user, I want management assumptions to record scope, period, unit, source, owner and effective version, so that the model does not rely on undocumented spreadsheet inputs.
14. As a Group FP&A user, I want Baseline, Upside, Downside and custom scenarios, so that I can maintain multiple views of the future.
15. As a Group FP&A user, I want scenarios to produce P&L, balance sheet, cash flow, net debt and operational KPIs together, so that accounting and business effects are evaluated consistently.
16. As a Finance Analyst, I want close blockers, lease variances, rent-to-sales exceptions, critical dates and data-quality problems in one action center, so that I can prioritize work by impact instead of searching multiple pages.
17. As a Finance Analyst, I want exceptions ranked by amount, control risk, deadline, recurrence and fixability, so that scarce review time is spent on the most important items.
18. As a Finance Analyst, I want to batch assign, acknowledge and export exceptions, so that high-volume periods are manageable.
19. As a Finance Analyst, I want zero, missing and not-applicable values to be distinct, so that missing evidence is not mistaken for no impact.
20. As a Finance Analyst, I want unmapped master-data records to enter an exception queue, so that imports never silently discard records.
21. As a Finance Analyst, I want every exception to identify the detection rule, data version and source record, so that the issue can be reproduced.
22. As a Finance Analyst, I want AI extraction corrections and low-confidence backlogs included in data-quality metrics, so that intake risks are visible before close.
23. As a Finance Reviewer, I want deterministic attribution, human explanation and AI suggestion stored separately, so that the audit trail preserves who or what concluded each item.
24. As a Finance Reviewer, I want to assign an owner, due date, root cause, planned action and expected financial benefit, so that variance review produces accountable follow-up.
25. As a Finance Reviewer, I want a completed action verified against a later Actual period, so that users cannot claim benefits without evidence.
26. As a Finance Approver, I want Official Forecasts and approved decision conclusions frozen, so that changes require a new version rather than an in-place edit.
27. As an Auditor, I want every query, export, explanation confirmation and action transition logged with user, time, scope and version, so that the decision process can be replayed.
28. As an Auditor, I want Working, Official and Scenario outputs clearly labeled in the UI and exports, so that draft analysis cannot be mistaken for official evidence.
29. As a Retail Finance BP, I want a store-period fact containing revenue, gross profit, transactions, footfall, area, labor and controllable costs, so that lease economics can be evaluated against store performance.
30. As a Retail Finance BP, I want store data imported from controlled CSV/XLSX templates or APIs, so that I can begin before a full POS integration is available.
31. As a Retail Finance BP, I want store aliases, fiscal period, brand, region and currency mappings governed by effective date, so that operational and lease records join reliably.
32. As a Retail Finance BP, I want store data freshness, coverage and reconciliation status, so that I can judge whether a four-wall P&L is decision-ready.
33. As a Retail Finance BP, I want a four-wall P&L with revenue, gross profit, labor, fixed rent, variable rent, non-lease cost and other controllable cost, so that I can evaluate store contribution.
34. As a Retail Finance BP, I want Four-wall EBITDA, contribution margin, rent-to-sales, occupancy-cost ratio, sales per square meter and break-even sales, so that stores can be compared using consistent economics.
35. As a Retail Finance BP, I want Actual, Budget, Forecast and Prior Year comparisons for each store, so that I can distinguish structural underperformance from temporary variance.
36. As a Retail Finance BP, I want same-store, new-store ramp, closure and store-age cohort analysis, so that network performance is not distorted by portfolio changes.
37. As a Retail Finance BP, I want stores ranked against regional and brand benchmarks, so that I can identify outliers and negotiation priorities.
38. As a Retail Finance BP, I want renew, renegotiate, downsize, relocate and close scenarios, so that all realistic estate actions are compared.
39. As a Retail Finance BP, I want sales, margin, labor, relocation CAPEX, landlord contribution, cannibalization and closure downtime in store scenarios, so that decisions reflect operating economics rather than rent alone.
40. As a Retail Finance BP, I want NPV, payback, exit cost, break-even rent and a target negotiation range, so that I can communicate a financially grounded position to the property team.
41. As a Retail Finance BP, I want break options, notice periods and renewal windows surfaced automatically, so that decision opportunities are not missed.
42. As a Retail Finance BP, I want an approved store decision to create only a draft lease event, so that the existing event approval and remeasurement controls remain authoritative.
43. As a Retail Finance BP, I want promotion ROI to include turnover-rent effects, so that sales growth and variable rent are assessed together without capitalizing sales-based rent.
44. As a Manufacturing Finance BP, I want leases linked to plants, production lines, equipment, cost centers and asset identifiers, so that leased equipment can be evaluated in its operating context.
45. As a Manufacturing Finance BP, I want a contract to support multiple equipment items and separated lease/service components, so that complex equipment arrangements are represented accurately.
46. As a Manufacturing Finance BP, I want plant-line-equipment period facts for output, yield, scrap, downtime, OEE, labor, energy and utilization, so that equipment economics can be linked to operational performance.
47. As a Manufacturing Finance BP, I want standard cost, actual cost, purchase price, material usage and overhead absorption data, so that manufacturing variances can be explained.
48. As a Manufacturing Finance BP, I want a bridge for purchase price, material usage, labor efficiency, yield/scrap, energy and overhead absorption, so that plant leaders receive actionable cost explanations.
49. As a Manufacturing Finance BP, I want fixed lease-cost under-absorption highlighted when capacity falls, so that structural capacity risk is visible.
50. As a Manufacturing Finance BP, I want manufacturing and contractual rent drivers attributed separately, so that operational inefficiency is not confused with lease change.
51. As a Manufacturing Finance BP, I want Buy, Lease, Renew, Replace and Outsource scenarios, so that equipment decisions are evaluated on comparable terms.
52. As a Manufacturing Finance BP, I want purchase price, rent, maintenance, energy, residual value, tax, downtime, installation and exit cost included, so that total economic cost is complete.
53. As a Manufacturing Finance BP, I want capacity, yield, efficiency and delivery-risk effects included in equipment scenarios, so that the cheapest accounting option is not mistaken for the best operating option.
54. As a Manufacturing Finance BP, I want NPV, IRR, payback, unit-capacity cost, cash flow and IFRS 16 impact reported separately, so that business and accounting conclusions remain distinct.
55. As a Manufacturing Finance BP, I want idle, low-utilization and near-expiry leased equipment identified, so that renegotiation or exit candidates are reviewed before value is lost.
56. As a Management Reporting user, I want WBR, MBR and QBR packs for group, retail and manufacturing views, so that recurring reviews use one governed narrative.
57. As a Management Reporting user, I want each report to include results, drivers, risks, opportunities, decisions requested, actions and prior-period realization, so that reporting leads to action.
58. As a Management Reporting user, I want HTML, XLSX, PDF and presentation-friendly outputs, so that the same governed facts can serve different audiences.
59. As a decision owner, I want a standard memo for renewal, closure, relocation, equipment investment and contract negotiation, so that proposals are comparable and reviewable.
60. As a decision owner, I want the memo to separate system facts, deterministic calculations, human inputs and AI narrative, so that readers understand the evidence status.
61. As an AI Chat user, I want a daily brief scoped to my permissions and responsibilities, so that I start with the highest-impact issues.
62. As an AI Chat user, I want a brief to include major variances, cash risks, lease deadlines, data gaps and overdue actions, so that I can act without visiting every module.
63. As an AI Chat user, I want the AI to state when there is no material change, so that it does not create artificial urgency.
64. As an AI Chat user, I want to ask why a region or business unit is above Forecast and receive a deterministic bridge with cited sources, so that the answer is management-ready.
65. As an AI Chat user, I want the AI to return residual or insufficient evidence when it cannot explain an amount, so that uncertainty remains explicit.
66. As an AI Chat user, I want a generated explanation saved only after I confirm it, so that AI prose does not silently become an official conclusion.
67. As an AI Chat user, I want natural-language scenarios converted into structured assumptions for review, so that modeling is faster without hiding inputs.
68. As an AI Chat user, I want missing discount rate, exchange rate, sales-growth or capacity assumptions to trigger human confirmation, so that the model does not guess material inputs.
69. As an AI Chat user, I want scenario results calculated by deterministic services and saved as Scenario drafts, so that AI cannot overwrite Budget or Forecast.
70. As an AI Chat user, I want a one-page decision summary based on an executed scenario, so that I can communicate the result efficiently.
71. As an AI Chat user, I want the AI to list sensitive assumptions, data gaps, counterarguments and questions for the business, so that recommendations are critically reviewed.
72. As a meeting owner, I want AI-generated pre-reads and question lists based on governed data, so that WBR/MBR/S&OP meetings focus on decisions.
73. As a meeting owner, I want meeting notes converted into action drafts requiring confirmation, so that follow-up is faster without unauthorized task creation.
74. As a meeting owner, I want later Actuals matched to prior commitments, so that action benefits are validated.
75. As an Agent Runtime administrator, I want portfolio, rent-to-sales, budget variance, cash flow, Close Readiness, renewal and operating-performance read tools, so that Agent clients share one governed capability surface.
76. As an Agent Runtime administrator, I want deal, pre-deal, renewal, cash-flow, store and equipment simulations exposed as side-effect-free tools, so that models can orchestrate calculations without mutating business state.
77. As an Agent Runtime administrator, I want explanation, action, Scenario and memo writes exposed only as draft tools with idempotency and Review Gates, so that retries and model proposals remain controlled.
78. As a Security administrator, I want tool identity derived from authenticated execution context rather than model arguments, so that forged identity fields cannot bypass access control.
79. As a Security administrator, I want legal-entity, store, region, brand, plant, line and equipment scope enforced before every linked read, so that natural-language questions cannot disclose out-of-scope data.
80. As a Security administrator, I want uploaded documents treated only as evidence and not as executable instructions, so that prompt injection cannot grant SQL, HTTP, Shell or command access.
81. As a Control owner, I want Agent Signals physically and logically separated from Detection Events and Control Conclusions, so that only versioned deterministic rules create formal close findings.
82. As a Product owner, I want tool success, rejection, latency, evidence coverage, human adoption and unsupported-answer metrics, so that AI quality is managed as an operational product.
83. As a Product owner, I want the North Star Metric to be time from finance question to evidenced, explainable and actionable conclusion, so that automation is measured by decision value rather than chat volume.
84. As a System integrator, I want controlled CSV/XLSX templates before full API integration, so that customers can adopt the capability incrementally.
85. As a System integrator, I want source system, import batch, as-of time, version and reconciliation status recorded for every operating fact, so that integrations remain auditable.
86. As a System integrator, I want error-row isolation, retry, deduplication and idempotent imports, so that partial integration failure cannot corrupt official data.
87. As a Data owner, I want versioned metric definitions for rent-to-sales, occupancy-cost ratio, Four-wall EBITDA, OEE and utilization, so that the UI and AI return the same answer.
88. As a Data owner, I want the UI and Agent tools to use one metric service, so that calculations are not independently reimplemented.
89. As a cross-entity manager, I want aggregated access without unauthorized detail disclosure, so that group oversight does not weaken tenant isolation.
90. As an external auditor, I want any management number or AI conclusion reproducible from the same fact, assumption, metric and model versions, so that historical Official artifacts remain stable.

## Implementation Decisions

- The product remains lease-centered. Operating facts enrich lease and asset decisions but do not turn the application into a general-ledger, POS, MES or full-account EPM replacement.
- Existing contract, store/asset, payment schedule and event concepts remain the core domain model. Store-period and plant/line/equipment-period facts are adjacent versioned fact slices linked through governed master-data mappings.
- The Core Service remains the sole business authority. Web, CLI, AI Chat and independent Agent Runner clients do not receive direct database, MinIO-admin, arbitrary HTTP or Shell access.
- The primary application contract is the authenticated Core Service boundary. Normal UI workflows use HTTP APIs; Agent clients use the Agent Gateway Tool Runtime, which delegates to the same authorization, repository and deterministic service rules.
- Read tools are delivered before simulation and draft tools. The first read-tool set covers portfolio summary, rent-to-sales, budget variance, cash-flow forecast, Close Readiness, renewal decisions, store performance and equipment performance.
- Simulation tools are side-effect free and deterministic. They return result artifacts and immutable input metadata but do not create contracts, events, actions, Forecasts or journal entries.
- Draft tools may create explanation drafts, action drafts, Scenario drafts and decision-memo drafts only. They require idempotency keys, server-owned capability checks, a Review Gate and complete audit traces.
- Level 2 command behavior for approval, posting, period lock/unlock, ERP writeback and formal contract change remains disabled for default Agent use.
- Agent identity and scope are always taken from authenticated execution context. Tool arguments cannot supply or override user, role, legal-entity or organizational scope.
- Access scope expands from legal entity, store, region and brand to plant, production line and equipment. Linked reads first resolve the target under the caller's scope and return not-found semantics for absent or out-of-scope identifiers.
- Agent Signals remain physically separate from deterministic Detection Events, Control Conclusions and Close Exceptions. Only versioned control rules may author formal control state.
- Operating fact imports record source system, import batch, as-of time, fact version, reconciliation status and error disposition. Corrections create new versions; they do not rewrite lease contracts or historical Official artifacts.
- Master-data mappings cover legal entity, cost center, store, plant, line, equipment, contract and external-system identifiers, including aliases, effective dates and one-to-many relationships.
- A shared metric-definition service owns formula, grain, currency, fiscal-period rules, exclusions, owner and effective version. UI reports, exports and Agent tools consume this service rather than recalculate metrics independently.
- Budget, Forecast and Scenario versions remain immutable. Actual remains a read-only fact basis. Official status affects governance and presentation, not calculation logic.
- A unified assumption registry separates contract facts, system policy, management assumptions and AI suggestions. Material assumption changes require review and create new versions.
- Driver bridges must mathematically tie to the selected basis difference within configured tolerance. Unattributed values remain visible residuals; AI is not permitted to allocate them heuristically.
- Actions store baseline, target, expected benefit, owner, due date, status and verification period. Completion is distinct from verified realization against later Actuals.
- Retail four-wall P&L uses governed store-period facts and existing lease cash/expense facts. Sales-based variable rent remains period expense and is never included in lease liability present value.
- Store decisions support renew, renegotiate, downsize, relocate and close. Approved decisions produce a draft lease event and continue through the existing event review, approval and remeasurement workflow.
- Manufacturing equipment analysis separates operating conclusions from IFRS 16 accounting conclusions. Missing utilization or capacity evidence prevents deterministic disposal/termination recommendations.
- Management reports and decision memos identify Working, Official or Scenario basis and separately present system facts, deterministic calculations, human inputs and AI narrative.
- Long-running scenario and report generation uses the existing Run, checkpoint, SSE, cancellation, recovery and Artifact infrastructure. Failed generation cannot publish a complete-looking Official artifact.
- Common interactive dashboard queries target a three-second response under the agreed production data profile. Large scenario and export jobs are asynchronous.
- The phased delivery order is: existing read tools and action center; retail operating facts and four-wall decisions; manufacturing facts and equipment decisions; then predictive forecasting and proactive monitoring after data quality is proven.

## Testing Decisions

- The highest primary test seam is the authenticated Core Service application contract. Tests exercise observable HTTP responses and Agent Gateway tool results rather than handler internals, repository call order or UI component implementation.
- API contract tests cover reporting basis, source/version metadata, coverage status, tie-out, residuals, error semantics, idempotency, Review Gates and immutable version behavior.
- Agent Tool Runtime contract tests cover descriptor discovery, strict schemas, allowed tool levels, authenticated execution context, scope enforcement, stable structured errors, dry-run/simulation side-effect freedom and draft review behavior.
- Multi-tenant integration tests use PostgreSQL fixtures to prove legal-entity, store, region, brand, plant, line and equipment isolation. Tests must verify that forged IDs return the same public not-found semantics as absent records.
- Deterministic service tests cover driver-bridge tie-out, cross-currency policy, four-wall metrics, turnover-rent treatment, store decision economics, manufacturing variance attribution and equipment scenarios.
- Import contract tests cover valid batches, partial invalid rows, duplicate retry, alias mapping, effective dates, missing-versus-zero semantics and fact-version preservation.
- Version lifecycle tests cover Budget/Forecast/Scenario immutability, Actual read-only behavior, Official/Working separation, assumption versioning and historical artifact reproducibility.
- Action lifecycle tests cover assignment, due dates, status changes, expected benefit, later-period realization verification and the prohibition on treating a user-completed action as verified Actual benefit.
- Agent safety tests extend the existing deterministic evaluation dataset with operating-analysis routing, cross-scope attempts, prompt injection, evidence insufficiency, missing material assumptions and high-risk command refusal.
- AI answer evaluations assert source presence, fact/calculation/input/inference classification, explicit residuals, unsupported-answer behavior and no invented amounts. They do not grade prose style as a substitute for factual correctness.
- Critical Web workflow tests cover dashboard-to-evidence drill-down, action review, Scenario assumption confirmation and draft-to-review transitions. Component snapshot tests are avoided unless visual state is itself the contract.
- Export tests verify Working/Official/Scenario labeling, data version, scope, exchange-rate version, generation time and manifest integrity.
- Performance tests use an agreed representative group dataset and measure common dashboard queries separately from asynchronous Scenario and report jobs.
- Prior art includes existing IFRS 16 regression cases, budget variance tie-out tests, FP&A version lifecycle tests, close-readiness/exception governance tests, access-scope PostgreSQL tests, Agent Tool Runtime policy tests and the versioned Agent evaluation dataset.
- A good test proves externally visible finance behavior and control guarantees. It must remain valid if handlers, repositories, UI components or internal algorithms are refactored without changing the product contract.

## Out of Scope

- Building a complete group general ledger, consolidation engine, supply-chain execution system, POS, WMS, MES, maintenance platform or full-account enterprise planning system.
- Replacing the IFRS 16 deterministic calculation engine with an LLM.
- Allowing AI to choose or invent discount rates, exchange rates, sales growth, capacity, accounting policy or other material assumptions.
- Allowing AI to approve contracts, post entries, lock or unlock periods, perform ERP writeback, issue a formal accounting conclusion or directly modify Official Forecasts.
- Automatically closing, renewing, relocating or disposing of a store or equipment asset based only on an AI signal.
- Treating incomplete operating data as zero, healthy performance or evidence for a deterministic closure/termination recommendation.
- Performing unapproved cross-currency aggregation or silently filling missing organizational mappings.
- Replacing the existing event-driven contract-change architecture with direct field edits.
- Weakening Working/Official separation, segregation of duties, tenant scope, immutable versions, period locks or audit trails.
- Delivering predictive machine-learning forecasts before operating facts, master data, reconciliation and historical coverage satisfy agreed quality thresholds.
- Customer-specific live ERP, POS or MES implementation beyond establishing governed import/API contracts; each production integration requires a separately scoped mapping and rollout.
- Third-party accounting sign-off on IFRS 16 regression expected values.

## Further Notes

- The source requirements intentionally span P0 through P3. Delivery should be split into vertical slices that each produce an end-to-end user outcome rather than separate database, API and UI projects.
- The recommended first vertical slice is “evidenced variance investigation”: expose current budget variance, rent-to-sales, portfolio, cash-flow and Close Readiness reads through the Agent Runtime; present them in the action center; allow a reviewed explanation/action draft; preserve complete evidence and audit traces.
- The recommended second vertical slice is “retail store decision”: ingest governed store-period facts, produce four-wall economics, run renew/renegotiate/downsize/relocate/close scenarios, and create a reviewed draft lease event.
- The recommended third vertical slice is “manufacturing equipment decision”: map equipment to contracts, ingest operating facts, explain cost/utilization, compare Buy/Lease/Replace/Outsource, and produce a reviewed decision memo.
- External dependencies include third-party accounting review, customer policy confirmation, a real POS/BI store-data integration, a real manufacturing ERP/MES integration and customer-specific ERP posting mappings.
- The North Star Metric is median time from a finance question to an evidenced, explainable and actionable conclusion. Supporting metrics include close-cycle time, unresolved variance amount, Forecast Accuracy/Bias, decision-cycle time, action realization, Agent evidence coverage and unsupported-answer rate.
- Reference research includes AFP finance business partnering guidance, AICPA & CIMA business-partnering guidance, Amazon Retail Finance role evidence and Cargill Manufacturing & Supply Chain Finance role evidence.

