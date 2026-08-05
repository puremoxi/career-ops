package data

import "testing"

func TestParsePortalsConfigMissingFile(t *testing.T) {
	s := ParsePortalsConfig(t.TempDir())
	if s.TotalCompanies != 0 {
		t.Fatalf("expected zero-value summary for missing file, got %+v", s)
	}
}

func TestParsePortalsConfigCountsCompanies(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "portals.yml", `title_filter:
  positive:
    - "Creative Director"

tracked_companies:

  - name: Anthropic
    careers_url: https://job-boards.greenhouse.io/anthropic
    api: https://boards-api.greenhouse.io/v1/boards/anthropic/jobs
    enabled: true

  - name: DeadCo
    careers_url: https://example.com/careers
    notes: "Disabled after repeated 404s."
    enabled: false

  - name: NoExplicitFlag
    careers_url: https://example.org/careers

search_queries:
  - name: Ashby — Creative Technology
    query: 'site:jobs.ashbyhq.com "Creative Technologist"'
    enabled: true
`)

	s := ParsePortalsConfig(root)
	if s.TotalCompanies != 3 {
		t.Fatalf("expected 3 tracked companies, got %d", s.TotalCompanies)
	}
	if s.EnabledCompanies != 2 {
		t.Fatalf("expected 2 enabled (explicit true + default), got %d", s.EnabledCompanies)
	}
	if s.DisabledCompanies != 1 {
		t.Fatalf("expected 1 disabled, got %d", s.DisabledCompanies)
	}
	if s.TotalBoardQueries != 1 {
		t.Fatalf("expected 1 board query (search_queries entry), got %d", s.TotalBoardQueries)
	}
	if s.EnabledBoardQueries != 1 {
		t.Fatalf("expected 1 enabled board query, got %d", s.EnabledBoardQueries)
	}
	if s.DisabledBoardQueries != 0 {
		t.Fatalf("expected 0 disabled board queries, got %d", s.DisabledBoardQueries)
	}
}

func TestParsePortalsConfigBoardQueriesCountedIndependently(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "portals.yml", `search_queries:
  - name: Ashby — Creative Director
    query: 'site:jobs.ashbyhq.com "Creative Director"'
    enabled: true

  - name: Greenhouse — Creative Director
    query: 'site:job-boards.greenhouse.io "Creative Director"'
    enabled: false

  - name: Remotive — Creative Technology & Direction
    query: 'site:remotive.com "Creative Director"'

tracked_companies:
  - name: Anthropic
    careers_url: https://job-boards.greenhouse.io/anthropic
    api: https://boards-api.greenhouse.io/v1/boards/anthropic/jobs
    enabled: true
`)

	s := ParsePortalsConfig(root)
	if s.TotalBoardQueries != 3 {
		t.Fatalf("expected 3 board queries (Ashby/Greenhouse/Remotive), got %d", s.TotalBoardQueries)
	}
	if s.EnabledBoardQueries != 2 {
		t.Fatalf("expected 2 enabled (explicit true + default), got %d", s.EnabledBoardQueries)
	}
	if s.DisabledBoardQueries != 1 {
		t.Fatalf("expected 1 disabled, got %d", s.DisabledBoardQueries)
	}
	if s.TotalCompanies != 1 {
		t.Fatalf("expected board_queries block not to leak into TotalCompanies, got %d", s.TotalCompanies)
	}
}

func TestParseQueryProfiles(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "portals.yml", `search_queries:
  - name: Ashby — Creative Director
    query: 'site:jobs.ashbyhq.com "Creative Director" OR "Head of Creative" OR "VP of Creative"'
    enabled: true

tracked_companies:

  - name: Retool
    careers_url: https://retool.com/careers
    scan_method: websearch
    scan_query: 'site:retool.com/careers "Creative Director" OR "Head of Creative"'
    enabled: true

  - name: Anthropic
    careers_url: https://job-boards.greenhouse.io/anthropic
    api: https://boards-api.greenhouse.io/v1/boards/anthropic/jobs
    enabled: true
`)

	profiles := ParseQueryProfiles(root)

	if got := profiles["Ashby — Creative Director"]; got != `site:jobs.ashbyhq.com "Creative Director" OR "Head of Creative" OR "VP of Creative"` {
		t.Errorf("unexpected search_queries entry: %q", got)
	}
	if got := profiles["Retool"]; got != `site:retool.com/careers "Creative Director" OR "Head of Creative"` {
		t.Errorf("unexpected tracked_companies scan_query entry: %q", got)
	}
	if _, ok := profiles["Anthropic"]; ok {
		t.Error("expected no entry for a company with no scan_query (API-driven)")
	}
}

func TestParsePortalsConfigNoTrackedCompaniesKey(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "portals.yml", "title_filter:\n  positive:\n    - \"Creative Director\"\n")
	s := ParsePortalsConfig(root)
	if s.TotalCompanies != 0 {
		t.Fatalf("expected 0 companies when tracked_companies key is absent, got %d", s.TotalCompanies)
	}
}
