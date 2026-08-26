package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/repository"
)

type fakeTrialBalanceStore struct {
	versions []*repository.TrialBalanceVersion
	lines    []*repository.TrialBalanceLine
	replay   *repository.TrialBalanceVersion
	deleted  bool
}

func (f *fakeTrialBalanceStore) CreateTrialBalanceVersion(_ context.Context, item *repository.TrialBalanceVersion) (*repository.TrialBalanceVersion, error) {
	if f.replay != nil {
		return nil, repository.ErrTrialBalanceVersionReplay
	}
	f.versions = append(f.versions, item)
	return item, nil
}

func (f *fakeTrialBalanceStore) GetTrialBalanceVersionByContent(context.Context, access.EntityFilter, string, string, string) (*repository.TrialBalanceVersion, error) {
	return f.replay, nil
}

func (f *fakeTrialBalanceStore) CreateTrialBalanceLine(_ context.Context, item *repository.TrialBalanceLine) (*repository.TrialBalanceLine, error) {
	f.lines = append(f.lines, item)
	return item, nil
}

func (f *fakeTrialBalanceStore) ListTrialBalanceVersions(context.Context, access.EntityFilter, string) ([]*repository.TrialBalanceVersion, error) {
	return f.versions, nil
}

func (f *fakeTrialBalanceStore) DeleteTrialBalanceVersion(context.Context, string) error {
	f.deleted = true
	return nil
}

func newTBMultipart(t *testing.T, csvContent string, fields map[string]string) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatal(err)
		}
	}
	part, err := writer.CreateFormFile("file", "trial-balance.csv")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte(csvContent)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return body, writer.FormDataContentType()
}

func TestTrialBalanceImportContentIdentityAndReplay(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &fakeTrialBalanceStore{}
	handler := NewTrialBalanceHandler(store)
	csvContent := "account_code,account_name,debit,credit\n1001,现金,100.00,0.00\n2001,租赁负债,0.00,100.00\n"
	body, contentType := newTBMultipart(t, csvContent, map[string]string{
		"name": "TB 2026-07", "source_system": "gl-export", "period": "2026-07", "functional_currency": "CNY",
	})
	run := func() *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/gl/trial-balances/import", body)
		request.Header.Set("Content-Type", contentType)
		c, _ := gin.CreateTestContext(recorder)
		c.Request = request
		c.Set("legal_entity_id", "legal-entity-a")
		c.Set("access_scope", access.Scope{LegalEntityID: "legal-entity-a"})
		c.Set("user_id", "user-a")
		handler.Import(c)
		return recorder
	}
	first := run()
	if first.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", first.Code, first.Body.String())
	}
	var response struct {
		Version struct {
			ContentSHA256 string `json:"content_sha256"`
		} `json:"version"`
		Accepted int  `json:"accepted_rows"`
		Balanced bool `json:"balanced"`
	}
	if err := json.Unmarshal(first.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Accepted != 2 || !response.Balanced || response.Version.ContentSHA256 == "" {
		t.Fatalf("response=%+v", response)
	}
	if len(store.lines) != 2 || store.lines[0].AccountCode != "1001" || store.lines[0].Debit != 100 {
		t.Fatalf("lines=%+v", store.lines)
	}
	// Same content identity → replay, no second version, no second lines.
	store.replay = store.versions[0]
	replayBody, replayContentType := newTBMultipart(t, csvContent, map[string]string{
		"name": "TB 2026-07", "source_system": "gl-export", "period": "2026-07", "functional_currency": "CNY",
	})
	_ = replayBody
	_ = replayContentType
	// second request reuses the first recorder path with a fresh body
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/gl/trial-balances/import", replayBody)
	request.Header.Set("Content-Type", replayContentType)
	c, _ := gin.CreateTestContext(recorder)
	c.Request = request
	c.Set("legal_entity_id", "legal-entity-a")
	c.Set("access_scope", access.Scope{LegalEntityID: "legal-entity-a"})
	c.Set("user_id", "user-a")
	handler.Import(c)
	var replay struct {
		IdempotentReplay bool `json:"idempotent_replay"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &replay); err != nil {
		t.Fatal(err)
	}
	if !replay.IdempotentReplay || len(store.lines) != 2 {
		t.Fatalf("replay=%+v lines=%d", replay, len(store.lines))
	}
}

// P1-2: a repeated account code is a reported data error, never a silent
// drop that would desync totals from the stored lines.
func TestTrialBalanceImportRejectsDuplicateAccountCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &fakeTrialBalanceStore{}
	handler := NewTrialBalanceHandler(store)
	csvContent := "account_code,debit,credit\n1001,100.00,0.00\n1001,0.00,100.00\n"
	body, contentType := newTBMultipart(t, csvContent, map[string]string{
		"name": "TB dup", "source_system": "gl", "period": "2026-07", "functional_currency": "CNY",
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/gl/trial-balances/import", body)
	request.Header.Set("Content-Type", contentType)
	c, _ := gin.CreateTestContext(recorder)
	c.Request = request
	c.Set("legal_entity_id", "legal-entity-a")
	c.Set("access_scope", access.Scope{LegalEntityID: "legal-entity-a"})
	c.Set("user_id", "user-a")
	handler.Import(c)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Accepted int `json:"accepted_rows"`
		Rejected int `json:"rejected_rows"`
		Errors   []struct {
			Code string `json:"code"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Accepted != 1 || response.Rejected != 1 || len(response.Errors) != 1 || response.Errors[0].Code != "duplicate_account_code" {
		t.Fatalf("response=%+v", response)
	}
}
