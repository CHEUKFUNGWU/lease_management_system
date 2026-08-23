// Package draftreview is BG3: the draft-review workbench service (Ch2).
//
// 隔离语义（底线 1，fail-closed）：列表与详情都要求行的 legal_entity_id 与
// 调用者 scope 严格相等；legal_entity_id 为 NULL 的历史行不匹配任何调用者，
// global 管理员（无法人）同样一行都看不到——对所有人不可见，而不是对所有人
// 可见。异法人与不存在对外同形：同一错误码、同一文案，无存在性泄漏。
//
// 置信度闸（D-B10）在 Decide 内部：只有 Revise 能把字段标记为已人工确认，
// 低置信度字段未经 Revise 确认时批准被服务端拒绝——绕过前端直接调 Decide
// 同样被拒。控制项在服务端，不在前端按钮。
//
// 差异留痕（D-B9）：AI 提取值与人工终值分列存储（contract_data 原键 vs
// contract_data.human_edits），互不覆盖。审计能回答「这个值是 AI 读出来的
// 还是人改的」。
//
// 部分失败逐条（D-B8）：第 N 条失败时前 N-1 条已入库、后续继续；每条批准
// 是独立事务。同一草稿批准两次经 agent_draft_idempotency 幂等回放，不产生
// 第二条正式记录（底线 4）。
package draftreview

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/errcontract"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/draftapp"
)

// ErrScopeDenied 是唯一对外措辞：跨法人 / NULL 行 / 不存在一律同一个码加同
// 一句话（无存在性泄漏），且保持 scope_denied 措辞不被软化成「无数据」（底
// 线 1）。HTTP 与 Agent Tool 两个 seam 都从它取码。
var ErrScopeDenied = errcontract.New(errcontract.CodeScopeDenied,
	"scope_denied: contract draft outside caller legal entity")

// approveOperation 是本服务在 agent_draft_idempotency 里的操作命名空间，
// 与 draftapp 的 operation 键互不相交。
const approveOperation = "draftreview.approve"

// LowConfidenceThreshold mirrors the intake rule: fields below this must be
// individually confirmed via Revise before approval.
const LowConfidenceThreshold = 0.60

// FieldEdit is one human revision: the final value plus whether the reviewer
// explicitly confirmed it. Revise is the only path that can set confirmed.
type FieldEdit struct {
	Field     string `json:"field"`
	Value     string `json:"value"`
	Confirmed bool   `json:"confirmed"`
}

// Decision is one approve/reject verdict in a batch.
type Decision struct {
	DraftID string `json:"draft_id"`
	Approve bool   `json:"approve"`
	Reason  string `json:"reason,omitempty"` // reject 必填
}

// OutcomeItem is the per-item result of a Decide batch: 部分失败逐条。
type OutcomeItem struct {
	DraftID string `json:"draft_id"`
	Verdict string `json:"verdict"` // approved | rejected | failed | replayed
	Error   string `json:"error,omitempty"`
}

type Outcome struct {
	Items       []OutcomeItem `json:"items"`
	ApprovedAll bool          `json:"approved_all"`
}

// Filter narrows the list. The legal entity never enters here: it is derived
// from the authenticated scope inside the service (绝不接受参数传入的法人)。
type Filter struct {
	Status string // pending | prepared | approved | rejected（空=全部未终态）
	Limit  int
}

// DraftDetail is the review surface: ai 提取值、人工终值、置信度、确认清单。
type DraftDetail struct {
	ID                 string             `json:"id"`
	TaskID             string             `json:"task_id"`
	LegalEntityID      *string            `json:"legal_entity_id,omitempty"`
	DataClassification string             `json:"data_classification,omitempty"`
	Status             string             `json:"status"`
	AiValues           map[string]any     `json:"ai_values"`
	HumanValues        map[string]any     `json:"human_values,omitempty"`
	ConfirmedFields    []string           `json:"confirmed_fields"`
	ConfidenceScores   map[string]float64 `json:"confidence_scores"`
	CreatedAt          time.Time          `json:"created_at"`
}

// ContractStore is the narrow write seam for approving a draft. It is a
// structural subset of draftapp.DraftStore, so the production postgres store
// satisfies it without an adapter — BG3 不新建写路径，复用既有幂等接缝。
type ContractStore interface {
	LookupIdempotency(ctx context.Context, operation, key string) (*draftapp.ItemResult, bool, error)
	CreateContractDraft(ctx context.Context, contract *repository.Contract) (*repository.Contract, error)
	SaveIdempotency(ctx context.Context, operation, key string, result draftapp.ItemResult) error
}

// UnitOfWork bounds each approval in one transaction. 批准是逐条事务：
// 第 N 条失败不回滚前 N-1 条。
type UnitOfWork interface {
	Execute(ctx context.Context, fn func(ContractStore) error) error
}

// Reader is the read seam over ai_contract_drafts (existing repository).
// The two Resolve methods are the master-data resolution the intake chain
// already uses（lease_contracts.store_id / landlord_id 均 NOT NULL，正式记录
// 需要它们）；复用同一实现避免第二套主数据解析逻辑。
type Reader interface {
	ListDraftsForReview(ctx context.Context, entity access.EntityFilter, status string, limit int) ([]repository.DraftReviewRow, error)
	GetDraftForReview(ctx context.Context, entity access.EntityFilter, id string) (repository.DraftReviewRow, error)
	UpdateDraftReview(ctx context.Context, id string, in repository.UpdateDraftReviewInput) error
	SaveDraftEdits(ctx context.Context, id string, humanEdits json.RawMessage) error
	ResolveOrCreateStoreID(ctx context.Context, name, address string, legalEntityID *string) (*string, error)
	ResolveOrCreateLandlordID(ctx context.Context, name string) (*string, error)
}

// Service is BG3's four-method surface.
type Service struct {
	reader Reader
	uow    UnitOfWork
	now    func() time.Time
}

func NewService(reader Reader, uow UnitOfWork) *Service {
	return &Service{reader: reader, uow: uow, now: time.Now}
}

// List returns drafts whose legal_entity_id equals the caller's. NULL rows and
// global accounts match nothing — locked by integration tests; 反向改动必须红。
func (s *Service) List(ctx context.Context, f Filter) ([]DraftDetail, error) {
	entity, err := scopeEntity(ctx)
	if err != nil {
		return []DraftDetail{}, nil // 无法人上下文 → 列不到任何行（fail-closed）
	}
	rows, err := s.reader.ListDraftsForReview(ctx, entity, f.Status, f.Limit)
	if err != nil {
		return nil, err
	}
	out := make([]DraftDetail, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDetail(row))
	}
	return out, nil
}

// Get returns one draft. 异法人 / NULL 行 / 不存在都返回 ErrScopeDenied：
// repository 层三者同为 pgx.ErrNoRows，这里统一映射，同形由构造保证。
func (s *Service) Get(ctx context.Context, id string) (DraftDetail, error) {
	row, err := s.loadRow(ctx, id)
	if err != nil {
		return DraftDetail{}, err
	}
	return toDetail(row), nil
}

// Revise applies field edits into the human layer and marks them confirmed.
// It is the only path that can satisfy the confidence gate, and it writes both
// layers so neither overwrites the other（差异留痕）。
func (s *Service) Revise(ctx context.Context, id string, edits []FieldEdit) (DraftDetail, error) {
	row, err := s.loadRow(ctx, id)
	if err != nil {
		return DraftDetail{}, err
	}
	if row.Status == "approved" || row.Status == "rejected" {
		return DraftDetail{}, &errcontract.Error{Code: errcontract.CodeBusinessFailure,
			Message: "contract draft is already decided; revision closed"}
	}
	human := humanEditsOf(row)
	for _, edit := range edits {
		key := strings.TrimSpace(edit.Field)
		if key == "" || key == reservedHumanEditsKey {
			continue
		}
		human[key] = humanEdit{Value: edit.Value, Confirmed: edit.Confirmed}
	}
	raw, err := json.Marshal(human)
	if err != nil {
		return DraftDetail{}, fmt.Errorf("encode human edits: %w", err)
	}
	if err := s.reader.SaveDraftEdits(ctx, id, raw); err != nil {
		return DraftDetail{}, err
	}
	next := row
	next.Status = "prepared"
	if err := s.reader.UpdateDraftReview(ctx, id, repository.UpdateDraftReviewInput{
		Status: next.Status, ReviewerUserID: ReviewerFrom(ctx),
	}); err != nil {
		return DraftDetail{}, err
	}
	return toDetail(next), nil
}

// Decide approves/rejects a batch of drafts. 单条批准 = 长度为 1 的列表；
// 置信度闸、幂等、部分失败语义只在这一份实现里（D-B20）。
func (s *Service) Decide(ctx context.Context, decisions []Decision) (Outcome, error) {
	outcome := Outcome{Items: make([]OutcomeItem, 0, len(decisions))}
	for _, decision := range decisions {
		outcome.Items = append(outcome.Items, s.decideOne(ctx, decision))
	}
	outcome.ApprovedAll = allApproved(outcome.Items)
	return outcome, nil
}

func (s *Service) decideOne(ctx context.Context, decision Decision) OutcomeItem {
	item := OutcomeItem{DraftID: decision.DraftID}
	switch {
	case strings.TrimSpace(decision.DraftID) == "":
		item.Verdict, item.Error = "failed", "draft_id is required"
		return item
	case !decision.Approve && strings.TrimSpace(decision.Reason) == "":
		item.Verdict, item.Error = "failed", "reject requires a reason"
		return item
	}
	row, err := s.loadRow(ctx, decision.DraftID)
	if err != nil {
		item.Verdict, item.Error = "failed", err.Error()
		return item
	}
	// 终态语义：rejected 是终局（不允许复活）；approved 只对再次批准放行 ——
	// 那条路会命中幂等键成为回放，不产生第二条正式记录（底线 4）。
	switch {
	case !decision.Approve && (row.Status == "approved" || row.Status == "rejected"):
		item.Verdict, item.Error = "failed", "contract draft already "+row.Status
		return item
	case decision.Approve && row.Status == "rejected":
		item.Verdict, item.Error = "failed", "contract draft already rejected"
		return item
	}
	if decision.Approve {
		return s.approveOne(ctx, row)
	}
	if err := s.reader.UpdateDraftReview(ctx, row.ID, repository.UpdateDraftReviewInput{
		Status: "rejected", ReviewerUserID: ReviewerFrom(ctx), RejectedReason: strings.TrimSpace(decision.Reason),
	}); err != nil {
		item.Verdict, item.Error = "failed", "reject failed"
		return item
	}
	item.Verdict = "rejected"
	return item
}

// approveOne runs one idempotent promotion inside its own transaction:
// advisory lock → idempotency lookup → validated create → save result.
func (s *Service) approveOne(ctx context.Context, row repository.DraftReviewRow) OutcomeItem {
	item := OutcomeItem{DraftID: row.ID}
	if blockers := lowConfidenceBlockers(row); len(blockers) > 0 {
		item.Verdict = "failed"
		item.Error = "low_confidence_fields_unconfirmed: " + strings.Join(blockers, ", ")
		return item
	}
	formal, err := s.buildFormal(ctx, row)
	if err != nil {
		item.Verdict, item.Error = "failed", err.Error()
		return item
	}
	err = s.uow.Execute(ctx, func(store ContractStore) error {
		key := idempotencyKey(row.ID)
		if _, hit, err := store.LookupIdempotency(ctx, approveOperation, key); err != nil {
			return err
		} else if hit {
			return nil // 回放：已批准过，不再产生第二条正式记录（底线 4）
		}
		created, err := store.CreateContractDraft(ctx, formal)
		if err != nil {
			return err
		}
		if created == nil || strings.TrimSpace(created.ID) == "" {
			return errors.New("contract draft writer returned no id")
		}
		return store.SaveIdempotency(ctx, approveOperation, key, draftapp.ItemResult{
			Operation: approveOperation, IdempotencyKey: key,
			Status: draftapp.ItemCreated, ID: created.ID,
		})
	})
	if err != nil {
		// 写路径的底层错误（SQL、约束名）不进对外载荷，统一具名为
		// approve failed；字段级问题（缺列、键名错位）在进入事务前已被拦下。
		item.Verdict, item.Error = "failed", "approve failed"
		return item
	}
	// 事务外推进草稿状态：若在此处中断，重跑批准会命中幂等键并再次推进，
	// 最终收敛；正式记录不会被重复创建。
	if err := s.reader.UpdateDraftReview(ctx, row.ID, repository.UpdateDraftReviewInput{
		Status: "approved", ReviewerUserID: ReviewerFrom(ctx),
	}); err != nil {
		item.Verdict, item.Error = "failed", "approve recorded but draft update failed"
		return item
	}
	item.Verdict = "approved"
	return item
}

// loadRow derives the caller's entity from ctx and fetches through the reader.
// Any miss (foreign entity, NULL-entity legacy row, unknown id) surfaces as
// ErrScopeDenied —— 同形，无存在性泄漏，措辞不被软化。
func (s *Service) loadRow(ctx context.Context, id string) (repository.DraftReviewRow, error) {
	entity, err := scopeEntity(ctx)
	if err != nil {
		return repository.DraftReviewRow{}, ErrScopeDenied
	}
	row, err := s.reader.GetDraftForReview(ctx, entity, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return repository.DraftReviewRow{}, ErrScopeDenied
	}
	if err != nil {
		return repository.DraftReviewRow{}, err
	}
	return row, nil
}

// lowConfidenceBlockers lists fields whose AI confidence is below threshold
// and that were not individually confirmed via Revise.
func lowConfidenceBlockers(row repository.DraftReviewRow) []string {
	confidence := decodeNumbers(row.ConfidenceScores)
	confirmed := confirmedFieldsOf(row)
	blockers := make([]string, 0)
	for field, score := range confidence {
		if score < LowConfidenceThreshold && !confirmed[field] {
			blockers = append(blockers, field)
		}
	}
	sortStrings(blockers)
	return blockers
}

func allApproved(items []OutcomeItem) bool {
	if len(items) == 0 {
		return false
	}
	for _, item := range items {
		if item.Verdict != "approved" && item.Verdict != "replayed" {
			return false
		}
	}
	return true
}

func idempotencyKey(draftID string) string { return "contract-draft:" + draftID }

func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}
