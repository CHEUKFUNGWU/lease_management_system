package reporting

import (
	"fmt"
	"sort"

	"github.com/lease-management-system/core-service/internal/repository"
)

// Unit-price groupings. Store answers "is this shop expensive?"; brand and
// region answer "expensive compared with what?".
const (
	GroupByStore  = "store"
	GroupByBrand  = "brand"
	GroupByRegion = "region"
)

// UnitPriceRow is the rent-per-square-metre comparison for one group.
//
// MonthlyRentPerSqm divides a straight-lined monthly rent by leased area, so
// contracts with different free-rent and escalation structures stay comparable.
// Both sides of that ratio only count contracts that actually carry an area —
// otherwise a single lease with no area recorded would deflate the whole group's
// unit price. ContractCount versus AreaCoverageCount makes that basis visible.
type UnitPriceRow struct {
	GroupKey   string `json:"group_key"`
	GroupLabel string `json:"group_label"`
	Brand      string `json:"brand,omitempty"`
	Region     string `json:"region,omitempty"`
	Currency   string `json:"currency"`

	ContractCount     int `json:"contract_count"`
	AreaCoverageCount int `json:"area_coverage_count"`

	TotalAreaSqm      float64 `json:"total_area_sqm"`
	MonthlyFixedRent  float64 `json:"monthly_fixed_rent"`
	MonthlyRentPerSqm float64 `json:"monthly_rent_per_sqm"`
	AnnualFixedRent   float64 `json:"annual_fixed_rent"`
}

// projectUnitPrice compares rent per square metre across stores, brands or
// regions. Currency is part of every group key: averaging rent across
// currencies would produce a number that means nothing.
func projectUnitPrice(snapshot *Snapshot, request ProjectionRequest) (ProjectionResult, error) {
	groupBy := request.View
	if groupBy == "" {
		groupBy = GroupByStore
	}
	if groupBy != GroupByStore && groupBy != GroupByBrand && groupBy != GroupByRegion {
		return ProjectionResult{}, fmt.Errorf("invalid unit price grouping %q, must be store|brand|region", groupBy)
	}

	rowsByKey := make(map[string]*UnitPriceRow)
	contractsWithoutArea := 0

	for index := range snapshot.Contracts {
		fact := &snapshot.Contracts[index]
		contract := fact.Contract

		currency := contract.Currency
		if currency == "" {
			currency = defaultUnitPriceCurrency
		}
		label := unitPriceGroupLabel(fact, groupBy)
		key := label + "|" + currency

		row := rowsByKey[key]
		if row == nil {
			row = &UnitPriceRow{
				GroupKey: key, GroupLabel: label, Currency: currency,
				Brand: fact.Brand, Region: fact.Region,
			}
			rowsByKey[key] = row
		}
		row.ContractCount++

		// A lease with no recorded area contributes to neither side of the
		// ratio, so the reported unit price stays true to the area it covers.
		if contract.AreaSqm == nil || *contract.AreaSqm <= 0 {
			contractsWithoutArea++
			continue
		}
		monthlyRent := straightLineMonthlyRent(fact)
		if monthlyRent <= 0 {
			contractsWithoutArea++
			continue
		}

		row.AreaCoverageCount++
		row.TotalAreaSqm += *contract.AreaSqm
		row.MonthlyFixedRent += monthlyRent
	}

	rows := make([]UnitPriceRow, 0, len(rowsByKey))
	for _, row := range rowsByKey {
		if row.TotalAreaSqm > 0 {
			row.MonthlyRentPerSqm = roundProjection(row.MonthlyFixedRent / row.TotalAreaSqm)
		}
		row.TotalAreaSqm = roundProjection(row.TotalAreaSqm)
		row.MonthlyFixedRent = roundProjection(row.MonthlyFixedRent)
		row.AnnualFixedRent = roundProjection(row.MonthlyFixedRent * 12)
		rows = append(rows, *row)
	}

	// Most expensive first: that is the order a BP reads the list in.
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].MonthlyRentPerSqm != rows[j].MonthlyRentPerSqm {
			return rows[i].MonthlyRentPerSqm > rows[j].MonthlyRentPerSqm
		}
		return rows[i].GroupLabel < rows[j].GroupLabel
	})

	return ProjectionResult{Payload: projectionPayload(snapshot, rows, map[string]any{
		"group_by":               groupBy,
		"total":                  len(rows),
		"contracts_without_area": contractsWithoutArea,
		"area_basis_caveat":      contractsWithoutArea > 0,
	})}, nil
}

const defaultUnitPriceCurrency = "CNY"

// unassignedGroupLabel marks contracts whose store, brand or region is unknown,
// so they stay visible in the comparison instead of silently merging into a
// real group.
const unassignedGroupLabel = "未分配"

func unitPriceGroupLabel(fact *ContractFact, groupBy string) string {
	var value string
	switch groupBy {
	case GroupByBrand:
		value = fact.Brand
	case GroupByRegion:
		value = fact.Region
	default:
		value = fact.Contract.StoreName
	}
	if value == "" {
		return unassignedGroupLabel
	}
	return value
}

// straightLineMonthlyRent averages a contract's fixed lease payments over its
// term. Using the average rather than the current instalment keeps leases with
// rent-free periods or step increases comparable with flat ones.
func straightLineMonthlyRent(fact *ContractFact) float64 {
	var totalFixed float64
	for _, schedule := range fact.PaymentSchedules {
		if schedule.IsVariable || schedule.IsNonLeaseComponent {
			continue
		}
		totalFixed += schedule.Amount
	}
	months := leaseTermMonths(fact.Contract)
	if months <= 0 {
		return 0
	}
	return totalFixed / months
}

// leaseTermMonths is the contract term in months, at least one, so a very short
// lease cannot divide by zero.
func leaseTermMonths(contract *repository.Contract) float64 {
	days := contract.LeaseEndDate.Sub(contract.CommencementDate).Hours() / 24
	if days <= 0 {
		return 0
	}
	months := days / averageDaysPerMonth
	if months < 1 {
		return 1
	}
	return months
}

const averageDaysPerMonth = 365.0 / 12.0
