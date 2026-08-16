export type Language = "zh-CN" | "zh-HK" | "en";

interface TranslationDict {
  [key: string]: {
    "zh-CN": string;
    "zh-HK": string;
    en: string;
  };
}

const dict: TranslationDict = {
  // API error messages
  "api.session_expired": {
    "zh-CN": "登录已过期，请重新登录。",
    "zh-HK": "登入已過期，請重新登入。",
    en: "Your session has expired. Please sign in again.",
  },
  "api.forbidden": {
    "zh-CN": "当前账号没有执行此操作的权限。",
    "zh-HK": "目前帳號沒有執行此操作的權限。",
    en: "This account does not have permission to perform this action.",
  },
  // ERR-002: scope_denied must be distinguishable from permission_denied —
  // the object exists but is outside the caller's data scope. DESIGN.md §9
  // forbids softening this into "no data".
  "api.scope_denied": {
    "zh-CN": "该对象不在你的数据范围内，无法访问。请确认所属法人或门店范围。",
    "zh-HK": "該對象不在你的數據範圍內，無法存取。請確認所屬法人或門店範圍。",
    en: "This object is outside your data scope and cannot be accessed. Check your legal entity or store scope.",
  },
  // ERR-001 source_conflict detail reason: multiple source systems exist for
  // the same store-day, so the fact set is ambiguous until a source is picked.
  "api.source_conflict": {
    "zh-CN": "事实存在多个来源，请指定唯一 source_system 后重试。",
    "zh-HK": "事實存在多個來源，請指定唯一 source_system 後重試。",
    en: "The facts have multiple sources. Specify a single source_system and retry.",
  },
  "api.not_found": {
    "zh-CN": "请求的数据不存在或已被移除。",
    "zh-HK": "請求的資料不存在或已被移除。",
    en: "The requested data does not exist or has been removed.",
  },
  // FIX-002 data_unavailable detail reason: the engine refuses to measure
  // lease liability while a contract has no confirmed discount rate. The fix
  // is data entry, not retrying — AGENTS.md forbids guessing a rate.
  "api.discount_rate_missing": {
    "zh-CN": "合同 {contracts} 尚未确认折现率，无法计量租赁指标。请先在合同工作台补录折现率，或在系统设置配置全局折现率政策。",
    "zh-HK": "合同 {contracts} 尚未確認折現率，無法計量租賃指標。請先在合同工作台補錄折現率，或在系統設定配置全域折現率政策。",
    en: "Contract {contracts} has no confirmed discount rate, so lease metrics cannot be measured. Confirm the rate in the contract workspace first, or set a global rate policy in system settings.",
  },
  "api.discount_rate_missing_no_contracts": {
    "zh-CN": "折现率缺失，无法计量租赁指标。请先在合同工作台确认折现率，或在系统设置配置全局折现率政策。",
    "zh-HK": "折現率缺失，無法計量租賃指標。請先在合同工作台確認折現率，或在系統設定配置全域折現率政策。",
    en: "The discount rate is missing, so lease metrics cannot be measured. Confirm the rate in the contract workspace first, or set a global rate policy in system settings.",
  },
  // DIAG-001: rent-to-sales needs its healthy/warning ceilings from policy
  // settings — a config gap, so the copy says what to configure, not retry.
  "api.policy_thresholds_missing": {
    "zh-CN": "租售比健康线与预警线尚未配置，无法计算租售比。请到系统设置的参数配置中补齐后重试。",
    "zh-HK": "租售比健康線與預警線尚未配置，無法計算租售比。請到系統設定的參數配置中補齊後重試。",
    en: "The rent-to-sales healthy and warning ceilings are not configured. Set them under system settings parameters, then retry.",
  },
  "api.server_unavailable": {
    "zh-CN": "服务暂时不可用，请稍后重试。",
    "zh-HK": "服務暫時不可用，請稍後重試。",
    en: "The service is temporarily unavailable. Please try again later.",
  },
  "api.network_error": {
    "zh-CN": "网络连接失败，请检查服务状态后重试。",
    "zh-HK": "網絡連線失敗，請檢查服務狀態後重試。",
    en: "The network connection failed. Check the service status and try again.",
  },
  // Reached by any status that is not 401/403/404/5xx, which includes plain
  // page loads. Telling a user to check their input when they only opened a
  // page sends them looking for a form that is not there.
  "api.request_failed": {
    "zh-CN": "请求未成功，请重试。",
    "zh-HK": "請求未成功，請重試。",
    en: "The request did not succeed. Please try again.",
  },
  // DIAG-001: the unclassified fallback names the failing capability so the
  // toast points at the broken call instead of hiding it.
  "api.request_failed_with_endpoint": {
    "zh-CN": "请求未成功（{endpoint}），请重试。",
    "zh-HK": "請求未成功（{endpoint}），請重試。",
    en: "The request to {endpoint} did not succeed. Please try again.",
  },

  // I18N-001 — retail shared labels (CONTEXT.md terminology)
  "retail.kpi.revenue": {
    "zh-CN": "销售额",
    "zh-HK": "銷售額",
    en: "Revenue",
  },
  "retail.kpi.gross_profit": {
    "zh-CN": "毛利额",
    "zh-HK": "毛利額",
    en: "Gross profit",
  },
  "retail.kpi.gross_margin_rate": {
    "zh-CN": "毛利率",
    "zh-HK": "毛利率",
    en: "Gross margin rate",
  },
  "retail.kpi.footfall": {
    "zh-CN": "客流",
    "zh-HK": "客流",
    en: "Footfall",
  },
  "retail.kpi.conversion_rate": {
    "zh-CN": "转化率",
    "zh-HK": "轉化率",
    en: "Conversion rate",
  },
  // FIX-029: 「门店贡献」是行话，用户看不懂就得问。改叫「门店经营利润」——
  // 保留「利润」的白话，用「门店」限定范围，避免被当成利润表的营业利润
  // （本指标不含折旧摊销与总部分摊，且用现金租金而非 IFRS 16 租赁费用）。
  // 英文保持 store contribution：那是零售业对这个指标的标准叫法，不存在同名歧义。
  "retail.kpi.store_contribution": {
    "zh-CN": "门店经营利润",
    "zh-HK": "門店經營利潤",
    en: "Store contribution",
  },
  "retail.kpi.average_transaction_value": {
    "zh-CN": "客单价",
    "zh-HK": "客單價",
    en: "Average transaction value",
  },
  "retail.kpi.labor_cost": {
    "zh-CN": "人工成本",
    "zh-HK": "人工成本",
    en: "Labor cost",
  },
  "retail.kpi.labor_cost_rate": {
    "zh-CN": "人工成本率",
    "zh-HK": "人工成本率",
    en: "Labor cost rate",
  },
  "retail.kpi.occupancy_cash_cost_rate": {
    "zh-CN": "经营占用成本率",
    "zh-HK": "經營佔用成本率",
    en: "Occupancy cash cost rate",
  },
  "retail.kpi.store_contribution_margin": {
    "zh-CN": "门店经营利润率",
    "zh-HK": "門店經營利潤率",
    en: "Store contribution margin",
  },
  "retail.kpi.sales_per_sqm": {
    "zh-CN": "期间坪效",
    "zh-HK": "期間坪效",
    en: "Sales per sqm",
  },
  "retail.kpi.fixed_rent": {
    "zh-CN": "固定现金租金",
    "zh-HK": "固定現金租金",
    en: "Fixed cash rent",
  },
  "retail.kpi.variable_rent_rate": {
    "zh-CN": "变动租金率",
    "zh-HK": "變動租金率",
    en: "Variable rent rate",
  },
  "retail.kpi.variable_rent": {
    "zh-CN": "变动租金",
    "zh-HK": "變動租金",
    en: "Variable rent",
  },
  "retail.kpi.non_lease_cost": {
    "zh-CN": "非租赁占用成本",
    "zh-HK": "非租賃佔用成本",
    en: "Non-lease cost",
  },
  "retail.kpi.other_controllable_cost": {
    "zh-CN": "其他可控成本",
    "zh-HK": "其他可控成本",
    en: "Other controllable cost",
  },
  "retail.kpi.occupancy_cash_cost": {
    "zh-CN": "经营占用现金成本",
    "zh-HK": "經營佔用現金成本",
    en: "Occupancy cash cost",
  },
  "retail.signal.revenue_decline": {
    "zh-CN": "销售额下降",
    "zh-HK": "銷售額下降",
    en: "Revenue decline",
  },
  "retail.signal.footfall_decline": {
    "zh-CN": "客流下降",
    "zh-HK": "客流下降",
    en: "Footfall decline",
  },
  "retail.signal.footfall_continuous_decline": {
    "zh-CN": "连续客流下降",
    "zh-HK": "連續客流下降",
    en: "Continuous footfall decline",
  },
  "retail.signal.conversion_drop": {
    "zh-CN": "转化率下降",
    "zh-HK": "轉化率下降",
    en: "Conversion drop",
  },
  "retail.signal.conversion_rate_drop": {
    "zh-CN": "转化率下降",
    "zh-HK": "轉化率下降",
    en: "Conversion rate drop",
  },
  "retail.signal.average_ticket_drop": {
    "zh-CN": "客单价下降",
    "zh-HK": "客單價下降",
    en: "Average ticket drop",
  },
  "retail.signal.gross_margin_compression": {
    "zh-CN": "毛利率收窄",
    "zh-HK": "毛利率收窄",
    en: "Gross margin compression",
  },
  "retail.signal.labor_cost_rate_spike": {
    "zh-CN": "人工成本率上升",
    "zh-HK": "人工成本率上升",
    en: "Labor cost rate spike",
  },
  "retail.signal.labor_cost_spike": {
    "zh-CN": "人工成本率上升",
    "zh-HK": "人工成本率上升",
    en: "Labor cost spike",
  },
  "retail.signal.occupancy_cost_rate_spike": {
    "zh-CN": "经营占用成本率上升",
    "zh-HK": "經營佔用成本率上升",
    en: "Occupancy cost rate spike",
  },
  "retail.signal.occupancy_cost_burden": {
    "zh-CN": "经营占用成本率上升",
    "zh-HK": "經營佔用成本率上升",
    en: "Occupancy cost burden",
  },
  "retail.signal.contribution_turns_negative": {
    "zh-CN": "门店经营利润转负",
    "zh-HK": "門店經營利潤轉負",
    en: "Contribution turns negative",
  },
  "retail.unit.currency": {
    "zh-CN": "金额",
    "zh-HK": "金額",
    en: "amount",
  },
  "retail.unit.count": {
    "zh-CN": "笔/人次",
    "zh-HK": "筆/人次",
    en: "transactions/visits",
  },
  "retail.status.complete": {
    "zh-CN": "完整",
    "zh-HK": "完整",
    en: "Complete",
  },
  "retail.status.partial": {
    "zh-CN": "部分",
    "zh-HK": "部分",
    en: "Partial",
  },
  "retail.status.missing": {
    "zh-CN": "缺失",
    "zh-HK": "缺失",
    en: "Missing",
  },
  "retail.status_reason.unavailable": {
    "zh-CN": "指标不可用",
    "zh-HK": "指標不可用",
    en: "Metric unavailable",
  },
  "retail.status_reason.facts_unavailable": {
    "zh-CN": "所需事实不可用",
    "zh-HK": "所需事實不可用",
    en: "Required facts unavailable",
  },
  "retail.status_reason.coverage_incomplete": {
    "zh-CN": "覆盖不足或字段不完整",
    "zh-HK": "覆蓋不足或字段不完整",
    en: "Coverage incomplete or fields missing",
  },
  "retail.months": {
    "zh-CN": "个月",
    "zh-HK": "個月",
    en: " months",
  },
  "retail.classification.simulated": {
    "zh-CN": "模拟数据",
    "zh-HK": "模擬數據",
    en: "Simulated",
  },
  "retail.classification.production": {
    "zh-CN": "正式数据",
    "zh-HK": "正式數據",
    en: "Production",
  },
  "common.retry": {
    "zh-CN": "重试",
    "zh-HK": "重試",
    en: "Retry",
  },
  // STATE-003: StateBlock shared presentation labels
  "state.empty_label": { "zh-CN": "暂无数据", "zh-HK": "暫無數據", en: "No data" },
  "state.actionable_label": { "zh-CN": "需要操作", "zh-HK": "需要操作", en: "Action needed" },
  "state.failed_label": { "zh-CN": "加载失败", "zh-HK": "載入失敗", en: "Failed to load" },
  "state.scope_denied_label": { "zh-CN": "数据范围外", "zh-HK": "數據範圍外", en: "Outside your scope" },
  "common.refresh": {
    "zh-CN": "刷新",
    "zh-HK": "刷新",
    en: "Refresh",
  },
  "common.contrast": {
    "zh-CN": "对比",
    "zh-HK": "對比",
    en: "comparison",
  },
  "common.current": {
    "zh-CN": "当前",
    "zh-HK": "當前",
    en: "current",
  },
  "common.threshold": {
    "zh-CN": "阈值",
    "zh-HK": "閾值",
    en: "threshold",
  },
  "common.view_kpi_drilldown": {
    "zh-CN": "查看 KPI 下钻",
    "zh-HK": "查看 KPI 下鑽",
    en: "View KPI drilldown",
  },
  "common.go_pulse": {
    "zh-CN": "前往经营脉搏",
    "zh-HK": "前往經營脈搏",
    en: "Go to Operating Pulse",
  },

  // store-360 shared labels
  "store360.peer_status.complete": {
    "zh-CN": "可用",
    "zh-HK": "可用",
    en: "Available",
  },
  "store360.peer_status.insufficient_peers": {
    "zh-CN": "同群样本不足",
    "zh-HK": "同群樣本不足",
    en: "Insufficient peer cohort",
  },
  "store360.peer_status.unavailable": {
    "zh-CN": "不可用",
    "zh-HK": "不可用",
    en: "Unavailable",
  },
  "store360.trend.target": {
    "zh-CN": "目标门店",
    "zh-HK": "目標門店",
    en: "Target store",
  },
  "store360.trend.peer_median": {
    "zh-CN": "同群中位数",
    "zh-HK": "同群中位數",
    en: "Peer median",
  },
  "store360.trend.data_gap": {
    "zh-CN": "数据缺口",
    "zh-HK": "數據缺口",
    en: "Data gap",
  },

  // I18N-001 — operating-pulse page
  // HELP-001: usage tutorial panel（样板页：经营脉搏 + 租金谈判测算）
  "help.open_tutorial": { "zh-CN": "使用教程", "zh-HK": "使用教程", en: "Usage guide" },
  "help.pulse.title": { "zh-CN": "经营脉搏使用教程", "zh-HK": "經營脈搏使用教程", en: "Operating pulse guide" },
  "help.pulse.flow.classification": { "zh-CN": "选数据分类", "zh-HK": "選數據分類", en: "Pick data" },
  "help.pulse.flow.window": { "zh-CN": "设窗口与截止日", "zh-HK": "設窗口與截止日", en: "Set window" },
  "help.pulse.flow.read": { "zh-CN": "看指标与关注门店", "zh-HK": "看指標與關注門店", en: "Read KPIs" },
  "help.pulse.flow.drill": { "zh-CN": "下钻或追问", "zh-HK": "下鑽或追問", en: "Drill down" },
  "help.pulse.s1.heading": { "zh-CN": "数据分类", "zh-HK": "數據分類", en: "Data classification" },
  "help.pulse.s1.body": { "zh-CN": "模拟数据来自固定 seed，正式数据来自 POS 导入。切换分类会改变全部指标口径，页面顶部会标注当前来源。", "zh-HK": "模擬數據來自固定 seed，正式數據來自 POS 導入。切換分類會改變全部指標口徑，頁面頂部會標注當前來源。", en: "Simulated data comes from a fixed seed; production data comes from POS imports. Switching classification changes every metric's basis, and the source is always labelled at the top." },
  "help.pulse.s2.heading": { "zh-CN": "窗口与截止日", "zh-HK": "窗口與截止日", en: "Window and as-of" },
  "help.pulse.s2.body": { "zh-CN": "7 / 14 / 28 天窗口与 as-of 截止日决定「当前 vs 对比」两个区间；指标变化与关注门店都相对这两个区间计算。", "zh-HK": "7 / 14 / 28 天窗口與 as-of 截止日決定「當前 vs 對比」兩個區間；指標變化與關注門店都相對這兩個區間計算。", en: "The 7 / 14 / 28-day window and the as-of date define the current vs comparison spans; changes and attention stores are all relative to them." },
  "help.pulse.s3.heading": { "zh-CN": "关注门店", "zh-HK": "關注門店", en: "Attention stores" },
  "help.pulse.s3.body": { "zh-CN": "异常门店按严重度排序，信号标注指标与变化量。点「查看门店脉搏」下钻到门店 360 看拆解。", "zh-HK": "異常門店按嚴重度排序，信號標注指標與變化量。點「查看門店脈搏」下鑽到門店 360 看拆解。", en: "Anomalous stores rank by severity with the metric and change on each signal. Open a store's pulse to drill into its 360 breakdown." },
  "help.scenario.title": { "zh-CN": "租金谈判测算使用教程", "zh-HK": "租金談判測算使用教程", en: "Rent negotiation guide" },
  "help.scenario.flow.pick": { "zh-CN": "选门店与数据", "zh-HK": "選門店與數據", en: "Pick store" },
  "help.scenario.flow.assume": { "zh-CN": "调整假设", "zh-HK": "調整假設", en: "Adjust inputs" },
  "help.scenario.flow.result": { "zh-CN": "看利润变化", "zh-HK": "看利潤變化", en: "See impact" },
  "help.scenario.flow.act": { "zh-CN": "沉淀行动草稿", "zh-HK": "沉澱行動草稿", en: "Draft action" },
  "help.scenario.s1.heading": { "zh-CN": "门店与数据", "zh-HK": "門店與數據", en: "Store and data" },
  "help.scenario.s1.body": { "zh-CN": "先选门店，再确认模拟 / 正式分类与 as-of 基准日；基准决定整张表的起点。", "zh-HK": "先選門店，再確認模擬 / 正式分類與 as-of 基準日；基準決定整張表的起點。", en: "Pick a store, then confirm the simulated / production classification and the as-of baseline; the baseline anchors every figure." },
  "help.scenario.s2.heading": { "zh-CN": "调整假设", "zh-HK": "調整假設", en: "Adjusting inputs" },
  "help.scenario.s2.body": { "zh-CN": "租金、销售、人工三组假设按百分比步进调整；结果由服务端基于同一 Working 事实重算。", "zh-HK": "租金、銷售、人工三組假設按百分比步進調整；結果由服務端基於同一 Working 事實重算。", en: "Rent, sales and labor inputs step in percentages; the server recomputes on the same Working facts." },
  "help.scenario.s3.heading": { "zh-CN": "沉淀行动草稿", "zh-HK": "沉澱行動草稿", en: "Drafting an action" },
  "help.scenario.s3.body": { "zh-CN": "把结论写成行动草稿（负责人、期限、验证期间），确认前不写入任何业务表。", "zh-HK": "把結論寫成行動草稿（負責人、期限、驗證期間），確認前不寫入任何業務表。", en: "Turn the conclusion into an action draft (owner, due date, verification period); nothing is written before you confirm." },
  // HELP-002: 教程面板铺开（门店 360 / 经营驾驶舱 / 组合分析）
  "help.store360.title": { "zh-CN": "门店 360 使用教程", "zh-HK": "門店 360 使用教程", en: "Store 360 guide" },
  "help.store360.flow.pick": { "zh-CN": "选门店与分类", "zh-HK": "選門店與分類", en: "Pick store" },
  "help.store360.flow.read": { "zh-CN": "读六项指标", "zh-HK": "讀六項指標", en: "Read KPIs" },
  "help.store360.flow.bridge": { "zh-CN": "看利润流向", "zh-HK": "看利潤流向", en: "Profit flow" },
  "help.store360.flow.act": { "zh-CN": "对比与行动", "zh-HK": "對比與行動", en: "Benchmark" },
  "help.store360.s1.heading": { "zh-CN": "门店与数据分类", "zh-HK": "門店與數據分類", en: "Store and classification" },
  "help.store360.s1.body": { "zh-CN": "先选门店，再确认模拟 / 正式分类与 as-of 截止日。正式数据下没有该门店的经营事实时，页面会提示切换模拟数据或先导入正式数据。", "zh-HK": "先選門店，再確認模擬 / 正式分類與 as-of 截止日。正式數據下沒有該門店的經營事實時，頁面會提示切換模擬數據或先導入正式數據。", en: "Pick a store, then confirm the simulated / production classification and the as-of date. If production data has no facts for the store, the page offers switching to simulated data or importing production facts first." },
  "help.store360.s2.heading": { "zh-CN": "六项指标", "zh-HK": "六項指標", en: "The six KPIs" },
  "help.store360.s2.body": { "zh-CN": "营收、毛利、交易、客流、人工与租金占用按同一套 retail-kpi-v1 口径计算；覆盖不足或币种混杂时指标会显式降级为不可决策，不会编造数据点。", "zh-HK": "營收、毛利、交易、客流、人工與租金佔用按同一套 retail-kpi-v1 口徑計算；覆蓋不足或幣種混雜時指標會顯式降級為不可決策，不會編造數據點。", en: "Revenue, gross profit, transactions, traffic, labor and rent occupancy share one retail-kpi-v1 basis; insufficient coverage or mixed currencies degrade a metric to not-decision-ready explicitly — no fabricated points." },
  "help.store360.s3.heading": { "zh-CN": "利润流向与同群对比", "zh-HK": "利潤流向與同群對比", en: "Profit flow and peers" },
  "help.store360.s3.body": { "zh-CN": "桑基图展示营收如何分流到人工、租金、非租赁成本与其他成本，残差与状态显式标注。同群对比在样本不足或币种混杂时降级，不给出看似确定的结论。", "zh-HK": "桑基圖展示營收如何分流到人工、租金、非租賃成本與其他成本，殘差與狀態顯式標注。同群對比在樣本不足或幣種混雜時降級，不給出看似確定的結論。", en: "The Sankey shows how revenue flows into labor, rent, non-lease and other costs, with residual and status explicit. Peer benchmarks degrade when the sample is thin or currencies mix — no false certainty." },
  "help.performance.title": { "zh-CN": "经营驾驶舱使用教程", "zh-HK": "經營駕駛艙使用教程", en: "Performance cockpit guide" },
  "help.performance.flow.period": { "zh-CN": "选分析期间", "zh-HK": "選分析期間", en: "Pick period" },
  "help.performance.flow.overview": { "zh-CN": "看四格概览", "zh-HK": "看四格概覽", en: "Overview" },
  "help.performance.flow.actions": { "zh-CN": "处理行动", "zh-HK": "處理行動", en: "Actions" },
  "help.performance.flow.ask": { "zh-CN": "让 AI 解释", "zh-HK": "讓 AI 解釋", en: "Ask AI" },
  "help.performance.s1.heading": { "zh-CN": "分析期间", "zh-HK": "分析期間", en: "Analysis period" },
  "help.performance.s1.body": { "zh-CN": "输入 YYYY-MM 切换分析期间；概览四格与行动中心都相对该期间计算。页面顶部标注 Working 经营事实与数据截止时刻。", "zh-HK": "輸入 YYYY-MM 切換分析期間；概覽四格與行動中心都相對該期間計算。頁面頂部標注 Working 經營事實與數據截止時刻。", en: "Enter YYYY-MM to switch the analysis period; the overview and action center recompute relative to it. The header labels the Working facts basis and the data cutoff." },
  "help.performance.s2.heading": { "zh-CN": "行动中心与数据治理", "zh-HK": "行動中心與數據治理", en: "Actions and governance" },
  "help.performance.s2.body": { "zh-CN": "行动中心列出待处理行动，可按状态确认或批量确认并导出 Working CSV。数据治理边界把缺失、未映射、未对账分开展示，AI 只引用系统事实。", "zh-HK": "行動中心列出待處理行動，可按狀態確認或批量確認並導出 Working CSV。數據治理邊界把缺失、未映射、未對賬分開展示，AI 只引用系統事實。", en: "The action center lists open actions; acknowledge individually or in batch and export a Working CSV. The governance banner separates missing, unmapped and unreconciled facts; AI cites only system facts." },
  "help.performance.s3.heading": { "zh-CN": "零售四墙与方案模拟", "zh-HK": "零售四牆與方案模擬", en: "Store walls and simulation" },
  "help.performance.s3.body": { "zh-CN": "零售四墙与制造设备页签逐店展示事实与对账状态；门店方案模拟只进入确定性模拟，不会覆盖预算、创建正式合同或触发会计重算。", "zh-HK": "零售四牆與製造設備頁簽逐店展示事實與對賬狀態；門店方案模擬只進入確定性模擬，不會覆蓋預算、創建正式合同或觸發會計重算。", en: "The retail-walls and equipment tabs list per-store facts with reconciliation status; the store simulation is deterministic only — it never overwrites budgets, creates contracts or triggers accounting recalculation." },
  "help.portfolio.title": { "zh-CN": "组合分析使用教程", "zh-HK": "組合分析使用教程", en: "Portfolio analysis guide" },
  "performance.load_failed": { "zh-CN": "经营数据加载失败", "zh-HK": "經營數據載入失敗", en: "Failed to load operating data" },
  "help.portfolio.flow.mode": { "zh-CN": "选 Working / Official", "zh-HK": "選 Working / Official", en: "Pick mode" },
  "help.portfolio.flow.group": { "zh-CN": "选分组维度", "zh-HK": "選分組維度", en: "Grouping" },
  "help.portfolio.flow.read": { "zh-CN": "读组合指标", "zh-HK": "讀組合指標", en: "Read metrics" },
  "help.portfolio.flow.export": { "zh-CN": "导出与下钻", "zh-HK": "導出與下鑽", en: "Export" },
  "help.portfolio.s1.heading": { "zh-CN": "Working 与 Official", "zh-HK": "Working 與 Official", en: "Working vs Official" },
  "help.portfolio.s1.body": { "zh-CN": "Working 报表可含草稿与待审批数据，用于内部试算；Official 报表只含已审批数据，用于正式财务与审计。两种模式都标注数据来源与版本。", "zh-HK": "Working 報表可含草稿與待審批數據，用於內部試算；Official 報表只含已審批數據，用於正式財務與審計。兩種模式都標注數據來源與版本。", en: "Working reports may include draft and pending-approval data for internal runs; Official reports contain only approved data for finance and audit. Both modes label source and version." },
  "help.portfolio.s2.heading": { "zh-CN": "分组与指标", "zh-HK": "分組與指標", en: "Grouping and metrics" },
  "help.portfolio.s2.body": { "zh-CN": "按法人、品牌、区域或门店分组查看租赁负债、使用权资产与费用分布；切换分组会重算同组小计与占比。", "zh-HK": "按法人、品牌、區域或門店分組查看租賃負債、使用權資產與費用分佈；切換分組會重算同組小計與佔比。", en: "Group by legal entity, brand, region or store to see the distribution of lease liabilities, right-of-use assets and expenses; switching groups recomputes subtotals and shares." },
  "help.portfolio.s3.heading": { "zh-CN": "导出与下钻", "zh-HK": "導出與下鑽", en: "Export and drill-down" },
  "help.portfolio.s3.body": { "zh-CN": "组合视图可导出当前口径的明细；导出文件标注 Working / Official，与页面口径一致，不会混入另一种模式的数据。", "zh-HK": "組合視圖可導出當前口徑的明細；導出文件標注 Working / Official，與頁面口徑一致，不會混入另一種模式的數據。", en: "Export the current view's detail; the file carries the Working / Official label matching the page basis — the two modes never mix." },
  "common.ai_analysis": {
    "zh-CN": "交给 AI 分析",
    "zh-HK": "交給 AI 分析",
    en: "Analyze with AI",
  },
  "portfolio.summary_failed": { "zh-CN": "组合分析加载失败", "zh-HK": "組合分析載入失敗", en: "Failed to load portfolio summary" },
  "portfolio.unit_price_failed": { "zh-CN": "单价对比加载失败", "zh-HK": "單價對比載入失敗", en: "Failed to load unit-price comparison" },
  "common.source_system": {
    "zh-CN": "来源系统",
    "zh-HK": "來源系統",
    en: "Source system",
  },
  "common.source_system_optional": {
    "zh-CN": "source_system（可选）",
    "zh-HK": "source_system（可選）",
    en: "source_system (optional)",
  },
  "common.store360": {
    "zh-CN": "门店 360",
    "zh-HK": "門店 360",
    en: "Store 360",
  },
  "common.no_trend": {
    "zh-CN": "暂无趋势事实",
    "zh-HK": "暫無趨勢事實",
    en: "No trend facts",
  },
  "common.days_suffix": {
    "zh-CN": "天",
    "zh-HK": "天",
    en: " days",
  },
  "pulse.title": {
    "zh-CN": "经营脉搏",
    "zh-HK": "經營脈搏",
    en: "Operating Pulse",
  },
  "pulse.signal_mix_title": {
    "zh-CN": "信号构成",
    "zh-HK": "信號構成",
    en: "Signal mix",
  },
  "pulse.signal_mix_weight": {
    "zh-CN": "累计权重",
    "zh-HK": "累計權重",
    en: "Cumulative weight",
  },
  "pulse.signal_mix_stores": {
    "zh-CN": "涉及 {count} 家门店",
    "zh-HK": "涉及 {count} 家門店",
    en: "{count} stores affected",
  },
  "pulse.trend_title": {
    "zh-CN": "每日趋势",
    "zh-HK": "每日趨勢",
    en: "Daily trend",
  },
  "pulse.col.priority": {
    "zh-CN": "优先",
    "zh-HK": "優先",
    en: "Priority",
  },
  "pulse.col.store": {
    "zh-CN": "门店",
    "zh-HK": "門店",
    en: "Store",
  },
  "pulse.col.signal": {
    "zh-CN": "信号",
    "zh-HK": "信號",
    en: "Signals",
  },
  "pulse.col.change": {
    "zh-CN": "变化",
    "zh-HK": "變化",
    en: "Change",
  },
  "pulse.col.score": {
    "zh-CN": "评分",
    "zh-HK": "評分",
    en: "Score",
  },
  "pulse.col.source": {
    "zh-CN": "数据来源",
    "zh-HK": "數據來源",
    en: "Source",
  },
  "pulse.col.action": {
    "zh-CN": "操作",
    "zh-HK": "操作",
    en: "Action",
  },
  "pulse.view_store_pulse": {
    "zh-CN": "查看门店脉搏",
    "zh-HK": "查看門店脈搏",
    en: "View store pulse",
  },
  "pulse.no_signals": {
    "zh-CN": "当前筛选下未触发固定经营信号",
    "zh-HK": "當前篩選下未觸發固定經營信號",
    en: "No fixed operating signals triggered under the current filter",
  },
  "pulse.suppressed_title": {
    "zh-CN": "数据不足而被抑制的门店",
    "zh-HK": "數據不足而被抑制的門店",
    en: "Suppressed stores with insufficient data",
  },
  "pulse.err_missing_dataset_version": {
    "zh-CN": "模拟数据缺少 dataset_version，请从最新数据集重新进入。",
    "zh-HK": "模擬數據缺少 dataset_version，請從最新數據集重新進入。",
    en: "Simulated data is missing a dataset_version. Re-enter from the latest dataset.",
  },
  "pulse.err_invalid_window": {
    "zh-CN": "窗口仅支持 7、14 或 28 天，请选择一个有效窗口。",
    "zh-HK": "窗口僅支持 7、14 或 28 天，請選擇一個有效窗口。",
    en: "The window only supports 7, 14 or 28 days. Pick a valid window.",
  },
  "pulse.demo_generated": {
    "zh-CN": "固定演示数据已生成",
    "zh-HK": "固定演示數據已生成",
    en: "Fixed demo dataset generated",
  },
  "pulse.anomaly_select": {
    "zh-CN": "模拟场景",
    "zh-HK": "模擬場景",
    en: "Simulation scenario",
  },
  "pulse.all_anomalies": {
    "zh-CN": "全部固定异常",
    "zh-HK": "全部固定異常",
    en: "All fixed anomalies",
  },
  "pulse.as_of": {
    "zh-CN": "截至日期",
    "zh-HK": "截至日期",
    en: "As-of date",
  },
  "pulse.window": {
    "zh-CN": "窗口",
    "zh-HK": "窗口",
    en: "Window",
  },
  "pulse.back_all_stores": {
    "zh-CN": "返回全部门店",
    "zh-HK": "返回全部門店",
    en: "Back to all stores",
  },
  "pulse.all_authorized_stores": {
    "zh-CN": "全部授权门店",
    "zh-HK": "全部授權門店",
    en: "All authorized stores",
  },
  "pulse.no_dataset_title": {
    "zh-CN": "当前法人还没有模拟数据集",
    "zh-HK": "當前法人還沒有模擬數據集",
    en: "This legal entity has no simulated dataset yet",
  },
  "pulse.no_dataset_desc": {
    "zh-CN": "生成固定 60 店演示数据后，可复演六类经营信号。页面不会自动写入数据。",
    "zh-HK": "生成固定 60 店演示數據後，可復演六類經營信號。頁面不會自動寫入數據。",
    en: "After generating the fixed 60-store demo dataset, the six operating signals can be replayed. The page never writes data by itself.",
  },
  "pulse.generate_demo": {
    "zh-CN": "生成固定演示数据",
    "zh-HK": "生成固定演示數據",
    en: "Generate fixed demo data",
  },
  "pulse.contact_admin": {
    "zh-CN": "请联系当前法人管理员生成演示数据。",
    "zh-HK": "請聯繫當前法人管理員生成演示數據。",
    en: "Ask the legal entity admin to generate the demo dataset.",
  },
  "pulse.unavailable_title": {
    "zh-CN": "经营脉搏暂不可用",
    "zh-HK": "經營脈搏暫不可用",
    en: "Operating Pulse is temporarily unavailable",
  },
  "pulse.loading": {
    "zh-CN": "读取经营脉搏…",
    "zh-HK": "讀取經營脈搏…",
    en: "Loading Operating Pulse…",
  },
  "pulse.no_facts_title": {
    "zh-CN": "当前正式数据窗口没有事实",
    "zh-HK": "當前正式數據窗口沒有事實",
    en: "No facts in the current production window",
  },
  "pulse.no_facts_desc": {
    "zh-CN": "请先导入并完成门店日事实映射，再刷新经营脉搏。系统不会用 0 填补缺失。",
    "zh-HK": "請先導入並完成門店日事實映射，再刷新經營脈搏。系統不會用 0 填補缺失。",
    en: "Import and map store-day facts first, then refresh. The system never fills gaps with 0.",
  },
  "pulse.currency_partition": {
    "zh-CN": "币种分区",
    "zh-HK": "幣種分區",
    en: "Currency partition",
  },
  "pulse.unknown_currency": {
    "zh-CN": "未知币种",
    "zh-HK": "未知幣種",
    en: "Unknown currency",
  },
  "pulse.aux_metrics": {
    "zh-CN": "辅助指标",
    "zh-HK": "輔助指標",
    en: "Auxiliary metrics",
  },
  "pulse.cash_basis_title": {
    "zh-CN": "经营现金口径",
    "zh-HK": "經營現金口徑",
    en: "Operating cash basis",
  },
  "pulse.cash_basis_desc": {
    "zh-CN": "经营占用现金成本不等于 IFRS 16 折旧、利息、ROU 或租赁负债变动。",
    "zh-HK": "經營佔用現金成本不等於 IFRS 16 折舊、利息、ROU 或租賃負債變動。",
    en: "Operating cash occupancy cost is not IFRS 16 depreciation, interest, ROU, or lease liability movement.",
  },
  "pulse.store_pulse_title": {
    "zh-CN": "门店脉搏",
    "zh-HK": "門店脈搏",
    en: "Store pulse",
  },
  "pulse.priority_stores": {
    "zh-CN": "优先关注门店",
    "zh-HK": "優先關注門店",
    en: "Priority stores",
  },
  "pulse.api_order": {
    "zh-CN": "按 API rank 原序 · 不在前端重算评分",
    "zh-HK": "按 API rank 原序 · 不在前端重算評分",
    en: "In API rank order · never re-scored on the frontend",
  },

  // I18N-001 — operating-pulse suppressed table
  "pulse.col.brand_region": {
    "zh-CN": "品牌 / 区域",
    "zh-HK": "品牌 / 區域",
    en: "Brand / region",
  },
  "pulse.col.reason": {
    "zh-CN": "原因",
    "zh-HK": "原因",
    en: "Reason",
  },
  "pulse.col.coverage": {
    "zh-CN": "覆盖",
    "zh-HK": "覆蓋",
    en: "Coverage",
  },

  // I18N-001 — operating-pulse table
  "pulse.col.status": {
    "zh-CN": "状态",
    "zh-HK": "狀態",
    en: "Status",
  },

  // I18N-001 — store-360 page
  "store360.title": {
    "zh-CN": "门店 360",
    "zh-HK": "門店 360",
    en: "Store 360",
  },
  "store360.scope_note": {
    "zh-CN": "仅供 Working 经营分析，不作解释性判断。",
    "zh-HK": "僅供 Working 經營分析，不作解釋性判斷。",
    en: "Working analysis only, never an interpretive judgment.",
  },
  "store360.scenario_analysis": {
    "zh-CN": "情景分析",
    "zh-HK": "情景分析",
    en: "Scenario analysis",
  },
  "store360.back_pulse": {
    "zh-CN": "返回经营脉搏",
    "zh-HK": "返回經營脈搏",
    en: "Back to Operating Pulse",
  },
  "store360.select_store": {
    "zh-CN": "选择授权门店",
    "zh-HK": "選擇授權門店",
    en: "Select an authorized store",
  },
  "store360.loading_stores": {
    "zh-CN": "加载门店…",
    "zh-HK": "加載門店…",
    en: "Loading stores…",
  },
  "store360.no_selectable_stores": {
    "zh-CN": "当前范围没有可选门店",
    "zh-HK": "當前範圍沒有可選門店",
    en: "No selectable stores in the current scope",
  },
  "store360.apply_source": {
    "zh-CN": "应用来源",
    "zh-HK": "應用來源",
    en: "Apply source",
  },
  "pulse.apply_source": {
    "zh-CN": "应用来源",
    "zh-HK": "應用來源",
    en: "Apply source",
  },
  "store360.no_dataset_title": {
    "zh-CN": "还没有可用的模拟数据集",
    "zh-HK": "還沒有可用的模擬數據集",
    en: "No simulated dataset available yet",
  },
  "store360.no_dataset_desc": {
    "zh-CN": "请先在经营脉搏由管理员按固定流程生成演示数据，之后从门店关注行进入门店 360。",
    "zh-HK": "請先在經營脈搏由管理員按固定流程生成演示數據，之後從門店關注行進入門店 360。",
    en: "Generate the demo dataset in Operating Pulse first, then enter Store 360 from a priority store row.",
  },
  "store360.missing_version_title": {
    "zh-CN": "模拟数据集版本缺失",
    "zh-HK": "模擬數據集版本缺失",
    en: "Simulated dataset version missing",
  },
  "store360.missing_version_desc": {
    "zh-CN": "请从经营脉搏选择一个可用数据集后再进入门店 360；本页不会自动生成或补写数据。",
    "zh-HK": "請從經營脈搏選擇一個可用數據集後再進入門店 360；本頁不會自動生成或補寫數據。",
    en: "Pick an available dataset in Operating Pulse before entering Store 360; this page never generates or writes data.",
  },
  "store360.options_error": {
    "zh-CN": "门店列表加载失败",
    "zh-HK": "門店列表加載失敗",
    en: "Failed to load the store list",
  },
  "store360.no_authorized_stores": {
    "zh-CN": "当前范围没有授权门店",
    "zh-HK": "當前範圍沒有授權門店",
    en: "No authorized stores in the current scope",
  },
  "store360.no_authorized_desc": {
    "zh-CN": "请检查法人、region/brand/store 数据权限或选择其他 classification/dataset；系统不会自动选择或补造门店。",
    "zh-HK": "請檢查法人、region/brand/store 數據權限或選擇其他 classification/dataset；系統不會自動選擇或補造門店。",
    en: "Check legal entity, region/brand/store data permissions or pick another classification/dataset; the system never auto-selects or fabricates stores.",
  },
  "store360.loading": {
    "zh-CN": "读取门店诊断…",
    "zh-HK": "讀取門店診斷…",
    en: "Loading store diagnostics…",
  },
  "store360.unavailable": {
    "zh-CN": "门店诊断暂不可用",
    "zh-HK": "門店診斷暫不可用",
    en: "Store diagnostics temporarily unavailable",
  },
  "store360.pick_filters": {
    "zh-CN": "请选择完整筛选条件后读取门店事实。",
    "zh-HK": "請選擇完整篩選條件後讀取門店事實。",
    en: "Pick the full filter set to read store facts.",
  },
  "store360.no_facts_title": {
    "zh-CN": "当前窗口没有门店事实",
    "zh-HK": "當前窗口沒有門店事實",
    en: "No store facts in the current window",
  },
  "store360.no_facts_desc": {
    "zh-CN": "请先导入并完成该门店的经营日事实，或选择包含有效事实的数据集；系统不会用 0 填补缺失。",
    "zh-HK": "請先導入並完成該門店的經營日事實，或選擇包含有效事實的數據集；系統不會用 0 填補缺失。",
    en: "Import and map the store's daily facts, or choose a dataset that contains them; the system never fills gaps with 0.",
  },
  "store360.identity": {
    "zh-CN": "门店身份",
    "zh-HK": "門店身份",
    en: "Store identity",
  },
  "store360.field.store": {
    "zh-CN": "门店",
    "zh-HK": "門店",
    en: "Store",
  },
  "store360.field.brand_region": {
    "zh-CN": "品牌 / 区域",
    "zh-HK": "品牌 / 區域",
    en: "Brand / region",
  },
  "store360.field.currency": {
    "zh-CN": "币种",
    "zh-HK": "幣種",
    en: "Currency",
  },
  "store360.field.fact_version": {
    "zh-CN": "事实版本",
    "zh-HK": "事實版本",
    en: "Fact version",
  },
  "store360.aux_metrics": {
    "zh-CN": "辅助指标",
    "zh-HK": "輔助指標",
    en: "Auxiliary metrics",
  },
  "store360.cash_basis_title": {
    "zh-CN": "经营口径",
    "zh-HK": "經營口徑",
    en: "Operating basis",
  },
  "store360.cash_basis_desc": {
    "zh-CN": "经营占用现金成本仅用于经营分析，未混入 IFRS 16 计量或 Official 过账链路。",
    "zh-HK": "經營佔用現金成本僅用於經營分析，未混入 IFRS 16 計量或 Official 過賬鏈路。",
    en: "Operating cash occupancy cost is for operating analysis only and never enters IFRS 16 measurement or the Official posting chain.",
  },
  // SANKEY-001 一期：利润流向桑基图
  "store360.pl_flow.title": { "zh-CN": "利润流向", "zh-HK": "利潤流向", en: "Profit flow" },
  "store360.pl_flow.status": { "zh-CN": "状态", "zh-HK": "狀態", en: "Status" },
  "store360.pl_flow.residual": { "zh-CN": "未归因金额", "zh-HK": "未歸因金額", en: "Unattributed" },
  "store360.pl_flow.missing": { "zh-CN": "缺失字段", "zh-HK": "缺失字段", en: "Missing fields" },
  "store360.pl_flow.formula": { "zh-CN": "口径", "zh-HK": "口徑", en: "Formula" },
  "store360.pl_flow.unavailable": { "zh-CN": "当前窗口没有可用于利润流向的经营事实", "zh-HK": "當前窗口沒有可用於利潤流向的經營事實", en: "No operating facts in the current window for a profit-flow view" },
  "store360.pl_flow.pick_store": { "zh-CN": "选择门店后展示利润流向", "zh-HK": "選擇門店後展示利潤流向", en: "Pick a store to see its profit flow" },
  // STATE-001：门店 360 正式数据下无事实（404）→ actionable
  "store360.actionable_production_empty": { "zh-CN": "正式数据下没有该门店的经营事实。切换到模拟数据，或先导入正式数据。", "zh-HK": "正式數據下沒有該門店的經營事實。切換到模擬數據，或先導入正式數據。", en: "No operating facts for this store under production data. Switch to simulated data, or import production facts first." },
  "store360.actionable_switch_simulated": { "zh-CN": "切换到模拟数据", "zh-HK": "切換到模擬數據", en: "Switch to simulated" },
  // STATE-001：合同详情无付款计划时计量 → actionable
  "contract_detail.calculate_no_schedules": { "zh-CN": "该合同还没有付款计划，无法计量。已为你打开付款计划页签，去添加付款计划。", "zh-HK": "該合同還沒有付款計劃，無法計量。已為你打開付款計劃頁簽，去添加付款計劃。", en: "This contract has no payment schedule yet, so it cannot be measured. The payments tab is open — add a schedule there." },
  // STATE-001：设置页标签统计被折现率缺失阻塞 → actionable
  "settings.tags_actionable": { "zh-CN": "标签统计需要先补全折现率：{contracts}。请到合同工作台补录，或在本页配置全局折现率。", "zh-HK": "標籤統計需要先補全折現率：{contracts}。請到合同工作台補錄，或在本頁配置全域折現率。", en: "Tag statistics need a confirmed discount rate first: {contracts}. Confirm it on the contract, or set a global rate on this page." },
  "store360.pl_flow.load_failed": { "zh-CN": "利润流向加载失败", "zh-HK": "利潤流向載入失敗", en: "Profit flow failed to load" },
  "store360.peer_benchmark": {
    "zh-CN": "同群基准",
    "zh-HK": "同群基準",
    en: "Peer benchmark",
  },
  "store360.peer_definition": {
    "zh-CN": "最少 {n} 家同群门店",
    "zh-HK": "最少 {n} 家同群門店",
    en: "minimum {n} peer stores",
  },
  "store360.col.metric": {
    "zh-CN": "指标",
    "zh-HK": "指標",
    en: "Metric",
  },
  "store360.col.target": {
    "zh-CN": "目标",
    "zh-HK": "目標",
    en: "Target",
  },
  "store360.col.quartiles": {
    "zh-CN": "P25 / 中位 / P75",
    "zh-HK": "P25 / 中位 / P75",
    en: "P25 / median / P75",
  },
  "store360.col.sample_percentile": {
    "zh-CN": "样本 / 百分位",
    "zh-HK": "樣本 / 百分位",
    en: "Sample / percentile",
  },
  "store360.col.status": {
    "zh-CN": "状态",
    "zh-HK": "狀態",
    en: "Status",
  },
  "store360.observations": {
    "zh-CN": "观察信号",
    "zh-HK": "觀察信號",
    en: "Observations",
  },
  "store360.no_observations": {
    "zh-CN": "当前期间没有可用观察信号",
    "zh-HK": "當前期間沒有可用觀察信號",
    en: "No usable observations in the current period",
  },
  "store360.evidence_title": {
    "zh-CN": "证据与可追溯性",
    "zh-HK": "證據與可追溯性",
    en: "Evidence & traceability",
  },
  "store360.evidence.coverage_source": {
    "zh-CN": "覆盖 {observed}/{expected} store-days · 来源 {sources} · dataset {datasets}",
    "zh-HK": "覆蓋 {observed}/{expected} store-days · 來源 {sources} · dataset {datasets}",
    en: "Coverage {observed}/{expected} store-days · source {sources} · dataset {datasets}",
  },
  "store360.evidence.fact_version": {
    "zh-CN": "fact version {min}–{max}",
    "zh-HK": "fact version {min}–{max}",
    en: "fact version {min}–{max}",
  },
  "store360.bridge.title": {
    "zh-CN": "变化贡献桥（仅观察信号）",
    "zh-HK": "變化貢獻橋（僅觀察信號）",
    en: "Change contribution bridge (observational)",
  },
  "store360.bridge.complete": {
    "zh-CN": "可用",
    "zh-HK": "可用",
    en: "Available",
  },
  "store360.bridge.unavailable": {
    "zh-CN": "不可用",
    "zh-HK": "不可用",
    en: "Unavailable",
  },
  "store360.bridge.reason_default": {
    "zh-CN": "所需字段不可用，未补零。",
    "zh-HK": "所需字段不可用，未補零。",
    en: "Required fields unavailable; nothing was zero-filled.",
  },
  "store360.bridge.contrast": {
    "zh-CN": "对比",
    "zh-HK": "對比",
    en: "comparison",
  },
  "store360.bridge.current": {
    "zh-CN": "当前",
    "zh-HK": "當前",
    en: "current",
  },
  "store360.bridge.change": {
    "zh-CN": "变化",
    "zh-HK": "變化",
    en: "change",
  },
  "store360.bridge.residual": {
    "zh-CN": "守恒残差",
    "zh-HK": "守恆殘差",
    en: "conservation residual",
  },
  "store360.bridge.chart_title": {
    "zh-CN": "变化贡献分解",
    "zh-HK": "變化貢獻分解",
    en: "Change contribution breakdown",
  },
  "store360.bridge.start": {
    "zh-CN": "期初",
    "zh-HK": "期初",
    en: "Opening",
  },
  "store360.bridge.end": {
    "zh-CN": "期末",
    "zh-HK": "期末",
    en: "Closing",
  },
  "store360.bridge.no_complete": {
    "zh-CN": "本期没有可分解的完整变化贡献",
    "zh-HK": "本期沒有可分解的完整變化貢獻",
    en: "No complete change bridge to break down this period",
  },
  "store360.bridge.item": {
    "zh-CN": "变化项",
    "zh-HK": "變化項",
    en: "Change item",
  },
  "store360.bridge.contribution": {
    "zh-CN": "贡献",
    "zh-HK": "貢獻",
    en: "Contribution",
  },
  "store360.trend_title": {
    "zh-CN": "每日趋势（目标门店 / 同群中位数）",
    "zh-HK": "每日趨勢（目標門店 / 同群中位數）",
    en: "Daily trend (target store / peer median)",
  },

  // I18N-001 — scenario-workbench page（FIX-014：用户定名「租金谈判测算」；
  // 路由 /scenario-workbench 与组件名、API 路径不变）
  "scenario.title": {
    "zh-CN": "租金谈判测算",
    "zh-HK": "租金談判測算",
    en: "Rent Negotiation Scenario",
  },
  // FIX-014: the compliance wording stays — it moves under the plain
  // subtitle as the scope note, exactly as before, only repositioned.
  "scenario.scope_note": {
    "zh-CN": "门店经营 What-if；服务端基于同一 Working 事实重算 30-day run-rate，不输出最优方案或 IFRS 16 影响。",
    "zh-HK": "門店經營 What-if；服務端基於同一 Working 事實重算 30-day run-rate，不輸出最優方案或 IFRS 16 影響。",
    en: "Store what-if; the server recomputes a 30-day run-rate on the same Working facts, with no optimal plan or IFRS 16 impact.",
  },
  "scenario.calculate": {
    "zh-CN": "计算情景",
    "zh-HK": "計算情景",
    en: "Evaluate scenario",
  },
  "scenario.back_store360": {
    "zh-CN": "返回门店 360",
    "zh-HK": "返回門店 360",
    en: "Back to Store 360",
  },
  "scenario.delta.revenue_change_pct": {
    "zh-CN": "销售额变化",
    "zh-HK": "銷售額變化",
    en: "Revenue change",
  },
  "scenario.delta.gross_margin_rate_change_pp": {
    "zh-CN": "毛利率变化",
    "zh-HK": "毛利率變化",
    en: "Gross margin rate change",
  },
  "scenario.delta.labor_cost_change_pct": {
    "zh-CN": "人工成本变化",
    "zh-HK": "人工成本變化",
    en: "Labor cost change",
  },
  "scenario.delta.fixed_rent_change_pct": {
    "zh-CN": "固定现金租金变化",
    "zh-HK": "固定現金租金變化",
    en: "Fixed cash rent change",
  },
  "scenario.delta.variable_rent_rate_change_pp": {
    "zh-CN": "变动租金率变化",
    "zh-HK": "變動租金率變化",
    en: "Variable rent rate change",
  },
  "scenario.delta.non_lease_cost_change_pct": {
    "zh-CN": "非租赁占用成本变化",
    "zh-HK": "非租賃佔用成本變化",
    en: "Non-lease cost change",
  },
  "scenario.delta.other_controllable_cost_change_pct": {
    "zh-CN": "其他可控成本变化",
    "zh-HK": "其他可控成本變化",
    en: "Other controllable cost change",
  },
  "scenario.horizon": {
    "zh-CN": "预测期",
    "zh-HK": "預測期",
    en: "Horizon",
  },
  "scenario.months_option": {
    "zh-CN": "{n}个月",
    "zh-HK": "{n}個月",
    en: "{n} months",
  },
  "scenario.baseline_note": {
    "zh-CN": "Baseline 为固定零 delta；Plan 的七类变化会由服务端重新计算。",
    "zh-HK": "Baseline 為固定零 delta；Plan 的七類變化會由服務端重新計算。",
    en: "Baseline is a fixed zero-delta; the seven Plan changes are recomputed server-side.",
  },
  "scenario.col.metric": {
    "zh-CN": "指标",
    "zh-HK": "指標",
    en: "Metric",
  },
  "scenario.col.baseline": {
    "zh-CN": "Baseline",
    "zh-HK": "Baseline",
    en: "Baseline",
  },
  "scenario.col.plan": {
    "zh-CN": "Plan",
    "zh-HK": "Plan",
    en: "Plan",
  },
  "scenario.col.change": {
    "zh-CN": "变化",
    "zh-HK": "變化",
    en: "Change",
  },
  "scenario.col.status": {
    "zh-CN": "状态",
    "zh-HK": "狀態",
    en: "Status",
  },
  "scenario.err_select_store": {
    "zh-CN": "请先选择授权门店。",
    "zh-HK": "請先選擇授權門店。",
    en: "Select an authorized store first.",
  },
  "scenario.err_dataset_version": {
    "zh-CN": "模拟数据必须明确 dataset version。",
    "zh-HK": "模擬數據必須明確 dataset version。",
    en: "Simulated data must specify a dataset version.",
  },
  "scenario.confirm_save_title": {
    "zh-CN": "保存行动草稿？",
    "zh-HK": "保存行動草稿？",
    en: "Save the action draft?",
  },
  "scenario.confirm_save_content": {
    "zh-CN": "服务端会重新读取 Working 事实并只写入一条 open 行动草稿。",
    "zh-HK": "服務端會重新讀取 Working 事實並只寫入一條 open 行動草稿。",
    en: "The server re-reads Working facts and writes a single open action draft.",
  },
  "scenario.saved_replay": {
    "zh-CN": "已安全重放现有草稿",
    "zh-HK": "已安全重放現有草稿",
    en: "Existing draft replayed safely",
  },
  "scenario.saved": {
    "zh-CN": "行动草稿已保存",
    "zh-HK": "行動草稿已保存",
    en: "Action draft saved",
  },
  "scenario.pick_store": {
    "zh-CN": "请选择授权门店后计算情景；本页不会自动生成模拟数据。",
    "zh-HK": "請選擇授權門店後計算情景；本頁不會自動生成模擬數據。",
    en: "Pick an authorized store to evaluate; this page never generates simulated data.",
  },
  "scenario.missing_version_title": {
    "zh-CN": "模拟 dataset version 缺失",
    "zh-HK": "模擬 dataset version 缺失",
    en: "Simulated dataset version missing",
  },
  "scenario.missing_version_desc": {
    "zh-CN": "请从经营脉搏选择固定模拟数据集；页面不会把 production 或旧版本伪装成 latest。",
    "zh-HK": "請從經營脈搏選擇固定模擬數據集；頁面不會把 production 或舊版本偽裝成 latest。",
    en: "Pick the fixed simulated dataset in Operating Pulse; the page never disguises production or old versions as latest.",
  },
  "scenario.options_error": {
    "zh-CN": "授权门店加载失败",
    "zh-HK": "授權門店加載失敗",
    en: "Failed to load authorized stores",
  },
  "scenario.no_authorized": {
    "zh-CN": "当前范围没有授权门店",
    "zh-HK": "當前範圍沒有授權門店",
    en: "No authorized stores in the current scope",
  },
  "scenario.no_authorized_desc": {
    "zh-CN": "请检查法人、classification、dataset 或数据权限；系统不会自动选择或补造门店。",
    "zh-HK": "請檢查法人、classification、dataset 或數據權限；系統不會自動選擇或補造門店。",
    en: "Check legal entity, classification, dataset or data permissions; the system never auto-selects or fabricates stores.",
  },
  "scenario.stale_title": {
    "zh-CN": "结果已过期，不能保存",
    "zh-HK": "結果已過期，不能保存",
    en: "Result is stale and cannot be saved",
  },
  "scenario.stale_desc": {
    "zh-CN": "门店、日期窗口、来源、预测期或七项情景假设已变化。请重新计算后再生成行动草稿。",
    "zh-HK": "門店、日期窗口、來源、預測期或七項情景假設已變化。請重新計算後再生成行動草稿。",
    en: "Store, window, source, horizon or the seven assumptions changed. Re-evaluate before saving a draft.",
  },
  "scenario.recalculate": {
    "zh-CN": "重新计算",
    "zh-HK": "重新計算",
    en: "Re-evaluate",
  },
  "scenario.not_evaluated_title": {
    "zh-CN": "尚未评估任何情景",
    "zh-HK": "尚未評估任何情景",
    en: "No scenario evaluated yet",
  },
  "scenario.not_evaluated_desc": {
    "zh-CN": "选择门店并点击「开始测算」后，这里才会显示数据口径与可信度信息；在此之前不展示任何就绪状态。",
    "zh-HK": "選擇門店並點擊「開始測算」後，這裡才會顯示數據口徑與可信度信息；在此之前不展示任何就緒狀態。",
    en: "Data lineage and trust information appear only after you pick a store and run an evaluation; no readiness is implied before that.",
  },
  "scenario.option_baseline": {
    "zh-CN": "Baseline（什么都不做）",
    "zh-HK": "Baseline（什麼都不做）",
    en: "Baseline (do nothing)",
  },
  "scenario.unavailable": {
    "zh-CN": "情景不可用",
    "zh-HK": "情景不可用",
    en: "Scenario unavailable",
  },
  "scenario.loading": {
    "zh-CN": "服务端读取事实并计算 30-day run-rate…",
    "zh-HK": "服務端讀取事實並計算 30-day run-rate…",
    en: "Reading facts and computing the 30-day run-rate…",
  },
  "scenario.baseline_title": {
    "zh-CN": "Baseline / {name}",
    "zh-HK": "Baseline / {name}",
    en: "Baseline / {name}",
  },
  "scenario.stale": {
    "zh-CN": "结果已过期",
    "zh-HK": "結果已過期",
    en: "Stale result",
  },
  "scenario.monthly_change": {
    "zh-CN": "月度贡献变化",
    "zh-HK": "月度貢獻變化",
    en: "Monthly contribution change",
  },
  "scenario.evidence_title": {
    "zh-CN": "证据与公式",
    "zh-HK": "證據與公式",
    en: "Evidence & formulas",
  },
  "scenario.evidence.facts": {
    "zh-CN": "当前事实：{from}–{to} · 覆盖 {observed}/{expected}（{rate}）",
    "zh-HK": "當前事實：{from}–{to} · 覆蓋 {observed}/{expected}（{rate}）",
    en: "Current facts: {from}–{to} · coverage {observed}/{expected} ({rate})",
  },
  "scenario.evidence.formula": {
    "zh-CN": "公式：30-day run-rate = 30 ÷ observed store-days；贡献额 = 毛利额 − 人工 − 经营占用现金成本 − 其他可控成本。经营占用现金成本不等同 IFRS 16 会计费用。",
    "zh-HK": "公式：30-day run-rate = 30 ÷ observed store-days；貢獻額 = 毛利額 − 人工 − 經營佔用現金成本 − 其他可控成本。經營佔用現金成本不等同 IFRS 16 會計費用。",
    en: "Formula: 30-day run-rate = 30 ÷ observed store-days; contribution = gross profit − labor − occupancy cash cost − other controllable cost. Occupancy cash cost is not IFRS 16 accounting.",
  },
  "scenario.view_drilldown": {
    "zh-CN": "查看 KPI 事实下钻",
    "zh-HK": "查看 KPI 事實下鑽",
    en: "View KPI drilldown",
  },
  "scenario.bridge.title": {
    "zh-CN": "变化贡献桥（守恒，非根因）",
    "zh-HK": "變化貢獻橋（守恆，非根因）",
    en: "Change contribution bridge (conserved, not causal)",
  },
  "scenario.bridge.item": {
    "zh-CN": "变化项",
    "zh-HK": "變化項",
    en: "Change item",
  },
  "scenario.bridge.contribution": {
    "zh-CN": "贡献",
    "zh-HK": "貢獻",
    en: "Contribution",
  },
  "scenario.bridge.total": {
    "zh-CN": "总变化",
    "zh-HK": "總變化",
    en: "Total change",
  },
  "scenario.bridge.residual": {
    "zh-CN": "残差",
    "zh-HK": "殘差",
    en: "Residual",
  },
  "scenario.bridge.conservation": {
    "zh-CN": "守恒误差",
    "zh-HK": "守恆誤差",
    en: "Conservation error",
  },
  "scenario.draft.title": {
    "zh-CN": "行动草稿（只写 open）",
    "zh-HK": "行動草稿（只寫 open）",
    en: "Action draft (open only)",
  },
  "scenario.draft.title_placeholder": {
    "zh-CN": "标题",
    "zh-HK": "標題",
    en: "Title",
  },
  "scenario.draft.action_placeholder": {
    "zh-CN": "计划动作（需复核验证）",
    "zh-HK": "計劃動作（需覆核驗證）",
    en: "Planned action (requires review)",
  },
  "scenario.draft.owner_placeholder": {
    "zh-CN": "Owner（可空）",
    "zh-HK": "Owner（可空）",
    en: "Owner (optional)",
  },
  "scenario.draft.period_placeholder": {
    "zh-CN": "验证期间 YYYY-MM",
    "zh-HK": "驗證期間 YYYY-MM",
    en: "Verification period YYYY-MM",
  },
  "scenario.draft.save": {
    "zh-CN": "保存行动草稿",
    "zh-HK": "保存行動草稿",
    en: "Save action draft",
  },
  "scenario.draft.saved_title": {
    "zh-CN": "行动草稿已安全保存/重放",
    "zh-HK": "行動草稿已安全保存/重放",
    en: "Action draft saved/replayed safely",
  },
  "scenario.draft.real_id": {
    "zh-CN": "真实 ID：{id} · status={status}",
    "zh-HK": "真實 ID：{id} · status={status}",
    en: "Real ID: {id} · status={status}",
  },
  "scenario.draft.idempotent_yes": {
    "zh-CN": "是",
    "zh-HK": "是",
    en: "Yes",
  },
  "scenario.draft.idempotent_no": {
    "zh-CN": "否",
    "zh-HK": "否",
    en: "No",
  },
  "scenario.draft.idempotency": {
    "zh-CN": "Idempotency replay：{value}",
    "zh-HK": "Idempotency replay：{value}",
    en: "Idempotency replay: {value}",
  },
  "scenario.draft.go_workbench": {
    "zh-CN": "前往经营工作台查看",
    "zh-HK": "前往經營工作台查看",
    en: "Open the performance workbench",
  },

  // AI-001 — explainability components (DESIGN.md §9)
  "ai.tool.completed": {
    "zh-CN": "已完成",
    "zh-HK": "已完成",
    en: "Completed",
  },
  "ai.tool.failed": {
    "zh-CN": "失败",
    "zh-HK": "失敗",
    en: "Failed",
  },
  "ai.tool.needs_review": {
    "zh-CN": "需复核",
    "zh-HK": "需覆核",
    en: "Needs review",
  },
  "ai.tool.output_chars": {
    "zh-CN": "输出 {n} 字符",
    "zh-HK": "輸出 {n} 字符",
    en: "{n} chars out",
  },
  "ai.tool.duration": {
    "zh-CN": "{ms}ms",
    "zh-HK": "{ms}ms",
    en: "{ms}ms",
  },
  "ai.citation.anonymous": {
    "zh-CN": "来源",
    "zh-HK": "來源",
    en: "Source",
  },
  "ai.confidence.low": {
    "zh-CN": "低置信",
    "zh-HK": "低置信",
    en: "Low confidence",
  },
  "ai.confidence.medium": {
    "zh-CN": "中置信",
    "zh-HK": "中置信",
    en: "Medium confidence",
  },
  "ai.confidence.high": {
    "zh-CN": "高置信",
    "zh-HK": "高置信",
    en: "High confidence",
  },
  "ai.confidence.reason": {
    "zh-CN": "降级原因",
    "zh-HK": "降級原因",
    en: "Degradation reason",
  },

  // AI-002 — approval card and retail AI drawer
  "ai.approval.role": {
    "zh-CN": "AI 行动提议",
    "zh-HK": "AI 行動提議",
    en: "AI action proposal",
  },
  "ai.approval.untitled": {
    "zh-CN": "未命名提议",
    "zh-HK": "未命名提議",
    en: "Untitled proposal",
  },
  "ai.approval.evidence_complete": {
    "zh-CN": "证据完整",
    "zh-HK": "證據完整",
    en: "Evidence complete",
  },
  "ai.approval.evidence_incomplete": {
    "zh-CN": "需补证据",
    "zh-HK": "需補證據",
    en: "Evidence incomplete",
  },
  "ai.approval.expected_benefit": {
    "zh-CN": "预期影响",
    "zh-HK": "預期影響",
    en: "Expected impact",
  },
  "ai.approval.adopt": {
    "zh-CN": "采纳",
    "zh-HK": "採納",
    en: "Adopt",
  },
  "ai.approval.modify": {
    "zh-CN": "修改",
    "zh-HK": "修改",
    en: "Modify",
  },
  "ai.approval.reject": {
    "zh-CN": "拒绝",
    "zh-HK": "拒絕",
    en: "Reject",
  },
  "ai.drawer.title": {
    "zh-CN": "交给 AI 分析",
    "zh-HK": "交給 AI 分析",
    en: "Analyze with AI",
  },
  "ai.drawer.context": {
    "zh-CN": "当前页面上下文",
    "zh-HK": "當前頁面上下文",
    en: "Current page context",
  },
  "ai.drawer.empty": {
    "zh-CN": "针对当前页面提问，AI 回答会附引用来源。",
    "zh-HK": "針對當前頁面提問，AI 回答會附引用來源。",
    en: "Ask about this page; answers carry citations.",
  },
  "ai.drawer.placeholder": {
    "zh-CN": "输入问题…",
    "zh-HK": "輸入問題…",
    en: "Ask a question…",
  },
  "ai.drawer.send": {
    "zh-CN": "发送",
    "zh-HK": "發送",
    en: "Send",
  },
  "ai.drawer.no_answer": {
    "zh-CN": "（无回答）",
    "zh-HK": "（無回答）",
    en: "(no answer)",
  },

  // AI-002 — proposal notice
  "ai.approval.notice": {
    "zh-CN": "仅为行动提议：确认前不会写入行动清单或正式台账。",
    "zh-HK": "僅為行動提議：確認前不會寫入行動清單或正式台賬。",
    en: "This is only a proposal: nothing is written until you confirm.",
  },
  "ai.approval.rejected": {
    "zh-CN": "已拒绝该提议",
    "zh-HK": "已拒絕該提議",
    en: "Proposal rejected",
  },

  // ENV-002 DataTrustBar — DESIGN.md §10 数据可信度展示
  "trust.classification_production": {
    "zh-CN": "正式数据 · Working",
    "zh-HK": "正式數據 · Working",
    en: "Production data · Working",
  },
  "trust.classification_simulated": {
    "zh-CN": "模拟数据 · 不进入 Official",
    "zh-HK": "模擬數據 · 不進入 Official",
    en: "Simulated data · never Official",
  },
  "trust.classification_mixed": {
    "zh-CN": "混合数据 · 不进入 Official",
    "zh-HK": "混合數據 · 不進入 Official",
    en: "Mixed data · never Official",
  },
  "trust.ready": {
    "zh-CN": "决策就绪",
    "zh-HK": "決策就緒",
    en: "Decision ready",
  },
  "trust.not_ready": {
    "zh-CN": "未达决策就绪 · 仅可查看",
    "zh-HK": "未達決策就緒 · 僅可查看",
    en: "Not decision ready · view only",
  },
  "trust.kpi_not_ready": {
    "zh-CN": "仅供查看",
    "zh-HK": "僅供查看",
    en: "View only",
  },
  "trust.expand": {
    "zh-CN": "查看全部口径",
    "zh-HK": "查看全部口徑",
    en: "View full provenance",
  },
  "trust.collapse": {
    "zh-CN": "收起",
    "zh-HK": "收起",
    en: "Collapse",
  },
  "trust.store_days": {
    "zh-CN": "store-days",
    "zh-HK": "store-days",
    en: "store-days",
  },
  "trust.comparison": {
    "zh-CN": "对比",
    "zh-HK": "對比",
    en: "comparison",
  },
  "trust.source": {
    "zh-CN": "来源",
    "zh-HK": "來源",
    en: "Source",
  },
  "trust.dataset": {
    "zh-CN": "dataset",
    "zh-HK": "dataset",
    en: "Dataset",
  },
  "trust.fact_version": {
    "zh-CN": "fact version",
    "zh-HK": "fact version",
    en: "Fact version",
  },
  "trust.as_of": {
    "zh-CN": "最高事实截至",
    "zh-HK": "最高事實截至",
    en: "Highest as-of",
  },
  "trust.formula": {
    "zh-CN": "formula",
    "zh-HK": "formula",
    en: "Formula",
  },
  "trust.pulse": {
    "zh-CN": "pulse",
    "zh-HK": "pulse",
    en: "Pulse",
  },
  "trust.semantic": {
    "zh-CN": "envelope",
    "zh-HK": "envelope",
    en: "Envelope",
  },
  "trust.reason.incomplete_store_day_coverage": {
    "zh-CN": "门店日覆盖不足",
    "zh-HK": "門店日覆蓋不足",
    en: "Incomplete store-day coverage",
  },
  "trust.reason.not_decision_ready": {
    "zh-CN": "未达决策就绪标准",
    "zh-HK": "未達決策就緒標準",
    en: "Not decision ready",
  },
  "trust.reason.scenario_not_ready": {
    "zh-CN": "情景计算未完成",
    "zh-HK": "情景計算未完成",
    en: "Scenario not evaluated",
  },
  "trust.reason.currency_conflict": {
    "zh-CN": "币种冲突",
    "zh-HK": "幣種衝突",
    en: "Currency conflict",
  },
  "trust.reason.insufficient_peer_count": {
    "zh-CN": "同群样本不足",
    "zh-HK": "同群樣本不足",
    en: "Insufficient peer cohort",
  },
  "trust.reason.data_quality_invalid": {
    "zh-CN": "数据质量无效",
    "zh-HK": "數據質量無效",
    en: "Invalid data quality",
  },
  "trust.reason.diagnostics_not_decision_ready": {
    "zh-CN": "诊断未达决策就绪",
    "zh-HK": "診斷未達決策就緒",
    en: "Diagnostics not decision ready",
  },
  "trust.reason.no_facts": {
    "zh-CN": "无事实数据",
    "zh-HK": "無事實數據",
    en: "No facts",
  },
  "trust.reason.raw_facts_read": {
    "zh-CN": "原始事实读取，非决策结论",
    "zh-HK": "原始事實讀取，非決策結論",
    en: "Raw facts read, not a decision",
  },

  // AppLayout
  "nav.home": {
    "zh-CN": "首页",
    "zh-HK": "首頁",
    en: "Home",
  },
  "nav.contracts": {
    "zh-CN": "合同台账",
    "zh-HK": "合同台賬",
    en: "Contracts",
  },
  "nav.ai_chat": {
    "zh-CN": "AI Chat",
    "zh-HK": "AI Chat",
    en: "AI Chat",
  },
  "nav.upload": { "zh-CN": "批量上传", "zh-HK": "批量上傳", en: "Batch upload" },
  "nav.reports": {
    "zh-CN": "报表查询",
    "zh-HK": "報表查詢",
    en: "Reports",
  },
  "nav.operating_pulse": {
    "zh-CN": "经营脉搏",
    "zh-HK": "經營脈搏",
    en: "Operating Pulse",
  },
  "nav.store_360": {
    "zh-CN": "门店 360",
    "zh-HK": "門店 360",
    en: "Store 360",
  },
  "nav.scenario_workbench": {
    "zh-CN": "租金谈判测算",
    "zh-HK": "租金談判測算",
    en: "Rent Negotiation Scenario",
  },
  "nav.cashflow": {
    "zh-CN": "现金流预测",
    "zh-HK": "現金流預測",
    en: "Cashflow Forecast",
  },
  "nav.monthly_closing": {
    "zh-CN": "结账中心",
    "zh-HK": "結賬中心",
    en: "Monthly Closing",
  },
  "nav.audit_logs": {
    "zh-CN": "审计日志",
    "zh-HK": "審計日誌",
    en: "Audit Logs",
  },
  "nav.agent_metrics": {
    "zh-CN": "Agent 运营",
    "zh-HK": "Agent 運營",
    en: "Agent Operations",
  },
  "nav.retail_data_import": {
    "zh-CN": "经营数据导入",
    "zh-HK": "經營數據導入",
    en: "Retail Data Import",
  },
  "retail_import.title": {
    "zh-CN": "经营数据导入",
    "zh-HK": "經營數據導入",
    en: "Retail Data Import",
  },
  "retail_import.scope_note": {
    "zh-CN": "上传 POS / 财务导出的 CSV 或 XLSX，生成 production 的 store-day 经营事实；导入后直接进入经营脉搏查看覆盖情况",
    "zh-HK": "上傳 POS / 財務導出的 CSV 或 XLSX，生成 production 的 store-day 經營事實；導入後直接進入經營脈搏查看覆蓋情況",
    en: "Upload a POS / finance CSV or XLSX export to create production store-day facts, then review coverage in the operating pulse",
  },
  "retail_import.upload_hint": {
    "zh-CN": "受控模板：第一行为表头；支持 .csv 与 .xlsx，单文件 ≤10MB；重复上传同一内容将以新事实版本取代旧行",
    "zh-HK": "受控模板：第一行為表頭；支持 .csv 與 .xlsx，單文件 ≤10MB；重複上傳同一內容將以新事實版本取代舊行",
    en: "Controlled template: header row first; .csv and .xlsx up to 10MB; re-importing overlapping store-days supersedes them as a new fact version",
  },
  "retail_import.source_system": { "zh-CN": "来源系统", "zh-HK": "來源系統", en: "Source system" },
  "retail_import.as_of": { "zh-CN": "As-of 日期", "zh-HK": "As-of 日期", en: "As-of date" },
  "retail_import.revalidate": { "zh-CN": "重新校验", "zh-HK": "重新校驗", en: "Re-validate" },
  "retail_import.commit": { "zh-CN": "确认导入", "zh-HK": "確認導入", en: "Commit import" },
  "retail_import.committing": { "zh-CN": "导入中…", "zh-HK": "導入中…", en: "Importing…" },
  "retail_import.mapping_title": { "zh-CN": "列映射（建议已自动填入，请确认）", "zh-HK": "列映射（建議已自動填入，請確認）", en: "Column mapping (suggestion prefilled — confirm or adjust)" },
  "retail_import.field": { "zh-CN": "标准字段", "zh-HK": "標準字段", en: "Standard field" },
  "retail_import.file_column": { "zh-CN": "文件列", "zh-HK": "文件列", en: "File column" },
  "retail_import.unmapped": { "zh-CN": "（未映射）", "zh-HK": "（未映射）", en: "(unmapped)" },
  "retail_import.required": { "zh-CN": "必填", "zh-HK": "必填", en: "required" },
  "retail_import.report_title": { "zh-CN": "校验报告", "zh-HK": "校驗報告", en: "Validation report" },
  "retail_import.total_rows": { "zh-CN": "总行数", "zh-HK": "總行數", en: "Total rows" },
  "retail_import.valid_rows": { "zh-CN": "有效行", "zh-HK": "有效行", en: "Valid rows" },
  "retail_import.store_count": { "zh-CN": "门店数", "zh-HK": "門店數", en: "Stores" },
  "retail_import.overlap": { "zh-CN": "重叠 store-day（新版本取代）", "zh-HK": "重疊 store-day（新版本取代）", en: "Overlapping store-days (superseded)" },
  "retail_import.new_days": { "zh-CN": "新增 store-day", "zh-HK": "新增 store-day", en: "New store-days" },
  "retail_import.unmatched_stores": { "zh-CN": "未匹配门店（不在当前法人门店主数据）", "zh-HK": "未匹配門店（不在當前法人門店主數據）", en: "Unmatched stores (not in this entity's store master)" },
  "retail_import.row_errors": { "zh-CN": "行级错误", "zh-HK": "行級錯誤", en: "Row errors" },
  "retail_import.error_row": { "zh-CN": "行", "zh-HK": "行", en: "Row" },
  "retail_import.error_code": { "zh-CN": "错误", "zh-HK": "錯誤", en: "Error" },
  "retail_import.error_message": { "zh-CN": "说明", "zh-HK": "說明", en: "Message" },
  "retail_import.missing_fields": { "zh-CN": "必填字段未映射：", "zh-HK": "必填字段未映射：", en: "Unmapped required fields: " },
  "retail_import.ambiguous": { "zh-CN": "映射歧义（多列指向同一字段）：", "zh-HK": "映射歧義（多列指向同一字段）：", en: "Ambiguous mappings (multiple columns → one field): " },
  "retail_import.commit_success": { "zh-CN": "导入完成", "zh-HK": "導入完成", en: "Import completed" },
  "retail_import.go_pulse": { "zh-CN": "查看经营脉搏（production）", "zh-HK": "查看經營脈搏（production）", en: "Open operating pulse (production)" },
  "retail_import.empty_title": { "zh-CN": "尚未选择文件", "zh-HK": "尚未選擇文件", en: "No file selected yet" },
  "retail_import.empty_desc": { "zh-CN": "选择文件后，这里会显示列映射建议、门店匹配结果与行级校验报告；确认前不会写入任何数据", "zh-HK": "選擇文件後，這裡會顯示列映射建議、門店匹配結果與行級校驗報告；確認前不會寫入任何數據", en: "Pick a file to see the suggested mapping, store matching and row-level validation; nothing is written before you confirm" },
  "retail_import.previewing": { "zh-CN": "解析中…", "zh-HK": "解析中…", en: "Parsing…" },
  "retail_import.field_label.store": { "zh-CN": "门店", "zh-HK": "門店", en: "Store" },
  "retail_import.field_label.business_date": { "zh-CN": "日期", "zh-HK": "日期", en: "Business date" },
  "retail_import.field_label.currency": { "zh-CN": "币种", "zh-HK": "幣種", en: "Currency" },
  "retail_import.field_label.revenue": { "zh-CN": "营业额", "zh-HK": "營業額", en: "Revenue" },
  "retail_import.field_label.gross_profit": { "zh-CN": "毛利", "zh-HK": "毛利", en: "Gross profit" },
  "retail_import.field_label.transactions": { "zh-CN": "交易数", "zh-HK": "交易數", en: "Transactions" },
  "retail_import.field_label.footfall": { "zh-CN": "客流", "zh-HK": "客流", en: "Footfall" },
  "retail_import.field_label.area_sqm": { "zh-CN": "面积（㎡）", "zh-HK": "面積（㎡）", en: "Area (sqm)" },
  "retail_import.field_label.labor_cost": { "zh-CN": "人工成本", "zh-HK": "人工成本", en: "Labor cost" },
  "retail_import.field_label.fixed_rent": { "zh-CN": "固定租金", "zh-HK": "固定租金", en: "Fixed rent" },
  "retail_import.field_label.variable_rent": { "zh-CN": "变量租金", "zh-HK": "變量租金", en: "Variable rent" },
  "retail_import.field_label.non_lease_cost": { "zh-CN": "非租赁成本", "zh-HK": "非租賃成本", en: "Non-lease cost" },
  "retail_import.field_label.other_controllable_cost": { "zh-CN": "其他可控成本", "zh-HK": "其他可控成本", en: "Other controllable cost" },
  "agent_metrics.title": {
    "zh-CN": "Agent 运营与用量",
    "zh-HK": "Agent 運營與用量",
    en: "Agent Operations & Usage",
  },
  "agent_metrics.range_24h": {
    "zh-CN": "最近 24 小时",
    "zh-HK": "最近 24 小時",
    en: "Last 24 hours",
  },
  "agent_metrics.range_7d": {
    "zh-CN": "最近 7 天",
    "zh-HK": "最近 7 天",
    en: "Last 7 days",
  },
  "agent_metrics.range_31d": {
    "zh-CN": "最近 31 天",
    "zh-HK": "最近 31 天",
    en: "Last 31 days",
  },
  "agent_metrics.refresh": {
    "zh-CN": "刷新",
    "zh-HK": "刷新",
    en: "Refresh",
  },
  "agent_metrics.permission_required": {
    "zh-CN": "需要管理员或审计查看权限",
    "zh-HK": "需要管理員或審計查看權限",
    en: "Administrator or auditor access is required",
  },
  "agent_metrics.load_failed": {
    "zh-CN": "Agent 用量加载失败",
    "zh-HK": "Agent 用量加載失敗",
    en: "Failed to load Agent usage",
  },
  "agent_metrics.calls": {
    "zh-CN": "Planner 调用次数",
    "zh-HK": "Planner 調用次數",
    en: "Planner calls",
  },
  "agent_metrics.input_tokens": {
    "zh-CN": "输入 Token",
    "zh-HK": "輸入 Token",
    en: "Input tokens",
  },
  "agent_metrics.output_tokens": {
    "zh-CN": "输出 Token",
    "zh-HK": "輸出 Token",
    en: "Output tokens",
  },
  "agent_metrics.provider": {
    "zh-CN": "供应商",
    "zh-HK": "供應商",
    en: "Provider",
  },
  "agent_metrics.model": {
    "zh-CN": "模型",
    "zh-HK": "模型",
    en: "Model",
  },
  "agent_metrics.pricing_version": {
    "zh-CN": "价格版本",
    "zh-HK": "價格版本",
    en: "Pricing version",
  },
  "agent_metrics.tokens": {
    "zh-CN": "总 Token",
    "zh-HK": "總 Token",
    en: "Total tokens",
  },
  "agent_metrics.cost_status": {
    "zh-CN": "成本状态",
    "zh-HK": "成本狀態",
    en: "Cost status",
  },
  "agent_metrics.cost": {
    "zh-CN": "成本（USD）",
    "zh-HK": "成本（USD）",
    en: "Cost (USD)",
  },
  "agent_metrics.cost_unavailable": {
    "zh-CN": "成本暂不可用",
    "zh-HK": "成本暫不可用",
    en: "Cost accounting is unavailable",
  },
  "agent_metrics.cost_unavailable_desc": {
    "zh-CN": "部分 Planner 事件缺少可验证的价格版本或成本数据。系统保留 Token 用量，并明确显示 unavailable，不会猜测金额。",
    "zh-HK": "部分 Planner 事件缺少可驗證的價格版本或成本數據。系統保留 Token 用量，並明確顯示 unavailable，不會猜測金額。",
    en: "Some Planner events lack a verifiable pricing version or cost value. Token usage remains visible, while cost stays unavailable instead of being guessed.",
  },
  "agent_metrics.breakdown": {
    "zh-CN": "按供应商与模型汇总",
    "zh-HK": "按供應商與模型匯總",
    en: "Breakdown by provider and model",
  },
  "agent_metrics.empty": {
    "zh-CN": "所选期间暂无 Planner 用量",
    "zh-HK": "所選期間暫無 Planner 用量",
    en: "No Planner usage in the selected period",
  },
  "agent_metrics.audit_note": {
    "zh-CN": "口径：仅统计服务端写入的 planner_usage 事件；范围由当前登录身份和法人权限决定；成本以微美元持久化，只有 measured/calculated 才会展示金额。",
    "zh-HK": "口徑：僅統計服務端寫入的 planner_usage 事件；範圍由當前登入身份和法人權限決定；成本以微美元持久化，只有 measured/calculated 才會展示金額。",
    en: "Scope: server-generated planner_usage events only. The current identity and legal-entity permissions define visibility. Costs are persisted in micros and shown only for measured/calculated statuses.",
  },
  "nav.settings": {
    "zh-CN": "设置",
    "zh-HK": "設置",
    en: "Settings",
  },
  "nav.admin": {
    "zh-CN": "管理后台",
    "zh-HK": "管理後台",
    en: "Admin",
  },
  "app.title": {
    "zh-CN": "零售经营分析工作站",
    "zh-HK": "零售經營分析工作站",
    en: "Retail Performance Workstation",
  },
  "user.profile": {
    "zh-CN": "个人资料",
    "zh-HK": "個人資料",
    en: "Profile",
  },
  "user.logout": {
    "zh-CN": "退出登录",
    "zh-HK": "退出登錄",
    en: "Logout",
  },

  // AI Chat
  "ai.welcome": {
    "zh-CN": "你好！我是零售经营分析助手。我可以读取经营数据并调用内置 skill 帮你完成任务：\n\n1. 读取经营脉搏，定位需要关注的门店与异常信号\n2. 对关注门店做门店 360 诊断，解释营收与贡献变化\n3. 评估确定性经营情景，生成待确认的行动建议\n4. 解析合同与台账文件并生成草稿（租赁与 IFRS 16 计量保留）\n\n上传文件或直接提问后，我会先生成草稿和工具执行轨迹，确认后才进入正式流程。",
    "zh-HK": "你好！我是零售經營分析助手。我可以讀取經營數據並調用內置 skill 幫你完成任務：\n\n1. 讀取經營脈搏，定位需要關注的門店與異常訊號\n2. 對關注門店做門店 360 診斷，解釋營收與貢獻變化\n3. 評估確定性經營情景，生成待確認的行動建議\n4. 解析合同與台賬文件並生成草稿（租賃與 IFRS 16 計量保留）\n\n上傳文件或直接提問後，我會先生成草稿和工具執行軌跡，確認後才進入正式流程。",
    en: "Hello! I am the Retail Performance Assistant. I read operating data and call built-in skills to help you:\n\n1. Read the operating pulse and surface stores that need attention\n2. Diagnose attention stores with Store 360, explaining revenue and contribution changes\n3. Evaluate deterministic operating scenarios and draft actions for confirmation\n4. Parse contracts and ledger files into drafts (lease and IFRS 16 measurement remain)\n\nAfter you upload a file or ask a question, I first produce drafts and a tool execution trace; only confirmed items enter the formal flow.",
  },
  "ai.context": {
    "zh-CN": "当前上下文：",
    "zh-HK": "當前上下文：",
    en: "Current Context:",
  },
  "ai.quick_questions": {
    "zh-CN": "快捷提问：",
    "zh-HK": "快捷提問：",
    en: "Quick Questions:",
  },
  "ai.placeholder": {
    "zh-CN": "告诉 Agent 你的目标，或上传合同/台账文件...",
    "zh-HK": "告訴 Agent 你的目標，或上傳合同/台賬文件...",
    en: "Tell the Agent your goal, or upload a contract/ledger file...",
  },
  "ai.thinking": {
    "zh-CN": "AI 思考中...",
    "zh-HK": "AI 思考中...",
    en: "AI thinking...",
  },
  "ai.sources": {
    "zh-CN": "引用来源:",
    "zh-HK": "引用來源:",
    en: "Sources:",
  },
  "ai.model": {
    "zh-CN": "模型: ",
    "zh-HK": "模型: ",
    en: "Model: ",
  },
  "ai.upload_success": {
    "zh-CN": "上传成功",
    "zh-HK": "上傳成功",
    en: "Upload successful",
  },
  "ai.upload_failed": {
    "zh-CN": "上传失败",
    "zh-HK": "上傳失敗",
    en: "Upload failed",
  },
  "ai.unsupported_file": {
    "zh-CN": "不支持的文件类型，请上传 PDF、Excel 或图片文件",
    "zh-HK": "不支持的文件類型，請上傳 PDF、Excel 或圖片文件",
    en: "Unsupported file type. Please upload PDF, Excel, or image files.",
  },
  "ai.file_too_large": {
    "zh-CN": "文件大小不能超过 50MB",
    "zh-HK": "文件大小不能超過 50MB",
    en: "File size cannot exceed 50MB",
  },
  "ai.new_session": {
    "zh-CN": "新会话",
    "zh-HK": "新會話",
    en: "New Session",
  },
  "ai.copied": {
    "zh-CN": "已复制",
    "zh-HK": "已複製",
    en: "Copied",
  },
  "ai.copy": {
    "zh-CN": "复制",
    "zh-HK": "複製",
    en: "Copy",
  },
  "ai.thinking_process": {
    "zh-CN": "思考过程",
    "zh-HK": "思考過程",
    en: "Thinking Process",
  },
  "ai.model_label": {
    "zh-CN": "模型:",
    "zh-HK": "模型:",
    en: "Model:",
  },
  "ai.new_session_btn": {
    "zh-CN": "新建会话",
    "zh-HK": "新建會話",
    en: "New Session",
  },
  "ai.no_sessions": {
    "zh-CN": "暂无会话",
    "zh-HK": "暫無會話",
    en: "No Sessions",
  },
  "ai.chip_risk": {
    "zh-CN": "这份合同有什么风险？",
    "zh-HK": "這份合同有什麼風險？",
    en: "What are the risks of this contract?",
  },
  "ai.chip_why_no_calc": {
    "zh-CN": "为什么不能计算？",
    "zh-HK": "為什麼不能計算？",
    en: "Why can't it be calculated?",
  },
  "ai.chip_accounting_impact": {
    "zh-CN": "这份合同的关键会计影响是什么？",
    "zh-HK": "這份合同的關鍵會計影響是什麼？",
    en: "What are the key accounting impacts of this contract?",
  },
  "ai.chip_explain_report": {
    "zh-CN": "帮我解释这张报表的结果",
    "zh-HK": "幫我解釋這張報表的結果",
    en: "Help me interpret the results of this report",
  },
  "ai.chip_query_scope": {
    "zh-CN": "这次查询用了什么口径？",
    "zh-HK": "這次查詢用了什麼口徑？",
    en: "What scope was used for this query?",
  },
  "ai.chip_anomalies": {
    "zh-CN": "有哪些异常值得关注？",
    "zh-HK": "有哪些異常值得關注？",
    en: "What anomalies deserve attention?",
  },
  "ai.chip_blockers": {
    "zh-CN": "当前期间还有哪些阻塞项？",
    "zh-HK": "當前期間還有哪些阻塞項？",
    en: "What blockers remain in the current period?",
  },
  "ai.chip_entries_source": {
    "zh-CN": "这些分录主要来自什么？",
    "zh-HK": "這些分錄主要來自什麼？",
    en: "What is the main source of these entries?",
  },
  "ai.chip_next_steps": {
    "zh-CN": "下一步建议做什么？",
    "zh-HK": "下一步建議做什麼？",
    en: "What are the recommended next steps?",
  },
  "ai.chip_missing_dr": {
    "zh-CN": "哪些门店数据覆盖不足？",
    "zh-HK": "哪些門店數據覆蓋不足？",
    en: "Which stores have insufficient fact coverage?",
  },
  "ai.chip_pending": {
    "zh-CN": "有哪些待审批事项？",
    "zh-HK": "有哪些待審批事項？",
    en: "What items are pending approval?",
  },
  "ai.chip_expiring": {
    "zh-CN": "哪些门店需要关注？",
    "zh-HK": "哪些門店需要關注？",
    en: "Which stores need attention?",
  },

  // Reports
  "reports.title": {
    "zh-CN": "报表查询",
    "zh-HK": "報表查詢",
    en: "Reports",
  },
  "reports.mode": {
    "zh-CN": "报表模式",
    "zh-HK": "報表模式",
    en: "Report Mode",
  },
  "reports.working": {
    "zh-CN": "工作报表 (Working)",
    "zh-HK": "工作報表 (Working)",
    en: "Working Report",
  },
  "reports.official": {
    "zh-CN": "正式报表 (Official)",
    "zh-HK": "正式報表 (Official)",
    en: "Official Report",
  },
  "reports.working_alert_title": {
    "zh-CN": "工作报表模式",
    "zh-HK": "工作報表模式",
    en: "Working Report Mode",
  },
  "reports.working_alert_desc": {
    "zh-CN": "包含 Draft、Submitted、Reviewed、Pending Approval 状态的数据。用于内部试算、讨论和预演。",
    "zh-HK": "包含 Draft、Submitted、Reviewed、Pending Approval 狀態的數據。用於內部試算、討論和預演。",
    en: "Includes data with Draft, Submitted, Reviewed, and Pending Approval statuses. For internal trial calculations, discussions, and rehearsals.",
  },
  "reports.official_alert_title": {
    "zh-CN": "正式报表模式",
    "zh-HK": "正式報表模式",
    en: "Official Report Mode",
  },
  "reports.official_alert_desc": {
    "zh-CN": "仅包含 Approved 状态的数据。用于正式财务报告、审计提交和法定披露。",
    "zh-HK": "僅包含 Approved 狀態的數據。用於正式財務報告、審計提交和法定披露。",
    en: "Includes only Approved status data. For formal financial reporting, audit submissions, and statutory disclosures.",
  },
  "reports.tab_ledger": {
    "zh-CN": "合同台账",
    "zh-HK": "合同台賬",
    en: "Contract Ledger",
  },
  "reports.tab_amortization": {
    "zh-CN": "摊销报表",
    "zh-HK": "攤銷報表",
    en: "Amortization Report",
  },
  "reports.total_contracts": {
    "zh-CN": "合同总数",
    "zh-HK": "合同總數",
    en: "Total Contracts",
  },
  "reports.approved": {
    "zh-CN": "已审批",
    "zh-HK": "已審批",
    en: "Approved",
  },
  "reports.draft_pending": {
    "zh-CN": "草稿/待处理",
    "zh-HK": "草稿/待處理",
    en: "Draft / Pending",
  },
  "reports.contract_number": {
    "zh-CN": "合同编号",
    "zh-HK": "合同編號",
    en: "Contract Number",
  },
  "reports.contract_name": {
    "zh-CN": "合同名称",
    "zh-HK": "合同名稱",
    en: "Contract Name",
  },
  "reports.approval_status": {
    "zh-CN": "审批状态",
    "zh-HK": "審批狀態",
    en: "Approval Status",
  },
  "reports.is_official": {
    "zh-CN": "正式版本",
    "zh-HK": "正式版本",
    en: "Official Version",
  },
  "reports.discount_rate_missing": {
    "zh-CN": "折现率缺失",
    "zh-HK": "折現率缺失",
    en: "Discount Rate Missing",
  },
  "reports.currency": {
    "zh-CN": "币种",
    "zh-HK": "幣種",
    en: "Currency",
  },
  "reports.commencement_date": {
    "zh-CN": "起始日",
    "zh-HK": "起始日",
    en: "Start Date",
  },
  "reports.lease_end_date": {
    "zh-CN": "结束日",
    "zh-HK": "結束日",
    en: "End Date",
  },
  "reports.yes": {
    "zh-CN": "是",
    "zh-HK": "是",
    en: "Yes",
  },
  "reports.no": {
    "zh-CN": "否",
    "zh-HK": "否",
    en: "No",
  },
  "reports.missing": {
    "zh-CN": "缺失",
    "zh-HK": "缺失",
    en: "Missing",
  },
  "reports.filled": {
    "zh-CN": "已填",
    "zh-HK": "已填",
    en: "Filled",
  },
  "reports.empty": {
    "zh-CN": "暂无数据",
    "zh-HK": "暫無數據",
    en: "No data",
  },
  "reports.empty_hint": { "zh-CN": "当前查询没有可展示的数据。可以先录入合同，或使用 AI 解析文件。", "zh-HK": "當前查詢沒有可展示的數據。可以先錄入合同，或使用 AI 解析文件。", en: "There is no data for this query. Add a contract or parse a file with AI to get started." },
  "reports.export_csv": {
    "zh-CN": "导出 CSV",
    "zh-HK": "導出 CSV",
    en: "Export CSV",
  },
  "reports.search": {
    "zh-CN": "查询",
    "zh-HK": "查詢",
    en: "Search",
  },
  "reports.reset": {
    "zh-CN": "重置",
    "zh-HK": "重置",
    en: "Reset",
  },
  "reports.ai_analysis": {
    "zh-CN": "AI 分析",
    "zh-HK": "AI 分析",
    en: "AI Analysis",
  },
  "reports.view_dimension": {
    "zh-CN": "视图维度",
    "zh-HK": "視圖維度",
    en: "View Dimension",
  },
  "reports.granularity": {
    "zh-CN": "粒度",
    "zh-HK": "粒度",
    en: "Granularity",
  },
  "reports.date_range": {
    "zh-CN": "日期范围",
    "zh-HK": "日期範圍",
    en: "Date Range",
  },
  "reports.contract_id": {
    "zh-CN": "合同 ID",
    "zh-HK": "合同 ID",
    en: "Contract ID",
  },
  "reports.store": {
    "zh-CN": "门店",
    "zh-HK": "門店",
    en: "Store",
  },
  "reports.tags": {
    "zh-CN": "标签",
    "zh-HK": "標籤",
    en: "Tags",
  },
  "reports.contract_view": {
    "zh-CN": "合同维度",
    "zh-HK": "合同維度",
    en: "Contract View",
  },
  "reports.store_view": {
    "zh-CN": "门店维度",
    "zh-HK": "門店維度",
    en: "Store View",
  },
  "reports.tag_view": {
    "zh-CN": "标签维度",
    "zh-HK": "標籤維度",
    en: "Tag View",
  },
  "reports.summary_view": {
    "zh-CN": "汇总",
    "zh-HK": "匯總",
    en: "Summary",
  },
  "reports.day": {
    "zh-CN": "日",
    "zh-HK": "日",
    en: "Day",
  },
  "reports.month": {
    "zh-CN": "月",
    "zh-HK": "月",
    en: "Month",
  },
  "reports.quarter": {
    "zh-CN": "季",
    "zh-HK": "季",
    en: "Quarter",
  },
  "reports.half_year": {
    "zh-CN": "半年",
    "zh-HK": "半年",
    en: "Half Year",
  },
  "reports.year": {
    "zh-CN": "年",
    "zh-HK": "年",
    en: "Year",
  },
  "reports.expand_filters": {
    "zh-CN": "展开筛选条件 ▼",
    "zh-HK": "展開篩選條件 ▼",
    en: "Expand Filters ▼",
  },
  "reports.collapse_filters": {
    "zh-CN": "收起筛选条件 ▲",
    "zh-HK": "收起篩選條件 ▲",
    en: "Collapse Filters ▲",
  },
  "reports.discount_rate_override": {
    "zh-CN": "折现率覆盖",
    "zh-HK": "折現率覆蓋",
    en: "Discount Rate Override",
  },
  "reports.report_currency": {
    "zh-CN": "报表货币",
    "zh-HK": "報表貨幣",
    en: "Report Currency",
  },
  "reports.exchange_rate": {
    "zh-CN": "汇率",
    "zh-HK": "匯率",
    en: "Exchange Rate",
  },
  "reports.closing_liability": {
    "zh-CN": "期末负债合计",
    "zh-HK": "期末負債合計",
    en: "Total Closing Liability",
  },
  "reports.closing_rou": {
    "zh-CN": "期末使用权资产合计",
    "zh-HK": "期末使用權資產合計",
    en: "Total Closing ROU Asset",
  },
  "reports.total_interest": {
    "zh-CN": "期间利息合计",
    "zh-HK": "期間利息合計",
    en: "Total Interest",
  },
  "reports.total_depreciation": {
    "zh-CN": "期间折旧合计",
    "zh-HK": "期間折舊合計",
    en: "Total Depreciation",
  },
  "reports.tag_caveat": {
    "zh-CN": "同一合同可归入多个标签组，因此标签汇总总额可能大于总计汇总。",
    "zh-HK": "同一合同可歸入多個標籤組，因此標籤匯總總額可能大於總計匯總。",
    en: "A single contract may belong to multiple tag groups, so tag totals may exceed the overall summary.",
  },
  "reports.amortization_table": {
    "zh-CN": "摊销报表",
    "zh-HK": "攤銷報表",
    en: "Amortization Report",
  },
  "reports.no_data_hint": {
    "zh-CN": "请设置查询条件后点击「查询」",
    "zh-HK": "請設置查詢條件後點擊「查詢」",
    en: "Please set query conditions and click Search",
  },
  "reports.query_complete": {
    "zh-CN": "摊销报表查询完成，共 {count} 条",
    "zh-HK": "攤銷報表查詢完成，共 {count} 條",
    en: "Amortization report query complete, {count} rows",
  },
  "reports.query_failed": {
    "zh-CN": "摊销报表查询失败",
    "zh-HK": "攤銷報表查詢失敗",
    en: "Amortization report query failed",
  },
  "reports.please_select_dates": {
    "zh-CN": "请选择开始日期和结束日期",
    "zh-HK": "請選擇開始日期和結束日期",
    en: "Please select start and end dates",
  },

  // Status
  "status.draft": {
    "zh-CN": "草稿",
    "zh-HK": "草稿",
    en: "Draft",
  },
  "status.submitted": {
    "zh-CN": "已提交",
    "zh-HK": "已提交",
    en: "Submitted",
  },
  "status.reviewed": {
    "zh-CN": "已复核",
    "zh-HK": "已覆核",
    en: "Reviewed",
  },
  "status.pending_approval": {
    "zh-CN": "待审批",
    "zh-HK": "待審批",
    en: "Pending Approval",
  },
  "status.approved": {
    "zh-CN": "已审批",
    "zh-HK": "已審批",
    en: "Approved",
  },
  "status.rejected": {
    "zh-CN": "已驳回",
    "zh-HK": "已駁回",
    en: "Rejected",
  },
  "status.returned_to_editor": {
    "zh-CN": "退回编辑",
    "zh-HK": "退回編輯",
    en: "Returned to Editor",
  },

  // Dashboard
  "dashboard.liability_trend": {
    "zh-CN": "租赁负债趋势",
    "zh-HK": "租賃負債趨勢",
    en: "Lease Liability Trend",
  },
  "dashboard.view_reports": {
    "zh-CN": "查看报表",
    "zh-HK": "查看報表",
    en: "View Reports",
  },
  "dashboard.contract_status": {
    "zh-CN": "合同状态分布",
    "zh-HK": "合同狀態分佈",
    en: "Contract Status Distribution",
  },
  "dashboard.recent_contracts": {
    "zh-CN": "最近合同",
    "zh-HK": "最近合同",
    en: "Recent Contracts",
  },
  "dashboard.view_all": {
    "zh-CN": "查看全部",
    "zh-HK": "查看全部",
    en: "View All",
  },
  "dashboard.no_contracts": {
    "zh-CN": "暂无合同数据",
    "zh-HK": "暫無合同數據",
    en: "No contract data",
  },
  "dashboard.no_liability_data": {
    "zh-CN": "暂无租赁负债数据",
    "zh-HK": "暫無租賃負債數據",
    en: "No lease liability data",
  },
  "dashboard.no_status_data": {
    "zh-CN": "暂无合同状态数据",
    "zh-HK": "暫無合同狀態數據",
    en: "No contract status data",
  },
  "dashboard.add_contract": {
    "zh-CN": "新增合同",
    "zh-HK": "新增合同",
    en: "Add Contract",
  },
  "dashboard.upload_file": {
    "zh-CN": "在 AI Chat 上传文件",
    "zh-HK": "在 AI Chat 上傳文件",
    en: "Upload Files in AI Chat",
  },
  "dashboard.view_report": {
    "zh-CN": "查看报表",
    "zh-HK": "查看報表",
    en: "View Reports",
  },
  "dashboard.copies": {
    "zh-CN": "份",
    "zh-HK": "份",
    en: "",
  },

  // Contract List
  "contracts.title": {
    "zh-CN": "合同台账",
    "zh-HK": "合同台賬",
    en: "Contract Ledger",
  },
  "contracts.subtitle": {
    "zh-CN": "共 {count} 份合同",
    "zh-HK": "共 {count} 份合同",
    en: "{count} contracts",
  },
  "contracts.add_contract": {
    "zh-CN": "新增合同",
    "zh-HK": "新增合同",
    en: "Add Contract",
  },
  "contracts.search_placeholder": {
    "zh-CN": "搜索合同编号、名称、承租方、出租方、门店...",
    "zh-HK": "搜索合同編號、名稱、承租方、出租方、門店...",
    en: "Search contract number, name, lessee, lessor, store...",
  },
  "contracts.filter_status": {
    "zh-CN": "筛选状态",
    "zh-HK": "篩選狀態",
    en: "Filter Status",
  },
  "contracts.all_status": {
    "zh-CN": "全部状态",
    "zh-HK": "全部狀態",
    en: "All Status",
  },
  "contracts.filter_risk": { "zh-CN": "风险筛选", "zh-HK": "風險篩選", en: "Risk filter" },
  "contracts.risk_missing_discount_rate": { "zh-CN": "缺折现率", "zh-HK": "缺折現率", en: "Discount rate missing" },
  "contracts.filter_scope": { "zh-CN": "租赁范围", "zh-HK": "租賃範圍", en: "Lease scope" },
  "contracts.filter_asset_type": { "zh-CN": "资产类型", "zh-HK": "資產類型", en: "Asset type" },
  "contracts.filter_expiry": { "zh-CN": "到期区间", "zh-HK": "到期區間", en: "Expiry window" },
  "contracts.expiry_90": { "zh-CN": "90 天内到期", "zh-HK": "90 天內到期", en: "Due within 90 days" },
  "contracts.expiry_180": { "zh-CN": "180 天内到期", "zh-HK": "180 天內到期", en: "Due within 180 days" },
  "contracts.scope_in_scope": { "zh-CN": "资本化", "zh-HK": "資本化", en: "In scope" },
  "contracts.scope_short_term_exempt": { "zh-CN": "短期豁免", "zh-HK": "短期豁免", en: "Short-term exempt" },
  "contracts.scope_low_value_exempt": { "zh-CN": "低价值豁免", "zh-HK": "低價值豁免", en: "Low-value exempt" },
  "contracts.scope_not_a_lease": { "zh-CN": "非租赁", "zh-HK": "非租賃", en: "Not a lease" },
  "contracts.asset_real_estate": { "zh-CN": "不动产", "zh-HK": "不動產", en: "Real estate" },
  "contracts.asset_vehicle": { "zh-CN": "车辆", "zh-HK": "車輛", en: "Vehicle" },
  "contracts.asset_it_equipment": { "zh-CN": "IT 设备", "zh-HK": "IT 設備", en: "IT equipment" },
  "contracts.asset_machinery": { "zh-CN": "机器设备", "zh-HK": "機器設備", en: "Machinery" },
  "contracts.asset_other": { "zh-CN": "其他", "zh-HK": "其他", en: "Other" },
  "contracts.open": { "zh-CN": "打开合同", "zh-HK": "打開合同", en: "Open contract" },
  "contracts.col_name": {
    "zh-CN": "合同名称",
    "zh-HK": "合同名稱",
    en: "Contract Name",
  },
  "contracts.col_currency": {
    "zh-CN": "币种",
    "zh-HK": "幣種",
    en: "Currency",
  },
  "contracts.col_start_date": {
    "zh-CN": "租赁起始日",
    "zh-HK": "租賃起始日",
    en: "Lease Start Date",
  },
  "contracts.col_end_date": {
    "zh-CN": "租期结束日",
    "zh-HK": "租期結束日",
    en: "Lease End Date",
  },
  "contracts.col_status": {
    "zh-CN": "审批状态",
    "zh-HK": "審批狀態",
    en: "Approval Status",
  },
  "contracts.official": {
    "zh-CN": "正式",
    "zh-HK": "正式",
    en: "Official",
  },
  "contracts.working": {
    "zh-CN": "工作版",
    "zh-HK": "工作版",
    en: "Working",
  },
  "contracts.missing": {
    "zh-CN": "缺失",
    "zh-HK": "缺失",
    en: "Missing",
  },
  "contracts.total_items": {
    "zh-CN": "共 {total} 条",
    "zh-HK": "共 {total} 條",
    en: "{total} items",
  },
  "contracts.no_data": {
    "zh-CN": "暂无合同数据",
    "zh-HK": "暫無合同數據",
    en: "No contract data",
  },
  "contracts.no_search_results": {
    "zh-CN": "未找到匹配的合同",
    "zh-HK": "未找到匹配的合同",
    en: "No matching contracts found",
  },
  "contracts.clear_filters": { "zh-CN": "清除筛选", "zh-HK": "清除篩選", en: "Clear filters" },
  "contracts.load_failed": {
    "zh-CN": "加载合同失败",
    "zh-HK": "加載合同失敗",
    en: "Failed to load contracts",
  },

  // Contract Detail
  "contract.back_to_list": {
    "zh-CN": "返回列表",
    "zh-HK": "返回列表",
    en: "Back to List",
  },
  "contract.detail_title": {
    "zh-CN": "合同详情",
    "zh-HK": "合同詳情",
    en: "Contract Detail",
  },
  "contract.tab_info": {
    "zh-CN": "合同信息",
    "zh-HK": "合同信息",
    en: "Contract Info",
  },
  "contract.tab_payments": {
    "zh-CN": "付款计划",
    "zh-HK": "付款計劃",
    en: "Payment Schedule",
  },
  "contract.tab_events": {
    "zh-CN": "变更事件",
    "zh-HK": "變更事件",
    en: "Change Events",
  },
  "contract.tab_calculation": {
    "zh-CN": "IFRS 16 计算",
    "zh-HK": "IFRS 16 計算",
    en: "IFRS 16 Calculation",
  },
  "contract.submit_review": {
    "zh-CN": "提交复核",
    "zh-HK": "提交覆核",
    en: "Submit for Review",
  },
  "contract.review_pass": {
    "zh-CN": "复核通过",
    "zh-HK": "覆核通過",
    en: "Review Pass",
  },
  "contract.return_editor": {
    "zh-CN": "退回编辑",
    "zh-HK": "退回編輯",
    en: "Return to Editor",
  },
  "contract.approve": {
    "zh-CN": "审批通过",
    "zh-HK": "審批通過",
    en: "Approve",
  },
  "contract.reject": {
    "zh-CN": "驳回",
    "zh-HK": "駁回",
    en: "Reject",
  },
  "contract.resubmit": {
    "zh-CN": "重新提交",
    "zh-HK": "重新提交",
    en: "Resubmit",
  },
  "contract.edit": {
    "zh-CN": "编辑",
    "zh-HK": "編輯",
    en: "Edit",
  },
  "contract.calculate": {
    "zh-CN": "IFRS 16 计算",
    "zh-HK": "IFRS 16 計算",
    en: "IFRS 16 Calculate",
  },
  "contract.contract_number": {
    "zh-CN": "合同编号",
    "zh-HK": "合同編號",
    en: "Contract Number",
  },
  "contract.currency": {
    "zh-CN": "币种",
    "zh-HK": "幣種",
    en: "Currency",
  },
  "contract.discount_rate": {
    "zh-CN": "折现率",
    "zh-HK": "折現率",
    en: "Discount Rate",
  },
  "contract.commencement_date": {
    "zh-CN": "租赁起始日",
    "zh-HK": "租賃起始日",
    en: "Commencement Date",
  },
  "contract.lease_start_date": {
    "zh-CN": "租赁开始日",
    "zh-HK": "租賃開始日",
    en: "Lease Start Date",
  },
  "contract.lease_end_date": {
    "zh-CN": "租赁结束日",
    "zh-HK": "租賃結束日",
    en: "Lease End Date",
  },
  "contract.approval_progress": {
    "zh-CN": "审批进度",
    "zh-HK": "審批進度",
    en: "Approval Progress",
  },
  "contract.created": {
    "zh-CN": "创建",
    "zh-HK": "創建",
    en: "Created",
  },
  "contract.submitted": {
    "zh-CN": "提交",
    "zh-HK": "提交",
    en: "Submitted",
  },
  "contract.reviewed": {
    "zh-CN": "复核",
    "zh-HK": "覆核",
    en: "Reviewed",
  },
  "contract.approved": {
    "zh-CN": "审批",
    "zh-HK": "審批",
    en: "Approved",
  },
  "contract.pending": {
    "zh-CN": "待",
    "zh-HK": "待",
    en: "Pending",
  },
  "contract.add_payment": {
    "zh-CN": "添加付款计划",
    "zh-HK": "添加付款計劃",
    en: "Add Payment Schedule",
  },
  "contract.ai_agent_intake": {
    "zh-CN": "在 AI Agent 中处理",
    "zh-HK": "在 AI Agent 中處理",
    en: "Open in AI Agent",
  },
  "contract.manual_add": {
    "zh-CN": "手动添加",
    "zh-HK": "手動添加",
    en: "Manual Add",
  },
  "contract.register_event": {
    "zh-CN": "登记事件",
    "zh-HK": "登記事件",
    en: "Register Event",
  },
  "contract.view_adjustment": {
    "zh-CN": "查看调整",
    "zh-HK": "查看調整",
    en: "View Adjustment",
  },
  "contract.preview_impact": {
    "zh-CN": "预览影响",
    "zh-HK": "預覽影響",
    en: "Preview Impact",
  },
  "contract.recalculate": {
    "zh-CN": "重算",
    "zh-HK": "重算",
    en: "Recalculate",
  },
  "contract.event_type.area_adjustment": {
    "zh-CN": "面积调整",
    "zh-HK": "面積調整",
    en: "Area Adjustment",
  },
  "contract.event_type.rent_change": {
    "zh-CN": "租金变更",
    "zh-HK": "租金變更",
    en: "Rent Change",
  },
  "contract.event_type.renewal": {
    "zh-CN": "续租",
    "zh-HK": "續租",
    en: "Renewal",
  },
  "contract.event_type.early_termination": {
    "zh-CN": "提前终止",
    "zh-HK": "提前終止",
    en: "Early Termination",
  },
  "contract.event_type.index_update": {
    "zh-CN": "指数更新",
    "zh-HK": "指數更新",
    en: "Index Update",
  },
  "contract.event_type.discount_rate_change": {
    "zh-CN": "折现率变更",
    "zh-HK": "折現率變更",
    en: "Discount Rate Change",
  },
  "contract.event_type.impairment": {
    "zh-CN": "减值",
    "zh-HK": "減值",
    en: "Impairment",
  },
  "contract.payment_date": {
    "zh-CN": "付款日",
    "zh-HK": "付款日",
    en: "Payment Date",
  },
  "contract.amount": {
    "zh-CN": "金额",
    "zh-HK": "金額",
    en: "Amount",
  },
  "contract.payment_timing": {
    "zh-CN": "时点",
    "zh-HK": "時點",
    en: "Timing",
  },
  "contract.prepaid": {
    "zh-CN": "先付",
    "zh-HK": "先付",
    en: "Prepaid",
  },
  "contract.postpaid": {
    "zh-CN": "后付",
    "zh-HK": "後付",
    en: "Postpaid",
  },
  "contract.amount_type": {
    "zh-CN": "类型",
    "zh-HK": "類型",
    en: "Type",
  },
  "contract.fixed": {
    "zh-CN": "固定",
    "zh-HK": "固定",
    en: "Fixed",
  },
  "contract.variable": {
    "zh-CN": "变量",
    "zh-HK": "變量",
    en: "Variable",
  },
  "contract.lease_component": {
    "zh-CN": "租赁成分",
    "zh-HK": "租賃成分",
    en: "Lease Component",
  },
  "contract.include_liability": {
    "zh-CN": "计入负债",
    "zh-HK": "計入負債",
    en: "Include in Liability",
  },
  "contract.yes": {
    "zh-CN": "是",
    "zh-HK": "是",
    en: "Yes",
  },
  "contract.no": {
    "zh-CN": "否",
    "zh-HK": "否",
    en: "No",
  },
  "contract.confidence": {
    "zh-CN": "置信度",
    "zh-HK": "置信度",
    en: "Confidence",
  },
  "contract.action": {
    "zh-CN": "操作",
    "zh-HK": "操作",
    en: "Action",
  },
  "contract.no_events": {
    "zh-CN": "暂无事件记录",
    "zh-HK": "暫無事件記錄",
    en: "No event records",
  },
  "contract.no_schedules": {
    "zh-CN": "暂无付款计划",
    "zh-HK": "暫無付款計劃",
    en: "No payment schedules",
  },
  "contract.initial_liability": {
    "zh-CN": "初始租赁负债",
    "zh-HK": "初始租賃負債",
    en: "Initial Lease Liability",
  },
  "contract.initial_rou": {
    "zh-CN": "初始使用权资产",
    "zh-HK": "初始使用權資產",
    en: "Initial ROU Asset",
  },
  "contract.total_days": {
    "zh-CN": "租赁总天数",
    "zh-HK": "租賃總天數",
    en: "Total Lease Days",
  },
  "contract.monthly_amortization": {
    "zh-CN": "月度摊销表",
    "zh-HK": "月度攤銷表",
    en: "Monthly Amortization",
  },
  "contract.click_calculate": {
    "zh-CN": "点击上方「IFRS 16 计算」按钮生成摊销表",
    "zh-HK": "點擊上方「IFRS 16 計算」按鈕生成攤銷表",
    en: "Click the IFRS 16 Calculate button above to generate the amortization table",
  },
  "contract.period": {
    "zh-CN": "期间",
    "zh-HK": "期間",
    en: "Period",
  },
  "contract.opening_liability": {
    "zh-CN": "期初负债",
    "zh-HK": "期初負債",
    en: "Opening Liability",
  },
  "contract.interest_expense": {
    "zh-CN": "利息费用",
    "zh-HK": "利息費用",
    en: "Interest Expense",
  },
  "contract.payment": {
    "zh-CN": "付款",
    "zh-HK": "付款",
    en: "Payment",
  },
  "contract.closing_liability": {
    "zh-CN": "期末负债",
    "zh-HK": "期末負債",
    en: "Closing Liability",
  },
  "contract.opening_rou": {
    "zh-CN": "期初ROU",
    "zh-HK": "期初ROU",
    en: "Opening ROU",
  },
  "contract.depreciation": {
    "zh-CN": "折旧",
    "zh-HK": "折舊",
    en: "Depreciation",
  },
  "contract.closing_rou": {
    "zh-CN": "期末ROU",
    "zh-HK": "期末ROU",
    en: "Closing ROU",
  },
  "contract.variable_rent": {
    "zh-CN": "变量租金",
    "zh-HK": "變量租金",
    en: "Variable Rent",
  },
  "contract.non_lease_expense": {
    "zh-CN": "非租赁费用",
    "zh-HK": "非租賃費用",
    en: "Non-lease Expense",
  },
  "contract.ok": {
    "zh-CN": "确认",
    "zh-HK": "確認",
    en: "OK",
  },
  "contract.cancel": {
    "zh-CN": "取消",
    "zh-HK": "取消",
    en: "Cancel",
  },
  "contract.close": {
    "zh-CN": "关闭",
    "zh-HK": "關閉",
    en: "Close",
  },
  "contract.edit_contract": {
    "zh-CN": "编辑合同",
    "zh-HK": "編輯合同",
    en: "Edit Contract",
  },
  "contract.lessee": {
    "zh-CN": "承租方",
    "zh-HK": "承租方",
    en: "Lessee",
  },
  "contract.lessor": {
    "zh-CN": "出租方",
    "zh-HK": "出租方",
    en: "Lessor",
  },
  "contract.store_name": {
    "zh-CN": "门店名称",
    "zh-HK": "門店名稱",
    en: "Store Name",
  },
  "contract.store_address": {
    "zh-CN": "门店地址",
    "zh-HK": "門店地址",
    en: "Store Address",
  },
  "contract.signing_date": {
    "zh-CN": "签约日期",
    "zh-HK": "簽約日期",
    en: "Signing Date",
  },
  "contract.discount_rate_type": {
    "zh-CN": "折现率类型",
    "zh-HK": "折現率類型",
    en: "Discount Rate Type",
  },
  "contract.discount_rate_version": {
    "zh-CN": "折现率版本",
    "zh-HK": "折現率版本",
    en: "Discount Rate Version",
  },
  "contract.tags": {
    "zh-CN": "标签",
    "zh-HK": "標籤",
    en: "Tags",
  },
  "contract.contract_id": {
    "zh-CN": "合同ID",
    "zh-HK": "合同ID",
    en: "Contract ID",
  },
  "contract.legal_entity_id": {
    "zh-CN": "法人主体ID",
    "zh-HK": "法人主體ID",
    en: "Legal Entity ID",
  },
  "contract.store_id": {
    "zh-CN": "门店ID",
    "zh-HK": "門店ID",
    en: "Store ID",
  },
  "contract.landlord_id": {
    "zh-CN": "出租方ID",
    "zh-HK": "出租方ID",
    en: "Landlord ID",
  },
  "contract.created_at": {
    "zh-CN": "创建时间",
    "zh-HK": "創建時間",
    en: "Created At",
  },
  "contract.rejected_reason": {
    "zh-CN": "驳回原因",
    "zh-HK": "駁回原因",
    en: "Rejected Reason",
  },
  "contract.effective_date": {
    "zh-CN": "生效日期",
    "zh-HK": "生效日期",
    en: "Effective Date",
  },
  "contract.original_value": {
    "zh-CN": "原值",
    "zh-HK": "原值",
    en: "Original Value",
  },
  "contract.new_value": {
    "zh-CN": "新值",
    "zh-HK": "新值",
    en: "New Value",
  },
  "contract.change_reason": {
    "zh-CN": "变更原因",
    "zh-HK": "變更原因",
    en: "Change Reason",
  },
  "contract.judgment_basis": {
    "zh-CN": "判断依据",
    "zh-HK": "判斷依據",
    en: "Judgment Basis",
  },
  "contract.change_reason_placeholder": {
    "zh-CN": "例如：根据合同补充协议第3条，租金调整为每月12万元",
    "zh-HK": "例如：根據合同補充協議第3條，租金調整為每月12萬元",
    en: "e.g. According to contract supplement clause 3, rent adjusted to 120,000 per month",
  },
  "contract.judgment_basis_placeholder": {
    "zh-CN": "例如：合同补充协议签署日期 2024-06-15",
    "zh-HK": "例如：合同補充協議簽署日期 2024-06-15",
    en: "e.g. Contract supplement signed on 2024-06-15",
  },
  "contract.select_event_type": {
    "zh-CN": "选择事件类型",
    "zh-HK": "選擇事件類型",
    en: "Select Event Type",
  },
  "contract.review_reject_title": {
    "zh-CN": "退回编辑",
    "zh-HK": "退回編輯",
    en: "Return to Editor",
  },
  "contract.approve_reject_title": {
    "zh-CN": "驳回",
    "zh-HK": "駁回",
    en: "Reject",
  },
  "contract.review_reject_reason": {
    "zh-CN": "请输入退回编辑的原因：",
    "zh-HK": "請輸入退回編輯的原因：",
    en: "Please enter reason for returning to editor:",
  },
  "contract.approve_reject_reason": {
    "zh-CN": "请输入驳回的原因：",
    "zh-HK": "請輸入駁回的原因：",
    en: "Please enter reason for rejection:",
  },
  "contract.reason_placeholder": {
    "zh-CN": "请输入原因...",
    "zh-HK": "請輸入原因...",
    en: "Enter reason...",
  },
  "contract.amount_type_placeholder": {
    "zh-CN": "例如：fixed_rent, turnover_rent, CAM",
    "zh-HK": "例如：fixed_rent, turnover_rent, CAM",
    en: "e.g. fixed_rent, turnover_rent, CAM",
  },
  "contract.is_fixed": {
    "zh-CN": "固定租金",
    "zh-HK": "固定租金",
    en: "Fixed Rent",
  },
  "contract.is_lease_component": {
    "zh-CN": "租赁成分",
    "zh-HK": "租賃成分",
    en: "Lease Component",
  },
  "contract.included_in_liability_pv": {
    "zh-CN": "计入负债",
    "zh-HK": "計入負債",
    en: "Include in Liability PV",
  },
  "contract.payment_timing_label": {
    "zh-CN": "付款时点",
    "zh-HK": "付款時點",
    en: "Payment Timing",
  },
  "contract.prepaid_label": {
    "zh-CN": "先付 (Prepaid)",
    "zh-HK": "先付 (Prepaid)",
    en: "Prepaid",
  },
  "contract.postpaid_label": {
    "zh-CN": "后付 (Postpaid)",
    "zh-HK": "後付 (Postpaid)",
    en: "Postpaid",
  },
  "contract.effective_start_date": {
    "zh-CN": "生效开始日",
    "zh-HK": "生效開始日",
    en: "Effective Start Date",
  },
  "contract.effective_end_date": {
    "zh-CN": "生效结束日",
    "zh-HK": "生效結束日",
    en: "Effective End Date",
  },
  "contract.lease_end_date_label": {
    "zh-CN": "租期结束日",
    "zh-HK": "租期結束日",
    en: "Lease End Date",
  },
  "contract.discount_rate_type_placeholder": {
    "zh-CN": "例如：incremental_borrowing_rate",
    "zh-HK": "例如：incremental_borrowing_rate",
    en: "e.g. incremental_borrowing_rate",
  },
  "contract.discount_rate_value": {
    "zh-CN": "折现率数值 (%)",
    "zh-HK": "折現率數值 (%)",
    en: "Discount Rate Value (%)",
  },
  "contract.discount_rate_help": {
    "zh-CN": "可直接填写年化折现率，填写 5 会自动按 5% 处理。",
    "zh-HK": "可直接填寫年化折現率，填寫 5 會自動按 5% 處理。",
    en: "Enter annual discount rate directly, e.g. 5 will be treated as 5%.",
  },
  "contract.discount_rate_version_placeholder": {
    "zh-CN": "例如：v1.0",
    "zh-HK": "例如：v1.0",
    en: "e.g. v1.0",
  },
  "contract.tags_tooltip": {
    "zh-CN": "用于报表按标签汇总，例如 #华东 #直营 #旗舰店",
    "zh-HK": "用於報表按標籤匯總，例如 #華東 #直營 #旗艦店",
    en: "Used for report grouping, e.g. #EastChina #Direct #Flagship",
  },
  "contract.tags_placeholder": {
    "zh-CN": "输入标签后回车，例如 #华东、#直营、#旗舰店",
    "zh-HK": "輸入標籤後回車，例如 #華東、#直營、#旗艦店",
    en: "Enter tags and press Enter, e.g. #EastChina, #Direct, #Flagship",
  },

  // Monthly Closing
  "monthly.title": {
    "zh-CN": "结账中心",
    "zh-HK": "結賬中心",
    en: "Monthly Closing",
  },
  "monthly.current_period": {
    "zh-CN": "当前期间",
    "zh-HK": "當前期間",
    en: "Current Period",
  },
  "monthly.status_summary": {
    "zh-CN": "草稿 {draftCount} / 已审批 {approvedCount} / 已过账 {postedCount}",
    "zh-HK": "草稿 {draftCount} / 已審批 {approvedCount} / 已過賬 {postedCount}",
    en: "Draft {draftCount} / Approved {approvedCount} / Posted {postedCount}",
  },
  "monthly.select_period": {
    "zh-CN": "请选择期间",
    "zh-HK": "請選擇期間",
    en: "Please select period",
  },
  "monthly.generate_success": {
    "zh-CN": "结账生成完成：处理 {count} 份合同",
    "zh-HK": "結賬生成完成：處理 {count} 份合同",
    en: "Closing generated: processed {count} contracts",
  },
  "monthly.generate_failed": {
    "zh-CN": "生成失败",
    "zh-HK": "生成失敗",
    en: "Generation failed",
  },
  "monthly.load_entries_failed": {
    "zh-CN": "加载分录失败",
    "zh-HK": "加載分錄失敗",
    en: "Failed to load entries",
  },
  "monthly.approve_success": {
    "zh-CN": "分录审批成功",
    "zh-HK": "分錄審批成功",
    en: "Entry approved",
  },
  "monthly.approve_failed": {
    "zh-CN": "审批失败",
    "zh-HK": "審批失敗",
    en: "Approval failed",
  },
  "monthly.post_success": {
    "zh-CN": "分录过账成功",
    "zh-HK": "分錄過賬成功",
    en: "Entry posted",
  },
  "monthly.post_failed": {
    "zh-CN": "过账失败",
    "zh-HK": "過賬失敗",
    en: "Posting failed",
  },
  "monthly.batch_approve_success": {
    "zh-CN": "批次审批成功：{count} 笔分录已审批",
    "zh-HK": "批次審批成功：{count} 筆分錄已審批",
    en: "Batch approved: {count} entries",
  },
  "monthly.batch_approve_failed": {
    "zh-CN": "批次审批失败",
    "zh-HK": "批次審批失敗",
    en: "Batch approval failed",
  },
  "monthly.batch_post_success": {
    "zh-CN": "批次过账成功：{count} 笔分录已过账",
    "zh-HK": "批次過賬成功：{count} 筆分錄已過賬",
    en: "Batch posted: {count} entries",
  },
  "monthly.batch_post_failed": {
    "zh-CN": "批次过账失败",
    "zh-HK": "批次過賬失敗",
    en: "Batch posting failed",
  },
  "monthly.lock_success": {
    "zh-CN": "期间 {period} 已锁账",
    "zh-HK": "期間 {period} 已鎖賬",
    en: "Period {period} locked",
  },
  "monthly.lock_failed": {
    "zh-CN": "锁账失败",
    "zh-HK": "鎖賬失敗",
    en: "Lock failed",
  },
  "monthly.unlock_success": {
    "zh-CN": "期间 {period} 已解锁",
    "zh-HK": "期間 {period} 已解鎖",
    en: "Period {period} unlocked",
  },
  "monthly.unlock_failed": {
    "zh-CN": "解锁失败",
    "zh-HK": "解鎖失敗",
    en: "Unlock failed",
  },
  "monthly.no_period": {
    "zh-CN": "请先生成结账结果",
    "zh-HK": "請先生成結賬結果",
    en: "Please generate closing results first",
  },
  "monthly.tab_generate": {
    "zh-CN": "生成结账",
    "zh-HK": "生成結賬",
    en: "Generate",
  },
  "monthly.tab_entries": {
    "zh-CN": "分录预览",
    "zh-HK": "分錄預覽",
    en: "Entries",
  },
  "monthly.tab_batches": {
    "zh-CN": "批次历史",
    "zh-HK": "批次歷史",
    en: "Batches",
  },
  "monthly.tab_lock": {
    "zh-CN": "锁账控制",
    "zh-HK": "鎖賬控制",
    en: "Lock Control",
  },
  "monthly.generate_closing": {
    "zh-CN": "生成结账分录",
    "zh-HK": "生成結賬分錄",
    en: "Generate Closing Entries",
  },
  "monthly.accounting_period": {
    "zh-CN": "会计期间",
    "zh-HK": "會計期間",
    en: "Accounting Period",
  },
  "monthly.discount_rate": {
    "zh-CN": "折现率",
    "zh-HK": "折現率",
    en: "Discount Rate",
  },
  "monthly.generate_btn": {
    "zh-CN": "生成结账分录",
    "zh-HK": "生成結賬分錄",
    en: "Generate",
  },
  "monthly.result_title": {
    "zh-CN": "结账生成结果",
    "zh-HK": "結賬生成結果",
    en: "Closing Result",
  },
  "monthly.batch_number": {
    "zh-CN": "批次号",
    "zh-HK": "批次號",
    en: "Batch Number",
  },
  "monthly.status": {
    "zh-CN": "状态",
    "zh-HK": "狀態",
    en: "Status",
  },
  "monthly.processed_contracts": {
    "zh-CN": "处理合同",
    "zh-HK": "處理合同",
    en: "Processed Contracts",
  },
  "monthly.failed_contracts": {
    "zh-CN": "失败",
    "zh-HK": "失敗",
    en: "Failed",
  },
  "monthly.total_entries": {
    "zh-CN": "生成分录",
    "zh-HK": "生成分錄",
    en: "Total Entries",
  },
  "monthly.entries_preview": {
    "zh-CN": "会计分录预览",
    "zh-HK": "會計分錄預覽",
    en: "Accounting Entries Preview",
  },
  "monthly.batch_approve": {
    "zh-CN": "批量审批",
    "zh-HK": "批量審批",
    en: "Batch Approve",
  },
  "monthly.batch_post": {
    "zh-CN": "批量过账",
    "zh-HK": "批量過賬",
    en: "Batch Post",
  },
  "monthly.refresh": {
    "zh-CN": "刷新",
    "zh-HK": "刷新",
    en: "Refresh",
  },
  "monthly.locked_warning": {
    "zh-CN": "期间 {period} 已锁账，分录不可修改",
    "zh-HK": "期間 {period} 已鎖賬，分錄不可修改",
    en: "Period {period} is locked, entries cannot be modified",
  },
  "monthly.no_entries": {
    "zh-CN": "暂无分录，请先生成结账",
    "zh-HK": "暫無分錄，請先生成結賬",
    en: "No entries, please generate closing first",
  },
  "monthly.batch_history": {
    "zh-CN": "结账批次历史",
    "zh-HK": "結賬批次歷史",
    en: "Closing Batch History",
  },
  "monthly.no_batches": {
    "zh-CN": "暂无批次记录，请先生成结账",
    "zh-HK": "暫無批次記錄，請先生成結賬",
    en: "No batch records, please generate closing first",
  },
  "monthly.lock_control": {
    "zh-CN": "期间锁账控制",
    "zh-HK": "期間鎖賬控制",
    en: "Period Lock Control",
  },
  "monthly.lock_first": {
    "zh-CN": "请先生成结账后再进行锁账操作",
    "zh-HK": "請先生成結賬後再進行鎖賬操作",
    en: "Please generate closing before locking",
  },
  "monthly.accounting_period_label": {
    "zh-CN": "会计期间",
    "zh-HK": "會計期間",
    en: "Accounting Period",
  },
  "monthly.locked": {
    "zh-CN": "已锁账",
    "zh-HK": "已鎖賬",
    en: "Locked",
  },
  "monthly.unlocked": {
    "zh-CN": "未锁账",
    "zh-HK": "未鎖賬",
    en: "Unlocked",
  },
  "monthly.lock_desc_locked": {
    "zh-CN": "该期间已锁账，分录不可修改，重新生成需要先解锁",
    "zh-HK": "該期間已鎖賬，分錄不可修改，重新生成需要先解鎖",
    en: "This period is locked. Entries cannot be modified. Unlock to regenerate.",
  },
  "monthly.lock_desc_unlocked": {
    "zh-CN": "该期间未锁账，可以正常生成或修改结账分录",
    "zh-HK": "該期間未鎖賬，可以正常生成或修改結賬分錄",
    en: "This period is unlocked. You can generate or modify closing entries.",
  },
  "monthly.lock_btn": {
    "zh-CN": "锁账",
    "zh-HK": "鎖賬",
    en: "Lock",
  },
  "monthly.lock_btn_disabled": {
    "zh-CN": "锁账（仅审批员/管理员可操作）",
    "zh-HK": "鎖賬（僅審批員/管理員可操作）",
    en: "Lock (Approver/Admin only)",
  },
  "monthly.unlock_btn": {
    "zh-CN": "解锁",
    "zh-HK": "解鎖",
    en: "Unlock",
  },
  "monthly.unlock_btn_disabled": {
    "zh-CN": "解锁（仅管理员可操作）",
    "zh-HK": "解鎖（僅管理員可操作）",
    en: "Unlock (Admin only)",
  },
  "monthly.refresh_status": {
    "zh-CN": "刷新状态",
    "zh-HK": "刷新狀態",
    en: "Refresh Status",
  },
  "monthly.contact_admin": {
    "zh-CN": "如需解锁，请联系管理员",
    "zh-HK": "如需解鎖，請聯繫管理員",
    en: "Contact admin to unlock",
  },
  "monthly.posting_confirm": {
    "zh-CN": "过账确认",
    "zh-HK": "過賬確認",
    en: "Posting Confirmation",
  },
  "monthly.posting_confirm_desc": {
    "zh-CN": "确认将以下分录过账？",
    "zh-HK": "確認將以下分錄過賬？",
    en: "Confirm posting the following entry?",
  },
  "monthly.entry_type": {
    "zh-CN": "分录类型",
    "zh-HK": "分錄類型",
    en: "Entry Type",
  },
  "monthly.amount": {
    "zh-CN": "金额",
    "zh-HK": "金額",
    en: "Amount",
  },
  "monthly.description": {
    "zh-CN": "描述",
    "zh-HK": "描述",
    en: "Description",
  },
  "monthly.erp_reference": {
    "zh-CN": "ERP 凭证号（可选）",
    "zh-HK": "ERP 憑證號（可選）",
    en: "ERP Reference (Optional)",
  },
  "monthly.erp_placeholder": {
    "zh-CN": "输入 ERP 凭证号",
    "zh-HK": "輸入 ERP 憑證號",
    en: "Enter ERP reference",
  },
  "monthly.export_erp_csv": {
    "zh-CN": "导出 ERP CSV",
    "zh-HK": "導出 ERP CSV",
    en: "Export ERP CSV",
  },
  "monthly.export_template_placeholder": {
    "zh-CN": "ERP 模板标识",
    "zh-HK": "ERP 模板標識",
    en: "ERP template identifier",
  },
  "monthly.export_template_required": {
    "zh-CN": "请先填写 ERP 导出模板标识",
    "zh-HK": "請先填寫 ERP 導出模板標識",
    en: "Enter an ERP export template identifier first",
  },
  "monthly.ok": {
    "zh-CN": "确认",
    "zh-HK": "確認",
    en: "OK",
  },
  "monthly.cancel": {
    "zh-CN": "取消",
    "zh-HK": "取消",
    en: "Cancel",
  },
  "monthly.col_period": {
    "zh-CN": "期间",
    "zh-HK": "期間",
    en: "Period",
  },
  "monthly.col_entry_type": {
    "zh-CN": "分录类型",
    "zh-HK": "分錄類型",
    en: "Entry Type",
  },
  "monthly.col_debit_account": {
    "zh-CN": "借方科目",
    "zh-HK": "借方科目",
    en: "Debit Account",
  },
  "monthly.col_credit_account": {
    "zh-CN": "贷方科目",
    "zh-HK": "貸方科目",
    en: "Credit Account",
  },
  "monthly.col_amount": {
    "zh-CN": "金额",
    "zh-HK": "金額",
    en: "Amount",
  },
  "monthly.col_currency": {
    "zh-CN": "币种",
    "zh-HK": "幣種",
    en: "Currency",
  },
  "monthly.col_description": {
    "zh-CN": "描述",
    "zh-HK": "描述",
    en: "Description",
  },
  "monthly.col_status": {
    "zh-CN": "状态",
    "zh-HK": "狀態",
    en: "Status",
  },
  "monthly.col_actions": {
    "zh-CN": "操作",
    "zh-HK": "操作",
    en: "Actions",
  },
  "monthly.entry_interest": {
    "zh-CN": "利息",
    "zh-HK": "利息",
    en: "Interest",
  },
  "monthly.entry_depreciation": {
    "zh-CN": "折旧",
    "zh-HK": "折舊",
    en: "Depreciation",
  },
  "monthly.entry_payment": {
    "zh-CN": "付款",
    "zh-HK": "付款",
    en: "Payment",
  },
  "monthly.status_posted": {
    "zh-CN": "已过账",
    "zh-HK": "已過賬",
    en: "Posted",
  },
  "monthly.status_approved": {
    "zh-CN": "已审批",
    "zh-HK": "已審批",
    en: "Approved",
  },
  "monthly.status_draft": {
    "zh-CN": "草稿",
    "zh-HK": "草稿",
    en: "Draft",
  },
  "monthly.approve_entry": {
    "zh-CN": "审批",
    "zh-HK": "審批",
    en: "Approve",
  },
  "monthly.post_entry": {
    "zh-CN": "过账",
    "zh-HK": "過賬",
    en: "Post",
  },
  "monthly.reject_entry": {
    "zh-CN": "驳回",
    "zh-HK": "駁回",
    en: "Reject",
  },
  "monthly.reverse_entry": {
    "zh-CN": "冲销",
    "zh-HK": "沖銷",
    en: "Reverse",
  },
  "monthly.approve_confirm": {
    "zh-CN": "确认审批该分录？",
    "zh-HK": "確認審批該分錄？",
    en: "Confirm approving this entry?",
  },
  "monthly.reverse_title": {
    "zh-CN": "红冲已过账分录",
    "zh-HK": "紅沖已過賬分錄",
    en: "Reverse Posted Entry",
  },
  "monthly.reverse_desc": {
    "zh-CN": "红冲不会修改或删除原分录:系统会生成一笔借贷方向相反的分录并与原分录关联,原分录标记为已红冲。已锁定的期间不会被改动。",
    "zh-HK": "紅沖不會修改或刪除原分錄:系統會生成一筆借貸方向相反的分錄並與原分錄關聯,原分錄標記為已紅沖。已鎖定的期間不會被改動。",
    en: "Reversal never edits or deletes the original: the system posts an opposite entry linked to it and marks the original reversed. Locked periods stay untouched.",
  },
  "monthly.reverse_direction": {
    "zh-CN": "红冲方向",
    "zh-HK": "紅沖方向",
    en: "Reversal Direction",
  },
  "monthly.reverse_direction_value": {
    "zh-CN": "借:{debit} / 贷:{credit}",
    "zh-HK": "借:{debit} / 貸:{credit}",
    en: "Dr {debit} / Cr {credit}",
  },
  "monthly.reverse_reason": {
    "zh-CN": "红冲原因",
    "zh-HK": "紅沖原因",
    en: "Reversal Reason",
  },
  "monthly.reverse_reason_placeholder": {
    "zh-CN": "例如:科目录错、金额有误、合同条款更正",
    "zh-HK": "例如:科目錄錯、金額有誤、合同條款更正",
    en: "e.g. wrong account, incorrect amount, corrected lease terms",
  },
  "monthly.reverse_reason_required": {
    "zh-CN": "请填写红冲原因",
    "zh-HK": "請填寫紅沖原因",
    en: "A reversal reason is required",
  },
  "monthly.reverse_period": {
    "zh-CN": "红冲入账期间(可选)",
    "zh-HK": "紅沖入賬期間(可選)",
    en: "Reversal Period (optional)",
  },
  "monthly.reverse_period_hint": {
    "zh-CN": "留空则记入原分录所属期间。若该期间已锁账,请填写一个未锁定的期间(YYYY-MM)。",
    "zh-HK": "留空則記入原分錄所屬期間。若該期間已鎖賬,請填寫一個未鎖定的期間(YYYY-MM)。",
    en: "Leave blank to use the original entry's period. If that period is locked, enter an open period (YYYY-MM).",
  },
  "monthly.reverse_success": {
    "zh-CN": "红冲成功",
    "zh-HK": "紅沖成功",
    en: "Entry reversed",
  },
  "monthly.reverse_failed": {
    "zh-CN": "红冲失败",
    "zh-HK": "紅沖失敗",
    en: "Reversal failed",
  },
  "monthly.status_reversed": {
    "zh-CN": "已红冲",
    "zh-HK": "已紅沖",
    en: "Reversed",
  },
  "monthly.is_reversal_entry": {
    "zh-CN": "红冲分录",
    "zh-HK": "紅沖分錄",
    en: "Reversal",
  },
  "monthly.reject_title": {
    "zh-CN": "驳回已审批分录",
    "zh-HK": "駁回已審批分錄",
    en: "Reject Approved Entry",
  },
  "monthly.reject_desc": {
    "zh-CN": "驳回后分录状态回到草稿,原审批痕迹被清除,需重新审批;分录尚未过账,账面不受影响。驳回理由记入审计日志。",
    "zh-HK": "駁回後分錄狀態回到草稿,原審批痕跡被清除,需重新審批;分錄尚未過賬,賬面不受影響。駁回理由記入審計日誌。",
    en: "The entry returns to draft and its approval marks are cleared, so it must be approved again. Nothing was posted, so the ledger is unaffected. The reason is recorded in the audit log.",
  },
  "monthly.reject_reason": {
    "zh-CN": "驳回理由",
    "zh-HK": "駁回理由",
    en: "Rejection Reason",
  },
  "monthly.reject_reason_placeholder": {
    "zh-CN": "例如:折现率有误、付款计划需更正、待业务确认",
    "zh-HK": "例如:折現率有誤、付款計劃需更正、待業務確認",
    en: "e.g. wrong discount rate, payment schedule needs correcting, pending business confirmation",
  },
  "monthly.reject_reason_required": {
    "zh-CN": "请填写驳回理由",
    "zh-HK": "請填寫駁回理由",
    en: "A rejection reason is required",
  },
  "monthly.reject_success": {
    "zh-CN": "分录已驳回,状态回到草稿",
    "zh-HK": "分錄已駁回,狀態回到草稿",
    en: "Entry rejected and returned to draft",
  },
  "monthly.reject_failed": {
    "zh-CN": "驳回失败",
    "zh-HK": "駁回失敗",
    en: "Rejection failed",
  },
  "monthly.approve_all_confirm": {
    "zh-CN": "确认审批批次 {batch} 中所有草稿分录？",
    "zh-HK": "確認審批批次 {batch} 中所有草稿分錄？",
    en: "Confirm approving all draft entries in batch {batch}?",
  },
  "monthly.post_all_confirm": {
    "zh-CN": "确认过账批次 {batch} 中所有已审批分录？",
    "zh-HK": "確認過賬批次 {batch} 中所有已審批分錄？",
    en: "Confirm posting all approved entries in batch {batch}?",
  },
  "monthly.approve_all": {
    "zh-CN": "审批全部",
    "zh-HK": "審批全部",
    en: "Approve All",
  },
  "monthly.post_all": {
    "zh-CN": "过账全部",
    "zh-HK": "過賬全部",
    en: "Post All",
  },
  "monthly.no_draft_entries": {
    "zh-CN": "没有可审批的草稿分录",
    "zh-HK": "沒有可審批的草稿分錄",
    en: "No draft entries to approve",
  },
  "monthly.no_approved_entries": {
    "zh-CN": "没有可过账的已审批分录",
    "zh-HK": "沒有可過賬的已審批分錄",
    en: "No approved entries to post",
  },
  "monthly.batch_approve_success_msg": {
    "zh-CN": "批量审批完成：{count} 笔",
    "zh-HK": "批量審批完成：{count} 筆",
    en: "Batch approval complete: {count} entries",
  },
  "monthly.batch_post_success_msg": {
    "zh-CN": "批量过账完成：{count} 笔",
    "zh-HK": "批量過賬完成：{count} 筆",
    en: "Batch posting complete: {count} entries",
  },
  "monthly.col_batch_number": {
    "zh-CN": "批次号",
    "zh-HK": "批次號",
    en: "Batch Number",
  },
  "monthly.col_total_contracts": {
    "zh-CN": "合同数",
    "zh-HK": "合同數",
    en: "Total Contracts",
  },
  "monthly.col_processed": {
    "zh-CN": "处理数",
    "zh-HK": "處理數",
    en: "Processed",
  },
  "monthly.col_failed": {
    "zh-CN": "失败数",
    "zh-HK": "失敗數",
    en: "Failed",
  },
  "monthly.col_total_entries": {
    "zh-CN": "分录数",
    "zh-HK": "分錄數",
    en: "Total Entries",
  },
  "monthly.col_posted_entries": {
    "zh-CN": "已过账",
    "zh-HK": "已過賬",
    en: "Posted Entries",
  },
  "monthly.col_created_at": {
    "zh-CN": "创建时间",
    "zh-HK": "創建時間",
    en: "Created At",
  },

  // Login
  "login.title": {
    "zh-CN": "零售经营分析工作站",
    "zh-HK": "零售經營分析工作站",
    en: "Retail Performance Workstation",
  },
  "login.tagline": {
    "zh-CN": "门店经营数据、租约台账与情景推演，收在同一个工作台。",
    "zh-HK": "門店經營數據、租約台賬與情景推演，收在同一個工作台。",
    en: "Store performance, lease ledgers and scenario work, on one desk.",
  },
  "login.point_pulse": {
    "zh-CN": "经营脉搏：门店与品类表现一屏可见",
    "zh-HK": "經營脈搏：門店與品類表現一屏可見",
    en: "Operating Pulse — store and category performance on one screen",
  },
  "login.point_store": {
    "zh-CN": "门店 360：单店诊断与归因分析",
    "zh-HK": "門店 360：單店診斷與歸因分析",
    en: "Store 360 — single-store diagnosis and attribution",
  },
  "login.point_lease": {
    "zh-CN": "租赁台账：IFRS 16 计量、披露与现金流",
    "zh-HK": "租賃台賬：IFRS 16 計量、披露與現金流",
    en: "Lease ledger — IFRS 16 measurement, disclosure and cash flow",
  },
  "login.welcome_back": {
    "zh-CN": "欢迎回来",
    "zh-HK": "歡迎回來",
    en: "Welcome back",
  },
  "login.continue_hint": {
    "zh-CN": "登录以继续",
    "zh-HK": "登錄以繼續",
    en: "Sign in to continue",
  },
  "login.username": {
    "zh-CN": "用户名",
    "zh-HK": "用戶名",
    en: "Username",
  },
  "login.password": {
    "zh-CN": "密码",
    "zh-HK": "密碼",
    en: "Password",
  },
  "login.submit": {
    "zh-CN": "登录",
    "zh-HK": "登錄",
    en: "Login",
  },
  "login.success": {
    "zh-CN": "登录成功！",
    "zh-HK": "登錄成功！",
    en: "Login successful!",
  },
  "login.failed": {
    "zh-CN": "登录失败",
    "zh-HK": "登錄失敗",
    en: "Login failed",
  },
  // A 401 from the login endpoint means the credentials are wrong, not that a
  // session lapsed — the generic 401 copy sends people looking for a session
  // they never had.
  "login.invalid_credentials": {
    "zh-CN": "用户名或密码错误，请重试。",
    "zh-HK": "用戶名或密碼錯誤，請重試。",
    en: "Incorrect username or password. Please try again.",
  },
  "login.no_register": {
    "zh-CN": "公开注册已关闭，如需账号请联系管理员",
    "zh-HK": "公開註冊已關閉，如需賬號請聯繫管理員",
    en: "Public registration is closed. Contact admin for an account.",
  },
  "login.username_required": {
    "zh-CN": "请输入用户名",
    "zh-HK": "請輸入用戶名",
    en: "Please enter username",
  },
  "login.password_required": {
    "zh-CN": "请输入密码",
    "zh-HK": "請輸入密碼",
    en: "Please enter password",
  },

  // Common Actions

  // ─── New Contract ──────────────────────────────────────────────
  "contract_new.title": {
    "zh-CN": "新增合同",
    "zh-HK": "新增合同",
    en: "New Contract",
  },
  "contract_new.back": {
    "zh-CN": "返回",
    "zh-HK": "返回",
    en: "Back",
  },
  "contract_new.discount_rate_missing_title": {
    "zh-CN": "折现率缺失警告",
    "zh-HK": "折現率缺失警告",
    en: "Discount Rate Missing",
  },
  "contract_new.discount_rate_missing_desc": {
    "zh-CN": "此合同未填写折现率。根据会计政策，折现率缺失的合同不能进行正式会计计算。请后续在合同详情页补充折现率。",
    "zh-HK": "此合同未填寫折現率。根據會計政策，折現率缺失的合同不能進行正式會計計算。請後續在合同詳情頁補充折現率。",
    en: "This contract has no discount rate. Per accounting policy, contracts without a discount rate cannot be used for formal accounting calculations. Please supplement the discount rate on the contract detail page later.",
  },
  "contract_new.contract_number": {
    "zh-CN": "合同编号",
    "zh-HK": "合同編號",
    en: "Contract Number",
  },
  "contract_new.contract_number_placeholder": {
    "zh-CN": "例如：LEASE-2024-001",
    "zh-HK": "例如：LEASE-2024-001",
    en: "e.g. LEASE-2024-001",
  },
  "contract_new.contract_name": {
    "zh-CN": "合同名称",
    "zh-HK": "合同名稱",
    en: "Contract Name",
  },
  "contract_new.contract_name_placeholder": {
    "zh-CN": "例如：南京东路旗舰店租赁合同",
    "zh-HK": "例如：南京東路旗艦店租賃合同",
    en: "e.g. Nanjing Road Flagship Lease",
  },
  "contract_new.legal_entity": {
    "zh-CN": "法人主体",
    "zh-HK": "法人主體",
    en: "Legal Entity",
  },
  "contract_new.select_legal_entity": {
    "zh-CN": "选择法人主体",
    "zh-HK": "選擇法人主體",
    en: "Select Legal Entity",
  },
  "contract_new.store": {
    "zh-CN": "门店",
    "zh-HK": "門店",
    en: "Store",
  },
  "contract_new.select_store": {
    "zh-CN": "选择门店",
    "zh-HK": "選擇門店",
    en: "Select Store",
  },
  "contract_new.lessor": {
    "zh-CN": "出租方",
    "zh-HK": "出租方",
    en: "Lessor",
  },
  "contract_new.select_lessor": {
    "zh-CN": "选择出租方",
    "zh-HK": "選擇出租方",
    en: "Select Lessor",
  },
  "contract_new.currency": {
    "zh-CN": "币种",
    "zh-HK": "幣種",
    en: "Currency",
  },
  "reports.tab_budget": {
    "zh-CN": "预算对比",
    "zh-HK": "預算對比",
    en: "Budget vs Actual",
  },
  "budget.version": {
    "zh-CN": "预算版本",
    "zh-HK": "預算版本",
    en: "Budget Version",
  },
  "budget.pick_version": {
    "zh-CN": "请选择预算版本",
    "zh-HK": "請選擇預算版本",
    en: "Select a budget version",
  },
  "budget.compare": {
    "zh-CN": "对比",
    "zh-HK": "對比",
    en: "Compare",
  },
  "budget.compare_to": {
    "zh-CN": "比较至",
    "zh-HK": "比較至",
    en: "Compare to",
  },
  "budget.period_placeholder": {
    "zh-CN": "期间 YYYY-MM",
    "zh-HK": "期間 YYYY-MM",
    en: "Period YYYY-MM",
  },
  "budget.freeze": {
    "zh-CN": "固化当前计量为预算",
    "zh-HK": "固化當前計量為預算",
    en: "Freeze Current Measurement",
  },
  "budget.freeze_hint": {
    "zh-CN": "以当前已审批合同的计量前瞻表作为本年度预算基线",
    "zh-HK": "以當前已審批合同的計量前瞻表作為本年度預算基線",
    en: "Freezes today's projected schedule for approved contracts as this year's baseline",
  },
  "budget.new_name_placeholder": {
    "zh-CN": "预算版本名称,如 2026 年度预算",
    "zh-HK": "預算版本名稱,如 2026 年度預算",
    en: "Version name, e.g. FY2026 budget",
  },
  "budget.source_placeholder": {
    "zh-CN": "来源（必填）",
    "zh-HK": "來源（必填）",
    en: "Source (required)",
  },
  "budget.from_period": {
    "zh-CN": "起始期间",
    "zh-HK": "起始期間",
    en: "From period",
  },
  "budget.to_period": {
    "zh-CN": "结束期间",
    "zh-HK": "結束期間",
    en: "To period",
  },
  "budget.coverage_scope": {
    "zh-CN": "覆盖范围",
    "zh-HK": "覆蓋範圍",
    en: "Coverage scope",
  },
  "budget.version_metadata_required": {
    "zh-CN": "来源、起始期间和结束期间均为必填项",
    "zh-HK": "來源、起始期間和結束期間均為必填項",
    en: "Source, from period and to period are required",
  },
  "budget.name_required": {
    "zh-CN": "请填写预算版本名称",
    "zh-HK": "請填寫預算版本名稱",
    en: "A version name is required",
  },
  "budget.created": {
    "zh-CN": "预算版本已固化:{contracts} 份合同 / {lines} 行",
    "zh-HK": "預算版本已固化:{contracts} 份合同 / {lines} 行",
    en: "Budget frozen: {contracts} contracts / {lines} lines",
  },
  "budget.create_failed": {
    "zh-CN": "预算版本固化失败",
    "zh-HK": "預算版本固化失敗",
    en: "Failed to freeze the budget",
  },
  "budget.load_failed": {
    "zh-CN": "预算数据加载失败",
    "zh-HK": "預算數據載入失敗",
    en: "Failed to load budget data",
  },
  "budget.budget_total": {
    "zh-CN": "预算租赁费用",
    "zh-HK": "預算租賃費用",
    en: "Budgeted Lease Cost",
  },
  "budget.actual_total": {
    "zh-CN": "实际租赁费用",
    "zh-HK": "實際租賃費用",
    en: "Actual Lease Cost",
  },
  "budget.variance": {
    "zh-CN": "差异(实际−预算)",
    "zh-HK": "差異(實際−預算)",
    en: "Variance (actual - budget)",
  },
  "budget.bridge_title": {
    "zh-CN": "差异归因桥",
    "zh-HK": "差異歸因橋",
    en: "Variance Bridge",
  },
  "budget.bridge_desc": {
    "zh-CN": "归因由事件驱动:新签、续租/终止、租金变更、汇率各成一项,事件无法解释的部分留作「其他」,因此各项之和恒等于总差异。",
    "zh-HK": "歸因由事件驅動:新簽、續租/終止、租金變更、匯率各成一項,事件無法解釋的部分留作「其他」,因此各項之和恆等於總差異。",
    en: "Attribution is event-driven: new leases, renewals and terminations, rent changes and exchange movements each get a line, and whatever the events cannot explain stays as a residual, so the lines always sum to the total variance.",
  },
  "budget.bridge_broken": {
    "zh-CN": "归因桥未闭合:各项之和不等于总差异,请勿据此汇报",
    "zh-HK": "歸因橋未閉合:各項之和不等於總差異,請勿據此匯報",
    en: "The bridge does not tie out: its lines do not sum to the variance. Do not report from it.",
  },
  "budget.by_contract_title": {
    "zh-CN": "按合同明细",
    "zh-HK": "按合同明細",
    en: "By Contract",
  },
  "budget.cause": {
    "zh-CN": "归因",
    "zh-HK": "歸因",
    en: "Cause",
  },
  "budget.amount": {
    "zh-CN": "金额",
    "zh-HK": "金額",
    en: "Amount",
  },
  "budget.contract_count": {
    "zh-CN": "合同数",
    "zh-HK": "合同數",
    en: "Contracts",
  },
  "budget.cause_new_lease": {
    "zh-CN": "新签合同",
    "zh-HK": "新簽合同",
    en: "New lease",
  },
  "budget.cause_ended": {
    "zh-CN": "已结束/退出",
    "zh-HK": "已結束/退出",
    en: "Ended",
  },
  "budget.cause_renewal": {
    "zh-CN": "续租/终止",
    "zh-HK": "續租/終止",
    en: "Renewal or termination",
  },
  "budget.cause_rent_change": {
    "zh-CN": "租金变更",
    "zh-HK": "租金變更",
    en: "Rent change",
  },
  "budget.cause_index_adjustment": {
    "zh-CN": "指数调整",
    "zh-HK": "指數調整",
    en: "Index adjustment",
  },
  "budget.cause_discount_rate": {
    "zh-CN": "折现率/重估利率",
    "zh-HK": "折現率/重估利率",
    en: "Discount rate",
  },
  "budget.cause_payment_timing": {
    "zh-CN": "付款时点",
    "zh-HK": "付款時點",
    en: "Payment timing",
  },
  "budget.cause_data_correction": {
    "zh-CN": "数据修正",
    "zh-HK": "數據修正",
    en: "Data correction",
  },
  "budget.cause_exchange_rate": {
    "zh-CN": "汇率",
    "zh-HK": "匯率",
    en: "Exchange rate",
  },
  "budget.cause_other": {
    "zh-CN": "其他(事件未解释)",
    "zh-HK": "其他(事件未解釋)",
    en: "Other (unexplained)",
  },
  "budget.actual_measurement_readonly": {
    "zh-CN": "Actual（计量结果，只读）",
    "zh-HK": "Actual（計量結果，只讀）",
    en: "Actual (measurement results, read-only)",
  },
  "budget.type_budget": {
    "zh-CN": "预算",
    "zh-HK": "預算",
    en: "Budget",
  },
  "budget.type_forecast": {
    "zh-CN": "预测 / LE",
    "zh-HK": "預測 / LE",
    en: "Forecast / LE",
  },
  "budget.type_scenario": {
    "zh-CN": "情景",
    "zh-HK": "情景",
    en: "Scenario",
  },
  "budget.measurement_source_hint": {
    "zh-CN": "Actual 来自 measurement_results；Forecast / Scenario 只作为冻结假设层，不会覆盖原 Budget。",
    "zh-HK": "Actual 來自 measurement_results；Forecast / Scenario 只作為凍結假設層，不會覆蓋原 Budget。",
    en: "Actual comes from measurement_results. Forecast and Scenario remain frozen assumption layers and do not overwrite the Budget.",
  },
  "budget.brief_title": {
    "zh-CN": "三口径管理简报 · {period}",
    "zh-HK": "三口徑管理簡報 · {period}",
    en: "Three-view management brief · {period}",
  },
  "budget.brief_budget": {
    "zh-CN": "Budget · {name}",
    "zh-HK": "Budget · {name}",
    en: "Budget · {name}",
  },
  "budget.brief_forecast": {
    "zh-CN": "Latest Estimate · {name}",
    "zh-HK": "Latest Estimate · {name}",
    en: "Latest Estimate · {name}",
  },
  "budget.brief_actual": {
    "zh-CN": "Actual · measurement_results",
    "zh-HK": "Actual · measurement_results",
    en: "Actual · measurement_results",
  },
  "budget.brief_variance": {
    "zh-CN": "Forecast vs Budget：{forecastBudget}；Actual vs Budget：{actualBudget}；Actual vs Forecast：{actualForecast}。三者均为租赁成本（利息 + 折旧）口径。",
    "zh-HK": "Forecast vs Budget：{forecastBudget}；Actual vs Budget：{actualBudget}；Actual vs Forecast：{actualForecast}。三者均為租賃成本（利息 + 折舊）口徑。",
    en: "Forecast vs Budget: {forecastBudget}; Actual vs Budget: {actualBudget}; Actual vs Forecast: {actualForecast}. All three use lease cost (interest + depreciation).",
  },
  "budget.explanation_coverage": {
    "zh-CN": "解释覆盖率",
    "zh-HK": "解釋覆蓋率",
    en: "Explanation coverage",
  },
  "budget.open_actions": {
    "zh-CN": "待跟进项",
    "zh-HK": "待跟進項",
    en: "Open actions",
  },
  "budget.open_action_amount": {
    "zh-CN": "待跟进金额",
    "zh-HK": "待跟進金額",
    en: "Open action amount",
  },
  "budget.comparison_basis": {
    "zh-CN": "比较口径",
    "zh-HK": "比較口徑",
    en: "Comparison basis",
  },
  "budget.plan_actual": {
    "zh-CN": "Plan → Actual",
    "zh-HK": "Plan → Actual",
    en: "Plan → Actual",
  },
  "budget.plan_plan": {
    "zh-CN": "Plan → Plan",
    "zh-HK": "Plan → Plan",
    en: "Plan → Plan",
  },
  "budget.save_actions": {
    "zh-CN": "保存差异行动",
    "zh-HK": "保存差異行動",
    en: "Save variance actions",
  },
  "budget.actions_saved": {
    "zh-CN": "差异跟进行动已保存",
    "zh-HK": "差異跟進行動已保存",
    en: "Variance actions saved",
  },
  "budget.actions_save_failed": {
    "zh-CN": "差异行动保存失败",
    "zh-HK": "差異行動保存失敗",
    en: "Failed to save variance actions",
  },
  "budget.explanation": {
    "zh-CN": "解释",
    "zh-HK": "解釋",
    en: "Explanation",
  },
  "budget.explanation_placeholder": {
    "zh-CN": "补充人工解释",
    "zh-HK": "補充人工解釋",
    en: "Add a human explanation",
  },
  "budget.owner": {
    "zh-CN": "责任人",
    "zh-HK": "責任人",
    en: "Owner",
  },
  "budget.owner_placeholder": {
    "zh-CN": "团队/责任人",
    "zh-HK": "團隊/責任人",
    en: "Team / owner",
  },
  "budget.due_date": {
    "zh-CN": "截止日期",
    "zh-HK": "截止日期",
    en: "Due date",
  },
  "budget.status": {
    "zh-CN": "状态",
    "zh-HK": "狀態",
    en: "Status",
  },
  "budget.status_open": {
    "zh-CN": "待处理",
    "zh-HK": "待處理",
    en: "Open",
  },
  "budget.status_in_progress": {
    "zh-CN": "处理中",
    "zh-HK": "處理中",
    en: "In progress",
  },
  "budget.status_resolved": {
    "zh-CN": "已解决",
    "zh-HK": "已解決",
    en: "Resolved",
  },
  "budget.status_accepted": {
    "zh-CN": "已接受",
    "zh-HK": "已接受",
    en: "Accepted",
  },
  "renewal.title": {
    "zh-CN": "续租决策卡",
    "zh-HK": "續租決策卡",
    en: "Renewal decision card",
  },
  "renewal.term": {
    "zh-CN": "续租租期",
    "zh-HK": "續租租期",
    en: "Renewal term",
  },
  "renewal.month": {
    "zh-CN": "月",
    "zh-HK": "月",
    en: "months",
  },
  "renewal.uplift": {
    "zh-CN": "续租涨幅",
    "zh-HK": "續租漲幅",
    en: "Renewal uplift",
  },
  "renewal.rent_free": {
    "zh-CN": "免租期",
    "zh-HK": "免租期",
    en: "Rent-free period",
  },
  "renewal.escalation": {
    "zh-CN": "年递增",
    "zh-HK": "年遞增",
    en: "Annual escalation",
  },
  "renewal.exit_penalty": {
    "zh-CN": "退出罚金",
    "zh-HK": "退出罰金",
    en: "Exit penalty",
  },
  "renewal.intro": {
    "zh-CN": "续租不是日期提醒，而是一次经营与会计联合决策。请明确输入谈判假设后查看租金、现值、利润表和退出成本影响。",
    "zh-HK": "續租不是日期提醒，而是一次經營與會計聯合決策。請明確輸入談判假設後查看租金、現值、損益表和退出成本影響。",
    en: "Renewal is an operating and accounting decision, not just a date reminder. Enter explicit negotiation assumptions to view rent, present value, P&L and exit-cost impacts.",
  },
  "renewal.expiry_date": {
    "zh-CN": "到期日",
    "zh-HK": "到期日",
    en: "Expiry date",
  },
  "renewal.day": {
    "zh-CN": "天",
    "zh-HK": "天",
    en: "days",
  },
  "renewal.remaining_commitment": {
    "zh-CN": "剩余承诺",
    "zh-HK": "剩餘承諾",
    en: "Remaining commitment",
  },
  "renewal.current_rent": {
    "zh-CN": "当前月租",
    "zh-HK": "當前月租",
    en: "Current monthly rent",
  },
  "renewal.uplift_cost": {
    "zh-CN": "涨幅成本（{percent}%）",
    "zh-HK": "漲幅成本（{percent}%）",
    en: "Uplift cost ({percent}%)",
  },
  "renewal.offer": {
    "zh-CN": "方案",
    "zh-HK": "方案",
    en: "Offer",
  },
  "renewal.effective_monthly_rent": {
    "zh-CN": "有效月租",
    "zh-HK": "有效月租",
    en: "Effective monthly rent",
  },
  "renewal.rent_per_sqm": {
    "zh-CN": "每平米有效单价",
    "zh-HK": "每平方米有效單價",
    en: "Effective rent per sqm",
  },
  "renewal.total_rent": {
    "zh-CN": "全期租金",
    "zh-HK": "全期租金",
    en: "Total rent",
  },
  "renewal.present_value": {
    "zh-CN": "现值",
    "zh-HK": "現值",
    en: "Present value",
  },
  "renewal.scenarios": {
    "zh-CN": "经营与会计情景",
    "zh-HK": "經營與會計情景",
    en: "Operating and accounting scenarios",
  },
  "renewal.scenario_notice": {
    "zh-CN": "仅为情景假设，不会修改合同",
    "zh-HK": "僅為情景假設，不會修改合同",
    en: "Scenario assumptions only; the contract is not modified",
  },
  "renewal.year": {
    "zh-CN": "年度",
    "zh-HK": "年度",
    en: "Year",
  },
  "renewal.cash_outflow": {
    "zh-CN": "现金流出",
    "zh-HK": "現金流出",
    en: "Cash outflow",
  },
  "renewal.ifrs16_expense": {
    "zh-CN": "IFRS 16 费用",
    "zh-HK": "IFRS 16 費用",
    en: "IFRS 16 expense",
  },
  "renewal.ebitda_impact": {
    "zh-CN": "EBITDA 影响",
    "zh-HK": "EBITDA 影響",
    en: "EBITDA impact",
  },
  "renewal.ebit_impact": {
    "zh-CN": "EBIT 影响",
    "zh-HK": "EBIT 影響",
    en: "EBIT impact",
  },
  "renewal.closing_liability": {
    "zh-CN": "期末负债",
    "zh-HK": "期末負債",
    en: "Closing liability",
  },
  "renewal.closing_rou": {
    "zh-CN": "期末 ROU",
    "zh-HK": "期末 ROU",
    en: "Closing ROU",
  },
  "renewal.liability_released": {
    "zh-CN": "释放负债",
    "zh-HK": "釋放負債",
    en: "Liability released",
  },
  "renewal.rou_written_off": {
    "zh-CN": "转销 ROU",
    "zh-HK": "轉銷 ROU",
    en: "ROU written off",
  },
  "renewal.penalty": {
    "zh-CN": "退出罚金",
    "zh-HK": "退出罰金",
    en: "Penalty",
  },
  "renewal.total_cash_to_exit": {
    "zh-CN": "退出现金成本",
    "zh-HK": "退出現金成本",
    en: "Total cash to exit",
  },
  "renewal.exit_summary": {
    "zh-CN": "退出现金成本 {cash}；损益影响 {pnl}",
    "zh-HK": "退出現金成本 {cash}；損益影響 {pnl}",
    en: "Exit cash cost {cash}; P&L impact {pnl}",
  },
  "renewal.exit_detail": {
    "zh-CN": "剩余承诺 {commitment}；释放负债 {liability}；转销 ROU {rou}",
    "zh-HK": "剩餘承諾 {commitment}；釋放負債 {liability}；轉銷 ROU {rou}",
    en: "Remaining commitment {commitment}; liability released {liability}; ROU written off {rou}",
  },
  "renewal.decision": {
    "zh-CN": "决策",
    "zh-HK": "決策",
    en: "Decision",
  },
  "renewal.type": {
    "zh-CN": "类型",
    "zh-HK": "類型",
    en: "Type",
  },
  "renewal.type_terminate": {
    "zh-CN": "终止",
    "zh-HK": "終止",
    en: "Terminate",
  },
  "renewal.type_renegotiate": {
    "zh-CN": "重谈",
    "zh-HK": "重談",
    en: "Renegotiate",
  },
  "renewal.type_renew": {
    "zh-CN": "续租",
    "zh-HK": "續租",
    en: "Renew",
  },
  "renewal.monthly_rent": {
    "zh-CN": "月租金",
    "zh-HK": "月租金",
    en: "Monthly rent",
  },
  "renewal.total_cash_outflow": {
    "zh-CN": "现金总流出",
    "zh-HK": "現金總流出",
    en: "Total cash outflow",
  },
  "renewal.total_ifrs16_expense": {
    "zh-CN": "IFRS 16 总费用",
    "zh-HK": "IFRS 16 總費用",
    en: "Total IFRS 16 expense",
  },
  "renewal.source": {
    "zh-CN": "来源",
    "zh-HK": "來源",
    en: "Source",
  },
  "renewal.scenario_assumption": {
    "zh-CN": "情景假设",
    "zh-HK": "情景假設",
    en: "Scenario assumption",
  },
  "renewal.owner_placeholder": {
    "zh-CN": "业务负责人",
    "zh-HK": "業務負責人",
    en: "Business owner",
  },
  "renewal.opinion_placeholder": {
    "zh-CN": "业务意见 / 谈判结论",
    "zh-HK": "業務意見 / 談判結論",
    en: "Business opinion / negotiation conclusion",
  },
  "renewal.evidence_placeholder": {
    "zh-CN": "依据（经营数据、报价或合同条款）",
    "zh-HK": "依據（經營數據、報價或合同條款）",
    en: "Evidence (operating data, offer or contract clause)",
  },
  "renewal.save_snapshot": {
    "zh-CN": "保存决策快照",
    "zh-HK": "保存決策快照",
    en: "Save decision snapshot",
  },
  "renewal.saved_count": {
    "zh-CN": "已保存 {count} 次",
    "zh-HK": "已保存 {count} 次",
    en: "Saved {count} time(s)",
  },
  "renewal.snapshot_saved": {
    "zh-CN": "决策快照已保存",
    "zh-HK": "決策快照已保存",
    en: "Decision snapshot saved",
  },
  "renewal.snapshot_save_failed": {
    "zh-CN": "决策快照保存失败",
    "zh-HK": "決策快照保存失敗",
    en: "Failed to save decision snapshot",
  },
  "renewal.load_failed": {
    "zh-CN": "续租决策数据加载失败",
    "zh-HK": "續租決策數據載入失敗",
    en: "Failed to load renewal decision data",
  },
  "renewal.scenario_current_terms": {
    "zh-CN": "按现租金续租",
    "zh-HK": "按現租金續租",
    en: "Renew at current rent",
  },
  "renewal.scenario_renegotiate": {
    "zh-CN": "重谈续租条款",
    "zh-HK": "重談續租條款",
    en: "Renegotiate renewal terms",
  },
  "renewal.scenario_terminate": {
    "zh-CN": "终止 / 不续租",
    "zh-HK": "終止 / 不續租",
    en: "Terminate / do not renew",
  },
  "renewal.health_healthy": {
    "zh-CN": "健康",
    "zh-HK": "健康",
    en: "Healthy",
  },
  "renewal.health_watch": {
    "zh-CN": "关注",
    "zh-HK": "關注",
    en: "Watch",
  },
  "renewal.health_over_threshold": {
    "zh-CN": "超过预警线",
    "zh-HK": "超過預警線",
    en: "Above warning line",
  },
  "renewal.health_no_revenue": {
    "zh-CN": "无营收数据",
    "zh-HK": "無營收數據",
    en: "No revenue data",
  },
  "renewal.health_zero_revenue": {
    "zh-CN": "营收为零",
    "zh-HK": "營收為零",
    en: "Zero revenue",
  },
  "renewal.health_currency_mismatch": {
    "zh-CN": "币种不一致",
    "zh-HK": "幣種不一致",
    en: "Currency mismatch",
  },
  "renewal.health_description": {
    "zh-CN": "暂无可用的门店经营指标",
    "zh-HK": "暫無可用的門店經營指標",
    en: "No store operating metrics are available",
  },
  "renewal.no_revenue": {
    "zh-CN": "暂无门店经营数据",
    "zh-HK": "暫無門店經營數據",
    en: "No store operating data",
  },
  "renewal.no_revenue_description": {
    "zh-CN": "请先导入对应期间的 POS / ERP 营收数据，并配置租售比政策阈值。",
    "zh-HK": "請先導入對應期間的 POS / ERP 營收數據，並配置租售比政策閾值。",
    en: "Import POS / ERP revenue for the relevant period and configure rent-to-sales policy thresholds first.",
  },
  "contracts.selected_count": {
    "zh-CN": "已选 {count} 份草稿合同",
    "zh-HK": "已選 {count} 份草稿合同",
    en: "{count} draft contracts selected",
  },
  "contracts.bulk_submit": {
    "zh-CN": "批量提交复核",
    "zh-HK": "批量提交覆核",
    en: "Submit for Review",
  },
  "contracts.clear_selection": {
    "zh-CN": "取消选择",
    "zh-HK": "取消選擇",
    en: "Clear",
  },
  "contracts.bulk_submit_done": {
    "zh-CN": "{count} 份合同已提交复核",
    "zh-HK": "{count} 份合同已提交覆核",
    en: "{count} contracts submitted for review",
  },
  "contracts.bulk_submit_partial": {
    "zh-CN": "{succeeded} 份已提交,{failed} 份失败(可能状态已变更或权限不足)",
    "zh-HK": "{succeeded} 份已提交,{failed} 份失敗(可能狀態已變更或權限不足)",
    en: "{succeeded} submitted, {failed} failed (status may have changed, or permission is missing)",
  },
  "todo.title": {
    "zh-CN": "我的待办",
    "zh-HK": "我的待辦",
    en: "My Work Queue",
  },
  "todo.subtitle": {
    "zh-CN": "共 {count} 项等待处理",
    "zh-HK": "共 {count} 項等待處理",
    en: "{count} items waiting",
  },
  "todo.refresh": {
    "zh-CN": "刷新",
    "zh-HK": "刷新",
    en: "Refresh",
  },
  "todo.load_failed": {
    "zh-CN": "待办加载失败",
    "zh-HK": "待辦載入失敗",
    en: "Failed to load the work queue",
  },
  "todo.section_clear": {
    "zh-CN": "这一类已清空",
    "zh-HK": "這一類已清空",
    en: "Nothing waiting here",
  },
  "todo.all_clear": { "zh-CN": "当前没有待处理事项", "zh-HK": "當前沒有待處理事項", en: "There is nothing waiting for you" },
  "todo.start_contract": { "zh-CN": "新增合同", "zh-HK": "新增合同", en: "Add a contract" },
  "todo.start_ai": { "zh-CN": "用 AI 解析合同", "zh-HK": "用 AI 解析合同", en: "Parse a contract with AI" },
  "todo.contracts_pending_review": {
    "zh-CN": "待复核合同",
    "zh-HK": "待覆核合同",
    en: "Contracts Awaiting Review",
  },
  "todo.contracts_pending_approval": {
    "zh-CN": "待审批合同",
    "zh-HK": "待審批合同",
    en: "Contracts Awaiting Approval",
  },
  "todo.events_pending": {
    "zh-CN": "待处理租赁事件",
    "zh-HK": "待處理租賃事件",
    en: "Lease Events Awaiting Action",
  },
  "todo.entries_pending_approval": {
    "zh-CN": "待审批分录",
    "zh-HK": "待審批分錄",
    en: "Journal Entries Awaiting Approval",
  },
  "todo.entries_pending_posting": {
    "zh-CN": "待过账分录",
    "zh-HK": "待過賬分錄",
    en: "Journal Entries Awaiting Posting",
  },
  "todo.critical_dates_due": {
    "zh-CN": "临近关键日期",
    "zh-HK": "臨近關鍵日期",
    en: "Critical Dates Due",
  },
  "todo.overdue": {
    "zh-CN": "已逾期 {days} 天",
    "zh-HK": "已逾期 {days} 天",
    en: "Overdue by {days} days",
  },
  "todo.due_today": {
    "zh-CN": "今天到期",
    "zh-HK": "今天到期",
    en: "Due today",
  },
  "todo.due_in": {
    "zh-CN": "剩余 {days} 天",
    "zh-HK": "剩餘 {days} 天",
    en: "{days} days left",
  },
  "todo.scope_note": {
    "zh-CN": "待办按你的数据权限范围呈现;每项能否执行,仍由对应操作的权限与职责分离规则决定。",
    "zh-HK": "待辦按你的數據權限範圍呈現;每項能否執行,仍由對應操作的權限與職責分離規則決定。",
    en: "The queue reflects your data scope. Whether you can act on an item is still decided by that action's own permission and segregation-of-duties rules.",
  },
  "todo.readiness_title": {
    "zh-CN": "月结准备度预检",
    "zh-HK": "月結準備度預檢",
    en: "Close readiness preflight",
  },
  "todo.readiness_refresh": {
    "zh-CN": "刷新预检",
    "zh-HK": "刷新預檢",
    en: "Refresh",
  },
  "todo.readiness_period": {
    "zh-CN": "会计期间",
    "zh-HK": "會計期間",
    en: "Period",
  },
  "todo.readiness_population": {
    "zh-CN": "评估合同",
    "zh-HK": "評估合同",
    en: "Contracts evaluated",
  },
  "todo.readiness_blocking": {
    "zh-CN": "阻塞项",
    "zh-HK": "阻塞項",
    en: "Blocking findings",
  },
  "todo.readiness_evaluated": {
    "zh-CN": "评估时间",
    "zh-HK": "評估時間",
    en: "Evaluated",
  },
  "todo.readiness_scope_complete": {
    "zh-CN": "法人范围完整",
    "zh-HK": "法人範圍完整",
    en: "Legal-entity-wide scope",
  },
  "todo.readiness_scope_limited": {
    "zh-CN": "当前权限为局部范围",
    "zh-HK": "當前權限為局部範圍",
    en: "Scope limited",
  },
  "todo.readiness_scope_warning": {
    "zh-CN": "当前结果仅覆盖你的数据权限范围，不能代表整个法人期间已准备就绪。",
    "zh-HK": "當前結果僅覆蓋你的數據權限範圍，不能代表整個法人期間已準備就緒。",
    en: "This result covers only your data scope and must not be treated as legal-entity-wide readiness.",
  },
  "todo.readiness_ready": {
    "zh-CN": "准备就绪",
    "zh-HK": "準備就緒",
    en: "Ready",
  },
  "todo.readiness_blocked": {
    "zh-CN": "存在阻塞项",
    "zh-HK": "存在阻塞項",
    en: "Blocked",
  },
  "todo.readiness_not_run": {
    "zh-CN": "尚未评估",
    "zh-HK": "尚未評估",
    en: "Not evaluated",
  },
  "todo.readiness_blocking_tag": {
    "zh-CN": "阻塞",
    "zh-HK": "阻塞",
    en: "Blocking",
  },
  "todo.readiness_clear": {
    "zh-CN": "当前可见范围没有预检发现。请确认范围完整后再进行正式月结。",
    "zh-HK": "當前可見範圍沒有預檢發現。請確認範圍完整後再進行正式月結。",
    en: "No findings in the visible scope. Confirm scope completeness before the formal close.",
  },
  "todo.exceptions_title": {
    "zh-CN": "正式月结异常",
    "zh-HK": "正式月結異常",
    en: "Formal close exceptions",
  },
  "todo.exceptions_detect": {
    "zh-CN": "运行异常检测",
    "zh-HK": "運行異常檢測",
    en: "Detect exceptions",
  },
  "todo.exceptions_empty": {
    "zh-CN": "当前期间还没有持久化异常。运行检测后，预检发现会进入治理队列。",
    "zh-HK": "當前期間還沒有持久化異常。運行檢測後，預檢發現會進入治理隊列。",
    en: "No persisted exceptions for this period. Run detection to turn preflight findings into governed work.",
  },
  "todo.exceptions_scope_warning": {
    "zh-CN": "当前异常列表只覆盖你的数据范围，不能作为法人全量的关闭结论。",
    "zh-HK": "當前異常列表只覆蓋你的數據範圍，不能作為法人全量的關閉結論。",
    en: "This exception list covers only your data scope and is not a legal-entity-wide close conclusion.",
  },
  "todo.exceptions_detected": {
    "zh-CN": "异常检测完成",
    "zh-HK": "異常檢測完成",
    en: "Exception detection completed",
  },
  "todo.exceptions_action_done": {
    "zh-CN": "异常治理动作已完成",
    "zh-HK": "異常治理動作已完成",
    en: "Exception action completed",
  },
  "todo.exceptions_action_failed": {
    "zh-CN": "异常治理动作失败",
    "zh-HK": "異常治理動作失敗",
    en: "Exception action failed",
  },
  "todo.exceptions_note_prompt": {
    "zh-CN": "请输入本次治理动作的证据或说明",
    "zh-HK": "請輸入本次治理動作的證據或說明",
    en: "Enter evidence or a note for this exception action",
  },
  "todo.exceptions_owner_prompt": {
    "zh-CN": "请输入负责人用户 ID",
    "zh-HK": "請輸入負責人用戶 ID",
    en: "Enter the owner user ID",
  },
  "todo.exceptions_assign": {
    "zh-CN": "分派",
    "zh-HK": "分派",
    en: "Assign",
  },
  "todo.exceptions_verify": {
    "zh-CN": "验证解决",
    "zh-HK": "驗證解決",
    en: "Verify resolution",
  },
  "todo.exceptions_conclude": {
    "zh-CN": "记录会计结论",
    "zh-HK": "記錄會計結論",
    en: "Record accounting conclusion",
  },
  "todo.exceptions_waive": {
    "zh-CN": "期间豁免",
    "zh-HK": "期間豁免",
    en: "Period waiver",
  },
  "todo.exceptions_close": {
    "zh-CN": "关闭异常",
    "zh-HK": "關閉異常",
    en: "Close exception",
  },
  "todo.exceptions_disposition": {
    "zh-CN": "处置",
    "zh-HK": "處置",
    en: "Disposition",
  },
  "todo.exceptions_detected_at": {
    "zh-CN": "最近检测",
    "zh-HK": "最近檢測",
    en: "Last detected",
  },
  "nav.todo": {
    "zh-CN": "我的待办",
    "zh-HK": "我的待辦",
    en: "My Work Queue",
  },
  "settings.fx_title": {
    "zh-CN": "汇率维护",
    "zh-HK": "匯率維護",
    en: "Exchange Rates",
  },
  "settings.fx_desc": {
    "zh-CN": "外币租赁按 IAS 21 折算为法人主体职能货币:租赁负债为货币性项目,期末按收盘价重估、差额进汇兑损益;使用权资产为非货币性项目,按历史成本不重估。月结缺少所需汇率时会拒绝该合同,不会自行猜测。",
    "zh-HK": "外幣租賃按 IAS 21 折算為法人主體職能貨幣:租賃負債為貨幣性項目,期末按收盤價重估、差額進匯兌損益;使用權資產為非貨幣性項目,按歷史成本不重估。月結缺少所需匯率時會拒絕該合同,不會自行猜測。",
    en: "Foreign-currency leases are translated into the entity's functional currency under IAS 21: the lease liability is monetary and is remeasured at the closing rate with the difference in profit or loss, while the right-of-use asset is non-monetary and stays at historical cost. A close missing a required rate refuses that contract rather than guessing.",
  },
  "settings.fx_type_closing": {
    "zh-CN": "期末收盘价",
    "zh-HK": "期末收盤價",
    en: "Closing",
  },
  "settings.fx_type_average": {
    "zh-CN": "当期平均价",
    "zh-HK": "當期平均價",
    en: "Average",
  },
  "settings.fx_rate_placeholder": {
    "zh-CN": "1 原币 = ? 目标币",
    "zh-HK": "1 原幣 = ? 目標幣",
    en: "1 from-unit = ? to-units",
  },
  "settings.fx_save": {
    "zh-CN": "保存汇率",
    "zh-HK": "保存匯率",
    en: "Save Rate",
  },
  "settings.fx_saved": {
    "zh-CN": "汇率已保存",
    "zh-HK": "匯率已保存",
    en: "Rate saved",
  },
  "settings.fx_save_failed": {
    "zh-CN": "汇率保存失败",
    "zh-HK": "匯率保存失敗",
    en: "Failed to save rate",
  },
  "settings.fx_incomplete": {
    "zh-CN": "请填写汇率日期与汇率值",
    "zh-HK": "請填寫匯率日期與匯率值",
    en: "Rate date and rate value are required",
  },
  "settings.fx_empty": {
    "zh-CN": "暂无汇率;外币租赁月结前需先维护期末收盘价",
    "zh-HK": "暫無匯率;外幣租賃月結前需先維護期末收盤價",
    en: "No rates yet. A foreign-currency close needs a closing rate first.",
  },
  "settings.fx_pair": {
    "zh-CN": "币种对",
    "zh-HK": "幣種對",
    en: "Currency Pair",
  },
  "settings.fx_date": {
    "zh-CN": "汇率日期",
    "zh-HK": "匯率日期",
    en: "Rate Date",
  },
  "settings.fx_type": {
    "zh-CN": "类型",
    "zh-HK": "類型",
    en: "Type",
  },
  "settings.fx_rate": {
    "zh-CN": "汇率",
    "zh-HK": "匯率",
    en: "Rate",
  },
  "settings.fx_source": {
    "zh-CN": "来源",
    "zh-HK": "來源",
    en: "Source",
  },
  "contract_new.area_sqm": {
    "zh-CN": "租赁面积(㎡)",
    "zh-HK": "租賃面積(㎡)",
    en: "Leased Area (sqm)",
  },
  "contract_new.area_sqm_help": {
    "zh-CN": "本合同承租的面积,用于每平米月租对比;车辆、设备等无面积的租赁可留空",
    "zh-HK": "本合同承租的面積,用於每平米月租對比;車輛、設備等無面積的租賃可留空",
    en: "The area leased under this contract, used for rent-per-square-metre comparison. Leave blank for vehicles, equipment and other leases without an area.",
  },
  "contract_new.area_sqm_placeholder": {
    "zh-CN": "例如 320",
    "zh-HK": "例如 320",
    en: "e.g. 320",
  },
  "contract_detail.area_sqm": {
    "zh-CN": "租赁面积",
    "zh-HK": "租賃面積",
    en: "Leased Area",
  },
  "contract_new.tags": {
    "zh-CN": "标签",
    "zh-HK": "標籤",
    en: "Tags",
  },
  "contract_new.tags_help": {
    "zh-CN": "用于报表按标签汇总，例如 #华东 #直营 #旗舰店",
    "zh-HK": "用於報表按標籤匯總，例如 #華東 #直營 #旗艦店",
    en: "Used for grouping reports by tag, e.g. #EastChina #Direct #Flagship",
  },
  "contract_new.tags_placeholder": {
    "zh-CN": "输入标签后回车，例如 #华东、#直营、#旗舰店",
    "zh-HK": "輸入標籤後回車，例如 #華東、#直營、#旗艦店",
    en: "Enter tags and press Enter, e.g. #EastChina, #Direct, #Flagship",
  },
  "contract_new.commencement_date": {
    "zh-CN": "租赁起始日 (Commencement Date)",
    "zh-HK": "租賃起始日 (Commencement Date)",
    en: "Commencement Date",
  },
  "contract_new.lease_start_date": {
    "zh-CN": "租赁开始日 (Lease Start Date)",
    "zh-HK": "租賃開始日 (Lease Start Date)",
    en: "Lease Start Date",
  },
  "contract_new.lease_end_date": {
    "zh-CN": "租赁结束日 (Lease End Date)",
    "zh-HK": "租賃結束日 (Lease End Date)",
    en: "Lease End Date",
  },
  "contract_new.discount_rate_section": {
    "zh-CN": "折现率设置",
    "zh-HK": "折現率設置",
    en: "Discount Rate Settings",
  },
  "contract_new.discount_rate_tip": {
    "zh-CN": "提示",
    "zh-HK": "提示",
    en: "Tip",
  },
  "contract_new.discount_rate_tip_desc": {
    "zh-CN": "如果合同未明确折现率，可以留空。系统会标记为折现率缺失，后续可在合同详情页补充。",
    "zh-HK": "如果合同未明確折現率，可以留空。系統會標記為折現率缺失，後續可在合同詳情頁補充。",
    en: "If the contract does not specify a discount rate, you may leave it blank. The system will mark it as missing, and you can supplement it on the contract detail page later.",
  },
  "contract_new.discount_rate_value": {
    "zh-CN": "折现率数值 (%)",
    "zh-HK": "折現率數值 (%)",
    en: "Discount Rate Value (%)",
  },
  "contract_new.discount_rate_help": {
    "zh-CN": "可直接填写年化折现率，例如 5 表示 5%",
    "zh-HK": "可直接填寫年化折現率，例如 5 表示 5%",
    en: "Enter annual discount rate directly, e.g. 5 means 5%",
  },
  "contract_new.discount_rate_placeholder": {
    "zh-CN": "例如 5 或 5.25",
    "zh-HK": "例如 5 或 5.25",
    en: "e.g. 5 or 5.25",
  },
  "contract_new.discount_rate_type": {
    "zh-CN": "折现率类型",
    "zh-HK": "折現率類型",
    en: "Discount Rate Type",
  },
  "contract_new.select_discount_rate_type": {
    "zh-CN": "选择折现率类型",
    "zh-HK": "選擇折現率類型",
    en: "Select Discount Rate Type",
  },
  "contract_new.rate_type_group_ibr": {
    "zh-CN": "集团 IBR",
    "zh-HK": "集團 IBR",
    en: "Group IBR",
  },
  "contract_new.rate_type_entity": {
    "zh-CN": "法人特定利率",
    "zh-HK": "法人特定利率",
    en: "Entity Specific Rate",
  },
  "contract_new.rate_type_contract": {
    "zh-CN": "合同特定利率",
    "zh-HK": "合同特定利率",
    en: "Contract Specific Rate",
  },
  "contract_new.rate_type_implicit": {
    "zh-CN": "隐含利率",
    "zh-HK": "隱含利率",
    en: "Implicit Rate",
  },
  "contract_new.discount_rate_version": {
    "zh-CN": "折现率版本/编号",
    "zh-HK": "折現率版本/編號",
    en: "Discount Rate Version",
  },
  "contract_new.discount_rate_version_placeholder": {
    "zh-CN": "例如：IBR-2024-Q1",
    "zh-HK": "例如：IBR-2024-Q1",
    en: "e.g. IBR-2024-Q1",
  },
  "contract_new.create_button": {
    "zh-CN": "创建合同",
    "zh-HK": "創建合同",
    en: "Create Contract",
  },
  "contract_new.please_login": {
    "zh-CN": "请先登录",
    "zh-HK": "請先登錄",
    en: "Please login first",
  },
  "contract_new.create_success": {
    "zh-CN": "合同创建成功！",
    "zh-HK": "合同創建成功！",
    en: "Contract created successfully!",
  },
  "contract_new.create_failed": {
    "zh-CN": "创建失败",
    "zh-HK": "創建失敗",
    en: "Create failed",
  },
  "contract_new.discount_rate_empty_warning": {
    "zh-CN": "折现率未填写，合同将标记为折现率缺失状态",
    "zh-HK": "折現率未填寫，合同將標記為折現率缺失狀態",
    en: "Discount rate not filled, contract will be marked as discount rate missing",
  },
  "contract_new.please_enter_number": {
    "zh-CN": "请输入合同编号",
    "zh-HK": "請輸入合同編號",
    en: "Please enter contract number",
  },
  "contract_new.please_enter_name": {
    "zh-CN": "请输入合同名称",
    "zh-HK": "請輸入合同名稱",
    en: "Please enter contract name",
  },
  "contract_new.please_select_entity": {
    "zh-CN": "请选择法人主体",
    "zh-HK": "請選擇法人主體",
    en: "Please select legal entity",
  },
  "contract_new.please_select_store": {
    "zh-CN": "请选择门店",
    "zh-HK": "請選擇門店",
    en: "Please select store",
  },
  "contract_new.please_select_lessor": {
    "zh-CN": "请选择出租方",
    "zh-HK": "請選擇出租方",
    en: "Please select lessor",
  },
  "contract_new.please_select_date": {
    "zh-CN": "请选择租赁起始日",
    "zh-HK": "請選擇租賃起始日",
    en: "Please select commencement date",
  },
  "contract_new.please_select_start": {
    "zh-CN": "请选择租赁开始日",
    "zh-HK": "請選擇租賃開始日",
    en: "Please select lease start date",
  },
  "contract_new.please_select_end": {
    "zh-CN": "请选择租赁结束日",
    "zh-HK": "請選擇租賃結束日",
    en: "Please select lease end date",
  },

  // ─── Cashflow Forecast ───────────────────────────────────────
  "cashflow.title": {
    "zh-CN": "未来租金现金流预测",
    "zh-HK": "未來租金現金流預測",
    en: "Future Rent Cashflow Forecast",
  },
  "cashflow.report_mode": {
    "zh-CN": "报表模式",
    "zh-HK": "報表模式",
    en: "Report Mode",
  },
  "cashflow.working_mode": {
    "zh-CN": "工作报表 (Working)",
    "zh-HK": "工作報表 (Working)",
    en: "Working Report",
  },
  "cashflow.official_mode": {
    "zh-CN": "正式报表 (Official)",
    "zh-HK": "正式報表 (Official)",
    en: "Official Report",
  },
  "cashflow.view_dimension": {
    "zh-CN": "视图维度：",
    "zh-HK": "視圖維度：",
    en: "View Dimension:",
  },
  "cashflow.dimension_contract": {
    "zh-CN": "合同维度",
    "zh-HK": "合同維度",
    en: "By Contract",
  },
  "cashflow.dimension_store": {
    "zh-CN": "门店维度",
    "zh-HK": "門店維度",
    en: "By Store",
  },
  "cashflow.dimension_summary": {
    "zh-CN": "汇总",
    "zh-HK": "匯總",
    en: "Summary",
  },
  "cashflow.granularity": {
    "zh-CN": "粒度：",
    "zh-HK": "粒度：",
    en: "Granularity:",
  },
  "cashflow.granularity_month": {
    "zh-CN": "月",
    "zh-HK": "月",
    en: "Month",
  },
  "cashflow.granularity_quarter": {
    "zh-CN": "季",
    "zh-HK": "季",
    en: "Quarter",
  },
  "cashflow.granularity_year": {
    "zh-CN": "年",
    "zh-HK": "年",
    en: "Year",
  },
  "cashflow.date_range": {
    "zh-CN": "日期范围：",
    "zh-HK": "日期範圍：",
    en: "Date Range:",
  },
  "cashflow.query": {
    "zh-CN": "查询",
    "zh-HK": "查詢",
    en: "Query",
  },
  "cashflow.reset": {
    "zh-CN": "重置",
    "zh-HK": "重置",
    en: "Reset",
  },
  "cashflow.export_csv": {
    "zh-CN": "导出 CSV",
    "zh-HK": "導出 CSV",
    en: "Export CSV",
  },
  "cashflow.toggle_collapse": {
    "zh-CN": "收起筛选条件",
    "zh-HK": "收起篩選條件",
    en: "Collapse Filters",
  },
  "cashflow.toggle_expand": {
    "zh-CN": "展开筛选条件",
    "zh-HK": "展開篩選條件",
    en: "Expand Filters",
  },
  "cashflow.filter_contract_id": {
    "zh-CN": "合同 ID：",
    "zh-HK": "合同 ID：",
    en: "Contract ID:",
  },
  "cashflow.filter_contract_placeholder": {
    "zh-CN": "输入合同 ID 筛选",
    "zh-HK": "輸入合同 ID 篩選",
    en: "Enter contract ID to filter",
  },
  "cashflow.filter_store": {
    "zh-CN": "门店：",
    "zh-HK": "門店：",
    en: "Store:",
  },
  "cashflow.filter_store_placeholder": {
    "zh-CN": "输入门店名称筛选",
    "zh-HK": "輸入門店名稱篩選",
    en: "Enter store name to filter",
  },
  "cashflow.filter_tags": {
    "zh-CN": "标签：",
    "zh-HK": "標籤：",
    en: "Tags:",
  },
  "cashflow.filter_tags_placeholder": {
    "zh-CN": "选择或输入一个或多个标签",
    "zh-HK": "選擇或輸入一個或多個標籤",
    en: "Select or enter one or more tags",
  },
  "cashflow.stat_total_outflow": {
    "zh-CN": "总现金流出",
    "zh-HK": "總現金流出",
    en: "Total Cash Outflow",
  },
  "cashflow.stat_fixed_rent": {
    "zh-CN": "固定租金",
    "zh-HK": "固定租金",
    en: "Fixed Rent",
  },
  "cashflow.stat_variable_rent": {
    "zh-CN": "变量租金",
    "zh-HK": "變量租金",
    en: "Variable Rent",
  },
  "cashflow.stat_non_lease": {
    "zh-CN": "非租赁成分",
    "zh-HK": "非租賃成分",
    en: "Non-lease Component",
  },
  "cashflow.table_title": {
    "zh-CN": "现金流预测",
    "zh-HK": "現金流預測",
    en: "Cashflow Forecast",
  },
  "cashflow.table_working_hint": {
    "zh-CN": "含未审批数据",
    "zh-HK": "含未審批數據",
    en: "Includes unapproved data",
  },
  "cashflow.table_official_hint": {
    "zh-CN": "仅正式数据",
    "zh-HK": "僅正式數據",
    en: "Official data only",
  },
  "cashflow.empty_title": {
    "zh-CN": "当前范围内暂无未来付款计划",
    "zh-HK": "當前範圍內暫無未來付款計劃",
    en: "No future payment plans in current range",
  },
  "cashflow.empty_hint": {
    "zh-CN": "请设置查询条件后点击「查询」",
    "zh-HK": "請設置查詢條件後點擊「查詢」",
    en: "Please set query conditions and click Query",
  },
  "cashflow.please_select_date": {
    "zh-CN": "请选择开始日期和结束日期",
    "zh-HK": "請選擇開始日期和結束日期",
    en: "Please select start and end dates",
  },
  "cashflow.query_success": {
    "zh-CN": "现金流预测查询完成，共 {total} 条",
    "zh-HK": "現金流預測查詢完成，共 {total} 條",
    en: "Cashflow forecast query completed, {total} records",
  },
  "cashflow.query_failed": {
    "zh-CN": "现金流预测查询失败",
    "zh-HK": "現金流預測查詢失敗",
    en: "Cashflow forecast query failed",
  },
  "cashflow.col_contract_number": {
    "zh-CN": "合同编号",
    "zh-HK": "合同編號",
    en: "Contract Number",
  },
  "cashflow.col_contract_name": {
    "zh-CN": "合同名称",
    "zh-HK": "合同名稱",
    en: "Contract Name",
  },
  "cashflow.col_store": {
    "zh-CN": "门店",
    "zh-HK": "門店",
    en: "Store",
  },
  "cashflow.col_currency": {
    "zh-CN": "币种",
    "zh-HK": "幣種",
    en: "Currency",
  },
  "cashflow.col_group": {
    "zh-CN": "分组",
    "zh-HK": "分組",
    en: "Group",
  },
  "cashflow.col_period": {
    "zh-CN": "期间",
    "zh-HK": "期間",
    en: "Period",
  },
  "cashflow.col_period_start": {
    "zh-CN": "期间起",
    "zh-HK": "期間起",
    en: "Period Start",
  },
  "cashflow.col_period_end": {
    "zh-CN": "期间止",
    "zh-HK": "期間止",
    en: "Period End",
  },
  "cashflow.col_cash_outflow": {
    "zh-CN": "现金流出",
    "zh-HK": "現金流出",
    en: "Cash Outflow",
  },
  "cashflow.col_fixed_rent": {
    "zh-CN": "固定租金",
    "zh-HK": "固定租金",
    en: "Fixed Rent",
  },
  "cashflow.col_variable_rent": {
    "zh-CN": "变量租金",
    "zh-HK": "變量租金",
    en: "Variable Rent",
  },
  "cashflow.col_non_lease": {
    "zh-CN": "非租赁",
    "zh-HK": "非租賃",
    en: "Non-lease",
  },
  "cashflow.col_tax": {
    "zh-CN": "税额",
    "zh-HK": "稅額",
    en: "Tax",
  },
  "cashflow.col_total_outflow": {
    "zh-CN": "总现金流出",
    "zh-HK": "總現金流出",
    en: "Total Cash Outflow",
  },
  "cashflow.col_count": {
    "zh-CN": "笔数",
    "zh-HK": "筆數",
    en: "Count",
  },

  // ─── Settings ──────────────────────────────────────────────────
  // FIX-032: the page was a tag manager once. It now holds device sessions,
  // the group discount rate, rent-to-sales policy, budget variance policy and
  // journal materiality — the nav calls it 设置 and the header said 标签总管.
  "settings.title": {
    "zh-CN": "设置",
    "zh-HK": "設置",
    en: "Settings",
  },
  "settings.group_device_sessions": {
    "zh-CN": "设备会话",
    "zh-HK": "設備會話",
    en: "Device Sessions",
  },
  "settings.device_sessions_desc": {
    "zh-CN": "查看当前账号的登录设备，并撤销不再信任的 refresh session。",
    "zh-HK": "查看當前帳號的登入設備，並撤銷不再信任的 refresh session。",
    en: "View signed-in devices and revoke refresh sessions you no longer trust.",
  },
  "settings.sessions_refresh": {
    "zh-CN": "刷新",
    "zh-HK": "刷新",
    en: "Refresh",
  },
  "settings.sessions_logout_all": {
    "zh-CN": "撤销全部会话",
    "zh-HK": "撤銷全部會話",
    en: "Revoke All Sessions",
  },
  "settings.sessions_empty": {
    "zh-CN": "暂无设备会话",
    "zh-HK": "暫無設備會話",
    en: "No device sessions",
  },
  "settings.session_created": {
    "zh-CN": "登录时间",
    "zh-HK": "登入時間",
    en: "Signed in",
  },
  "settings.session_device": {
    "zh-CN": "设备信息",
    "zh-HK": "設備資訊",
    en: "Device",
  },
  "settings.session_ip": {
    "zh-CN": "IP 地址",
    "zh-HK": "IP 地址",
    en: "IP address",
  },
  "settings.session_status": {
    "zh-CN": "状态",
    "zh-HK": "狀態",
    en: "Status",
  },
  "settings.session_active": {
    "zh-CN": "有效",
    "zh-HK": "有效",
    en: "Active",
  },
  "settings.session_revoked_status": {
    "zh-CN": "已撤销",
    "zh-HK": "已撤銷",
    en: "Revoked",
  },
  "settings.session_revoke": {
    "zh-CN": "撤销",
    "zh-HK": "撤銷",
    en: "Revoke",
  },
  "settings.sessions_load_failed": {
    "zh-CN": "设备会话加载失败",
    "zh-HK": "設備會話加載失敗",
    en: "Failed to load device sessions",
  },
  "settings.session_revoked": {
    "zh-CN": "设备会话已撤销",
    "zh-HK": "設備會話已撤銷",
    en: "Device session revoked",
  },
  "settings.session_revoke_failed": {
    "zh-CN": "设备会话撤销失败",
    "zh-HK": "設備會話撤銷失敗",
    en: "Failed to revoke device session",
  },
  "settings.sessions_revoked": {
    "zh-CN": "全部设备会话已撤销",
    "zh-HK": "全部設備會話已撤銷",
    en: "All device sessions revoked",
  },
  "settings.sessions_revoke_failed": {
    "zh-CN": "全部设备会话撤销失败",
    "zh-HK": "全部設備會話撤銷失敗",
    en: "Failed to revoke all device sessions",
  },
  "settings.group_discount_rate": {
    "zh-CN": "集团默认折现率",
    "zh-HK": "集團默認折現率",
    en: "Group Default Discount Rate",
  },
  "settings.current_effective": {
    "zh-CN": "当前生效：",
    "zh-HK": "當前生效：",
    en: "Currently Effective:",
  },
  "settings.discount_rate_desc": {
    "zh-CN": "用于集团统一试算、摊销报表、月结与 IFRS 16 计算的默认折现率。",
    "zh-HK": "用於集團統一試算、攤銷報表、月結與 IFRS 16 計算的默認折現率。",
    en: "Default discount rate used for group unified trial calculations, amortization reports, monthly closing and IFRS 16 calculations.",
  },
  "settings.default_discount_rate": {
    "zh-CN": "默认折现率 (%)",
    "zh-HK": "默認折現率 (%)",
    en: "Default Discount Rate (%)",
  },
  "settings.discount_rate_placeholder": {
    "zh-CN": "例如 5 或 5.25",
    "zh-HK": "例如 5 或 5.25",
    en: "e.g. 5 or 5.25",
  },
  "settings.save_discount_rate": {
    "zh-CN": "保存集团折现率",
    "zh-HK": "保存集團折現率",
    en: "Save Group Discount Rate",
  },
  "settings.save_success": {
    "zh-CN": "集团默认折现率已保存",
    "zh-HK": "集團默認折現率已保存",
    en: "Group default discount rate saved",
  },
  "settings.save_failed": {
    "zh-CN": "保存集团折现率失败",
    "zh-HK": "保存集團折現率失敗",
    en: "Failed to save group discount rate",
  },
  "settings.group_rent_to_sales": {
    "zh-CN": "租售比政策",
    "zh-HK": "租售比政策",
    en: "Rent-to-sales policy",
  },
  "settings.ratio_policy_desc": {
    "zh-CN": "健康线和预警线必须由法人或业态政策明确配置，系统不会自行假设阈值。",
    "zh-HK": "健康線和預警線必須由法人或業態政策明確配置，系統不會自行假設閾值。",
    en: "The healthy and warning lines must be explicitly configured by legal entity or format; the system will not assume them.",
  },
  "settings.ratio_healthy": {
    "zh-CN": "健康线",
    "zh-HK": "健康線",
    en: "Healthy line",
  },
  "settings.ratio_warning": {
    "zh-CN": "预警线",
    "zh-HK": "預警線",
    en: "Warning line",
  },
  "settings.save_ratio_policy": {
    "zh-CN": "保存租售比政策",
    "zh-HK": "保存租售比政策",
    en: "Save ratio policy",
  },
  "settings.ratio_policy_invalid": {
    "zh-CN": "请确认两条线均大于零，且预警线不低于健康线。",
    "zh-HK": "請確認兩條線均大於零，且預警線不低於健康線。",
    en: "Both lines must be positive, and the warning line must not be below the healthy line.",
  },
  "settings.ratio_policy_saved": {
    "zh-CN": "租售比政策已保存",
    "zh-HK": "租售比政策已保存",
    en: "Rent-to-sales policy saved",
  },
  "settings.group_variance_policy": {
    "zh-CN": "预算差异政策",
    "zh-HK": "預算差異政策",
    en: "Budget variance policy",
  },
  "settings.variance_policy_desc": {
    "zh-CN": "配置预算差异的重大性阈值和桥接勾稽容差；系统不会自行采用金额假设。",
    "zh-HK": "配置預算差異的重要性閾值和橋接勾稽容差；系統不會自行採用金額假設。",
    en: "Configure the materiality threshold and bridge tie-out tolerance; the system will not assume monetary values.",
  },
  "settings.variance_materiality": {
    "zh-CN": "重大性阈值",
    "zh-HK": "重要性閾值",
    en: "Materiality threshold",
  },
  "settings.tie_out_tolerance": {
    "zh-CN": "勾稽容差",
    "zh-HK": "勾稽容差",
    en: "Tie-out tolerance",
  },
  "settings.save_variance_policy": {
    "zh-CN": "保存差异政策",
    "zh-HK": "保存差異政策",
    en: "Save variance policy",
  },
  "settings.variance_policy_invalid": {
    "zh-CN": "请填写大于零的重大性阈值和勾稽容差。",
    "zh-HK": "請填寫大於零的重要性閾值和勾稽容差。",
    en: "Enter positive values for the materiality threshold and tie-out tolerance.",
  },
  "settings.variance_policy_saved": {
    "zh-CN": "预算差异政策已保存",
    "zh-HK": "預算差異政策已保存",
    en: "Budget variance policy saved",
  },
  "settings.group_journal_policy": {
    "zh-CN": "会计分录重大性政策",
    "zh-HK": "會計分錄重要性政策",
    en: "Journal entry materiality policy",
  },
  "settings.journal_policy_desc": {
    "zh-CN": "控制月结和事件重算生成分录时的最小金额；未配置时不静默丢弃非零金额。",
    "zh-HK": "控制月結和事件重算生成分錄時的最小金額；未配置時不靜默丟棄非零金額。",
    en: "Controls the minimum amount for month-end and event journal entries; non-zero amounts are not silently dropped when unset.",
  },
  "settings.journal_materiality": {
    "zh-CN": "分录重大性阈值",
    "zh-HK": "分錄重要性閾值",
    en: "Journal entry materiality threshold",
  },
  "settings.save_journal_policy": {
    "zh-CN": "保存分录政策",
    "zh-HK": "保存分錄政策",
    en: "Save journal policy",
  },
  "settings.journal_policy_invalid": {
    "zh-CN": "请填写大于零的分录重大性阈值。",
    "zh-HK": "請填寫大於零的分錄重要性閾值。",
    en: "Enter a positive journal entry materiality threshold.",
  },
  "settings.load_failed": {
    "zh-CN": "加载集团折现率失败",
    "zh-HK": "加載集團折現率失敗",
    en: "Failed to load group discount rate",
  },
  "settings.load_tags_failed": {
    "zh-CN": "加载标签数据失败",
    "zh-HK": "加載標籤數據失敗",
    en: "Failed to load tag data",
  },
  "settings.tag_copied": {
    "zh-CN": "标签已复制",
    "zh-HK": "標籤已複製",
    en: "Tag copied",
  },
  "settings.copy_failed": {
    "zh-CN": "复制失败，请手动复制",
    "zh-HK": "複製失敗，請手動複製",
    en: "Copy failed, please copy manually",
  },
  "settings.stat_total_tags": {
    "zh-CN": "标签总数",
    "zh-HK": "標籤總數",
    en: "Total Tags",
  },
  "settings.stat_tagged_contracts": {
    "zh-CN": "已打标签合同数",
    "zh-HK": "已打標籤合同數",
    en: "Tagged Contracts",
  },
  "settings.stat_avg_contracts_per_tag": {
    "zh-CN": "平均每标签合同数",
    "zh-HK": "平均每標籤合同數",
    en: "Avg Contracts per Tag",
  },
  "settings.search_tag": {
    "zh-CN": "搜索标签：",
    "zh-HK": "搜索標籤：",
    en: "Search Tags:",
  },
  "settings.search_tag_placeholder": {
    "zh-CN": "输入标签名称搜索",
    "zh-HK": "輸入標籤名稱搜索",
    en: "Enter tag name to search",
  },
  "settings.min_contract_count": {
    "zh-CN": "最低合同数：",
    "zh-HK": "最低合同數：",
    en: "Min Contract Count:",
  },
  "settings.min_contract_all": {
    "zh-CN": "全部",
    "zh-HK": "全部",
    en: "All",
  },
  "settings.empty_no_tags": {
    "zh-CN": "暂无标签数据。请在合同创建/编辑页面为合同添加标签。",
    "zh-HK": "暫無標籤數據。請在合同創建/編輯頁面為合同添加標籤。",
    en: "No tag data yet. Please add tags to contracts on the contract creation/editing page.",
  },
  "settings.empty_no_match": {
    "zh-CN": "无匹配标签",
    "zh-HK": "無匹配標籤",
    en: "No matching tags",
  },
  "settings.col_tag": {
    "zh-CN": "标签",
    "zh-HK": "標籤",
    en: "Tag",
  },
  "settings.col_contract_count": {
    "zh-CN": "合同数",
    "zh-HK": "合同數",
    en: "Contract Count",
  },
  "settings.col_example_contract": {
    "zh-CN": "示例合同编号",
    "zh-HK": "示例合同編號",
    en: "Example Contract Number",
  },
  "settings.col_action": {
    "zh-CN": "操作",
    "zh-HK": "操作",
    en: "Action",
  },
  "settings.action_copy_tag": {
    "zh-CN": "复制标签",
    "zh-HK": "複製標籤",
    en: "Copy Tag",
  },
  "settings.action_view_contracts": {
    "zh-CN": "查看合同",
    "zh-HK": "查看合同",
    en: "View Contracts",
  },
  "settings.action_view_reports": {
    "zh-CN": "查看报表",
    "zh-HK": "查看報表",
    en: "View Reports",
  },
  "settings.modal_tag_contracts": {
    "zh-CN": "标签「{tag}」关联合同",
    "zh-HK": "標籤「{tag}」關聯合同",
    en: "Contracts associated with tag \"{tag}\"",
  },
  "settings.modal_no_contracts": {
    "zh-CN": "无关联合同",
    "zh-HK": "無關聯合同",
    en: "No associated contracts",
  },
  "settings.modal_contract_number": {
    "zh-CN": "合同编号",
    "zh-HK": "合同編號",
    en: "Contract Number",
  },
  "settings.modal_contract_name": {
    "zh-CN": "合同名称",
    "zh-HK": "合同名稱",
    en: "Contract Name",
  },

  // ─── Upload ────────────────────────────────────────────────────

  // ─── Audit Logs ────────────────────────────────────────────────
  "audit.title": {
    "zh-CN": "审计日志",
    "zh-HK": "審計日誌",
    en: "Audit Logs",
  },
  "audit.subtitle": {
    "zh-CN": "共 {total} 条记录 · 最近 30 天",
    "zh-HK": "共 {total} 條記錄 · 最近 30 天",
    en: "{total} records · Last 30 days",
  },
  "audit.filter_table": {
    "zh-CN": "表名",
    "zh-HK": "表名",
    en: "Table Name",
  },
  "audit.filter_action": {
    "zh-CN": "操作类型",
    "zh-HK": "操作類型",
    en: "Action Type",
  },
  "audit.filter_record_id": {
    "zh-CN": "记录ID",
    "zh-HK": "記錄ID",
    en: "Record ID",
  },
  "audit.filter_record_placeholder": {
    "zh-CN": "UUID 搜索",
    "zh-HK": "UUID 搜索",
    en: "UUID Search",
  },
  "audit.filter_run_id": {
    "zh-CN": "Run ID",
    "zh-HK": "Run ID",
    en: "Run ID",
  },
  "audit.filter_tool_name": {
    "zh-CN": "Tool 名称",
    "zh-HK": "Tool 名稱",
    en: "Tool Name",
  },
  "audit.filter_run_placeholder": {
    "zh-CN": "运行 ID 搜索",
    "zh-HK": "運行 ID 搜索",
    en: "Run ID Search",
  },
  "audit.filter_tool_placeholder": {
    "zh-CN": "例如 lease.contract.get",
    "zh-HK": "例如 lease.contract.get",
    en: "e.g. lease.contract.get",
  },
  "audit.filter_time_range": {
    "zh-CN": "时间范围",
    "zh-HK": "時間範圍",
    en: "Time Range",
  },
  "audit.query": {
    "zh-CN": "查询",
    "zh-HK": "查詢",
    en: "Query",
  },
  "audit.reset": {
    "zh-CN": "重置",
    "zh-HK": "重置",
    en: "Reset",
  },
  "audit.col_time": {
    "zh-CN": "时间",
    "zh-HK": "時間",
    en: "Time",
  },
  "audit.col_operator": {
    "zh-CN": "操作人",
    "zh-HK": "操作人",
    en: "Operator",
  },
  "audit.col_action": {
    "zh-CN": "操作类型",
    "zh-HK": "操作類型",
    en: "Action Type",
  },
  "audit.col_table": {
    "zh-CN": "表名",
    "zh-HK": "表名",
    en: "Table Name",
  },
  "audit.col_record_id": {
    "zh-CN": "记录ID",
    "zh-HK": "記錄ID",
    en: "Record ID",
  },
  "audit.col_old_value": {
    "zh-CN": "旧值",
    "zh-HK": "舊值",
    en: "Old Value",
  },
  "audit.col_new_value": {
    "zh-CN": "新值",
    "zh-HK": "新值",
    en: "New Value",
  },
  "audit.view_old": {
    "zh-CN": "变更前数据",
    "zh-HK": "變更前數據",
    en: "Data Before Change",
  },
  "audit.view_new": {
    "zh-CN": "变更后数据",
    "zh-HK": "變更後數據",
    en: "Data After Change",
  },
  "audit.view": {
    "zh-CN": "查看",
    "zh-HK": "查看",
    en: "View",
  },
  "audit.none": {
    "zh-CN": "无",
    "zh-HK": "無",
    en: "None",
  },
  "audit.total_records": {
    "zh-CN": "共 {total} 条记录",
    "zh-HK": "共 {total} 條記錄",
    en: "{total} records total",
  },
  "audit.query_failed": {
    "zh-CN": "查询审计日志失败",
    "zh-HK": "查詢審計日誌失敗",
    en: "Failed to query audit logs",
  },
  "audit.table_all": {
    "zh-CN": "全部表",
    "zh-HK": "全部表",
    en: "All Tables",
  },
  "audit.table_contract": {
    "zh-CN": "合同",
    "zh-HK": "合同",
    en: "Contract",
  },
  "audit.table_event": {
    "zh-CN": "事件",
    "zh-HK": "事件",
    en: "Event",
  },
  "audit.table_schedule": {
    "zh-CN": "付款计划",
    "zh-HK": "付款計劃",
    en: "Payment Schedule",
  },
  "audit.table_batch": {
    "zh-CN": "月结批次",
    "zh-HK": "月結批次",
    en: "Monthly Closing Batch",
  },
  "audit.table_entry": {
    "zh-CN": "会计分录",
    "zh-HK": "會計分錄",
    en: "Journal Entry",
  },
  "audit.table_lock": {
    "zh-CN": "期间锁账",
    "zh-HK": "期間鎖賬",
    en: "Period Lock",
  },
  "audit.action_all": {
    "zh-CN": "全部操作",
    "zh-HK": "全部操作",
    en: "All Actions",
  },
  "audit.action_create": {
    "zh-CN": "创建",
    "zh-HK": "創建",
    en: "Create",
  },
  "audit.action_update": {
    "zh-CN": "更新",
    "zh-HK": "更新",
    en: "Update",
  },
  "audit.action_delete": {
    "zh-CN": "删除",
    "zh-HK": "刪除",
    en: "Delete",
  },
  "audit.action_submit": {
    "zh-CN": "提交",
    "zh-HK": "提交",
    en: "Submit",
  },
  "audit.action_review_pass": {
    "zh-CN": "复核通过",
    "zh-HK": "複核通過",
    en: "Review Passed",
  },
  "audit.action_review_reject": {
    "zh-CN": "复核退回",
    "zh-HK": "複核退回",
    en: "Review Rejected",
  },
  "audit.action_approve": {
    "zh-CN": "审批通过",
    "zh-HK": "審批通過",
    en: "Approve",
  },
  "audit.action_reject": {
    "zh-CN": "驳回",
    "zh-HK": "駁回",
    en: "Reject",
  },
  "audit.action_generate": {
    "zh-CN": "生成",
    "zh-HK": "生成",
    en: "Generate",
  },
  "audit.action_post": {
    "zh-CN": "过账",
    "zh-HK": "過賬",
    en: "Post",
  },
  "audit.action_lock": {
    "zh-CN": "锁账",
    "zh-HK": "鎖賬",
    en: "Lock",
  },
  "audit.action_unlock": {
    "zh-CN": "解锁",
    "zh-HK": "解鎖",
    en: "Unlock",
  },
  "audit.action_recalculate": {
    "zh-CN": "重算",
    "zh-HK": "重算",
    en: "Recalculate",
  },

  // ─── Admin Users ───────────────────────────────────────────────
  "admin_users.title": {
    "zh-CN": "用户管理",
    "zh-HK": "用戶管理",
    en: "User Management",
  },
  "admin_users.subtitle": {
    "zh-CN": "共 {count} 个用户",
    "zh-HK": "共 {count} 個用戶",
    en: "{count} users",
  },
  "admin_users.new_user": {
    "zh-CN": "新建用户",
    "zh-HK": "新建用戶",
    en: "New User",
  },
  "admin_users.need_admin": {
    "zh-CN": "需要管理员权限",
    "zh-HK": "需要管理員權限",
    en: "Admin privileges required",
  },
  "admin_users.load_failed": {
    "zh-CN": "获取用户列表失败",
    "zh-HK": "獲取用戶列表失敗",
    en: "Failed to load user list",
  },
  "admin_users.create_success": {
    "zh-CN": "用户创建成功",
    "zh-HK": "用戶創建成功",
    en: "User created successfully",
  },
  "admin_users.create_failed": {
    "zh-CN": "创建用户失败",
    "zh-HK": "創建用戶失敗",
    en: "Failed to create user",
  },
  "admin_users.col_username": {
    "zh-CN": "用户名",
    "zh-HK": "用戶名",
    en: "Username",
  },
  "admin_users.col_email": {
    "zh-CN": "邮箱",
    "zh-HK": "郵箱",
    en: "Email",
  },
  "admin_users.col_role": {
    "zh-CN": "角色",
    "zh-HK": "角色",
    en: "Role",
  },
  "admin_users.col_legal_entity": {
    "zh-CN": "法人主体",
    "zh-HK": "法人主體",
    en: "Legal Entity",
  },
  "admin_users.col_status": {
    "zh-CN": "状态",
    "zh-HK": "狀態",
    en: "Status",
  },
  "admin_users.col_created_at": {
    "zh-CN": "创建时间",
    "zh-HK": "創建時間",
    en: "Created At",
  },
  "admin_users.status_active": {
    "zh-CN": "活跃",
    "zh-HK": "活躍",
    en: "Active",
  },
  "admin_users.status_disabled": {
    "zh-CN": "禁用",
    "zh-HK": "禁用",
    en: "Disabled",
  },
  "admin_users.role_admin": {
    "zh-CN": "管理员",
    "zh-HK": "管理員",
    en: "Admin",
  },
  "admin_users.role_reviewer": {
    "zh-CN": "复核员",
    "zh-HK": "複核員",
    en: "Reviewer",
  },
  "admin_users.role_approver": {
    "zh-CN": "审批员",
    "zh-HK": "審批員",
    en: "Approver",
  },
  "admin_users.role_user": {
    "zh-CN": "普通用户",
    "zh-HK": "普通用戶",
    en: "User",
  },
  "admin_users.modal_title": {
    "zh-CN": "新建用户",
    "zh-HK": "新建用戶",
    en: "New User",
  },
  "admin_users.label_username": {
    "zh-CN": "用户名",
    "zh-HK": "用戶名",
    en: "Username",
  },
  "admin_users.username_placeholder": {
    "zh-CN": "请输入用户名",
    "zh-HK": "請輸入用戶名",
    en: "Please enter username",
  },
  "admin_users.username_min": {
    "zh-CN": "至少3个字符",
    "zh-HK": "至少3個字符",
    en: "At least 3 characters",
  },
  "admin_users.label_email": {
    "zh-CN": "邮箱",
    "zh-HK": "郵箱",
    en: "Email",
  },
  "admin_users.email_placeholder": {
    "zh-CN": "请输入邮箱",
    "zh-HK": "請輸入郵箱",
    en: "Please enter email",
  },
  "admin_users.email_invalid": {
    "zh-CN": "请输入有效邮箱",
    "zh-HK": "請輸入有效郵箱",
    en: "Please enter a valid email",
  },
  "admin_users.label_password": {
    "zh-CN": "密码",
    "zh-HK": "密碼",
    en: "Password",
  },
  "admin_users.password_placeholder": {
    "zh-CN": "请输入密码",
    "zh-HK": "請輸入密碼",
    en: "Please enter password",
  },
  "admin_users.password_min": {
    "zh-CN": "至少6个字符",
    "zh-HK": "至少6個字符",
    en: "At least 6 characters",
  },
  "admin_users.label_role": {
    "zh-CN": "角色",
    "zh-HK": "角色",
    en: "Role",
  },
  "admin_users.role_placeholder": {
    "zh-CN": "请选择角色",
    "zh-HK": "請選擇角色",
    en: "Please select role",
  },
  "admin_users.label_legal_entity": {
    "zh-CN": "所属法人",
    "zh-HK": "所屬法人",
    en: "Legal Entity",
  },
  "admin_users.legal_entity_placeholder": {
    "zh-CN": "请选择所属法人",
    "zh-HK": "請選擇所屬法人",
    en: "Please select legal entity",
  },
  "admin_users.cancel": {
    "zh-CN": "取消",
    "zh-HK": "取消",
    en: "Cancel",
  },
  "admin_users.create": {
    "zh-CN": "创建",
    "zh-HK": "創建",
    en: "Create",
  },

  // ─── Admin Login ───────────────────────────────────────────────
  "admin_login.title": {
    "zh-CN": "管理后台登录",
    "zh-HK": "管理後台登錄",
    en: "Admin Login",
  },
  "admin_login.subtitle": {
    "zh-CN": "零售经营分析工作站 — 管理员通道",
    "zh-HK": "零售經營分析工作站 — 管理員通道",
    en: "Retail Performance Workstation — Admin Channel",
  },
  "admin_login.not_admin": {
    "zh-CN": "非管理员账号，请使用普通登录入口",
    "zh-HK": "非管理員賬號，請使用普通登錄入口",
    en: "Not an admin account, please use the regular login",
  },
  "admin_login.success": {
    "zh-CN": "管理员登录成功！",
    "zh-HK": "管理員登錄成功！",
    en: "Admin login successful!",
  },
  "admin_login.failed": {
    "zh-CN": "登录失败",
    "zh-HK": "登錄失敗",
    en: "Login failed",
  },
  "admin_login.username_required": {
    "zh-CN": "请输入管理员用户名",
    "zh-HK": "請輸入管理員用戶名",
    en: "Please enter admin username",
  },
  "admin_login.username_placeholder": {
    "zh-CN": "管理员用户名",
    "zh-HK": "管理員用戶名",
    en: "Admin username",
  },
  "admin_login.password_required": {
    "zh-CN": "请输入密码",
    "zh-HK": "請輸入密碼",
    en: "Please enter password",
  },
  "admin_login.password_placeholder": {
    "zh-CN": "密码",
    "zh-HK": "密碼",
    en: "Password",
  },
  "admin_login.login_button": {
    "zh-CN": "管理员登录",
    "zh-HK": "管理員登錄",
    en: "Admin Login",
  },
  "admin_login.back_to_user": {
    "zh-CN": "返回普通用户登录",
    "zh-HK": "返回普通用戶登錄",
    en: "Back to user login",
  },

  // ─── AI Chat Draft Panel ───────────────────────────────────────
  "ai.delete_session": {
    "zh-CN": "删除会话",
    "zh-HK": "刪除會話",
    en: "Delete Session",
  },
  "ai.draft_panel_title": {
    "zh-CN": "合同台账草稿确认",
    "zh-HK": "合同台賬草稿確認",
    en: "Contract Ledger Draft Confirmation",
  },
  "ai.draft_panel_subtitle": {
    "zh-CN": "共 {total} 份 | 总体置信度 {confidence}%",
    "zh-HK": "共 {total} 份 | 總體置信度 {confidence}%",
    en: "{total} contracts | Overall confidence {confidence}%",
  },
  "ai.select_all": {
    "zh-CN": "全选",
    "zh-HK": "全選",
    en: "Select All",
  },
  "ai.deselect_all": {
    "zh-CN": "取消全选",
    "zh-HK": "取消全選",
    en: "Deselect All",
  },
  "ai.skip": {
    "zh-CN": "跳过",
    "zh-HK": "跳過",
    en: "Skip",
  },
  "ai.draft_warning": {
    "zh-CN": "部分合同存在缺失字段或低置信度识别结果，请逐条核对后再确认入库",
    "zh-HK": "部分合同存在缺失字段或低置信度識別結果，請逐條核對後再確認入庫",
    en: "Some contracts have missing fields or low-confidence recognition results. Please review each one before confirming.",
  },
  "ai.draft_contract_number": {
    "zh-CN": "合同编号",
    "zh-HK": "合同編號",
    en: "Contract Number",
  },
  "ai.draft_contract_name": {
    "zh-CN": "合同名称",
    "zh-HK": "合同名稱",
    en: "Contract Name",
  },
  "ai.draft_currency": {
    "zh-CN": "币种",
    "zh-HK": "幣種",
    en: "Currency",
  },
  "ai.draft_lessee": {
    "zh-CN": "承租方",
    "zh-HK": "承租方",
    en: "Lessee",
  },
  "ai.draft_lessor": {
    "zh-CN": "出租方",
    "zh-HK": "出租方",
    en: "Lessor",
  },
  "ai.draft_store": {
    "zh-CN": "门店",
    "zh-HK": "門店",
    en: "Store",
  },
  "ai.draft_commencement_date": {
    "zh-CN": "租赁起始日",
    "zh-HK": "租賃起始日",
    en: "Commencement Date",
  },
  "ai.draft_lease_start": {
    "zh-CN": "租赁开始日",
    "zh-HK": "租賃開始日",
    en: "Lease Start Date",
  },
  "ai.draft_lease_end": {
    "zh-CN": "租期结束日",
    "zh-HK": "租期結束日",
    en: "Lease End Date",
  },
  "ai.draft_fixed_rent": {
    "zh-CN": "固定租金",
    "zh-HK": "固定租金",
    en: "Fixed Rent",
  },
  "ai.draft_payment_timing": {
    "zh-CN": "付款时点",
    "zh-HK": "付款時點",
    en: "Payment Timing",
  },
  "ai.draft_discount_rate": {
    "zh-CN": "折现率",
    "zh-HK": "折現率",
    en: "Discount Rate",
  },
  "ai.draft_confidence": {
    "zh-CN": "置信度 {value}%",
    "zh-HK": "置信度 {value}%",
    en: "Confidence {value}%",
  },
  "ai.draft_missing_fields": {
    "zh-CN": "缺失: {fields}",
    "zh-HK": "缺失: {fields}",
    en: "Missing: {fields}",
  },
  "ai.draft_selected_count": {
    "zh-CN": "已选择 {selected} / {total} 份合同",
    "zh-HK": "已選擇 {selected} / {total} 份合同",
    en: "Selected {selected} / {total} contracts",
  },
  "ai.draft_confirm_import": {
    "zh-CN": "确认入库",
    "zh-HK": "確認入庫",
    en: "Confirm Import",
  },
  "ai.request_failed": {
    "zh-CN": "请求失败：{error}",
    "zh-HK": "請求失敗：{error}",
    en: "Request failed: {error}",
  },
  "ai.unknown_error": {
    "zh-CN": "未知错误",
    "zh-HK": "未知錯誤",
    en: "Unknown error",
  },
  "ai.delete_session_title": {
    "zh-CN": "删除会话",
    "zh-HK": "刪除會話",
    en: "Delete Session",
  },
  "ai.delete_session_content": {
    "zh-CN": "确定要删除这个会话吗？此操作不可撤销。",
    "zh-HK": "確定要刪除這個會話嗎？此操作不可撤銷。",
    en: "Are you sure you want to delete this session? This action cannot be undone.",
  },
  "ai.delete": {
    "zh-CN": "删除",
    "zh-HK": "刪除",
    en: "Delete",
  },
  "ai.cancel": {
    "zh-CN": "取消",
    "zh-HK": "取消",
    en: "Cancel",
  },
  "ai.assistant_name": {
    "zh-CN": "零售经营分析助手",
    "zh-HK": "零售經營分析助手",
    en: "Retail Performance Assistant",
  },
  "ai.agent_trace_title": {
    "zh-CN": "Agent 技能执行轨迹",
    "zh-HK": "Agent 技能執行軌跡",
    en: "Agent Skill Trace",
  },
  "ai.view_trace": {
    "zh-CN": "查看 Trace",
    "zh-HK": "查看 Trace",
    en: "View Trace",
  },
  "ai.agent_trace_empty": {
    "zh-CN": "暂无可用的运行轨迹",
    "zh-HK": "暫無可用的運行軌跡",
    en: "No run trace is available",
  },
  "ai.confirm": {
    "zh-CN": "确认",
    "zh-HK": "確認",
    en: "Confirm",
  },
  "ai.run_cancel_title": {
    "zh-CN": "取消 Agent 运行",
    "zh-HK": "取消 Agent 運行",
    en: "Cancel Agent Run",
  },
  "ai.run_cancel_content": {
    "zh-CN": "确定要取消当前运行吗？",
    "zh-HK": "確定要取消目前運行嗎？",
    en: "Cancel the current run?",
  },
  "ai.run_cancel": {
    "zh-CN": "取消运行",
    "zh-HK": "取消運行",
    en: "Cancel Run",
  },
  "ai.run_steer_title": {
    "zh-CN": "调整 Agent 运行方向",
    "zh-HK": "調整 Agent 運行方向",
    en: "Steer Agent Run",
  },
  "ai.run_follow_up_title": {
    "zh-CN": "创建运行后续任务",
    "zh-HK": "建立運行後續任務",
    en: "Create Run Follow-up",
  },
  "ai.run_branch_title": {
    "zh-CN": "从运行检查点创建分支",
    "zh-HK": "從運行檢查點建立分支",
    en: "Branch from Run Checkpoint",
  },
  "ai.run_control_placeholder": {
    "zh-CN": "输入指令",
    "zh-HK": "輸入指令",
    en: "Enter an instruction",
  },
  "ai.run_control_required": {
    "zh-CN": "请输入指令",
    "zh-HK": "請輸入指令",
    en: "Enter an instruction",
  },
  "ai.run_control_sent": {
    "zh-CN": "运行控制指令已提交",
    "zh-HK": "運行控制指令已提交",
    en: "Run control command submitted",
  },
  "ai.run_steer": {
    "zh-CN": "调整方向",
    "zh-HK": "調整方向",
    en: "Steer",
  },
  "ai.run_follow_up": {
    "zh-CN": "后续任务",
    "zh-HK": "後續任務",
    en: "Follow-up",
  },
  "ai.run_branch": {
    "zh-CN": "创建分支",
    "zh-HK": "建立分支",
    en: "Branch",
  },
  "ai.agent_skills": {
    "zh-CN": "Agent skills:",
    "zh-HK": "Agent skills:",
    en: "Agent skills:",
  },
  "ai.skill_excel_ledger": {
    "zh-CN": "Excel 台账",
    "zh-HK": "Excel 台賬",
    en: "Excel Ledger",
  },
  "ai.skill_excel_ledger_prompt": {
    "zh-CN": "请解析这个 Excel 台账并生成合同草稿卡片。请按非标准台账处理，先做字段理解、缺失字段和范围判断提示，等待我确认后再创建 draft 合同。",
    "zh-HK": "請解析這個 Excel 台賬並生成合同草稿卡片。請按非標準台賬處理，先做字段理解、缺失字段和範圍判斷提示，等待我確認後再創建 draft 合同。",
    en: "Parse this Excel ledger and generate contract draft cards. Treat it as a non-standard ledger, identify fields semantically, surface missing fields and scope judgments, and wait for my confirmation before creating draft contracts.",
  },
  "ai.skill_contract_review": {
    "zh-CN": "合同复核",
    "zh-HK": "合同覆核",
    en: "Contract Review",
  },
  "ai.skill_contract_review_prompt": {
    "zh-CN": "请复核这份租赁合同，提取关键条款、租期、租金、续租/终止选择权、非租赁成分和 IFRS 16 范围判断，并列出需要人工确认的问题。",
    "zh-HK": "請覆核這份租賃合同，提取關鍵條款、租期、租金、續租/終止選擇權、非租賃成分和 IFRS 16 範圍判斷，並列出需要人工確認的問題。",
    en: "Review this lease contract, extract key terms, lease term, rent, renewal/termination options, non-lease components, IFRS 16 scope judgment, and list questions requiring human confirmation.",
  },
  "ai.skill_payment_schedule": {
    "zh-CN": "租金表",
    "zh-HK": "租金表",
    en: "Rent Schedule",
  },
  "ai.skill_payment_schedule_prompt": {
    "zh-CN": "请解析这个租金表，识别付款期间、应付日期、金额、先付/后付、固定/变量租金和非租赁成分，生成付款计划草稿并等待我确认。",
    "zh-HK": "請解析這個租金表，識別付款期間、應付日期、金額、先付/後付、固定/變量租金和非租賃成分，生成付款計劃草稿並等待我確認。",
    en: "Parse this rent schedule, identify covered periods, due dates, amounts, prepaid/postpaid timing, fixed/variable rent, non-lease components, and generate payment schedule drafts for confirmation.",
  },
  "ai.skill_audit_pack": {
    "zh-CN": "审计包",
    "zh-HK": "審計包",
    en: "Audit Pack",
  },
  "ai.skill_audit_pack_prompt": {
    "zh-CN": "请为当前合同、法人或期间整理审计包清单，包含原始文件、AI 提取字段、人工确认事项、计量结果、分录和审批留痕。",
    "zh-HK": "請為當前合同、法人或期間整理審計包清單，包含原始文件、AI 提取字段、人工確認事項、計量結果、分錄和審批留痕。",
    en: "Prepare an audit pack checklist for the current contract, legal entity, or period, including source files, AI extracted fields, human confirmations, measurement results, journal entries, and approval trail.",
  },
  "ai.skill_retail_operations": {
    "zh-CN": "零售经营分析",
    "zh-HK": "零售經營分析",
    en: "Retail Operations",
  },
  "ai.skill_retail_operations_prompt": {
    "zh-CN": "请读取当前经营脉搏；如有关注门店，再给出门店诊断或确定性经营情景。所有数字请保留来源、覆盖和数据集上下文，不要写入业务行动。",
    "zh-HK": "請讀取當前經營脈搏；如有關注門店，再給出門店診斷或確定性經營情景。所有數字請保留來源、覆蓋和數據集上下文，不要寫入業務行動。",
    en: "Read the current operating pulse, then inspect an attention store or evaluate a deterministic operating scenario when requested. Keep source, coverage, and dataset context; do not write a business action.",
  },
  "ai.agent_review_title": {
    "zh-CN": "需要你确认的问题",
    "zh-HK": "需要你確認的問題",
    en: "Questions Requiring Review",
  },
  "ai.agent_severity_critical": {
    "zh-CN": "关键",
    "zh-HK": "關鍵",
    en: "Critical",
  },
  "ai.agent_severity_warning": {
    "zh-CN": "需确认",
    "zh-HK": "需確認",
    en: "Review",
  },
  "ai.agent_severity_info": {
    "zh-CN": "提示",
    "zh-HK": "提示",
    en: "Info",
  },
  "ai.agent_status_completed": {
    "zh-CN": "已完成",
    "zh-HK": "已完成",
    en: "Completed",
  },
  "ai.agent_status_needs_review": {
    "zh-CN": "需人工确认",
    "zh-HK": "需人工確認",
    en: "Needs Review",
  },
  "ai.agent_status_failed": {
    "zh-CN": "失败",
    "zh-HK": "失敗",
    en: "Failed",
  },
  "ai.agent_status_running": {
    "zh-CN": "执行中",
    "zh-HK": "執行中",
    en: "Running",
  },
  "ai.agent_status_pending": {
    "zh-CN": "等待中",
    "zh-HK": "等待中",
    en: "Pending",
  },
  "ai.batch_create_result": {
    "zh-CN": "批量创建完成：成功 {success} 份，失败 {failed} 份。{details}",
    "zh-HK": "批量創建完成：成功 {success} 份，失敗 {failed} 份。{details}",
    en: "Batch creation completed: {success} succeeded, {failed} failed. {details}",
  },
  "ai.batch_create_success": {
    "zh-CN": "成功创建 {count} 份合同",
    "zh-HK": "成功創建 {count} 份合同",
    en: "Successfully created {count} contracts",
  },
  "ai.batch_create_failed": {
    "zh-CN": "批量创建失败: {error}",
    "zh-HK": "批量創建失敗: {error}",
    en: "Batch creation failed: {error}",
  },
  "ai.schedule_panel_title": {
    "zh-CN": "付款计划草稿",
    "zh-HK": "付款計劃草稿",
    en: "Payment Schedule Drafts",
  },
  "ai.schedule_bind_contract_first": {
    "zh-CN": "请先绑定目标合同。付款计划必须挂到具体合同后才能导入。",
    "zh-HK": "請先綁定目標合同。付款計劃必須掛到具體合同後才能導入。",
    en: "Bind a target contract first. Payment schedules must be attached to a specific contract before import.",
  },
  "ai.schedule_review_warning": {
    "zh-CN": "需要人工核对付款期间、应付日、金额、先付/后付、变量租金和非租赁成分。",
    "zh-HK": "需要人工核對付款期間、應付日、金額、先付/後付、變量租金和非租賃成分。",
    en: "Review covered periods, due dates, amounts, prepaid/postpaid timing, variable rent, and non-lease components.",
  },
  "ai.schedule_period_start": {
    "zh-CN": "期间开始",
    "zh-HK": "期間開始",
    en: "Period Start",
  },
  "ai.schedule_period_end": {
    "zh-CN": "期间结束",
    "zh-HK": "期間結束",
    en: "Period End",
  },
  "ai.schedule_due_date": {
    "zh-CN": "应付日",
    "zh-HK": "應付日",
    en: "Due Date",
  },
  "ai.schedule_amount": {
    "zh-CN": "金额",
    "zh-HK": "金額",
    en: "Amount",
  },
  "ai.schedule_amount_type": {
    "zh-CN": "金额类型",
    "zh-HK": "金額類型",
    en: "Amount Type",
  },
  "ai.schedule_is_fixed": {
    "zh-CN": "固定租金",
    "zh-HK": "固定租金",
    en: "Fixed Rent",
  },
  "ai.schedule_is_lease_component": {
    "zh-CN": "租赁成分",
    "zh-HK": "租賃成分",
    en: "Lease Component",
  },
  "ai.schedule_variable_rent": {
    "zh-CN": "变量租金",
    "zh-HK": "變量租金",
    en: "Variable Rent",
  },
  "ai.schedule_non_lease_component": {
    "zh-CN": "非租赁成分",
    "zh-HK": "非租賃成分",
    en: "Non-lease Component",
  },
  "ai.schedule_confirm_import": {
    "zh-CN": "导入付款计划",
    "zh-HK": "導入付款計劃",
    en: "Import Payment Schedules",
  },
  "ai.schedule_import_success": {
    "zh-CN": "成功导入 {count} 笔付款计划",
    "zh-HK": "成功導入 {count} 筆付款計劃",
    en: "Successfully imported {count} payment schedules",
  },
  "ai.schedule_import_failed": {
    "zh-CN": "付款计划导入失败: {error}",
    "zh-HK": "付款計劃導入失敗: {error}",
    en: "Payment schedule import failed: {error}",
  },
  "ai.upload_file_tooltip": {
    "zh-CN": "在 AI Chat 上传文件",
    "zh-HK": "在 AI Chat 上傳文件",
    en: "Upload Files in AI Chat",
  },
  "ai.continue_from_message": {
    "zh-CN": "继续此消息",
    "zh-HK": "繼續此訊息",
    en: "Continue from Message",
  },
  "ai.continue_from_run": {
    "zh-CN": "继续此运行",
    "zh-HK": "繼續此運行",
    en: "Continue from Run",
  },
  "ai.continue_from_artifact": {
    "zh-CN": "继续此草稿",
    "zh-HK": "繼續此草稿",
    en: "Continue from Artifact",
  },
  "ai.continue_from_action": {
    "zh-CN": "继续此动作",
    "zh-HK": "繼續此動作",
    en: "Continue from Action",
  },
  "ai.review_action_history": {
    "zh-CN": "人工动作历史",
    "zh-HK": "人工動作歷史",
    en: "Review Action History",
  },
  "ai.review_action_confirm": {
    "zh-CN": "已确认",
    "zh-HK": "已確認",
    en: "Confirmed",
  },
  "ai.review_action_skip": {
    "zh-CN": "已跳过",
    "zh-HK": "已跳過",
    en: "Skipped",
  },
  "ai.review_action_import": {
    "zh-CN": "已导入",
    "zh-HK": "已導入",
    en: "Imported",
  },
  "ai.review_action_create_draft": {
    "zh-CN": "已创建草稿",
    "zh-HK": "已建立草稿",
    en: "Draft Created",
  },
  "ai.review_action_reject": {
    "zh-CN": "已驳回",
    "zh-HK": "已駁回",
    en: "Rejected",
  },
  "ai.disclaimer": {
    "zh-CN": "AI 生成内容仅供参考，请以系统正式数据为准",
    "zh-HK": "AI 生成內容僅供參考，請以系統正式數據為準",
    en: "AI-generated content is for reference only. Please refer to official system data.",
  },

  // ─── Reports (supplemental) ────────────────────────────────────
  "reports.col_period": {
    "zh-CN": "期间",
    "zh-HK": "期間",
    en: "Period",
  },
  "reports.col_period_start": {
    "zh-CN": "期间起",
    "zh-HK": "期間起",
    en: "Period Start",
  },
  "reports.col_period_end": {
    "zh-CN": "期间止",
    "zh-HK": "期間止",
    en: "Period End",
  },
  "reports.group_liability": {
    "zh-CN": "租赁负债 Roll-forward",
    "zh-HK": "租賃負債 Roll-forward",
    en: "Lease Liability Roll-forward",
  },
  "reports.col_opening_liability": {
    "zh-CN": "负债期初",
    "zh-HK": "負債期初",
    en: "Opening Liability",
  },
  "reports.col_interest": {
    "zh-CN": "负债利息",
    "zh-HK": "負債利息",
    en: "Interest",
  },
  "reports.col_payment": {
    "zh-CN": "负债付款",
    "zh-HK": "負債付款",
    en: "Payment",
  },
  "reports.col_prepaid": {
    "zh-CN": "先付租金",
    "zh-HK": "先付租金",
    en: "Prepaid Rent",
  },
  "reports.col_liability_adjustment": {
    "zh-CN": "负债调整",
    "zh-HK": "負債調整",
    en: "Liability Adjustment",
  },
  "reports.col_closing_liability": {
    "zh-CN": "负债期末",
    "zh-HK": "負債期末",
    en: "Closing Liability",
  },
  "reports.group_rou": {
    "zh-CN": "使用权资产 Roll-forward",
    "zh-HK": "使用權資產 Roll-forward",
    en: "ROU Asset Roll-forward",
  },
  "reports.col_opening_rou": {
    "zh-CN": "资产期初",
    "zh-HK": "資產期初",
    en: "Opening ROU",
  },
  "reports.col_depreciation": {
    "zh-CN": "资产折旧",
    "zh-HK": "資產折舊",
    en: "Depreciation",
  },
  "reports.col_impairment": {
    "zh-CN": "资产减值",
    "zh-HK": "資產減值",
    en: "Impairment",
  },
  "reports.col_rou_adjustment": {
    "zh-CN": "资产调整",
    "zh-HK": "資產調整",
    en: "ROU Adjustment",
  },
  "reports.col_closing_rou": {
    "zh-CN": "资产期末",
    "zh-HK": "資產期末",
    en: "Closing ROU",
  },
  "reports.group_expenses": {
    "zh-CN": "期间费用与调整",
    "zh-HK": "期間費用與調整",
    en: "Period Expenses & Adjustments",
  },
  "reports.col_variable_rent": {
    "zh-CN": "变动租金",
    "zh-HK": "變動租金",
    en: "Variable Rent",
  },
  "reports.col_non_lease": {
    "zh-CN": "非租赁",
    "zh-HK": "非租賃",
    en: "Non-lease",
  },
  "reports.col_pl_adjustment": {
    "zh-CN": "损益调整",
    "zh-HK": "損益調整",
    en: "P&L Adjustment",
  },
  "reports.working_hint": {
    "zh-CN": "工作报表：包含 Draft / Pending Approval 数据，用于内部试算",
    "zh-HK": "工作報表：包含 Draft / Pending Approval 數據，用於內部試算",
    en: "Working: includes Draft / Pending Approval data for internal testing",
  },
  "reports.official_hint": {
    "zh-CN": "正式报表：仅包含 Approved 数据，用于正式财务和审计",
    "zh-HK": "正式報表：僅包含 Approved 數據，用於正式財務和審計",
    en: "Official: only Approved data for formal financials and audit",
  },
  "reports.mode_working": {
    "zh-CN": "工作",
    "zh-HK": "工作",
    en: "Working",
  },
  "reports.mode_official": {
    "zh-CN": "正式",
    "zh-HK": "正式",
    en: "Official",
  },
  "reports.tags_imported": {
    "zh-CN": "已从标签总管带入标签筛选：",
    "zh-HK": "已從標籤總管帶入標籤篩選：",
    en: "Tags imported from Tag Manager:",
  },
  "reports.dismiss": {
    "zh-CN": "关闭",
    "zh-HK": "關閉",
    en: "Dismiss",
  },
  "reports.filter_contract_id": {
    "zh-CN": "输入合同 ID 筛选",
    "zh-HK": "輸入合同 ID 篩選",
    en: "Enter contract ID to filter",
  },
  "reports.filter_store": {
    "zh-CN": "输入门店名称筛选",
    "zh-HK": "輸入門店名稱篩選",
    en: "Enter store name to filter",
  },
  "reports.filter_tags": {
    "zh-CN": "选择或输入标签",
    "zh-HK": "選擇或輸入標籤",
    en: "Select or enter tags",
  },
  "reports.override_title": {
    "zh-CN": "折现率与汇率覆盖（可选）",
    "zh-HK": "折現率與匯率覆蓋（可選）",
    en: "Discount Rate & Currency Override (Optional)",
  },
  "reports.override_desc": {
    "zh-CN": "用于试算场景，不影响合同主数据",
    "zh-HK": "用於試算場景，不影響合同主數據",
    en: "For trial calculation only, does not affect contract master data",
  },
  "reports.override_placeholder": {
    "zh-CN": "例如 5 或 5.25",
    "zh-HK": "例如 5 或 5.25",
    en: "e.g. 5 or 5.25",
  },
  "reports.exchange_rate_placeholder": {
    "zh-CN": "例如 7.20",
    "zh-HK": "例如 7.20",
    en: "e.g. 7.20",
  },
  "reports.csv_filename": {
    "zh-CN": "摊销报表",
    "zh-HK": "攤銷報表",
    en: "Amortization Report",
  },

  // ─── Contract Detail (page.tsx supplemental) ───────────────────
  // Error / fallback messages
  "contract_detail.load_contract_failed": {
    "zh-CN": "加载合同失败",
    "zh-HK": "加載合同失敗",
    en: "Failed to load contract",
  },
  "contract_detail.load_schedules_failed": {
    "zh-CN": "加载付款计划失败",
    "zh-HK": "加載付款計劃失敗",
    en: "Failed to load payment schedules",
  },
  "contract_detail.load_events_failed": {
    "zh-CN": "加载事件失败",
    "zh-HK": "加載事件失敗",
    en: "Failed to load events",
  },
  "contract_detail.create_event_failed": {
    "zh-CN": "创建事件失败",
    "zh-HK": "創建事件失敗",
    en: "Failed to create event",
  },
  "contract_detail.submit_failed": {
    "zh-CN": "提交失败",
    "zh-HK": "提交失敗",
    en: "Submit failed",
  },
  "contract_detail.review_failed": {
    "zh-CN": "复核失败",
    "zh-HK": "覆核失敗",
    en: "Review failed",
  },
  "contract_detail.approve_failed": {
    "zh-CN": "审批失败",
    "zh-HK": "審批失敗",
    en: "Approval failed",
  },
  "contract_detail.operation_failed": {
    "zh-CN": "操作失败",
    "zh-HK": "操作失敗",
    en: "Operation failed",
  },
  "contract_detail.preview_failed": {
    "zh-CN": "预览失败",
    "zh-HK": "預覽失敗",
    en: "Preview failed",
  },
  "contract_detail.get_adjustment_failed": {
    "zh-CN": "获取调整详情失败",
    "zh-HK": "獲取調整詳情失敗",
    en: "Failed to get adjustment details",
  },
  "contract_detail.recalculate_failed": {
    "zh-CN": "重算失败",
    "zh-HK": "重算失敗",
    en: "Recalculate failed",
  },
  "contract_detail.calculate_failed": {
    "zh-CN": "计算失败",
    "zh-HK": "計算失敗",
    en: "Calculation failed",
  },
  "contract_detail.update_failed": {
    "zh-CN": "更新失败",
    "zh-HK": "更新失敗",
    en: "Update failed",
  },
  "contract_detail.create_schedule_failed": {
    "zh-CN": "创建失败",
    "zh-HK": "創建失敗",
    en: "Creation failed",
  },

  // Success messages
  "contract_detail.event_created": {
    "zh-CN": "事件创建成功",
    "zh-HK": "事件創建成功",
    en: "Event created",
  },
  "contract_detail.event_submitted": {
    "zh-CN": "事件已提交复核",
    "zh-HK": "事件已提交覆核",
    en: "Event submitted for review",
  },
  "contract_detail.review_passed": {
    "zh-CN": "复核通过",
    "zh-HK": "覆核通過",
    en: "Review passed",
  },
  "contract_detail.approval_passed": {
    "zh-CN": "审批通过",
    "zh-HK": "審批通過",
    en: "Approval passed",
  },
  "contract_detail.returned_to_editor": {
    "zh-CN": "已退回编辑",
    "zh-HK": "已退回編輯",
    en: "Returned to editor",
  },
  "contract_detail.rejected": {
    "zh-CN": "已驳回",
    "zh-HK": "已駁回",
    en: "Rejected",
  },
  "contract_detail.event_recalculated": {
    "zh-CN": "事件重算完成",
    "zh-HK": "事件重算完成",
    en: "Event recalculated",
  },
  "contract_detail.submit_review_success": {
    "zh-CN": "提交复核成功",
    "zh-HK": "提交覆核成功",
    en: "Submitted for review",
  },
  "contract_detail.ifrs16_calculated": {
    "zh-CN": "IFRS 16 计算完成",
    "zh-HK": "IFRS 16 計算完成",
    en: "IFRS 16 calculation complete",
  },
  "contract_detail.contract_updated": {
    "zh-CN": "合同更新成功",
    "zh-HK": "合同更新成功",
    en: "Contract updated",
  },
  "contract_detail.schedule_created": {
    "zh-CN": "付款计划创建成功",
    "zh-HK": "付款計劃創建成功",
    en: "Payment schedule created",
  },

  // Warning messages
  "contract_detail.please_enter_reason": {
    "zh-CN": "请输入原因",
    "zh-HK": "請輸入原因",
    en: "Please enter reason",
  },

  // Template messages (with {count} placeholder)
  "contract_detail.agent_payment_summary": {
    "zh-CN": "当前任务：为此合同解析租金表并生成付款计划草稿。",
    "zh-HK": "當前任務：為此合同解析租金表並生成付款計劃草稿。",
    en: "Current task: parse a rent schedule for this contract and generate payment schedule drafts.",
  },

  // Inline UI texts
  "contract_detail.item_unit": {
    "zh-CN": "笔",
    "zh-HK": "筆",
    en: "items",
  },
  "contract_detail.days_unit": {
    "zh-CN": "天",
    "zh-HK": "天",
    en: "days",
  },
  "contract_detail.adjustment_event_impact_preview": {
    "zh-CN": "事件影响预览",
    "zh-HK": "事件影響預覽",
    en: "Event Impact Preview",
  },
  "contract_detail.adjustment_event_detail": {
    "zh-CN": "事件调整详情",
    "zh-HK": "事件調整詳情",
    en: "Event Adjustment Detail",
  },
  "contract_detail.adjustment_type_label": {
    "zh-CN": "调整类型",
    "zh-HK": "調整類型",
    en: "Adjustment Type",
  },
  "contract_detail.liability_before": {
    "zh-CN": "调整前租赁负债",
    "zh-HK": "調整前租賃負債",
    en: "Liability Before Adjustment",
  },
  "contract_detail.liability_after": {
    "zh-CN": "调整后租赁负债",
    "zh-HK": "調整後租賃負債",
    en: "Liability After Adjustment",
  },
  "contract_detail.liability_change": {
    "zh-CN": "负债变动额",
    "zh-HK": "負債變動額",
    en: "Liability Change",
  },
  "contract_detail.asset_before": {
    "zh-CN": "调整前使用权资产",
    "zh-HK": "調整前使用權資產",
    en: "ROU Asset Before Adjustment",
  },
  "contract_detail.asset_after": {
    "zh-CN": "调整后使用权资产",
    "zh-HK": "調整後使用權資產",
    en: "ROU Asset After Adjustment",
  },
  "contract_detail.asset_change": {
    "zh-CN": "资产变动额",
    "zh-HK": "資產變動額",
    en: "Asset Change",
  },
  "contract_detail.pnl_impact": {
    "zh-CN": "损益影响 (PnL)",
    "zh-HK": "損益影響 (PnL)",
    en: "PnL Impact",
  },

  // Validation / placeholder messages
  "contract_detail.validation.payment_date": {
    "zh-CN": "请选择付款日",
    "zh-HK": "請選擇付款日",
    en: "Please select payment date",
  },
  "contract_detail.validation.amount": {
    "zh-CN": "请输入金额",
    "zh-HK": "請輸入金額",
    en: "Please enter amount",
  },
  "contract_detail.validation.lease_end_date": {
    "zh-CN": "请选择租期结束日",
    "zh-HK": "請選擇租期結束日",
    en: "Please select lease end date",
  },
  "contract_detail.validation.event_type": {
    "zh-CN": "请选择事件类型",
    "zh-HK": "請選擇事件類型",
    en: "Please select event type",
  },
  "contract_detail.validation.effective_date": {
    "zh-CN": "请选择生效日期",
    "zh-HK": "請選擇生效日期",
    en: "Please select effective date",
  },
  "contract_detail.validation.change_reason": {
    "zh-CN": "请输入变更原因",
    "zh-HK": "請輸入變更原因",
    en: "Please enter change reason",
  },
  "contract_detail.validation.contract_number": {
    "zh-CN": "请输入合同编号",
    "zh-HK": "請輸入合同編號",
    en: "Please enter contract number",
  },
  "contract_detail.validation.contract_name": {
    "zh-CN": "请输入合同名称",
    "zh-HK": "請輸入合同名稱",
    en: "Please enter contract name",
  },
  "contract_detail.validation.currency": {
    "zh-CN": "请选择币种",
    "zh-HK": "請選擇幣種",
    en: "Please select currency",
  },
  "contract_detail.validation.commencement_date": {
    "zh-CN": "请选择租赁起始日",
    "zh-HK": "請選擇租賃起始日",
    en: "Please select commencement date",
  },
  "contract_detail.validation.lease_start_date": {
    "zh-CN": "请选择租赁开始日",
    "zh-HK": "請選擇租賃開始日",
    en: "Please select lease start date",
  },
  "contract_detail.discount_rate_placeholder": {
    "zh-CN": "例如 5 或 5.25",
    "zh-HK": "例如 5 或 5.25",
    en: "e.g. 5 or 5.25",
  },
  "contract_detail.original_value_placeholder": {
    "zh-CN": "变更前的值",
    "zh-HK": "變更前的值",
    en: "Value before change",
  },
  "contract_detail.new_value_placeholder": {
    "zh-CN": "变更后的值",
    "zh-HK": "變更後的值",
    en: "Value after change",
  },
  "contract_detail.load_critical_dates_failed": {
    "zh-CN": "关键日期加载失败",
    "zh-HK": "關鍵日期載入失敗",
    en: "Failed to load critical dates",
  },
  "contract_detail.load_documents_failed": {
    "zh-CN": "文档列表加载失败",
    "zh-HK": "文件列表載入失敗",
    en: "Failed to load documents",
  },
  "contract_detail.load_obligations_failed": {
    "zh-CN": "条款义务加载失败",
    "zh-HK": "條款義務載入失敗",
    en: "Failed to load obligations",
  },
  "contract_detail.critical_date_created": {
    "zh-CN": "关键日期已创建",
    "zh-HK": "關鍵日期已建立",
    en: "Critical date created",
  },
  "contract_detail.create_critical_date_failed": {
    "zh-CN": "关键日期创建失败",
    "zh-HK": "關鍵日期建立失敗",
    en: "Failed to create critical date",
  },
  "contract_detail.document_created": {
    "zh-CN": "文档记录已创建",
    "zh-HK": "文件記錄已建立",
    en: "Document record created",
  },
  "contract_detail.create_document_failed": {
    "zh-CN": "文档记录创建失败",
    "zh-HK": "文件記錄建立失敗",
    en: "Failed to create document record",
  },
  "contract_detail.obligation_created": {
    "zh-CN": "条款义务已创建",
    "zh-HK": "條款義務已建立",
    en: "Obligation created",
  },
  "contract_detail.create_obligation_failed": {
    "zh-CN": "条款义务创建失败",
    "zh-HK": "條款義務建立失敗",
    en: "Failed to create obligation",
  },
  "contract_detail.status_updated": {
    "zh-CN": "状态已更新",
    "zh-HK": "狀態已更新",
    en: "Status updated",
  },
  "contract_detail.update_status_failed": {
    "zh-CN": "状态更新失败",
    "zh-HK": "狀態更新失敗",
    en: "Failed to update status",
  },
  // Disclosure report
  "reports.tab_disclosure": {
    "zh-CN": "披露报表",
    "zh-HK": "披露報表",
    en: "Disclosure Notes",
  },
  "reports.disclosure_period": {
    "zh-CN": "报告期间",
    "zh-HK": "報告期間",
    en: "Reporting Period",
  },
  "reports.disclosure_generate": {
    "zh-CN": "生成披露报表",
    "zh-HK": "生成披露報表",
    en: "Generate",
  },
  "reports.disclosure_export": {
    "zh-CN": "导出验证底稿 Excel",
    "zh-HK": "導出驗證底稿 Excel",
    en: "Export Workpaper (Excel)",
  },
  "reports.disclosure_as_of": {
    "zh-CN": "截至",
    "zh-HK": "截至",
    en: "As of",
  },
  "reports.disclosure_as_of_hint": {
    "zh-CN": "到期分析以期间截止日 {date} 为基准日;滚动调节与费用分解覆盖整个报告期间",
    "zh-HK": "到期分析以期間截止日 {date} 為基準日;滾動調節與費用分解覆蓋整個報告期間",
    en: "Maturity analysis is as of {date}; roll-forwards and expense breakdown cover the full reporting period",
  },
  "reports.disclosure_multi_currency_caveat": {
    "zh-CN": "组合包含多种币种({currencies}),各表金额为原币直接加总,未折算——请按币种分别核对或在总账层处理折算",
    "zh-HK": "組合包含多種幣種({currencies}),各表金額為原幣直接加總,未折算——請按幣種分別核對或在總賬層處理折算",
    en: "Portfolio spans multiple currencies ({currencies}); amounts are summed in original currency without translation — reconcile by currency or translate at GL level",
  },
  "reports.disclosure_maturity_title": {
    "zh-CN": "一、租赁负债到期分析(未折现)",
    "zh-HK": "一、租賃負債到期分析(未折現)",
    en: "1. Maturity Analysis of Lease Liabilities (Undiscounted)",
  },
  "reports.disclosure_rou_title": {
    "zh-CN": "二、使用权资产期初期末调节(按资产类别)",
    "zh-HK": "二、使用權資產期初期末調節(按資產類別)",
    en: "2. ROU Asset Reconciliation by Asset Class",
  },
  "reports.disclosure_liability_roll_title": {
    "zh-CN": "三、租赁负债滚动调节",
    "zh-HK": "三、租賃負債滾動調節",
    en: "3. Lease Liability Roll-forward",
  },
  "reports.disclosure_expense_title": {
    "zh-CN": "四、租赁相关费用分解",
    "zh-HK": "四、租賃相關費用分解",
    en: "4. Lease-related Expense Breakdown",
  },
  "reports.disclosure_cash_title": {
    "zh-CN": "五、租赁现金流出总额",
    "zh-HK": "五、租賃現金流出總額",
    en: "5. Total Cash Outflow for Leases",
  },
  "reports.disclosure_band_1y": {
    "zh-CN": "1 年内",
    "zh-HK": "1 年內",
    en: "Within 1y",
  },
  "reports.disclosure_band_1_2y": {
    "zh-CN": "1–2 年",
    "zh-HK": "1–2 年",
    en: "1–2y",
  },
  "reports.disclosure_band_2_3y": {
    "zh-CN": "2–3 年",
    "zh-HK": "2–3 年",
    en: "2–3y",
  },
  "reports.disclosure_band_3_4y": {
    "zh-CN": "3–4 年",
    "zh-HK": "3–4 年",
    en: "3–4y",
  },
  "reports.disclosure_band_4_5y": {
    "zh-CN": "4–5 年",
    "zh-HK": "4–5 年",
    en: "4–5y",
  },
  "reports.disclosure_band_5y_plus": {
    "zh-CN": "5 年以上",
    "zh-HK": "5 年以上",
    en: "Over 5y",
  },
  "reports.disclosure_discount_rate": {
    "zh-CN": "折现率",
    "zh-HK": "折現率",
    en: "Discount Rate",
  },
  "reports.disclosure_total_undiscounted": {
    "zh-CN": "未折现合计",
    "zh-HK": "未折現合計",
    en: "Total Undiscounted",
  },
  "reports.disclosure_unearned_finance": {
    "zh-CN": "未确认融资费用",
    "zh-HK": "未確認融資費用",
    en: "Unearned Finance Cost",
  },
  "reports.disclosure_carrying_liability": {
    "zh-CN": "账面租赁负债",
    "zh-HK": "賬面租賃負債",
    en: "Carrying Liability",
  },
  "reports.disclosure_asset_type": {
    "zh-CN": "资产类别",
    "zh-HK": "資產類別",
    en: "Asset Class",
  },
  "reports.disclosure_contract_count": {
    "zh-CN": "合同数",
    "zh-HK": "合同數",
    en: "Contracts",
  },
  "reports.disclosure_opening": {
    "zh-CN": "期初余额",
    "zh-HK": "期初餘額",
    en: "Opening",
  },
  "reports.disclosure_additions": {
    "zh-CN": "本期新增",
    "zh-HK": "本期新增",
    en: "Additions",
  },
  "reports.disclosure_remeasurement": {
    "zh-CN": "重计量",
    "zh-HK": "重計量",
    en: "Remeasurement",
  },
  "reports.disclosure_other_adjustments": {
    "zh-CN": "其他调整",
    "zh-HK": "其他調整",
    en: "Other Adj.",
  },
  "reports.disclosure_closing": {
    "zh-CN": "期末余额",
    "zh-HK": "期末餘額",
    en: "Closing",
  },
  "reports.disclosure_payments": {
    "zh-CN": "租金支付",
    "zh-HK": "租金支付",
    en: "Payments",
  },
  "reports.disclosure_short_term_exempt": {
    "zh-CN": "短期租赁豁免费用",
    "zh-HK": "短期租賃豁免費用",
    en: "Short-term Exempt Expense",
  },
  "reports.disclosure_low_value_exempt": {
    "zh-CN": "低价值租赁豁免费用",
    "zh-HK": "低價值租賃豁免費用",
    en: "Low-value Exempt Expense",
  },
  "reports.disclosure_expense_total": {
    "zh-CN": "费用合计(不含非租赁)",
    "zh-HK": "費用合計(不含非租賃)",
    en: "Total (excl. non-lease)",
  },
  "reports.disclosure_fixed_payments": {
    "zh-CN": "固定租金支付",
    "zh-HK": "固定租金支付",
    en: "Fixed Payments",
  },
  "reports.disclosure_prepaid_payments": {
    "zh-CN": "预付租金",
    "zh-HK": "預付租金",
    en: "Prepaid Rent",
  },
  "reports.disclosure_variable_payments": {
    "zh-CN": "变量租金",
    "zh-HK": "變量租金",
    en: "Variable Payments",
  },
  "reports.disclosure_non_lease_payments": {
    "zh-CN": "非租赁成分",
    "zh-HK": "非租賃成分",
    en: "Non-lease Components",
  },
  "reports.disclosure_total": {
    "zh-CN": "合计",
    "zh-HK": "合計",
    en: "Total",
  },
  "reports.disclosure_band": {
    "zh-CN": "时间带",
    "zh-HK": "時間帶",
    en: "Maturity Band",
  },
  "reports.disclosure_undiscounted_amount": {
    "zh-CN": "未折现金额",
    "zh-HK": "未折現金額",
    en: "Undiscounted Amount",
  },
  "reports.disclosure_reconciliation_title": {
    "zh-CN": "折现调节:未折现总额 − 未确认融资费用 = 账面租赁负债",
    "zh-HK": "折現調節:未折現總額 − 未確認融資費用 = 賬面租賃負債",
    en: "Reconciliation: Total undiscounted − Unearned finance cost = Carrying liability",
  },
  "reports.disclosure_less_unearned": {
    "zh-CN": "减:未确认融资费用",
    "zh-HK": "減:未確認融資費用",
    en: "Less: Unearned finance cost",
  },
  "reports.disclosure_sheet_detail": {
    "zh-CN": "1-合同明细",
    "zh-HK": "1-合同明細",
    en: "1-Contract Detail",
  },
  "reports.disclosure_sheet_summary": {
    "zh-CN": "2-分带汇总与调节",
    "zh-HK": "2-分帶匯總與調節",
    en: "2-Band Summary",
  },
  "reports.disclosure_sheet_roll": {
    "zh-CN": "3-负债滚动调节",
    "zh-HK": "3-負債滾動調節",
    en: "3-Liability Rollforward",
  },
  "reports.disclosure_sheet_rou": {
    "zh-CN": "4-ROU调节",
    "zh-HK": "4-ROU調節",
    en: "4-ROU Reconciliation",
  },
  "reports.disclosure_sheet_expense": {
    "zh-CN": "5-费用与现金流",
    "zh-HK": "5-費用與現金流",
    en: "5-Expense & Cash",
  },
  "reports.disclosure_report_basis": {
    "zh-CN": "报告基准",
    "zh-HK": "報告基準",
    en: "Report Basis",
  },
  "reports.disclosure_snapshot": {
    "zh-CN": "数据快照",
    "zh-HK": "數據快照",
    en: "Snapshot",
  },
  "reports.disclosure_policy_version": {
    "zh-CN": "规则版本",
    "zh-HK": "規則版本",
    en: "Policy Version",
  },
  "reports.disclosure_mode": {
    "zh-CN": "报表模式",
    "zh-HK": "報表模式",
    en: "Report Mode",
  },
  "reports.disclosure_generated_at": {
    "zh-CN": "生成时间",
    "zh-HK": "生成時間",
    en: "Generated At",
  },
  "reports.disclosure_population": {
    "zh-CN": "覆盖合同数",
    "zh-HK": "覆蓋合同數",
    en: "Population",
  },
  "reports.disclosure_computed": {
    "zh-CN": "已计算",
    "zh-HK": "已計算",
    en: "Computed",
  },
  "reports.disclosure_skipped": {
    "zh-CN": "跳过",
    "zh-HK": "跳過",
    en: "Skipped",
  },
  "reports.disclosure_excluded": {
    "zh-CN": "非租赁排除",
    "zh-HK": "非租賃排除",
    en: "Non-lease Excluded",
  },
  "reports.disclosure_approval_policy": {
    "zh-CN": "审批口径",
    "zh-HK": "審批口徑",
    en: "Approval Policy",
  },
  "reports.disclosure_sheet_basis": {
    "zh-CN": "0-报告基准",
    "zh-HK": "0-報告基準",
    en: "0-Report Basis",
  },
  "reports.disclosure_sheet_audit": {
    "zh-CN": "6-审计底稿",
    "zh-HK": "6-審計底稿",
    en: "6-Audit Workpaper",
  },
  "reports.disclosure_audit_title": {
    "zh-CN": "六、合同级审计底稿",
    "zh-HK": "六、合同級審計底稿",
    en: "6. Contract-level Audit Workpaper",
  },
  "reports.disclosure_audit_hint": {
    "zh-CN": "底稿金额来自同一份披露计算结果；差异检查用于定位输入或重算问题，不代表自动审计结论。",
    "zh-HK": "底稿金額來自同一份披露計算結果；差異檢查用於定位輸入或重算問題，不代表自動審計結論。",
    en: "Workpaper amounts use the same disclosure calculation; tie-out flags help locate input or recalculation issues and are not an automated audit conclusion.",
  },
  "reports.disclosure_rate_source": {
    "zh-CN": "折现率来源",
    "zh-HK": "折現率來源",
    en: "Rate Source",
  },
  "reports.disclosure_input_count": {
    "zh-CN": "付款计划数",
    "zh-HK": "付款計劃數",
    en: "Payment Plans",
  },
  "reports.disclosure_event_count": {
    "zh-CN": "事件调整数",
    "zh-HK": "事件調整數",
    en: "Event Adjustments",
  },
  "reports.disclosure_opening_liability": {
    "zh-CN": "期初负债",
    "zh-HK": "期初負債",
    en: "Opening Liability",
  },
  "reports.disclosure_closing_liability": {
    "zh-CN": "期末负债",
    "zh-HK": "期末負債",
    en: "Closing Liability",
  },
  "reports.disclosure_tie_out": {
    "zh-CN": "调节检查",
    "zh-HK": "調節檢查",
    en: "Tie-out",
  },
  "reports.disclosure_tied": {
    "zh-CN": "已勾稽",
    "zh-HK": "已勾稽",
    en: "Tied",
  },
  "reports.disclosure_not_tied": {
    "zh-CN": "需检查",
    "zh-HK": "需檢查",
    en: "Review",
  },
  "reports.disclosure_rows": {
    "zh-CN": "行",
    "zh-HK": "行",
    en: "rows",
  },
  "reports.disclosure_liability_tie_out": {
    "zh-CN": "负债调节差额",
    "zh-HK": "負債調節差額",
    en: "Liability Tie-out",
  },
  "reports.disclosure_rou_tie_out": {
    "zh-CN": "ROU调节差额",
    "zh-HK": "ROU調節差額",
    en: "ROU Tie-out",
  },
  "reports.asset_type_real_estate": {
    "zh-CN": "房产",
    "zh-HK": "房產",
    en: "Real Estate",
  },
  "reports.asset_type_vehicle": {
    "zh-CN": "车辆",
    "zh-HK": "車輛",
    en: "Vehicles",
  },
  "reports.asset_type_it_equipment": {
    "zh-CN": "IT 设备",
    "zh-HK": "IT 設備",
    en: "IT Equipment",
  },
  "reports.asset_type_machinery": {
    "zh-CN": "机器设备",
    "zh-HK": "機器設備",
    en: "Machinery",
  },

  // Dashboard money KPIs & trend chart
  "dashboard.kpi_total_liability": {
    "zh-CN": "总租赁负债",
    "zh-HK": "總租賃負債",
    en: "Total Lease Liability",
  },
  "dashboard.kpi_month_expense": {
    "zh-CN": "本月租赁费用",
    "zh-HK": "本月租賃費用",
    en: "This Month's Lease Expense",
  },
  "dashboard.kpi_month_expense_sub": {
    "zh-CN": "利息 + 折旧",
    "zh-HK": "利息 + 折舊",
    en: "Interest + Depreciation",
  },
  "dashboard.trend_liability": {
    "zh-CN": "租赁负债",
    "zh-HK": "租賃負債",
    en: "Lease Liability",
  },
  "dashboard.trend_rou": {
    "zh-CN": "使用权资产",
    "zh-HK": "使用權資產",
    en: "ROU Asset",
  },

  // Notification bell
  "notif.title": {
    "zh-CN": "关键日期提醒",
    "zh-HK": "關鍵日期提醒",
    en: "Critical Date Alerts",
  },
  "notif.empty": {
    "zh-CN": "暂无临近的关键日期",
    "zh-HK": "暫無臨近的關鍵日期",
    en: "No upcoming critical dates",
  },
  "notif.overdue": {
    "zh-CN": "已逾期 {days} 天",
    "zh-HK": "已逾期 {days} 天",
    en: "Overdue by {days} days",
  },
  "notif.due_in": {
    "zh-CN": "剩余 {days} 天",
    "zh-HK": "剩餘 {days} 天",
    en: "{days} days left",
  },
  "notif.due_today": {
    "zh-CN": "今天到期",
    "zh-HK": "今天到期",
    en: "Due today",
  },

  // Global search
  "search.placeholder": {
    "zh-CN": "搜索合同 / 门店 / 出租方...",
    "zh-HK": "搜尋合同 / 門店 / 出租方...",
    en: "Search contracts / stores / lessors...",
  },
  "search.no_results": {
    "zh-CN": "未找到匹配的合同",
    "zh-HK": "未找到匹配的合同",
    en: "No matching contracts",
  },
  "search.loading": {
    "zh-CN": "搜索中...",
    "zh-HK": "搜尋中...",
    en: "Searching...",
  },
  "search.action_new_contract": { "zh-CN": "新增合同", "zh-HK": "新增合同", en: "New contract" },
  "search.action_ai_entry": { "zh-CN": "用 AI 录入合同", "zh-HK": "用 AI 錄入合同", en: "Enter contract with AI" },
  "search.action_todo": { "zh-CN": "打开我的待办", "zh-HK": "打開我的待辦", en: "Open my work" },
  "search.action_reports": { "zh-CN": "打开报表查询", "zh-HK": "打開報表查詢", en: "Open reports" },
  "search.open_command_palette": { "zh-CN": "打开命令面板", "zh-HK": "打開命令面板", en: "Open command palette" },
  "search.placeholder_short": { "zh-CN": "搜索", "zh-HK": "搜尋", en: "Search" },
  "search.command_title": { "zh-CN": "搜索与快捷操作", "zh-HK": "搜尋與快捷操作", en: "Search and quick actions" },
  "search.command_placeholder": { "zh-CN": "搜索合同、页面或操作…", "zh-HK": "搜尋合同、頁面或操作…", en: "Search contracts, pages, or actions…" },
  "search.command_keyboard_hint": { "zh-CN": "↑↓ 选择 · Enter 打开 · Esc 关闭", "zh-HK": "↑↓ 選擇 · Enter 打開 · Esc 關閉", en: "↑↓ select · Enter open · Esc close" },
  "search.command_scope_hint": { "zh-CN": "合同搜索走服务端筛选", "zh-HK": "合同搜尋走服務端篩選", en: "Contract search uses server-side filtering" },
  "search.group_contracts": { "zh-CN": "合同", "zh-HK": "合同", en: "Contracts" },
  "search.group_pages": { "zh-CN": "页面", "zh-HK": "頁面", en: "Pages" },
  "search.group_actions": { "zh-CN": "操作", "zh-HK": "操作", en: "Actions" },
  "search.group_stores": { "zh-CN": "门店", "zh-HK": "門店", en: "Stores" },
  "search.group_daily": { "zh-CN": "日常作业", "zh-HK": "日常作業", en: "Daily work" },
  "search.group_accounting": { "zh-CN": "会计与合规", "zh-HK": "會計與合規", en: "Accounting & compliance" },
  "search.group_analysis": { "zh-CN": "分析与决策", "zh-HK": "分析與決策", en: "Analysis & decisions" },
  "search.group_system": { "zh-CN": "系统", "zh-HK": "系統", en: "System" },

  // Dashboard / report hand-off copy. Keep these labels stable because some
  // are also embedded in the structured context sent to the AI assistant.
  "dashboard.upcoming_critical_dates": { "zh-CN": "未来关键日期", "zh-HK": "未來關鍵日期", en: "Upcoming critical dates" },
  "dashboard.reminder_days": { "zh-CN": "提醒提前 {days} 天", "zh-HK": "提醒提前 {days} 天", en: "Reminder {days} days ahead" },
  "dashboard.overdue_days": { "zh-CN": "已逾期 {days} 天", "zh-HK": "已逾期 {days} 天", en: "Overdue by {days} days" },
  "dashboard.within_days": { "zh-CN": "{days} 天内", "zh-HK": "{days} 天內", en: "Within {days} days" },
  "dashboard.remaining_days": { "zh-CN": "剩余 {days} 天", "zh-HK": "剩餘 {days} 天", en: "{days} days remaining" },
  "dashboard.no_upcoming_dates": { "zh-CN": "未来 90 天没有待处理关键日期", "zh-HK": "未來 90 天沒有待處理關鍵日期", en: "No critical dates due in the next 90 days" },
  "critical_date.renewal_deadline": { "zh-CN": "续租截止", "zh-HK": "續租截止", en: "Renewal deadline" },
  "critical_date.break_notice": { "zh-CN": "Break 通知", "zh-HK": "Break 通知", en: "Break notice" },
  "critical_date.rent_review": { "zh-CN": "租金 Review", "zh-HK": "租金 Review", en: "Rent review" },
  "critical_date.lease_expiry": { "zh-CN": "租约到期", "zh-HK": "租約到期", en: "Lease expiry" },
  "critical_date.insurance_renewal": { "zh-CN": "保险续保", "zh-HK": "保險續保", en: "Insurance renewal" },
  "critical_date.other": { "zh-CN": "其他", "zh-HK": "其他", en: "Other" },
  "reports.ai_chat_mode": { "zh-CN": "报表口径", "zh-HK": "報表口徑", en: "Report basis" },
  "reports.ai_chat_working": { "zh-CN": "Working 工作版", "zh-HK": "Working 工作版", en: "Working" },
  "reports.ai_chat_official": { "zh-CN": "Official 正式版", "zh-HK": "Official 正式版", en: "Official" },
  "reports.ai_chat_view": { "zh-CN": "视图", "zh-HK": "視圖", en: "View" },
  "reports.ai_chat_granularity": { "zh-CN": "粒度", "zh-HK": "粒度", en: "Granularity" },
  "reports.ai_chat_period": { "zh-CN": "期间", "zh-HK": "期間", en: "Period" },
  "reports.ai_chat_interest": { "zh-CN": "利息", "zh-HK": "利息", en: "Interest" },
  "reports.ai_chat_depreciation": { "zh-CN": "折旧", "zh-HK": "折舊", en: "Depreciation" },
  "reports.ai_chat_closing_liability": { "zh-CN": "期末租赁负债", "zh-HK": "期末租賃負債", en: "Closing lease liability" },
  "reports.ai_chat_closing_rou": { "zh-CN": "期末使用权资产", "zh-HK": "期末使用權資產", en: "Closing ROU asset" },
  "reports.ai_chat_report_title": { "zh-CN": "报表", "zh-HK": "報表", en: "Report" },
  "reports.amortization_group": { "zh-CN": "分组", "zh-HK": "分組", en: "Group" },
  "ai.draft_select_at_least_one": { "zh-CN": "请至少选择一条草稿", "zh-HK": "請至少選擇一條草稿", en: "Select at least one draft" },
  "settings.journal_policy_saved": { "zh-CN": "分录政策已保存", "zh-HK": "分錄政策已保存", en: "Journal policy saved" },
  "nav.performance": { "zh-CN": "经营驾驶舱", "zh-HK": "經營駕駛艙", en: "Performance cockpit" },
  "nav.portfolio": { "zh-CN": "组合分析", "zh-HK": "組合分析", en: "Portfolio analysis" },
  "nav.sensitivity": { "zh-CN": "敏感性分析", "zh-HK": "敏感性分析", en: "Sensitivity analysis" },
  "nav.deal_compare": { "zh-CN": "条款比价", "zh-HK": "條款比價", en: "Deal comparison" },
  "nav.pre_deal": { "zh-CN": "签约前决策", "zh-HK": "簽約前決策", en: "Pre-deal decision" },
  "nav.standards": { "zh-CN": "多准则对比", "zh-HK": "多準則對比", en: "Standards comparison" },
  "nav.roi": { "zh-CN": "ROI 测算", "zh-HK": "ROI 測算", en: "ROI model" },
  "nav.users": { "zh-CN": "用户管理", "zh-HK": "用戶管理", en: "User management" },
  "nav.new": { "zh-CN": "新增", "zh-HK": "新增", en: "New" },
  "nav.group_daily": { "zh-CN": "日常作业", "zh-HK": "日常作業", en: "Daily work" },
  "nav.group_analysis": { "zh-CN": "分析与决策", "zh-HK": "分析與決策", en: "Analysis & decisions" },
  "nav.group_accounting": { "zh-CN": "会计与合规", "zh-HK": "會計與合規", en: "Accounting & compliance" },
  "nav.group_system": { "zh-CN": "系统", "zh-HK": "系統", en: "System" },
  "nav.collapse": { "zh-CN": "折叠导航", "zh-HK": "摺疊導航", en: "Collapse navigation" },
  "nav.expand": { "zh-CN": "展开导航", "zh-HK": "展開導航", en: "Expand navigation" },
  "nav.language": { "zh-CN": "切换语言", "zh-HK": "切換語言", en: "Change language" },
  "user.menu": { "zh-CN": "打开用户菜单", "zh-HK": "打開用戶選單", en: "Open user menu" },
  "notif.pending": { "zh-CN": "待处理", "zh-HK": "待處理", en: "Pending" },
  "notif.work_queue_item": { "zh-CN": "工作队列事项", "zh-HK": "工作隊列事項", en: "Work queue item" },
  "notif.view_all": { "zh-CN": "查看全部待办 →", "zh-HK": "查看全部待辦 →", en: "View all work →" },
  "dashboard.todo_title": { "zh-CN": "今日待办", "zh-HK": "今日待辦", en: "Today’s work" },
  "dashboard.data_as_of": { "zh-CN": "数据截至", "zh-HK": "數據截至", en: "Data as of" },
  "dashboard.multi_currency_note": { "zh-CN": "按币种拆分展示，未做跨币种相加", "zh-HK": "按幣種拆分展示，未做跨幣種相加", en: "Shown by currency; currencies are not added together" },
  "dashboard.kpi_closing_basis": { "zh-CN": "按各币种最近一期期末余额", "zh-HK": "按各幣種最近一期期末餘額", en: "Latest closing balance by currency" },
  "dashboard.work_queue_title": { "zh-CN": "需要你处理", "zh-HK": "需要你處理", en: "Needs your attention" },
  "dashboard.open_work_queue": { "zh-CN": "去处理", "zh-HK": "去處理", en: "Open work queue" },
  "dashboard.close_readiness": { "zh-CN": "本月结账就绪度", "zh-HK": "本月結賬就緒度", en: "Close readiness" },
  "dashboard.blocking_items": { "zh-CN": "阻断项", "zh-HK": "阻斷項", en: "Blocking items" },
  "dashboard.readiness_not_evaluated": { "zh-CN": "尚未执行就绪检查", "zh-HK": "尚未執行就緒檢查", en: "Readiness has not been evaluated" },
  "home.right_title": { "zh-CN": "行动与待办", "zh-HK": "行動與待辦", en: "Actions & to-dos" },
  "home.mobile_todo_trigger": { "zh-CN": "行动与待办", "zh-HK": "行動與待辦", en: "Actions & to-dos" },
  "home.brief_title": { "zh-CN": "今日经营简报", "zh-HK": "今日經營簡報", en: "Today's brief" },
  "home.brief_coming_desc": { "zh-CN": "今日经营简报将在这里自动生成", "zh-HK": "今日經營簡報將在這裡自動生成", en: "Your operations brief will be generated here automatically" },
  "home.brief_composer_disabled": { "zh-CN": "追问功能即将开放", "zh-HK": "追問功能即將開放", en: "Follow-up questions are coming soon" },
  "home.brief_prompt": {
    "zh-CN": "请读取当前经营脉搏并生成今日经营简报：总结销售、毛利、客流与占用成本表现，按严重度列出关注门店与信号，并说明数据分类、覆盖与可信度；不要写入业务行动。",
    "zh-HK": "請讀取當前經營脈搏並生成今日經營簡報：總結銷售、毛利、客流與佔用成本表現，按嚴重度列出關注門店與信號，並說明數據分類、覆蓋與可信度；不要寫入業務行動。",
    en: "Read the current operating pulse and produce today's brief: summarize sales, gross profit, footfall and occupancy cost performance, list attention stores and signals by severity, and state the data classification, coverage and trust; do not write a business action.",
  },
  "home.brief_loading": { "zh-CN": "正在生成今日经营简报…", "zh-HK": "正在生成今日經營簡報…", en: "Preparing today's brief…" },
  "home.brief_no_data": { "zh-CN": "当前数据分类下没有可用的经营事实", "zh-HK": "當前數據分類下沒有可用的經營事實", en: "No operating facts are available for the current data classification" },
  "home.brief_not_ready_title": { "zh-CN": "覆盖不足，不给出确定结论", "zh-HK": "覆蓋不足，不給出確定結論", en: "Coverage is insufficient; no definitive conclusion is drawn" },
  "home.brief_not_ready": { "zh-CN": "经营事实未达到 decision-ready，本次晨检不给出确定结论，也不生成行动提议。", "zh-HK": "經營事實未達到 decision-ready，本次晨檢不給出確定結論，也不生成行動提議。", en: "The operating facts are not decision-ready, so this brief draws no definitive conclusion and creates no action proposal." },
  "home.brief_needs_input_title": { "zh-CN": "需要补充数据上下文", "zh-HK": "需要補充數據上下文", en: "Data context required" },
  "home.brief_error_title": { "zh-CN": "今日经营简报生成失败", "zh-HK": "今日經營簡報生成失敗", en: "Failed to generate today's brief" },
  "home.brief_attention": { "zh-CN": "关注门店", "zh-HK": "關注門店", en: "Attention stores" },
  // HOME-004: the collapsed brief band carries the attention count beside the
  // title so one strip answers "how many stores need attention" unexpanded.
  "home.band_attention_count": { "zh-CN": "关注门店 {count}", "zh-HK": "關注門店 {count}", en: "Attention stores: {count}" },
  "home.severity_critical": { "zh-CN": "严重", "zh-HK": "嚴重", en: "Critical" },
  "home.severity_high": { "zh-CN": "高", "zh-HK": "高", en: "High" },
  "home.severity_medium": { "zh-CN": "中", "zh-HK": "中", en: "Medium" },
  "home.severity_low": { "zh-CN": "低", "zh-HK": "低", en: "Low" },
  // HOME-004 §3: the home conversation — pending bubble copy and the
  // starters reuse the /ai-chat quick-question keys (ai.chip_*).
  "home.chat_thinking": { "zh-CN": "正在思考…", "zh-HK": "正在思考…", en: "Thinking…" },
  "home.brief_plan_trace": { "zh-CN": "推理轨迹", "zh-HK": "推理軌跡", en: "Reasoning trace" },
  "home.proposals_title": { "zh-CN": "待确认建议", "zh-HK": "待確認建議", en: "Proposals to confirm" },
  "home.proposals_empty": { "zh-CN": "暂无待确认建议；Agent 提出的行动建议会出现在这里。", "zh-HK": "暫無待確認建議；Agent 提出的行動建議會出現在這裡。", en: "No proposals awaiting confirmation; agent action proposals will appear here." },
  "home.proposals_empty_short": { "zh-CN": "暂无待确认建议", "zh-HK": "暫無待確認建議", en: "No proposals awaiting confirmation" },
  "contracts.col_identity": { "zh-CN": "合同标识", "zh-HK": "合同標識", en: "Contract" },
  "contracts.col_liability": { "zh-CN": "租赁负债余额", "zh-HK": "租賃負債餘額", en: "Lease liability" },
  "contracts.col_rou": { "zh-CN": "ROU 余额", "zh-HK": "ROU 餘額", en: "ROU balance" },
  "contracts.col_current_rent": { "zh-CN": "当期租金", "zh-HK": "當期租金", en: "Current rent" },
  "contracts.col_lease_scope": { "zh-CN": "租赁范围", "zh-HK": "租賃範圍", en: "Lease scope" },
  "contracts.col_asset": { "zh-CN": "资产类型", "zh-HK": "資產類型", en: "Asset type" },
  "contracts.discount_rate_missing": { "zh-CN": "缺折现率", "zh-HK": "缺折現率", en: "Discount rate missing" },
  "monthly.process_title": { "zh-CN": "结账流程", "zh-HK": "結賬流程", en: "Close process" },
  "monthly.process_select_period": { "zh-CN": "请选择会计期间", "zh-HK": "請選擇會計期間", en: "Select an accounting period" },
  "monthly.process_readiness": { "zh-CN": "就绪检查", "zh-HK": "就緒檢查", en: "Readiness" },
  "monthly.process_generate": { "zh-CN": "生成分录", "zh-HK": "生成分錄", en: "Generate entries" },
  "monthly.process_review": { "zh-CN": "复核", "zh-HK": "覆核", en: "Review" },
  "monthly.process_approve": { "zh-CN": "审批", "zh-HK": "審批", en: "Approve" },
  "monthly.process_post": { "zh-CN": "过账", "zh-HK": "過賬", en: "Post" },
  "monthly.process_lock": { "zh-CN": "锁定期间", "zh-HK": "鎖定期間", en: "Lock period" },
  "monthly.process_complete": { "zh-CN": "本期已锁定", "zh-HK": "本期已鎖定", en: "Period locked" },

  // Landing Page translations
  "landing.brand_name": { "zh-CN": "线下零售经营决策工作站", "zh-HK": "線下零售經營決策工作站", en: "Retail Performance Workstation" },
  "login.view_landing": { "zh-CN": "了解产品能力 · 查看落地页 Demo →", "zh-HK": "了解產品能力 · 查看落地頁 Demo →", en: "Explore Capabilities · View Landing Page Demo →" },
  "landing.brand_badge": { "zh-CN": "面向连锁零售承租方 · 经营分析与租赁合规一体化工作站", "zh-HK": "面向連鎖零售承租方 · 經營分析與租賃合規一體化工作站", en: "For Chain Retail Tenants · Operating Decision & Lease Compliance Platform" },
  "landing.nav_features": { "zh-CN": "核心能力", "zh-HK": "核心能力", en: "Capabilities" },
  "landing.nav_demo": { "zh-CN": "交互展台", "zh-HK": "交互展台", en: "Live Demo" },
  "landing.nav_comparison": { "zh-CN": "方案对比", "zh-HK": "方案對比", en: "Comparison" },
  "landing.nav_calculator": { "zh-CN": "效益测算", "zh-HK": "效益測算", en: "ROI Calculator" },
  "landing.nav_personas": { "zh-CN": "赋能角色", "zh-HK": "賦能角色", en: "Personas" },
  "landing.nav_security": { "zh-CN": "安全合规", "zh-HK": "安全合規", en: "Security & Compliance" },
  "landing.nav_pricing": { "zh-CN": "价格方案", "zh-HK": "價格方案", en: "Pricing" },
  "landing.nav_faq": { "zh-CN": "常见问题", "zh-HK": "常見問題", en: "FAQ" },
  "landing.nav_login": { "zh-CN": "进入工作台", "zh-HK": "進入工作台", en: "Sign In" },
  "landing.nav_book_demo": { "zh-CN": "预约专家演示", "zh-HK": "預約專家演示", en: "Book a Demo" },

  // Hero Section
  "landing.hero_tag": { "zh-CN": "连锁零售承租方经营决策新范式", "zh-HK": "連鎖零售承租方經營決策新範式", en: "New Paradigm for Chain Retail Tenants" },
  "landing.hero_free_badge": { "zh-CN": "现已开放个人免费体验 · 每日赠送 5 次 AI 智能经营分析", "zh-HK": "現已開放個人免費體驗 · 每日贈送 5 次 AI 智能經營分析", en: "Open for Individual Free Access · 5 Free AI Requests Daily" },
  "landing.hero_title_prefix": { "zh-CN": "打破销售、人工与租约孤岛", "zh-HK": "打破銷售、人工與租約孤島", en: "Unify Sales, Labor & Lease Contracts" },
  "landing.hero_title_suffix": { "zh-CN": "让每家门店的经营异常转化为确定性收益", "zh-HK": "讓每家門店的經營異常轉化為確定性收益", en: "Turn Every Store Anomaly into Verifiable Profit" },
  "landing.hero_rating": { "zh-CN": "4.9/5 · 500+ 连锁门店对账验证", "zh-HK": "4.9/5 · 500+ 連鎖門店對賬驗證", en: "4.9/5 · Validated across 500+ Chain Stores" },
  "landing.mosaic_live_stores": { "zh-CN": "128 门店实时对账", "zh-HK": "128 門店即時對賬", en: "128 Stores Live" },
  "landing.mosaic_reconciled": { "zh-CN": "99.4% 准确率", "zh-HK": "99.4% 準確率", en: "99.4% Reconciled" },
  "landing.mosaic_arch_badge": { "zh-CN": "500+ 场景推演", "zh-HK": "500+ 場景推演", en: "500+ Scenarios" },
  "landing.mosaic_triage_team": { "zh-CN": "T+1 晨检已就绪", "zh-HK": "T+1 晨檢已就緒", en: "T+1 Triage Active" },
  "landing.mosaic_growth_val": { "zh-CN": "+¥382,000", "zh-HK": "+¥382,000", en: "+¥382,000" },
  "landing.mosaic_growth_label": { "zh-CN": "NPV 增厚收益 ↗", "zh-HK": "NPV 增厚收益 ↗", en: "NPV Value Lift ↗" },

  // Asymmetric Bento Section
  "landing.asym_badge": { "zh-CN": "双核引擎全景", "zh-HK": "雙核引擎全景", en: "Dual Engine Bento" },
  "landing.asym_title": { "zh-CN": "全景连通经营与租约，掌控每一分门店利润", "zh-HK": "全景連通經營與租約，掌控每一分門店利潤", en: "Unified Command: Connect Operations & Leases" },
  "landing.asym_subtitle": { "zh-CN": "打破营运、财务与拓展团队的割裂，在统一工作台上实现四墙利润穿透与租约情景推演", "zh-HK": "打破營運、財務與拓展團隊的割裂，在統一工作台上實現四牆利潤穿透與租約情景推演", en: "Break organizational silos to achieve real-time four-wall profit decomposition and lease NPV simulations." },
  "landing.asym_card1_title": { "zh-CN": "四墙利润 (4-Wall EBITDA) 极速穿透", "zh-HK": "四牆利潤 (4-Wall EBITDA) 極速穿透", en: "Four-Wall EBITDA Decomposition" },
  "landing.asym_card1_desc": { "zh-CN": "自动合并 POS 流水、排班工时与租赁台账，实时生成瀑布流拆解与同群 P25/P50/P75 分位数标杆。", "zh-HK": "自動合併 POS 流水、排班工時與租賃台賬，即時生成瀑布流拆解與同群 P25/P50/P75 分位數標桿。", en: "Consolidates POS, labor hours, and leases to generate instant waterfall breakdowns and P25/P50/P75 peer benchmarks." },
  "landing.asym_card1_growth": { "zh-CN": "+3.2% 利润率优化", "zh-HK": "+3.2% 利潤率優化", en: "+3.2% Margin Lift" },
  "landing.asym_card2_title": { "zh-CN": "租约谈判与 NPV 敏感性推演", "zh-HK": "租約談判與 NPV 敏感性推演", en: "Lease Scenario & NPV Simulation" },
  "landing.asym_card2_desc": { "zh-CN": "在常规经营基准线之上，自由设定销售预期、租金折让幅度与免租期，即时测算投资净现值与回收期。", "zh-HK": "在常規經營基準線之上，自由設定銷售預期、租金折讓幅度與免租期，即時測算投資淨現值與回收期。", en: "Adjust sales targets, rent concessions, and rent-free periods dynamically to model investment NPV and payback timelines." },
  "landing.asym_card2_npv": { "zh-CN": "+¥382,000 NPV", "zh-HK": "+¥382,000 NPV", en: "+¥382,000 NPV" },

  // Dark Service Matrix
  "landing.matrix_dark_badge": { "zh-CN": "企业级赋能", "zh-HK": "企業級賦能", en: "Enterprise Capabilities" },
  "landing.matrix_dark_title": { "zh-CN": "6 大高确定性经营分析与合规能力", "zh-HK": "6 大高確定性經營分析與合規能力", en: "6 Core Deterministic Operating & Compliance Engines" },
  "landing.matrix_dark_subtitle": { "zh-CN": "覆盖连锁承租方从日常晨检到准则过账的每一个关键节点", "zh-HK": "覆蓋連鎖承租方從日常晨檢到準則過賬的每一個關鍵節點", en: "Covering every critical milestone from daily store triage to ERP audit posting." },
  "landing.matrix_card1_title": { "zh-CN": "01. Store-Day 事实管道", "zh-HK": "01. Store-Day 事實管道", en: "01. Store-Day Fact Pipeline" },
  "landing.matrix_card1_desc": { "zh-CN": "原子级日经营事实接入，多币种物理分区，杜绝数据打架与虚假零值。", "zh-HK": "原子級日經營事實接入，多幣種物理分區，杜絕數據打架與虛假零值。", en: "Atomic daily store facts ingestion with multi-currency strict partitioning." },
  "landing.matrix_card2_title": { "zh-CN": "02. 晨检异动雷达与排行榜", "zh-HK": "02. 晨檢異動雷達與排行榜", en: "02. Daily Anomaly Radar & Triage" },
  "landing.matrix_card2_desc": { "zh-CN": "T+1 自动计算异常关注度得分，智能锁定租售比超标与排班错配门店。", "zh-HK": "T+1 自動計算異常關注度得分，智能鎖定租售比超標與排班錯配門店。", en: "T+1 anomaly severity ranking to catch rent spikes and labor overstaffing." },
  "landing.matrix_card3_title": { "zh-CN": "03. 同商圈同业态分位数对标", "zh-HK": "03. 同商圈同業態分位數對標", en: "03. Peer Cohort Benchmark" },
  "landing.matrix_card3_desc": { "zh-CN": "将门店置于真实商圈与品牌群组中，直观拆解毛利与租金对利润的贡献。", "zh-HK": "將門店置於真實商圈與品牌群組中，直觀拆解毛利與租金對利潤的貢獻。", en: "Benchmarking stores within regional cohorts across P25, P50, and P75 quartiles." },
  "landing.matrix_card4_title": { "zh-CN": "04. 租约情景敏感性推演", "zh-HK": "04. 租約情景敏感性推演", en: "04. Scenario Sensitivity Modeling" },
  "landing.matrix_card4_desc": { "zh-CN": "动态推演降租、改抽成扣率、提前解约等方案的 NPV 收益与回收期。", "zh-HK": "動態推演降租、改抽成扣率、提前解約等方案的 NPV 收益與回收期。", en: "Dynamic sensitivity modeling for rent discounts, variable fees, and early exits." },
  "landing.matrix_card5_title": { "zh-CN": "05. IFRS 16 准则计量与过账", "zh-HK": "05. IFRS 16 準則計量與過賬", en: "05. IFRS 16 Automated Posting" },
  "landing.matrix_card5_desc": { "zh-CN": "使用权资产与负债全自动滚动，一键过账至 SAP / Oracle / 用友 / 金蝶。", "zh-HK": "使用權資產與負債全自動滾動，一鍵過賬至 SAP / Oracle / 用友 / 金蝶。", en: "Automated ROU and liability rollforward with direct ERP journal export." },
  "landing.matrix_card6_title": { "zh-CN": "06. AI Copilot 协同与原文定位", "zh-HK": "06. AI Copilot 協同與原文定位", en: "06. Grounded AI Copilot" },
  "landing.matrix_card6_desc": { "zh-CN": "严守 AI 辅助模式，提取条款带置信度与原文坐标高亮，人工终审入库。", "zh-HK": "嚴守 AI 輔助模式，提取條款帶置信度與原文座標高亮，人工終審入庫。", en: "Assist-mode AI extracting clauses with coordinate bounding boxes and audit log." },

  // Concentric Radar Section
  "landing.radar_badge": { "zh-CN": "全渠道生态连接", "zh-HK": "全渠道生態連接", en: "Ecosystem Integration" },
  "landing.radar_title": { "zh-CN": "无缝集成主流 POS、ERP 与协同系统", "zh-HK": "無縫集成主流 POS、ERP 與協同系統", en: "Seamless Integration with Core Retail Systems" },
  "landing.radar_subtitle": { "zh-CN": "双向数据管道打通企业既有数字化资产，无需推倒重来，即插即用", "zh-HK": "雙向數據管道打通企業既有數字化資產，無需推倒重來，即插即用", en: "Bi-directional data pipelines unifying POS, ERP, HR, and lease contracts effortlessly." },
  "landing.radar_center_node": { "zh-CN": "零售经营工作台", "zh-HK": "零售經營工作台", en: "Retail Workstation" },
  "landing.radar_pos_node": { "zh-CN": "POS 销售流水", "zh-HK": "POS 銷售流水", en: "POS Sales Data" },
  "landing.radar_erp_node": { "zh-CN": "ERP 财务总账", "zh-HK": "ERP 財務總賬", en: "ERP General Ledger" },
  "landing.radar_hr_node": { "zh-CN": "HR 考勤排班", "zh-HK": "HR 考勤排班", en: "HR Shift Schedule" },
  "landing.radar_lease_node": { "zh-CN": "租赁合同台账", "zh-HK": "租賃合同台賬", en: "Lease Contract Ledgers" },
  "landing.radar_brand_yonyou": { "zh-CN": "用友 NC/BIP", "zh-HK": "用友 NC/BIP", en: "Yonyou NC/BIP" },
  "landing.radar_brand_kingdee": { "zh-CN": "金蝶 Cloud", "zh-HK": "金蝶 Cloud", en: "Kingdee Cloud" },
  "landing.radar_brand_meituan": { "zh-CN": "美团 / 客如云", "zh-HK": "美團 / 客如雲", en: "Meituan POS" },
  "landing.radar_brand_feishu": { "zh-CN": "飞书 / 钉钉", "zh-HK": "飛書 / 釘釘", en: "Feishu / DingTalk" },

  "landing.hero_subtitle": {
    "zh-CN": "专为拥有 10+ 门店的连锁零售承租方打造。从单店按日（Store-Day）四墙利润穿透、同类商圈科学对标，到租约降租/续签情景推演与 IFRS 16 自动化合规，驱动“发现问题—解释原因—模拟方案—闭环行动”全流程。",
    "zh-HK": "專為擁有 10+ 門店的連鎖零售承租方打造。從單店按日（Store-Day）四牆利潤穿透、同類商圈科學對標，到租約降租/續簽情景推演與 IFRS 16 自動化合規，驅動「發現問題—解釋原因—模擬方案—閉環行動」全流程。",
    en: "Engineered for chain retail tenants with 10+ stores. From daily store-level (Store-Day) four-wall profit decomposition and peer cohort benchmarking to lease renegotiation simulation and audit-grade IFRS 16 compliance, driving the full cycle of 'Discover - Explain - Simulate - Action'.",
  },
  "landing.hero_cta_free": { "zh-CN": "免费体验工作台 (每日5次AI)", "zh-HK": "免費體驗工作台 (每日5次AI)", en: "Start Free (5 Daily AI Requests)" },
  "landing.hero_cta_primary": { "zh-CN": "预约专家演示 & 获取ROI评估", "zh-HK": "預約專家演示 & 獲取ROI評估", en: "Book Expert Demo & Get ROI Audit" },
  "landing.hero_cta_secondary": { "zh-CN": "交互式体验 Demo ↓", "zh-HK": "交互式體驗 Demo ↓", en: "Explore Interactive Demo ↓" },

  "landing.hero_chip_1": { "zh-CN": "Store-Day 日级对账", "zh-HK": "Store-Day 日級對賬", en: "Store-Day Reconciliation" },
  "landing.hero_chip_2": { "zh-CN": "四墙利润 EBITDA 穿透", "zh-HK": "四牆利潤 EBITDA 穿透", en: "4-Wall EBITDA Decomposition" },
  "landing.hero_chip_3": { "zh-CN": "每日 5 次免费 AI 诊断", "zh-HK": "每日 5 次免費 AI 診斷", en: "5 Free AI Requests Daily" },
  "landing.hero_chip_4": { "zh-CN": "IFRS 16 准则审计合规", "zh-HK": "IFRS 16 準則審計合規", en: "IFRS 16 Audit Compliance" },

  "landing.social_proof_title": { "zh-CN": "已被广泛应用于连锁零售各核心业态", "zh-HK": "已被廣泛應用於連鎖零售各核心業態", en: "Empowering Chain Retailers Across Core Verticals" },
  "landing.sector_1": { "zh-CN": "新茶饮与精品咖啡", "zh-HK": "新茶飲與精品咖啡", en: "Tea & Specialty Coffee" },
  "landing.sector_2": { "zh-CN": "快时尚与鞋服配饰", "zh-HK": "快時尚與鞋服配飾", en: "Fashion & Apparel" },
  "landing.sector_3": { "zh-CN": "连锁餐饮与快餐", "zh-HK": "連鎖餐飲與快餐", en: "Dining & Fast Food Chains" },
  "landing.sector_4": { "zh-CN": "美妆与潮玩集合店", "zh-HK": "美妝與潮玩集合店", en: "Beauty & Lifestyle Retail" },
  "landing.sector_5": { "zh-CN": "便利店与精品商超", "zh-HK": "便利店與精品商超", en: "Convenience & Supermarkets" },

  "landing.hero_metric_stores": { "zh-CN": "500+", "zh-HK": "500+", en: "500+" },
  "landing.hero_metric_stores_label": { "zh-CN": "连锁门店真实模拟验证场景", "zh-HK": "連鎖門店真實模擬驗證場景", en: "Store Scenarios Validated" },
  "landing.hero_metric_compliance": { "zh-CN": "100%", "zh-HK": "100%", en: "100%" },
  "landing.hero_metric_compliance_label": { "zh-CN": "IFRS 16 国际准则计量与对账", "zh-HK": "IFRS 16 國際準則計量與對賬", en: "IFRS 16 Audit-Ready Compliance" },
  "landing.hero_metric_margin": { "zh-CN": "+3.2%", "zh-HK": "+3.2%", en: "+3.2%" },
  "landing.hero_metric_margin_label": { "zh-CN": "平均门店四墙利润率优化空间", "zh-HK": "平均門店四牆利潤率優化空間", en: "Avg Four-Wall Margin Lift Potential" },
  "landing.hero_metric_closing": { "zh-CN": "<2h", "zh-HK": "<2h", en: "<2h" },
  "landing.hero_metric_closing_label": { "zh-CN": "月末租赁结账耗时（由 5 天缩短至 2 小时）", "zh-HK": "月末租賃結賬耗時（由 5 天縮短至 2 小時）", en: "Month-End Lease Close Time (Down from 5 days)" },

  // Workflow Section
  "landing.workflow_badge": { "zh-CN": "4 步闭环工作流", "zh-HK": "4 步閉環工作流", en: "4-Step Operating Flywheel" },
  "landing.workflow_title": { "zh-CN": "从数据感知到利润增厚的端到端闭环", "zh-HK": "從數據感知到利潤增厚的端到端閉環", en: "End-to-End Loop from Anomaly to Realized Margin Uplift" },
  "landing.workflow_subtitle": { "zh-CN": "打破营运、财务与拓展团队的数据孤岛，在统一工作台上协同实现经营决策闭环", "zh-HK": "打破營運、財務與拓展團隊的數據孤島，在統一工作台上協同實現經營決策閉環", en: "Unify operations, finance, and real estate teams onto a single collaborative cockpit." },
  "landing.step1_num": { "zh-CN": "01", "zh-HK": "01", en: "01" },
  "landing.step1_title": { "zh-CN": "全渠道数据自动接入", "zh-HK": "全渠道數據自動接入", en: "Automated Data Ingestion" },
  "landing.step1_desc": { "zh-CN": "自动同步 POS 销售日流水、商品毛利、HR 排班工时与租赁合同，统一指标语义标准，拒绝数据打架。", "zh-HK": "自動同步 POS 銷售日流水、商品毛利、HR 排班工時與租賃合同，統一指標語義標準，拒絕數據打架。", en: "Auto-sync POS store sales, gross margins, HR shifts and lease contracts under a single semantic standard." },
  "landing.step2_num": { "zh-CN": "02", "zh-HK": "02", en: "02" },
  "landing.step2_title": { "zh-CN": "晨检异常智能归因", "zh-HK": "晨檢異常智能歸因", en: "Morning Anomaly Triage" },
  "landing.step2_desc": { "zh-CN": "单店日级异常感知，智能识别租售比超标、排班错配与掉队门店，输出直观的四墙利润瀑布流。", "zh-HK": "單店日級異常感知，智能識別租售比超標、排班錯配與掉隊門店，輸出直觀的四牆利潤瀑布流。", en: "Store-day health alerts instantly flagging rent-to-sales spikes and labor mismatch with EBITDA waterfalls." },
  "landing.step3_num": { "zh-CN": "03", "zh-HK": "03", en: "03" },
  "landing.step3_title": { "zh-CN": "租约推演与谈判决策", "zh-HK": "租約推演與談判決策", en: "Scenario & Lease Negotiation" },
  "landing.step3_desc": { "zh-CN": "多方案实时测算投资净现值（NPV）、租金折让与免租期组合，提供数据驱动的降租与续约策略。", "zh-HK": "多方案即時測算投資淨現值（NPV）、租金折讓與免租期組合，提供數據驅動的降租與續約策略。", en: "Simulate rent concessions, NPV, and payback periods dynamically to empower landlord negotiations." },
  "landing.step4_num": { "zh-CN": "04", "zh-HK": "04", en: "04" },
  "landing.step4_title": { "zh-CN": "财务合规与行动闭环", "zh-HK": "財務合規與行動閉環", en: "Compliance & Profit Review" },
  "landing.step4_desc": { "zh-CN": "IFRS 16 资产负债自动结账并过账至 ERP，闭环追踪降租行动落地后的实际增厚收益与 ROI 复盘。", "zh-HK": "IFRS 16 資產負債自動結賬並過賬至 ERP，閉環追蹤降租行動落地後的實際增厚收益與 ROI 復盤。", en: "Automate IFRS 16 month-end entries to ERP and track realized cost savings across store portfolios." },

  // Live Demo Section
  "landing.demo_badge": { "zh-CN": "免登录实时交互体验", "zh-HK": "免登入即時交互體驗", en: "Interactive Live Demo" },
  "landing.demo_title": { "zh-CN": "5 大核心业务引擎，还原真实经营决策现场", "zh-HK": "5 大核心業務引擎，還原真實經營決策現場", en: "5 Core Engines Powering Real Operating Decisions" },
  "landing.demo_subtitle": { "zh-CN": "免登录交互体验：从每日晨检、单店诊断到租约谈判与财务结账，探索完整的经营决策闭环", "zh-HK": "免登入交互體驗：從每日晨檢、單店診斷到租約談判與財務結賬，探索完整的經營決策閉環", en: "Interactive live showcase: explore the full loop from daily morning check and store diagnostics to lease negotiation and month-end closing." },

  "landing.demo_tab_pulse": { "zh-CN": "经营脉搏 Operating Pulse", "zh-HK": "經營脈搏 Operating Pulse", en: "Operating Pulse" },
  "landing.demo_tab_store": { "zh-CN": "门店 360 诊断 Store 360", "zh-HK": "門店 360 診斷 Store 360", en: "Store 360 Diagnostics" },
  "landing.demo_tab_scenario": { "zh-CN": "情景推演工作台 Scenario", "zh-HK": "情景推演工作台 Scenario", en: "Scenario Workbench" },
  "landing.demo_tab_ifrs": { "zh-CN": "IFRS 16 租赁合规 Compliance", "zh-HK": "IFRS 16 租賃合規 Compliance", en: "IFRS 16 Compliance" },
  "landing.demo_tab_ai": { "zh-CN": "AI 经营 Copilot Assist", "zh-HK": "AI 經營 Copilot Assist", en: "AI Operating Copilot" },

  "landing.demo_pulse_headline": { "zh-CN": "每日晨检：单店按日异常感知与重点关注排序", "zh-HK": "每日晨檢：單店按日異常感知與重點關注排序", en: "Daily Morning Check: Store-Day Attention Ranking & Health Triage" },
  "landing.demo_pulse_desc": { "zh-CN": "基于单店按日（Store-Day）真实经营数据，智能计算异常关注度得分。清晰标注数据完整度，绝不用虚假零值掩盖缺失，助您第一时间锁定掉队门店。", "zh-HK": "基於單店按日（Store-Day）真實經營數據，智能計算異常關注度得分。清晰標註數據完整度，絕不用虛假零值掩蓋缺失，助您第一時間鎖定掉隊門店。", en: "Built on atomic store-day facts, deterministically scoring anomaly severity. Clearly flags data coverage and never pads missing data with zeroes, surfacing at-risk stores instantly." },
  "landing.demo_pulse_store1": { "zh-CN": "上海淮海中路旗舰店", "zh-HK": "上海淮海中路旗艦店", en: "Shanghai Huaihai Flagship" },
  "landing.demo_pulse_store2": { "zh-CN": "北京三里屯太古里店", "zh-HK": "北京三里屯太古里店", en: "Beijing Sanlitun Store" },
  "landing.demo_pulse_store3": { "zh-CN": "广州天河城购物中心店", "zh-HK": "廣州天河城購物中心店", en: "Guangzhou Teemall Store" },
  "landing.demo_pulse_store4": { "zh-CN": "深圳万象天地体验店", "zh-HK": "深圳萬象天地體驗店", en: "Shenzhen MixC World Store" },
  "landing.demo_pulse_attention_high": { "zh-CN": "高关注度 (得分 88)", "zh-HK": "高關注度 (得分 88)", en: "High Attention (Score 88)" },
  "landing.demo_pulse_attention_med": { "zh-CN": "中关注度 (得分 62)", "zh-HK": "中關注度 (得分 62)", en: "Medium Attention (Score 62)" },
  "landing.demo_pulse_attention_low": { "zh-CN": "正常经营 (得分 24)", "zh-HK": "正常經營 (得分 24)", en: "Healthy (Score 24)" },
  "landing.demo_pulse_ready_tag": { "zh-CN": "数据可信度：98.4% 完整就绪（Decision-Ready）", "zh-HK": "數據可信度：98.4% 完整就緒（Decision-Ready）", en: "Data Trust: 98.4% Decision-Ready Coverage" },
  "landing.demo_pulse_meta": { "zh-CN": "数据源：POS与排班系统自动同步 · 批次对账已通过", "zh-HK": "數據源：POS與排班系統自動同步 · 批次對賬已通過", en: "Source: POS & Scheduling Auto-Sync · Batch Reconciled" },
  "landing.demo_pulse_signal1": { "zh-CN": "租售比偏高 28.4% (合理基准线 18%)", "zh-HK": "租售比偏高 28.4% (合理基準線 18%)", en: "Rent-to-Sales 28.4% (Baseline 18%)" },
  "landing.demo_pulse_signal2": { "zh-CN": "客流转化率同比下滑 -4.2%", "zh-HK": "客流轉化率同比下滑 -4.2%", en: "Footfall Conversion -4.2% YoY" },
  "landing.demo_pulse_signal3": { "zh-CN": "晚间高峰排班错配 15 工时/周", "zh-HK": "晚間高峰排班錯配 15 工時/周", en: "Labor Peak Mismatch 15 hrs/wk" },

  "landing.demo_store_headline": { "zh-CN": "门店 360 诊断：四墙利润瀑布流与同类门店科学对标", "zh-HK": "門店 360 診斷：四牆利潤瀑布流與同類門店科學對標", en: "Store 360: Four-Wall Profit Waterfall & Scientific Peer Cohort" },
  "landing.demo_store_desc": { "zh-CN": "打通销售额、商品毛利、排班人工与租赁合同（固定租金、保底抽成、物业费），直观呈现单店四墙经营利润（4-Wall EBITDA）及在同商圈门店中的分位水平。", "zh-HK": "打通銷售額、商品毛利、排班人工與租賃合同（固定租金、保底抽成、物業費），直觀呈現單店四牆經營利潤（4-Wall EBITDA）及在同商圈門店中的分位水平。", en: "Bridges revenue, gross margin, labor, and lease contracts (fixed rent, turnover rent, CAM) to show true four-wall EBITDA and quartile rankings." },
  "landing.demo_store_rev": { "zh-CN": "销售总额 ¥420,000", "zh-HK": "銷售總額 ¥420,000", en: "Total Revenue ¥420,000" },
  "landing.demo_store_gp": { "zh-CN": "商品毛利 ¥231,000 (毛利率 55.0%)", "zh-HK": "商品毛利 ¥231,000 (毛利率 55.0%)", en: "Gross Profit ¥231,000 (55.0%)" },
  "landing.demo_store_labor": { "zh-CN": "门店人工成本 -¥75,600 (人工率 18.0%)", "zh-HK": "門店人工成本 -¥75,600 (人工率 18.0%)", en: "Store Labor -¥75,600 (18.0%)" },
  "landing.demo_store_rent": { "zh-CN": "租金与占用成本 -¥92,400 (租售比 22.0% · 偏高)", "zh-HK": "租金與佔用成本 -¥92,400 (租售比 22.0% · 偏高)", en: "Occupancy Cost -¥92,400 (22.0% · High)" },
  "landing.demo_store_other": { "zh-CN": "其他营运支出 -¥21,000 (费率 5.0%)", "zh-HK": "其他營運支出 -¥21,000 (費率 5.0%)", en: "Other Controllable -¥21,000 (5.0%)" },
  "landing.demo_store_ebit": { "zh-CN": "门店四墙利润 (4-Wall EBITDA) ¥42,000 (利润率 10.0%)", "zh-HK": "門店四牆利潤 (4-Wall EBITDA) ¥42,000 (利潤率 10.0%)", en: "Four-Wall EBITDA ¥42,000 (10.0%)" },
  "landing.demo_store_cohort_tag": { "zh-CN": "同群对标：处于同品牌华东商圈门店 P35 分位（低于中位数 P50 水平）", "zh-HK": "同群對標：處於同品牌華東商圈門店 P35 分位（低於中位數 P50 水平）", en: "Peer Benchmark: P35 rank within East China shopping mall cohort (Below median P50)" },
  "landing.demo_store_meta": { "zh-CN": "对标样本：华东区同业 28 家门店 · 币种：CNY", "zh-HK": "對標樣本：華東區同業 28 間門店 · 幣種：CNY", en: "Cohort Sample: 28 Regional Stores · Currency: CNY" },

  "landing.demo_scenario_headline": { "zh-CN": "情景推演工作台：租约重谈、降租与闭店方案实时试算", "zh-HK": "情景推演工作台：租約重談、降租與閉店方案即時試算", en: "Scenario Workbench: Dynamic Lease Renegotiation, Discount & Closure Simulation" },
  "landing.demo_scenario_desc": { "zh-CN": "在常规经营基准线之上，自由设定销售预期、租金折让幅度与免租期，即时测算不同方案的投资净现值（NPV）、投资回收期与四墙利润改善幅度。", "zh-HK": "在常規經營基準線之上，自由設定銷售預期、租金折讓幅度與免租期，即時測算不同方案的投資淨現值（NPV）、投資回收期與四牆利潤改善幅度。", en: "Adjust revenue bounce-back, rent concessions, and rent-free periods over baseline run-rates to simulate NPV, payback period, and margin uplift instantly." },
  "landing.demo_scenario_opt1": { "zh-CN": "方案 A：争取 15% 租金折让 (月租降至 ¥78,540)", "zh-HK": "方案 A：爭取 15% 租金折讓 (月租降至 ¥78,540)", en: "Option A: 15% Rent Concession (Down to ¥78,540/mo)" },
  "landing.demo_scenario_opt2": { "zh-CN": "方案 B：转为「保底租金 + 12% 扣率」抽成组合", "zh-HK": "方案 B：轉為「保底租金 + 12% 扣率」抽成組合", en: "Option B: Base Rent + 12% Turnover Rent Hybrid" },
  "landing.demo_scenario_opt3": { "zh-CN": "方案 C：行使提前解约权闭店 (违约赔偿与资产处置)", "zh-HK": "方案 C：行使提前解約權閉店 (違約賠償與資產處置)", en: "Option C: Exercise Early Termination (Penalty & Write-off)" },
  "landing.demo_scenario_result": { "zh-CN": "方案 A 测算结果：年化四墙利润提升 +¥166,320，租售比降至 18.7%，投资净现值（NPV）增加 ¥382,000", "zh-HK": "方案 A 測算結果：年化四牆利潤提升 +¥166,320，租售比降至 18.7%，投資淨現值（NPV）增加 ¥382,000", en: "Option A Outcome: Annual 4-Wall Profit +¥166,320, Rent-to-Sales down to 18.7%, NPV +¥382,000" },

  "landing.demo_ifrs_headline": { "zh-CN": "IFRS 16 租赁会计合规：全自动摊销、分录与 ERP 凭证过账", "zh-HK": "IFRS 16 租賃會計合規：全自動攤銷、分錄與 ERP 憑證過賬", en: "IFRS 16 Lease Compliance: Automated Amortization, Journal Entries & ERP Export" },
  "landing.demo_ifrs_desc": { "zh-CN": "严格遵循国际财务报告准则第 16 号规范。自动维护使用权资产（ROU）、租赁负债滚动表、折旧与利息分录，支持一键审批并过账至 ERP。", "zh-HK": "嚴格遵循國際財務報告準則第 16 號規範。自動維護使用權資產（ROU）、租賃負債滾動表、折舊與利息分錄，支持一鍵審批並過賬至 ERP。", en: "Strict compliance with IFRS 16 / ASC 842. Automated ROU assets & lease liabilities rollforward, interest & depreciation schedules, and close approval gates." },
  "landing.demo_ifrs_liability": { "zh-CN": "租赁负债期末余额: ¥3,189,450.00", "zh-HK": "租賃負債期末餘額: ¥3,189,450.00", en: "Lease Liability Ending: ¥3,189,450.00" },
  "landing.demo_ifrs_rou": { "zh-CN": "使用权资产（ROU）账面净值: ¥3,072,200.00", "zh-HK": "使用權資產（ROU）賬面淨值: ¥3,072,200.00", en: "ROU Asset Carrying Net: ¥3,072,200.00" },
  "landing.demo_ifrs_entries": { "zh-CN": "本期计提：利息费用 ¥13,318.00 / 折旧费用 ¥92,170.00 / 实付租金 ¥50,000.00", "zh-HK": "本期計提：利息費用 ¥13,318.00 / 折舊費用 ¥92,170.00 / 實付租金 ¥50,000.00", en: "Current Accrual: Interest Expense ¥13,318.00 / Depr Expense ¥92,170.00 / Cash Paid ¥50,000.00" },
  "landing.demo_ifrs_meta": { "zh-CN": "ERP 过账状态：已通过审核，待生成凭证 (SAP / Oracle / 用友)", "zh-HK": "ERP 過賬狀態：已通過審核，待生成憑證 (SAP / Oracle / 用友)", en: "ERP Status: Approved · Ready to Post (SAP / Oracle / Yonyou)" },

  "landing.demo_ai_headline": { "zh-CN": "AI 经营分析 Copilot：透明推理、原文定位与人机协同", "zh-HK": "AI 經營分析 Copilot：透明推理、原文定位與人機協同", en: "AI Operating Copilot: Transparent Reasoning, Grounded Evidence & Assist Mode" },
  "landing.demo_ai_desc": { "zh-CN": "严守‘AI 辅助建议、专家审核把关’原则。AI 提取租赁条款附带置信度与原文坐标高亮，经营归因过程透明可追溯，彻底消除黑盒疑虑。", "zh-HK": "嚴守「AI 輔助建議、專家審核把關」原則。AI 提取租賃條款附帶置信度與原文座標高亮，經營歸因過程透明可追溯，徹底消除黑盒疑慮。", en: "Strict human-in-the-loop governance. AI extracts lease clauses with coordinate bounding boxes and confidence scores, providing explainable reasoning." },
  "landing.demo_ai_query": { "zh-CN": "问：为什么上海淮海店上周四墙利润同比下滑 12%？请分析原因并给出行动建议。", "zh-HK": "問：為什麼上海淮海店上周四牆利潤同比下滑 12%？請分析原因並給出行動建議。", en: "User: Why did Shanghai Huaihai store's 4-wall profit drop 12% YoY last week? Provide root causes and action items." },
  "landing.demo_ai_response": { "zh-CN": "答：1. 销售额同比 -8.5%，主要受周边商圈新开业分流影响；2. 当前已触发租约保底扣率跳档点，固定租金未随销售下降；3. 晚间 19:00-21:00 排班过剩 18 工时。建议行动：下调晚间兼职排班，并启动租约抽成比例协商流程。", "zh-HK": "答：1. 銷售額同比 -8.5%，主要受周邊商圈新開業分流影響；2. 當前已觸發租約保底扣率跳檔點，固定租金未隨銷售下降；3. 晚間 19:00-21:00 排班過剩 18 工時。建議行動：下調晚間兼職排班，並啟動租約抽成比例協商流程。", en: "Agent: 1. Sales down 8.5% due to mall footfall diversion; 2. Fixed rent remained rigid despite sales decline; 3. Overstaffed by 18 labor hours during 19:00-21:00. Action: Trim evening shifts and initiate lease turnover rent concession workflow." },
  "landing.demo_ai_free_badge": { "zh-CN": "现已开放个人免费注册：每人每天享有 5 次免费 AI 智能经营诊断与合同提取额度，无需绑卡！", "zh-HK": "現已開放個人免費註冊：每人每天享有 5 次免費 AI 智能經營診斷與合同提取額度，無需綁卡！", en: "Now open for individual free sign-up: 5 free AI requests daily. No credit card required!" },
  "landing.demo_ai_free_cta": { "zh-CN": "立即免费开始使用 →", "zh-HK": "立即免費開始使用 →", en: "Try Free AI Copilot Now →" },

  // Pain Points Matrix
  "landing.pain_badge": { "zh-CN": "传统方案 vs 经营决策工作站", "zh-HK": "傳統方案 vs 經營決策工作站", en: "Traditional Tools vs Workstation" },
  "landing.pain_title": { "zh-CN": "为什么传统 BI 与离线 Excel 难以支撑现代连锁零售？", "zh-HK": "為什麼傳統 BI 與離線 Excel 難以支撐現代連鎖零售？", en: "Why Generic BI & Dispersed Excel Fail Modern Retail Chains" },
  "landing.pain_subtitle": { "zh-CN": "连锁承租方的核心痛点在于：营运销售数据与租赁合同成本脱节，导致大量利润在管理盲区中悄然流失", "zh-HK": "連鎖承租方的核心痛點在於：營運銷售數據與租賃合同成本脫節，導致大量利潤在管理盲區中悄然流失", en: "The core bottleneck for retail tenants is the disconnect between operating sales and lease contracts, leaking massive profits in blind spots." },
  
  "landing.pain_col_trad": { "zh-CN": "传统模式 (通用 BI + 离线 Excel)", "zh-HK": "傳統模式 (通用 BI + 離線 Excel)", en: "Traditional (Generic BI + Offline Excel)" },
  "landing.pain_col_our": { "zh-CN": "线下零售经营决策工作站", "zh-HK": "線下零售經營決策工作站", en: "Retail Performance Workstation" },
  "landing.matrix_dimension": { "zh-CN": "对比维度", "zh-HK": "對比維度", en: "Dimension" },

  "landing.pain_dim1_title": { "zh-CN": "数据时效与响应速度", "zh-HK": "數據時效與響應速度", en: "Data Timeliness & Response" },
  "landing.pain_dim1_trad": { "zh-CN": "依赖月末财务大盘汇总，发现问题已滞后 30–45 天，无法指导日常排班与周度营运调优。", "zh-HK": "依賴月末財務大盤匯總，發現問題已滯後 30–45 天，無法指導日常排班與週度營運調優。", en: "Monthly aggregated reports arrive 30-45 days late, too slow to guide daily/weekly scheduling." },
  "landing.pain_dim1_our": { "zh-CN": "单店·按日（Store-Day）经营颗粒度，每日晨检自动锁定掉队门店与异常信号，实现 T+1 敏捷响应。", "zh-HK": "單店·按日（Store-Day）經營顆粒度，每日晨檢自動鎖定掉隊門店與異常信號，實現 T+1 敏捷響應。", en: "Atomic Store-Day granularity with automated daily morning ranking and T+1 responsive triage." },

  "landing.pain_dim2_title": { "zh-CN": "成本穿透与四墙利润", "zh-HK": "成本穿透與四牆利潤", en: "Cost Transparency & 4-Wall Margin" },
  "landing.pain_dim2_trad": { "zh-CN": "销售归营运、租约归财务/法务，四墙利润难以算清，保底租金多付、非租赁物业费错计常年存在。", "zh-HK": "銷售歸營運、租約歸財務/法務，四牆利潤難以算清，保底租金多付、非租賃物業費錯計常年存在。", en: "Sales managed by ops, leases siloed in legal/finance; true 4-wall margin is opaque, leading to rent leakages." },
  "landing.pain_dim2_our": { "zh-CN": "贯通销售、毛利、人工、固定/抽成租金与物业费，四墙利润瀑布流与租售比一览无余。", "zh-HK": "貫通銷售、毛利、人工、固定/抽成租金與物業費，四牆利潤瀑布流與租售比一覽無餘。", en: "Seamlessly connects sales, gross margin, labor, fixed/variable rent and CAM into unified 4-wall EBITDA." },

  "landing.pain_dim3_title": { "zh-CN": "方案推演与决策依据", "zh-HK": "方案推演與決策依據", en: "Scenario Modeling & Verification" },
  "landing.pain_dim3_trad": { "zh-CN": "续租谈判或降租诉求依赖拍脑袋估算或脆弱的手工模型，缺乏净现值（NPV）与回收期量化支撑。", "zh-HK": "續租談判或降租訴求依賴拍腦袋估算或脆弱的手工模型，缺乏淨現值（NPV）與回收期量化支撐。", en: "Lease renewal or closure decisions rely on gut feelings or fragile spreadsheets with no quantified NPV." },
  "landing.pain_dim3_our": { "zh-CN": "情景工作台动态推演多种谈判假设，一键生成带责任人、截止期与量化预期收益的行动方案。", "zh-HK": "情景工作台動態推演多種談判假設，一鍵生成帶責任人、截止期與量化預期收益的行動方案。", en: "Scenario workbench simulates multi-variable options, outputting verifiable action proposals with owners." },

  "landing.pain_dim4_title": { "zh-CN": "租赁会计与审计合规", "zh-HK": "租賃會計與審計合規", en: "Lease Accounting & Audit" },
  "landing.pain_dim4_trad": { "zh-CN": "IFRS 16 准则测算极其繁琐易错，每年需支付高昂的外部咨询费用，月末结账耗时数天。", "zh-HK": "IFRS 16 準則測算極其繁瑣易錯，每年需支付高昂的外部諮詢費用，月末結賬耗時數天。", en: "IFRS 16 calculation is error-prone, costing high advisory fees and taking days to close each month." },
  "landing.pain_dim4_our": { "zh-CN": "内置准则级 IFRS 16 会计引擎，资产与负债自动滚动，一键过账并导出审计穿透底稿。", "zh-HK": "內置準則級 IFRS 16 會計引擎，資產與負債自動滾動，一鍵過賬並導出審計穿透底稿。", en: "Built-in 100% compliant IFRS 16 engine, automated rollforward, one-click ERP posting and audit trail." },

  "landing.lens_dim1_trad_tag": { "zh-CN": "延迟 30~45 天 · 被动响应", "zh-HK": "延遲 30~45 天 · 被動響應", en: "30-45d Latency · Reactive" },
  "landing.lens_dim1_our_tag": { "zh-CN": "T+1 晨检 · 敏捷归因", "zh-HK": "T+1 晨檢 · 敏捷歸因", en: "T+1 Morning Triage · Agile" },
  "landing.lens_dim2_trad_tag": { "zh-CN": "数据孤岛 · 成本暗流", "zh-HK": "數據孤島 · 成本暗流", en: "Data Silos · Hidden Leakage" },
  "landing.lens_dim2_our_tag": { "zh-CN": "四墙 EBITDA · 租售比透明", "zh-HK": "四牆 EBITDA · 租售比透明", en: "4-Wall EBITDA · Transparent" },
  "landing.lens_dim3_trad_tag": { "zh-CN": "拍脑袋估算 · 谈判失据", "zh-HK": "拍腦袋估算 · 談判失據", en: "Gut Feeling · Fragile Models" },
  "landing.lens_dim3_our_tag": { "zh-CN": "NPV 量化 · 责任人闭环", "zh-HK": "NPV 量化 · 責任人閉環", en: "Quantified NPV · Accountable" },
  "landing.lens_dim4_trad_tag": { "zh-CN": "手工易错 · 耗时数天", "zh-HK": "手工易錯 · 耗時數天", en: "Manual Errors · Days to Close" },
  "landing.lens_dim4_our_tag": { "zh-CN": "准则级自动化 · 2小时关账", "zh-HK": "準則級自動化 · 2小時關賬", en: "Automated · <2h Month-End Close" },

  // Lens Detailed Visual Diagnostics & Solutions
  "landing.lens_d0_t_m1_l": { "zh-CN": "月度报表周期", "zh-HK": "月度報表週期", en: "Monthly Reporting Lag" },
  "landing.lens_d0_t_m1_v": { "zh-CN": "滞后 30~45 天 (T+45)", "zh-HK": "滯後 30~45 天 (T+45)", en: "30-45 Days Lag (T+45)" },
  "landing.lens_d0_t_m2_l": { "zh-CN": "排班错配损失", "zh-HK": "排班錯配損失", en: "Labor Scheduling Mismatch" },
  "landing.lens_d0_t_m2_v": { "zh-CN": "无法及时调整", "zh-HK": "無法及時調整", en: "Cannot Adjust In-Time" },
  "landing.lens_d0_t_m3_l": { "zh-CN": "异常处理机制", "zh-HK": "異常處理機制", en: "Anomaly Triage Mechanism" },
  "landing.lens_d0_t_m3_v": { "zh-CN": "事后被动复盘", "zh-HK": "事後被動復盤", en: "Passive Post-Mortem" },

  "landing.lens_d0_o_m1_l": { "zh-CN": "Store-Day 日级对账", "zh-HK": "Store-Day 日級對賬", en: "Store-Day Reconciliation" },
  "landing.lens_d0_o_m1_v": { "zh-CN": "T+1 晨检 (09:00 就绪)", "zh-HK": "T+1 晨檢 (09:00 就緒)", en: "T+1 Morning Triage (09:00)" },
  "landing.lens_d0_o_m2_l": { "zh-CN": "异常自动归因", "zh-HK": "異常自動歸因", en: "Automated Root-Cause" },
  "landing.lens_d0_o_m2_v": { "zh-CN": "健康分与关注度排行", "zh-HK": "健康分與關注度排行", en: "Deterministic Severity Score" },
  "landing.lens_d0_o_m3_l": { "zh-CN": "营运响应时效", "zh-HK": "營運響應時效", en: "Operational Agility" },
  "landing.lens_d0_o_m3_v": { "zh-CN": "当日指令 · 周度调优", "zh-HK": "當日指令 · 週度調優", en: "Daily Task · Weekly Tuning" },

  "landing.lens_d1_t_m1_l": { "zh-CN": "四墙利润可见度", "zh-HK": "四牆利潤可見度", en: "4-Wall Profit Visibility" },
  "landing.lens_d1_t_m1_v": { "zh-CN": "数据孤岛 · 无法穿透", "zh-HK": "數據孤島 · 無法穿透", en: "Siloed Data · Opaque Margins" },
  "landing.lens_d1_t_m2_l": { "zh-CN": "租金多付/漏计", "zh-HK": "租金多付/漏計", en: "Rent Concession Leakage" },
  "landing.lens_d1_t_m2_v": { "zh-CN": "暗流年化 ~¥45,000", "zh-HK": "暗流年化 ~¥45,000", en: "Hidden Loss ~¥45K/Year" },
  "landing.lens_d1_t_m3_l": { "zh-CN": "物业非租分摊", "zh-HK": "物業非租分攤", en: "Non-Lease CAM Split" },
  "landing.lens_d1_t_m3_v": { "zh-CN": "手工粗分 · 审计风险", "zh-HK": "手工粗分 · 審計風險", en: "Rough Manual Split · Audit Risk" },

  "landing.lens_d1_o_m1_l": { "zh-CN": "四墙 EBITDA 穿透", "zh-HK": "四牆 EBITDA 穿透", en: "4-Wall EBITDA Decomposition" },
  "landing.lens_d1_o_m1_v": { "zh-CN": "销售+毛利+人工+租金贯通", "zh-HK": "銷售+毛利+人工+租金貫通", en: "Sales + Margin + HR + Rent" },
  "landing.lens_d1_o_m2_l": { "zh-CN": "租金保底/抽成", "zh-HK": "租金保底/抽成", en: "Turnover vs Base Rent" },
  "landing.lens_d1_o_m2_v": { "zh-CN": "合同条款秒级自动校验", "zh-HK": "合同條款秒級自動校驗", en: "Automated Clause Validation" },
  "landing.lens_d1_o_m3_l": { "zh-CN": "利润率优化空间", "zh-HK": "利潤率優化空間", en: "Margin Optimization" },
  "landing.lens_d1_o_m3_v": { "zh-CN": "平均增厚 +3.2%", "zh-HK": "平均增厚 +3.2%", en: "Avg +3.2% Margin Lift" },

  "landing.lens_d2_t_m1_l": { "zh-CN": "谈判方案测算", "zh-HK": "談判方案測算", en: "Negotiation Modeling" },
  "landing.lens_d2_t_m1_v": { "zh-CN": "拍脑袋 / 简单乘除", "zh-HK": "拍腦袋 / 簡單乘除", en: "Gut Feeling / Primitive Math" },
  "landing.lens_d2_t_m2_l": { "zh-CN": "NPV 净现值评估", "zh-HK": "NPV 淨現值評估", en: "NPV Valuation" },
  "landing.lens_d2_t_m2_v": { "zh-CN": "缺失动态现金流贴现", "zh-HK": "缺失動態現金流貼現", en: "Lacks Discounted Cash Flow" },
  "landing.lens_d2_t_m3_l": { "zh-CN": "行动跟进机制", "zh-HK": "行動跟進機制", en: "Action Execution Track" },
  "landing.lens_d2_t_m3_v": { "zh-CN": "无责任人与截止期追踪", "zh-HK": "無責任人與截止期追蹤", en: "No Owners or Deadlines" },

  "landing.lens_d2_o_m1_l": { "zh-CN": "情景敏感性推演", "zh-HK": "情景敏感性推演", en: "Scenario Sensitivity Engine" },
  "landing.lens_d2_o_m1_v": { "zh-CN": "NPV / 回收期 / 折让多方案", "zh-HK": "NPV / 回收期 / 折讓多方案", en: "NPV / Payback / Concessions" },
  "landing.lens_d2_o_m2_l": { "zh-CN": "谈判底牌备忘录", "zh-HK": "談判底牌備忘錄", en: "Decision Memo Export" },
  "landing.lens_d2_o_m2_v": { "zh-CN": "一键导出标准报告递交业主", "zh-HK": "一鍵導出標準報告遞交業主", en: "One-Click PDF for Landlord" },
  "landing.lens_d2_o_m3_l": { "zh-CN": "闭环追踪机制", "zh-HK": "閉環追蹤機制", en: "Closed-Loop Governance" },
  "landing.lens_d2_o_m3_v": { "zh-CN": "带责任人与截止期跟进", "zh-HK": "帶責任人與截止期跟進", en: "Assigned Owners & Target Dates" },

  "landing.lens_d3_t_m1_l": { "zh-CN": "IFRS 16 准则计量", "zh-HK": "IFRS 16 準則計量", en: "IFRS 16 Compliance" },
  "landing.lens_d3_t_m1_v": { "zh-CN": "离线 Excel 复杂公式", "zh-HK": "離線 Excel 複雜公式", en: "Fragile Offline Spreadsheets" },
  "landing.lens_d3_t_m2_l": { "zh-CN": "月末关账周期", "zh-HK": "月末關賬週期", en: "Month-End Close Timelines" },
  "landing.lens_d3_t_m2_v": { "zh-CN": "耗时 5 天 / 每月", "zh-HK": "耗時 5 天 / 每月", en: "5 Days Spent Every Month" },
  "landing.lens_d3_t_m3_l": { "zh-CN": "外部咨询成本", "zh-HK": "外部諮詢成本", en: "External Advisory Cost" },
  "landing.lens_d3_t_m3_v": { "zh-CN": "每年 ¥100K+ 额外费用", "zh-HK": "每年 ¥100K+ 額外費用", en: "¥100K+/Year Advisory Fees" },

  "landing.lens_d3_o_m1_l": { "zh-CN": "IFRS 16 准则引擎", "zh-HK": "IFRS 16 準則引擎", en: "IFRS 16 Rule Engine" },
  "landing.lens_d3_o_m1_v": { "zh-CN": "ROU 资产与负债全自动滚动", "zh-HK": "ROU 資產與負債全自動滾動", en: "Automated ROU & Liability" },
  "landing.lens_d3_o_m2_l": { "zh-CN": "月末关账耗时", "zh-HK": "月末關賬耗時", en: "Month-End Close Speed" },
  "landing.lens_d3_o_m2_v": { "zh-CN": "<2 小时完成核算与审批", "zh-HK": "<2 小時完成核算與審批", en: "<2 Hours Automated Posting" },
  "landing.lens_d3_o_m3_l": { "zh-CN": "审计级穿透底稿", "zh-HK": "審計級穿透底稿", en: "Audit-Ready Lead Sheets" },
  "landing.lens_d3_o_m3_v": { "zh-CN": "22/22 审计断言通过", "zh-HK": "22/22 審計斷言通過", en: "22/22 Audit Assertions Passed" },

  // Data Architecture Section
  "landing.arch_badge": { "zh-CN": "全链路数据架构", "zh-HK": "全鏈路數據架構", en: "Technical Architecture" },
  "landing.arch_title": { "zh-CN": "从源头业务系统到财务总账的一体化数据底座", "zh-HK": "從源頭業務系統到財務總賬的一體化數據底座", en: "Unified Data Pipeline from POS to General Ledger" },
  "landing.arch_subtitle": { "zh-CN": "打破系统壁垒与数据口径割裂，实现跨系统 T+1 自动治理、原子级核算与准则级合规", "zh-HK": "打破系統壁壘與數據口徑割裂，實現跨系統 T+1 自動治理、原子級核算與準則級合規", en: "Connecting siloed systems into a unified deterministic retail semantic layer." },

  "landing.arch_layer1": { "zh-CN": "全渠道数据源 Ingestion", "zh-HK": "全渠道數據源 Ingestion", en: "Multi-Source Ingestion" },
  "landing.arch_layer2": { "zh-CN": "原子级指标治理引擎 Governance", "zh-HK": "原子級指標治理引擎 Governance", en: "Store-Day Fact Engine" },
  "landing.arch_layer3": { "zh-CN": "双核分析与合规计算 Core Compute", "zh-HK": "雙核分析與合規計算 Core Compute", en: "Dual Analytics & IFRS 16" },
  "landing.arch_layer4": { "zh-CN": "企业级价值输出 Outputs", "zh-HK": "企業級價值輸出 Outputs", en: "Enterprise Outputs" },

  "landing.arch_node_pos": { "zh-CN": "POS 门店销售日流水", "zh-HK": "POS 門店銷售日流水", en: "POS Store Daily Sales" },
  "landing.arch_node_hr": { "zh-CN": "HR 考勤与兼职排班", "zh-HK": "HR 考勤與兼職排班", en: "HR Attendance & Shifts" },
  "landing.arch_node_lease": { "zh-CN": "承租合同与关键条款", "zh-HK": "承租合同與關鍵條款", en: "Lease Contracts & Terms" },
  "landing.arch_node_erp_flow": { "zh-CN": "ERP 实际付款流水", "zh-HK": "ERP 實際付款流水", en: "ERP Cash Payment Records" },
  "landing.arch_node_reconcile": { "zh-CN": "Store-Day 原子级对账", "zh-HK": "Store-Day 原子級對賬", en: "Store-Day Reconciliation" },
  "landing.arch_node_semantics": { "zh-CN": "统一连锁指标语义标准", "zh-HK": "統一連鎖指標語義標準", en: "Retail Metric Semantics" },
  "landing.arch_node_currency": { "zh-CN": "多币种物理分区隔离", "zh-HK": "多幣種物理分區隔離", en: "Multi-Currency Partitioning" },
  "landing.arch_node_provenance": { "zh-CN": "全链路数据凭证留痕", "zh-HK": "全鏈路數據憑證留痕", en: "Audit Provenance Envelopes" },
  "landing.arch_node_ebitda": { "zh-CN": "四墙 EBITDA 瀑布流模型", "zh-HK": "四牆 EBITDA 瀑布流模型", en: "4-Wall EBITDA Waterfall Model" },
  "landing.arch_node_cohort": { "zh-CN": "同类门店群组科学对标", "zh-HK": "同類門店群組科學對標", en: "Peer Cohort Benchmarking" },
  "landing.arch_node_ifrs": { "zh-CN": "IFRS 16 准则计量引擎", "zh-HK": "IFRS 16 準則計量引擎", en: "IFRS 16 Compliance Engine" },
  "landing.arch_node_scenario": { "zh-CN": "租约 NPV 情景敏感性推演", "zh-HK": "租約 NPV 情景敏感性推演", en: "Lease NPV Scenario Modeling" },
  "landing.arch_node_pulse": { "zh-CN": "T+1 晨检异动关注清单", "zh-HK": "T+1 晨檢異動關注清單", en: "T+1 Anomaly Triage List" },
  "landing.arch_node_action": { "zh-CN": "带责任人的降租行动方案", "zh-HK": "帶責任人的降租行動方案", en: "Accountable Action Proposals" },
  "landing.arch_node_posting": { "zh-CN": "SAP / Oracle / 用友 凭证过账", "zh-HK": "SAP / Oracle / 用友 憑證過賬", en: "SAP / Oracle Journal Posting" },
  "landing.arch_node_audit": { "zh-CN": "四大审计级穿透底稿", "zh-HK": "四大審計級穿透底稿", en: "Audit-Ready Lead Sheets" },

  "landing.deepdive_anomaly_critical": { "zh-CN": "淮海中路店：租售比达到 28.4%（触发出租方阶梯保底条款跳档）", "zh-HK": "淮海中路店：租售比達到 28.4%（觸發出租方階梯保底條款跳檔）", en: "Huaihai Flagship: Rent-to-sales hit 28.4% (Triggering base rent tier bump)" },
  "landing.deepdive_anomaly_warning": { "zh-CN": "三里屯太古里店：晚间 19:00-21:00 销售与工时错配（多余排班 14 小时）", "zh-HK": "三里屯太古里店：晚間 19:00-21:00 銷售與工時錯配（多餘排班 14 小時）", en: "Sanlitun Store: 19:00-21:00 labor mismatch (14 surplus hours scheduled)" },
  "landing.deepdive_anomaly_resolved": { "zh-CN": "天河城店：新营销套餐上线，四墙毛利恢复至 58.2%", "zh-HK": "天河城店：新營銷套餐上線，四牆毛利恢復至 58.2%", en: "Teemall Store: Promo campaign launched, gross margin restored to 58.2%" },
  "landing.deepdive_scenario_opt1_title": { "zh-CN": "方案 A：争取 15% 租金折让 (月租由 ¥92.4K 降至 ¥78.5K)", "zh-HK": "方案 A：爭取 15% 租金折讓 (月租由 ¥92.4K 降至 ¥78.5K)", en: "Option A: 15% Rent Concession (Down from ¥92.4K to ¥78.5K/mo)" },
  "landing.deepdive_memo_desc": { "zh-CN": "已生成标准谈判底牌备忘录，带责任人（华东租赁总监）、截止日期（30天内），支持一键导出 PDF 递交业主协商。", "zh-HK": "已生成標準談判底牌備忘錄，帶責任人（華東租賃總監）、截止日期（30天內），支持一鍵導出 PDF 遞交業主協商。", en: "Generated standardized negotiation memo with owner (East China Lease Dir), 30-day deadline, and PDF export." },

  // 4 Core Value Pillars
  "landing.pillars_badge": { "zh-CN": "三大核心业务纵深", "zh-HK": "三大核心業務縱深", en: "Three Core Capabilities" },
  "landing.pillars_title": { "zh-CN": "构建连锁零售承租方的高确定性经营飞轮", "zh-HK": "構建連鎖零售承租方的高確定性經營飛輪", en: "Building a High-Certainty Operating Flywheel" },
  "landing.pillars_subtitle": { "zh-CN": "从日常监测、深度归因到模拟决策与审计过账，全方位提升连锁经营质量与资本回报率", "zh-HK": "從日常監測、深度歸因到模擬決策與審計過賬，全方位提升連鎖經營質量與資本回報率", en: "From daily monitoring and root cause analysis to scenario simulation and audit posting, elevating retail capital efficiency." },

  "landing.pillar_widget_sync": { "zh-CN": "POS流水 + HR工时 + 租赁台账", "zh-HK": "POS流水 + HR工時 + 租賃台賬", en: "POS Sales + HR Shifts + Leases" },
  "landing.pillar_widget_sync_status": { "zh-CN": "全自动对账", "zh-HK": "全自動對賬", en: "Auto-Reconciled" },
  "landing.pillar_widget_npv_opt": { "zh-CN": "方案 A: 降租 15%", "zh-HK": "方案 A: 降租 15%", en: "Option A: 15% Discount" },
  "landing.pillar_widget_npv_val": { "zh-CN": "+¥382,000 NPV", "zh-HK": "+¥382,000 NPV", en: "+¥382,000 NPV" },
  "landing.pillar_widget_ifrs_ledger": { "zh-CN": "ROU 资产 + 负债滚动表", "zh-HK": "ROU 資產 + 負債滾動表", en: "ROU Asset & Liability Schedules" },
  "landing.pillar_widget_ifrs_assert": { "zh-CN": "22/22 审计断言通过", "zh-HK": "22/22 審計斷言通過", en: "22/22 Audit Assertions Passed" },

  "landing.pillar1_title": { "zh-CN": "01. 单店日级经营脉搏与严密指标治理", "zh-HK": "01. 單店日級經營脈搏與嚴密指標治理", en: "01. Store-Day Operating Pulse & Deterministic Semantics" },
  "landing.pillar1_desc": { "zh-CN": "统一接入 POS、客流与排班系统数据，确立严密的指标计算口径。自动处理跨系统数据版本，绝不用虚假零值掩盖缺失，确保每项管理决策真实可信。", "zh-HK": "統一接入 POS、客流與排班系統數據，確立嚴密的指標計算口徑。自動處理跨系統數據版本，絕不用虛假零值掩蓋缺失，確保每項管理決策真實可信。", en: "Consolidates POS, footfall, and scheduling data with strict retail semantics. Prevents source conflicts and never zero-fills missing data, ensuring decision-ready integrity." },
  "landing.pillar1_f1": { "zh-CN": "日级经营异常自动打分与关注度排行榜", "zh-HK": "日級經營異常自動打分與關注度排行榜", en: "Daily store anomaly attention scoring and ranking" },
  "landing.pillar1_f2": { "zh-CN": "数据完整就绪（Decision-Ready）状态标识", "zh-HK": "數據完整就緒（Decision-Ready）狀態標識", en: "Explicit Decision-Ready badge & provenance tracking" },
  "landing.pillar1_f3": { "zh-CN": "多币种物理分区，杜绝汇率失真与混算", "zh-HK": "多幣種物理分區，杜絕匯率失真與混算", en: "Multi-currency strict partitioning without distortions" },

  "landing.pillar2_title": { "zh-CN": "02. 门店 360 诊断与同类门店科学对标", "zh-HK": "02. 門店 360 診斷與同類門店科學對標", en: "02. Store 360 Diagnostics & Scientific Peer Cohort" },
  "landing.pillar2_desc": { "zh-CN": "不仅看销售大盘，更穿透门店四墙经济性与占用成本率。将门店置于同品牌、同区域、同商圈的真实群组中对标，直观拆解毛利、人工、租金对利润波动的具体贡献。", "zh-HK": "不僅看銷售大盤，更穿透門店四牆經濟性與佔用成本率。將門店置於同品牌、同區域、同商圈的真實群組中對標，直觀拆解毛利、人工、租金對利潤波動的具體貢獻。", en: "Deep dives into four-wall margins and occupancy ratios. Benchmarks stores against true peer cohorts, decomposing profit variances into revenue, margin, labor, and lease drivers." },
  "landing.pillar2_f1": { "zh-CN": "门店四墙利润 (4-Wall EBITDA) 瀑布流拆解", "zh-HK": "門店四牆利潤 (4-Wall EBITDA) 瀑布流拆解", en: "Four-wall EBITDA contribution waterfall breakdown" },
  "landing.pillar2_f2": { "zh-CN": "P25 / P50 / P75 同类门店分位数科学对标", "zh-HK": "P25 / P50 / P75 同類門店分位數科學對標", en: "P25 / P50 / P75 quartile positioning within cohort" },
  "landing.pillar2_f3": { "zh-CN": "固定租金 vs 营业额抽成租金精细化核算", "zh-HK": "固定租金 vs 營業額抽成租金精細化核算", en: "Fixed vs turnover variable rent precise reconciliation" },

  "landing.pillar3_title": { "zh-CN": "03. 租约情景推演与可量化行动闭环", "zh-HK": "03. 租約情景推演與可量化行動閉環", en: "03. Lease Scenario Workbench & Verifiable Action Loop" },
  "landing.pillar3_desc": { "zh-CN": "在合同续签、面积变更、降租谈判或提前解约等关键节点，一键启动情景推演。量化不同假设下的投资净现值（NPV）、租售比与回收期，生成带责任人的决策行动方案。", "zh-HK": "在合同續簽、面積變更、降租談判或提前解約等關鍵節點，一鍵啟動情景推演。量化不同假設下的投資淨現值（NPV）、租售比與回收期，生成帶責任人的決策行動方案。", en: "Launch scenario modeling for lease renewals, rent concessions, or early terminations. Quantify NPV, rent-to-sales, and payback periods to generate accountable action proposals." },
  "landing.pillar3_f1": { "zh-CN": "多变量敏感性模拟（销售弹性 / 租金折让 / 免租期）", "zh-HK": "多變量敏感性模擬（銷售彈性 / 租金折讓 / 免租期）", en: "Multi-variable sensitivity modeling (Sales elastic / Rent discount)" },
  "landing.pillar3_f2": { "zh-CN": "一键生成标准化决策备忘录 (Decision Memo)", "zh-HK": "一鍵生成標準化決策備忘錄 (Decision Memo)", en: "One-click standardized Decision Memo generation" },
  "landing.pillar3_f3": { "zh-CN": "行动闭环跟踪，对比预期收益与实际兑现效果", "zh-HK": "行動閉環跟蹤，對比預期收益與實際兌現效果", en: "Action tracking comparing expected vs realized profit" },

  "landing.pillar4_title": { "zh-CN": "04. 审计级 IFRS 16 租赁会计引擎与自动化关账", "zh-HK": "04. 審計級 IFRS 16 租賃會計引擎與自動化關賬", en: "04. Audit-Grade IFRS 16 Lease Engine & Month-End Close" },
  "landing.pillar4_desc": { "zh-CN": "无需采购昂贵且独立的租赁合规软件。内置准则级计量模型，精准处理先付/后付、免租期、租约重估与指数调整，自动生成利息与折旧分录，无缝对接主流 ERP。", "zh-HK": "無需採購昂貴且獨立的租賃合規軟件。內置準則級計量模型，精準處理先付/後付、免租期、租約重估與指數調整，自動生成利息與折舊分錄，無縫對接主流 ERP。", en: "No need for disconnected lease tools. Built-in standard accounting handles advance/arrears, rent-free terms, modifications, generating entries for ERP integration." },
  "landing.pillar4_f1": { "zh-CN": "使用权资产（ROU）与租赁负债全自动滚动", "zh-HK": "使用權資產（ROU）與租賃負債全自動滾動", en: "ROU asset & lease liability automated rollforward" },
  "landing.pillar4_f2": { "zh-CN": "严格的月末关账审批闸门与期间锁定机制", "zh-HK": "嚴格的月末關賬審批閘門與期間鎖定機制", en: "Strict month-end closing approval gate & lock control" },
  "landing.pillar4_f3": { "zh-CN": "一键导出标准总账凭证与审计穿透底稿", "zh-HK": "一鍵導出標準總賬憑證與審計穿透底稿", en: "One-click ERP journal entry export & audit trails" },

  // ROI Calculator
  "landing.calc_badge": { "zh-CN": "效益与优化空间测算", "zh-HK": "效益與優化空間測算", en: "Interactive ROI Calculator" },
  "landing.calc_title": { "zh-CN": "测算您的连锁品牌在四墙利润与租约优化中的潜在收益", "zh-HK": "測算您的連鎖品牌在四牆利潤與租約優化中的潛在收益", en: "Estimate Your Brand's Four-Wall Margin & Lease Lift" },
  "landing.calc_subtitle": { "zh-CN": "设定您的门店规模与营运参数，即时获取年化利润提升潜力与财务工时节省评估", "zh-HK": "設定您的門店規模與營運參數，即時獲取年化利潤提升潛力與財務工時節省評估", en: "Adjust parameters below to see projected annual profit uplift and finance time savings." },
  "landing.calc_stores": { "zh-CN": "门店总数（家）", "zh-HK": "門店總數（間）", en: "Total Store Count" },
  "landing.calc_revenue": { "zh-CN": "单店月均销售额（万元）", "zh-HK": "單店月均銷售額（萬元）", en: "Avg Monthly Store Sales (10k RMB)" },
  "landing.calc_rent_ratio": { "zh-CN": "当前租售比（%）", "zh-HK": "當前租售比（%）", en: "Current Rent-to-Sales Ratio (%)" },
  "landing.calc_labor_ratio": { "zh-CN": "当前人工成本率（%）", "zh-HK": "當前人工成本率（%）", en: "Current Labor Ratio (%)" },

  "landing.calc_res_profit": { "zh-CN": "预估年化四墙利润提升", "zh-HK": "預估年化四牆利潤提升", en: "Estimated Annual 4-Wall Profit Lift" },
  "landing.calc_res_profit_tip": { "zh-CN": "基于平均 2.5%~3.5% 四墙利润率优化（租约谈判降租、排班削峰填谷、异常门店治理）", "zh-HK": "基於平均 2.5%~3.5% 四牆利潤率優化（租約談判降租、排班削峰填谷、異常門店治理）", en: "Based on 2.5%~3.5% margin lift via lease renegotiation, labor optimization & store triage" },
  "landing.calc_res_leakage": { "zh-CN": "识别并挽回的租约成本渗漏", "zh-HK": "識別並挽回的租約成本滲漏", en: "Recovered Lease Cost Leakages" },
  "landing.calc_res_leakage_tip": { "zh-CN": "涵盖抽成保底租金错算、未享免租期、物业费非租赁成本虚高等历史多付费用", "zh-HK": "涵蓋抽成保底租金錯算、未享免租期、物業費非租賃成本虛高等歷史多付費用", en: "Includes turnover rent threshold errors, missed rent-free terms, and CAM overcharges" },
  "landing.calc_res_hours": { "zh-CN": "节省的财务与营运分析工时", "zh-HK": "節省的財務與營運分析工時", en: "Finance & FP&A Hours Saved" },
  "landing.calc_res_hours_unit": { "zh-CN": "小时 / 年", "zh-HK": "小時 / 年", en: "Hours / Year" },
  "landing.calc_res_hours_tip": { "zh-CN": "替代繁重的手工 Excel 关账、IFRS 16 摊销测算与周度营运报表拼表", "zh-HK": "替代繁重的手工 Excel 關賬、IFRS 16 攤銷測算與週度營運報表拼表", en: "Eliminating manual spreadsheets, IFRS 16 calculations, and WBR report consolidation" },
  "landing.calc_cta": { "zh-CN": "获取专属门店诊断报告与试算明细", "zh-HK": "獲取專屬門店診斷報告與試算明細", en: "Get Customized Store Diagnostics & ROI Breakdown" },

  // Personas
  "landing.persona_badge": { "zh-CN": "赋能核心决策角色", "zh-HK": "賦能核心決策角色", en: "Empowering Key Stakeholders" },
  "landing.persona_title": { "zh-CN": "让连锁经营链条上的每一个岗位获得确定性抓手", "zh-HK": "讓連鎖經營鏈條上的每一個崗位獲得確定性抓手", en: "Delivering Actionable Certainty for Every Retail Leader" },
  "landing.persona_subtitle": { "zh-CN": "从高管层战略研判到一线门店执行，统一数据口径与协同语言", "zh-HK": "從高管層戰略研判到一線門店執行，統一數據口徑與協同語言", en: "Aligning executive strategy with frontline store execution through single source of truth." },

  "landing.persona_coo_title": { "zh-CN": "营运副总裁 / COO", "zh-HK": "營運副總裁 / COO", en: "VP of Operations / COO" },
  "landing.persona_coo_quote": { "zh-CN": "“不用再等月末报表。每天早上自动锁定异常门店，原因直指转化率或排班峰值，闭环任务跟进率从 30% 提升到了 85%。”", "zh-HK": "「不用再等月末報表。每天早上自動鎖定異常門店，原因直指轉化率或排班峰值，閉環任務跟進率從 30% 提升到了 85%。」", en: "\"No more waiting for monthly spreadsheets. Daily morning pulse pins down dropping stores with exact drivers, boosting task resolution from 30% to 85%.\"" },
  "landing.persona_coo_tag": { "zh-CN": "关注点：日常经营节奏、掉队门店归因、行动执行率", "zh-HK": "關注點：日常經營節奏、掉隊門店歸因、行動執行率", en: "Focus: Operating cadence, anomaly root cause, action execution" },

  "landing.persona_cfo_title": { "zh-CN": "财务总监 / CFO & FP&A", "zh-HK": "財務總監 / CFO & FP&A", en: "CFO & FP&A Director" },
  "landing.persona_cfo_quote": { "zh-CN": "“把承租合同与四墙利润彻底贯通，租售比与续租谈判有了最硬核的底牌；同时 IFRS 16 准则月结自动化，关账时间缩短了 70%。”", "zh-HK": "「把承租合同與四牆利潤徹底貫通，租售比與續租談判有了最硬核的底牌；同時 IFRS 16 準則月結自動化，關賬時間縮短了 70%。」", en: "\"Connecting lease contracts with 4-wall margins gives us unmatched leverage in renewals. Plus, automated IFRS 16 cuts close time by 70%.\"" },
  "landing.persona_cfo_tag": { "zh-CN": "关注点：四墙利润率、租金资本化合规、ERP 凭证过账", "zh-HK": "關注點：四牆利潤率、租金資本化合規、ERP 憑證過賬", en: "Focus: Four-wall EBITDA, lease capitalization, ERP journal posting" },

  "landing.persona_dev_title": { "zh-CN": "拓展与租赁谈判负责人", "zh-HK": "拓展與租賃談判負責人", en: "Real Estate & Lease Director" },
  "landing.persona_dev_quote": { "zh-CN": "“到期前 90 天自动触发预警，一键推演转租、降租、改抽成扣率的净现值收益，让每一场商业谈判都带着清晰的量化模型。”", "zh-HK": "「到期前 90 天自動觸發預警，一鍵推演轉租、降租、改抽成扣率的淨現值收益，讓每一場商業談判都帶著清晰的量化模型。」", en: "\"Automated 90-day expiry triggers and instant NPV simulations for concessions/renegotiations give us rigorous quantitative negotiation decks.\"" },
  "landing.persona_dev_tag": { "zh-CN": "关注点：关键日期预警、租约条款情景模拟、免租期追踪", "zh-HK": "關注點：關鍵日期預警、租約條款情景模擬、免租期追蹤", en: "Focus: Critical date alerts, lease terms simulation, rent-free periods" },

  "landing.persona_mgr_title": { "zh-CN": "区域督导 / 店长", "zh-HK": "區域督導 / 店長", en: "Regional Coach & Store Manager" },
  "landing.persona_mgr_quote": { "zh-CN": "“平台不再只是冷冰冰的数据图表，而是直接派发具体且有期限的行动建议。做完动作后，能直观看到自己为门店贡献了多少真金白银。”", "zh-HK": "「平台不再只是冷冰冰的數據圖表，而是直接派發具體且有期限的行動建議。做完動作後，能直觀看到自己為門店貢獻了多少真金白銀。」", en: "\"Instead of cold dashboards, it gives actionable tasks with clear deadlines. After executing, we can directly see the realized profit uplift.\"" },
  "landing.persona_mgr_tag": { "zh-CN": "关注点：排班优化建议、促销增量毛利、收益兑现复盘", "zh-HK": "關注點：排班優化建議、促銷增量毛利、收益兌現復盤", en: "Focus: Shift scheduling, promotion margin, realized profit review" },

  // Trust & Security
  "landing.trust_badge": { "zh-CN": "企业级安全与审计保障", "zh-HK": "企業級安全與審計保障", en: "Enterprise Security & Compliance" },
  "landing.trust_title": { "zh-CN": "五条严密安全底线，保障集团核心数据主权", "zh-HK": "五條嚴密安全底線，保障集團核心數據主權", en: "Five Strict Guardrails Protecting Enterprise Data Sovereignty" },
  "landing.trust_subtitle": { "zh-CN": "专为多品牌、多法人主体的连锁零售集团设计，提供行级数据隔离与全链路审计追溯", "zh-HK": "專為多品牌、多法人主體的連鎖零售集團設計，提供行級數據隔離與全鏈路審計追溯", en: "Architected for enterprise multi-brand, multi-entity retailers with strict isolation and audit trails." },

  "landing.trust_g1_title": { "zh-CN": "多法人与多品牌安全隔离", "zh-HK": "多法人與多品牌安全隔離", en: "Cross-Entity Hard Isolation" },
  "landing.trust_g1_desc": { "zh-CN": "基于组织架构的行级权限隔离，法人 A 绝无法查看或修改法人 B 的任何经营事实、合同及行动方案。", "zh-HK": "基於組織架構的行級權限隔離，法人 A 絕無法查看或修改法人 B 的任何經營事實、合同及行動方案。", en: "Row-level tenant guardrails ensure Legal Entity A can never access Entity B's facts or contracts." },

  "landing.trust_g2_title": { "zh-CN": "推演与正式数据物理区分", "zh-HK": "推演與正式數據物理區分", en: "Simulation / Official Data Separation" },
  "landing.trust_g2_desc": { "zh-CN": "情景推演与模拟数据全程携带隔离标记，绝不进入正式的 IFRS 16 财务过账与总账链路。", "zh-HK": "情景推演與模擬數據全程攜帶隔離標記，絕不進入正式的 IFRS 16 財務過賬與總賬鏈路。", en: "Simulated scenarios carry strict classification tags and never enter the official financial posting chain." },

  "landing.trust_g3_title": { "zh-CN": "全链路数据凭证与追溯", "zh-HK": "全鏈路數據憑證與追溯", en: "Full-Chain Audit Provenance" },
  "landing.trust_g3_desc": { "zh-CN": "每条经营事实与合同条款均包含来源系统、导入批次、版本号与时间戳，审计随时可复查复演。", "zh-HK": "每條經營事實與合同條款均包含來源系統、導入批次、版本號與時間戳，審計隨時可復查復演。", en: "Every operating fact and clause carries source system, batch ID, version, and as-of timestamp for audit replay." },

  "landing.trust_g4_title": { "zh-CN": "AI 辅助模式 · 人工终审把关", "zh-HK": "AI 輔助模式 · 人工終審把關", en: "Human-in-the-Loop Assist Mode" },
  "landing.trust_g4_desc": { "zh-CN": "AI 识别提取结果必须经草稿层、置信度校验与人工确认方可入库，绝不直接擅自修改正式台账。", "zh-HK": "AI 識別提取結果必須經草稿層、置信度校驗與人工確認方可入庫，絕不直接擅自修改正式台賬。", en: "AI extractions pass through draft layers and human approval gates, never directly writing to formal ledgers." },

  // Pricing
  "landing.pricing_badge": { "zh-CN": "透明阶梯方案", "zh-HK": "透明階梯方案", en: "Transparent Plans" },
  "landing.pricing_title": { "zh-CN": "选择适合您连锁规模的专业赋能方案", "zh-HK": "選擇適合您連鎖規模的專業賦能方案", en: "Select the Right Plan for Your Chain Scale" },
  "landing.pricing_subtitle": { "zh-CN": "现已开放个人免费体验，支持从单店个人用户到大型跨国多法人集团的平滑扩展", "zh-HK": "現已開放個人免費體驗，支持從單店個人用戶到大型跨國多法人集團的平滑擴展", en: "Now open for individual free access. Scales seamlessly from single stores to enterprise groups." },

  "landing.plan_free_title": { "zh-CN": "个人免费版 Free", "zh-HK": "個人免費版 Free", en: "Individual Free" },
  "landing.plan_free_badge": { "zh-CN": "免费体验", "zh-HK": "免費體驗", en: "Free Forever" },
  "landing.plan_free_desc": { "zh-CN": "适合单店店长、个体零售经营者与独立分析师，无需绑定信用卡", "zh-HK": "適合單店店長、個體零售經營者與獨立分析師，無需綁定信用卡", en: "For single-store owners, retail freelancers & independent analysts" },
  "landing.plan_free_price": { "zh-CN": "¥0 永久免费", "zh-HK": "¥0 永久免費", en: "¥0 / Free Forever" },
  "landing.plan_free_f1": { "zh-CN": "每日赠送 5 次 AI 智能经营分析", "zh-HK": "每日贈送 5 次 AI 智能經營分析", en: "5 Free AI Copilot Requests Daily" },
  "landing.plan_free_f2": { "zh-CN": "单店日级经营脉搏与四墙利润诊断", "zh-HK": "單店日級經營脈搏與四牆利潤診斷", en: "Single-store pulse & 4-wall margin diagnostics" },
  "landing.plan_free_f3": { "zh-CN": "基础租赁合同录入与到期预警", "zh-HK": "基礎租賃合同錄入與到期預警", en: "Basic lease intake & critical date alerts" },
  "landing.plan_free_f4": { "zh-CN": "标准 Excel / CSV 经营数据导入", "zh-HK": "標準 Excel / CSV 經營數據導入", en: "Standard Excel / CSV data upload" },
  "landing.plan_free_btn": { "zh-CN": "立即免费注册体验", "zh-HK": "立即免費註冊體驗", en: "Sign Up Free" },

  "landing.plan_starter_title": { "zh-CN": "专业版 Professional", "zh-HK": "專業版 Professional", en: "Professional" },
  "landing.plan_starter_desc": { "zh-CN": "适合拥有 10-50 家门店的快速成长期连锁零售品牌", "zh-HK": "適合擁有 10-50 家門店的快速成長期連鎖零售品牌", en: "Ideal for growing retail chains with 10-50 stores" },
  "landing.plan_starter_price": { "zh-CN": "按门店规模定制", "zh-HK": "按門店規模定制", en: "Custom by Store Count" },
  "landing.plan_starter_f1": { "zh-CN": "无限次 AI 经营分析与合同条款解析", "zh-HK": "無限次 AI 經營分析與合同條款解析", en: "Unlimited AI Analysis & Contract OCR" },
  "landing.plan_starter_f2": { "zh-CN": "多店经营脉搏与四墙利润瀑布流", "zh-HK": "多店經營脈搏與四牆利潤瀑布流", en: "Multi-store Pulse & 4-Wall Margin Waterfall" },
  "landing.plan_starter_f3": { "zh-CN": "基础租约台账与关键日期到期预警", "zh-HK": "基礎租約台賬與關鍵日期到期預警", en: "Core Lease Ledger & Expiry Alerts" },
  "landing.plan_starter_f4": { "zh-CN": "标准 CSV / Excel 批量数据导入与校验", "zh-HK": "標準 CSV / Excel 批量數據導入與校驗", en: "Standard CSV / Excel Batch Import" },
  "landing.plan_starter_btn": { "zh-CN": "申请专业版试用", "zh-HK": "申請專業版試用", en: "Start Professional Trial" },

  "landing.plan_pro_title": { "zh-CN": "企业版 Enterprise (推荐)", "zh-HK": "企業版 Enterprise (推薦)", en: "Enterprise (Recommended)" },
  "landing.plan_pro_badge": { "zh-CN": "最受欢迎", "zh-HK": "最受歡迎", en: "Most Popular" },
  "landing.plan_pro_desc": { "zh-CN": "适合 50-300 家门店，追求经营精细化与财务合规一体化的成熟零售集团", "zh-HK": "適合 50-300 家門店，追求經營精細化與財務合規一體化的成熟零售集團", en: "For 50-300 stores seeking unified operations & audit-grade compliance" },
  "landing.plan_pro_price": { "zh-CN": "年度订阅制", "zh-HK": "年度訂閱制", en: "Annual Subscription" },
  "landing.plan_pro_f1": { "zh-CN": "包含专业版全部功能", "zh-HK": "包含專業版全部功能", en: "Everything in Professional" },
  "landing.plan_pro_f2": { "zh-CN": "租约情景推演工作台 (降租/续签/闭店 净现值模拟)", "zh-HK": "租約情景推演工作台 (降租/續簽/閉店 淨現值模擬)", en: "Scenario Workbench (Lease NPV simulations)" },
  "landing.plan_pro_f3": { "zh-CN": "审计级 IFRS 16 会计引擎与月结凭证自动化", "zh-HK": "審計級 IFRS 16 會計引擎與月結憑證自動化", en: "Audit-Grade IFRS 16 Engine & Close Automation" },
  "landing.plan_pro_f4": { "zh-CN": "AI 经营分析 Copilot (智能归因与合同 OCR 解析)", "zh-HK": "AI 經營分析 Copilot (智能歸因與合同 OCR 解析)", en: "AI Operating Copilot (OCR & Root Cause Analysis)" },
  "landing.plan_pro_f5": { "zh-CN": "行动闭环追踪与预期收益复盘", "zh-HK": "行動閉環追蹤與預期收益復盤", en: "Action Proposal Tracking & Realized Profit Review" },
  "landing.plan_pro_btn": { "zh-CN": "预约企业版方案演示", "zh-HK": "預約企業版方案演示", en: "Book Enterprise Demo" },

  "landing.plan_ent_title": { "zh-CN": "旗舰定制版 Ultimate Group", "zh-HK": "旗艦定制版 Ultimate Group", en: "Ultimate Group" },
  "landing.plan_ent_desc": { "zh-CN": "适合 300+ 门店的大型跨国零售集团、多法人多品牌运营商", "zh-HK": "適合 300+ 門店的大型跨國零售集團、多法人多品牌運營商", en: "For 300+ stores, multinational retail groups and multi-brand operators" },
  "landing.plan_ent_price": { "zh-CN": "专属专家定制", "zh-HK": "專屬專家定制", en: "Tailored Architecture" },
  "landing.plan_ent_f1": { "zh-CN": "包含企业版全部功能", "zh-HK": "包含企業版全部功能", en: "Everything in Enterprise" },
  "landing.plan_ent_f2": { "zh-CN": "支持本地私有化或专属云部署 (On-Premises / VPC)", "zh-HK": "支持本地私有化或專屬雲部署 (On-Premises / VPC)", en: "On-Premises or Dedicated VPC Deployment" },
  "landing.plan_ent_f3": { "zh-CN": "深度对接 SAP / Oracle / 用友 / 金蝶 ERP 与 POS 管道", "zh-HK": "深度對接 SAP / Oracle / 用友 / 金蝶 ERP 與 POS 管道", en: "Custom SAP/Oracle/ERP & POS Live Pipelines" },
  "landing.plan_ent_f4": { "zh-CN": "专属零售 FP&A 经营模型专家顾问团队支持", "zh-HK": "專屬零售 FP&A 經營模型專家顧問團隊支持", en: "Dedicated FP&A Retail Economics Advisory" },
  "landing.plan_ent_f5": { "zh-CN": "99.95% 可用性 SLA 响应与专属技术保障", "zh-HK": "99.95% 可用性 SLA 響應與專屬技術保障", en: "99.95% SLA & 24/7 Dedicated Support" },
  "landing.plan_ent_btn": { "zh-CN": "联系大客户顾问", "zh-HK": "聯繫大客戶顧問", en: "Contact Enterprise Advisor" },

  // FAQ
  "landing.faq_badge": { "zh-CN": "疑虑解答", "zh-HK": "疑慮解答", en: "FAQ" },
  "landing.faq_title": { "zh-CN": "常见问题解答", "zh-HK": "常見問題解答", en: "Frequently Asked Questions" },
  "landing.faq_subtitle": { "zh-CN": "了解如何快速在您的连锁体系中接入并发挥经营决策工作站的价值", "zh-HK": "了解如何快速在您的連鎖體系中接入並發揮經營決策工作站的價值", en: "Learn how to deploy and capture value across your retail chain." },

  "landing.faq_q0": { "zh-CN": "个人客户可以使用吗？每人每天是否有免费额度？", "zh-HK": "個人客戶可以使用嗎？每人每天是否有免費額度？", en: "Can individual users use this? Is there a free daily quota?" },
  "landing.faq_a0": {
    "zh-CN": "完全可以！我们现已全面开放个人用户自主注册体验，无需绑定信用卡。每位用户每天均可获得 5 次免费的 AI 智能经营分析与合同条款 OCR 提取额度，随时自助体验单店四墙利润诊断与租约管理。",
    "zh-HK": "完全可以！我們現已全面開放個人用戶自主註冊體驗，無需綁定信用卡。每位用戶每天均可獲得 5 次免費的 AI 智能經營分析與合同條款 OCR 提取額度，隨時自助體驗單店四牆利潤診斷與租約管理。",
    en: "Absolutely! We now offer open self-service registration for individual users with no credit card required. Every user receives 5 free AI requests every day for store diagnostics and contract extraction.",
  },

  "landing.faq_q1": { "zh-CN": "工作站如何与我们现有的 POS、ERP 和 HR 排班系统打通？", "zh-HK": "工作站如何與我們現有的 POS、ERP 和 HR 排班系統打通？", en: "How does the workstation integrate with our existing POS, ERP, and scheduling tools?" },
  "landing.faq_a1": {
    "zh-CN": "工作站提供灵活的双模集成方案：既支持开箱即用的标准 Excel / CSV 模板批量导入与智能校验，也提供高吞吐 REST API 管道直接与主流 POS（如海信、科脉等）、ERP（SAP、Oracle、用友、金蝶等）及 HR 排班系统完成自动化日级数据同步。",
    "zh-HK": "工作站提供靈活的雙模集成方案：既支持開箱即用的標準 Excel / CSV 模板批量導入與智能校驗，也提供高吞吐 REST API 管道直接與主流 POS（如海信、科脈等）、ERP（SAP、Oracle、用友、金蝶等）及 HR 排班系統完成自動化日級數據同步。",
    en: "We offer flexible dual-mode integration: out-of-the-box standard Excel/CSV templates with automated validation, plus robust REST APIs connecting directly to POS, ERP (SAP, Oracle, Kingdee, Yonyou) and HR systems.",
  },

  "landing.faq_q2": { "zh-CN": "IFRS 16 租赁计量引擎是否能够通过四大（Big 4）会计师事务所的审计审查？", "zh-HK": "IFRS 16 租賃計量引擎是否能夠通過四大（Big 4）會計師事務所的審計審查？", en: "Can the IFRS 16 lease measurement engine pass Big 4 audit scrutiny?" },
  "landing.faq_a2": {
    "zh-CN": "是的。我们的会计引擎严格遵循国际财务报告准则第 16 号（IFRS 16 / 企业会计准则第 21 号）规范，经过 22 组基准用例与 148 项核心断言的严格回归测试。系统提供完整的利息摊销表、使用权资产折旧表、变动调整留痕以及可一键导出的审计穿透底稿，历史计算可完整复演。",
    "zh-HK": "是的。我們的會計引擎嚴格遵循國際財務報告準則第 16 號（IFRS 16 / 企業會計準則第 21 號）規範，經過 22 組基準用例與 148 項核心斷言的嚴格回歸測試。系統提供完整的利息攤銷表、使用權資產折舊表、變動調整留痕以及可一鍵導出的審計穿透底稿，歷史計算可完整復演。",
    en: "Yes. Our accounting engine strictly adheres to IFRS 16 standards with 22 benchmark cases and 148 regression assertions. It provides transparent amortization schedules, ROU rollforwards, and exportable audit trail workbooks.",
  },

  "landing.faq_q3": { "zh-CN": "上线实施通常需要多长时间？我们需要投入多少IT资源？", "zh-HK": "上線實施通常需要多長時間？我們需要投入多少IT資源？", en: "What is the typical deployment timeline and IT effort required?" },
  "landing.faq_a3": {
    "zh-CN": "标准版采用轻量化 SaaS 模式，通常只需 3–5 个工作日完成历史合同数据清洗导入与门店映射配置即可开始使用；对于需要定制深度 ERP 接口的大型集团，实施周期通常在 2–4 周之间。我们提供全程专属数据顾问与培训支持。",
    "zh-HK": "標準版採用輕量化 SaaS 模式，通常只需 3–5 個工作日完成歷史合同數據清洗導入與門店映射配置即可開始使用；對於需要定制深度 ERP 接口的大型集團，實施週期通常在 2–4 週之間。我們提供全程專屬數據顧問與培訓支持。",
    en: "Standard deployment takes 3-5 business days using our guided intake pipelines. Enterprise custom ERP integrations typically take 2-4 weeks with dedicated FP&A advisory and training support.",
  },

  "landing.faq_q4": { "zh-CN": "我们的经营数据和租赁合同隐私安全如何保障？", "zh-HK": "我們的經營數據和租賃合同隱私安全如何保障？", en: "How is our sensitive operating data and lease privacy protected?" },
  "landing.faq_a4": {
    "zh-CN": "系统采用金融级安全标准，传输与静态存储全程 256 位加密。跨法人、跨品牌硬性隔离，行级数据权限精细控制；同时支持企业本地私有化部署或 VPC 专属部署，核心数据完全留在企业自身网络防火墙内。",
    "zh-HK": "系統採用金融級安全標準，傳輸與靜態存儲全程 256 位加密。跨法人、跨品牌硬性隔離，行級數據權限精細控制；同時支持企業本地私有化部署或 VPC 專屬部署，核心數據完全留在企業自身網絡防火牆內。",
    en: "We implement banking-grade 256-bit encryption in transit and at rest. Strict row-level multi-tenant isolation ensures zero cross-entity data leakage. Private on-premises and VPC deployments are fully supported.",
  },

  // Lead Capture Modal
  "landing.modal_title": { "zh-CN": "预约专家演示 & 获取专属 ROI 方案", "zh-HK": "預約專家演示 & 獲取專屬 ROI 方案", en: "Book Expert Demo & Receive Custom ROI Assessment" },
  "landing.modal_subtitle": { "zh-CN": "填写以下信息，我们的零售 FP&A 顾问将在 2 小时内与您联系，安排产品深度演示与数据试算", "zh-HK": "填寫以下資訊，我們的零售 FP&A 顧問將在 2 小時內與您聯繫，安排產品深度演示與數據試算", en: "Fill out the form below. Our retail FP&A advisor will reach out within 2 hours to arrange a live demo." },
  "landing.modal_name": { "zh-CN": "您的姓名", "zh-HK": "您的姓名", en: "Your Name" },
  "landing.modal_company": { "zh-CN": "企业 / 品牌名称", "zh-HK": "企業 / 品牌名稱", en: "Company / Brand Name" },
  "landing.modal_phone": { "zh-CN": "联系电话 / 手机号", "zh-HK": "聯繫電話 / 手機號", en: "Phone Number" },
  "landing.modal_email": { "zh-CN": "工作邮箱", "zh-HK": "工作郵箱", en: "Business Email" },
  "landing.modal_stores": { "zh-CN": "门店规模", "zh-HK": "門店規模", en: "Store Scale" },
  "landing.modal_stores_opt1": { "zh-CN": "10 - 50 家门店", "zh-HK": "10 - 50 間門店", en: "10 - 50 Stores" },
  "landing.modal_stores_opt2": { "zh-CN": "50 - 200 家门店", "zh-HK": "50 - 200 間門店", en: "50 - 200 Stores" },
  "landing.modal_stores_opt3": { "zh-CN": "200+ 家门店 (集团级)", "zh-HK": "200+ 間門店 (集團級)", en: "200+ Stores (Group Level)" },
  "landing.modal_interest": { "zh-CN": "最关注的业务模块", "zh-HK": "最關注的業務模組", en: "Primary Interest" },
  "landing.modal_interest_pulse": { "zh-CN": "经营脉搏与每日掉队归因", "zh-HK": "經營脈搏與每日掉隊歸因", en: "Operating Pulse & Anomaly Triage" },
  "landing.modal_interest_scenario": { "zh-CN": "租约情景推演与降租谈判", "zh-HK": "租約情景推演與降租談判", en: "Scenario Workbench & Lease Negotiation" },
  "landing.modal_interest_ifrs": { "zh-CN": "IFRS 16 租赁会计与关账", "zh-HK": "IFRS 16 租賃會計與關賬", en: "IFRS 16 Lease Accounting & Close" },
  "landing.modal_submit": { "zh-CN": "立即提交预约", "zh-HK": "立即提交預約", en: "Submit Demo Request" },
  "landing.modal_submitting": { "zh-CN": "正在提交…", "zh-HK": "正在提交…", en: "Submitting…" },
  "landing.modal_success": { "zh-CN": "提交成功！我们的零售经营顾问将尽快与您取得联系。", "zh-HK": "提交成功！我們的零售經營顧問將儘快與您取得聯繫。", en: "Success! Our retail performance advisor will contact you shortly." },

  // Bottom CTA
  "landing.cta_title": { "zh-CN": "准备好让每家门店的经营异常转化为确定性收益了吗？", "zh-HK": "準備好讓每家門店的經營異常轉化為確定性收益了嗎？", en: "Ready to Turn Every Store Anomaly into Verifiable Profit?" },
  "landing.cta_desc": { "zh-CN": "立即预约专家 1 对 1 演示，体验从单店按日（Store-Day）四墙利润穿透到 IFRS 16 自动化合规的完整闭环。", "zh-HK": "立即預約專家 1 對 1 演示，體驗從單店按日（Store-Day）四牆利潤穿透到 IFRS 16 自動化合規的完整閉環。", en: "Book a 1-on-1 demo today and experience the complete loop from store-day four-wall margins to IFRS 16 compliance." },
  "landing.cta_btn": { "zh-CN": "免费预约产品演示", "zh-HK": "免費預約產品演示", en: "Book Free Product Demo" },

  // Footer
  "landing.footer_desc": { "zh-CN": "面向连锁零售承租方的经营决策与租赁合规工作站，连接销售、毛利、排班、占用成本与 IFRS 16 准则计量。", "zh-HK": "面向連鎖零售承租方的經營決策與租賃合規工作站，連接銷售、毛利、排班、佔用成本與 IFRS 16 準則計量。", en: "Retail performance workstation for chain retail tenants, bridging sales, gross margin, labor, occupancy costs and IFRS 16 compliance." },
  "landing.footer_rights": { "zh-CN": "保留所有权利。", "zh-HK": "保留所有權利。", en: "All rights reserved." },
  "landing.footer_compliance_tag": { "zh-CN": "IFRS 16 / 企业会计准则第 21 号规范对齐", "zh-HK": "IFRS 16 / 企業會計準則第 21 號規範對齊", en: "Aligned with IFRS 16 / ASC 842 Standards" },
  // FIX-023: ROI 测算页接入 i18n（此前零接线，22 处硬编码中文）
  "roi.title": { "zh-CN": "ROI 测算", "zh-HK": "ROI 測算", en: "ROI Calculator" },
  "roi.header_count": { "zh-CN": "当前假设 · {count} 份合同", "zh-HK": "當前假設 · {count} 份合同", en: "Current assumptions · {count} contracts" },
  "roi.card_assumptions": { "zh-CN": "测算参数", "zh-HK": "測算參數", en: "Assumptions" },
  "roi.assumption_contracts": { "zh-CN": "合同数量", "zh-HK": "合同數量", en: "Contracts" },
  "roi.assumption_intake": { "zh-CN": "单份录入节省", "zh-HK": "單份錄入節省", en: "Intake hours saved" },
  "roi.assumption_close": { "zh-CN": "月结节省", "zh-HK": "月結節省", en: "Close saved" },
  "roi.assumption_audit": { "zh-CN": "审计返工减少", "zh-HK": "審計返工減少", en: "Audit rework saved" },
  "roi.note_contracts": { "zh-CN": "门店/设备租赁合同总量", "zh-HK": "門店/設備租賃合同總量", en: "Total store and equipment lease contracts" },
  "roi.note_intake": { "zh-CN": "传统 Excel/表单录入 vs AI 草稿确认", "zh-HK": "傳統 Excel/表單錄入 vs AI 草稿確認", en: "Manual Excel/form entry vs AI draft confirmation" },
  "roi.note_close": { "zh-CN": "分录生成、复核、锁账、报表导出", "zh-HK": "分錄生成、複核、鎖賬、報表導出", en: "Entry generation, review, lock, export" },
  "roi.note_audit": { "zh-CN": "对数报告、审批留痕、范围判定减少返工", "zh-HK": "對數報告、審批留痕、範圍判定減少返工", en: "Reconciliation reports, approval trails, scope decisions" },
  "roi.unit_hours": { "zh-CN": "小时", "zh-HK": "小時", en: "hours" },
  "roi.unit_hours_per_month": { "zh-CN": "人天/月", "zh-HK": "人天/月", en: "person-days/month" },
  "roi.unit_hours_per_year": { "zh-CN": "小时/年", "zh-HK": "小時/年", en: "hours/year" },
  "roi.label_currency": { "zh-CN": "计价币种", "zh-HK": "計價幣種", en: "Pricing currency" },
  "roi.label_hourly_cost": { "zh-CN": "财务人员小时成本", "zh-HK": "財務人員小時成本", en: "Finance hourly cost" },
  "roi.label_manual_hours": { "zh-CN": "传统单份录入小时", "zh-HK": "傳統單份錄入小時", en: "Manual hours per entry" },
  "roi.label_ai_hours": { "zh-CN": "AI 草稿确认小时", "zh-HK": "AI 草稿確認小時", en: "AI draft hours per entry" },
  "roi.label_close_days": { "zh-CN": "传统月结人天/月", "zh-HK": "傳統月結人天/月", en: "Manual close person-days/month" },
  "roi.label_system_close_days": { "zh-CN": "系统月结人天/月", "zh-HK": "系統月結人天/月", en: "System close person-days/month" },
  "roi.label_audit_hours": { "zh-CN": "年度审计返工减少小时", "zh-HK": "年度審計返工減少小時", en: "Annual audit rework saved hours" },
  "roi.stat_hours_saved": { "zh-CN": "年度节省工时", "zh-HK": "年度節省工時", en: "Hours saved per year" },
  "roi.stat_labor_savings": { "zh-CN": "年度人力成本节省", "zh-HK": "年度人力成本節省", en: "Labor cost saved per year" },
  "roi.stat_ai_saved": { "zh-CN": "AI 录入节省", "zh-HK": "AI 錄入節省", en: "AI entry saved" },
  "roi.stat_audit_reduced": { "zh-CN": "审计返工减少", "zh-HK": "審計返工減少", en: "Audit rework reduced" },
  "roi.card_basis": { "zh-CN": "测算口径", "zh-HK": "測算口徑", en: "Calculation basis" },
  "roi.col_item": { "zh-CN": "项目", "zh-HK": "項目", en: "Item" },
  "roi.col_value": { "zh-CN": "数值", "zh-HK": "數值", en: "Value" },
  "roi.col_note": { "zh-CN": "说明", "zh-HK": "說明", en: "Note" },

  // FIX-023: 签约前决策页接入 i18n（49 处硬编码中文）
  "pre_deal.title": { "zh-CN": "签约前决策", "zh-HK": "簽約前決策", en: "Pre-signing decision" },
  "pre_deal.header_count": { "zh-CN": "{currency} · {count} 个年度期间", "zh-HK": "{currency} · {count} 個年度期間", en: "{currency} · {count} annual periods" },
  "pre_deal.rate_source": { "zh-CN": "集团 IBR · 5 年期 · 2026-07 版", "zh-HK": "集團 IBR · 5 年期 · 2026-07 版", en: "Group IBR · 5-year · 2026-07" },
  "pre_deal.rate_overridden": { "zh-CN": "已覆盖默认值", "zh-HK": "已覆蓋默認值", en: "Overriding the default" },
  "pre_deal.rate_default": { "zh-CN": "当前使用默认值", "zh-HK": "當前使用默認值", en: "Using the default" },
  "pre_deal.rate_note": { "zh-CN": "仅用于本次情景测算", "zh-HK": "僅用於本次情景測算", en: "Used for this scenario only" },
  "pre_deal.card_terms": { "zh-CN": "条款草案", "zh-HK": "條款草案", en: "Term draft" },
  "pre_deal.label_name": { "zh-CN": "方案名称", "zh-HK": "方案名稱", en: "Plan name" },
  "pre_deal.err_name": { "zh-CN": "请填写名称", "zh-HK": "請填寫名稱", en: "Name is required" },
  "pre_deal.label_start": { "zh-CN": "起租日", "zh-HK": "起租日", en: "Commencement" },
  "pre_deal.err_start": { "zh-CN": "请选择起租日", "zh-HK": "請選擇起租日", en: "Commencement date is required" },
  "pre_deal.label_term": { "zh-CN": "租期（月）", "zh-HK": "租期（月）", en: "Term (months)" },
  "pre_deal.err_term": { "zh-CN": "请填写租期", "zh-HK": "請填寫租期", en: "Term is required" },
  "pre_deal.label_rent": { "zh-CN": "月租金", "zh-HK": "月租金", en: "Monthly rent" },
  "pre_deal.err_rent": { "zh-CN": "请填写月租金", "zh-HK": "請填寫月租金", en: "Monthly rent is required" },
  "pre_deal.label_rate": { "zh-CN": "折现率", "zh-HK": "折現率", en: "Discount rate" },
  "pre_deal.err_rate": { "zh-CN": "请填写折现率", "zh-HK": "請填寫折現率", en: "Discount rate is required" },
  "pre_deal.label_currency": { "zh-CN": "币种", "zh-HK": "幣種", en: "Currency" },
  "pre_deal.err_currency": { "zh-CN": "请填写币种", "zh-HK": "請填寫幣種", en: "Currency is required" },
  "pre_deal.label_free": { "zh-CN": "免租期（月）", "zh-HK": "免租期（月）", en: "Rent-free (months)" },
  "pre_deal.label_escalation": { "zh-CN": "年递增（%）", "zh-HK": "年遞增（%）", en: "Annual escalation (%)" },
  "pre_deal.label_direct_cost": { "zh-CN": "初始直接费用", "zh-HK": "初始直接費用", en: "Initial direct cost" },
  "pre_deal.hint_direct_cost": { "zh-CN": "计入资产，不计入负债", "zh-HK": "計入資產，不計入負債", en: "Capitalized to the asset, not the liability" },
  "pre_deal.label_exit_penalty": { "zh-CN": "提前退出罚金（月租金）", "zh-HK": "提前退出罰金（月租金）", en: "Early-exit penalty (months of rent)" },
  "pre_deal.hint_exit_penalty": { "zh-CN": "按退出时点在租的租金计", "zh-HK": "按退出時點在租的租金計", en: "Based on rent in force at exit" },
  "pre_deal.btn_brief": { "zh-CN": "生成决策简报", "zh-HK": "生成決策簡報", en: "Build decision briefing" },
  "pre_deal.err_failed": { "zh-CN": "测算失败", "zh-HK": "測算失敗", en: "Calculation failed" },
  "pre_deal.alert_title": { "zh-CN": "决策简报", "zh-HK": "決策簡報", en: "Decision briefing" },
  "pre_deal.stat_liability": { "zh-CN": "入表负债", "zh-HK": "入表負債", en: "Balance-sheet liability" },
  "pre_deal.stat_rou": { "zh-CN": "使用权资产", "zh-HK": "使用權資產", en: "Right-of-use asset" },
  "pre_deal.stat_commitment": { "zh-CN": "全期承诺（未折现）", "zh-HK": "全期承諾（未折現）", en: "Full-term commitment (undiscounted)" },
  "pre_deal.stat_discount_effect": { "zh-CN": "折现影响", "zh-HK": "折現影響", en: "Discounting effect" },
  "pre_deal.card_expense_curve": { "zh-CN": "IFRS 16 费用曲线 vs 直线租金", "zh-HK": "IFRS 16 費用曲線 vs 直線租金", en: "IFRS 16 expense vs straight-line rent" },
  "pre_deal.expense_curve_note": { "zh-CN": "利息在负债最大时最高，因此会计费用前高后低。两条线交叉之前的年份，实际入账费用高于按租金做的预算——本方案前 {years} 年如此，之后反向，全期相抵。", "zh-HK": "利息在負債最大時最高，因此會計費用前高後低。兩條線交叉之前的年份，實際入賬費用高於按租金做的預算——本方案前 {years} 年如此，之後反向，全期相抵。", en: "Interest peaks when the liability peaks, so accounting expense is front-loaded. Until the two lines cross, booked expense runs above the rent-based budget — {years} years for this plan, then the reverse, netting out over the term." },
  "pre_deal.year_suffix": { "zh-CN": "第{value}年", "zh-HK": "第{value}年", en: "Year {value}" },
  "pre_deal.axis_wan": { "zh-CN": "{value}万", "zh-HK": "{value}萬", en: "{value}0k" },
  "pre_deal.series_ifrs16": { "zh-CN": "IFRS 16 费用", "zh-HK": "IFRS 16 費用", en: "IFRS 16 expense" },
  "pre_deal.series_straight": { "zh-CN": "直线租金", "zh-HK": "直線租金", en: "Straight-line rent" },
  "pre_deal.series_cash": { "zh-CN": "现金租金", "zh-HK": "現金租金", en: "Cash rent" },
  "pre_deal.card_ebitda": { "zh-CN": "EBITDA 三层影响", "zh-HK": "EBITDA 三層影響", en: "EBITDA three-layer impact" },
  "pre_deal.ebitda_note": { "zh-CN": "租金从 EBITDA 线上移到线下，EBITDA 被动抬升——业务没有任何改善。抬升额等于原本计入经营费用的租金，它去了折旧（EBITDA 与 EBIT 之间）和利息（EBIT 与净利润之间）。", "zh-HK": "租金從 EBITDA 線上移到線下，EBITDA 被動抬升——業務沒有任何改善。抬升額等於原本計入經營費用的租金，它去了折舊（EBITDA 與 EBIT 之間）和利息（EBIT 與淨利潤之間）。", en: "Rent moves below the EBITDA line, passively lifting EBITDA — the business is unchanged. The uplift equals the rent that used to sit in operating expense; it lands in depreciation (between EBITDA and EBIT) and interest (between EBIT and net profit)." },
  "pre_deal.series_ebitda_uplift": { "zh-CN": "EBITDA 抬升", "zh-HK": "EBITDA 抬升", en: "EBITDA uplift" },
  "pre_deal.series_depreciation": { "zh-CN": "折旧（线下）", "zh-HK": "折舊（線下）", en: "Depreciation (below the line)" },
  "pre_deal.series_interest": { "zh-CN": "利息（EBIT 之下）", "zh-HK": "利息（EBIT 之下）", en: "Interest (below EBIT)" },
  "pre_deal.series_net_profit": { "zh-CN": "净利润影响", "zh-HK": "淨利潤影響", en: "Net profit impact" },
  "pre_deal.card_exit": { "zh-CN": "退出成本曲线", "zh-HK": "退出成本曲線", en: "Exit cost curve" },
  "pre_deal.exit_note": { "zh-CN": "策略变化时才会问、却没人备着答案的问题：第 N 年退出要花多少。剩余租金因退出而免付，故不计入「退出现金支出」；罚金按退出时点在租的租金计算。", "zh-HK": "策略變化時才會問、卻沒人備著答案的問題：第 N 年退出要花多少。剩餘租金因退出而免付，故不計入「退出現金支出」；罰金按退出時點在租的租金計算。", en: "The question nobody keeps an answer for until strategy changes: what does exiting in year N cost. Remaining rent is waived on exit, so it stays out of cash-to-exit; the penalty follows the rent in force at the exit point." },
  "pre_deal.col_exit_point": { "zh-CN": "退出时点", "zh-HK": "退出時點", en: "Exit point" },
  "pre_deal.col_remaining": { "zh-CN": "剩余承诺（免付）", "zh-HK": "剩餘承諾（免付）", en: "Remaining commitment (waived)" },
  "pre_deal.col_released": { "zh-CN": "解除负债", "zh-HK": "解除負債", en: "Liability released" },
  "pre_deal.col_rou": { "zh-CN": "核销使用权资产", "zh-HK": "核銷使用權資產", en: "ROU written off" },
  "pre_deal.col_penalty": { "zh-CN": "罚金", "zh-HK": "罰金", en: "Penalty" },
  "pre_deal.col_pnl": { "zh-CN": "损益影响", "zh-HK": "損益影響", en: "P&L impact" },
  "pre_deal.col_cash_out": { "zh-CN": "退出现金支出", "zh-HK": "退出現金支出", en: "Cash to exit" },
  "pre_deal.exit_year": { "zh-CN": "第 {value} 年末", "zh-HK": "第 {value} 年末", en: "End of year {value}" },

  // FIX-023: 条款比价页接入 i18n（39 处硬编码中文）
  "deal_compare.title": { "zh-CN": "条款比价", "zh-HK": "條款比價", en: "Term comparison" },
  "deal_compare.header_count": { "zh-CN": "已比较 {count} 个方案", "zh-HK": "已比較 {count} 個方案", en: "Compared {count} plans" },
  "deal_compare.plan_a": { "zh-CN": "方案 A", "zh-HK": "方案 A", en: "Plan A" },
  "deal_compare.plan_b": { "zh-CN": "方案 B", "zh-HK": "方案 B", en: "Plan B" },
  "deal_compare.label_rate": { "zh-CN": "折现率（年化，小数）", "zh-HK": "折現率（年化，小數）", en: "Discount rate (annual, decimal)" },
  "deal_compare.err_rate": { "zh-CN": "请填写折现率", "zh-HK": "請填寫折現率", en: "Discount rate is required" },
  "deal_compare.hint_rate": { "zh-CN": "排序结果取决于它，系统不会替你假设一个", "zh-HK": "排序結果取決於它，系統不會替你假設一個", en: "Ranking depends on it; the system never assumes one" },
  "deal_compare.label_currency": { "zh-CN": "币种", "zh-HK": "幣種", en: "Currency" },
  "deal_compare.err_currency": { "zh-CN": "请填写币种", "zh-HK": "請填寫幣種", en: "Currency is required" },
  "deal_compare.err_name": { "zh-CN": "请填写方案名称", "zh-HK": "請填寫方案名稱", en: "Plan name is required" },
  "deal_compare.label_term": { "zh-CN": "租期（月）", "zh-HK": "租期（月）", en: "Term (months)" },
  "deal_compare.err_term": { "zh-CN": "请填写租期", "zh-HK": "請填寫租期", en: "Term is required" },
  "deal_compare.label_rent": { "zh-CN": "月租金", "zh-HK": "月租金", en: "Monthly rent" },
  "deal_compare.err_rent": { "zh-CN": "请填写月租金", "zh-HK": "請填寫月租金", en: "Monthly rent is required" },
  "deal_compare.label_free": { "zh-CN": "免租期（月）", "zh-HK": "免租期（月）", en: "Rent-free (months)" },
  "deal_compare.label_esc": { "zh-CN": "年递增（%）", "zh-HK": "年遞增（%）", en: "Annual escalation (%)" },
  "deal_compare.label_other": { "zh-CN": "月度其他成本", "zh-HK": "月度其他成本", en: "Other monthly cost" },
  "deal_compare.hint_other": { "zh-CN": "物业费等，不随调租变动", "zh-HK": "物業費等，不隨調租變動", en: "Property fees etc., fixed across the term" },
  "deal_compare.label_area": { "zh-CN": "面积（㎡）", "zh-HK": "面積（㎡）", en: "Area (sqm)" },
  "deal_compare.hint_area": { "zh-CN": "留空则不出每平米单价", "zh-HK": "留空則不出每平米單價", en: "Leave blank to skip per-sqm pricing" },
  "deal_compare.label_upfront": { "zh-CN": "前期投入", "zh-HK": "前期投入", en: "Upfront cost" },
  "deal_compare.label_contrib": { "zh-CN": "出租方装修补贴", "zh-HK": "出租方裝修補貼", en: "Landlord fit-out contribution" },
  "deal_compare.add_plan": { "zh-CN": "添加方案", "zh-HK": "添加方案", en: "Add plan" },
  "deal_compare.plan_name": { "zh-CN": "方案 {letter}", "zh-HK": "方案 {letter}", en: "Plan {letter}" },
  "deal_compare.btn_compare": { "zh-CN": "比价", "zh-HK": "比價", en: "Compare" },
  "deal_compare.err_failed": { "zh-CN": "比价失败", "zh-HK": "比價失敗", en: "Comparison failed" },
  "deal_compare.agree": { "zh-CN": "两个口径结论一致", "zh-HK": "兩個口徑結論一致", en: "Both measures agree" },
  "deal_compare.disagree": { "zh-CN": "两个口径结论不一致", "zh-HK": "兩個口徑結論不一致", en: "The two measures disagree" },
  "deal_compare.card_result": { "zh-CN": "条款比价结果", "zh-HK": "條款比價結果", en: "Comparison results" },
  "deal_compare.col_plan": { "zh-CN": "方案", "zh-HK": "方案", en: "Plan" },
  "deal_compare.badge_pv": { "zh-CN": "现值最优", "zh-HK": "現值最優", en: "Best by PV" },
  "deal_compare.badge_rent": { "zh-CN": "有效租金最优", "zh-HK": "有效租金最優", en: "Best by effective rent" },
  "deal_compare.col_eff_rent": { "zh-CN": "有效租金（月）", "zh-HK": "有效租金（月）", en: "Effective rent (monthly)" },
  "deal_compare.col_eff_sqm": { "zh-CN": "每平米有效单价", "zh-HK": "每平米有效單價", en: "Effective rate per sqm" },
  "deal_compare.col_first_year": { "zh-CN": "首年租金", "zh-HK": "首年租金", en: "First-year rent" },
  "deal_compare.col_total_rent": { "zh-CN": "全期租金", "zh-HK": "全期租金", en: "Total rent" },
  "deal_compare.col_total_cost": { "zh-CN": "全期总成本", "zh-HK": "全期總成本", en: "Total cost" },
  "deal_compare.col_pv": { "zh-CN": "现值", "zh-HK": "現值", en: "Present value" },
  "deal_compare.card_cash": { "zh-CN": "累计现金支出", "zh-HK": "累計現金支出", en: "Cumulative cash outflow" },
  "deal_compare.cash_note": { "zh-CN": "免租期是一段平线，年递增是一段逐渐变陡的曲线——两条线交叉的位置，就是两个方案成本反超的时点。", "zh-HK": "免租期是一段平線，年遞增是一段逐漸變陡的曲線——兩條線交叉的位置，就是兩個方案成本反超的時點。", en: "A rent-free period is a flat run, an escalation a steepening curve — where the lines cross is when one plan's cumulative cost overtakes the other." },
  "deal_compare.month_suffix": { "zh-CN": "{value}月", "zh-HK": "{value}月", en: "Month {value}" },
  "deal_compare.axis_wan": { "zh-CN": "{value}万", "zh-HK": "{value}萬", en: "{value}0k" },
};

export function t(key: string, lang: Language, replacements?: Record<string, string>): string {
  const entry = dict[key];
  if (!entry) {
    if (process.env.NODE_ENV !== "production") {
      console.error(`[i18n] Missing translation key: ${key}`);
    }
    return "";
  }
  let text = entry[lang] || entry["zh-CN"];
  if (replacements) {
    Object.entries(replacements).forEach(([k, v]) => {
      text = text.replace(`{${k}}`, v);
    });
  }
  return text;
}
