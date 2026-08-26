package ecomfact

import (
	"fmt"
	"time"
)

// VersionKey / Version 让三类事实满足 ecomfact.Versioned，从而共用 Highest。
// 站点日事实的业务键 =（storefront × 日 × 渠道 × SKU × source_system）——
// 多来源并存：shopify 出销售类度量、procurement 出落地成本、3pl 出履约成本，
// 读取端先按（业务键 × source_system）取最高版本，再按度量求和。

func (f StorefrontDayFact) VersionKey() string {
	return fmt.Sprintf("%s|%s|%s|%s", f.StorefrontID, f.BusinessDate.Format(time.DateOnly), f.Channel, f.SKU) +
		"|" + f.SourceEnvelope.SourceSystem
}

func (f StorefrontDayFact) Version() int { return f.SourceEnvelope.FactVersion }

func (f CampaignDayFact) VersionKey() string {
	return fmt.Sprintf("%s|%s|%s|%s", f.StorefrontID, f.CampaignID, f.BusinessDate.Format(time.DateOnly), f.Basis) +
		"|" + f.SourceEnvelope.SourceSystem
}

func (f CampaignDayFact) Version() int { return f.SourceEnvelope.FactVersion }

// HighestStorefrontDays 对站点日事实做 Highest Fact Version 解析的强类型包装。
func HighestStorefrontDays(facts []StorefrontDayFact) []StorefrontDayFact {
	versioned := make([]Versioned, len(facts))
	for i := range facts {
		versioned[i] = facts[i]
	}
	out := Highest(versioned)
	res := make([]StorefrontDayFact, 0, len(out))
	for _, v := range out {
		if f, ok := v.(StorefrontDayFact); ok {
			res = append(res, f)
		}
	}
	return res
}

// HighestCampaignDays 对广告日事实做 Highest Fact Version 解析的强类型包装。
func HighestCampaignDays(facts []CampaignDayFact) []CampaignDayFact {
	versioned := make([]Versioned, len(facts))
	for i := range facts {
		versioned[i] = facts[i]
	}
	out := Highest(versioned)
	res := make([]CampaignDayFact, 0, len(out))
	for _, v := range out {
		if f, ok := v.(CampaignDayFact); ok {
			res = append(res, f)
		}
	}
	return res
}

// RestatedPeriods 返回窗口内被重述过的（storefront × 日）集合：
// 同一业务键存在多个 fact_version 时旧版本仍在库、新版本进读路径，
// 被重述期间带修订标记（R-E2-2）。输入应为未经 Highest 解析的全量行。
func RestatedPeriods(facts []StorefrontDayFact) map[string]bool {
	marks := map[string]bool{}
	for _, f := range facts {
		key := f.StorefrontID + "|" + f.BusinessDate.Format(time.DateOnly)
		if f.SourceEnvelope.FactVersion > 1 || f.SourceEnvelope.Restated {
			marks[key] = true
		}
	}
	return marks
}
