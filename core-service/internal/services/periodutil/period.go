package periodutil

import (
	"fmt"
	"time"
)

// GenerateMonthlyPeriods returns a slice of "YYYY-MM" strings from 'from' to 'to' inclusive.
func GenerateMonthlyPeriods(from, to string) ([]string, error) {
	if from == "" || to == "" {
		return nil, fmt.Errorf("from and to period strings cannot be empty")
	}
	if from > to {
		return nil, fmt.Errorf("from period %q is after to period %q", from, to)
	}

	var y1, m1, y2, m2 int
	if n, err := fmt.Sscanf(from, "%d-%d", &y1, &m1); err != nil || n != 2 {
		return nil, fmt.Errorf("invalid from period format %q, expected YYYY-MM", from)
	}
	if n, err := fmt.Sscanf(to, "%d-%d", &y2, &m2); err != nil || n != 2 {
		return nil, fmt.Errorf("invalid to period format %q, expected YYYY-MM", to)
	}

	var periods []string
	currY, currM := y1, m1
	for {
		if currY > y2 || (currY == y2 && currM > m2) {
			break
		}
		periods = append(periods, fmt.Sprintf("%04d-%02d", currY, currM))
		currM++
		if currM > 12 {
			currY++
			currM = 1
		}
	}
	return periods, nil
}

// DatesOverlap returns true if two date ranges [s1, e1] and [s2, e2] overlap.
func DatesOverlap(s1, e1, s2, e2 string) bool {
	if s1 == "" || e1 == "" || s2 == "" || e2 == "" {
		return false
	}
	// Overlaps if s1 <= e2 AND s2 <= e1
	return s1 <= e2 && s2 <= e1
}

// DaysBetween calculates inclusive days between two "YYYY-MM-DD" date strings.
func DaysBetween(fromDate, toDate string) (int, error) {
	t1, err := time.Parse("2006-01-02", fromDate)
	if err != nil {
		return 0, fmt.Errorf("invalid fromDate %q: %w", fromDate, err)
	}
	t2, err := time.Parse("2006-01-02", toDate)
	if err != nil {
		return 0, fmt.Errorf("invalid toDate %q: %w", toDate, err)
	}
	if t2.Before(t1) {
		return 0, fmt.Errorf("toDate %q is before fromDate %q", toDate, fromDate)
	}
	return int(t2.Sub(t1).Hours()/24) + 1, nil
}
