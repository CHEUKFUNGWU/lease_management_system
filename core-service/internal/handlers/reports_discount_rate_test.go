package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/reporting"
)

type zeroRateSource struct{}

func (zeroRateSource) GetFloat64(context.Context, string, float64) float64 { return 0 }

func missingRateSnapshotHandler() *ReportHandler {
	return &ReportHandler{snapshotBuilder: reporting.NewSnapshotBuilder(
		reportContractSource{contracts: []*repository.Contract{
			{ID: "c1", ContractNumber: "CT-LE001", ApprovalStatus: "approved"},
			{ID: "c2", ContractNumber: "CT-LE002", ApprovalStatus: "approved"},
		}},
		reportPaymentSource{payments: map[string][]*repository.PaymentSchedule{}},
		zeroRateSource{},
	)}
}

// FIX-002 A1/A2: with no confirmed discount rate the two endpoints the
// portfolio page loads must answer 422 data_unavailable and name the affected
// contracts by contract number, not a bare UUID and not a retryable 500.
func TestProjectionEndpointsMissingDiscountRateReturn422WithContractNumbers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	routes := map[string]func(*gin.Context){
		"/reports/portfolio-summary": missingRateSnapshotHandler().PortfolioSummary,
		"/reports/unit-price":        missingRateSnapshotHandler().UnitPrice,
	}
	for path, handler := range routes {
		router := gin.New()
		router.GET(path, handler)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path+"?group_by=store", nil))

		if recorder.Code != http.StatusUnprocessableEntity {
			t.Fatalf("%s status = %d body=%s, want 422", path, recorder.Code, recorder.Body.String())
		}
		var body struct {
			Code    string `json:"code"`
			Error   string `json:"error"`
			Details struct {
				DiscountRateMissing bool     `json:"discount_rate_missing"`
				Contracts           []string `json:"contracts"`
			} `json:"details"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s decode response: %v", path, err)
		}
		if body.Code != "data_unavailable" {
			t.Fatalf("%s code = %q, want data_unavailable", path, body.Code)
		}
		if !body.Details.DiscountRateMissing {
			t.Fatalf("%s details.discount_rate_missing = false, want true", path)
		}
		if len(body.Details.Contracts) != 2 || body.Details.Contracts[0] != "CT-LE001" || body.Details.Contracts[1] != "CT-LE002" {
			t.Fatalf("%s details.contracts = %#v, want [CT-LE001 CT-LE002]", path, body.Details.Contracts)
		}
		if got := recorder.Body.String(); !json.Valid([]byte(got)) {
			t.Fatalf("%s body is not valid JSON: %s", path, got)
		}
	}
}

// A5: with a usable global policy rate the same endpoints return 200 and the
// historical payload shape.
func TestProjectionEndpointsWithRatesStillReturn200(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &ReportHandler{snapshotBuilder: reporting.NewSnapshotBuilder(
		reportContractSource{contracts: []*repository.Contract{
			{ID: "c1", ContractNumber: "CT-LE001", ApprovalStatus: "approved"},
		}},
		reportPaymentSource{payments: map[string][]*repository.PaymentSchedule{}},
		reportRateSource{},
	)}
	router := gin.New()
	router.GET("/reports/portfolio-summary", handler.PortfolioSummary)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/reports/portfolio-summary", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", recorder.Code, recorder.Body.String())
	}
}
