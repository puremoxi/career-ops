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
