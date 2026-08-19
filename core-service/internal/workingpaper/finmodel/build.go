// Package finmodel is SM6: the working-paper builder over one persisted
// model run. Every cell is a 1:1 pass-through of a RunResult LineValue —
// the builder holds no formula (the保值 assertions lock this). Tie-outs
// render as a check section; a failed tie-out flags the paper twice (cover
// and section) — the lint gate at export is the second fail-closed point
// (D-S5).
package finmodel

import (
	"errors"
	"fmt"
	"strings"

	"github.com/lease-management-system/core-service/internal/workingpaper"
)

// Input is one persisted run plus the cover metadata the paper must state.
type Input struct {
	Title              string `json:"title"`
	LegalEntityID      string `json:"legal_entity_id"`
	Currency           string `json:"currency"`
	DataClassification string `json:"data_classification"`

	// Five version lines (cover).
	ModelDefinitionVersion  string `json:"model_definition_version"`
	TemplateVersion         string `json:"template_version"`
	DataVersion             string `json:"data_version"`
	AssumptionVersion       string `json:"assumption_version"`
	ExchangeRateVersion     string `json:"exchange_rate_version"`
	MetricDefinitionVersion string `json:"metric_definition_version"`

	Periods    []string      `json:"periods"`
	Lines      []LineValue   `json:"lines"`
	TieOuts    []TieOutValue `json:"tie_outs"`
	GapDetails []string      `json:"gap_details,omitempty"`
	// ToolCallID anchors certified cells to the generating tool call (I2).
	ToolCallID  string `json:"tool_call_id"`
	GeneratedBy string `json:"generated_by,omitempty"`
}

// LineValue mirrors the engine result (Value nil = missing — never zero).
type LineValue struct {
	RowKey         string   `json:"row_key"`
	Label          string   `json:"label"`
	Period         string   `json:"period"`
	Value          *float64 `json:"value"`
	SourceType     string   `json:"source_type"` // fact_aggregate | ifrs16_engine | contract_schedule | assumption | formula | opening_balance
	Classification string   `json:"classification,omitempty"`
}

// TieOutValue mirrors one tie-out result.
type TieOutValue struct {
	CheckCode string   `json:"check_code"`
	Period    string   `json:"period"`
	Expected  *float64 `json:"expected"`
	Actual    *float64 `json:"actual"`
	Diff      *float64 `json:"diff"`
	Status    string   `json:"status"`
}

// cellBasis maps the engine source type to the working-paper basis class.
func cellBasis(sourceType string) workingpaper.Basis {
	switch sourceType {
	case "fact_aggregate":
		return workingpaper.BasisSystemFact
	case "assumption":
		return workingpaper.BasisHumanInput
	default:
		return workingpaper.BasisCertified // ifrs16_engine | contract_schedule | formula | opening_balance
	}
}

// Build assembles the paper. It fails only on contract breaks (missing
// periods, empty anchor); data gaps are recorded, never covered.
func Build(in Input) (workingpaper.Paper, error) {
	if strings.TrimSpace(in.ToolCallID) == "" {
		return workingpaper.Paper{}, errors.New("finmodel: tool_call_id is required for certified provenance (I2)")
	}
	if len(in.Periods) == 0 {
		return workingpaper.Paper{}, errors.New("finmodel: at least one period is required")
	}

	byPeriod := map[string]bool{}
	for _, p := range in.Periods {
		byPeriod[p] = true
	}

	var cells []workingpaper.Cell
	var failedTieOuts []string
	for _, line := range in.Lines {
		if line.Value == nil {
			continue // 缺失跳格，永不填 0（D-S4）
		}
		basis := cellBasis(line.SourceType)
		prov := workingpaper.Provenance{
			Basis:         basis,
			ToolCallID:    in.ToolCallID,
			EngineVersion: "finmodel@" + in.ModelDefinitionVersion,
			DataVersion:   in.DataVersion,
		}
		if basis == workingpaper.BasisSystemFact {
			prov.SourceTable = "retail_store_day_facts"
			prov.SourceRecordID = line.RowKey + "@" + line.Period
		}
		if basis == workingpaper.BasisHumanInput {
			prov.ConfirmedBy = in.GeneratedBy
		}
		cells = append(cells, workingpaper.Cell{
			Ref: line.RowKey + "@" + line.Period, Label: line.Label + "（" + line.Period + "）",
			Value: *line.Value, Currency: in.Currency,
			Provenance: prov,
		})
	}

	checkCells, failed := tieOutCells(in)
	failedTieOuts = append(failedTieOuts, failed...)

	section := workingpaper.Section{
		ID: "model", Title: "三表模型运行结果", Kind: workingpaper.KindTable, Cells: cells,
	}
	checkSection := workingpaper.Section{
		ID: "tie_outs", Title: "勾稽校验（T1–T16）", Kind: workingpaper.KindTable, Cells: checkCells,
	}
	if len(failedTieOuts) > 0 {
		checkSection.Narrative = "存在勾稽失败：" + strings.Join(failedTieOuts, "；") + "。该 run 不得发布（fail-closed）。"
	}

	gaps := append([]string(nil), in.GapDetails...)
	if in.DataClassification == "simulated" {
		gaps = append(gaps, "数据标记为模拟（SIMULATED）：模型结果不得用作正式结论")
	}

	review := workingpaper.ReviewNeedsReview
	for _, out := range in.TieOuts {
		if out.Status == "failed" {
			review = workingpaper.ReviewNeedsReview
		}
	}

	return workingpaper.Paper{
		Title:                   in.Title,
		Period:                  strings.Join(in.Periods, " ~ "),
		LegalEntityScope:        in.LegalEntityID,
		ReviewState:             review,
		DataVersion:             in.DataVersion,
		AssumptionVersion:       in.AssumptionVersion,
		EngineVersion:           "finmodel@" + in.ModelDefinitionVersion,
		ExchangeRateVersion:     in.ExchangeRateVersion,
		MetricDefinitionVersion: in.MetricDefinitionVersion,
		TemplateVersion:         in.TemplateVersion,
		GeneratedBy:             in.GeneratedBy,
		DataGaps:                gaps,
		UnexplainedResidual:     "",
		OpenQuestions:           []string{"IFRS 16 租赁附表数字均来自计量引擎投影（模型不自算，D-S3）"},
		Sections:                []workingpaper.Section{section, checkSection},
	}, nil
}

func tieOutCells(in Input) ([]workingpaper.Cell, []string) {
	var cells []workingpaper.Cell
	var failed []string
	for _, out := range in.TieOuts {
		value := out.Diff
		status := workingpaper.BasisSystemFact
		label := fmt.Sprintf("%s @ %s（%s）", out.CheckCode, out.Period, out.Status)
		if value == nil {
			status = workingpaper.BasisSystemFact
		}
		v := 0.0
		if value != nil {
			v = *value
		}
		cells = append(cells, workingpaper.Cell{
			Ref: out.CheckCode + "@" + out.Period, Label: label,
			Value: v, Provenance: workingpaper.Provenance{Basis: status, SourceTable: "fin_model_tie_outs", DataVersion: in.DataVersion},
		})
		if out.Status == "failed" {
			failed = append(failed, out.CheckCode+"@"+out.Period)
		}
	}
	return cells, failed
}
