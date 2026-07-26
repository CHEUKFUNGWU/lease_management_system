package handlers

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/services/ifrs16"
)

// The product speaks Chinese, so a clause a finance user typed wrong must say
// so in Chinese. These cases pair each message with the engine error that
// actually produces it, so a reworded engine error surfaces here rather than
// silently falling back to the untranslated wrapper.
func TestClauseError_TranslatesWhatTheEngineActuallyReturns(t *testing.T) {
	cases := []struct {
		name     string
		revision ifrs16.PaymentRevision
		want     string
	}{
		{"index without readings", ifrs16.PaymentRevision{Kind: ifrs16.RevisionIndex, BaseIndex: 100}, "指数联动条款需要基期与现期两个指数，且均须为正数"},
		{"stepped with no steps", ifrs16.PaymentRevision{Kind: ifrs16.RevisionStepped}, "阶梯租金条款至少需要一级阶梯"},
		{"step without a date", ifrs16.PaymentRevision{Kind: ifrs16.RevisionStepped, Steps: []ifrs16.StepChange{{Amount: 100}}}, "每一级阶梯都需要填写起始日"},
		{"negative rent", ifrs16.PaymentRevision{Kind: ifrs16.RevisionSetAmount, Amount: -1}, "调整后租金不能为负数"},
		{"unknown kind", ifrs16.PaymentRevision{Kind: "guesswork"}, "条款类型无法识别"},
		{"reduction past zero", ifrs16.PaymentRevision{Kind: ifrs16.RevisionPercentage, Percentage: -100}, "降幅过大，租金将降至零或以下"},
		{"window inverted", ifrs16.PaymentRevision{
			Kind: ifrs16.RevisionPercentage, Percentage: 5,
			AppliesFrom: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
			AppliesTo:   time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		}, "条款结束日早于起始日"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := ifrs16.DeriveRevisedPayments(nil, testCase.revision, time.Time{})
			if err == nil {
				t.Fatal("the engine accepted terms this case expects it to reject")
			}
			got := clauseError(err)
			if got != testCase.want {
				t.Errorf("clauseError = %q, want %q (engine said %q)", got, testCase.want, err)
			}
		})
	}
}

// An error nobody anticipated still has to reach the user rather than vanish.
func TestClauseError_FallsBackWithTheOriginalDetail(t *testing.T) {
	got := clauseError(errors.New("something nobody planned for"))
	if !strings.Contains(got, "条款参数无法推导付款流") || !strings.Contains(got, "something nobody planned for") {
		t.Errorf("fallback lost either the Chinese framing or the detail: %q", got)
	}
}
