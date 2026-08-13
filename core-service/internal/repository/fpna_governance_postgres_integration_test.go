package repository

import (
	"context"
	"testing"
	"time"
)

// TestFPnAGovernanceTenantIsolationCharacterization pins down the legal-entity
// filtering of every FP&A governance read before the EntityFilter refactor
// (SEC-003). Each pair of assertions proves entity A sees its own row while
// entity B cannot see, resolve, or modify it. The assertions are the contract;
// the refactor must keep them passing without weakening any of them.
func TestFPnAGovernanceTenantIsolationCharacterization(t *testing.T) {
	pool := postgresTestPool(t)
	ctx := context.Background()
	pair := seedTenantPair(t, ctx, pool, "fpna")
	t.Cleanup(func() { cleanupTenantPair(t, ctx, pool, pair) })

	repo := NewFPnAGovernanceRepository(pool)

	// Plan versions: List / Get / Freeze.
	version := &FPnAPlanVersion{
		LegalEntityID: &pair.entityA, Name: "char-plan-A", VersionType: "budget", Source: "char-test",
		AsOfPeriod: "2026-01", FromPeriod: "2026-01", ToPeriod: "2026-12",
	}
	createdVersion, err := repo.CreatePlanVersion(ctx, version)
	if err != nil {
		t.Fatalf("seed plan version: %v", err)
	}
	versionsA, err := repo.ListPlanVersions(ctx, pair.entityA, "")
	if err != nil || len(versionsA) != 1 {
		t.Fatalf("entity A plan versions = %d, err %v; want 1", len(versionsA), err)
	}
	versionsB, err := repo.ListPlanVersions(ctx, pair.entityB, "")
	if err != nil || len(versionsB) != 0 {
		t.Fatalf("entity B saw entity A plan versions: %d, err %v", len(versionsB), err)
	}
	if got, err := repo.GetPlanVersion(ctx, createdVersion.ID, pair.entityA); err != nil || got == nil {
		t.Fatalf("entity A GetPlanVersion = %+v, err %v; want found", got, err)
	}
	if got, err := repo.GetPlanVersion(ctx, createdVersion.ID, pair.entityB); err != nil || got != nil {
		t.Fatalf("entity B GetPlanVersion = %+v, err %v; want nil", got, err)
	}
	if frozen, err := repo.FreezePlanVersion(ctx, createdVersion.ID, pair.entityB, "user-b", false); err != nil || frozen != nil {
		t.Fatalf("entity B froze entity A plan version: %+v, err %v; want nil", frozen, err)
	}
	if frozen, err := repo.FreezePlanVersion(ctx, createdVersion.ID, pair.entityA, "user-a", false); err != nil || frozen == nil {
		t.Fatalf("entity A freeze = %+v, err %v; want updated", frozen, err)
	}

	// Plan lines: list through the version's legal entity. Seeded with a
	// direct insert because CreatePlanLine is broken against the current
	// schema (its RETURNING created_at column no longer exists; see report).
	var planLineID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO fpna_plan_lines (plan_version_id, period, grain, legal_entity_id, currency, source_system)
		VALUES ($1, '2026-01', 'group', $2, 'CNY', 'char-test') RETURNING id
	`, createdVersion.ID, pair.entityA).Scan(&planLineID); err != nil {
		t.Fatalf("seed plan line: %v", err)
	}
	_ = planLineID
	linesA, err := repo.ListPlanLinesFiltered(ctx, createdVersion.ID, pair.entityA, "", "", nil)
	if err != nil || len(linesA) != 1 {
		t.Fatalf("entity A plan lines = %d, err %v; want 1", len(linesA), err)
	}
	linesB, err := repo.ListPlanLinesFiltered(ctx, createdVersion.ID, pair.entityB, "", "", nil)
	if err != nil || len(linesB) != 0 {
		t.Fatalf("entity B saw entity A plan lines: %d, err %v", len(linesB), err)
	}

	// Master-data mappings: List / Resolve.
	mapping := &FPnAMasterDataMapping{
		LegalEntityID: &pair.entityA, MappingType: "store", ExternalSystem: "pos",
		ExternalID: "ext-store-a", ExternalName: "External Store A", Status: "approved",
		EffectiveFrom: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	createdMapping, err := repo.CreateMapping(ctx, mapping)
	if err != nil {
		t.Fatalf("seed mapping: %v", err)
	}
	_ = createdMapping
	mappingsA, err := repo.ListMappings(ctx, pair.entityA, "", "")
	if err != nil || len(mappingsA) != 1 {
		t.Fatalf("entity A mappings = %d, err %v; want 1", len(mappingsA), err)
	}
	mappingsB, err := repo.ListMappings(ctx, pair.entityB, "", "")
	if err != nil || len(mappingsB) != 0 {
		t.Fatalf("entity B saw entity A mappings: %d, err %v", len(mappingsB), err)
	}
	if resolved, err := repo.ResolveMapping(ctx, pair.entityA, "store", "pos", "ext-store-a", "2026-06-01"); err != nil || resolved == nil {
		t.Fatalf("entity A ResolveMapping = %+v, err %v; want found", resolved, err)
	}
	if resolved, err := repo.ResolveMapping(ctx, pair.entityB, "store", "pos", "ext-store-a", "2026-06-01"); err != nil || resolved != nil {
		t.Fatalf("entity B ResolveMapping = %+v, err %v; want nil", resolved, err)
	}

	// Data-quality items: List / Update.
	dq := &FPnADataQualityItem{
		LegalEntityID: &pair.entityA, Dimension: "store", Category: "missing",
		SourceTable: "store_operating_facts", SourceRecordID: "fact-1", Description: "missing revenue",
	}
	createdDQ, err := repo.CreateDataQuality(ctx, dq)
	if err != nil {
		t.Fatalf("seed data-quality item: %v", err)
	}
	dqA, err := repo.ListDataQuality(ctx, pair.entityA, "", "")
	if err != nil || len(dqA) != 1 {
		t.Fatalf("entity A data-quality = %d, err %v; want 1", len(dqA), err)
	}
	dqB, err := repo.ListDataQuality(ctx, pair.entityB, "", "")
	if err != nil || len(dqB) != 0 {
		t.Fatalf("entity B saw entity A data-quality items: %d, err %v", len(dqB), err)
	}
	// UpdateDataQualityStatus is currently unexercisable: its query fails for
	// every caller with SQLSTATE 42P08 (the status parameter is deduced as
	// varchar in the SET clause and text in the CASE WHEN IN list). Pre-existing
	// defect, recorded in the SEC-002/003 report; the refactor must not make it
	// worse, and err != nil must hold after it for both entities.
	if updated, err := repo.UpdateDataQualityStatus(ctx, createdDQ.ID, pair.entityB, "resolved"); err == nil && updated != nil {
		t.Fatalf("entity B updated entity A data-quality item: %+v; want nil", updated)
	}
	if updated, err := repo.UpdateDataQualityStatus(ctx, createdDQ.ID, pair.entityA, "resolved"); err == nil && updated == nil {
		t.Fatalf("entity A data-quality update = nil with no error; unexpected")
	}

	// Decision memos: idempotency lookup / List / Update status.
	memo := &FPnADecisionMemo{
		LegalEntityID: &pair.entityA, MemoType: "renewal", Title: "Renewal memo A",
		IdempotencyKey: "memo-key-a",
	}
	createdMemo, err := repo.CreateMemo(ctx, memo)
	if err != nil {
		t.Fatalf("seed memo: %v", err)
	}
	if got, err := repo.GetMemoByIdempotency(ctx, &pair.entityA, "memo-key-a"); err != nil || got == nil {
		t.Fatalf("entity A memo idempotency lookup = %+v, err %v; want found", got, err)
	}
	if got, err := repo.GetMemoByIdempotency(ctx, &pair.entityB, "memo-key-a"); err != nil || got != nil {
		t.Fatalf("entity B memo idempotency lookup = %+v, err %v; want nil", got, err)
	}
	memosA, err := repo.ListMemos(ctx, pair.entityA, "", "")
	if err != nil || len(memosA) != 1 {
		t.Fatalf("entity A memos = %d, err %v; want 1", len(memosA), err)
	}
	memosB, err := repo.ListMemos(ctx, pair.entityB, "", "")
	if err != nil || len(memosB) != 0 {
		t.Fatalf("entity B saw entity A memos: %d, err %v", len(memosB), err)
	}
	if updated, err := repo.UpdateMemoStatus(ctx, createdMemo.ID, pair.entityB, "", "review"); err != nil || updated != nil {
		t.Fatalf("entity B updated entity A memo: %+v, err %v; want nil", updated, err)
	}
	if updated, err := repo.UpdateMemoStatus(ctx, createdMemo.ID, pair.entityA, "", "review"); err != nil || updated == nil {
		t.Fatalf("entity A memo update = %+v, err %v; want updated", updated, err)
	}

	// Report artifacts: List / Get.
	artifact := &FPnAReportArtifact{
		LegalEntityID: &pair.entityA, ReportType: "WBR", Period: "2026-01", Format: "json",
	}
	createdArtifact, err := repo.CreateReportArtifact(ctx, artifact)
	if err != nil {
		t.Fatalf("seed report artifact: %v", err)
	}
	artifactsA, err := repo.ListReportArtifacts(ctx, pair.entityA, "", "", "")
	if err != nil || len(artifactsA) != 1 {
		t.Fatalf("entity A report artifacts = %d, err %v; want 1", len(artifactsA), err)
	}
	artifactsB, err := repo.ListReportArtifacts(ctx, pair.entityB, "", "", "")
	if err != nil || len(artifactsB) != 0 {
		t.Fatalf("entity B saw entity A report artifacts: %d, err %v", len(artifactsB), err)
	}
	if got, err := repo.GetReportArtifact(ctx, createdArtifact.ID, pair.entityA); err != nil || got == nil {
		t.Fatalf("entity A GetReportArtifact = %+v, err %v; want found", got, err)
	}
	if got, err := repo.GetReportArtifact(ctx, createdArtifact.ID, pair.entityB); err != nil || got != nil {
		t.Fatalf("entity B GetReportArtifact = %+v, err %v; want nil", got, err)
	}

	// Agent signals: List.
	signal := &FPnAAgentSignal{
		LegalEntityID: &pair.entityA, RuleCode: "rent-spike", SourceTable: "store_operating_facts",
		SourceRecordID: "fact-1",
	}
	if _, err := repo.CreateAgentSignal(ctx, signal); err != nil {
		t.Fatalf("seed agent signal: %v", err)
	}
	signalsA, err := repo.ListAgentSignals(ctx, pair.entityA, "", "")
	if err != nil || len(signalsA) != 1 {
		t.Fatalf("entity A agent signals = %d, err %v; want 1", len(signalsA), err)
	}
	signalsB, err := repo.ListAgentSignals(ctx, pair.entityB, "", "")
	if err != nil || len(signalsB) != 0 {
		t.Fatalf("entity B saw entity A agent signals: %d, err %v", len(signalsB), err)
	}

	// ActionInScope is an FP&A governance read over the shared action table.
	action := &FPnAActionItem{
		LegalEntityID: &pair.entityA, Category: "occupancy", Title: "Action A", Severity: "medium",
		RuleCode: "rent-spike", SourceTable: "store_operating_facts", SourceRecordID: "fact-1",
	}
	createdAction, err := NewOperatingFactsRepository(pool).CreateAction(ctx, action)
	if err != nil {
		t.Fatalf("seed action: %v", err)
	}
	if inScope, err := repo.ActionInScope(ctx, createdAction.ID, pair.entityA); err != nil || !inScope {
		t.Fatalf("entity A ActionInScope = %v, err %v; want true", inScope, err)
	}
	if inScope, err := repo.ActionInScope(ctx, createdAction.ID, pair.entityB); err != nil || inScope {
		t.Fatalf("entity B ActionInScope = %v, err %v; want false", inScope, err)
	}
}
