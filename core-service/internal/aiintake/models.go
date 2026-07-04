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

type IntakeMetadata struct {
	SchemaVersion             string             `json:"schema_version"`
	IntakeID                  string             `json:"intake_id"`
	TaskID                    string             `json:"task_id"`
	FileID                    string             `json:"file_id"`
	Mode                      string             `json:"mode"`
	DraftType                 string             `json:"draft_type"`
	Status                    string             `json:"status"`
	ConfidenceScores          map[string]float64 `json:"confidence_scores"`
	MissingFields             []string           `json:"missing_fields"`
	Warnings                  []string           `json:"warnings"`
	RequiresHumanConfirmation bool               `json:"requires_human_confirmation"`
	Evidence                  Evidence           `json:"evidence"`
	ReviewGate                ReviewGate         `json:"review_gate"`
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
	IntakeMetadata
	Schedules []PaymentSchedule `json:"schedules"`
}

type ContractDraftData struct {
	ContractNumber    string   `json:"contract_number"`
	ContractName      string   `json:"contract_name"`
	Lessee            string   `json:"lessee"`
	Lessor            string   `json:"lessor"`
	StoreName         string   `json:"store_name"`
	StoreAddress      string   `json:"store_address"`
	CommencementDate  string   `json:"commencement_date"`
	LeaseStartDate    string   `json:"lease_start_date"`
	LeaseEndDate      string   `json:"lease_end_date"`
	Currency          string   `json:"currency"`
	AssetType         string   `json:"asset_type"`
	FixedRentAmount   float64  `json:"fixed_rent_amount"`
	PaymentFrequency  string   `json:"payment_frequency"`
	PaymentTiming     string   `json:"payment_timing"`
	RenewalOption     bool     `json:"renewal_option"`
	TerminationOption bool     `json:"termination_option"`
	CAMAmount         float64  `json:"cam_amount"`
	ServiceFee        float64  `json:"service_fee"`
	DiscountRateType  string   `json:"discount_rate_type"`
	DiscountRate      float64  `json:"discount_rate"`
	IsLease           bool     `json:"is_lease"`
	LeaseScope        string   `json:"lease_scope"`
	SuggestedScope    string   `json:"suggested_scope"`
	ExemptionReason   string   `json:"exemption_reason"`
	ScopeSource       string   `json:"scope_source"`
	ScopeConfidence   float64  `json:"scope_confidence"`
	Confidence        float64  `json:"confidence"`
	MissingFields     []string `json:"missing_fields"`
	Warnings          []string `json:"warnings"`
}

type ContractDraft struct {
	IntakeMetadata
	ExtractedData ContractDraftData `json:"extracted_data"`
}

type ContractBatchDraft struct {
	IntakeMetadata
	Contracts  []ContractDraftData `json:"contracts"`
	TotalCount int                 `json:"total_count"`
}
