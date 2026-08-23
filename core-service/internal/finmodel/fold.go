package finmodel

// S2-9 期间折叠：模型 run 的逐月行值折叠为季 / 年展示视图（折叠是呈现，
// 不是重算——源值保持逐月，折叠只在导出时发生）。两条语义固定：
//   - 流量行求和；桶内任一月份缺失 → 桶值缺失（不填 0，D-S4）；
//   - 存量行取桶内最新非空月值（期末口径）；全空 → 缺失。
// 哪些行是存量由默认模板的资产负债表行注册决定；自定义行可在模板行上
// 显式声明 fold=stock/flow（F1 D-F4），声明优先于默认。

import (
	"sort"
	"strings"

	"github.com/lease-management-system/core-service/internal/finmodel/template"
)

// FoldKind is the presentation grain of an export.
type FoldKind string

const (
	FoldMonth   FoldKind = "month"
	FoldQuarter FoldKind = "quarter"
	FoldYear    FoldKind = "year"
)

// ValidFoldKind reports whether kind is an export grain. The default is
// month (no folding).
func ValidFoldKind(kind string) bool {
	switch FoldKind(strings.TrimSpace(kind)) {
	case FoldMonth, FoldQuarter, FoldYear:
		return true
	}
	return false
}

// FoldBucket is one folded period: the months it contains and its display
// label. A bucket missing months (a partial final quarter / current year)
// states its coverage in the label — never presented as a full period.
type FoldBucket struct {
	Periods []string
	Label   string
}

// foldStockKeys registers the default template's balance-sheet rows whose
// folded value is the period-end (latest non-nil month) rather than the
// sum. Single-sourced from template.ReservedSheetKeys（F1 D-F1）；custom rows
// declare their own 存量/流量 on the row, defaulting to flow semantics —
// documented on the row.
var foldStockKeys = func() map[string]bool {
	keys := make(map[string]bool)
	for _, key := range template.ReservedSheetKeys {
		keys[key] = true
	}
	return keys
}()

// DefaultStockRow reports the fold default for a row key: reserved sheet
// keys are stocks, everything else flows.
func DefaultStockRow(rowKey string) bool { return foldStockKeys[rowKey] }

// FoldBuckets groups ascending monthly periods into month/quarter/year
// buckets. Months outside a bucket boundary (a partial quarter) stay
// together with a coverage-annotated label; the month grain is the
// identity fold (one bucket per month).
func FoldBuckets(months []string, kind FoldKind) []FoldBucket {
	sortedMonths := append([]string(nil), months...)
	sort.Strings(sortedMonths)
	if kind == FoldMonth {
		out := make([]FoldBucket, 0, len(sortedMonths))
		for _, month := range sortedMonths {
			out = append(out, FoldBucket{Periods: []string{month}, Label: month})
		}
		return out
	}
	size := 3
	if kind == FoldYear {
		size = 12
	}
	out := []FoldBucket{}
	var current FoldBucket
	flush := func() {
		if len(current.Periods) == 0 {
			return
		}
		current.Label = bucketLabel(current, size)
		out = append(out, current)
		current = FoldBucket{}
	}
	bucketStart := ""
	for _, month := range sortedMonths {
		if len(current.Periods) == 0 {
			current.Periods = []string{month}
			bucketStart = month
			continue
		}
		if bucketCompatible(bucketStart, month, size, kind) {
			current.Periods = append(current.Periods, month)
			continue
		}
		flush()
		current.Periods = []string{month}
		bucketStart = month
	}
	flush()
	return out
}

// bucketCompatible keeps consecutive months with the same quarter (or
// year) in one bucket — a bucket never mixes quarters, so a partial
// quarter keeps exactly the months it has.
func bucketCompatible(start, next string, size int, kind FoldKind) bool {
	if len(next) < 7 || len(start) < 7 {
		return false
	}
	switch kind {
	case FoldYear:
		return start[:4] == next[:4] && monthGap(start, next) <= size
	default:
		return quarterOf(start) == quarterOf(next) && monthGap(start, next) <= size
	}
}

func quarterOf(period string) string {
	if len(period) < 7 {
		return period
	}
	month := period[5:7]
	switch {
	case month <= "03":
		return period[:4] + "-Q1"
	case month <= "06":
		return period[:4] + "-Q2"
	case month <= "09":
		return period[:4] + "-Q3"
	default:
		return period[:4] + "-Q4"
	}
}

// monthGap is the ordinal distance between two YYYY-MM strings in months.
func monthGap(from, to string) int {
	if len(from) < 7 || len(to) < 7 {
		return 0
	}
	yearFrom, monthFrom, yearTo, monthTo := 0, 0, 0, 0
	for i := 0; i < 4; i++ {
		yearFrom = yearFrom*10 + int(from[i]-'0')
		yearTo = yearTo*10 + int(to[i]-'0')
	}
	for i := 5; i < 7; i++ {
		monthFrom = monthFrom*10 + int(from[i]-'0')
		monthTo = monthTo*10 + int(to[i]-'0')
	}
	return (yearTo-yearFrom)*12 + (monthTo - monthFrom)
}

func bucketLabel(bucket FoldBucket, size int) string {
	first := bucket.Periods[0]
	switch size {
	case 12:
		if len(bucket.Periods) == 12 {
			return first[:4]
		}
		return first[:4] + "(" + itoa(len(bucket.Periods)) + "/12)"
	default:
		if len(bucket.Periods) == 3 {
			return quarterOf(first)
		}
		return quarterOf(first) + "(" + itoa(len(bucket.Periods)) + "/3)"
	}
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := []byte{}
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}

// FoldMonthValues folds row→month values into row→bucketLabel values using
// the default stock registry.
func FoldMonthValues(values map[string]map[string]*float64, buckets []FoldBucket) map[string]map[string]*float64 {
	return FoldMonthValuesWithStocks(values, buckets, DefaultStockRow)
}

// FoldMonthValuesWithStocks folds with an explicit per-row stock predicate —
// F1 D-F4: custom rows carry their declared 存量/流量 from the template row
// (declared choice wins over the default).
func FoldMonthValuesWithStocks(values map[string]map[string]*float64, buckets []FoldBucket, stock func(rowKey string) bool) map[string]map[string]*float64 {
	out := map[string]map[string]*float64{}
	for rowKey, months := range values {
		folded := map[string]*float64{}
		for _, bucket := range buckets {
			folded[bucket.Label] = foldOne(rowKey, months, bucket, stock(rowKey))
		}
		out[rowKey] = folded
	}
	return out
}

func foldOne(rowKey string, months map[string]*float64, bucket FoldBucket, stock bool) *float64 {
	// 存量行：期末口径——桶内反向取最新非空月。
	if stock {
		var latest *float64
		for i := len(bucket.Periods) - 1; i >= 0; i-- {
			if value, ok := months[bucket.Periods[i]]; ok && value != nil {
				copy := *value
				latest = &copy
				break
			}
		}
		return latest
	}
	// 流量行：求和；任一月份缺失即整桶缺失。
	sum := 0.0
	if len(bucket.Periods) == 0 {
		return nil
	}
	for _, period := range bucket.Periods {
		value, ok := months[period]
		if !ok || value == nil {
			return nil
		}
		sum += *value
	}
	return &sum
}
