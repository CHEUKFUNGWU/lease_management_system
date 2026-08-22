package handlers

// R2-4（RH6）：模板校验端点契约测试。三类错误各一条 + 合法输入。
//
// 自检句：把 handler 里的 errors.As 分支删掉（循环引用落进 schema 兜底），
// cycle 那条红；把 Parse 换成恒 nil，valid 那条仍绿但三条错误全红——
// 所以错误路径的断言才是承重墙。

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func postValidate(t *testing.T, payload map[string]any) (int, templateValidationResult) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	handler := NewFinModelHandler(nil)
	router := gin.New()
	router.POST("/validate", func(c *gin.Context) {
		c.Set("user_id", "00000000-0000-0000-0000-000000000001")
		c.Set("legal_entity_id", "entity-a")
		handler.ValidateTemplate(c)
	})
	raw, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/validate", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var res templateValidationResult
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode response %d: %v body=%s", w.Code, err, w.Body.String())
	}
	return w.Code, res
}

func TestValidateTemplateAcceptsValidDef(t *testing.T) {
	code, res := postValidate(t, map[string]any{
		"name": "t", "version": 1,
		"rows": []map[string]any{
			{"key": "rev", "label": "收入", "kind": "link", "basis": "shared", "source": "fact.revenue"},
			{"key": "margin", "label": "毛利", "kind": "formula", "basis": "shared", "formula": "rows.rev * 1"},
		},
	})
	if code != http.StatusOK || !res.Valid || len(res.Errors) != 0 {
		t.Fatalf("valid def: status=%d result=%+v", code, res)
	}
}

func TestValidateTemplateCircularReferenceCarriesPath(t *testing.T) {
	code, res := postValidate(t, map[string]any{
		"name": "cycle", "version": 1,
		"rows": []map[string]any{
			{"key": "a", "label": "A", "kind": "formula", "basis": "shared", "formula": "rows.b"},
			{"key": "b", "label": "B", "kind": "formula", "basis": "shared", "formula": "rows.a"},
		},
	})
	if code != http.StatusOK || res.Valid {
		t.Fatalf("cycle must be invalid, status=%d result=%+v", code, res)
	}
	if len(res.Errors) != 1 || res.Errors[0].Kind != "circular_reference" {
		t.Fatalf("expected structured circular_reference, got %+v", res.Errors)
	}
	path := res.Errors[0].CyclePath
	if len(path) != 3 || path[0] != "a" || path[1] != "b" || path[2] != "a" {
		t.Fatalf("cycle path = %v, want [a b a]", path)
	}
}

func TestValidateTemplateUnknownReferenceHasRowKeyAndKind(t *testing.T) {
	code, res := postValidate(t, map[string]any{
		"name": "unknown-ref", "version": 1,
		"rows": []map[string]any{
			{"key": "a", "label": "A", "kind": "formula", "basis": "shared", "formula": "rows.nope"},
		},
	})
	if code != http.StatusOK || res.Valid {
		t.Fatalf("unknown ref must be invalid, status=%d result=%+v", code, res)
	}
	e := res.Errors[0]
	if e.Kind != "unknown_reference" || e.RowKey != "a" {
		t.Fatalf("expected unknown_reference on row a, got %+v", e)
	}
}

func TestValidateTemplateSyntaxErrorHasRowKeyAndKind(t *testing.T) {
	code, res := postValidate(t, map[string]any{
		"name": "syntax", "version": 1,
		"rows": []map[string]any{
			{"key": "a", "label": "A", "kind": "formula", "basis": "shared", "formula": "rows.a +"},
		},
	})
	if code != http.StatusOK || res.Valid {
		t.Fatalf("syntax error must be invalid, status=%d result=%+v", code, res)
	}
	e := res.Errors[0]
	if e.RowKey != "a" || (e.Kind != "syntax" && e.Kind != "unknown_reference") {
		t.Fatalf("expected row-scoped syntax-class error, got %+v", e)
	}
}
