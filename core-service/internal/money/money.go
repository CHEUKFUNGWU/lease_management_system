// Package money is the single seam for monetary amounts (ADR-0020). It owns
// the currency scale table, the rounding policy and the JSON contract;
// shopspring/decimal is the numeric engine underneath. Intermediate
// arithmetic carries full precision — amounts are quantised only at the three
// named boundaries: persisting to a DECIMAL column, emitting a journal entry
// line, and serialising an API response.
package money

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"

	"github.com/shopspring/decimal"
)

// currencyScale is the number of decimal places each currency allows (ADR
// §Decision 5). Default is 2; the exceptions are explicit. Adding a currency
// is a data change, not a code change.
var currencyScale = map[string]int32{
	"JPY": 0,
	"KWD": 3, "BHD": 3, "OMR": 3,
}

// DefaultScale is the scale used when a currency has no explicit entry.
const DefaultScale int32 = 2

// Amount is a monetary value in a currency. The zero value is not a valid
// amount — construct with New / FromFloat / FromDecimal.
type Amount struct {
	value decimal.Decimal
	set   bool
}

// ErrInvalidAmount marks a zero-value Amount used as if it were set.
var ErrInvalidAmount = errors.New("money: amount is not set")

// ErrPrecisionExceeded marks an amount carrying more precision than its
// currency allows; the caller must reject it, never round it away silently
// (ADR §Decision 5).
var ErrPrecisionExceeded = errors.New("money: amount exceeds the currency's allowed precision")

func New(value decimal.Decimal) Amount { return Amount{value: value, set: true} }

func NewFromInt64(value int64) Amount { return New(decimal.NewFromInt(value)) }

// NewFromFloat converts a float64 with full-precision decimal semantics.
func NewFromFloat(value float64) Amount { return New(decimal.NewFromFloat(value)) }

// FromDecimalString parses a decimal literal.
func FromDecimalString(value string) (Amount, error) {
	parsed, err := decimal.NewFromString(value)
	if err != nil {
		return Amount{}, fmt.Errorf("money: parse %q: %w", value, err)
	}
	return New(parsed), nil
}

// IsZero reports whether the amount is set and zero.
func (a Amount) IsZero() bool { return a.set && a.value.IsZero() }

// IsSet reports whether the amount was constructed (the zero value is not).
func (a Amount) IsSet() bool { return a.set }

// Decimal returns the underlying decimal; a zero-value Amount yields the
// zero decimal so callers that only carry amounts can format defensively.
func (a Amount) Decimal() decimal.Decimal {
	if !a.set {
		return decimal.Zero
	}
	return a.value
}

func (a Amount) Add(other Amount) Amount { return New(a.Decimal().Add(other.Decimal())) }
func (a Amount) Sub(other Amount) Amount { return New(a.Decimal().Sub(other.Decimal())) }
func (a Amount) Mul(other Amount) Amount { return New(a.Decimal().Mul(other.Decimal())) }
func (a Amount) Div(other Amount) (Amount, error) {
	if other.Decimal().IsZero() {
		return Amount{}, errors.New("money: division by zero")
	}
	return New(a.Decimal().Div(other.Decimal())), nil
}
func (a Amount) Neg() Amount { return New(a.Decimal().Neg()) }

// Abs returns the amount with a non-negative sign.
func (a Amount) Abs() Amount { return New(a.Decimal().Abs()) }

// Cmp compares two amounts: -1, 0 or 1.
func (a Amount) Cmp(other Amount) int { return a.Decimal().Cmp(other.Decimal()) }

// Equal reports exact decimal equality.
func (a Amount) Equal(other Amount) bool { return a.Decimal().Equal(other.Decimal()) }

// ScaleFor returns the scale a currency allows (ADR §Decision 5).
func ScaleFor(currency string) int32 {
	if scale, ok := currencyScale[strings.ToUpper(strings.TrimSpace(currency))]; ok {
		return scale
	}
	return DefaultScale
}

// Round applies the single rounding policy: half-up, symmetric about zero
// (0.005 → 0.01, -0.005 → -0.01) at the currency's scale (ADR §Decision 3).
func (a Amount) Round(currency string) Amount {
	return New(a.Decimal().Round(ScaleFor(currency)))
}

// RoundTo rounds at an explicit scale with the same half-up policy — used at
// the persistence boundary when a column scale differs from the currency
// scale (rates and index factors in DECIMAL(18,4)/(18,8) columns).
func (a Amount) RoundTo(scale int32) Amount {
	return New(a.Decimal().Round(scale))
}

// ValidatePrecision rejects an amount that carries more precision than its
// currency allows instead of rounding it away (ADR §Decision 5).
func (a Amount) ValidatePrecision(currency string) error {
	scale := ScaleFor(currency)
	rounded := a.Decimal().Round(scale)
	if !a.Decimal().Equal(rounded) {
		return fmt.Errorf("%w: currency %s scale %d amount %s", ErrPrecisionExceeded, currency, scale, a.Decimal().String())
	}
	return nil
}

// Value implements driver.Valuer — DECIMAL columns take the exact decimal.
func (a Amount) Value() (driver.Value, error) {
	if !a.set {
		return nil, fmt.Errorf("%w: cannot persist an unset amount", ErrInvalidAmount)
	}
	return a.value.String(), nil
}

// Scan implements sql.Scanner — reads DECIMAL(18,2)/(18,4)/(18,8) columns
// digit-for-digit.
func (a *Amount) Scan(src any) error {
	var value string
	switch source := src.(type) {
	case nil:
		*a = Amount{}
		return nil
	case string:
		value = source
	case []byte:
		value = string(source)
	case decimal.Decimal:
		*a = New(source)
		return nil
	default:
		// pgx hands NUMERIC columns to the scanner as pgtype.Numeric, which
		// exposes its text form through driver.Valuer. Accept anything that
		// can render itself as a decimal string so the Scan plan stays
		// transport-agnostic.
		if valuers, ok := src.(interface{ Value() (driver.Value, error) }); ok {
			rendered, err := valuers.Value()
			if err != nil {
				return fmt.Errorf("money: scan value: %w", err)
			}
			text, ok := rendered.(string)
			if !ok {
				return fmt.Errorf("money: scan value rendered %T, want string", rendered)
			}
			parsed, err := decimal.NewFromString(strings.TrimSpace(text))
			if err != nil {
				return fmt.Errorf("money: scan %q: %w", text, err)
			}
			*a = New(parsed)
			return nil
		}
		return fmt.Errorf("money: cannot scan %T", src)
	}
	parsed, err := decimal.NewFromString(strings.TrimSpace(value))
	if err != nil {
		return fmt.Errorf("money: scan %q: %w", value, err)
	}
	*a = New(parsed)
	return nil
}

// MarshalJSON emits an unquoted JSON number (ADR §Decision 6) — the frontend
// money fields are typed number and a string would silently break them.
func (a Amount) MarshalJSON() ([]byte, error) {
	if !a.set {
		return []byte("0"), nil
	}
	return []byte(a.value.String()), nil
}

// UnmarshalJSON accepts both a JSON number and a quoted decimal string.
func (a *Amount) UnmarshalJSON(raw []byte) error {
	text := strings.TrimSpace(string(raw))
	if len(text) >= 2 && text[0] == '"' && text[len(text)-1] == '"' {
		text = text[1 : len(text)-1]
	}
	parsed, err := decimal.NewFromString(text)
	if err != nil {
		return fmt.Errorf("money: unmarshal %q: %w", string(raw), err)
	}
	*a = New(parsed)
	return nil
}

// Allocate splits an amount into parts that sum exactly to the total using
// the largest-remainder method (ADR §Decision 1 verification). Each part is
// rounded to the currency scale; the rounding residual is distributed one
// unit at a time to the parts with the largest fractional remainders.
func (a Amount) Allocate(currency string, weights []int64) ([]Amount, error) {
	if len(weights) == 0 {
		return nil, errors.New("money: allocation requires at least one weight")
	}
	totalWeight := int64(0)
	for _, weight := range weights {
		if weight < 0 {
			return nil, errors.New("money: allocation weights must be non-negative")
		}
		totalWeight += weight
	}
	if totalWeight == 0 {
		return nil, errors.New("money: allocation weights sum to zero")
	}
	scale := ScaleFor(currency)
	unit := decimal.New(1, -scale)
	scaled := a.Decimal().Mul(decimal.New(1, scale))
	whole := scaled.IntPart()
	if !scaled.Equal(decimal.NewFromInt(whole)) {
		return nil, fmt.Errorf("%w: total %s has more than %d decimal places", ErrPrecisionExceeded, a.Decimal().String(), scale)
	}
	parts := make([]Amount, len(weights))
	exact := make([]decimal.Decimal, len(weights))
	total := decimal.Zero
	for i, weight := range weights {
		value := decimal.NewFromInt(whole).Mul(decimal.NewFromInt(weight)).Div(decimal.NewFromInt(totalWeight))
		exact[i] = value
		total = total.Add(value)
	}
	// The exact parts already sum to `whole`; round each down and keep the
	// remainders, then hand out the leftover units by largest remainder.
	allocated := decimal.Zero
	type remainder struct {
		index int
		value decimal.Decimal
	}
	remainders := make([]remainder, 0, len(weights))
	for i, value := range exact {
		floor := value.Floor()
		parts[i] = New(floor.Mul(unit))
		allocated = allocated.Add(floor)
		remainders = append(remainders, remainder{i, value.Sub(floor)})
	}
	leftover := decimal.NewFromInt(whole).Sub(allocated)
	for leftover.IsPositive() {
		best := 0
		for i := 1; i < len(remainders); i++ {
			if remainders[i].value.Cmp(remainders[best].value) > 0 {
				best = i
			}
		}
		remainders[best].value = decimal.Zero
		parts[remainders[best].index] = New(parts[remainders[best].index].Decimal().Add(unit))
		leftover = leftover.Sub(decimal.NewFromInt(1))
	}
	return parts, nil
}

// Sum returns the exact sum of amounts.
func Sum(amounts []Amount) Amount {
	total := decimal.Zero
	for _, amount := range amounts {
		total = total.Add(amount.Decimal())
	}
	return New(total)
}

// Float64 exposes the amount as float64 for the few remaining seams that
// still require it (e.g. the regression fixture format). It is not the
// storage or wire representation.
func (a Amount) Float64() float64 {
	value, _ := a.Decimal().Float64()
	return value
}
