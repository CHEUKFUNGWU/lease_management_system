package money

import (
	"database/sql"
	"encoding/json"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/shopspring/decimal"
)

// M2: the single rounding policy — half-up, symmetric about zero.
func TestRoundHalfUpSymmetric(t *testing.T) {
	cases := []struct {
		input  string
		expect string
	}{
		{"0.005", "0.01"},
		{"-0.005", "-0.01"},
		{"0.004", "0.00"},
		{"-0.004", "-0.00"},
		{"1.005", "1.01"},
		{"-1.005", "-1.01"},
		{"2.675", "2.68"},
		{"-2.675", "-2.68"},
	}
	for _, testCase := range cases {
		amount, err := FromDecimalString(testCase.input)
		if err != nil {
			t.Fatal(err)
		}
		expected, err := decimal.NewFromString(testCase.expect)
		if err != nil {
			t.Fatal(err)
		}
		got := amount.Round("CNY").Decimal()
		if !got.Equal(expected) {
			t.Fatalf("Round(%s) = %s, want %s", testCase.input, got.String(), testCase.expect)
		}
	}
}

// M2: the reversal of an amount is the exact mirror (红冲 symmetry).
func TestRoundNegativeMirror(t *testing.T) {
	positive, _ := FromDecimalString("0.005")
	negative, _ := FromDecimalString("-0.005")
	if got := positive.Round("CNY").Neg().Decimal().String(); got != negative.Round("CNY").Decimal().String() {
		t.Fatalf("mirror mismatch: %s vs %s", got, negative.Round("CNY").Decimal().String())
	}
}

// M5: an amount carrying more precision than its currency allows is an
// error, never silently rounded.
func TestValidatePrecisionRejects(t *testing.T) {
	amount, _ := FromDecimalString("1.005")
	if err := amount.ValidatePrecision("CNY"); err == nil {
		t.Fatal("CNY amount with 3 decimals must be rejected")
	}
	jpy, _ := FromDecimalString("100.5")
	if err := jpy.ValidatePrecision("JPY"); err == nil {
		t.Fatal("JPY amount with decimals must be rejected")
	}
	kuwaiti, _ := FromDecimalString("1.0005")
	if err := kuwaiti.ValidatePrecision("KWD"); err == nil {
		t.Fatal("KWD amount with 4 decimals must be rejected")
	}
	exact, _ := FromDecimalString("1.00")
	if err := exact.ValidatePrecision("CNY"); err != nil {
		t.Fatalf("exact CNY amount rejected: %v", err)
	}
	whole, _ := FromDecimalString("100")
	if err := whole.ValidatePrecision("JPY"); err != nil {
		t.Fatalf("whole JPY amount rejected: %v", err)
	}
}

// M3: MarshalJSON emits an unquoted number, byte-identical with the JSON a
// float64 field produced for the same value.
func TestMarshalJSONUnquotedNumber(t *testing.T) {
	amount, _ := FromDecimalString("3255676.79")
	raw, err := json.Marshal(amount)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "3255676.79" {
		t.Fatalf("json = %s, want 3255676.79 (no quotes)", raw)
	}
	// Byte-for-byte with the existing float64 wire format.
	asFloat, _ := json.Marshal(3255676.79)
	if string(raw) != string(asFloat) {
		t.Fatalf("json %s differs from float64 wire %s", raw, asFloat)
	}
	whole, _ := FromDecimalString("100")
	rawWhole, _ := json.Marshal(whole)
	if string(rawWhole) != "100" {
		t.Fatalf("whole json = %s, want 100", rawWhole)
	}
}

// M3: UnmarshalJSON accepts both numbers and quoted strings.
func TestUnmarshalJSON(t *testing.T) {
	var amount Amount
	if err := json.Unmarshal([]byte(`"123.45"`), &amount); err != nil || amount.Decimal().String() != "123.45" {
		t.Fatalf("quoted unmarshal = %v err=%v", amount.Decimal().String(), err)
	}
	var numeric Amount
	if err := json.Unmarshal([]byte(`987.65`), &numeric); err != nil || numeric.Decimal().String() != "987.65" {
		t.Fatalf("numeric unmarshal = %v err=%v", numeric.Decimal().String(), err)
	}
}

// M4: allocation sums exactly to the total, including amounts that do not
// divide evenly (largest-remainder method).
func TestAllocateConservesTotal(t *testing.T) {
	total, _ := FromDecimalString("100.00")
	parts, err := total.Allocate("CNY", []int64{1, 1, 1})
	if err != nil {
		t.Fatal(err)
	}
	sum := Sum(parts)
	if !sum.Equal(total) {
		t.Fatalf("allocated sum %s != total %s (parts %v)", sum.Decimal().String(), total.Decimal().String(), parts)
	}
	// 100 / 3 — largest remainder distributes the extra cent.
	if parts[0].Decimal().String() != "33.34" || parts[1].Decimal().String() != "33.33" || parts[2].Decimal().String() != "33.33" {
		t.Fatalf("uneven allocation = %v", parts)
	}

	odd, _ := FromDecimalString("0.01")
	partsOdd, err := odd.Allocate("CNY", []int64{1, 1})
	if err != nil {
		t.Fatal(err)
	}
	if got := Sum(partsOdd); !got.Equal(odd) {
		t.Fatalf("odd allocation sum %s != %s", got.Decimal().String(), odd.Decimal().String())
	}
}

// M1: DECIMAL columns round-trip digit-for-digit against a real PostgreSQL
// instance at all three column scales.
func TestDecimalColumnRoundTripPostgres(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// Each value must fit the column exactly: DECIMAL(18,N) holds up to
	// 18-N integer digits and N fractional digits without rounding.
	byColumn := map[string][]string{
		"DECIMAL(18,2)": {"3255676.79", "0.01", "-0.01", "1234567890123456.78", "0.00"},
		"DECIMAL(18,4)": {"0.0001", "-0.0001", "12345.6789", "12345678901234.5678"},
		"DECIMAL(18,8)": {"0.00000001", "12345678.12345678", "-0.00000001", "1234567890.12345678"},
	}
	for columnType, values := range byColumn {
		for _, value := range values {
			var column string
			query := "SELECT $1::" + columnType
			if err := db.QueryRow(query, value).Scan(&column); err != nil {
				t.Fatalf("%s round-trip %s: %v", columnType, value, err)
			}
			parsed, err := decimal.NewFromString(column)
			if err != nil {
				t.Fatalf("parse %q: %v", column, err)
			}
			expected, err := decimal.NewFromString(value)
			if err != nil {
				t.Fatal(err)
			}
			if !parsed.Equal(expected) {
				t.Fatalf("%s round-trip %s -> %s: digit mismatch", columnType, value, column)
			}
		}
	}
}

// M1: the money type persists and reads back through driver.Valuer /
// sql.Scanner via a real DECIMAL column.
func TestAmountValuerScannerPostgres(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	amount, _ := FromDecimalString("3255676.79")
	var readBack Amount
	if err := db.QueryRow("SELECT $1::DECIMAL(18,2)", amount).Scan(&readBack); err != nil {
		t.Fatal(err)
	}
	if !readBack.Equal(amount) {
		t.Fatalf("read back %s != %s", readBack.Decimal().String(), amount.Decimal().String())
	}
	negative, _ := FromDecimalString("-0.01")
	var negativeBack Amount
	if err := db.QueryRow("SELECT $1::DECIMAL(18,2)", negative).Scan(&negativeBack); err != nil {
		t.Fatal(err)
	}
	if !negativeBack.Equal(negative) {
		t.Fatalf("negative read back %s != %s", negativeBack.Decimal().String(), negative.Decimal().String())
	}
}
