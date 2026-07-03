// Package aiintake owns the versioned cross-process seam between AI draft
// production and the core system. Callers receive typed drafts only after the
// schema, evidence, confidence, and review gate satisfy the contract.
package aiintake

const SchemaVersion = "ai-intake.v1"

type EvidenceLocator struct {
	Field       string    `json:"field"`
	Source      string    `json:"source"`
	Page        *int      `json:"page"`
	Coordinates []float64 `json:"coordinates"`
	Quote       string    `json:"quote"`
}

type Evidence struct {
	SourceFileID  string            `json:"source_file_id"`
	ObjectName    string            `json:"object_name"`
	ContentType   string            `json:"content_type"`
	Locators      []EvidenceLocator `json:"locators"`
	Complete      bool              `json:"complete"`
	MissingReason string            `json:"missing_reason"`
}

type ReviewGate struct {
	Required            bool     `json:"required"`
	Reasons             []string `json:"reasons"`
	ConfidenceThreshold float64  `json:"confidence_threshold"`
}

type PaymentSchedule struct {
	PeriodStart      string  `json:"period_start"`
	PeriodEnd        string  `json:"period_end"`
	DueDate          string  `json:"due_date"`
	Amount           float64 `json:"amount"`
	PaymentTiming    string  `json:"payment_timing"`
	IsFixed          bool    `json:"is_fixed"`
	IsLeaseComponent bool    `json:"is_lease_component"`
	AmountType       string  `json:"amount_type"`
	Currency         string  `json:"currency"`
	Confidence       float64 `json:"confidence"`
}

type PaymentScheduleDraft struct {
	SchemaVersion             string             `json:"schema_version"`
	IntakeID                  string             `json:"intake_id"`
	TaskID                    string             `json:"task_id"`
	FileID                    string             `json:"file_id"`
	Mode                      string             `json:"mode"`
	DraftType                 string             `json:"draft_type"`
	Status                    string             `json:"status"`
	Schedules                 []PaymentSchedule  `json:"schedules"`
	ConfidenceScores          map[string]float64 `json:"confidence_scores"`
	MissingFields             []string           `json:"missing_fields"`
	Warnings                  []string           `json:"warnings"`
	RequiresHumanConfirmation bool               `json:"requires_human_confirmation"`
	Evidence                  Evidence           `json:"evidence"`
	ReviewGate                ReviewGate         `json:"review_gate"`
}
