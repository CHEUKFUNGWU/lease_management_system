package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/storepnl"
)

// recordingPnlKPI captures the StoreRef the projection resolved so the test
// can prove the retailperiod window actually reaches the KPI port (S1-2
// grain wiring — the resolved From/To is what the semantic layer sums).
type recordingPnlKPI struct {
	got storepnl.StoreRef
}

func (r *recordingPnlKPI) Operating(_ context.Context, ref storepnl.StoreRef) (storepnl.KPIAggregates, error) {
	r.got = ref
	revenue := 1000.0
	return storepnl.KPIAggregates{
		Revenue: &revenue, DecisionReady: true, Classification: "production", Currency: "CNY",
	}, nil
}

func TestStorePnlProjectionResolvesPeriodGrains(t *testing.T) {
	cases := []struct {
		spec     string
		asOf     string
		from, to string
		label    string
		kind     string
	}{
		{"2026-Q3", "2026-08-19", "2026-07-01", "2026-09-30", "2026-Q3", "calendar"},
		{"2026-07", "2026-08-19", "2026-07-01", "2026-07-31", "2026-07", "calendar"},
		{"2026-W02", "2026-08-19", "2026-01-05", "2026-01-11", "2026-W02", "calendar"},
		{"2026", "2026-08-19", "2026-01-01", "2026-12-31", "2026", "calendar"},
		{"14", "2026-08-19", "2026-08-06", "2026-08-19", "近 14 天", "rolling"},
		{"last-month", "2026-08-19", "2026-07-01", "2026-07-31", "2026-07", "calendar"},
	}
	for _, tc := range cases {
		t.Run(tc.spec, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			kpi := &recordingPnlKPI{}
			handler := NewStorePnlHandler(kpi, nil, nil).
				WithMasterData(memStoreLookup{storeID: "S1", legalEntityID: "LE-1"})
			router := gin.New()
			router.GET("/stores/:id/pnl", func(c *gin.Context) {
				c.Set("legal_entity_id", "LE-1")
				handler.Projection(c)
			})
			req := httptest.NewRequest(http.MethodGet, "/stores/S1/pnl?period="+tc.spec+"&as_of="+tc.asOf, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
			if kpi.got.DateFrom != tc.from || kpi.got.DateTo != tc.to {
				t.Fatalf("resolved window = %s..%s, want %s..%s", kpi.got.DateFrom, kpi.got.DateTo, tc.from, tc.to)
			}
			if kpi.got.PeriodLabel != tc.label || kpi.got.PeriodKind != tc.kind {
				t.Fatalf("label/kind = %q/%q, want %q/%q", kpi.got.PeriodLabel, kpi.got.PeriodKind, tc.label, tc.kind)
			}
			if tc.kind == "calendar" && !strings.Contains(w.Body.String(), tc.label) {
				t.Fatalf("response must echo the period label, body=%s", w.Body.String())
			}
		})
	}
}

func TestStorePnlProjectionRejectsIllegalPeriod(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewStorePnlHandler(&recordingPnlKPI{}, nil, nil).
		WithMasterData(memStoreLookup{storeID: "S1", legalEntityID: "LE-1"})
	router := gin.New()
	router.GET("/stores/:id/pnl", func(c *gin.Context) {
		c.Set("legal_entity_id", "LE-1")
		handler.Projection(c)
	})
	req := httptest.NewRequest(http.MethodGet, "/stores/S1/pnl?period=not-a-period&as_of=2026-08-19", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("illegal period must 400, got %d", w.Code)
	}
}
