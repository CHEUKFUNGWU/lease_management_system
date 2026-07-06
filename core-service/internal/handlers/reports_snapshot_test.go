package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/reporting"
)

type reportContractSource struct {
	contracts []*repository.Contract
}

func TestLiabilityExportOfficialUsesSnapshotPopulationAndTraceHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	builder := reporting.NewSnapshotBuilder(
		reportContractSource{contracts: []*repository.Contract{
			{ID: "approved", ContractNumber: "LC-001", ContractName: "Approved Contract", ApprovalStatus: "approved"},
			{ID: "draft", ContractNumber: "LC-002", ContractName: "Draft Contract", ApprovalStatus: "draft"},
		}},
		reportPaymentSource{payments: map[string][]*repository.PaymentSchedule{}},
		reportRateSource{},
	)
	handler := &ReportHandler{snapshotBuilder: builder}
	router := gin.New()
	router.GET("/reports/liability-rolling/export", handler.ExportLiabilityRolling)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/reports/liability-rolling/export?mode=official&language=en", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get("X-Report-Snapshot-ID") == "" || recorder.Header().Get("X-Report-Policy-Version") != "report-snapshot-v1" || recorder.Header().Get("X-Report-Mode") != "official" {
		t.Fatalf("trace headers = %#v", recorder.Header())
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "Approved Contract") || strings.Contains(body, "Draft Contract") {
		t.Fatalf("official export population = %q", body)
	}
}

func TestTagsUsesWorkingSnapshotPopulation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	builder := reporting.NewSnapshotBuilder(
		reportContractSource{contracts: []*repository.Contract{
			{ID: "draft", ApprovalStatus: "draft", Tags: "beta, alpha"},
			{ID: "rejected", ApprovalStatus: "rejected", Tags: "hidden"},
		}},
		reportPaymentSource{payments: map[string][]*repository.PaymentSchedule{}},
		reportRateSource{},
	)
	handler := &ReportHandler{snapshotBuilder: builder}
	router := gin.New()
	router.GET("/reports/tags", handler.Tags)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/reports/tags", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Mode       reporting.Mode `json:"mode"`
		IsOfficial bool           `json:"is_official"`
		Data       []string       `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Mode != reporting.Working || response.IsOfficial || !reflect.DeepEqual(response.Data, []string{"alpha", "beta"}) {
		t.Fatalf("working tags = %#v", response)
	}
}

func (s reportContractSource) GetByStatuses(context.Context, []string, string) ([]*repository.Contract, error) {
	return s.contracts, nil
}

type reportPaymentSource struct {
	payments map[string][]*repository.PaymentSchedule
}

func (s reportPaymentSource) GetByContractIDs(context.Context, []string) (map[string][]*repository.PaymentSchedule, error) {
	return s.payments, nil
}

type reportRateSource struct{}

func (reportRateSource) GetFloat64(context.Context, string, float64) float64 { return 0.05 }

func TestCashflowForecastOfficialUsesControlledSnapshotFacts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	contract := &repository.Contract{
		ID: "contract-1", ContractNumber: "LC-001", ContractName: "Flagship",
		ApprovalStatus: "approved", Currency: "CNY",
	}
	dueDate := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	builder := reporting.NewSnapshotBuilder(
		reportContractSource{contracts: []*repository.Contract{contract}},
		reportPaymentSource{payments: map[string][]*repository.PaymentSchedule{
			contract.ID: {
				{ID: "approved", ContractID: contract.ID, DueDate: dueDate, Amount: 100, IsFixed: true, ApprovalStatus: "approved"},
				{ID: "draft", ContractID: contract.ID, DueDate: dueDate, Amount: 999, IsFixed: true, ApprovalStatus: "draft"},
			},
		}},
		reportRateSource{},
	)
	handler := &ReportHandler{snapshotBuilder: builder}
	router := gin.New()
	router.GET("/reports/cashflow-forecast", handler.CashflowForecast)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/reports/cashflow-forecast?mode=official&view=contract&granularity=month&start_date=2026-01-01&end_date=2026-01-31", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		SnapshotID    string                `json:"snapshot_id"`
		PolicyVersion string                `json:"policy_version"`
		Mode          reporting.Mode        `json:"mode"`
		IsOfficial    bool                  `json:"is_official"`
		Data          []CashflowForecastRow `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.SnapshotID == "" || response.PolicyVersion != "report-snapshot-v1" || response.Mode != reporting.Official || !response.IsOfficial {
		t.Fatalf("snapshot metadata = %#v", response)
	}
	if len(response.Data) != 1 || response.Data[0].FixedRent != 100 || response.Data[0].PaymentCount != 1 {
		t.Fatalf("official cashflow rows = %#v", response.Data)
	}
}

func TestSensitivityAnalysisProjectsFromControlledSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	commencement := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	contract := &repository.Contract{
		ID: "contract-1", ContractNumber: "LC-001", ContractName: "Flagship",
		ApprovalStatus: "approved", LeaseScope: "in_scope", Currency: "CNY",
		CommencementDate: commencement, LeaseEndDate: time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
	}
	builder := reporting.NewSnapshotBuilder(
		reportContractSource{contracts: []*repository.Contract{contract}},
		reportPaymentSource{payments: map[string][]*repository.PaymentSchedule{
			contract.ID: {{
				ContractID: contract.ID, DueDate: contract.LeaseEndDate, Amount: 1200,
				IsFixed: true, IsLeaseComponent: true, IncludedInLiabilityPV: true,
				PaymentTiming: "postpaid", ApprovalStatus: "approved",
			}},
		}},
		reportRateSource{},
	)
	handler := &ReportHandler{snapshotBuilder: builder}
	router := gin.New()
	router.GET("/reports/sensitivity", handler.SensitivityAnalysis)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/reports/sensitivity?contract_id=contract-1&shocks=0,0.01", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		SnapshotID string                     `json:"snapshot_id"`
		BaseRate   float64                    `json:"base_rate"`
		Data       []reporting.SensitivityRow `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.SnapshotID == "" || response.BaseRate != 0.05 || len(response.Data) != 2 || response.Data[1].InitialLiability >= response.Data[0].InitialLiability {
		t.Fatalf("sensitivity response = %#v", response)
	}
}
