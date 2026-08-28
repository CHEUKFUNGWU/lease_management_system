package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lease-management-system/core-service/internal/repository"
)

// seedDefinitionFixture builds an entity + user and a handler whose repo
// points at the throwaway database.
func seedDefinitionFixture(t *testing.T, pool *pgxpool.Pool) (string, string, *FinModelHandler) {
	t.Helper()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
			t.Fatalf("fixture exec: %v", err)
		}
	}
	suffix := uuid.NewString()[:8]
	entity := uuid.NewString()
	exec(`INSERT INTO legal_entities (id, code, name, country, currency) VALUES ($1,$2,$3,'CN','CNY')`,
		entity, "FMDEF-E-"+suffix, "FmDef "+suffix)
	userID := uuid.NewString()
	exec(`INSERT INTO users (id, username, email, password_hash, legal_entity_id)
		VALUES ($1,$2,$3,'integration-only',$4)`, userID, "fmdef-user-"+suffix, "fmdef-user-"+suffix+"@example.com", entity)
	return entity, userID, NewFinModelHandler(repository.NewFinModelRepository(pool))
}

// ginServeDefinitions mounts the definitions routes with the tenant/user
// context middleware the production chain provides.
func ginServeDefinitions(handler *FinModelHandler, entity, userID, method, path, body string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	wrap := func(fn func(*gin.Context)) gin.HandlerFunc {
		return func(c *gin.Context) {
			c.Set("legal_entity_id", entity)
			c.Set("user_id", userID)
			fn(c)
		}
	}
	router.POST("/financial-model/definitions", wrap(handler.CreateDefinition))
	router.GET("/financial-model/definitions", wrap(handler.ListDefinitions))
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// TestCreateDefinitionSeedsFactoryTemplatePostgres locks the P1 demo-path
// flow (FP&A 反馈 2026-08-27): one POST materializes the factory statement
// template + a draft definition; the created definition shows up in
// ListDefinitions; a replayed request is idempotent on
// UNIQUE (legal_entity_id, name, version); another tenant neither sees the
// definition in its list nor leaks across — bottom line 1.
func TestCreateDefinitionSeedsFactoryTemplatePostgres(t *testing.T) {
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
	entity, userID, handler := seedDefinitionFixture(t, pool)

	createCode := func(body string) (int, string) {
		rec := ginServeDefinitions(handler, entity, userID, http.MethodPost, "/financial-model/definitions", body)
		return rec.Code, rec.Body.String()
	}

	code, body := createCode(`{}`)
	if code != http.StatusCreated {
		t.Fatalf("create status %d: %s", code, body)
	}
	var created struct {
		Definition struct {
			ID         string `json:"id"`
			Name       string `json:"name"`
			Status     string `json:"status"`
			TemplateID string `json:"template_id"`
		} `json:"definition"`
	}
	if err := json.Unmarshal([]byte(body), &created); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	if created.Definition.ID == "" || created.Definition.TemplateID == "" || created.Definition.Status != "draft" {
		t.Fatalf("created definition incomplete: %s", body)
	}
	if !strings.HasPrefix(created.Definition.Name, "三表模型 · ") {
		t.Fatalf("default factory name missing prefix: %q", created.Definition.Name)
	}

	listRec := ginServeDefinitions(handler, entity, userID, http.MethodGet, "/financial-model/definitions", "")
	var listed struct {
		Definitions []struct {
			ID string `json:"id"`
		} `json:"definitions"`
	}
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status %d: %s", listRec.Code, listRec.Body.String())
	}
	if err := json.Unmarshal([]byte(listRec.Body.String()), &listed); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	found := false
	for _, row := range listed.Definitions {
		if row.ID == created.Definition.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("created definition not in the list (%d rows)", len(listed.Definitions))
	}

	replayCode, replayBody := createCode(`{}`)
	if replayCode != http.StatusOK {
		t.Fatalf("replay should be 200 idempotent, got %d: %s", replayCode, replayBody)
	}
	if !strings.Contains(replayBody, `"idempotent_replay":true`) {
		t.Fatalf("replay must mark idempotent_replay: %s", replayBody)
	}

	// 跨法人：另一法人看不到该定义；自己空体重复种子可以继续（名字按实体 ID 区分）。
	otherEntity, otherUserID, otherHandler := seedDefinitionFixture(t, pool)
	otherList := ginServeDefinitions(otherHandler, otherEntity, otherUserID, http.MethodGet, "/financial-model/definitions", "")
	if strings.Contains(otherList.Body.String(), created.Definition.ID) {
		t.Fatalf("entity B leaked entity A's definition: %s", otherList.Body.String())
	}

	// 名字守卫：保留前缀拒绝。
	badCode, badBody := createCode(`{"name":"三表模型 · 冒名"}`)
	if badCode != http.StatusBadRequest {
		t.Fatalf("reserved prefix must be refused, got %d: %s", badCode, badBody)
	}
}
