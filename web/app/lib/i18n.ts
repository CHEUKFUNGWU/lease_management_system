export type Language = "zh-CN" | "zh-HK" | "en";

interface TranslationDict {
  [key: string]: {
    "zh-CN": string;
    "zh-HK": string;
    en: string;
  };
}

const dict: TranslationDict = {
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
  "nav.upload": {
    "zh-CN": "文件上传",
    "zh-HK": "文件上傳",
    en: "Upload",
  },
  "nav.ai_chat": {
    "zh-CN": "AI Agent",
    "zh-HK": "AI Agent",
    en: "AI Agent",
  },
  "nav.reports": {
    "zh-CN": "报表查询",
    "zh-HK": "報表查詢",
    en: "Reports",
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
    "zh-CN": "租赁管理系统",
    "zh-HK": "租賃管理系統",
    en: "Lease Management",
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
    "zh-CN": "你好！我是租赁管理 AI Agent。我可以把文件和数据作为工作入口，调用内置 skill 帮你完成任务：\n\n1. 读取非标准 Excel 台账、租金表和付款流水\n2. 解析 PDF/Word/图片合同并生成草稿\n3. 查询合同台账、计量结果、分录和审批状态\n4. 生成需要人工确认的缺失字段和风险提示\n\n上传合同文件或台账后，我会先生成草稿和工具执行轨迹，确认后才创建 draft 合同并进入审批流程。",
    "zh-HK": "你好！我是租賃管理 AI Agent。我可以把文件和數據作為工作入口，調用內置 skill 幫你完成任務：\n\n1. 讀取非標準 Excel 台賬、租金表和付款流水\n2. 解析 PDF/Word/圖片合同並生成草稿\n3. 查詢合同台賬、計量結果、分錄和審批狀態\n4. 生成需要人工確認的缺失字段和風險提示\n\n上傳合同文件或台賬後，我會先生成草稿和工具執行軌跡，確認後才創建 draft 合同並進入審批流程。",
    en: "Hello! I am the Lease Management AI Agent. I use files and system data as the working entrypoint and call built-in skills to complete tasks:\n\n1. Read non-standard Excel ledgers, rent schedules, and payment files\n2. Parse PDF/Word/image contracts and generate drafts\n3. Query contracts, measurements, journal entries, and approval status\n4. Surface missing fields and risk prompts for human review\n\nAfter you upload a contract or ledger, I will generate drafts and a tool execution trace first. Only confirmed items are created as draft contracts for approval.",
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
    "zh-CN": "列出折现率缺失合同",
    "zh-HK": "列出折現率缺失合同",
    en: "List contracts with missing discount rates",
  },
  "ai.chip_pending": {
    "zh-CN": "有哪些待审批事项？",
    "zh-HK": "有哪些待審批事項？",
    en: "What items are pending approval?",
  },
  "ai.chip_expiring": {
    "zh-CN": "哪些合同即将到期？",
    "zh-HK": "哪些合同即將到期？",
    en: "Which contracts are expiring soon?",
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
    "zh-CN": "上传文件",
    "zh-HK": "上傳文件",
    en: "Upload File",
  },
  "dashboard.upload_file_desc": {
    "zh-CN": "AI 解析合同或付款计划",
    "zh-HK": "AI 解析合同或付款計劃",
    en: "AI parses contracts or payment schedules",
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
  "contracts.col_discount_rate": {
    "zh-CN": "折现率",
    "zh-HK": "折現率",
    en: "Discount Rate",
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
  "contracts.set": {
    "zh-CN": "已设置",
    "zh-HK": "已設置",
    en: "Set",
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
  "monthly.reject_confirm": {
    "zh-CN": "确认驳回该分录？",
    "zh-HK": "確認駁回該分錄？",
    en: "Confirm rejecting this entry?",
  },
  "monthly.reverse_confirm": {
    "zh-CN": "冲销功能待实现",
    "zh-HK": "沖銷功能待實現",
    en: "Reverse feature coming soon",
  },
  "monthly.reject_coming_soon": {
    "zh-CN": "驳回功能待实现",
    "zh-HK": "駁回功能待實現",
    en: "Reject feature coming soon",
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
    "zh-CN": "租赁管理系统",
    "zh-HK": "租賃管理系統",
    en: "Lease Management",
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
    "zh-CN": "租赁管理系统 — 管理员通道",
    "zh-HK": "租賃管理系統 — 管理員通道",
    en: "Lease Management System — Admin Channel",
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
    "zh-CN": "租赁管理 AI Agent",
    "zh-HK": "租賃管理 AI Agent",
    en: "Lease Management AI Agent",
  },
  "ai.agent_trace_title": {
    "zh-CN": "Agent 技能执行轨迹",
    "zh-HK": "Agent 技能執行軌跡",
    en: "Agent Skill Trace",
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
    "zh-CN": "上传文件",
    "zh-HK": "上傳文件",
    en: "Upload File",
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
};

export function t(key: string, lang: Language, replacements?: Record<string, string>): string {
  const entry = dict[key];
  if (!entry) {
    return key;
  }
  let text = entry[lang] || entry["zh-CN"];
  if (replacements) {
    Object.entries(replacements).forEach(([k, v]) => {
      text = text.replace(`{${k}}`, v);
    });
  }
  return text;
}
