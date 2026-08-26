package ecomintake

import (
	"context"
	"strings"
	"testing"
	"time"
)

func csvBytes(header string, rows ...string) []byte {
	out := header + "\n" + strings.Join(rows, "\n") + "\n"
	return []byte(out)
}

func baseSpec() ImportSpec {
	return ImportSpec{
		LegalEntityID:   "LE-1",
		StorefrontID:    "SF-1",
		Filename:        "f.csv",
		Data:            csvBytes("business_date,channel,sku,currency,gmv_amount", "2026-08-01,direct,,USD,1000"),
		Source:          SourceShopify,
		TemplateVersion: "1",
		IdempotencyKey:  "req-1",
		Envelope: EnvelopeSpec{
			SourceSystem:       "shopify",
			AsOfAt:             time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC),
			DataClassification: "simulated",
			SimulationDatasetVersion: "ecom-sim-v1",
		},
	}
}

// failingSink 一旦被调用即 fail——证明信封/template 失败发生在建批次与落库之前。
type failingSink struct{ t *testing.T }

func (f failingSink) BeginBatch(context.Context, ImportSpec, int) (*BatchInfo, bool, error) {
	f.t.Fatal("BeginBatch 不该被调用")
	return nil, false, nil
}
func (f failingSink) FinalizeBatch(context.Context, string, int, int, string, string) error {
	f.t.Fatal("FinalizeBatch 不该被调用")
	return nil
}
func (f failingSink) CommitChunk(context.Context, ImportSpec, string, []ParsedRow, string, string) (*CommitResult, error) {
	f.t.Fatal("CommitChunk 不该被调用")
	return nil, nil
}
func (f failingSink) RegisterInvoiceHeaders(context.Context, ImportSpec, []InvoiceHeader) error {
	f.t.Fatal("RegisterInvoiceHeaders 不该被调用")
	return nil
}

func assertions(t *testing.T, err error, kind ImportErrorKind, contain string) {
	t.Helper()
	if err == nil {
		t.Fatalf("必须拒绝：%s", contain)
	}
	importErr, ok := err.(*ImportError)
	if !ok {
		t.Fatalf("必须返回 ImportError：%v", err)
	}
	if importErr.Kind != kind {
		t.Fatalf("失败类别应 %s 实际 %s：%v", kind, importErr.Kind, err)
	}
	if !strings.Contains(importErr.Message, contain) {
		t.Fatalf("错误信息必须包含 %q：%v", contain, err)
	}
}

// R-E1-1：信封缺任一字段整批拒绝（fail-closed）
func TestEnvelopeIncompleteRejectsWholeBatch(t *testing.T) {
	cases := []struct {
		name     string
		mutate   func(*ImportSpec)
		contains string
	}{
		{"source_system", func(s *ImportSpec) { s.Envelope.SourceSystem = "" }, "source_system 必填"},
		{"as_of_at", func(s *ImportSpec) { s.Envelope.AsOfAt = time.Time{} }, "as_of_at 必填"},
		{"classification", func(s *ImportSpec) { s.Envelope.DataClassification = "" }, "data_classification 必填"},
		{"simulated 缺 dataset version", func(s *ImportSpec) { s.Envelope.SimulationDatasetVersion = "" }, "simulation_dataset_version"},
		{"production 带 dataset version", func(s *ImportSpec) {
			s.Envelope.DataClassification = "production"
			s.Envelope.SimulationDatasetVersion = "ecom-sim-v1"
		}, "production 数据不得带"},
		{"非法 classification", func(s *ImportSpec) { s.Envelope.DataClassification = "sandbox" }, "production|simulated"},
		{"缺法人", func(s *ImportSpec) { s.LegalEntityID = "" }, "legal_entity_id 与 storefront_id 必填"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := baseSpec()
			tc.mutate(&spec)
			_, err := Preview(spec)
			assertions(t, err, FailureEnvelope, tc.contains)
			// 同批不落任何一行（fail-closed）：IngestBatch 在建批次前就拒绝
			_, err = IngestBatch(context.Background(), spec, failingSink{t: t})
			assertions(t, err, FailureEnvelope, tc.contains)
		})
	}
}

// R-E1-4：旧模板版本被拒并提示当前版本
func TestOldTemplateVersionRejectedWithCurrent(t *testing.T) {
	spec := baseSpec()
	spec.TemplateVersion = "0"
	_, err := Preview(spec)
	assertions(t, err, FailureTemplate, "已过时")
	if !strings.Contains(err.Error(), "当前版本 \"1\"") && !strings.Contains(err.Error(), `当前版本 "1"`) {
		t.Fatalf("必须提示当前版本：%v", err)
	}
	// 信封字段一个不少：标准模板只含数据列 + 请求级信封参数
	data, err := TemplateCSV(SourceShopify)
	if err != nil {
		t.Fatal(err)
	}
	header := strings.Split(strings.TrimSpace(string(data)), ",")[0]
	if header != "business_date" {
		t.Fatalf("模板首列应为 business_date：%q", header)
	}
}

func TestUnknownSourceAndUnknownColumnRejected(t *testing.T) {
	spec := baseSpec()
	spec.Source = "amazon"
	_, err := Preview(spec)
	assertions(t, err, FailureTemplate, "未知来源")

	spec = baseSpec()
	spec.Data = csvBytes("business_date,channel,sku,currency,gmv_amount,evil_column", "2026-08-01,direct,,USD,1000,1")
	_, err = Preview(spec)
	assertions(t, err, FailureParse, "未知列")
}

func TestMissingRequiredColumnRejected(t *testing.T) {
	spec := baseSpec()
	spec.Data = csvBytes("business_date,channel,sku,currency", "2026-08-01,direct,,")
	_, err := Preview(spec)
	assertions(t, err, FailureParse, "缺少必需列")
}

func TestRowErrorsAndDuplicateInFile(t *testing.T) {
	spec := baseSpec()
	spec.Data = csvBytes(
		"business_date,channel,sku,currency,gmv_amount",
		"2026-08-01,direct,,USD,1000",  // 合法
		"2026-08-01,direct,,USD,abc",   // 非法数值
		"2026-08-01,direct,,USD,2000",  // 与第 2 行业务键重复
		"2026-08-02,direct,,USD,9999",  // 合法（不同业务键）
	)
	report, err := Preview(spec)
	if err != nil {
		t.Fatalf("行级错误不该整批拒绝：%v", err)
	}
	seenCodes := map[string]bool{}
	for _, e := range report.Errors {
		seenCodes[e.Code] = true
	}
	if !seenCodes["invalid_value"] {
		t.Fatalf("非法数值必须报 invalid_value：%+v", report.Errors)
	}
	if !seenCodes["duplicate_in_file"] {
		t.Fatalf("同文件重复业务键必须拒绝：%+v", report.Errors)
	}
	if report.AcceptedRows != 2 {
		t.Fatalf("合法行仍应可导入：%d %+v", report.AcceptedRows, report.Errors)
	}
}

func TestBusinessKeysPerSource(t *testing.T) {
	cases := []struct {
		source Source
		values map[string]any
		want   string
	}{
		{SourceShopify, map[string]any{"business_date": "2026-08-01", "channel": "direct", "sku": ""}, "2026-08-01|direct|"},
		{SourceAdsBooked, map[string]any{"business_date": "2026-08-01", "campaign_id": "c1"}, "2026-08-01|c1"},
		{SourceAdInvoice, map[string]any{"invoice_no": "INV-1", "business_date": "2026-08-01", "campaign_id": "c1"}, "INV-1|2026-08-01|c1"},
		{SourceSettlement, map[string]any{"provider": "paypal", "payout_id": "P-1"}, "paypal|P-1"},
		{SourceBank, map[string]any{"bank_ref": "B-1"}, "B-1"},
		{SourceGLRevenue, map[string]any{"period": "2026-08"}, "2026-08"},
	}
	for _, tc := range cases {
		if got := BusinessKey(tc.source, tc.values); got != tc.want {
			t.Fatalf("%s 业务键 %q != %q", tc.source, got, tc.want)
		}
	}
}

func TestAdInvoicePaidBasisValidation(t *testing.T) {
	// paid 口径必须来自代理发票（invoice_no 必填）
	spec := baseSpec()
	spec.Source = SourceAdInvoice
	spec.TemplateVersion = "1"
	spec.Data = csvBytes(
		"invoice_no,invoice_date,agent_name,media_owner,period_start,period_end,gross_amount,rebate_amount,payable_amount,business_date,campaign_id,currency,spend_amount",
		"INV-1,2026-08-01,AgentA,meta,2026-08-01,2026-08-31,1000,100,900,2026-08-05,c1,USD,100",
		",2026-08-01,AgentA,meta,,,,,,2026-08-06,c2,USD,50", // 缺发票号 → 行错误
	)
	report, err := Preview(spec)
	if err != nil {
		t.Fatalf("行级错误不该整批拒绝：%v", err)
	}
	if report.AcceptedRows != 1 {
		t.Fatalf("只有带发票号的行可导入：%+v", report.Errors)
	}
}
