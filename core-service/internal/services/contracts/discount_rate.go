package contracts

import (
	"context"
	"errors"

	"github.com/lease-management-system/core-service/internal/repository"
)

var ErrDiscountRateRequired = errors.New("discount rate requires policy matching or human confirmation")

type RateSource interface {
	GetFloat64(ctx context.Context, key string, fallback float64) float64
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

func RequiresDiscountRate(leaseScope string) bool {
	return leaseScope == "" || leaseScope == "in_scope"
}

// ResolveDiscountRate applies traceable inputs only. It never invents a
// default: an explicit human override wins, followed by the contract-confirmed
// rate and the configured policy rate.
func ResolveDiscountRate(ctx context.Context, override float64, rates RateSource, contract *repository.Contract) (float64, string, error) {
	globalRate := 0.0
	if rates != nil {
		globalRate = rates.GetFloat64(ctx, "global_discount_rate", 0)
	}
	var contractRate *float64
	leaseScope := ""
	if contract != nil {
		contractRate = contract.DiscountRateValue
		leaseScope = contract.LeaseScope
	}
	return ResolveDiscountRateValues(override, globalRate, contractRate, leaseScope)
}

func ResolveDiscountRateValues(override, globalRate float64, contractRate *float64, leaseScope string) (float64, string, error) {
	if rate := NormalizeDiscountRate(override); rate > 0 && rate <= 1 {
		return rate, "request_override", nil
	}
	if contractRate != nil {
		if rate := NormalizeDiscountRate(*contractRate); rate > 0 && rate <= 1 {
			return rate, "contract_confirmed", nil
		}
	}
	if rate := NormalizeDiscountRate(globalRate); rate > 0 && rate <= 1 {
		return rate, "global_policy", nil
	}
	if !RequiresDiscountRate(leaseScope) {
		return 0, "not_required", nil
	}
	return 0, "", ErrDiscountRateRequired
}
