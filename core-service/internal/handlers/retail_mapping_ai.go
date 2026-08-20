package handlers

// RetailMappingAI adapts the in-process LLM client (W4-1) to the
// retailingest.MappingSuggester seam — Assist Mode: it only returns proposals
// (headers + masked profiles; raw values never leave Go, D13). It replaces the
// former /api/v1/suggest-mapping HTTP hop (W5-4).

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/lease-management-system/core-service/internal/llm"
	"github.com/lease-management-system/core-service/internal/services/retailingest"
)

// standardMappingFields mirrors the ai-service mapping.py STANDARD_FIELDS and
// retailingest.AllFields: the AI may only return one of these names or null.
var standardMappingFields = map[string]bool{
	"store": true, "business_date": true, "currency": true, "revenue": true,
	"gross_profit": true, "transactions": true, "footfall": true, "area_sqm": true,
	"labor_cost": true, "fixed_rent": true, "variable_rent": true,
	"non_lease_cost": true, "other_controllable_cost": true,
}

type RetailMappingAI struct {
	client *llm.Client
}

// NewRetailMappingAI builds the adapter from the environment (LLM_*). A
// missing API key fails at call time (fail-closed), matching the old 502.
func NewRetailMappingAI() *RetailMappingAI {
	client, _ := llm.NewClient(llm.ConfigFromEnv())
	return &RetailMappingAI{client: client}
}

// WithClient injects a client for tests/production wiring.
func (a *RetailMappingAI) WithClient(c *llm.Client) *RetailMappingAI {
	a.client = c
	return a
}

type aiMappingProfile struct {
	Header       string `json:"header"`
	NonEmpty     int    `json:"non_empty"`
	NumericLike  int    `json:"numeric_like"`
	DateLike     int    `json:"date_like"`
	MaskedSample string `json:"masked_sample,omitempty"`
}

var mappingFenceRe = regexp.MustCompile("(?is)^\\s*```(?:json)?\\s*|\\s*```\\s*$")

func (a *RetailMappingAI) SuggestMapping(ctx context.Context, headers []string, columnProfiles []retailingest.ColumnProfile) (retailingest.Mapping, error) {
	if a == nil || a.client == nil {
		return nil, fmt.Errorf("mapping suggestion unavailable: llm client is not configured")
	}
	profiles := make([]aiMappingProfile, 0, len(columnProfiles))
	for _, profile := range columnProfiles {
		profiles = append(profiles, aiMappingProfile{Header: profile.Header, NonEmpty: profile.NonEmpty, NumericLike: profile.Numeric, DateLike: profile.DateLike, MaskedSample: profile.MaskedSample})
	}
	userContent, err := json.Marshal(map[string]any{"headers": headers, "column_profiles": profiles})
	if err != nil {
		return nil, err
	}
	result, err := a.client.Chat(ctx, llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: mappingSystemPrompt},
			{Role: "user", Content: string(userContent)},
		},
		Temp:      0,
		MaxTokens: 800,
	})
	if err != nil {
		return nil, fmt.Errorf("mapping suggestion unavailable: %w", err)
	}
	text := mappingFenceRe.ReplaceAllString(strings.TrimSpace(result.Answer), "")
	text = strings.TrimSpace(text)
	var suggestions map[string]*string
	if err := json.Unmarshal([]byte(text), &suggestions); err != nil {
		return nil, fmt.Errorf("mapping suggestion unavailable: model did not return a mapping object: %w", err)
	}
	cleaned := retailingest.Mapping{}
	for _, header := range headers {
		if value := suggestions[header]; value != nil && standardMappingFields[*value] {
			cleaned[header] = *value
		}
	}
	return cleaned, nil
}

const mappingSystemPrompt = "你是门店经营数据导入的列映射助手。把文件列头映射到标准字段。" +
	"标准字段：store(门店/店铺), business_date(日期), currency(币种), " +
	"revenue(营业额/销售额), gross_profit(毛利), transactions(交易数), " +
	"footfall(客流), area_sqm(面积), labor_cost(人工成本), fixed_rent(固定租金), " +
	"variable_rent(变量租金), non_lease_cost(非租赁成本), other_controllable_cost(其他可控成本)。" +
	"判定规则：date_like 高的列优先映射 business_date；numeric_like 高且语义为金额/数量的" +
	"优先对应数值指标；含门店/店铺/店号/store 语义的映射 store；无法可靠判断的列映射为 null。" +
	"只输出一个 JSON 对象 {列头: 字段名或 null}，字段名必须取自标准字段清单，" +
	"不要输出任何解释或多余文本。"
