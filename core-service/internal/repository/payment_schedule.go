package repository

import (
	"context"
	"fmt"
	"time"
	
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ifrs16/core-service/internal/services/ifrs16"
)

type PaymentSchedule struct {
	ID                     string     `json:"id"`
	ContractID             string     `json:"contract_id"`
	EffectiveStartDate     time.Time  `json:"effective_start_date"`
	EffectiveEndDate       time.Time  `json:"effective_end_date"`
	CoverageStartDate      time.Time  `json:"coverage_start_date"`
	CoverageEndDate        time.Time  `json:"coverage_end_date"`
	DueDate                time.Time  `json:"due_date"`
	ActualPaymentDate      *time.Time `json:"actual_payment_date"`
	PaymentTiming          string     `json:"payment_timing"`
	Amount                 float64    `json:"amount"`
	Currency               string     `json:"currency"`
	TaxAmount              *float64   `json:"tax_amount"`
	AmountType             string     `json:"amount_type"`
	IsFixed                bool       `json:"is_fixed"`
	IsVariable             bool       `json:"is_variable"`
	IsIndexAdjusted        bool       `json:"is_index_adjusted"`
	IsLeaseComponent       bool       `json:"is_lease_component"`
	IsNonLeaseComponent    bool       `json:"is_non_lease_component"`
	IncludedInLiabilityPV  bool       `json:"included_in_liability_pv"`
	ApprovalStatus         string     `json:"approval_status"`
	IsOfficialVersion      bool       `json:"is_official_version"`
	ReviewedBy             *string    `json:"reviewed_by"`
	ApprovedBy             *string    `json:"approved_by"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

type PaymentScheduleRepository struct {
	db *pgxpool.Pool
}

func NewPaymentScheduleRepository(db *pgxpool.Pool) *PaymentScheduleRepository {
	return &PaymentScheduleRepository{db: db}
}

func (r *PaymentScheduleRepository) Create(ctx context.Context, ps *PaymentSchedule) (*PaymentSchedule, error) {
	ps.ID = uuid.New().String()
	ps.CreatedAt = time.Now()
	ps.UpdatedAt = time.Now()
	if ps.ApprovalStatus == "" {
		ps.ApprovalStatus = "approved"
	}
	if ps.Currency == "" {
		ps.Currency = "CNY"
	}
	
	query := `
		INSERT INTO lease_payment_schedules (
			id, contract_id, effective_start_date, effective_end_date,
			coverage_start_date, coverage_end_date, due_date, actual_payment_date,
			payment_timing, amount, currency, tax_amount, amount_type,
			is_fixed, is_variable, is_index_adjusted, is_lease_component,
			is_non_lease_component, included_in_liability_pv,
			approval_status, is_official_version, reviewed_by, approved_by,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25)
		RETURNING approval_status, is_official_version, created_at, updated_at
	`
	
	err := r.db.QueryRow(ctx, query,
		ps.ID, ps.ContractID, ps.EffectiveStartDate, ps.EffectiveEndDate,
		ps.CoverageStartDate, ps.CoverageEndDate, ps.DueDate, ps.ActualPaymentDate,
		ps.PaymentTiming, ps.Amount, ps.Currency, ps.TaxAmount, ps.AmountType,
		ps.IsFixed, ps.IsVariable, ps.IsIndexAdjusted, ps.IsLeaseComponent,
		ps.IsNonLeaseComponent, ps.IncludedInLiabilityPV,
		ps.ApprovalStatus, ps.IsOfficialVersion, ps.ReviewedBy, ps.ApprovedBy,
		ps.CreatedAt, ps.UpdatedAt,
	).Scan(&ps.ApprovalStatus, &ps.IsOfficialVersion, &ps.CreatedAt, &ps.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create payment schedule: %w", err)
	}
	
	return ps, nil
}

func (r *PaymentScheduleRepository) GetByContractID(ctx context.Context, contractID string) ([]*PaymentSchedule, error) {
	query := `
		SELECT id, contract_id, effective_start_date, effective_end_date,
			coverage_start_date, coverage_end_date, due_date, actual_payment_date,
			payment_timing, amount, currency, tax_amount, amount_type,
			is_fixed, is_variable, is_index_adjusted, is_lease_component,
			is_non_lease_component, included_in_liability_pv,
			approval_status, is_official_version, reviewed_by, approved_by,
			created_at, updated_at
		FROM lease_payment_schedules
		WHERE contract_id = $1
		ORDER BY due_date ASC
	`
	
	rows, err := r.db.Query(ctx, query, contractID)
	if err != nil {
		return nil, fmt.Errorf("failed to list payment schedules: %w", err)
	}
	defer rows.Close()
	
	var schedules []*PaymentSchedule
	for rows.Next() {
		ps := &PaymentSchedule{}
		err := rows.Scan(
			&ps.ID, &ps.ContractID, &ps.EffectiveStartDate, &ps.EffectiveEndDate,
			&ps.CoverageStartDate, &ps.CoverageEndDate, &ps.DueDate, &ps.ActualPaymentDate,
			&ps.PaymentTiming, &ps.Amount, &ps.Currency, &ps.TaxAmount, &ps.AmountType,
			&ps.IsFixed, &ps.IsVariable, &ps.IsIndexAdjusted, &ps.IsLeaseComponent,
			&ps.IsNonLeaseComponent, &ps.IncludedInLiabilityPV,
			&ps.ApprovalStatus, &ps.IsOfficialVersion, &ps.ReviewedBy, &ps.ApprovedBy,
			&ps.CreatedAt, &ps.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan payment schedule: %w", err)
		}
		schedules = append(schedules, ps)
	}
	
	return schedules, nil
}

func (r *PaymentScheduleRepository) GetByID(ctx context.Context, id string) (*PaymentSchedule, error) {
	query := `
		SELECT id, contract_id, effective_start_date, effective_end_date,
			coverage_start_date, coverage_end_date, due_date, actual_payment_date,
			payment_timing, amount, currency, tax_amount, amount_type,
			is_fixed, is_variable, is_index_adjusted, is_lease_component,
			is_non_lease_component, included_in_liability_pv,
			approval_status, is_official_version, reviewed_by, approved_by,
			created_at, updated_at
		FROM lease_payment_schedules WHERE id = $1
	`
	
	ps := &PaymentSchedule{}
	err := r.db.QueryRow(ctx, query, id).Scan(
		&ps.ID, &ps.ContractID, &ps.EffectiveStartDate, &ps.EffectiveEndDate,
		&ps.CoverageStartDate, &ps.CoverageEndDate, &ps.DueDate, &ps.ActualPaymentDate,
		&ps.PaymentTiming, &ps.Amount, &ps.Currency, &ps.TaxAmount, &ps.AmountType,
		&ps.IsFixed, &ps.IsVariable, &ps.IsIndexAdjusted, &ps.IsLeaseComponent,
		&ps.IsNonLeaseComponent, &ps.IncludedInLiabilityPV,
		&ps.ApprovalStatus, &ps.IsOfficialVersion, &ps.ReviewedBy, &ps.ApprovedBy,
		&ps.CreatedAt, &ps.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get payment schedule: %w", err)
	}
	
	return ps, nil
}

func (r *PaymentScheduleRepository) DeleteByContractID(ctx context.Context, contractID string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM lease_payment_schedules WHERE contract_id = $1`, contractID)
	if err != nil {
		return fmt.Errorf("failed to delete payment schedules: %w", err)
	}
	return nil
}

// ToIFRS16Payments converts repository payment schedules to IFRS 16 engine format.
// ALL payments are passed to the engine — filtering (variable/non-lease exclusion from PV)
// is handled by the calculation engine itself.
func ToIFRS16Payments(schedules []*PaymentSchedule) []ifrs16.LeasePayment {
	var payments []ifrs16.LeasePayment
	for _, ps := range schedules {
		paymentType := "fixed"
		if ps.IsVariable || ps.AmountType == "turnover_rent" {
			paymentType = "variable"
		} else if ps.IsNonLeaseComponent || ps.AmountType == "cam" || ps.AmountType == "service_charge" {
			paymentType = "non_lease"
		}

		payments = append(payments, ifrs16.LeasePayment{
			Date:   ps.DueDate,
			Amount: ps.Amount,
			Timing: ps.PaymentTiming,
			Type:   paymentType,
		})
	}
	return payments
}
