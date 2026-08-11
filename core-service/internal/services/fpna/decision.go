// Package fpna owns deterministic cross-domain calculations used by the UI,
// exports and Agent tools.  It deliberately accepts plain value objects so
// none of the callers can introduce a second, model-generated formula.
package fpna

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lease-management-system/core-service/internal/repository"
)

type PlanDifference struct {
	Key       string  `json:"key"`
	Dimension string  `json:"dimension"`
	Period    string  `json:"period"`
	Currency  string  `json:"currency"`
	Left      float64 `json:"left"`
	Right     float64 `json:"right"`
	Variance  float64 `json:"variance"`
}

type PlanComparison struct {
	Period      string           `json:"period"`
	LeftBasis   string           `json:"left_basis"`
	RightBasis  string           `json:"right_basis"`
	LeftTotal   float64          `json:"left_total"`
	RightTotal  float64          `json:"right_total"`
	Variance    float64          `json:"variance"`
	Residual    float64          `json:"residual"`
	TiesOut     bool             `json:"ties_out"`
	Currency    string           `json:"currency"`
	Coverage    Coverage         `json:"coverage"`
	Differences []PlanDifference `json:"differences"`
	Source      string           `json:"source"`
	DataVersion string           `json:"data_version"`
	GeneratedAt time.Time        `json:"generated_at"`
}

type Coverage struct {
	Expected int     `json:"expected"`
	Observed int     `json:"observed"`
	Percent  float64 `json:"percent"`
	Complete bool    `json:"complete"`
}

// ComparePlanLines compares a frozen plan basis with an actual/selected basis
// by the same dimensional key.  Missing values are represented by a zero in
// the arithmetic but coverage exposes the omission; callers must not label a
// zero as a confirmed zero when the row was absent.
func ComparePlanLines(period, leftBasis, rightBasis, currency, dataVersion string, left, right []*repository.FPnAPlanLine, tolerance float64) PlanComparison {
	result := PlanComparison{Period: period, LeftBasis: leftBasis, RightBasis: rightBasis, Currency: currency, Source: "fpna_plan_lines", DataVersion: dataVersion, GeneratedAt: time.Now().UTC(), Differences: make([]PlanDifference, 0)}
	leftByKey := make(map[string]*repository.FPnAPlanLine, len(left))
	rightByKey := make(map[string]*repository.FPnAPlanLine, len(right))
	for _, line := range left {
		leftByKey[planLineKey(line)] = line
	}
	for _, line := range right {
		rightByKey[planLineKey(line)] = line
	}
	keys := make(map[string]struct{}, len(leftByKey)+len(rightByKey))
	for key := range leftByKey {
		keys[key] = struct{}{}
	}
	for key := range rightByKey {
		keys[key] = struct{}{}
	}
	result.Coverage.Expected = len(keys)
	for key := range keys {
		l, lok := leftByKey[key]
		r, rok := rightByKey[key]
		if lok && rok {
			result.Coverage.Observed++
		}
		lv, rv := lineAmount(l), lineAmount(r)
		result.LeftTotal += lv
		result.RightTotal += rv
		identity := l
		if identity == nil {
			identity = r
		}
		result.Differences = append(result.Differences, PlanDifference{Key: key, Dimension: planLineDimension(identity), Period: period, Currency: lineCurrency(identity), Left: round2(lv), Right: round2(rv), Variance: round2(rv - lv)})
	}
	result.LeftTotal, result.RightTotal = round2(result.LeftTotal), round2(result.RightTotal)
	result.Variance = round2(result.RightTotal - result.LeftTotal)
	var detail float64
	for _, diff := range result.Differences {
		detail += diff.Variance
	}
	result.Residual = round2(result.Variance - detail)
	result.TiesOut = math.Abs(result.Residual) <= tolerance
	if result.Coverage.Expected > 0 {
		result.Coverage.Percent = round2(float64(result.Coverage.Observed) / float64(result.Coverage.Expected) * 100)
	}
	result.Coverage.Complete = result.Coverage.Expected > 0 && result.Coverage.Expected == result.Coverage.Observed
	sort.Slice(result.Differences, func(i, j int) bool {
		return math.Abs(result.Differences[i].Variance) > math.Abs(result.Differences[j].Variance)
	})
	return result
}

// HybridForecast returns a new, immutable working view.  Actual rows replace
// forecast rows through cutoff; future forecast rows remain untouched.
func HybridForecast(forecast, actual []*repository.FPnAPlanLine, actualCutoff string) ([]*repository.FPnAPlanLine, error) {
	if strings.TrimSpace(actualCutoff) == "" {
		return nil, fmt.Errorf("actual cutoff period is required")
	}
	if _, err := time.Parse("2006-01", actualCutoff); err != nil {
		return nil, fmt.Errorf("actual cutoff period must be YYYY-MM")
	}
	actualByKey := make(map[string]*repository.FPnAPlanLine, len(actual))
	for _, row := range actual {
		if row.Period <= actualCutoff {
			actualByKey[planLineKey(row)] = row
		}
	}
	result := make([]*repository.FPnAPlanLine, 0, len(forecast)+len(actualByKey))
	seen := make(map[string]struct{})
	for _, row := range forecast {
		key := planLineKey(row)
		seen[key] = struct{}{}
		if replacement, ok := actualByKey[key]; ok {
			clone := *replacement
			clone.ActualFlag = true
			clone.ForecastFlag = false
			result = append(result, &clone)
		} else {
			clone := *row
			result = append(result, &clone)
		}
	}
	for key, row := range actualByKey {
		if _, ok := seen[key]; ok {
			continue
		}
		clone := *row
		clone.ActualFlag = true
		clone.ForecastFlag = false
		result = append(result, &clone)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Period < result[j].Period })
	return result, nil
}

type AccuracyRow struct {
	Key       string   `json:"key"`
	Dimension string   `json:"dimension"`
	Period    string   `json:"period"`
	Forecast  float64  `json:"forecast"`
	Actual    float64  `json:"actual"`
	Variance  float64  `json:"variance"`
	Accuracy  *float64 `json:"accuracy,omitempty"`
	Bias      float64  `json:"bias"`
	Driver    string   `json:"driver,omitempty"`
}

type AccuracyResult struct {
	Rows       []AccuracyRow `json:"rows"`
	MeanAbsPct *float64      `json:"mean_abs_pct,omitempty"`
	Bias       float64       `json:"bias"`
	Coverage   Coverage      `json:"coverage"`
}

func ForecastAccuracy(forecast, actual []*repository.FPnAPlanLine) AccuracyResult {
	result := AccuracyResult{Rows: make([]AccuracyRow, 0)}
	fByKey, aByKey := make(map[string]*repository.FPnAPlanLine), make(map[string]*repository.FPnAPlanLine)
	for _, row := range forecast {
		fByKey[planLineKey(row)] = row
	}
	for _, row := range actual {
		aByKey[planLineKey(row)] = row
	}
	keys := make(map[string]struct{}, len(fByKey)+len(aByKey))
	for key := range fByKey {
		keys[key] = struct{}{}
	}
	for key := range aByKey {
		keys[key] = struct{}{}
	}
	result.Coverage.Expected = len(keys)
	var absPctSum, biasSum float64
	absPctCount := 0
	for key := range keys {
		f, fok := fByKey[key]
		a, aok := aByKey[key]
		if fok && aok {
			result.Coverage.Observed++
		}
		fv, av := lineAmount(f), lineAmount(a)
		variance := round2(av - fv)
		var accuracy *float64
		if math.Abs(av) > 1e-9 {
			value := round2(100 - math.Abs(variance)/math.Abs(av)*100)
			accuracy = &value
		}
		result.Rows = append(result.Rows, AccuracyRow{Key: key, Dimension: planLineDimension(firstLine(f, a)), Period: firstPeriod(f, a), Forecast: round2(fv), Actual: round2(av), Variance: variance, Accuracy: accuracy, Bias: variance, Driver: driverFromLine(firstLine(f, a))})
		if math.Abs(av) > 1e-9 {
			absPctSum += math.Abs(variance) / math.Abs(av) * 100
			absPctCount++
		}
		biasSum += variance
	}
	if absPctCount > 0 {
		value := round2(absPctSum / float64(absPctCount))
		result.MeanAbsPct = &value
	}
	result.Bias = round2(biasSum)
	result.Coverage.Percent = 0
	if result.Coverage.Expected > 0 {
		result.Coverage.Percent = round2(float64(result.Coverage.Observed) / float64(result.Coverage.Expected) * 100)
	}
	result.Coverage.Complete = result.Coverage.Expected > 0 && result.Coverage.Expected == result.Coverage.Observed
	sort.Slice(result.Rows, func(i, j int) bool { return math.Abs(result.Rows[i].Variance) > math.Abs(result.Rows[j].Variance) })
	return result
}

type ActionRank struct {
	ID      string   `json:"id"`
	Score   float64  `json:"score"`
	Reasons []string `json:"reasons"`
}

func RankAction(action repository.FPnAActionItem, now time.Time) ActionRank {
	score := 0.0
	reasons := make([]string, 0, 8)
	switch action.Severity {
	case "critical":
		score += 100
		reasons = append(reasons, "critical severity")
	case "high":
		score += 70
		reasons = append(reasons, "high severity")
	case "medium":
		score += 40
	default:
		score += 10
	}
	if action.ImpactAmount != nil {
		score += math.Min(math.Abs(*action.ImpactAmount)/1000, 50)
		reasons = append(reasons, "material impact")
	}
	if action.DueDate != nil {
		days := action.DueDate.Sub(now).Hours() / 24
		switch {
		case days < 0:
			score += 50
			reasons = append(reasons, "overdue")
		case days <= 7:
			score += 30
			reasons = append(reasons, "due within seven days")
		case days <= 30:
			score += 10
			reasons = append(reasons, "due within thirty days")
		}
	}
	if action.VerificationStatus == "pending" {
		score += 15
		reasons = append(reasons, "realization verification pending")
	}
	// These review-priority dimensions are deliberately carried in evidence so
	// the action schema stays compatible with existing clients. They are not
	// inferred from prose: only explicit numeric/string values are considered.
	var evidence map[string]any
	if len(action.Evidence) > 0 && strings.HasPrefix(strings.TrimSpace(string(action.Evidence)), "{") {
		_ = json.Unmarshal(action.Evidence, &evidence)
	}
	if value := evidenceNumber(evidence, "control_risk_score"); value > 0 {
		score += math.Min(value, 40)
		reasons = append(reasons, "control risk")
	}
	if value := evidenceNumber(evidence, "recurrence_count"); value > 0 {
		score += math.Min(value*5, 25)
		reasons = append(reasons, "recurring exception")
	}
	if value := evidenceNumber(evidence, "fixability_score"); value > 0 {
		score += math.Min(value/4, 25)
		reasons = append(reasons, "fixable intervention")
	}
	return ActionRank{ID: action.ID, Score: round2(score), Reasons: reasons}
}

func evidenceNumber(evidence map[string]any, key string) float64 {
	if evidence == nil {
		return 0
	}
	switch value := evidence[key].(type) {
	case float64:
		return value
	case int:
		return float64(value)
	case string:
		trimmed := strings.ToLower(strings.TrimSpace(value))
		levels := map[string]float64{"critical": 100, "high": 80, "medium": 50, "low": 20}
		if level, ok := levels[trimmed]; ok {
			return level
		}
		parsed, err := strconv.ParseFloat(trimmed, 64)
		if err == nil {
			return parsed
		}
	case map[string]any:
		if score, ok := value["score"]; ok {
			return evidenceNumber(map[string]any{"score": score}, "score")
		}
	}
	return 0
}

func VerifyRealization(expected, baseline, actual *float64, tolerance float64) (status string, realized *float64) {
	if actual == nil {
		return "pending", nil
	}
	if baseline == nil {
		baseline = expected
	}
	if baseline == nil {
		return "pending", nil
	}
	value := round2(*baseline - *actual)
	if expected != nil && math.Abs(value-*expected) > tolerance {
		return "failed", &value
	}
	return "verified", &value
}

func planLineKey(row *repository.FPnAPlanLine) string {
	if row == nil {
		return ""
	}
	return fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s", row.Period, row.Grain, safe(row.BusinessSegment), safe(row.Brand), safe(row.Region), safePtr(row.StoreID), safe(row.PlantCode), safe(row.ProductionLineCode), safePtr(row.EquipmentID), safe(row.AssetType), safe(row.Currency))
}
func planLineDimension(row *repository.FPnAPlanLine) string {
	if row == nil {
		return ""
	}
	for _, value := range []string{safePtr(row.StoreID), safePtr(row.EquipmentID), row.ProductionLineCode, row.PlantCode, row.Region, row.Brand, row.BusinessSegment} {
		if value != "" {
			return value
		}
	}
	return row.Grain
}
func lineAmount(row *repository.FPnAPlanLine) float64 {
	if row == nil {
		return 0
	}
	if row.FourWallEBITDA != nil {
		return *row.FourWallEBITDA
	}
	if row.CashFlow != nil {
		return *row.CashFlow
	}
	if row.Revenue != nil {
		return *row.Revenue
	}
	return 0
}
func lineCurrency(row *repository.FPnAPlanLine) string {
	if row == nil {
		return ""
	}
	return row.Currency
}
func firstLine(a, b *repository.FPnAPlanLine) *repository.FPnAPlanLine {
	if a != nil {
		return a
	}
	return b
}
func firstPeriod(a, b *repository.FPnAPlanLine) string {
	if a != nil {
		return a.Period
	}
	if b != nil {
		return b.Period
	}
	return ""
}
func driverFromLine(row *repository.FPnAPlanLine) string {
	if row == nil {
		return ""
	}
	if row.OperationalKPIs != nil {
		return "operational_kpi"
	}
	return "total"
}
func safe(value string) string { return value }
func safePtr(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
func round2(value float64) float64 { return math.Round(value*100) / 100 }
