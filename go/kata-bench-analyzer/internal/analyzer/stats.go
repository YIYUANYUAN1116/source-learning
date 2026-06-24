package analyzer

import (
	"sort"

	"kata-bench-analyzer/internal/parser"
)

type Result struct {
	BaselineConcurrency int
	Groups              []Group
}

type Group struct {
	Target    string
	Operation string
	Items     []Item
}

type Item struct {
	Concurrency int
	Count       int
	Min         float64
	Max         float64
	Avg         float64
	Efficiency  float64
}

type bucket struct {
	count int
	sum   float64
	min   float64
	max   float64
}

func Analyze(records []parser.Record, baselineConcurrency int) Result {
	if baselineConcurrency <= 0 {
		baselineConcurrency = 1
	}

	buckets := make(map[string]map[int]*bucket)
	meta := make(map[string]Group)

	for _, r := range records {
		key := r.Target + "\x00" + r.Operation
		if _, ok := buckets[key]; !ok {
			buckets[key] = make(map[int]*bucket)
			meta[key] = Group{Target: r.Target, Operation: r.Operation}
		}

		b, ok := buckets[key][r.Concurrency]
		if !ok {
			b = &bucket{min: r.Score, max: r.Score}
			buckets[key][r.Concurrency] = b
		}

		b.count++
		b.sum += r.Score
		if r.Score < b.min {
			b.min = r.Score
		}
		if r.Score > b.max {
			b.max = r.Score
		}
	}

	keys := make([]string, 0, len(buckets))
	for key := range buckets {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	groups := make([]Group, 0, len(keys))
	for _, key := range keys {
		group := meta[key]
		byConcurrency := buckets[key]

		concurrencyList := make([]int, 0, len(byConcurrency))
		for concurrency := range byConcurrency {
			concurrencyList = append(concurrencyList, concurrency)
		}
		sort.Ints(concurrencyList)

		baselineAvg := 0.0
		if b, ok := byConcurrency[baselineConcurrency]; ok && b.count > 0 {
			baselineAvg = b.sum / float64(b.count)
		}

		for _, concurrency := range concurrencyList {
			b := byConcurrency[concurrency]
			avg := b.sum / float64(b.count)
			efficiency := 0.0
			if baselineAvg > 0 && concurrency > 0 {
				efficiency = avg / (baselineAvg * float64(concurrency) / float64(baselineConcurrency))
			}

			group.Items = append(group.Items, Item{
				Concurrency: concurrency,
				Count:       b.count,
				Min:         b.min,
				Max:         b.max,
				Avg:         avg,
				Efficiency:  efficiency,
			})
		}

		groups = append(groups, group)
	}

	return Result{
		BaselineConcurrency: baselineConcurrency,
		Groups:              groups,
	}
}
