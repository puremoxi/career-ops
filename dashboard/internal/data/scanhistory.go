package data

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/santifer/career-ops/dashboard/internal/model"
)

// ParseScanRuns reads data/scan-runs.tsv (written by scan.mjs, one row per
// non-dry `node scan.mjs` run) and returns aggregate metrics plus the raw
// rows, newest first. Header-name parsing, never positional — mirrors
// stats.mjs's computeRunStats() so the dashboard and the CLI report the same
// numbers, and so a schema change (a column appended at the end) never
// silently miscounts here.
//
// Returns a zero-value ScanHistoryMetrics (empty Runs slice) when the file
// is missing or has fewer than 2 lines (header only) — never nil, so callers
// don't need a separate not-found branch.
func ParseScanRuns(careerOpsPath string) model.ScanHistoryMetrics {
	filePath := filepath.Join(careerOpsPath, "data", "scan-runs.tsv")
	raw, err := os.ReadFile(filePath)
	if err != nil {
		return model.ScanHistoryMetrics{}
	}

	content := strings.ReplaceAll(string(raw), "\r", "")
	lines := strings.Split(content, "\n")
	// Drop trailing blank lines.
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) < 2 {
		return model.ScanHistoryMetrics{}
	}

	header := strings.Split(lines[0], "\t")
	idx := make(map[string]int, len(header))
	for i, h := range header {
		idx[h] = i
	}
	timestampIdx, hasTimestamp := idx["timestamp"]
	foundIdx, hasFound := idx["found"]
	if !hasTimestamp || !hasFound {
		return model.ScanHistoryMetrics{} // unknown schema
	}

	var filterCols []int
	for name, i := range idx {
		if strings.HasPrefix(name, "filtered_") {
			filterCols = append(filterCols, i)
		}
	}

	numAt := func(cols []string, colIdx int, ok bool) int {
		if !ok || colIdx >= len(cols) {
			return 0
		}
		v, err := strconv.Atoi(strings.TrimSpace(cols[colIdx]))
		if err != nil {
			return 0
		}
		return v
	}
	strAt := func(cols []string, colIdx int, ok bool, def string) string {
		if !ok || colIdx >= len(cols) {
			return def
		}
		return cols[colIdx]
	}

	statusIdx, hasStatus := idx["status"]
	companiesIdx, hasCompanies := idx["companies"]
	boardsIdx, hasBoards := idx["boards"]
	dupesIdx, hasDupes := idx["dupes"]
	newAddedIdx, hasNewAdded := idx["new_added"]
	errorsIdx, hasErrors := idx["errors"]
	durationIdx, hasDuration := idx["duration_ms"]

	var runs []model.ScanRun
	for _, line := range lines[1:] {
		cols := strings.Split(line, "\t")
		if len(cols) < len(header) {
			continue // torn row (e.g. a crash mid-append)
		}
		ts := strAt(cols, timestampIdx, hasTimestamp, "")
		if len(ts) < 4 || ts[:4] < "0001" {
			continue
		}

		filtered := 0
		for _, fi := range filterCols {
			filtered += numAt(cols, fi, true)
		}

		runs = append(runs, model.ScanRun{
			Timestamp:  ts,
			Status:     strAt(cols, statusIdx, hasStatus, "completed"),
			Companies:  numAt(cols, companiesIdx, hasCompanies),
			Boards:     numAt(cols, boardsIdx, hasBoards),
			Found:      numAt(cols, foundIdx, hasFound),
			Filtered:   filtered,
			Dupes:      numAt(cols, dupesIdx, hasDupes),
			NewAdded:   numAt(cols, newAddedIdx, hasNewAdded),
			Errors:     numAt(cols, errorsIdx, hasErrors),
			DurationMs: numAt(cols, durationIdx, hasDuration),
		})
	}

	if len(runs) == 0 {
		return model.ScanHistoryMetrics{}
	}

	// Newest first for display.
	reversed := make([]model.ScanRun, len(runs))
	for i, r := range runs {
		reversed[len(runs)-1-i] = r
	}

	completedCount := 0
	failedCount := 0
	var sumFound, sumNew, sumFiltered, sumDuration int
	durationSamples := 0
	for _, r := range runs {
		if r.Status == "failed" {
			failedCount++
			continue
		}
		completedCount++
		sumFound += r.Found
		sumNew += r.NewAdded
		sumFiltered += r.Filtered
		if r.DurationMs > 0 {
			sumDuration += r.DurationMs
			durationSamples++
		}
	}

	metrics := model.ScanHistoryMetrics{
		Runs:       reversed,
		TotalRuns:  len(runs),
		FailedRuns: failedCount,
	}
	if completedCount > 0 {
		metrics.AvgFoundPerRun = round1(float64(sumFound) / float64(completedCount))
		metrics.AvgNewPerRun = round1(float64(sumNew) / float64(completedCount))
		if sumFound > 0 {
			metrics.FilterRemovalPct = round1(float64(sumFiltered) / float64(sumFound) * 100)
		}
	}
	if durationSamples > 0 {
		metrics.AvgDurationMs = float64(sumDuration) / float64(durationSamples)
	}
	return metrics
}

func round1(v float64) float64 {
	return float64(int(v*10+0.5)) / 10
}
