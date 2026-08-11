package agentreaders

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lease-management-system/core-service/internal/agenttools/tools"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/budget"
	"github.com/lease-management-system/core-service/internal/services/cashflow"
)

type BudgetVarianceReader struct {
	Repo     *repository.BudgetRepository
	Settings *repository.SystemSettingRepository
}

func NewBudgetVarianceReader(repo *repository.BudgetRepository, settings *repository.SystemSettingRepository) *BudgetVarianceReader {
	return &BudgetVarianceReader{Repo: repo, Settings: settings}
}

func (r *BudgetVarianceReader) ReadVariance(ctx context.Context, legalEntityID, versionID, period string) (any, error) {
	if r == nil || r.Repo == nil {
		return nil, fmt.Errorf("budget variance reader unavailable")
	}
	version, err := r.Repo.GetVersion(ctx, versionID, legalEntityID)
	if err != nil {
		return nil, err
	}
	if version == nil {
		return nil, fmt.Errorf("budget version not found")
	}
	planned, err := r.Repo.LinesForPeriod(ctx, versionID, period)
	if err != nil {
		return nil, err
	}
	actual, err := r.Repo.ActualsForPeriod(ctx, legalEntityID, period)
	if err != nil {
		return nil, err
	}
	if currency, err := singleCurrency(planned, actual); err != nil {
		return nil, err
	} else if currency == "" && (len(planned) > 0 || len(actual) > 0) {
		return nil, fmt.Errorf("variance scope contains missing currency")
	}
	events, err := r.Repo.EventTypesByContract(ctx, period)
	if err != nil {
		return nil, err
	}
	fx, err := r.Repo.FXByContract(ctx, period)
	if err != nil {
		return nil, err
	}
	threshold := r.setting(ctx, "budget_variance_materiality_threshold")
	tolerance := r.setting(ctx, "budget_tie_out_tolerance")
	result := budget.Explain(budget.Input{Period: period, Budget: toBudgetPeriods(planned), Actual: toBudgetPeriods(actual), MaterialityThreshold: threshold, TieOutTolerance: tolerance, EventsByContract: events, FXByContract: fx})
	return map[string]any{"version": version, "actual_basis": "measurement_results", "basis": "Working", "period": period, "result": result, "source": map[string]any{"version_id": version.ID, "as_of_period": version.AsOfPeriod, "coverage_scope": version.CoverageScope}}, nil
}

func (r *BudgetVarianceReader) setting(ctx context.Context, key string) float64 {
	if r.Settings == nil {
		return 0
	}
	return r.Settings.GetFloat64(ctx, key, 0)
}

type CashflowScenarioReader struct {
	Contracts *repository.ContractRepository
	Payments  *repository.PaymentScheduleRepository
}

func NewCashflowScenarioReader(contracts *repository.ContractRepository, payments *repository.PaymentScheduleRepository) *CashflowScenarioReader {
	return &CashflowScenarioReader{Contracts: contracts, Payments: payments}
}

func (r *CashflowScenarioReader) ReadScenario(ctx context.Context, legalEntityID string, args tools.CashflowScenarioArguments) (any, error) {
	if r == nil || r.Contracts == nil || r.Payments == nil {
		return nil, fmt.Errorf("cashflow scenario reader unavailable")
	}
	if args.HorizonMonths <= 0 {
		return nil, fmt.Errorf("horizon_months must be positive")
	}
	asOf, err := time.Parse("2006-01-02", strings.TrimSpace(args.AsOf))
	if err != nil {
		return nil, fmt.Errorf("as_of must be YYYY-MM-DD")
	}
	contracts, err := r.Contracts.List(ctx, legalEntityID, repository.ListContractsFilter{})
	if err != nil {
		return nil, err
	}
	leases := make([]cashflow.Lease, 0, len(contracts))
	currency := ""
	for _, contract := range contracts {
		if strings.TrimSpace(contract.Currency) == "" {
			return nil, fmt.Errorf("portfolio contains a contract with missing currency")
		}
		if currency == "" {
			currency = contract.Currency
		} else if currency != contract.Currency {
			return nil, fmt.Errorf("portfolio contains multiple currencies; run each currency separately")
		}
		schedules, scheduleErr := r.Payments.GetByContractID(ctx, contract.ID)
		if scheduleErr != nil {
			return nil, scheduleErr
		}
		leases = append(leases, cashflow.Lease{ContractID: contract.ID, ContractNumber: contract.ContractNumber, ContractName: contract.ContractName, StoreName: contract.StoreName, Currency: contract.Currency, LeaseEndDate: contract.LeaseEndDate, Payments: repository.ToIFRS16Payments(schedules)})
	}
	results := make([]cashflow.Result, 0, len(args.Scenarios))
	for _, scenario := range args.Scenarios {
		result, projectErr := cashflow.Project(cashflow.Input{AsOf: asOf, Currency: currency, HorizonMonths: args.HorizonMonths, Leases: leases, Scenario: scenario})
		if projectErr != nil {
			return nil, projectErr
		}
		results = append(results, result)
	}
	return map[string]any{"basis": "Scenario", "as_of": asOf.Format("2006-01-02"), "results": results, "side_effects": false}, nil
}

type RenewalDecisionReader struct {
	Contracts *repository.ContractRepository
	Decisions *repository.RenewalDecisionRepository
}

func NewRenewalDecisionReader(contracts *repository.ContractRepository, decisions *repository.RenewalDecisionRepository) *RenewalDecisionReader {
	return &RenewalDecisionReader{Contracts: contracts, Decisions: decisions}
}

func (r *RenewalDecisionReader) ReadDecisions(ctx context.Context, legalEntityID, contractID string) (any, error) {
	if r == nil || r.Contracts == nil || r.Decisions == nil {
		return nil, fmt.Errorf("renewal decision reader unavailable")
	}
	contract, err := r.Contracts.GetByID(ctx, contractID, legalEntityID)
	if err != nil {
		return nil, err
	}
	if contract == nil {
		return nil, fmt.Errorf("contract not found")
	}
	items, err := r.Decisions.List(ctx, contractID, legalEntityID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"contract_id": contractID, "contract_number": contract.ContractNumber, "currency": contract.Currency, "data": items, "total": len(items), "basis": "Scenario"}, nil
}

func toBudgetPeriods(rows []repository.BudgetContractPeriod) []budget.ContractPeriod {
	result := make([]budget.ContractPeriod, 0, len(rows))
	for _, row := range rows {
		result = append(result, budget.ContractPeriod{ContractID: row.ContractID, ContractNumber: row.ContractNumber, ContractName: row.ContractName, Currency: row.Currency, LeaseCost: row.LeaseCost, TotalPayment: row.TotalPayment})
	}
	return result
}

func singleCurrency(groups ...[]repository.BudgetContractPeriod) (string, error) {
	currency := ""
	for _, rows := range groups {
		for _, row := range rows {
			value := strings.TrimSpace(row.Currency)
			if value == "" {
				return "", fmt.Errorf("comparison contains missing currency")
			}
			if currency == "" {
				currency = value
			} else if currency != value {
				return "", fmt.Errorf("comparison contains multiple currencies")
			}
		}
	}
	return currency, nil
}
