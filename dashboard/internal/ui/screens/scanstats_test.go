package screens

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/santifer/career-ops/dashboard/internal/data"
	"github.com/santifer/career-ops/dashboard/internal/theme"
)

func newTestScanStatsModel() ScanStatsModel {
	runs := data.ScanRunMetrics{
		Runs: []data.ScanRun{
			{Timestamp: "2026-08-03T17:00:28.593Z", Status: "completed", Companies: 121, Boards: 0, Found: 15374, NewAdded: 1, Errors: 0},
			{Timestamp: "2026-07-28T19:25:06.950Z", Status: "completed", Companies: 118, Boards: 0, Found: 12616, NewAdded: 9, Errors: 1},
		},
		TotalRuns:        2,
		AvgFoundPerRun:   13995,
		AvgNewPerRun:     5,
		TotalFound:       27990,
		TotalFilteredAll: 27884,
		TotalDupes:       48,
		TotalNewAdded:    10,
		FilterRemovalPct: 99.6,
	}
	history := data.ScanHistoryMetrics{
		TotalRows:           5,
		AddedCount:          2,
		SkippedDupCount:     1,
		SkippedTitleCount:   1,
		SkippedExpiredCount: 1,
		CompaniesSeen:       4,
		CompaniesMatched:    2,
		TopPortals:          []data.PortalYield{{Name: "greenhouse-api", Added: 1, Total: 2}},
		TopCompanies:        []data.CompanyYield{{Name: "Acme", Added: 1}},
	}
	health := data.PortalHealthMetrics{
		Runs: []data.PortalHealthRun{
			{RunKey: "2026-08-03T17:00", Companies: 121, Reachable: 118, Empty: 2, Errored: 1},
		},
		LatestReachable:       118,
		LatestEmpty:           2,
		LatestErrored:         1,
		LatestTotal:           121,
		PersistentlyDead:      []string{"DeadCo"},
		PersistentlyDeadCount: 1,
	}
	portalsCfg := data.PortalsConfigSummary{TotalCompanies: 213, EnabledCompanies: 211, DisabledCompanies: 2}
	queryProfiles := map[string]string{
		"greenhouse-api": "", // API-tag rows never resolve via the map — exercised for completeness
	}

	return NewScanStatsModel(theme.NewTheme("catppuccin-mocha"), runs, history, health, portalsCfg, queryProfiles, 120, 40)
}

func TestScanStatsModelEscClosesScreen(t *testing.T) {
	m := newTestScanStatsModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected a command to be returned on esc")
	}
	msg := cmd()
	if _, ok := msg.(ScanStatsClosedMsg); !ok {
		t.Fatalf("expected ScanStatsClosedMsg, got %T", msg)
	}
}

func TestScanStatsModelQClosesScreen(t *testing.T) {
	m := newTestScanStatsModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatal("expected a command to be returned on q")
	}
	msg := cmd()
	if _, ok := msg.(ScanStatsClosedMsg); !ok {
		t.Fatalf("expected ScanStatsClosedMsg, got %T", msg)
	}
}

func TestScanStatsModelScroll(t *testing.T) {
	m := newTestScanStatsModel()
	if m.scrollOffset != 0 {
		t.Fatalf("expected initial scrollOffset 0, got %d", m.scrollOffset)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.scrollOffset != 1 {
		t.Fatalf("expected scrollOffset 1 after down, got %d", m.scrollOffset)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.scrollOffset != 0 {
		t.Fatalf("expected scrollOffset 0 after up, got %d", m.scrollOffset)
	}
	// Up at 0 must not go negative.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.scrollOffset != 0 {
		t.Fatalf("expected scrollOffset clamped at 0, got %d", m.scrollOffset)
	}
}

func TestScanStatsModelResize(t *testing.T) {
	m := newTestScanStatsModel()
	m.Resize(80, 24)
	if m.width != 80 || m.height != 24 {
		t.Fatalf("expected resize to 80x24, got %dx%d", m.width, m.height)
	}
}

func TestScanStatsModelViewRendersKeyData(t *testing.T) {
	m := newTestScanStatsModel()
	out := m.View()
	for _, want := range []string{"greenhouse-api", "Acme", "DeadCo", "213"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected View() output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestScanStatsModelColPickerToggle(t *testing.T) {
	m := newTestScanStatsModel()
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("C")})
	if !m.colPicker {
		t.Fatal("expected colPicker to open on C")
	}

	firstCol := getScanRunOptionalCols()[0]
	if !m.visibleCols[firstCol.id] {
		t.Fatal("expected first column visible by default")
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if m.visibleCols[firstCol.id] {
		t.Fatal("expected first column hidden after space toggle")
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.colPicker {
		t.Fatal("expected colPicker closed after esc")
	}
	// Hidden column should no longer appear in the rendered table.
	out := m.View()
	if strings.Contains(out, firstCol.header) {
		t.Errorf("expected hidden column header %q to be absent from View(), got:\n%s", firstCol.header, out)
	}
}

func TestScanStatsModelJobProfileResolution(t *testing.T) {
	m := newTestScanStatsModel()
	m.queryProfiles = map[string]string{
		"Ashby — Creative Director": `site:jobs.ashbyhq.com "Creative Director"`,
		"Retool":                    `site:retool.com/careers "Creative Director"`,
	}

	if got := m.jobProfileFor("Ashby — Creative Director"); got != `site:jobs.ashbyhq.com "Creative Director"` {
		t.Errorf("expected direct match, got %q", got)
	}
	if got := m.jobProfileFor("Retool scan_query"); got != `site:retool.com/careers "Creative Director"` {
		t.Errorf("expected suffix-stripped match, got %q", got)
	}
	if got := m.jobProfileFor("Retool scan_query (post-title_filter-loosening recheck)"); got != `site:retool.com/careers "Creative Director"` {
		t.Errorf("expected parenthetical-stripped match, got %q", got)
	}
	if got := m.jobProfileFor("greenhouse-api"); got == "" {
		t.Error("expected non-empty fallback label for API-tag portal")
	}
	if got := m.jobProfileFor("some-unknown-portal"); got == "" {
		t.Error("expected non-empty fallback label for unresolved portal")
	}
}

func TestScanStatsModelViewHandlesEmptyMetrics(t *testing.T) {
	m := NewScanStatsModel(theme.NewTheme("catppuccin-mocha"), data.ScanRunMetrics{}, data.ScanHistoryMetrics{}, data.PortalHealthMetrics{}, data.PortalsConfigSummary{}, map[string]string{}, 120, 40)
	// Must not panic on zero-value metrics (missing files case).
	out := m.View()
	if out == "" {
		t.Fatal("expected non-empty View() output even with zero-value metrics")
	}
}
