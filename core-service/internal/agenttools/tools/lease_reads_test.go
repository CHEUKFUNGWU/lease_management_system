package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/money"
	"github.com/lease-management-system/core-service/internal/repository"
)

type fakeMeasurementReader struct {
	results []*repository.MeasurementResult
	calls   int
}

func (f *fakeMeasurementReader) GetMeasurementResults(context.Context, string, string) ([]*repository.MeasurementResult, error) {
	f.calls++
	return f.results, nil
}

type fakeEventReader struct {
	events []*repository.LeaseEvent
	calls  int
}

func (f *fakeEventReader) GetByContractID(context.Context, string) ([]*repository.LeaseEvent, error) {
	f.calls++
	return f.events, nil
}

type fakeJournalReader struct {
	entries []*repository.JournalEntry
	calls   int
}

func (f *fakeJournalReader) GetJournalEntries(context.Context, string, string, string) ([]*repository.JournalEntry, error) {
	f.calls++
	return f.entries, nil
}

func TestLeaseReadToolsDeclareValidReadOnlyContracts(t *testing.T) {
	contractReader := &fakeContractReader{}
	definitions := []agenttools.ToolDefinition{
		NewMeasurementListDefinition(contractReader, &fakeMeasurementReader{}),
		NewEventListDefinition(contractReader, &fakeEventReader{}),
		NewJournalListDefinition(contractReader, &fakeJournalReader{}),
	}
	for _, definition := range definitions {
		if err := definition.Descriptor.Validate(); err != nil {
			t.Fatalf("%s descriptor invalid: %v", definition.Descriptor.Name, err)
		}
		if definition.Descriptor.Level != agenttools.LevelRead || !definition.Descriptor.ReadOnly {
			t.Fatalf("%s is not read-only: %#v", definition.Descriptor.Name, definition.Descriptor)
		}
	}
}

func TestLeaseReadToolsCheckContractScopeBeforeLinkedReads(t *testing.T) {
	contractReader := &fakeContractReader{
		attributes: access.ContractAttributes{LegalEntityID: "le-001", StoreID: "store-foreign"},
		contract:   &repository.Contract{ID: "contract-foreign"},
	}
	measurementReader := &fakeMeasurementReader{}
	eventReader := &fakeEventReader{}
	journalReader := &fakeJournalReader{}
	ctx := contractToolContext(access.Scope{LegalEntityID: "le-001", StoreIDs: []string{"store-allowed"}})

	tests := []struct {
		name       string
		definition agenttools.ToolDefinition
		arguments  string
		calls      *int
	}{
		{name: "measurement", definition: NewMeasurementListDefinition(contractReader, measurementReader), arguments: `{"contract_id":"contract-foreign"}`, calls: &measurementReader.calls},
		{name: "event", definition: NewEventListDefinition(contractReader, eventReader), arguments: `{"contract_id":"contract-foreign"}`, calls: &eventReader.calls},
		{name: "journal", definition: NewJournalListDefinition(contractReader, journalReader), arguments: `{"contract_id":"contract-foreign"}`, calls: &journalReader.calls},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := test.definition.Handler(ctx, agenttools.ToolCall{
				CallID: "call-1", RunID: "run-1", ToolName: test.definition.Descriptor.Name,
				ToolVersion: test.definition.Descriptor.Version, Arguments: json.RawMessage(test.arguments),
			})
			if err != nil || result.Status != agenttools.StatusRejected || result.Error == nil || result.Error.Code != agenttools.ErrorNotFound {
				t.Fatalf("result=%#v err=%v", result, err)
			}
			if *test.calls != 0 {
				t.Fatalf("linked reader calls=%d, want 0", *test.calls)
			}
		})
	}
}

func TestLeaseReadToolsProjectNarrowViews(t *testing.T) {
	description := "monthly close"
	effectiveDate := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	contractReader := &fakeContractReader{
		attributes: access.ContractAttributes{LegalEntityID: "le-001"},
		contract:   &repository.Contract{ID: "contract-1"},
	}
	measurementReader := &fakeMeasurementReader{results: []*repository.MeasurementResult{{
		AccountingPeriod: "2026-01", OpeningLiability: money.NewFromInt64(100), ClosingLiability: money.NewFromInt64(90),
		InterestExpense: money.NewFromInt64(2), PrincipalRepayment: money.NewFromInt64(12), Depreciation: money.NewFromInt64(8), ClosingROUAsset: money.NewFromInt64(80),
	}}}
	eventReader := &fakeEventReader{events: []*repository.LeaseEvent{{
		EventType: "rent_change", EffectiveDate: effectiveDate, Status: "approved",
		ApprovalStatus: "approved", ChangeReason: &description,
	}}}
	journalReader := &fakeJournalReader{entries: []*repository.JournalEntry{{
		AccountingPeriod: "2026-01", EntryType: "interest", DebitAccount: "6601",
		CreditAccount: "2201", Amount: money.NewFromInt64(2), Currency: "CNY", PostingStatus: "posted",
		Description: &description,
	}}}
	ctx := agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{
			UserID: "user-1", Scope: access.Scope{LegalEntityID: "le-001"},
			Permissions: []string{"calculations:read", "events:read", "monthly_closing:read"},
		}, RunID: "run-1",
	})

	measurement := NewMeasurementListDefinition(contractReader, measurementReader)
	measurementResult, err := measurement.Handler(ctx, toolCallFor(measurement, `{"contract_id":"contract-1"}`))
	if err != nil || measurementResult.Status != agenttools.StatusCompleted {
		t.Fatalf("measurement result=%#v err=%v", measurementResult, err)
	}
	measurementData, ok := measurementResult.Data.(MeasurementListData)
	if !ok || measurementData.Total != 1 || measurementData.Items[0].ClosingLiability != 90 {
		t.Fatalf("measurement data=%#v", measurementResult.Data)
	}

	event := NewEventListDefinition(contractReader, eventReader)
	eventResult, err := event.Handler(ctx, toolCallFor(event, `{"contract_id":"contract-1"}`))
	if err != nil || eventResult.Status != agenttools.StatusCompleted {
		t.Fatalf("event result=%#v err=%v", eventResult, err)
	}
	eventData, ok := eventResult.Data.(EventListData)
	if !ok || eventData.Total != 1 || eventData.Items[0].ChangeReason != description {
		t.Fatalf("event data=%#v", eventResult.Data)
	}

	journal := NewJournalListDefinition(contractReader, journalReader)
	journalResult, err := journal.Handler(ctx, toolCallFor(journal, `{"contract_id":"contract-1"}`))
	if err != nil || journalResult.Status != agenttools.StatusCompleted {
		t.Fatalf("journal result=%#v err=%v", journalResult, err)
	}
	journalData, ok := journalResult.Data.(JournalListData)
	if !ok || journalData.Total != 1 || journalData.Items[0].Description != description {
		t.Fatalf("journal data=%#v", journalResult.Data)
	}
}

func toolCallFor(definition agenttools.ToolDefinition, arguments string) agenttools.ToolCall {
	return agenttools.ToolCall{
		CallID: "call-1", RunID: "run-1", ToolName: definition.Descriptor.Name,
		ToolVersion: definition.Descriptor.Version, Arguments: json.RawMessage(arguments),
	}
}
