package data

import "testing"

func TestParseScanRunsMissingFile(t *testing.T) {
	metrics := ParseScanRuns(t.TempDir())
	if len(metrics.Runs) != 0 || metrics.TotalRuns != 0 {
		t.Fatalf("expected zero-value metrics for missing file, got %+v", metrics)
	}
}

func TestParseScanRunsHeaderOnly(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "data/scan-runs.tsv",
		"timestamp\tstatus\tcompanies\tboards\tfound\tfiltered_title\tfiltered_tier\tfiltered_location\tfiltered_posting_age\tfiltered_salary\tfiltered_content\tfiltered_cooldown\tdupes\tnew_added\terrors\tfiltered_blacklist\tduration_ms\n")
	metrics := ParseScanRuns(root)
	if len(metrics.Runs) != 0 {
		t.Fatalf("expected no runs for header-only file, got %d", len(metrics.Runs))
	}
}

// TestParseScanRunsNewSchema exercises the current 17-column schema
// (post duration_ms), including a failed run that must be excluded from averages.
func TestParseScanRunsNewSchema(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "data/scan-runs.tsv",
		"timestamp\tstatus\tcompanies\tboards\tfound\tfiltered_title\tfiltered_tier\tfiltered_location\tfiltered_posting_age\tfiltered_salary\tfiltered_content\tfiltered_cooldown\tdupes\tnew_added\terrors\tfiltered_blacklist\tduration_ms\n"+
			"2026-07-13T23:07:19.145Z\tcompleted\t113\t0\t8337\t8329\t0\t0\t0\t0\t0\t0\t2\t6\t15\t0\t\n"+
			"2026-07-15T18:58:12.710Z\tcompleted\t121\t0\t12684\t12660\t0\t0\t0\t0\t0\t0\t24\t0\t11\t0\t107271\n"+
			"2026-07-16T09:00:00.000Z\tfailed\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t1\t0\t500\n")

	metrics := ParseScanRuns(root)
	if metrics.TotalRuns != 3 {
		t.Fatalf("expected 3 total runs, got %d", metrics.TotalRuns)
	}
	if metrics.FailedRuns != 1 {
		t.Fatalf("expected 1 failed run, got %d", metrics.FailedRuns)
	}
	// Newest first.
	if metrics.Runs[0].Status != "failed" {
		t.Fatalf("expected newest run (failed) first, got status %q", metrics.Runs[0].Status)
	}
	if metrics.Runs[2].DurationMs != 0 {
		t.Fatalf("expected first historical run to have unknown (0) duration, got %d", metrics.Runs[2].DurationMs)
	}
	if metrics.Runs[1].DurationMs != 107271 {
		t.Fatalf("expected second run duration 107271ms, got %d", metrics.Runs[1].DurationMs)
	}
	// Averages must exclude the failed run: (6+0)/2=3, (8337+12684)/2=10510.5
	if metrics.AvgNewPerRun != 3 {
		t.Fatalf("expected avg new/run 3 (failed run excluded), got %v", metrics.AvgNewPerRun)
	}
	if metrics.AvgFoundPerRun != 10510.5 {
		t.Fatalf("expected avg found/run 10510.5, got %v", metrics.AvgFoundPerRun)
	}
	// Only one completed run has a known duration; the other's is unset (0),
	// so the average must be computed over the 1 known sample only, not both.
	if metrics.AvgDurationMs != 107271 {
		t.Fatalf("expected avg duration 107271 (only 1 known sample), got %v", metrics.AvgDurationMs)
	}
}

// TestParseScanRunsOldSchemaBackwardCompat exercises a file still carrying
// the pre-duration_ms 16-column header (as scan-runs.tsv looked before this
// change) to confirm header-name parsing degrades gracefully — duration is
// simply unknown, nothing crashes or miscounts.
func TestParseScanRunsOldSchemaBackwardCompat(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "data/scan-runs.tsv",
		"timestamp\tstatus\tcompanies\tboards\tfound\tfiltered_title\tfiltered_tier\tfiltered_location\tfiltered_posting_age\tfiltered_salary\tfiltered_content\tfiltered_cooldown\tdupes\tnew_added\terrors\tfiltered_blacklist\n"+
			"2026-07-13T23:07:19.145Z\tcompleted\t113\t0\t8337\t8329\t0\t0\t0\t0\t0\t0\t2\t6\t15\t0\n")

	metrics := ParseScanRuns(root)
	if len(metrics.Runs) != 1 {
		t.Fatalf("expected 1 run from old-schema file, got %d", len(metrics.Runs))
	}
	if metrics.Runs[0].DurationMs != 0 {
		t.Fatalf("expected unknown (0) duration for old-schema row, got %d", metrics.Runs[0].DurationMs)
	}
	if metrics.Runs[0].NewAdded != 6 {
		t.Fatalf("expected new_added=6 still parsed correctly by name, got %d", metrics.Runs[0].NewAdded)
	}
}

// TestParseScanRunsAgentHandoffColumn exercises the current 21-column schema
// (post agent_handoff_companies), including backward compat with a row that
// predates the column.
func TestParseScanRunsAgentHandoffColumn(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "data/scan-runs.tsv",
		"timestamp\tstatus\tcompanies\tboards\tfound\tfiltered_title\tfiltered_tier\tfiltered_location\tfiltered_posting_age\tfiltered_salary\tfiltered_content\tfiltered_cooldown\tdupes\tnew_added\terrors\tfiltered_blacklist\tfiltered_visa\tfiltered_posted_date\tfiltered_country_eligibility\tduration_ms\tagent_handoff_companies\n"+
			// Old row predating the column — trailing cells simply absent.
			"2026-07-27T17:02:29.511Z\tcompleted\t121\t0\t12727\t12703\t0\t0\t0\t0\t0\t0\t24\t0\t11\t0\n"+
			// New row with the full schema, including a real agent-handoff count.
			"2026-08-03T17:49:20.500Z\tcompleted\t1\t0\t397\t397\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t0\t253\t89\n")

	metrics := ParseScanRuns(root)
	if len(metrics.Runs) != 2 {
		t.Fatalf("expected 2 runs, got %d", len(metrics.Runs))
	}

	// Newest first.
	newest := metrics.Runs[0]
	if !newest.HasAgentHandoffData() {
		t.Fatal("expected newest row to have known agent-handoff data")
	}
	if newest.AgentHandoffCompanies != 89 {
		t.Fatalf("expected AgentHandoffCompanies=89, got %d", newest.AgentHandoffCompanies)
	}

	oldest := metrics.Runs[1]
	if oldest.HasAgentHandoffData() {
		t.Fatal("expected old-schema row to report unknown agent-handoff data, not a false 0")
	}
}

// TestParseScanRunsFunnelSums exercises the cumulative run-level funnel
// (TotalFound/TotalFilteredAll/TotalDupes/TotalNewAdded/FilterRemovalPct),
// confirming a failed run is excluded from the sums same as from the averages.
func TestParseScanRunsFunnelSums(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "data/scan-runs.tsv",
		"timestamp\tstatus\tcompanies\tboards\tfound\tfiltered_title\tfiltered_tier\tfiltered_location\tfiltered_posting_age\tfiltered_salary\tfiltered_content\tfiltered_cooldown\tdupes\tnew_added\terrors\tfiltered_blacklist\n"+
			"2026-07-13T23:07:19.145Z\tcompleted\t113\t0\t1000\t900\t10\t5\t0\t0\t0\t0\t20\t6\t15\t5\n"+
			"2026-07-15T18:58:12.710Z\tcompleted\t121\t0\t2000\t1800\t0\t0\t0\t0\t0\t0\t30\t0\t11\t0\n"+
			"2026-07-16T09:00:00.000Z\tfailed\t500\t0\t5000\t5000\t0\t0\t0\t0\t0\t0\t0\t0\t1\t0\n")

	metrics := ParseScanRuns(root)

	// Failed run's 5000 found / 5000 filtered must NOT be folded in.
	wantFound := 1000 + 2000
	wantFiltered := (900 + 10 + 5 + 5) + 1800
	wantDupes := 20 + 30
	wantNew := 6 + 0

	if metrics.TotalFound != wantFound {
		t.Errorf("expected TotalFound=%d, got %d", wantFound, metrics.TotalFound)
	}
	if metrics.TotalFilteredAll != wantFiltered {
		t.Errorf("expected TotalFilteredAll=%d, got %d", wantFiltered, metrics.TotalFilteredAll)
	}
	if metrics.TotalDupes != wantDupes {
		t.Errorf("expected TotalDupes=%d, got %d", wantDupes, metrics.TotalDupes)
	}
	if metrics.TotalNewAdded != wantNew {
		t.Errorf("expected TotalNewAdded=%d, got %d", wantNew, metrics.TotalNewAdded)
	}
	wantPct := float64(wantFiltered) / float64(wantFound) * 100
	if diff := metrics.FilterRemovalPct - wantPct; diff > 0.001 || diff < -0.001 {
		t.Errorf("expected FilterRemovalPct=%.4f, got %.4f", wantPct, metrics.FilterRemovalPct)
	}
}

func TestParseScanRunsTornRowSkipped(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "data/scan-runs.tsv",
		"timestamp\tstatus\tcompanies\tboards\tfound\tfiltered_title\tfiltered_tier\tfiltered_location\tfiltered_posting_age\tfiltered_salary\tfiltered_content\tfiltered_cooldown\tdupes\tnew_added\terrors\tfiltered_blacklist\tduration_ms\n"+
			"2026-07-13T23:07:19.145Z\tcompleted\t113\t0\t8337\n"+ // torn (crash mid-append)
			"2026-07-15T18:58:12.710Z\tcompleted\t121\t0\t12684\t12660\t0\t0\t0\t0\t0\t0\t24\t0\t11\t0\t107271\n")

	metrics := ParseScanRuns(root)
	if len(metrics.Runs) != 1 {
		t.Fatalf("expected torn row skipped, 1 valid run remaining, got %d", len(metrics.Runs))
	}
}
