package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/storepnl"
)

type memStorePnlKPI struct{}

func (memStorePnlKPI) Operating(_ context.Context, _ storepnl.StoreRef) (storepnl.KPIAggregates, error) {
	v := 1000.0
	return storepnl.KPIAggregates{
		Revenue: &v, DecisionReady: true, Classification: "production", Currency: "CNY",
	}, nil
}

func TestStorePnlHandlerProjection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewStorePnlHandler(memStorePnlKPI{}, nil, nil)
	router := gin.New()
	router.GET("/stores/:id/pnl", handler.Projection)

	req := httptest.NewRequest(http.MethodGet, "/stores/11111111-1111-4111-8111-111111111111/pnl?as_of=2026-08-18&window_days=7&basis=side_by_side", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		Pnl struct {
			BasisMode string `json:"basis_mode"`
			Operating *struct {
				Basis string `json:"basis"`
				Rows  []struct {
					Key    string   `json:"key"`
					Actual *float64 `json:"actual"`
				} `json:"rows"`
			} `json:"operating"`
			Ifrs16 *struct {
				Basis string `json:"basis"`
				Rows  []struct {
					Key string `json:"key"`
				} `json:"rows"`
			} `json:"ifrs16"`
		} `json:"pnl"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Pnl.Operating == nil || body.Pnl.Operating.Basis != "operating_basis" {
		t.Fatalf("operating block missing: %+v", body.Pnl.Operating)
	}
	// IFRS 16 lease port is honest-unavailable: rows carry nil values with the
	// gap, never fabricated depreciation.
	if body.Pnl.Ifrs16 == nil || body.Pnl.Ifrs16.Basis != "ifrs16_basis" {
		t.Fatalf("ifrs16 block must still render (nil rows are honest), got %+v", body.Pnl.Ifrs16)
	}
}
