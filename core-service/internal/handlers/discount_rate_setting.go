package handlers

import (
	"context"

	"github.com/lease-management-system/core-service/internal/repository"
)

func resolveGlobalDiscountRate(ctx context.Context, repo *repository.SystemSettingRepository) float64 {
	if repo == nil {
		return 0
	}
	return repo.GetFloat64(ctx, "global_discount_rate", 0)
}
