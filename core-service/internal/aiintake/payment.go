package aiintake

// Payment schedule production ports producer.py:794-972: LLM content parsing
// (with a deterministic table fallback when the model does not answer), row
// validation with the exactly-skip rules, and the honor of never guessing a
// missing payment timing.

import (
	"strconv"
	"strings"
)

// parsePaymentContentString mirrors _parse_payment_llm_content exactly. A
// non-JSON answer degrades to an empty, flagged schedule list ("无法解析 LLM
// 输出") — the deterministic table fallback is reserved for LLM transport
// failures, mirroring the old path where json.loads failing inside the parser
// is swallowed.
func parsePaymentContentString(content string) map[string]any {
	cleaned := strings.TrimSpace(content)
	if strings.HasPrefix(cleaned, "```json") {
		cleaned = strings.TrimSpace(cleaned[7:])
	} else if strings.HasPrefix(cleaned, "```") {
		cleaned = strings.TrimSpace(cleaned[3:])
	}
	if strings.HasSuffix(cleaned, "```") {
		cleaned = strings.TrimSpace(cleaned[:len(cleaned)-3])
	}
	cleaned = strings.TrimSpace(cleaned)
	var parsed any
	if err := jsonUnmarshal([]byte(cleaned), &parsed); err == nil {
		switch v := parsed.(type) {
		case map[string]any:
			return v
		case []any:
			return map[string]any{
				"schedules":          v,
				"overall_confidence": 0.8,
				"missing_fields":     []any{},
				"warnings":           []any{},
				"total_schedules":    len(v),
			}
		}
	}
	start := strings.Index(cleaned, "[")
	end := strings.LastIndex(cleaned, "]")
	if start >= 0 && end > start {
		var schedules any
		if err := jsonUnmarshal([]byte(cleaned[start:end+1]), &schedules); err == nil {
			if list, ok := schedules.([]any); ok {
				return map[string]any{
					"schedules":          list,
					"overall_confidence": 0.7,
					"missing_fields":     []any{},
					"warnings":           []any{"JSON 解析使用了启发式提取"},
					"total_schedules":    len(list),
				}
			}
		}
	}
	return map[string]any{
		"schedules":          []any{},
		"overall_confidence": 0.0,
		"missing_fields":     []any{"all"},
		"warnings":           []any{"无法解析 LLM 输出"},
		"total_schedules":    0,
	}
}

// validatePaymentSchedules mirrors _validate_payment_schedules.
func validatePaymentSchedules(parsed map[string]any) ([]PaymentSchedule, []string, []string) {
	var validated []PaymentSchedule
	missingFields := asStringList(parsed["missing_fields"])
	warnings := asStringList(parsed["warnings"])
	rawSchedules := asList(parsed["schedules"])
	hasPostpaid := false
	hasPrepaid := false
	for index, scheduleAny := range rawSchedules {
		schedule := asMap(scheduleAny)
		dueDate := strings.TrimSpace(toString(schedule["due_date"]))
		amountAny := schedule["amount"]
		if dueDate == "" || isEmptyValue(amountAny) {
			warnings = append(warnings, "第 "+strconv.Itoa(index+1)+" 行缺少必要字段 (due_date/amount)，已跳过")
			continue
		}
		for _, field := range []string{"period_start", "period_end", "due_date"} {
			value := strings.TrimSpace(toString(schedule[field]))
			if value != "" && len([]rune(value)) != 10 {
				warnings = append(warnings, "第 "+strconv.Itoa(index+1)+" 行 "+field+" 日期格式可能不正确: "+value)
			}
		}
		amount, err := strconv.ParseFloat(strings.ReplaceAll(toString(amountAny), ",", ""), 64)
		if err != nil {
			warnings = append(warnings, "第 "+strconv.Itoa(index+1)+" 行金额无法解析为数字: "+toString(amountAny))
			continue
		}
		if amount <= 0 {
			warnings = append(warnings, "第 "+strconv.Itoa(index+1)+" 行金额 <= 0，已跳过")
			continue
		}
		timing := normalizePaymentTiming(schedule["payment_timing"])
		if timing == "" {
			warnings = append(warnings, "第 "+strconv.Itoa(index+1)+" 行缺少有效付款时点 (prepaid/postpaid)，已跳过")
			missingFields = append(missingFields, "payment_timing")
			continue
		}
		if timing == "prepaid" {
			hasPrepaid = true
		}
		if timing == "postpaid" {
			hasPostpaid = true
		}
		validated = append(validated, PaymentSchedule{
			PeriodStart:      unptrStringOrDefault(schedule["period_start"], dueDate),
			PeriodEnd:        unptrStringOrDefault(schedule["period_end"], dueDate),
			DueDate:          dueDate,
			Amount:           amount,
			PaymentTiming:    timing,
			IsFixed:          coerceBool(schedule["is_fixed"], true),
			IsLeaseComponent: coerceBool(schedule["is_lease_component"], true),
			AmountType:       toString(schedule["amount_type"]),
			Currency:         toString(schedule["currency"]),
			Confidence:       sanitizeConfidence(schedule["confidence"]),
		})
	}
	low := 0
	for _, item := range validated {
		if item.Confidence < 0.8 {
			low++
		}
	}
	if low > 0 {
		warnings = append(warnings, "有 "+strconv.Itoa(low)+" 笔付款的置信度低于 0.8，建议人工复核")
	}
	if hasPrepaid && hasPostpaid {
		warnings = append(warnings, "租金表中同时出现先付和后付，请确认是否正确")
	}
	return validated, missingFields, warnings
}

func unptrStringOrDefault(v any, def string) string {
	if s := strings.TrimSpace(toString(v)); s != "" {
		return s
	}
	return def
}

// headerAliases mirrors _normalize_header: aliases map Chinese merchant
// headers onto the canonical schedule field names.
var headerAliases = map[string]string{
	"覆盖期间起始日": "period_start",
	"期间开始":    "period_start",
	"覆盖期间结束日": "period_end",
	"期间结束":    "period_end",
	"应付日期":    "due_date",
	"应付日":     "due_date",
	"金额":      "amount",
	"付款时点":    "payment_timing",
	"金额类型":    "amount_type",
	"币种":      "currency",
	"成分":      "component",
	"固定租金":    "is_fixed",
	"租赁成分":    "is_lease_component",
}

func normalizeHeader(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(strings.ReplaceAll(normalized, " ", "_"), "-", "_")
	if alias, ok := headerAliases[normalized]; ok {
		return alias
	}
	return normalized
}

func truthyCell(value string, def bool) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "yes", "y", "1", "是", "租赁", "lease":
		return true
	case "false", "no", "n", "0", "否", "非租赁", "non_lease", "non-lease":
		return false
	}
	return def
}

// fallbackParsePaymentScheduleText mirrors _fallback_parse_payment_schedule_text:
// a deterministic markdown-table reader used when the LLM path failed. It is
// the honesty floor — never a fabricated schedule.
func fallbackParsePaymentScheduleText(fileContent, reason string) map[string]any {
	var rows [][]string
	for _, rawLine := range strings.Split(fileContent, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "###") {
			continue
		}
		// the python skip: set(line.replace('|','').replace('-','').strip()) == set()
		stripped := strings.ReplaceAll(strings.ReplaceAll(line, "|", ""), "-", "")
		if strings.TrimSpace(stripped) == "" {
			continue
		}
		if strings.Contains(line, "|") {
			cells := make([]string, 0)
			for _, cell := range strings.Split(line, "|") {
				cells = append(cells, strings.TrimSpace(cell))
			}
			if len(cells) >= 3 {
				rows = append(rows, cells)
			}
		}
	}
	if len(rows) == 0 {
		return map[string]any{
			"schedules":          []any{},
			"overall_confidence": 0.0,
			"missing_fields":     []any{"all"},
			"warnings":           []any{"LLM 解析不可用，且 Office 表格兜底未找到可读行: " + reason},
		}
	}
	header := make([]string, 0, len(rows[0]))
	for _, cell := range rows[0] {
		header = append(header, normalizeHeader(cell))
	}
	headerSet := make(map[string]bool, len(header))
	for _, h := range header {
		headerSet[h] = true
	}
	if !headerSet["due_date"] || !headerSet["amount"] {
		var missing []string
		for _, required := range []string{"due_date", "amount"} {
			if !headerSet[required] {
				missing = append(missing, required)
			}
		}
		return map[string]any{
			"schedules":          []any{},
			"overall_confidence": 0.0,
			"missing_fields":     missing,
			"warnings":           []any{"LLM 解析不可用，Office 表格兜底无法识别必要列: " + reason},
		}
	}
	dataRows := rows[1:]
	if len(rows) > 1 {
		allDash := true
		for _, cell := range rows[1] {
			if strings.TrimSpace(cell) != "" && strings.ReplaceAll(cell, "-", "") != "" {
				allDash = false
				break
			}
		}
		if allDash {
			dataRows = rows[2:]
		}
	}
	var schedules []any
	for _, cells := range dataRows {
		row := make(map[string]string, len(header))
		for index, h := range header {
			if index < len(cells) {
				row[h] = cells[index]
			} else {
				row[h] = ""
			}
		}
		if row["due_date"] == "" || row["amount"] == "" {
			continue
		}
		amount, err := strconv.ParseFloat(strings.ReplaceAll(row["amount"], ",", ""), 64)
		if err != nil {
			continue
		}
		amountType := row["amount_type"]
		if amountType == "" {
			amountType = "fixed_rent"
		}
		component := row["component"]
		schedules = append(schedules, map[string]any{
			"period_start":       firstNonEmpty(row["period_start"], row["due_date"]),
			"period_end":         firstNonEmpty(row["period_end"], row["due_date"]),
			"due_date":           row["due_date"],
			"amount":             amount,
			"payment_timing":     normalizePaymentTiming(row["payment_timing"]),
			"is_fixed":           truthyCell(row["is_fixed"], amountType != "turnover_rent" && amountType != "variable_rent"),
			"is_lease_component": truthyCell(row["is_lease_component"], component != "non_lease" && amountType != "cam" && amountType != "service_fee"),
			"amount_type":        amountType,
			"currency":           row["currency"],
			"confidence":         0.65,
		})
	}
	if len(schedules) > 0 {
		return map[string]any{
			"schedules":          schedules,
			"overall_confidence": 0.65,
			"missing_fields":     []any{},
			"warnings":           []any{"LLM 解析不可用，已使用 Office 表格兜底读取，必须人工复核: " + reason},
		}
	}
	return map[string]any{
		"schedules":          []any{},
		"overall_confidence": 0.0,
		"missing_fields":     []any{"all"},
		"warnings":           []any{"LLM 解析不可用，已使用 Office 表格兜底读取，必须人工复核: " + reason},
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
