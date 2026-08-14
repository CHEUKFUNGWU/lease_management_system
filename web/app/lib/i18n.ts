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
  "retail.kpi.store_contribution": {
    "zh-CN": "门店贡献",
    "zh-HK": "門店貢獻",
    en: "Store contribution",
  },
  "retail.kpi.average_transaction_value": {
    "zh-CN": "客单价",
    "zh-HK": "客單價",
    en: "Average transaction value",
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
    "zh-CN": "门店贡献率",
    "zh-HK": "門店貢獻率",
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
    "zh-CN": "门店贡献转负",
    "zh-HK": "門店貢獻轉負",
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
  "common.ai_analysis": {
    "zh-CN": "交给 AI 分析",
    "zh-HK": "交給 AI 分析",
    en: "Analyze with AI",
  },
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
  "pulse.subtitle": {
    "zh-CN": "两分钟完成整体表现、数据可信度与优先门店晨检。",
    "zh-HK": "兩分鐘完成整體表現、數據可信度與優先門店晨檢。",
    en: "A two-minute check of overall performance, data trust, and priority stores.",
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
  "store360.subtitle": {
    "zh-CN": "围绕单店的事实、同群对比与变化贡献；仅供 Working 经营分析，不作解释性判断。",
    "zh-HK": "圍繞單店的事實、同群對比與變化貢獻；僅供 Working 經營分析，不作解釋性判斷。",
    en: "Facts, peer comparison and change contribution for one store; Working analysis only, never an interpretive judgment.",
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
  "store360.status.missing": {
    "zh-CN": "缺失",
    "zh-HK": "缺失",
    en: "Missing",
  },
  "store360.status.partial": {
    "zh-CN": "部分",
    "zh-HK": "部分",
    en: "Partial",
  },
  "store360.status.complete": {
    "zh-CN": "完整",
    "zh-HK": "完整",
    en: "Complete",
  },

  // I18N-001 — scenario-workbench page
  "scenario.title": {
    "zh-CN": "情景工作台",
    "zh-HK": "情景工作台",
    en: "Scenario Workbench",
  },
  "scenario.subtitle": {
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
  "scenario.horizon_change": {
    "zh-CN": "预测期贡献变化",
    "zh-HK": "預測期貢獻變化",
    en: "Horizon contribution change",
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
  "ai.approval.adopt_failed": {
    "zh-CN": "采纳失败，请重试",
    "zh-HK": "採納失敗，請重試",
    en: "Adopt failed, retry",
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
    "zh-CN": "情景工作台",
    "zh-HK": "情景工作台",
    en: "Scenario Workbench",
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
  "agent_metrics.title": {
    "zh-CN": "Agent 运营与用量",
    "zh-HK": "Agent 運營與用量",
    en: "Agent Operations & Usage",
  },
  "agent_metrics.description": {
    "zh-CN": "查看当前权限范围内 Planner 跨 Run 的调用、Token 和成本核算状态。数据来自持久化 Trace，不展示业务数据或敏感提示词。",
    "zh-HK": "查看當前權限範圍內 Planner 跨 Run 的調用、Token 和成本核算狀態。數據來自持久化 Trace，不展示業務數據或敏感提示詞。",
    en: "Review Planner calls, tokens, and cost-accounting status across Runs in your authorized scope. The data comes from persisted traces and excludes business data and sensitive prompts.",
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
  "lang.zh_CN": {
    "zh-CN": "简体中文",
    "zh-HK": "簡體中文",
    en: "Simplified Chinese",
  },
  "lang.zh_TW": {
    "zh-CN": "繁体中文",
    "zh-HK": "繁體中文",
    en: "Traditional Chinese",
  },
  "lang.en": {
    "zh-CN": "英文",
    "zh-HK": "英文",
    en: "English",
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
  "ai.send": {
    "zh-CN": "发送",
    "zh-HK": "發送",
    en: "Send",
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
  "ai.file_uploaded": {
    "zh-CN": "已上传文件: ",
    "zh-HK": "已上傳文件: ",
    en: "Uploaded file: ",
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
  "reports.export_excel": {
    "zh-CN": "导出 Excel",
    "zh-HK": "導出 Excel",
    en: "Export Excel",
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
  "reports.contains_unapproved": {
    "zh-CN": "含未审批数据",
    "zh-HK": "含未審批數據",
    en: "Includes Unapproved Data",
  },
  "reports.official_only": {
    "zh-CN": "仅正式数据",
    "zh-HK": "僅正式數據",
    en: "Official Data Only",
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
  "dashboard.title": {
    "zh-CN": "仪表板",
    "zh-HK": "儀表板",
    en: "Dashboard",
  },
  "dashboard.subtitle": {
    "zh-CN": "欢迎回来，这是您今天的租赁管理概览",
    "zh-HK": "歡迎回來，這是您今天的租賃管理概覽",
    en: "Welcome back, here is your lease management overview for today",
  },
  "dashboard.total_contracts": {
    "zh-CN": "合同总数",
    "zh-HK": "合同總數",
    en: "Total Contracts",
  },
  "dashboard.approved": {
    "zh-CN": "已审批",
    "zh-HK": "已審批",
    en: "Approved",
  },
  "dashboard.pending": {
    "zh-CN": "待处理",
    "zh-HK": "待處理",
    en: "Pending",
  },
  "dashboard.draft": {
    "zh-CN": "草稿",
    "zh-HK": "草稿",
    en: "Draft",
  },
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
  "dashboard.quick_actions": {
    "zh-CN": "快捷操作",
    "zh-HK": "快捷操作",
    en: "Quick Actions",
  },
  "dashboard.add_contract": {
    "zh-CN": "新增合同",
    "zh-HK": "新增合同",
    en: "Add Contract",
  },
  "dashboard.add_contract_desc": {
    "zh-CN": "手动录入新租赁合同",
    "zh-HK": "手動錄入新租賃合同",
    en: "Manually enter a new lease contract",
  },
  "dashboard.upload_file": {
    "zh-CN": "在 AI Chat 上传文件",
    "zh-HK": "在 AI Chat 上傳文件",
    en: "Upload Files in AI Chat",
  },
  "dashboard.upload_file_desc": {
    "zh-CN": "统一通过 AI Chat 上传合同、台账和租金表",
    "zh-HK": "統一通過 AI Chat 上傳合同、台賬和租金表",
    en: "Upload contracts, ledgers, and rent schedules only through AI Chat",
  },
  "dashboard.view_report": {
    "zh-CN": "查看报表",
    "zh-HK": "查看報表",
    en: "View Reports",
  },
  "dashboard.view_report_desc": {
    "zh-CN": "负债滚动表与合同汇总",
    "zh-HK": "負債滾動表與合同匯總",
    en: "Liability roll-forward and contract summary",
  },
  "dashboard.ai_assistant": {
    "zh-CN": "AI 助手",
    "zh-HK": "AI 助手",
    en: "AI Assistant",
  },
  "dashboard.ai_assistant_desc": {
    "zh-CN": "智能问答与数据分析",
    "zh-HK": "智能問答與數據分析",
    en: "Intelligent Q&A and data analysis",
  },
  "dashboard.loading": {
    "zh-CN": "加载中...",
    "zh-HK": "加載中...",
    en: "Loading...",
  },
  "dashboard.month_short": {
    "zh-CN": "月",
    "zh-HK": "月",
    en: "",
  },
  "dashboard.ten_thousand": {
    "zh-CN": "万",
    "zh-HK": "萬",
    en: "k",
  },
  "dashboard.lease_liability": {
    "zh-CN": "租赁负债",
    "zh-HK": "租賃負債",
    en: "Lease Liability",
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
    "zh-CN": "共 {count} 份合同，管理您的 IFRS 16 租赁合约",
    "zh-HK": "共 {count} 份合同，管理您的 IFRS 16 租賃合約",
    en: "{count} contracts, managing your IFRS 16 lease contracts",
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
  "contracts.col_number": {
    "zh-CN": "合同编号",
    "zh-HK": "合同編號",
    en: "Contract Number",
  },
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
  "contract.ai_parse": {
    "zh-CN": "AI 识别租金表",
    "zh-HK": "AI 識別租金表",
    en: "AI Parse Rent Schedule",
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
  "contract.confirmed": {
    "zh-CN": "已确认",
    "zh-HK": "已確認",
    en: "Confirmed",
  },
  "contract.skip": {
    "zh-CN": "跳过",
    "zh-HK": "跳過",
    en: "Skip",
  },
  "contract.restore": {
    "zh-CN": "恢复",
    "zh-HK": "恢復",
    en: "Restore",
  },
  "contract.confirm_all": {
    "zh-CN": "全选确认",
    "zh-HK": "全選確認",
    en: "Confirm All",
  },
  "contract.import_confirmed": {
    "zh-CN": "导入已确认行",
    "zh-HK": "導入已確認行",
    en: "Import Confirmed",
  },
  "contract.ai_draft_title": {
    "zh-CN": "AI 识别结果草稿",
    "zh-HK": "AI 識別結果草稿",
    en: "AI Recognition Draft",
  },
  "contract.ai_warning": {
    "zh-CN": "AI 警告",
    "zh-HK": "AI 警告",
    en: "AI Warning",
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
  "contract.posting_confirm": {
    "zh-CN": "过账确认",
    "zh-HK": "過賬確認",
    en: "Posting Confirmation",
  },
  "contract.posting_confirm_desc": {
    "zh-CN": "确认将以下分录过账？",
    "zh-HK": "確認將以下分錄過賬？",
    en: "Confirm posting the following entry?",
  },
  "contract.entry_type": {
    "zh-CN": "分录类型",
    "zh-HK": "分錄類型",
    en: "Entry Type",
  },
  "contract.erp_reference": {
    "zh-CN": "ERP 凭证号（可选）",
    "zh-HK": "ERP 憑證號（可選）",
    en: "ERP Reference (Optional)",
  },
  "contract.erp_placeholder": {
    "zh-CN": "输入 ERP 凭证号",
    "zh-HK": "輸入 ERP 憑證號",
    en: "Enter ERP reference",
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
  "contract.reason_required": {
    "zh-CN": "请输入原因",
    "zh-HK": "請輸入原因",
    en: "Please enter reason",
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
  "contract.edit_draft": {
    "zh-CN": "编辑草稿行",
    "zh-HK": "編輯草稿行",
    en: "Edit Draft Row",
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
  "contract.currency_cny": {
    "zh-CN": "人民币 (CNY)",
    "zh-HK": "人民幣 (CNY)",
    en: "CNY",
  },
  "contract.currency_usd": {
    "zh-CN": "美元 (USD)",
    "zh-HK": "美元 (USD)",
    en: "USD",
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
  "monthly.subtitle": {
    "zh-CN": "IFRS 16 租赁负债月结生成、分录预览与过账管理",
    "zh-HK": "IFRS 16 租賃負債月結生成、分錄預覽與過賬管理",
    en: "IFRS 16 lease liability monthly closing, entry preview and posting management",
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
  "monthly.lock_status": {
    "zh-CN": "锁账状态",
    "zh-HK": "鎖賬狀態",
    en: "Lock Status",
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
  "common.save": {
    "zh-CN": "保存",
    "zh-HK": "保存",
    en: "Save",
  },
  "common.delete": {
    "zh-CN": "删除",
    "zh-HK": "刪除",
    en: "Delete",
  },
  "common.edit": {
    "zh-CN": "编辑",
    "zh-HK": "編輯",
    en: "Edit",
  },
  "common.create": {
    "zh-CN": "创建",
    "zh-HK": "創建",
    en: "Create",
  },
  "common.cancel": {
    "zh-CN": "取消",
    "zh-HK": "取消",
    en: "Cancel",
  },
  "common.confirm": {
    "zh-CN": "确认",
    "zh-HK": "確認",
    en: "Confirm",
  },
  "common.submit": {
    "zh-CN": "提交",
    "zh-HK": "提交",
    en: "Submit",
  },
  "common.search": {
    "zh-CN": "搜索",
    "zh-HK": "搜索",
    en: "Search",
  },
  "common.reset": {
    "zh-CN": "重置",
    "zh-HK": "重置",
    en: "Reset",
  },
  "common.loading": {
    "zh-CN": "加载中...",
    "zh-HK": "加載中...",
    en: "Loading...",
  },
  "common.no_data": {
    "zh-CN": "暂无数据",
    "zh-HK": "暫無數據",
    en: "No data",
  },
  "common.success": {
    "zh-CN": "成功",
    "zh-HK": "成功",
    en: "Success",
  },
  "common.failed": {
    "zh-CN": "失败",
    "zh-HK": "失敗",
    en: "Failed",
  },
  "common.please_select": {
    "zh-CN": "请选择",
    "zh-HK": "請選擇",
    en: "Please select",
  },
  "common.please_enter": {
    "zh-CN": "请输入",
    "zh-HK": "請輸入",
    en: "Please enter",
  },

  // ─── New Contract ──────────────────────────────────────────────
  "contract_new.title": {
    "zh-CN": "新增合同",
    "zh-HK": "新增合同",
    en: "New Contract",
  },
  "contract_new.subtitle": {
    "zh-CN": "手工录入 · 也可改用 AI 上传解析",
    "zh-HK": "手工錄入 · 也可改用 AI 上載解析",
    en: "Manual entry · or use AI upload and parsing",
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
  "contract_new.entity_le001": {
    "zh-CN": "零售集团总公司",
    "zh-HK": "零售集團總公司",
    en: "Retail Group Headquarters",
  },
  "contract_new.entity_le002": {
    "zh-CN": "零售集团上海公司",
    "zh-HK": "零售集團上海公司",
    en: "Retail Group Shanghai",
  },
  "contract_new.store_nanjing": {
    "zh-CN": "南京东路旗舰店",
    "zh-HK": "南京東路旗艦店",
    en: "Nanjing East Road Flagship",
  },
  "contract_new.store_huaihai": {
    "zh-CN": "淮海路店",
    "zh-HK": "淮海路店",
    en: "Huaihai Road Store",
  },
  "contract_new.lessor_shanghai": {
    "zh-CN": "上海商业地产集团",
    "zh-HK": "上海商業地產集團",
    en: "Shanghai Commercial Real Estate Group",
  },
  "contract_new.lessor_beijing": {
    "zh-CN": "北京购物中心管理",
    "zh-HK": "北京購物中心管理",
    en: "Beijing Shopping Center Management",
  },
  "contract_new.currency_cny": {
    "zh-CN": "人民币 (CNY)",
    "zh-HK": "人民幣 (CNY)",
    en: "CNY (Renminbi)",
  },
  "contract_new.currency_usd": {
    "zh-CN": "美元 (USD)",
    "zh-HK": "美元 (USD)",
    en: "USD (US Dollar)",
  },
  "contract_new.currency_eur": {
    "zh-CN": "欧元 (EUR)",
    "zh-HK": "歐元 (EUR)",
    en: "EUR (Euro)",
  },

  // ─── Cashflow Forecast ───────────────────────────────────────
  "cashflow.title": {
    "zh-CN": "未来租金现金流预测",
    "zh-HK": "未來租金現金流預測",
    en: "Future Rent Cashflow Forecast",
  },
  "cashflow.description": {
    "zh-CN": "基于合同与付款计划预测未来期间的租金现金流出",
    "zh-HK": "基於合同與付款計劃預測未來期間的租金現金流出",
    en: "Forecast future rent cash outflows based on contracts and payment schedules",
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
  "cashflow.working_hint": {
    "zh-CN": "工作报表模式：包含所有合同（含草稿、待审批），用于内部试算和测试",
    "zh-HK": "工作報表模式：包含所有合同（含草稿、待審批），用於內部試算和測試",
    en: "Working mode: includes all contracts (draft, pending) for internal testing",
  },
  "cashflow.official_hint": {
    "zh-CN": "正式报表模式：仅包含已审批合同，用于正式财务和审计",
    "zh-HK": "正式報表模式：僅包含已審批合同，用於正式財務和審計",
    en: "Official mode: only approved contracts for formal financials and audit",
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
  "settings.title": {
    "zh-CN": "标签总管",
    "zh-HK": "標籤總管",
    en: "Tag Manager",
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
  "settings.description": {
    "zh-CN": "标签用于驱动 IFRS 16 摊销报表的多维度分组与汇总分析",
    "zh-HK": "標籤用於驅動 IFRS 16 攤銷報表的多維度分組與匯總分析",
    en: "Tags are used to drive multi-dimensional grouping and summary analysis in IFRS 16 amortization reports",
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
  "upload.title": {
    "zh-CN": "智能合同录入 — AI 辅助批量解析",
    "zh-HK": "智能合同錄入 — AI 輔助批量解析",
    en: "Smart Contract Entry — AI Assisted Batch Parsing",
  },
  "upload.step_upload": {
    "zh-CN": "上传文件",
    "zh-HK": "上傳文件",
    en: "Upload Files",
  },
  "upload.step_parse": {
    "zh-CN": "AI 解析",
    "zh-HK": "AI 解析",
    en: "AI Parse",
  },
  "upload.step_confirm": {
    "zh-CN": "批量确认",
    "zh-HK": "批量確認",
    en: "Batch Confirm",
  },
  "upload.step_complete": {
    "zh-CN": "完成",
    "zh-HK": "完成",
    en: "Complete",
  },
  "upload.upload_card_title": {
    "zh-CN": "上传合同文件",
    "zh-HK": "上傳合同文件",
    en: "Upload Contract Files",
  },
  "upload.file_count": {
    "zh-CN": "{n} 个文件",
    "zh-HK": "{n} 個文件",
    en: "{n} files",
  },
  "upload.reupload": {
    "zh-CN": "重新上传",
    "zh-HK": "重新上傳",
    en: "Re-upload",
  },
  "upload.drag_hint": {
    "zh-CN": "点击或拖拽合同文件到此区域（支持批量）",
    "zh-HK": "點擊或拖拽合同文件到此區域（支持批量）",
    en: "Click or drag contract files here (batch supported)",
  },
  "upload.support_formats": {
    "zh-CN": "支持 PDF、Excel、JPG、PNG、TIFF 格式，单个文件不超过 50MB",
    "zh-HK": "支持 PDF、Excel、JPG、PNG、TIFF 格式，單個文件不超過 50MB",
    en: "Supports PDF, Excel, JPG, PNG, TIFF. Max 50MB per file.",
  },
  "upload.uploaded_files": {
    "zh-CN": "已上传文件",
    "zh-HK": "已上傳文件",
    en: "Uploaded Files",
  },
  "upload.start_parse": {
    "zh-CN": "开始解析 ({n} 个)",
    "zh-HK": "開始解析 ({n} 個)",
    en: "Start Parsing ({n})",
  },
  "upload.upload_success": {
    "zh-CN": "{name} 上传成功",
    "zh-HK": "{name} 上傳成功",
    en: "{name} uploaded successfully",
  },
  "upload.upload_failed": {
    "zh-CN": "{name} 上传失败",
    "zh-HK": "{name} 上傳失敗",
    en: "{name} upload failed",
  },
  "upload.unsupported_file": {
    "zh-CN": "不支持的文件类型，请上传 PDF、Excel 或图片文件",
    "zh-HK": "不支持的文件類型，請上傳 PDF、Excel 或圖片文件",
    en: "Unsupported file type. Please upload PDF, Excel or image files.",
  },
  "upload.file_too_large": {
    "zh-CN": "文件大小不能超过 50MB",
    "zh-HK": "文件大小不能超過 50MB",
    en: "File size cannot exceed 50MB",
  },
  "upload.status_uploading": {
    "zh-CN": "上传中",
    "zh-HK": "上傳中",
    en: "Uploading",
  },
  "upload.status_uploaded": {
    "zh-CN": "已上传",
    "zh-HK": "已上傳",
    en: "Uploaded",
  },
  "upload.status_parsing": {
    "zh-CN": "解析中",
    "zh-HK": "解析中",
    en: "Parsing",
  },
  "upload.status_parsed": {
    "zh-CN": "已解析",
    "zh-HK": "已解析",
    en: "Parsed",
  },
  "upload.status_failed": {
    "zh-CN": "失败",
    "zh-HK": "失敗",
    en: "Failed",
  },
  "upload.remove": {
    "zh-CN": "移除",
    "zh-HK": "移除",
    en: "Remove",
  },
  "upload.parsing_progress": {
    "zh-CN": "解析第 {n}/{total} 份合同...",
    "zh-HK": "解析第 {n}/{total} 份合同...",
    en: "Parsing contract {n}/{total}...",
  },
  "upload.parsing_detail": {
    "zh-CN": "正在使用 PaddleOCR + DeepSeek...",
    "zh-HK": "正在使用 PaddleOCR + DeepSeek...",
    en: "Using PaddleOCR + DeepSeek...",
  },
  "upload.confirm_title": {
    "zh-CN": "批量确认合同信息",
    "zh-HK": "批量確認合同信息",
    en: "Batch Confirm Contract Information",
  },
  "upload.parsed_count": {
    "zh-CN": "{n} 份已解析",
    "zh-HK": "{n} 份已解析",
    en: "{n} parsed",
  },
  "upload.failed_count": {
    "zh-CN": "{n} 份失败",
    "zh-HK": "{n} 份失敗",
    en: "{n} failed",
  },
  "upload.batch_create": {
    "zh-CN": "批量创建合同 ({n})",
    "zh-HK": "批量創建合同 ({n})",
    en: "Batch Create Contracts ({n})",
  },
  "upload.parse_failed_alert": {
    "zh-CN": "{n} 份文件解析失败，请检查后重新上传",
    "zh-HK": "{n} 份文件解析失敗，請檢查後重新上傳",
    en: "{n} files failed to parse, please check and re-upload",
  },
  "upload.creating_progress": {
    "zh-CN": "创建第 {n}/{total} 份合同...",
    "zh-HK": "創建第 {n}/{total} 份合同...",
    en: "Creating contract {n}/{total}...",
  },
  "upload.partial_success": {
    "zh-CN": "部分创建成功",
    "zh-HK": "部分創建成功",
    en: "Partially created",
  },
  "upload.all_success": {
    "zh-CN": "全部创建成功！",
    "zh-HK": "全部創建成功！",
    en: "All created successfully!",
  },
  "upload.success_count": {
    "zh-CN": "成功创建 {n} 份合同",
    "zh-HK": "成功創建 {n} 份合同",
    en: "Successfully created {n} contracts",
  },
  "upload.failed_count_result": {
    "zh-CN": "{n} 份失败",
    "zh-HK": "{n} 份失敗",
    en: "{n} failed",
  },
  "upload.view_contracts": {
    "zh-CN": "查看合同列表",
    "zh-HK": "查看合同列表",
    en: "View Contract List",
  },
  "upload.reupload_btn": {
    "zh-CN": "重新上传",
    "zh-HK": "重新上傳",
    en: "Re-upload",
  },
  "upload.processing": {
    "zh-CN": "处理中...",
    "zh-HK": "處理中...",
    en: "Processing...",
  },
  "upload.edit_modal_title": {
    "zh-CN": "编辑合同信息",
    "zh-HK": "編輯合同信息",
    en: "Edit Contract Information",
  },
  "upload.save": {
    "zh-CN": "保存",
    "zh-HK": "保存",
    en: "Save",
  },
  "upload.cancel": {
    "zh-CN": "取消",
    "zh-HK": "取消",
    en: "Cancel",
  },
  "upload.ai_tips": {
    "zh-CN": "AI 解析提示 ({n} 条)",
    "zh-HK": "AI 解析提示 ({n} 條)",
    en: "AI Parse Tips ({n})",
  },
  "upload.col_file": {
    "zh-CN": "文件",
    "zh-HK": "文件",
    en: "File",
  },
  "upload.col_contract_number": {
    "zh-CN": "合同编号",
    "zh-HK": "合同編號",
    en: "Contract Number",
  },
  "upload.col_lessee": {
    "zh-CN": "承租方",
    "zh-HK": "承租方",
    en: "Lessee",
  },
  "upload.col_lessor": {
    "zh-CN": "出租方",
    "zh-HK": "出租方",
    en: "Lessor",
  },
  "upload.col_store": {
    "zh-CN": "门店",
    "zh-HK": "門店",
    en: "Store",
  },
  "upload.col_start_date": {
    "zh-CN": "起始日",
    "zh-HK": "起始日",
    en: "Start Date",
  },
  "upload.col_end_date": {
    "zh-CN": "结束日",
    "zh-HK": "結束日",
    en: "End Date",
  },
  "upload.col_rent": {
    "zh-CN": "租金",
    "zh-HK": "租金",
    en: "Rent",
  },
  "upload.col_status": {
    "zh-CN": "状态",
    "zh-HK": "狀態",
    en: "Status",
  },
  "upload.col_action": {
    "zh-CN": "操作",
    "zh-HK": "操作",
    en: "Action",
  },
  "upload.action_edit": {
    "zh-CN": "编辑",
    "zh-HK": "編輯",
    en: "Edit",
  },
  "upload.section_basic": {
    "zh-CN": "合同基本信息",
    "zh-HK": "合同基本信息",
    en: "Basic Information",
  },
  "upload.section_dates": {
    "zh-CN": "日期与金额",
    "zh-HK": "日期與金額",
    en: "Dates & Amounts",
  },
  "upload.section_discount": {
    "zh-CN": "折现率设置",
    "zh-HK": "折現率設置",
    en: "Discount Rate Settings",
  },
  "upload.section_other": {
    "zh-CN": "其他费用",
    "zh-HK": "其他費用",
    en: "Other Fees",
  },
  "upload.section_tags": {
    "zh-CN": "标签 / 备注",
    "zh-HK": "標籤 / 備註",
    en: "Tags / Notes",
  },
  "upload.please_login": {
    "zh-CN": "请先登录",
    "zh-HK": "請先登錄",
    en: "Please login first",
  },
  "upload.no_files": {
    "zh-CN": "没有待解析的文件",
    "zh-HK": "沒有待解析的文件",
    en: "No files to parse",
  },
  "upload.parse_failed": {
    "zh-CN": "解析失败",
    "zh-HK": "解析失敗",
    en: "Parse failed",
  },
  "upload.edit_only_parsed": {
    "zh-CN": "只能编辑已解析完成的合同",
    "zh-HK": "只能編輯已解析完成的合同",
    en: "Can only edit fully parsed contracts",
  },
  "upload.select_at_least_one": {
    "zh-CN": "请至少选择一个合同",
    "zh-HK": "請至少選擇一個合同",
    en: "Please select at least one contract",
  },
  "upload.modify_saved": {
    "zh-CN": "修改已保存",
    "zh-HK": "修改已保存",
    en: "Changes saved",
  },
  "upload.hint_text": {
    "zh-CN": "支持批量上传合同文件，上传后 AI 将依次解析并提取关键信息...",
    "zh-HK": "支持批量上傳合同文件，上傳後 AI 將依次解析並提取關鍵信息...",
    en: "Supports batch upload of contract files. After upload, AI will parse and extract key information...",
  },

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
    "zh-CN": "共 {count} 个用户 · 管理员可创建和维护账号",
    "zh-HK": "共 {count} 個用戶 · 管理員可建立及維護帳號",
    en: "{count} users · Admins can create and maintain accounts",
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
  "ai.could_not_understand": {
    "zh-CN": "抱歉，我无法理解您的问题。",
    "zh-HK": "抱歉，我無法理解您的問題。",
    en: "Sorry, I couldn't understand your question.",
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
  "ai.agent_step_human_review_done": {
    "zh-CN": "人工已确认草稿",
    "zh-HK": "人工已確認草稿",
    en: "Human Review Confirmed",
  },
  "ai.agent_step_create_draft": {
    "zh-CN": "创建 draft 合同",
    "zh-HK": "創建 draft 合同",
    en: "Create Draft Contracts",
  },
  "ai.agent_create_input": {
    "zh-CN": "提交 {count} 份已确认草稿到 Core Service",
    "zh-HK": "提交 {count} 份已確認草稿到 Core Service",
    en: "Submit {count} confirmed drafts to Core Service",
  },
  "ai.agent_create_output": {
    "zh-CN": "创建完成：成功 {success} 份，失败 {failed} 份",
    "zh-HK": "創建完成：成功 {success} 份，失敗 {failed} 份",
    en: "Creation completed: {success} succeeded, {failed} failed",
  },
  "ai.agent_create_failed_title": {
    "zh-CN": "处理失败合同",
    "zh-HK": "處理失敗合同",
    en: "Resolve Failed Contracts",
  },
  "ai.agent_create_failed_description": {
    "zh-CN": "{count} 份合同未能创建为 draft，需要检查错误详情。",
    "zh-HK": "{count} 份合同未能創建為 draft，需要檢查錯誤詳情。",
    en: "{count} contracts could not be created as drafts. Review the failure details.",
  },
  "ai.agent_create_failed_action": {
    "zh-CN": "根据失败详情修正草稿字段、主数据或重复合同编号后重新确认。",
    "zh-HK": "根據失敗詳情修正草稿字段、主數據或重複合同編號後重新確認。",
    en: "Fix draft fields, master data, or duplicate contract numbers based on the failure details, then confirm again.",
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
  "ai.batch_create_failed_details": {
    "zh-CN": "\n\n失败详情：\n",
    "zh-HK": "\n\n失敗詳情：\n",
    en: "\n\nFailure details:\n",
  },
  "ai.batch_create_failed": {
    "zh-CN": "批量创建失败: {error}",
    "zh-HK": "批量創建失敗: {error}",
    en: "Batch creation failed: {error}",
  },
  "ai.skip_import": {
    "zh-CN": "已跳过本次合同入库。您可以修改文件后重新上传，或手动录入合同。",
    "zh-HK": "已跳過本次合同入庫。您可以修改文件後重新上傳，或手動錄入合同。",
    en: "Import skipped. You can modify the file and re-upload, or manually enter the contract.",
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
  "ai.schedule_step_import": {
    "zh-CN": "导入付款计划",
    "zh-HK": "導入付款計劃",
    en: "Import Payment Schedules",
  },
  "ai.schedule_import_input": {
    "zh-CN": "提交 {count} 笔已确认付款计划到 Core Service",
    "zh-HK": "提交 {count} 筆已確認付款計劃到 Core Service",
    en: "Submit {count} confirmed payment schedules to Core Service",
  },
  "ai.schedule_import_output": {
    "zh-CN": "付款计划导入完成：成功 {count} 笔",
    "zh-HK": "付款計劃導入完成：成功 {count} 筆",
    en: "Payment schedule import completed: {count} succeeded",
  },
  "ai.schedule_import_result": {
    "zh-CN": "付款计划导入完成：已向合同 {contract} 写入 {count} 笔付款计划。",
    "zh-HK": "付款計劃導入完成：已向合同 {contract} 寫入 {count} 筆付款計劃。",
    en: "Payment schedule import completed: wrote {count} schedules to contract {contract}.",
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
  "reports.subtitle": {
    "zh-CN": "租赁负债滚动表与摊销报表查询",
    "zh-HK": "租賃負債滾動表與攤銷報表查詢",
    en: "Lease Liability Roll-forward & Amortization Report Query",
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
  "reports.select_currency": {
    "zh-CN": "选择货币",
    "zh-HK": "選擇貨幣",
    en: "Select Currency",
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
  "reports.ai_chat_params": {
    "zh-CN": "AI 对话参数",
    "zh-HK": "AI 對話參數",
    en: "AI Chat Parameters",
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
  "contract_detail.ai_parse_failed": {
    "zh-CN": "AI 解析失败",
    "zh-HK": "AI 解析失敗",
    en: "AI parse failed",
  },
  "contract_detail.import_failed": {
    "zh-CN": "导入失败",
    "zh-HK": "導入失敗",
    en: "Import failed",
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
  "contract_detail.discount_rate_missing_title": {
    "zh-CN": "折现率未设置",
    "zh-HK": "折現率未設置",
    en: "Discount rate not set",
  },
  "contract_detail.discount_rate_missing_content": {
    "zh-CN": "该合同尚未设置折现率，无法生成 IFRS 16 计算表格。请先前往设置页面配置集团默认折现率，或在合同中填写具体折现率。",
    "zh-HK": "該合同尚未設置折現率，無法生成 IFRS 16 計算表格。請先前往設置頁面配置集團默認折現率，或在合同中填寫具體折現率。",
    en: "This contract does not have a discount rate set and cannot generate an IFRS 16 calculation table. Please go to Settings to configure the group default discount rate, or enter a specific discount rate for this contract.",
  },
  "contract_detail.go_to_settings": {
    "zh-CN": "前往设置",
    "zh-HK": "前往設置",
    en: "Go to Settings",
  },
  "contract_detail.cancel": {
    "zh-CN": "取消",
    "zh-HK": "取消",
    en: "Cancel",
  },
  "contract_detail.contract_updated": {
    "zh-CN": "合同更新成功",
    "zh-HK": "合同更新成功",
    en: "Contract updated",
  },
  "contract_detail.draft_updated": {
    "zh-CN": "草稿行已更新",
    "zh-HK": "草稿行已更新",
    en: "Draft row updated",
  },
  "contract_detail.confirmed_all": {
    "zh-CN": "已全选确认",
    "zh-HK": "已全選確認",
    en: "All confirmed",
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
  "contract_detail.ai_parse_warning": {
    "zh-CN": "AI 识别结果需要人工复核，请检查低置信度字段",
    "zh-HK": "AI 識別結果需要人工覆核，請檢查低置信度字段",
    en: "AI recognition requires human review, please check low-confidence fields",
  },
  "contract_detail.no_confirmed_drafts": {
    "zh-CN": "没有已确认的草稿可导入",
    "zh-HK": "沒有已確認的草稿可導入",
    en: "No confirmed drafts to import",
  },

  // Template messages (with {count} placeholder)
  "contract_detail.ai_parse_success": {
    "zh-CN": "AI 识别完成：{count} 笔付款计划",
    "zh-HK": "AI 識別完成：{count} 筆付款計劃",
    en: "AI recognition complete: {count} payment schedules",
  },
  "contract_detail.agent_payment_summary": {
    "zh-CN": "当前任务：为此合同解析租金表并生成付款计划草稿。",
    "zh-HK": "當前任務：為此合同解析租金表並生成付款計劃草稿。",
    en: "Current task: parse a rent schedule for this contract and generate payment schedule drafts.",
  },
  "contract_detail.import_success": {
    "zh-CN": "成功导入 {count} 笔付款计划",
    "zh-HK": "成功導入 {count} 筆付款計劃",
    en: "Successfully imported {count} payment schedules",
  },

  // Inline UI texts
  "contract_detail.item_unit": {
    "zh-CN": "笔",
    "zh-HK": "筆",
    en: "items",
  },
  "contract_detail.ai_draft_count": {
    "zh-CN": "{confirmed} 笔已确认 / {total} 笔总计",
    "zh-HK": "{confirmed} 筆已確認 / {total} 筆總計",
    en: "{confirmed} confirmed / {total} total",
  },
  "contract_detail.ai_warning_more": {
    "zh-CN": "... 等 {count} 条警告",
    "zh-HK": "... 等 {count} 條警告",
    en: "... and {count} more warnings",
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
  "dashboard.kpi_total_rou": {
    "zh-CN": "使用权资产净额",
    "zh-HK": "使用權資產淨額",
    en: "Net ROU Assets",
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
  "dashboard.kpi_next12m_cashout": {
    "zh-CN": "未来 12 个月现金流出",
    "zh-HK": "未來 12 個月現金流出",
    en: "Next 12M Cash Outflow",
  },
  "dashboard.kpi_contracts_sub": {
    "zh-CN": "合同 {total} 份 · 已批 {approved} · 待批 {pending} · 草稿 {draft}",
    "zh-HK": "合同 {total} 份 · 已批 {approved} · 待批 {pending} · 草稿 {draft}",
    en: "{total} contracts · {approved} approved · {pending} pending · {draft} draft",
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
