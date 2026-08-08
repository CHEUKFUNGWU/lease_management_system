// Package closecontrol owns the persisted governance layer on top of the
// deterministic Close Readiness facts. It deliberately keeps detection,
// exception state and closing disposition as separate concepts.
package closecontrol

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lease-management-system/core-service/internal/services/closereadiness"
	"github.com/lease-management-system/core-service/internal/services/controlrules"
)

const ProjectionVersion = "close-readiness-v1"

const (
	RuleMissingPaymentSchedule = controlrules.RuleMissingPaymentSchedule
	RuleMissingDiscountRate    = controlrules.RuleMissingDiscountRate
	RulePendingEvent           = controlrules.RulePendingEvent
	RuleFailedCloseBatch       = controlrules.RuleFailedCloseBatch
)

const (
	StateOpen          = "open"
	StateInvestigating = "investigating"
	StateResolved      = "resolved"
	StateWaived        = "waived"
	StateClosed        = "closed"

	DispositionUnresolved           = "unresolved"
	DispositionVerifiedResolution   = "verified_resolution"
	DispositionAccountingConclusion = "accounting_conclusion"
	DispositionPeriodWaiver         = "period_waiver"
	DispositionStandingWaiver       = "standing_waiver"
)

var (
	ErrNotFound          = errors.New("close exception not found")
	ErrInvalidTransition = errors.New("invalid close exception transition")
	ErrNoteRequired      = errors.New("exception action note is required")
	ErrOwnerRequired     = errors.New("exception owner is required")
	ErrRoleSeparation    = errors.New("exception actors must be separated")
	ErrScopeRequired     = errors.New("exception governance requires legal-entity-wide scope")
)

type Rule = controlrules.Definition

type Detection struct {
	RuleCode          string
	RuleVersion       string
	Severity          string
	GateEffect        string
	AccountingPeriod  string
	LegalEntityID     string
	SubjectType       string
	SubjectID         string
	SubjectContractID string
	ProjectionVersion string
	Fingerprint       string
	Evidence          map[string]any
	DetectedAt        time.Time
}

type Exception struct {
	ID                 string     `json:"id"`
	DetectionEventID   string     `json:"detection_event_id"`
	RuleCode           string     `json:"rule_code"`
	RuleVersion        string     `json:"rule_version"`
	Severity           string     `json:"severity"`
	GateEffect         string     `json:"gate_effect"`
	AccountingPeriod   string     `json:"accounting_period"`
	LegalEntityID      string     `json:"legal_entity_id"`
	SubjectType        string     `json:"subject_type"`
	SubjectID          string     `json:"subject_id"`
	SubjectContractID  *string    `json:"subject_contract_id,omitempty"`
	ContractNumber     string     `json:"contract_number,omitempty"`
	ContractName       string     `json:"contract_name,omitempty"`
	BatchNumber        string     `json:"batch_number,omitempty"`
	Fingerprint        string     `json:"fingerprint"`
	ProjectionVersion  string     `json:"projection_version"`
	Evidence           any        `json:"evidence"`
	ExceptionState     string     `json:"exception_state"`
	ClosingDisposition string     `json:"closing_disposition"`
	OwnerID            *string    `json:"owner_id,omitempty"`
	ReviewerID         *string    `json:"reviewer_id,omitempty"`
	ApproverID         *string    `json:"approver_id,omitempty"`
	OpenedAt           time.Time  `json:"opened_at"`
	LastDetectedAt     time.Time  `json:"last_detected_at"`
	InvestigatingAt    *time.Time `json:"investigating_at,omitempty"`
	ResolvedAt         *time.Time `json:"resolved_at,omitempty"`
	WaivedAt           *time.Time `json:"waived_at,omitempty"`
	ClosedAt           *time.Time `json:"closed_at,omitempty"`
	ResolutionNote     *string    `json:"resolution_note,omitempty"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type ExceptionUpdate struct {
	ExceptionState     string
	ClosingDisposition string
	OwnerID            *string
	ReviewerID         *string
	ApproverID         *string
	ResolutionNote     string
	InvestigatingAt    *time.Time
	ResolvedAt         *time.Time
	WaivedAt           *time.Time
	ClosedAt           *time.Time
}

type Action string

const (
	ActionAssign               Action = "assign"
	ActionVerifyResolution     Action = "verify_resolution"
	ActionAccountingConclusion Action = "accounting_conclusion"
	ActionPeriodWaiver         Action = "period_waiver"
	ActionStandingWaiver       Action = "standing_waiver"
	ActionClose                Action = "close"
)

type ActionCommand struct {
	ExceptionID string
	Action      Action
	ActorID     string
	OwnerID     string
	Note        string
	Now         time.Time
}

type Repository interface {
	controlrules.Source
	PersistDetections(ctx context.Context, detections []Detection) ([]Exception, error)
	ListExceptions(ctx context.Context, period, legalEntityID string) ([]Exception, error)
	GetException(ctx context.Context, id string) (*Exception, error)
	UpdateException(ctx context.Context, id string, update ExceptionUpdate) (*Exception, error)
	HasUnresolvedBlocking(ctx context.Context, period, legalEntityID string) (bool, error)
}

type Service struct {
	facts closereadiness.FactsSource
	rates closereadiness.RateSource
	repo  Repository
	now   func() time.Time
}

func NewService(facts closereadiness.FactsSource, rates closereadiness.RateSource, repo Repository) *Service {
	return &Service{facts: facts, rates: rates, repo: repo, now: func() time.Time { return time.Now().UTC() }}
}

type DetectCommand struct {
	AccountingPeriod  string
	LegalEntityID     string
	ProjectionVersion string
	ScopeComplete     bool
	Now               time.Time
}

type DetectionResult struct {
	AccountingPeriod  string      `json:"accounting_period"`
	ProjectionVersion string      `json:"projection_version"`
	DetectedAt        time.Time   `json:"detected_at"`
	DetectionCount    int         `json:"detection_count"`
	Exceptions        []Exception `json:"exceptions"`
}

func (s *Service) Detect(ctx context.Context, cmd DetectCommand) (*DetectionResult, error) {
	if s == nil || s.facts == nil || s.repo == nil {
		return nil, errors.New("close exception service dependencies are required")
	}
	period := strings.TrimSpace(cmd.AccountingPeriod)
	if _, err := time.Parse("2006-01", period); err != nil {
		return nil, errors.New("accounting period must use YYYY-MM format")
	}
	if cmd.ProjectionVersion == "" {
		cmd.ProjectionVersion = ProjectionVersion
	}
	now := cmd.Now
	if now.IsZero() {
		now = s.now()
	}
	facts, err := s.facts.LoadFacts(ctx, period, cmd.LegalEntityID)
	if err != nil {
		return nil, fmt.Errorf("load close exception facts: %w", err)
	}
	globalRate := 0.0
	if s.rates != nil {
		globalRate = s.rates.GetFloat64(ctx, "global_discount_rate", 0)
	}
	periodEnd, _ := time.Parse("2006-01", period)
	periodEnd = periodEnd.AddDate(0, 1, -1)
	rules, err := controlrules.LoadActive(ctx, s.repo, periodEnd)
	if err != nil {
		return nil, err
	}
	readiness := closereadiness.EvaluateWithRules(closereadiness.Input{
		AccountingPeriod: period, ScopeComplete: cmd.ScopeComplete, GlobalDiscountRate: globalRate, EvaluatedAt: now,
	}, facts, rules)

	detections := make([]Detection, 0, len(readiness.Findings))
	for _, finding := range readiness.Findings {
		rule := rules[finding.RuleCode]
		subjectType := finding.SourceKind
		if subjectType == "contract" {
			subjectType = "contract"
		} else if subjectType == "monthly_closing_batch" {
			subjectType = "monthly_closing_batch"
		}
		subjectContractID := finding.ContractID
		fingerprint := Fingerprint(period, cmd.LegalEntityID, finding.RuleCode, subjectType, finding.SourceID)
		detections = append(detections, Detection{
			RuleCode: rule.Code, RuleVersion: rule.Version, Severity: rule.Severity, GateEffect: rule.GateEffect,
			AccountingPeriod: period, LegalEntityID: cmd.LegalEntityID, SubjectType: subjectType, SubjectID: finding.SourceID,
			SubjectContractID: subjectContractID, ProjectionVersion: cmd.ProjectionVersion, Fingerprint: fingerprint,
			Evidence: map[string]any{
				"rule_code": finding.RuleCode, "title": finding.Title, "reason": finding.Reason,
				"remediation": finding.Remediation, "contract_number": finding.ContractNumber,
				"contract_name": finding.ContractName, "source_kind": finding.SourceKind, "source_id": finding.SourceID,
			}, DetectedAt: now,
		})
	}
	exceptions, err := s.repo.PersistDetections(ctx, detections)
	if err != nil {
		return nil, fmt.Errorf("persist close exceptions: %w", err)
	}
	return &DetectionResult{AccountingPeriod: period, ProjectionVersion: cmd.ProjectionVersion, DetectedAt: now, DetectionCount: len(detections), Exceptions: exceptions}, nil
}

func Fingerprint(period, legalEntityID, ruleCode, subjectType, subjectID string) string {
	value := strings.Join([]string{period, legalEntityID, ruleCode, subjectType, subjectID}, "|")
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func (s *Service) List(ctx context.Context, period, legalEntityID string) ([]Exception, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("close exception repository is required")
	}
	if _, err := time.Parse("2006-01", strings.TrimSpace(period)); err != nil {
		return nil, errors.New("accounting period must use YYYY-MM format")
	}
	return s.repo.ListExceptions(ctx, strings.TrimSpace(period), legalEntityID)
}

func (s *Service) ApplyAction(ctx context.Context, cmd ActionCommand) (*Exception, *Exception, error) {
	if s == nil || s.repo == nil {
		return nil, nil, errors.New("close exception repository is required")
	}
	before, err := s.repo.GetException(ctx, cmd.ExceptionID)
	if err != nil {
		return nil, nil, err
	}
	if before == nil {
		return nil, nil, ErrNotFound
	}
	now := cmd.Now
	if now.IsZero() {
		now = s.now()
	}
	note := strings.TrimSpace(cmd.Note)
	if note == "" {
		return before, nil, ErrNoteRequired
	}
	if strings.TrimSpace(cmd.ActorID) == "" {
		return before, nil, ErrRoleSeparation
	}
	update := ExceptionUpdate{
		ExceptionState: before.ExceptionState, ClosingDisposition: before.ClosingDisposition,
		OwnerID: before.OwnerID, ReviewerID: before.ReviewerID, ApproverID: before.ApproverID,
		ResolutionNote: note, InvestigatingAt: before.InvestigatingAt, ResolvedAt: before.ResolvedAt,
		WaivedAt: before.WaivedAt, ClosedAt: before.ClosedAt,
	}
	switch cmd.Action {
	case ActionAssign:
		if before.ExceptionState != StateOpen {
			return before, nil, ErrInvalidTransition
		}
		if strings.TrimSpace(cmd.OwnerID) == "" {
			return before, nil, ErrOwnerRequired
		}
		update.ExceptionState = StateInvestigating
		update.ClosingDisposition = DispositionUnresolved
		update.OwnerID = stringPtr(strings.TrimSpace(cmd.OwnerID))
		update.InvestigatingAt = timePtr(now)
	case ActionVerifyResolution, ActionAccountingConclusion:
		if before.ExceptionState != StateInvestigating || before.OwnerID == nil || *before.OwnerID == cmd.ActorID {
			return before, nil, ErrInvalidTransition
		}
		update.ExceptionState = StateResolved
		if cmd.Action == ActionVerifyResolution {
			update.ClosingDisposition = DispositionVerifiedResolution
		} else {
			update.ClosingDisposition = DispositionAccountingConclusion
		}
		update.ReviewerID = stringPtr(cmd.ActorID)
		update.ResolvedAt = timePtr(now)
	case ActionPeriodWaiver, ActionStandingWaiver:
		if before.ExceptionState != StateInvestigating || before.OwnerID == nil || *before.OwnerID == cmd.ActorID || (before.ReviewerID != nil && *before.ReviewerID == cmd.ActorID) {
			return before, nil, ErrInvalidTransition
		}
		update.ExceptionState = StateWaived
		if cmd.Action == ActionPeriodWaiver {
			update.ClosingDisposition = DispositionPeriodWaiver
		} else {
			update.ClosingDisposition = DispositionStandingWaiver
		}
		update.ApproverID = stringPtr(cmd.ActorID)
		update.WaivedAt = timePtr(now)
	case ActionClose:
		if (before.ExceptionState != StateResolved && before.ExceptionState != StateWaived) || before.ClosingDisposition == DispositionUnresolved {
			return before, nil, ErrInvalidTransition
		}
		if before.OwnerID != nil && *before.OwnerID == cmd.ActorID || before.ReviewerID != nil && *before.ReviewerID == cmd.ActorID {
			return before, nil, ErrRoleSeparation
		}
		update.ExceptionState = StateClosed
		update.ApproverID = stringPtr(cmd.ActorID)
		update.ClosedAt = timePtr(now)
	default:
		return before, nil, ErrInvalidTransition
	}
	after, err := s.repo.UpdateException(ctx, before.ID, update)
	if err != nil {
		return before, nil, err
	}
	return before, after, nil
}

func stringPtr(value string) *string     { return &value }
func timePtr(value time.Time) *time.Time { return &value }
