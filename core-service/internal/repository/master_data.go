package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lease-management-system/core-service/internal/access"
)

type LegalEntityOption struct {
	ID       string `json:"id"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	Country  string `json:"country"`
	Currency string `json:"currency"`
}

type StoreOption struct {
	ID            string  `json:"id"`
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	LegalEntityID string  `json:"legal_entity_id"`
	Brand         *string `json:"brand"`
	Region        *string `json:"region"`
	Address       *string `json:"address"`
}

type LandlordOption struct {
	ID      string  `json:"id"`
	Code    string  `json:"code"`
	Name    string  `json:"name"`
	Address *string `json:"address"`
}

type MasterDataRepository struct {
	db *pgxpool.Pool
}

func NewMasterDataRepository(db *pgxpool.Pool) *MasterDataRepository {
	return &MasterDataRepository{db: db}
}

func (r *MasterDataRepository) ListLegalEntities(ctx context.Context, tenantID string) ([]LegalEntityOption, error) {
	query := `
		SELECT id, code, name, country, currency
		FROM legal_entities
		WHERE is_active = true
	`
	args := []any{}
	if tenantID != "" {
		query += ` AND id = $1`
		args = append(args, tenantID)
	}
	query += ` ORDER BY code`

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list legal entities: %w", err)
	}
	defer rows.Close()

	var entities []LegalEntityOption
	for rows.Next() {
		var entity LegalEntityOption
		if err := rows.Scan(&entity.ID, &entity.Code, &entity.Name, &entity.Country, &entity.Currency); err != nil {
			return nil, fmt.Errorf("failed to scan legal entity: %w", err)
		}
		entities = append(entities, entity)
	}

	return entities, rows.Err()
}

// FunctionalCurrency returns the legal entity's reporting currency, which is
// what a foreign-currency lease is translated into. An unknown entity yields an
// empty string so callers can decide whether that is an error in their context.
func (r *MasterDataRepository) FunctionalCurrency(ctx context.Context, legalEntityID string) (string, error) {
	if legalEntityID == "" {
		return "", nil
	}
	var currency string
	err := r.db.QueryRow(ctx, `SELECT currency FROM legal_entities WHERE id = $1`, legalEntityID).Scan(&currency)
	if err == pgx.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to read functional currency: %w", err)
	}
	return currency, nil
}

func (r *MasterDataRepository) ListStores(ctx context.Context, tenantID, legalEntityID string) ([]StoreOption, error) {
	query := `
		SELECT id, code, name, legal_entity_id, brand, region, address
		FROM stores
		WHERE is_active = true
	`
	args := []any{}
	argIdx := 1
	filterEntityID := legalEntityID
	if scope, scoped := access.ScopeFromContext(ctx); scoped && !scope.Global {
		filterEntityID = scope.LegalEntityID
		if filterEntityID == "" {
			query += ` AND false`
		}
		if len(scope.StoreIDs) > 0 {
			query += fmt.Sprintf(" AND id::text = ANY($%d)", argIdx)
			args = append(args, scope.StoreIDs)
			argIdx++
		}
		if len(scope.Regions) > 0 {
			query += fmt.Sprintf(" AND region = ANY($%d)", argIdx)
			args = append(args, scope.Regions)
			argIdx++
		}
		if len(scope.Brands) > 0 {
			query += fmt.Sprintf(" AND brand = ANY($%d)", argIdx)
			args = append(args, scope.Brands)
			argIdx++
		}
	} else if tenantID != "" {
		filterEntityID = tenantID
	}
	if filterEntityID != "" {
		query += fmt.Sprintf(" AND legal_entity_id::text = $%d", argIdx)
		args = append(args, filterEntityID)
		argIdx++
	}
	query += ` ORDER BY name`

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list stores: %w", err)
	}
	defer rows.Close()

	var stores []StoreOption
	for rows.Next() {
		var store StoreOption
		if err := rows.Scan(&store.ID, &store.Code, &store.Name, &store.LegalEntityID, &store.Brand, &store.Region, &store.Address); err != nil {
			return nil, fmt.Errorf("failed to scan store: %w", err)
		}
		stores = append(stores, store)
	}

	return stores, rows.Err()
}

func (r *MasterDataRepository) ListLandlords(ctx context.Context) ([]LandlordOption, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, code, name, address
		FROM landlords
		WHERE is_active = true
		ORDER BY name
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list landlords: %w", err)
	}
	defer rows.Close()

	var landlords []LandlordOption
	for rows.Next() {
		var landlord LandlordOption
		if err := rows.Scan(&landlord.ID, &landlord.Code, &landlord.Name, &landlord.Address); err != nil {
			return nil, fmt.Errorf("failed to scan landlord: %w", err)
		}
		landlords = append(landlords, landlord)
	}

	return landlords, rows.Err()
}
