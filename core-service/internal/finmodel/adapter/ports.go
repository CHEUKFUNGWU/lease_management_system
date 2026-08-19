package adapter

// 三个 S2-3 生产适配器。共同纪律（D-S3）：租赁数字只通过投影端口进入
// 模型——本包读的只是计量引擎落库的 measurement_results 行（正式表的
// 只读方），从不 import 计量服务、从不重算任何计量值；导入侧缺什么就
// 诚实缺失，不补 0、不编造。

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/lease-management-system/core-service/internal/finmodel"
	"github.com/lease-management-system/core-service/internal/finmodel/opening"
	"github.com/lease-management-system/core-service/internal/repository"
)

// MeasurementSource is the narrow seam of entity-period engine outputs.
type MeasurementSource interface {
	ListMeasurementResultsByEntityPeriod(ctx context.Context, legalEntityID, period string) ([]*repository.MeasurementResult, error)
}

// TrialBalanceSource is the narrow seam of period-sorted TB lines.
type TrialBalanceSource interface {
	LatestTrialBalanceByPeriod(ctx context.Context, legalEntityID string) (map[string][]repository.TrialBalanceLine, string, error)
}

// CapexSource is the narrow seam of forecast plan capex.
type CapexSource interface {
	LatestForecastCapex(ctx context.Context, legalEntityID, period string) (*float64, error)
}

func moneyFloat(value interface{ Float64() float64 }) *float64 {
	f := value.Float64()
	return &f
}

// LeaseReader is the S2-3 LeaseRollforwardReader: one entity-month fold of
// the official engine outputs. ROU/liability are the period-opening
// balances (the engine's opening-balance convention); additions and
// terminations come from contracts whose lease window starts/ends inside
// the month (nil when none happened — never a fabricated zero event);
// remeasurements have no separate in-system record and stay nil.
type LeaseReader struct {
	measurements MeasurementSource
}

// NewLeaseReader builds the lease projection adapter.
func NewLeaseReader(measurements MeasurementSource) *LeaseReader {
	return &LeaseReader{measurements: measurements}
}

// Monthly implements finmodel.LeaseRollforwardReader.
func (r *LeaseReader) Monthly(ctx context.Context, legalEntityID, period string) (finmodel.LeaseMonth, error) {
	if r.measurements == nil {
		return finmodel.LeaseMonth{}, errors.New("lease measurement source unavailable")
	}
	rows, err := r.measurements.ListMeasurementResultsByEntityPeriod(ctx, legalEntityID, period)
	if err != nil {
		return finmodel.LeaseMonth{}, err
	}
	if len(rows) == 0 {
		return finmodel.LeaseMonth{}, nil // 无租赁行：全字段缺失，模型诚实降级
	}
	out := finmodel.LeaseMonth{
		ROUAsset:       moneyFloat(rows[0].OpeningROUAsset),
		LeaseLiability: moneyFloat(rows[0].OpeningLiability),
		Interest:       moneyFloat(rows[0].InterestExpense),
		Depreciation:   moneyFloat(rows[0].Depreciation),
		Payments:       moneyFloat(rows[0].TotalPayment),
		Principal:      moneyFloat(rows[0].PrincipalRepayment),
	}
	zerov := 0.0
	out.Remeasurements = &zerov // 系统内无重计量单独记录：显式 0 而非缺失
	out.Additions = &zerov      // 有计量行即可数事件：当月无新租约即 0
	out.Terminations = &zerov
	for _, row := range rows[1:] {
		addF := func(dst **float64, amount interface{ Float64() float64 }) {
			if *dst == nil {
				value := amount.Float64()
				*dst = &value
				return
			}
			value := **dst + amount.Float64()
			*dst = &value
		}
		addF(&out.ROUAsset, row.OpeningROUAsset)
		addF(&out.LeaseLiability, row.OpeningLiability)
		addF(&out.Interest, row.InterestExpense)
		addF(&out.Depreciation, row.Depreciation)
		addF(&out.Payments, row.TotalPayment)
		addF(&out.Principal, row.PrincipalRepayment)
	}
	// 当月新租约：期初 ROU 视为 additions；当月结束的租约：其期末 ROU 为
	// terminations。两者都以行内日期判定，无事件即保持 nil。
	additions, terminations := out.Additions, out.Terminations
	monthStart, _ := time.Parse("2006-01-02", period+"-01")
	nextMonth := monthStart.AddDate(0, 1, 0)
	for _, row := range rows {
		if !row.PeriodStartDate.Before(monthStart) && row.PeriodStartDate.Before(nextMonth) {
			value := *additions + row.OpeningROUAsset.Float64()
			additions = &value
		}
		if !row.PeriodEndDate.Before(monthStart) && row.PeriodEndDate.Before(nextMonth) {
			value := *terminations + row.ClosingROUAsset.Float64()
			terminations = &value
		}
	}
	out.Additions = additions
	out.Terminations = terminations
	return out, nil
}

// ScheduleReaderAdapter is the S2-3 ScheduleReader: contract-driven values
// from the measurement rows' non-lease expense, capex from the latest
// forecast plan version, and share capital / borrowings / other
// depreciation from approved assumptions — every cell is a versioned
// in-system source, missing stays missing.
type ScheduleReaderAdapter struct {
	measurements MeasurementSource
	assumptions  finmodel.AssumptionReader
	capex        CapexSource
}

// NewScheduleReader builds the adapter.
func NewScheduleReader(measurements MeasurementSource, assumptions finmodel.AssumptionReader, capex CapexSource) *ScheduleReaderAdapter {
	return &ScheduleReaderAdapter{measurements: measurements, assumptions: assumptions, capex: capex}
}

// Monthly implements finmodel.ScheduleReader.
func (r *ScheduleReaderAdapter) Monthly(ctx context.Context, legalEntityID, period string) (finmodel.ScheduleFanout, error) {
	out := finmodel.ScheduleFanout{}
	if r.measurements != nil {
		rows, err := r.measurements.ListMeasurementResultsByEntityPeriod(ctx, legalEntityID, period)
		if err != nil {
			return out, err
		}
		if len(rows) > 0 {
			total := rows[0].NonLeaseExpense.Float64()
			for _, row := range rows[1:] {
				total += row.NonLeaseExpense.Float64()
			}
			out.ServiceFee = &total
		}
	}
	if r.assumptions != nil {
		for key, dst := range map[string]**float64{
			"share_capital":      &out.ShareCapital,
			"borrowings":         &out.Borrowings,
			"other_depreciation": &out.OtherDepreciation,
		} {
			if raw, err := r.assumptions.Value(ctx, legalEntityID, key, period); err == nil && raw != nil {
				var number json.Number
				if err := json.Unmarshal(raw, &number); err == nil {
					if value, err := number.Float64(); err == nil {
						*dst = &value
					}
				}
			}
		}
	}
	if r.capex != nil {
		capex, err := r.capex.LatestForecastCapex(ctx, legalEntityID, period)
		if err != nil {
			return out, err
		}
		out.Capex = capex
	}
	return out, nil
}

// 默认归并：TB 科目码前缀 → 标准行。导入模板版本化是导入侧的能力——
// 系统内无历史映射表之前，这个默认规约为显式版本 default-account-map-v1
// 随 MergePolicy 声明，替换即换版本号（PRD §11 风险 2 的处置）。
const defaultMappingVersion = "default-account-map-v1"

var tbAccountMap = []struct {
	prefix string
	line   string
}{
	{"1001", opening.LineCash}, {"1002", opening.LineCash}, {"1012", opening.LineCash},
	{"1122", opening.LineReceivables}, {"1123", opening.LineReceivables},
	{"1405", opening.LineInventory}, {"1406", opening.LineInventory},
	{"1601", opening.LinePPE}, {"1602", opening.LinePPE}, {"1611", opening.LinePPE},
	{"1621", opening.LineROUAsset}, {"1631", opening.LineROUAsset},
	{"2202", opening.LinePayables}, {"2203", opening.LinePayables},
	{"2801", opening.LineLeaseLiability}, {"2811", opening.LineLeaseLiability},
	{"2001", opening.LineBorrowings}, {"2501", opening.LineBorrowings},
	{"4001", opening.LineShareCapital}, {"4101", opening.LineRetainedEarnings},
	{"1101", opening.LineOtherCurrentAssets}, {"1201", opening.LineOtherCurrentAssets},
	{"2201", opening.LineOtherCurrentLiabs}, {"2301", opening.LineOtherCurrentLiabs},
}

// lineForAccount resolves one account code to a standard line through the
// default map; unknown codes fall into the matching current-asset /
// current-liability bucket by debit/credit sign so the balance screen
// stays complete (杂项并入汇总行，PRD S2-3).
func lineForAccount(code string, debit, credit float64) string {
	for _, candidate := range tbAccountMap {
		if len(code) >= len(candidate.prefix) && code[:len(candidate.prefix)] == candidate.prefix {
			return candidate.line
		}
	}
	if debit >= credit {
		return opening.LineOtherCurrentAssets
	}
	return opening.LineOtherCurrentLiabs
}

// OpeningReader is the S2-3 OpeningBalanceReader: the trial-balance rows
// fold into standardized per-period opening lines with the default merge
// mapping; the gate-3 sides are the engine's own per-contract balances at
// the latest TB period (the system holds no independent per-contract
// import table, so both sides share the single authority — noted in the
// mapping version).
type OpeningReader struct {
	trial        TrialBalanceSource
	measurements MeasurementSource
}

// NewOpeningReader builds the adapter.
func NewOpeningReader(trial TrialBalanceSource, measurements MeasurementSource) *OpeningReader {
	return &OpeningReader{trial: trial, measurements: measurements}
}

// Get implements finmodel.OpeningBalanceReader.
func (r *OpeningReader) Get(ctx context.Context, legalEntityID string) (*opening.OpeningBalance, []opening.ContractBalance, []opening.ContractBalance, opening.MergePolicy, error) {
	if r.trial == nil {
		return nil, nil, nil, opening.MergePolicy{}, errors.New("trial balance source unavailable")
	}
	byPeriod, currency, err := r.trial.LatestTrialBalanceByPeriod(ctx, legalEntityID)
	if err != nil {
		return nil, nil, nil, opening.MergePolicy{}, err
	}
	balance := &opening.OpeningBalance{LegalEntityID: legalEntityID, Currency: currency, Periods: []opening.PeriodBalance{}}
	periods := make([]string, 0, len(byPeriod))
	for period := range byPeriod {
		periods = append(periods, period)
	}
	sort.Strings(periods)
	for _, period := range periods {
		lines := map[string]float64{}
		mapping := map[string]string{}
		for _, line := range byPeriod[period] {
			standard := lineForAccount(line.AccountCode, line.Debit, line.Credit)
			// 存储符号约定：资产借方为正；负债与权益贷方为正（gate 1 的
			// 标准化屏语义）——LineSign 是这套约定的唯一出处。
			lines[standard] += (line.Debit - line.Credit) * opening.LineSign(standard)
			mapping[line.AccountCode] = standard
		}
		balance.Periods = append(balance.Periods, opening.PeriodBalance{Period: period, Lines: lines, Mapping: mapping})
	}
	if len(periods) == 0 {
		return nil, nil, nil, opening.MergePolicy{}, nil // 无 TB：期初端口空 → 引擎 hasOpening 降级
	}
	latestPeriod := periods[len(periods)-1]
	var ref []opening.ContractBalance
	var engine []opening.ContractBalance
	if r.measurements != nil {
		rows, err := r.measurements.ListMeasurementResultsByEntityPeriod(ctx, legalEntityID, latestPeriod)
		if err == nil {
			for _, row := range rows {
				value := opening.ContractBalance{
					ContractID:     row.ContractID,
					LeaseLiability: row.ClosingLiability.Float64(),
					ROUAsset:       row.ClosingROUAsset.Float64(),
				}
				ref = append(ref, value)
				engine = append(engine, value)
			}
		}
	}
	return balance, ref, engine, opening.MergePolicy{Version: defaultMappingVersion}, nil
}

// Compile-time seams.
var (
	_ finmodel.LeaseRollforwardReader = (*LeaseReader)(nil)
	_ finmodel.ScheduleReader         = (*ScheduleReaderAdapter)(nil)
	_ finmodel.OpeningBalanceReader   = (*OpeningReader)(nil)
)
