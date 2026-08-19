package opening

import "testing"

// 三道闸各自的「破坏必红」反向用例：先证破坏后被拒，再证修复后通过。

func balancedPeriod() PeriodBalance {
	return PeriodBalance{
		Period: "2026-01",
		Lines: map[string]float64{
			"cash": 500, "ar": 100, "inventory": 60, "ppe": 300, "rou_asset": 1200,
			"ap": 40, "lease_liability": 1000, "borrowings": 200,
			"share_capital": 500, "retained_earnings": 420,
		},
		Mapping: map[string]string{"1101": "cash", "1201": "ar"},
	}
}

func passingInput() ValidateInput {
	return ValidateInput{
		Balance: OpeningBalance{
			LegalEntityID: "LE-1", Currency: "CNY",
			Periods: []PeriodBalance{balancedPeriod(), {
				Period: "2026-02",
				Lines: map[string]float64{
					"cash": 520, "ar": 110, "inventory": 62, "ppe": 310, "rou_asset": 1150,
					"ap": 48, "lease_liability": 910, "borrowings": 180,
					"share_capital": 500, "retained_earnings": 514,
				},
				Mapping: map[string]string{"1101": "cash", "1201": "ar"},
			}},
		},
		LeaseRef: []ContractBalance{{ContractID: "C1", LeaseLiability: 1000, ROUAsset: 1200}},
		Engine:   []ContractBalance{{ContractID: "C1", LeaseLiability: 1000, ROUAsset: 1200}},
		Policy:   MergePolicy{Version: "v1"},
	}
}

func TestGate1SelfBalanceReverse(t *testing.T) {
	in := passingInput()
	if failures := Validate(in); len(failures) != 0 {
		t.Fatalf("intact input must pass, got %+v", failures)
	}
	// 破坏：抬高资产，使 2026-02 失衡。
	in.Balance.Periods[1].Lines["cash"] += 40
	failures := Validate(in)
	if len(failures) == 0 {
		t.Fatal("gate 1 must fire on an unbalanced period")
	}
	for _, failure := range failures {
		if failure.Gate == "1" && failure.Period == "2026-02" {
			return
		}
	}
	t.Fatalf("gate 1 must locate the broken period, got %+v", failures)
}

func TestGate2MergeStabilityReverse(t *testing.T) {
	in := passingInput()
	if failures := Validate(in); len(failures) != 0 {
		t.Fatalf("intact input must pass, got %+v", failures)
	}
	// 破坏：同一外部科目在第二期归并到不同标准行。
	in.Balance.Periods[1].Mapping["1101"] = "other_current_assets"
	failures := Validate(in)
	if len(failures) == 0 {
		t.Fatal("gate 2 must fire on merge drift")
	}
	for _, failure := range failures {
		if failure.Gate == "2" {
			return
		}
	}
	t.Fatalf("gate 2 failure missing: %+v", failures)
}

func TestGate3LeaseReconciliationReverse(t *testing.T) {
	in := passingInput()
	if failures := Validate(in); len(failures) != 0 {
		t.Fatalf("intact input must pass, got %+v", failures)
	}
	// 破坏 1：期初租赁负债 ≠ 引擎余额（容差 0）。
	in.LeaseRef = []ContractBalance{{ContractID: "C1", LeaseLiability: 999.99, ROUAsset: 1200}}
	failures := Validate(in)
	if len(failures) == 0 {
		t.Fatal("gate 3 must fire on liability mismatch")
	}
	found := false
	for _, failure := range failures {
		if failure.Gate == "3" {
			found = true
		}
	}
	if !found {
		t.Fatalf("gate 3 failure missing: %+v", failures)
	}
	// 破坏 2：导入合同在引擎侧无同期余额。
	in = passingInput()
	in.Engine = nil
	if failures := Validate(in); len(failures) == 0 {
		t.Fatal("gate 3 must fire when the engine has no balance for the contract")
	}
}

func TestOpeningBalancedFixtures(t *testing.T) {
	// 夹具本身必须平衡——防止反向测试建立在坏的基线上（假绿防护）。
	first := passingInput().Balance.Periods[0]
	assets := first.Lines["cash"] + first.Lines["ar"] + first.Lines["inventory"] + first.Lines["ppe"] + first.Lines["rou_asset"]
	liabEquity := first.Lines["ap"] + first.Lines["lease_liability"] + first.Lines["borrowings"] + first.Lines["share_capital"] + first.Lines["retained_earnings"]
	if assets-liabEquity > 0.01 || assets-liabEquity < -0.01 {
		t.Fatalf("fixture must balance (assets %.2f vs L+E %.2f)", assets, liabEquity)
	}
}
