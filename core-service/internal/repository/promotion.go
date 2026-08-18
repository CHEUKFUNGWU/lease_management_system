package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/lease-management-system/core-service/internal/services/promotionattribution"
)

type Promotion struct {
	ID             string    `json:"id"`
	LegalEntityID  string    `json:"legal_entity_id"`
	PromoCode      string    `json:"promo_code"`
	Name           string    `json:"name"`
	PromoType      string    `json:"promo_type"`
	StartDate      string    `json:"start_date"`
	EndDate        string    `json:"end_date"`
	TargetScope    string    `json:"target_scope"`
	ScopeValues    []string  `json:"scope_values"`
	Currency       string    `json:"currency"`
	BudgetAmount   float64   `json:"budget_amount"`
	ApprovalStatus string    `json:"approval_status"`
	Owner          *string   `json:"owner,omitempty"`
	Description    *string   `json:"description,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type PromotionCost struct {
	ID           string    `json:"id"`
	PromotionID  string    `json:"promotion_id"`
	Period       string    `json:"period"`
	CostCategory string    `json:"cost_category"`
	Amount       float64   `json:"amount"`
	Currency     string    `json:"currency"`
	Notes        *string   `json:"notes,omitempty"`
	SourceSystem string    `json:"source_system"`
	ImportBatchID *string  `json:"import_batch_id,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type PromotionRepository struct {
	db DBTX
}

func NewPromotionRepository(db DBTX) *PromotionRepository {
	return &PromotionRepository{db: db}
}

func (r *PromotionRepository) ListPromotions(ctx context.Context, legalEntityID, status string) ([]Promotion, error) {
	query := `
		SELECT id, legal_entity_id, promo_code, name, promo_type, start_date, end_date,
		       target_scope, scope_values, currency, budget_amount, approval_status,
		       owner, description, created_at
		FROM promotions
		WHERE legal_entity_id = $1
	`
	args := []interface{}{legalEntityID}
	if status != "" {
		query += " AND approval_status = $2"
		args = append(args, status)
	}
	query += " ORDER BY start_date DESC, created_at DESC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list promotions: %w", err)
	}
	defer rows.Close()

	var list []Promotion
	for rows.Next() {
		var p Promotion
		var st, et time.Time
		if err := rows.Scan(
			&p.ID, &p.LegalEntityID, &p.PromoCode, &p.Name, &p.PromoType,
			&st, &et, &p.TargetScope, &p.ScopeValues, &p.Currency,
			&p.BudgetAmount, &p.ApprovalStatus, &p.Owner, &p.Description, &p.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan promo: %w", err)
		}
		p.StartDate = st.Format("2006-01-02")
		p.EndDate = et.Format("2006-01-02")
		list = append(list, p)
	}
	return list, nil
}

func (r *PromotionRepository) GetPromotion(ctx context.Context, legalEntityID, id string) (*Promotion, error) {
	query := `
		SELECT id, legal_entity_id, promo_code, name, promo_type, start_date, end_date,
		       target_scope, scope_values, currency, budget_amount, approval_status,
		       owner, description, created_at
		FROM promotions
		WHERE legal_entity_id = $1 AND (id::text = $2 OR promo_code = $2)
	`
	var p Promotion
	var st, et time.Time
	err := r.db.QueryRow(ctx, query, legalEntityID, id).Scan(
		&p.ID, &p.LegalEntityID, &p.PromoCode, &p.Name, &p.PromoType,
		&st, &et, &p.TargetScope, &p.ScopeValues, &p.Currency,
		&p.BudgetAmount, &p.ApprovalStatus, &p.Owner, &p.Description, &p.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get promotion: %w", err)
	}
	p.StartDate = st.Format("2006-01-02")
	p.EndDate = et.Format("2006-01-02")
	return &p, nil
}

func (r *PromotionRepository) CreatePromotion(ctx context.Context, p *Promotion) error {
	query := `
		INSERT INTO promotions (
			legal_entity_id, promo_code, name, promo_type, start_date, end_date,
			target_scope, scope_values, currency, budget_amount, approval_status,
			owner, description
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id, created_at
	`
	return r.db.QueryRow(
		ctx, query,
		p.LegalEntityID, p.PromoCode, p.Name, p.PromoType, p.StartDate, p.EndDate,
		p.TargetScope, p.ScopeValues, p.Currency, p.BudgetAmount, p.ApprovalStatus,
		p.Owner, p.Description,
	).Scan(&p.ID, &p.CreatedAt)
}

func (r *PromotionRepository) UpdatePromotion(ctx context.Context, p *Promotion) error {
	query := `
		UPDATE promotions
		SET name = $1, promo_type = $2, start_date = $3, end_date = $4,
		    target_scope = $5, scope_values = $6, budget_amount = $7,
		    approval_status = $8, owner = $9, description = $10
		WHERE legal_entity_id = $11 AND id = $12
	`
	_, err := r.db.Exec(
		ctx, query,
		p.Name, p.PromoType, p.StartDate, p.EndDate,
		p.TargetScope, p.ScopeValues, p.BudgetAmount,
		p.ApprovalStatus, p.Owner, p.Description,
		p.LegalEntityID, p.ID,
	)
	return err
}

func (r *PromotionRepository) ListPromotionCosts(ctx context.Context, promoID string) ([]PromotionCost, error) {
	query := `
		SELECT id, promotion_id, period, cost_category, amount, currency, notes, created_at
		FROM promotion_costs
		WHERE promotion_id = $1
		ORDER BY created_at ASC
	`
	rows, err := r.db.Query(ctx, query, promoID)
	if err != nil {
		return nil, fmt.Errorf("list promo costs: %w", err)
	}
	defer rows.Close()

	var list []PromotionCost
	for rows.Next() {
		var c PromotionCost
		if err := rows.Scan(
			&c.ID, &c.PromotionID, &c.Period, &c.CostCategory, &c.Amount, &c.Currency, &c.Notes, &c.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan promo cost: %w", err)
		}
		list = append(list, c)
	}
	return list, nil
}

func (r *PromotionRepository) AddPromotionCost(ctx context.Context, c *PromotionCost) error {
	if c.SourceSystem == "" {
		c.SourceSystem = "manual_import"
	}
	query := `
		INSERT INTO promotion_costs (
			promotion_id, period, cost_category, amount, currency, notes, source_system, import_batch_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (promotion_id, period, cost_category, amount, currency)
		DO UPDATE SET
			notes = EXCLUDED.notes,
			source_system = EXCLUDED.source_system,
			import_batch_id = EXCLUDED.import_batch_id
		RETURNING id, created_at
	`
	return r.db.QueryRow(
		ctx, query,
		c.PromotionID, c.Period, c.CostCategory, c.Amount, c.Currency, c.Notes, c.SourceSystem, c.ImportBatchID,
	).Scan(&c.ID, &c.CreatedAt)
}

func (r *PromotionRepository) GetOverlappingPromotions(ctx context.Context, legalEntityID, promoID, startDate, endDate string) ([]Promotion, error) {
	query := `
		SELECT id, legal_entity_id, promo_code, name, promo_type, start_date, end_date,
		       target_scope, scope_values, currency, budget_amount, approval_status,
		       owner, description, created_at
		FROM promotions
		WHERE legal_entity_id = $1 AND id != $2
		  AND NOT (end_date < $3 OR start_date > $4)
	`
	rows, err := r.db.Query(ctx, query, legalEntityID, promoID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("get overlapping promos: %w", err)
	}
	defer rows.Close()

	var list []Promotion
	for rows.Next() {
		var p Promotion
		var st, et time.Time
		if err := rows.Scan(
			&p.ID, &p.LegalEntityID, &p.PromoCode, &p.Name, &p.PromoType,
			&st, &et, &p.TargetScope, &p.ScopeValues, &p.Currency,
			&p.BudgetAmount, &p.ApprovalStatus, &p.Owner, &p.Description, &p.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan promo overlap: %w", err)
		}
		p.StartDate = st.Format("2006-01-02")
		p.EndDate = et.Format("2006-01-02")
		list = append(list, p)
	}
	return list, nil
}

func (r *PromotionRepository) GetPromotionActualFacts(ctx context.Context, legalEntityID, startDate, endDate string, storeIDs []string) ([]promotionattribution.DailyFact, error) {
	// retail_store_day_facts carries no legal_entity_id; tenancy is reached
	// through the store, and the join is INNER so a fact whose store is missing
	// cannot slip past the tenant filter.
	query := `
		SELECT f.store_id, f.business_date, f.currency, f.revenue, f.gross_profit, f.transactions
		FROM retail_store_day_facts f
		JOIN stores s ON s.id = f.store_id
		WHERE s.legal_entity_id = $1
		  AND f.business_date >= $2 AND f.business_date <= $3
	`
	args := []interface{}{legalEntityID, startDate, endDate}
	if len(storeIDs) > 0 {
		query += " AND f.store_id = ANY($4)"
		args = append(args, storeIDs)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query promo actuals: %w", err)
	}
	defer rows.Close()

	var list []promotionattribution.DailyFact
	for rows.Next() {
		var f promotionattribution.DailyFact
		var bDate time.Time
		// revenue/gross_profit/transactions 保留 NULL（扫描进指针），缺失绝不当作 0。
		if err := rows.Scan(&f.StoreID, &bDate, &f.Currency, &f.Revenue, &f.GrossProfit, &f.Transactions); err != nil {
			return nil, fmt.Errorf("scan promo fact: %w", err)
		}
		f.BusinessDate = bDate.Format("2006-01-02")
		list = append(list, f)
	}
	return list, nil
}
