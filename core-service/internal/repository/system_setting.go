package repository

import (
	"context"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type SystemSetting struct {
	SettingKey   string    `json:"setting_key"`
	SettingValue string    `json:"setting_value"`
	Description  *string   `json:"description"`
	UpdatedBy    *string   `json:"updated_by"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type SystemSettingRepository struct {
	db *pgxpool.Pool
}

func NewSystemSettingRepository(db *pgxpool.Pool) *SystemSettingRepository {
	return &SystemSettingRepository{db: db}
}

func (r *SystemSettingRepository) Get(ctx context.Context, key string) (*SystemSetting, error) {
	var s SystemSetting
	err := r.db.QueryRow(ctx,
		`SELECT setting_key, setting_value, description, updated_by, updated_at
		 FROM system_settings WHERE setting_key = $1`, key).
		Scan(&s.SettingKey, &s.SettingValue, &s.Description, &s.UpdatedBy, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *SystemSettingRepository) Upsert(ctx context.Context, setting *SystemSetting) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO system_settings (setting_key, setting_value, description, updated_by, updated_at)
		 VALUES ($1, $2, $3, $4, NOW())
		 ON CONFLICT (setting_key) DO UPDATE
		 SET setting_value = EXCLUDED.setting_value,
		     description = EXCLUDED.description,
		     updated_by = EXCLUDED.updated_by,
		     updated_at = NOW()`,
		setting.SettingKey, setting.SettingValue, setting.Description, setting.UpdatedBy)
	return err
}

// GetFloat64 parses the stored value and returns it as float64.
// If stored value > 1, it is treated as a percentage and divided by 100.
// If parse fails or the setting is missing, returns fallback.
func (r *SystemSettingRepository) GetFloat64(ctx context.Context, key string, fallback float64) float64 {
	s, err := r.Get(ctx, key)
	if err != nil || s == nil {
		return fallback
	}
	val, err := strconv.ParseFloat(s.SettingValue, 64)
	if err != nil {
		return fallback
	}
	if val < 0 {
		return fallback
	}
	if val > 1 {
		val = val / 100.0
	}
	return val
}
