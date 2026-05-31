package main

import (
	"flag"
	"fmt"
	"os"

	ifrs16 "github.com/lease-management-system/core-service/internal/services/ifrs16"
)

func main() {
	fixturePath := flag.String("fixture", "internal/services/ifrs16/testdata/ifrs16_regression_cases.json", "path to IFRS 16 regression fixture JSON")
	outputPath := flag.String("out", "", "optional markdown report output path")
	flag.Parse()

	suite, err := ifrs16.LoadRegressionSuite(*fixturePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load fixture: %v\n", err)
		os.Exit(1)
	}

	run := ifrs16.RunRegressionSuite(suite)
	report := ifrs16.RenderRegressionMarkdown(run)

	if *outputPath == "" {
		fmt.Print(report)
	} else if err := os.WriteFile(*outputPath, []byte(report), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "write report: %v\n", err)
		os.Exit(1)
	}

	if run.Failed > 0 {
		os.Exit(1)
	}
}
