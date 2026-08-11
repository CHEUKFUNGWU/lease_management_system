package aiagent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/lease-management-system/core-service/internal/agenttools"
)

// withAIServiceAuth keeps the downstream AI Service credential in the
// authenticated server context. It is intentionally not part of ToolCall and
// is never included in Tool Execution audit fields.
func withAIServiceAuth(ctx context.Context, authHeader string) context.Context {
	return agenttools.WithDelegationCredential(ctx, authHeader)
}

func aiServiceAuthFromContext(ctx context.Context) string {
	return agenttools.DelegationCredentialFromContext(ctx)
}

type fileParseArguments struct {
	FileID      string `json:"file_id"`
	ObjectName  string `json:"object_name"`
	ContentType string `json:"content_type"`
	ContractID  string `json:"contract_id,omitempty"`
}

func (h *Agent) fileParseDefinitions() []agenttools.ToolDefinition {
	return []agenttools.ToolDefinition{
		{
			Descriptor: fileParseDescriptor(
				"lease.file.parse_contract",
				"解析单份合同文件",
				"调用 AI Service 对单份合同文件进行 OCR/文本抽取并生成带证据的合同草稿",
				false,
			),
			SkillIDs: []string{"contract_review"},
			Handler:  h.parseContractToolHandler,
		},
		{
			Descriptor: fileParseDescriptor(
				"lease.file.parse_contract_batch",
				"解析合同台账文件",
				"调用 AI Service 对 Excel/PDF 合同台账进行批量抽取并生成合同草稿",
				false,
			),
			SkillIDs: []string{"contract_review", "excel_ledger", "contract_batch_intake"},
			Handler:  h.parseContractBatchToolHandler,
		},
		{
			Descriptor: fileParseDescriptor(
				"lease.file.parse_payment_schedule",
				"解析租金表文件",
				"调用 AI Service 对租金表进行抽取并生成付款计划草稿，不直接导入正式台账",
				true,
			),
			SkillIDs: []string{"payment_schedule", "payment_schedule_intake"},
			Handler:  h.parsePaymentScheduleToolHandler,
		},
		{
			Descriptor: fileParseDescriptor(
				"lease.file.parse_event",
				"解析合同事件文件",
				"调用 AI Service 对补充协议、闭店通知或扫描件提取结构化事件草稿，保留人工复核闸门",
				true,
			),
			SkillIDs: []string{"event_change"},
			Handler:  h.parseEventToolHandler,
		},
	}
}

func fileParseDescriptor(name, displayName, description string, includeContractID bool) agenttools.ToolDescriptor {
	properties := `"file_id":{"type":"string","minLength":1},"object_name":{"type":"string","minLength":1},"content_type":{"type":"string","minLength":1}`
	if includeContractID {
		properties += `,"contract_id":{"type":"string"}`
	}
	return agenttools.ToolDescriptor{
		Name:        name,
		Version:     "v1",
		DisplayName: displayName,
		Description: description,
		Level:       agenttools.LevelDraft,
		ReadOnly:    false,
		Permissions: []agenttools.Permission{{Resource: "ai_chat", Action: "use"}},
		InputSchema: json.RawMessage(fmt.Sprintf(`{
  "type":"object",
  "additionalProperties":false,
  "required":["file_id","object_name","content_type"],
  "properties":{%s}
}`, properties)),
		OutputSchema: json.RawMessage(`{
  "type":"object",
  "required":["draft_type","review_required","source_file_id"],
  "properties":{"draft_type":{"type":"string"},"review_required":{"type":"boolean"},"source_file_id":{"type":"string"}}
}`),
		Review: agenttools.ReviewPolicy{
			Required:      true,
			Reasons:       []string{"AI parsing produces a draft and requires human confirmation"},
			ConfirmAction: "review_ai_draft",
		},
		Retry:               agenttools.RetryPolicy{Retryable: true, MaxAttempts: 2},
		SupportsDryRun:      true,
		SupportsIdempotency: true,
		TimeoutSeconds:      180,
	}
}

func (h *Agent) parseContractToolHandler(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
	args, err := decodeFileParseArguments(call.Arguments, false)
	if err != nil {
		return rejectedFileParse(call.CallID, err), nil
	}
	draft, err := h.parseFile(ctx, aiServiceAuthFromContext(ctx), args.FileID, args.ObjectName, args.ContentType)
	if err != nil {
		return agenttools.ToolResult{}, err
	}
	return agenttools.ToolResult{
		CallID: call.CallID, Status: agenttools.StatusCompleted, Data: draft,
		Sources: toolSourcesFromAgentSources([]Source{{Type: "file", ID: draft.Evidence.SourceFileID, Title: "合同文件", Snippet: draft.Evidence.ObjectName}}),
	}, nil
}

func (h *Agent) parseContractBatchToolHandler(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
	args, err := decodeFileParseArguments(call.Arguments, false)
	if err != nil {
		return rejectedFileParse(call.CallID, err), nil
	}
	result, err := h.parseContractBatch(ctx, aiServiceAuthFromContext(ctx), args.FileID, args.ObjectName, args.ContentType)
	if err != nil {
		return agenttools.ToolResult{}, err
	}
	return agenttools.ToolResult{
		CallID: call.CallID, Status: agenttools.StatusCompleted, Data: result,
		Sources: toolSourcesFromAgentSources(result.Sources),
	}, nil
}

func (h *Agent) parsePaymentScheduleToolHandler(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
	args, err := decodeFileParseArguments(call.Arguments, true)
	if err != nil {
		return rejectedFileParse(call.CallID, err), nil
	}
	result, err := h.parsePaymentSchedule(ctx, aiServiceAuthFromContext(ctx), args.FileID, args.ObjectName, args.ContentType, args.ContractID)
	if err != nil {
		return agenttools.ToolResult{}, err
	}
	return agenttools.ToolResult{
		CallID: call.CallID, Status: agenttools.StatusCompleted, Data: result,
		Sources: toolSourcesFromAgentSources(result.Sources),
	}, nil
}

func (h *Agent) parseEventToolHandler(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
	args, err := decodeFileParseArguments(call.Arguments, true)
	if err != nil {
		return rejectedFileParse(call.CallID, err), nil
	}
	result, err := h.parseEvent(ctx, aiServiceAuthFromContext(ctx), args.FileID, args.ObjectName, args.ContentType, args.ContractID)
	if err != nil {
		return agenttools.ToolResult{}, err
	}
	return agenttools.ToolResult{
		CallID: call.CallID, Status: agenttools.StatusCompleted, Data: result,
		Sources: toolSourcesFromAgentSources(result.Sources),
	}, nil
}

func (h *Agent) executeFileParseTool(ctx context.Context, toolRuntime *agenttools.Runtime, toolName string, arguments fileParseArguments) (agenttools.ToolResult, error) {
	result, err := h.executeToolCall(ctx, toolRuntime, toolName, arguments, fileParseIdempotencyKey(toolName, arguments))
	if err != nil {
		return agenttools.ToolResult{}, err
	}
	if result.Error != nil {
		return result, errors.New(result.Error.Message)
	}
	if result.Status != agenttools.StatusCompleted && result.Status != agenttools.StatusNeedsReview {
		return result, fmt.Errorf("file parse tool returned status %s", result.Status)
	}
	return result, nil
}

func fileParseIdempotencyKey(toolName string, arguments fileParseArguments) string {
	return strings.Join([]string{toolName, arguments.FileID, arguments.ObjectName, arguments.ContractID}, ":")
}

func decodeFileParseArguments(raw json.RawMessage, allowContractID bool) (fileParseArguments, error) {
	var args fileParseArguments
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&args); err != nil {
		return fileParseArguments{}, fmt.Errorf("invalid file parse arguments: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return fileParseArguments{}, errors.New("invalid file parse arguments: multiple JSON values")
	}
	if !allowContractID {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			return fileParseArguments{}, errors.New("invalid file parse arguments: object required")
		}
		if _, present := fields["contract_id"]; present {
			return fileParseArguments{}, errors.New("contract_id is not accepted by this file parser")
		}
		args.ContractID = ""
	}
	for field, value := range map[string]string{
		"file_id": args.FileID, "object_name": args.ObjectName, "content_type": args.ContentType,
	} {
		if strings.TrimSpace(value) == "" {
			return fileParseArguments{}, fmt.Errorf("%s is required", field)
		}
		if strings.ContainsAny(value, "\r\n") {
			return fileParseArguments{}, fmt.Errorf("%s contains invalid line breaks", field)
		}
	}
	if strings.Contains(args.ObjectName, "://") || strings.HasPrefix(args.ObjectName, "/") {
		return fileParseArguments{}, errors.New("object_name must be a storage object, not a URL or absolute path")
	}
	return args, nil
}

func rejectedFileParse(callID string, err error) agenttools.ToolResult {
	return agenttools.ToolResult{
		CallID: callID, Status: agenttools.StatusRejected,
		Error: &agenttools.ToolError{Code: agenttools.ErrorInvalidArguments, Message: err.Error()},
	}
}

func toolSourcesFromAgentSources(sources []Source) []agenttools.ToolSource {
	converted := make([]agenttools.ToolSource, 0, len(sources))
	for _, source := range sources {
		converted = append(converted, agenttools.ToolSource{
			Type: source.Type, ID: source.ID, Title: source.Title, Locator: source.Snippet,
		})
	}
	return converted
}
