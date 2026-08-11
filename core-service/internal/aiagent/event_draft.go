package aiagent

import (
	"regexp"
	"strings"
	"time"

	"github.com/lease-management-system/core-service/internal/agentartifact"
)

var eventDatePattern = regexp.MustCompile(`20[0-9]{2}[-/年][0-9]{1,2}[-/月][0-9]{1,2}日?`)

// extractEventDraft is deliberately deterministic. It only creates an event
// draft when the user supplied a contract context, an explicit event intent,
// a supported event type and a complete effective date. It never invents a
// date, amount, revised lease term or accounting treatment.
func extractEventDraft(message, contractID string, sources []Source) (*EventDraftData, []agentartifact.EvidenceReference) {
	message = strings.TrimSpace(message)
	contractID = strings.TrimSpace(contractID)
	if message == "" || contractID == "" || !isEventDraftIntent(message) {
		return nil, nil
	}
	eventType := eventTypeFromMessage(message)
	effectiveDate := eventDateFromMessage(message)
	if eventType == "" || effectiveDate == "" {
		return nil, nil
	}
	if len([]rune(message)) > 1000 {
		message = string([]rune(message)[:1000])
	}
	evidenceRefs := evidenceReferencesFromSources(sources)
	return &EventDraftData{
		ContractID:    contractID,
		EventType:     eventType,
		EffectiveDate: effectiveDate,
		ChangeReason:  message,
		JudgmentBasis: "由用户指令提取，需人工复核事件分类、原值/新值和会计处理",
	}, evidenceRefs
}

func isEventDraftIntent(message string) bool {
	message = strings.ToLower(strings.TrimSpace(message))
	return strings.Contains(message, "事件草稿") || strings.Contains(message, "登记事件") ||
		strings.Contains(message, "创建合同事件") || strings.Contains(message, "录入 modification") ||
		strings.Contains(message, "录入 reassessment") || strings.Contains(message, "登记 modification") ||
		strings.Contains(message, "登记 reassessment") || strings.Contains(message, "创建 modification") ||
		strings.Contains(message, "event draft") ||
		strings.Contains(message, "请登记") || strings.Contains(message, "请创建事件")
}

func eventTypeFromMessage(message string) string {
	message = strings.ToLower(strings.TrimSpace(message))
	switch {
	case strings.Contains(message, "impairment"), strings.Contains(message, "减值"):
		return "impairment"
	case strings.Contains(message, "discount_rate_change"), strings.Contains(message, "折现率变更"), strings.Contains(message, "利率变更"):
		return "discount_rate_change"
	case strings.Contains(message, "index_update"), strings.Contains(message, "指数更新"), strings.Contains(message, "cpi"):
		return "index_update"
	case strings.Contains(message, "early_termination"), strings.Contains(message, "提前终止"), strings.Contains(message, "闭店"):
		return "early_termination"
	case strings.Contains(message, "renewal"), strings.Contains(message, "续租"):
		return "renewal"
	case strings.Contains(message, "reassessment"), strings.Contains(message, "重估"), strings.Contains(message, "重新评估"):
		return "reassessment"
	case strings.Contains(message, "area_adjustment"), strings.Contains(message, "面积调整"):
		return "area_adjustment"
	case strings.Contains(message, "rent_change"), strings.Contains(message, "租金变更"), strings.Contains(message, "租金调整"):
		return "rent_change"
	case strings.Contains(message, "modification"), strings.Contains(message, "合同修改"), strings.Contains(message, "合同变更"):
		return "modification"
	default:
		return ""
	}
}

func eventDateFromMessage(message string) string {
	match := eventDatePattern.FindString(message)
	if match == "" {
		return ""
	}
	normalized := strings.NewReplacer("年", "-", "月", "-", "日", "", "/", "-").Replace(match)
	parsed, err := time.Parse("2006-1-2", normalized)
	if err != nil {
		parsed, err = time.Parse("2006-01-02", normalized)
	}
	if err != nil {
		return ""
	}
	return parsed.Format("2006-01-02")
}
