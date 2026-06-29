package contracts

import (
	"context"

	"github.com/lease-management-system/core-service/internal/repository"
)

const DefaultDiscountRate = 0.05

type MeasurementResultReader interface {
	GetMeasurementResults(ctx context.Context, contractID, accountingPeriod string) ([]*repository.MeasurementResult, error)
}

func ResolveGlobalDiscountRate(ctx context.Context, repo *repository.SystemSettingRepository) float64 {
	if repo == nil {
		return 0
	}
	return NormalizeDiscountRate(repo.GetFloat64(ctx, "global_discount_rate", 0))
}

func ResolveDiscountRate(
	ctx context.Context,
	override float64,
	systemSettingRepo *repository.SystemSettingRepository,
	contract *repository.Contract,
	measurementReader MeasurementResultReader,
	includeMeasurementFallback bool,
) float64 {
	if rate := NormalizeDiscountRate(override); rate > 0 {
		return rate
	}

	if rate := ResolveGlobalDiscountRate(ctx, systemSettingRepo); rate > 0 {
		return rate
	}

	if contract != nil && contract.DiscountRateValue != nil {
		if rate := NormalizeDiscountRate(*contract.DiscountRateValue); rate > 0 {
			return rate
		}
	}

	if includeMeasurementFallback && measurementReader != nil && contract != nil {
		results, err := measurementReader.GetMeasurementResults(ctx, contract.ID, "")
		if err == nil && len(results) > 0 {
			latest := results[len(results)-1]
			if rate := NormalizeDiscountRate(latest.DiscountRate); rate > 0 {
				return rate
			}
		}
	}

	return DefaultDiscountRate
}

func NormalizeDiscountRate(rate float64) float64 {
	if rate <= 0 {
		return 0
	}
	if rate > 1 {
		return rate / 100
	}
	return rate
}
