// Package retailperiod is the single product-wide authority for analysis
// periods (design M2, PRD P0-7/P1-16/17): rolling windows (7–28 days) and
// calendar periods (YYYY-MM month, YYYY-Qn quarter, plus the natural-language
// shorthands for last month / this quarter). Every retail endpoint and page
// resolves its window through Parse so defaults, labels and reset semantics
// are defined once.
//
// Comparison semantics: a rolling window compares against the equal-length
// window immediately before it; a calendar period compares against the
// previous calendar period (Feb is compared with January even though the
// spans differ — retail review convention; coverage rates normalize by each
// window's own length).
package retailperiod

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Kind distinguishes rolling windows from calendar periods.
type Kind string

const (
	KindRolling  Kind = "rolling"
	KindCalendar Kind = "calendar"
)

// DefaultRollingDays is the unified product default (2026-08-16 decision:
// pulse moves 7 → 14 to match store-360 and the rent-negotiation page).
const DefaultRollingDays = 14

// MinRollingDays / MaxRollingDays bound the rolling range; the pulse handler
// accepts custom windows (from single-day 1 to full-year 365).
const (
	MinRollingDays = 1
	MaxRollingDays = 365
)

// Period is the parsed, normalized request: either a rolling day count or a
// calendar month/quarter.
type Period struct {
	Kind    Kind   `json:"kind"`
	Days    int    `json:"days,omitempty"`     // rolling only
	Year    int    `json:"year,omitempty"`     // calendar only
	Month   int    `json:"month,omitempty"`    // calendar month 1..12
	Quarter int    `json:"quarter,omitempty"`  // calendar quarter 1..4
}

// Window is the resolved date ranges plus a display label.
type Window struct {
	Period       Period
	From         time.Time
	To           time.Time
	CompareFrom  time.Time
	CompareTo    time.Time
	Label        string
}

var (
	calendarMonthPattern   = regexp.MustCompile(`^(\d{4})-(0[1-9]|1[0-2])$`)
	calendarQuarterPattern = regexp.MustCompile(`^(\d{4})-Q([1-4])$`)
	rollingPattern         = regexp.MustCompile(`^(\d{1,3})$`)
)

// Parse resolves a period spec against the as-of anchor. Accepted forms:
//   - ""                      → default rolling window (14 days) ending at asOf
//   - "7".."28"               → rolling window of N days ending at asOf
//   - "2026-07"               → calendar month
//   - "2026-Q3"               → calendar quarter
//   - "last-month"            → the calendar month before asOf's month
//   - "this-quarter"          → the quarter containing asOf, truncated to asOf
//
// Everything else is an error — no silent fallbacks (missing data is never
// fabricated, and neither are missing periods).
func Parse(spec string, asOf time.Time) (Window, error) {
	trimmed := strings.TrimSpace(spec)
	if trimmed == "" {
		return rolling(DefaultRollingDays, asOf)
	}
	if match := rollingPattern.FindStringSubmatch(trimmed); match != nil {
		days, _ := strconv.Atoi(match[1])
		return rolling(days, asOf)
	}
	if match := calendarMonthPattern.FindStringSubmatch(trimmed); match != nil {
		year, _ := strconv.Atoi(match[1])
		month, _ := strconv.Atoi(match[2])
		return calendarMonth(year, time.Month(month)), nil
	}
	if match := calendarQuarterPattern.FindStringSubmatch(trimmed); match != nil {
		year, _ := strconv.Atoi(match[1])
		quarter, _ := strconv.Atoi(match[2])
		return calendarQuarter(year, quarter), nil
	}
	switch strings.ToLower(trimmed) {
	case "last-month":
		firstOfThisMonth := time.Date(asOf.Year(), asOf.Month(), 1, 0, 0, 0, 0, asOf.Location())
		return calendarMonth(firstOfThisMonth.AddDate(0, -1, 0).Year(), firstOfThisMonth.AddDate(0, -1, 0).Month()), nil
	case "this-quarter":
		quarter := int(asOf.Month()-1)/3 + 1
		from := time.Date(asOf.Year(), time.Month((quarter-1)*3+1), 1, 0, 0, 0, 0, asOf.Location())
		to := minTime(endOfQuarter(asOf.Year(), quarter, asOf.Location()), truncateToDate(asOf))
		// Quarter-to-date compares against the same number of days directly
		// before the quarter start — an equal-length comparison, matching
		// rolling semantics while the quarter is still in progress.
		length := int(to.Sub(from).Hours()/24) + 1
		compareTo := from.AddDate(0, 0, -1)
		compareFrom := compareTo.AddDate(0, 0, -(length - 1))
		return Window{Period: Period{Kind: KindCalendar, Year: asOf.Year(), Quarter: quarter}, From: from, To: to, CompareFrom: compareFrom, CompareTo: compareTo, Label: fmt.Sprintf("%d-Q%d", asOf.Year(), quarter)}, nil
	}
	return Window{}, fmt.Errorf("invalid period %q: use 7-28 (rolling), YYYY-MM, YYYY-Qn, last-month or this-quarter", trimmed)
}

// ParseRollingDays validates a bare window_days value against the shared
// range contract; handlers keep accepting the legacy parameter.
func ParseRollingDays(days int) (Period, error) {
	if days < MinRollingDays || days > MaxRollingDays {
		return Period{}, fmt.Errorf("window_days must be between %d and %d", MinRollingDays, MaxRollingDays)
	}
	return Period{Kind: KindRolling, Days: days}, nil
}

func rolling(days int, asOf time.Time) (Window, error) {
	if days < MinRollingDays || days > MaxRollingDays {
		return Window{}, fmt.Errorf("rolling window must be between %d and %d days, got %d", MinRollingDays, MaxRollingDays, days)
	}
	to := truncateToDate(asOf)
	from := to.AddDate(0, 0, -(days - 1))
	compareTo := from.AddDate(0, 0, -1)
	compareFrom := compareTo.AddDate(0, 0, -(days - 1))
	return Window{
		Period:      Period{Kind: KindRolling, Days: days},
		From:        from, To: to, CompareFrom: compareFrom, CompareTo: compareTo,
		Label:       fmt.Sprintf("近 %d 天", days),
	}, nil
}

func calendarMonth(year int, month time.Month) Window {
	from := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 1, -1)
	previousFirst := from.AddDate(0, -1, 0)
	return Window{
		Period:      Period{Kind: KindCalendar, Year: year, Month: int(month)},
		From:        from, To: to,
		CompareFrom: previousFirst, CompareTo: previousFirst.AddDate(0, 1, -1),
		Label:       fmt.Sprintf("%04d-%02d", year, int(month)),
	}
}

func calendarQuarter(year, quarter int) Window {
	from := time.Date(year, time.Month((quarter-1)*3+1), 1, 0, 0, 0, 0, time.UTC)
	to := endOfQuarter(year, quarter, time.UTC)
	previous := previousQuarterWindow(year, quarter, time.UTC)
	return Window{
		Period:      Period{Kind: KindCalendar, Year: year, Quarter: quarter},
		From:        from, To: to, CompareFrom: previous.From, CompareTo: previous.To,
		Label:       fmt.Sprintf("%d-Q%d", year, quarter),
	}
}

// Default returns the product-wide default period for a kind.
func Default(kind Kind) Period {
	if kind == KindCalendar {
		return Period{Kind: KindCalendar} // resolved against asOf by Parse("this-quarter", …) callers
	}
	return Period{Kind: KindRolling, Days: DefaultRollingDays}
}

// Normalize validates the enum fields of an already-built Period.
func Normalize(p Period) (Period, error) {
	switch p.Kind {
	case KindRolling:
		if p.Days < MinRollingDays || p.Days > MaxRollingDays {
			return p, fmt.Errorf("rolling window must be between %d and %d days", MinRollingDays, MaxRollingDays)
		}
		if p.Year != 0 || p.Month != 0 || p.Quarter != 0 {
			return p, fmt.Errorf("rolling period cannot carry calendar fields")
		}
	case KindCalendar:
		if p.Month != 0 && p.Quarter != 0 {
			return p, fmt.Errorf("calendar period cannot be both month and quarter")
		}
		if p.Month < 0 || p.Month > 12 || p.Quarter < 0 || p.Quarter > 4 {
			return p, fmt.Errorf("calendar month/quarter out of range")
		}
		if p.Year < 2000 || p.Year > 2100 {
			return p, fmt.Errorf("calendar year out of range")
		}
	default:
		return p, fmt.Errorf("unknown period kind %q", p.Kind)
	}
	return p, nil
}

func truncateToDate(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, value.Location())
}

func endOfQuarter(year, quarter int, location *time.Location) time.Time {
	firstOfNext := time.Date(year, time.Month(quarter*3+1), 1, 0, 0, 0, 0, location)
	return firstOfNext.AddDate(0, 0, -1)
}

// previousQuarterWindow computes the date range of the quarter before the
// given one without recursing (calendarQuarter consumes it for its
// comparison range).
func previousQuarterWindow(year, quarter int, location *time.Location) Window {
	if quarter == 1 {
		year, quarter = year-1, 4
	} else {
		quarter--
	}
	from := time.Date(year, time.Month((quarter-1)*3+1), 1, 0, 0, 0, 0, location)
	to := endOfQuarter(year, quarter, location)
	return Window{Period: Period{Kind: KindCalendar, Year: year, Quarter: quarter}, From: from, To: to, Label: fmt.Sprintf("%d-Q%d", year, quarter)}
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}
