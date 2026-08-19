package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/lease-management-system/core-service/internal/agentseval"
	"github.com/lease-management-system/core-service/internal/agentskill"
)

func main() {
	outPath := flag.String("out", "", "optional Markdown report path")
	jsonPath := flag.String("json", "", "optional JSON report path")
	flag.Parse()
	cases, err := agentskill.ProductionEvaluationCases()
	if err != nil {
		fail(err)
	}
	report := agentskill.Evaluate(agentskill.ProductionRegistry(), cases)
	contractReport := agentskill.EvaluateProductionSkillContract()
	invariantCases, err := agentseval.ProductionInvariantCases()
	if err != nil {
		fail(err)
	}
	invariantReport := agentseval.EvaluateInvariantCases(invariantCases)
	markdown := report.Markdown() + "\n" + invariantMarkdown(invariantReport)
	if *outPath != "" {
		if err := os.WriteFile(*outPath, []byte(markdown), 0o644); err != nil {
			fail(err)
		}
	} else {
		fmt.Print(markdown)
	}
	if *jsonPath != "" {
		payload := struct {
			Routing    agentskill.EvaluationReport         `json:"routing"`
			Contract   agentskill.ContractEvaluationReport `json:"skill_contract"`
			Invariants agentseval.InvariantReport          `json:"invariants"`
		}{Routing: report, Contract: contractReport, Invariants: invariantReport}
		raw, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			fail(err)
		}
		if err := os.WriteFile(*jsonPath, append(raw, '\n'), 0o644); err != nil {
			fail(err)
		}
	}
	if report.Failed > 0 || !contractReport.Passed || invariantReport.Failed > 0 {
		if !contractReport.Passed {
			fmt.Fprintf(os.Stderr, "skill contract replay failed: %s\n", contractReport.Error)
		}
		if invariantReport.Failed > 0 {
			fmt.Fprintf(os.Stderr, "invariant evaluation failed: %d of %d cases\n", invariantReport.Failed, invariantReport.Total)
		}
		os.Exit(1)
	}
}

func invariantMarkdown(report agentseval.InvariantReport) string {
	out := fmt.Sprintf("## Invariants (agent-invariants.v1): %d/%d passed\n\n", report.Passed, report.Total)
	for _, r := range report.Results {
		mark := "✅"
		if !r.Passed {
			mark = "❌"
		}
		out += fmt.Sprintf("- %s %s [%s] %s\n", mark, r.CaseID, r.Category, r.Detail)
	}
	return out
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(2)
}
