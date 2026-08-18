package repository

import (
	"context"
	"fmt"
	"time"
)

type StoreDayInventoryFact struct {
	ID               string    `json:"id"`
	LegalEntityID    string    `json:"legal_entity_id"`
	StoreID          string    `json:"store_id"`
	BusinessDate     string    `json:"business_date"`
	Currency         string    `json:"currency"`
	CategoryCode     *string   `json:"category_code,omitempty"`
	SKUCode          *string   `json:"sku_code,omitempty"`
	StockQty         float64   `json:"stock_qty"`
	StockCost        float64   `json:"stock_cost"`
	InTransitQty     float64   `json:"in_transit_qty"`
	InTransitCost    float64   `json:"in_transit_cost"`
	DaysOfInventory  *float64  `json:"days_of_inventory,omitempty"`
	SourceSystem     string    `json:"source_system"`
	ImportBatchID    *string   `json:"import_batch_id,omitempty"`
	AsOfAt           *time.Time `json:"as_of_at,omitempty"`
	Version          int       `json:"version"`
	DataClassification string  `json:"data_classification"`
	CreatedAt        time.Time `json:"created_at"`
}

type InventoryRepository struct {
	db DBTX
}

func NewInventoryRepository(db DBTX) *InventoryRepository {
	return &InventoryRepository{db: db}
}

func (r *InventoryRepository) ListInventoryFacts(ctx context.Context, legalEntityID, storeID, fromDate, toDate string) ([]StoreDayInventoryFact, error) {
	query := `
		SELECT id, legal_entity_id, store_id, business_date, currency, category_code, sku_code,
		       stock_qty, stock_cost, in_transit_qty, in_transit_cost, days_of_inventory,
		       source_system, import_batch_id, as_of_at, version, data_classification, created_at
		FROM retail_store_day_inventory_facts
		WHERE legal_entity_id = $1
	`
	args := []interface{}{legalEntityID}
	idx := 2

	if storeID != "" {
		query += fmt.Sprintf(" AND store_id = $%d", idx)
		args = append(args, storeID)
		idx++
	}
	if fromDate != "" {
		query += fmt.Sprintf(" AND business_date >= $%d", idx)
		args = append(args, fromDate)
		idx++
	}
	if toDate != "" {
		query += fmt.Sprintf(" AND business_date <= $%d", idx)
		args = append(args, toDate)
		idx++
	}
	query += " ORDER BY business_date DESC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list inventory facts: %w", err)
	}
	defer rows.Close()

	var list []StoreDayInventoryFact
	for rows.Next() {
		var f StoreDayInventoryFact
		var bDate time.Time
		if err := rows.Scan(
			&f.ID, &f.LegalEntityID, &f.StoreID, &bDate, &f.Currency, &f.CategoryCode, &f.SKUCode,
			&f.StockQty, &f.StockCost, &f.InTransitQty, &f.InTransitCost, &f.DaysOfInventory,
			&f.SourceSystem, &f.ImportBatchID, &f.AsOfAt, &f.Version, &f.DataClassification, &f.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan inventory fact: %w", err)
		}
		f.BusinessDate = bDate.Format("2006-01-02")
		list = append(list, f)
	}
	return list, nil
}

func (r *InventoryRepository) UpsertInventoryFact(ctx context.Context, f *StoreDayInventoryFact) error {
	if f.Version <= 0 {
		f.Version = 1
	}
	if f.SourceSystem == "" {
		f.SourceSystem = "manual_import"
	}
	if f.DataClassification == "" {
		f.DataClassification = "production"
	}
	query := `
		INSERT INTO retail_store_day_inventory_facts (
			legal_entity_id, store_id, business_date, currency, category_code, sku_code,
			stock_qty, stock_cost, in_transit_qty, in_transit_cost, days_of_inventory,
			source_system, import_batch_id, as_of_at, version, data_classification
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		ON CONFLICT (legal_entity_id, store_id, business_date, currency, COALESCE(category_code, ''), COALESCE(sku_code, ''))
		DO UPDATE SET
			stock_qty = EXCLUDED.stock_qty,
			stock_cost = EXCLUDED.stock_cost,
			in_transit_qty = EXCLUDED.in_transit_qty,
			in_transit_cost = EXCLUDED.in_transit_cost,
			days_of_inventory = EXCLUDED.days_of_inventory,
			source_system = EXCLUDED.source_system,
			import_batch_id = EXCLUDED.import_batch_id,
			as_of_at = EXCLUDED.as_of_at,
			version = EXCLUDED.version,
			data_classification = EXCLUDED.data_classification
		RETURNING id, created_at
	`
	return r.db.QueryRow(
		ctx, query,
		f.LegalEntityID, f.StoreID, f.BusinessDate, f.Currency, f.CategoryCode, f.SKUCode,
		f.StockQty, f.StockCost, f.InTransitQty, f.InTransitCost, f.DaysOfInventory,
		f.SourceSystem, f.ImportBatchID, f.AsOfAt, f.Version, f.DataClassification,
	).Scan(&f.ID, &f.CreatedAt)
}
