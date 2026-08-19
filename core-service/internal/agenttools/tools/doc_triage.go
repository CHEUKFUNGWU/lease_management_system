package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/lease-management-system/core-service/internal/agenttools"
)

// DocClass is the triage vocabulary from the working-paper design §6.1.
type DocClass string

const (
	DocLeaseContract      DocClass = "lease_contract"
	DocRentSchedule       DocClass = "rent_schedule"
	DocAmendment          DocClass = "amendment"
	DocContractLedger     DocClass = "contract_ledger"
	DocOperatingData      DocClass = "operating_data"
	DocFinancialStatement DocClass = "financial_statement"
	DocInvoice            DocClass = "invoice"
	DocMeetingMinutes     DocClass = "meeting_minutes"
	DocUnknown            DocClass = "unknown"
)

// TriageRequest is the triage input: metadata only. Document content is data,
// never instructions — the classifier sees names and the user's message, not
// extracted text.
type TriageRequest struct {
	FileID      string `json:"file_id"`
	ObjectName  string `json:"object_name"`
	ContentType string `json:"content_type"`
	UserMessage string `json:"user_message"`
}

// TriageResult is the triage output. Unknown or low-confidence results must
// stop the pipeline and ask the user — never fall through to a default.
type TriageResult struct {
	DocClass         DocClass       `json:"doc_class"`
	Confidence       float64        `json:"confidence"`
	DetectedEntities map[string]any `json:"detected_entities,omitempty"`
	Reason           string         `json:"reason"`
	// Candidates is populated for unknown results so the UI can offer a
	// choice instead of a bare refusal.
	Candidates []DocClass `json:"candidates,omitempty"`
}

// TriageThreshold is the confidence below which the pipeline must ask the
// user (working-paper design §6.1).
const TriageThreshold = 0.6

// TriageClassifier is the seam between the LLM classifier and the
// deterministic one. Both implementations must share the same failure
// semantics: unknown is a legitimate answer, never a fallback to contract.
type TriageClassifier interface {
	Classify(ctx context.Context, req TriageRequest) (TriageResult, error)
}

// DeterministicTriage classifies from keyword evidence in the file name and
// user message. It is the honest fallback: no keyword match resolves to
// DocUnknown with candidates, never to lease_contract.
func DeterministicTriage(req TriageRequest) TriageResult {
	text := strings.ToLower(req.ObjectName + " " + req.UserMessage)
	// Known out-of-domain files resolve to unknown up front — the generic
	// 合同 keyword must never capture a labor contract (CORR-6's core
	// failure mode).
	if containsAnyText(text, []string{"劳动合同", "劳务合同", "labor contract", "employment contract", "宣传册", "brochure", "培训材料", "简历"}) {
		return unknownResult()
	}
	type rule struct {
		class DocClass
		conf  float64
		terms []string
	}
	rules := []rule{
		{DocRentSchedule, 0.85, []string{"租金表", "付款计划", "付款表", "rent schedule", "payment schedule", "rental schedule"}},
		{DocContractLedger, 0.85, []string{"台账", "批量创建", "批量录入", "contract ledger", "批量导入合同", "导入台账"}},
		{DocAmendment, 0.8, []string{"补充协议", "变更协议", "amendment", "修改协议", "终止协议", "续签协议"}},
		{DocInvoice, 0.9, []string{"发票", "invoice", "增值税发票"}},
		{DocFinancialStatement, 0.85, []string{"财务报表", "利润表", "资产负债表", "现金流量表", "financial statement", "income statement", "balance sheet"}},
		{DocMeetingMinutes, 0.85, []string{"会议纪要", "meeting minutes", "会议记录"}},
		{DocOperatingData, 0.8, []string{"经营数据", "门店销售", "销售数据", "门店数据", "operating data", "store sales", "p&l", "客流"}},
		{DocLeaseContract, 0.8, []string{"租赁合同", "租赁协议", "lease agreement", "lease contract", "合同扫描件", "合同"}},
	}
	for _, r := range rules {
		if containsAnyText(text, r.terms) {
			return TriageResult{
				DocClass:   r.class,
				Confidence: r.conf,
				Reason:     "文件名/用户消息命中 " + string(r.class) + " 关键词",
			}
		}
	}
	return unknownResult()
}

func unknownResult() TriageResult {
	return TriageResult{
		DocClass:   DocUnknown,
		Confidence: 0,
		Reason:     "无法确定文件类型，请从候选中选择",
		Candidates: []DocClass{DocLeaseContract, DocRentSchedule, DocContractLedger, DocOperatingData, DocFinancialStatement, DocInvoice, DocMeetingMinutes, DocAmendment},
	}
}

func containsAnyText(text string, terms []string) bool {
	for _, t := range terms {
		if strings.Contains(text, t) {
			return true
		}
	}
	return false
}

// DocTriageArguments is the strict input schema of the triage tool.
type DocTriageArguments struct {
	FileID      string `json:"file_id"`
	ObjectName  string `json:"object_name"`
	ContentType string `json:"content_type"`
	UserMessage string `json:"user_message"`
}

// NewDocTriageDefinition registers the triage tool. It is LevelRead: it
// changes nothing and may be called by the agent freely.
func NewDocTriageDefinition(classifier TriageClassifier) agenttools.ToolDefinition {
	if classifier == nil {
		classifier = deterministicClassifier{}
	}
	return agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name:        "lease.file.triage",
			Version:     "v1",
			DisplayName: "文件分诊",
			Description: "判断上传文件的业务类型（合同/租金表/台账/经营数据/发票等）。识别不了时返回 unknown 与候选，绝不默认按合同处理。",
			Level:       agenttools.LevelRead,
			ReadOnly:    true,
			InputSchema: json.RawMessage(`{
				"type": "object",
				"required": ["file_id", "object_name", "content_type"],
				"properties": {
					"file_id": {"type": "string"},
					"object_name": {"type": "string"},
					"content_type": {"type": "string"},
					"user_message": {"type": "string"}
				}
			}`),
		},
		Handler: func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
			var args DocTriageArguments
			dec := json.NewDecoder(strings.NewReader(string(call.Arguments)))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&args); err != nil {
				return agenttools.ToolResult{}, errors.New("invalid triage arguments")
			}
			if args.FileID == "" || args.ObjectName == "" || args.ContentType == "" {
				return agenttools.ToolResult{}, errors.New("file_id, object_name and content_type are required")
			}
			result, err := classifier.Classify(ctx, TriageRequest{
				FileID:      args.FileID,
				ObjectName:  args.ObjectName,
				ContentType: args.ContentType,
				UserMessage: args.UserMessage,
			})
			if err != nil {
				return agenttools.ToolResult{}, err
			}
			return agenttools.ToolResult{
				CallID: call.CallID,
				Status: agenttools.StatusCompleted,
				Data:   result,
			}, nil
		},
	}
}

type deterministicClassifier struct{}

func (deterministicClassifier) Classify(_ context.Context, req TriageRequest) (TriageResult, error) {
	return DeterministicTriage(req), nil
}
