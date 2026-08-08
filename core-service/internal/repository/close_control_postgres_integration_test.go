package repository_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/closecontrol"
)

func TestCloseControlRepositoryPersistsFingerprintAndLockDisposition(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to Postgres: %v", err)
	}
	t.Cleanup(pool.Close)
	contract := seedContract(t, ctx, pool)
	if contract.LegalEntityID == nil {
		t.Fatal("fixture has no legal entity")
	}
	if _, err := pool.Exec(ctx, `UPDATE lease_contracts SET approval_status = 'approved', is_official_version = true WHERE id = $1`, contract.ID); err != nil {
		t.Fatalf("approve fixture contract: %v", err)
	}

	repo := repository.NewCloseControlRepository(pool)
	now := time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC)
	detection := closecontrol.Detection{
		RuleCode: closecontrol.RuleMissingPaymentSchedule, RuleVersion: "v1", Severity: "blocking", GateEffect: "formal_calculation",
		AccountingPeriod: "2026-08", LegalEntityID: *contract.LegalEntityID, SubjectType: "contract", SubjectID: contract.ID,
		SubjectContractID: contract.ID, ProjectionVersion: closecontrol.ProjectionVersion,
		Fingerprint: closecontrol.Fingerprint("2026-08", *contract.LegalEntityID, closecontrol.RuleMissingPaymentSchedule, "contract", contract.ID),
		Evidence:    map[string]any{"contract_id": contract.ID}, DetectedAt: now,
	}
	exceptions, err := repo.PersistDetections(ctx, []closecontrol.Detection{detection, detection})
	if err != nil {
		t.Fatalf("PersistDetections(): %v", err)
	}
	if len(exceptions) != 2 || exceptions[0].ID != exceptions[1].ID {
		t.Fatalf("deduplicated exceptions = %#v", exceptions)
	}
	listed, err := repo.ListExceptions(ctx, "2026-08", *contract.LegalEntityID)
	if err != nil || len(listed) != 1 {
		t.Fatalf("ListExceptions() = %#v, err=%v", listed, err)
	}
	blocked, err := repo.HasUnresolvedBlocking(ctx, "2026-08", *contract.LegalEntityID)
	if err != nil || !blocked {
		t.Fatalf("HasUnresolvedBlocking() = %v, err=%v; want true", blocked, err)
	}

	closedAt := now.Add(time.Hour)
	if _, err := repo.UpdateException(ctx, listed[0].ID, closecontrol.ExceptionUpdate{
		ExceptionState: "closed", ClosingDisposition: closecontrol.DispositionVerifiedResolution,
		ResolutionNote: "已补充付款计划", ClosedAt: &closedAt,
	}); err != nil {
		t.Fatalf("UpdateException(): %v", err)
	}
	blocked, err = repo.HasUnresolvedBlocking(ctx, "2026-08", *contract.LegalEntityID)
	if err != nil || blocked {
		t.Fatalf("HasUnresolvedBlocking() after close = %v, err=%v; want false", blocked, err)
	}
}
