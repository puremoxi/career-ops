package data

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// ScanRun is one row from data/scan-runs.tsv, appended by scan.mjs after
// every scan (see AGENTS.md's Main Files table). Field names mirror the
// TSV's header-name columns, resolved by name (not position) so an older
// file missing a later-added column — duration_ms was added after the
// initial 16-column schema — degrades gracefully instead of misreading.
type ScanRun struct {
	Timestamp          string
	Status             string
	Companies          int
	Boards             int
	Found              int
	FilteredTitle      int
	FilteredTier       int
	FilteredLocation   int
	FilteredPostingAge int
	FilteredSalary     int
	FilteredContent    int
	FilteredCooldown   int
	Dupes              int
	NewAdded           int
	Errors             int
	FilteredBlacklist  int
	DurationMs         int64 // 0 when absent/unknown (older schema, or an empty cell)
	durationKnown      bool  // unexported: "actually 0ms" vs "column absent/empty" for averaging

	// AgentHandoffCompanies is how many tracked_companies scan.mjs could NOT
	// resolve to a zero-token provider — the companies that need a Level 1-3
	// (Playwright/WebSearch) agent pass, printed under "Agent/WebSearch
	// handoff" in scan.mjs's own output. Companies above is the disjoint
	// zero-token-resolved count — the two are NOT companies-minus-covered;
	// scan.mjs never counts agent-handoff companies in Companies to begin
	// with, so subtracting one from the other always yields ~0. This field
	// exists because that subtraction is wrong, not redundant.
	AgentHandoffCompanies int
	agentHandoffKnown     bool // unexported: absent column (pre-agent_handoff_companies schema) vs. a genuine 0
}

// HasAgentHandoffData reports whether this row's agent_handoff_companies
// column was present and parseable — false for rows written before that
// column existed, so callers can distinguish "0 companies needed an agent
// pass" from "this run predates the column."
func (r ScanRun) HasAgentHandoffData() bool {
	return r.agentHandoffKnown
}

// ScanRunMetrics is the aggregated view ParseScanRuns returns: individual
// runs (newest first) plus summary averages. Averages are computed over
// completed runs only — a failed run's 0 companies/jobs/duration would
// understate every other run's typical performance if folded in (same
// reasoning as funnel-velocity.mjs excluding non-comparable outcomes from
// its stage-velocity medians).
type ScanRunMetrics struct {
	Runs           []ScanRun
	TotalRuns      int
	FailedRuns     int
	AvgNewPerRun   float64
	AvgFoundPerRun float64
	AvgDurationMs  float64 // averaged only over completed runs with a known duration_ms

	// Cumulative funnel across completed runs — how the raw jobs-found volume
	// gets whittled down to new pipeline additions. Mirrors stats.mjs's
	// computeRunStats filterRemovalPct, extended with the intermediate counts
	// so the dashboard can render each stage rather than just the final ratio.
	TotalFound       int
	TotalFilteredAll int // sum of every filtered_* column (title/tier/location/posting_age/salary/content/cooldown/blacklist)
	TotalDupes       int
	TotalNewAdded    int
	FilterRemovalPct float64 // TotalFilteredAll / TotalFound, 0 when TotalFound is 0
}

// requiredScanRunColumns are the columns every valid row must have, by name.
// duration_ms is deliberately excluded — the pre-duration_ms schema never had
// it, and that's a supported older file, not a torn row.
var requiredScanRunColumns = []string{
	"timestamp", "status", "companies", "boards", "found",
	"filtered_title", "filtered_tier", "filtered_location", "filtered_posting_age",
	"filtered_salary", "filtered_content", "filtered_cooldown", "dupes",
	"new_added", "errors", "filtered_blacklist",
}

// ParseScanRuns reads data/scan-runs.tsv under careerOpsPath. A missing file
// is not an error (same convention as loadSalaryObservations / LoadPDFManifest)
// and returns a zero-value ScanRunMetrics.
func ParseScanRuns(careerOpsPath string) ScanRunMetrics {
	raw, err := os.ReadFile(filepath.Join(careerOpsPath, "data", "scan-runs.tsv"))
	if err != nil {
		return ScanRunMetrics{}
	}

	lines := strings.Split(string(raw), "\n")
	if len(lines) == 0 {
		return ScanRunMetrics{}
	}

	header := strings.Split(strings.TrimRight(lines[0], "\r"), "\t")
	colIdx := make(map[string]int, len(header))
	for i, name := range header {
		colIdx[strings.TrimSpace(name)] = i
	}

	get := func(fields []string, name string) (string, bool) {
		idx, ok := colIdx[name]
		if !ok || idx >= len(fields) {
			return "", false
		}
		return strings.TrimSpace(fields[idx]), true
	}
	getInt := func(fields []string, name string) int {
		v, ok := get(fields, name)
		if !ok || v == "" {
			return 0
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			return 0
		}
		return n
	}

	var runs []ScanRun
	for _, line := range lines[1:] {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")

		// Torn row (a crash mid-append can leave a short trailing line):
		// missing any required column for the file's own header. Skip it
		// entirely rather than parsing partial/zeroed data as a real run.
		complete := true
		for _, name := range requiredScanRunColumns {
			if _, ok := get(fields, name); !ok {
				complete = false
				break
			}
		}
		if !complete {
			continue
		}

		ts, _ := get(fields, "timestamp")
		status, _ := get(fields, "status")
		run := ScanRun{
			Timestamp:          ts,
			Status:             status,
			Companies:          getInt(fields, "companies"),
			Boards:             getInt(fields, "boards"),
			Found:              getInt(fields, "found"),
			FilteredTitle:      getInt(fields, "filtered_title"),
			FilteredTier:       getInt(fields, "filtered_tier"),
			FilteredLocation:   getInt(fields, "filtered_location"),
			FilteredPostingAge: getInt(fields, "filtered_posting_age"),
			FilteredSalary:     getInt(fields, "filtered_salary"),
			FilteredContent:    getInt(fields, "filtered_content"),
			FilteredCooldown:   getInt(fields, "filtered_cooldown"),
			Dupes:              getInt(fields, "dupes"),
			NewAdded:           getInt(fields, "new_added"),
			Errors:             getInt(fields, "errors"),
			FilteredBlacklist:  getInt(fields, "filtered_blacklist"),
		}
		if v, ok := get(fields, "duration_ms"); ok && v != "" {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				run.DurationMs = n
				run.durationKnown = true
			}
		}
		if v, ok := get(fields, "agent_handoff_companies"); ok && v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				run.AgentHandoffCompanies = n
				run.agentHandoffKnown = true
			}
		}
		runs = append(runs, run)
	}

	// Newest first — ISO 8601 timestamps sort lexicographically.
	sort.SliceStable(runs, func(i, j int) bool {
		return runs[i].Timestamp > runs[j].Timestamp
	})

	metrics := ScanRunMetrics{Runs: runs, TotalRuns: len(runs)}

	var sumNew, sumFound, sumDuration float64
	var completedCount, durationCount int
	for _, r := range runs {
		if r.Status == "failed" {
			metrics.FailedRuns++
			continue
		}
		completedCount++
		sumNew += float64(r.NewAdded)
		sumFound += float64(r.Found)
		if r.durationKnown {
			sumDuration += float64(r.DurationMs)
			durationCount++
		}

		metrics.TotalFound += r.Found
		metrics.TotalDupes += r.Dupes
		metrics.TotalNewAdded += r.NewAdded
		metrics.TotalFilteredAll += r.FilteredTitle + r.FilteredTier + r.FilteredLocation +
			r.FilteredPostingAge + r.FilteredSalary + r.FilteredContent + r.FilteredCooldown + r.FilteredBlacklist
	}
	if completedCount > 0 {
		metrics.AvgNewPerRun = sumNew / float64(completedCount)
		metrics.AvgFoundPerRun = sumFound / float64(completedCount)
	}
	if durationCount > 0 {
		metrics.AvgDurationMs = sumDuration / float64(durationCount)
	}
	if metrics.TotalFound > 0 {
		metrics.FilterRemovalPct = float64(metrics.TotalFilteredAll) / float64(metrics.TotalFound) * 100
	}

	return metrics
}
