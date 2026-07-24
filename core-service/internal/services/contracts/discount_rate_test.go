package contracts

import (
	"errors"
	"testing"
)

func TestResolveDiscountRateValues(t *testing.T) {
	contractRate := 0.06
	tests := []struct {
		name         string
		override     float64
		global       float64
		contractRate *float64
		scope        string
		wantRate     float64
		wantSource   string
		wantErr      error
	}{
		{"override", 0.04, 0.05, &contractRate, "in_scope", 0.04, "request_override", nil},
		{"contract confirmation", 0, 0.05, &contractRate, "in_scope", 0.06, "contract_confirmed", nil},
		{"global policy", 0, 5, nil, "in_scope", 0.05, "global_policy", nil},
		{"missing controlled input", 0, 0, nil, "in_scope", 0, "", ErrDiscountRateRequired},
		{"exempt lease", 0, 0, nil, "short_term_exempt", 0, "not_required", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rate, source, err := ResolveDiscountRateValues(tt.override, tt.global, tt.contractRate, tt.scope)
			if rate != tt.wantRate || source != tt.wantSource || !errors.Is(err, tt.wantErr) {
				t.Fatalf("got (%v, %q, %v), want (%v, %q, %v)", rate, source, err, tt.wantRate, tt.wantSource, tt.wantErr)
			}
		})
	}
}
