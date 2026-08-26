package gateway

import (
	"context"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
)

// ── Ch3a integration evidence (real DB) ────────────────────────────────────
//
// 验收 1 的关键不是「Resolve 能跑」，而是「渠道产出的 Scope 与同一用户走 JWT
// 中间件链的产出逐字段相等」——「同一个解析器」的可执行证明。skip 掉的集成
// 测试不构成证据。

type gatewayFixture struct {
	entityA, entityB  string
	userA, userB      string
	storeA            string
	taskID            string
	draftB            string // 法人 B 的草稿，越权探测目标
	bindingExternalID string // feishu:open-A → userA
}

func TestResolveMatchesJWTChainOnRealDB(t *testing.T) {
	pool := gatewayTestPool(t)
	ctx := context.Background()
	fixture := seedGatewayFixture(t, ctx, pool)
	t.Cleanup(func() { cleanupGatewayFixture(t, ctx, pool, fixture) })

	bindings := repository.NewChannelIdentityBindingRepository(pool)
	resolver := NewIdentityResolver(bindings, repository.NewUserRepository(pool), repository.NewRoleRepository(pool))

	principal, err := resolver.Resolve(ctx, ChannelRef{Channel: "feishu", ExternalUserID: fixture.bindingExternalID})
	if err != nil {
		t.Fatalf("resolve bound identity: %v", err)
	}
	if principal.UserID != fixture.userA {
		t.Fatalf("resolved to %s, want %s", principal.UserID, fixture.userA)
	}
	if principal.SubjectType != "channel_identity" || principal.AgentMode != "assist" {
		t.Fatalf("principal shape wrong: %+v", principal)
	}

	// 验收 1：同一用户走 JWT 中间件链（真 repo、真中间件），Scope 逐字段比对。
	jwtScope := runJWTMiddlewareChain(t, pool, fixture.userA, entityOfUserA(t, ctx, pool, fixture))
	compareScopes(t, "channel-vs-jwt", principal.Scope, jwtScope)

	// 权限列表也必须与中间件产物一致。
	c := ginTestContext(t, fixture.userA, entityOfUserA(t, ctx, pool, fixture))
	middleware.LoadUserPermissions(repository.NewRoleRepository(pool))(c)
	middleware.DataScopeMiddleware()(c)
	permsValue, _ := c.Get("permissions")
	wantPerms, _ := permsValue.([]string)
	if len(wantPerms) == 0 {
		t.Fatal("fixture produced no permissions; equality check would be vacuous")
	}
	if len(wantPerms) != len(principal.Permissions) {
		t.Fatalf("permission count diverges: channel=%d jwt=%d", len(principal.Permissions), len(wantPerms))
	}
	for i := range wantPerms {
		if principal.Permissions[i] != wantPerms[i] {
			t.Fatalf("permission[%d] diverges: channel=%q jwt=%q", i, principal.Permissions[i], wantPerms[i])
		}
	}
}

// D-B13 核心反向测试：IsAllowed(sender)==true 只是防打扰粗过滤（本单用闭包
// 代表 vendor 将来的行为），不构成任何数据可见性结论。构造「粗过滤放行、
// Scope 不覆盖」的场景，断言数据被拒。
func TestIsAllowedTrueDoesNotGrantVisibility(t *testing.T) {
	pool := gatewayTestPool(t)
	ctx := context.Background()
	fixture := seedGatewayFixture(t, ctx, pool)
	t.Cleanup(func() { cleanupGatewayFixture(t, ctx, pool, fixture) })

	bindings := repository.NewChannelIdentityBindingRepository(pool)
	resolver := NewIdentityResolver(bindings, repository.NewUserRepository(pool), repository.NewRoleRepository(pool))

	sender := "feishu:" + fixture.bindingExternalID
	isAllowed := func(senderID string) bool { return senderID == sender } // vendor-style coarse pass
	if !isAllowed(sender) {
		t.Fatal("fixture broken: coarse filter should pass this sender")
	}

	principal, err := resolver.Resolve(ctx, ChannelRef{Channel: "feishu", ExternalUserID: fixture.bindingExternalID})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	filter, err := access.FromScope(principal.Scope)
	if err != nil {
		t.Fatalf("entity filter from resolved scope: %v", err)
	}
	repo := repository.NewContractRepository(pool)
	if _, err := repo.GetDraftForReview(ctx, filter, fixture.draftB); err != pgx.ErrNoRows {
		t.Fatalf("IsAllowed=true but scope-uncovering read = %v; want ErrNoRows", err)
	}
}

func TestUnboundIdentityRejectedOnRealDB(t *testing.T) {
	pool := gatewayTestPool(t)
	ctx := context.Background()
	fixture := seedGatewayFixture(t, ctx, pool)
	t.Cleanup(func() { cleanupGatewayFixture(t, ctx, pool, fixture) })

	bindings := repository.NewChannelIdentityBindingRepository(pool)
	resolver := NewIdentityResolver(bindings, repository.NewUserRepository(pool), repository.NewRoleRepository(pool))

	principal, err := resolver.Resolve(ctx, ChannelRef{Channel: "wecom", ExternalUserID: "nobody-" + uuid.NewString()})
	if !errors.Is(err, ErrUnbound) || principal.UserID != "" || len(principal.Permissions) > 0 {
		t.Fatalf("unbound identity: principal=%+v err=%v", principal, err)
	}
}

// migration 061 与 01_init 等价：init 基线自身就具备 CHECK 约束与唯一键
// （先于重放断言——27ccdd2 那次漂移的教训），在其上重放 061 是干净 no-op。
func TestMigration061MatchesInitBaseline(t *testing.T) {
	pool := gatewayTestPool(t)
	ctx := context.Background()

	var channelCheck, uniqueConstraint int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM pg_constraint WHERE conname = 'channel_identity_bindings_channel_check'`,
	).Scan(&channelCheck); err != nil || channelCheck != 1 {
		t.Fatalf("channel CHECK missing from init baseline (drift): count=%d err=%v", channelCheck, err)
	}
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM pg_constraint WHERE conname = 'uq_channel_identity'`,
	).Scan(&uniqueConstraint); err != nil || uniqueConstraint != 1 {
		t.Fatalf("unique constraint missing from init baseline (drift): count=%d err=%v", uniqueConstraint, err)
	}

	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "db", "migrations", "061_channel_identity_bindings.sql"))
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if _, err := pool.Exec(ctx, string(raw)); err != nil {
		t.Fatalf("replaying 061 onto the init baseline failed: %v", err)
	}
}

// ── helpers ────────────────────────────────────────────────────────────────

func seedGatewayFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool) gatewayFixture {
	t.Helper()
	suffix := uuid.NewString()[:8]
	fixture := gatewayFixture{bindingExternalID: "ou-gw-" + suffix}

	if err := pool.QueryRow(ctx, `
		INSERT INTO legal_entities (code, name, country, currency, is_active)
		VALUES ($1, $2, 'CN', 'CNY', true) RETURNING id`,
		"GW-LE-A-"+suffix, "gateway tenant A").Scan(&fixture.entityA); err != nil {
		t.Fatalf("seed entity A: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO legal_entities (code, name, country, currency, is_active)
		VALUES ($1, $2, 'CN', 'CNY', true) RETURNING id`,
		"GW-LE-B-"+suffix, "gateway tenant B").Scan(&fixture.entityB); err != nil {
		t.Fatalf("seed entity B: %v", err)
	}
	entityA := fixture.entityA
	entityB := fixture.entityB

	if err := pool.QueryRow(ctx, `
		INSERT INTO users (username, email, password_hash, role, legal_entity_id, is_active)
		VALUES ($1, $2, 'x', 'editor', $3, true) RETURNING id`,
		"gw-a-"+suffix, "gw-a-"+suffix+"@test.local", entityA).Scan(&fixture.userA); err != nil {
		t.Fatalf("seed user A: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (username, email, password_hash, role, legal_entity_id, is_active)
		VALUES ($1, $2, 'x', 'editor', $3, true) RETURNING id`,
		"gw-b-"+suffix, "gw-b-"+suffix+"@test.local", entityB).Scan(&fixture.userB); err != nil {
		t.Fatalf("seed user B: %v", err)
	}

	if err := pool.QueryRow(ctx, `
		INSERT INTO stores (code, name, legal_entity_id, brand, region, is_active)
		VALUES ($1, $2, $3, 'brand', 'east', true) RETURNING id`,
		"GW-ST-"+suffix, "gateway store A", entityA).Scan(&fixture.storeA); err != nil {
		t.Fatalf("seed store: %v", err)
	}

	// 一条真实权限 + 一个真实数据维度，让 Scope 相等断言非空转。
	var roleID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO roles (code, name) VALUES ($1, $2) RETURNING id`,
		"gw-editor-"+suffix, "Gateway editor "+suffix).Scan(&roleID); err != nil {
		t.Fatalf("seed role: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO permissions (role_id, resource, action) VALUES ($1, 'contracts', 'read')`, roleID); err != nil {
		t.Fatalf("seed permission: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)`, fixture.userA, roleID); err != nil {
		t.Fatalf("seed user_roles: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO user_data_scopes (user_id, dimension, target_id, target_name)
		VALUES ($1, 'store', $2, 'Gateway store A')`, fixture.userA, fixture.storeA); err != nil {
		t.Fatalf("seed data scope: %v", err)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO channel_identity_bindings (channel, external_user_id, internal_user_id, bound_by)
		VALUES ('feishu', $1, $2, $3)`, fixture.bindingExternalID, fixture.userA, fixture.userB); err != nil {
		t.Fatalf("seed binding: %v", err)
	}

	// 法人 B 的草稿：越权探测的目标。
	if err := pool.QueryRow(ctx, `
		INSERT INTO ai_tasks (task_type, status) VALUES ('contract_parse', 'completed') RETURNING id`,
	).Scan(&fixture.taskID); err != nil {
		t.Fatalf("seed task: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO ai_contract_drafts (task_id, contract_data, confidence_scores, status, legal_entity_id, data_classification)
		VALUES ($1::uuid, $2::jsonb, '{}'::jsonb, 'pending', $3::uuid, 'production') RETURNING id`,
		fixture.taskID, `{"contract_number":"GW-B-DRAFT","lessee_name":"乙","lessor_name":"丙","currency":"CNY"}`, entityB,
	).Scan(&fixture.draftB); err != nil {
		t.Fatalf("seed draft B: %v", err)
	}

	return fixture
}

func cleanupGatewayFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool, fixture gatewayFixture) {
	t.Helper()
	// users/roles 均带 ON DELETE CASCADE（user_data_scopes/user_roles/
	// permissions 随之删除），按子表在前清理。
	statements := []struct {
		sql  string
		args []any
	}{
		{`DELETE FROM ai_contract_drafts WHERE id = $1`, []any{fixture.draftB}},
		{`DELETE FROM channel_identity_bindings WHERE external_user_id = $1`, []any{fixture.bindingExternalID}},
		{`DELETE FROM ai_tasks WHERE id = $1`, []any{fixture.taskID}},
		{`DELETE FROM stores WHERE id = $1`, []any{fixture.storeA}},
		{`DELETE FROM roles WHERE code LIKE 'gw-editor-%'`, nil},
		{`DELETE FROM users WHERE id = ANY($1::uuid[])`, []any{[]string{fixture.userA, fixture.userB}}},
		{`DELETE FROM legal_entities WHERE id = ANY($1::uuid[])`, []any{[]string{fixture.entityA, fixture.entityB}}},
	}
	for _, stmt := range statements {
		if _, err := pool.Exec(context.Background(), stmt.sql, stmt.args...); err != nil {
			t.Logf("cleanup statement failed (non-fatal): %v", err)
		}
	}
}

func entityOfUserA(t *testing.T, ctx context.Context, pool *pgxpool.Pool, fixture gatewayFixture) string {
	t.Helper()
	var entity string
	if err := pool.QueryRow(ctx,
		`SELECT legal_entity_id::text FROM users WHERE id = $1`, fixture.userA).Scan(&entity); err != nil {
		t.Fatalf("read user entity: %v", err)
	}
	return entity
}

// runJWTMiddlewareChain runs the real JWT permission chain over a test gin
// context — LoadUserPermissions + DataScopeMiddleware with the production
// repositories — and returns the scope it produces.
func runJWTMiddlewareChain(t *testing.T, pool *pgxpool.Pool, userID, legalEntityID string) access.Scope {
	t.Helper()
	c := ginTestContext(t, userID, legalEntityID)
	middleware.LoadUserPermissions(repository.NewRoleRepository(pool))(c)
	middleware.DataScopeMiddleware()(c)
	scope, ok := middleware.GetAccessScope(c)
	if !ok {
		t.Fatal("JWT chain produced no access scope")
	}
	return scope
}

func ginTestContext(t *testing.T, userID, legalEntityID string) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Set("user_id", userID)
	c.Set("legal_entity_id", legalEntityID)
	return c
}

func compareScopes(t *testing.T, label string, got, want access.Scope) {
	t.Helper()
	join := func(values []string) string { return strings.Join(values, ",") }
	if got.Global != want.Global ||
		got.LegalEntityID != want.LegalEntityID ||
		join(got.StoreIDs) != join(want.StoreIDs) ||
		join(got.Regions) != join(want.Regions) ||
		join(got.Brands) != join(want.Brands) ||
		join(got.Plants) != join(want.Plants) ||
		join(got.ProductionLines) != join(want.ProductionLines) ||
		join(got.EquipmentIDs) != join(want.EquipmentIDs) {
		t.Fatalf("%s scope diverges:\n  channel: %+v\n  jwt:     %+v", label, got, want)
	}
}
