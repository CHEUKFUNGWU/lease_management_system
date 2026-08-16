package retailingest

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/repository"
)

type fakeDirectory struct{ stores []StoreRef }

func (d fakeDirectory) Stores(context.Context, string) ([]StoreRef, error) { return d.stores, nil }

type fakeUpsertCall struct {
	key   string
	sha   string
	facts []*repository.RetailStoreDayFact
}

type fakeSink struct {
	state     map[string]int
	calls     []fakeUpsertCall
	replayOn  map[string]bool
	failOnKey string
}

func (s *fakeSink) ExistingState(context.Context, string, string, []string, []string) (map[string]int, error) {
	return s.state, nil
}

func (s *fakeSink) UpsertChunk(_ context.Context, chunk []*repository.RetailStoreDayFact, key, sha string) (*repository.RetailStoreDayFactWriteResult, error) {
	if s.failOnKey != "" && s.failOnKey == key {
		return nil, errors.New("sink failure")
	}
	s.calls = append(s.calls, fakeUpsertCall{key: key, sha: sha, facts: chunk})
	return &repository.RetailStoreDayFactWriteResult{Facts: chunk, Replayed: s.replayOn[key]}, nil
}

func sampleService(state map[string]int) (*Service, *fakeSink) {
	sink := &fakeSink{state: state}
	directory := fakeDirectory{stores: []StoreRef{
		{StoreID: "11111111-1111-1111-1111-111111111111", StoreCode: "S001", StoreName: "一号店"},
		{StoreID: "22222222-2222-2222-2222-222222222222", StoreCode: "S002", StoreName: "二号店"},
	}}
	return NewService(directory, sink), sink
}

func sampleCSV(rows int) []byte {
	builder := strings.Builder{}
	builder.WriteString("门店编号,日期,币种,营业额,毛利\n")
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < rows; i++ {
		builder.WriteString(fmt.Sprintf("S001,%s,CNY,%d.50,%d\n", start.AddDate(0, 0, i).Format("2006-01-02"), 100+i, 30+i))
	}
	return []byte(builder.String())
}

func sampleMapping() Mapping {
	return Mapping{"门店编号": FieldStore, "日期": FieldBusinessDate, "币种": FieldCurrency, "营业额": FieldRevenue, "毛利": FieldGrossProfit}
}

func TestParseTemplateCSVTrimsAndDropsEmptyRows(t *testing.T) {
	file := []byte("门店编号, 日期 ,币种,营业额\nS001,2026-07-01,CNY,100\n\nS001,2026-07-02,CNY,101\n,,,\n")
	headers, rows, err := ParseTemplate(file, FormatCSV)
	if err != nil {
		t.Fatal(err)
	}
	if len(headers) != 4 || headers[1] != "日期" {
		t.Fatalf("headers=%v", headers)
	}
	if len(rows) != 2 || rows[0][0] != "S001" || rows[1][3] != "101" {
		t.Fatalf("rows=%v", rows)
	}
}

func TestParseTemplateRejectsEmptyTemplates(t *testing.T) {
	if _, _, err := ParseTemplate([]byte(""), FormatCSV); err == nil {
		t.Fatal("empty file accepted")
	}
	if _, _, err := ParseTemplate([]byte("a,b,c\n"), FormatCSV); err == nil {
		t.Fatal("header-only file accepted")
	}
	if _, _, err := ParseTemplate([]byte("a,b\n1,2\n"), Format("pdf")); err == nil {
		t.Fatal("unknown format accepted")
	}
}

func TestSuggestMappingUsesAliasesFirstWins(t *testing.T) {
	headers := []string{"门店编号", "门店名称", "日期", "币种", "销售额", "毛利"}
	mapping := SuggestMapping(headers)
	if mapping["门店编号"] != FieldStore || mapping["日期"] != FieldBusinessDate ||
		mapping["币种"] != FieldCurrency || mapping["销售额"] != FieldRevenue || mapping["毛利"] != FieldGrossProfit {
		t.Fatalf("mapping=%v", mapping)
	}
	// Both store columns normalize to the store field; the first wins and the
	// second stays unmapped rather than shadowing the confirmed choice.
	if _, shadowed := mapping["门店名称"]; shadowed {
		t.Fatalf("second store column also mapped: %v", mapping)
	}
}

func TestColumnProfilesMaskDigits(t *testing.T) {
	headers := []string{"营业额", "日期"}
	rows := [][]string{{"123.45", "2026-07-01"}, {"67", "2026-07-02"}, {"", ""}}
	profiles := ColumnProfiles(headers, rows)
	if profiles[0].NonEmpty != 2 || profiles[0].Numeric != 2 || profiles[0].DateLike != 0 {
		t.Fatalf("revenue profile=%+v", profiles[0])
	}
	if profiles[0].MaskedSample != "###.##" {
		t.Fatalf("masked sample=%q leaks digits", profiles[0].MaskedSample)
	}
	if profiles[1].DateLike != 2 {
		t.Fatalf("date profile=%+v", profiles[1])
	}
}

func TestResolveStoresByCodeNameAndUUID(t *testing.T) {
	service, _ := sampleService(nil)
	headers := []string{"门店编号"}
	rows := [][]string{{"S001"}, {"二号店"}, {"11111111-1111-1111-1111-111111111111"}, {"NOPE"}}
	resolution, err := service.ResolveStores(context.Background(), "entity-a", Mapping{"门店编号": FieldStore}, headers, rows)
	if err != nil {
		t.Fatal(err)
	}
	if len(resolution.Unmatched) != 1 || resolution.Unmatched[0] != "NOPE" {
		t.Fatalf("unmatched=%v", resolution.Unmatched)
	}
	if id, ok := resolution.Resolved("S001"); !ok || id != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("code resolution=%q %v", id, ok)
	}
	if id, ok := resolution.Resolved("二号店"); !ok || id != "22222222-2222-2222-2222-222222222222" {
		t.Fatalf("name resolution=%q %v", id, ok)
	}
	if id, ok := resolution.Resolved("11111111-1111-1111-1111-111111111111"); !ok || id != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("uuid resolution=%q %v", id, ok)
	}
}

func sampleResolution() StoreResolution {
	return StoreResolution{
		RawToStoreID: map[string]string{"s001": "11111111-1111-1111-1111-111111111111"},
		StoreByID: map[string]StoreRef{"11111111-1111-1111-1111-111111111111": {StoreID: "11111111-1111-1111-1111-111111111111"}},
	}
}

func TestValidateReportsRowErrorsAndPartialSuccess(t *testing.T) {
	headers := []string{"门店编号", "日期", "币种", "营业额"}
	rows := [][]string{
		{"S001", "2026-07-01", "CNY", "100"},
		{"S001", "2026/07/02", "cny", "1,234.50"},  // slash date, lowercase currency, comma amount
		{"S001", "31/07/2026", "CNY", "100"},       // bad date layout
		{"S001", "2026-07-04", "CNY", "-5"},        // negative revenue
		{"S001", "2026-07-01", "CNY", "100"},       // duplicate (store, date) in file
		{"NOPE", "2026-07-06", "CNY", "100"},       // unmatched store
		{"S001", "2026-07-07", "CNY", ""},          // missing revenue
	}
	report := Validate(headers, rows, sampleMapping(), sampleResolution())
	if report.TotalRows != 7 || report.ValidRows != 2 {
		t.Fatalf("total=%d valid=%d errors=%+v", report.TotalRows, report.ValidRows, report.Errors)
	}
	codes := map[string]bool{}
	for _, rowErr := range report.Errors {
		codes[rowErr.Code] = true
	}
	for _, want := range []string{"bad_date", "negative_value", "duplicate_in_file", "unmatched_store", "missing_required"} {
		if !codes[want] {
			t.Fatalf("missing error code %q in %+v", want, report.Errors)
		}
	}
	if report.Facts[1].Revenue != 1234.5 {
		t.Fatalf("comma amount not parsed: %+v", report.Facts[1])
	}
	if report.Facts[1].BusinessDate != "2026-07-02" || report.Facts[1].Currency != "CNY" {
		t.Fatalf("normalized values: %+v", report.Facts[1])
	}
}

func TestValidateFlagsAmbiguousAndMissingMappings(t *testing.T) {
	headers := []string{"门店编号", "门店代码", "日期", "营业额"}
	rows := [][]string{{"S001", "S001", "2026-07-01", "100"}}
	ambiguous := Mapping{"门店编号": FieldStore, "门店代码": FieldStore, "日期": FieldBusinessDate, "营业额": FieldRevenue}
	report := Validate(headers, rows, ambiguous, sampleResolution())
	if len(report.AmbiguousMappings) != 1 || !strings.HasPrefix(report.AmbiguousMappings[0], "store=") {
		t.Fatalf("ambiguous=%v", report.AmbiguousMappings)
	}
	if len(report.MissingFields) != 1 || report.MissingFields[0] != FieldCurrency {
		t.Fatalf("missing=%v", report.MissingFields)
	}
}

func TestCommitRefusesIncompleteEnvelope(t *testing.T) {
	service, _ := sampleService(nil)
	_, rows, _ := ParseTemplate(sampleCSV(2), FormatCSV)
	_, err := service.Commit(context.Background(), "entity-a", nil, rows, nil, StoreResolution{}, Envelope{SourceSystem: "pos"}, "key")
	if !errors.Is(err, ErrEnvelopeIncomplete) || !strings.Contains(err.Error(), "import_batch_id") {
		t.Fatalf("err=%v", err)
	}
}

func TestCommitRefusesWhenEveryRowFails(t *testing.T) {
	service, _ := sampleService(nil)
	headers := []string{"门店编号", "日期", "币种", "营业额"}
	rows := [][]string{{"NOPE", "2026-07-01", "CNY", "100"}}
	report, err := service.Commit(context.Background(), "entity-a", headers, rows, sampleMapping(), sampleResolution(), Envelope{SourceSystem: "pos", ImportBatchID: "b1", AsOfAt: time.Now()}, "key")
	if !errors.Is(err, ErrNoValidRows) || report.RejectedRows != 1 {
		t.Fatalf("err=%v report=%+v", err, report)
	}
}

func TestCommitAssignsSupersedingVersionsAndEnvelope(t *testing.T) {
	service, sink := sampleService(map[string]int{"11111111-1111-1111-1111-111111111111|2026-01-01": 3})
	headers, rows, _ := ParseTemplate(sampleCSV(2), FormatCSV)
	asOf := time.Date(2026, 8, 16, 8, 0, 0, 0, time.UTC)
	report, err := service.Commit(context.Background(), "entity-a", headers, rows, sampleMapping(), sampleResolution(), Envelope{SourceSystem: "pos", ImportBatchID: "batch-1", AsOfAt: asOf}, "idem-1")
	if err != nil {
		t.Fatal(err)
	}
	if report.AcceptedRows != 2 || report.Chunks != 1 || len(sink.calls) != 1 {
		t.Fatalf("report=%+v calls=%d", report, len(sink.calls))
	}
	facts := sink.calls[0].facts
	if facts[0].Version != 4 { // existing max 3 superseded
		t.Fatalf("overlap version=%d, want 4", facts[0].Version)
	}
	if facts[1].Version != 1 {
		t.Fatalf("fresh version=%d, want 1", facts[1].Version)
	}
	if report.SupersededStoreDays != 1 || report.NewStoreDays != 1 {
		t.Fatalf("superseded/new=%d/%d", report.SupersededStoreDays, report.NewStoreDays)
	}
	for _, fact := range facts {
		if fact.DataClassification != "production" || fact.SourceSystem != "pos" {
			t.Fatalf("classification/source=%+v", fact)
		}
		if fact.ImportBatchID == nil || *fact.ImportBatchID != "batch-1" {
			t.Fatalf("import batch missing: %+v", fact)
		}
		if !fact.AsOfAt.Equal(asOf.UTC()) {
			t.Fatalf("as_of_at=%v", fact.AsOfAt)
		}
		if fact.MappingStatus != "mapped" || fact.DataQualityStatus != "valid" {
			t.Fatalf("statuses=%+v", fact)
		}
	}
}

func TestCommitChunksAndDerivesPerChunkIdempotency(t *testing.T) {
	service, sink := sampleService(nil)
	headers, rows, _ := ParseTemplate(sampleCSV(MaxChunkRows+1), FormatCSV)
	report, err := service.Commit(context.Background(), "entity-a", headers, rows, sampleMapping(), sampleResolution(), Envelope{SourceSystem: "pos", ImportBatchID: "b", AsOfAt: time.Now()}, "idem-2")
	if err != nil {
		t.Fatal(err)
	}
	if report.Chunks != 2 || len(sink.calls) != 2 {
		t.Fatalf("chunks=%d calls=%d", report.Chunks, len(sink.calls))
	}
	if sink.calls[0].key != "idem-2#chunk:0" || sink.calls[1].key != "idem-2#chunk:1" {
		t.Fatalf("chunk keys=%q %q", sink.calls[0].key, sink.calls[1].key)
	}
	if sink.calls[0].sha == "" || sink.calls[0].sha == sink.calls[1].sha {
		t.Fatalf("chunk payload hashes must be present and distinct: %q %q", sink.calls[0].sha, sink.calls[1].sha)
	}
	if report.AcceptedRows != MaxChunkRows+1 {
		t.Fatalf("accepted=%d", report.AcceptedRows)
	}
}

func TestCommitPropagatesReplayFlag(t *testing.T) {
	service, sink := sampleService(nil)
	sink.replayOn = map[string]bool{"idem-3#chunk:0": true}
	headers, rows, _ := ParseTemplate(sampleCSV(2), FormatCSV)
	report, err := service.Commit(context.Background(), "entity-a", headers, rows, sampleMapping(), sampleResolution(), Envelope{SourceSystem: "pos", ImportBatchID: "b", AsOfAt: time.Now()}, "idem-3")
	if err != nil {
		t.Fatal(err)
	}
	if !report.ReplayDetected {
		t.Fatal("replay flag not propagated")
	}
}
