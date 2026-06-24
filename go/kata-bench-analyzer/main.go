package main

import (
	"flag"
	"fmt"
	"os"

	"kata-bench-analyzer/internal/analyzer"
	"kata-bench-analyzer/internal/parser"
	"kata-bench-analyzer/internal/report"
)

func main() {
	input := flag.String("input", "testdata/unixbench.csv", "benchmark csv input file")
	output := flag.String("output", "", "markdown report output file, empty means stdout")
	baseline := flag.Int("baseline", 1, "baseline concurrency used to calculate scaling efficiency")
	flag.Parse()

	records, err := parser.ReadCSV(*input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read csv failed: %v\n", err)
		os.Exit(1)
	}

	result := analyzer.Analyze(records, *baseline)
	markdown := report.Markdown(result)

	if *output == "" {
		fmt.Print(markdown)
		return
	}

	if err := os.WriteFile(*output, []byte(markdown), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "write report failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("report generated: %s\n", *output)
}
