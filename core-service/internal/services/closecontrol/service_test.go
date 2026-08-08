package closecontrol

import (
	"context"
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/services/closereadiness"
	"github.com/lease-management-system/core-service/internal/services/controlrules"
)

type factsStub struct {
	facts closereadiness.Facts
}

func (s factsStub) LoadFacts(context.Context, string, string) (closereadiness.Facts, error) {
	return s.facts, nil
}

type rateStub struct{}

func (rateStub) GetFloat64(context.Context, string, float64) float64 { return 0 }

type exceptionRepoStub struct {
	detections []Detection
	exceptions map[string]*Exception
}

func (s *exceptionRepoStub) GetActiveRule(_ context.Context, code string, _ time.Time) (controlrules.Definition, bool, error) {
	return controlrules.Definition{Code: code, Version: "test", Severity: controlrules.SeverityBlocking, GateEffect: "formal_calculation"}, true, nil
}

func (s *exceptionRepoStub) PersistDetections(_ context.Context, detections []Detection) ([]Exception, error) {
	s.detections = append(s.detections, detections...)
	result := make([]Exception, 0, len(detections))
	for _, detection := range detections {
		if s.exceptions == nil {
			s.exceptions = map[string]*Exception{}
		}
		exception := s.exceptions[detection.Fingerprint]
		if exception == nil {
			exception = &Exception{
				ID: "exception-" + detection.Fingerprint[:8], DetectionEventID: "event-" + detection.Fingerprint[:8],
				RuleCode: detection.RuleCode, RuleVersion: detection.RuleVersion, Severity: detection.Severity,
				GateEffect: detection.GateEffect, AccountingPeriod: detection.AccountingPeriod,
				LegalEntityID: detection.LegalEntityID, SubjectType: detection.SubjectType, SubjectID: detection.SubjectID,
				Fingerprint: detection.Fingerprint, ProjectionVersion: detection.ProjectionVersion,
				ExceptionState: StateOpen, ClosingDisposition: DispositionUnresolved, OpenedAt: detection.DetectedAt,
			}
			s.exceptions[detection.Fingerprint] = exception
		}
		result = append(result, *exception)
	}
	return result, nil
}

func (s *exceptionRepoStub) ListExceptions(context.Context, string, string) ([]Exception, error) {
	result := make([]Exception, 0, len(s.exceptions))
	for _, exception := range s.exceptions {
		result = append(result, *exception)
	}
	return result, nil
}

func (s *exceptionRepoStub) GetException(_ context.Context, id string) (*Exception, error) {
	for _, exception := range s.exceptions {
		if exception.ID == id {
			copy := *exception
			return &copy, nil
		}
	}
	return nil, ErrNotFound
}

func (s *exceptionRepoStub) UpdateException(_ context.Context, id string, update ExceptionUpdate) (*Exception, error) {
	for _, exception := range s.exceptions {
		if exception.ID != id {
			continue
		}
		exception.ExceptionState = update.ExceptionState
		exception.ClosingDisposition = update.ClosingDisposition
		exception.OwnerID = update.OwnerID
		exception.ReviewerID = update.ReviewerID
		exception.ApproverID = update.ApproverID
		exception.InvestigatingAt = update.InvestigatingAt
		exception.ResolvedAt = update.ResolvedAt
		exception.WaivedAt = update.WaivedAt
		exception.ClosedAt = update.ClosedAt
		next := *exception
		return &next, nil
	}
	return nil, ErrNotFound
}

func (s *exceptionRepoStub) HasUnresolvedBlocking(context.Context, string, string) (bool, error) {
	for _, exception := range s.exceptions {
		if exception.Severity == "blocking" && exception.ExceptionState != StateClosed {
			return true, nil
		}
	}
	return false, nil
}

func TestFingerprintIsStableAndScopedToSubject(t *testing.T) {
	first := Fingerprint("2026-08", "le-1", RuleMissingPaymentSchedule, "contract", "contract-1")
	if first != Fingerprint("2026-08", "le-1", RuleMissingPaymentSchedule, "contract", "contract-1") {
		t.Fatal("fingerprint is not stable")
	}
	if first == Fingerprint("2026-09", "le-1", RuleMissingPaymentSchedule, "contract", "contract-1") {
		t.Fatal("period must participate in fingerprint")
	}
}

func TestDetectDeduplicatesByFingerprintAndPreservesDetectionEvent(t *testing.T) {
	repo := &exceptionRepoStub{}
	service := NewService(factsStub{facts: closereadiness.Facts{Contracts: []closereadiness.ContractFact{
		{ContractID: "contract-1", ContractNumber: "LC-001", LeaseScope: "in_scope"},
	}}}, rateStub{}, repo)
	when := time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 2; i++ {
		result, err := service.Detect(context.Background(), DetectCommand{AccountingPeriod: "2026-08", LegalEntityID: "le-1", Now: when})
		if err != nil {
			t.Fatalf("detect: %v", err)
		}
		if result.DetectionCount != 2 {
			t.Fatalf("detection count = %d, want payment + rate", result.DetectionCount)
		}
	}
	if len(repo.detections) != 4 {
		t.Fatalf("detection events = %d, want two runs of two findings", len(repo.detections))
	}
	if len(repo.exceptions) != 2 {
		t.Fatalf("exceptions = %d, want one per fingerprint", len(repo.exceptions))
	}
}

func TestExceptionLifecycleSeparatesDispositionAndActors(t *testing.T) {
	repo := &exceptionRepoStub{}
	exception := &Exception{ID: "ex-1", RuleCode: RuleMissingPaymentSchedule, Severity: "blocking", ExceptionState: StateOpen, ClosingDisposition: DispositionUnresolved}
	repo.exceptions = map[string]*Exception{"fp": exception}
	service := NewService(factsStub{}, nil, repo)
	when := time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC)

	before, after, err := service.ApplyAction(context.Background(), ActionCommand{ExceptionID: "ex-1", Action: ActionAssign, ActorID: "reviewer-1", OwnerID: "owner-1", Note: "已分派", Now: when})
	if err != nil || before.ExceptionState != StateOpen || after.ExceptionState != StateInvestigating || after.ClosingDisposition != DispositionUnresolved {
		t.Fatalf("assign result = before=%+v after=%+v err=%v", before, after, err)
	}
	_, after, err = service.ApplyAction(context.Background(), ActionCommand{ExceptionID: "ex-1", Action: ActionVerifyResolution, ActorID: "owner-1", Note: "不应自我复核", Now: when})
	if err != ErrInvalidTransition || after != nil {
		t.Fatalf("self-review err = %v after=%+v", err, after)
	}
	_, after, err = service.ApplyAction(context.Background(), ActionCommand{ExceptionID: "ex-1", Action: ActionVerifyResolution, ActorID: "reviewer-2", Note: "付款计划已完成审批", Now: when})
	if err != nil || after.ExceptionState != StateResolved || after.ClosingDisposition != DispositionVerifiedResolution || after.ReviewerID == nil {
		t.Fatalf("resolve result = %+v err=%v", after, err)
	}
	_, _, err = service.ApplyAction(context.Background(), ActionCommand{ExceptionID: "ex-1", Action: ActionClose, ActorID: "reviewer-2", Note: "不能由复核人关闭", Now: when})
	if err != ErrRoleSeparation {
		t.Fatalf("reviewer close err = %v, want role separation", err)
	}
}
