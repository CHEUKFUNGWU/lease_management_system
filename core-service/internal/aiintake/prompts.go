package aiintake

import "fmt"

// contractPrompt mirrors the old Python prompt template. Rendered verbatim for the
// CORR-2 golden prompt gate (门 B); any wording change must update the golden
// file and be justified in the PR.
func contractPrompt(fileContent string) string {
	return fmt.Sprintf(`
    你是一位专业的 IFRS 16 租赁合同解析专家。请从以下合同文本中提取关键字段。

    【重要规则 — 必须遵守】
    1. 如果合同中未明确提到折现率/利率，不要猜测，标记为缺失
    2. 如果合同中未明确提到货币（币种），不要猜测，标记为缺失或 unknown
    3. 区分先付租金(prepaid)和后付租金(postpaid)
    4. 区分固定租金和变量租金
    5. 识别租赁成分和非租赁成分(CAM、服务费)
    6. 承租方(lessee)是租赁合同的乙方，即使用物业并支付租金的一方
    7. 出租方(lessor)是租赁合同的甲方，即提供物业并收取租金的一方
    8. 必须做 IFRS 16 范围初判：是否存在已识别资产、承租方是否控制使用、租期是否 ≤12 个月、是否低价值资产。AI 只能建议，不能直接入正式账。

    合同文本:
    %s

    请提取以下字段（JSON 格式）:
    - contract_number: 合同编号
    - contract_name: 合同名称
    - lessee: 承租方名称（合同中承租人/乙方对应的完整公司名称）
    - lessor: 出租方名称（合同中出租人/甲方对应的完整公司名称）
    - store_name: 门店/物业名称（如有明确提及，否则留空）
    - store_address: 门店/物业地址（如有明确提及，否则留空）
    - commencement_date: 租赁起始日 (YYYY-MM-DD)
    - lease_start_date: 租赁开始日 (YYYY-MM-DD)
    - lease_end_date: 租期结束日 (YYYY-MM-DD)
    - currency: 币种 (CNY/USD/EUR)。如果合同中没有明确提到货币，返回 null 或空字符串，不要猜测
    - asset_type: 标的资产类型 "real_estate" | "vehicle" | "it_equipment" | "machinery" | "other"
    - area_sqm: 租赁面积（平方米，仅数字）。合同未写明面积时返回 0 或 null，不要估算
    - fixed_rent_amount: 固定租金金额（仅数字，不含货币单位）
    - payment_frequency: 付款频率 (monthly/quarterly/yearly)
    - payment_timing: 付款时点 (prepaid/postpaid)。如果合同写明"每月X日前支付"则为prepaid；"每月X日后支付"或"月末支付"则为postpaid
    - renewal_option: 是否有续租选择权 (true/false)
    - termination_option: 是否有终止选择权 (true/false)
    - cam_amount: 物业管理费 (如有，仅数字)
    - service_fee: 服务费 (如有，仅数字)
    - discount_rate_type: 折现率类型 (如合同中提及)
    - discount_rate: 折现率数值 (如合同中提及)
    - is_lease: 是否构成 IFRS 16 租赁 (true/false)
    - suggested_scope: "in_scope" | "short_term_exempt" | "low_value_exempt" | "not_a_lease"
    - exemption_reason: 范围判定依据，如"租期 10 个月且无续租意图"、"未识别特定资产"
    - scope_confidence: 范围判定置信度 (0-1)

    请以 JSON 格式输出，包含:
    - extracted_fields: 提取的字段
    - confidence_scores: 每个字段的置信度 (0-1)
    - overall_confidence: 总体置信度
    - missing_fields: 识别为缺失的字段列表
    - warnings: 需要人工注意的问题列表
    - evidence: 字段级原文定位数组。每项必须包含 field、page（如有）、quote；只能引用上面文本中明确出现的短语，不得自行编造坐标。
    `, fileContent)
}

// paymentPrompt mirrors the old Python prompt template. Rendered verbatim for the
// CORR-2 golden prompt gate (门 B); any wording change must update the golden
// file and be justified in the PR.
func paymentPrompt(fileContent string) string {
	return fmt.Sprintf(`
你是一位专业的 IFRS 16 租金表解析专家。请从以下租金表内容中提取付款计划信息。

【重要规则 - 必须遵守】
1. 每笔付款必须识别：先付(prepaid)还是后付(postpaid)
   - 先付：在覆盖期间开始前支付（如月初预付当月租金）
   - 后付：在覆盖期间结束后支付（如月末支付当月租金）
2. 区分固定租金和变量租金（turnover rent / sales-based rent 必须标记为变量）
3. 区分租赁成分和非租赁成分（CAM、管理费、服务费等）
4. 金额必须是数字，不要包含货币符号
5. 日期格式必须为 YYYY-MM-DD
6. 如果租金表是月度数据，期间起始日=当月1日，期间结束日=当月最后一日
7. 如果某期金额为空或为0，跳过该行

租金表内容:
%s

请提取每笔付款，以 JSON 数组格式输出。每个元素包含:
- period_start: 覆盖期间起始日 (YYYY-MM-DD)
- period_end: 覆盖期间结束日 (YYYY-MM-DD)
- due_date: 应付日期 (YYYY-MM-DD)
- amount: 金额 (纯数字)
- payment_timing: "prepaid" 或 "postpaid"
- is_fixed: true/false
- is_lease_component: true/false
- amount_type: "fixed_rent" | "turnover_rent" | "cam" | "service_fee" | "tax" | "deposit" | "other"
- currency: "CNY" 或文件中出现的币种。如果文件中未明确提到货币，返回 null 或空字符串，不要猜测
- confidence: 该笔识别的置信度 (0.0-1.0)

额外输出字段（JSON 对象顶层）:
- overall_confidence: 总体置信度 (0.0-1.0)
- missing_fields: 识别中遇到问题的字段列表
- warnings: 需要人工注意的问题列表（如：日期格式不确定、金额可能有误等）
- total_schedules: 识别到的付款笔数
- evidence: 字段级原文定位数组。每项包含 field（如 schedules[0].amount）、page（如有）和 quote；只能引用原文。

请以纯 JSON 格式输出，不要包含任何 markdown 代码块标记。
`, fileContent)
}

// eventPrompt mirrors the old Python prompt template. Rendered verbatim for the
// CORR-2 golden prompt gate (门 B); any wording change must update the golden
// file and be justified in the PR.
func eventPrompt(fileContent, contractID string) string {
	return fmt.Sprintf(`
你是一位 IFRS 16 合同事件识别助手。请从以下合同变更通知、补充协议、闭店通知或扫描件文本中提取“事件草稿”。

硬性规则：
1. 只提取原文明确出现的事实；缺失或有歧义的字段必须留空并加入 missing_fields。
2. 不要猜测最终会计处理、折现率、租期、金额、是否合理确定或是否需要重算。
3. modification（合同范围/对价改变）与 reassessment（选择权判断变化）必须按原文证据区分；不确定时 event_type 留空。
4. 事件草稿只能进入人工复核，不得表示已批准、已重算或已过账。
5. 日期统一为 YYYY-MM-DD；金额或原值/新值保留原文语义，可放入 original_value/new_value。
6. revision_parameters 只填原文明确的结构化变化，例如 new_area_sqm、new_monthly_rent、new_lease_end_date、index_name、index_value；不明确就返回空对象。

当前合同上下文 ID（仅作绑定提示，不代表文档证据）：%s

文档文本：
%s

请只返回 JSON：
{
  "event": {
    "contract_id": "",
    "contract_number": "",
    "event_type": "modification|reassessment|impairment|early_termination|renewal|area_adjustment|rent_change|index_update|discount_rate_change",
    "effective_date": "YYYY-MM-DD",
    "original_value": "",
    "new_value": "",
    "change_reason": "",
    "judgment_basis": "仅引用或概括原文依据，不给出最终会计结论",
    "revision_parameters": {},
    "field_confidence": {"event_type": 0.0, "effective_date": 0.0, "change_reason": 0.0}
  },
  "overall_confidence": 0.0,
  "missing_fields": [],
  "warnings": []
  ,"evidence": [
    {"field": "event.event_type", "page": 1, "quote": "原文短语"}
  ]
}
`, contractID, fileContent)
}

// contractBatchPrompt mirrors the old Python prompt template. Rendered verbatim for the
// CORR-2 golden prompt gate (门 B); any wording change must update the golden
// file and be justified in the PR.
func contractBatchPrompt(fileContent string) string {
	return fmt.Sprintf(`
    你是一位专业的 IFRS 16 租赁合同台账解析专家。请从以下合同台账内容中提取每一份合同的字段。

    【重要规则 — 必须遵守】
    1. 台账中可能包含多份合同，请逐条提取
    2. 如果合同中未明确提到折现率/利率，不要猜测，标记为缺失
    3. 如果合同中未明确提到货币（币种），不要猜测，标记为缺失或 unknown
    4. 区分先付租金(prepaid)和后付租金(postpaid)
    5. 区分固定租金和变量租金
    6. 识别租赁成分和非租赁成分(CAM、服务费)
    7. 承租方(lessee)是租赁合同的乙方，即使用物业并支付租金的一方
    8. 出租方(lessor)是租赁合同的甲方，即提供物业并收取租金的一方
    9. 必须做 IFRS 16 范围初判：是否存在已识别资产、承租方是否控制使用、租期是否 ≤12 个月、是否低价值资产。AI 只能建议，不能直接入正式账。
    10. 如果内容来自 Excel，台账可能是非标准排版、多 sheet、多行标题、合并单元格展开后的文本；请按语义理解 sheet 名、标题行、相邻单元格和字段含义，不要依赖固定列名或固定顺序。
    11. 如果出现"法人主体"、"租赁主体"、"承租公司"等字段，通常可作为 lessee/承租方；但仍需结合上下文判断。
    12. 续租/终止选择权必须按否定语义优先判断："不行使"、"未行使"、"不会行使"、"不合理确定"、"无" 均为 false。不要因为文本出现"终止选择权"或"续租选择权"几个字就返回 true。

    合同台账内容:
    %s

    请以 JSON 格式输出，包含以下顶层字段:
    - contracts: 合同列表，每个元素包含:
      - contract_number: 合同编号
      - contract_name: 合同名称
      - lessee: 承租方名称
      - lessor: 出租方名称
      - store_name: 门店/物业名称（如有）
      - store_address: 门店/物业地址（如有）
      - commencement_date: 租赁起始日 (YYYY-MM-DD)
      - lease_start_date: 租赁开始日 (YYYY-MM-DD)
      - lease_end_date: 租期结束日 (YYYY-MM-DD)
      - currency: 币种 (CNY/USD/EUR)。如果未明确提到，返回 null 或空字符串，不要猜测
      - asset_type: 标的资产类型 "real_estate" | "vehicle" | "it_equipment" | "machinery" | "other"
      - area_sqm: 租赁面积（平方米，仅数字）。未写明时返回 0 或 null，不要估算
      - fixed_rent_amount: 固定租金金额（仅数字）
      - payment_frequency: 付款频率 (monthly/quarterly/yearly)
      - payment_timing: 付款时点 (prepaid/postpaid)
      - renewal_option: 是否有续租选择权 (true/false)
      - termination_option: 是否有终止选择权 (true/false)
      - cam_amount: 物业管理费 (如有，仅数字)
      - service_fee: 服务费 (如有，仅数字)
      - discount_rate_type: 折现率类型 (如合同中提及)
      - discount_rate: 折现率数值 (如合同中提及)
      - is_lease: 是否构成 IFRS 16 租赁 (true/false)
      - suggested_scope: "in_scope" | "short_term_exempt" | "low_value_exempt" | "not_a_lease"
      - exemption_reason: 范围判定依据，如"租期 10 个月且无续租意图"、"未识别特定资产"
      - scope_confidence: 范围判定置信度 (0-1)
      - confidence: 该份合同识别的置信度 (0-1)
      - missing_fields: 该份合同缺失的字段列表
      - warnings: 该份合同的警告列表
    - total_count: 识别到的合同总数
    - overall_confidence: 总体置信度 (0-1)
    - missing_fields: 全局缺失字段汇总
    - warnings: 全局警告列表
    - evidence: 字段级原文定位数组，每项包含 field（如 contracts[0].contract_number）、page（如有）和 quote；只能引用原文。
    `, fileContent)
}
