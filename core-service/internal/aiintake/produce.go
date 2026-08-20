package aiintake

// The intake producer (W5-3): this is the production side that turns a source
// document + an LLM response into a versioned ai-intake.v1 draft envelope. It
// is a faithful port of ai-service/app/intake/producer.py's four _produce_*
// methods; the CORR-2 baseline (internal/agentseval/testdata/corr2) locks the
// exact normalized output, and the parity test in this package replays every
// recorded input through Go and must reproduce `expected` byte for byte.
//
// Assist Mode is the only supported mode: AI drafts everything, humans approve.
// Auto-Post Mode is rejected before any adapter runs.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

const sizedProductMode = "assist"

// LLMCompleter is the model seam the producer calls (W5-2 门 A: the parity test
// injects a fixed recorded response; the production path wires internal/llm).
type LLMCompleter interface {
	Complete(ctx context.Context, system, prompt string, temperature float64, maxTokens int, responseFormat map[string]any) (string, error)
}

// SourceMaterial is what a source adapter produced (producer.py SourceMaterial).
type SourceMaterial struct {
	Text                 string
	ContentType          string
	FileData             []byte
	DeterministicRecords []map[string]any
	EvidenceLocators     []EvidenceLocator
}

// Produce dispatches by kind and returns the draft envelope as a map so the
// exact wire shape (including area_sqm and each review reason) is preserved.
func Produce(ctx context.Context, mode string, kind string, command IntakeCommandLike, material SourceMaterial, llm LLMCompleter) (map[string]any, error) {
	if mode != sizedProductMode {
		return nil, fmt.Errorf("当前仅支持 Assist Mode。Auto-Post Mode 需另行配置")
	}
	input := command
	if input.MaxCharacters() <= 0 {
		input = withMaxCharacters(command, 15000)
	}
	if kind == "contract_batch" && input.MaxCharacters() <= 15000 {
		input = withMaxCharacters(command, 30000)
	}
	switch kind {
	case "contract":
		return produceContract(ctx, input, material, llm)
	case "payment_schedule":
		return producePayment(ctx, input, material, llm)
	case "contract_batch":
		return produceContractBatch(ctx, input, material, llm)
	case "event":
		return produceEvent(ctx, input, material, llm)
	default:
		return nil, fmt.Errorf("unsupported intake kind: %s", kind)
	}
}

// IntakeCommandLike abstracts the command envelope so both the HTTP wiring and
// tests can pass their own source identity.
type IntakeCommandLike interface {
	Kind() string
	FileID() string
	ObjectName() string
	ContentType() string
	ContractID() string
	MaxCharacters() int
}

type intakeCommand struct {
	kind          string
	fileID        string
	objectName    string
	contentType   string
	contractID    string
	maxCharacters int
}

func (c intakeCommand) Kind() string        { return c.kind }
func (c intakeCommand) FileID() string      { return c.fileID }
func (c intakeCommand) ObjectName() string  { return c.objectName }
func (c intakeCommand) ContentType() string { return c.contentType }
func (c intakeCommand) ContractID() string  { return c.contractID }
func (c intakeCommand) MaxCharacters() int  { return c.maxCharacters }

// Command builds an IntakeCommandLike for tests and the HTTP adapter.
func Command(kind, fileID, objectName, contentType, contractID string) IntakeCommandLike {
	max := 15000
	if kind == "contract_batch" {
		max = 30000
	}
	return intakeCommand{kind: kind, fileID: fileID, objectName: objectName, contentType: contentType, contractID: contractID, maxCharacters: max}
}

func withMaxCharacters(c IntakeCommandLike, n int) IntakeCommandLike {
	return intakeCommand{kind: c.Kind(), fileID: c.FileID(), objectName: c.ObjectName(), contentType: c.ContentType(), contractID: c.ContractID(), maxCharacters: n}
}

var errLLMUnavailable = errors.New("llm completer is required")

func completeContent(ctx context.Context, llm LLMCompleter, system, prompt string, temperature float64, maxTokens int, responseFormat map[string]any) (string, error) {
	if llm == nil {
		return "", errLLMUnavailable
	}
	return llm.Complete(ctx, system, prompt, temperature, maxTokens, responseFormat)
}

// ---- contract (producer.py:192-288) ----

func produceContract(ctx context.Context, cmd IntakeCommandLike, material SourceMaterial, llm LLMCompleter) (map[string]any, error) {
	parsed := map[string]any{}
	content, err := completeContent(ctx, llm,
		"你是一位专业的 IFRS 16 租赁合同解析专家。请准确提取合同字段。如果字段未在合同中出现，不要猜测，标记为缺失。",
		contractPrompt(material.Text), 0.1, 2500,
		map[string]any{"type": "json_object"})
	if err != nil {
		parsed = map[string]any{
			"extracted_fields":   map[string]any{},
			"confidence_scores":  map[string]any{},
			"overall_confidence": 0.5,
			"missing_fields":     []any{},
			"warnings":           []any{"解析响应格式异常，请人工检查"},
		}
	} else if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		parsed = map[string]any{
			"extracted_fields":   map[string]any{},
			"confidence_scores":  map[string]any{},
			"overall_confidence": 0.5,
			"missing_fields":     []any{},
			"warnings":           []any{"解析响应格式异常，请人工检查"},
		}
	}

	extracted := asMap(parsed["extracted_fields"])
	confidence := sanitizeConfidenceScores(asMap(parsed["confidence_scores"]))
	confidence["overall"] = sanitizeConfidence(parsed["overall_confidence"])

	discountMissing, discountWarnings := checkDiscountRateMissing(extracted)
	currencyMissing, currencyWarnings := checkCurrencyMissing(extracted)
	missingFields, fieldWarnings := checkCriticalFields(extracted)
	_, scopeWarnings := normalizeLeaseScope(extracted)
	extracted["asset_type"] = normalizeAssetType(extracted["asset_type"])
	extracted["area_sqm"] = parseFloat(extracted["area_sqm"])
	if !hasNonEmpty(extracted, "payment_timing") || asString(extracted["payment_timing"]) == "" {
	} else {
		extracted["payment_timing"] = normalizePaymentTiming(extracted["payment_timing"])
	}

	warnings := append([]string{}, discountWarnings...)
	warnings = append(warnings, currencyWarnings...)
	warnings = append(warnings, fieldWarnings...)
	warnings = append(warnings, scopeWarnings...)
	warnings = append(warnings, asStringList(parsed["warnings"])...)
	warnings = append(warnings, "AI Assist Mode: 合同草稿需人工确认后入库")

	missing := append([]string{}, missingFields...)
	missing = append(missing, asStringList(parsed["missing_fields"])...)
	if discountMissing {
		missing = append(missing, "discount_rate")
	}
	if currencyMissing {
		missing = append(missing, "currency")
	}
	missing = sortedUnique(missing)

	normalized := normalizedContractRecord(extracted, "")
	confidenceForContract := confidence["overall"]
	if raw, ok := extracted["confidence"]; ok && raw != nil {
		confidenceForContract = sanitizeConfidence(raw)
	}
	normalized["confidence"] = confidenceForContract
	normalized["missing_fields"] = ensureList(missing)
	normalized["warnings"] = ensureList(warnings)

	verified := resolveLLMEvidence(evidenceFromParsed(parsed), material.EvidenceLocators)
	evidenceFields := contractEvidenceFields(extracted)
	evidenceComplete := evidenceCoversFields(verified, evidenceFields)
	missingReason := "field_mapping_not_verified_by_llm"
	if len(material.EvidenceLocators) == 0 {
		missingReason = "field_locators_not_produced_by_document_adapter"
	}

	return contractEnvelope(cmd, normalized, confidence, missing, warnings, verified, material.EvidenceLocators, evidenceComplete, missingReason), nil
}

// ---- payment (producer.py:290-348) ----

func producePayment(ctx context.Context, cmd IntakeCommandLike, material SourceMaterial, llm LLMCompleter) (map[string]any, error) {
	var parsed map[string]any
	content, err := completeContent(ctx, llm,
		"你是一位专业的 IFRS 16 租金表解析专家。请准确提取付款计划信息，严格遵守日期格式和金额格式要求。",
		paymentPrompt(material.Text), 0.1, 4000, nil)
	if err != nil {
		// Transport failure: degrade to the deterministic table reader.
		reason := fmt.Sprintf("%T: %v", err, err)
		parsed = fallbackParsePaymentScheduleText(material.Text, reason)
	} else {
		// A non-JSON answer degrades to a flagged empty schedule list, exactly
		// like the old path (the table fallback is not reached for garbage text).
		parsed = parsePaymentContentString(content)
	}

	schedules, missingFields, warnings := validatePaymentSchedules(parsed)
	overall := sanitizeConfidence(parsed["overall_confidence"])
	verified := resolveLLMEvidence(evidenceFromParsed(parsed), material.EvidenceLocators)
	rawSchedules, _ := parsed["schedules"].([]any)
	evidenceFields := paymentEvidenceFields(rawSchedules, schedules)
	evidenceComplete := evidenceCoversFields(verified, evidenceFields)
	missingReason := "field_mapping_not_verified_by_llm"
	if len(material.EvidenceLocators) == 0 {
		missingReason = "field_locators_not_produced_by_document_adapter"
	}

	avg := 0.0
	if len(schedules) > 0 {
		sum := 0.0
		for _, s := range schedules {
			sum += s.Confidence
		}
		avg = sum / float64(len(schedules))
	}
	var allWarnings []string
	allWarnings = append(allWarnings, warnings...)
	allWarnings = append(allWarnings, "AI Assist Mode: 付款计划草稿需人工确认后入库")

	return paymentEnvelope(cmd, schedules, map[string]float64{"overall": overall, "average_item": avg},
		sortedUnique(missingFields), allWarnings, verified, material.EvidenceLocators, evidenceComplete, missingReason), nil
}

// ---- event (producer.py:90-190) ----

var eventWarnings = []string{
	"AI Assist Mode: 事件草稿必须由人工确认后提交复核",
	"事件解析不会决定最终会计处理，也不会自动触发重算或过账",
	"扫描件字段定位当前只能回溯到提取文本，需人工核对原文页码/坐标",
}

var allowedEventTypes = map[string]bool{
	"modification": true, "reassessment": true, "impairment": true, "early_termination": true,
	"renewal": true, "area_adjustment": true, "rent_change": true, "index_update": true,
	"discount_rate_change": true,
}

func produceEvent(ctx context.Context, cmd IntakeCommandLike, material SourceMaterial, llm LLMCompleter) (map[string]any, error) {
	var parsed map[string]any
	content, err := completeContent(ctx, llm,
		"你是一位 IFRS 16 合同事件识别助手。只提取原文明确出现的事实，不要猜测事件类型、日期、金额、租期或会计处理；无法确定就留空并列入 missing_fields。输出 JSON，不要输出解释性文字。",
		eventPrompt(material.Text, cmd.ContractID()), 0.1, 3000,
		map[string]any{"type": "json_object"})
	eventFailure := ""
	if err != nil {
		eventFailure = fmt.Sprintf("%T: %v", err, err)
		parsed = map[string]any{}
	} else if uerr := json.Unmarshal([]byte(content), &parsed); uerr != nil {
		eventFailure = "JSONDecodeError"
		parsed = map[string]any{}
	}

	warnings := append([]string{}, eventWarnings...)
	if eventFailure != "" {
		warnings = append(warnings, "事件文档 LLM 提取失败，已生成空草稿供人工转录: "+eventFailure)
	}
	raw := asMap(parsed["event"])
	if raw == nil {
		raw = parsed
	}
	fieldConfidence := asMapFloat(raw["field_confidence"])
	event := EventDraftData{
		ContractID:         toString(raw["contract_id"]),
		ContractNumber:     toString(raw["contract_number"]),
		EventType:          toString(raw["event_type"]),
		EffectiveDate:      toString(raw["effective_date"]),
		OriginalValue:      optionalText(raw["original_value"]),
		NewValue:           optionalText(raw["new_value"]),
		ChangeReason:       toString(raw["change_reason"]),
		JudgmentBasis:      toString(raw["judgment_basis"]),
		RevisionParameters: asMap(raw["revision_parameters"]),
		FieldConfidence:    fieldConfidence,
	}
	if event.ContractID == "" && cmd.ContractID() != "" {
		event.ContractID = cmd.ContractID()
	}
	missing := asStringList(parsed["missing_fields"])
	requiredValues := map[string]string{
		"contract_id": event.ContractID, "event_type": event.EventType, "effective_date": event.EffectiveDate,
		"change_reason": event.ChangeReason, "judgment_basis": event.JudgmentBasis,
	}
	for field, value := range requiredValues {
		if value == "" {
			missing = append(missing, field)
		}
	}
	if event.EventType != "" && !allowedEventTypes[event.EventType] {
		missing = append(missing, "event_type_review")
		warnings = append(warnings, "识别到未标准化事件类型，必须人工分类")
	}
	warnings = append(warnings, asStringList(parsed["warnings"])...)
	if len(event.RevisionParameters) == 0 {
		warnings = append(warnings, "未提取到结构化变更参数；若事件需要重算，请人工补录原值/新值和参数")
	}
	overall := sanitizeConfidence(parsed["overall_confidence"])
	if len(missing) > 0 {
		overall = minFloat(overall, 0.7)
	}
	verified := resolveLLMEvidence(evidenceFromParsed(parsed), material.EvidenceLocators)
	evidenceFields := eventEvidenceFields(event)
	evidenceComplete := evidenceCoversFields(verified, evidenceFields)
	missingReason := "field_mapping_not_verified_by_llm"
	if len(material.EvidenceLocators) == 0 {
		missingReason = "document_adapter_does_not_yet_provide_page_or_coordinate_locators"
	}
	confidenceScores := map[string]float64{"overall": overall}
	for k, v := range event.FieldConfidence {
		confidenceScores[k] = v
	}
	return eventEnvelope(cmd, event, confidenceScores, sortedUnique(missing), warnings, verified, material.EvidenceLocators, evidenceComplete, missingReason), nil
}

// ---- helpers ----

func asMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func asMapFloat(v any) map[string]float64 {
	out := make(map[string]float64)
	for k, val := range asMap(v) {
		out[k] = sanitizeConfidence(val)
	}
	return out
}

func asString(v any) string { return toString(v) }

func evidenceFromParsed(parsed map[string]any) any {
	if evidence, ok := parsed["evidence"]; ok {
		return evidence
	}
	return parsed["evidence_refs"]
}

func sortedUnique(in []string) []string {
	seen := make(map[string]bool, len(in))
	var out []string
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func contractEvidenceFields(extracted map[string]any) []string {
	var fields []string
	for field, value := range extracted {
		if value == nil {
			continue
		}
		switch t := value.(type) {
		case string:
			if strings.TrimSpace(t) == "" {
				continue
			}
		case []any:
			if len(t) == 0 {
				continue
			}
		case map[string]any:
			if len(t) == 0 {
				continue
			}
		}
		fields = append(fields, "extracted_data."+field)
	}
	return fields
}

func paymentEvidenceFields(rawSchedules []any, schedules []PaymentSchedule) []string {
	if len(rawSchedules) != len(schedules) {
		return nil
	}
	var fields []string
	for i, schedule := range schedules {
		values := map[string]any{
			"period_start":   unptrString(schedule.PeriodStart),
			"period_end":     unptrString(schedule.PeriodEnd),
			"due_date":       unptrString(schedule.DueDate),
			"amount":         schedule.Amount,
			"payment_timing": unptrString(schedule.PaymentTiming),
			"currency":       unptrString(schedule.Currency),
		}
		for field, value := range values {
			switch t := value.(type) {
			case string:
				if t != "" {
					fields = append(fields, fmt.Sprintf("schedules[%d].%s", i, field))
				}
			case float64:
				if t != 0 {
					fields = append(fields, fmt.Sprintf("schedules[%d].%s", i, field))
				}
			}
		}
	}
	return fields
}

func unptrString(s string) string { return s }

func eventEvidenceFields(event EventDraftData) []string {
	values := map[string]any{
		"event.event_type":          event.EventType,
		"event.effective_date":      event.EffectiveDate,
		"event.original_value":      event.OriginalValue,
		"event.new_value":           event.NewValue,
		"event.change_reason":       event.ChangeReason,
		"event.judgment_basis":      event.JudgmentBasis,
		"event.revision_parameters": event.RevisionParameters,
	}
	var fields []string
	for field, value := range values {
		skip := false
		switch t := value.(type) {
		case nil:
			skip = true
		case string:
			skip = t == ""
		case map[string]any:
			skip = len(t) == 0
		case *string:
			skip = t == nil || *t == ""
		case map[string]any2:
			skip = len(t) == 0
		}
		if !skip {
			fields = append(fields, field)
		}
	}
	return fields
}

// any2 is only for type-switch completeness; it is never matched at runtime.
type any2 = map[string]interface{}
