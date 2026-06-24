package analyzer

import (
	"testing"

	"kata-bench-analyzer/internal/parser"
)

func TestAnalyzeEfficiency(t *testing.T) {
	records := []parser.Record{
		{Target: "kata", Concurrency: 1, Round: 1, Operation: "syscall", Score: 100},
		{Target: "kata", Concurrency: 1, Round: 2, Operation: "syscall", Score: 110},
		{Target: "kata", Concurrency: 2, Round: 1, Operation: "syscall", Score: 160},
		{Target: "kata", Concurrency: 2, Round: 2, Operation: "syscall", Score: 180},
	}

	result := Analyze(records, 1)
	if len(result.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(result.Groups))
	}

	items := result.Groups[0].Items
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	if items[0].Avg != 105 {
		t.Fatalf("expected baseline avg 105, got %.2f", items[0].Avg)
	}

	// concurrency=2 avg=170, ideal linear avg=105*2=210, efficiency=170/210=0.8095
	if items[1].Efficiency < 0.80 || items[1].Efficiency > 0.82 {
		t.Fatalf("expected efficiency around 0.81, got %.4f", items[1].Efficiency)
	}
}
