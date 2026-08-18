package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/lease-management-system/core-service/internal/services/competitorreference"
)

type CompetitorRepository struct {
	db DBTX
}

func NewCompetitorRepository(db DBTX) *CompetitorRepository {
	return &CompetitorRepository{db: db}
}

func (r *CompetitorRepository) ListObservations(ctx context.Context, legalEntityID, storeID string) ([]competitorreference.Observation, error) {
	query := `
		SELECT id, legal_entity_id, store_id, competitor_name, competitor_brand,
		       distance_meters, observation_date, price_index, promo_intensity,
		       footfall_estimate, observer, notes, created_at
		FROM retail_competitor_observations
		WHERE legal_entity_id = $1
	`
	args := []interface{}{legalEntityID}
	if storeID != "" {
		query += " AND store_id = $2"
		args = append(args, storeID)
	}
	query += " ORDER BY observation_date DESC, created_at DESC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list competitor observations: %w", err)
	}
	defer rows.Close()

	var list []competitorreference.Observation
	for rows.Next() {
		var o competitorreference.Observation
		var oDate time.Time
		var brand, observer, notes *string
		if err := rows.Scan(
			&o.ID, &o.LegalEntityID, &o.StoreID, &o.CompetitorName, &brand,
			&o.DistanceMeters, &oDate, &o.PriceIndex, &o.PromoIntensity,
			&o.FootfallEstimate, &observer, &notes, &o.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan competitor obs: %w", err)
		}
		o.ObservationDate = oDate.Format("2006-01-02")
		if brand != nil {
			o.CompetitorBrand = *brand
		}
		if observer != nil {
			o.Observer = *observer
		}
		if notes != nil {
			o.Notes = *notes
		}
		list = append(list, o)
	}
	return list, nil
}

func (r *CompetitorRepository) AddObservation(ctx context.Context, o *competitorreference.Observation) error {
	query := `
		INSERT INTO retail_competitor_observations (
			legal_entity_id, store_id, competitor_name, competitor_brand,
			distance_meters, observation_date, price_index, promo_intensity,
			footfall_estimate, observer, notes
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at
	`
	return r.db.QueryRow(
		ctx, query,
		o.LegalEntityID, o.StoreID, o.CompetitorName, o.CompetitorBrand,
		o.DistanceMeters, o.ObservationDate, o.PriceIndex, o.PromoIntensity,
		o.FootfallEstimate, o.Observer, o.Notes,
	).Scan(&o.ID, &o.CreatedAt)
}
