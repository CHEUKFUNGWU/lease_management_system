package repository

// R2-1（RH3）：投前保本端点的跨法人隔离证据（底线 1）。
//
// 端点在 promo_id 模式下的取数路径是：
//
//	GetPromotion(ctx, tenantID, promoID)        → 404 边界
//	GetPromotionActualFacts(ctx, tenantID, ...) → 基线事实
//
// 本文件直接对这两个仓库方法验隔离：法人 B 的租户 ID 拿不到法人 A 的
// 活动，也读不到法人 A 门店的事实；同时用法人 A 自己的数字跑一次
// Breakeven()，证明「拿不到」是隔离在工作，不是数据没种上。
// 只跑单元测试证明不了底线 1 —— 这些断言全部需要真库。

import (
	"context"
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/services/promotionattribution"
)

func seedBreakevenTenantWithPromo(t *testing.T, ctx context.Context, label string) (legalEntityID, storeID, promoID string) {
	t.Helper()
	pool := retailStoreDayFactsPool(t)
	suffix := label + "-" + time.Now().UTC().Format("150405") + "-" + label
	if err := pool.QueryRow(ctx, `
		INSERT INTO legal_entities (code, name, country, currency, is_active)
		VALUES ($1, $2, 'CN', 'CNY', true) RETURNING id
	`, "BE-LE-"+suffix, "Breakeven entity "+label).Scan(&legalEntityID); err != nil {
		t.Fatalf("seed legal entity: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO stores (code, name, legal_entity_id, brand, region, is_active)
		VALUES ($1, $2, $3, $4, $5, true) RETURNING id
	`, "BE-ST-"+suffix, "Breakeven store "+label, legalEntityID, "Brand-"+label, "Region-"+label).Scan(&storeID); err != nil {
		t.Fatalf("seed store: %v", err)
	}
	repo := NewPromotionRepository(pool)
	promo := &Promotion{
		LegalEntityID: legalEntityID,
		PromoCode:     "BE-PROMO-" + suffix,
		Name:          "Breakeven promo " + label,
		PromoType:     "discount",
		StartDate:     "2026-07-01",
		EndDate:       "2026-07-07",
		TargetScope:   "store",
		ScopeValues:   []string{storeID},
		Currency:      "CNY",
		BudgetAmount:  1000,
	}
	if err := repo.CreatePromotion(ctx, promo); err != nil {
		t.Fatalf("create promotion: %v", err)
	}
	return legalEntityID, storeID, promo.ID
}

func TestBreakevenBaselineIsolationAcrossLegalEntities(t *testing.T) {
	ctx := context.Background()
	pool := retailStoreDayFactsPool(t)
	repo := NewPromotionRepository(pool)

	entityA, storeA, promoA := seedBreakevenTenantWithPromo(t, ctx, "ent-a")
	entityB, _, _ := seedBreakevenTenantWithPromo(t, ctx, "ent-b")

	// A 的门店种 7 天基线期事实（活动前窗口），供基线计算
	for d := 0; d < 7; d++ {
		date := time.Date(2026, 6, 24+d, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
		if _, err := pool.Exec(ctx, `
			INSERT INTO retail_store_day_facts (store_id, business_date, currency, revenue, gross_profit, transactions, source_system, data_classification)
			VALUES ($1, $2, 'CNY', 10000, 3000, 400, 'breakeven-itest', 'production')
		`, storeA, date); err != nil {
			t.Fatalf("seed fact: %v", err)
		}
	}

	// 断言 1（授权边界）：法人 B 用自己的租户 ID 读 A 的活动 → 必须查无此物。
	// 这正是 breakeven handler 的第一道取数；跨法人 promo_id 在这里就死掉，
	// 到不了任何计算。
	crossEntity, err := repo.GetPromotion(ctx, entityB, promoA)
	if err == nil && crossEntity != nil && crossEntity.ID == promoA {
		t.Fatal("entity B must not read entity A's promotion through GetPromotion")
	}

	// 断言 2（基线事实边界）：B 租户查询同一时间窗，即使把 scope 精确指向
	// A 的门店 ID，也必须拿到零行——事实的租户边界走 stores join。
	factsForB, err := repo.GetPromotionActualFacts(ctx, entityB, "2026-06-24", "2026-06-30", []string{storeA})
	if err != nil {
		t.Fatalf("facts query for entity B: %v", err)
	}
	if len(factsForB) != 0 {
		t.Fatalf("entity B saw %d facts belonging to entity A's store", len(factsForB))
	}

	// 断言 3（不是空库假阳性）：A 自己能读到活动与事实，且 Breakeven()
	// 在这些数字上产出可算的结果——证明上面的「空」来自隔离而非没种数。
	own, err := repo.GetPromotion(ctx, entityA, promoA)
	if err != nil || own.ID != promoA {
		t.Fatalf("entity A must read its own promotion, got %v / %v", own, err)
	}
	ownFacts, err := repo.GetPromotionActualFacts(ctx, entityA, "2026-06-24", "2026-06-30", []string{storeA})
	if err != nil {
		t.Fatalf("facts query for entity A: %v", err)
	}
	if len(ownFacts) != 7 {
		t.Fatalf("entity A seeded 7 baseline days, saw %d", len(ownFacts))
	}
	var dailyRev float64
	for _, f := range ownFacts {
		if f.Revenue != nil {
			dailyRev += *f.Revenue
		}
	}
	res := promotionattribution.Breakeven(promotionattribution.BreakevenInput{
		Currency:           "CNY",
		EventDays:          7,
		Baseline:           promotionattribution.RunRate{DailyRevenue: dailyRev / 7},
		BaselineMarginRate: 0.30,
		PromoMarginRate:    0.22,
		FixedMarketingCost: 5000,
	})
	if res.Status != "achievable" || res.RequiredIncrementalRevenue == nil {
		t.Fatalf("breakeven on entity A's own numbers must be computable, got %+v", res)
	}
}
