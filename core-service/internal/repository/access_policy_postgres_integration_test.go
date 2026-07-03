package repository

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lease-management-system/core-service/internal/access"
)

func TestAccessScopePostgresFiltersContractsJournalsAndAudit(t *testing.T) {
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

	fixture := createAccessFixture(t, ctx, pool)
	scoped := access.WithScope(ctx, access.Scope{
		LegalEntityID: fixture.allowedLegalEntity,
		StoreIDs:      []string{fixture.allowedStore},
	})

	contracts, err := NewContractRepository(pool).GetAll(scoped, "")
	if err != nil {
		t.Fatalf("GetAll(): %v", err)
	}
	if len(contracts) != 1 || contracts[0].ID != fixture.allowedContract {
		t.Fatalf("scoped contracts = %#v", contracts)
	}

	closing := NewMonthlyClosingRepository(pool)
	entries, err := closing.GetJournalEntries(scoped, "", "2098-01", "draft")
	if err != nil {
		t.Fatalf("GetJournalEntries(): %v", err)
	}
	if len(entries) != 1 || entries[0].ID != fixture.allowedEntry {
		t.Fatalf("scoped journal entries = %#v", entries)
	}
	if err := closing.ApproveJournalEntry(scoped, fixture.deniedEntry, fixture.userID); err == nil {
		t.Fatal("out-of-scope journal approval succeeded")
	}

	auditLogs, total, err := NewAuditRepository(pool).List(scoped, AuditLogFilter{})
	if err != nil {
		t.Fatalf("List audit logs: %v", err)
	}
	if total != 1 || len(auditLogs) != 1 || auditLogs[0].LegalEntityID == nil || *auditLogs[0].LegalEntityID != fixture.allowedLegalEntity {
		t.Fatalf("scoped audit logs total=%d logs=%#v", total, auditLogs)
	}
}

type accessFixture struct {
	allowedLegalEntity string
	deniedLegalEntity  string
	allowedStore       string
	deniedStore        string
	allowedLandlord    string
	deniedLandlord     string
	allowedContract    string
	deniedContract     string
	allowedEntry       string
	deniedEntry        string
	userID             string
}

func createAccessFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool) accessFixture {
	t.Helper()
	suffix := uuid.NewString()[:8]
	fixture := accessFixture{
		allowedLegalEntity: uuid.NewString(), deniedLegalEntity: uuid.NewString(),
		allowedStore: uuid.NewString(), deniedStore: uuid.NewString(),
		allowedLandlord: uuid.NewString(), deniedLandlord: uuid.NewString(),
		allowedContract: uuid.NewString(), deniedContract: uuid.NewString(),
		allowedEntry: uuid.NewString(), deniedEntry: uuid.NewString(), userID: uuid.NewString(),
	}
	statements := []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO legal_entities (id, code, name, country, currency) VALUES ($1, $2, $3, 'CN', 'CNY'), ($4, $5, $6, 'CN', 'CNY')`, []any{fixture.allowedLegalEntity, "AP-A-" + suffix, "Allowed " + suffix, fixture.deniedLegalEntity, "AP-D-" + suffix, "Denied " + suffix}},
		{`INSERT INTO stores (id, code, name, legal_entity_id, region, brand) VALUES ($1, $2, $3, $4, 'east', 'brand-a'), ($5, $6, $7, $8, 'west', 'brand-b')`, []any{fixture.allowedStore, "AP-AS-" + suffix, "Allowed Store " + suffix, fixture.allowedLegalEntity, fixture.deniedStore, "AP-DS-" + suffix, "Denied Store " + suffix, fixture.deniedLegalEntity}},
		{`INSERT INTO landlords (id, code, name) VALUES ($1, $2, $3), ($4, $5, $6)`, []any{fixture.allowedLandlord, "AP-AL-" + suffix, "Allowed Landlord " + suffix, fixture.deniedLandlord, "AP-DL-" + suffix, "Denied Landlord " + suffix}},
		{`INSERT INTO users (id, username, email, password_hash, role, legal_entity_id, is_active) VALUES ($1, $2, $3, 'integration-only', 'approver', $4, true)`, []any{fixture.userID, "ap-user-" + suffix, "ap-" + suffix + "@example.com", fixture.allowedLegalEntity}},
		{`INSERT INTO lease_contracts (id, contract_number, contract_name, legal_entity_id, store_id, landlord_id, commencement_date, lease_start_date, lease_end_date, currency, status, approval_status, lease_scope)
		 VALUES ($1, $2, $3, $4, $5, $6, '2097-01-01', '2097-01-01', '2099-01-01', 'CNY', 'active', 'approved', 'in_scope'),
		        ($7, $8, $9, $10, $11, $12, '2097-01-01', '2097-01-01', '2099-01-01', 'CNY', 'active', 'approved', 'in_scope')`, []any{fixture.allowedContract, "AP-AC-" + suffix, "Allowed Contract " + suffix, fixture.allowedLegalEntity, fixture.allowedStore, fixture.allowedLandlord, fixture.deniedContract, "AP-DC-" + suffix, "Denied Contract " + suffix, fixture.deniedLegalEntity, fixture.deniedStore, fixture.deniedLandlord}},
		{`INSERT INTO journal_entries (id, contract_id, accounting_period, entry_date, entry_type, debit_account, credit_account, amount, currency, posting_status)
		 VALUES ($1, $2, '2098-01', '2098-01-31', 'interest', '6601', '2801', 10, 'CNY', 'draft'),
		        ($3, $4, '2098-01', '2098-01-31', 'interest', '6601', '2801', 20, 'CNY', 'draft')`, []any{fixture.allowedEntry, fixture.allowedContract, fixture.deniedEntry, fixture.deniedContract}},
		{`INSERT INTO audit_logs (id, table_name, record_id, legal_entity_id, action, changed_at) VALUES ($1, 'lease_contracts', $2, $3, 'test', NOW()), ($4, 'lease_contracts', $5, $6, 'test', NOW())`, []any{uuid.NewString(), fixture.allowedContract, fixture.allowedLegalEntity, uuid.NewString(), fixture.deniedContract, fixture.deniedLegalEntity}},
	}
	for _, statement := range statements {
		if _, err := pool.Exec(ctx, statement.sql, statement.args...); err != nil {
			t.Fatalf("create access fixture: %v", err)
		}
	}
	t.Cleanup(func() {
		for _, statement := range []struct {
			sql  string
			args []any
		}{
			{`DELETE FROM audit_logs WHERE record_id IN ($1, $2)`, []any{fixture.allowedContract, fixture.deniedContract}},
			{`DELETE FROM journal_entries WHERE id IN ($1, $2)`, []any{fixture.allowedEntry, fixture.deniedEntry}},
			{`DELETE FROM lease_contracts WHERE id IN ($1, $2)`, []any{fixture.allowedContract, fixture.deniedContract}},
			{`DELETE FROM users WHERE id = $1`, []any{fixture.userID}},
			{`DELETE FROM stores WHERE id IN ($1, $2)`, []any{fixture.allowedStore, fixture.deniedStore}},
			{`DELETE FROM landlords WHERE id IN ($1, $2)`, []any{fixture.allowedLandlord, fixture.deniedLandlord}},
			{`DELETE FROM legal_entities WHERE id IN ($1, $2)`, []any{fixture.allowedLegalEntity, fixture.deniedLegalEntity}},
		} {
			_, _ = pool.Exec(context.Background(), statement.sql, statement.args...)
		}
	})
	return fixture
}
