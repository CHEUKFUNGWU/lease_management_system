package aiagent

import "testing"

// The fill dispatch gates must stay narrow: an explicit prefill request is
// required, and ordinary parse/upload phrasings must never be hijacked into
// the page-fill seams (agent-universal-pagefill-v1 P0-B①).
//
// Division of labour: wantsScheduleFill checks only the prefill wording —
// the rent-schedule context comes from the DocRentSchedule triage check at
// its call site, so a terse "预填一下" with a rent-sheet file still routes.
// wantsPlanFill / wantsTrialBalanceFill have no triage class to lean on, so
// they additionally require their own domain words in the message.
func TestFillDispatchGatesStayNarrow(t *testing.T) {
	cases := []struct {
		name               string
		message            string
		schedule, plan, tb bool
	}{
		{"rent schedule prefill", "把这份租金表预填到付款计划表单", true, false, false},
		{"rent schedule plain parse", "解析这份租金表", false, false, false},
		{"rent schedule english prefill", "prefill the rent schedule into the form", true, false, false},
		{"budget prefill", "预填预算 2026-08 到 2026-12", true, true, false},
		{"forecast prefill", "help me prefill the forecast version", true, true, false},
		{"budget plain import", "导入这份预算文件", false, false, false},
		{"trial balance prefill", "预填试算平衡表 来源系统 gl-export 2026-07", true, false, true},
		{"trial balance plain parse", "解析试算平衡表", false, false, false},
		{"tb alone must not trigger plan or tb", "预填这份 TB 文件", true, false, false},
		{"scenario beats forecast", "预填情景预测 2026-09", true, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := wantsScheduleFill(tc.message); got != tc.schedule {
				t.Fatalf("wantsScheduleFill(%q) = %v, want %v", tc.message, got, tc.schedule)
			}
			if got := wantsPlanFill(tc.message); got != tc.plan {
				t.Fatalf("wantsPlanFill(%q) = %v, want %v", tc.message, got, tc.plan)
			}
			if got := wantsTrialBalanceFill(tc.message); got != tc.tb {
				t.Fatalf("wantsTrialBalanceFill(%q) = %v, want %v", tc.message, got, tc.tb)
			}
		})
	}
}

func TestExtractVersionTypePrefersScenarioOverForecast(t *testing.T) {
	if got := extractVersionType("预填情景预测 2026-09"); got != "scenario" {
		t.Fatalf("scenario must win over forecast, got %q", got)
	}
	if got := extractVersionType("预填滚动预测 2026-09"); got != "forecast" {
		t.Fatalf("rolling forecast = %q", got)
	}
	if got := extractVersionType("随便看看"); got != "" {
		t.Fatalf("no hint must stay empty, got %q", got)
	}
}

func TestExtractPlanPeriodsPinsRangeFromMessage(t *testing.T) {
	from, to := extractPlanPeriods("预填预算 2026-08 到 2026-12")
	if from != "2026-08" || to != "2026-12" {
		t.Fatalf("range = %s ~ %s", from, to)
	}
	from, to = extractPlanPeriods("预填预算 2026-08")
	if from != "2026-08" || to != "2026-08" {
		t.Fatalf("single month must pin both, got %s ~ %s", from, to)
	}
	if from, to = extractPlanPeriods("预填预算"); from != "" || to != "" {
		t.Fatalf("no periods must stay empty, got %s ~ %s", from, to)
	}
}
