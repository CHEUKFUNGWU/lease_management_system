package tools

// Ch1 BG1 接线：诊断工具在返回时附带产出利润差异瀑布图。
//
// 翻译发生在消费方（本文件）——varianceattribution 把结果翻译成共享词汇
// charts.Waterfall，渲染层不认识任何业务包（D-B17）。Status != complete 时
// 返回 nil：不画空坐标系、不用 0 填段、不产出 artifact（D-B5 诚实拒绝）。

import (
	"context"
	"strings"

	"github.com/lease-management-system/core-service/internal/charts"
	"github.com/lease-management-system/core-service/internal/services/retailstore360"
	"github.com/lease-management-system/core-service/internal/services/varianceattribution"
)

// profitWaterfallForDiagnostics computes the profit-variance waterfall for
// the diagnostics' own current/comparison windows and renders it. Nil means
// "no chart" — attribution unavailable is an honest outcome, never a blank
// canvas filled with zeros.
func profitWaterfallForDiagnostics(ctx context.Context, reader RetailOperationsReader, legalEntityID string, response *retailstore360.Response) *ProfitWaterfallBlock {
	if reader == nil || response == nil {
		return nil
	}
	if response.Current.DateFrom == "" || response.Current.DateTo == "" ||
		response.Comparison.DateFrom == "" || response.Comparison.DateTo == "" ||
		strings.TrimSpace(response.Store.StoreID) == "" {
		return nil
	}
	classification := strings.TrimSpace(response.DataClassification)
	if classification == "" {
		classification = "production"
	}
	dataset := response.DatasetVersion
	source := firstString(response.SourceSystems)
	storeIDs := []string{response.Store.StoreID}

	baseSet, err := reader.QueryFacts(ctx, strings.TrimSpace(legalEntityID), response.Comparison.DateFrom, response.Comparison.DateTo,
		classification, dataset, source, storeIDs)
	if err != nil || baseSet == nil {
		return nil
	}
	currentSet, err := reader.QueryFacts(ctx, strings.TrimSpace(legalEntityID), response.Current.DateFrom, response.Current.DateTo,
		classification, dataset, source, storeIDs)
	if err != nil || currentSet == nil {
		return nil
	}

	basePeriod, _ := varianceattribution.AggregateWindow(baseSet.Facts)
	currentPeriod, _ := varianceattribution.AggregateWindow(currentSet.Facts)
	result, err := varianceattribution.Attribute(basePeriod, currentPeriod, response.Currency, nil)
	if err != nil || result.Status != "complete" {
		// D-B5：不可得即诚实拒绝——没有图，也没有被 0 填满的假图。
		return nil
	}

	waterfall := charts.Waterfall{
		StartLabel:     "基期利润",
		StartValue:     result.BaseProfit,
		EndLabel:       "当期利润",
		EndValue:       result.CurrentProfit,
		Currency:       result.Currency,
		Classification: classification,
		OrderNote:      strings.Join(result.DecompositionOrder, " → "),
	}
	for _, factor := range result.Factors {
		// 残差不进 Steps（D-B4）：精确连环替代下它是浮点噪声。
		waterfall.Steps = append(waterfall.Steps, charts.Step{Label: factor.Factor, Delta: factor.Effect})
	}
	svg, err := charts.Render(waterfall)
	if err != nil {
		return nil
	}
	return &ProfitWaterfallBlock{
		SVG:                svg,
		DecompositionOrder: append([]string(nil), result.DecompositionOrder...),
		DataClassification: classification,
		Status:             "complete",
		Currency:           result.Currency,
	}
}

func firstString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
