package parser

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Record represents one benchmark result row.
// Expected CSV header:
// target,concurrency,round,operation,score
//
// Example:
// kata-qemu,1,1,System Call Overhead,120000.5
type Record struct {
	Target      string
	Concurrency int
	Round       int
	Operation   string
	Score       float64
}

func ReadCSV(path string) ([]Record, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = true

	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read csv: %w", err)
	}

	if len(rows) <= 1 {
		return nil, fmt.Errorf("csv has no data rows")
	}

	var records []Record
	for i, row := range rows {
		if i == 0 {
			continue
		}

		record, err := parseRow(i+1, row)
		if err != nil {
			return nil, err
		}

		records = append(records, record)
	}

	return records, nil
}

func parseRow(line int, row []string) (Record, error) {
	if len(row) != 5 {
		return Record{}, fmt.Errorf("line %d: expected 5 columns, got %d", line, len(row))
	}

	concurrency, err := strconv.Atoi(strings.TrimSpace(row[1]))
	if err != nil {
		return Record{}, fmt.Errorf("line %d: invalid concurrency %q: %w", line, row[1], err)
	}

	round, err := strconv.Atoi(strings.TrimSpace(row[2]))
	if err != nil {
		return Record{}, fmt.Errorf("line %d: invalid round %q: %w", line, row[2], err)
	}

	score, err := strconv.ParseFloat(strings.TrimSpace(row[4]), 64)
	if err != nil {
		return Record{}, fmt.Errorf("line %d: invalid score %q: %w", line, row[4], err)
	}

	return Record{
		Target:      strings.TrimSpace(row[0]),
		Concurrency: concurrency,
		Round:       round,
		Operation:   strings.TrimSpace(row[3]),
		Score:       score,
	}, nil
}
