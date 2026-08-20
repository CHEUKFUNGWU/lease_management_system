package aiintake

// This file is the Go production side of the intake producer (W5-3). It is a
// faithful port of ai-service/app/intake/producer.py normalization rules; the
// CORR-2 baseline (internal/agentseval/testdata/corr2) locked the old path's
// output and these functions must reproduce it. AI must not guess: missing
// critical fields, a missing currency and a missing discount rate are marked
// and surfaced to the human reviewer, never invented.

import (
	"fmt"
	"strconv"
	"strings"
)

// toString coerces any decoded JSON value to its wire string the way Python's
// str() would for the producer's scalar fields.
func toString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		if t {
			return "true"
		}
		return "false"
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(t), 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", t))
	}
}

// ---- float/int coercion helpers (mirror producer.py:1166-1203) ----

func parseFloat(v any) float64 {
	if v == nil {
		return 0.0
	}
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case bool:
		return 0.0
	}
	text := strings.TrimSpace(toString(v))
	text = strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(text, "¥"), "￥"), "元"), "%")
	text = strings.ReplaceAll(text, ",", "")
	if text == "" {
		return 0.0
	}
	low := strings.ToLower(text)
	for _, marker := range []string{"缺失", "待确认", "unknown", "null"} {
		if strings.Contains(low, marker) {
			return 0.0
		}
	}
	if f, err := strconv.ParseFloat(text, 64); err == nil {
		return f
	}
	return 0.0
}

func optionalFloat(v any) *float64 {
	if v == nil {
		return nil
	}
	if s, ok := v.(string); ok && s == "" {
		return nil
	}
	f, err := strconv.ParseFloat(toString(v), 64)
	if err != nil {
		return nil
	}
	return &f
}

func optionalText(v any) *string {
	if v == nil {
		return nil
	}
	text := strings.TrimSpace(toString(v))
	if text == "" {
		return nil
	}
	return &text
}

// optionalInt mirrors _optional_int: empty/zero/non-numeric map to nil.
func optionalInt(v any) *int {
	if v == nil {
		return nil
	}
	if s, ok := v.(string); ok && s == "" {
		return nil
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(toString(v)))
	if err != nil {
		return nil
	}
	if parsed <= 0 {
		return nil
	}
	return &parsed
}

func coerceBool(v any, def bool) bool {
	if v == nil {
		return def
	}
	if s, ok := v.(string); ok && s == "" {
		return def
	}
	switch t := v.(type) {
	case bool:
		return t
	case float64:
		return t != 0
	case int:
		return t != 0
	case int64:
		return t != 0
	}
	text := strings.ToLower(strings.TrimSpace(toString(v)))
	switch text {
	case "true", "yes", "y", "1", "是", "有":
		return true
	case "false", "no", "n", "0", "否", "无":
		return false
	}
	for _, marker := range []string{"不行使", "未行使", "不会行使", "不合理确定"} {
		if strings.Contains(text, marker) {
			return false
		}
	}
	for _, marker := range []string{"合理确定", "行使"} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return def
}

func asList(v any) []any {
	if v == nil {
		return nil
	}
	switch t := v.(type) {
	case []any:
		return t
	case []map[string]any:
		out := make([]any, 0, len(t))
		for _, m := range t {
			out = append(out, m)
		}
		return out
	case []string:
		out := make([]any, 0, len(t))
		for _, s := range t {
			out = append(out, s)
		}
		return out
	case []float64:
		out := make([]any, 0, len(t))
		for _, f := range t {
			out = append(out, f)
		}
		return out
	}
	return []any{v}
}

func asStringList(v any) []string {
	var out []string
	for _, item := range asList(v) {
		out = append(out, toString(item))
	}
	return out
}

// ---- confidence (producer.py:1090-1095) ----

func sanitizeConfidence(v any) float64 {
	f := parseFloat(v)
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}

func sanitizeConfidenceScores(scores map[string]any) map[string]float64 {
	out := make(map[string]float64, len(scores))
	for k, v := range scores {
		out[k] = sanitizeConfidence(v)
	}
	return out
}

// ---- critical rules (producer.py:1013-1087) ----

// checkDiscountRateMissing: AI 不得猜折现率。A contract carrying neither a
// discount_rate_type nor a discount_rate value must be flagged for human
// confirmation, never silently defaulted.
func checkDiscountRateMissing(extracted map[string]any) (bool, []string) {
	if hasNonEmpty(extracted, "discount_rate_type") || hasNonEmpty(extracted, "discount_rate") {
		return false, nil
	}
	return true, []string{
		"【关键】合同缺少折现率信息。AI 不得猜测折现率，需要人工确认。",
		"建议处理方式：",
		"1. 检查合同文本中是否明确提到利率",
		"2. 从系统政策库中查找适用的 IBR",
		"3. 按法人/租期区间/门店类型匹配利率政策",
		"4. 如无法唯一确定，请人工输入或选择",
	}
}

func checkCurrencyMissing(extracted map[string]any) (bool, []string) {
	if currency := strings.TrimSpace(toString(extracted["currency"])); currency != "" {
		low := strings.ToLower(currency)
		if low != "unknown" && low != "null" && low != "none" {
			return false, nil
		}
	}
	return true, []string{
		"【必须确认】AI 未识别到合同货币。根据 IFRS 16 计量要求，货币直接影响租赁负债现值计算和后续会计分录。",
		"请在上传后手动选择货币（CNY / USD / EUR 等）。AI 不会猜测货币。",
	}
}

var criticalFields = []struct {
	key   string
	label string
}{
	{"contract_number", "合同编号"},
	{"lessee", "承租方"},
	{"lessor", "出租方"},
	{"commencement_date", "租赁起始日"},
	{"lease_start_date", "租赁开始日"},
	{"lease_end_date", "租期结束日"},
	{"fixed_rent_amount", "固定租金金额"},
	{"payment_timing", "付款时点（先付/后付）"},
}

func checkCriticalFields(extracted map[string]any) ([]string, []string) {
	labels := make(map[string]string, len(criticalFields))
	for _, f := range criticalFields {
		labels[f.key] = f.label
	}
	var missing []string
	for _, f := range criticalFields {
		if !hasNonEmpty(extracted, f.key) {
			missing = append(missing, f.key)
		}
	}
	warnings := make([]string, 0, len(missing))
	for _, key := range missing {
		warnings = append(warnings, "【关键字段缺失】"+labels[key]+"("+key+") 未识别到，必须人工补充")
	}
	return missing, warnings
}

// normalizeLeaseScope is the scope gate that fronts the measurement engine. An
// invalid value defaults to in_scope for human confirmation; a low confidence
// forces a confirmation warning. AI never decides the scope silently.
func normalizeLeaseScope(extracted map[string]any) (string, []string) {
	var warnings []string
	scope := strings.TrimSpace(toString(extracted["suggested_scope"]))
	if scope == "" {
		scope = strings.TrimSpace(toString(extracted["lease_scope"]))
	}
	if scope == "" {
		scope = "in_scope"
	}
	allowed := map[string]bool{"in_scope": true, "short_term_exempt": true, "low_value_exempt": true, "not_a_lease": true}
	if !allowed[scope] {
		warnings = append(warnings, "【范围判定】AI 未能给出有效 lease_scope，默认按 in_scope 进入人工确认。")
		scope = "in_scope"
	}
	confidence := optionalFloat(extracted["scope_confidence"])
	if confidence != nil {
		c := sanitizeConfidence(*confidence)
		confidence = &c
	}
	if confidence == nil || *confidence < 0.8 {
		warnings = append(warnings, "【必须确认】租赁范围判定置信度不足，需要人工确认是否资本化、短期/低价值豁免或非租赁。")
	}
	extracted["lease_scope"] = scope
	extracted["suggested_scope"] = scope
	extracted["scope_source"] = "ai_suggested"
	if confidence != nil {
		extracted["scope_confidence"] = *confidence
	}
	return scope, warnings
}

// ---- value normalization (producer.py:1098-1163) ----

func normalizePaymentTiming(v any) string {
	switch strings.ToLower(strings.TrimSpace(toString(v))) {
	case "prepaid", "advance", "先付", "期初", "预付":
		return "prepaid"
	case "postpaid", "arrears", "后付", "期末":
		return "postpaid"
	}
	return ""
}

func normalizeAssetType(v any) string {
	text := strings.ToLower(strings.TrimSpace(toString(v)))
	for _, kw := range []string{"店", "铺", "物业", "房", "real"} {
		if strings.Contains(text, kw) {
			return "real_estate"
		}
	}
	for _, kw := range []string{"车", "vehicle"} {
		if strings.Contains(text, kw) {
			return "vehicle"
		}
	}
	for _, kw := range []string{"电脑", "it", "服务器", "设备"} {
		if strings.Contains(text, kw) {
			return "it_equipment"
		}
	}
	for _, kw := range []string{"机器", "machinery"} {
		if strings.Contains(text, kw) {
			return "machinery"
		}
	}
	if text == "" {
		return "real_estate"
	}
	return "other"
}

func normalizePaymentFrequency(v any, def string) string {
	switch strings.ToLower(strings.TrimSpace(toString(v))) {
	case "monthly", "quarterly", "yearly":
		return strings.ToLower(strings.TrimSpace(toString(v)))
	}
	return def
}

// normalizedContractRecord mirrors producer.py:_normalize_contract_record.
func normalizedContractRecord(candidate map[string]any, defaultPaymentFrequency string) map[string]any {
	scope := strings.TrimSpace(toString(candidate["lease_scope"]))
	if scope == "" {
		scope = strings.TrimSpace(toString(candidate["suggested_scope"]))
	}
	if scope == "" {
		scope = "in_scope"
	}
	return map[string]any{
		"contract_number":    strings.TrimSpace(toString(candidate["contract_number"])),
		"contract_name":      strings.TrimSpace(toString(candidate["contract_name"])),
		"lessee":             strings.TrimSpace(toString(candidate["lessee"])),
		"lessor":             strings.TrimSpace(toString(candidate["lessor"])),
		"store_name":         strings.TrimSpace(toString(candidate["store_name"])),
		"store_address":      strings.TrimSpace(toString(candidate["store_address"])),
		"commencement_date":  strings.TrimSpace(toString(candidate["commencement_date"])),
		"lease_start_date":   strings.TrimSpace(toString(candidate["lease_start_date"])),
		"lease_end_date":     strings.TrimSpace(toString(candidate["lease_end_date"])),
		"currency":           strings.TrimSpace(toString(candidate["currency"])),
		"asset_type":         normalizeAssetType(candidate["asset_type"]),
		"area_sqm":           parseFloat(candidate["area_sqm"]),
		"fixed_rent_amount":  parseFloat(candidate["fixed_rent_amount"]),
		"payment_frequency":  normalizePaymentFrequency(candidate["payment_frequency"], defaultPaymentFrequency),
		"payment_timing":     normalizePaymentTiming(candidate["payment_timing"]),
		"renewal_option":     coerceBool(candidate["renewal_option"], false),
		"termination_option": coerceBool(candidate["termination_option"], false),
		"cam_amount":         parseFloat(candidate["cam_amount"]),
		"service_fee":        parseFloat(candidate["service_fee"]),
		"discount_rate_type": strings.TrimSpace(toString(candidate["discount_rate_type"])),
		"discount_rate":      parseFloat(candidate["discount_rate"]),
		"is_lease":           coerceBool(candidate["is_lease"], scope != "not_a_lease"),
		"lease_scope":        scope,
		"suggested_scope":    strings.TrimSpace(toString(candidate["suggested_scope"])),
		"exemption_reason":   strings.TrimSpace(toString(candidate["exemption_reason"])),
		"scope_source":       "ai_suggested",
		"scope_confidence":   sanitizeConfidence(candidate["scope_confidence"]),
	}
}

func hasNonEmpty(m map[string]any, key string) bool {
	v, ok := m[key]
	if !ok {
		return false
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t) != ""
	case nil:
		return false
	case bool:
		return t
	default:
		return true
	}
}
