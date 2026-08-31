/* ============================================================================
   data.js — 演示数据
   ----------------------------------------------------------------------------
   字段名刻意对齐 CONTEXT.md 的领域语言（Store-Day Fact / Fact Coverage /
   Attention Signal / Source Envelope / Decision Ready），这样这套界面里的每
   一个标签都能追到一个真实存在的领域概念，而不是为了排版编出来的假指标。

   全部标注为 simulated —— 演示数据永远不冒充正式数据。
   ========================================================================== */

const DEMO = {};

/* ── 导航 ─────────────────────────────────────────────────────────── */

DEMO.nav = [
  {
    label: "经营分析",
    items: [
      { id: "home", href: "home.html", icon: "grid", text: "工作台" },
      { id: "pulse", href: "pulse.html", icon: "pulse", text: "经营脉搏", badge: "6" },
      { id: "store360", href: "store360.html", icon: "store", text: "门店 360" },
      { id: "scenario", href: "#", icon: "sliders", text: "情景工作台" },
      { id: "portfolio", href: "#", icon: "pie", text: "组合分析" },
    ],
  },
  {
    label: "日常工作",
    items: [
      { id: "tasks", href: "closing.html", icon: "inbox", text: "任务中心", badge: "12" },
      { id: "contracts", href: "contracts.html", icon: "file", text: "合同台账" },
      { id: "chat", href: "#", icon: "spark", text: "AI 助手" },
    ],
  },
  {
    label: "会计与合规",
    items: [
      { id: "closing", href: "closing.html", icon: "calc", text: "月度结账" },
      { id: "reports", href: "#", icon: "chart", text: "报表" },
      { id: "audit", href: "#", icon: "shield", text: "审计日志" },
    ],
  },
  {
    label: "设计系统",
    items: [
      { id: "tokens", href: "tokens.html", icon: "swatch", text: "设计令牌" },
      { id: "compare", href: "compare.html", icon: "columns", text: "A/B 并排" },
    ],
  },
];

/* ── 来源信封 · Source Envelope ───────────────────────────────────── */

DEMO.envelope = {
  classification: "simulated",
  dataset_version: "sim-2026.08-r3",
  generator_version: "retail-simgen-1.4.2",
  source_systems: ["retail_simulator"],
  as_of: "2026-08-16",
  window_days: 7,
  comparison: "2026-08-03 → 2026-08-09",
  current: "2026-08-10 → 2026-08-16",
  currency: "CNY",
  formula_version: "retail-kpi-v1",
  pulse_version: "pulse-v1.2",
  fact_version_range: "v1 – v3",
  observed_store_days: 271,
  expected_store_days: 308,
  decision_ready: false,
  not_ready_reason: "事实覆盖率 88.0% 低于阈值 95%，华南区 4 家门店 8/12–8/16 无回传",
};

DEMO.coverage_rate =
  (DEMO.envelope.observed_store_days / DEMO.envelope.expected_store_days) * 100;

/* ── 汇总指标 · Retail KPI Semantics ──────────────────────────────── */

DEMO.kpis = [
  {
    code: "revenue",
    label: "营业额",
    unit: "currency",
    current: 18426800,
    comparison: 19733400,
    change_pct: -6.62,
    good_when: "up",
    status: "complete",
  },
  {
    code: "store_contribution",
    label: "门店贡献",
    unit: "currency",
    current: 3142600,
    comparison: 3861900,
    change_pct: -18.63,
    good_when: "up",
    status: "complete",
  },
  {
    code: "gross_margin_rate",
    label: "毛利率",
    unit: "percent",
    current: 54.2,
    comparison: 55.8,
    change_pct: -1.6,
    good_when: "up",
    status: "complete",
  },
  {
    code: "sales_per_sqm",
    label: "坪效",
    unit: "currency_per_sqm",
    current: 428.6,
    comparison: 459.1,
    change_pct: -6.64,
    good_when: "up",
    status: "complete",
  },
  {
    code: "conversion_rate",
    label: "进店转化率",
    unit: "percent",
    current: null,
    comparison: 23.4,
    change_pct: null,
    good_when: "up",
    status: "partial",
    status_reason: "客流数据缺 4 店 5 天",
  },
  {
    code: "occupancy_cost_ratio",
    label: "租售比",
    unit: "percent",
    current: 17.8,
    comparison: 16.1,
    change_pct: 1.7,
    good_when: "down",
    status: "complete",
  },
];

DEMO.aux = [
  { code: "transactions", label: "交易笔数", current: 142860, comparison: 151030, unit: "count" },
  { code: "average_transaction_value", label: "客单价", current: 129.0, comparison: 130.7, unit: "currency" },
  { code: "labour_cost", label: "人工成本", current: 2214400, comparison: 2186700, unit: "currency" },
  { code: "fixed_rent", label: "固定租金", current: 2760000, comparison: 2760000, unit: "currency" },
  { code: "variable_rent", label: "抽成租金", current: 521400, comparison: 617200, unit: "currency" },
];

/* ── 日趋势 · 含真实数据缺口（null 不能用 0 冒充）────────────────── */

DEMO.trend = [
  { date: "08-03", revenue: 2795000, contribution: 566000 },
  { date: "08-04", revenue: 2681000, contribution: 541000 },
  { date: "08-05", revenue: 2740000, contribution: 552000 },
  { date: "08-06", revenue: 2988000, contribution: 601000 },
  { date: "08-07", revenue: 3216000, contribution: 648000 },
  { date: "08-08", revenue: 2702000, contribution: 519000 },
  { date: "08-09", revenue: 2611400, contribution: 434900 },
  { date: "08-10", revenue: 2648000, contribution: 498000 },
  { date: "08-11", revenue: 2591000, contribution: 470000 },
  { date: "08-12", revenue: null, contribution: null },
  { date: "08-13", revenue: null, contribution: null },
  { date: "08-14", revenue: 2704000, contribution: 452000 },
  { date: "08-15", revenue: 2812000, contribution: 461000 },
  { date: "08-16", revenue: 2478800, contribution: 398600 },
];

/* ── 关注排名 · Attention Ranking（服务端产生，前端不重新打分）──── */

DEMO.attention = [
  {
    rank: 1,
    store_id: "st-0412",
    store_code: "SH-0412",
    store_name: "上海静安嘉里中心店",
    brand: "MERIDIAN",
    region: "华东",
    currency: "CNY",
    severity: "critical",
    score: 8.64,
    source_systems: ["retail_simulator"],
    signals: [
      { code: "revenue_drop", label: "营业额下滑", change: -24.8, threshold: -10, unit: "percent" },
      { code: "conversion_drop", label: "转化率下滑", change: -6.2, threshold: -3, unit: "percent" },
    ],
  },
  {
    rank: 2,
    store_id: "st-0118",
    store_code: "GZ-0118",
    store_name: "广州天环汇店",
    brand: "MERIDIAN",
    region: "华南",
    currency: "CNY",
    severity: "critical",
    score: 7.91,
    source_systems: ["retail_simulator"],
    signals: [
      { code: "contribution_drop", label: "门店贡献下滑", change: -31.4, threshold: -12, unit: "percent" },
      { code: "occupancy_ratio_up", label: "租售比上升", change: 5.8, threshold: 3, unit: "percent" },
    ],
  },
  {
    rank: 3,
    store_id: "st-0733",
    store_code: "CD-0733",
    store_name: "成都太古里店",
    brand: "AURA",
    region: "西南",
    currency: "CNY",
    severity: "high",
    score: 6.42,
    source_systems: ["retail_simulator"],
    signals: [
      { code: "margin_drop", label: "毛利率下滑", change: -4.1, threshold: -2, unit: "percent" },
    ],
  },
  {
    rank: 4,
    store_id: "st-0257",
    store_code: "BJ-0257",
    store_name: "北京国贸商城店",
    brand: "MERIDIAN",
    region: "华北",
    currency: "CNY",
    severity: "high",
    score: 5.88,
    source_systems: ["retail_simulator"],
    signals: [
      { code: "labour_cost_up", label: "人工成本上升", change: 12.7, threshold: 8, unit: "percent" },
      { code: "revenue_drop", label: "营业额下滑", change: -11.3, threshold: -10, unit: "percent" },
    ],
  },
  {
    rank: 5,
    store_id: "st-0891",
    store_code: "HZ-0891",
    store_name: "杭州湖滨银泰店",
    brand: "AURA",
    region: "华东",
    currency: "CNY",
    severity: "medium",
    score: 4.13,
    source_systems: ["retail_simulator"],
    signals: [
      { code: "sales_per_sqm_drop", label: "坪效下滑", change: -8.9, threshold: -6, unit: "percent" },
    ],
  },
  {
    rank: 6,
    store_id: "st-0605",
    store_code: "SZ-0605",
    store_name: "深圳万象天地店",
    brand: "MERIDIAN",
    region: "华南",
    currency: "CNY",
    severity: "medium",
    score: 3.76,
    source_systems: ["retail_simulator"],
    signals: [
      { code: "variable_rent_up", label: "抽成租金上升", change: 4.2, threshold: 3, unit: "percent" },
    ],
  },
];

/* ── 被抑制的关注 · Suppressed Attention（证据不足，不是表现良好）── */

DEMO.suppressed = [
  { store_code: "GZ-0446", store_name: "广州正佳广场店", region: "华南", reason: "覆盖率不足", coverage: "3/7 store-days · 42.9%" },
  { store_code: "FS-0512", store_name: "佛山岭南天地店", region: "华南", reason: "覆盖率不足", coverage: "2/7 store-days · 28.6%" },
  { store_code: "DG-0388", store_name: "东莞民盈国贸店", region: "华南", reason: "来源冲突", coverage: "7/7 store-days · 100%" },
  { store_code: "ZH-0271", store_name: "珠海华发商都店", region: "华南", reason: "覆盖率不足", coverage: "4/7 store-days · 57.1%" },
];

/* ── 信号构成 · 哪一类原因在主导 ─────────────────────────────────── */

DEMO.signalMix = [
  { label: "营业额下滑", weight: 5.31, stores: 3 },
  { label: "门店贡献下滑", weight: 4.02, stores: 2 },
  { label: "租售比上升", weight: 2.44, stores: 2 },
  { label: "人工成本上升", weight: 1.87, stores: 1 },
  { label: "毛利率下滑", weight: 1.62, stores: 1 },
  { label: "转化率下滑", weight: 1.18, stores: 1 },
];

/* ── 门店 360 · Contribution Bridge（各项相加必须等于总变动）────── */

DEMO.store360 = {
  store_code: "SH-0412",
  store_name: "上海静安嘉里中心店",
  brand: "MERIDIAN",
  region: "华东",
  currency: "CNY",
  area_sqm: 286,
  opened: "2021-04-18",
  lease_end: "2027-03-31",
  severity: "critical",
  score: 8.64,
  kpis: [
    { label: "营业额", current: 1284600, comparison: 1708200, change_pct: -24.8, good_when: "up" },
    { label: "门店贡献", current: 142800, comparison: 318400, change_pct: -55.1, good_when: "up" },
    { label: "坪效", current: 4491, comparison: 5973, change_pct: -24.8, good_when: "up" },
    { label: "租售比", current: 24.6, comparison: 18.5, change_pct: 6.1, good_when: "down", unit: "percent" },
  ],
  bridge: [
    { label: "客流变化", value: -186400 },
    { label: "转化率变化", value: -142700 },
    { label: "客单价变化", value: -38200 },
    { label: "毛利率变化", value: -61300 },
    { label: "人工成本变化", value: -21400 },
    { label: "抽成租金变化", value: 34600 },
  ],
  // 分项之和是 −415,400，实测总变动是 −407,400：差额 +8,000 就是残差。
  // 这个数字是故意留着对不上的——界面必须能如实显示"没配平"这种状态。
  bridge_total: -407400,
  bridge_residual: 8000,
  peer: {
    cohort_size: 11,
    basis: "MERIDIAN · 华东 · CNY",
    metrics: [
      { label: "坪效", value: 4491, p25: 4880, median: 5610, p75: 6340, percentile: 9 },
      { label: "毛利率", value: 51.2, p25: 52.8, median: 55.1, p75: 57.4, percentile: 14 },
      { label: "租售比", value: 24.6, p25: 15.2, median: 17.8, p75: 20.1, percentile: 96 },
    ],
  },
  observations: [
    {
      text: "营业额下滑主要由客流而非客单价驱动：客流贡献 −18.64 万，客单价仅贡献 −3.82 万。",
      evidence: "对比 2026-08-03→08-09 与 08-10→08-16，事实数 14/14 store-days，来源 retail_simulator，口径 retail-kpi-v1",
    },
    {
      text: "租售比 24.6% 位于同类门店第 96 百分位，固定租金未变，分母恶化是唯一原因。",
      evidence: "同类样本 11 家（MERIDIAN · 华东 · CNY），固定租金 8 月无事件变更",
    },
  ],
};

/* ── 合同台账 · IFRS 16 ───────────────────────────────────────────── */

DEMO.contracts = [
  { id: "LC-2024-0188", store: "SH-0412 静安嘉里", lessor: "静安嘉里商业管理", start: "2021-04-18", end: "2027-03-31", ccy: "CNY", rate: 4.75, liability: 12846000, rou: 11982000, monthly: 286000, status: "posted", statusLabel: "已过账", critical: "2026-09-30 续约窗口" },
  { id: "LC-2024-0192", store: "BJ-0257 国贸商城", lessor: "中国国际贸易中心", start: "2022-01-01", end: "2028-12-31", ccy: "CNY", rate: 4.75, liability: 21470000, rou: 20114000, monthly: 342000, status: "posted", statusLabel: "已过账", critical: "—" },
  { id: "LC-2025-0031", store: "GZ-0118 天环汇", lessor: "广州天汇广场", start: "2023-06-01", end: "2029-05-31", ccy: "CNY", rate: 5.10, liability: 9328000, rou: 8871000, monthly: 178000, status: "approved", statusLabel: "待过账", critical: "2026-08-31 租金复审" },
  { id: "LC-2025-0047", store: "CD-0733 太古里", lessor: "成都远洋太古里", start: "2024-03-15", end: "2030-03-14", ccy: "CNY", rate: 5.10, liability: 14206000, rou: 13744000, monthly: 231000, status: "review", statusLabel: "待复核", critical: "—" },
  { id: "LC-2026-0004", store: "HK-0021 铜锣湾", lessor: "希慎興業", start: "2025-11-01", end: "2031-10-31", ccy: "HKD", rate: 5.85, liability: 33180000, rou: 32406000, monthly: 512000, status: "review", statusLabel: "待复核", critical: "2026-10-31 免租期结束" },
  { id: "LC-2026-0009", store: "SZ-0605 万象天地", lessor: "华润置地深圳", start: "2026-02-01", end: "2032-01-31", ccy: "CNY", rate: 5.35, liability: 18940000, rou: 18940000, monthly: 297000, status: "draft", statusLabel: "草稿", critical: "—" },
  { id: "LC-2026-0011", store: "HZ-0891 湖滨银泰", lessor: "杭州银泰百货", start: "2026-05-01", end: "2031-04-30", ccy: "CNY", rate: 5.35, liability: 11072000, rou: 11072000, monthly: 194000, status: "draft", statusLabel: "草稿", critical: "2026-09-15 起租确认" },
  { id: "LC-2026-0014", store: "HK-0033 尖沙咀", lessor: "新世界發展", start: "2026-07-01", end: "2032-06-30", ccy: "HKD", rate: 5.85, liability: 28640000, rou: 28640000, monthly: 448000, status: "draft", statusLabel: "草稿", critical: "—" },
];

DEMO.savedViews = [
  { id: "all", label: "全部合同", count: 8 },
  { id: "mine", label: "待我复核", count: 2 },
  { id: "critical", label: "90 天内关键日期", count: 4 },
  { id: "hkd", label: "港币合同", count: 2 },
];

/* ── 统一任务中心 · 跨模块收件箱 ─────────────────────────────────── */

DEMO.tasks = [
  { id: "t1", source: "合同审批", title: "LC-2026-0004 铜锣湾旗舰店 · 待复核", meta: "HKD 33,180,000 · 提交人 陈家豪 · 停留 2 天", severity: "high", sla: "今日到期", actions: ["批准", "退回"] },
  { id: "t2", source: "月度结账", title: "2026-08 期间 · 4 家门店未评估", meta: "阻断项 4 · 影响 华南区 · 评估于 09:12", severity: "critical", sla: "阻断结账", actions: ["查看", "指派"] },
  { id: "t3", source: "分录过账", title: "LC-2025-0031 折旧与利息分录 · 待过账", meta: "CNY 178,000 · 已批准 · 等待 1 天", severity: "medium", sla: "3 天内", actions: ["过账", "查看"] },
  { id: "t4", source: "关键日期", title: "LC-2024-0188 静安嘉里 · 续约窗口 45 天后关闭", meta: "2026-09-30 · 现租金 CNY 286,000/月", severity: "medium", sla: "45 天", actions: ["查看", "延后"] },
  { id: "t5", source: "AI 行动建议", title: "SH-0412 客流转化改善方案 · 待确认", meta: "预期收益 CNY 92,000/月 · 置信度 0.62", severity: "low", sla: "无期限", actions: ["采纳", "拒绝"], ai: true },
  { id: "t6", source: "事件复核", title: "GZ-0118 租金复审事件 · 待复核", meta: "CNY 178,000 → 191,000 · 生效 2026-09-01", severity: "high", sla: "2 天内", actions: ["复核", "查看"] },
];

/* ── 月结就绪度 · Evaluation Coverage ─────────────────────────────── */

DEMO.closing = {
  period: "2026-08",
  entity: "MERIDIAN 中国零售有限公司",
  status: "blocked",
  statusLabel: "存在阻断项",
  evaluated_at: "2026-08-17 09:12",
  population: 112,
  evaluated: 108,
  not_evaluated: [
    { subject: "GZ-0446 正佳广场", reason: "关联合同缺少付款计划", owner: "李思远" },
    { subject: "FS-0512 岭南天地", reason: "起租日未确认", owner: "李思远" },
    { subject: "DG-0388 民盈国贸", reason: "折现率待审批", owner: "王予彤" },
    { subject: "ZH-0271 华发商都", reason: "关联合同缺少付款计划", owner: "李思远" },
  ],
  steps: [
    { key: "prepare", label: "准备", state: "done" },
    { key: "calculate", label: "计量", state: "done" },
    { key: "review", label: "复核", state: "active" },
    { key: "post", label: "过账", state: "blocked" },
    { key: "writeback", label: "ERP 回写", state: "todo" },
    { key: "lock", label: "期间锁定", state: "todo" },
  ],
};

/* ── 首页简报 · AI 产出，必须与正式数据视觉可分 ─────────────────── */

DEMO.brief = {
  generated_at: "2026-08-17 08:05",
  confidence: 0.62,
  confidence_reason: "事实覆盖率 88.0% 低于阈值，华南区结论已排除",
  tools: [
    { name: "retail.operating_pulse", ms: 412, rows: 271 },
    { name: "retail.store_diagnostics", ms: 1180, rows: 14 },
    { name: "lease.critical_dates", ms: 96, rows: 8 },
  ],
  headline: "本周门店贡献下滑 18.6%，主因是客流而非定价",
  points: [
    "6 家门店进入关注排名，其中 2 家为严重级；华南区 4 家因覆盖率不足被抑制，不代表表现正常。",
    "SH-0412 静安嘉里贡献下滑 55.1%，Contribution Bridge 显示客流与转化合计解释了 79% 的降幅。",
    "租售比整体从 16.1% 升至 17.8%，固定租金未变——分母恶化是唯一原因。",
  ],
  proposal: {
    title: "对 SH-0412 启动客流转化改善方案",
    benefit: "预期门店贡献 +CNY 92,000/月",
    owner: "华东区运营 · 周敏",
    due: "2026-09-15",
    basis: "情景评估：转化率回到同类中位数 55.1% 的一半水平",
  },
};

/* ── 命令面板条目 ─────────────────────────────────────────────────── */

DEMO.commands = [
  { group: "跳转", icon: "grid", label: "工作台", sub: "Bento 首页", href: "home.html", keys: ["G", "H"] },
  { group: "跳转", icon: "pulse", label: "经营脉搏", sub: "关注排名 · 6 家门店", href: "pulse.html", keys: ["G", "P"] },
  { group: "跳转", icon: "store", label: "门店 360", sub: "SH-0412 静安嘉里", href: "store360.html", keys: ["G", "S"] },
  { group: "跳转", icon: "file", label: "合同台账", sub: "8 份合同", href: "contracts.html", keys: ["G", "C"] },
  { group: "跳转", icon: "inbox", label: "任务中心 / 月结", sub: "12 项待办 · 4 项阻断", href: "closing.html", keys: ["G", "T"] },
  { group: "跳转", icon: "swatch", label: "设计令牌", sub: "色阶 · 字阶 · 深度", href: "tokens.html" },
  { group: "跳转", icon: "columns", label: "A/B 并排对比", sub: "克制版 vs 高质感版", href: "compare.html" },

  { group: "操作", icon: "moon", label: "切换明暗主题", sub: "当前跟随手动选择", action: "toggle-theme", keys: ["⇧", "D"] },
  { group: "操作", icon: "swatch", label: "切换视觉风格", sub: "克制版 ⇄ 高质感版", action: "toggle-skin", keys: ["⇧", "S"] },
  { group: "操作", icon: "sliders", label: "侧栏收起 / 展开", action: "toggle-nav", keys: ["["] },

  { group: "筛选", icon: "filter", label: "只看严重级门店", sub: "severity = critical", action: "filter-critical" },
  { group: "筛选", icon: "filter", label: "对比窗口改为 28 天", sub: "window_days = 28", action: "filter-28d" },
  { group: "筛选", icon: "filter", label: "切到正式数据", sub: "classification = production", action: "filter-production" },

  { group: "门店", icon: "store", label: "SH-0412 上海静安嘉里中心店", sub: "MERIDIAN · 华东 · 严重", href: "store360.html" },
  { group: "门店", icon: "store", label: "GZ-0118 广州天环汇店", sub: "MERIDIAN · 华南 · 严重", href: "store360.html" },
  { group: "门店", icon: "store", label: "CD-0733 成都太古里店", sub: "AURA · 西南 · 高", href: "store360.html" },
  { group: "门店", icon: "store", label: "BJ-0257 北京国贸商城店", sub: "MERIDIAN · 华北 · 高", href: "store360.html" },

  { group: "合同", icon: "file", label: "LC-2024-0188 静安嘉里", sub: "CNY 12,846,000 · 已过账", href: "contracts.html" },
  { group: "合同", icon: "file", label: "LC-2026-0004 铜锣湾", sub: "HKD 33,180,000 · 待复核", href: "contracts.html" },
];

/* ── 格式化 ───────────────────────────────────────────────────────── */

DEMO.fmt = {
  /** 缺失值一律 —，绝不用 0 冒充 */
  nil: "—",

  money(value, ccy) {
    if (value == null) return DEMO.fmt.nil;
    const abs = Math.abs(value);
    if (abs >= 1e8) return `${(value / 1e8).toFixed(2)} 亿`;
    if (abs >= 1e4) return `${(value / 1e4).toFixed(1)} 万`;
    return value.toLocaleString("zh-CN", { maximumFractionDigits: 0 });
  },

  moneyFull(value) {
    if (value == null) return DEMO.fmt.nil;
    return value.toLocaleString("zh-CN", { maximumFractionDigits: 0 });
  },

  pct(value, digits = 1) {
    if (value == null) return DEMO.fmt.nil;
    return `${value.toFixed(digits)}%`;
  },

  count(value) {
    if (value == null) return DEMO.fmt.nil;
    return value.toLocaleString("zh-CN");
  },

  /** 带符号与箭头字符：颜色不是唯一信号 */
  delta(value, digits = 1) {
    if (value == null) return DEMO.fmt.nil;
    const arrow = value > 0 ? "↑" : value < 0 ? "↓" : "→";
    return `${arrow} ${Math.abs(value).toFixed(digits)}%`;
  },

  /** 变化的好坏取决于指标方向：租售比上升是坏事 */
  tone(value, goodWhen) {
    if (value == null || value === 0) return "flat";
    const isUp = value > 0;
    if (goodWhen === "down") return isUp ? "bad" : "good";
    return isUp ? "good" : "bad";
  },

  kpiValue(kpi) {
    if (kpi.current == null) return DEMO.fmt.nil;
    if (kpi.unit === "percent") return DEMO.fmt.pct(kpi.current);
    if (kpi.unit === "count") return DEMO.fmt.count(kpi.current);
    if (kpi.unit === "currency_per_sqm") return DEMO.fmt.moneyFull(Math.round(kpi.current));
    return DEMO.fmt.money(kpi.current);
  },

  kpiComparison(kpi) {
    if (kpi.comparison == null) return DEMO.fmt.nil;
    if (kpi.unit === "percent") return DEMO.fmt.pct(kpi.comparison);
    if (kpi.unit === "count") return DEMO.fmt.count(kpi.comparison);
    if (kpi.unit === "currency_per_sqm") return DEMO.fmt.moneyFull(Math.round(kpi.comparison));
    return DEMO.fmt.money(kpi.comparison);
  },
};
