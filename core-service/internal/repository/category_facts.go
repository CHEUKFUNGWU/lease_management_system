package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/lease-management-system/core-service/internal/services/categoryreconciliation"
)

type RetailCategory struct {
	ID            string     `json:"id"`
	LegalEntityID string     `json:"legal_entity_id"`
	CategoryCode  string     `json:"category_code"`
	Name          string     `json:"name"`
	ParentCode    *string    `json:"parent_code,omitempty"`
	EffectiveFrom string     `json:"effective_from"`
	EffectiveTo   *string    `json:"effective_to,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

type RetailStoreDayCategoryFact struct {
	ID                       string    `json:"id"`
	LegalEntityID            string    `json:"legal_entity_id"`
	StoreID                  string    `json:"store_id"`
	BusinessDate             string    `json:"business_date"`
	Currency                 string    `json:"currency"`
	CategoryCode             string    `json:"category_code"`
	Revenue                  *float64  `json:"revenue,omitempty"`
	GrossProfit              *float64  `json:"gross_profit,omitempty"`
	Transactions             *int      `json:"transactions,omitempty"`
	Units                    *float64  `json:"units,omitempty"`
	SourceSystem             string    `json:"source_system"`
	ImportBatchID            string    `json:"import_batch_id"`
	AsOfAt                   time.Time `json:"as_of_at"`
	Version                  int       `json:"version"`
	DataClassification       string    `json:"data_classification"`
	SimulationDatasetVersion *string   `json:"simulation_dataset_version,omitempty"`
	DataQualityStatus        string    `json:"data_quality_status"`
	CreatedAt                time.Time `json:"created_at"`
}

type CategoryFactFilter struct {
	LegalEntityID            string
	StoreIDs                 []string
	FromDate                 string
	ToDate                   string
	CategoryCodes            []string
	DataClassification       string
	SimulationDatasetVersion string
}

type CategoryRepository struct {
	db DBTX
}

func NewCategoryRepository(db DBTX) *CategoryRepository {
	return &CategoryRepository{db: db}
}

func (r *CategoryRepository) ListCategories(ctx context.Context, legalEntityID string) ([]RetailCategory, error) {
	query := `
		SELECT id, legal_entity_id, category_code, name, parent_code, effective_from, effective_to, created_at
		FROM retail_categories
		WHERE legal_entity_id = $1
		ORDER BY category_code ASC
	`
	rows, err := r.db.Query(ctx, query, legalEntityID)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	defer rows.Close()

	var list []RetailCategory
	for rows.Next() {
		var c RetailCategory
		var parent, effTo *string
		var effFrom time.Time
		if err := rows.Scan(&c.ID, &c.LegalEntityID, &c.CategoryCode, &c.Name, &parent, &effFrom, &effTo, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}
		c.EffectiveFrom = effFrom.Format("2006-01-02")
		c.ParentCode = parent
		c.EffectiveTo = effTo
		list = append(list, c)
	}
	return list, nil
}

func (r *CategoryRepository) UpsertCategory(ctx context.Context, c *RetailCategory) error {
	query := `
		INSERT INTO retail_categories (
			legal_entity_id, category_code, name, parent_code, effective_from, effective_to
		) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (legal_entity_id, category_code, effective_from)
		DO UPDATE SET name = EXCLUDED.name, parent_code = EXCLUDED.parent_code, effective_to = EXCLUDED.effective_to
		RETURNING id, created_at
	`
	return r.db.QueryRow(
		ctx, query,
		c.LegalEntityID, c.CategoryCode, c.Name, c.ParentCode, c.EffectiveFrom, c.EffectiveTo,
	).Scan(&c.ID, &c.CreatedAt)
}

func (r *CategoryRepository) ListCategoryFacts(ctx context.Context, filter CategoryFactFilter) ([]RetailStoreDayCategoryFact, error) {
	query := `
		SELECT id, legal_entity_id, store_id, business_date, currency, category_code,
		       revenue, gross_profit, transactions, units, source_system, import_batch_id,
		       as_of_at, version, data_classification, simulation_dataset_version,
		       data_quality_status, created_at
		FROM retail_store_day_category_facts
		WHERE legal_entity_id = $1
	`
	args := []interface{}{filter.LegalEntityID}
	idx := 2

	if len(filter.StoreIDs) > 0 {
		query += fmt.Sprintf(" AND store_id = ANY($%d)", idx)
		args = append(args, filter.StoreIDs)
		idx++
	}
	if filter.FromDate != "" {
		query += fmt.Sprintf(" AND business_date >= $%d", idx)
		args = append(args, filter.FromDate)
		idx++
	}
	if filter.ToDate != "" {
		query += fmt.Sprintf(" AND business_date <= $%d", idx)
		args = append(args, filter.ToDate)
		idx++
	}
	if filter.DataClassification != "" {
		query += fmt.Sprintf(" AND data_classification = $%d", idx)
		args = append(args, filter.DataClassification)
		idx++
	}

	query += " ORDER BY business_date ASC, store_id ASC, category_code ASC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list category facts: %w", err)
	}
	defer rows.Close()

	var list []RetailStoreDayCategoryFact
	for rows.Next() {
		var f RetailStoreDayCategoryFact
		var bDate time.Time
		var simVer *string
		if err := rows.Scan(
			&f.ID, &f.LegalEntityID, &f.StoreID, &bDate, &f.Currency, &f.CategoryCode,
			&f.Revenue, &f.GrossProfit, &f.Transactions, &f.Units,
			&f.SourceSystem, &f.ImportBatchID, &f.AsOfAt, &f.Version,
			&f.DataClassification, &simVer, &f.DataQualityStatus, &f.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan category fact: %w", err)
		}
		f.BusinessDate = bDate.Format("2006-01-02")
		f.SimulationDatasetVersion = simVer
		list = append(list, f)
	}
	return list, nil
}

func (r *CategoryRepository) GetCategoryReconciliationData(
	ctx context.Context,
	legalEntityID string,
	storeIDs []string,
	fromDate string,
	toDate string,
	dataClassification string,
) ([]categoryreconciliation.CategoryFact, []categoryreconciliation.DailySummaryFact, error) {
	// 1. Fetch category facts
	catQuery := `
		SELECT store_id, business_date, currency, category_code, COALESCE(revenue, 0), COALESCE(gross_profit, 0), COALESCE(transactions, 0), COALESCE(units, 0)
		FROM retail_store_day_category_facts
		WHERE legal_entity_id = $1
	`
	catArgs := []interface{}{legalEntityID}
	cIdx := 2
	if len(storeIDs) > 0 {
		catQuery += fmt.Sprintf(" AND store_id = ANY($%d)", cIdx)
		catArgs = append(catArgs, storeIDs)
		cIdx++
	}
	if fromDate != "" {
		catQuery += fmt.Sprintf(" AND business_date >= $%d", cIdx)
		catArgs = append(catArgs, fromDate)
		cIdx++
	}
	if toDate != "" {
		catQuery += fmt.Sprintf(" AND business_date <= $%d", cIdx)
		catArgs = append(catArgs, toDate)
		cIdx++
	}
	if dataClassification != "" {
		catQuery += fmt.Sprintf(" AND data_classification = $%d", cIdx)
		catArgs = append(catArgs, dataClassification)
		cIdx++
	}

	catRows, err := r.db.Query(ctx, catQuery, catArgs...)
	if err != nil {
		return nil, nil, fmt.Errorf("query category facts for reconcile: %w", err)
	}
	defer catRows.Close()

	var details []categoryreconciliation.CategoryFact
	for catRows.Next() {
		var cf categoryreconciliation.CategoryFact
		var bDate time.Time
		if err := catRows.Scan(&cf.StoreID, &bDate, &cf.Currency, &cf.CategoryCode, &cf.Revenue, &cf.GrossProfit, &cf.Transactions, &cf.Units); err != nil {
			return nil, nil, fmt.Errorf("scan cat fact: %w", err)
		}
		cf.BusinessDate = bDate.Format("2006-01-02")
		details = append(details, cf)
	}

	// 2. Fetch store daily summary facts
	// Unlike retail_store_day_category_facts above, retail_store_day_facts has no
	// legal_entity_id: tenancy is reached through the store. The join is INNER so
	// a fact whose store is missing cannot slip past the tenant filter.
	sumQuery := `
		SELECT f.store_id, f.business_date, f.currency, COALESCE(f.revenue, 0), COALESCE(f.gross_profit, 0)
		FROM retail_store_day_facts f
		JOIN stores s ON s.id = f.store_id
		WHERE s.legal_entity_id = $1
	`
	sumArgs := []interface{}{legalEntityID}
	sIdx := 2
	if len(storeIDs) > 0 {
		sumQuery += fmt.Sprintf(" AND f.store_id = ANY($%d)", sIdx)
		sumArgs = append(sumArgs, storeIDs)
		sIdx++
	}
	if fromDate != "" {
		sumQuery += fmt.Sprintf(" AND f.business_date >= $%d", sIdx)
		sumArgs = append(sumArgs, fromDate)
		sIdx++
	}
	if toDate != "" {
		sumQuery += fmt.Sprintf(" AND f.business_date <= $%d", sIdx)
		sumArgs = append(sumArgs, toDate)
		sIdx++
	}
	if dataClassification != "" {
		sumQuery += fmt.Sprintf(" AND f.data_classification = $%d", sIdx)
		sumArgs = append(sumArgs, dataClassification)
		sIdx++
	}

	sumRows, err := r.db.Query(ctx, sumQuery, sumArgs...)
	if err != nil {
		return nil, nil, fmt.Errorf("query summary facts for reconcile: %w", err)
	}
	defer sumRows.Close()

	var summaries []categoryreconciliation.DailySummaryFact
	for sumRows.Next() {
		var sf categoryreconciliation.DailySummaryFact
		var bDate time.Time
		if err := sumRows.Scan(&sf.StoreID, &bDate, &sf.Currency, &sf.Revenue, &sf.GrossProfit); err != nil {
			return nil, nil, fmt.Errorf("scan summary fact: %w", err)
		}
		sf.BusinessDate = bDate.Format("2006-01-02")
		summaries = append(summaries, sf)
	}

	return details, summaries, nil
}
