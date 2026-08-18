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
		       stock_qty, stock_cost, in_transit_qty, in_transit_cost, days_of_inventory, created_at
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
			&f.StockQty, &f.StockCost, &f.InTransitQty, &f.InTransitCost, &f.DaysOfInventory, &f.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan inventory fact: %w", err)
		}
		f.BusinessDate = bDate.Format("2006-01-02")
		list = append(list, f)
	}
	return list, nil
}

func (r *InventoryRepository) UpsertInventoryFact(ctx context.Context, f *StoreDayInventoryFact) error {
	query := `
		INSERT INTO retail_store_day_inventory_facts (
			legal_entity_id, store_id, business_date, currency, category_code, sku_code,
			stock_qty, stock_cost, in_transit_qty, in_transit_cost, days_of_inventory
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (legal_entity_id, store_id, business_date, currency, COALESCE(category_code, ''), COALESCE(sku_code, ''))
		DO UPDATE SET
			stock_qty = EXCLUDED.stock_qty,
			stock_cost = EXCLUDED.stock_cost,
			in_transit_qty = EXCLUDED.in_transit_qty,
			in_transit_cost = EXCLUDED.in_transit_cost,
			days_of_inventory = EXCLUDED.days_of_inventory
		RETURNING id, created_at
	`
	return r.db.QueryRow(
		ctx, query,
		f.LegalEntityID, f.StoreID, f.BusinessDate, f.Currency, f.CategoryCode, f.SKUCode,
		f.StockQty, f.StockCost, f.InTransitQty, f.InTransitCost, f.DaysOfInventory,
	).Scan(&f.ID, &f.CreatedAt)
}
