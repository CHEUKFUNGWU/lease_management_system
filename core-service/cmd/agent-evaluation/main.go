package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

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
	markdown := report.Markdown()
	if *outPath != "" {
		if err := os.WriteFile(*outPath, []byte(markdown), 0o644); err != nil {
			fail(err)
		}
	} else {
		fmt.Print(markdown)
	}
	if *jsonPath != "" {
		payload := struct {
			Routing  agentskill.EvaluationReport         `json:"routing"`
			Contract agentskill.ContractEvaluationReport `json:"skill_contract"`
		}{Routing: report, Contract: contractReport}
		raw, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			fail(err)
		}
		if err := os.WriteFile(*jsonPath, append(raw, '\n'), 0o644); err != nil {
			fail(err)
		}
	}
	if report.Failed > 0 || !contractReport.Passed {
		if !contractReport.Passed {
			fmt.Fprintf(os.Stderr, "skill contract replay failed: %s\n", contractReport.Error)
		}
		os.Exit(1)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(2)
}
