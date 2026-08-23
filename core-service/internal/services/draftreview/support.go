package draftreview

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/repository"
)

// reservedHumanEditsKey 是 contract_data 内人工修订层的保留键。Contract 的
// json tag 不含这个名字，合法抽取载荷不会撞上它。
const reservedHumanEditsKey = "human_edits"

// reviewerContextKey carries the authenticated reviewer's user id into ctx.
type reviewerContextKey struct{}

// WithReviewer attaches the authenticated user id (JWT claims 的 user_id，
// 即 users.id) to the context.
func WithReviewer(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, reviewerContextKey{}, strings.TrimSpace(userID))
}

// ReviewerFrom reads the reviewer identity back out; empty when absent —
// UpdateDraftReview 会把空值写成 NULL 而不是编一个 ID。
func ReviewerFrom(ctx context.Context) string {
	value, _ := ctx.Value(reviewerContextKey{}).(string)
	return value
}

// scopeEntity derives the caller's legal-entity filter from the access scope
// projected into ctx（与 JWT 同一条解析路径）。global 管理员与无法人上下文
// 一律拒绝：本接缝的隔离规则是严格相等，NULL 行对任何账号不可见。
func scopeEntity(ctx context.Context) (access.EntityFilter, error) {
	scope, ok := access.ScopeFromContext(ctx)
	if !ok || scope.Global {
		return access.EntityFilter{}, fmt.Errorf("draft review requires a legal-entity scope")
	}
	return access.EntityFilterFor(scope.LegalEntityID)
}

// humanEdit is one field's human layer: final value + explicit confirmation.
type humanEdit struct {
	Value     string `json:"value"`
	Confirmed bool   `json:"confirmed"`
}

func humanEditsOf(row repository.DraftReviewRow) map[string]humanEdit {
	object := decodeObject(row.ContractData)
	raw, _ := json.Marshal(object[reservedHumanEditsKey])
	edits := map[string]humanEdit{}
	if err := json.Unmarshal(raw, &edits); err != nil || edits == nil {
		// "null"（无人工层）会把目标 map 置为 nil，后续写入会 panic。
		return map[string]humanEdit{}
	}
	return edits
}

// aiValuesOf returns the original extraction object without the human layer.
func aiValuesOf(row repository.DraftReviewRow) map[string]any {
	object := decodeObject(row.ContractData)
	delete(object, reservedHumanEditsKey)
	return object
}

// effectiveValues merges the human layer over the AI extraction: 人工终值
// 优先，但 AI 原值仍在原键可查（差异留痕）。
func effectiveValues(row repository.DraftReviewRow) map[string]any {
	values := aiValuesOf(row)
	for key, edit := range humanEditsOf(row) {
		values[key] = edit.Value
	}
	return values
}

func confirmedFieldsOf(row repository.DraftReviewRow) map[string]bool {
	confirmed := map[string]bool{}
	for key, edit := range humanEditsOf(row) {
		if edit.Confirmed {
			confirmed[key] = true
		}
	}
	return confirmed
}

// buildFormal promotes a draft row into a formal lease_contracts payload。
// 这里不做盲 Unmarshal：AI 抽取的键名与 repository.Contract 的 json tag 必须
// 逐一对齐，缺关键列、日期不可解析都让该条失败并具名报错，绝不静默产出承
// 租方为空的正式合同。主数据（门店、出租方）经既有 ResolveOrCreate 解析，
// 与 intake 链路同一套实现。
//
// 隔离锚点：formal.LegalEntityID 强制取行上的 legal_entity_id，调用者与
// 载荷都无法把它指向另一个法人。
func (s *Service) buildFormal(ctx context.Context, row repository.DraftReviewRow) (*repository.Contract, error) {
	values := effectiveValues(row)
	// lease_scope 是 IFRS 16 范围分类（会计判断，红线 1），缺失即具名失败，
	// 绝不默认填值；取值受 lease_contracts_lease_scope_check 约束。
	for _, field := range []string{"contract_number", "lessee_name", "lessor_name", "currency", "store_name", "lease_scope"} {
		if strings.TrimSpace(stringOf(values[field])) == "" {
			return nil, fmt.Errorf("contract payload missing %s", field)
		}
	}
	for _, field := range []string{"commencement_date", "lease_start_date", "lease_end_date"} {
		parsed, err := parseDraftDate(values[field])
		if err != nil {
			return nil, fmt.Errorf("contract payload %s: %v", field, err)
		}
		values[field] = parsed.UTC().Format(time.RFC3339) // 归一化为 Contract 可解码的形状
	}
	currency := strings.ToUpper(strings.TrimSpace(stringOf(values["currency"])))
	values["currency"] = currency

	storeID, err := s.reader.ResolveOrCreateStoreID(ctx, stringOf(values["store_name"]), stringOf(values["store_address"]), row.LegalEntityID)
	if err != nil {
		return nil, fmt.Errorf("resolve store: %w", err)
	}
	landlordID, err := s.reader.ResolveOrCreateLandlordID(ctx, stringOf(values["lessor_name"]))
	if err != nil {
		return nil, fmt.Errorf("resolve landlord: %w", err)
	}

	raw, err := json.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("encode contract payload: %w", err)
	}
	formal := &repository.Contract{}
	if err := json.Unmarshal(raw, formal); err != nil {
		return nil, fmt.Errorf("contract payload is not a valid contract shape")
	}
	formal.ID = ""
	formal.LegalEntityID = row.LegalEntityID // 隔离锚点：以行为准，不信载荷
	formal.StoreID = storeID
	formal.LandlordID = landlordID
	formal.Status = ""
	formal.ApprovalStatus = "" // Create 落为 draft，走既有正式审批流
	return formal, nil
}

// parseDraftDate accepts the date shapes AI extraction produces (YYYY-MM-DD
// and RFC3339) and normalizes them for Contract's time.Time fields.
func parseDraftDate(value any) (time.Time, error) {
	text := strings.TrimSpace(stringOf(value))
	if text == "" {
		return time.Time{}, fmt.Errorf("required")
	}
	for _, layout := range []string{"2006-01-02", time.RFC3339} {
		if parsed, err := time.Parse(layout, text); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("unparseable date %q", text)
}

func decodeNumbers(raw json.RawMessage) map[string]float64 {
	out := map[string]float64{}
	if err := json.Unmarshal(raw, &out); err != nil || out == nil {
		return map[string]float64{}
	}
	return out
}

func decodeObject(raw json.RawMessage) map[string]any {
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil || out == nil {
		return map[string]any{}
	}
	return out
}

func stringOf(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func toDetail(row repository.DraftReviewRow) DraftDetail {
	human := map[string]any{}
	for key, edit := range humanEditsOf(row) {
		human[key] = edit.Value
	}
	confirmed := make([]string, 0, len(humanEditsOf(row)))
	for key, edit := range humanEditsOf(row) {
		if edit.Confirmed {
			confirmed = append(confirmed, key)
		}
	}
	sortStrings(confirmed)
	return DraftDetail{
		ID:                 row.ID,
		TaskID:             row.TaskID,
		LegalEntityID:      row.LegalEntityID,
		DataClassification: row.DataClassification,
		Status:             row.Status,
		AiValues:           aiValuesOf(row),
		HumanValues:        human,
		ConfirmedFields:    confirmed,
		ConfidenceScores:   decodeNumbers(row.ConfidenceScores),
		CreatedAt:          row.CreatedAt,
	}
}
