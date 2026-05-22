export type Language = "zh-CN" | "zh-TW" | "en";

interface TranslationDict {
  [key: string]: {
    "zh-CN": string;
    "zh-TW": string;
    en: string;
  };
}

const dict: TranslationDict = {
  // AppLayout
  "nav.home": {
    "zh-CN": "首页",
    "zh-TW": "首頁",
    en: "Home",
  },
  "nav.contracts": {
    "zh-CN": "合同台账",
    "zh-TW": "合同台賬",
    en: "Contracts",
  },
  "nav.upload": {
    "zh-CN": "文件上传",
    "zh-TW": "文件上傳",
    en: "Upload",
  },
  "nav.ai_chat": {
    "zh-CN": "AI 助手",
    "zh-TW": "AI 助手",
    en: "AI Assistant",
  },
  "nav.reports": {
    "zh-CN": "报表查询",
    "zh-TW": "報表查詢",
    en: "Reports",
  },
  "nav.cashflow": {
    "zh-CN": "现金流预测",
    "zh-TW": "現金流預測",
    en: "Cashflow Forecast",
  },
  "nav.monthly_closing": {
    "zh-CN": "结账中心",
    "zh-TW": "結賬中心",
    en: "Monthly Closing",
  },
  "nav.audit_logs": {
    "zh-CN": "审计日志",
    "zh-TW": "審計日誌",
    en: "Audit Logs",
  },
  "nav.settings": {
    "zh-CN": "设置",
    "zh-TW": "設置",
    en: "Settings",
  },
  "nav.admin": {
    "zh-CN": "管理后台",
    "zh-TW": "管理後台",
    en: "Admin",
  },
  "app.title": {
    "zh-CN": "租赁管理系统",
    "zh-TW": "租賃管理系統",
    en: "Lease Management",
  },
  "user.profile": {
    "zh-CN": "个人资料",
    "zh-TW": "個人資料",
    en: "Profile",
  },
  "user.logout": {
    "zh-CN": "退出登录",
    "zh-TW": "退出登錄",
    en: "Logout",
  },
  "lang.zh_CN": {
    "zh-CN": "简体中文",
    "zh-TW": "簡體中文",
    en: "Simplified Chinese",
  },
  "lang.zh_TW": {
    "zh-CN": "繁体中文",
    "zh-TW": "繁體中文",
    en: "Traditional Chinese",
  },
  "lang.en": {
    "zh-CN": "英文",
    "zh-TW": "英文",
    en: "English",
  },

  // AI Chat
  "ai.welcome": {
    "zh-CN": "你好！我是 IFRS 16 AI 助手。我可以帮你：\n\n1. 查询合同台账信息\n2. 查看 IFRS 16 计量结果\n3. 了解审批状态\n4. 回答 IFRS 16 会计问题\n\n你还可以上传合同文件或租金表，我会帮你解析其中的关键信息。",
    "zh-TW": "你好！我是 IFRS 16 AI 助手。我可以幫你：\n\n1. 查詢合同台賬信息\n2. 查看 IFRS 16 計量結果\n3. 了解審批狀態\n4. 回答 IFRS 16 會計問題\n\n你還可以上傳合同文件或租金表，我會幫你解析其中的關鍵信息。",
    en: "Hello! I am the IFRS 16 AI Assistant. I can help you:\n\n1. Query contract ledger information\n2. View IFRS 16 measurement results\n3. Check approval status\n4. Answer IFRS 16 accounting questions\n\nYou can also upload contract files or rent schedules, and I will help you parse the key information.",
  },
  "ai.context": {
    "zh-CN": "当前上下文：",
    "zh-TW": "當前上下文：",
    en: "Current Context:",
  },
  "ai.quick_questions": {
    "zh-CN": "快捷提问：",
    "zh-TW": "快捷提問：",
    en: "Quick Questions:",
  },
  "ai.placeholder": {
    "zh-CN": "输入问题，按 Enter 发送，Shift+Enter 换行...",
    "zh-TW": "輸入問題，按 Enter 發送，Shift+Enter 換行...",
    en: "Type a question, press Enter to send, Shift+Enter for new line...",
  },
  "ai.send": {
    "zh-CN": "发送",
    "zh-TW": "發送",
    en: "Send",
  },
  "ai.thinking": {
    "zh-CN": "AI 思考中...",
    "zh-TW": "AI 思考中...",
    en: "AI thinking...",
  },
  "ai.sources": {
    "zh-CN": "引用来源:",
    "zh-TW": "引用來源:",
    en: "Sources:",
  },
  "ai.model": {
    "zh-CN": "模型: ",
    "zh-TW": "模型: ",
    en: "Model: ",
  },
  "ai.upload_success": {
    "zh-CN": "上传成功",
    "zh-TW": "上傳成功",
    en: "Upload successful",
  },
  "ai.upload_failed": {
    "zh-CN": "上传失败",
    "zh-TW": "上傳失敗",
    en: "Upload failed",
  },
  "ai.file_uploaded": {
    "zh-CN": "已上传文件: ",
    "zh-TW": "已上傳文件: ",
    en: "Uploaded file: ",
  },
  "ai.unsupported_file": {
    "zh-CN": "不支持的文件类型，请上传 PDF、Excel 或图片文件",
    "zh-TW": "不支持的文件類型，請上傳 PDF、Excel 或圖片文件",
    en: "Unsupported file type. Please upload PDF, Excel, or image files.",
  },
  "ai.file_too_large": {
    "zh-CN": "文件大小不能超过 50MB",
    "zh-TW": "文件大小不能超過 50MB",
    en: "File size cannot exceed 50MB",
  },

  // Reports
  "reports.title": {
    "zh-CN": "报表查询",
    "zh-TW": "報表查詢",
    en: "Reports",
  },
  "reports.mode": {
    "zh-CN": "报表模式",
    "zh-TW": "報表模式",
    en: "Report Mode",
  },
  "reports.working": {
    "zh-CN": "工作报表 (Working)",
    "zh-TW": "工作報表 (Working)",
    en: "Working Report",
  },
  "reports.official": {
    "zh-CN": "正式报表 (Official)",
    "zh-TW": "正式報表 (Official)",
    en: "Official Report",
  },
  "reports.working_alert_title": {
    "zh-CN": "工作报表模式",
    "zh-TW": "工作報表模式",
    en: "Working Report Mode",
  },
  "reports.working_alert_desc": {
    "zh-CN": "包含 Draft、Submitted、Reviewed、Pending Approval 状态的数据。用于内部试算、讨论和预演。",
    "zh-TW": "包含 Draft、Submitted、Reviewed、Pending Approval 狀態的數據。用於內部試算、討論和預演。",
    en: "Includes data with Draft, Submitted, Reviewed, and Pending Approval statuses. For internal trial calculations, discussions, and rehearsals.",
  },
  "reports.official_alert_title": {
    "zh-CN": "正式报表模式",
    "zh-TW": "正式報表模式",
    en: "Official Report Mode",
  },
  "reports.official_alert_desc": {
    "zh-CN": "仅包含 Approved 状态的数据。用于正式财务报告、审计提交和法定披露。",
    "zh-TW": "僅包含 Approved 狀態的數據。用於正式財務報告、審計提交和法定披露。",
    en: "Includes only Approved status data. For formal financial reporting, audit submissions, and statutory disclosures.",
  },
  "reports.tab_ledger": {
    "zh-CN": "合同台账",
    "zh-TW": "合同台賬",
    en: "Contract Ledger",
  },
  "reports.tab_amortization": {
    "zh-CN": "摊销报表",
    "zh-TW": "攤銷報表",
    en: "Amortization Report",
  },
  "reports.total_contracts": {
    "zh-CN": "合同总数",
    "zh-TW": "合同總數",
    en: "Total Contracts",
  },
  "reports.approved": {
    "zh-CN": "已审批",
    "zh-TW": "已審批",
    en: "Approved",
  },
  "reports.draft_pending": {
    "zh-CN": "草稿/待处理",
    "zh-TW": "草稿/待處理",
    en: "Draft / Pending",
  },
  "reports.contract_number": {
    "zh-CN": "合同编号",
    "zh-TW": "合同編號",
    en: "Contract Number",
  },
  "reports.contract_name": {
    "zh-CN": "合同名称",
    "zh-TW": "合同名稱",
    en: "Contract Name",
  },
  "reports.approval_status": {
    "zh-CN": "审批状态",
    "zh-TW": "審批狀態",
    en: "Approval Status",
  },
  "reports.is_official": {
    "zh-CN": "正式版本",
    "zh-TW": "正式版本",
    en: "Official Version",
  },
  "reports.discount_rate_missing": {
    "zh-CN": "折现率缺失",
    "zh-TW": "折現率缺失",
    en: "Discount Rate Missing",
  },
  "reports.currency": {
    "zh-CN": "币种",
    "zh-TW": "幣種",
    en: "Currency",
  },
  "reports.commencement_date": {
    "zh-CN": "起始日",
    "zh-TW": "起始日",
    en: "Start Date",
  },
  "reports.lease_end_date": {
    "zh-CN": "结束日",
    "zh-TW": "結束日",
    en: "End Date",
  },
  "reports.yes": {
    "zh-CN": "是",
    "zh-TW": "是",
    en: "Yes",
  },
  "reports.no": {
    "zh-CN": "否",
    "zh-TW": "否",
    en: "No",
  },
  "reports.missing": {
    "zh-CN": "缺失",
    "zh-TW": "缺失",
    en: "Missing",
  },
  "reports.filled": {
    "zh-CN": "已填",
    "zh-TW": "已填",
    en: "Filled",
  },
  "reports.empty": {
    "zh-CN": "暂无数据",
    "zh-TW": "暫無數據",
    en: "No data",
  },
  "reports.export_csv": {
    "zh-CN": "导出 CSV",
    "zh-TW": "導出 CSV",
    en: "Export CSV",
  },
  "reports.export_excel": {
    "zh-CN": "导出 Excel",
    "zh-TW": "導出 Excel",
    en: "Export Excel",
  },
  "reports.search": {
    "zh-CN": "查询",
    "zh-TW": "查詢",
    en: "Search",
  },
  "reports.reset": {
    "zh-CN": "重置",
    "zh-TW": "重置",
    en: "Reset",
  },
  "reports.ai_analysis": {
    "zh-CN": "AI 分析",
    "zh-TW": "AI 分析",
    en: "AI Analysis",
  },
  "reports.view_dimension": {
    "zh-CN": "视图维度",
    "zh-TW": "視圖維度",
    en: "View Dimension",
  },
  "reports.granularity": {
    "zh-CN": "粒度",
    "zh-TW": "粒度",
    en: "Granularity",
  },
  "reports.date_range": {
    "zh-CN": "日期范围",
    "zh-TW": "日期範圍",
    en: "Date Range",
  },
  "reports.contract_id": {
    "zh-CN": "合同 ID",
    "zh-TW": "合同 ID",
    en: "Contract ID",
  },
  "reports.store": {
    "zh-CN": "门店",
    "zh-TW": "門店",
    en: "Store",
  },
  "reports.tags": {
    "zh-CN": "标签",
    "zh-TW": "標籤",
    en: "Tags",
  },
  "reports.contract_view": {
    "zh-CN": "合同维度",
    "zh-TW": "合同維度",
    en: "Contract View",
  },
  "reports.store_view": {
    "zh-CN": "门店维度",
    "zh-TW": "門店維度",
    en: "Store View",
  },
  "reports.tag_view": {
    "zh-CN": "标签维度",
    "zh-TW": "標籤維度",
    en: "Tag View",
  },
  "reports.summary_view": {
    "zh-CN": "汇总",
    "zh-TW": "匯總",
    en: "Summary",
  },
  "reports.day": {
    "zh-CN": "日",
    "zh-TW": "日",
    en: "Day",
  },
  "reports.month": {
    "zh-CN": "月",
    "zh-TW": "月",
    en: "Month",
  },
  "reports.quarter": {
    "zh-CN": "季",
    "zh-TW": "季",
    en: "Quarter",
  },
  "reports.half_year": {
    "zh-CN": "半年",
    "zh-TW": "半年",
    en: "Half Year",
  },
  "reports.year": {
    "zh-CN": "年",
    "zh-TW": "年",
    en: "Year",
  },
  "reports.expand_filters": {
    "zh-CN": "展开筛选条件 ▼",
    "zh-TW": "展開篩選條件 ▼",
    en: "Expand Filters ▼",
  },
  "reports.collapse_filters": {
    "zh-CN": "收起筛选条件 ▲",
    "zh-TW": "收起篩選條件 ▲",
    en: "Collapse Filters ▲",
  },
  "reports.discount_rate_override": {
    "zh-CN": "折现率覆盖 (%)",
    "zh-TW": "折現率覆蓋 (%)",
    en: "Discount Rate Override (%)",
  },
  "reports.report_currency": {
    "zh-CN": "报表货币",
    "zh-TW": "報表貨幣",
    en: "Report Currency",
  },
  "reports.exchange_rate": {
    "zh-CN": "汇率",
    "zh-TW": "匯率",
    en: "Exchange Rate",
  },
  "reports.closing_liability": {
    "zh-CN": "期末负债合计",
    "zh-TW": "期末負債合計",
    en: "Total Closing Liability",
  },
  "reports.closing_rou": {
    "zh-CN": "期末使用权资产合计",
    "zh-TW": "期末使用權資產合計",
    en: "Total Closing ROU Asset",
  },
  "reports.total_interest": {
    "zh-CN": "期间利息合计",
    "zh-TW": "期間利息合計",
    en: "Total Interest",
  },
  "reports.total_depreciation": {
    "zh-CN": "期间折旧合计",
    "zh-TW": "期間折舊合計",
    en: "Total Depreciation",
  },
  "reports.tag_caveat": {
    "zh-CN": "同一合同可归入多个标签组，因此标签汇总总额可能大于总计汇总。",
    "zh-TW": "同一合同可歸入多個標籤組，因此標籤匯總總額可能大於總計匯總。",
    en: "A single contract may belong to multiple tag groups, so tag totals may exceed the overall summary.",
  },
  "reports.amortization_table": {
    "zh-CN": "摊销报表",
    "zh-TW": "攤銷報表",
    en: "Amortization Report",
  },
  "reports.contains_unapproved": {
    "zh-CN": "含未审批数据",
    "zh-TW": "含未審批數據",
    en: "Includes Unapproved Data",
  },
  "reports.official_only": {
    "zh-CN": "仅正式数据",
    "zh-TW": "僅正式數據",
    en: "Official Data Only",
  },
  "reports.no_data_hint": {
    "zh-CN": "请设置查询条件后点击「查询」",
    "zh-TW": "請設置查詢條件後點擊「查詢」",
    en: "Please set query conditions and click Search",
  },
  "reports.query_complete": {
    "zh-CN": "摊销报表查询完成，共 {count} 条",
    "zh-TW": "攤銷報表查詢完成，共 {count} 條",
    en: "Amortization report query complete, {count} rows",
  },
  "reports.query_failed": {
    "zh-CN": "摊销报表查询失败",
    "zh-TW": "攤銷報表查詢失敗",
    en: "Amortization report query failed",
  },
  "reports.please_select_dates": {
    "zh-CN": "请选择开始日期和结束日期",
    "zh-TW": "請選擇開始日期和結束日期",
    en: "Please select start and end dates",
  },

  // Status
  "status.draft": {
    "zh-CN": "草稿",
    "zh-TW": "草稿",
    en: "Draft",
  },
  "status.submitted": {
    "zh-CN": "已提交",
    "zh-TW": "已提交",
    en: "Submitted",
  },
  "status.reviewed": {
    "zh-CN": "已复核",
    "zh-TW": "已覆核",
    en: "Reviewed",
  },
  "status.pending_approval": {
    "zh-CN": "待审批",
    "zh-TW": "待審批",
    en: "Pending Approval",
  },
  "status.approved": {
    "zh-CN": "已审批",
    "zh-TW": "已審批",
    en: "Approved",
  },
  "status.rejected": {
    "zh-CN": "已驳回",
    "zh-TW": "已駁回",
    en: "Rejected",
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
