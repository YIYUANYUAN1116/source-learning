package report

import (
	"fmt"
	"strings"

	"kata-bench-analyzer/internal/analyzer"
)

func Markdown(result analyzer.Result) string {
	var b strings.Builder

	b.WriteString("# Kata Bench Analyzer Report\n\n")
	b.WriteString(fmt.Sprintf("Baseline concurrency: `%d`\n\n", result.BaselineConcurrency))
	b.WriteString("> Efficiency = current_avg / ideal_linear_avg. 1.00 means perfect linear scaling.\n\n")

	for _, group := range result.Groups {
		b.WriteString(fmt.Sprintf("## %s / %s\n\n", group.Target, group.Operation))
		b.WriteString("| concurrency | count | min | max | avg | efficiency |\n")
		b.WriteString("| ---: | ---: | ---: | ---: | ---: | ---: |\n")

		for _, item := range group.Items {
			b.WriteString(fmt.Sprintf(
				"| %d | %d | %.2f | %.2f | %.2f | %.2f |\n",
				item.Concurrency,
				item.Count,
				item.Min,
				item.Max,
				item.Avg,
				item.Efficiency,
			))
		}

		b.WriteString("\n")
	}

	return b.String()
}
