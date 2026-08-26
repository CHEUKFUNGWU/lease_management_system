package draftreview

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/draftapp"
)

// ── fakes ───────────────────────────────────────────────────────────────────
//
// fakeReader reproduces the SQL semantics the repository must keep: a row is
// visible only when its legal_entity_id is non-NULL and equals the filter's
// entity; every other miss is pgx.ErrNoRows. If these semantics drift, the
// integration tests in internal/repository are the second line of defense.

type fakeReader struct {
	rows      []repository.DraftReviewRow
	updated   []repository.UpdateDraftReviewInput
	edits     []json.RawMessage
	failOnGet string // id that makes GetDraftForReview return a storage error
}

func (f *fakeReader) ListDraftsForReview(_ context.Context, entity access.EntityFilter, status string, limit int) ([]repository.DraftReviewRow, error) {
	if entity.IsGlobal() {
		return nil, nil
	}
	id, err := entity.LegalEntityID()
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	out := make([]repository.DraftReviewRow, 0)
	for _, row := range f.rows {
		if row.LegalEntityID == nil || *row.LegalEntityID != id {
			continue
		}
		if status != "" && row.Status != status {
			continue
		}
		if len(out) < limit {
			out = append(out, row)
		}
	}
	return out, nil
}

func (f *fakeReader) GetDraftForReview(_ context.Context, entity access.EntityFilter, id string) (repository.DraftReviewRow, error) {
	if id == f.failOnGet {
		return repository.DraftReviewRow{}, errors.New("storage unavailable")
	}
	if entity.IsGlobal() {
		return repository.DraftReviewRow{}, pgx.ErrNoRows
	}
	scoped, err := entity.LegalEntityID()
	if err != nil {
		return repository.DraftReviewRow{}, err
	}
	for _, row := range f.rows {
		if row.ID != id {
			continue
		}
		if row.LegalEntityID == nil || *row.LegalEntityID != scoped {
			return repository.DraftReviewRow{}, pgx.ErrNoRows // 异法人 = 不存在
		}
		return row, nil
	}
	return repository.DraftReviewRow{}, pgx.ErrNoRows
}

func (f *fakeReader) ResolveOrCreateStoreID(_ context.Context, _, _ string, _ *string) (*string, error) {
	id := "store-fixed"
	return &id, nil
}

func (f *fakeReader) ResolveOrCreateLandlordID(_ context.Context, _ string) (*string, error) {
	id := "landlord-fixed"
	return &id, nil
}

func (f *fakeReader) UpdateDraftReview(_ context.Context, id string, in repository.UpdateDraftReviewInput) error {
	f.updated = append(f.updated, in)
	for i := range f.rows {
		if f.rows[i].ID == id {
			f.rows[i].Status = in.Status
		}
	}
	return nil
}

func (f *fakeReader) SaveDraftEdits(_ context.Context, id string, humanEdits json.RawMessage) error {
	f.edits = append(f.edits, humanEdits)
	// 模拟 jsonb_set(contract_data,'{human_edits}',...) 的持久化效果。
	for i := range f.rows {
		if f.rows[i].ID != id {
			continue
		}
		object := map[string]any{}
		_ = json.Unmarshal(f.rows[i].ContractData, &object)
		human := map[string]any{}
		_ = json.Unmarshal(humanEdits, &human)
		if object == nil {
			object = map[string]any{}
		}
		object["human_edits"] = human
		raw, err := json.Marshal(object)
		if err != nil {
			panic(err) // fake 内部不可达路径
		}
		f.rows[i].ContractData = raw
	}
	return nil
}

type fakeStore struct {
	idempotency map[string]draftapp.ItemResult
	created     []*repository.Contract
	failCreateN int // 前 N 次 CreateContractDraft 失败（模拟部分失败）
}

func newFakeStore() *fakeStore {
	return &fakeStore{idempotency: map[string]draftapp.ItemResult{}}
}

func (s *fakeStore) LookupIdempotency(_ context.Context, operation, key string) (*draftapp.ItemResult, bool, error) {
	if result, ok := s.idempotency[operation+"\x00"+key]; ok {
		return &result, true, nil
	}
	return nil, false, nil
}

func (s *fakeStore) CreateContractDraft(_ context.Context, contract *repository.Contract) (*repository.Contract, error) {
	if s.failCreateN > 0 {
		s.failCreateN--
		return nil, errors.New("simulated create failure")
	}
	contract.ID = uuid.NewString()
	contract.Status = "draft"
	s.created = append(s.created, contract)
	return contract, nil
}

func (s *fakeStore) SaveIdempotency(_ context.Context, operation, key string, result draftapp.ItemResult) error {
	s.idempotency[operation+"\x00"+key] = result
	return nil
}

type fakeUOW struct{ store ContractStore }

func (u fakeUOW) Execute(ctx context.Context, fn func(ContractStore) error) error { return fn(u.store) }

// ── helpers ────────────────────────────────────────────────────────────────

const (
	entityA = "11111111-1111-1111-1111-111111111111"
	entityB = "22222222-2222-2222-2222-222222222222"
)

var completePayload = map[string]any{
	"contract_number":   "CT-2026-001",
	"contract_name":     "旗舰店铺租约",
	"lessee_name":       "承租方甲",
	"lessor_name":       "出租方乙",
	"store_name":        "旗舰店",
	"lease_scope":       "in_scope",
	"currency":          "CNY",
	"commencement_date": "2026-01-01",
	"lease_start_date":  "2026-01-01",
	"lease_end_date":    "2027-12-31",
}

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return raw
}

func draftRow(t *testing.T, id, entityID string, payload map[string]any, confidence map[string]float64) repository.DraftReviewRow {
	t.Helper()
	row := repository.DraftReviewRow{
		ID: id, TaskID: uuid.NewString(), Status: "pending",
		ContractData: mustJSON(t, payload),
	}
	if entityID != "" {
		row.LegalEntityID = &entityID
	}
	if confidence != nil {
		row.ConfidenceScores = mustJSON(t, confidence)
	}
	return row
}

func scopedCtx(entityID string) context.Context {
	return access.WithScope(context.Background(), access.Scope{LegalEntityID: entityID})
}

func reviewerCtx(entityID, userID string) context.Context {
	return WithReviewer(scopedCtx(entityID), userID)
}

func newHarness(reader *fakeReader, store *fakeStore) *Service {
	return NewService(reader, fakeUOW{store: store})
}

// ── 验收 2：调用者法人为空时列不到任何行 ─────────────────────────────────

func TestListWithNoLegalEntityReturnsNothing(t *testing.T) {
	reader := &fakeReader{rows: []repository.DraftReviewRow{
		draftRow(t, "d-a", entityA, completePayload, nil),
	}}
	service := newHarness(reader, newFakeStore())

	for name, ctx := range map[string]context.Context{
		"no-scope":     context.Background(),
		"global-admin": access.WithScope(context.Background(), access.Scope{Global: true}),
		"empty-entity": scopedCtx(""),
		"blank-entity": scopedCtx("   "),
	} {
		rows, err := service.List(ctx, Filter{})
		if err != nil {
			t.Fatalf("%s: list errored: %v", name, err)
		}
		if len(rows) != 0 {
			t.Fatalf("%s: expected zero rows, got %d", name, len(rows))
		}
	}
}

// ── 验收 1（服务侧半边）：NULL 法人行任何账号都不可达 ────────────────────

func TestNullEntityRowInvisibleToEveryCaller(t *testing.T) {
	reader := &fakeReader{rows: []repository.DraftReviewRow{
		draftRow(t, "d-null", "", completePayload, nil), // 存量行：法人未知
	}}
	service := newHarness(reader, newFakeStore())

	for name, ctx := range map[string]context.Context{
		"entity-a":     scopedCtx(entityA),
		"entity-b":     scopedCtx(entityB),
		"global-admin": access.WithScope(context.Background(), access.Scope{Global: true}),
	} {
		if rows, err := service.List(ctx, Filter{}); err != nil || len(rows) != 0 {
			t.Fatalf("%s: NULL-entity row leaked: %d rows, err %v", name, len(rows), err)
		}
		if _, err := service.Get(ctx, "d-null"); !errors.Is(err, ErrScopeDenied) {
			t.Fatalf("%s: Get NULL-entity row err=%v; want ErrScopeDenied", name, err)
		}
	}
}

// ── 验收 3：跨法人列不到，拒绝措辞保持 scope_denial 不软化 ────────────────

func TestCrossTenantDraftDeniedWithExactWording(t *testing.T) {
	reader := &fakeReader{rows: []repository.DraftReviewRow{
		draftRow(t, "d-b", entityB, completePayload, nil),
	}}
	service := newHarness(reader, newFakeStore())

	rows, err := service.List(scopedCtx(entityA), Filter{})
	if err != nil || len(rows) != 0 {
		t.Fatalf("entity A listed entity B drafts: %d rows, err %v", len(rows), err)
	}
	_, err = service.Get(scopedCtx(entityA), "d-b")
	if !errors.Is(err, ErrScopeDenied) {
		t.Fatalf("err=%v; want ErrScopeDenied", err)
	}
	const want = "scope_denied: contract draft outside caller legal entity"
	if err.Error() != want {
		t.Fatalf("softened wording %q; want exact %q", err.Error(), want)
	}
}

// ── 验收 4：异法人与不存在对外同形 ────────────────────────────────────────

func TestForeignAndMissingIndistinguishable(t *testing.T) {
	reader := &fakeReader{rows: []repository.DraftReviewRow{
		draftRow(t, "d-b", entityB, completePayload, nil),
	}}
	service := newHarness(reader, newFakeStore())
	ctx := scopedCtx(entityA)

	_, foreignErr := service.Get(ctx, "d-b")
	_, missingErr := service.Get(ctx, "00000000-0000-0000-0000-000000000000")
	if foreignErr == nil || missingErr == nil {
		t.Fatalf("both must error: foreign=%v missing=%v", foreignErr, missingErr)
	}
	if foreignErr.Error() != missingErr.Error() {
		t.Fatalf("existence leak: foreign %q vs missing %q", foreignErr.Error(), missingErr.Error())
	}
	if errors.Is(foreignErr, pgx.ErrNoRows) {
		t.Fatalf("raw pgx sentinel leaked past the service seam")
	}
}

// ── 验收 5：置信度闸在服务端 —— 绕过前端直接调 Decide 仍被拒 ─────────────

func TestDecideRejectsLowConfidenceWithoutConfirmation(t *testing.T) {
	payload := map[string]any{}
	for k, v := range completePayload {
		payload[k] = v
	}
	reader := &fakeReader{rows: []repository.DraftReviewRow{
		draftRow(t, "d-low", entityA, payload, map[string]float64{
			"lessee_name": 0.42, "commencement_date": 0.55,
		}),
	}}
	store := newFakeStore()
	service := newHarness(reader, store)

	outcome, err := service.Decide(reviewerCtx(entityA, "user-1"), []Decision{{DraftID: "d-low", Approve: true}})
	if err != nil {
		t.Fatal(err)
	}
	item := outcome.Items[0]
	if item.Verdict != "failed" {
		t.Fatalf("low-confidence approval passed the gate: %+v", item)
	}
	if !strings.Contains(item.Error, "low_confidence_fields_unconfirmed") ||
		!strings.Contains(item.Error, "lessee_name") || !strings.Contains(item.Error, "commencement_date") {
		t.Fatalf("blockers not named per field: %q", item.Error)
	}
	if len(store.created) != 0 {
		t.Fatalf("formal record created despite gate: %d", len(store.created))
	}
}

func TestReviseIsTheOnlyConfirmationPath(t *testing.T) {
	reader := &fakeReader{rows: []repository.DraftReviewRow{
		draftRow(t, "d-fix", entityA, completePayload, map[string]float64{"lessee_name": 0.3}),
	}}
	store := newFakeStore()
	service := newHarness(reader, store)

	// 未确认直接批准：拒。
	outcome, _ := service.Decide(reviewerCtx(entityA, "user-1"), []Decision{{DraftID: "d-fix", Approve: true}})
	if outcome.Items[0].Verdict != "failed" {
		t.Fatalf("gate bypassed: %+v", outcome.Items[0])
	}

	// Revise 逐字段确认后批准：过。
	if _, err := service.Revise(reviewerCtx(entityA, "user-1"), "d-fix", []FieldEdit{
		{Field: "lessee_name", Value: "承租方甲（更正）", Confirmed: true},
	}); err != nil {
		t.Fatal(err)
	}
	if len(reader.edits) != 1 {
		t.Fatalf("human edits not persisted: %d", len(reader.edits))
	}
	// 差异留痕：人工层记录了新值，AI 原值仍在原键。
	var edits map[string]humanEdit
	if err := json.Unmarshal(reader.edits[0], &edits); err != nil {
		t.Fatal(err)
	}
	if edits["lessee_name"].Value != "承租方甲（更正）" || !edits["lessee_name"].Confirmed {
		t.Fatalf("human layer wrong: %+v", edits)
	}

	outcome, _ = service.Decide(reviewerCtx(entityA, "user-1"), []Decision{{DraftID: "d-fix", Approve: true}})
	if outcome.Items[0].Verdict != "approved" {
		t.Fatalf("confirmed revision still blocked: %+v", outcome.Items[0])
	}
	if len(store.created) != 1 || store.created[0].LesseeName != "承租方甲（更正）" {
		t.Fatalf("approved contract should carry the human value: %+v", store.created)
	}
}

// ── 验收 6：部分失败逐条，前 N-1 条已入库，续跑不重复入库 ────────────────

func TestPartialFailureRetainsPriorItemsAndResumeDoesNotDuplicate(t *testing.T) {
	good := map[string]any{}
	for k, v := range completePayload {
		good[k] = v
	}
	blocked := map[string]any{}
	for k, v := range completePayload {
		blocked[k] = v
	}
	reader := &fakeReader{rows: []repository.DraftReviewRow{
		draftRow(t, "d-1", entityA, good, nil),
		draftRow(t, "d-2", entityA, blocked, map[string]float64{"currency": 0.1}), // 会被闸拦下
		draftRow(t, "d-3", entityA, good, nil),
	}}
	store := newFakeStore()
	service := newHarness(reader, store)

	decisions := []Decision{{DraftID: "d-1", Approve: true}, {DraftID: "d-2", Approve: true}, {DraftID: "d-3", Approve: true}}
	outcome, err := service.Decide(reviewerCtx(entityA, "user-1"), decisions)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Items[0].Verdict != "approved" || outcome.Items[1].Verdict != "failed" || outcome.Items[2].Verdict != "approved" {
		t.Fatalf("partial-failure shape broken: %+v", outcome.Items)
	}
	if outcome.ApprovedAll {
		t.Fatal("ApprovedAll must be false when an item failed")
	}
	if len(store.created) != 2 {
		t.Fatalf("prior items not retained: created=%d", len(store.created))
	}

	// 续跑：只剩失败那条；已入库的两条不重复创建。
	retry, err := service.Decide(reviewerCtx(entityA, "user-1"), []Decision{{DraftID: "d-1", Approve: true}, {DraftID: "d-3", Approve: true}})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range retry.Items {
		if item.Verdict != "approved" {
			t.Fatalf("retry item not approved: %+v", item)
		}
	}
	if len(store.created) != 2 {
		t.Fatalf("resume duplicated formal records: created=%d", len(store.created))
	}
}

// ── 验收 7：同一草稿批准两次不产生第二条正式记录 ─────────────────────────

func TestDoubleApproveIsReplayedNotDuplicated(t *testing.T) {
	reader := &fakeReader{rows: []repository.DraftReviewRow{
		draftRow(t, "d-idem", entityA, completePayload, nil),
	}}
	store := newFakeStore()
	service := newHarness(reader, store)

	first, _ := service.Decide(reviewerCtx(entityA, "user-1"), []Decision{{DraftID: "d-idem", Approve: true}})
	second, _ := service.Decide(reviewerCtx(entityA, "user-1"), []Decision{{DraftID: "d-idem", Approve: true}})
	if first.Items[0].Verdict != "approved" || second.Items[0].Verdict != "approved" {
		t.Fatalf("first=%+v second=%+v", first.Items[0], second.Items[0])
	}
	if len(store.created) != 1 {
		t.Fatalf("duplicate formal record: created=%d", len(store.created))
	}
}

// ── 风险点 1：抽取键名与 Contract tag 对齐由校验强制 ─────────────────────

func TestApproveFailsOnMisalignedExtractionKeys(t *testing.T) {
	aiShaped := map[string]any{
		// aiintake 的抽取键：lessee 而不是 lessee_name。盲 Unmarshal 会静默
		// 产出承租方为空的正式合同——这里必须具名失败而不是静默通过。
		"contract_number":   "CT-AI-1",
		"lease_scope":       "in_scope",
		"store_name":        "AI 店",
		"lessee":            "AI 抽的承租方",
		"lessor":            "AI 抽的出租方",
		"currency":          "CNY",
		"commencement_date": "2026-01-01",
		"lease_start_date":  "2026-01-01",
		"lease_end_date":    "2027-12-31",
	}
	reader := &fakeReader{rows: []repository.DraftReviewRow{
		draftRow(t, "d-misaligned", entityA, aiShaped, nil),
	}}
	store := newFakeStore()
	service := newHarness(reader, store)

	outcome, _ := service.Decide(reviewerCtx(entityA, "user-1"), []Decision{{DraftID: "d-misaligned", Approve: true}})
	item := outcome.Items[0]
	if item.Verdict != "failed" || !strings.Contains(item.Error, "missing lessee_name") {
		t.Fatalf("misaligned keys silently approved: %+v", item)
	}
	if len(store.created) != 0 {
		t.Fatalf("empty-lessee contract created: %d", len(store.created))
	}
}

func TestApproveAnchorsLegalEntityToRowNotPayload(t *testing.T) {
	payload := map[string]any{}
	for k, v := range completePayload {
		payload[k] = v
	}
	payload["legal_entity_id"] = entityB // 载荷声称属于法人 B
	reader := &fakeReader{rows: []repository.DraftReviewRow{
		draftRow(t, "d-anchor", entityA, payload, nil),
	}}
	store := newFakeStore()
	service := newHarness(reader, store)

	outcome, _ := service.Decide(reviewerCtx(entityA, "user-1"), []Decision{{DraftID: "d-anchor", Approve: true}})
	if outcome.Items[0].Verdict != "approved" {
		t.Fatalf("approve failed: %+v", outcome.Items[0])
	}
	if len(store.created) != 1 || store.created[0].LegalEntityID == nil || *store.created[0].LegalEntityID != entityA {
		t.Fatalf("legal entity not anchored to the row: %+v", store.created)
	}
}

// ── 决定语义的边界 ────────────────────────────────────────────────────────

func TestRejectRequiresReasonAndRecordsIt(t *testing.T) {
	reader := &fakeReader{rows: []repository.DraftReviewRow{
		draftRow(t, "d-rej", entityA, completePayload, nil),
	}}
	service := newHarness(reader, newFakeStore())

	outcome, _ := service.Decide(reviewerCtx(entityA, "user-1"), []Decision{{DraftID: "d-rej", Approve: false}})
	if outcome.Items[0].Verdict != "failed" || !strings.Contains(outcome.Items[0].Error, "reason") {
		t.Fatalf("reject without reason accepted: %+v", outcome.Items[0])
	}

	outcome, _ = service.Decide(reviewerCtx(entityA, "user-1"), []Decision{{DraftID: "d-rej", Approve: false, Reason: "面积与合同原件不符"}})
	if outcome.Items[0].Verdict != "rejected" {
		t.Fatalf("reject with reason failed: %+v", outcome.Items[0])
	}
	last := reader.updated[len(reader.updated)-1]
	if last.Status != "rejected" || last.RejectedReason != "面积与合同原件不符" {
		t.Fatalf("rejected reason not recorded: %+v", last)
	}
}

func TestStorageErrorIsSanitizedNotSoftenedToNoData(t *testing.T) {
	reader := &fakeReader{failOnGet: "d-x"}
	service := newHarness(reader, newFakeStore())

	outcome, _ := service.Decide(scopedCtx(entityA), []Decision{{DraftID: "d-x", Approve: true}})
	if outcome.Items[0].Verdict != "failed" {
		t.Fatalf("expected failure item: %+v", outcome.Items[0])
	}
	if strings.Contains(outcome.Items[0].Error, "no data") || strings.Contains(outcome.Items[0].Error, "not found") {
		t.Fatalf("permission/storage failure softened into data absence: %q", outcome.Items[0].Error)
	}
}
