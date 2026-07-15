package screens

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/santifer/career-ops/dashboard/internal/i18n"
	"github.com/santifer/career-ops/dashboard/internal/model"
	"github.com/santifer/career-ops/dashboard/internal/theme"
)

// ScanHistoryClosedMsg is emitted when the scan history screen is dismissed.
type ScanHistoryClosedMsg struct{}

// ScanHistoryModel implements the scan-run history screen — a table view
// over data/scan-runs.tsv (one row per `node scan.mjs` run), so the user can
// review scan cadence and duration the same way they review pipeline/progress data.
type ScanHistoryModel struct {
	metrics      model.ScanHistoryMetrics
	scrollOffset int
	width        int
	height       int
	theme        theme.Theme
}

// NewScanHistoryModel creates a new scan history screen.
func NewScanHistoryModel(t theme.Theme, metrics model.ScanHistoryMetrics, width, height int) ScanHistoryModel {
	return ScanHistoryModel{
		metrics: metrics,
		width:   width,
		height:  height,
		theme:   t,
	}
}

// Init implements tea.Model.
func (m ScanHistoryModel) Init() tea.Cmd {
	return nil
}

// Resize updates dimensions.
func (m *ScanHistoryModel) Resize(width, height int) {
	m.width = width
	m.height = height
}

// Update handles input for the scan history screen.
func (m ScanHistoryModel) Update(msg tea.Msg) (ScanHistoryModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			return m, func() tea.Msg { return ScanHistoryClosedMsg{} }
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

// View renders the scan history screen.
func (m ScanHistoryModel) View() string {
	header := m.renderHeader()
	body := m.renderTable()
	help := m.renderHelp()

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

	return lipgloss.JoinVertical(lipgloss.Left, header, body, help)
}

func (m ScanHistoryModel) renderHeader() string {
	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(m.theme.Text).
		Background(m.theme.Surface).
		Width(m.width).
		Padding(0, 2)

	title := lipgloss.NewStyle().Bold(true).Foreground(m.theme.Mauve).Render(i18n.Current.ScanHistoryTitle)

	right := lipgloss.NewStyle().Foreground(m.theme.Subtext)
	info := right.Render(fmt.Sprintf(i18n.Current.ScanHistorySummary,
		m.metrics.TotalRuns, m.metrics.AvgFoundPerRun, m.metrics.AvgNewPerRun))

	gap := m.width - lipgloss.Width(title) - lipgloss.Width(info) - 4
	if gap < 1 {
		gap = 1
	}

	return style.Render(title + strings.Repeat(" ", gap) + info)
}

func (m ScanHistoryModel) renderTable() string {
	padStyle := lipgloss.NewStyle().Padding(0, 2)
	dimStyle := lipgloss.NewStyle().Foreground(m.theme.Subtext)

	if len(m.metrics.Runs) == 0 {
		return padStyle.Render(dimStyle.Render(i18n.Current.NoData))
	}

	var lines []string

	// Summary line: filter removal % + average duration, mirrors `node stats.mjs --summary`.
	summary := dimStyle.Render(fmt.Sprintf(i18n.Current.ScanHistoryRunsInfo,
		m.metrics.FilterRemovalPct, formatDuration(m.metrics.AvgDurationMs)))
	lines = append(lines, padStyle.Render(summary), "")

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(m.theme.Sky)
	colTime, colCompanies, colFound, colNew, colDupes, colErrors, colDuration := 20, 10, 8, 6, 7, 7, 10

	headerRow := padTo(i18n.Current.ColScanTime, colTime) +
		padTo(i18n.Current.ColScanCompanies, colCompanies) +
		padTo(i18n.Current.ColScanFound, colFound) +
		padTo(i18n.Current.ColScanNew, colNew) +
		padTo(i18n.Current.ColScanDupes, colDupes) +
		padTo(i18n.Current.ColScanErrors, colErrors) +
		padTo(i18n.Current.ColScanDuration, colDuration)
	lines = append(lines, padStyle.Render(headerStyle.Render(headerRow)))

	rowStyle := lipgloss.NewStyle().Foreground(m.theme.Text)
	newStyle := lipgloss.NewStyle().Foreground(m.theme.Green)
	errStyle := lipgloss.NewStyle().Foreground(m.theme.Red)
	failedStyle := lipgloss.NewStyle().Foreground(m.theme.Red).Bold(true)

	for _, run := range m.metrics.Runs {
		if run.Status == "failed" {
			row := padTo(formatScanTimestamp(run.Timestamp), colTime) +
				failedStyle.Render(i18n.Current.ScanStatusFailed)
			lines = append(lines, padStyle.Render(row))
			continue
		}

		newCell := padTo(fmt.Sprintf("%d", run.NewAdded), colNew)
		if run.NewAdded > 0 {
			newCell = newStyle.Render(padTo(fmt.Sprintf("%d", run.NewAdded), colNew))
		}
		errCell := padTo(fmt.Sprintf("%d", run.Errors), colErrors)
		if run.Errors > 0 {
			errCell = errStyle.Render(padTo(fmt.Sprintf("%d", run.Errors), colErrors))
		}

		row := rowStyle.Render(padTo(formatScanTimestamp(run.Timestamp), colTime)) +
			rowStyle.Render(padTo(fmt.Sprintf("%d", run.Companies), colCompanies)) +
			rowStyle.Render(padTo(fmt.Sprintf("%d", run.Found), colFound)) +
			newCell +
			rowStyle.Render(padTo(fmt.Sprintf("%d", run.Dupes), colDupes)) +
			errCell +
			rowStyle.Render(padTo(formatDuration(float64(run.DurationMs)), colDuration))

		lines = append(lines, padStyle.Render(row))
	}

	return strings.Join(lines, "\n")
}

func (m ScanHistoryModel) renderHelp() string {
	style := lipgloss.NewStyle().
		Foreground(m.theme.Subtext).
		Background(m.theme.Surface).
		Width(m.width).
		Padding(0, 1)

	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(m.theme.Text)
	descStyle := lipgloss.NewStyle().Foreground(m.theme.Subtext)

	brand := lipgloss.NewStyle().Foreground(m.theme.Overlay).Render("career-ops by santifer.io")

	keys := keyStyle.Render("↑↓") + descStyle.Render(i18n.Current.HelpScroll) +
		keyStyle.Render("PgUp/Dn") + descStyle.Render(i18n.Current.HelpPage) +
		keyStyle.Render("t") + descStyle.Render(i18n.Current.HelpLanguage) +
		keyStyle.Render("Esc") + descStyle.Render(i18n.Current.HelpBack)

	gap := m.width - lipgloss.Width(keys) - lipgloss.Width(brand) - 2
	if gap < 1 {
		gap = 1
	}

	return style.Render(keys + strings.Repeat(" ", gap) + brand)
}

// padTo right-pads (or truncates) s to width columns, ASCII-safe for the
// fixed-width table cells above (company/timestamp/number strings only).
func padTo(s string, width int) string {
	if len(s) >= width {
		if width <= 1 {
			return s
		}
		return s[:width-1] + " "
	}
	return s + strings.Repeat(" ", width-len(s))
}

// formatScanTimestamp renders a raw ISO 8601 UTC timestamp (as written by
// scan.mjs) as a compact local "MM-DD HH:MM" string. Falls back to the raw
// value when parsing fails, so a malformed row never crashes the view.
func formatScanTimestamp(ts string) string {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ts
	}
	return t.Local().Format("01-02 15:04")
}

// formatDuration renders milliseconds as a short human string ("1m 47s",
// "42s"). Returns the localized "unknown" marker for zero/unset durations
// (historical rows written before duration tracking was added).
func formatDuration(ms float64) string {
	if ms <= 0 {
		return i18n.Current.ScanDurationUnknown
	}
	d := time.Duration(ms) * time.Millisecond
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) - minutes*60
	return fmt.Sprintf("%dm %ds", minutes, seconds)
}
