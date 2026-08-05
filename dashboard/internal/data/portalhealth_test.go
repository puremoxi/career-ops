package data

import "testing"

func TestParsePortalHealthMissingFile(t *testing.T) {
	m := ParsePortalHealth(t.TempDir())
	if len(m.Runs) != 0 || m.LatestTotal != 0 {
		t.Fatalf("expected zero-value metrics for missing file, got %+v", m)
	}
}

func TestParsePortalHealthHeaderOnly(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "data/portal-health.tsv", "timestamp\tcompany\tstatus\n")
	m := ParsePortalHealth(root)
	if len(m.Runs) != 0 {
		t.Fatalf("expected no runs for header-only file, got %d", len(m.Runs))
	}
}

func TestParsePortalHealthLatestRunAndStreaks(t *testing.T) {
	root := t.TempDir()
	// Two runs. Acme is slug_gone in both -> streak 2 (below default threshold 3).
	// Beta is slug_gone in both plus a third run -> streak 3, persistently dead.
	writeFixture(t, root, "data/portal-health.tsv",
		"timestamp\tcompany\tstatus\n"+
			"2026-07-28T19:25:06.949Z\tAcme\treachable\n"+
			"2026-07-28T19:25:06.949Z\tBeta\tslug_gone\n"+
			"2026-07-28T19:25:06.949Z\tGamma\tempty\n"+
			"2026-07-29T18:19:21.658Z\tAcme\tslug_gone\n"+
			"2026-07-29T18:19:21.658Z\tBeta\tslug_gone\n"+
			"2026-07-29T18:19:21.658Z\tGamma\treachable\n"+
			"2026-08-03T16:54:58.628Z\tAcme\tslug_gone\n"+
			"2026-08-03T16:54:58.628Z\tBeta\tslug_gone\n"+
			"2026-08-03T16:54:58.628Z\tGamma\tnetwork\n")

	m := ParsePortalHealth(root)

	if len(m.Runs) != 3 {
		t.Fatalf("expected 3 distinct runs, got %d: %+v", len(m.Runs), m.Runs)
	}
	// Newest first.
	if m.Runs[0].RunKey != "2026-08-03T16:54" {
		t.Fatalf("expected newest run first, got %s", m.Runs[0].RunKey)
	}

	if m.LatestTotal != 3 {
		t.Fatalf("expected latest run to have 3 companies checked, got %d", m.LatestTotal)
	}
	if m.LatestReachable != 0 || m.LatestEmpty != 0 || m.LatestErrored != 3 {
		t.Fatalf("expected latest run all errored, got reachable=%d empty=%d errored=%d",
			m.LatestReachable, m.LatestEmpty, m.LatestErrored)
	}

	// Acme: reachable, slug_gone, slug_gone -> streak 2 -> not persistently dead.
	// Beta: slug_gone x3 -> streak 3 -> persistently dead.
	// Gamma: empty, reachable, network -> streak 1 -> not persistently dead.
	if m.PersistentlyDeadCount != 1 {
		t.Fatalf("expected exactly 1 persistently dead company, got %d: %v", m.PersistentlyDeadCount, m.PersistentlyDead)
	}
	if len(m.PersistentlyDead) != 1 || m.PersistentlyDead[0] != "Beta" {
		t.Fatalf("expected Beta persistently dead, got %v", m.PersistentlyDead)
	}
}

func TestParsePortalHealthTornRowSkipped(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "data/portal-health.tsv",
		"timestamp\tcompany\tstatus\n"+
			"2026-07-28T19:25:06.949Z\tAcme\n"+ // torn: only 2 fields
			"2026-07-28T19:25:06.949Z\tBeta\treachable\n")

	m := ParsePortalHealth(root)
	if m.LatestTotal != 1 {
		t.Fatalf("expected torn row skipped, leaving 1 valid record, got %d", m.LatestTotal)
	}
}
