// 电商独立站模式的固定 Seed 模拟数据集（PRD D15：无设计伙伴时复演经营场景）。
//
// 所有行 data_classification=simulated、simulation_dataset_version='ecom-sim-v1'，
// 永不进入 production 读路径（底线 2）；结论一律 unvalidated（红线 11）。
// 幂等：同一法人下已存在 storefronts 时跳过，绝不产生第二套。
package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ErrEcomAlreadySeeded 该法人已有站点——重放不产生第二套。
var ErrEcomAlreadySeeded = errors.New("ecom simulated dataset already seeded for this legal entity")

const ecomSimDatasetVersion = "ecom-sim-v1"

// SeedEcomSimulatedData 固定 seed 生成：三站（US/EUR/JP）× 窗口 2026-08-01..14 的
// 站点日事实、广告双口径、payout/银行、准备金、订单证据、GL 收入与固定费。
// 固定公式（无随机）：确定性可复演。
func (r *EcommerceRepository) SeedEcomSimulatedData(ctx context.Context, legalEntityID string, userID *string) error {
	var count int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM storefronts WHERE legal_entity_id::text = $1`, legalEntityID).Scan(&count); err != nil {
		return fmt.Errorf("check storefronts: %w", err)
	}
	if count > 0 {
		return ErrEcomAlreadySeeded
	}

	batchID := uuid.NewString()
	if _, err := r.db.Exec(ctx, `
		INSERT INTO operating_fact_batches (id, legal_entity_id, source_system, source_file, status, total_rows,
			accepted_rows, rejected_rows, reconciliation_status, error_summary, created_by, idempotency_key)
		VALUES ($1,$2,'ecom_seed','ecom-sim-v1', 'completed', 0, 0, 0, 'unreconciled', '[]'::jsonb, $3, $4)`,
		batchID, legalEntityID, userID, "ecom:seed:sim-v1:"+legalEntityID); err != nil {
		return fmt.Errorf("create seed batch: %w", err)
	}

	sites := []struct {
		code, name, market, currency string
	}{
		{"US", "美国独立站", "US", "USD"},
		{"EU", "欧洲独立站", "DE", "EUR"},
		{"JP", "日本独立站", "JP", "JPY"},
	}
	now := time.Now().UTC()
	channels := []string{"direct", "paid_social", "email"}
	const fromDay, toDay = 1, 14

	for _, site := range sites {
		siteID := uuid.NewString()
		if _, err := r.db.Exec(ctx, `
			INSERT INTO storefronts (id, legal_entity_id, code, name, market, currency, platform, status)
			VALUES ($1,$2,$3,$4,$5,$6,'shopify','active')`,
			siteID, legalEntityID, site.code, site.name, site.market, site.currency); err != nil {
			return fmt.Errorf("seed storefront %s: %w", site.code, err)
		}

		baseGmv := map[string]float64{"USD": 12000, "EUR": 9000, "JPY": 1500000}[site.currency]
		for day := fromDay; day <= toDay; day++ {
			businessDate := time.Date(2026, 8, day, 0, 0, 0, 0, time.UTC)
			growth := 1 + float64(day-1)*0.015 // 渐进增长
			for ci, channel := range channels {
				share := []float64{0.6, 0.3, 0.1}[ci]
				gmv := round100(baseGmv * growth * share / 7)
				orders := int(gmv / (60 + float64(day%5)*2))
				if orders < 1 {
					orders = 1
				}
				discount := round100(gmv * (0.04 + float64(day%3)*0.02))
				refund := round100(gmv * (0.02 + float64(day%4)*0.005))
				chargeback := round100(gmv * 0.005)
				netRev := gmv - discount - refund - chargeback
				newOrders := orders / 3
				landed := round100(netRev * 0.35)
				fulfillment := round100(netRev * 0.06)
				paymentFee := round100(netRev * 0.025)
				tax := round100(netRev * 0.19)
				if _, err := r.db.Exec(ctx, `
					INSERT INTO storefront_day_facts (id, legal_entity_id, storefront_id, business_date, channel, sku, currency,
						gmv_amount, discount_amount, refund_amount, chargeback_loss_amount, order_count, new_customer_orders,
						landed_cost_amount, fulfillment_amount, payment_fee_amount, tax_collected_amount,
						source_system, import_batch_id, fact_version, as_of_at, data_classification, simulation_dataset_version, restated, created_by)
					VALUES ($1,$2,$3,$4,$5,'',$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,'shopify',$17,1,$18,$19,$20,false,$21)`,
					uuid.NewString(), legalEntityID, siteID, businessDate, channel, site.currency,
					gmv, discount, refund, chargeback, orders, newOrders, landed, fulfillment, paymentFee, tax,
					batchID, businessDate.Add(24*time.Hour), "simulated", ecomSimDatasetVersion, userID); err != nil {
					return fmt.Errorf("seed day fact %s d%d: %w", site.code, day, err)
				}
			}
			// 广告：campaign × booked/paid 双行（R-E1-2）
			for ci, channel := range channels {
				spend := round100(baseGmv * (0.05 + float64(day%3)*0.01) * []float64{0.5, 0.4, 0.1}[ci] / 7)
				campaign := "camp_" + channel
				for _, basis := range []string{"booked", "paid"} {
					source := "ads_booked"
					var invoiceNo any
					if basis == "paid" {
						source = "ad_invoice"
						invoiceNo = fmt.Sprintf("INV-2026-08-%02d-%s", day, channel)
					}
					impressions := int64(spend / 30 * 1000)
					clicks := int64(spend / 300)
					conversions := spend / 900
					if _, err := r.db.Exec(ctx, `
						INSERT INTO campaign_day_facts (id, legal_entity_id, storefront_id, campaign_id, campaign_name, business_date,
							basis, media_owner, spend_amount, impressions, clicks, conversions, invoice_no, currency,
							source_system, import_batch_id, fact_version, as_of_at, data_classification, simulation_dataset_version, restated)
						VALUES ($1,$2,$3,$4,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,1,$16,$17,$18,false)`,
						uuid.NewString(), legalEntityID, siteID, campaign, businessDate, basis, channel,
						spend, impressions, clicks, conversions, invoiceNo, site.currency,
						source, batchID, businessDate.Add(24*time.Hour), "simulated", ecomSimDatasetVersion); err != nil {
						return fmt.Errorf("seed campaign %s d%d: %w", site.code, day, err)
					}
				}
			}
			// 订单行证据（对账证据）
			for i := 0; i < 3; i++ {
				gross := round100(float64(80+day*3+i*7))
				if _, err := r.db.Exec(ctx, `
					INSERT INTO order_line_evidence (id, legal_entity_id, storefront_id, platform_order_no, line_no, business_date,
						channel, sku, quantity, gross_amount, discount_amount, refund_amount, tax_amount, currency, payout_id,
						evidence, source_system, import_batch_id, fact_version, as_of_at, data_classification, simulation_dataset_version)
					VALUES ($1,$2,$3,$4::text,1,$5,'direct','SKU-'||$4::text,1,$6,$7,0,$8,$9,$10,'{}'::jsonb,'shopify',$11,1,$12,$13,$14)`,
					uuid.NewString(), legalEntityID, siteID,
					fmt.Sprintf("%s-ORD-%02d-%d", site.code, day, i),
					businessDate, gross, round100(gross*0.05), round100(gross*0.19), site.currency,
					fmt.Sprintf("PO-%s-%02d", site.code, day),
					batchID, businessDate.Add(24*time.Hour), "simulated", ecomSimDatasetVersion); err != nil {
					return fmt.Errorf("seed order line %s d%d: %w", site.code, day, err)
				}
			}
			// payout + 银行（每 3 天一次；day 9 故意无银行匹配 → in_transit 差异示例）
			if day%3 == 0 {
				payoutID := fmt.Sprintf("PO-%s-%02d", site.code, day)
				gross := round100(gmvSum(baseGmv, growth, site.currency))
				fee := round100(gross * 0.03)
				net := gross - fee
				hold := 0.0
				if day == 6 {
					hold = round100(net * 0.08) // 准备金占用示例
				}
				if _, err := r.db.Exec(ctx, `
					INSERT INTO payout_lines (id, legal_entity_id, storefront_id, provider, payout_id, payout_date, currency,
						gross_amount, fee_amount, refund_amount, chargeback_amount, fx_amount, adjustment_amount,
						reserve_hold_amount, reserve_release_amount, net_amount,
						source_system, import_batch_id, fact_version, as_of_at, data_classification, simulation_dataset_version)
					VALUES ($1,$2,$3,'shopify_payments',$4,$5,$6,$7,$8,0,0,0,0,$9,0,$10,'settlement',$11,1,$12,$13,$14)`,
					uuid.NewString(), legalEntityID, siteID, payoutID, businessDate, site.currency,
					gross, fee, hold, net-hold,
					batchID, businessDate.Add(24*time.Hour), "simulated", ecomSimDatasetVersion); err != nil {
					return fmt.Errorf("seed payout %s d%d: %w", site.code, day, err)
				}
				if day != 9 { // day 9 的 payout 无银行到账 → 在途差异
					if _, err := r.db.Exec(ctx, `
						INSERT INTO bank_lines (id, legal_entity_id, storefront_id, bank_ref, value_date, currency, amount,
							direction, counterparty, envelope, source_system, import_batch_id, fact_version, as_of_at,
							data_classification, simulation_dataset_version)
						VALUES ($1,$2,$3,$4,$5,$6,$7,'in','Shopify Payouts','{}'::jsonb,'bank',$8,1,$9,$10,$11)`,
						uuid.NewString(), legalEntityID, siteID, "BNK-"+payoutID, businessDate.Add(24*time.Hour), site.currency,
						net-hold, batchID, businessDate.Add(48*time.Hour), "simulated", ecomSimDatasetVersion); err != nil {
						return fmt.Errorf("seed bank %s d%d: %w", site.code, day, err)
					}
				}
				if hold > 0 {
					if _, err := r.db.Exec(ctx, `
						INSERT INTO rolling_reserve_events (id, legal_entity_id, storefront_id, provider, event_type, event_date,
							currency, amount, payout_id, status, source_system, import_batch_id, fact_version, as_of_at,
							data_classification, simulation_dataset_version)
						VALUES ($1,$2,$3,'shopify_payments','hold',$4,$5,$6,$7,'open','settlement',$8,1,$9,$10,$11)`,
						uuid.NewString(), legalEntityID, siteID, businessDate, site.currency, hold, payoutID,
						batchID, businessDate.Add(24*time.Hour), "simulated", ecomSimDatasetVersion); err != nil {
						return fmt.Errorf("seed reserve %s: %w", site.code, err)
					}
				}
			}
		}
		// GL 收入 + 固定费（月度口径）
		var glAmount, fixedAmount float64
		switch site.currency {
		case "USD":
			glAmount, fixedAmount = 158000, 26500
		case "EUR":
			glAmount, fixedAmount = 124000, 21900
		default:
			glAmount, fixedAmount = 20500000, 3900000
		}
		if _, err := r.db.Exec(ctx, `
			INSERT INTO storefront_gl_revenues (id, legal_entity_id, storefront_id, period, currency, revenue_amount, gl_account,
				source_system, import_batch_id, fact_version, as_of_at, data_classification, simulation_dataset_version)
			VALUES ($1,$2,$3,'2026-08',$4,$5,'4000-revenue','gl_revenue',$6,1,$7,$8,$9)`,
			uuid.NewString(), legalEntityID, siteID, site.currency, glAmount, batchID, now, "simulated", ecomSimDatasetVersion); err != nil {
			return fmt.Errorf("seed gl %s: %w", site.code, err)
		}
		if _, err := r.db.Exec(ctx, `
			INSERT INTO storefront_fixed_costs (id, legal_entity_id, storefront_id, period, currency, fixed_cost_amount, memo,
				source_system, import_batch_id, fact_version, as_of_at, data_classification, simulation_dataset_version)
			VALUES ($1,$2,$3,'2026-08',$4,$5,'ecom-sim-v1 seed','overhead',$6,1,$7,$8,$9)`,
			uuid.NewString(), legalEntityID, siteID, site.currency, fixedAmount, batchID, now, "simulated", ecomSimDatasetVersion); err != nil {
			return fmt.Errorf("seed fixed %s: %w", site.code, err)
		}
	}
	return nil
}

func gmvSum(baseGmv float64, growth float64, currency string) float64 {
	return round100(baseGmv * growth * 0.35)
}

func round100(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
