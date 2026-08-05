package screens

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/santifer/career-ops/dashboard/internal/data"
	"github.com/santifer/career-ops/dashboard/internal/i18n"
	"github.com/santifer/career-ops/dashboard/internal/theme"
)

// ScanStatsClosedMsg is emitted when the scan-effectiveness screen is dismissed.
type ScanStatsClosedMsg struct{}

// ScanRunColID identifies an optional column in the Scan Runs table. Date is
// the only non-optional column — it's the row identity.
type ScanRunColID int

const (
	ScanRunColCompanies       ScanRunColID = iota // "N 0-token / M agent" coverage, this run
	ScanRunColCompaniesConfig                     // total tracked_companies configured in portals.yml (constant)
	ScanRunColBoards                              // total search_queries (job-board queries) configured in portals.yml (constant)
	ScanRunColFound
	ScanRunColNew
	ScanRunColErrors
	ScanRunColDuration
	ScanRunColReachable // from data/portal-health.tsv, correlated by run
	ScanRunColEmpty
	ScanRunColErrored
)

// scanRunColDef describes one optional Scan Runs column for the picker UI.
// align controls how both the header and its data cells are padded — numeric
// columns right-align (so digits stack under the header instead of trailing
// off to its right), the coverage text column stays left-aligned.
type scanRunColDef struct {
	id     ScanRunColID
	header string
	width  int
	align  string // "left" or "right"
}

func getScanRunOptionalCols() []scanRunColDef {
	return []scanRunColDef{
		{ScanRunColCompanies, i18n.Current.ScanRunsColCompanies, 22, "left"},
		{ScanRunColCompaniesConfig, i18n.Current.ScanRunsColCompaniesConfig, 16, "right"},
		{ScanRunColBoards, i18n.Current.ScanRunsColBoards, 24, "right"},
		{ScanRunColFound, i18n.Current.ScanRunsColFound, 7, "right"},
		{ScanRunColNew, i18n.Current.ScanRunsColNew, 5, "right"},
		{ScanRunColErrors, i18n.Current.ScanRunsColErrors, 6, "right"},
		{ScanRunColDuration, i18n.Current.ScanRunsColDuration, 8, "right"},
		{ScanRunColReachable, i18n.Current.ScanRunsColReachable, 10, "right"},
		{ScanRunColEmpty, i18n.Current.ScanRunsColEmpty, 6, "right"},
		{ScanRunColErrored, i18n.Current.ScanRunsColErrored, 14, "right"},
	}
}

// alignPad pads s to width runes, left-padding (right-align) or
// right-padding (left-align) depending on align.
func alignPad(s string, width int, align string) string {
	if align == "right" {
		n := width - len([]rune(s))
		if n <= 0 {
			return s
		}
		return strings.Repeat(" ", n) + s
	}
	return padRunes(s, width)
}

// ScanStatsModel implements the scan-effectiveness analytics screen: how well
// the portal scanner (`node scan.mjs`) is performing over time, sourced from
// data/scan-runs.tsv (per-run counters), data/scan-history.tsv (per-URL
// portal/company yield), data/portal-health.tsv (per-company reachability),
// and portals.yml (configured company count).
type ScanStatsModel struct {
	runs          data.ScanRunMetrics
	history       data.ScanHistoryMetrics
	health        data.PortalHealthMetrics
	portalsCfg    data.PortalsConfigSummary
	queryProfiles map[string]string
	scrollOffset  int
	width         int
	height        int
	theme         theme.Theme

	// Column picker sub-state for the Scan Runs table — opened with C, closed with esc.
	colPicker    bool
	colPickerIdx int
	visibleCols  map[ScanRunColID]bool
}

// NewScanStatsModel creates a new scan-effectiveness screen. queryProfiles
// maps a portal/company name (as logged in scan-history.tsv's `portal`
// column) to the search-query text that produced it, from
// data.ParseQueryProfiles — used to show what job profile each portal row
// was actually searching for.
func NewScanStatsModel(t theme.Theme, runs data.ScanRunMetrics, history data.ScanHistoryMetrics, health data.PortalHealthMetrics, portalsCfg data.PortalsConfigSummary, queryProfiles map[string]string, width, height int) ScanStatsModel {
	visible := make(map[ScanRunColID]bool)
	for _, col := range getScanRunOptionalCols() {
		visible[col.id] = true // all columns shown by default
	}
	return ScanStatsModel{
		runs:          runs,
		history:       history,
		health:        health,
		portalsCfg:    portalsCfg,
		queryProfiles: queryProfiles,
		width:         width,
		height:        height,
		theme:         t,
		visibleCols:   visible,
	}
}

// Init implements tea.Model.
func (m ScanStatsModel) Init() tea.Cmd {
	return nil
}

// Resize updates dimensions.
func (m *ScanStatsModel) Resize(width, height int) {
	m.width = width
	m.height = height
}

// Update handles input for the scan-effectiveness screen.
func (m ScanStatsModel) Update(msg tea.Msg) (ScanStatsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.colPicker {
			return m.handleColPicker(msg)
		}
		switch msg.String() {
		case "q", "esc":
			return m, func() tea.Msg { return ScanStatsClosedMsg{} }
		case "C":
			m.colPicker = true
			m.colPickerIdx = 0
		case "down", "j":
			m.scrollOffset++
		case "up", "k":
			if m.scrollOffset > 0 {
				m.scrollOffset--
			}
		case "pgdown", "ctrl+d":
			m.scrollOffset += m.height / 2
		case "pgup", "ctrl+u":
			m.scrollOffset -= m.height / 2
			if m.scrollOffset < 0 {
				m.scrollOffset = 0
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

// handleColPicker consumes keys while the Scan Runs column picker overlay is open.
func (m ScanStatsModel) handleColPicker(msg tea.KeyMsg) (ScanStatsModel, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", "C":
		m.colPicker = false
	case "down", "j":
		m.colPickerIdx++
		if m.colPickerIdx >= len(getScanRunOptionalCols()) {
			m.colPickerIdx = len(getScanRunOptionalCols()) - 1
		}
	case "up", "k":
		m.colPickerIdx--
		if m.colPickerIdx < 0 {
			m.colPickerIdx = 0
		}
	case " ":
		col := getScanRunOptionalCols()[m.colPickerIdx]
		m.visibleCols[col.id] = !m.visibleCols[col.id]
	}
	return m, nil
}

// View renders the scan-effectiveness screen.
func (m ScanStatsModel) View() string {
	header := m.renderHeader()
	coverage := m.renderCoverageLine()
	runsTable := m.renderRunsTable()
	health := m.renderHealth()
	jobFunnel := m.renderJobFunnel()
	uniqueFunnel := m.renderUniqueFunnel()
	portals := m.renderTopPortals()
	companies := m.renderTopCompanies()
	help := m.renderHelp()

	body := lipgloss.JoinVertical(lipgloss.Left,
		coverage,
		"",
		runsTable,
		"",
		health,
		"",
		jobFunnel,
		"",
		uniqueFunnel,
		"",
		portals,
		"",
		companies,
	)

	bodyLines := strings.Split(body, "\n")
	offset := m.scrollOffset
	if offset >= len(bodyLines) {
		offset = len(bodyLines) - 1
	}
	if offset < 0 {
		offset = 0
	}
	if offset > 0 {
		bodyLines = bodyLines[offset:]
	}

	availHeight := m.height - 4 // header + help + padding
	if availHeight < 3 {
		availHeight = 3
	}
	if len(bodyLines) > availHeight {
		bodyLines = bodyLines[:availHeight]
	}

	body = strings.Join(bodyLines, "\n")

	if m.colPicker {
		body = m.overlayColPicker(body)
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, body, help)
}

func (m ScanStatsModel) renderHeader() string {
	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(m.theme.Text).
		Background(m.theme.Surface).
		Width(m.width).
		Padding(0, 2)

	title := lipgloss.NewStyle().Bold(true).Foreground(m.theme.Mauve).Render(i18n.Current.ScanStatsTitle)

	right := lipgloss.NewStyle().Foreground(m.theme.Subtext)
	info := right.Render(fmt.Sprintf(i18n.Current.ScanStatsSummary, m.runs.TotalRuns, m.history.AddedCount, m.history.CompaniesMatched, m.history.CompaniesSeen))

	gap := m.width - lipgloss.Width(title) - lipgloss.Width(info) - 4
	if gap < 1 {
		gap = 1
	}

	return style.Render(title + strings.Repeat(" ", gap) + info)
}

func (m ScanStatsModel) renderCoverageLine() string {
	padStyle := lipgloss.NewStyle().Padding(0, 2)
	dimStyle := lipgloss.NewStyle().Foreground(m.theme.Subtext)
	if m.portalsCfg.TotalCompanies == 0 {
		return padStyle.Render(dimStyle.Render(i18n.Current.NoData))
	}
	line := fmt.Sprintf(i18n.Current.ScanCoverageLine,
		m.portalsCfg.TotalCompanies, m.portalsCfg.EnabledCompanies, m.portalsCfg.DisabledCompanies,
		m.portalsCfg.TotalBoardQueries, m.portalsCfg.EnabledBoardQueries, m.portalsCfg.DisabledBoardQueries)
	return padStyle.Render(dimStyle.Render(line))
}

// scanRunCell computes one column's display text for a run row. hr/hasHealth
// carry the data/portal-health.tsv run correlated to r by minute-truncated
// timestamp (empty/false when no health data exists for that run — older
// runs predate portal-health.tsv).
func (m ScanStatsModel) scanRunCell(id ScanRunColID, r data.ScanRun, hr data.PortalHealthRun, hasHealth bool) (string, lipgloss.Color) {
	text := m.theme.Text
	subt := m.theme.Subtext
	red := m.theme.Red
	green := m.theme.Green

	switch id {
	case ScanRunColCompanies:
		// r.Companies IS the zero-token count directly — scan.mjs only ever
		// counts companies it resolved to a provider, so it never needs
		// portal-health.tsv to derive that half. AgentHandoffCompanies is a
		// separate persisted counter (added #dashboard-scan-effectiveness);
		// it is NOT r.Companies-minus-anything — scan.mjs never included
		// agent-handoff companies in Companies to begin with, so that
		// subtraction always produced ~0 and is not used here.
		if !r.HasAgentHandoffData() {
			return fmt.Sprintf("%d %s / — %s", r.Companies, i18n.Current.ScanCoverageZeroToken, i18n.Current.ScanCoverageAgent), text
		}
		return fmt.Sprintf("%d %s / %d %s", r.Companies, i18n.Current.ScanCoverageZeroToken, r.AgentHandoffCompanies, i18n.Current.ScanCoverageAgent), text
	case ScanRunColCompaniesConfig:
		// Constant across every row — the full tracked_companies count in
		// portals.yml right now, not a per-run counter (portals.yml isn't
		// snapshotted per run). Shown alongside Coverage above so a
		// filtered/partial run (e.g. `--company anthropic`) is visually
		// obvious against the true fleet size.
		return fmt.Sprintf("%d", m.portalsCfg.TotalCompanies), subt
	case ScanRunColBoards:
		// r.Boards is scan.mjs's own job_boards: counter — always 0 here
		// since this project has no job_boards: entries. The actual job
		// boards in use (Ashby, Greenhouse, Workable, Remotive, etc.) are
		// portals.yml's search_queries — a config-level count, same caveat
		// as ScanRunColCompaniesConfig above (not a per-run counter, since
		// scan.mjs never executes these itself; they're an agent/WebSearch
		// pass — see modes/scan.md Level 3).
		return fmt.Sprintf("%d", m.portalsCfg.TotalBoardQueries), subt
	case ScanRunColFound:
		return fmt.Sprintf("%d", r.Found), text
	case ScanRunColNew:
		return fmt.Sprintf("%d", r.NewAdded), text
	case ScanRunColErrors:
		if r.Errors > 0 {
			return fmt.Sprintf("%d", r.Errors), red
		}
		return fmt.Sprintf("%d", r.Errors), subt
	case ScanRunColDuration:
		if r.DurationMs > 0 {
			return fmt.Sprintf("%.0fs", float64(r.DurationMs)/1000), text
		}
		return "—", subt
	case ScanRunColReachable:
		if !hasHealth {
			return "—", subt
		}
		return fmt.Sprintf("%d", hr.Reachable), green
	case ScanRunColEmpty:
		if !hasHealth {
			return "—", subt
		}
		return fmt.Sprintf("%d", hr.Empty), subt
	case ScanRunColErrored:
		if !hasHealth {
			return "—", subt
		}
		pct := 0.0
		if hr.Companies > 0 {
			pct = float64(hr.Errored) / float64(hr.Companies) * 100
		}
		if hr.Errored > 0 {
			return fmt.Sprintf("%d (%.0f%%)", hr.Errored, pct), red
		}
		return fmt.Sprintf("%d (%.0f%%)", hr.Errored, pct), subt
	}
	return "", text
}

// renderRunsTable lists each recorded scan run — date, then every visible
// optional column from getScanRunOptionalCols() (companies/boards/found/new/
// errors/duration, plus data/portal-health.tsv's reachable/empty/errored%
// correlated by run) — newest first, capped to the most recent 12 so the
// screen stays scannable. Column visibility is user-toggleable (C key).
func (m ScanStatsModel) renderRunsTable() string {
	padStyle := lipgloss.NewStyle().Padding(0, 2)
	sectionTitle := lipgloss.NewStyle().Bold(true).Foreground(m.theme.Sky)
	dimStyle := lipgloss.NewStyle().Foreground(m.theme.Subtext)

	var lines []string
	lines = append(lines, padStyle.Render(sectionTitle.Render(i18n.Current.ScanRunsTrendTitle)))

	if len(m.runs.Runs) == 0 {
		lines = append(lines, padStyle.Render(dimStyle.Render(i18n.Current.NoData)))
		return strings.Join(lines, "\n")
	}

	avgLine := dimStyle.Render(fmt.Sprintf(i18n.Current.ScanRunsAverages, m.runs.AvgFoundPerRun, m.runs.AvgNewPerRun, m.runs.FailedRuns))
	lines = append(lines, padStyle.Render(avgLine))

	healthByKey := make(map[string]data.PortalHealthRun, len(m.health.Runs))
	for _, hr := range m.health.Runs {
		healthByKey[hr.RunKey] = hr
	}

	cols := getScanRunOptionalCols()

	headerStyle := lipgloss.NewStyle().Foreground(m.theme.Overlay).Bold(true)
	headerCells := []string{padRunes(i18n.Current.ScanRunsColDate, 8)}
	for _, col := range cols {
		if !m.visibleCols[col.id] {
			continue
		}
		headerCells = append(headerCells, alignPad(col.header, col.width, col.align))
	}
	lines = append(lines, padStyle.Render(headerStyle.Render(strings.Join(headerCells, "  "))))

	recent := m.runs.Runs
	if len(recent) > 12 {
		recent = recent[:12]
	}

	dateStyle := lipgloss.NewStyle().Foreground(m.theme.Text)
	dateFailedStyle := lipgloss.NewStyle().Foreground(m.theme.Red)

	for _, r := range recent {
		hr, hasHealth := healthByKey[runKeyFor(r.Timestamp)]

		ds := dateStyle
		if r.Status == "failed" {
			ds = dateFailedStyle
		}
		rowCells := []string{ds.Render(padRunes(shortDate(r.Timestamp), 8))}

		for _, col := range cols {
			if !m.visibleCols[col.id] {
				continue
			}
			cellText, color := m.scanRunCell(col.id, r, hr, hasHealth)
			cellStyle := lipgloss.NewStyle().Foreground(color)
			rowCells = append(rowCells, cellStyle.Render(alignPad(cellText, col.width, col.align)))
		}

		lines = append(lines, padStyle.Render(strings.Join(rowCells, "  ")))
	}

	return strings.Join(lines, "\n")
}

// overlayColPicker renders the Scan Runs column-visibility picker inline at
// the bottom of the body. SPACE toggles the focused column; ESC/q/C closes.
// Mirrors PipelineModel.overlayColPicker's UI shape.
func (m ScanStatsModel) overlayColPicker(body string) string {
	bodyLines := strings.Split(body, "\n")
	pickerWidth := 30
	padStyle := lipgloss.NewStyle().Padding(0, 2)
	borderStyle := lipgloss.NewStyle().Foreground(m.theme.Blue).Bold(true)

	var picker []string
	picker = append(picker, padStyle.Render(borderStyle.Render(i18n.Current.PickerColumnsTitle)))

	for i, col := range getScanRunOptionalCols() {
		on := m.visibleCols[col.id]
		check := "[ ]"
		checkColor := m.theme.Subtext
		if on {
			check = "[✓]"
			checkColor = m.theme.Green
		}
		style := lipgloss.NewStyle().Foreground(m.theme.Text).Width(pickerWidth)
		if i == m.colPickerIdx {
			style = style.Background(m.theme.Overlay).Bold(true)
		}
		checkStr := lipgloss.NewStyle().Foreground(checkColor).Render(check)
		row := checkStr + " " + col.header
		picker = append(picker, padStyle.Render(style.Render(row)))
	}

	bodyLines = append(bodyLines, picker...)
	return strings.Join(bodyLines, "\n")
}

// renderHealth lists companies on a persistent data/portal-health.tsv
// failure streak — the "needs pruning or fixing" list. Per-run reachable/
// empty/errored counts live in the Scan Runs table above; this section only
// covers what a single run can't show: a streak across multiple runs.
func (m ScanStatsModel) renderHealth() string {
	padStyle := lipgloss.NewStyle().Padding(0, 2)
	sectionTitle := lipgloss.NewStyle().Bold(true).Foreground(m.theme.Sky)
	dimStyle := lipgloss.NewStyle().Foreground(m.theme.Subtext)

	var lines []string
	lines = append(lines, padStyle.Render(sectionTitle.Render(i18n.Current.ScanHealthTitle)))

	if len(m.health.Runs) == 0 {
		lines = append(lines, padStyle.Render(dimStyle.Render(i18n.Current.NoData)))
		return strings.Join(lines, "\n")
	}

	deadTitleStyle := lipgloss.NewStyle().Foreground(m.theme.Peach)
	if m.health.PersistentlyDeadCount == 0 {
		lines = append(lines, padStyle.Render(dimStyle.Render(i18n.Current.ScanHealthDeadNone)))
		return strings.Join(lines, "\n")
	}
	lines = append(lines, padStyle.Render(deadTitleStyle.Render(i18n.Current.ScanHealthDeadTitle)))

	redStyle := lipgloss.NewStyle().Foreground(m.theme.Red)
	names := m.health.PersistentlyDead
	if len(names) > 20 {
		names = names[:20]
	}
	lines = append(lines, padStyle.Render(redStyle.Render("  "+strings.Join(names, ", "))))

	return strings.Join(lines, "\n")
}

// renderJobFunnel shows the cumulative, run-level raw-impression funnel
// (found → filtered out → duplicate → new added), each stage as a % of
// Found — this is the "how much of what the scanner sees survives" view,
// distinct from renderUniqueFunnel's distinct-URL breakdown below.
func (m ScanStatsModel) renderJobFunnel() string {
	padStyle := lipgloss.NewStyle().Padding(0, 2)
	sectionTitle := lipgloss.NewStyle().Bold(true).Foreground(m.theme.Sky)
	dimStyle := lipgloss.NewStyle().Foreground(m.theme.Subtext)

	var lines []string
	lines = append(lines, padStyle.Render(sectionTitle.Render(i18n.Current.ScanJobFunnelTitle)))

	if m.runs.TotalFound == 0 {
		lines = append(lines, padStyle.Render(dimStyle.Render(i18n.Current.NoData)))
		return strings.Join(lines, "\n")
	}

	type stage struct {
		label string
		count int
		color lipgloss.Color
	}
	stages := []stage{
		{i18n.Current.ScanJobFunnelFound, m.runs.TotalFound, m.theme.Blue},
		{i18n.Current.ScanJobFunnelFiltered, m.runs.TotalFilteredAll, m.theme.Yellow},
		{i18n.Current.ScanJobFunnelDupes, m.runs.TotalDupes, m.theme.Peach},
		{i18n.Current.ScanJobFunnelAdded, m.runs.TotalNewAdded, m.theme.Green},
	}

	labelW := 14
	barMaxW := m.width - labelW - 22
	if barMaxW < 10 {
		barMaxW = 10
	}

	for _, s := range stages {
		barW := 0
		if m.runs.TotalFound > 0 {
			barW = s.count * barMaxW / m.runs.TotalFound
		}
		if barW < 1 && s.count > 0 {
			barW = 1
		}
		pct := float64(s.count) / float64(m.runs.TotalFound) * 100

		barStyle := lipgloss.NewStyle().Foreground(s.color)
		labelStyle := lipgloss.NewStyle().Foreground(m.theme.Text).Width(labelW)
		countStyle := lipgloss.NewStyle().Foreground(m.theme.Subtext)

		bar := barStyle.Render(strings.Repeat("█", barW))
		label := labelStyle.Render(s.label)
		count := countStyle.Render(fmt.Sprintf("  %d (%.1f%%)", s.count, pct))

		lines = append(lines, padStyle.Render(label+bar+count))
	}

	return strings.Join(lines, "\n")
}

// renderUniqueFunnel shows the all-time scan-history status breakdown
// (added vs. filtered-by-title vs. duplicate vs. expired), one row per
// distinct URL ever seen — complements renderJobFunnel's raw-impression view.
func (m ScanStatsModel) renderUniqueFunnel() string {
	padStyle := lipgloss.NewStyle().Padding(0, 2)
	sectionTitle := lipgloss.NewStyle().Bold(true).Foreground(m.theme.Sky)
	dimStyle := lipgloss.NewStyle().Foreground(m.theme.Subtext)

	var lines []string
	lines = append(lines, padStyle.Render(sectionTitle.Render(i18n.Current.ScanFunnelTitle)))

	if m.history.TotalRows == 0 {
		lines = append(lines, padStyle.Render(dimStyle.Render(i18n.Current.NoData)))
		return strings.Join(lines, "\n")
	}

	type stage struct {
		label string
		count int
		pct   float64
		color lipgloss.Color
	}
	stages := []stage{
		{i18n.Current.ScanFunnelAdded, m.history.AddedCount, m.history.AddedPct, m.theme.Green},
		{i18n.Current.ScanFunnelTitleFiltered, m.history.SkippedTitleCount, m.history.SkippedTitlePct, m.theme.Yellow},
		{i18n.Current.ScanFunnelDup, m.history.SkippedDupCount, m.history.SkippedDupPct, m.theme.Peach},
		{i18n.Current.ScanFunnelExpired, m.history.SkippedExpiredCount, m.history.SkippedExpiredPct, m.theme.Red},
	}

	maxCount := 0
	for _, s := range stages {
		if s.count > maxCount {
			maxCount = s.count
		}
	}

	labelW := 14
	barMaxW := m.width - labelW - 22
	if barMaxW < 10 {
		barMaxW = 10
	}

	for _, s := range stages {
		barW := 0
		if maxCount > 0 {
			barW = s.count * barMaxW / maxCount
		}
		if barW < 1 && s.count > 0 {
			barW = 1
		}

		barStyle := lipgloss.NewStyle().Foreground(s.color)
		labelStyle := lipgloss.NewStyle().Foreground(m.theme.Text).Width(labelW)
		countStyle := lipgloss.NewStyle().Foreground(m.theme.Subtext)

		bar := barStyle.Render(strings.Repeat("█", barW))
		label := labelStyle.Render(s.label)
		count := countStyle.Render(fmt.Sprintf("  %d (%.1f%%)", s.count, s.pct))

		lines = append(lines, padStyle.Render(label+bar+count))
	}

	zeroYield := m.history.CompaniesZeroYield()
	if m.history.CompaniesSeen > 0 {
		zeroPct := float64(zeroYield) / float64(m.history.CompaniesSeen) * 100
		zeroStyle := lipgloss.NewStyle().Foreground(m.theme.Subtext)
		lines = append(lines, padStyle.Render(zeroStyle.Render(fmt.Sprintf(i18n.Current.ScanZeroYieldLine, zeroYield, m.history.CompaniesSeen, zeroPct))))
	}

	return strings.Join(lines, "\n")
}

// jobProfileFor resolves what search terms a scan-history `portal` value was
// actually searching for. Direct hits come from data.ParseQueryProfiles
// (search_queries + tracked_companies scan_query, keyed by name). Agent
// workers log company-driven Level-3 hits as "{Company} scan_query" or
// "{Company} scan_query (some note)" (see modes/scan.md's Level 3 workflow)
// rather than the literal portals.yml key, so that suffix is stripped before
// a second lookup. Zero-token API/local-parser rows (e.g. "greenhouse-api")
// have no single query — they scan the whole company board and rely on the
// global title_filter — so those get a fixed explanatory label instead of a
// lookup miss.
func (m ScanStatsModel) jobProfileFor(portal string) string {
	if q, ok := m.queryProfiles[portal]; ok {
		return q
	}
	if stripped, found := strings.CutSuffix(portal, " scan_query"); found {
		if q, ok := m.queryProfiles[stripped]; ok {
			return q
		}
	}
	if idx := strings.Index(portal, " scan_query ("); idx > 0 {
		if q, ok := m.queryProfiles[portal[:idx]]; ok {
			return q
		}
	}
	if strings.HasSuffix(portal, "-api") || portal == "local-parser" {
		return i18n.Current.ScanProfileAPIBoard
	}
	return i18n.Current.ScanProfileUnknown
}

// renderTopPortals renders a 4-column table: portal name, the job profile
// (search terms) it was scanning for, hit rate as added/total, and hit rate
// as a percentage.
func (m ScanStatsModel) renderTopPortals() string {
	padStyle := lipgloss.NewStyle().Padding(0, 2)
	sectionTitle := lipgloss.NewStyle().Bold(true).Foreground(m.theme.Sky)
	dimStyle := lipgloss.NewStyle().Foreground(m.theme.Subtext)

	var lines []string
	lines = append(lines, padStyle.Render(sectionTitle.Render(i18n.Current.ScanTopPortalsTitle)))

	if len(m.history.TopPortals) == 0 {
		lines = append(lines, padStyle.Render(dimStyle.Render(i18n.Current.NoData)))
		return strings.Join(lines, "\n")
	}

	const nameW, profileW, hitsW = 26, 44, 10

	headerStyle := lipgloss.NewStyle().Foreground(m.theme.Overlay).Bold(true)
	headerLine := fmt.Sprintf("%-*s  %-*s  %-*s  %s",
		nameW, i18n.Current.ScanTopPortalsColPortal,
		profileW, i18n.Current.ScanTopPortalsColProfile,
		hitsW, i18n.Current.ScanTopPortalsColHits,
		i18n.Current.ScanTopPortalsColRate,
	)
	lines = append(lines, padStyle.Render(headerStyle.Render(headerLine)))

	nameStyle := lipgloss.NewStyle().Foreground(m.theme.Text)
	profileStyle := lipgloss.NewStyle().Foreground(m.theme.Subtext)
	hitsStyle := lipgloss.NewStyle().Foreground(m.theme.Green)

	for _, p := range m.history.TopPortals {
		pct := 0.0
		if p.Total > 0 {
			pct = float64(p.Added) / float64(p.Total) * 100
		}
		name := nameStyle.Render(padRunes(truncateRunes(p.Name, nameW), nameW))
		profile := profileStyle.Render(padRunes(truncateRunes(m.jobProfileFor(p.Name), profileW), profileW))
		hits := hitsStyle.Render(padRunes(fmt.Sprintf("%d/%d", p.Added, p.Total), hitsW))
		rate := hitsStyle.Render(fmt.Sprintf("%.1f%%", pct))
		lines = append(lines, padStyle.Render(name+"  "+profile+"  "+hits+"  "+rate))
	}

	return strings.Join(lines, "\n")
}

// renderTopCompanies renders a 2-column table: company name and hit count.
func (m ScanStatsModel) renderTopCompanies() string {
	padStyle := lipgloss.NewStyle().Padding(0, 2)
	sectionTitle := lipgloss.NewStyle().Bold(true).Foreground(m.theme.Sky)
	dimStyle := lipgloss.NewStyle().Foreground(m.theme.Subtext)

	var lines []string
	lines = append(lines, padStyle.Render(sectionTitle.Render(i18n.Current.ScanTopCompaniesTitle)))

	if len(m.history.TopCompanies) == 0 {
		lines = append(lines, padStyle.Render(dimStyle.Render(i18n.Current.NoData)))
		return strings.Join(lines, "\n")
	}

	const nameW = 40

	headerStyle := lipgloss.NewStyle().Foreground(m.theme.Overlay).Bold(true)
	headerLine := fmt.Sprintf("%-*s  %s", nameW, i18n.Current.ScanTopCompaniesColCompany, i18n.Current.ScanTopCompaniesColHits)
	lines = append(lines, padStyle.Render(headerStyle.Render(headerLine)))

	nameStyle := lipgloss.NewStyle().Foreground(m.theme.Text)
	valueStyle := lipgloss.NewStyle().Foreground(m.theme.Green)

	for _, c := range m.history.TopCompanies {
		name := nameStyle.Render(padRunes(truncateRunes(c.Name, nameW), nameW))
		value := valueStyle.Render(fmt.Sprintf("%d", c.Added))
		lines = append(lines, padStyle.Render(name+"  "+value))
	}

	return strings.Join(lines, "\n")
}

// padRunes right-pads s with spaces to width runes — fmt's %-*s pads by
// byte length, which misaligns as soon as a company/portal name has a
// multi-byte rune (e.g. the "—" in several portal names already in
// portals.yml).
func padRunes(s string, width int) string {
	n := width - len([]rune(s))
	if n <= 0 {
		return s
	}
	return s + strings.Repeat(" ", n)
}

func (m ScanStatsModel) renderHelp() string {
	style := lipgloss.NewStyle().
		Foreground(m.theme.Subtext).
		Background(m.theme.Surface).
		Width(m.width).
		Padding(0, 1)

	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(m.theme.Text)
	descStyle := lipgloss.NewStyle().Foreground(m.theme.Subtext)

	if m.colPicker {
		return style.Render(
			keyStyle.Render("↑↓/jk") + descStyle.Render(i18n.Current.HelpNavigate) +
				keyStyle.Render("SPACE") + descStyle.Render(i18n.Current.HelpToggle) +
				keyStyle.Render("Esc/C") + descStyle.Render(i18n.Current.HelpClose))
	}

	brand := lipgloss.NewStyle().Foreground(m.theme.Overlay).Render("career-ops by santifer.io")

	keys := keyStyle.Render("↑↓") + descStyle.Render(i18n.Current.HelpScroll) +
		keyStyle.Render("PgUp/Dn") + descStyle.Render(i18n.Current.HelpPage) +
		keyStyle.Render("C") + descStyle.Render(i18n.Current.HelpColumns) +
		keyStyle.Render("t") + descStyle.Render(i18n.Current.HelpLanguage) +
		keyStyle.Render("Esc") + descStyle.Render(i18n.Current.HelpBack)

	gap := m.width - lipgloss.Width(keys) - lipgloss.Width(brand) - 2
	if gap < 1 {
		gap = 1
	}

	return style.Render(keys + strings.Repeat(" ", gap) + brand)
}

// shortDate trims an ISO 8601 timestamp ("2026-08-03T16:54:58.629Z") down to
// "08-03" for compact display.
func shortDate(ts string) string {
	if idx := strings.Index(ts, "T"); idx > 5 {
		ts = ts[:idx]
	}
	if len(ts) >= 10 {
		return ts[5:10]
	}
	return ts
}

// runKeyFor truncates an ISO 8601 timestamp to minute precision, matching
// data.PortalHealthRun.RunKey so the two files can be correlated per run.
func runKeyFor(ts string) string {
	if len(ts) >= 16 {
		return ts[:16]
	}
	return ts
}
