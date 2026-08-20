package aiintake

// Contract ledger (batch) production ports producer.py:350-505, including the
// Excel deterministic fallback (a real xlsx drive through the source adapter
// fills material.DeterministicRecords) and the Excel evidence safety checks.

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func produceContractBatch(ctx context.Context, cmd IntakeCommandLike, material SourceMaterial, llm LLMCompleter) (map[string]any, error) {
	deterministic := false
	var fallbackWarning string
	parsed := map[string]any{}
	content, err := completeContent(ctx, llm,
		"你是一位专业的 IFRS 16 租赁合同台账解析专家。请准确提取每份合同字段。如果字段未在合同中出现，不要猜测，标记为缺失。",
		contractBatchPrompt(material.Text), 0.1, 8000,
		map[string]any{"type": "json_object"})
	switch {
	case err != nil:
		if len(material.DeterministicRecords) == 0 {
			return nil, fmt.Errorf("批量解析失败: %w", err)
		}
		deterministic = true
		fallbackWarning = "LLM 主解析暂不可用或返回异常，已启用 Excel 表格读取兜底；合同草稿必须人工逐条确认。原因: " + err.Error()
		parsed = map[string]any{"contracts": material.DeterministicRecords, "missing_fields": []any{}, "warnings": []any{}}
	case jsonUnmarshalOK(content, &parsed):
		if contracts, ok := parsed["contracts"]; !ok || len(asList(contracts)) == 0 {
			if len(material.DeterministicRecords) > 0 {
				deterministic = true
				fallbackWarning = "LLM 主解析未能从该 Excel 台账稳定提取合同，已启用表格读取兜底；这不是正式入库结果，必须人工逐条确认。"
				parsed = map[string]any{"contracts": material.DeterministicRecords, "missing_fields": []any{}, "warnings": []any{}}
			}
		}
	default:
		if len(material.DeterministicRecords) > 0 {
			deterministic = true
			fallbackWarning = "LLM 主解析暂不可用或返回异常，已启用 Excel 表格读取兜底；合同草稿必须人工逐条确认。"
			parsed = map[string]any{"contracts": material.DeterministicRecords, "missing_fields": []any{}, "warnings": []any{}}
		} else {
			return nil, fmt.Errorf("批量解析失败: 响应非 JSON")
		}
	}

	var contracts []map[string]any
	warnings := asStringList(parsed["warnings"])
	missing := asStringList(parsed["missing_fields"])
	if fallbackWarning != "" {
		warnings = append([]string{fallbackWarning}, warnings...)
	}
	isExcel := isExcelContentType(cmd.ContentType())
	rawContracts := asList(parsed["contracts"])
	for index, rawAny := range rawContracts {
		candidate := asMap(rawAny)
		if isExcel {
			candidate = applyExcelEvidenceSafetyChecks(candidate, material.Text)
		}
		if !hasNonEmpty(candidate, "contract_number") || !hasNonEmpty(candidate, "lessee") || !hasNonEmpty(candidate, "lessor") {
			if !deterministic {
				warnings = append(warnings, fmt.Sprintf("第 %d 份合同缺少必要字段 (contract_number/lessee/lessor)，已跳过", index+1))
				continue
			}
			if !hasNonEmpty(candidate, "lessee") {
				candidate["warnings"] = append(asStringList(candidate["warnings"]), fmt.Sprintf("第 %d 行缺少承租方/法人主体", index+1))
			}
			if !hasNonEmpty(candidate, "lessor") {
				candidate["warnings"] = append(asStringList(candidate["warnings"]), fmt.Sprintf("第 %d 行缺少出租方", index+1))
			}
		}
		discountMissing, discountWarnings := checkDiscountRateMissing(candidate)
		currencyMissing, currencyWarnings := checkCurrencyMissing(candidate)
		fieldMissing, fieldWarnings := checkCriticalFields(candidate)
		_, scopeWarnings := normalizeLeaseScope(candidate)
		confidence := sanitizeConfidence(candidate["confidence"])
		if len(fieldMissing) > 0 || discountMissing || currencyMissing {
			base := confidence
			if base == 0 {
				base = 0.9 // Python: `confidence or 0.9` — an absent 0.0 counts as missing
			}
			confidence = minFloat(base, 0.7)
		}
		if sanitizeConfidence(candidate["scope_confidence"]) < 0.8 {
			base := confidence
			if base == 0 {
				base = 0.9
			}
			confidence = minFloat(base, 0.7)
		}
		itemMissing := append([]string{}, fieldMissing...)
		if discountMissing {
			itemMissing = append(itemMissing, "discount_rate")
		}
		if currencyMissing {
			itemMissing = append(itemMissing, "currency")
		}
		itemMissing = sortedUnique(itemMissing)
		itemWarnings := append([]string{}, discountWarnings...)
		itemWarnings = append(itemWarnings, currencyWarnings...)
		itemWarnings = append(itemWarnings, fieldWarnings...)
		itemWarnings = append(itemWarnings, scopeWarnings...)
		itemWarnings = append(itemWarnings, asStringList(candidate["warnings"])...)
		normalized := normalizedContractRecord(candidate, "monthly")
		normalized["confidence"] = confidence
		normalized["missing_fields"] = ensureList(itemMissing)
		normalized["warnings"] = ensureList(itemWarnings)
		if deterministic {
			normalized["scope_source"] = "ledger"
		}
		contracts = append(contracts, normalized)
		warnings = append(warnings, itemWarnings...)
		missing = append(missing, itemMissing...)
	}

	// W5-3: the old path's pydantic model rejects negative amounts (a hard
	// ValidationError that fails the whole batch). Reproduce that fail-closed.
	if err := validateContractAmountNonNegative(contracts); err != nil {
		return nil, err
	}
	overall := sanitizeConfidence(parsed["overall_confidence"])
	if deterministic || overall <= 0 {
		sum := 0.0
		for _, c := range contracts {
			sum += c["confidence"].(float64)
		}
		if len(contracts) > 0 {
			overall = sum / float64(len(contracts))
		}
	}
	warnings = append(warnings, "AI Assist Mode: 合同台账草稿需人工逐条确认后入库")

	verified := resolveLLMEvidence(evidenceFromParsed(parsed), material.EvidenceLocators)
	var batchFields []string
	if !deterministic {
		for index := range contracts {
			rawCandidate := map[string]any{}
			if index < len(rawContracts) {
				rawCandidate = asMap(rawContracts[index])
			}
			for field, value := range rawCandidate {
				if field == "missing_fields" || field == "warnings" || field == "confidence" {
					continue
				}
				if value != nil && !isEmptyValue(value) {
					batchFields = append(batchFields, fmt.Sprintf("contracts[%d].%s", index, field))
				}
			}
		}
	}
	evidenceComplete := (deterministic && len(material.EvidenceLocators) > 0) || evidenceCoversFields(verified, batchFields)
	evidenceLocators := verified
	if deterministic {
		evidenceLocators = material.EvidenceLocators
	} else if len(evidenceLocators) == 0 {
		evidenceLocators = material.EvidenceLocators
	}
	missingReason := "field_locators_not_produced_by_llm_adapter"
	if !isExcel {
		missingReason = "field_locators_not_produced_by_document_adapter"
	}
	return batchEnvelope(cmd, contracts, map[string]float64{"overall": overall, "average_item": avgConfidence(contracts)},
		sortedUnique(missing), warnings, evidenceLocators, evidenceComplete, missingReason), nil
}

func avgConfidence(contracts []map[string]any) float64 {
	if len(contracts) == 0 {
		return 0
	}
	sum := 0.0
	for _, c := range contracts {
		if f, ok := c["confidence"].(float64); ok {
			sum += f
		}
	}
	return sum / float64(len(contracts))
}

func isEmptyValue(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(t) == ""
	case []any:
		return len(t) == 0
	case map[string]any:
		return len(t) == 0
	case float64:
		return t == 0
	}
	return false
}

func jsonUnmarshalOK(data string, out any) bool {
	return jsonUnmarshal([]byte(data), out) == nil
}

// validateNegativeAmount enforces the Python pydantic ge=0 contract for the
// batch rows: a negative fixed_rent_amount must hard-reject the batch, the
// same way ContractDraftData.model_validate fails on the old path.
func validateContractAmountNonNegative(contracts []map[string]any) error {
	for _, c := range contracts {
		if f, err := floatValue(c["fixed_rent_amount"]); err == nil && f < 0 {
			return fmt.Errorf("ValidationError: fixed_rent_amount must be >= 0 (got %v)", f)
		}
	}
	return nil
}

// applyExcelEvidenceSafetyChecks mirrors producer.py:1225-1250: a row whose
// source line carries a termination/renewal negation keyword flips the option
// to false — the model's claim cannot outweigh the document.
func applyExcelEvidenceSafetyChecks(contract map[string]any, fileContent string) map[string]any {
	contractNumber := strings.TrimSpace(toString(contract["contract_number"]))
	if contractNumber == "" || fileContent == "" {
		return contract
	}
	evidenceLine := ""
	for _, line := range strings.Split(fileContent, "\n") {
		if strings.Contains(line, contractNumber) {
			evidenceLine = line
			break
		}
	}
	if evidenceLine == "" {
		return contract
	}
	if strings.Contains(evidenceLine, "终止") {
		for _, keyword := range []string{"不行使", "未行使", "不会行使", "不合理确定"} {
			if strings.Contains(evidenceLine, keyword) {
				contract["termination_option"] = false
				break
			}
		}
	}
	if strings.Contains(evidenceLine, "续租") {
		for _, keyword := range []string{"不续租", "不行使续租", "未行使续租", "不会续租", "不合理确定续租"} {
			if strings.Contains(evidenceLine, keyword) {
				contract["renewal_option"] = false
				break
			}
		}
	}
	return contract
}

func isExcelContentType(contentType string) bool {
	switch contentType {
	case "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"application/vnd.ms-excel":
		return true
	}
	return false
}

func floatValue(v any) (float64, error) {
	switch t := v.(type) {
	case float64:
		return t, nil
	case string:
		return parseFloat(v), nil
	case int:
		return float64(t), nil
	default:
		return 0, fmt.Errorf("not a number")
	}
}

var _ = sort.Strings

func jsonUnmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }

func ensureList(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}
