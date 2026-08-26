package leasescenario

// C2 golden 锚（架构重构任务书 2026-08-26）：
//
// /reports/rent-to-sales 的 HTTP 层依赖真实 PG 仓储（StoreMetricsHandler 持有
// 具体仓库指针），服务层是这个包可独立钉住的全部。本 golden 用固定四家门店
// （覆盖 healthy / watch / over_threshold / no_revenue 四条路径，组合比可给、
// 覆盖声明可给）把 Result 的 JSON 逐字节钉死——键序即契约。
// 再生：UPDATE_RATIO_GOLDEN=1 go test ./internal/services/leasescenario/ -run TestRatioGolden

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func ratioGoldenInput() RatioInput {
	area := 500.0
	return RatioInput{
		Period:         "2026-05",
		HealthyCeiling: 8,
		WarningCeiling: 12,
		Stores: []StoreInput{
			{StoreID: "s1", StoreCode: "S001", StoreName: "旗舰店", Brand: "北岛", Region: "华东",
				CashRent: ptrF(60000), RentCurrency: "CNY", Revenue: ptrF(1000000), RevenueCurrency: "CNY",
				RevenueVersion: ptrI(3), RevenueSource: "pos", AreaSqm: &area},
			{StoreID: "s2", StoreCode: "S002", StoreName: "社区店", Brand: "北岛", Region: "华北",
				CashRent: ptrF(50000), RentCurrency: "CNY", Revenue: ptrF(500000), RevenueCurrency: "CNY",
				RevenueVersion: ptrI(1), RevenueSource: "pos"},
			{StoreID: "s3", StoreCode: "S003", StoreName: "老城店", Brand: "南屿", Region: "华南",
				CashRent: ptrF(90000), RentCurrency: "CNY", Revenue: ptrF(700000), RevenueCurrency: "CNY",
				RevenueVersion: ptrI(2), RevenueSource: "bi"},
			{StoreID: "s4", StoreCode: "S004", StoreName: "新店", Brand: "南屿", Region: "西南",
				CashRent: ptrF(40000), RentCurrency: "CNY", Revenue: nil, RevenueCurrency: "",
				RevenueVersion: nil, RevenueSource: ""},
		},
	}
}

func ptrF(v float64) *float64 { return &v }
func ptrI(v int) *int         { return &v }

func TestRatioGolden(t *testing.T) {
	result, err := Calculate(ratioGoldenInput())
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}
	got, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got = append(got, '\n')

	goldenPath := filepath.Join("testdata", "ratio_golden.json")
	if os.Getenv("UPDATE_RATIO_GOLDEN") != "" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden (seed once with UPDATE_RATIO_GOLDEN=1): %v", err)
	}
	if string(want) != string(got) {
		t.Fatalf("ratio golden drift (byte-exact contract changed):\nwant:\n%s\ngot:\n%s", want, got)
	}
}
