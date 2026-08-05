package data

import "testing"

func TestParseScanHistoryMissingFile(t *testing.T) {
	m := ParseScanHistory(t.TempDir())
	if m.TotalRows != 0 {
		t.Fatalf("expected zero-value metrics for missing file, got %+v", m)
	}
}

func TestParseScanHistoryHeaderOnly(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "data/scan-history.tsv", "url\tfirst_seen\tportal\ttitle\tcompany\tstatus\tlocation\n")
	m := ParseScanHistory(root)
	if m.TotalRows != 0 {
		t.Fatalf("expected no rows for header-only file, got %d", m.TotalRows)
	}
}

func TestParseScanHistoryAggregates(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "data/scan-history.tsv",
		"url\tfirst_seen\tportal\ttitle\tcompany\tstatus\tlocation\tjd_fingerprint\tpostedAt\n"+
			"https://a.example/1\t2026-07-13\tgreenhouse-api\tCreative Director\tAcme\tadded\tRemote\t\t2026-07-10\n"+
			"https://a.example/2\t2026-07-14\tgreenhouse-api\tCreative Lead\tAcme\tskipped_dup\tRemote\t\t\n"+
			"https://b.example/1\t2026-07-15\tashby-api\tVFX Supervisor\tBeta\tadded\tLondon\t\t\n"+
			"https://c.example/1\t2026-07-16\tAshby — Creative Director\tArt Director\tGamma\tskipped_title\t\t\t\n"+
			"https://d.example/1\t2026-07-17\tashby-api\tHead of Creative\tDelta\tskipped_expired\tNYC\t\t\n")

	m := ParseScanHistory(root)

	if m.TotalRows != 5 {
		t.Fatalf("expected 5 rows, got %d", m.TotalRows)
	}
	if m.AddedCount != 2 {
		t.Fatalf("expected 2 added, got %d", m.AddedCount)
	}
	if m.SkippedDupCount != 1 || m.SkippedTitleCount != 1 || m.SkippedExpiredCount != 1 {
		t.Fatalf("unexpected skip counts: %+v", m)
	}
	if m.CompaniesSeen != 4 {
		t.Fatalf("expected 4 distinct companies, got %d", m.CompaniesSeen)
	}
	if m.CompaniesMatched != 2 {
		t.Fatalf("expected 2 matched companies (Acme, Beta), got %d", m.CompaniesMatched)
	}
	if got := m.CompaniesZeroYield(); got != 2 {
		t.Fatalf("expected 2 zero-yield companies (Gamma, Delta), got %d", got)
	}
	if m.EarliestFirstSeen != "2026-07-13" || m.LatestFirstSeen != "2026-07-17" {
		t.Fatalf("unexpected first_seen range: earliest=%s latest=%s", m.EarliestFirstSeen, m.LatestFirstSeen)
	}

	wantAddedPct := 2.0 / 5.0 * 100
	if diff := m.AddedPct - wantAddedPct; diff > 0.001 || diff < -0.001 {
		t.Fatalf("expected AddedPct %.4f, got %.4f", wantAddedPct, m.AddedPct)
	}

	if len(m.TopCompanies) != 2 {
		t.Fatalf("expected 2 companies in TopCompanies (zero-yield excluded), got %d: %+v", len(m.TopCompanies), m.TopCompanies)
	}

	foundGreenhouse := false
	for _, p := range m.TopPortals {
		if p.Name == "greenhouse-api" {
			foundGreenhouse = true
			if p.Added != 1 || p.Total != 2 {
				t.Fatalf("expected greenhouse-api Added=1 Total=2, got %+v", p)
			}
		}
	}
	if !foundGreenhouse {
		t.Fatalf("expected greenhouse-api in TopPortals, got %+v", m.TopPortals)
	}
}

func TestParseScanHistoryTornRowSkipped(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "data/scan-history.tsv",
		"url\tfirst_seen\tportal\ttitle\tcompany\tstatus\tlocation\n"+
			"https://a.example/1\t2026-07-13\tgreenhouse-api\tCreative Director\tAcme\n"+ // torn: only 5 fields
			"https://b.example/1\t2026-07-14\tashby-api\tVFX Supervisor\tBeta\tadded\tLondon\n")

	m := ParseScanHistory(root)
	if m.TotalRows != 1 {
		t.Fatalf("expected torn row skipped, leaving 1 valid row, got %d", m.TotalRows)
	}
}

func TestParseScanHistoryLegacyShortRows(t *testing.T) {
	// Older rows may lack a location column entirely (6 fields).
	root := t.TempDir()
	writeFixture(t, root, "data/scan-history.tsv",
		"url\tfirst_seen\tportal\ttitle\tcompany\tstatus\n"+
			"https://a.example/1\t2026-07-13\tsomequery\tCreative Director\tAcme\tskipped_expired\n")

	m := ParseScanHistory(root)
	if m.TotalRows != 1 {
		t.Fatalf("expected 1 row, got %d", m.TotalRows)
	}
	if m.SkippedExpiredCount != 1 {
		t.Fatalf("expected 1 skipped_expired, got %d", m.SkippedExpiredCount)
	}
}
