package template

// DefaultStorePnlTemplate is the factory template for /store-pnl (PRD
// appendix D). Actual cells are link rows over store-day facts and the IFRS
// 16 engine; budget/forecast cells bind the same rows to plan data at
// projection time. both口径 blocks share one structure — the basis labels do
// the isolation (T15).
func DefaultStorePnlTemplate() (*Template, error) {
	return Parse(TemplateDef{
		Name:    "单店利润表 · 出厂模板",
		Version: 1,
		Rows: []RowDef{
			{Key: "revenue", Label: "营业收入（净销售）", Kind: RowLink, Basis: BasisShared, Source: "fact.revenue"},
			{Key: "gross_profit", Label: "毛利", Kind: RowLink, Basis: BasisShared, Source: "fact.gross_profit"},
			{Key: "cogs", Label: "营业成本", Kind: RowFormula, Basis: BasisShared, Formula: "rows.revenue - rows.gross_profit"},
			{Key: "gross_profit_line", Label: "毛利（合计）", Kind: RowSubtotal, Basis: BasisShared, Children: []string{"revenue", "cogs"}, Subtract: []string{"cogs"}, Format: Format{Bold: true}},
			{Key: "labor_cost", Label: "门店直接人工", Kind: RowLink, Basis: BasisShared, Source: "fact.labor_cost"},
			{Key: "marketing", Label: "营销费用（门店级）", Kind: RowLink, Basis: BasisShared, Source: "assumption.marketing"},
			{Key: "non_lease_cost", Label: "非租赁物业费用", Kind: RowLink, Basis: BasisShared, Source: "fact.non_lease_cost"},
			{Key: "other_controllable", Label: "其他可控成本", Kind: RowLink, Basis: BasisShared, Source: "fact.other_controllable_cost"},
			{Key: "fixed_rent", Label: "其中：固定租金", Kind: RowLink, Basis: BasisOperating, Source: "fact.fixed_rent"},
			{Key: "service_fee", Label: "其中：服务费", Kind: RowLink, Basis: BasisOperating, Source: "contract.service_fee"},
			{Key: "variable_rent", Label: "其中：变量租金", Kind: RowLink, Basis: BasisOperating, Source: "fact.variable_rent"},
			{Key: "occupancy_cost", Label: "经营占用成本（Occupancy Cost）", Kind: RowSubtotal, Basis: BasisOperating, Children: []string{"fixed_rent", "service_fee", "variable_rent"}, Format: Format{Bold: true}},
			{Key: "store_contribution", Label: "门店贡献（Store Contribution）", Kind: RowSubtotal, Basis: BasisOperating, Children: []string{"gross_profit", "labor_cost", "marketing", "non_lease_cost", "other_controllable", "occupancy_cost"}, Subtract: []string{"labor_cost", "marketing", "non_lease_cost", "other_controllable", "occupancy_cost"}, Format: Format{Bold: true}},
			{Key: "allocation", Label: "分摊与总部费用", Kind: RowInput, Basis: BasisOperating},
			{Key: "four_wall_ebitda", Label: "四墙 EBITDA", Kind: RowLink, Basis: BasisOperating, Source: "fact.four_wall_ebitda"},
			{Key: "rou_depreciation", Label: "ROU 折旧", Kind: RowLink, Basis: BasisIFRS16, Source: "ifrs16.rou_depreciation"},
			{Key: "lease_interest", Label: "租赁负债利息", Kind: RowLink, Basis: BasisIFRS16, Source: "ifrs16.lease_interest"},
			{Key: "other_depreciation", Label: "其他折旧摊销", Kind: RowLink, Basis: BasisIFRS16, Source: "sched.other_depreciation"},
			{Key: "store_operating_profit", Label: "门店营业利润（IFRS 16 口径）", Kind: RowSubtotal, Basis: BasisIFRS16, Children: []string{"gross_profit", "labor_cost", "marketing", "non_lease_cost", "other_controllable", "rou_depreciation", "other_depreciation", "lease_interest"}, Subtract: []string{"labor_cost", "marketing", "non_lease_cost", "other_controllable", "rou_depreciation", "other_depreciation", "lease_interest"}, Format: Format{Bold: true}},
			{Key: "break_even_sales", Label: "盈亏平衡销售额", Kind: RowLink, Basis: BasisOperating, Source: "fact.break_even_sales"},
		},
	})
}

// DefaultStatementTemplate is the factory three-statement template (PRD
// S2-2 appendix A). The engine (SM2) evaluates it; tie-outs T1–T16 run on
// the evaluated result, not as template check rows.
func DefaultStatementTemplate() (*Template, error) {
	return Parse(DefaultStatementTemplateDef())
}

// DefaultStatementTemplateDef is the parseable declaration of the factory
// template — the single source both the in-memory engine template and the
// persisted seed (一键创建定义 / POST /financial-model/definitions) flow
// through. Name stays unique per legal entity: the create path versions it
// instead of overwriting.
func DefaultStatementTemplateDef() TemplateDef {
	return TemplateDef{
		Name:    "三表财务模型 · 出厂模板",
		Version: 1,
		Rows:    statementRows(),
	}
}

func statementRows() []RowDef {
	return []RowDef{
		// — 利润表 IS（shared 行双口径共用于收入侧；IFRS 16 口径行另列）—
		{Key: "rev", Label: "营业收入", Kind: RowLink, Basis: BasisShared, Source: "fact.revenue"},
		{Key: "gross_margin_rate", Label: "毛利率假设", Kind: RowInput, Basis: BasisShared},
		// 毛利在 Actual 冻结线左侧只读事实层（PRD C7）：actual_source 让引擎
		// 在该期间跳过公式、直接取 store-day 聚合毛利——真实 Actual 的 T13
		// 才能通过；预测期才用毛利率假设驱动。
		{Key: "gp", Label: "毛利", Kind: RowFormula, Basis: BasisShared, Formula: "rows.rev * rows.gross_margin_rate", ActualSource: "fact.gross_profit"},
		{Key: "labor", Label: "人工成本", Kind: RowLink, Basis: BasisShared, Source: "fact.labor_cost"},
		{Key: "fixed_rent", Label: "固定租金", Kind: RowLink, Basis: BasisOperating, Source: "fact.fixed_rent"},
		{Key: "variable_rent", Label: "变量租金", Kind: RowLink, Basis: BasisOperating, Source: "fact.variable_rent"},
		{Key: "non_lease", Label: "非租赁成本", Kind: RowLink, Basis: BasisShared, Source: "fact.non_lease_cost"},
		{Key: "other_ctrl", Label: "其他可控成本", Kind: RowLink, Basis: BasisShared, Source: "fact.other_controllable_cost"},
		{Key: "opex", Label: "经营费用合计", Kind: RowSubtotal, Basis: BasisOperating, Children: []string{"labor", "fixed_rent", "variable_rent", "non_lease", "other_ctrl"}, Format: Format{Bold: true}},
		{Key: "operating_ebitda", Label: "经营 EBITDA", Kind: RowFormula, Basis: BasisOperating, Formula: "rows.gp - rows.opex"},
		{Key: "dna_other", Label: "其他折旧摊销", Kind: RowLink, Basis: BasisIFRS16, Source: "sched.other_depreciation"},
		{Key: "rou_dep", Label: "ROU 折旧", Kind: RowLink, Basis: BasisIFRS16, Source: "ifrs16.rou_depreciation"},
		{Key: "lease_interest", Label: "租赁负债利息", Kind: RowLink, Basis: BasisIFRS16, Source: "ifrs16.lease_interest"},
		{Key: "borrow_interest_rate", Label: "借款利率假设", Kind: RowInput, Basis: BasisShared},
		{Key: "borrowings", Label: "借款余额", Kind: RowLink, Basis: BasisShared, Source: "sched.borrowings"},
		{Key: "borrow_interest", Label: "借款利息（期初余额法）", Kind: RowFormula, Basis: BasisShared, Formula: "rows.borrowings_opening * rows.borrow_interest_rate"},
		{Key: "borrowings_opening", Label: "期初借款余额", Kind: RowFormula, Basis: BasisShared, Formula: "lag(rows.borrowings, 1)"},
		{Key: "ebit", Label: "EBIT", Kind: RowFormula, Basis: BasisIFRS16, Formula: "rows.gp - rows.labor - rows.non_lease - rows.other_ctrl - rows.dna_other - rows.rou_dep"},
		{Key: "pretax", Label: "税前利润", Kind: RowFormula, Basis: BasisIFRS16, Formula: "rows.ebit - rows.lease_interest - rows.borrow_interest"},
		{Key: "tax_rate", Label: "有效税率假设", Kind: RowInput, Basis: BasisShared},
		{Key: "tax", Label: "所得税", Kind: RowFormula, Basis: BasisShared, Formula: "rows.pretax * rows.tax_rate"},
		{Key: "net_income", Label: "净利润", Kind: RowFormula, Basis: BasisShared, Formula: "rows.pretax - rows.tax"},
		// — 营运资本附表 —
		{Key: "dso", Label: "DSO 假设", Kind: RowInput, Basis: BasisShared},
		{Key: "dio", Label: "DIO 假设", Kind: RowInput, Basis: BasisShared},
		{Key: "dpo", Label: "DPO 假设", Kind: RowInput, Basis: BasisShared},
		{Key: "days", Label: "期间天数", Kind: RowInput, Basis: BasisShared},
		{Key: "ar", Label: "应收账款", Kind: RowFormula, Basis: BasisShared, Formula: "rows.dso * rows.rev / rows.days"},
		{Key: "cogs_line", Label: "营业成本", Kind: RowFormula, Basis: BasisShared, Formula: "rows.rev - rows.gp"},
		{Key: "inventory", Label: "存货", Kind: RowFormula, Basis: BasisShared, Formula: "rows.dio * rows.cogs_line / rows.days"},
		{Key: "ap", Label: "应付账款", Kind: RowFormula, Basis: BasisShared, Formula: "rows.dpo * rows.cogs_line / rows.days"},
		// — 长期资产附表 —
		{Key: "capex", Label: "CAPEX", Kind: RowLink, Basis: BasisShared, Source: "sched.capex"},
		{Key: "ppe", Label: "PP&E / 装修净值", Kind: RowFormula, Basis: BasisShared, Formula: "lag(rows.ppe, 1) + rows.capex - rows.dna_other"},
		// — 租赁附表（根据引擎只读投影，模型不自算，D-S3）—
		{Key: "rou_asset", Label: "使用权资产", Kind: RowLink, Basis: BasisShared, Source: "ifrs16.rou_asset"},
		{Key: "lease_liability", Label: "租赁负债", Kind: RowLink, Basis: BasisShared, Source: "ifrs16.lease_liability"},
		{Key: "lease_principal", Label: "租赁本金偿还", Kind: RowLink, Basis: BasisShared, Source: "ifrs16.lease_principal"},
		{Key: "lease_payments", Label: "租赁付款", Kind: RowLink, Basis: BasisShared, Source: "ifrs16.lease_payments"},
		// — 资产负债表 BS —
		{Key: "cash", Label: "货币资金", Kind: RowLink, Basis: BasisShared, Source: "cf.ending_cash"},
		{Key: "total_assets", Label: "资产合计", Kind: RowSubtotal, Basis: BasisShared, Children: []string{"cash", "ar", "inventory", "ppe", "rou_asset"}, Format: Format{Bold: true}},
		{Key: "total_liabilities", Label: "负债合计", Kind: RowSubtotal, Basis: BasisShared, Children: []string{"ap", "lease_liability", "borrowings"}, Format: Format{Bold: true}},
		{Key: "dividend_payout_rate", Label: "股利支付率假设", Kind: RowInput, Basis: BasisShared},
		{Key: "retained_earnings", Label: "留存收益", Kind: RowFormula, Basis: BasisShared, Formula: "lag(rows.retained_earnings, 1) + rows.net_income - rows.dividends"},
		{Key: "dividends", Label: "股利", Kind: RowFormula, Basis: BasisShared, Formula: "rows.net_income * rows.dividend_payout_rate"},
		{Key: "share_capital", Label: "实收资本", Kind: RowLink, Basis: BasisShared, Source: "sched.share_capital"},
		{Key: "total_equity", Label: "所有者权益合计", Kind: RowSubtotal, Basis: BasisShared, Children: []string{"share_capital", "retained_earnings"}, Format: Format{Bold: true}},
		// — 现金流量表 CF（间接法）—
		{Key: "dna", Label: "折旧摊销（含 ROU 折旧）", Kind: RowFormula, Basis: BasisShared, Formula: "rows.dna_other + rows.rou_dep"},
		{Key: "delta_nwc", Label: "营运资本变动（现金流出为正）", Kind: RowFormula, Basis: BasisShared, Formula: "lag(rows.nwc, 1) - rows.nwc"},
		{Key: "nwc", Label: "营运资本净额", Kind: RowFormula, Basis: BasisShared, Formula: "rows.ar + rows.inventory - rows.ap"},
		{Key: "cfo", Label: "经营活动现金流", Kind: RowFormula, Basis: BasisShared, Formula: "rows.net_income + rows.dna + rows.delta_nwc"},
		{Key: "cfi", Label: "投资活动现金流", Kind: RowFormula, Basis: BasisShared, Formula: "0 - rows.capex"},
		{Key: "borrowings_delta", Label: "借款净变动", Kind: RowFormula, Basis: BasisShared, Formula: "rows.borrowings - lag(rows.borrowings, 1)"},
		{Key: "cfe", Label: "筹资活动现金流", Kind: RowFormula, Basis: BasisShared, Formula: "rows.borrowings_delta - rows.lease_principal - rows.dividends"},
		{Key: "net_cash_flow", Label: "现金净变动", Kind: RowFormula, Basis: BasisShared, Formula: "rows.cfo + rows.cfi + rows.cfe"},
		{Key: "ending_cash", Label: "期末现金", Kind: RowFormula, Basis: BasisShared, Formula: "lag(rows.ending_cash, 1) + rows.net_cash_flow"},
	}
}
