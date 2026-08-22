package repository

import (
	"context"
	"fmt"

	"github.com/lease-management-system/core-service/internal/services/newstorefeasibility"
)

// RH4（R2-2）：LeaseProjectionReader 的生产适配器——从 measurement_results
// 只读投影月度租赁数字，与月结报表同源同数。租约金额不进新店测算的输入，
// 也不在这里重算：本适配器只 SELECT。
//
// 租户边界在构造时绑定 legal_entity_id；SQL 再以 lease_contracts.legal_entity_id
// 双重过滤（底线 1）。

type TenantBoundLeaseProjection struct {
	db           DBTX
	legalEntityID string
}

func NewTenantBoundLeaseProjection(db DBTX, legalEntityID string) *TenantBoundLeaseProjection {
	return &TenantBoundLeaseProjection{db: db, legalEntityID: legalEntityID}
}

func (r *TenantBoundLeaseProjection) MonthlyProjection(ctx context.Context, contractID, fromMonth string, months int) ([]newstorefeasibility.LeaseMonth, error) {
	rows, err := r.db.Query(ctx, `
		SELECT mr.accounting_period,
		       COALESCE(mr.total_payment, 0),
		       COALESCE(mr.depreciation, 0),
		       COALESCE(mr.interest_expense, 0)
		FROM measurement_results mr
		JOIN lease_contracts lc ON lc.id = mr.contract_id
		WHERE lc.legal_entity_id = $1
		  AND (mr.contract_id::text = $2 OR lc.contract_number = $2)
		  AND mr.is_calculated = true
		  AND mr.accounting_period >= $3
		ORDER BY mr.accounting_period
		LIMIT $4
	`, r.legalEntityID, contractID, fromMonth, months)
	if err != nil {
		return nil, fmt.Errorf("query lease projection: %w", err)
	}
	defer rows.Close()

	out := make([]newstorefeasibility.LeaseMonth, 0, months)
	for rows.Next() {
		var period string
		var payment, dep, interest float64
		if err := rows.Scan(&period, &payment, &dep, &interest); err != nil {
			return nil, fmt.Errorf("scan lease projection: %w", err)
		}
		out = append(out, newstorefeasibility.LeaseMonth{
			Month:           period,
			LeaseExpense:    payment, // 现金口径租金：可行性现金流按现金占用计
			ROUDepreciation: dep,
			InterestExpense: interest,
			TotalPayment:    payment,
		})
	}
	return out, rows.Err()
}
